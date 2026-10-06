import { fail, redirect, type Actions } from "@sveltejs/kit";

import {
  acaoPeriodo,
  listaDePeriodos,
  listarPeriodos,
  mensagemDaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Lista de períodos letivos (staff): criar via /admin/periodos/nova;
// excluir direto (em uso → 409); consultar turmas por período.
export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const r = await listarPeriodos(request, token);
    if (r.status === 401) redirect(303, "/");
    return {
      periodos: r.ok ? listaDePeriodos(r) : [],
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { periodos: [], erro: "Sem comunicação com a API." };
  }
};

export const actions: Actions = {
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number((await evento.request.formData()).get("id"));
    if (!Number.isInteger(id) || id < 1) {
      return fail(422, { erro: "Identificador inválido." });
    }
    const r = await acaoPeriodo(evento.request, token, id, "excluir");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Período removido." };
  },
};
