import {
  duplicarAtividade,
  excluirAtividade,
  listaDeAtividades,
  listarAtividades,
  mensagemDaApi,
} from "#lib/server/ensino";
import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

// Banco de atividades: listagem (escopo por dono); criar via /nova,
// detail em /[id] com ediÃ§Ã£o.
export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const r = await listarAtividades(request, token);
    if (r.status === 401) redirect(303, "/");
    return {
      atividades: r.ok ? listaDeAtividades(r) : [],
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { atividades: [], erro: "Sem comunicaÃ§Ã£o com a API." };
  }
};

export const actions: Actions = {
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "SessÃ£o encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("id"));
    const r = await excluirAtividade(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.ok) return { ok: "Atividade excluÃ­da." };
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "NÃ£o foi possÃ­vel excluir."),
    });
  },
  duplicar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "SessÃ£o encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("id"));
    const r = await duplicarAtividade(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      redirect(303, `/professor/atividades/${(r.body as { id?: number } | null)?.id}`);
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "NÃ£o foi possÃ­vel duplicar."),
    });
  },
};
