import type { ConfigTarefaArquivo, TipoTarefa } from "#lib/server/ensino";

export interface CandidatoMatricula {
  email: string;
  first_name: string;
  last_name: string;
}

export interface TarefaPreviewAtividade {
  tarefa_id: number;
  ordem: number;
  valor_pts: number;
  linguagem: string;
  tarefa_nome: string;
  tarefa_enunciado: string;
  tarefa_tipo?: TipoTarefa | string;
  tarefa_config_arquivo?: ConfigTarefaArquivo | null;
}

export interface AtividadeComPreview {
  id: number;
  nome: string;
  created_at?: string;
  tarefas: TarefaPreviewAtividade[];
}

export interface ResumoAtividadeCard {
  totalTarefas: number;
  totalPontos: number;
  etiquetas: string[];
}

const CABECALHOS_EMAIL = new Set([
  "email",
  "e-mail",
  "mail",
  "correio",
  "email_aluno",
]);

function limparCampoCsv(valor: string): string {
  const s = valor.trim();
  if (
    (s.startsWith('"') && s.endsWith('"')) ||
    (s.startsWith("'") && s.endsWith("'"))
  ) {
    return s.slice(1, -1).trim();
  }
  return s;
}

/**
 * Interpreta texto CSV/lote de alunos.
 * Aceita separadores `,`, `;` ou tabulação (`\t`), ignora cabeçalho opcional
 * (`email,nome,sobrenome`) e suporta linhas com:
 * - `email,nome,sobrenome`
 * - `email,nome completo`
 * - `email` (para alunos já cadastrados)
 */
export function parseCsvAlunos(bruto: string): CandidatoMatricula[] {
  const linhas = bruto
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);

  const saida: CandidatoMatricula[] = [];
  const vistos = new Set<string>();

  for (let i = 0; i < linhas.length; i++) {
    const linha = linhas[i];
    if (!linha) continue;
    const partes = linha
      .split(/[,;\t]/)
      .map(limparCampoCsv)
      .filter((p) => p.length > 0);
    if (partes.length === 0) continue;

    // Ignora linha de cabeçalho se a primeira coluna for "email"/"e-mail" sem "@"
    const p0 = partes[0] ?? "";
    if (i === 0 && !p0.includes("@") && CABECALHOS_EMAIL.has(p0.toLowerCase())) {
      continue;
    }

    // Localiza a coluna que contém "@" (padrão: primeira coluna)
    let idxEmail = partes.findIndex((p) => p.includes("@"));
    if (idxEmail === -1) idxEmail = 0;

    const email = (partes[idxEmail] ?? "").toLowerCase().trim();
    if (!email || vistos.has(email)) continue;
    vistos.add(email);

    const restantes = partes.filter((_, idx) => idx !== idxEmail);
    let first_name = "";
    let last_name = "";

    if (restantes.length === 1) {
      const tokens = (restantes[0] ?? "").split(/\s+/).filter(Boolean);
      first_name = tokens[0] ?? "";
      last_name = tokens.slice(1).join(" ");
    } else if (restantes.length >= 2) {
      first_name = restantes[0] ?? "";
      last_name = restantes.slice(1).join(" ");
    }

    saida.push({ email, first_name, last_name });
  }

  return saida;
}

const ROTULO_LINGUAGEM: Record<string, string> = {
  python: "Python",
  c: "C",
  cpp: "C++",
};

/**
 * Resume uma atividade do banco para exibição no grid de atribuição.
 */
export function resumirAtividade(
  atividade: AtividadeComPreview,
): ResumoAtividadeCard {
  const tarefas = atividade.tarefas ?? [];
  const totalTarefas = tarefas.length;
  const totalPontos = tarefas.reduce((acc, t) => acc + (t.valor_pts || 0), 0);

  const conj = new Set<string>();
  for (const t of tarefas) {
    if (t.tarefa_tipo === "arquivo_submissao") {
      conj.add("Arquivo");
    } else if (t.linguagem) {
      conj.add(ROTULO_LINGUAGEM[t.linguagem] ?? t.linguagem.toUpperCase());
    }
  }

  return {
    totalTarefas,
    totalPontos,
    etiquetas: Array.from(conj),
  };
}
