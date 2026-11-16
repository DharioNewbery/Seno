import { describe, expect, it } from "vitest";
import { PORTAIS } from "#lib/portais";
import {
  filtrarTurmasAtivasAluno,
  formatarSubtituloCardTurmaAluno,
  formatarTituloCardTurmaAluno,
} from "#lib/turmas-aluno";
import { LUCIDE_ICONS } from "#lib/vendor/lucide-icons";
import type { TurmaDoAluno } from "#lib/server/ensino";

function criarTurmaMock(parcial: Partial<TurmaDoAluno> = {}): TurmaDoAluno {
  return {
    id: 1,
    titulo: "T1",
    materia_codigo: "PROG1",
    materia_nome: "Programação I",
    trimestre: 4,
    ano: 2026,
    professor: "Carlos Silva",
    professor_email: "carlos@seno.dev",
    ativa: true,
    created_at: "2026-10-01T12:00:00Z",
    ...parcial,
  };
}

describe("Portal do Aluno — Minhas turmas", () => {
  it("remove a subnavegação 'Listar turmas' e mantém apenas o item 'Minhas turmas'", () => {
    const itemTurmas = PORTAIS.aluno.nav.find(
      (n) => n.href === "/aluno/turmas",
    );
    expect(itemTurmas).toBeDefined();
    expect(itemTurmas?.rotulo).toBe("Minhas turmas");
    expect(itemTurmas?.sub).toBeUndefined();
  });

  it("filtra apenas turmas ativas e omite turmas inativas", () => {
    const tAtiva = criarTurmaMock({ id: 1, ativa: true });
    const tInativa = criarTurmaMock({ id: 2, ativa: false });
    const filtradas = filtrarTurmasAtivasAluno([tAtiva, tInativa]);

    expect(filtradas).toHaveLength(1);
    expect(filtradas[0]?.id).toBe(1);
  });

  it("formata o cabeçalho e o subtítulo do card conforme especificado", () => {
    const turma = criarTurmaMock({
      id: 7,
      materia_nome: "Estrutura de Dados",
      titulo: "Turma A",
      trimestre: 2,
      ano: 2026,
      professor: "Maria Souza",
    });

    expect(formatarTituloCardTurmaAluno(turma)).toBe(
      "Estrutura de Dados (Turma A)",
    );
    expect(formatarSubtituloCardTurmaAluno(turma)).toBe(
      "2T/2026 / prof. (a) Maria Souza",
    );
  });

  it("usa o identificador da turma quando o título estiver vazio", () => {
    const turmaSemTitulo = criarTurmaMock({
      id: 12,
      materia_nome: "Algoritmos",
      titulo: "",
    });
    expect(formatarTituloCardTurmaAluno(turmaSemTitulo)).toBe(
      "Algoritmos (#12)",
    );
  });

  it("possui ícones Lucide vendored localmente para ações do card", () => {
    expect(LUCIDE_ICONS.eye.length).toBeGreaterThan(0);
    expect(LUCIDE_ICONS["clipboard-list"].length).toBeGreaterThan(0);
  });
});
