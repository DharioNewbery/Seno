package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

const seg = "0123456789abcdef0123456789abcdef0123456789abcdef" // 48 bytes

func roundTrip(t *testing.T, mac, tempo string, corpo []byte, agora time.Time) error {
	t.Helper()
	return VerificarMAC([]byte(seg), corpo, mac, tempo, time.Minute)
}

func TestMACIdaEVolta(t *testing.T) {
	corpo := []byte(`{"api_id":42}`)
	mac, tempo := AssinarMAC([]byte(seg), time.Now(), corpo)
	if err := roundTrip(t, mac, tempo, corpo, time.Now()); err != nil {
		t.Fatalf("MAC válido rejeitado: %v", err)
	}
}

func TestMACCorpoAlterado(t *testing.T) {
	corpo := []byte(`{"api_id":42}`)
	mac, tempo := AssinarMAC([]byte(seg), time.Now(), corpo)
	if err := roundTrip(t, mac, tempo, []byte(`{"api_id":43}`), time.Now()); !errors.Is(err, ErrMACInvalido) {
		t.Fatalf("corpo alterado deveria dar ErrMACInvalido, veio: %v", err)
	}
}

func TestMACSegredoDiferente(t *testing.T) {
	corpo := []byte(`x`)
	mac, tempo := AssinarMAC([]byte("outro-segredo-totalmente-diferente-bah"), time.Now(), corpo) // emissor com segredo errado
	if err := roundTrip(t, mac, tempo, corpo, time.Now()); !errors.Is(err, ErrMACInvalido) {
		t.Fatalf("segredo diferente deveria dar ErrMACInvalido, veio: %v", err)
	}
}

func TestMACReplayForaDaJanela(t *testing.T) {
	corpo := []byte(`x`)
	mac, tempo := AssinarMAC([]byte(seg), time.Now().Add(-2*time.Minute), corpo)
	if err := roundTrip(t, mac, tempo, corpo, time.Now()); !errors.Is(err, ErrTempoFora) {
		t.Fatalf("replay deveria dar ErrTempoFora, veio: %v", err)
	}
}

func TestMACSemMACNDSemTempo(t *testing.T) {
	corpo := []byte(`x`)
	if err := roundTrip(t, "", "2026-10-09T12:00:00Z", corpo, time.Now()); !errors.Is(err, ErrMACInvalido) {
		t.Fatalf("MAC vazio deveria dar ErrMACInvalido, veio: %v", err)
	}
	mac, _ := AssinarMAC([]byte(seg), time.Now(), corpo)
	if err := roundTrip(t, mac, "", corpo, time.Now()); !errors.Is(err, ErrMACInvalido) {
		t.Fatalf("tempo vazio deveria dar ErrMACInvalido, veio: %v", err)
	}
	if err := roundTrip(t, mac, "não-é-rfc3339", corpo, time.Now()); !errors.Is(err, ErrTempo) {
		t.Fatalf("tempo ilegível deveria dar ErrTempo, veio: %v", err)
	}
}

func TestEnvelopeArredonda(t *testing.T) {
	// O JSON da transferência sobrevive ida e volta sem tocar nos campos
	// (backup da estrutura pensada para o fio).
	origem := `{"api_id":42,"monitor_id":"","gerada_em":"2026-10-07T20:00:00Z",
	  "turma":{"api_id":7,"nome":"2026.1 Filas"},
	  "alunos":[{"api_id":12,"nome":"Ana R."},{"api_id":13,"nome":"B. Lima"}],
	  "janela":{"inicio":"2026-10-10T08:00:00Z","prazo":"2026-10-10T12:00:00Z",
	    "duracao_seg":5400,"pode_atrasado":true,"limite_atraso":"P1D"},
	  "regras":{"submissoes_multiplas":false,"max_submissoes":1,
	    "linguagens_permitidas":["python"],"autocomplete":true},
	  "carga":{"atividade":{"api_id":12,"nome":"Prova 1","enunciado":"..."},
	    "tarefas":[{"ordem":1,"tarefa_api_id":5,"nome":"Dobro","valor_pts":10,
	      "enunciado":"...","linguagem":"python",
	      "limites":{"tempo_cpu_ms":2000,"tempo_total_ms":5000,"memoria_mb":256},
	      "testes":[{"stdin":"3","stdout_esperado":"6","publico":true},
	               {"stdin":"","stdout_esperado":"","publico":false}]}]}}`
	var tr Transferencia
	if err := json.Unmarshal([]byte(origem), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tr.APIID != 42 || len(tr.Alunos) != 2 || len(tr.Carga.Tarefas) != 1 ||
		len(tr.Carga.Tarefas[0].Testes) != 2 {
		t.Fatalf("campos do envelope se perderam: %+v", tr)
	}
	// Congelado: o JSON remarshaleado usa os mesmos nomes de campo (api_id etc).
	var deVolta bytes.Buffer
	enc := json.NewEncoder(&deVolta)
	if err := enc.Encode(tr); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again Transferencia
	if err := json.Unmarshal(deVolta.Bytes(), &again); err != nil {
		t.Fatalf("remarshal: %v", err)
	}
	if again.APIID != 42 || again.Carga.Tarefas[0].Limites.TempoCPUMs != 2000 {
		t.Fatalf("round trip alterou conteúdo: %+v", again)
	}
}
