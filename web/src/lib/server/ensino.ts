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
