// Sessão do BFF: o token opaco vive apenas no cookie seno_session e é
// convertido em Authorization: Bearer nas chamadas à API (ARQUITETURA §4).
import { dev } from "$app/env";
import type { Cookies, RequestEvent } from "@sveltejs/kit";

import { apiFetch } from "./api";
import type { Usuario } from "#lib/tipos";

export const COOKIE_SESSAO = "seno_session";

/** Tempo de vida do cookie: teto da sessão na API (7 dias, §5.2). */
const COOKIE_MAX_AGE = 7 * 24 * 60 * 60;

export function opcoesCookie() {
  return {
    path: "/",
    httpOnly: true,
    sameSite: "lax" as const,
    secure: !dev,
    maxAge: COOKIE_MAX_AGE,
  };
}

export function comBearer(token: string): Headers {
  const headers = new Headers();
  headers.set("Authorization", `Bearer ${token}`);
  return headers;
}

/** Resposta de login da API: {token, pessoa, cargos} (ARQUITETURA §5.2). */
export interface RespostaLogin {
  token: string;
  pessoa: { id: number; email: string; first_name: string; last_name: string };
  cargos: string[];
}

/**
 * Valida a sessão do navegador contra a API (revalida sessão e cargos no
 * banco em cada requisição). Cookie inválido é removido; API fora do ar é
 * tratada como ausência de sessão para não derrubar páginas públicas.
 */
export async function lerSessao(
  request: Request,
  cookies: Cookies,
): Promise<Usuario | null> {
  const token = cookies.get(COOKIE_SESSAO);
  if (!token) return null;

  let res: Response;
  try {
    res = await apiFetch("/v1/auth/me", request, { headers: comBearer(token) });
  } catch {
    return null;
  }
  if (res.status === 401) {
    cookies.delete(COOKIE_SESSAO, { path: "/" });
    return null;
  }
  if (!res.ok) return null;
  return (await res.json()) as Usuario;
}

/** Encerra a sessão na API (logout) e apaga o cookie do navegador. */
export async function fecharSessao(event: RequestEvent): Promise<void> {
  const token = event.cookies.get(COOKIE_SESSAO);
  if (token) {
    try {
      await apiFetch("/v1/auth/logout", event.request, {
        headers: comBearer(token),
      });
    } catch {
      // API indisponível: o cookie local é removido mesmo assim.
    }
    event.cookies.delete(COOKIE_SESSAO, { path: "/" });
  }
}
