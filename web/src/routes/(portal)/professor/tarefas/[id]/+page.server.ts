import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import {
  detalhesDoVerTarefa,
  duplicarTarefa,
  editarTarefa,
  excluirTarefa,
  mensagemDaApi,
  verTarefaBruta,
  obterAnalyticsTarefa,
  type TarefaAnalytics,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { TesteFormulario } from "#lib/tarefas-testes";

export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  params,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/professor/tarefas");
  try {
    const [r, rAnalytics] = await Promise.all([
      verTarefaBruta(request, token, id),
      obterAnalyticsTarefa(request, token, id),
    ]);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) redirect(303, "/professor/tarefas");
    const detalhes = detalhesDoVerTarefa(r);
    return {
      id,
      tarefa: detalhes.tarefa,
      testes: detalhes.testes,
      analytics: rAnalytics.ok ? (rAnalytics.body as TarefaAnalytics) : null,
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
    const tipoRaw = String(form.get("tipo") ?? "programacao").trim();
    const tipo =
      tipoRaw === "arquivo_submissao" ? "arquivo_submissao" : "programacao";
    if (!nome || !enunciado) {
      return fail(422, { erro: "Informe nome e enunciado da tarefa." });
    }
    if (tipo === "arquivo_submissao") {
      const extBruto = String(form.get("extensoes_permitidas") ?? "").trim();
      const extensoes_permitidas = extBruto
        ? extBruto
            .split(",")
            .map((s) => s.trim().toLowerCase())
            .filter(Boolean)
            .map((s) => (s.startsWith(".") ? s : `.${s}`))
        : [".pdf", ".zip", ".txt", ".md", ".py", ".c", ".cpp", ".png", ".jpg"];
      const maxMb = Number(form.get("max_mb") ?? 10);
      const max_bytes =
        Number.isFinite(maxMb) && maxMb > 0
          ? Math.min(50 * 1024 * 1024, Math.round(maxMb * 1024 * 1024))
          : 10 * 1024 * 1024;
      const r = await editarTarefa(evento.request, token, id, {
        nome,
        enunciado,
        tipo: "arquivo_submissao",
        config_arquivo: {
          extensoes_permitidas,
          max_bytes,
        },
        testes: [],
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
    }
    let testes: TesteFormulario[] = [];
    const bruto = form.get("testes");
    if (typeof bruto === "string" && bruto.trim() !== "") {
      try {
        testes = JSON.parse(bruto);
      } catch {
        return fail(422, {
          erro: "Não entendi os testes: recarregue a página.",
        });
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
      tipo: "programacao",
      testes: testes.map((t: TesteFormulario) => ({
        stdin: t.stdin,
        stdout_esperado: t.stdout,
        publico: Boolean(t.publico),
      })),
      ...(num("tempo_cpu_ms") !== undefined
        ? { tempo_cpu_ms: num("tempo_cpu_ms") }
        : {}),
      ...(num("tempo_total_ms") !== undefined
        ? { tempo_total_ms: num("tempo_total_ms") }
        : {}),
      ...(num("memoria_mb") !== undefined
        ? { memoria_mb: num("memoria_mb") }
        : {}),
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
    const r = await duplicarTarefa(
      evento.request,
      token,
      Number(evento.params.id),
    );
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      redirect(
        303,
        `/professor/tarefas/${(r.body as { id?: number } | null)?.id}`,
      );
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível duplicar."),
    });
  },
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const r = await excluirTarefa(
      evento.request,
      token,
      Number(evento.params.id),
    );
    if (r.status === 401) redirect(303, "/");
    if (r.ok) redirect(303, "/professor/tarefas");
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível excluir."),
    });
  },
};
