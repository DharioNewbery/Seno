export interface TesteFormulario {
  stdin: string;
  stdout: string;
  publico: boolean;
}

/**
 * Garante que o índice ativo aponte para um teste válido da lista
 * (ou 0 quando a lista estiver vazia).
 */
export function ajustarIndiceTeste(
  indiceAtual: number,
  totalTestes: number,
): number {
  if (totalTestes <= 0) return 0;
  if (indiceAtual < 0) return 0;
  if (indiceAtual >= totalTestes) return totalTestes - 1;
  return indiceAtual;
}

/**
 * Navega para o teste anterior (-1) ou próximo (+1) mantendo o índice nos limites.
 */
export function navegarIndiceTeste(
  indiceAtual: number,
  delta: -1 | 1,
  totalTestes: number,
): number {
  return ajustarIndiceTeste(indiceAtual + delta, totalTestes);
}

/**
 * Adiciona um novo teste ao final da lista e retorna a nova lista + o índice do teste criado.
 */
export function adicionarTesteNaLista(
  lista: TesteFormulario[],
  novo: TesteFormulario,
): { lista: TesteFormulario[]; indice: number } {
  const novaLista = [
    ...lista,
    {
      stdin: novo.stdin,
      stdout: novo.stdout,
      publico: Boolean(novo.publico),
    },
  ];
  return {
    lista: novaLista,
    indice: novaLista.length - 1,
  };
}

/**
 * Atualiza um teste existente pelo índice e retorna a nova lista + o índice mantido.
 */
export function atualizarTesteNaLista(
  lista: TesteFormulario[],
  indice: number,
  atualizado: TesteFormulario,
): { lista: TesteFormulario[]; indice: number } {
  if (indice < 0 || indice >= lista.length) {
    return adicionarTesteNaLista(lista, atualizado);
  }
  const novaLista = lista.map((item, idx) =>
    idx === indice
      ? {
          stdin: atualizado.stdin,
          stdout: atualizado.stdout,
          publico: Boolean(atualizado.publico),
        }
      : item,
  );
  return {
    lista: novaLista,
    indice,
  };
}

/**
 * Remove um teste pelo índice e retorna a nova lista + o próximo índice válido.
 */
export function removerTesteDaLista(
  lista: TesteFormulario[],
  indiceRemover: number,
): { lista: TesteFormulario[]; indice: number } {
  const novaLista = lista.filter((_, idx) => idx !== indiceRemover);
  return {
    lista: novaLista,
    indice: ajustarIndiceTeste(indiceRemover, novaLista.length),
  };
}
