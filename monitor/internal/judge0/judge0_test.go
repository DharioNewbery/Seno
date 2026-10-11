// Package judge0 — testes do contrato JSON (corpos capturados ao vivo em
// PROBES.md). O Judge0 devolve memory (e às vezes time) como número no GET e
// como string no callback: NumStr absorve os dois (bug real preso no e2e).
package judge0

import (
	"encoding/json"
	"testing"
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
