import { fail, redirect, type Actions } from "@sveltejs/kit";

import { criarMateria, mensagemDaApi } from "#lib/server/ensino";
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
    const codigo = String(form.get("codigo") ?? "").trim();
    const nome = String(form.get("nome") ?? "").trim();
    if (!codigo || !nome) {
      return fail(422, { erro: "Informe código e nome." });
    }
    const r = await criarMateria(evento.request, token, { codigo, nome });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    redirect(303, "/admin/materias");
  },
};
