import { fail, redirect, type Actions } from "@sveltejs/kit";

import {
  criarTurma,
  listaDeMaterias,
  listaDePeriodos,
  listarMaterias,
  listarPeriodos,
  mensagemDaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Matérias e períodos para os selects; a turma referencia um período letivo
// cadastrado pelo admin (nenhum trimestre/ano é digitado à mão).
export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const [rm, rp] = await Promise.all([
      listarMaterias(request, token),
      listarPeriodos(request, token),
    ]);
    if (rm.status === 401 || rp.status === 401) redirect(303, "/");
    return {
      materias: rm.ok ? listaDeMaterias(rm) : [],
      periodos: rp.ok ? listaDePeriodos(rp) : [],
      erroMaterias: rm.ok ? null : mensagemDaApi(rm.body),
      erroPeriodos: rp.ok ? null : mensagemDaApi(rp.body),
    };
  } catch {
    return {
      materias: [],
      periodos: [],
      erroMaterias: "Sem comunicação com a API.",
      erroPeriodos: null,
    };
  }
};

export const actions: Actions = {
  criar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const materia_id = Number(form.get("materia_id"));
    const periodo_id = Number(form.get("periodo_id"));
    const titulo = String(form.get("titulo") ?? "").trim();
    if (!Number.isInteger(materia_id) || materia_id < 1) {
      return fail(422, { erro: "Selecione uma matéria." });
    }
    if (!Number.isInteger(periodo_id) || periodo_id < 1) {
      return fail(422, { erro: "Selecione o período letivo." });
    }
    const r = await criarTurma(evento.request, token, {
      materia_id,
      periodo_id,
      titulo,
    });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    redirect(
      303,
      `/professor/turmas/${(r.body as { id?: number } | null)?.id}`,
    );
  },
};
