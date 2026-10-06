import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import {
  detalhesDoVerTarefa,
  duplicarTarefa,
  editarTarefa,
  excluirTarefa,
  mensagemDaApi,
  verTarefaBruta,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";

interface Teste {
  stdin: string;
  stdout: string;
  publico: boolean;
}

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/professor/tarefas");
  try {
    const r = await verTarefaBruta(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) redirect(303, "/professor/tarefas");
    const detalhes = detalhesDoVerTarefa(r);
    return {
      id,
      tarefa: detalhes.tarefa,
      testes: detalhes.testes,
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    redirect(303, "/professor/tarefas");
  }
};

export const actions: Actions = {
  salvar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const form = await evento.request.formData();
    const nome = String(form.get("nome") ?? "").trim();
    const enunciado = String(form.get("enunciado") ?? "").trim();
    if (!nome || !enunciado) {
      return fail(422, { erro: "Informe nome e enunciado da tarefa." });
    }
    let testes: Teste[] = [];
    const bruto = form.get("testes");
    if (typeof bruto === "string" && bruto.trim() !== "") {
      try {
        testes = JSON.parse(bruto);
      } catch {
        return fail(422, { erro: "Não entendi os testes: recarregue a página." });
      }
    }
    const num = (campo: string): number | undefined => {
      const v = form.get(campo);
      if (v === null || String(v).trim() === "") return undefined;
      const n = Number(v);
      return Number.isFinite(n) && n > 0 ? Math.floor(n) : undefined;
    };
    const r = await editarTarefa(evento.request, token, id, {
      nome,
      enunciado,
      testes: testes.map((t: Teste) => ({
        stdin: t.stdin,
        stdout_esperado: t.stdout,
        publico: Boolean(t.publico),
      })),
      ...(num("tempo_cpu_ms") !== undefined ? { tempo_cpu_ms: num("tempo_cpu_ms") } : {}),
      ...(num("tempo_total_ms") !== undefined ? { tempo_total_ms: num("tempo_total_ms") } : {}),
      ...(num("memoria_mb") !== undefined ? { memoria_mb: num("memoria_mb") } : {}),
    });
    if (r.status === 401) redirect(303, "/");
    if (r.status === 409) {
      return fail(409, {
        erro:
          mensagemDaApi(r.body) ??
          "Tarefa em uso por atividade — duplique para editar.",
      });
    }
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Tarefa atualizada." };
  },
  duplicar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const r = await duplicarTarefa(evento.request, token, Number(evento.params.id));
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      redirect(303, `/professor/tarefas/${(r.body as { id?: number } | null)?.id}`);
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível duplicar."),
    });
  },
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const r = await excluirTarefa(evento.request, token, Number(evento.params.id));
    if (r.status === 401) redirect(303, "/");
    if (r.ok) redirect(303, "/professor/tarefas");
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível excluir."),
    });
  },
};
