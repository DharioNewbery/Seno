import { fail, redirect, type Actions } from "@sveltejs/kit";

import {
  criarTurma,
  listaDeMaterias,
  listarMaterias,
  mensagemDaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Materiais para o select; período é informado direto (a API cria o
// período letivo na primeira turma do trimestre/ano — FindOrCreate).
export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const r = await listarMaterias(request, token);
    if (r.status === 401) redirect(303, "/");
    return {
      materias: r.ok ? listaDeMaterias(r) : [],
      erroMaterias: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { materias: [], erroMaterias: "Sem comunicação com a API." };
  }
};

export const actions: Actions = {
  criar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const materia_id = Number(form.get("materia_id"));
    const trimestre = Number(form.get("trimestre"));
    const ano = Number(form.get("ano"));
    const titulo = String(form.get("titulo") ?? "").trim();
    if (!Number.isInteger(materia_id) || materia_id < 1) {
      return fail(422, { erro: "Selecione uma matéria." });
    }
    if (![1, 2, 3].includes(trimestre) || ano < 2000 || ano > 2100) {
      return fail(422, {
        erro: "Período inválido (trimestre 1–3, ano entre 2000 e 2100).",
      });
    }
    const r = await criarTurma(evento.request, token, {
      materia_id,
      trimestre,
      ano,
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
