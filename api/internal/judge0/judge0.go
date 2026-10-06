// Package judge0 — cliente da integração com o Judge0 self-hosted
// (PROJETO §Execução do código): API fala apenas com o Judge0; envio em
// lote com callback por submissão (sem polling). Sem SENO_JUDGE0_URL a
// integração fica desligada e as correções seguem pendentes (correção
// manual pelo professor).
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

// IDs de linguagem do Judge0 CE (padrões; ajustáveis por env).
const (
	PythonID = 71 // Python 3
	CID      = 50 // C (GCC)
	CppID    = 54 // C++ (GCC)
)

// Veredito aceito pelo Judge0.
const StatusAcceptedID = 3

// RawStatus absorve `status` do Judge0 nos dois formatos (objeto ou id).
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

// Config da integração.
type Config struct {
	URL      string // ex.: http://judge0:2358 (vazio = desligado)
	Token    string // X-Auth-Token do Judge0
	Secret   string // chave compartilhada validada no callback (?key=)
	Origem   string // base do callback (ex.: http://api:8080)
	PythonID int
	CID      int
	CppID    int
}

// ligada deriva da existência do URL.
func (c *Config) Ligada() bool { return c.URL != "" }

// LinguagemDo seno → id Judge0.
func (c *Config) LinguagemDo(linguagem string) (int, bool) {
	switch linguagem {
	case "python":
		return c.pythonID(), true
	case "c":
		return c.cID(), true
	case "cpp":
		return c.cppID(), true
	}
	return 0, false
}

func (c *Config) pythonID() int {
	if c.PythonID != 0 {
		return c.PythonID
	}
	return PythonID
}

func (c *Config) cID() int {
	if c.CID != 0 {
		return c.CID
	}
	return CID
}

func (c *Config) cppID() int {
	if c.CppID != 0 {
		return c.CppID
	}
	return CppID
}

// CarregarConfigDasEnv lê SENO_JUDGE0_URL/TOKEN/SECRET/ORIGEM.
func CarregarConfigDasEnvLigada(url, token, secret, origem string) Config {
	return Config{URL: url, Token: token, Secret: secret, Origem: origem}
}

// SubmissaoJudge0 é um item do batch (PROJETO: lote).
type SubmissaoJudge0 struct {
	LanguageID     int    `json:"language_id"`
	SourceCode     string `json:"source_code"`
	Stdin          string `json:"stdin,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	CallbackURL    string `json:"callback_url,omitempty"`
}

// RespostaJudge0 é o corpo do callback (objeto do Judge0; status vem
// como objeto {id} no corpo completo, id direto quando pedido por fields).
type RespostaJudge0 struct {
	Token         string    `json:"token"`
	Status        RawStatus `json:"status"`
	Stdout        string    `json:"stdout"`
	Stderr        string    `json:"stderr"`
	CompileOutput string    `json:"compile_output"`
	Time          string    `json:"time"`
	Memory        string    `json:"memory"`
	CPUTime       string    `json:"cpu_time"`
}

func (r RespostaJudge0) StatusID() int { return int(r.Status) }

// StatusVeredito descreve o status id ps do Judge0.
func StatusVeredito(statusID int) string {
	if statusID == StatusAcceptedID {
		return "accepted"
	}
	return fmt.Sprintf("judge0:%d", statusID)
}

// Client fala HTTPS/HTTP com o Judge0 interno (rede da UFPA).
type Client struct {
	base   string
	token  string
	httpDo *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		base:   cfg.URL,
		token:  cfg.Token,
		httpDo: &http.Client{Timeout: 30 * time.Second},
	}
}

// EnviarBatch cria submissões em lote (PROJETO §Backend: sem polling —
// resultados chegam via callback_url de cada item).
func (cli *Client) EnviarBatch(ctx context.Context, itens []SubmissaoJudge0) error {
	body, err := json.Marshal(itens)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, cli.base+"/submissions/batch", bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cli.token != "" {
		req.Header.Set("X-Auth-Token", cli.token)
	}
	resp, err := cli.httpDo.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		dados, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("judge0 batch: status %d: %s", resp.StatusCode, dados)
	}
	return nil
}

// Saude ping simples (não usado no fluxo; diagnóstico).
func (cli *Client) Saude(ctx context.Context) error {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, cli.base+"/system_info", nil,
	)
	if err != nil {
		return err
	}
	if cli.token != "" {
		req.Header.Set("X-Auth-Token", cli.token)
	}
	resp, err := cli.httpDo.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("judge0 indisponível: status %d", resp.StatusCode)
	}
	return nil
}
