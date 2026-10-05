// BFF da redefinição: repassa {token, senha} à API
// (/v1/auth/password/reset), consumindo o token de uso único.
import { json, type RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";

export const POST: RequestHandler = async (event) => {
  const res = await apiFetch("/v1/auth/password/reset", event.request);
  const corpo = await res.text();
  return json(corpo.length ? JSON.parse(corpo) : {}, { status: res.status });
};
