// BFF do pedido de recuperação: repassa {email} à API
// (/v1/auth/password/reset-request); resposta é sempre silenciosa.
import { json, type RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";

export const POST: RequestHandler = async (event) => {
  const res = await apiFetch("/v1/auth/password/reset-request", event.request);
  const corpo = await res.text();
  return json(corpo.length ? JSON.parse(corpo) : {}, { status: res.status });
};
