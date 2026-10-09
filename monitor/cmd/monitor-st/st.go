// Command monitor-st emite um token de SSO para testar o Monitor SEM a API
// (PLANO_MONITOR Fase 3): assina com SENO_MONITOR_SECRET e imprime o `st`
// (emitor de dev; produção emite via GET /v1/atribuicoes/:id/entrar da API).
//
// Uso (dev):
//
//	$env:SENO_MONITOR_SECRET="..."
//	go run ./cmd/monitor-st -sub 12 -mid atv_8f840f5b7f23
//	# cole a saída em: http://localhost:8081/entrar?st=...
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/seno-project/seno/contract"
)

func main() {
	sub := flag.Int64("sub", 0, "api_id do aluno")
	mid := flag.String("mid", "", "monitor_id da atividade")
	ttl := flag.Duration("ttl", 5*time.Minute, "vida do token")
	url := flag.String("url", "http://localhost:8081", "base do Monitor (para montar a URL)")
	flag.Parse()

	secret := os.Getenv("SENO_MONITOR_SECRET")
	if err := contract.VerificarSecret(secret); err != nil {
		fmt.Fprintln(os.Stderr, "SENO_MONITOR_SECRET:", err)
		os.Exit(1)
	}
	if *sub == 0 || *mid == "" {
		fmt.Fprintln(os.Stderr, "uso: -sub <api_id do aluno> -mid <monitor_id>")
		os.Exit(2)
	}
	jti, err := contract.STNonce()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nonce:", err)
		os.Exit(1)
	}
	tok, err := contract.AssinarST([]byte(secret), time.Now(), *ttl, contract.SSOIss, contract.SSOAud, *sub, *mid, jti)
	if err != nil {
		fmt.Fprintln(os.Stderr, "assinando:", err)
		os.Exit(1)
	}
	fmt.Printf("st=%s\nurl=%s/entrar?st=%s\n", tok, *url, tok)
}
