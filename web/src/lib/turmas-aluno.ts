import type { TurmaDoAluno } from "#lib/server/ensino";

/**
 * Filtra apenas as turmas ativas do aluno, omitindo turmas encerradas/inativas.
 */
export function filtrarTurmasAtivasAluno(
  turmas: TurmaDoAluno[] | null | undefined,
): TurmaDoAluno[] {
  return (turmas ?? []).filter((t) => Boolean(t?.ativa));
}

/**
 * Formata a primeira linha do card da turma do aluno:
 * `<Nome por extenso> (<título>)`
 */
export function formatarTituloCardTurmaAluno(
  t: Pick<TurmaDoAluno, "id" | "materia_nome" | "titulo">,
): string {
  const titulo = (t.titulo ?? "").trim() || `#${t.id}`;
  return `${t.materia_nome} (${titulo})`;
}

/**
 * Formata a segunda linha do card da turma do aluno:
 * `<período> / prof. (a) <nome do professor>`
 */
export function formatarSubtituloCardTurmaAluno(
  t: Pick<TurmaDoAluno, "trimestre" | "ano" | "professor">,
): string {
  return `${t.trimestre}T/${t.ano} / prof. (a) ${t.professor}`;
}
