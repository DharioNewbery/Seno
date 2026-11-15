package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seno-project/seno/contract"
	"github.com/seno-project/seno/monitor/internal/sessao"
)

func semeiaAtividadeArquivoDB(pool *pgxpool.Pool, aluno int64) error {
	now := time.Now().UTC()
	env := contract.Transferencia{
		APIID:    77,
		GeradaEm: now,
		Turma:    contract.Turma{APIID: 3, Nome: "Turma Arquivo"},
		Alunos:   []contract.Aluno{{APIID: aluno, Nome: "Aluna Arquivo"}},
		Janela: contract.Janela{
			Inicio:       now.Add(-10 * time.Minute),
			Prazo:        now.Add(30 * time.Minute),
			PodeAtrasado: false,
		},
		Regras: contract.Regras{
			SubmissoesMultiplas:  false,
			MaxSubmissoes:        1,
			LinguagensPermitidas: []string{},
		},
		Carga: contract.Carga{
			Atividade: contract.AtividadeCarga{
				APIID: 501, Nome: "Entrega de Relatório", Enunciado: "Anexe o PDF.",
			},
			Tarefas: []contract.TarefaCarga{
				{
					Ordem:       1,
					TarefaAPIID: 9001,
					Nome:        "Relatório Técnico",
					ValorPts:    10,
					Enunciado:   "Envie o relatório em PDF ou TXT.",
					Tipo:        contract.TipoTarefaArquivo,
					ConfigArquivo: &contract.ConfigTarefaArquivo{
						MaxBytes:            4096,
						ExtensoesPermitidas: []string{".pdf", ".txt"},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(env)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO atividade (monitor_id, api_id, carga, recebida_em, status)
	    VALUES ($1, 77, $2::jsonb, now(), 'ativa')`, midTest, string(b)); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `INSERT INTO atividade_aluno (monitor_id, aluno_api_id) VALUES ($1, $2)`, midTest, aluno)
	return err
}

func uploadMultipart(t *testing.T, r actorRouter, sess, path, filename string, content []byte) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("arquivo", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if sess != "" {
		req.Header.Set("Cookie", sessao.CookieNome+"="+sess)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

func TestMonitorArquivoSubmissaoCicloCompleto(t *testing.T) {
	pool := testeDB(t)
	deps, r := montaMonitorJudge0(t, pool)
	if err := semeiaAtividadeArquivoDB(pool, int64(subTest)); err != nil {
		t.Fatalf("semeia: %v", err)
	}

	// Mock da API principal para receber o push imediato do Monitor na confirmação.
	var pacoteRecebido *contract.PacoteSubmissaoMonitor
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		var p contract.PacoteSubmissaoMonitor
		if json.Unmarshal(b, &p) == nil {
			pacoteRecebido = &p
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"submissao_id":123}`))
	}))
	defer apiSrv.Close()
	deps.SubmissaoS.WithAPIPush(apiSrv.URL, segTest)

	sess := entrarCookie(t, r, subTest, "10.8.0.1:1")

	// 1. Abrir tentativa: tarefa 1 vem com tipo="arquivo_submissao" e config_arquivo.
	w, out := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", w.Code, w.Body.String())
	}
	ativ := out["atividade"].(map[string]any)
	t0 := ativ["tarefas"].([]any)[0].(map[string]any)
	if t0["tipo"] != contract.TipoTarefaArquivo {
		t.Fatalf("tipo esperado %q, obteve %v", contract.TipoTarefaArquivo, t0["tipo"])
	}

	// 2. Rejeita /testar para tarefa de arquivo (422).
	w, _ = chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/testar",
		`{"ordem":1,"linguagem":"python","codigo":"print(1)"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("testar em tarefa de arquivo deveria retornar 422, obteve %d (%s)", w.Code, w.Body.String())
	}

	// 3. Pré-avaliação antes de anexar arquivo: veredito = "sem_arquivo" (sem chamar Judge0!).
	w, preSemArq := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("avaliar sem arquivo: %d %s", w.Code, w.Body.String())
	}
	tarefasPre := preSemArq["tarefas"].([]any)
	if tarefasPre[0].(map[string]any)["veredito"] != "sem_arquivo" {
		t.Fatalf("esperava veredito sem_arquivo, obteve %v", tarefasPre[0])
	}

	// 4. Rejeita upload de extensão não permitida (.png) → 422.
	w, _ = uploadMultipart(t, r, sess, "/a/atividade/"+midTest+"/tarefas/1/arquivo",
		"imagem.png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 1, 2, 3})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("extensão não permitida deveria retornar 422, obteve %d (%s)", w.Code, w.Body.String())
	}

	// 5. Rejeita upload de executável ELF disfarçado de .pdf → 422.
	w, _ = uploadMultipart(t, r, sess, "/a/atividade/"+midTest+"/tarefas/1/arquivo",
		"falso.pdf", []byte{0x7f, 'E', 'L', 'F', 1, 2, 3, 4})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ELF disfarçado de .pdf deveria retornar 422, obteve %d (%s)", w.Code, w.Body.String())
	}

	// 6. Rejeita upload acima de max_bytes (4096 bytes) → 413.
	grande := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("A"), 5000)...)
	w, _ = uploadMultipart(t, r, sess, "/a/atividade/"+midTest+"/tarefas/1/arquivo",
		"grande.pdf", grande)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("arquivo acima do limite deveria retornar 413, obteve %d (%s)", w.Code, w.Body.String())
	}

	// 7. Upload válido de PDF via multipart/form-data → 200 OK.
	pdfValido := []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF\n")
	w, upOut := uploadMultipart(t, r, sess, "/a/atividade/"+midTest+"/tarefas/1/arquivo",
		"entrega_final.pdf", pdfValido)
	if w.Code != http.StatusOK {
		t.Fatalf("upload válido falhou: %d %s", w.Code, w.Body.String())
	}
	arqMeta := upOut["arquivo"].(map[string]any)
	if arqMeta["nome_arquivo"] != "entrega_final.pdf" || int(arqMeta["tamanho_bytes"].(float64)) != len(pdfValido) {
		t.Fatalf("metadados de upload inesperados: %v", arqMeta)
	}

	// 8. Download do arquivo anexado pelo aluno durante a tentativa → 200 OK + bytes idênticos.
	reqGet := httptest.NewRequest("GET", "/a/atividade/"+midTest+"/tarefas/1/arquivo", nil)
	reqGet.Header.Set("Cookie", sessao.CookieNome+"="+sess)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK || !bytes.Equal(wGet.Body.Bytes(), pdfValido) {
		t.Fatalf("download na tentativa falhou: status=%d len=%d", wGet.Code, wGet.Body.Len())
	}

	// 9. Pré-avaliação com arquivo anexado: veredito = "arquivo_anexado".
	w, preComArq := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/avaliar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("avaliar com arquivo: %d %s", w.Code, w.Body.String())
	}
	tPre := preComArq["tarefas"].([]any)[0].(map[string]any)
	if tPre["veredito"] != "arquivo_anexado" || tPre["nome_arquivo"] != "entrega_final.pdf" {
		t.Fatalf("esperava arquivo_anexado + nome_arquivo, obteve %v", tPre)
	}
	preID := preComArq["pre_id"].(string)

	// 10. Confirmar submissão: move arquivo para submissao_arquivo e envia push para a API.
	w, confOut := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/submissao/confirmar",
		`{"pre_id":"`+preID+`"}`)
	if w.Code != http.StatusOK || confOut["status"] != "entregue" {
		t.Fatalf("confirmar: %d %s", w.Code, w.Body.String())
	}
	if pacoteRecebido == nil || pacoteRecebido.Submissao == nil {
		t.Fatalf("pacote não foi enviado no push para a API")
	}
	arqEnviado, ok := pacoteRecebido.Submissao.Arquivos["1"]
	if !ok || arqEnviado.NomeArquivo != "entrega_final.pdf" || !bytes.Equal(arqEnviado.Conteudo, pdfValido) {
		t.Fatalf("arquivo no pacote enviado à API inválido: %+v", arqEnviado)
	}

	// 11. Excluir submissão restaura o arquivo de volta para tentativa_arquivo.
	if err := deps.SubmissaoS.Excluir(context.Background(), midTest, int64(subTest)); err != nil {
		t.Fatalf("excluir submissão: %v", err)
	}
	w, reaberta := chamaAluno(t, r, sess, "POST", "/a/atividade/"+midTest+"/abrir", "")
	if w.Code != http.StatusOK {
		t.Fatalf("reabrir após exclusão: %d %s", w.Code, w.Body.String())
	}
	arqsRestaurados := reaberta["tentativa"].(map[string]any)["arquivos"].(map[string]any)
	if _, ok := arqsRestaurados["1"]; !ok {
		t.Fatalf("arquivo deveria ter sido restaurado na tentativa após exclusão da submissão: %v", arqsRestaurados)
	}
}
