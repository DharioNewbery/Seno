import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import {
  acaoCorrecao,
  editarCorrecao,
  mensagemDaApi,
  verCorrecao,
  type VisaoCorrecaoProfessor,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/professor/turmas");
  try {
    const r = await verCorrecao(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) redirect(303, "/professor/turmas");
    return {
      submissaoID: id,
      visao: (r.body as VisaoCorrecaoProfessor | null) ?? null,
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { submissaoID: id, visao: null, erro: "Sem comunicação com a API." };
  }
};

async function extrairEdicao(request: Request) {
  const form = await request.formData();
  const feedback = String(form.get("feedback") ?? "").trim();
  const tarefas: { id: number; nota_final?: number; feedback?: string }[] = [];
  for (const raw of form.getAll("ct_ids")) {
    const id2 = Number(raw);
    if (!Number.isInteger(id2) || id2 < 1) continue;
    const telaNota = String(form.get(`nota_${id2}`) ?? "").trim();
    const fb = String(form.get(`fb_${id2}`) ?? "").trim();
    const item: { id: number; nota_final?: number; feedback?: string } = { id: id2 };
    if (telaNota !== "") {
      const n = Number(telaNota);
      if (!Number.isFinite(n) || n < 0) {
        return { erro: "Nota inválida." };
      }
      item.nota_final = Math.floor(n);
    }
    if (fb !== "") item.feedback = fb;
    tarefas.push(item);
  }
  return {
    dados: {
      ...(feedback !== "" ? { feedback } : {}),
      ...(tarefas.length > 0 ? { tarefas } : {}),
    },
    temCampos: form.has("feedback") || form.has("ct_ids"),
  };
}

export const actions: Actions = {
  salvar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const parse = await extrairEdicao(evento.request);
    if ("erro" in parse && parse.erro) return fail(422, { erro: parse.erro });
    const r = await editarCorrecao(evento.request, token, id, parse.dados ?? {});
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Correção salva com sucesso." };
  },
  confirmar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const parse = await extrairEdicao(evento.request);
    if ("erro" in parse && parse.erro) return fail(422, { erro: parse.erro });
    if (parse.temCampos) {
      const rEdit = await editarCorrecao(evento.request, token, id, parse.dados ?? {});
      if (rEdit.status === 401) redirect(303, "/");
      if (!rEdit.ok) return fail(rEdit.status || 500, { erro: mensagemDaApi(rEdit.body) });
    }
    const r = await acaoCorrecao(evento.request, token, id, "confirmar");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Correção validada e confirmada com sucesso." };
  },
  reexecutar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const r = await acaoCorrecao(evento.request, token, Number(evento.params.id), "reexecutar");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Correção reenviada para execução automática." };
  },
};

