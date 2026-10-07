// Cliente server-side do domínio de ensino (/v1/materias, /v1/periodos,
// /v1/turmas, /v1/matriculas, /v1/alunos) para os +page.server.ts dos
// portais professor/admin.
import { apiFetch } from "./api";
import { comBearer } from "./sessao";
import type { Usuario } from "#lib/tipos";

// Tipos espelhando a API (JSON em português).
export interface Materia {
  id: number;
  codigo: string;
  nome: string;
}

export interface Periodo {
  id: number;
  trimestre: number;
  ano: number;
}

export interface Professor {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
}

export interface Turma {
  id: number;
  materia: Materia;
  trimestre: number;
  ano: number;
  titulo?: string;
  professor: Professor;
  encerrado_em?: string | null;
  created_at: string;
}

export interface ListaTurmas {
  total: number;
  pagina: number;
  por: number;
  turmas: Turma[];
}

export interface Aluno {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
  status: string;
}

export interface Matricula {
  id: number;
  aluno: Aluno;
  entrada_em: string;
  saida_em?: string | null;
}

export interface ListaMatriculas {
  total: number;
  matriculas: Matricula[];
}

export interface Resultado {
  ok: boolean;
  status: number;
  body: unknown;
}

/** Extrai a mensagem do envelope único de erros da API. */
export function mensagemDaApi(
  body: unknown,
  padrao = "Erro inesperado na API.",
): string {
  const erro = (body as { erro?: { mensagem?: string } } | null)?.erro;
  return erro?.mensagem ?? padrao;
}

/** Lista de matérias de uma resposta. */
export function listaDeMaterias(r: Resultado): Materia[] {
  return (r.body as { materias?: Materia[] } | null)?.materias ?? [];
}

/** Lista de períodos de uma resposta. */
export function listaDePeriodos(r: Resultado): Periodo[] {
  return (r.body as { periodos?: Periodo[] } | null)?.periodos ?? [];
}

/** Listagem paginada de turmas de uma resposta. */
export function turmasDe(r: Resultado): ListaTurmas | null {
  return r.body as ListaTurmas | null;
}

// -----------------------------------------------------------------------

export async function listarTurmas(
  request: Request,
  token: string,
  filtros: {
    encerradas?: string;
    materia_id?: number;
    periodo_id?: number;
    pagina?: number;
  },
): Promise<Resultado> {
  const qs = new URLSearchParams();
  if (filtros.encerradas) qs.set("encerradas", filtros.encerradas);
  if (filtros.materia_id) qs.set("materia_id", String(filtros.materia_id));
  if (filtros.periodo_id) qs.set("periodo_id", String(filtros.periodo_id));
  if (filtros.pagina && filtros.pagina > 1)
    qs.set("pagina", String(filtros.pagina));
  return chamar(request, token, `/v1/turmas${param(qs)}`, { method: "GET" });
}

export async function criarTurma(
  request: Request,
  token: string,
  cad: { materia_id: number; periodo_id: number; titulo?: string },
): Promise<Resultado> {
  return chamar(request, token, "/v1/turmas", {
    method: "POST",
    body: JSON.stringify({
      materia_id: cad.materia_id,
      periodo_id: cad.periodo_id,
      titulo: cad.titulo ?? "",
    }),
  });
}

export async function verTurma(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/turmas/${id}`, { method: "GET" });
}

export async function renomearTurma(
  request: Request,
  token: string,
  id: number,
  titulo: string,
): Promise<Resultado> {
  return chamar(request, token, `/v1/turmas/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ titulo }),
  });
}

/** Ações por turma: encerrar (POST /encerrar) ou excluir (DELETE). */
export async function acaoTurma(
  request: Request,
  token: string,
  id: number,
  acao: "encerrar" | "excluir",
): Promise<Resultado> {
  if (acao === "excluir") {
    return chamar(request, token, `/v1/turmas/${id}`, {
      method: "DELETE",
      body: "{}",
    });
  }
  return chamar(request, token, `/v1/turmas/${id}/encerrar`, {
    method: "POST",
    body: "{}",
  });
}

export async function listarMatriculas(
  request: Request,
  token: string,
  turmaID: number,
  incluirEncerradas: boolean,
): Promise<Resultado> {
  return chamar(
    request,
    token,
    `/v1/turmas/${turmaID}/matriculas${incluirEncerradas ? "?incluir_encerradas=true" : ""}`,
    { method: "GET" },
  );
}

export async function matricular(
  request: Request,
  token: string,
  turmaID: number,
  cad: { email: string; first_name?: string; last_name?: string },
): Promise<Resultado> {
  return chamar(request, token, `/v1/turmas/${turmaID}/matriculas`, {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

export async function encerrarMatricula(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/matriculas/${id}`, {
    method: "DELETE",
    body: "{}",
  });
}

/** Busca de alunos (professor+staff): GET /v1/alunos?busca&pagina. */
export async function listarAlunos(
  request: Request,
  token: string,
  busca: string,
  pagina = 1,
): Promise<Resultado> {
  const qs = new URLSearchParams();
  if (busca) qs.set("busca", busca);
  if (pagina > 1) qs.set("pagina", String(pagina));
  return chamar(request, token, `/v1/alunos${param(qs)}`, { method: "GET" });
}

// --------------------------- tarefas -----------------------------------

export interface Tarefa {
  id: number;
  professor_id?: number;
  nome: string;
  enunciado?: string;
  tempo_cpu_ms?: number;
  tempo_total_ms?: number;
  memoria_mb?: number;
  created_at?: string;
}

export interface Teste {
  id: number;
  tarefa_id?: number;
  stdin: string;
  stdout_esperado: string;
  publico: boolean;
}

export async function listarTarefas(
  request: Request,
  token: string,
): Promise<Resultado> {
  return chamar(request, token, "/v1/tarefas", { method: "GET" });
}

/** Lista de tarefas de uma resposta. */
export function listaDeTarefas(r: Resultado): Tarefa[] {
  return (r.body as { tarefas?: Tarefa[] } | null)?.tarefas ?? [];
}

export async function criarTarefa(
  request: Request,
  token: string,
  cad: {
    nome: string;
    enunciado: string;
    tempo_cpu_ms?: number;
    tempo_total_ms?: number;
    memoria_mb?: number;
    testes?: { stdin: string; stdout_esperado: string; publico: boolean }[];
  },
): Promise<Resultado> {
  return chamar(request, token, "/v1/tarefas", {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

export async function verTarefaBruta(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/tarefas/${id}`, { method: "GET" });
}

/** Detalhe da tarefa: {tarefa, testes}. */
export function detalhesDoVerTarefa(
  r: Resultado,
): { tarefa: Tarefa | null; testes: Teste[] } {
  const corpo = r.body as { tarefa?: Tarefa; testes?: Teste[] } | null;
  return { tarefa: corpo?.tarefa ?? null, testes: corpo?.testes ?? [] };
}

export async function verTarefa(
  request: Request,
  token: string,
  id: number,
): Promise<{ tarefa: Tarefa | null; testes: Teste[] }> {
  return detalhesDoVerTarefa(await verTarefaBruta(request, token, id));
}

export async function editarTarefa(
  request: Request,
  token: string,
  id: number,
  cad: {
    nome: string;
    enunciado: string;
    tempo_cpu_ms?: number;
    tempo_total_ms?: number;
    memoria_mb?: number;
    testes?: { stdin: string; stdout_esperado: string; publico: boolean }[];
  },
): Promise<Resultado> {
  return chamar(request, token, `/v1/tarefas/${id}`, {
    method: "PATCH",
    body: JSON.stringify(cad),
  });
}

export async function duplicarTarefa(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/tarefas/${id}/duplicar`, {
    method: "POST",
    body: "{}",
  });
}

export async function excluirTarefa(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/tarefas/${id}`, {
    method: "DELETE",
    body: "{}",
  });
}

// --------------------------- atividades --------------------------------

export interface Atividade {
  id: number;
  professor_id?: number;
  nome: string;
  conteudo?: unknown;
  tarefas?: TarefaAtividade[];
  created_at?: string;
}

export interface TarefaAtividade {
  tarefa_id: number;
  ordem: number;
  valor_pts: number;
  linguagem: string;
  tarefa_nome?: string;
  tarefa_enunciado?: string;
  tarefa_excluida?: string;
}

export async function listarAtividades(
  request: Request,
  token: string,
): Promise<Resultado> {
  return chamar(request, token, "/v1/atividades", { method: "GET" });
}

/** Lista de atividades de uma resposta (sem tarefas). */
export function listaDeAtividades(r: Resultado): Atividade[] {
  return (r.body as { atividades?: Atividade[] } | null)?.atividades ?? [];
}

/** Monta o JSON pré-hidratação (schema_version 1) a partir das tarefas
 * escolhidas pelo professor: título grande + task_default por tarefa. */
export function montarConteudo(
  nome: string,
  tarefas: { id: number; valor_pts: number; linguagem: string }[],
): unknown {
  return {
    schema_version: 1,
    components: [
      { type: "large_text", text: nome },
      ...tarefas.map((t) => ({
        type: "task_default" as const,
        id: t.id,
        value_pts: t.valor_pts,
        language: t.linguagem,
      })),
    ],
  };
}

export async function criarAtividade(
  request: Request,
  token: string,
  cad: {
    nome: string;
    conteudo: unknown;
    tarefas: {
      tarefa_id: number;
      ordem: number;
      valor_pts: number;
      linguagem: string;
    }[];
  },
): Promise<Resultado> {
  return chamar(request, token, "/v1/atividades", {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

export async function editarAtividade(
  request: Request,
  token: string,
  id: number,
  cad: {
    nome: string;
    conteudo: unknown;
    tarefas: {
      tarefa_id: number;
      ordem: number;
      valor_pts: number;
      linguagem: string;
    }[];
  },
): Promise<Resultado> {
  return chamar(request, token, `/v1/atividades/${id}`, {
    method: "PATCH",
    body: JSON.stringify(cad),
  });
}

export async function duplicarAtividade(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atividades/${id}/duplicar`, {
    method: "POST",
    body: "{}",
  });
}

/**
 * Ver detalhado (professor+staff): corpo já decodificado em
 * {atividade, tarefas}.
 */
export async function verAtividadePorId(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atividades/${id}`, { method: "GET" });
}

export async function excluirAtividade(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atividades/${id}`, {
    method: "DELETE",
    body: "{}",
  });
}

// --------------------------- atribuições -------------------------------

export interface Atribuicao {
  id: number;
  turma_id: number;
  atividade_id: number;
  autocomplete: boolean;
  inicio?: string | null;
  prazo?: string | null;
  duracao_seg?: number | null;
  pode_atrasado: boolean;
  turma_titulo?: string;
  atividade_nome?: string;
}

export async function listarAtribuicoes(
  request: Request,
  token: string,
  filtro: { turma_id?: number; atividade_id?: number },
): Promise<Resultado> {
  const qs = new URLSearchParams();
  if (filtro.turma_id) qs.set("turma_id", String(filtro.turma_id));
  if (filtro.atividade_id) qs.set("atividade_id", String(filtro.atividade_id));
  return chamar(request, token, `/v1/atribuicoes${param(qs)}`, { method: "GET" });
}

export async function criarAtribuicao(
  request: Request,
  token: string,
  cad: {
    turma_id: number;
    atividade_id: number;
    autocomplete?: boolean;
    inicio?: string | null;
    prazo?: string | null;
    duracao_seg?: number | null;
    pode_atrasado?: boolean;
  },
): Promise<Resultado> {
  return chamar(request, token, "/v1/atribuicoes", {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

export async function verAtribuicao(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${id}`, { method: "GET" });
}

export async function editarAtribuicao(
  request: Request,
  token: string,
  id: number,
  cad: {
    autocomplete?: boolean;
    inicio?: string | null;
    prazo?: string | null;
    duracao_seg?: number | null;
    pode_atrasado?: boolean;
  },
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${id}`, {
    method: "PATCH",
    body: JSON.stringify(cad),
  });
}

export async function excluirAtribuicao(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${id}`, {
    method: "DELETE",
    body: "{}",
  });
}

// --------------------- tentativa (fluxo do aluno) ----------------------

export interface VisaoAluno {
  atribuicao: {
    id: number;
    prazo?: string | null;
    pode_atrasado: boolean;
  };
  atividade: {
    id: number;
    nome: string;
    tarefas: {
      id: number;
      ordem: number;
      valor_pts: number;
      linguagem: string;
      nome: string;
      enunciado: string;
      excluida: boolean;
    }[];
  };
  tentativa?: { revisao: number; snapshot: { tasks?: { id: number; text: string }[] } };
}

export async function visaoAluno(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${id}/aluno`, {
    method: "GET",
  });
}

// ---------------------- portal aluno (v1/me) ---------------------------

export interface TurmaDoAluno {
  id: number;
  titulo?: string;
  materia_codigo: string;
  materia_nome: string;
  trimestre: number;
  ano: number;
  professor: string;
  professor_email: string;
  ativa: boolean;
  created_at: string;
}

export interface AtribuicaoDoAluno {
  id: number;
  turma_id: number;
  atividade_id: number;
  autocomplete: boolean;
  inicio?: string;
  prazo?: string;
  duracao_seg?: number;
  pode_atrasado: boolean;
  materia_codigo: string;
  turma_titulo?: string;
  turma_ativa: boolean;
  atividade_nome: string;
  em_andamento: boolean;
  minha_revisao?: number;
  gravado_em?: string;
  entregue: boolean;
  entregue_em?: string;
  atrasada: boolean;
}

export async function listarMinhasTurmas(
  request: Request,
  token: string,
): Promise<Resultado> {
  return chamar(request, token, "/v1/me/turmas", { method: "GET" });
}

export async function listarMinhasAtribuicoes(
  request: Request,
  token: string,
  turmaID?: number,
): Promise<Resultado> {
  const qs = new URLSearchParams();
  if (turmaID) qs.set("turma_id", String(turmaID));
  return chamar(request, token, `/v1/me/atribuicoes${param(qs)}`, {
    method: "GET",
  });
}

export async function gravarTentativa(
  request: Request,
  token: string,
  id: number,
  grav: { revisao: number; snapshot: { tasks: { id: number; text: string }[] } },
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${id}/tentativa`, {
    method: "PUT",
    body: JSON.stringify(grav),
  });
}

/** Entrega final (PROJETO §Submissão): snapshot corrente da tentativa. */
export async function entregarAtividade(
  request: Request,
  token: string,
  id: number,
  observacao?: string,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${id}/submissao`, {
    method: "POST",
    body: JSON.stringify({ observacao: observacao ?? "" }),
  });
}

export interface Entrega {
  id: number;
  aluno: string;
  aluno_email: string;
  comecou_em: string;
  entregue_em: string;
  atrasada: boolean;
}

/** Entregas da atribuição (professor dono|staff). */
export async function listarEntregas(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${id}/submissoes`, {
    method: "GET",
  });
}

export function entregasDe(r: Resultado): Entrega[] {
  return (r.body as { submissoes?: Entrega[] } | null)?.submissoes ?? [];
}

// ------------------------- correção ------------------------------------

export interface ResultadoTeste {
  teste_id: number;
  status?: string;
  stdout?: string;
  stderr?: string;
  compile_output?: string;
  tempo?: string;
  memoria?: string;
  cpu?: string;
  publico: boolean;
}

export interface CorrecaoTarefa {
  id: number;
  tarefa_id: number;
  tarefa_nome: string;
  valor_pts: number;
  nota_auto?: number;
  nota_final?: number;
  feedback?: string;
  resultados: ResultadoTeste[];
}

export interface Correcao {
  id: number;
  submissao_id: number;
  status: string;
  feedback?: string;
  confirmada_em?: string;
  publicada_em?: string;
  tarefas: CorrecaoTarefa[];
}

export interface VisaoCorrecaoProfessor {
  submissao: {
    id: number;
    aluno: string;
    aluno_email: string;
    atividade_nome: string;
    entregue_em: string;
    atrasada: boolean;
  };
  correcao: Correcao | null;
}

export async function verCorrecao(
  request: Request,
  token: string,
  submissaoID: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/submissoes/${submissaoID}/correcao`, {
    method: "GET",
  });
}

export async function editarCorrecao(
  request: Request,
  token: string,
  submissaoID: number,
  cad: {
    feedback?: string;
    tarefas?: { id: number; nota_final?: number; feedback?: string }[];
  },
): Promise<Resultado> {
  return chamar(request, token, `/v1/submissoes/${submissaoID}/correcao`, {
    method: "PATCH",
    body: JSON.stringify(cad),
  });
}

/** Ações: confirmar | publicar. */
export async function acaoCorrecao(
  request: Request,
  token: string,
  submissaoID: number,
  acao: "confirmar" | "publicar",
): Promise<Resultado> {
  return chamar(
    request,
    token,
    `/v1/submissoes/${submissaoID}/correcao/${acao}`,
    { method: "POST", body: "{}" },
  );
}

// ------------------ Execução de teste (§ modalidade) -------------------

export interface ResultadoExec {
  teste_id: number;
  publico: boolean;
  status?: string;
  stdout?: string;
  stderr?: string;
  compile_output?: string;
  tempo?: string;
  memoria?: string;
  cpu?: string;
}

/** Aluno testa (§Execução de teste): código do editor, testes públicos. */
export async function testarTarefa(
  request: Request,
  token: string,
  atribuicaoID: number,
  cad: { tarefa_id: number; codigo: string },
): Promise<Resultado> {
  return chamar(request, token, `/v1/atribuicoes/${atribuicaoID}/testar`, {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

/** Histórico do aluno nos últimos 7 dias (todas ou por atribuição). */
export async function historicoTestes(
  request: Request,
  token: string,
  atribuicaoID?: number,
): Promise<Resultado> {
  const qs = new URLSearchParams();
  if (atribuicaoID) qs.set("atribuicao_id", String(atribuicaoID));
  return chamar(request, token, `/v1/me/testes${param(qs)}`, { method: "GET" });
}

export interface Execucao {
  id: number;
  atribuicao_id: number;
  tarefa_id: number;
  atividade_nome?: string;
  linguagem: string;
  resultados?: ResultadoExec[];
  criado_em: string;
}

export function execucoesDe(r: Resultado): Execucao[] {
  return (r.body as { execucoes?: Execucao[] } | null)?.execucoes ?? [];
}

/** Professor: testar no banco (linguagem livre, sem histórico). */
export async function testarTarefaProfessor(
  request: Request,
  token: string,
  tarefaID: number,
  cad: { linguagem: string; codigo: string },
): Promise<Resultado> {
  return chamar(request, token, `/v1/tarefas/${tarefaID}/testar`, {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

// ------------------------- matérias e períodos -------------------------

export async function listarMaterias(
  request: Request,
  token: string,
): Promise<Resultado> {
  return chamar(request, token, "/v1/materias", { method: "GET" });
}

export async function criarMateria(
  request: Request,
  token: string,
  cad: { codigo: string; nome: string },
): Promise<Resultado> {
  return chamar(request, token, "/v1/materias", {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

export async function editarMateria(
  request: Request,
  token: string,
  id: number,
  cad: { codigo: string; nome: string },
): Promise<Resultado> {
  return chamar(request, token, `/v1/materias/${id}`, {
    method: "PATCH",
    body: JSON.stringify(cad),
  });
}

export async function excluirMateria(
  request: Request,
  token: string,
  id: number,
): Promise<Resultado> {
  return chamar(request, token, `/v1/materias/${id}`, {
    method: "DELETE",
    body: "{}",
  });
}

export async function listarPeriodos(
  request: Request,
  token: string,
): Promise<Resultado> {
  return chamar(request, token, "/v1/periodos", { method: "GET" });
}

export async function criarPeriodo(
  request: Request,
  token: string,
  cad: { trimestre: number; ano: number },
): Promise<Resultado> {
  return chamar(request, token, "/v1/periodos", {
    method: "POST",
    body: JSON.stringify(cad),
  });
}

/** Ações de período: excluir. */
export async function acaoPeriodo(
  request: Request,
  token: string,
  id: number,
  _acao: "excluir",
): Promise<Resultado> {
  return chamar(request, token, `/v1/periodos/${id}`, {
    method: "DELETE",
    body: "{}",
  });
}

// -----------------------------------------------------------------------

function param(qs: URLSearchParams): string {
  return qs.size ? `?${qs.toString()}` : "";
}

/** Chamada comum: tolerante a falha de rede (ver Resultado). */
async function chamar(
  request: Request,
  token: string,
  path: string,
  init: { method: string; body?: string },
): Promise<Resultado> {
  let res: Response;
  try {
    res = await apiFetch(path, request, {
      method: init.method,
      headers: comBearer(token),
      ...(init.body !== undefined ? { body: init.body } : {}),
    });
  } catch {
    return { ok: false, status: 502, body: null };
  }
  let body: unknown = null;
  if (res.status !== 204) {
    try {
      body = await res.json();
    } catch {
      body = null;
    }
  }
  return { ok: res.ok, status: res.status, body };
}

/** Guarda de tipos reexportada para o portal professor. */
export type { Usuario };
