import type { AtribuicaoDoAluno, TurmaDoAluno } from "#lib/server/ensino";

// Mesmo fuso de `formato.ts`: datas de entrega valem no horário de Belém.
const FUSO = "America/Belem";

const fmtChave = new Intl.DateTimeFormat("en-CA", {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  timeZone: FUSO,
});

const fmtHora = new Intl.DateTimeFormat("pt-BR", {
  hour: "2-digit",
  minute: "2-digit",
  timeZone: FUSO,
});

export type EstadoAgenda =
  | "pendente" // ainda dá para entregar (inclui hoje e atrasado aceito)
  | "perdida" // prazo passou e não aceita atraso
  | "no_prazo"
  | "atrasada"; // entregue com atraso

export type AtribuicaoAgenda = Pick<
  AtribuicaoDoAluno,
  "entregue" | "atrasada" | "prazo" | "pode_atrasado" | "turma_ativa"
>;

/** "2026-11-20" no fuso de Belém. */
export function chaveDia(valor: string | number | Date): string {
  return fmtChave.format(new Date(valor));
}

export function horaDoPrazo(valor: string): string {
  return fmtHora.format(new Date(valor));
}

export function estadoNaAgenda(a: AtribuicaoAgenda, agora: Date): EstadoAgenda {
  if (a.entregue) return a.atrasada ? "atrasada" : "no_prazo";
  const prazo = new Date(a.prazo as string);
  if (prazo.getTime() < agora.getTime()) {
    return a.pode_atrasado ? "pendente" : "perdida";
  }
  return "pendente";
}

export type TomColuna = "futuro" | "hoje" | "passado" | "passado-aberto";

export interface ItemAgenda<T extends AtribuicaoAgenda> {
  atribuicao: T;
  estado: EstadoAgenda;
}

export interface ColunaAgenda<T extends AtribuicaoAgenda> {
  dia: string;
  prazo: string; // ISO do primeiro prazo do dia (para formatar o cabeçalho)
  itens: ItemAgenda<T>[];
  entregues: number;
  total: number;
  tom: TomColuna;
}

/**
 * Agrupa atribuições de turmas ativas (e com prazo) em colunas por data de
 * entrega, em ordem cronológica. Recalcula tudo a partir de `agora`.
 */
export function montarAgenda<T extends AtribuicaoAgenda>(
  lista: T[] | null | undefined,
  agora: Date,
): ColunaAgenda<T>[] {
  const hoje = chaveDia(agora);
  const porDia = new Map<string, ItemAgenda<T>[]>();
  for (const a of lista ?? []) {
    if (!a.turma_ativa || !a.prazo) continue;
    if (Number.isNaN(new Date(a.prazo).getTime())) continue;
    const dia = chaveDia(a.prazo);
    const itens = porDia.get(dia) ?? [];
    itens.push({ atribuicao: a, estado: estadoNaAgenda(a, agora) });
    porDia.set(dia, itens);
  }
  return [...porDia.entries()]
    .sort(([x], [y]) => (x < y ? -1 : x > y ? 1 : 0))
    .map(([dia, itens]) => {
      itens.sort(
        (x, y) =>
          new Date(x.atribuicao.prazo as string).getTime() -
          new Date(y.atribuicao.prazo as string).getTime(),
      );
      let tom: TomColuna = "futuro";
      if (dia === hoje) tom = "hoje";
      else if (dia < hoje) {
        tom = itens.some((i) => i.estado === "pendente")
          ? "passado-aberto"
          : "passado";
      }
      return {
        dia,
        prazo: itens[0]?.atribuicao.prazo as string,
        itens,
        entregues: itens.filter((i) => i.atribuicao.entregue).length,
        total: itens.length,
        tom,
      };
    });
}

export interface ResumoPainelAluno<T extends AtribuicaoDoAluno = AtribuicaoDoAluno> {
  paraHoje: number;
  pendentes: number;
  atrasadasEntregaveis: number;
  emAndamento: T[];
  entregues: number;
  corrigidasRecentes: T[];
  turmasAtivas: number;
}

/**
 * Resume indicadores e destaques do painel inicial do aluno a partir das
 * atribuições e turmas carregadas, reagindo ao instante `agora`.
 */
export function resumirPainelAluno<T extends AtribuicaoDoAluno>(
  atribuicoes: T[] | null | undefined,
  turmas: Pick<TurmaDoAluno, "id" | "ativa">[] | null | undefined,
  agora: Date,
): ResumoPainelAluno<T> {
  const lista = atribuicoes ?? [];
  const turmasAtivas = (turmas ?? []).filter((t) => t.ativa).length;
  const hoje = chaveDia(agora);

  let paraHoje = 0;
  let pendentes = 0;
  let atrasadasEntregaveis = 0;
  let entregues = 0;
  const emAndamento: T[] = [];
  const corrigidas: T[] = [];

  for (const a of lista) {
    if (a.entregue) {
      if (a.turma_ativa) entregues++;
      if (a.publicada_em) corrigidas.push(a);
      continue;
    }
    if (!a.turma_ativa) continue;

    if (!a.prazo || Number.isNaN(new Date(a.prazo).getTime())) {
      pendentes++;
      if (a.em_andamento) emAndamento.push(a);
      continue;
    }

    const est = estadoNaAgenda(a, agora);
    if (est === "pendente") {
      pendentes++;
      const prazoDate = new Date(a.prazo);
      if (prazoDate.getTime() < agora.getTime()) {
        atrasadasEntregaveis++;
      } else if (chaveDia(prazoDate) === hoje) {
        paraHoje++;
      }
    }

    if (a.em_andamento && est !== "perdida") {
      emAndamento.push(a);
    }
  }

  emAndamento.sort((x, y) => {
    const px = x.prazo ? new Date(x.prazo).getTime() : Number.POSITIVE_INFINITY;
    const py = y.prazo ? new Date(y.prazo).getTime() : Number.POSITIVE_INFINITY;
    return px - py;
  });

  corrigidas.sort((x, y) => {
    const px = x.publicada_em ? new Date(x.publicada_em).getTime() : 0;
    const py = y.publicada_em ? new Date(y.publicada_em).getTime() : 0;
    return py - px;
  });

  return {
    paraHoje,
    pendentes,
    atrasadasEntregaveis,
    emAndamento,
    entregues,
    corrigidasRecentes: corrigidas.slice(0, 4),
    turmasAtivas,
  };
}
