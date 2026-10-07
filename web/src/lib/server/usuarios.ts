// Cliente server-side da gestão de usuários (/v1/users) para os
// +page.server.ts do portal admin (ARQUITETURA §5.4).
import { apiFetch } from "./api";
import { comBearer } from "./sessao";
import type { Cargo, Usuario } from "#lib/tipos";

export interface PaginaUsuarios {
  total: number;
  pagina: number;
  por: number;
  usuarios: Usuario[];
}

/** Extrai a mensagem do envelope único de erros da API. */
export function mensagemErro(
  corpo: unknown,
  padrao = "Erro inesperado na API.",
): string {
  const erro = (corpo as { erro?: { mensagem?: string } } | null)?.erro;
  return erro?.mensagem ?? padrao;
}

export async function listarUsuarios(
  request: Request,
  token: string,
  query: string,
): Promise<{ res: Response; body: PaginaUsuarios | unknown }> {
  const res = await apiFetch(`/v1/users${query}`, request, {
    method: "GET",
    headers: comBearer(token),
  });
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    // corpo não-JSON da API: mantém null
  }
  return { res, body };
}

/** Detalhe de um usuário: GET /v1/users/:id (inspeção). */
export async function obterUsuario(
  request: Request,
  token: string,
  id: number,
): Promise<{ res: Response; body: Usuario | unknown }> {
  const res = await apiFetch(`/v1/users/${id}`, request, {
    method: "GET",
    headers: comBearer(token),
  });
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    // corpo não-JSON da API: mantém null
  }
  return { res, body };
}

export async function criarUsuario(
  request: Request,
  token: string,
  cadastro: { email: string; first_name: string; last_name: string },
): Promise<Response> {
  return apiFetch("/v1/users", request, {
    method: "POST",
    headers: comBearer(token),
    body: JSON.stringify(cadastro),
  });
}

export async function editarUsuario(
  request: Request,
  token: string,
  id: number,
  dados: { first_name: string; last_name: string },
): Promise<Response> {
  return apiFetch(`/v1/users/${id}`, request, {
    method: "PATCH",
    headers: comBearer(token),
    body: JSON.stringify(dados),
  });
}

/** Substitui o conjunto de cargos: PUT /v1/users/:id/cargos. */
export async function alterarCargos(
  request: Request,
  token: string,
  id: number,
  cargos: Cargo[],
): Promise<Response> {
  return apiFetch(`/v1/users/${id}/cargos`, request, {
    method: "PUT",
    headers: comBearer(token),
    body: JSON.stringify({ cargos }),
  });
}

/** Ações por id: POST /v1/users/:id/{acao}. */
export async function acaoUsuario(
  request: Request,
  token: string,
  id: number,
  acao: "desativar" | "ativar" | "resetar-senha",
): Promise<Response> {
  return apiFetch(`/v1/users/${id}/${acao}`, request, {
    method: "POST",
    headers: comBearer(token),
    body: "{}",
  });
}
