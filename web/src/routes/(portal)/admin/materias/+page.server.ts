import { fail, redirect, type Actions } from "@sveltejs/kit";

import {
  editarMateria,
  excluirMateria,
  listarMaterias,
  listaDeMaterias,
  mensagemDaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Lista de matérias (staff) com edição/exclusão inline; “ver turmas” leva
// a /admin/turmas?materia_id=id.
export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const r = await listarMaterias(request, token);
    if (r.status === 401) redirect(303, "/");
    return {
      materias: r.ok ? listaDeMaterias(r) : [],
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { materias: [], erro: "Sem comunicação com a API." };
  }
};

export const actions: Actions = {
  editar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("id"));
    const codigo = String(form.get("codigo") ?? "").trim();
    const nome = String(form.get("nome") ?? "").trim();
    if (!Number.isInteger(id) || id < 1) {
      return fail(422, { erro: "Identificador inválido." });
    }
    if (!codigo || !nome) {
      return fail(422, { erro: "Informe código e nome." });
    }
    const r = await editarMateria(evento.request, token, id, { codigo, nome });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: `Matéria ${codigo} atualizada.` };
  },

  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number((await evento.request.formData()).get("id"));
    if (!Number.isInteger(id) || id < 1) {
      return fail(422, { erro: "Identificador inválido." });
    }
    const r = await excluirMateria(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Matéria removida (soft delete)." };
  },
};
