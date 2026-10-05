import type { Handle } from "@sveltejs/kit/hooks";

import { lerSessao } from "#lib/server/sessao";

// Toda requisição server-side revalida a sessão e os cargos na API
// (ARQUITETURA §4.2); páginas públicas ignoram locals.usuario.
export const handle: Handle = async ({ event, resolve }) => {
  event.locals.usuario = await lerSessao(event.request, event.cookies);
  return resolve(event);
};
