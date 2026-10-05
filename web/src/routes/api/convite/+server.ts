// BFF do aceite de convite: repassa {token, senha} à API
// (/v1/auth/invite/accept); usuário pendente ativa a conta aqui.
import { json, type RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";

export const POST: RequestHandler = async (event) => {
  const res = await apiFetch("/v1/auth/invite/accept", event.request);
  const corpo = await res.text();
  return json(corpo.length ? JSON.parse(corpo) : {}, { status: res.status });
};
