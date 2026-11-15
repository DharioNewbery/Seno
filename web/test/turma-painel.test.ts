import { describe, expect, it } from "vitest";
import { parseCsvAlunos, resumirAtividade } from "../src/lib/turma-painel";

describe("turma-painel helpers", () => {
  it("parseCsvAlunos interpreta CSV com cabeçalho, vírgula, ponto-e-vírgula e remove duplicatas", () => {
    const csv = `
email,nome,sobrenome
carla@ufpa.br,Carla,Souza
"joao@ufpa.br";"João";"Silva Santos"
CARLA@ufpa.br,Carla,Duplicada
maria@ufpa.br,Maria Oliveira
ana@ufpa.br
    `;
    const lidos = parseCsvAlunos(csv);
    expect(lidos).toEqual([
      { email: "carla@ufpa.br", first_name: "Carla", last_name: "Souza" },
      {
        email: "joao@ufpa.br",
        first_name: "João",
        last_name: "Silva Santos",
      },
      { email: "maria@ufpa.br", first_name: "Maria", last_name: "Oliveira" },
      { email: "ana@ufpa.br", first_name: "", last_name: "" },
    ]);
  });

  it("resumirAtividade calcula total de tarefas, pontos e etiquetas de linguagem/arquivo", () => {
    const resumo = resumirAtividade({
      id: 7,
      nome: "Prova 1",
      tarefas: [
        {
          tarefa_id: 1,
          ordem: 1,
          valor_pts: 40,
          linguagem: "python",
          tarefa_nome: "Soma",
          tarefa_enunciado: "Calcule a soma.",
          tarefa_tipo: "programacao",
        },
        {
          tarefa_id: 2,
          ordem: 2,
          valor_pts: 60,
          linguagem: "",
          tarefa_nome: "Relatório",
          tarefa_enunciado: "Envie o PDF.",
          tarefa_tipo: "arquivo_submissao",
        },
      ],
    });
    expect(resumo.totalTarefas).toBe(2);
    expect(resumo.totalPontos).toBe(100);
    expect(resumo.etiquetas).toEqual(["Python", "Arquivo"]);
  });
});
