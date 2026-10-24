package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/seno-project/seno/api/internal/monitor"
	"github.com/seno-project/seno/contract"
)

// TestSubmissoes valida a entrega final (PROJETO §Submissão/§Atribuição):
// exige tentativa, prazo efetivo no servidor, atraso condicional,
// unique por aluno/atribuição, tentativa apagada e listagem do professor.
func TestSubmissoes(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "paula.subm@seno.dev", "Paula", "Ribeiro")
	ativaNovoUsuario(t, r, pool, "paula.subm@seno.dev", "senha-paula-s1")
	tokPaula := logar(t, r, "paula.subm@seno.dev", "senha-paula-s1")

	// Turma aberta (2) com atraso permitido + turma com prazo curto (1).
	w, materia := chama(t, r, "POST", "/v1/materias",
		`{"codigo":"SUBM","nome":"Submissões"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos",
		`{"trimestre":1,"ano":2028}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, t1 := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"P1"}`,
		autenticado(tokPaula))
	idT1 := int64(t1["id"].(float64))
	w, t2 := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"P2"}`,
		autenticado(tokPaula))
	idT2 := int64(t2["id"].(float64))

	// Atividade única atribuída às duas turmas (mesmo professor).
	w, tarefa := chama(t, r, "POST", "/v1/tarefas",
		`{"nome":"S1","enunciado":"."}`, autenticado(tokPaula))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[{"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":5,"language":"python"}]}`
	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Ativ S","conteudo":`+conteudo+`,
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":5,"linguagem":"python"}]}`,
		autenticado(tokPaula))
	idAtv := int64(atv["id"].(float64))
	w, atrasada := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idT1)+`,"atividade_id":`+intStr(idAtv)+`,
		"prazo":"`+time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)+`",
		"pode_atrasado":true}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição lenta: %d corpo %v", w.Code, atrasada)
	}
	idAtr1 := int64(atrasada["id"].(float64))
	w, dura := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idT2)+`,"atividade_id":`+intStr(idAtv)+`,
		"prazo":"`+time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)+`"}`,
		autenticado(tokPaula))
	if w.Code != http.StatusCreated {
		t.Fatalf("atribuição dura: %d corpo %v", w.Code, dura)
	}
	idAtr2 := int64(dura["id"].(float64))

	// Dois alunos: Carla (P1) e Bruno (P2).
	criarAluno := func(turma int64, email string) (string, int64) {
		t.Helper()
		w, mat := chama(t, r, "POST", "/v1/turmas/"+intStr(turma)+"/matriculas",
			`{"email":"`+email+`","first_name":"`+email[0:4]+`","last_name":"Al"}`,
			autenticado(tokPaula))
		if w.Code != http.StatusCreated {
			t.Fatalf("matrícula %s: %d corpo %v", email, w.Code, mat)
		}
		tokConvite := tokenDoLog(t, pool)
		aceitarConvite(t, r, tokConvite, "senha-"+email[0:4]+"-1")
		tok := logar(t, r, email, "senha-"+email[0:4]+"-1")
		if len(email) < 4 {
			return tok, 0
		}
		return tok, int64(mat["id"].(float64))
	}
	tokCarla, _ := criarAluno(idT1, "carla.subm@seno.dev")
	tokBruno, _ := criarAluno(idT2, "bruno.subm@seno.dev")

	// Sem tentativa aberta: 422.
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissao",
		`{}`, autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("entrega sem tentativa: %d", w.Code)
	}

	// Carla: prato passada com atraso permitido → submissão atrasada.
	w, abriu := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("abrir tentativa: %d corpo %v", w.Code, abriu)
	}
	w, entregou := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissao",
		`{"observacao":"corrigi o 2"}`, autenticado(tokCarla))
	if w.Code != http.StatusCreated {
		t.Fatalf("entrega atrasada: %d corpo %v", w.Code, entregou)
	}
	if at, _ := entregou["atrasada"].(bool); !at {
		t.Errorf("prazo gap: atrasada=true: %v", entregou)
	}
	idSub := int64(entregou["id"].(float64))

	// Dupla → 409 (rascunho não volta: POST /tentativa também bloqueia).
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissao",
		`{}`, autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("entrega dupla: %d", w.Code)
	}
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr1)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusConflict {
		t.Errorf("tentativa após entrega: %d", w.Code)
	}
	// Tentativa apagada: GET /tentativa → 404.
	w, _ = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/tentativa",
		"", autenticado(tokCarla))
	if w.Code != http.StatusNotFound {
		t.Errorf("tentativa seguiu lá: %d", w.Code)
	}

	// Bruno: prazo encerrado sem atraso permitido — tentativa e entrega
	// bloqueadas (422).
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr2)+"/tentativa",
		"", autenticado(tokBruno))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("abrir tentativa após prazo: %d", w.Code)
	}
	w, falha := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr2)+"/submissao",
		`{}`, autenticado(tokBruno))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("prazo bloqueado era 422: %d corpo %v", w.Code, falha)
	}

	// Professor dono lista entregas: 1 (Carla, atrasada); staff igual.
	w, corpo := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("listar entregas: %d corpo %v", w.Code, corpo)
	}
	if intOf(corpo["total"]) != 1 {
		t.Errorf("total de entregas: %v", corpo["total"])
	}
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokSuper))
	if w.Code != http.StatusOK {
		t.Errorf("staff lista entregas: %d corpo %v", w.Code, corpo)
	}
	// Bruno não lista entregas (escopo).
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokBruno))
	if w.Code != http.StatusNotFound {
		t.Errorf("aluno lista entregas: %d corpo %v", w.Code, corpo)
	}

	// Entrega via Monitor (espelho do /estado, Fase 9.3): Dalva entregou
	// lá — API não tem submissão; com atribuição transferida, aparece na
	// lista do professor como do_monitor (atrasada: entregue após prazo),
	// sem link de correção; a entrega da API (Carla) vence na dedupe.
	criarAluno(idT1, "dalva.subm@seno.dev")
	var idDalva int64
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", "dalva.subm@seno.dev").Scan(&idDalva); err != nil {
		t.Fatalf("dalva: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE atribuicoes SET monitor_id = 'atv_teste_entregas' WHERE id = $1`, idAtr1); err != nil {
		t.Fatalf("monitor_id: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO monitor_estado_aluno
		   (atribuicao_id, aluno_id, estado, n_t, entregue_em, nota_auto, correcao_status, prazo_efetivo, atualizado_em)
		 VALUES ($1, $2, 'entregue', 1, now() + interval '1 second', 4.5, 'concluida', now(), now())`,
		idAtr1, idDalva); err != nil {
		t.Fatalf("espelho do monitor: %v", err)
	}
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("entregas com espelho: %d corpo %v", w.Code, corpo)
	}
	if intOf(corpo["total"]) != 2 {
		t.Fatalf("total com espelho: %v (%v)", corpo["total"], corpo["submissoes"])
	}
	var dalva, carlaRow map[string]any
	for _, it := range corpo["submissoes"].([]any) {
		e := it.(map[string]any)
		if e["do_monitor"] == true {
			dalva = e
		} else {
			carlaRow = e
		}
	}
	if dalva == nil || carlaRow == nil {
		t.Fatalf("esperava entrega da API e do monitor: %v", corpo["submissoes"])
	}
	if !boolOf(dalva["atrasada"]) {
		t.Errorf("entrega do monitor atrasada: %v", dalva)
	}
	if nota, _ := dalva["nota_monitor"].(float64); nota != 4.5 {
		t.Errorf("nota do monitor: %v", dalva["nota_monitor"])
	}
	if cm, ok := dalva["correcao_monitor"].(string); !ok || cm != "concluida" {
		t.Errorf("correcao do monitor: %v", dalva["correcao_monitor"])
	}
	if _, ok := carlaRow["comecou_em"]; !ok {
		t.Errorf("submissão da API sem comecou_em: %v", carlaRow)
	}
	// Carla também tem espelho: a submissão da API vence (não duplica).
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO monitor_estado_aluno
		   (atribuicao_id, aluno_id, estado, n_t, entregue_em, correcao_status, prazo_efetivo, atualizado_em)
		 VALUES ($1, (SELECT id FROM users WHERE email = 'carla.subm@seno.dev'),
		         'entregue', 1, now(), 'pendente', now(), now())`, idAtr1); err != nil {
		t.Fatalf("espelho da carla: %v", err)
	}
	w, corpo = chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr1)+"/submissoes",
		"", autenticado(tokPaula))
	if intOf(corpo["total"]) != 2 {
		t.Errorf("dedupe total (esperado 2): %v", corpo["total"])
	}

	// /v1/me: entregue=true atrasada=true e sem tentativa.
	w, corpo = chama(t, r, "GET", "/v1/me/atribuicoes?turma_id="+intStr(idT1),
		"", autenticado(tokCarla))
	if lista, _ := corpo["atribuicoes"].([]any); len(lista) == 1 {
		item, _ := lista[0].(map[string]any)
		if em, _ := item["entregue"].(bool); !em {
			t.Errorf("entregue=true no /me: %v", item)
		}
	}

	// Aluno lê a própria entrega pelo id; os demais casos são 404
	// (outro aluno, professor — nem a existência é revelada).
	w, minha := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub),
		"", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("minha submissão: %d corpo %v", w.Code, minha)
	}
	if minha["atribuicao_id"].(float64) != float64(idAtr1) || !minha["atrasada"].(bool) {
		t.Errorf("conteúdo da submissão: %v", minha)
	}
	if minha["snapshot"] != nil {
		t.Errorf("snapshot antes de publicação deve ser nulo para o aluno: %v", minha["snapshot"])
	}
	for _, outro := range []string{tokBruno, tokPaula, tokSuper} {
		w, _ = chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub), "", autenticado(outro))
		if w.Code != http.StatusNotFound {
			t.Errorf("submissão alheia: %d", w.Code)
		}
	}

	// Professor visualiza a correção com entregue_em e atrasada preenchidos
	w, profCor := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub)+"/correcao", "", autenticado(tokPaula))
	if w.Code != http.StatusOK {
		t.Fatalf("professor ver correcao: %d corpo %v", w.Code, profCor)
	}
	submResumo := profCor["submissao"].(map[string]any)
	if !submResumo["atrasada"].(bool) {
		t.Errorf("visao professor esperava atrasada=true: %v", submResumo)
	}
	if ent, ok := submResumo["entregue_em"].(string); !ok || ent == "" || ent == "0001-01-01T00:00:00Z" {
		t.Errorf("visao professor esperava entregue_em válido: %v", submResumo["entregue_em"])
	}

	// Após publicação da correção, o aluno passa a ter acesso ao snapshot com as próprias respostas
	if _, err := pool.Exec(context.Background(),
		"UPDATE correcoes SET confirmada_em = now(), publicada_em = now() WHERE submissao_id = $1", idSub); err != nil {
		t.Fatalf("publicando correcao: %v", err)
	}
	w, minhaPub := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub), "", autenticado(tokCarla))
	if w.Code != http.StatusOK {
		t.Fatalf("minha submissão pós publicação: %d corpo %v", w.Code, minhaPub)
	}
	if minhaPub["snapshot"] == nil {
		t.Errorf("snapshot pós publicação deve ser liberado para o aluno: %v", minhaPub)
	}

	// Id inválido → 422.
	w, _ = chama(t, r, "POST", "/v1/atribuicoes/abc/submissao", "{}",
		autenticado(tokCarla))
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("id inválido: %d", w.Code)
	}
	_ = pool
}

func TestAcoesProfessorEntregas(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "prof.acoes@seno.dev", "Carlos", "Acoes")
	ativaNovoUsuario(t, r, pool, "prof.acoes@seno.dev", "senha-prof-acoes")
	tokProf := logar(t, r, "prof.acoes@seno.dev", "senha-prof-acoes")

	// Matéria, período e turma
	_, materia := chama(t, r, "POST", "/v1/materias", `{"codigo":"ACAO","nome":"Acoes"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	_, periodo := chama(t, r, "POST", "/v1/periodos", `{"trimestre":1,"ano":2029}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	_, turma := chama(t, r, "POST", "/v1/turmas", `{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"T-Acoes"}`, autenticado(tokProf))
	idTurma := int64(turma["id"].(float64))

	// Aluno
	w, mat := chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"aluno.acoes@seno.dev","first_name":"Bia","last_name":"Acoes"}`,
		autenticado(tokProf))
	if w.Code != http.StatusCreated {
		t.Fatalf("matrícula aluno: %d %v", w.Code, mat)
	}
	tokConvite := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConvite, "senha-aluno-acoes")
	tokAluno := logar(t, r, "aluno.acoes@seno.dev", "senha-aluno-acoes")


	// Tarefa + Atividade
	_, tarefa := chama(t, r, "POST", "/v1/tarefas", `{"nome":"Tarefa1","enunciado":"enunciado"}`, autenticado(tokProf))
	idTarefa := int64(tarefa["id"].(float64))
	conteudo := `{"schema_version":1,"components":[{"type":"task_default","id":` + intStr(idTarefa) + `,"value_pts":10,"language":"python"}]}`
	_, atv := chama(t, r, "POST", "/v1/atividades", `{"nome":"Ativ Acoes","conteudo":`+conteudo+`,"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":10,"linguagem":"python"}]}`, autenticado(tokProf))
	idAtv := int64(atv["id"].(float64))

	// Atribuição com prazo no passado e SEM atraso permitido
	_, atr := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idTurma)+`,
		"atividade_id":`+intStr(idAtv)+`,
		"prazo":"`+time.Now().Add(-10*time.Minute).UTC().Format(time.RFC3339Nano)+`",
		"pode_atrasado":false}`, autenticado(tokProf))
	idAtr := int64(atr["id"].(float64))

	// Aluno abre tentativa antes do prazo (simulando via banco com comecou_em no passado)
	var alunoID int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM users WHERE email = 'aluno.acoes@seno.dev'`).Scan(&alunoID); err != nil {
		t.Fatalf("aluno id: %v", err)
	}
	comecouPassado := time.Now().Add(-30 * time.Minute)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO tentativas (atribuicao_id, aluno_id, comecou_em, revisao, gravado_em, snapshot)
		VALUES ($1, $2, $3, 1, $3, '{"tasks":[{"id":`+intStr(idTarefa)+`,"text":"print(1)"}]}'::jsonb)`,
		idAtr, alunoID, comecouPassado); err != nil {
		t.Fatalf("inserir tentativa: %v", err)
	}

	// 1. Professor lista entregas: deve ver o aluno em rascunho com pode_coletar=true
	w, lista := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr)+"/submissoes", "", autenticado(tokProf))
	if w.Code != http.StatusOK {
		t.Fatalf("listar: %d", w.Code)
	}
	subs := lista["submissoes"].([]any)
	if len(subs) != 1 {
		t.Fatalf("esperava 1 entrega/rascunho: %v", subs)
	}
	item := subs[0].(map[string]any)
	if item["estado"] != "rascunho" || item["pode_coletar"] != true {
		t.Errorf("esperava rascunho coletável: %v", item)
	}

	// 2. Professor coleta o rascunho
	w, col := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/coletar/"+intStr(alunoID), "{}", autenticado(tokProf))
	if w.Code != http.StatusCreated {
		t.Fatalf("coletar falhou: %d %v", w.Code, col)
	}
	subID := int64(col["id"].(float64))
	if !col["atrasada"].(bool) {
		t.Errorf("coleta deve marcar atrasada=true")
	}

	// 3. Reexecutar correção (permitido pois não confirmada)
	w, reex := chama(t, r, "POST", "/v1/submissoes/"+intStr(subID)+"/correcao/reexecutar", "{}", autenticado(tokProf))
	if w.Code != http.StatusOK {
		t.Fatalf("reexecutar falhou: %d %v", w.Code, reex)
	}

	// 4. Professor exclui a submissão
	w, del := chama(t, r, "DELETE", "/v1/submissoes/"+intStr(subID), "", autenticado(tokProf))
	if w.Code != http.StatusOK {
		t.Fatalf("excluir falhou: %d %v", w.Code, del)
	}

	// 5. Após exclusão, aluno tem tentativa restaurada
	w, tent := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr)+"/tentativa", "", autenticado(tokAluno))
	if w.Code != http.StatusOK {
		t.Fatalf("tentativa restaurada não encontrada: %d %v", w.Code, tent)
	}
	_ = pool
}

func TestIngestaoMonitorHMAC(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)

	secretMonitor := "chave-secreta-hmac-monitor-32-chars-long"
	deps.Monitor = monitor.New(deps.Store, deps.Audit, "http://monitor:8081", "http://localhost:5174", secretMonitor)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "prof.mon@seno.dev", "Pedro", "Monitor")
	ativaNovoUsuario(t, r, pool, "prof.mon@seno.dev", "senha-prof-mon-1")
	tokProf := logar(t, r, "prof.mon@seno.dev", "senha-prof-mon-1")

	w, materia := chama(t, r, "POST", "/v1/materias", `{"codigo":"MON","nome":"Monitor"}`, autenticado(tokSuper))
	idMateria := int64(materia["id"].(float64))
	w, periodo := chama(t, r, "POST", "/v1/periodos", `{"trimestre":1,"ano":2029}`, autenticado(tokSuper))
	idPeriodo := int64(periodo["id"].(float64))
	w, turma := chama(t, r, "POST", "/v1/turmas", `{"materia_id":`+intStr(idMateria)+`,"periodo_id":`+intStr(idPeriodo)+`,"titulo":"M1"}`, autenticado(tokProf))
	idTurma := int64(turma["id"].(float64))

	w, mat := chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"aluno.mon@seno.dev","first_name":"Lucas","last_name":"Mon"}`, autenticado(tokProf))
	if w.Code != http.StatusCreated {
		t.Fatalf("matricula falhou: %d %v", w.Code, mat)
	}
	alunoID := int64(mat["id"].(float64))

	w, tarefa := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Tarefa Mon",
		"enunciado":"Enunciado mon",
		"testes":[{"stdin":"1","stdout_esperado":"1","publico":true}]
	}`, autenticado(tokProf))
	idTarefa := int64(tarefa["id"].(float64))

	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Atividade Mon",
		"conteudo":{"schema_version":1,"components":[{"type":"task_default","id":`+intStr(idTarefa)+`,"value_pts":10,"language":"python"}]},
		"tarefas":[{"tarefa_id":`+intStr(idTarefa)+`,"ordem":1,"valor_pts":10,"linguagem":"python"}]
	}`, autenticado(tokProf))
	idAtv := int64(atv["id"].(float64))

	w, atrib := chama(t, r, "POST", "/v1/atribuicoes", `{
		"turma_id":`+intStr(idTurma)+`,
		"atividade_id":`+intStr(idAtv)+`
	}`, autenticado(tokProf))
	idAtr := int64(atrib["id"].(float64))

	// Associa um monitor_id à atribuição
	mid := "mon_test_123"
	if _, err := pool.Exec(context.Background(), `UPDATE atribuicoes SET monitor_id = $1 WHERE id = $2`, mid, idAtr); err != nil {
		t.Fatalf("update monitor_id: %v", err)
	}

	// 1. Chamada sem HMAC ou com HMAC inválido deve ser rejeitada com 401
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/interna/monitor/submissoes", bytes.NewReader([]byte("{}")))
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("esperava 401 para push sem HMAC, obteve %d", rec.Code)
	}

	// 2. Chamada com HMAC assinado corretamente
	agora := time.Now().UTC()
	notaVal := 10.0
	pacote := contract.PacoteSubmissaoMonitor{
		MonitorID:       mid,
		AtribuicaoAPIID: idAtr,
		AlunoAPIID:      alunoID,
		Submissao: &contract.SubmissaoMonitor{
			ID:         1,
			AlunoID:    alunoID,
			NT:         1,
			ComecouEm:  agora.Add(-30 * time.Minute),
			EntregueEm: agora,
			Atrasada:   false,
			Respostas:  map[string]string{"1": "print(1)"},
		},
		Correcao: &contract.CorrecaoMonitor{
			Status:   "concluida",
			NotaAuto: &notaVal,
			Tarefas: []contract.TarefaCorrecaoMonitor{
				{
					Ordem:       1,
					TarefaAPIID: idTarefa,
					NotaAuto:    &notaVal,
					Testes: []contract.ResultadoTesteMonitor{
						{
							Ordem:         1,
							Status:        "accepted",
							Stdout:        "1\n",
							Stderr:        "",
							CompileOutput: "",
							Tempo:         0.012,
							Memoria:       1024,
						},
					},
				},
			},
		},
	}
	corpo, err := json.Marshal(pacote)
	if err != nil {
		t.Fatalf("marshal pacote: %v", err)
	}

	mac, ts := contract.AssinarMAC([]byte(secretMonitor), time.Now(), corpo)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/interna/monitor/submissoes", bytes.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, ts)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("push com HMAC valido falhou: %d %s", rec.Code, rec.Body.String())
	}

	var resPush map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resPush)
	subID := int64(resPush["submissao_id"].(float64))
	if subID < 1 {
		t.Fatalf("submissao_id invalido gerado: %v", resPush)
	}

	// 3. Verifica que a submissão agora aparece nas entregas para o professor
	w, lista := chama(t, r, "GET", "/v1/atribuicoes/"+intStr(idAtr)+"/submissoes", "", autenticado(tokProf))
	if w.Code != http.StatusOK {
		t.Fatalf("listar entregas apos ingestao: %d %v", w.Code, lista)
	}
	subs := lista["submissoes"].([]any)
	if len(subs) != 1 {
		t.Fatalf("esperava 1 entrega ingerida, obteve %d", len(subs))
	}
	ent := subs[0].(map[string]any)
	if ent["correcao_status"] != "concluida" {
		t.Errorf("status da correcao: %v", ent["correcao_status"])
	}
	if nAuto, _ := ent["nota_auto"].(float64); nAuto != 10 {
		t.Errorf("nota_auto da entrega: %v", ent["nota_auto"])
	}

	// 4. Professor confirma a validação da correção
	w, _ = chama(t, r, "POST", "/v1/submissoes/"+intStr(subID)+"/correcao/confirmar", "", autenticado(tokProf))
	if w.Code != http.StatusOK {
		t.Fatalf("confirmar correcao: %d", w.Code)
	}

	// 5. Professor publica as notas para toda a turma de uma vez
	w, pub := chama(t, r, "POST", "/v1/atribuicoes/"+intStr(idAtr)+"/correcoes/publicar", "", autenticado(tokProf))
	if w.Code != http.StatusOK {
		t.Fatalf("publicar em lote: %d %v", w.Code, pub)
	}
	if pubN, _ := pub["publicadas"].(float64); pubN != 1 {
		t.Errorf("esperava 1 publicada, obteve %v", pub["publicadas"])
	}
}


