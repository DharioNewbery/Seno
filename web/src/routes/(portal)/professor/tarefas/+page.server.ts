import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import {
  duplicarTarefa,
  excluirTarefa,
  listaDeTarefas,
  listarTarefas,
  mensagemDaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";

// Banco de tarefas do professor: listagem (API forÃ§a escopo por dono);
// criar/nova e o detalhe ficam em pÃ¡ginas prÃ³prias.
export const load: PageServerLoad = async ({ locals, cookies, request, url }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const pagina = Number(url.searchParams.get("pagina")) || 1;
  try {
    const r = await listarTarefas(request, token);
    if (r.status === 401) redirect(303, "/");
    const lista = r.body as
      | { total?: number; por?: number; tarefas?: unknown[] }
      | null;
    return {
      tarefas: r.ok ? listaDeTarefas(r) : [],
      total: r.ok && lista?.total != null ? lista.total : listaDeTarefas(r).length,
      por: lista?.por ?? 50,
      pagina: pagina > 1 ? pagina : 1,
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return {
      tarefas: [],
      total: 0,
      por: 50,
      pagina,
      erro: "Sem comunicaÃ§Ã£o com a API.",
    };
  }
};

export const actions: Actions = {
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) fail(401, { erro: "SessÃ£o encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("id"));
    const r = await excluirTarefa(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      return { ok: "Tarefa excluÃ­da." };
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "NÃ£o foi possÃ­vel excluir."),
    });
  },
  duplicar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) fail(401, { erro: "SessÃ£o encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("id"));
    const r = await duplicarTarefa(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      const copia = (r.body as { id?: number } | null)?.id;
      redirect(303, `/professor/tarefas/${copia}`);
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "NÃ£o foi possÃ­vel duplicar."),
    });
  },
};
