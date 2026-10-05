// BFF de logout: encerra a sessão na API e apaga o cookie local.
import { type RequestHandler } from "@sveltejs/kit";

import { fecharSessao } from "#lib/server/sessao";

export const POST: RequestHandler = async (event) => {
  await fecharSessao(event);
  return new Response(null, { status: 204 });
};
