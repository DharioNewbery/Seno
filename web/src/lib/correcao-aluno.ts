import type {
  AtribuicaoDoAluno,
  CorrecaoTarefa,
  SubmissaoAluno,
} from "#lib/server/ensino";

/**
 * Calcula a nota obtida e o total de pontos possíveis de uma lista de tarefas corrigidas.
 */
export function calcularTotaisCorrecao(
  tarefas: CorrecaoTarefa[] | null | undefined,
): {
  notaObtida: number;
  valorTotal: number;
} {
  let notaObtida = 0;
  let valorTotal = 0;
  for (const t of tarefas ?? []) {
    notaObtida += t.nota_final ?? t.nota_auto ?? 0;
    valorTotal += t.valor_pts ?? 0;
  }
  return { notaObtida, valorTotal };
}

/**
 * Obtém o código-fonte submetido pelo aluno em uma tarefa, priorizando `t.codigo`
 * retornado na correção e recorrendo ao snapshot da entrega como fallback.
 */
export function extrairCodigoRespostaAluno(
  t: CorrecaoTarefa,
  entrega: SubmissaoAluno | null | undefined,
): string {
  if (typeof t.codigo === "string" && t.codigo.length > 0) {
    return t.codigo;
  }
  const snapTask = entrega?.snapshot?.tasks?.find(
    (tk) => tk.id === t.tarefa_id,
  );
  return snapTask?.text ?? "";
}

/**
 * Determina o rótulo e a classe visual do estado de uma atribuição para o aluno.
 * Quando corrigida e publicada, exibe a nota obtida.
 */
export function resumirEstadoAtribuicaoAluno(
  a:
    | Pick<
        AtribuicaoDoAluno,
        | "entregue"
        | "atrasada"
        | "publicada_em"
        | "nota_final"
        | "em_andamento"
        | "prazo"
        | "pode_atrasado"
      >
    | null
    | undefined,
): {
  rotulo: string;
  classe: "nota" | "entregue" | "andamento" | "encerrada" | "nao";
} {
  if (!a) return { rotulo: "Não começou", classe: "nao" };
  if (a.entregue) {
    if (a.publicada_em) {
      return { rotulo: "Corrigida", classe: "nota" };
    }
    return a.atrasada
      ? { rotulo: "Entregue atrasada", classe: "entregue" }
      : { rotulo: "Entregue", classe: "entregue" };
  }
  if (a.em_andamento) {
    return { rotulo: "Em andamento", classe: "andamento" };
  }
  if (a.prazo && new Date(a.prazo) < new Date() && !a.pode_atrasado) {
    return { rotulo: "Prazo encerrado", classe: "encerrada" };
  }
  return { rotulo: "Não começou", classe: "nao" };
}
