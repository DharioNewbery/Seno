// Package monitor — integração assíncrona da API com o serviço Monitor
// (Fase 9 do PLANO_MONITOR_FLUXO_ALUNO): monta o envelope de transferência
// de uma atribuição publicada (PROJETO §Atividade/§Atribuição) e o envia
// assinado por HMAC ao POST /interna/v1/atividades (contract §2). Integração
// desligada com SENO_MONITOR_URL vazio. O Monitor NUNCA é chamado fora da
// fila: o envio é job (monitor_transferir) com backoff.
package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/seno-project/seno/contract"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/fila"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// KindTransferir é o kind do job na fila de jobs da API.
const KindTransferir = "monitor_transferir"

type Service struct {
	st        *store.Store
	audit     *platform.Audit
	url       string // base do Monitor vista pela API (ex.: http://monitor:8081)
	publicURL string // base do Monitor vista pelo NAVEGADOR (redirect)
	secret    []byte
	http      *http.Client
}

// New cria o serviço; url vazio = integração desligada (Enfileirar no-op).
func New(st *store.Store, audit *platform.Audit, url, publicURL, secret string) *Service {
	if url == "" {
		return &Service{st: st}
	}
	return &Service{
		st: st, audit: audit, url: url, publicURL: publicURL,
		secret: []byte(secret),
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Ligado informa se a integração está configurada (nil-safe p/ testes).
func (s *Service) Ligado() bool { return s != nil && s.url != "" }

type payloadTransferencia struct {
	AtribuicaoID int64 `json:"atribuicao_id"`
}

// Enfileirar agenda o envio da atribuição ao Monitor (não bloqueia a
// publicação; falha da fila nunca derruba o request — fica no slog).
func (s *Service) Enfileirar(ctx context.Context, atribuicaoID int64) {
	if !s.Ligado() {
		return
	}
	if _, err := fila.Agenda(ctx, s.st, KindTransferir,
		payloadTransferencia{AtribuicaoID: atribuicaoID}, 5); err != nil {
		slog.Error("monitor_transferir: agendando", "atribuicao", atribuicaoID, "err", err)
	}
}

// Handler devolve o handler de fila do kind (registrado no main).
func (s *Service) Handler() fila.Handler {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p payloadTransferencia
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("monitor_transferir: payload: %w", err)
		}
		t, err := s.Montar(ctx, p.AtribuicaoID)
		if err != nil {
			return err
		}
		mid, err := s.Enviar(ctx, t)
		if err != nil {
			return err
		}
		return s.gravarMonitorID(ctx, p.AtribuicaoID, mid)
	}
}

// linhas brutas da montagem --------------------------------------------------

type atribuicaoRow struct {
	TurmaID      int64
	AtividadeID  int64
	Autocomplete bool
	Inicio       *time.Time
	Prazo        *time.Time
	DuracaoSeg   *int32
	PodeAtrasado bool
	MonitorID    *string
	// Turma:
	TurmaTitulo string
	MateriaCode string
	MateriaNome string
	Trimestre   int
	Ano         int
}

// Montar monta o envelope a partir do banco (spec §2 integral: tarefas com
// TODOS os testes, públicos e privados; provados nunca saem em respostas de
// aluno — só nesta transferência assinada entre serviços).
func (s *Service) Montar(ctx context.Context, atribuicaoID int64) (*contract.Transferencia, error) {
	var a atribuicaoRow
	err := s.st.Pool.QueryRow(ctx, `
		SELECT a.turma_id, a.atividade_id, a.autocomplete, a.inicio, a.prazo,
		       a.duracao_seg, a.pode_atrasado, a.monitor_id,
		       COALESCE(t.titulo, ''), m.codigo, m.nome, p.trimestre, p.ano
		FROM atribuicoes a
		JOIN turmas t ON t.id = a.turma_id
		JOIN materias m ON m.id = t.materia_id
		JOIN periodos_letivos p ON p.id = t.periodo_id
		WHERE a.id = $1`, atribuicaoID).Scan(
		&a.TurmaID, &a.AtividadeID, &a.Autocomplete, &a.Inicio, &a.Prazo,
		&a.DuracaoSeg, &a.PodeAtrasado, &a.MonitorID,
		&a.TurmaTitulo, &a.MateriaCode, &a.MateriaNome, &a.Trimestre, &a.Ano)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("monitor_transferir: atribuição %d inexistente", atribuicaoID)
		}
		return nil, fmt.Errorf("monitor_transferir: atribuição: %w", err)
	}

	// Matrículas ativas do snapshot (u.status='ativo': ainda não convidou).
	type aluno struct {
		ID   int64
		Nome string
	}
	rows, err := s.st.Pool.Query(ctx, `
		SELECT mt.aluno_id,
		       COALESCE(TRIM(p.first_name || ' ' || p.last_name), u.email)
		FROM matriculas mt
		JOIN users u ON u.id = mt.aluno_id
		LEFT JOIN persons p ON p.id = u.person_id
		WHERE mt.turma_id = $1 AND mt.saida_em IS NULL AND u.status = 'active'
		ORDER BY mt.aluno_id`, a.TurmaID)
	if err != nil {
		return nil, fmt.Errorf("monitor_transferir: matriculas: %w", err)
	}
	defer rows.Close()
	alunos := make([]contract.Aluno, 0, 32)
	for rows.Next() {
		var al aluno
		if err := rows.Scan(&al.ID, &al.Nome); err != nil {
			return nil, fmt.Errorf("monitor_transferir: matricula: %w", err)
		}
		alunos = append(alunos, contract.Aluno{APIID: al.ID, Nome: al.Nome})
	}
	if len(alunos) == 0 {
		// Monitor valida 422 alunos vazio: falha definitiva do payload.
		return nil, fmt.Errorf("monitor_transferir: atribuição %d sem alunos ativos", atribuicaoID)
	}

	tarefas, err := s.montarCarga(ctx, a.AtividadeID)
	if err != nil {
		return nil, err
	}

	t := &contract.Transferencia{
		APIID:     atribuicaoID,
		MonitorID: ptrStr(a.MonitorID), // reenvio idempotente
		GeradaEm:  time.Now().UTC(),
		Turma: contract.Turma{
			APIID: a.TurmaID,
			Nome:  fmt.Sprintf("%s %s %d.%d", a.MateriaCode, a.TurmaTitulo, a.Trimestre, a.Ano),
		},
		Alunos: alunos,
		Janela: janela(a),
		Regras: contract.Regras{
			// fluxo canônico da API: 1 submissão por aluno; linguagens são
			// as que o professor usou por tarefa.
			SubmissoesMultiplas:  false,
			MaxSubmissoes:        1,
			LinguagensPermitidas: linguagens(tarefas),
			Autocomplete:         a.Autocomplete,
		},
		Carga: contract.Carga{
			Atividade: contract.AtividadeCarga{APIID: a.AtividadeID, Nome: atividadeNome(ctx, s.st, a.AtividadeID)},
			Tarefas:   tarefas,
		},
	}
	if t.Turma.Nome == "" { // degenerate: materia vazia
		t.Turma.Nome = fmt.Sprintf("turma %d", a.TurmaID)
	}
	return t, nil
}

// janela (§Atribuição): sem início = disponível ao publicar (hora da
// transferência); sem prazo = aberta até encerramento — POCO: o Monitor exige
// prazo: usa encerramento da turma quando existente, senão horizonte largo.
func janela(a atribuicaoRow) contract.Janela {
	inicio := a.Inicio
	if inicio == nil {
		inicio = a.Prazo
	}
	base := time.Now().UTC()
	if inicio == nil {
		inicio = &base
	}
	prazo, limite := a.Prazo, (*string)(nil)
	if prazo == nil {
		far := base.AddDate(5, 0, 0)
		prazo = &far
	}
	if a.PodeAtrasado && a.Prazo == nil {
		limite = nil // atraso sem prazo: até encerramento (Monitor decide)
	}
	var dur *int
	if a.DuracaoSeg != nil {
		d := int(*a.DuracaoSeg)
		dur = &d
	}
	return contract.Janela{
		Inicio: *inicio, Prazo: *prazo, DuracaoSeg: dur,
		PodeAtrasado: a.PodeAtrasado, LimiteAtraso: limite,
	}
}

func (s *Service) montarCarga(ctx context.Context, atividadeID int64) ([]contract.TarefaCarga, error) {
	type vinc struct {
		TarefaID  int64
		Ordem     int32
		ValorPts  int32
		Linguagem string
		Nome      string
		Enunciado string
		CPUMs     *int32
		TotalMs   *int32
		MemoriaMB *int32
	}
	rows, err := s.st.Pool.Query(ctx, `
		SELECT at.tarefa_id, at.ordem, at.valor_pts, at.linguagem,
		       t.nome, t.enunciado, t.tempo_cpu_ms, t.tempo_total_ms, t.memoria_mb
		FROM atividade_tarefas at
		JOIN tarefas t ON t.id = at.tarefa_id
		WHERE at.atividade_id = $1 AND t.deleted_at IS NULL
		ORDER BY at.ordem, at.tarefa_id`, atividadeID)
	if err != nil {
		return nil, fmt.Errorf("monitor_transferir: tarefas: %w", err)
	}
	defer rows.Close()
	var vincs []vinc
	for rows.Next() {
		var v vinc
		if err := rows.Scan(&v.TarefaID, &v.Ordem, &v.ValorPts, &v.Linguagem,
			&v.Nome, &v.Enunciado, &v.CPUMs, &v.TotalMs, &v.MemoriaMB); err != nil {
			return nil, fmt.Errorf("monitor_transferir: tarefa: %w", err)
		}
		vincs = append(vincs, v)
	}
	out := make([]contract.TarefaCarga, 0, len(vincs))
	for _, v := range vincs {
		testes, err := s.st.Q.ListTestesTarefa(ctx, v.TarefaID)
		if err != nil {
			return nil, fmt.Errorf("monitor_transferir: testes da tarefa %d: %w", v.TarefaID, err)
		}
		tes := make([]contract.TesteCarga, 0, len(testes))
		for _, te := range testes {
			tes = append(tes, contract.TesteCarga{
				Stdin: te.Stdin, StdoutEsperado: te.StdoutEsperado, Publico: te.Publico,
			})
		}
		var limites *contract.Limites
		if v.CPUMs != nil { // limites definidos na tarefa; senão padrão global do Monitor
			limites = &contract.Limites{
				TempoCPUMs: int(*v.CPUMs), TempoTotalMs: int(derefC(v.TotalMs)),
				MemoriaMB: int(derefC(v.MemoriaMB)),
			}
		}
		out = append(out, contract.TarefaCarga{
			Ordem: int(v.Ordem), TarefaAPIID: v.TarefaID, Nome: v.Nome,
			ValorPts: float64(v.ValorPts), Enunciado: v.Enunciado,
			Linguagem: v.Linguagem, Limites: limites, Testes: tes,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("monitor_transferir: atividade %d sem tarefas ativas", atividadeID)
	}
	return out, nil
}

func atividadeNome(ctx context.Context, st *store.Store, id int64) string {
	var nome string
	_ = st.Pool.QueryRow(ctx, `SELECT nome FROM atividades WHERE id = $1`, id).Scan(&nome)
	return nome
}

func linguagens(tarefas []contract.TarefaCarga) []string {
	várias := map[string]bool{}
	var out []string
	for _, t := range tarefas {
		if t.Linguagem != "" && !várias[t.Linguagem] {
			várias[t.Linguagem] = true
			out = append(out, t.Linguagem)
		}
	}
	return out
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefC(v *int32) int {
	if v == nil {
		return 0
	}
	return int(*v)
}

// Enviar assina e envia (spec §2), com idempotência por api_id: o Monitor
// devolve 200 + monitor_id existente no reenvio.
func (s *Service) Enviar(ctx context.Context, t *contract.Transferencia) (string, error) {
	corpo, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("monitor_transferir: serializando: %w", err)
	}
	mac, tempo := contract.AssinarMAC(s.secret, time.Now(), corpo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.url+"/interna/v1/atividades", bytes.NewReader(corpo))
	if err != nil {
		return "", fmt.Errorf("monitor_transferir: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("monitor_transferir: POST: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("monitor_transferir: Monitor devolveu %d (%s): %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
	}
	var out struct {
		MonitorID string `json:"monitor_id"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.MonitorID == "" {
		return "", fmt.Errorf("monitor_transferir: resposta ilegível: %s", string(body))
	}
	return out.MonitorID, nil
}

// gravarMonitorID persiste o mapa API↔Monitor (idempotente).
func (s *Service) gravarMonitorID(ctx context.Context, atribuicaoID int64, mid string) error {
	_, err := s.st.Pool.Exec(ctx, `UPDATE atribuicoes SET monitor_id = $2 WHERE id = $1`,
		atribuicaoID, mid)
	if err != nil {
		return fmt.Errorf("monitor_transferir: gravando monitor_id: %w", err)
	}
	return nil
}

// URLDevolve no /entrar (Fase 9.1): sessão real + matrícula ativa (BFF já
// autenticou) e emite o token `st` do SSO (§3) com a URL de entrada do
// Monitor vista pelo navegador.
func (s *Service) Entrar(ctx context.Context, aluno domain.User, atribuicaoID int64) (string, error) {
	if !s.Ligado() {
		return "", platform.NewAPIError(
			http.StatusServiceUnavailable, platform.CodeConflict,
			"Realização pelo Monitor desligada nesta instalação.")
	}
	if !domain.HasAnyRole(aluno.Roles, domain.RoleStudent) {
		return "", platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Operação exclusiva de alunos.")
	}
	var turmaID int64
	var mid *string
	if err := s.st.Pool.QueryRow(ctx,
		`SELECT turma_id, monitor_id FROM atribuicoes WHERE id = $1`, atribuicaoID,
	).Scan(&turmaID, &mid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", platform.NewAPIError(
				http.StatusNotFound, platform.CodeNotFound, "Atribuição não encontrada.")
		}
		return "", err
	}
	mat, err := s.st.Q.MatriculaAtiva(ctx, gen.MatriculaAtivaParams{
		TurmaID: turmaID, AlunoID: aluno.ID,
	})
	if err != nil {
		return "", err
	}
	if !mat {
		return "", platform.NewAPIError(
			http.StatusNotFound, platform.CodeNotFound, "Atribuição não encontrada.")
	}
	if mid == nil || *mid == "" {
		return "", platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict,
			"Atribuição ainda não transferida ao Monitor.")
	}
	jti, err := contract.STNonce()
	if err != nil {
		return "", err
	}
	st, err := contract.AssinarST(s.secret, time.Now(), 5*time.Minute,
		contract.SSOIss, contract.SSOAud, aluno.ID, *mid, jti)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(s.publicURL, "/") + "/entrar?st=" + st, nil
}
