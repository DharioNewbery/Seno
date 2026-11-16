import { describe, expect, it } from "vitest";
import { paginarAtribuicoesAluno } from "#lib/atividades-aluno";
import { PORTAIS } from "#lib/portais";

describe("Portal do Aluno — Atividades", () => {
  it("remove 'Todas as atividades' e 'Execuções de teste', mantendo 'Atividades' como navegação única", () => {
    const itemAtividades = PORTAIS.aluno.nav.find(
      (n) => n.href === "/aluno/atividades",
    );
    expect(itemAtividades).toBeDefined();
    expect(itemAtividades?.rotulo).toBe("Atividades");
    expect(itemAtividades?.sub).toBeUndefined();
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
