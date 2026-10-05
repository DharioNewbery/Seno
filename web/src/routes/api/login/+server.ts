// BFF de login: repassa credenciais à API e guarda o token no cookie
// HttpOnly seno_session; o navegador nunca vê o token (§4.1/§5.2).
import { json, type RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";
import {
  COOKIE_SESSAO,
  opcoesCookie,
  type RespostaLogin,
} from "#lib/server/sessao";

export const POST: RequestHandler = async (event) => {
  const res = await apiFetch("/v1/auth/login", event.request);

  if (res.ok) {
    const login = (await res.json()) as RespostaLogin;
    event.cookies.set(COOKIE_SESSAO, login.token, opcoesCookie());
    return json({ pessoa: login.pessoa, cargos: login.cargos });
  }

  const corpo = await res.text();
  return json(corpo.length ? JSON.parse(corpo) : {}, {
    status: res.status,
  });
};
