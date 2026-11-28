// Package judge0 — cliente da integração com o Judge0 self-hosted do Monitor.
// Portado de api/internal/judge0, com as verificações feitas na instância real
// (deploy/judge0/reference/PROBES.md):
//   - POST /submissions?wait=true  → resultado inline (envio único);
//   - POST /submissions/batch      → resultados SÓ por callback/polling
//     (o `wait=true` do batch devolve apenas tokens nesta versão);
//   - callback = PUT callback_url, corpo base64, sem header de auth — o
//     segredo vai na própria callback_url (?key=...).
//
// Envio em item único com callback (ensaios) e single sync (pré-avaliação e correção).
// Também mapeia linguagens do Seno → language_id (python/c/cpp).
package judge0

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// IDs de linguagem do Judge0 CE.
const (
	PythonID = 71
	CID      = 50
	CppID    = 54
)

// StatusAcceptedID: veredito "accepted" do Judge0 (exibição: spec §5.4).
const (
	StatusAcceptedID    = 3
	StatusWrongAnswerID = 4
)

// ErrDesligado sinaliza que SENO_JUDGE0_URL não foi configurado.
var ErrDesligado = errors.New("judge0: integração desligada (URL vazia)")

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

// StatusID devolve o id numérico do status.
func (r Resposta) StatusID() int { return int(r.Status) }

// Config do cliente.
type Config struct {
	URL   string // ex.: http://judge0:2358 (vazio = desligada)
	Token string // X-Auth-Token (se exigido pela instância)
}

// Ligada deriva da existência do URL.
func (c *Config) Ligada() bool { return strings.TrimSpace(c.URL) != "" }

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
	return Config{URL: strings.TrimSuffix(strings.TrimSpace(url), "/"), Token: token}
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
// memory como número, o callback como string (verificado ao vivo em PROBES.md;
// sem isto o polling do fallback engole json error e o run nunca fecha).
type NumStr string

func (n *NumStr) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*n = ""
		return nil
	}
	if len(s) > 0 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*n = NumStr(str)
		return nil
	}
	*n = NumStr(s)
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
	return &Client{
		base:   strings.TrimSuffix(strings.TrimSpace(cfg.URL), "/"),
		token:  cfg.Token,
		httpDo: &http.Client{Timeout: 20 * time.Second},
	}
}

// Ligada informa se o cliente possui URL configurada.
func (c *Client) Ligada() bool {
	return c != nil && c.base != ""
}

// Saude verifica rapidamente se o Judge0 responde (timeout curto de 3s).
func (c *Client) Saude(ctx context.Context) error {
	if !c.Ligada() {
		return ErrDesligado
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var info map[string]any
	return c.call(pingCtx, "/system_info", nil, &info)
}

// EnviarSync cria submissão individual e espera o resultado (inline garantido
// pela instância — PROBES.md). Usado na pré-avaliação (§5.4) e correção.
func (c *Client) EnviarSync(ctx context.Context, s Submissao) (Resposta, error) {
	var r Resposta
	if !c.Ligada() {
		return r, ErrDesligado
	}
	if strings.TrimSpace(s.SourceCode) == "" {
		return Resposta{Status: RawStatus(StatusWrongAnswerID)}, nil
	}
	if err := c.call(ctx, `/submissions?wait=true&base64_encoded=false`, s, &r); err != nil {
		return r, err
	}
	return r, nil
}

// EnviarAsync cria submissão individual para callback (ensaio, §5.3).
func (c *Client) EnviarAsync(ctx context.Context, s Submissao) (string, error) {
	if !c.Ligada() {
		return "", ErrDesligado
	}
	var r struct {
		Token string `json:"token"`
	}
	if err := c.call(ctx, `/submissions?base64_encoded=false`, s, &r); err != nil {
		return "", err
	}
	return r.Token, nil
}

// ObterUmaVez consulta o estado atual do token sem loop de espera, evitando
// travar requisições HTTP de polling quando há vários testes pendentes.
func (c *Client) ObterUmaVez(ctx context.Context, token string) (Resposta, error) {
	if !c.Ligada() {
		return Resposta{}, ErrDesligado
	}
	return c.umaVez(ctx, token)
}

// Obter puxa o resultado direto (fallback de polling curto).
func (c *Client) Obter(ctx context.Context, token string) (Resposta, error) {
	if !c.Ligada() {
		return Resposta{}, ErrDesligado
	}
	var r Resposta
	var err error
	for i := 0; i < 10; i++ { // teto ~2 s local
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
	if !c.Ligada() {
		return ErrDesligado
	}
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
