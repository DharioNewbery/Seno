import type { AtribuicaoDoAluno } from "#lib/server/ensino";

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
  const limite = Math.max(
    1,
    Math.floor(porPagina) || POR_PAGINA_ATIVIDADES_ALUNO,
  );
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

/**
 * Indica se a atividade já foi iniciada pelo aluno, mas ainda não foi entregue.
 */
export function isAtividadeEmProgresso(
  a:
    | Pick<AtribuicaoDoAluno, "entregue" | "em_andamento" | "iniciada_em">
    | null
    | undefined,
): boolean {
  if (!a || a.entregue) return false;
  return Boolean(a.em_andamento || a.iniciada_em);
}

/**
 * Filtra as atribuições de turmas ativas do aluno. Quando `mostrarEntregues`
 * é `false`, exibe apenas atividades ainda não entregues; quando `true`, inclui
 * também as já entregues. Em ambos os casos, prioriza atividades em progresso.
 */
export function filtrarAtividadesAluno<
  T extends Pick<
    AtribuicaoDoAluno,
    "entregue" | "em_andamento" | "iniciada_em"
  > & { turma_ativa?: boolean },
>(lista: T[] | null | undefined, mostrarEntregues = false): T[] {
  const ativas = (lista ?? []).filter((a) => a.turma_ativa !== false);
  const visiveis = mostrarEntregues
    ? ativas
    : ativas.filter((a) => !a.entregue);
  return [...visiveis].sort((a, b) => {
    const progA = isAtividadeEmProgresso(a) ? 0 : a.entregue ? 2 : 1;
    const progB = isAtividadeEmProgresso(b) ? 0 : b.entregue ? 2 : 1;
    return progA - progB;
  });
}

/**
 * Retorna a sigla (iniciais) do usuário para exibição no avatar.
 */
export function iniciaisUsuario(
  firstName: string | null | undefined,
  lastName?: string | null | undefined,
): string {
  const p = (firstName ?? "").trim();
  const u = (lastName ?? "").trim();
  const primeira = p ? p.charAt(0).toUpperCase() : "";
  const ultima = u ? u.charAt(0).toUpperCase() : "";
  return `${primeira}${ultima}` || "?";
}

/**
 * Nome amigável da matéria da atividade para exibição no card.
 */
export function nomeMateriaAtividade(
  a: Pick<AtribuicaoDoAluno, "materia_nome" | "materia_codigo">,
): string {
  const nome = (a.materia_nome ?? "").trim();
  return nome || a.materia_codigo;
}

export type EtapaTimelineId =
  "inicio" | "em_progresso" | "entregue" | "corrigida";

export type EstadoEtapaTimeline =
  "concluida" | "atual" | "pendente" | "expirada";

export interface EtapaTimeline {
  id: EtapaTimelineId;
  rotulo: string;
  estado: EstadoEtapaTimeline;
}

/**
 * Calcula os 4 marcos da timeline do card de atividade:
 * Início -> Em progresso (ou Fim do prazo) -> Entregue -> Corrigida.
 */
export function timelineAtividadeAluno(
  a: Pick<
    AtribuicaoDoAluno,
    | "inicio"
    | "prazo"
    | "pode_atrasado"
    | "em_andamento"
    | "iniciada_em"
    | "entregue"
    | "publicada_em"
  >,
  agora: Date = new Date(),
): EtapaTimeline[] {
  const corrigida = Boolean(a.entregue && a.publicada_em);
  const entregue = Boolean(a.entregue);
  const emProgresso = !entregue && Boolean(a.em_andamento || a.iniciada_em);
  const prazoEncerrado =
    !entregue &&
    Boolean(a.prazo && new Date(a.prazo) < agora && !a.pode_atrasado);
  const jaAbriu = !a.inicio || new Date(a.inicio) <= agora;

  if (emProgresso && prazoEncerrado) {
    return [
      { id: "inicio", rotulo: "Início", estado: "concluida" },
      { id: "em_progresso", rotulo: "Fim do prazo", estado: "expirada" },
      { id: "entregue", rotulo: "Entregue", estado: "pendente" },
      { id: "corrigida", rotulo: "Corrigida", estado: "pendente" },
    ];
  }

  if (!emProgresso && prazoEncerrado) {
    return [
      { id: "inicio", rotulo: "Início", estado: "expirada" },
      { id: "em_progresso", rotulo: "Em progresso", estado: "pendente" },
      { id: "entregue", rotulo: "Entregue", estado: "pendente" },
      { id: "corrigida", rotulo: "Corrigida", estado: "pendente" },
    ];
  }

  const estadoInicio: EstadoEtapaTimeline =
    entregue || emProgresso ? "concluida" : jaAbriu ? "atual" : "pendente";

  const estadoEmProgresso: EstadoEtapaTimeline = entregue
    ? "concluida"
    : emProgresso
      ? "atual"
      : "pendente";

  const estadoEntregue: EstadoEtapaTimeline = corrigida
    ? "concluida"
    : entregue
      ? "atual"
      : "pendente";

  const estadoCorrigida: EstadoEtapaTimeline = corrigida
    ? "concluida"
    : "pendente";

  return [
    { id: "inicio", rotulo: "Início", estado: estadoInicio },
    { id: "em_progresso", rotulo: "Em progresso", estado: estadoEmProgresso },
    { id: "entregue", rotulo: "Entregue", estado: estadoEntregue },
    { id: "corrigida", rotulo: "Corrigida", estado: estadoCorrigida },
  ];
}

const FUSO_SISTEMA = "America/Belem";

const fmtChaveData = new Intl.DateTimeFormat("sv-SE", {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  timeZone: FUSO_SISTEMA,
});

const fmtRotuloDiaMes = new Intl.DateTimeFormat("pt-BR", {
  day: "2-digit",
  month: "short",
  timeZone: FUSO_SISTEMA,
});

/**
 * Converte um timestamp ISO para chave `YYYY-MM-DD` no fuso de Belém.
 */
export function chaveDataBelem(
  valor: string | number | Date | null | undefined,
): string | null {
  if (valor === null || valor === undefined || valor === "") return null;
  const d = valor instanceof Date ? valor : new Date(valor);
  if (Number.isNaN(d.getTime())) return null;
  return fmtChaveData.format(d);
}

/**
 * Formata uma chave `YYYY-MM-DD` para exibição curta (ex.: "23 nov").
 */
export function formatarRotuloData(chave: string): string {
  if (chave === "sem-data") return "Sem data";
  const d = new Date(`${chave}T12:00:00-03:00`);
  if (Number.isNaN(d.getTime())) return chave;
  return fmtRotuloDiaMes.format(d).replace(".", "").replace(" de ", " ");
}

export interface SetorData<T> {
  chave: string;
  rotulo: string;
  itens: T[];
}

/**
 * Determina a chave de data principal de uma atribuição para agrupamento em setores.
 */
export function chaveDataAtividade(
  a: Pick<AtribuicaoDoAluno, "prazo" | "inicio" | "entregue" | "entregue_em">,
): string {
  const bruto = a.entregue
    ? (a.entregue_em ?? a.prazo ?? a.inicio)
    : (a.prazo ?? a.inicio);
  return chaveDataBelem(bruto) ?? "sem-data";
}

/**
 * Filtra e agrupa as atividades do aluno em setores ordenados por data.
 */
export function agruparAtividadesPorData<
  T extends Pick<
    AtribuicaoDoAluno,
    | "prazo"
    | "inicio"
    | "entregue"
    | "entregue_em"
    | "em_andamento"
    | "iniciada_em"
  > & { turma_ativa?: boolean },
>(lista: T[] | null | undefined, mostrarEntregues = false): SetorData<T>[] {
  const filtradas = filtrarAtividadesAluno(lista, mostrarEntregues);
  const mapa = new Map<string, T[]>();
  for (const item of filtradas) {
    const chave = chaveDataAtividade(item);
    const grupo = mapa.get(chave);
    if (grupo) grupo.push(item);
    else mapa.set(chave, [item]);
  }
  const chaves = Array.from(mapa.keys()).sort((a, b) => {
    if (a === "sem-data") return 1;
    if (b === "sem-data") return -1;
    return a.localeCompare(b);
  });
  return chaves.map((chave) => ({
    chave,
    rotulo: formatarRotuloData(chave),
    itens: mapa.get(chave) ?? [],
  }));
}

/**
 * Retorna o índice do setor correspondente ao dia mais atual (hoje, próximo dia
 * futuro ou último dia com atividade).
 */
export function indiceDiaMaisAtual(
  setores: ReadonlyArray<{ chave: string }>,
  hojeChave: string,
): number {
  if (setores.length === 0) return 0;
  const exato = setores.findIndex((s) => s.chave === hojeChave);
  if (exato !== -1) return exato;
  const proximo = setores.findIndex(
    (s) => s.chave !== "sem-data" && s.chave >= hojeChave,
  );
  if (proximo !== -1) return proximo;
  for (let i = setores.length - 1; i >= 0; i--) {
    if (setores[i]?.chave !== "sem-data") return i;
  }
  return 0;
}

export interface DiaCalendario {
  chave: string;
  dia: number;
  doMes: boolean;
}

/**
 * Gera a grade semanal (domingo a sábado) de um mês para o calendário.
 */
export function gerarDiasCalendario(
  ano: number,
  mesZeroBased: number,
): DiaCalendario[] {
  const primeiroDia = new Date(ano, mesZeroBased, 1);
  const inicioSemana = primeiroDia.getDay();
  const diasNoMes = new Date(ano, mesZeroBased + 1, 0).getDate();
  const totalCelulas = Math.ceil((inicioSemana + diasNoMes) / 7) * 7;

  const celulas: DiaCalendario[] = [];
  for (let i = 0; i < totalCelulas; i++) {
    const d = new Date(ano, mesZeroBased, 1 - inicioSemana + i);
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    celulas.push({
      chave: `${y}-${m}-${day}`,
      dia: d.getDate(),
      doMes: d.getMonth() === mesZeroBased,
    });
  }
  return celulas;
}

/**
 * Gera uma lista de atribuições mocadas relativas à data informada, para
 * visualização de `/aluno/atividades?mock`.
 */
export function atribuicoesMockAluno(
  agora: Date = new Date(),
): AtribuicaoDoAluno[] {
  const diaIso = (offsetDias: number, hora = 23, min = 59) => {
    const d = new Date(agora);
    d.setDate(d.getDate() + offsetDias);
    d.setHours(hora, min, 0, 0);
    return d.toISOString();
  };

  return [
    {
      id: 9001,
      turma_id: 1,
      atividade_id: 101,
      autocomplete: false,
      inicio: diaIso(-2, 8, 0),
      prazo: diaIso(0, 23, 59),
      pode_atrasado: true,
      materia_codigo: "PROG1",
      materia_nome: "Programação I",
      turma_ativa: true,
      atividade_nome: "Estruturas de Repetição",
      pontuacao_maxima: 10,
      em_andamento: true,
      iniciada_em: diaIso(-1, 14, 20),
      entregue: false,
      atrasada: false,
    },
    {
      id: 9002,
      turma_id: 1,
      atividade_id: 102,
      autocomplete: false,
      inicio: diaIso(-1, 8, 0),
      prazo: diaIso(0, 23, 59),
      pode_atrasado: false,
      materia_codigo: "PROG1",
      materia_nome: "Programação I",
      turma_ativa: true,
      atividade_nome: "Funções Recursivas",
      pontuacao_maxima: 10,
      em_andamento: false,
      entregue: false,
      atrasada: false,
    },
    {
      id: 9003,
      turma_id: 2,
      atividade_id: 103,
      autocomplete: false,
      inicio: diaIso(-1, 10, 0),
      prazo: diaIso(0, 23, 59),
      pode_atrasado: false,
      materia_codigo: "ALG",
      materia_nome: "Algoritmos",
      turma_ativa: true,
      atividade_nome: "Vetores e Matrizes",
      pontuacao_maxima: 15,
      em_andamento: false,
      entregue: false,
      atrasada: false,
    },
    {
      id: 9004,
      turma_id: 3,
      atividade_id: 104,
      autocomplete: false,
      inicio: diaIso(0, 8, 0),
      prazo: diaIso(1, 23, 59),
      pode_atrasado: false,
      materia_codigo: "ED1",
      materia_nome: "Estruturas de Dados",
      turma_ativa: true,
      atividade_nome: "Árvores Binárias",
      pontuacao_maxima: 20,
      em_andamento: false,
      entregue: false,
      atrasada: false,
    },
    {
      id: 9005,
      turma_id: 1,
      atividade_id: 105,
      autocomplete: false,
      inicio: diaIso(0, 9, 0),
      prazo: diaIso(1, 23, 59),
      pode_atrasado: true,
      materia_codigo: "PROG1",
      materia_nome: "Programação I",
      turma_ativa: true,
      atividade_nome: "Ponteiros em C",
      pontuacao_maxima: 10,
      em_andamento: false,
      entregue: false,
      atrasada: false,
    },
    {
      id: 9006,
      turma_id: 3,
      atividade_id: 106,
      autocomplete: false,
      inicio: diaIso(-1, 8, 0),
      prazo: diaIso(3, 23, 59),
      pode_atrasado: false,
      materia_codigo: "ED1",
      materia_nome: "Estruturas de Dados",
      turma_ativa: true,
      atividade_nome: "Tabelas Hash",
      pontuacao_maxima: 15,
      em_andamento: true,
      iniciada_em: diaIso(0, 10, 15),
      entregue: false,
      atrasada: false,
    },
    {
      id: 9007,
      turma_id: 2,
      atividade_id: 107,
      autocomplete: false,
      inicio: diaIso(1, 8, 0),
      prazo: diaIso(3, 23, 59),
      pode_atrasado: false,
      materia_codigo: "ALG",
      materia_nome: "Algoritmos",
      turma_ativa: true,
      atividade_nome: "Grafos: BFS e DFS",
      pontuacao_maxima: 20,
      em_andamento: false,
      entregue: false,
      atrasada: false,
    },
    {
      id: 9008,
      turma_id: 4,
      atividade_id: 108,
      autocomplete: false,
      inicio: diaIso(1, 8, 0),
      prazo: diaIso(3, 23, 59),
      pode_atrasado: false,
      materia_codigo: "AA1",
      materia_nome: "Análise de Algoritmos",
      turma_ativa: true,
      atividade_nome: "Complexidade Assintótica",
      pontuacao_maxima: 10,
      em_andamento: false,
      entregue: false,
      atrasada: false,
    },
    {
      id: 9009,
      turma_id: 4,
      atividade_id: 109,
      autocomplete: false,
      inicio: diaIso(2, 8, 0),
      prazo: diaIso(6, 23, 59),
      pode_atrasado: false,
      materia_codigo: "AA1",
      materia_nome: "Análise de Algoritmos",
      turma_ativa: true,
      atividade_nome: "Programação Dinâmica",
      pontuacao_maxima: 25,
      em_andamento: false,
      entregue: false,
      atrasada: false,
    },
    {
      id: 9010,
      turma_id: 1,
      atividade_id: 110,
      autocomplete: false,
      inicio: diaIso(-7, 8, 0),
      prazo: diaIso(-3, 23, 59),
      pode_atrasado: false,
      materia_codigo: "PROG1",
      materia_nome: "Programação I",
      turma_ativa: true,
      atividade_nome: "Introdução ao Python",
      pontuacao_maxima: 10,
      em_andamento: false,
      entregue: true,
      entregue_em: diaIso(-3, 19, 10),
      atrasada: false,
      publicada_em: diaIso(-2, 11, 0),
      nota_final: 10,
    },
    {
      id: 9011,
      turma_id: 1,
      atividade_id: 111,
      autocomplete: false,
      inicio: diaIso(-6, 8, 0),
      prazo: diaIso(-3, 23, 59),
      pode_atrasado: false,
      materia_codigo: "PROG1",
      materia_nome: "Programação I",
      turma_ativa: true,
      atividade_nome: "Condicionais",
      pontuacao_maxima: 10,
      em_andamento: false,
      entregue: true,
      entregue_em: diaIso(-3, 20, 45),
      atrasada: false,
      publicada_em: diaIso(-2, 11, 5),
      nota_final: 9,
    },
    {
      id: 9012,
      turma_id: 3,
      atividade_id: 112,
      autocomplete: false,
      inicio: diaIso(-4, 8, 0),
      prazo: diaIso(-1, 23, 59),
      pode_atrasado: false,
      materia_codigo: "ED1",
      materia_nome: "Estruturas de Dados",
      turma_ativa: true,
      atividade_nome: "Listas Encadeadas",
      pontuacao_maxima: 15,
      em_andamento: false,
      entregue: true,
      entregue_em: diaIso(-1, 17, 30),
      atrasada: false,
    },
    {
      id: 9013,
      turma_id: 3,
      atividade_id: 113,
      autocomplete: false,
      inicio: diaIso(-4, 8, 0),
      prazo: diaIso(-1, 23, 59),
      pode_atrasado: false,
      materia_codigo: "ED1",
      materia_nome: "Estruturas de Dados",
      turma_ativa: true,
      atividade_nome: "Pilhas e Filas",
      pontuacao_maxima: 15,
      em_andamento: false,
      entregue: true,
      entregue_em: diaIso(-1, 18, 15),
      atrasada: false,
      publicada_em: diaIso(0, 9, 0),
      nota_final: 14,
    },
    {
      id: 9014,
      turma_id: 2,
      atividade_id: 114,
      autocomplete: false,
      inicio: diaIso(-3, 8, 0),
      prazo: diaIso(0, 23, 59),
      pode_atrasado: false,
      materia_codigo: "ALG",
      materia_nome: "Algoritmos",
      turma_ativa: true,
      atividade_nome: "Ordenação Quicksort",
      pontuacao_maxima: 15,
      em_andamento: false,
      entregue: true,
      entregue_em: diaIso(0, 15, 40),
      atrasada: false,
    },
  ];
}
