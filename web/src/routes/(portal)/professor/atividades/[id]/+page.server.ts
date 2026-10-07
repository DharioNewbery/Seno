import {
  duplicarAtividade,
  editarAtividade,
  excluirAtividade,
  listarTarefas,
  mensagemDaApi,
  montarConteudo,
  verAtividadePorId,
} from "#lib/server/ensino";
import {
  fail,
  redirect,
  type Actions,
} from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import { tokenDaSessao } from "#lib/server/sessao";

interface Atributo {
  tarefa_id: number;
  valor_pts: number;
  linguagem: string;
}

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/professor/atividades");
  try {
    const ra = await verAtividadePorId(request, token, id);
    if (ra.status === 401) redirect(303, "/");
    if (ra.status === 404) redirect(303, "/professor/atividades");
    const rt = await listarTarefas(request, token);
    if (rt.status === 401) redirect(303, "/");
    return {
      id,
      atividade: (ra.body as { nome?: string; tarefas?: unknown[] } | null) ?? null,
      tarefas: (rt.body as { tarefas?: { id: number; nome: string }[] } | null)
        ?.tarefas ?? [],
      erro: ra.ok ? null : mensagemDaApi(ra.body),
    };
  } catch {
    redirect(303, "/professor/atividades");
  }
};

export const actions: Actions = {
  salvar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const form = await evento.request.formData();
    const nome = String(form.get("nome") ?? "").trim();
    if (!nome) return fail(422, { erro: "Informe o nome da atividade." });
    let atributos: Atributo[] = [];
    try {
      atributos = JSON.parse(String(form.get("tarefas") ?? "[]"));
    } catch {
      return fail(422, { erro: "Lista de tarefas inválida." });
    }
    if (
      !Array.isArray(atributos) ||
      atributos.length === 0 ||
      !atributos.every(
        (a) =>
          Number.isInteger(a?.tarefa_id) &&
          Number.isInteger(a?.valor_pts) &&
          ["python", "c", "cpp"].includes(a?.linguagem),
      )
    ) {
      return fail(422, { erro: "Tarefas inválidas: verifique linguagem/pontos." });
    }
    const ordemAtributos = atributos.map((a, i) => ({ ...a, ordem: i + 1 }));
    const r = await editarAtividade(evento.request, token, id, {
      nome,
      conteudo: montarConteudo(
        nome,
        atributos.map((a) => ({
          id: a.tarefa_id,
          valor_pts: a.valor_pts,
          linguagem: a.linguagem,
        })),
      ),
      tarefas: ordemAtributos,
    });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    return { ok: "Atividade atualizada." };
  },
  duplicar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const r = await duplicarAtividade(
      evento.request,
      token,
      Number(evento.params.id),
    );
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      redirect(303, `/professor/atividades/${(r.body as { id?: number } | null)?.id}`);
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível duplicar."),
    });
  },
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const r = await excluirAtividade(
      evento.request,
      token,
      Number(evento.params.id),
    );
    if (r.status === 401) redirect(303, "/");
    if (r.ok) redirect(303, "/professor/atividades");
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível excluir."),
    });
  },
};
