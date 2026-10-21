// Formatação de datas para a interface. O Seno é um sistema da UFPA: prazos e
// entregas aparecem sempre no horário de Belém, independente do aparelho de
// quem olha (e igual no servidor e no navegador, sem divergência na hidratação).
const FUSO = "America/Belem";

const fmtDataHora = new Intl.DateTimeFormat("pt-BR", {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  timeZone: FUSO,
});

const fmtData = new Intl.DateTimeFormat("pt-BR", {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  timeZone: FUSO,
});

function valida(valor: string | number | Date | null | undefined): Date | null {
  if (valor === null || valor === undefined || valor === "") return null;
  const d = valor instanceof Date ? valor : new Date(valor);
  return Number.isNaN(d.getTime()) ? null : d;
}

/** "22/10/2026 19:43" — sem segundos. */
export function dataHora(valor: string | number | Date | null | undefined): string {
  const d = valida(valor);
  return d ? fmtDataHora.format(d).replace(",", "") : "—";
}

/** "22/10/2026". */
export function soData(valor: string | number | Date | null | undefined): string {
  const d = valida(valor);
  return d ? fmtData.format(d) : "—";
}

export type TomPrazo = "folga" | "breve" | "vencido";

/**
 * Distância de um prazo em linguagem natural ("em 2 dias", "há 3 horas") e o
 * tom correspondente: `breve` quando faltam menos de 48 h, `vencido` depois.
 */
export function prazoRelativo(
  iso: string | null | undefined,
  agora: Date = new Date(),
): { rotulo: string; tom: TomPrazo } | null {
  const d = valida(iso);
  if (!d) return null;
  const ms = d.getTime() - agora.getTime();
  const futuro = ms >= 0;
  const min = Math.round(Math.abs(ms) / 60_000);
  const horas = Math.round(min / 60);
  const dias = Math.round(horas / 24);

  let quanto: string;
  if (min < 1) quanto = "agora";
  else if (min < 60) quanto = `${min} min`;
  else if (horas < 24) quanto = horas === 1 ? "1 hora" : `${horas} horas`;
  else quanto = dias === 1 ? "1 dia" : `${dias} dias`;

  if (quanto === "agora") return { rotulo: "agora", tom: "breve" };
  const rotulo = futuro ? `em ${quanto}` : `há ${quanto}`;
  const tom: TomPrazo = !futuro ? "vencido" : ms < 48 * 3_600_000 ? "breve" : "folga";
  return { rotulo, tom };
}
