// Saída pública da carga do aluno (spec §5.1): somente conteúdo visível.
//
// Contraste com o envelope de transferência (§2): testes privados JAMAIS
// saem para o aluno. O handler deve usar somente os tipos desta arquivo
// para qualquer resposta de aluno — o teste de vazamento (realizacao_test.go)
// varre o JSON de saída contra marcadores exclusivos dos privados.
package atividades

import "github.com/seno-project/seno/contract"

// AtividadePublica é o corpo da atividade como o aluno pode ver.
type AtividadePublica struct {
	Nome      string          `json:"nome"`
	Enunciado string          `json:"enunciado"`
	Tarefas   []TarefaPublica `json:"tarefas"`
}

// TestePublica é o teste visível ao aluno: `ordem` é a posição do teste na
// lista integral da carga (1-based), igual ao `teste_ordem` dos resultados
// de ensaio — inclusive quando privados entrem no meio.
type TestePublica struct {
	Ordem          int    `json:"ordem"`
	Stdin          string `json:"stdin"`
	StdoutEsperado string `json:"stdout_esperado"`
}

type TarefaPublica struct {
	Ordem         int                           `json:"ordem"`
	Nome          string                        `json:"nome"`
	Enunciado     string                        `json:"enunciado"`
	Tipo          string                        `json:"tipo"`
	ConfigArquivo *contract.ConfigTarefaArquivo `json:"config_arquivo,omitempty"`
	Linguagem     string                        `json:"linguagem"`
	ValorPts      float64                       `json:"valor_pts"`
	Limites       *contract.Limites             `json:"limites"`
	Testes        []TestePublica                `json:"testes"`
}

// Sanitizada converte a carga integral na versão pública: só testes publicos.
func Sanitizada(c contract.Carga) AtividadePublica {
	out := AtividadePublica{
		Nome:      c.Atividade.Nome,
		Enunciado: c.Atividade.Enunciado,
		Tarefas:   make([]TarefaPublica, 0, len(c.Tarefas)),
	}
	for _, t := range c.Tarefas {
		tipo := contract.NormalizarTipoTarefa(t.Tipo)
		var cfgArquivo *contract.ConfigTarefaArquivo
		if tipo == contract.TipoTarefaArquivo {
			var cfgIn contract.ConfigTarefaArquivo
			if t.ConfigArquivo != nil {
				cfgIn = *t.ConfigArquivo
			}
			if norm, err := contract.NormalizarConfigArquivo(cfgIn, contract.DefaultArquivoMaxBytes); err == nil {
				cfgArquivo = &norm
			}
		}
		p := TarefaPublica{
			Ordem:         t.Ordem,
			Nome:          t.Nome,
			Enunciado:     t.Enunciado,
			Tipo:          tipo,
			ConfigArquivo: cfgArquivo,
			Linguagem:     t.Linguagem,
			ValorPts:      t.ValorPts,
			Limites:       t.Limites,
		}
		for i, te := range t.Testes {
			if te.Publico {
				p.Testes = append(p.Testes, TestePublica{
					Ordem: i + 1, Stdin: te.Stdin, StdoutEsperado: te.StdoutEsperado,
				})
			}
		}
		out.Tarefas = append(out.Tarefas, p)
	}
	return out
}
