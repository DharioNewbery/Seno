import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import {
  criarTarefa,
  duplicarTarefa,
  excluirTarefa,
  extrairLimite,
  listaDeTarefas,
  listarTarefas,
  mensagemDaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { TesteFormulario } from "#lib/tarefas-testes";

// Banco de tarefas do professor: listagem + modal de criação de tarefa e testes.
export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const pagina = Number(url.searchParams.get("pagina")) || 1;
  try {
    const r = await listarTarefas(request, token);
    if (r.status === 401) redirect(303, "/");
    const lista = r.body as {
      total?: number;
      por?: number;
      tarefas?: unknown[];
    } | null;
    return {
      tarefas: r.ok ? listaDeTarefas(r) : [],
      total:
        r.ok && lista?.total != null ? lista.total : listaDeTarefas(r).length,
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
      erro: "Sem comunicação com a API.",
    };
  }
};

export const actions: Actions = {
  criar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { acao: "criar", erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const nome = String(form.get("nome") ?? "").trim();
    const enunciado = String(form.get("enunciado") ?? "").trim();
    const tipoRaw = String(form.get("tipo") ?? "programacao").trim();
    const tipo =
      tipoRaw === "arquivo_submissao" ? "arquivo_submissao" : "programacao";
    if (!nome || !enunciado) {
      return fail(422, {
        acao: "criar",
        erro: "Informe nome e enunciado da tarefa.",
      });
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

      const r = await criarTarefa(evento.request, token, {
        nome,
        enunciado,
        tipo: "arquivo_submissao",
        config_arquivo: {
          extensoes_permitidas,
          max_bytes,
        },
      });
      if (r.status === 401) redirect(303, "/");
      if (!r.ok) {
        return fail(r.status || 500, {
          acao: "criar",
          erro: mensagemDaApi(r.body),
        });
      }
      redirect(
        303,
        `/professor/tarefas/${(r.body as { id?: number } | null)?.id}`,
      );
    }

    let testes: TesteFormulario[] = [];
    const bruto = form.get("testes");
    if (typeof bruto === "string" && bruto.trim() !== "") {
      try {
        const lido = JSON.parse(bruto) as unknown;
        if (Array.isArray(lido)) {
          testes = lido.filter(
            (t): t is TesteFormulario =>
              typeof (t as TesteFormulario)?.stdout === "string" &&
              typeof (t as TesteFormulario)?.stdin === "string",
          );
        }
      } catch {
        return fail(422, {
          acao: "criar",
          erro: "Não entendi os testes: recarregue a página.",
        });
      }
    }
    const cpu = extrairLimite(form, "tempo_cpu_ms", 2000);
    const total = extrairLimite(form, "tempo_total_ms", 5000);
    const mem = extrairLimite(form, "memoria_mb", 256);

    const r = await criarTarefa(evento.request, token, {
      nome,
      enunciado,
      tipo: "programacao",
      ...(cpu !== undefined ? { tempo_cpu_ms: cpu } : {}),
      ...(total !== undefined ? { tempo_total_ms: total } : {}),
      ...(mem !== undefined ? { memoria_mb: mem } : {}),
      testes: testes.map((t) => ({
        stdin: t.stdin,
        stdout_esperado: t.stdout,
        publico: Boolean(t.publico),
      })),
    });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, {
        acao: "criar",
        erro: mensagemDaApi(r.body),
      });
    }
    redirect(
      303,
      `/professor/tarefas/${(r.body as { id?: number } | null)?.id}`,
    );
  },
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("id"));
    const r = await excluirTarefa(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      return { ok: "Tarefa excluída." };
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível excluir."),
    });
  },
  duplicar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("id"));
    const r = await duplicarTarefa(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      const copia = (r.body as { id?: number } | null)?.id;
      redirect(303, `/professor/tarefas/${copia}`);
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível duplicar."),
    });
  },
};
