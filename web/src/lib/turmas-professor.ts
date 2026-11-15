import type { Materia, Periodo, Turma } from "#lib/server/ensino";

/**
 * Ordena períodos do mais recente para o mais antigo (ano DESC, trimestre DESC).
 */
export function ordenarPeriodosRecentes(periodos: Periodo[]): Periodo[] {
  return [...periodos].sort((a, b) => {
    if (b.ano !== a.ano) return b.ano - a.ano;
    if (b.trimestre !== a.trimestre) return b.trimestre - a.trimestre;
    return b.id - a.id;
  });
}

/**
 * Resolve o período selecionado na listagem de turmas do professor.
 * - Se não houver períodos cadastrados, retorna `undefined`.
 * - Se `param` corresponder a um período existente, retorna seu ID.
 * - Caso contrário, retorna o ID do período mais recente (não existe "todos os períodos").
 */
export function resolverPeriodoSelecionado(
  periodos: Periodo[],
  param: string | null | undefined,
): number | undefined {
  const ordenados = ordenarPeriodosRecentes(periodos);
  if (ordenados.length === 0) return undefined;
  const n = Number(param);
  if (Number.isInteger(n) && n > 0 && ordenados.some((p) => p.id === n)) {
    return n;
  }
  return ordenados[0]?.id;
}

/**
 * Resolve a matéria selecionada na listagem de turmas do professor.
 * Por padrão (vazio ou inválido), retorna `undefined` ("Todas as matérias").
 */
export function resolverMateriaSelecionada(
  materias: Materia[],
  param: string | null | undefined,
): number | undefined {
  if (!param) return undefined;
  const n = Number(param);
  if (Number.isInteger(n) && n > 0 && materias.some((m) => m.id === n)) {
    return n;
  }
  return undefined;
}

/**
 * Retorna o título principal do card da turma: o próprio título quando preenchido,
 * ou `#<id>` quando a turma não possui título.
 */
export function tituloCardTurma(turma: Pick<Turma, "id" | "titulo">): string {
  const limpo = turma.titulo?.trim();
  return limpo ? limpo : `#${turma.id}`;
}

/**
 * Formata a contagem de alunos matriculados na turma.
 */
export function formatarQuantidadeAlunos(total?: number | null): string {
  const n = Math.max(0, Math.trunc(total ?? 0));
  return n === 1 ? "1 aluno" : `${n} alunos`;
}
