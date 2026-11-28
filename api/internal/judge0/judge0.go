// Package judge0 — cliente da integração com o Judge0 self-hosted
// (PROJETO §Execução do código): API fala apenas com o Judge0; envio em
// lote com callback por submissão (PUT/POST) e envio individual síncrono
// para testes interativos. Sem SENO_JUDGE0_URL a integração fica desligada
// e as correções seguem pendentes (correção manual pelo professor).
package judge0

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// IDs de linguagem do Judge0 CE (padrões; ajustáveis por env).
const (
	PythonID = 71 // Python 3
	CID      = 50 // C (GCC)
	CppID    = 54 // C++ (GCC)
)

// Limite máximo por chamada /submissions/batch no Judge0 1.13 (PROBES.md).
const MaxBatchSize = 20

// Vereditos do Judge0.
const (
	StatusInQueueID     = 1
	StatusProcessingID  = 2
	StatusAcceptedID    = 3
	StatusWrongAnswerID = 4
)

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

// NumStr absorve número, string ou null do Judge0 no mesmo campo — o Judge0
// envia `memory` como número JSON e `time` como string (PROBES.md).
type NumStr string

func (n *NumStr) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*n = ""
		return nil
	}
	if s[0] == '"' {
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

func (n NumStr) String() string { return string(n) }

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

// Ligada deriva da existência do URL.
func (c *Config) Ligada() bool { return strings.TrimSpace(c.URL) != "" }

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

// CarregarConfigDasEnvLigada lê SENO_JUDGE0_URL/TOKEN/SECRET/ORIGEM.
func CarregarConfigDasEnvLigada(url, token, secret, origem string) Config {
	return Config{
		URL:    strings.TrimSuffix(strings.TrimSpace(url), "/"),
		Token:  token,
		Secret: secret,
		Origem: strings.TrimSuffix(strings.TrimSpace(origem), "/"),
	}
}

// SubmissaoJudge0 é um item de execução no Judge0.
type SubmissaoJudge0 struct {
	LanguageID     int     `json:"language_id"`
	SourceCode     string  `json:"source_code"`
	Stdin          string  `json:"stdin,omitempty"`
	ExpectedOutput string  `json:"expected_output,omitempty"`
	CallbackURL    string  `json:"callback_url,omitempty"`
	CPUTimeSec     float64 `json:"cpu_time_limit,omitempty"`
	WallTimeSec    float64 `json:"wall_time_limit,omitempty"`
	MemoriaKB      int     `json:"memory_limit,omitempty"`
}

// RespostaJudge0 é o corpo do callback ou retorno síncrono do Judge0.
type RespostaJudge0 struct {
	Token         string    `json:"token"`
	Status        RawStatus `json:"status"`
	Stdout        string    `json:"stdout"`
	Stderr        string    `json:"stderr"`
	CompileOutput string    `json:"compile_output"`
	Time          NumStr    `json:"time"`
	Memory        NumStr    `json:"memory"`
	CPUTime       NumStr    `json:"cpu_time"`
}

func (r RespostaJudge0) StatusID() int { return int(r.Status) }

// EmFila informa se o status do Judge0 ainda é transitório (In Queue / Processing).
func (r RespostaJudge0) EmFila() bool {
	id := r.StatusID()
	return id == StatusInQueueID || id == StatusProcessingID
}

// DecodificarBase64 converte os campos stdout/stderr/compile_output entregues em base64
// nos callbacks PUT do Judge0 (PROBES.md) para UTF-8 legível.
func (r *RespostaJudge0) DecodificarBase64() {
	r.Stdout = decodificaB64(r.Stdout)
	r.Stderr = decodificaB64(r.Stderr)
	r.CompileOutput = decodificaB64(r.CompileOutput)
	if r.CompileOutput != "" {
		r.Stdout = ""
	}
}

func decodificaB64(s string) string {
	if s == "" {
		return ""
	}
	limpo := strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
	if limpo == "" {
		return s
	}
	if b, err := base64.StdEncoding.DecodeString(limpo); err == nil && utf8.Valid(b) {
		return string(b)
	}
	return s
}

// StatusVeredito descreve o status id do Judge0.
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
		base:   strings.TrimSuffix(strings.TrimSpace(cfg.URL), "/"),
		token:  cfg.Token,
		httpDo: &http.Client{Timeout: 20 * time.Second},
	}
}

// EnviarBatch cria submissões em lote (PROJETO §Backend: resultados chegam via
// callback_url de cada item). Respeita o envelope {"submissions": [...]} e o
// limite de 20 itens por lote do Judge0 1.13 (PROBES.md).
func (cli *Client) EnviarBatch(ctx context.Context, itens []SubmissaoJudge0) error {
	if len(itens) == 0 {
		return nil
	}
	if cli.base == "" {
		return fmt.Errorf("judge0 desligado: URL vazia")
	}
	for inicio := 0; inicio < len(itens); inicio += MaxBatchSize {
		fim := inicio + MaxBatchSize
		if fim > len(itens) {
			fim = len(itens)
		}
		fatia := itens[inicio:fim]
		envelope := struct {
			Submissions []SubmissaoJudge0 `json:"submissions"`
		}{Submissions: fatia}
		body, err := json.Marshal(envelope)
		if err != nil {
			return err
		}
		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		req, err := http.NewRequestWithContext(
			callCtx, http.MethodPost, cli.base+"/submissions/batch?base64_encoded=false", bytes.NewReader(body),
		)
		if err != nil {
			cancel()
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if cli.token != "" {
			req.Header.Set("X-Auth-Token", cli.token)
		}
		resp, err := cli.httpDo.Do(req)
		if err != nil {
			cancel()
			return err
		}
		dados, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		cancel()
		if resp.StatusCode/100 != 2 {
			return fmt.Errorf("judge0 batch: status %d: %s", resp.StatusCode, dados)
		}
	}
	return nil
}

// EnviarSync cria submissão individual assíncrona (sem ?wait=true para não
// prender threads do servidor HTTP do Judge0) e faz polling curto até concluir.
func (cli *Client) EnviarSync(ctx context.Context, item SubmissaoJudge0) (RespostaJudge0, error) {
	if strings.TrimSpace(item.SourceCode) == "" {
		return RespostaJudge0{Status: RawStatus(StatusWrongAnswerID)}, nil
	}
	if cli.base == "" {
		return RespostaJudge0{}, fmt.Errorf("judge0 desligado: URL vazia")
	}
	body, err := json.Marshal(item)
	if err != nil {
		return RespostaJudge0{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	out, err := cli.postSubmissao(callCtx, body)
	if err != nil {
		return RespostaJudge0{}, err
	}
	if out.StatusID() > 2 {
		return out, nil
	}
	if out.Token == "" {
		return RespostaJudge0{}, fmt.Errorf("judge0 sync: resposta sem token e status=%d", out.StatusID())
	}
	for {
		select {
		case <-callCtx.Done():
			return out, callCtx.Err()
		case <-time.After(150 * time.Millisecond):
		}
		res, err := cli.getSubmissao(callCtx, out.Token)
		if err != nil {
			return out, err
		}
		out = res
		if out.StatusID() > 2 {
			return out, nil
		}
	}
}

func (cli *Client) postSubmissao(ctx context.Context, body []byte) (RespostaJudge0, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		cli.base+"/submissions?base64_encoded=false", bytes.NewReader(body),
	)
	if err != nil {
		return RespostaJudge0{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cli.token != "" {
		req.Header.Set("X-Auth-Token", cli.token)
	}
	return cli.executarReq(req)
}

func (cli *Client) getSubmissao(ctx context.Context, token string) (RespostaJudge0, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		cli.base+"/submissions/"+token+"?base64_encoded=false", nil,
	)
	if err != nil {
		return RespostaJudge0{}, err
	}
	if cli.token != "" {
		req.Header.Set("X-Auth-Token", cli.token)
	}
	return cli.executarReq(req)
}

func (cli *Client) executarReq(req *http.Request) (RespostaJudge0, error) {
	resp, err := cli.httpDo.Do(req)
	if err != nil {
		return RespostaJudge0{}, err
	}
	defer resp.Body.Close()
	dados, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return RespostaJudge0{}, err
	}
	if resp.StatusCode/100 != 2 {
		return RespostaJudge0{}, fmt.Errorf("judge0 sync: status %d: %s", resp.StatusCode, dados)
	}
	var out RespostaJudge0
	if err := json.Unmarshal(dados, &out); err != nil {
		return RespostaJudge0{}, err
	}
	return out, nil
}

// EnviarBatchSincrono envia todos os itens em lote (POST /submissions/batch,
// fatiado em até MaxBatchSize=20) e faz polling em lote (GET /submissions/batch?tokens=...)
// até todos concluírem, sem prender threads do servidor Judge0 com wait=true.
func (cli *Client) EnviarBatchSincrono(
	ctx context.Context, itens []SubmissaoJudge0,
) ([]RespostaJudge0, error) {
	if len(itens) == 0 {
		return nil, nil
	}
	if cli.base == "" {
		return nil, fmt.Errorf("judge0 desligado: URL vazia")
	}
	totalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out := make([]RespostaJudge0, len(itens))
	var pendentesIdx []int
	var pendentes []SubmissaoJudge0
	for i, it := range itens {
		if strings.TrimSpace(it.SourceCode) == "" {
			out[i] = RespostaJudge0{Status: RawStatus(StatusWrongAnswerID)}
			continue
		}
		pendentesIdx = append(pendentesIdx, i)
		pendentes = append(pendentes, it)
	}

	for inicio := 0; inicio < len(pendentes); inicio += MaxBatchSize {
		fim := inicio + MaxBatchSize
		if fim > len(pendentes) {
			fim = len(pendentes)
		}
		fatia := pendentes[inicio:fim]
		fatiaIdx := pendentesIdx[inicio:fim]

		envelope := struct {
			Submissions []SubmissaoJudge0 `json:"submissions"`
		}{Submissions: fatia}
		body, err := json.Marshal(envelope)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(
			totalCtx, http.MethodPost, cli.base+"/submissions/batch?base64_encoded=false", bytes.NewReader(body),
		)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if cli.token != "" {
			req.Header.Set("X-Auth-Token", cli.token)
		}
		dados, err := cli.executarReqRaw(req)
		if err != nil {
			return nil, err
		}
		var postResps []RespostaJudge0
		if err := json.Unmarshal(dados, &postResps); err != nil {
			return nil, err
		}
		if len(postResps) != len(fatia) {
			return nil, fmt.Errorf("judge0 batch: esperado %d itens, recebido %d", len(fatia), len(postResps))
		}

		todosProntos := true
		tokens := make([]string, len(fatia))
		for k, r := range postResps {
			out[fatiaIdx[k]] = r
			tokens[k] = r.Token
			if r.StatusID() <= 2 {
				todosProntos = false
			}
		}
		if todosProntos {
			continue
		}
		for _, tok := range tokens {
			if tok == "" {
				return nil, fmt.Errorf("judge0 batch: item sem token")
			}
		}
		tokensParam := strings.Join(tokens, ",")
		for {
			select {
			case <-totalCtx.Done():
				return nil, totalCtx.Err()
			case <-time.After(150 * time.Millisecond):
			}
			getReq, err := http.NewRequestWithContext(
				totalCtx, http.MethodGet,
				cli.base+"/submissions/batch?tokens="+tokensParam+"&base64_encoded=false", nil,
			)
			if err != nil {
				return nil, err
			}
			if cli.token != "" {
				getReq.Header.Set("X-Auth-Token", cli.token)
			}
			getDados, err := cli.executarReqRaw(getReq)
			if err != nil {
				return nil, err
			}
			var batchGet struct {
				Submissions []RespostaJudge0 `json:"submissions"`
			}
			if err := json.Unmarshal(getDados, &batchGet); err != nil {
				return nil, err
			}
			if len(batchGet.Submissions) != len(fatia) {
				return nil, fmt.Errorf("judge0 batch get: esperado %d itens, recebido %d", len(fatia), len(batchGet.Submissions))
			}
			prontos := true
			for k, r := range batchGet.Submissions {
				out[fatiaIdx[k]] = r
				if r.StatusID() <= 2 {
					prontos = false
				}
			}
			if prontos {
				break
			}
		}
	}
	return out, nil
}

func (cli *Client) executarReqRaw(req *http.Request) ([]byte, error) {
	resp, err := cli.httpDo.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dados, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("judge0 sync: status %d: %s", resp.StatusCode, dados)
	}
	return dados, nil
}

// Saude ping simples com timeout curto para diagnóstico / verificação prévia.
func (cli *Client) Saude(ctx context.Context) error {
	if cli == nil || cli.base == "" {
		return fmt.Errorf("judge0 desligado")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(
		pingCtx, http.MethodGet, cli.base+"/system_info", nil,
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
