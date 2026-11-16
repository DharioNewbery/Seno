// Package tarefas — banco de tarefas do professor (PROJETO §Tarefas,
// §Teste). A linguagem não pertence à tarefa: é definida na Atividade,
// por tarefa. Escopo §5.4: professor opera sobre as PRÓPRIAS tarefas
// (404 fora do escopo); admin/super enxergam tudo.
package tarefas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seno-project/seno/api/db/gen"
	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/store"
	"github.com/seno-project/seno/contract"
)

// Service coordena o banco de tarefas.
type Service struct {
	store *store.Store
	audit *platform.Audit
}

func New(st *store.Store, audit *platform.Audit) *Service {
	return &Service{store: st, audit: audit}
}

// Tipos de evento do Log (PROJETO §Log).
const (
	LogKindTarefaCreate   = "tarefa.create"
	LogKindTarefaUpdate   = "tarefa.update"
	LogKindTarefaDelete   = "tarefa.delete"
	LogKindTarefaDuplicar = "tarefa.duplicar"
)

// Tarefa é a visão completa de uma tarefa do banco do professor.
type Tarefa struct {
	ID            int64                         `json:"id"`
	ProfessorID   int64                         `json:"professor_id"`
	Nome          string                        `json:"nome"`
	Enunciado     string                        `json:"enunciado"`
	Tipo          string                        `json:"tipo"`
	Config        json.RawMessage               `json:"config,omitempty"`
	ConfigArquivo *contract.ConfigTarefaArquivo `json:"config_arquivo,omitempty"`
	TempoCPUMs    int32                         `json:"tempo_cpu_ms,omitempty"`   // 0 = padrão global
	TempoTotalMs  int32                         `json:"tempo_total_ms,omitempty"` // 0 = padrão global
	MemoriaMB     int32                         `json:"memoria_mb,omitempty"`     // 0 = padrão global
	DeletedAt     *time.Time                    `json:"deleted_at,omitempty"`
	CreatedAt     time.Time                     `json:"created_at"`
}

// Teste é um caso de teste (stdin → stdout esperado; comparação exata,
// nos termos do Judge0 — documentar na tela de criação).
type Teste struct {
	ID             int64     `json:"id"`
	TarefaID       int64     `json:"tarefa_id"`
	Stdin          string    `json:"stdin"`
	StdoutEsperado string    `json:"stdout_esperado"`
	Publico        bool      `json:"publico"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
}

// CadastroTeste é o corpo de um teste.
type CadastroTeste struct {
	Stdin          string `json:"stdin"`
	StdoutEsperado string `json:"stdout_esperado"`
	Publico        bool   `json:"publico"`
}

// CadastroTarefa é o corpo da criação/edição. Em PATCH, testes não-nulos
// substituem a lista inteira; nil preserva os testes existentes.
type CadastroTarefa struct {
	Nome          string                        `json:"nome"`
	Enunciado     string                        `json:"enunciado"`
	Tipo          string                        `json:"tipo,omitempty"`
	Config        json.RawMessage               `json:"config,omitempty"`
	ConfigArquivo *contract.ConfigTarefaArquivo `json:"config_arquivo,omitempty"`
	TempoCPUMs    *int32                        `json:"tempo_cpu_ms,omitempty"`
	TempoTotalMs  *int32                        `json:"tempo_total_ms,omitempty"`
	MemoriaMB     *int32                        `json:"memoria_mb,omitempty"`
	Testes        []CadastroTeste               `json:"testes,omitempty"`
}

// FiltroTarefas parametriza a listagem (professor é forçado ao próprio id).
type FiltroTarefas struct {
	ProfessorID *int64
	Pagina      *int
	Por         *int
}

// ListaTarefas é o resultado paginado do banco de tarefas.
type ListaTarefas struct {
	Total   int64    `json:"total"`
	Pagina  int      `json:"pagina"`
	Por     int      `json:"por"`
	Tarefas []Tarefa `json:"tarefas"`
}

// Limites plausíveis (§Limites de execução; 0 = usa o padrão global).
const (
	maxTempoMs = 600_000 // 10 min
	maxMemoria = 4096    // 4 GB
)

func limiteMs(v *int32) (int32, error) {
	if v == nil {
		return 0, nil
	}
	if *v < 0 || *v > maxTempoMs {
		return 0, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Tempo de execução inválido (0 = padrão; até 600000 ms).")
	}
	return *v, nil
}

func limiteMemoria(v *int32) (int32, error) {
	if v == nil {
		return 0, nil
	}
	if *v < 0 || *v > maxMemoria {
		return 0, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Memória inválida (0 = padrão; até 4096 MB).")
	}
	return *v, nil
}

func validaNomeEnunciado(cad CadastroTarefa) (string, string, error) {
	nome := strings.TrimSpace(cad.Nome)
	enunciado := strings.TrimSpace(cad.Enunciado)
	if nome == "" || enunciado == "" {
		return "", "", platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Informe nome e enunciado da tarefa.")
	}
	if len(nome) > 128 {
		return "", "", platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Nome muito longo (máx. 128 caracteres).")
	}
	return nome, enunciado, nil
}

// resultadoValidacaoTipo encapsula os campos normalizados pelo validador
// específico de cada tipo de tarefa. Novos tipos de tarefa devem registrar
// sua estratégia em validadoresTipoTarefa.
type resultadoValidacaoTipo struct {
	Tipo         string
	ConfigBytes  []byte
	TempoCPUMs   int32
	TempoTotalMs int32
	MemoriaMB    int32
}

type validadorTipoFunc func(cad CadastroTarefa) (resultadoValidacaoTipo, error)

var validadoresTipoTarefa = map[string]validadorTipoFunc{
	contract.TipoTarefaProgramacao: validarTarefaProgramacao,
	contract.TipoTarefaArquivo:     validarTarefaArquivo,
}

func validarPorTipo(cad CadastroTarefa, tipoPadrao string) (resultadoValidacaoTipo, error) {
	rawTipo := strings.TrimSpace(cad.Tipo)
	if rawTipo == "" {
		if tipoPadrao != "" {
			rawTipo = tipoPadrao
		} else {
			rawTipo = contract.TipoTarefaProgramacao
		}
	}
	if !contract.TipoTarefaValido(rawTipo) {
		return resultadoValidacaoTipo{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Tipo de tarefa inválido (use 'programacao' ou 'arquivo_submissao').")
	}
	tipo := contract.NormalizarTipoTarefa(rawTipo)
	fn, ok := validadoresTipoTarefa[tipo]
	if !ok {
		return resultadoValidacaoTipo{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Tipo de tarefa não suportado.")
	}
	return fn(cad)
}

func validarTarefaProgramacao(cad CadastroTarefa) (resultadoValidacaoTipo, error) {
	cpu, err := limiteMs(cad.TempoCPUMs)
	if err != nil {
		return resultadoValidacaoTipo{}, err
	}
	total, err := limiteMs(cad.TempoTotalMs)
	if err != nil {
		return resultadoValidacaoTipo{}, err
	}
	memoria, err := limiteMemoria(cad.MemoriaMB)
	if err != nil {
		return resultadoValidacaoTipo{}, err
	}
	return resultadoValidacaoTipo{
		Tipo:         contract.TipoTarefaProgramacao,
		ConfigBytes:  []byte("{}"),
		TempoCPUMs:   cpu,
		TempoTotalMs: total,
		MemoriaMB:    memoria,
	}, nil
}

func validarTarefaArquivo(cad CadastroTarefa) (resultadoValidacaoTipo, error) {
	if len(cad.Testes) > 0 {
		return resultadoValidacaoTipo{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Tarefas de submissão de arquivo não aceitam casos de teste de entrada/saída.")
	}
	var cfgIn contract.ConfigTarefaArquivo
	if cad.ConfigArquivo != nil {
		cfgIn = *cad.ConfigArquivo
	} else if len(cad.Config) > 0 && string(cad.Config) != "null" && string(cad.Config) != "{}" {
		if err := json.Unmarshal(cad.Config, &cfgIn); err != nil {
			return resultadoValidacaoTipo{}, platform.NewAPIError(
				http.StatusUnprocessableEntity, platform.CodeUnprocessable,
				"Configuração da tarefa de arquivo inválida.")
		}
	}
	norm, err := contract.NormalizarConfigArquivo(cfgIn, contract.DefaultArquivoMaxBytes)
	if err != nil {
		return resultadoValidacaoTipo{}, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			err.Error())
	}
	rawCfg, err := json.Marshal(norm)
	if err != nil {
		return resultadoValidacaoTipo{}, err
	}
	return resultadoValidacaoTipo{
		Tipo:         contract.TipoTarefaArquivo,
		ConfigBytes:  rawCfg,
		TempoCPUMs:   0,
		TempoTotalMs: 0,
		MemoriaMB:    0,
	}, nil
}

func int8Ptr(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: true} }

func horaDe(ts pgtype.Timestamptz) time.Time { return ts.Time }

func horaPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func tarefaDe(row gen.InsertTarefaRow) Tarefa {
	tipo := contract.NormalizarTipoTarefa(row.Tipo)
	t := Tarefa{
		ID: row.ID, ProfessorID: row.ProfessorID,
		Nome: row.Nome, Enunciado: row.Enunciado,
		Tipo:         tipo,
		TempoCPUMs:   row.TempoCpuMs,
		TempoTotalMs: row.TempoTotalMs,
		MemoriaMB:    row.MemoriaMb,
		DeletedAt:    horaPtr(row.DeletedAt),
		CreatedAt:    horaDe(row.CreatedAt),
	}
	if tipo == contract.TipoTarefaArquivo {
		var cfg contract.ConfigTarefaArquivo
		if len(row.Config) > 0 {
			_ = json.Unmarshal(row.Config, &cfg)
		}
		if norm, err := contract.NormalizarConfigArquivo(cfg, contract.DefaultArquivoMaxBytes); err == nil {
			t.ConfigArquivo = &norm
			if raw, err := json.Marshal(norm); err == nil {
				t.Config = raw
			}
		}
	} else if len(row.Config) > 0 && string(row.Config) != "{}" && string(row.Config) != "null" {
		t.Config = json.RawMessage(row.Config)
	}
	return t
}

// Criar registra a tarefa (com testes opcionais); dono = ator; cargo
// professor obrigatório.
func (s *Service) Criar(ctx context.Context, actor domain.User, cad CadastroTarefa) (Tarefa, error) {
	if !domain.HasAnyRole(actor.Roles, domain.RoleProfessor) {
		return Tarefa{}, platform.NewAPIError(
			http.StatusForbidden, platform.CodeForbidden,
			"Apenas professores criam tarefas.")
	}
	nome, enunciado, err := validaNomeEnunciado(cad)
	if err != nil {
		return Tarefa{}, err
	}
	val, err := validarPorTipo(cad, contract.TipoTarefaProgramacao)
	if err != nil {
		return Tarefa{}, err
	}

	var nova Tarefa
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		row, err := q.InsertTarefa(ctx, gen.InsertTarefaParams{
			ProfessorID:  actor.ID,
			Nome:         nome,
			Enunciado:    enunciado,
			Tipo:         val.Tipo,
			Config:       val.ConfigBytes,
			TempoCpuMs:   val.TempoCPUMs,
			TempoTotalMs: val.TempoTotalMs,
			MemoriaMb:    val.MemoriaMB,
			CreatedBy:    int8Ptr(actor.ID),
		})
		if err != nil {
			return err
		}
		for _, te := range cad.Testes {
			if _, err := q.InsertTesteTarefa(ctx, gen.InsertTesteTarefaParams{
				TarefaID:       row.ID,
				Stdin:          te.Stdin,
				StdoutEsperado: te.StdoutEsperado,
				Publico:        te.Publico,
				CreatedBy:      int8Ptr(actor.ID),
			}); err != nil {
				return err
			}
		}
		nova = tarefaDe(row)
		return nil
	})
	if err != nil {
		return Tarefa{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTarefaCreate,
		Detail: fmt.Sprintf("tarefa=%d tipo=%s testes=%d", nova.ID, nova.Tipo, len(cad.Testes)),
	})
	return nova, nil
}

func tarefaNaoEncontrada() error {
	return platform.NewAPIError(
		http.StatusNotFound, platform.CodeNotFound, "Tarefa não encontrada.")
}

// escopoTarefa aplica §5.4: staff passa; professor não-dono → 404.
func escopoTarefa(actor domain.User, professorID int64) error {
	if actor.IsStaff() {
		return nil
	}
	if actor.ID != professorID {
		return tarefaNaoEncontrada()
	}
	return nil
}

// Ver devolve a tarefa com os testes (ordem por id). Tarefa excluída
// (soft, quando as Atividades existirem) só se vê onde é referenciada.
func (s *Service) Ver(ctx context.Context, actor domain.User, id int64) (Tarefa, []Teste, error) {
	row, err := s.store.Q.GetTarefa(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tarefa{}, nil, tarefaNaoEncontrada()
		}
		return Tarefa{}, nil, err
	}
	if err := escopoTarefa(actor, row.ProfessorID); err != nil {
		return Tarefa{}, nil, err
	}
	rows, err := s.store.Q.ListTestesTarefa(ctx, id)
	if err != nil {
		return Tarefa{}, nil, err
	}
	testes := make([]Teste, 0, len(rows))
	for _, te := range rows {
		testes = append(testes, Teste{
			ID: te.ID, TarefaID: te.TarefaID,
			Stdin: te.Stdin, StdoutEsperado: te.StdoutEsperado,
			Publico: te.Publico, CreatedAt: horaDe(te.CreatedAt),
		})
	}
	return tarefaDe(gen.InsertTarefaRow{
		ID: row.ID, ProfessorID: row.ProfessorID,
		Nome: row.Nome, Enunciado: row.Enunciado,
		Tipo: row.Tipo, Config: row.Config,
		TempoCpuMs: row.TempoCpuMs, TempoTotalMs: row.TempoTotalMs,
		MemoriaMb: row.MemoriaMb,
		DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt,
	}), testes, nil
}

// Listar: professor recebe apenas as próprias (filtro forçado); staff tudo.
func (s *Service) Listar(ctx context.Context, actor domain.User, f FiltroTarefas) (ListaTarefas, error) {
	prof := f.ProfessorID
	if !actor.IsStaff() {
		pro := actor.ID
		prof = &pro
	}
	pagina, por := 1, 50
	if f.Pagina != nil && *f.Pagina > 1 {
		pagina = *f.Pagina
	}
	if f.Por != nil && *f.Por > 0 {
		por = *f.Por
	}
	if por > 200 {
		por = 200
	}

	total, err := s.store.Q.CountTarefas(ctx, profInt(prof))
	if err != nil {
		return ListaTarefas{}, err
	}
	rows, err := s.store.Q.ListTarefas(ctx, gen.ListTarefasParams{
		Professor: profInt(prof),
		Limit:     int32(por),
		Offset:    int32((pagina - 1) * por),
	})
	if err != nil {
		return ListaTarefas{}, err
	}
	out := make([]Tarefa, 0, len(rows))
	for _, row := range rows {
		out = append(out, tarefaDe(gen.InsertTarefaRow{
			ID: row.ID, ProfessorID: row.ProfessorID,
			Nome: row.Nome, Enunciado: row.Enunciado,
			Tipo: row.Tipo, Config: row.Config,
			TempoCpuMs: row.TempoCpuMs, TempoTotalMs: row.TempoTotalMs,
			MemoriaMb: row.MemoriaMb,
			DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt,
		}))
	}
	return ListaTarefas{Total: total, Pagina: pagina, Por: por, Tarefas: out}, nil
}

// emUso: quantas atividades ativas referenciam a tarefa (regras §Ciclo:
// usada → não edita e exclusão vira soft delete).
func (s *Service) emUso(ctx context.Context, id int64) (bool, error) {
	n, err := s.store.Q.CountAtividadesUsandoTarefa(ctx, id)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func tarefaEmUso() error {
	return platform.NewAPIError(
		http.StatusConflict, platform.CodeConflict,
		"Tarefa em uso por atividade — duplique para editar.")
}

// Editar muda nome/enunciado/limites e (opcional) substitui os testes.
// Tarefa usada por atividade → 409 (duplicar para obter editável).
func (s *Service) Editar(ctx context.Context, actor domain.User, id int64, cad CadastroTarefa) (Tarefa, []Teste, error) {
	atual, testes, err := s.Ver(ctx, actor, id)
	if err != nil {
		return Tarefa{}, nil, err
	}
	usada, err := s.emUso(ctx, id)
	if err != nil {
		return Tarefa{}, nil, err
	}
	if usada {
		return Tarefa{}, nil, tarefaEmUso()
	}
	nome, enunciado, err := validaNomeEnunciado(cad)
	if err != nil {
		return Tarefa{}, nil, err
	}
	// Se estiver mantendo tipo arquivo_submissao e não enviou config no PATCH, preserva a atual.
	if (cad.Tipo == "" || cad.Tipo == contract.TipoTarefaArquivo) &&
		atual.Tipo == contract.TipoTarefaArquivo &&
		cad.ConfigArquivo == nil && len(cad.Config) == 0 && atual.ConfigArquivo != nil {
		cad.ConfigArquivo = atual.ConfigArquivo
	}
	val, err := validarPorTipo(cad, atual.Tipo)
	if err != nil {
		return Tarefa{}, nil, err
	}
	if _, err := s.store.Q.UpdateTarefa(ctx, gen.UpdateTarefaParams{
		ID: id, Nome: nome, Enunciado: enunciado,
		Tipo: val.Tipo, Config: val.ConfigBytes,
		TempoCpuMs: val.TempoCPUMs, TempoTotalMs: val.TempoTotalMs, MemoriaMb: val.MemoriaMB,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tarefa{}, nil, tarefaNaoEncontrada()
		}
		return Tarefa{}, nil, err
	}
	if val.Tipo == contract.TipoTarefaArquivo {
		// Garante que tarefas de arquivo não retenham testes antigos.
		if _, err := s.store.Q.ReplaceTestesTarefa(ctx, id); err != nil {
			return Tarefa{}, nil, err
		}
	} else if cad.Testes != nil {
		if _, err := s.store.Q.ReplaceTestesTarefa(ctx, id); err != nil {
			return Tarefa{}, nil, err
		}
		for _, te := range cad.Testes {
			if _, err := s.store.Q.InsertTesteTarefa(ctx, gen.InsertTesteTarefaParams{
				TarefaID:       id,
				Stdin:          te.Stdin,
				StdoutEsperado: te.StdoutEsperado,
				Publico:        te.Publico,
				CreatedBy:      int8Ptr(actor.ID),
			}); err != nil {
				return Tarefa{}, nil, err
			}
		}
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTarefaUpdate,
		Detail: fmt.Sprintf("tarefa=%d tipo=%s testes=%d", id, val.Tipo, lenOrKeep(cad.Testes, testes)),
	})
	return s.Ver(ctx, actor, id)
}

// Duplicar copia a tarefa e os testes (cópia editável, PROJETO §Ciclo).
func (s *Service) Duplicar(ctx context.Context, actor domain.User, id int64) (Tarefa, error) {
	origem, _, err := s.Ver(ctx, actor, id)
	if err != nil {
		return Tarefa{}, err
	}
	novoNome := origem.Nome + " (cópia)"
	if len(novoNome) > 128 {
		novoNome = origem.Nome[:128-len(" (cópia)")] + " (cópia)"
	}
	var copia Tarefa
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *gen.Queries) error {
		row, err := q.CopyTarefa(ctx, gen.CopyTarefaParams{
			ID: id, Nome: novoNome, CreatedBy: int8Ptr(actor.ID),
		})
		if err != nil {
			return err
		}
		if _, err := q.CopyTestesTarefa(ctx, gen.CopyTestesTarefaParams{
			TarefaID:   id,
			TarefaID_2: row.ID,
			CreatedBy:  int8Ptr(actor.ID),
		}); err != nil {
			return err
		}
		copia = tarefaDe(gen.InsertTarefaRow{
			ID: row.ID, ProfessorID: row.ProfessorID,
			Nome: row.Nome, Enunciado: row.Enunciado,
			Tipo: row.Tipo, Config: row.Config,
			TempoCpuMs: row.TempoCpuMs, TempoTotalMs: row.TempoTotalMs,
			MemoriaMb: row.MemoriaMb,
			DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt,
		})
		return nil
	})
	if err != nil {
		return Tarefa{}, err
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTarefaDuplicar,
		Detail: fmt.Sprintf("tarefa=%d → %d", id, copia.ID),
	})
	return copia, nil
}

// Excluir segue §Ciclo de Tarefa e Atividade: sem referência de atividade
// → hard delete (testes caem junto, CASCADE); usada → soft delete e
// passa a ficar somente leitura nas atividades/submissões antigas.
func (s *Service) Excluir(ctx context.Context, actor domain.User, id int64) error {
	if _, _, err := s.Ver(ctx, actor, id); err != nil {
		return err
	}
	usada, err := s.emUso(ctx, id)
	if err != nil {
		return err
	}
	if usada {
		rows, err := s.store.Q.SoftDeleteTarefa(ctx, id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return tarefaNaoEncontrada()
		}
		s.audit.Record(ctx, platform.LogEntry{
			ActorID: &actor.ID, Actor: actor.Email,
			Kind:   LogKindTarefaDelete,
			Detail: fmt.Sprintf("tarefa=%d (soft: usada por atividade)", id),
		})
		return nil
	}
	rows, err := s.store.Q.DeleteTarefa(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return tarefaNaoEncontrada()
	}
	s.audit.Record(ctx, platform.LogEntry{
		ActorID: &actor.ID, Actor: actor.Email,
		Kind:   LogKindTarefaDelete,
		Detail: fmt.Sprintf("tarefa=%d", id),
	})
	return nil
}

// helpers -----------------------------------------------------------

func ptr(p *int64) *int64 { return p }

func profInt(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}

func lenOrKeep(testes []CadastroTeste, atuais []Teste) int {
	if testes != nil {
		return len(testes)
	}
	return len(atuais)
}
