import {
  criarAtividade,
  editarAtividade,
  listarTarefas,
  mensagemDaApi,
  montarConteudo,
} from "#lib/server/ensino";
import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

interface Atributo {
  tarefa_id: number;
  valor_pts: number;
  linguagem: string;
}

export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const r = await listarTarefas(request, token);
    if (r.status === 401) redirect(303, "/");
    return {
      tarefas: (r.body as { tarefas?: { id: number; nome: string }[] } | null)
        ?.tarefas ?? [],
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { tarefas: [], erro: "Sem comunicaÃ§Ã£o com a API." };
  }
};

/** LÃª o JSON de tarefas do formulÃ¡rio (gerado pelo front); valida. */
function lerAtributos(bruto: FormDataEntryValue | null): Atributo[] | null {
  if (typeof bruto !== "string" || bruto.trim() === "") return null;
  try {
    const lido = JSON.parse(bruto) as unknown;
    if (!Array.isArray(lido) || lido.length === 0) return null;
    const lista: Atributo[] = [];
    for (const item of lido as Record<string, unknown>[]) {
      const tarefa_id = Number(item.tarefa_id);
      const valor_pts = Number(item.valor_pts);
      const linguagem = String(item.linguagem ?? "");
      if (!Number.isInteger(tarefa_id) || tarefa_id < 1) return null;
      if (!Number.isInteger(valor_pts) || valor_pts < 0) return null;
      if (!["python", "c", "cpp"].includes(linguagem)) return null;
      lista.push({ tarefa_id, valor_pts, linguagem });
    }
    return lista;
  } catch {
    return null;
  }
}

export const actions: Actions = {
  criar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "SessÃ£o encerrada." });
    const form = await evento.request.formData();
    const nome = String(form.get("nome") ?? "").trim();
    if (!nome) return fail(422, { erro: "Informe o nome da atividade." });
    const atributos = lerAtributos(form.get("tarefas"));
    if (!atributos) {
      return fail(422, {
        erro: "Adicione ao menos uma tarefa com linguagem vÃ¡lida.",
      });
    }
    const ordemAtributos = atributos.map((a, i) => ({ ...a, ordem: i + 1 }));
    const r = await criarAtividade(evento.request, token, {
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
    redirect(303, `/professor/atividades/${(r.body as { id?: number } | null)?.id}`);
  },
};
