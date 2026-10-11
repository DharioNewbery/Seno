package ensino

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/api/db"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
)

// testeDB (mesmo padrão dos outros pacotes: banco seno_test + limpeza).
func testeDB(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("SENO_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://seno:seno@localhost:5432/seno_test?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Postgres de teste indisponível (%v)", err)
	}
	if err := platform.MigrateUp(ctx, pool, db.Migrations); err != nil {
		t.Fatalf("migrações: %v", err)
	}
	_, err = pool.Exec(ctx, `TRUNCATE persons, users, user_roles, sessions,
		one_time_tokens, log_entries, jobs, tarefas, testes_tarefa,
		atividades, atividade_tarefas, atribuicoes,
		tentativas, submissoes, resultados, correcao_tarefas,
		correcoes, execucoes_teste, materias, periodos_letivos, turmas, matriculas,
		monitor_estado_aluno RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("limpeza: %v", err)
	}
	return store.New(pool)
}

// seedAtribuicaoParaAluno devolve (atribuicao_id, aluno_id).
func seedAtribuicaoParaAluno(t *testing.T, ctx context.Context, st *store.Store) (int64, int64) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatalf("semeia: %v", err)
		}
	}
	mustExec := func(tag any, err error) { must(err) }
	var pid, uid, aid, alunoID int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO persons (first_name, last_name)
		VALUES ('Profe', 'Um') RETURNING id`).Scan(&pid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO users (person_id, email, password_hash, status)
		VALUES ($1, 'prof@test.dev', 'x', 'active') RETURNING id`, pid).Scan(&uid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO persons (first_name, last_name)
		VALUES ('Alice', 'Duas') RETURNING id`).Scan(&aid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO users (person_id, email, password_hash, status)
		VALUES ($1, 'aluno@test.dev', 'x', 'active') RETURNING id`, aid).Scan(&alunoID))
	var mid, per, tid, atv, atrID int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO materias (codigo, nome)
		VALUES ('PROG1', 'Programação I') RETURNING id`).Scan(&mid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO periodos_letivos (trimestre, ano)
		VALUES (1, 2026) RETURNING id`).Scan(&per))
	must(st.Pool.QueryRow(ctx, `INSERT INTO turmas (materia_id, periodo_id, professor_id, titulo)
		VALUES ($1, $2, $3, 'T1') RETURNING id`, mid, per, uid).Scan(&tid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO atividades (professor_id, nome, conteudo, created_by)
		VALUES ($1, 'Prova 1', '{}', $1) RETURNING id`, uid).Scan(&atv))
	must(st.Pool.QueryRow(ctx, `INSERT INTO atribuicoes (turma_id, atividade_id, autocomplete, inicio, prazo, duracao_seg, pode_atrasado, created_by)
		VALUES ($1, $2, false, now() - interval '5 minutes', now() + interval '30 minutes', 1800, true, $3)
		RETURNING id`, tid, atv, uid).Scan(&atrID))
	mustExec(st.Pool.Exec(ctx, `INSERT INTO matriculas (turma_id, aluno_id, created_by) VALUES ($1, $2, $3)`, tid, alunoID, uid))
	return atrID, alunoID
}

// TestAplicaEstadoMonitor — o snapshot do Monitor vira "em andamento" e
// "entregue" na listagem; equivalente entrega local prevalece se existir.
func TestAplicaEstadoMonitor(t *testing.T) {
	st := testeDB(t)
	ctx := context.Background()
	atrID, alunoID := seedAtribuicaoParaAluno(t, ctx, st)

	entrega := time.Now().Add(-5 * time.Minute)
	if _, err := st.Pool.Exec(ctx, `
		INSERT INTO monitor_estado_aluno
			(atribuicao_id, aluno_id, estado, n_t, entregue_em, correcao_status)
		VALUES ($1, $2, 'entregue', 1, $3, 'concluida')`, atrID, alunoID, entrega); err != nil {
		t.Fatal(err)
	}

	itens := []AtribuicaoDoAluno{{ID: atrID}}
	aplicaEstadoMonitor(ctx, st, alunoID, itens)
	if !itens[0].Entregue || itens[0].EntregueEm == nil {
		t.Fatalf("estado do Monitor não sobrescreveu: %+v", itens[0])
	}
	if itens[0].EntregueEm == nil || itens[0].EntregueEm.UnixMicro() != entrega.UnixMicro() {
		t.Fatalf("entregue_em inesperada: %v", itens[0].EntregueEm)
	}

	// Sem snapshot (atribuição sem estado): mantém o visto local.
	var atr2 int64
	if err := st.Pool.QueryRow(ctx, `SELECT id FROM atribuicoes WHERE id = $1`, atrID).Scan(&atr2); err != nil {
		t.Fatal(err)
	}
	itens2 := []AtribuicaoDoAluno{{ID: 999999}}
	aplicaEstadoMonitor(ctx, st, alunoID, itens2)
	if itens2[0].Entregue || itens2[0].EmAndamento {
		t.Fatalf("estado sem snapshot deveria ficar local: %+v", itens2[0])
	}
}
