export interface AtributoAtividade {
  tarefa_id: number;
  valor_pts: number;
  linguagem: string;
  expandida?: boolean;
}

export interface TarefaSimples {
  id: number;
  nome: string;
}

/**
 * Adiciona IDs ao workspace evitando duplicatas e tarefas que já estão na atividade.
 */
export function adicionarAoWorkspace(
  workspace: number[],
  novosIds: number[],
  atividadeIds: number[],
): number[] {
  const atvSet = new Set(atividadeIds);
  const wsSet = new Set(workspace);
  const resultado = [...workspace];

  for (const id of novosIds) {
    if (!wsSet.has(id) && !atvSet.has(id)) {
      wsSet.add(id);
      resultado.push(id);
    }
  }
  return resultado;
}

/**
 * Remove uma tarefa do workspace.
 */
export function removerDoWorkspace(workspace: number[], tarefaId: number): number[] {
  return workspace.filter((id) => id !== tarefaId);
}

/**
 * Move tarefa do workspace para a atividade na posição especificada (targetIndex).
 * Ao largar/adicionar na atividade, ela se expande por padrão.
 */
export function moverWorkspaceParaAtividade(
  workspace: number[],
  atributos: AtributoAtividade[],
  tarefaId: number,
  targetIndex: number,
  opcoes?: { valor_pts?: number; linguagem?: string; expandida?: boolean },
): { workspace: number[]; atributos: AtributoAtividade[] } {
  // Evita duplicatas na atividade se a tarefa já existir
  if (atributos.some((a) => a.tarefa_id === tarefaId)) {
    return { workspace: removerDoWorkspace(workspace, tarefaId), atributos };
  }

  const novoWorkspace = removerDoWorkspace(workspace, tarefaId);
  const novoItem: AtributoAtividade = {
    tarefa_id: tarefaId,
    valor_pts: opcoes?.valor_pts ?? 10,
    linguagem: opcoes?.linguagem ?? "python",
    expandida: opcoes?.expandida ?? false,
  };

  const clampedIndex = Math.max(0, Math.min(targetIndex, atributos.length));
  const novosAtributos = [...atributos];
  novosAtributos.splice(clampedIndex, 0, novoItem);

  return { workspace: novoWorkspace, atributos: novosAtributos };
}

/**
 * Remove tarefa da atividade e a devolve para o workspace.
 */
export function removerDaAtividade(
  atributos: AtributoAtividade[],
  index: number,
  workspace: number[],
): { workspace: number[]; atributos: AtributoAtividade[] } {
  if (index < 0 || index >= atributos.length) {
    return { workspace, atributos };
  }
  const item = atributos[index];
  if (!item) {
    return { workspace, atributos };
  }
  const novosAtributos = atributos.filter((_, i) => i !== index);
  const novoWorkspace = workspace.includes(item.tarefa_id)
    ? workspace
    : [...workspace, item.tarefa_id];

  return { workspace: novoWorkspace, atributos: novosAtributos };
}

/**
 * Reordena tarefas dentro da atividade (reposicionamento).
 */
export function reordenarAtividade(
  atributos: AtributoAtividade[],
  fromIndex: number,
  targetIndex: number,
): AtributoAtividade[] {
  if (
    fromIndex < 0 ||
    fromIndex >= atributos.length ||
    fromIndex === targetIndex
  ) {
    return atributos;
  }

  const item = atributos[fromIndex];
  if (!item) {
    return atributos;
  }
  const copia = [...atributos];
  copia.splice(fromIndex, 1);

  let dest = targetIndex;
  if (fromIndex < targetIndex) {
    dest = targetIndex - 1;
  }
  dest = Math.max(0, Math.min(dest, copia.length));
  copia.splice(dest, 0, item);
  return copia;
}

/**
 * Serializa os atributos para o formato JSON esperado pelo backend.
 */
export function serializarAtributos(atributos: AtributoAtividade[]): string {
  const validos = atributos
    .filter(
      (a) =>
        a.tarefa_id != null &&
        Number.isInteger(a.tarefa_id) &&
        a.tarefa_id > 0,
    )
    .map((a) => {
      const numPts = Number(a.valor_pts);
      const safePts =
        Number.isFinite(numPts) && numPts >= 0 ? Math.floor(numPts) : 10;
      const safeLing = ["python", "c", "cpp"].includes(a.linguagem)
        ? a.linguagem
        : "python";
      return {
        tarefa_id: a.tarefa_id,
        valor_pts: safePts,
        linguagem: safeLing,
      };
    });
  return JSON.stringify(validos);
}

/**
 * Filtra tarefas por nome ou ID.
 */
export function filtrarTarefas(
  tarefas: TarefaSimples[],
  busca: string,
): TarefaSimples[] {
  const q = busca.trim().toLowerCase();
  if (!q) return tarefas;
  return tarefas.filter(
    (t) => t.nome.toLowerCase().includes(q) || String(t.id).includes(q),
  );
}

/**
 * Pagina uma lista de itens.
 */
export function paginar<T>(
  itens: T[],
  pagina: number,
  porPagina: number,
): { itens: T[]; totalPaginas: number; paginaAtual: number } {
  const totalPaginas = Math.max(1, Math.ceil(itens.length / porPagina));
  const paginaAtual = Math.max(1, Math.min(pagina, totalPaginas));
  const inicio = (paginaAtual - 1) * porPagina;
  const itensPagina = itens.slice(inicio, inicio + porPagina);
  return { itens: itensPagina, totalPaginas, paginaAtual };
}
