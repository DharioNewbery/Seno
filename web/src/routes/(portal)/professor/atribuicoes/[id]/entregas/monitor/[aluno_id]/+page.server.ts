import {
  excluirMonitor,
  mensagemDaApi,
  obterSubmissaoMonitor,
  reexecutarMonitor,
  verAtribuicao,
  type Atribuicao,
  type DetalhesSubmissaoMonitor,
} from "#lib/server/ensino";
import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const atribuicaoID = Number(params.id);
  const alunoID = Number(params.aluno_id);
  if (!Number.isInteger(atribuicaoID) || atribuicaoID < 1) redirect(303, "/professor/turmas");
  if (!Number.isInteger(alunoID) || alunoID < 1) redirect(303, `/professor/atribuicoes/${atribuicaoID}/entregas`);

  try {
    const ra = await verAtribuicao(request, token, atribuicaoID);
    if (ra.status === 401) redirect(303, "/");
    if (ra.status === 404) redirect(303, "/professor/turmas");

    const rd = await obterSubmissaoMonitor(request, token, atribuicaoID, alunoID);
    if (rd.status === 401) redirect(303, "/");

    return {
      atribuicao: (ra.body as Atribuicao | null) ?? null,
      detalhes: rd.ok ? (rd.body as DetalhesSubmissaoMonitor) : null,
      alunoID,
      erro: rd.ok ? null : mensagemDaApi(rd.body),
    };
  } catch {
    return {
      atribuicao: null,
      detalhes: null,
      alunoID,
      erro: "Sem comunicação com a API.",
    };
  }
};

export const actions: Actions = {
  reexecutar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const atribuicaoID = Number(evento.params.id);
    const alunoID = Number(evento.params.aluno_id);
    const r = await reexecutarMonitor(evento.request, token, atribuicaoID, alunoID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Correção reenviada para execução no Monitor." };
  },
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const atribuicaoID = Number(evento.params.id);
    const alunoID = Number(evento.params.aluno_id);
    const r = await excluirMonitor(evento.request, token, atribuicaoID, alunoID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    redirect(303, `/professor/atribuicoes/${atribuicaoID}/entregas`);
  },
};
