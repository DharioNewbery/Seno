package contract

import "time"

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
	Stdin          string `json:"stdin"`
	StdoutEsperado string `json:"stdout_esperado"`
	Publico        bool   `json:"publico"`
}

// EstadoAtividade é o estado consolidado da atividade exposto à API
// principal (spec §6, GET /interna/v1/atividades/{monitor_id}/estado).
type EstadoAtividade struct {
	MonitorID string `json:"monitor_id"`
	Status    string `json:"status"`
	Agregados struct {
		Abertas   int `json:"abertas"`
		Entregues int `json:"entregues"`
		Atrasos   int `json:"atrasos"`
		Total     int `json:"total"`
	} `json:"agregados"`
	Alunos []EstadoAluno `json:"alunos"`
}

type EstadoAluno struct {
	AlunoAPIID      int64      `json:"aluno_api_id"`
	Estado          string     `json:"estado"` // aberta | entregue | atrasada (hierarquia definida na Fase 7)
	NT              int        `json:"n_t"`
	UltimaSubmissao *time.Time `json:"ultima_submissao"`
	PrazoEfetivo    *time.Time `json:"prazo_efetivo"`
	NotaAuto        *float64   `json:"nota_auto"`
	CorrecaoStatus  string     `json:"correcao_status"`
}
