// Package analytics provê agregações e métricas de desempenho para turmas,
// atribuições, tarefas e alunos (visão professor), além do painel de administração.
package analytics

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

type Service struct {
	store *store.Store
}

func NewService(st *store.Store) *Service {
	return &Service{store: st}
}

// ------------------------------ Modelos ------------------------------

type AtribuicaoAnalyticsItem struct {
	AtribuicaoID  int64      `json:"atribuicao_id"`
	AtividadeNome string     `json:"atividade_nome"`
	Prazo         *time.Time `json:"prazo,omitempty"`
	TotalEntregas int64      `json:"total_entregas"`
	TaxaEntrega   float64    `json:"taxa_entrega"`
	TotalAtrasos  int64      `json:"total_atrasos"`
	TaxaAtraso    float64    `json:"taxa_atraso"`
	NotaMedia     *float64   `json:"nota_media,omitempty"`
	NotaMediana   *float64   `json:"nota_mediana,omitempty"`
	TempoMedioSeg *float64   `json:"tempo_medio_seg,omitempty"`
}

type TurmaAnalytics struct {
	TurmaID           int64                     `json:"turma_id"`
	TotalMatriculados int64                     `json:"total_matriculados"`
	Atribuicoes       []AtribuicaoAnalyticsItem `json:"atribuicoes"`
}

type VereditoCount struct {
	Status   string `json:"status"`
	Contagem int64  `json:"contagem"`
}

type TarefaAtribuicaoAnalytics struct {
	TarefaID        int64           `json:"tarefa_id"`
	TarefaNome      string          `json:"tarefa_nome"`
	Ordem           int32           `json:"ordem"`
	ValorPts        int32           `json:"valor_pts"`
	TotalAvaliadas  int64           `json:"total_avaliadas"`
	TaxaAcerto      float64         `json:"taxa_acerto"`
	NotaMedia       *float64        `json:"nota_media,omitempty"`
	VereditosComuns []VereditoCount `json:"vereditos_comuns"`
}

type AtribuicaoAnalytics struct {
	AtribuicaoID  int64                       `json:"atribuicao_id"`
	AtividadeNome string                      `json:"atividade_nome"`
	Tarefas       []TarefaAtribuicaoAnalytics `json:"tarefas"`
}

type AlunoAtribuicaoLinhaTempo struct {
	AtribuicaoID  int64      `json:"atribuicao_id"`
	AtividadeNome string     `json:"atividade_nome"`
	Prazo         *time.Time `json:"prazo,omitempty"`
	Estado        string     `json:"estado"`
	EntregueEm    *time.Time `json:"entregue_em,omitempty"`
	Atrasada      bool       `json:"atrasada"`
	TempoGastoSeg *float64   `json:"tempo_gasto_seg,omitempty"`
	NotaPublicada *int32     `json:"nota_publicada,omitempty"`
}

type AlunoTurmaAnalytics struct {
	AlunoID      int64                       `json:"aluno_id"`
	NomeCompleto string                      `json:"nome_completo"`
	Email        string                      `json:"email"`
	Atribuicoes  []AlunoAtribuicaoLinhaTempo `json:"atribuicoes"`
}

type TarefaAnalytics struct {
	TarefaID                 int64    `json:"tarefa_id"`
	Nome                     string   `json:"nome"`
	Linguagem                string   `json:"linguagem"`
	TotalAtividades          int64    `json:"total_atividades"`
	TotalAtribuicoes         int64    `json:"total_atribuicoes"`
	TotalSubmissoesAvaliadas int64    `json:"total_submissoes_avaliadas"`
	TaxaAcertoGeral          float64  `json:"taxa_acerto_geral"`
	NotaMediaGeral           *float64 `json:"nota_media_geral,omitempty"`
}

type AdminMetricas struct {
	UsuariosPorCargo    map[string]int64 `json:"usuarios_por_cargo"`
	UsuariosPorStatus   map[string]int64 `json:"usuarios_por_status"`
	TotalTurmasAtivas   int64            `json:"total_turmas_ativas"`
	SubmissoesPorStatus map[string]int64 `json:"submissoes_por_status"`
	FilaJudge0          map[string]int64 `json:"fila_judge0"`
	FalhasUltimos7Dias  int64            `json:"falhas_ultimos_7_dias"`
}

// ------------------------------ Serviços ------------------------------

func (s *Service) checarTurma(ctx context.Context, actor domain.User, turmaID int64) error {
	if actor.IsStaff() {
		return nil
	}
	var profID int64
	err := s.store.Pool.QueryRow(ctx, `SELECT professor_id FROM turmas WHERE id = $1`, turmaID).Scan(&profID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Turma não encontrada.")
		}
		return err
	}
	if profID != actor.ID {
		return platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Turma não encontrada.")
	}
	return nil
}

func (s *Service) checarAtribuicao(ctx context.Context, actor domain.User, atribuicaoID int64) (int64, error) {
	var turmaID, profID int64
	err := s.store.Pool.QueryRow(ctx, `
		SELECT t.id, t.professor_id
		FROM atribuicoes a
		JOIN turmas t ON t.id = a.turma_id
		WHERE a.id = $1`, atribuicaoID).Scan(&turmaID, &profID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Atribuição não encontrada.")
		}
		return 0, err
	}
	if !actor.IsStaff() && profID != actor.ID {
		return 0, platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Atribuição não encontrada.")
	}
	return turmaID, nil
}

func (s *Service) checarTarefa(ctx context.Context, actor domain.User, tarefaID int64) error {
	if actor.IsStaff() {
		return nil
	}
	var profID int64
	err := s.store.Pool.QueryRow(ctx, `SELECT professor_id FROM tarefas WHERE id = $1`, tarefaID).Scan(&profID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Tarefa não encontrada.")
		}
		return err
	}
	if profID != actor.ID {
		return platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Tarefa não encontrada.")
	}
	return nil
}

// TurmaAnalytics: métricas por atribuição da turma.
func (s *Service) TurmaAnalytics(ctx context.Context, actor domain.User, turmaID int64) (*TurmaAnalytics, error) {
	if err := s.checarTurma(ctx, actor, turmaID); err != nil {
		return nil, err
	}

	var totalMatriculados int64
	if err := s.store.Pool.QueryRow(ctx, `
		SELECT count(*) FROM matriculas WHERE turma_id = $1 AND saida_em IS NULL`, turmaID).Scan(&totalMatriculados); err != nil {
		return nil, err
	}

	rows, err := s.store.Pool.Query(ctx, `
		SELECT
			a.id AS atribuicao_id,
			atv.nome AS atividade_nome,
			a.prazo,
			count(DISTINCT s.aluno_id) AS total_entregas,
			count(DISTINCT CASE WHEN s.atrasada THEN s.aluno_id END) AS total_atrasos,
			avg(notas.nota)::float8 AS nota_media,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY notas.nota)::float8 AS nota_mediana,
			avg(EXTRACT(EPOCH FROM (s.entregue_em - s.comecou_em)))::float8 AS tempo_medio_seg
		FROM atribuicoes a
		JOIN atividades atv ON atv.id = a.atividade_id
		LEFT JOIN submissoes s ON s.atribuicao_id = a.id
		LEFT JOIN LATERAL (
			SELECT sum(coalesce(ct.nota_final, ct.nota_auto)) AS nota
			FROM correcoes c
			JOIN correcao_tarefas ct ON ct.correcao_id = c.id
			WHERE c.submissao_id = s.id AND c.publicada_em IS NOT NULL
			GROUP BY c.id
		) notas ON true
		WHERE a.turma_id = $1
		GROUP BY a.id, atv.nome, a.prazo
		ORDER BY a.prazo NULLS LAST, a.id`, turmaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var itens []AtribuicaoAnalyticsItem
	for rows.Next() {
		var it AtribuicaoAnalyticsItem
		var prazo pgtype.Timestamptz
		var nMedia, nMediana, tMedio pgtype.Float8
		if err := rows.Scan(
			&it.AtribuicaoID,
			&it.AtividadeNome,
			&prazo,
			&it.TotalEntregas,
			&it.TotalAtrasos,
			&nMedia,
			&nMediana,
			&tMedio,
		); err != nil {
			return nil, err
		}
		if prazo.Valid {
			t := prazo.Time
			it.Prazo = &t
		}
		if nMedia.Valid {
			it.NotaMedia = &nMedia.Float64
		}
		if nMediana.Valid {
			it.NotaMediana = &nMediana.Float64
		}
		if tMedio.Valid {
			it.TempoMedioSeg = &tMedio.Float64
		}
		if totalMatriculados > 0 {
			it.TaxaEntrega = float64(it.TotalEntregas) / float64(totalMatriculados) * 100
		}
		if it.TotalEntregas > 0 {
			it.TaxaAtraso = float64(it.TotalAtrasos) / float64(it.TotalEntregas) * 100
		}
		itens = append(itens, it)
	}

	return &TurmaAnalytics{
		TurmaID:           turmaID,
		TotalMatriculados: totalMatriculados,
		Atribuicoes:       itens,
	}, nil
}

// AtribuicaoAnalytics: métricas por tarefa na atribuição.
func (s *Service) AtribuicaoAnalytics(ctx context.Context, actor domain.User, atribuicaoID int64) (*AtribuicaoAnalytics, error) {
	if _, err := s.checarAtribuicao(ctx, actor, atribuicaoID); err != nil {
		return nil, err
	}

	var atvNome string
	if err := s.store.Pool.QueryRow(ctx, `
		SELECT atv.nome
		FROM atribuicoes a
		JOIN atividades atv ON atv.id = a.atividade_id
		WHERE a.id = $1`, atribuicaoID).Scan(&atvNome); err != nil {
		return nil, err
	}

	rows, err := s.store.Pool.Query(ctx, `
		SELECT
			t.id AS tarefa_id,
			t.nome AS tarefa_nome,
			at.ordem,
			at.valor_pts,
			count(ct.id) AS total_avaliadas,
			count(CASE WHEN coalesce(ct.nota_final, ct.nota_auto) >= at.valor_pts THEN 1 END) AS total_acertos,
			avg(coalesce(ct.nota_final, ct.nota_auto))::float8 AS nota_media
		FROM atribuicoes a
		JOIN atividade_tarefas at ON at.atividade_id = a.atividade_id
		JOIN tarefas t ON t.id = at.tarefa_id
		LEFT JOIN submissoes s ON s.atribuicao_id = a.id
		LEFT JOIN correcoes c ON c.submissao_id = s.id AND c.status = 'concluida'
		LEFT JOIN correcao_tarefas ct ON ct.correcao_id = c.id AND ct.tarefa_id = t.id
		WHERE a.id = $1
		GROUP BY t.id, t.nome, at.ordem, at.valor_pts
		ORDER BY at.ordem, t.id`, atribuicaoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tarefas []TarefaAtribuicaoAnalytics
	for rows.Next() {
		var ta TarefaAtribuicaoAnalytics
		var nMedia pgtype.Float8
		var totalAcertos int64
		if err := rows.Scan(
			&ta.TarefaID,
			&ta.TarefaNome,
			&ta.Ordem,
			&ta.ValorPts,
			&ta.TotalAvaliadas,
			&totalAcertos,
			&nMedia,
		); err != nil {
			return nil, err
		}
		if nMedia.Valid {
			ta.NotaMedia = &nMedia.Float64
		}
		if ta.TotalAvaliadas > 0 {
			ta.TaxaAcerto = float64(totalAcertos) / float64(ta.TotalAvaliadas) * 100
		}
		tarefas = append(tarefas, ta)
	}

	// Carrega vereditos do Judge0 mais comuns por tarefa
	for i := range tarefas {
		tID := tarefas[i].TarefaID
		vRows, err := s.store.Pool.Query(ctx, `
			SELECT coalesce(r.status, 'desconhecido'), count(*) AS cont
			FROM submissoes s
			JOIN correcoes c ON c.submissao_id = s.id
			JOIN correcao_tarefas ct ON ct.correcao_id = c.id
			JOIN resultados r ON r.correcao_tarefa_id = ct.id
			WHERE s.atribuicao_id = $1 AND ct.tarefa_id = $2
			GROUP BY r.status
			ORDER BY cont DESC
			LIMIT 5`, atribuicaoID, tID)
		if err != nil {
			continue
		}
		for vRows.Next() {
			var vc VereditoCount
			if err := vRows.Scan(&vc.Status, &vc.Contagem); err == nil {
				tarefas[i].VereditosComuns = append(tarefas[i].VereditosComuns, vc)
			}
		}
		vRows.Close()
	}

	return &AtribuicaoAnalytics{
		AtribuicaoID:  atribuicaoID,
		AtividadeNome: atvNome,
		Tarefas:       tarefas,
	}, nil
}

// AlunoTurmaAnalytics: linha do tempo e desempenho do aluno na turma.
func (s *Service) AlunoTurmaAnalytics(ctx context.Context, actor domain.User, turmaID int64, alunoID int64) (*AlunoTurmaAnalytics, error) {
	if err := s.checarTurma(ctx, actor, turmaID); err != nil {
		return nil, err
	}

	var firstName, lastName, email string
	err := s.store.Pool.QueryRow(ctx, `
		SELECT p.first_name, p.last_name, u.email
		FROM matriculas m
		JOIN users u ON u.id = m.aluno_id
		JOIN persons p ON p.id = u.person_id
		WHERE m.turma_id = $1 AND m.aluno_id = $2`, turmaID, alunoID).Scan(&firstName, &lastName, &email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, platform.NewAPIError(http.StatusNotFound, platform.CodeNotFound, "Aluno não matriculado nesta turma.")
		}
		return nil, err
	}

	rows, err := s.store.Pool.Query(ctx, `
		SELECT
			a.id AS atribuicao_id,
			atv.nome AS atividade_nome,
			a.prazo,
			s.entregue_em,
			s.atrasada,
			EXTRACT(EPOCH FROM (s.entregue_em - s.comecou_em))::float8 AS tempo_gasto,
			tt.revisao AS tentativa_revisao,
			pub.nota
		FROM atribuicoes a
		JOIN atividades atv ON atv.id = a.atividade_id
		LEFT JOIN submissoes s ON s.atribuicao_id = a.id AND s.aluno_id = $2
		LEFT JOIN tentativas tt ON tt.atribuicao_id = a.id AND tt.aluno_id = $2
		LEFT JOIN LATERAL (
			SELECT sum(coalesce(ct.nota_final, ct.nota_auto))::int AS nota
			FROM correcoes c
			JOIN correcao_tarefas ct ON ct.correcao_id = c.id
			WHERE c.submissao_id = s.id AND c.publicada_em IS NOT NULL
			GROUP BY c.id
		) pub ON true
		WHERE a.turma_id = $1
		ORDER BY a.prazo NULLS FIRST, a.id`, turmaID, alunoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atribuicoes []AlunoAtribuicaoLinhaTempo
	for rows.Next() {
		var alt AlunoAtribuicaoLinhaTempo
		var prazo, entregueEm pgtype.Timestamptz
		var atrasada pgtype.Bool
		var tempoGasto pgtype.Float8
		var tentativaRev pgtype.Int4
		var notaPub pgtype.Int4

		if err := rows.Scan(
			&alt.AtribuicaoID,
			&alt.AtividadeNome,
			&prazo,
			&entregueEm,
			&atrasada,
			&tempoGasto,
			&tentativaRev,
			&notaPub,
		); err != nil {
			return nil, err
		}

		if prazo.Valid {
			t := prazo.Time
			alt.Prazo = &t
		}
		if entregueEm.Valid {
			t := entregueEm.Time
			alt.EntregueEm = &t
			alt.Estado = "entregue"
		} else if tentativaRev.Valid {
			alt.Estado = "rascunho"
		} else {
			alt.Estado = "pendente"
		}
		alt.Atrasada = atrasada.Bool
		if tempoGasto.Valid {
			alt.TempoGastoSeg = &tempoGasto.Float64
		}
		if notaPub.Valid {
			alt.NotaPublicada = &notaPub.Int32
		}
		atribuicoes = append(atribuicoes, alt)
	}

	return &AlunoTurmaAnalytics{
		AlunoID:      alunoID,
		NomeCompleto: firstName + " " + lastName,
		Email:        email,
		Atribuicoes:  atribuicoes,
	}, nil
}

// TarefaAnalytics: métricas de uso de uma tarefa em todas as atividades do professor.
func (s *Service) TarefaAnalytics(ctx context.Context, actor domain.User, tarefaID int64) (*TarefaAnalytics, error) {
	if err := s.checarTarefa(ctx, actor, tarefaID); err != nil {
		return nil, err
	}

	var nome, linguagem string
	err := s.store.Pool.QueryRow(ctx, `SELECT nome, linguagem FROM tarefas WHERE id = $1`, tarefaID).Scan(&nome, &linguagem)
	if err != nil {
		return nil, err
	}

	var res TarefaAnalytics
	res.TarefaID = tarefaID
	res.Nome = nome
	res.Linguagem = linguagem

	var totalAcertos int64
	var nMedia pgtype.Float8

	err = s.store.Pool.QueryRow(ctx, `
		SELECT
			count(DISTINCT at.atividade_id) AS total_atividades,
			count(DISTINCT a.id) AS total_atribuicoes,
			count(ct.id) AS total_submissoes,
			count(CASE WHEN coalesce(ct.nota_final, ct.nota_auto) >= ct.valor_pts THEN 1 END) AS total_acertos,
			avg(coalesce(ct.nota_final, ct.nota_auto))::float8 AS nota_media
		FROM tarefas t
		LEFT JOIN atividade_tarefas at ON at.tarefa_id = t.id
		LEFT JOIN atividades atv ON atv.id = at.atividade_id AND atv.deleted_at IS NULL
		LEFT JOIN atribuicoes a ON a.atividade_id = atv.id
		LEFT JOIN submissoes s ON s.atribuicao_id = a.id
		LEFT JOIN correcoes c ON c.submissao_id = s.id AND c.status = 'concluida'
		LEFT JOIN correcao_tarefas ct ON ct.correcao_id = c.id AND ct.tarefa_id = t.id
		WHERE t.id = $1`, tarefaID).Scan(
		&res.TotalAtividades,
		&res.TotalAtribuicoes,
		&res.TotalSubmissoesAvaliadas,
		&totalAcertos,
		&nMedia,
	)
	if err != nil {
		return nil, err
	}

	if nMedia.Valid {
		res.NotaMediaGeral = &nMedia.Float64
	}
	if res.TotalSubmissoesAvaliadas > 0 {
		res.TaxaAcertoGeral = float64(totalAcertos) / float64(res.TotalSubmissoesAvaliadas) * 100
	}

	return &res, nil
}

// AdminMetricas: métricas globais para equipe staff (admin/super).
func (s *Service) AdminMetricas(ctx context.Context, actor domain.User) (*AdminMetricas, error) {
	if !actor.IsStaff() {
		return nil, platform.NewAPIError(http.StatusForbidden, platform.CodeForbidden, "Acesso restrito à administração.")
	}

	metricas := &AdminMetricas{
		UsuariosPorCargo:    make(map[string]int64),
		UsuariosPorStatus:   make(map[string]int64),
		SubmissoesPorStatus: make(map[string]int64),
		FilaJudge0:          make(map[string]int64),
	}

	// Usuários por cargo
	cRows, err := s.store.Pool.Query(ctx, `SELECT r, count(*) FROM users u, unnest(u.roles) r WHERE u.deleted_at IS NULL GROUP BY r`)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var role string
			var count int64
			if err := cRows.Scan(&role, &count); err == nil {
				metricas.UsuariosPorCargo[role] = count
			}
		}
	}

	// Usuários por status
	sRows, err := s.store.Pool.Query(ctx, `SELECT status, count(*) FROM users WHERE deleted_at IS NULL GROUP BY status`)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var st string
			var count int64
			if err := sRows.Scan(&st, &count); err == nil {
				metricas.UsuariosPorStatus[st] = count
			}
		}
	}

	// Total turmas ativas
	_ = s.store.Pool.QueryRow(ctx, `SELECT count(*) FROM turmas WHERE encerrado_em IS NULL`).Scan(&metricas.TotalTurmasAtivas)

	// Correções / submissões por status
	subRows, err := s.store.Pool.Query(ctx, `SELECT status, count(*) FROM correcoes GROUP BY status`)
	if err == nil {
		defer subRows.Close()
		for subRows.Next() {
			var st string
			var count int64
			if err := subRows.Scan(&st, &count); err == nil {
				metricas.SubmissoesPorStatus[st] = count
			}
		}
	}

	// Fila do judge0
	jRows, err := s.store.Pool.Query(ctx, `SELECT status, count(*) FROM jobs GROUP BY status`)
	if err == nil {
		defer jRows.Close()
		for jRows.Next() {
			var st string
			var count int64
			if err := jRows.Scan(&st, &count); err == nil {
				metricas.FilaJudge0[st] = count
			}
		}
	}

	// Falhas nos últimos 7 dias
	_ = s.store.Pool.QueryRow(ctx, `
		SELECT count(*) FROM correcoes WHERE status = 'falhou' AND created_at >= now() - interval '7 days'`).Scan(&metricas.FalhasUltimos7Dias)

	return metricas, nil
}
