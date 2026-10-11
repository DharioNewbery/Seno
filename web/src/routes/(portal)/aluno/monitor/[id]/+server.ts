// BFF de entrada no Monitor: pede o `st` na API e redireciona (Fase 9.4).
// NOTE: redirect() do SvelteKit v2 recusa URL externa
// (redirect_external_not_allowed) — o Monitor é outra origem, então o
// 302 é construído manualmente. Qualquer falha → editor atual (transição
// suave; nada o substitui).
import type { RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";
import { comBearer, tokenDaSessao } from "#lib/server/sessao";

function para(location: string): Response {
  return new Response(null, { status: 302, headers: { Location: location } });
}

export const GET: RequestHandler = async (event) => {
  const { params, cookies, request } = event;
  const token = tokenDaSessao(cookies);
  if (!token) return para("/login");

  const editor = `/aluno/atividade/${params.id}`; // editor atual (fallback)
  try {
    const res = await apiFetch(`/v1/atribuicoes/${params.id}/entrar`, request, {
      method: "GET",
      headers: comBearer(token),
    });
    if (!res.ok) return para(editor);
    const body = (await res.json()) as { url?: string };
    if (!body.url) return para(editor);
    return para(body.url);
  } catch {
    // API fora do ar: fallback (transição suave).
    return para(editor);
  }
};
