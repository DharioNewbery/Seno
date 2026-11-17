package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seno-project/seno/api/internal/judge0"
	"github.com/seno-project/seno/api/internal/monitor"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/api/internal/testar"
	"github.com/seno-project/seno/contract"
)

// TestArquivoSubmissaoCicloAPI valida a criação/edição de tarefas de submissão de arquivo,
// atividades mistas (programacao + arquivo_submissao), ingestão de pacote do Monitor com
// arquivos binários (PDF), inspeção da correção e download autorizado do arquivo.
func TestArquivoSubmissaoCicloAPI(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaAPI(t, pool)
	segMonitor := "segredo-de-teste-monitor-api-32-bytes!!"
	st := store.New(pool)
	deps.Monitor = monitor.New(st, deps.Audit, "http://localhost:8081", "http://localhost:8081", segMonitor)
	j0Cfg := judge0.CarregarConfigDasEnvLigada("", "", "", "http://localhost:5173")
	deps.TestarS = testar.New(st, deps.Audit, judge0.NewClient(j0Cfg), j0Cfg)

	if err := bootstrapSuper(t, deps); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	tokSuper := logar(t, r, "super@seno.dev", "102938")

	criarProfessor(t, r, tokSuper, "prof.arq@seno.dev", "Clara", "Melo")
	ativaNovoUsuario(t, r, pool, "prof.arq@seno.dev", "senha-clara-1")
	tokClara := logar(t, r, "prof.arq@seno.dev", "senha-clara-1")

	// 1. Rejeita tarefa arquivo_submissao com testes de stdin/stdout (422).
	w, falha := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Relatório","enunciado":"Envie o PDF.",
		"tipo":"arquivo_submissao",
		"testes":[{"stdin":"1","stdout_esperado":"1","publico":true}]
	}`, autenticado(tokClara))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("esperava 422 ao criar arquivo_submissao com testes, obteve %d (%v)", w.Code, falha)
	}

	// 2. Rejeita tarefa arquivo_submissao com extensão executável (.exe) (422).
	w, falha = chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Relatório","enunciado":"Envie o PDF.",
		"tipo":"arquivo_submissao",
		"config_arquivo":{"max_bytes":1048576,"extensoes_permitidas":[".exe"]}
	}`, autenticado(tokClara))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("esperava 422 ao permitir .exe, obteve %d (%v)", w.Code, falha)
	}

	// 3. Cria tarefa de programação e tarefa de submissão de arquivo.
	w, tProg := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Soma Simples","enunciado":"Leia A e B e imprima A+B.",
		"tipo":"programacao",
		"testes":[{"stdin":"2 3","stdout_esperado":"5","publico":true}]
	}`, autenticado(tokClara))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar tarefa prog: %d %v", w.Code, tProg)
	}
	idTProg := int64(tProg["id"].(float64))

	w, tArq := chama(t, r, "POST", "/v1/tarefas", `{
		"nome":"Relatório PDF","enunciado":"Anexe o relatório em PDF ou TXT.",
		"tipo":"arquivo_submissao",
		"config_arquivo":{"max_bytes":2097152,"extensoes_permitidas":["pdf",".TXT"]}
	}`, autenticado(tokClara))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar tarefa arq: %d %v", w.Code, tArq)
	}
	idTArq := int64(tArq["id"].(float64))
	if tArq["tipo"] != contract.TipoTarefaArquivo {
		t.Fatalf("tipo esperado %q, obteve %v", contract.TipoTarefaArquivo, tArq["tipo"])
	}
	cfgArq, ok := tArq["config_arquivo"].(map[string]any)
	if !ok || int64(cfgArq["max_bytes"].(float64)) != 2097152 {
		t.Fatalf("config_arquivo inesperada: %v", tArq["config_arquivo"])
	}

	// 4. Rejeita testar código em tarefa de submissão de arquivo (422).
	w, _ = chama(t, r, "POST", "/v1/tarefas/"+intStr(idTArq)+"/testar",
		`{"linguagem":"python","codigo":"print(1)"}`, autenticado(tokClara))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("testar tarefa arquivo deveria retornar 422, obteve %d", w.Code)
	}

	// 5. Cria atividade mista (tarefa 1: programacao com python; tarefa 2: arquivo_submissao sem linguagem).
	w, atv := chama(t, r, "POST", "/v1/atividades", `{
		"nome":"Prova Mista",
		"conteudo":{"schema_version":1,"components":[]},
		"tarefas":[
			{"tarefa_id":`+intStr(idTProg)+`,"ordem":1,"valor_pts":6,"linguagem":"python"},
			{"tarefa_id":`+intStr(idTArq)+`,"ordem":2,"valor_pts":4,"linguagem":""}
		]
	}`, autenticado(tokClara))
	if w.Code != http.StatusCreated {
		t.Fatalf("criar atividade mista: %d %v", w.Code, atv)
	}
	idAtv := int64(atv["id"].(float64))

	w, atvDet := chama(t, r, "GET", "/v1/atividades/"+intStr(idAtv), "", autenticado(tokClara))
	if w.Code != http.StatusOK {
		t.Fatalf("ver atividade: %d %v", w.Code, atvDet)
	}
	tarefasAtv := atvDet["tarefas"].([]any)
	if len(tarefasAtv) != 2 {
		t.Fatalf("esperava 2 tarefas na atividade, obteve %d", len(tarefasAtv))
	}
	if tarefasAtv[1].(map[string]any)["tarefa_tipo"] != contract.TipoTarefaArquivo {
		t.Fatalf("tarefa 2 deveria ter tarefa_tipo=arquivo_submissao: %v", tarefasAtv[1])
	}

	// 6. Cria matéria, período, turma, atribuição e matricula aluno.
	_, mat := chama(t, r, "POST", "/v1/materias", `{"codigo":"ARQ1","nome":"Arquivos I"}`, autenticado(tokSuper))
	idMat := int64(mat["id"].(float64))
	_, per := chama(t, r, "POST", "/v1/periodos", `{"trimestre":1,"ano":2028}`, autenticado(tokSuper))
	idPer := int64(per["id"].(float64))
	_, turma := chama(t, r, "POST", "/v1/turmas",
		`{"materia_id":`+intStr(idMat)+`,"periodo_id":`+intStr(idPer)+`,"titulo":"T1"}`,
		autenticado(tokClara))
	idTurma := int64(turma["id"].(float64))
	prazo := time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	_, atrib := chama(t, r, "POST", "/v1/atribuicoes",
		`{"turma_id":`+intStr(idTurma)+`,"atividade_id":`+intStr(idAtv)+`,"prazo":"`+prazo+`"}`,
		autenticado(tokClara))
	idAtr := int64(atrib["id"].(float64))

	chama(t, r, "POST", "/v1/turmas/"+intStr(idTurma)+"/matriculas",
		`{"email":"lucas.arq@seno.dev","first_name":"Lucas","last_name":"Silva"}`,
		autenticado(tokClara))
	tokConv := tokenDoLog(t, pool)
	aceitarConvite(t, r, tokConv, "senha-lucas-1")
	tokLucas := logar(t, r, "lucas.arq@seno.dev", "senha-lucas-1")
	_, meLucas := chama(t, r, "GET", "/v1/auth/me", "", autenticado(tokLucas))
	idLucas := int64(meLucas["id"].(float64))

	// 7. Ingestão HMAC da submissão vinda do Monitor com código (tarefa 1) e arquivo PDF (tarefa 2).
	pdfBytes := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF\n")
	agora := time.Now().UTC()
	pacote := contract.PacoteSubmissaoMonitor{
		MonitorID:       "atv_teste_arq_1",
		AtribuicaoAPIID: idAtr,
		AlunoAPIID:      idLucas,
		Submissao: &contract.SubmissaoMonitor{
			ID:         1,
			AlunoID:    idLucas,
			NT:         1,
			ComecouEm:  agora.Add(-5 * time.Minute),
			EntregueEm: agora,
			Atrasada:   false,
			Respostas:  map[string]string{"1": "print(5)"},
			Arquivos: map[string]contract.ArquivoSubmetidoMonitor{
				"2": {
					TarefaOrdem:  2,
					TarefaAPIID:  idTArq,
					NomeArquivo:  "relatorio_final.pdf",
					MimeType:     "application/pdf",
					TamanhoBytes: int64(len(pdfBytes)),
					Conteudo:     pdfBytes,
				},
			},
		},
		Correcao: &contract.CorrecaoMonitor{
			Status: "concluida",
			Tarefas: []contract.TarefaCorrecaoMonitor{
				{Ordem: 1, TarefaAPIID: idTProg, Veredito: "accepted"},
				{Ordem: 2, TarefaAPIID: idTArq, Veredito: "arquivo_entregue"},
			},
		},
	}
	corpoPacote, _ := json.Marshal(pacote)
	mac, ts := contract.AssinarMAC([]byte(segMonitor), time.Now(), corpoPacote)
	reqIngest := httptest.NewRequest("POST", "/v1/interna/monitor/submissoes", bytes.NewReader(corpoPacote))
	reqIngest.Header.Set("Content-Type", "application/json")
	reqIngest.Header.Set(contract.HeaderMAC, mac)
	reqIngest.Header.Set(contract.HeaderTime, ts)
	wIngest := httptest.NewRecorder()
	r.ServeHTTP(wIngest, reqIngest)
	if wIngest.Code != http.StatusOK {
		t.Fatalf("ingestão monitor falhou: %d %s", wIngest.Code, wIngest.Body.String())
	}
	var respIngest map[string]any
	_ = json.Unmarshal(wIngest.Body.Bytes(), &respIngest)
	idSub := int64(respIngest["submissao_id"].(float64))

	// 8. Visão da correção do professor inclui metadados do arquivo na tarefa 2.
	w, visaoProf := chama(t, r, "GET", "/v1/submissoes/"+intStr(idSub)+"/correcao", "", autenticado(tokClara))
	if w.Code != http.StatusOK {
		t.Fatalf("ver correcao: %d %v", w.Code, visaoProf)
	}
	cor := visaoProf["correcao"].(map[string]any)
	corTarefas := cor["tarefas"].([]any)
	var ctArq map[string]any
	for _, item := range corTarefas {
		m := item.(map[string]any)
		if int64(m["tarefa_id"].(float64)) == idTArq {
			ctArq = m
		}
	}
	if ctArq == nil {
		t.Fatalf("tarefa de arquivo não encontrada na correção: %v", corTarefas)
	}
	if ctArq["tarefa_tipo"] != contract.TipoTarefaArquivo {
		t.Fatalf("tarefa_tipo esperado %q, obteve %v", contract.TipoTarefaArquivo, ctArq["tarefa_tipo"])
	}
	arqMeta, ok := ctArq["arquivo"].(map[string]any)
	if !ok || arqMeta["nome_arquivo"] != "relatorio_final.pdf" {
		t.Fatalf("metadados do arquivo ausentes ou incorretos na correção: %v", ctArq["arquivo"])
	}

	// 9. Download do arquivo pelo professor dono da turma (200 OK + bytes exatos).
	urlDownload := "/v1/submissoes/" + intStr(idSub) + "/tarefas/" + intStr(idTArq) + "/arquivo"
	reqDown := httptest.NewRequest("GET", urlDownload, nil)
	reqDown.Header.Set("Authorization", "Bearer "+tokClara)
	wDown := httptest.NewRecorder()
	r.ServeHTTP(wDown, reqDown)
	if wDown.Code != http.StatusOK {
		t.Fatalf("download professora: %d %s", wDown.Code, wDown.Body.String())
	}
	if !bytes.Equal(wDown.Body.Bytes(), pdfBytes) {
		t.Fatalf("conteúdo baixado diverge do PDF enviado")
	}
	if !strings.Contains(wDown.Header().Get("Content-Disposition"), "relatorio_final.pdf") {
		t.Fatalf("Content-Disposition inesperado: %s", wDown.Header().Get("Content-Disposition"))
	}
	if wDown.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options esperado nosniff, obteve %s", wDown.Header().Get("X-Content-Type-Options"))
	}

	// 10. Download pelo próprio aluno dono da submissão (200 OK).
	reqDownAluno := httptest.NewRequest("GET", urlDownload, nil)
	reqDownAluno.Header.Set("Authorization", "Bearer "+tokLucas)
	wDownAluno := httptest.NewRecorder()
	r.ServeHTTP(wDownAluno, reqDownAluno)
	if wDownAluno.Code != http.StatusOK || !bytes.Equal(wDownAluno.Body.Bytes(), pdfBytes) {
		t.Fatalf("download pelo aluno dono falhou: %d", wDownAluno.Code)
	}

	// 11. Download por outro professor não dono da turma → 404.
	criarProfessor(t, r, tokSuper, "outro.arq@seno.dev", "Outro", "Prof")
	ativaNovoUsuario(t, r, pool, "outro.arq@seno.dev", "senha-outro-1")
	tokOutro := logar(t, r, "outro.arq@seno.dev", "senha-outro-1")
	reqDownOutro := httptest.NewRequest("GET", urlDownload, nil)
	reqDownOutro.Header.Set("Authorization", "Bearer "+tokOutro)
	wDownOutro := httptest.NewRecorder()
	r.ServeHTTP(wDownOutro, reqDownOutro)
	if wDownOutro.Code != http.StatusNotFound {
		t.Fatalf("download por professor não-dono deveria ser 404, obteve %d", wDownOutro.Code)
	}
}
