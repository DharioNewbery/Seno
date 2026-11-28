package monitor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/seno-project/seno/api/internal/platform"
)

func TestMonitorForaDoArGera503ClaroESemTravar(t *testing.T) {
	// Simula Monitor fora do ar (servidor encerrado).
	srv := httptest.NewServer(http.NotFoundHandler())
	urlFechada := srv.URL
	srv.Close()

	svc := New(nil, nil, urlFechada, urlFechada, segTeste)

	if err := svc.Saude(context.Background()); !errors.Is(err, ErrMonitorIndisponivel) {
		t.Fatalf("Saude com Monitor offline: esperado ErrMonitorIndisponivel, obtido %v", err)
	}

	verificar503 := func(nome string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: esperava erro com Monitor fora do ar", nome)
		}
		var apiErr *platform.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("%s: erro %T (%v) não é *platform.APIError (viraria 500 genérico)", nome, err, err)
		}
		if apiErr.Status != http.StatusServiceUnavailable || apiErr.Code != platform.CodeServiceUnavailable {
			t.Fatalf("%s: esperado 503 %s, obtido %d %s (%s)",
				nome, platform.CodeServiceUnavailable, apiErr.Status, apiErr.Code, apiErr.Message)
		}
	}

	verificar503("Reexecutar", svc.Reexecutar(context.Background(), "atv_1", 10))
	verificar503("ExcluirSubmissao", svc.ExcluirSubmissao(context.Background(), "atv_1", 10))
	verificar503("ColetarTentativa", svc.ColetarTentativa(context.Background(), "atv_1", 10))
	_, errSub := svc.ObterSubmissao(context.Background(), "atv_1", 10)
	verificar503("ObterSubmissao", errSub)
	_, errPac := svc.ObterPacoteSubmissao(context.Background(), "atv_1", 10)
	verificar503("ObterPacoteSubmissao", errPac)
}

func TestMonitorMapeiaErrosDeNegocioESemTravamento(t *testing.T) {
	// 1. Monitor responde 422 / 404 com envelope padrão: mensagem preservada ao usuário.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"erro":{"codigo":"VALIDACAO","mensagem":"submissão não encontrada"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"erro":{"codigo":"NAO_ENCONTRADO","mensagem":"Atividade não encontrada no Monitor."}}`))
	}))
	defer srv.Close()

	svc := New(nil, nil, srv.URL, srv.URL, segTeste)

	errDel := svc.ExcluirSubmissao(context.Background(), "atv_1", 10)
	var apiErr *platform.APIError
	if !errors.As(errDel, &apiErr) || apiErr.Status != http.StatusUnprocessableEntity || apiErr.Message != "submissão não encontrada" {
		t.Fatalf("422 mal mapeado: %+v (err=%v)", apiErr, errDel)
	}

	_, errGet := svc.ObterSubmissao(context.Background(), "atv_inexistente", 10)
	if !errors.As(errGet, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("404 mal mapeado: %+v (err=%v)", apiErr, errGet)
	}

	// 2. Monitor travado (sem responder) respeita timeout do contexto e devolve 503 sem travar a API.
	desbloqueia := make(chan struct{})
	srvTravado := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-desbloqueia
	}))
	defer func() {
		close(desbloqueia)
		srvTravado.Close()
	}()

	svcTravado := New(nil, nil, srvTravado.URL, srvTravado.URL, segTeste)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	inicio := time.Now()
	errTrav := svcTravado.Reexecutar(ctx, "atv_1", 10)
	if !errors.As(errTrav, &apiErr) || apiErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("Monitor travado deveria resultar em 503 SERVICO_INDISPONIVEL; obtido %v", errTrav)
	}
	if dur := time.Since(inicio); dur > 2*time.Second {
		t.Fatalf("chamada bloqueou por %v; deveria abortar pelo contexto", dur)
	}
}
