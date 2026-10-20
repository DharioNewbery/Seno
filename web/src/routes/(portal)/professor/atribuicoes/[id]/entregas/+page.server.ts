import {
  coletarRascunho,
  entregasDe,
  excluirMonitor,
  excluirSubmissao,
  listarEntregas,
  mensagemDaApi,
  publicarCorrecoesLote,
  reexecutarMonitor,
  verAtribuicao,
  type Atribuicao,
} from "#lib/server/ensino";
import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/professor/turmas");
  try {
    const ra = await verAtribuicao(request, token, id);
    if (ra.status === 401) redirect(303, "/");
    if (ra.status === 404) redirect(303, "/professor/turmas");
    const re = await listarEntregas(request, token, id);
    if (re.status === 401) redirect(303, "/");
    return {
      atribuicao: (ra.body as Atribuicao | null) ?? null,
      entregas: re.ok ? entregasDe(re) : [],
      erro: re.ok ? null : mensagemDaApi(re.body),
    };
  } catch {
    return {
      atribuicao: null,
      entregas: [],
      erro: "Sem comunicação com a API.",
    };
  }
};

export const actions: Actions = {
  coletar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const atribuicaoID = Number(evento.params.id);
    const form = await evento.request.formData();
    const alunoID = Number(form.get("aluno_id"));
    if (!alunoID) return fail(422, { erro: "Aluno inválido." });
    const r = await coletarRascunho(evento.request, token, atribuicaoID, alunoID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Rascunho coletado com sucesso." };
  },
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const atribuicaoID = Number(evento.params.id);
    const form = await evento.request.formData();
    const submissaoID = Number(form.get("submissao_id"));
    const alunoID = Number(form.get("aluno_id"));
    const doMonitor = form.get("do_monitor") === "true";

    let r;
    if (doMonitor && alunoID) {
      r = await excluirMonitor(evento.request, token, atribuicaoID, alunoID);
    } else if (submissaoID) {
      r = await excluirSubmissao(evento.request, token, submissaoID);
    } else {
      return fail(422, { erro: "Submissão inválida para exclusão." });
    }

    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Submissão excluída. Uma nova tentativa pré-preenchida foi liberada ao aluno." };
  },
  reexecutar_monitor: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const atribuicaoID = Number(evento.params.id);
    const form = await evento.request.formData();
    const alunoID = Number(form.get("aluno_id"));
    if (!alunoID) return fail(422, { erro: "Aluno inválido." });
    const r = await reexecutarMonitor(evento.request, token, atribuicaoID, alunoID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Correção reenviada para execução no Monitor." };
  },
  publicar_tudo: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const atribuicaoID = Number(evento.params.id);
    const r = await publicarCorrecoesLote(evento.request, token, atribuicaoID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    const n = (r.body as { publicadas?: number } | null)?.publicadas ?? 0;
    if (n === 0) {
      return { ok: "Nada a publicar: nenhuma correção confirmada pendente." };
    }
    return { ok: `${n} correção(ões) publicada(s) para os alunos.` };
  },
};

