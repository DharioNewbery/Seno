import { describe, expect, it } from "vitest";
import { PORTAIS } from "../src/lib/portais";
import {
  formatarQuantidadeAlunos,
  ordenarPeriodosRecentes,
  resolverMateriaSelecionada,
  resolverPeriodoSelecionado,
  tituloCardTurma,
} from "../src/lib/turmas-professor";

describe("turmas-professor helpers", () => {
  const periodos = [
    { id: 1, trimestre: 1, ano: 2025 },
    { id: 3, trimestre: 2, ano: 2026 },
    { id: 2, trimestre: 1, ano: 2026 },
  ];

  const materias = [
    { id: 10, codigo: "PROG1", nome: "Programação I" },
    { id: 20, codigo: "EDA", nome: "Estruturas de Dados" },
  ];

  it("ordena períodos do mais recente para o mais antigo", () => {
    const ordenados = ordenarPeriodosRecentes(periodos);
    expect(ordenados.map((p) => p.id)).toEqual([3, 2, 1]);
  });

  it("seleciona o período mais recente por padrão e trata ausência de períodos", () => {
    expect(resolverPeriodoSelecionado([], null)).toBeUndefined();
    expect(resolverPeriodoSelecionado(periodos, null)).toBe(3);
    expect(resolverPeriodoSelecionado(periodos, "")).toBe(3);
    expect(resolverPeriodoSelecionado(periodos, "999")).toBe(3);
    expect(resolverPeriodoSelecionado(periodos, "2")).toBe(2);
  });

  it("resolve matéria selecionada com padrão para todas as matérias", () => {
    expect(resolverMateriaSelecionada(materias, null)).toBeUndefined();
    expect(resolverMateriaSelecionada(materias, "")).toBeUndefined();
    expect(resolverMateriaSelecionada(materias, "99")).toBeUndefined();
    expect(resolverMateriaSelecionada(materias, "20")).toBe(20);
  });

  it("exibe título da turma ou #<id> quando sem título", () => {
    expect(tituloCardTurma({ id: 42, titulo: "Turma A - Noturno" })).toBe(
      "Turma A - Noturno",
    );
    expect(tituloCardTurma({ id: 42, titulo: "   " })).toBe("#42");
    expect(tituloCardTurma({ id: 42, titulo: "" })).toBe("#42");
    expect(tituloCardTurma({ id: 42 })).toBe("#42");
  });

  it("formata quantidade de alunos no singular e plural", () => {
    expect(formatarQuantidadeAlunos(0)).toBe("0 alunos");
    expect(formatarQuantidadeAlunos(1)).toBe("1 aluno");
    expect(formatarQuantidadeAlunos(25)).toBe("25 alunos");
    expect(formatarQuantidadeAlunos(undefined)).toBe("0 alunos");
  });

  it("navegação do portal professor expõe item único Turmas sem subtelas", () => {
    const itemTurmas = PORTAIS.professor.nav.find(
      (i) => i.href === "/professor/turmas",
    );
    expect(itemTurmas).toBeDefined();
    expect(itemTurmas?.rotulo).toBe("Turmas");
    expect(itemTurmas?.sub).toBeUndefined();
  });
});
