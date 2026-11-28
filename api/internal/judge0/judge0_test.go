package judge0

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRespostaJudge0NumStrEBase64(t *testing.T) {
	// Corpo real do Judge0 com memory numérico e stdout base64 com quebra de linha (PROBES.md).
	raw := `{"stdout":"NDIK\n","time":"0.011","memory":3240,"stderr":null,"token":"tok1","compile_output":null,"status":{"id":3,"description":"Accepted"}}`
	var r RespostaJudge0
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal falhou: %v", err)
	}
	if r.StatusID() != StatusAcceptedID || r.EmFila() {
		t.Fatalf("status inesperado: %d", r.StatusID())
	}
	if r.Memory.String() != "3240" || r.Time.String() != "0.011" {
		t.Fatalf("memory/time inesperados: mem=%q time=%q", r.Memory, r.Time)
	}
	r.DecodificarBase64()
	if r.Stdout != "42\n" {
		t.Fatalf("stdout decodificado=%q, esperado %q", r.Stdout, "42\n")
	}

	// Status transitório (In Queue = 1, Processing = 2) deve ser detectado por EmFila().
	var emFila RespostaJudge0
	if err := json.Unmarshal([]byte(`{"status":{"id":2}}`), &emFila); err != nil {
		t.Fatalf("unmarshal status 2: %v", err)
	}
	if !emFila.EmFila() {
		t.Fatal("esperava EmFila() == true para status 2")
	}
}

func TestEnviarBatchFatiamento20EEnvelope(t *testing.T) {
	var lotes int32
	var totalRecebidos int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/submissions/batch" {
			http.NotFound(w, r)
			return
		}
		var env struct {
			Submissions []SubmissaoJudge0 `json:"submissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(env.Submissions) == 0 || len(env.Submissions) > MaxBatchSize {
			t.Errorf("tamanho do lote inválido: %d (máx %d)", len(env.Submissions), MaxBatchSize)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		atomic.AddInt32(&lotes, 1)
		atomic.AddInt32(&totalRecebidos, int32(len(env.Submissions)))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`[{"token":"t1"}]`))
	}))
	defer srv.Close()

	cli := NewClient(CarregarConfigDasEnvLigada(srv.URL, "tok", "sec", "http://api:8080"))
	itens := make([]SubmissaoJudge0, 45)
	for i := range itens {
		itens[i] = SubmissaoJudge0{LanguageID: PythonID, SourceCode: "print(1)"}
	}
	if err := cli.EnviarBatch(context.Background(), itens); err != nil {
		t.Fatalf("EnviarBatch: %v", err)
	}
	if got := atomic.LoadInt32(&lotes); got != 3 {
		t.Fatalf("lotes enviados=%d, esperado 3 (20+20+5)", got)
	}
	if got := atomic.LoadInt32(&totalRecebidos); got != 45 {
		t.Fatalf("itens recebidos=%d, esperado 45", got)
	}
}

func TestEnviarSyncCodigoVazioOfflineESemTravamento(t *testing.T) {
	// 1. Código em branco não chama o Judge0 (evita 422) e devolve Wrong Answer imediato.
	cliSemURL := NewClient(Config{})
	resp, err := cliSemURL.EnviarSync(context.Background(), SubmissaoJudge0{
		LanguageID: PythonID,
		SourceCode: "   \n\t ",
	})
	if err != nil {
		t.Fatalf("código vazio não deveria falhar: %v", err)
	}
	if resp.StatusID() != StatusWrongAnswerID {
		t.Fatalf("status para código vazio=%d, esperado %d", resp.StatusID(), StatusWrongAnswerID)
	}

	// 2. Judge0 fora do ar (porta fechada) falha rápido com erro explícito.
	srvFechado := httptest.NewServer(http.NotFoundHandler())
	srvFechado.Close()
	cliOffline := NewClient(CarregarConfigDasEnvLigada(srvFechado.URL, "", "s", "http://api:8080"))
	if _, err := cliOffline.EnviarBatchSincrono(context.Background(), []SubmissaoJudge0{
		{LanguageID: PythonID, SourceCode: "print(1)"},
	}); err == nil {
		t.Fatal("esperava erro com Judge0 fora do ar")
	}

	// 3. Judge0 travado respeita cancelamento de contexto sem bloquear indefinidamente.
	desbloqueia := make(chan struct{})
	srvLento := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-desbloqueia
	}))
	defer func() {
		close(desbloqueia)
		srvLento.Close()
	}()

	cliLento := NewClient(CarregarConfigDasEnvLigada(srvLento.URL, "", "s", "http://api:8080"))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	inicio := time.Now()
	if _, err := cliLento.EnviarSync(ctx, SubmissaoJudge0{LanguageID: PythonID, SourceCode: "print(1)"}); err == nil {
		t.Fatal("esperava timeout com Judge0 travado")
	}
	if dur := time.Since(inicio); dur > 2*time.Second {
		t.Fatalf("chamada demorou demais (%v); deveria respeitar o contexto", dur)
	}
}

func TestEnviarBatchSincronoPolling(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/submissions/batch" && r.Method == http.MethodPost {
			var env struct {
				Submissions []SubmissaoJudge0 `json:"submissions"`
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
			st := StatusProcessingID
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

	cli := NewClient(CarregarConfigDasEnvLigada(srv.URL, "", "s", "http://api:8080"))
	resps, err := cli.EnviarBatchSincrono(context.Background(), []SubmissaoJudge0{
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
