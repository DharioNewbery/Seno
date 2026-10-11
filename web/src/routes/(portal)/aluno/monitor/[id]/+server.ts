// BFF de entrada no Monitor: pede o `st` na API e redireciona (Fase 9.4).
// Integração desligada ou qualquer erro → volta ao editor atual (transição
// suave; nada o substitui).
import { redirect } from "@sveltejs/kit";
import type { RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";
import { comBearer, tokenDaSessao } from "#lib/server/sessao";

export const GET: RequestHandler = async (event) => {
  const { params, cookies, request } = event;
  const editor = `/aluno/atividade/${params.id}`; // editor atual (fallback)
  const token = tokenDaSessao(cookies);
  if (!token) throw redirect(302, "/login");

  try {
    const res = await apiFetch(`/v1/atribuicoes/${params.id}/entrar`, request, {
      method: "GET",
      headers: comBearer(token),
    });
    if (!res.ok) throw new Error(`entrar: status ${res.status}`);
    const body = (await res.json()) as { url?: string };
    if (!body.url) throw new Error("entrar: sem url");
    throw redirect(302, body.url);
  } catch (e) {
    // redirect() também chega como exceção — deixar passar.
    if (e && typeof e === "object" && "location" in e) throw e;
    throw redirect(302, editor);
  }
};
