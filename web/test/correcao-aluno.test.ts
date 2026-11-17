import { describe, it, expect } from "vitest";
import {
  calcularTotaisCorrecao,
  extrairCodigoRespostaAluno,
  resumirEstadoAtribuicaoAluno,
} from "#lib/correcao-aluno";
import type { CorrecaoTarefa, SubmissaoAluno } from "#lib/server/ensino";

describe("Visualização de respostas e notas do aluno (correcao-aluno)", () => {
  it("calcula nota obtida (priorizando nota_final sobre nota_auto) e total de pontos", () => {
    const tarefas: CorrecaoTarefa[] = [
      {
        id: 1,
        tarefa_id: 10,
        tarefa_nome: "Soma",
        valor_pts: 40,
        nota_auto: 40,
        nota_final: 35,
        resultados: [],
      },
      {
        id: 2,
        tarefa_id: 20,
        tarefa_nome: "Fibonacci",
        valor_pts: 60,
        nota_auto: 60,
        resultados: [],
      },
    ];

    expect(calcularTotaisCorrecao(tarefas)).toEqual({
      notaObtida: 95,
      valorTotal: 100,
    });
  });

  it("extrai o código submetido pelo aluno de t.codigo ou do snapshot da entrega", () => {
    const tComCodigo: CorrecaoTarefa = {
      id: 1,
      tarefa_id: 10,
      tarefa_nome: "Q1",
      codigo: "print('direto')",
      valor_pts: 10,
      resultados: [],
    };
    const tSemCodigo: CorrecaoTarefa = {
      id: 2,
      tarefa_id: 20,
      tarefa_nome: "Q2",
      valor_pts: 10,
      resultados: [],
    };
    const entrega: SubmissaoAluno = {
      id: 100,
      atribuicao_id: 5,
      aluno_id: 7,
      comecou_em: "2026-11-15T10:00:00Z",
      entregue_em: "2026-11-15T10:30:00Z",
      atrasada: false,
      snapshot: {
        tasks: [
          { id: 10, text: "print('snap1')" },
          { id: 20, text: "print('snap2')" },
        ],
      },
    };

    expect(extrairCodigoRespostaAluno(tComCodigo, entrega)).toBe(
      "print('direto')",
    );
    expect(extrairCodigoRespostaAluno(tSemCodigo, entrega)).toBe(
      "print('snap2')",
    );
    expect(extrairCodigoRespostaAluno(tSemCodigo, null)).toBe("");
  });

  it("resume o estado da atribuição exibindo 'Corrigida' quando corrigida e publicada", () => {
    expect(
      resumirEstadoAtribuicaoAluno({
        entregue: true,
        atrasada: false,
        publicada_em: "2026-11-15T12:00:00Z",
        nota_final: 85,
        em_andamento: false,
        pode_atrasado: false,
      }),
    ).toEqual({
      rotulo: "Corrigida",
      classe: "nota",
    });

    expect(
      resumirEstadoAtribuicaoAluno({
        entregue: true,
        atrasada: false,
        em_andamento: false,
        pode_atrasado: false,
      }),
    ).toEqual({
      rotulo: "Entregue",
      classe: "entregue",
    });

    expect(
      resumirEstadoAtribuicaoAluno({
        entregue: true,
        atrasada: true,
        em_andamento: false,
        pode_atrasado: true,
      }),
    ).toEqual({
      rotulo: "Entregue atrasada",
      classe: "entregue",
    });

    const ontem = new Date(Date.now() - 86_400_000).toISOString();
    expect(
      resumirEstadoAtribuicaoAluno({
        entregue: false,
        atrasada: false,
        em_andamento: false,
        prazo: ontem,
        pode_atrasado: false,
      }),
    ).toEqual({
      rotulo: "Prazo encerrado",
      classe: "encerrada",
    });

    expect(
      resumirEstadoAtribuicaoAluno({
        entregue: false,
        atrasada: false,
        em_andamento: false,
        prazo: ontem,
        pode_atrasado: true,
      }),
    ).toEqual({
      rotulo: "Não começou",
      classe: "nao",
    });
  });
});
