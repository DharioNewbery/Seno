import { describe, expect, it } from "vitest";
import {
  agruparAtividadesPorData,
  atribuicoesMockAluno,
  filtrarAtividadesAluno,
  gerarDiasCalendario,
  indiceDiaMaisAtual,
  iniciaisUsuario,
  isAtividadeEmProgresso,
  nomeMateriaAtividade,
  paginarAtribuicoesAluno,
  timelineAtividadeAluno,
} from "#lib/atividades-aluno";
import { PORTAIS } from "#lib/portais";
import type { AtribuicaoDoAluno } from "#lib/server/ensino";

function criarAtividadeMock(
  parcial: Partial<AtribuicaoDoAluno> = {},
): AtribuicaoDoAluno {
  return {
    id: 1,
    turma_id: 10,
    atividade_id: 100,
    autocomplete: false,
    pode_atrasado: false,
    materia_codigo: "PROG1",
    materia_nome: "Programação I",
    turma_ativa: true,
    atividade_nome: "Lista 1",
    pontuacao_maxima: 10,
    em_andamento: false,
    entregue: false,
    atrasada: false,
    ...parcial,
  };
}

describe("Portal do Aluno — Atividades", () => {
  it("mantém 'Atividades' como página única na navegação do aluno", () => {
    expect(PORTAIS.aluno.nav).toHaveLength(1);
    const itemAtividades = PORTAIS.aluno.nav[0];
    expect(itemAtividades?.href).toBe("/aluno/atividades");
    expect(itemAtividades?.rotulo).toBe("Atividades");
    expect(itemAtividades?.sub).toBeUndefined();
  });

  it("identifica atividades em progresso (iniciadas e não entregues)", () => {
    expect(
      isAtividadeEmProgresso(
        criarAtividadeMock({ em_andamento: true, entregue: false }),
      ),
    ).toBe(true);
    expect(
      isAtividadeEmProgresso(
        criarAtividadeMock({
          em_andamento: false,
          iniciada_em: "2026-11-01T10:00:00Z",
          entregue: false,
        }),
      ),
    ).toBe(true);
    expect(
      isAtividadeEmProgresso(
        criarAtividadeMock({ em_andamento: true, entregue: true }),
      ),
    ).toBe(false);
    expect(
      isAtividadeEmProgresso(
        criarAtividadeMock({ em_andamento: false, entregue: false }),
      ),
    ).toBe(false);
  });

  it("filtra atividades por mostrarEntregues (false/true) e omite turmas encerradas", () => {
    const naoIniciada = criarAtividadeMock({ id: 1, em_andamento: false, entregue: false });
    const emProgresso = criarAtividadeMock({ id: 2, em_andamento: true, entregue: false });
    const entregue = criarAtividadeMock({ id: 3, em_andamento: false, entregue: true });
    const turmaEncerradaAtual = criarAtividadeMock({
      id: 4,
      em_andamento: false,
      entregue: false,
      turma_ativa: false,
    });
    const turmaEncerradaEntregue = criarAtividadeMock({
      id: 5,
      em_andamento: false,
      entregue: true,
      turma_ativa: false,
    });

    const apenasPendentes = filtrarAtividadesAluno(
      [naoIniciada, emProgresso, entregue, turmaEncerradaAtual, turmaEncerradaEntregue],
      false,
    );
    expect(apenasPendentes.map((a) => a.id)).toEqual([2, 1]);

    const comEntregues = filtrarAtividadesAluno(
      [naoIniciada, emProgresso, entregue, turmaEncerradaAtual, turmaEncerradaEntregue],
      true,
    );
    expect(comEntregues.map((a) => a.id)).toEqual([2, 1, 3]);
  });

  it("gera sigla do usuário corretamente para o avatar", () => {
    expect(iniciaisUsuario("João", "Silva")).toBe("JS");
    expect(iniciaisUsuario("maria", "souza")).toBe("MS");
    expect(iniciaisUsuario("Ana", "")).toBe("A");
    expect(iniciaisUsuario("", "")).toBe("?");
  });

  it("exibe o nome da matéria com fallback para o código", () => {
    expect(
      nomeMateriaAtividade({
        materia_nome: "Algoritmos",
        materia_codigo: "ALG",
      }),
    ).toBe("Algoritmos");
    expect(
      nomeMateriaAtividade({
        materia_nome: "",
        materia_codigo: "ALG",
      }),
    ).toBe("ALG");
  });

  it("gera timeline com 4 etapas (inicio, em progresso, entregue, corrigida)", () => {
    const agora = new Date("2026-11-23T19:00:00Z");

    const aberta = timelineAtividadeAluno(
      criarAtividadeMock({ inicio: "2026-11-20T10:00:00Z" }),
      agora,
    );
    expect(aberta.map((e) => [e.id, e.estado])).toEqual([
      ["inicio", "atual"],
      ["em_progresso", "pendente"],
      ["entregue", "pendente"],
      ["corrigida", "pendente"],
    ]);

    const emProgresso = timelineAtividadeAluno(
      criarAtividadeMock({ em_andamento: true }),
      agora,
    );
    expect(emProgresso.map((e) => [e.id, e.estado])).toEqual([
      ["inicio", "concluida"],
      ["em_progresso", "atual"],
      ["entregue", "pendente"],
      ["corrigida", "pendente"],
    ]);

    const entregue = timelineAtividadeAluno(
      criarAtividadeMock({ entregue: true }),
      agora,
    );
    expect(entregue.map((e) => [e.id, e.estado])).toEqual([
      ["inicio", "concluida"],
      ["em_progresso", "concluida"],
      ["entregue", "atual"],
      ["corrigida", "pendente"],
    ]);

    const corrigida = timelineAtividadeAluno(
      criarAtividadeMock({
        entregue: true,
        publicada_em: "2026-11-23T18:00:00Z",
      }),
      agora,
    );
    expect(corrigida.map((e) => [e.id, e.estado])).toEqual([
      ["inicio", "concluida"],
      ["em_progresso", "concluida"],
      ["entregue", "concluida"],
      ["corrigida", "concluida"],
    ]);

    const emProgressoExpirada = timelineAtividadeAluno(
      criarAtividadeMock({
        em_andamento: true,
        prazo: "2026-11-22T23:59:00Z",
        pode_atrasado: false,
      }),
      agora,
    );
    expect(emProgressoExpirada.map((e) => [e.id, e.rotulo, e.estado])).toEqual([
      ["inicio", "Início", "concluida"],
      ["em_progresso", "Fim do prazo", "expirada"],
      ["entregue", "Entregue", "pendente"],
      ["corrigida", "Corrigida", "pendente"],
    ]);

    const naoIniciadaExpirada = timelineAtividadeAluno(
      criarAtividadeMock({
        em_andamento: false,
        prazo: "2026-11-22T23:59:00Z",
        pode_atrasado: false,
      }),
      agora,
    );
    expect(naoIniciadaExpirada.map((e) => [e.id, e.rotulo, e.estado])).toEqual([
      ["inicio", "Início", "expirada"],
      ["em_progresso", "Em progresso", "pendente"],
      ["entregue", "Entregue", "pendente"],
      ["corrigida", "Corrigida", "pendente"],
    ]);
  });

  it("agrupa atividades por data em ordem cronológica e encontra o dia mais atual", () => {
    const a1 = criarAtividadeMock({ id: 1, prazo: "2026-11-25T18:00:00-03:00" });
    const a2 = criarAtividadeMock({ id: 2, prazo: "2026-11-21T18:00:00-03:00" });
    const a3 = criarAtividadeMock({ id: 3, prazo: "2026-11-25T21:00:00-03:00" });

    const setores = agruparAtividadesPorData([a1, a2, a3], false);
    expect(setores.map((s) => s.chave)).toEqual(["2026-11-21", "2026-11-25"]);
    expect(setores[1]?.itens.map((i) => i.id)).toEqual([1, 3]);

    expect(indiceDiaMaisAtual(setores, "2026-11-23")).toBe(1);
    expect(indiceDiaMaisAtual(setores, "2026-11-21")).toBe(0);
    expect(indiceDiaMaisAtual(setores, "2026-11-30")).toBe(1);
  });

  it("gera grade completa do calendário mensal", () => {
    const dias = gerarDiasCalendario(2026, 10); // Novembro 2026
    expect(dias.length % 7).toBe(0);
    expect(dias.some((d) => d.chave === "2026-11-23" && d.doMes)).toBe(true);
  });

  it("gera dados mocados com atividades atuais e entregues em múltiplas datas", () => {
    const mock = atribuicoesMockAluno(new Date("2026-11-23T12:00:00-03:00"));
    const semEntregues = agruparAtividadesPorData(mock, false);
    const comEntregues = agruparAtividadesPorData(mock, true);
    expect(semEntregues.length).toBeGreaterThanOrEqual(3);
    expect(comEntregues.length).toBeGreaterThanOrEqual(semEntregues.length);
    expect(mock.some((a) => isAtividadeEmProgresso(a))).toBe(true);
    expect(mock.some((a) => a.entregue && Boolean(a.publicada_em))).toBe(true);
  });

  it("pagina a lista de atividades do aluno corretamente", () => {
    const lista = Array.from({ length: 23 }, (_, idx) => ({ id: idx + 1 }));

    const p1 = paginarAtribuicoesAluno(lista, 1, 10);
    expect(p1.total).toBe(23);
    expect(p1.totalPaginas).toBe(3);
    expect(p1.paginaAtual).toBe(1);
    expect(p1.itens.map((x) => x.id)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);

    const p3 = paginarAtribuicoesAluno(lista, 3, 10);
    expect(p3.paginaAtual).toBe(3);
    expect(p3.itens.map((x) => x.id)).toEqual([21, 22, 23]);

    const foraLimite = paginarAtribuicoesAluno(lista, 99, 10);
    expect(foraLimite.paginaAtual).toBe(3);
    expect(foraLimite.itens).toHaveLength(3);
  });
});
