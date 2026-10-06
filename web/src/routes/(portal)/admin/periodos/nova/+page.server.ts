import { fail, redirect, type Actions } from "@sveltejs/kit";

import { criarPeriodo, mensagemDaApi } from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ locals }) => {
  if (!locals.usuario) redirect(303, "/");
  return {};
};

export const actions: Actions = {
  criar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const trimestre = Number(form.get("trimestre"));
    const ano = Number(form.get("ano"));
    if (![1, 2, 3, 4].includes(trimestre) || ano < 2000 || ano > 2100) {
      return fail(422, {
        erro: "Período inválido (trimestre 1–4, ano entre 2000 e 2100).",
      });
    }
    const r = await criarPeriodo(evento.request, token, { trimestre, ano });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    redirect(303, "/admin/periodos");
  },
};
