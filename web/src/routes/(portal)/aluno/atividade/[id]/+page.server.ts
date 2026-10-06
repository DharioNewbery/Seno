import { mensagemDaApi, visaoAluno, gravarTentativa } from "#lib/server/ensino";
import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/aluno");
  try {
    const r = await visaoAluno(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) redirect(303, "/aluno");
    return {
      visao: r.ok
        ? (r.body as {
            atribuicao: {
              id: number;
              prazo?: string | null;
              pode_atrasado: boolean;
            };
            atividade: {
              id: number;
              nome: string;
              tarefas: {
                id: number;
                ordem: number;
                valor_pts: number;
                linguagem: string;
                nome: string;
                enunciado: string;
                excluida: boolean;
              }[];
            };
            tentativa?: {
              revisao: number;
              snapshot: { tasks?: { id: number; text: string }[] };
            };
          } | null)
        : null,
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { visao: null, erro: "Sem comunicação com a API." };
  }
};

/** Autosave: recebe revisão + snapshot (JSON) via form-urlencoded. */
export const actions: Actions = {
  gravar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const form = await evento.request.formData();
    const revisao = Number(form.get("revisao"));
    let snapshot: unknown;
    try {
      snapshot = JSON.parse(String(form.get("snapshot") ?? ""));
    } catch {
      return fail(422, { erro: "Snapshot inválido." });
    }
    if (!Number.isInteger(revisao) || revisao < 1) {
      return fail(422, { erro: "Revisão inválida." });
    }
    const r = await gravarTentativa(evento.request, token, id, {
      revisao,
      snapshot: snapshot as { tasks: { id: number; text: string }[] },
    });
    if (r.status === 401) return fail(401, { erro: "Sessão encerrada." });
    if (r.ok) {
      const cor = r.body as { revisao?: number };
      return { revisao: cor.revisao };
    }
    return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
  },
};
