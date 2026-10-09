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

type TarefaPublica struct {
	Ordem     int                   `json:"ordem"`
	Nome      string                `json:"nome"`
	Enunciado string                `json:"enunciado"`
	Linguagem string                `json:"linguagem"`
	ValorPts  float64               `json:"valor_pts"`
	Limites   *contract.Limites     `json:"limites"`
	Testes    []contract.TesteCarga `json:"testes"`
}

// Sanitizada converte a carga integral na versão pública: só testes publicos.
func Sanitizada(c contract.Carga) AtividadePublica {
	out := AtividadePublica{
		Nome:      c.Atividade.Nome,
		Enunciado: c.Atividade.Enunciado,
		Tarefas:   make([]TarefaPublica, 0, len(c.Tarefas)),
	}
	for _, t := range c.Tarefas {
		p := TarefaPublica{
			Ordem:     t.Ordem,
			Nome:      t.Nome,
			Enunciado: t.Enunciado,
			Linguagem: t.Linguagem,
			ValorPts:  t.ValorPts,
			Limites:   t.Limites,
		}
		for _, te := range t.Testes {
			if te.Publico {
				p.Testes = append(p.Testes, te)
			}
		}
		out.Tarefas = append(out.Tarefas, p)
	}
	return out
}
