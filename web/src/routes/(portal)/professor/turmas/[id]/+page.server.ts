import { error, fail, redirect, type Actions } from "@sveltejs/kit";

import {
  acaoTurma,
  mensagemDaApi,
  renomearTurma,
  verTurma,
  type Turma as TurmaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Painel de controle da turma: dados + atalhos; escopo no serviço
// (professor dono; admin/super; outsider → 404 → tela de erro do Kit).
export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  params,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) {
    error(404, "Turma não encontrada.");
  }
  const token = tokenDaSessao(cookies);
  try {
    const r = await verTurma(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) error(404, "Turma não encontrada.");
    if (!r.ok) {
      return { turma: null, erro: mensagemDaApi(r.body) };
    }
    return { turma: r.body as TurmaApi, erro: null };
  } catch (e) {
    if (e && (e as { status?: number }).status) throw e;
    return { turma: null, erro: "Sem comunicação com a API." };
  }
};

export const actions: Actions = {
  renomear: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const titulo = String(
      (await evento.request.formData()).get("titulo") ?? "",
    ).trim();
    const r = await renomearTurma(evento.request, token, id, titulo);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Título atualizado." };
  },

  encerrar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const r = await acaoTurma(evento.request, token, id, "encerrar");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Turma encerrada." };
  },

  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const r = await acaoTurma(evento.request, token, id, "excluir");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    redirect(303, "/professor/turmas");
  },
};
