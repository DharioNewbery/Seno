// Package judge0 — cliente da integração com o Judge0 self-hosted do Monitor.
// Portado de api/internal/judge0 (godé linter médio: os mesmos tipos de
// resposta/com Armageddon), mas com as verificações da extração à RODAR na
// instância real (deploy/judge0/reference/PROBES.md):
//   - POST /submissions?wait=true  → resultado inline (envio único);
//   - POST /submissions/batch      → resultados SÓ por callback/polling
//     (o `wait=true` do batch devolve apenas tokens nesta versão);
//   - callback = PUT callback_url, corpo base64, sem header de auth o
//     segredo vai na própria callback_url (?key=...).
//
// Envio em item único com callback (ensaios) e single sync (pré-avaliação).
// Também mapeia linguagens do Seno → language_id (python/c/cpp).
package judge0

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IDs de linguagem do Judge0 CE.
const (
	PythonID = 71
	CID      = 50
	CppID    = 54
)

// StatusAcceptedID: veredito "accepted" do Judge0 (exibição: spec §5.4).
const StatusAcceptedID = 3

// RawStatus absorve `status` nos dois formatos (id direto ou objeto).
type RawStatus int

func (r *RawStatus) UnmarshalJSON(b []byte) error {
	var direto int
	if err := json.Unmarshal(b, &direto); err == nil {
		*r = RawStatus(direto)
		return nil
	}
	var obj struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	*r = RawStatus(obj.ID)
	return nil
}

// StatusID devolve o id, já legacy UnmarshalJSON.
func (r Resposta) StatusID() int { return int(r.Status) }

// Config do cliente.
type Config struct {
	URL   string // ex.: http://judge0:2358 (vazio = desligada)
	Token string // X-Auth-Token (se exigido pela instância)
}

// Ligada deriva da existência do URL.
func (c *Config) Ligada() bool { return c.URL != "" }

// LinguagemDo mapeia o identificador próprio do Seno.
func LinguagemDo(linguagem string) (int, bool) {
	switch linguagem {
	case "python":
		return PythonID, true
	case "c":
		return CID, true
	case "cpp":
		return CppID, true
	}
	return 0, false
}

// CarregarConfig das variáveis já lidas (platform.Config).
func CarregarConfig(url, token string) Config {
	return Config{URL: url, Token: token}
}

// Submissao item do lote / envio único (campos conforme PROBES.md).
type Submissao struct {
	LanguageID     int    `json:"language_id"`
	SourceCode     string `json:"source_code"`
	Stdin          string `json:"stdin,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	CallbackURL    string `json:"callback_url,omitempty"`

	// Limites da tarefa (spec §2 carga.limits), já convertido.
	CPUTimeSec  float64 `json:"cpu_time_limit,omitempty"`
	WallTimeSec float64 `json:"wall_time_limit,omitempty"`
	MemoriaKB   int     `json:"memory_limit,omitempty"`
}

// NumStr absorve número ou string do Judge0 no mesmo campo — o GET devolve
// memory como número, o callback como string (verido ao vivo em PROBES.md;
// sem isto o polling do fallback engole json error e o run nunca fecha).
type NumStr string

func (n *NumStr) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*n = NumStr(s)
		return nil
	}
	*n = NumStr(b)
	return nil
}

// Resposta do callback/get (corpo "status" vem objeto {id}).
type Resposta struct {
	Token         string    `json:"token"`
	Status        RawStatus `json:"status"`
	Stdout        string    `json:"stdout"`
	Stderr        string    `json:"stderr"`
	CompileOutput string    `json:"compile_output"`
	Time          NumStr    `json:"time"`
	Memory        NumStr    `json:"memory"`
	CPUTime       NumStr    `json:"cpu_time"`
	Message       string    `json:"message"`
}

// Client fala com o Judge0 interno (rede da UFPA / Docker Desktop dev).
type Client struct {
	base   string
	token  string
	httpDo *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{base: cfg.URL, token: cfg.Token,
		httpDo: &http.Client{Timeout: 30 * time.Second}}
}

// EnviarSync cria submissão individual e espera o resultado (inline garantido
// pela instância — PROBES.md). Usado na pré-avaliação (§5.4).
func (c *Client) EnviarSync(ctx context.Context, s Submissao) (Resposta, error) {
	var r Resposta
	if err := c.call(ctx, `/submissions?wait=true&base64_encoded=false`, s, &r); err != nil {
		return r, err
	}
	return r, nil
}

// EnviarAsync cria submissão individual para callback (ensaio, §5.3).
func (c *Client) EnviarAsync(ctx context.Context, s Submissao) (string, error) {
	var r struct {
		Token string `json:"token"`
	}
	if err := c.call(ctx, `/submissions?base64_encoded=false`, s, &r); err != nil {
		return "", err
	}
	return r.Token, nil
}

// Obter puxa o resultado direto (fallback de polling). O GET com wait=true
// desta build devolve "In Queue" instantâneo (não bloqueia — verificado em
// PROBES.md), então re-polling local curto até um estado final.
func (c *Client) Obter(ctx context.Context, token string) (Resposta, error) {
	var r Resposta
	var err error
	for i := 0; i < 20; i++ { // teto ~4 s local (não repercute no GET do run)
		if r, err = c.umaVez(ctx, token); err != nil {
			return r, err
		}
		if r.StatusID() != 1 && r.StatusID() != 2 {
			return r, nil
		}
		select {
		case <-ctx.Done():
			return r, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return r, nil
}

func (c *Client) umaVez(ctx context.Context, token string) (Resposta, error) {
	var r Resposta
	if err := c.call(ctx, "/submissions/"+token+"?base64_encoded=false", nil, &r); err != nil {
		return r, err
	}
	return r, nil
}

func (c *Client) call(ctx context.Context, path string, corpo any, saida any) error {
	metodo := http.MethodGet
	var rd io.Reader
	if corpo != nil {
		b, err := json.Marshal(corpo)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
		metodo = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, metodo, c.base+path, rd)
	if err != nil {
		return err
	}
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("X-Auth-Token", c.token)
	}
	resp, err := c.httpDo.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dados, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("judge0 %s: status %d: %s", path, resp.StatusCode, dados)
	}
	return json.Unmarshal(dados, saida)
}
