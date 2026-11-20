import { describe, expect, it } from "vitest";
import {
  chaveDia,
  estadoNaAgenda,
  horaDoPrazo,
  montarAgenda,
  resumirPainelAluno,
} from "#lib/agenda-aluno";
import { atribuicoesMock, turmasMock } from "#lib/agenda-aluno-mock";
import type { AtribuicaoDoAluno } from "#lib/server/ensino";

function criarAtribuicao(
  parcial: Partial<AtribuicaoDoAluno> = {},
): AtribuicaoDoAluno {
  return {
    id: 1,
    turma_id: 1,
    atividade_id: 1,
    autocomplete: false,
    prazo: "2026-11-20T18:00:00-03:00",
    pode_atrasado: false,
    materia_codigo: "PRG101",
    turma_ativa: true,
    atividade_nome: "Atividade 1",
    em_andamento: false,
    entregue: false,
    atrasada: false,
    ...parcial,
  };
}

describe("Agenda e painel inicial do aluno (/aluno)", () => {
  const agora = new Date("2026-11-20T14:00:00-03:00");

  it("formata chaveDia e horaDoPrazo no fuso de Belém", () => {
    expect(chaveDia("2026-11-20T23:30:00-03:00")).toBe("2026-11-20");
    expect(horaDoPrazo("2026-11-20T09:05:00-03:00")).toBe("09:05");
  });

  it("classifica os 4 estados de uma atividade conforme prazo e entrega", () => {
    expect(
      estadoNaAgenda(
        criarAtribuicao({ prazo: "2026-11-21T10:00:00-03:00" }),
        agora,
      ),
    ).toBe("pendente");

    expect(
      estadoNaAgenda(
        criarAtribuicao({ prazo: "2026-11-20T18:00:00-03:00" }),
        agora,
      ),
    ).toBe("pendente");

    expect(
      estadoNaAgenda(
        criarAtribuicao({
          prazo: "2026-11-19T18:00:00-03:00",
          pode_atrasado: false,
        }),
        agora,
      ),
    ).toBe("perdida");

    expect(
      estadoNaAgenda(
        criarAtribuicao({
          prazo: "2026-11-19T18:00:00-03:00",
          pode_atrasado: true,
        }),
        agora,
      ),
    ).toBe("pendente");

    expect(
      estadoNaAgenda(
        criarAtribuicao({ entregue: true, atrasada: false }),
        agora,
      ),
    ).toBe("no_prazo");

    expect(
      estadoNaAgenda(
        criarAtribuicao({ entregue: true, atrasada: true }),
        agora,
      ),
    ).toBe("atrasada");
  });

  it("agrupa por data de entrega, filtra turmas inativas e define o tom da coluna", () => {
    const lista: AtribuicaoDoAluno[] = [
      // Passado sem pendências entregáveis -> "passado"
      criarAtribuicao({
        id: 1,
        prazo: "2026-11-18T10:00:00-03:00",
        entregue: true,
      }),
      criarAtribuicao({
        id: 2,
        prazo: "2026-11-18T15:00:00-03:00",
        entregue: false,
        pode_atrasado: false,
      }),
      // Passado com atividade entregável atrasada -> "passado-aberto"
      criarAtribuicao({
        id: 3,
        prazo: "2026-11-19T12:00:00-03:00",
        entregue: false,
        pode_atrasado: true,
      }),
      // Hoje -> "hoje"
      criarAtribuicao({
        id: 4,
        prazo: "2026-11-20T20:00:00-03:00",
        entregue: false,
      }),
      // Futuro -> "futuro"
      criarAtribuicao({
        id: 5,
        prazo: "2026-11-22T08:00:00-03:00",
        entregue: false,
      }),
      // Turma inativa -> ignorada
      criarAtribuicao({
        id: 6,
        prazo: "2026-11-20T16:00:00-03:00",
        turma_ativa: false,
      }),
    ];

    const colunas = montarAgenda(lista, agora);
    expect(colunas.map((c) => ({ dia: c.dia, tom: c.tom, entregues: c.entregues, total: c.total }))).toEqual([
      { dia: "2026-11-18", tom: "passado", entregues: 1, total: 2 },
      { dia: "2026-11-19", tom: "passado-aberto", entregues: 0, total: 1 },
      { dia: "2026-11-20", tom: "hoje", entregues: 0, total: 1 },
      { dia: "2026-11-22", tom: "futuro", entregues: 0, total: 1 },
    ]);
  });

  it("resume métricas, atividades em andamento e correções recentes do painel", () => {
    const mockAtrib = atribuicoesMock(agora);
    const mockTurmas = turmasMock();
    const resumo = resumirPainelAluno(mockAtrib, mockTurmas, agora);

    expect(resumo.turmasAtivas).toBe(4);
    expect(resumo.paraHoje).toBeGreaterThan(0);
    expect(resumo.atrasadasEntregaveis).toBeGreaterThan(0);
    expect(resumo.emAndamento.length).toBeGreaterThan(0);
    expect(resumo.corrigidasRecentes.length).toBeGreaterThan(0);
  });
});
