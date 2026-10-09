// Command monitor-fixtures alimenta o Monitor com atividades de teste,
// sem a API rodar (PLANO_MONITOR Fase 2.3): monta um envelope válido da
// spec §2, assina com SENO_MONITOR_SECRET e envia à URL dada.
//
// Uso (dev):
//
//	$env:SENO_MONITOR_SECRET="..."   # >= 32 bytes, o mesmo do Monitor
//	go run ./cmd/monitor-fixtures -url http://localhost:8081
//
// Com arquivo próprio do envelope:
//
//	go run ./cmd/monitor-fixtures -url ... -arquivo envelope.json
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/seno-project/seno/contract"
)

func main() {
	url := flag.String("url", "http://localhost:8081", "base do Monitor")
	arquivo := flag.String("arquivo", "", "arquivo JSON do envelope (vazio = exemplo da spec)")
	flag.Parse()

	secret := os.Getenv("SENO_MONITOR_SECRET")
	if err := contract.VerificarSecret(secret); err != nil {
		fmt.Fprintln(os.Stderr, "SENO_MONITOR_SECRET: ", err)
		os.Exit(1)
	}

	var corpo []byte
	if *arquivo != "" {
		var err error
		if corpo, err = os.ReadFile(*arquivo); err != nil {
			fmt.Fprintln(os.Stderr, "lendo envelope:", err)
			os.Exit(1)
		}
	} else {
		corpo = exemploEnvelope()
	}

	mac, tempo := contract.AssinarMAC([]byte(secret), time.Now(), corpo)
	req, err := http.NewRequestWithContext(context.Background(),
		"POST", *url+"/interna/v1/atividades", bytes.NewReader(corpo))
	if err != nil {
		fmt.Fprintln(os.Stderr, "request:", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(contract.HeaderMAC, mac)
	req.Header.Set(contract.HeaderTime, tempo)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "envio:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	fmt.Printf("HTTP %d\n%s\n", resp.StatusCode, body)
	// shortcut: resposta 200 já é re-envio idempotente (mesmo api_id);
	// retorne 1 para distinguir em automação futura, se necessário.
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

// exemploEnvelope é o jsonc da spec §2, na forma final do módulo contract.
func exemploEnvelope() []byte {
	t := contract.Transferencia{
		APIID:    42,
		GeradaEm: time.Now().UTC(),
		Turma:    contract.Turma{APIID: 7, Nome: "2026.1 Filas"},
		Alunos:   []contract.Aluno{{APIID: 12, Nome: "Ana R."}, {APIID: 13, Nome: "B. Lima"}},
		Janela: contract.Janela{
			Inicio:       time.Now().Add(-80 * time.Minute),
			Prazo:        time.Now().Add(40 * time.Minute),
			DuracaoSeg:   intPtr(5400),
			PodeAtrasado: true,
			LimiteAtraso: stringPtr("P1D"),
		},
		Regras: contract.Regras{
			SubmissoesMultiplas:  false,
			MaxSubmissoes:        1,
			LinguagensPermitidas: []string{"python"},
			Autocomplete:         true,
		},
		Carga: contract.Carga{
			Atividade: contract.AtividadeCarga{APIID: 12, Nome: "Prova 1", Enunciado: "Implemente o dobro."},
			Tarefas: []contract.TarefaCarga{{
				Ordem: 1, TarefaAPIID: 5, Nome: "Dobro", ValorPts: 10,
				Enunciado: "Leia um inteiro e imprima o dobro.", Linguagem: "python",
				Limites: &contract.Limites{TempoCPUMs: 2000, TempoTotalMs: 5000, MemoriaMB: 256},
				Testes: []contract.TesteCarga{
					{Stdin: "3", StdoutEsperado: "6", Publico: true},
					{Stdin: "", StdoutEsperado: "", Publico: false},
				},
			}},
		},
	}
	b, _ := json.MarshalIndent(t, "", "  ")
	return b
}

func intPtr(i int) *int          { return &i }
func stringPtr(s string) *string { return &s }
