package contract

import (
	"encoding/json"
	"time"
)

// Transferencia é o envelope integral da publicação de atividade da API para
// o Monitor (spec §2, POST /interna/v1/atividades). Congelado: ambos os
// serviços compilam contra este struct; campo novo = mudança de contrato.
type Transferencia struct {
	APIID     int64     `json:"api_id"`
	MonitorID string    `json:"monitor_id"` // vazio na primeira transferência (gerado pelo Monitor)
	GeradaEm  time.Time `json:"gerada_em"`
	Turma     Turma     `json:"turma"`

	// Matrículas ativas copiadas no instante da publicação (snapshot).
	Alunos []Aluno `json:"alunos"`

	Janela Janela `json:"janela"`
	Regras Regras `json:"regras"`
	Carga  Carga  `json:"carga"`
}

type Turma struct {
	APIID int64  `json:"api_id"`
	Nome  string `json:"nome"`
}

type Aluno struct {
	APIID int64  `json:"api_id"`
	Nome  string `json:"nome"`
}

type Janela struct {
	Inicio       time.Time `json:"inicio"`
	Prazo        time.Time `json:"prazo"`
	DuracaoSeg   *int      `json:"duracao_seg"` // null = sem limite por aluno
	PodeAtrasado bool      `json:"pode_atrasado"`
	LimiteAtraso *string   `json:"limite_atraso"` // ISO-8601 (ex. "P1D"); null = até encerramento da turma
}

type Regras struct {
	SubmissoesMultiplas  bool     `json:"submissoes_multiplas"` // false = 1 submissão, depois bloqueia
	MaxSubmissoes        int      `json:"max_submissoes"`       // 0 = ilimitado
	LinguagensPermitidas []string `json:"linguagens_permitidas"`
	Autocomplete         bool     `json:"autocomplete"`
}

type Carga struct {
	Atividade AtividadeCarga `json:"atividade"`
	Tarefas   []TarefaCarga  `json:"tarefas"`
}

type AtividadeCarga struct {
	APIID     int64  `json:"api_id"`
	Nome      string `json:"nome"`
	Enunciado string `json:"enunciado"`
}

type TarefaCarga struct {
	Ordem       int          `json:"ordem"`
	TarefaAPIID int64        `json:"tarefa_api_id"`
	Nome        string       `json:"nome"`
	ValorPts    float64      `json:"valor_pts"`
	Enunciado   string       `json:"enunciado"`
	Linguagem   string       `json:"linguagem"`
	Limites     *Limites     `json:"limites"`
	Testes      []TesteCarga `json:"testes"`
}

type Limites struct {
	TempoCPUMs   int `json:"tempo_cpu_ms"`
	TempoTotalMs int `json:"tempo_total_ms"`
	MemoriaMB    int `json:"memoria_mb"`
}

type TesteCarga struct {
	TesteAPIID     int64  `json:"teste_api_id,omitempty"`
	Stdin          string `json:"stdin"`
	StdoutEsperado string `json:"stdout_esperado"`
	Publico        bool   `json:"publico"`
}

// EstadoAtividade é o estado consolidado da atividade exposto à API
// principal (spec §6, GET /interna/v1/atividades/{monitor_id}/estado).
// Snapshot integral por chamada; quem persiste é a API (job periódico —
// decisão da Fase 7).
type EstadoAtividade struct {
	MonitorID string          `json:"monitor_id"`
	Status    string          `json:"status"` // ativa | encerrada | cancelada
	Agregados AgregadosEstado `json:"agregados"`
	Alunos    []EstadoAluno   `json:"alunos"`
}

// AgregadosEstado do painel do professor (§6).
type AgregadosEstado struct {
	Abertas   int `json:"abertas"`
	Entregues int `json:"entregues"`
	Atrasos   int `json:"atrasos"`
	Total     int `json:"total"`
}

// EstadoAluno do listado do aluno na API (§6). Estado: "pendente"
// (não abriu) | "aberta" | "entregue" (atraso vai no agregado).
type EstadoAluno struct {
	AlunoAPIID      int64      `json:"aluno_api_id"`
	Estado          string     `json:"estado"`
	NT              int        `json:"n_t"`
	UltimaSubmissao *time.Time `json:"ultima_submissao"`
	PrazoEfetivo    *time.Time `json:"prazo_efetivo"`
	NotaAuto        *float64   `json:"nota_auto"`
	CorrecaoStatus  string     `json:"correcao_status"`
}

// PacoteSubmissaoMonitor transfere a submissão definitiva e o resultado da
// correção automática do Monitor para a API (fonte única da verdade após entrega).
type PacoteSubmissaoMonitor struct {
	MonitorID       string               `json:"monitor_id"`
	AtribuicaoAPIID int64                `json:"atribuicao_api_id"`
	AlunoAPIID      int64                `json:"aluno_api_id"`
	Submissao       *SubmissaoMonitor    `json:"submissao,omitempty"`
	Correcao        *CorrecaoMonitor     `json:"correcao,omitempty"`
	Atividade       AtividadeInfoMonitor `json:"atividade"`
}

type SubmissaoMonitor struct {
	ID         int64             `json:"id"`
	AlunoID    int64             `json:"aluno_id"`
	NT         int               `json:"n_t"`
	ComecouEm  time.Time         `json:"comecou_em"`
	EntregueEm time.Time         `json:"entregue_em"`
	Atrasada   bool              `json:"atrasada"`
	Respostas  map[string]string `json:"respostas"` // ordem ("1", "2"...) -> código
}

type CorrecaoMonitor struct {
	Status      string                  `json:"status"` // pendente | executando | concluida | erro
	NotaAuto    *float64                `json:"nota_auto,omitempty"`
	ConcluidaEm *time.Time              `json:"concluida_em,omitempty"`
	Resultados  json.RawMessage         `json:"resultados,omitempty"`
	Tarefas     []TarefaCorrecaoMonitor `json:"tarefas,omitempty"`
}

type TarefaCorrecaoMonitor struct {
	Ordem       int                     `json:"ordem"`
	TarefaAPIID int64                   `json:"tarefa_api_id"`
	Veredito    string                  `json:"veredito"`
	NotaAuto    *float64                `json:"nota_auto,omitempty"`
	Testes      []ResultadoTesteMonitor `json:"testes"`
}

type ResultadoTesteMonitor struct {
	Ordem         int     `json:"ordem"`
	TesteAPIID    int64   `json:"teste_api_id"`
	Status        string  `json:"status"`
	Stdout        string  `json:"stdout,omitempty"`
	Stderr        string  `json:"stderr,omitempty"`
	CompileOutput string  `json:"compile_output,omitempty"`
	Tempo         float64 `json:"tempo"`
	CPU           float64 `json:"cpu"`
	Memoria       int     `json:"memoria"`
	Privado       bool    `json:"privado"`
}

type AtividadeInfoMonitor struct {
	Nome    string              `json:"nome"`
	Tarefas []TarefaInfoMonitor `json:"tarefas"`
}

type TarefaInfoMonitor struct {
	Ordem       int     `json:"ordem"`
	TarefaAPIID int64   `json:"tarefa_api_id"`
	Nome        string  `json:"nome"`
	Enunciado   string  `json:"enunciado"`
	Linguagem   string  `json:"linguagem"`
	ValorPts    float64 `json:"valor_pts"`
}
