package monitor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/testutil"
	"github.com/seno-project/seno/contract"
)

const segTeste = "segredo-de-teste-da-integracao-32-bytes!!"

// testeDB do pacote: banco dedicado (evita colisão com httpapi em ./...).
func testeDB(t *testing.T) *store.Store {
	return testutil.TestDB(t, "seno_test_monitor")
}

// semeia o mínimo: turma→matéria→período, atividade com 1 tarefa (2 testes,
// 1 público+1 privado) e 1 aluno ativo matriculado. Devolve ids essenciais.
func semeia(t *testing.T, st *store.Store, ctx context.Context) (atr, alunoID int64) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatalf("semeia: %v", err)
		}
	}
	mustExec := func(tag interface{}, err error) { must(err) }
	var pid, uid int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO persons (first_name, last_name)
		VALUES ('Profe', 'Um') RETURNING id`).Scan(&pid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO users (person_id, email, password_hash, status)
		VALUES ($1, 'prof@test.dev', 'x', 'active') RETURNING id`, pid).Scan(&uid))
	var aid int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO persons (first_name, last_name)
		VALUES ('Alice', 'Duas') RETURNING id`).Scan(&aid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO users (person_id, email, password_hash, status)
		VALUES ($1, 'aluno@test.dev', 'x', 'active') RETURNING id`, aid).Scan(&alunoID))
	var mid, per int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO materias (codigo, nome)
		VALUES ('PROG1', 'Programação I') RETURNING id`).Scan(&mid))
	must(st.Pool.QueryRow(ctx, `INSERT INTO periodos_letivos (trimestre, ano)
		VALUES (1, 2026) RETURNING id`).Scan(&per))
	var tid int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO turmas (materia_id, periodo_id, professor_id, titulo)
		VALUES ($1, $2, $3, 'T1') RETURNING id`, mid, per, uid).Scan(&tid))
	var atv int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO atividades (professor_id, nome, conteudo, created_by)
		VALUES ($1, 'Prova 1', '{}', $1) RETURNING id`, uid).Scan(&atv))
	var tar int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO tarefas (professor_id, nome, enunciado, tempo_cpu_ms, tempo_total_ms, memoria_mb, created_by)
		VALUES ($1, 'Dobro', 'Leia e imprima o dobro.', 2000, 5000, 256, $1) RETURNING id`, uid).Scan(&tar))
	mustExec(st.Pool.Exec(ctx, `INSERT INTO atividade_tarefas (atividade_id, tarefa_id, ordem, valor_pts, linguagem)
		VALUES ($1, $2, 1, 10, 'python')`, atv, tar))
	mustExec(st.Pool.Exec(ctx, `INSERT INTO testes_tarefa (tarefa_id, stdin, stdout_esperado, publico, created_by)
		VALUES ($1, '3', '6', true, $2)`, tar, uid))
	mustExec(st.Pool.Exec(ctx, `INSERT INTO testes_tarefa (tarefa_id, stdin, stdout_esperado, publico, created_by)
		VALUES ($1, '5', '10', false, $2)`, tar, uid))
	var atrID int64
	must(st.Pool.QueryRow(ctx, `INSERT INTO atribuicoes (turma_id, atividade_id, autocomplete, inicio, prazo, duracao_seg, pode_atrasado, created_by)
		VALUES ($1, $2, false, now() - interval '5 minutes', now() + interval '30 minutes', 1800, true, $3)
		RETURNING id`, tid, atv, uid).Scan(&atrID))
	mustExec(st.Pool.Exec(ctx, `INSERT INTO matriculas (turma_id, aluno_id, created_by)
		VALUES ($1, $2, $3)`, tid, alunoID, uid))
	return atrID, alunoID
}

func TestTransferenciaE2E(t *testing.T) {
	st := testeDB(t)
	ctx := context.Background()
	atrID, alunoID := semeia(t, st, ctx)

	var recebidos []contract.Transferencia
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := contract.VerificarMAC([]byte(segTeste), b,
			r.Header.Get(contract.HeaderMAC), r.Header.Get(contract.HeaderTime),
			time.Minute); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var tIn contract.Transferencia
		if err := json.Unmarshal(b, &tIn); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		recebidos = append(recebidos, tIn)
		// 1ª: criado; 2ª+: existente (200 idempotente).
		status := http.StatusCreated
		if len(recebidos) > 1 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		w.Write([]byte(`{"monitor_id":"atv_teste_1"}`))
	}))
	defer srv.Close()

	svc := New(st, platform.NewAudit(st.Pool), srv.URL, "http://monitor-publico", segTeste)

	// Montagem: envelope íntegro (alunos, carga, testes privados, regras).
	tr, err := svc.Montar(ctx, atrID)
	if err != nil {
		t.Fatalf("montar: %v", err)
	}
	if tr.APIID != atrID || len(tr.Alunos) != 1 || tr.Alunos[0].APIID != alunoID {
		t.Fatalf("alunos inesperados: %+v", tr.Alunos)
	}
	if tr.Carga.Atividade.Nome != "Prova 1" || len(tr.Carga.Tarefas) != 1 {
		t.Fatalf("carga inesperada: %+v", tr.Carga)
	}
	tf := tr.Carga.Tarefas[0]
	if tf.Linguagem != "python" || len(tf.Testes) != 2 || tf.Testes[1].Publico {
		t.Fatalf("testes (incl. privado) incompletos: %+v", tf)
	}
	if tr.Regras.Autocomplete != false || tr.Regras.MaxSubmissoes != 1 {
		t.Fatalf("regras inesperadas: %+v", tr.Regras)
	}
	if tr.Janela.DuracaoSeg == nil || *tr.Janela.DuracaoSeg != 1800 || !tr.Janela.PodeAtrasado {
		t.Fatalf("janela inesperada: %+v", tr.Janela)
	}

	// Envio (duas vezes: Monitor idempotente por api_id) + persistência.
	for i := 0; i < 2; i++ {
		mid, err := svc.Enviar(ctx, tr)
		if err != nil {
			t.Fatalf("enviar %d: %v", i, err)
		}
		if mid != "atv_teste_1" {
			t.Fatalf("monitor_id inesperado: %q", mid)
		}
	}
	if err := svc.gravarMonitorID(ctx, atrID, "atv_teste_1"); err != nil {
		t.Fatalf("gravar monitor_id: %v", err)
	}
	var gravado *string
	if err := st.Pool.QueryRow(ctx, `SELECT monitor_id FROM atribuicoes WHERE id = $1`,
		atrID).Scan(&gravado); err != nil || gravado == nil || *gravado != "atv_teste_1" {
		t.Fatalf("monitor_id não persistido: err=%v gravado=%v", err, gravado)
	}

	// Handler de fila: montar → enviar → gravar end-to-end.
	if _, err := st.Pool.Exec(ctx, `UPDATE atribuicoes SET monitor_id = NULL WHERE id = $1`, atrID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handler()(ctx, json.RawMessage(`{"atribuicao_id":`+strconv.FormatInt(atrID, 10)+`}`)); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(recebidos) != 3 {
		t.Fatalf("Monitor-fake recebeu %d transferências; esperado 3", len(recebidos))
	}
	if recebidos[2].Turma.Nome == "" || recebidos[2].Carga.Atividade.APIID == 0 {
		t.Fatalf("último envelope incompleto: %+v", recebidos[2])
	}
}

// TestEntrarEmissor — Fase 9.1: /entrar valida student+matrícula, exige
// monitor_id transferido e emite `st` temporário para o redirect do BFF.
func TestEntrarEmissor(t *testing.T) {
	st := testeDB(t)
	ctx := context.Background()
	atrID, alunoID := semeia(t, st, ctx)
	svc := New(st, platform.NewAudit(st.Pool), "http://monitor-interno", "http://monitor-publico", segTeste)
	aluno := domain.User{ID: alunoID, Roles: []domain.Role{domain.RoleStudent}}

	// Sem monitor_id: 409 (ainda não transferida).
	if _, err := svc.Entrar(ctx, aluno, atrID); err == nil {
		t.Fatal("entrar deve falhar sem monitor_id transferido")
	}
	if _, err := st.Pool.Exec(ctx,
		`UPDATE atribuicoes SET monitor_id = 'atv_entrar_1' WHERE id = $1`, atrID); err != nil {
		t.Fatal(err)
	}

	// Professor: 403 (exclusivo de alunos).
	var profID int64
	if err := st.Pool.QueryRow(ctx,
		`SELECT professor_id FROM atribuicoes a JOIN turmas t ON t.id = a.turma_id WHERE a.id = $1`,
		atrID).Scan(&profID); err != nil {
		t.Fatal(err)
	}
	prof := domain.User{ID: profID, Roles: []domain.Role{domain.RoleProfessor}}
	if _, err := svc.Entrar(ctx, prof, atrID); err == nil {
		t.Fatal("professor não deveria receber st")
	}

	url, err := svc.Entrar(ctx, aluno, atrID)
	if err != nil {
		t.Fatalf("entrar: %v", err)
	}
	const prefixo = "/entrar?st="
	if !strings.HasPrefix(url, "http://monitor-publico"+prefixo) {
		t.Fatalf("URL inesperada: %s", url)
	}
	claims, err := contract.VerificarST([]byte(segTeste),
		strings.TrimPrefix(url, "http://monitor-publico"+prefixo),
		contract.SSOAud, time.Now())
	if err != nil || claims.Sub != alunoID || claims.Mid != "atv_entrar_1" || claims.Iss != contract.SSOIss {
		t.Fatalf("claims inesperadas: %+v err=%v", claims, err)
	}
}
