// Package testar — Execução de teste (PROJETO §Execução de teste): o
// aluno roda a tarefa contra os testes PÚBLICOS da atribuição com
// execução síncrona no Judge0 (batch com wait=true — sem callback).
// Regras: precisa de tentativa aberta (não pode ter entregado), janela
// §Atribuição (início/turma/prazo), 1 objeto por execução com retenção
// de 7 dias e limite de 5 execuções por minuto por aluno (somadas, com
// mensagem do tempo de espera). O "testar" do PROFESSOR acontece no
// banco de tarefas com linguagem livre e não gera histórico.
package testar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/judge0"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// Service coordena executões de teste.
type Service struct {
	store  *store.Store
	audit  *platform.Audit
	judge0 *judge0.Client
	cfg    judge0.Config
}

func New(st *store.Store, audit *platform.Audit, cli *judge0.Client, cfg judge0.Config) *Service {
	return &Service{store: st, audit: audit, judge0: cli, cfg: cfg}
}

// Tipos de evento do Log (PROJETO §Log).
const LogKindTestar = "execucao.create"

// ResultadoTeste é a visão de um teste executado (retornado ao aluno;
// resultados são JSONB no histórico).
type ResultadoTeste struct {
	TesteID       int64   `json:"teste_id"`
	Publico       bool    `json:"publico"`
	Status        *string `json:"status,omitempty"`
	Stdout        *string `json:"stdout,omitempty"`
	Stderr        *string `json:"stderr,omitempty"`
	CompileOutput *string `json:"compile_output,omitempty"`
	Tempo         *string `json:"tempo,omitempty"`
	Memoria       *string `json:"memoria,omitempty"`
	CPU           *string `json:"cpu,omitempty"`
}

// Execucao é a visão de uma execução do histórico.
type Execucao struct {
	ID            int64           `json:"id"`
	AtribuicaoID  int64           `json:"atribuicao_id"`
	TarefaID      int64           `json:"tarefa_id"`
	AtividadeNome string          `json:"atividade_nome,omitempty"`
	Linguagem     string          `json:"linguagem"`
	Resultados    json.RawMessage `json:"resultados,omitempty"`
	CriadoEm      time.Time       `json:"criado_em"`
}

// CadastroTeste é o corpo do POST do aluno (o código vem do editor).
type CadastroTeste struct {
	TarefaID int64  `json:"tarefa_id"`
	Codigo   string `json:"codigo"`
}

// TestarProfessor é o corpo do professor no banco de tarefas (linguagem
// à escolha, sem histórico).
type CadastroTesteProfessor struct {
	Linguagem string `json:"linguagem"`
	Codigo    string `json:"codigo"`
}

func (s *Service) errIndisponivel(msg string) error {
	return platform.NewAPIError(
		http.StatusUnprocessableEntity, platform.CodeUnprocessable, msg)
}

// ============================================================ Aluno

// TestarTarefa executa o código atual contra os testes públicos e grava
// a execução do histórico (7 dias). Código vem do editor do aluno.
func (s *Service) TestarTarefa(
	ctx context.Context, actor domain.User, atribuicaoID, tarefaID int64,
	codigo string,
) (Execucao, error) {
	if !domain.HasAnyRole(actor.Roles, domain.RoleStudent) {
		return Execucao{}, platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Operação exclusiva de alunos.")
	}
	// Contexto: atribuição + turma + matrícula ativa.
	atr, err := s.store.Q.GetAtribuicao(ctx, atribuicaoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Execucao{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Atribuição não encontrada.")
		}
		return Execucao{}, err
	}
	turma, err := s.store.Q.GetTurma(ctx, atr.TurmaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Execucao{}, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Atribuição não encontrada.")
		}
		return Execucao{}, err
	}
	mat, err := s.store.Q.MatriculaAtiva(ctx, gen.MatriculaAtivaParams{
		TurmaID: turma.ID, AlunoID: actor.ID,
	})
	if err != nil {
		return Execucao{}, err
	}
	if !mat {
		return Execucao{}, platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound,
			"Atribuição não encontrada.")
	}

	// Janela (§Atribuição): início futuro, turma encerrada, prazo
	// efetivo (sem atraso) — "testar" só roda durante a atividade.
	agora := time.Now()
	if atr.Inicio.Valid && agora.Before(atr.Inicio.Time) {
		return Execucao{}, s.errIndisponivel("A atividade ainda não começou.")
	}
	if turma.EncerradoEm.Valid {
		return Execucao{}, s.errIndisponivel("A turma está encerrada.")
	}

	// Já entregou? O testar encerra com a submissão.
	if _, err := s.store.Q.GetSubmissaoAluno(ctx, gen.GetSubmissaoAlunoParams{
		AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
	}); err == nil {
		return Execucao{}, platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict,
			"Atividade já entregue — sem mais testes.")
	}
	// Sem tentativa: exige abrir a atividade primeiro.
	if _, err := s.store.Q.GetTentativa(ctx, gen.GetTentativaParams{
		AtribuicaoID: atribuicaoID, AlunoID: actor.ID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return Execucao{}, s.errIndisponivel("Abra a atividade antes de testar.")
	} else if err != nil {
		return Execucao{}, err
	}

	// Tarefa é da atividade e tem testes públicos.
	pertence := false
	var linguagem string
	tarefas, err := s.store.Q.TarefasDaAtribuicao(ctx, atribuicaoID)
	if err != nil {
		return Execucao{}, err
	}
	for _, at := range tarefas {
		if at.TarefaID == tarefaID {
			pertence = true
			linguagem = at.Linguagem
			break
		}
	}
	if !pertence {
		return Execucao{}, platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound,
			"Tarefa não está na atividade.")
	}
	pub := false
	testes, err := s.store.Q.ListTestesTarefa(ctx, tarefaID)
	if err != nil {
		return Execucao{}, err
	}
	for _, te := range testes {
		if te.Publico {
			pub = true
			break
		}
	}
	if !pub {
		return Execucao{}, s.errIndisponivel("Tarefa sem testes públicos.")
	}

	// Limite: 5 execuções por minuto por aluno (somadas) — antes de rodar.
	n, err := s.store.Q.ContarExecucoesRecentes(ctx, actor.ID)
	if err != nil {
		return Execucao{}, err
	}
	if n >= 5 {
		aguardar := time.Minute
		var pane pgtype.Timestamptz
		pane, err = s.store.Q.MaisAntiga1Minuto(ctx, actor.ID)
		if err == nil && pane.Valid {
			restante := time.Until(pane.Time.Add(time.Minute))
			if restante > 0 {
				aguardar = restante
			}
		}
		return Execucao{}, s.errIndisponivel(
			fmt.Sprintf("Limite de 5 testes por minuto: aguarde %d segundos.",
				int(aguardar.Seconds())),
		)
	}

	// Execução: batch síncrono (wait=true), um item por teste público.
	if !s.cfg.Ligada() {
		return Execucao{}, s.errIndisponivel("Judge0 não configurado.")
	}
	lingID, ok := s.cfg.LinguagemDo(linguagem)
	if !ok {
		return Execucao{}, s.errIndisponivel("Linguagem indisponível.")
	}
	var itens []judge0.SubmissaoJudge0
	var publicosID []int64
	for _, te := range testes {
		if !te.Publico {
			continue
		}
		publicosID = append(publicosID, te.ID)
		itens = append(itens, judge0.SubmissaoJudge0{
			LanguageID:     lingID,
			SourceCode:     codigo,
			Stdin:          te.Stdin,
			ExpectedOutput: te.StdoutEsperado,
		})
	}
	respostas, err := s.judge0.EnviarBatchSincrono(ctx, itens)
	if err != nil {
		return Execucao{}, s.errIndisponivel("Judge0 indisponível; tente de cedo.")
	}
	resultados := make([]ResultadoTeste, 0, len(respostas))
	for i, r := range respostas {
		if i >= len(publicosID) {
			break
		}
		resultados = append(resultados, ResultadoTeste{
			TesteID: publicosID[i], Publico: true,
			Status: ptrTexto(statusDe(r.StatusID())),
			Stdout: ptrTexto(r.Stdout), Stderr: ptrTexto(r.Stderr),
			CompileOutput: ptrTexto(r.CompileOutput),
			Tempo:         ptrTexto(r.Time), Memoria: ptrTexto(r.Memory),
			CPU: ptrTexto(r.CPUTime),
		})
	}
	venceus, err := json.Marshal(resultados)
	if err != nil {
		return Execucao{}, err
	}
	// Registra + GC de 7 dias.
	row, err := s.store.Q.InsertExecucao(ctx, gen.InsertExecucaoParams{
		AlunoID:      actor.ID,
		AtribuicaoID: atribuicaoID,
		TarefaID:     tarefaID,
		Linguagem:    linguagem,
		Codigo:       codigo,
		Resultados:   venceus,
	})
	if _, err := s.store.Q.ApagarExecucoesVencidas(ctx); err != nil {
		// Limpeza não deve bloquear o aluno.
		_ = err
	}
	if err != nil {
		return Execucao{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTestar,
		Detail: fmt.Sprintf("atribuicao=%d tarefa=%d", atribuicaoID, tarefaID),
	})
	return Execucao{
		ID: row.ID, AtribuicaoID: row.AtribuicaoID, TarefaID: row.TarefaID,
		Linguagem: row.Linguagem, Resultados: venceus,
		CriadoEm: row.CriadoEm.Time,
	}, nil
}

// Historico: as execuções do próprio aluno nos últimos 7 dias
// (resultados incluídos; detalhe completo via GetExecucaoAluno).
func (s *Service) Historico(
	ctx context.Context, actor domain.User, atribuicaoID *int64,
) ([]Execucao, error) {
	if !domain.HasAnyRole(actor.Roles, domain.RoleStudent) {
		return nil, platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Operação exclusiva de alunos.")
	}
	rows, err := s.store.Q.ListExecucoesAluno(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Execucao, 0, len(rows))
	for _, row := range rows {
		if atribuicaoID != nil && *atribuicaoID > 0 &&
			row.AtribuicaoID != *atribuicaoID {
			continue
		}
		detalhe, err := s.store.Q.GetExecucaoAluno(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, Execucao{
			ID: row.ID, AtribuicaoID: row.AtribuicaoID,
			TarefaID: row.TarefaID, AtividadeNome: row.AtividadeNome,
			Linguagem: row.Linguagem, Resultados: detalhe.Resultados,
			CriadoEm: row.CriadoEm.Time,
		})
	}
	return out, nil
}

// ============================================================ Professor

// TestarTarefaProfessor: banco de tarefas; linguagem à escolha, todos
// os testes (públicos e privados), SEM histórico. Dono|staff.
func (s *Service) TestarTarefaProfessor(
	ctx context.Context, actor domain.User, tarefaID int64, cad CadastroTesteProfessor,
) ([]ResultadoTeste, error) {
	var professorID int64
	row, err := s.store.Q.GetTarefa(ctx, tarefaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound,
				"Tarefa não encontrada.")
		}
		return nil, err
	}
	professorID = row.ProfessorID
	if !actor.IsStaff() && actor.ID != professorID {
		return nil, platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound,
			"Tarefa não encontrada.")
	}
	lingID, ok := s.cfg.LinguagemDo(strings.TrimSpace(cad.Linguagem))
	if !ok {
		return nil, s.errIndisponivel("Escolha python, c ou cpp.")
	}
	testes, err := s.store.Q.ListTestesTarefa(ctx, tarefaID)
	if err != nil {
		return nil, err
	}
	if len(testes) == 0 {
		return nil, s.errIndisponivel("Tarefa sem testes.")
	}
	if !s.cfg.Ligada() {
		return nil, s.errIndisponivel("Judge0 não configurado.")
	}
	var itens []judge0.SubmissaoJudge0
	for _, te := range testes {
		itens = append(itens, judge0.SubmissaoJudge0{
			LanguageID:     lingID,
			SourceCode:     cad.Codigo,
			Stdin:          te.Stdin,
			ExpectedOutput: te.StdoutEsperado,
		})
	}
	respostas, err := s.judge0.EnviarBatchSincrono(ctx, itens)
	if err != nil {
		return nil, s.errIndisponivel("Judge0 indisponível; tente de novo.")
	}
	out := make([]ResultadoTeste, 0, len(respostas))
	for i, r := range respostas {
		if i >= len(testes) {
			break
		}
		out = append(out, ResultadoTeste{
			TesteID: testes[i].ID, Publico: testes[i].Publico,
			Status: ptrTexto(statusDe(r.StatusID())),
			Stdout: ptrTexto(r.Stdout), Stderr: ptrTexto(r.Stderr),
			CompileOutput: ptrTexto(r.CompileOutput),
			Tempo:         ptrTexto(r.Time), Memoria: ptrTexto(r.Memory),
			CPU: ptrTexto(r.CPUTime),
		})
	}
	return out, nil
}

// ============================================================ helpers

func ptrTexto(s string) *string { return &s }

func statusDe(statusID int) string {
	if statusID == judge0.StatusAcceptedID {
		return "accepted"
	}
	return fmt.Sprintf("judge0:%d", statusID)
}
