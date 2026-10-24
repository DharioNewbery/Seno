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
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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
		return &Service{st: st, audit: audit, secret: []byte(secret)}
	}
	return &Service{
		st: st, audit: audit, url: url, publicURL: publicURL,
		secret: []byte(secret),
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Ligado informa se a integração está configurada (nil-safe p/ testes).
func (s *Service) Ligado() bool { return s != nil && s.url != "" }

// VerificarMAC valida a assinatura HMAC de uma chamada interna vinda do Monitor.
func (s *Service) VerificarMAC(corpo []byte, mac, tempo string) error {
	if s == nil || len(s.secret) == 0 {
		return contract.ErrMACInvalido
	}
	return contract.VerificarMAC(s.secret, corpo, mac, tempo, time.Minute)
}

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
				TesteAPIID:     te.ID,
				Stdin:          te.Stdin,
				StdoutEsperado: te.StdoutEsperado,
				Publico:        te.Publico,
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

// --- estado (Fase 9.3) ------------------------------------------------------

// Ciclo roda o poll do estado em loop (ticker; embutido no main). O uso
// da fila fica restrito à transferência: um job auto-reagendado com falha
// após reagendar criava ciclos duplicados — ticker não tem esse problema.
func (s *Service) Ciclo(ctx context.Context, intervalo time.Duration) {
	pollOnce := func() {
		w, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := s.PollEstado(w); err != nil {
			slog.Warn("monitor: poll de estado", "err", err)
		}
	}
	for {
		pollOnce()
		t := time.NewTicker(intervalo)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			t.Stop()
		}
	}
}

// PollEstado consome GET /interna/v1/atividades/:monitor_id/estado de cada
// atribuição transferida, grava o snapshot em monitor_estado_aluno e ingere
// definitivamente na API qualquer submissão entregue no Monitor.
func (s *Service) PollEstado(ctx context.Context) error {
	if !s.Ligado() {
		return nil
	}
	rows, err := s.st.Pool.Query(ctx,
		`SELECT id, monitor_id FROM atribuicoes WHERE monitor_id IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("poll: listando: %w", err)
	}
	type alvo struct {
		ID  int64
		Mid string
	}
	var alvos []alvo
	for rows.Next() {
		var a alvo
		if err := rows.Scan(&a.ID, &a.Mid); err != nil {
			rows.Close()
			return fmt.Errorf("poll: scan: %w", err)
		}
		alvos = append(alvos, a)
	}
	rows.Close()
	if len(alvos) > 0 {
		slog.Debug("monitor: poll de estado", "alvos", len(alvos))
	}
	for _, a := range alvos {
		est, err := s.pegaEstado(ctx, a.Mid)
		if err != nil {
			// 404 = atividade ainda não no Monitor (transferência em
			// andamento): pula; resto é retry no ciclo seguinte.
			slog.Warn("monitor: estado inacessível", "monitor_id", a.Mid, "err", err)
			continue
		}
		for _, al := range est.Alunos {
			if err := s.gravaEstado(ctx, a.ID, &al); err != nil {
				return err
			}
			if al.Estado == "entregue" {
				if _, errSync := s.SincronizarSubmissaoAluno(ctx, a.Mid, a.ID, al.AlunoAPIID); errSync != nil {
					slog.Debug("monitor: sincronização de submissão pendente", "monitor_id", a.Mid, "aluno", al.AlunoAPIID, "err", errSync)
				}
			}
		}
	}
	return nil
}

// pegaEstado assina o GET (MAC sobre tempo+"\n"+corpo — corpo vazio aqui).
func (s *Service) pegaEstado(ctx context.Context, mid string) (*contract.EstadoAtividade, error) {
	mac, tempo := contract.AssinarMAC(s.secret, time.Now(), nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.url+"/interna/v1/atividades/"+mid+"/estado", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound { // ainda não transferida
		return nil, errEstadoIndisponivel
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("estado %s: %d (%s)", mid, resp.StatusCode, string(corpo))
	}
	var est contract.EstadoAtividade
	if err := json.Unmarshal(corpo, &est); err != nil {
		return nil, fmt.Errorf("estado %s ilegível: %w", mid, err)
	}
	return &est, nil
}

var errEstadoIndisponivel = errors.New("monitor: estado não encontrado")

// gravaEstado faz o upsert do snapshot de exibição do aluno.
func (s *Service) gravaEstado(ctx context.Context, atribuicaoID int64, al *contract.EstadoAluno) error {
	var nt pgtype.Int4
	if al.NT > 0 {
		nt = pgtype.Int4{Int32: int32(al.NT), Valid: true}
	}
	var entrega pgtype.Timestamptz
	if al.UltimaSubmissao != nil {
		entrega = pgtype.Timestamptz{Time: *al.UltimaSubmissao, Valid: true}
	}
	var nota pgtype.Numeric
	if al.NotaAuto != nil {
		if err := nota.Scan(fmt.Sprint(*al.NotaAuto)); err != nil {
			return fmt.Errorf("poll: nota: %w", err)
		}
		nota.Valid = true
	}
	var prazo pgtype.Timestamptz
	if al.PrazoEfetivo != nil {
		prazo = pgtype.Timestamptz{Time: *al.PrazoEfetivo, Valid: true}
	}
	const q = `INSERT INTO monitor_estado_aluno
	    (atribuicao_id, aluno_id, estado, n_t, entregue_em, nota_auto, correcao_status, prazo_efetivo, atualizado_em)
	    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
	    ON CONFLICT (atribuicao_id, aluno_id) DO UPDATE SET
	      estado = $3, n_t = $4, entregue_em = $5, nota_auto = $6,
	      correcao_status = $7, prazo_efetivo = $8, atualizado_em = now()`
	if _, err := s.st.Pool.Exec(ctx, q, atribuicaoID, al.AlunoAPIID, al.Estado,
		nt, entrega, nota, al.CorrecaoStatus, prazo); err != nil {
		return fmt.Errorf("poll: upsert de estado aluno=%d: %w", al.AlunoAPIID, err)
	}
	return nil
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
	var jaEntregou bool
	_ = s.st.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM submissoes WHERE atribuicao_id = $1 AND aluno_id = $2)`,
		atribuicaoID, aluno.ID,
	).Scan(&jaEntregou)
	if jaEntregou {
		return "", platform.NewAPIError(
			http.StatusConflict, platform.CodeConflict,
			"Atividade já submetida.")
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

// Reexecutar aciona a reexecução de correção de um aluno no Monitor via rota interna HMAC.
func (s *Service) Reexecutar(ctx context.Context, mid string, alunoID int64) error {
	if !s.Ligado() {
		return errors.New("integração com Monitor desligada")
	}
	mac, tempo := contract.AssinarMAC(s.secret, time.Now(), nil)
	url := fmt.Sprintf("%s/interna/v1/atividades/%s/alunos/%d/reexecutar", s.url, mid, alunoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("monitor reexecutar %s/%d: %d (%s)", mid, alunoID, resp.StatusCode, string(corpo))
	}
	return nil
}

// ExcluirSubmissao aciona a exclusão de submissão no Monitor via rota interna HMAC.
func (s *Service) ExcluirSubmissao(ctx context.Context, mid string, alunoID int64) error {
	if !s.Ligado() {
		return errors.New("integração com Monitor desligada")
	}
	mac, tempo := contract.AssinarMAC(s.secret, time.Now(), nil)
	url := fmt.Sprintf("%s/interna/v1/atividades/%s/alunos/%d/submissao", s.url, mid, alunoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("monitor excluir %s/%d: %d (%s)", mid, alunoID, resp.StatusCode, string(corpo))
	}
	return nil
}

// ColetarTentativa aciona a coleta de tentativa no Monitor via rota interna HMAC
// e sincroniza a submissão gerada para as tabelas definitivas da API.
func (s *Service) ColetarTentativa(ctx context.Context, mid string, alunoID int64) error {
	if !s.Ligado() {
		return errors.New("integração com Monitor desligada")
	}
	mac, tempo := contract.AssinarMAC(s.secret, time.Now(), nil)
	url := fmt.Sprintf("%s/interna/v1/atividades/%s/alunos/%d/coletar", s.url, mid, alunoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("monitor coletar %s/%d: %d (%s)", mid, alunoID, resp.StatusCode, string(corpo))
	}
	_, _ = s.SincronizarSubmissaoAluno(ctx, mid, 0, alunoID)
	return nil
}

// DetalhesSubmissao reúne todos os dados de uma entrega do aluno no Monitor para o professor.
type DetalhesSubmissao struct {
	Submissao *SubmissaoDetalhe `json:"submissao,omitempty"`
	Correcao  *CorrecaoDetalhe  `json:"correcao,omitempty"`
	Atividade AtividadeDetalhe  `json:"atividade"`
}

type SubmissaoDetalhe struct {
	ID         int64             `json:"id"`
	AlunoID    int64             `json:"aluno_id"`
	NT         int               `json:"n_t"`
	EntregueEm time.Time         `json:"entregue_em"`
	Atrasada   bool              `json:"atrasada"`
	Respostas  map[string]string `json:"respostas"`
}

type CorrecaoDetalhe struct {
	Status      string          `json:"status"`
	NotaAuto    *float64        `json:"nota_auto,omitempty"`
	ConcluidaEm *time.Time      `json:"concluida_em,omitempty"`
	Resultados  json.RawMessage `json:"resultados,omitempty"`
}

type AtividadeDetalhe struct {
	Nome    string          `json:"nome"`
	Tarefas []TarefaDetalhe `json:"tarefas"`
}

type TarefaDetalhe struct {
	Ordem     int     `json:"ordem"`
	Nome      string  `json:"nome"`
	Enunciado string  `json:"enunciado"`
	Linguagem string  `json:"linguagem"`
	ValorPts  float64 `json:"valor_pts"`
}

// ObterSubmissao busca os detalhes da submissão no Monitor via rota interna HMAC.
func (s *Service) ObterSubmissao(ctx context.Context, mid string, alunoID int64) (*DetalhesSubmissao, error) {
	if !s.Ligado() {
		return nil, errors.New("integração com Monitor desligada")
	}
	mac, tempo := contract.AssinarMAC(s.secret, time.Now(), nil)
	url := fmt.Sprintf("%s/interna/v1/atividades/%s/alunos/%d/submissao", s.url, mid, alunoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("monitor obter submissao %s/%d: %d (%s)", mid, alunoID, resp.StatusCode, string(corpo))
	}
	var det DetalhesSubmissao
	if err := json.NewDecoder(resp.Body).Decode(&det); err != nil {
		return nil, err
	}
	return &det, nil
}

// ObterPacoteSubmissao busca o pacote completo de submissão e correção no Monitor via HMAC.
func (s *Service) ObterPacoteSubmissao(ctx context.Context, mid string, alunoID int64) (*contract.PacoteSubmissaoMonitor, error) {
	if !s.Ligado() {
		return nil, errors.New("integração com Monitor desligada")
	}
	mac, tempo := contract.AssinarMAC(s.secret, time.Now(), nil)
	url := fmt.Sprintf("%s/interna/v1/atividades/%s/alunos/%d/submissao", s.url, mid, alunoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("monitor obter pacote submissao %s/%d: %d (%s)", mid, alunoID, resp.StatusCode, string(corpo))
	}
	var pac contract.PacoteSubmissaoMonitor
	if err := json.NewDecoder(resp.Body).Decode(&pac); err != nil {
		return nil, err
	}
	if pac.MonitorID == "" {
		pac.MonitorID = mid
	}
	if pac.AlunoAPIID == 0 {
		pac.AlunoAPIID = alunoID
	}
	return &pac, nil
}

// SincronizarSubmissaoAluno consulta o Monitor para o aluno informado e ingere a submissão/correção na API.
func (s *Service) SincronizarSubmissaoAluno(ctx context.Context, mid string, atribuicaoID, alunoID int64) (int64, error) {
	pac, err := s.ObterPacoteSubmissao(ctx, mid, alunoID)
	if err != nil {
		return 0, err
	}
	if atribuicaoID > 0 && pac.AtribuicaoAPIID == 0 {
		pac.AtribuicaoAPIID = atribuicaoID
	}
	return s.IngerirSubmissao(ctx, pac)
}

// IngerirSubmissao persiste na API (submissoes, correcoes, correcao_tarefas e resultados)
// toda a informação gerada pelo Monitor após a entrega da atividade pelo aluno.
// Se a correção já tiver sido publicada pelo professor na API, preserva as notas finais e feedbacks.
func (s *Service) IngerirSubmissao(ctx context.Context, p *contract.PacoteSubmissaoMonitor) (int64, error) {
	if p == nil || p.Submissao == nil || p.Submissao.EntregueEm.IsZero() {
		return 0, nil
	}
	alunoID := p.AlunoAPIID
	if alunoID == 0 {
		alunoID = p.Submissao.AlunoID
	}
	if alunoID == 0 {
		return 0, errors.New("monitor: aluno_api_id ausente no pacote de submissão")
	}

	atribuicaoID := p.AtribuicaoAPIID
	if atribuicaoID == 0 && p.MonitorID != "" {
		err := s.st.Pool.QueryRow(ctx,
			`SELECT id FROM atribuicoes WHERE monitor_id = $1`, p.MonitorID).Scan(&atribuicaoID)
		if err != nil {
			return 0, fmt.Errorf("monitor: atribuição para monitor_id %q não encontrada: %w", p.MonitorID, err)
		}
	}
	if atribuicaoID == 0 {
		return 0, errors.New("monitor: atribuicao_api_id ausente no pacote de submissão")
	}

	tarefasAtrib, err := s.st.Q.TarefasDaAtribuicao(ctx, atribuicaoID)
	if err != nil {
		return 0, fmt.Errorf("monitor: tarefas da atribuição %d: %w", atribuicaoID, err)
	}

	type snapItem struct {
		ID   int64  `json:"id"`
		Text string `json:"text"`
	}
	type snapDoc struct {
		Tasks []snapItem `json:"tasks"`
	}
	ordemParaTarefa := make(map[int]int64, len(tarefasAtrib))
	snap := snapDoc{Tasks: make([]snapItem, 0, len(tarefasAtrib))}
	for _, ta := range tarefasAtrib {
		ordemParaTarefa[int(ta.Ordem)] = ta.TarefaID
		code := ""
		if p.Submissao.Respostas != nil {
			if c, ok := p.Submissao.Respostas[strconv.Itoa(int(ta.Ordem))]; ok {
				code = c
			} else if c, ok := p.Submissao.Respostas[strconv.FormatInt(ta.TarefaID, 10)]; ok {
				code = c
			}
		}
		snap.Tasks = append(snap.Tasks, snapItem{ID: ta.TarefaID, Text: code})
	}
	snapBytes, err := json.Marshal(snap)
	if err != nil {
		return 0, fmt.Errorf("monitor: serializando snapshot: %w", err)
	}

	comecouEm := p.Submissao.ComecouEm
	if comecouEm.IsZero() {
		comecouEm = p.Submissao.EntregueEm
	}

	// Prepara mapa de testes por tarefa caso o pacote venha sem TesteAPIID explícito.
	testesPorTarefa := make(map[int64][]gen.ListTestesTarefaRow, len(tarefasAtrib))
	for _, ta := range tarefasAtrib {
		if ts, errT := s.st.Q.ListTestesTarefa(ctx, ta.TarefaID); errT == nil {
			testesPorTarefa[ta.TarefaID] = ts
		}
	}

	corStatus := "pendente"
	if p.Correcao != nil {
		switch p.Correcao.Status {
		case "concluida":
			corStatus = "concluida"
		case "executando":
			corStatus = "executando"
		case "erro", "falhou":
			corStatus = "falhou"
		default:
			corStatus = "pendente"
		}
	}

	tx, err := s.st.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("monitor: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var submissaoID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO submissoes (atribuicao_id, aluno_id, snapshot, comecou_em, entregue_em, atrasada)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6)
		ON CONFLICT (atribuicao_id, aluno_id) DO UPDATE SET
			snapshot = EXCLUDED.snapshot,
			comecou_em = EXCLUDED.comecou_em,
			entregue_em = EXCLUDED.entregue_em,
			atrasada = EXCLUDED.atrasada
		RETURNING id`,
		atribuicaoID, alunoID, string(snapBytes), comecouEm, p.Submissao.EntregueEm, p.Submissao.Atrasada,
	).Scan(&submissaoID)
	if err != nil {
		return 0, fmt.Errorf("monitor: upsert submissoes: %w", err)
	}

	var correcaoID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO correcoes (submissao_id, status)
		VALUES ($1, $2)
		ON CONFLICT (submissao_id) DO UPDATE SET
			status = CASE WHEN correcoes.publicada_em IS NOT NULL THEN correcoes.status ELSE EXCLUDED.status END
		RETURNING id`,
		submissaoID, corStatus,
	).Scan(&correcaoID)
	if err != nil {
		return 0, fmt.Errorf("monitor: upsert correcoes: %w", err)
	}

	ctIDPorTarefa := make(map[int64]int64, len(tarefasAtrib))
	for _, ta := range tarefasAtrib {
		var ctID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO correcao_tarefas (correcao_id, tarefa_id, valor_pts)
			VALUES ($1, $2, $3)
			ON CONFLICT (correcao_id, tarefa_id) DO UPDATE SET
				valor_pts = EXCLUDED.valor_pts
			RETURNING id`,
			correcaoID, ta.TarefaID, ta.ValorPts,
		).Scan(&ctID); err != nil {
			return 0, fmt.Errorf("monitor: upsert correcao_tarefas: %w", err)
		}
		ctIDPorTarefa[ta.TarefaID] = ctID
	}

	if p.Correcao != nil {
		tarefasCor := p.Correcao.Tarefas
		if len(tarefasCor) == 0 && len(p.Correcao.Resultados) > 0 {
			var parsed struct {
				Tarefas []contract.TarefaCorrecaoMonitor `json:"tarefas"`
			}
			_ = json.Unmarshal(p.Correcao.Resultados, &parsed)
			tarefasCor = parsed.Tarefas
		}
		for _, tc := range tarefasCor {
			tarefaID := tc.TarefaAPIID
			if tarefaID == 0 {
				tarefaID = ordemParaTarefa[tc.Ordem]
			}
			ctID, ok := ctIDPorTarefa[tarefaID]
			if !ok {
				continue
			}
			if tc.NotaAuto != nil {
				notaInt := int32(math.Round(*tc.NotaAuto))
				if _, err := tx.Exec(ctx, `
					UPDATE correcao_tarefas
					SET nota_auto = $2
					WHERE id = $1`, ctID, notaInt); err != nil {
					return 0, fmt.Errorf("monitor: update nota_auto: %w", err)
				}
			}
			listaTestes := testesPorTarefa[tarefaID]
			for _, te := range tc.Testes {
				testeID := te.TesteAPIID
				if testeID == 0 && te.Ordem >= 1 && te.Ordem <= len(listaTestes) {
					testeID = listaTestes[te.Ordem-1].ID
				}
				if testeID == 0 {
					continue
				}
				tempoStr := fmt.Sprintf("%.3f", te.Tempo)
				cpuVal := te.CPU
				if cpuVal == 0 {
					cpuVal = te.Tempo
				}
				cpuStr := fmt.Sprintf("%.3f", cpuVal)
				memStr := strconv.Itoa(te.Memoria)
				if _, err := tx.Exec(ctx, `
					INSERT INTO resultados
						(correcao_tarefa_id, teste_id, status, stdout, stderr, compile_output, tempo, memoria, cpu)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
					ON CONFLICT (correcao_tarefa_id, teste_id) DO UPDATE SET
						status = EXCLUDED.status,
						stdout = EXCLUDED.stdout,
						stderr = EXCLUDED.stderr,
						compile_output = EXCLUDED.compile_output,
						tempo = EXCLUDED.tempo,
						memoria = EXCLUDED.memoria,
						cpu = EXCLUDED.cpu`,
					ctID, testeID, te.Status,
					nullIfEmpty(te.Stdout), nullIfEmpty(te.Stderr), nullIfEmpty(te.CompileOutput),
					tempoStr, memStr, cpuStr,
				); err != nil {
					return 0, fmt.Errorf("monitor: upsert resultados: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("monitor: commit ingestão: %w", err)
	}

	// Mantém monitor_estado_aluno sincronizado com os dados recebidos.
	nt := p.Submissao.NT
	if nt <= 0 {
		nt = 1
	}
	entregue := p.Submissao.EntregueEm
	var notaAuto *float64
	if p.Correcao != nil {
		notaAuto = p.Correcao.NotaAuto
	}
	_ = s.gravaEstado(ctx, atribuicaoID, &contract.EstadoAluno{
		AlunoAPIID:      alunoID,
		Estado:          "entregue",
		NT:              nt,
		UltimaSubmissao: &entregue,
		NotaAuto:        notaAuto,
		CorrecaoStatus:  corStatus,
	})

	return submissaoID, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
