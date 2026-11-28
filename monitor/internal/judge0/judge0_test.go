// Package judge0 — testes do contrato JSON (corpos capturados ao vivo em
// PROBES.md). O Judge0 devolve memory (e às vezes time) como número no GET e
// como string no callback: NumStr absorve os dois (bug real preso no e2e).
package judge0

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRespostaDecodificaCorposAoVivo: terminal no GET (memory número) e no
// callback (stdout base64, memory string).
func TestRespostaDecodificaCorposAoVivo(t *testing.T) {
	get := `{"stdout":"2\n","time":"0.016","memory":3240,"stderr":null,
	    "token":"t1","compile_output":null,"message":null,
	    "status":{"id":4,"description":"Wrong Answer"}}`
	var rget Resposta
	if err := json.Unmarshal([]byte(get), &rget); err != nil {
		t.Fatalf("GET terminal: %v", err)
	}
	if rget.StatusID() != 4 || string(rget.Memory) != "3240" || string(rget.Time) != "0.016" {
		t.Fatalf("GET terminal: %+v", rget)
	}

	cb := `{"stdout":"Ng==","time":"0.011","memory":"3496","stderr":null,
	    "token":"t2","compile_output":null,"message":null,
	    "status":{"id":3,"description":"Accepted"}}`
	var rcb Resposta
	if err := json.Unmarshal([]byte(cb), &rcb); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if rcb.StatusID() != 3 || string(rcb.Memory) != "3496" {
		t.Fatalf("callback: %+v", rcb)
	}
}

// TestNumStrFormato: os quatro casos — número cru, string, null e ausente.
func TestNumStrFormato(t *testing.T) {
	var casos = []struct {
		json, quer NumStr
	}{
		{`3240`, "3240"},
		{`"3496"`, "3496"},
		{`null`, ""},
	}
	for _, c := range casos {
		var n NumStr
		if err := json.Unmarshal([]byte(c.json), &n); err != nil {
			t.Fatalf("%s: %v", c.json, err)
		}
		if n != c.quer {
			t.Fatalf("%s: quero %q, veio %q", c.json, c.quer, n)
		}
	}
	var ausente struct {
		M NumStr `json:"m"`
	}
	if err := json.Unmarshal([]byte(`{}`), &ausente); err != nil {
		t.Fatalf("campo ausente: %v", err)
	}
	if ausente.M != "" {
		t.Fatalf("campo ausente: quero vazio, veio %q", ausente.M)
	}
}

func TestClientDesligadoCodigoVazioESemTravamento(t *testing.T) {
	// 1. Cliente sem URL sinaliza ErrDesligado imediatamente.
	desligado := NewClient(CarregarConfig("", ""))
	if desligado.Ligada() {
		t.Fatal("esperava Ligada() == false com URL vazia")
	}
	if _, err := desligado.EnviarAsync(context.Background(), Submissao{LanguageID: PythonID, SourceCode: "print(1)"}); !errors.Is(err, ErrDesligado) {
		t.Fatalf("EnviarAsync desligado: esperado ErrDesligado, obtido %v", err)
	}
	if err := desligado.Saude(context.Background()); !errors.Is(err, ErrDesligado) {
		t.Fatalf("Saude desligado: esperado ErrDesligado, obtido %v", err)
	}

	// 2. Código vazio em EnviarSync devolve Wrong Answer sem chamar Judge0 (evita 422).
	chamou := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamou = true
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	cli := NewClient(CarregarConfig(srv.URL, ""))
	resp, err := cli.EnviarSync(context.Background(), Submissao{LanguageID: PythonID, SourceCode: "  \n "})
	if err != nil {
		t.Fatalf("código vazio não deveria retornar erro: %v", err)
	}
	if chamou || resp.StatusID() != StatusWrongAnswerID {
		t.Fatalf("código vazio: chamou=%v status=%d, esperado false e %d", chamou, resp.StatusID(), StatusWrongAnswerID)
	}

	// 3. Judge0 travado não prende o Monitor além do timeout do contexto.
	liberar := make(chan struct{})
	srvTravado := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-liberar
	}))
	defer func() {
		close(liberar)
		srvTravado.Close()
	}()

	cliTravado := NewClient(CarregarConfig(srvTravado.URL, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	inicio := time.Now()
	if _, err := cliTravado.ObterUmaVez(ctx, "tok-x"); err == nil {
		t.Fatal("esperava timeout em Judge0 travado")
	}
	if dur := time.Since(inicio); dur > 2*time.Second {
		t.Fatalf("ObterUmaVez bloqueou por %v", dur)
	}
}

func TestEnviarBatchSincronoPolling(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/submissions/batch" && r.Method == http.MethodPost {
			var env struct {
				Submissions []Submissao `json:"submissions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
				t.Fatalf("decode post batch: %v", err)
			}
			if len(env.Submissions) != 2 {
				t.Fatalf("esperava 2 itens não vazios no batch, obteve %d", len(env.Submissions))
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"token": "tok-1"},
				{"token": "tok-2"},
			})
			return
		}
		if r.URL.Path == "/submissions/batch" && r.Method == http.MethodGet {
			polls++
			if r.URL.Query().Get("tokens") != "tok-1,tok-2" {
				t.Fatalf("tokens inesperados: %s", r.URL.Query().Get("tokens"))
			}
			st := 2 // Processing na 1ª consulta
			if polls >= 2 {
				st = StatusAcceptedID
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"submissions": []map[string]any{
					{"token": "tok-1", "status": map[string]any{"id": st}, "stdout": "8\n"},
					{"token": "tok-2", "status": map[string]any{"id": st}, "stdout": "10\n"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cli := NewClient(CarregarConfig(srv.URL, ""))
	resps, err := cli.EnviarBatchSincrono(context.Background(), []Submissao{
		{LanguageID: PythonID, SourceCode: "print(8)"},
		{LanguageID: PythonID, SourceCode: "  "},
		{LanguageID: PythonID, SourceCode: "print(10)"},
	})
	if err != nil {
		t.Fatalf("EnviarBatchSincrono falhou: %v", err)
	}
	if len(resps) != 3 {
		t.Fatalf("esperava 3 respostas, obteve %d", len(resps))
	}
	if resps[0].StatusID() != StatusAcceptedID || resps[0].Stdout != "8\n" {
		t.Fatalf("item 0 inesperado: %+v", resps[0])
	}
	if resps[1].StatusID() != StatusWrongAnswerID {
		t.Fatalf("item 1 (vazio) inesperado: %+v", resps[1])
	}
	if resps[2].StatusID() != StatusAcceptedID || resps[2].Stdout != "10\n" {
		t.Fatalf("item 2 inesperado: %+v", resps[2])
	}
}
