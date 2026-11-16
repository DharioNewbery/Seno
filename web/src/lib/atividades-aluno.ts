export const POR_PAGINA_ATIVIDADES_ALUNO = 10;

export interface PaginacaoResultado<T> {
  itens: T[];
  paginaAtual: number;
  totalPaginas: number;
  total: number;
}

/**
 * Pagina uma lista de atribuições do aluno, ajustando páginas inválidas
 * ou fora do intervalo para `[1, totalPaginas]`.
 */
export function paginarAtribuicoesAluno<T>(
  lista: T[] | null | undefined,
  pagina = 1,
  porPagina = POR_PAGINA_ATIVIDADES_ALUNO,
): PaginacaoResultado<T> {
  const itensLista = lista ?? [];
  const total = itensLista.length;
  const limite = Math.max(1, Math.floor(porPagina) || POR_PAGINA_ATIVIDADES_ALUNO);
  const totalPaginas = Math.max(1, Math.ceil(total / limite));
  const paginaLimpa = Number.isFinite(pagina) ? Math.floor(pagina) : 1;
  const paginaAtual = Math.min(Math.max(1, paginaLimpa), totalPaginas);
  const inicio = (paginaAtual - 1) * limite;
  return {
    itens: itensLista.slice(inicio, inicio + limite),
    paginaAtual,
    totalPaginas,
    total,
  };
}
