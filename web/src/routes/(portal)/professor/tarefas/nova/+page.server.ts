import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import { criarTarefa, extrairLimite, mensagemDaApi } from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals }) => {
  if (!locals.usuario) redirect(303, "/");
  return {};
};

interface TesteForm {
  stdin: string;
  stdout: string;
  publico: boolean;
}

export const actions: Actions = {
  criar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const nome = String(form.get("nome") ?? "").trim();
    const enunciado = String(form.get("enunciado") ?? "").trim();
    if (!nome || !enunciado) {
      return fail(422, { erro: "Informe nome e enunciado da tarefa." });
    }
    // Testes vindos do front (JSON); mata garantias de stdin/stdout.
    let testes: TesteForm[] = [];
    const bruto = form.get("testes");
    if (typeof bruto === "string" && bruto.trim() !== "") {
      try {
        const lido = JSON.parse(bruto) as unknown;
        if (Array.isArray(lido)) {
          testes = lido.filter(
            (t): t is TesteForm =>
              typeof (t as TesteForm)?.stdout === "string" &&
              typeof (t as TesteForm)?.stdin === "string",
          );
        }
      } catch {
        return fail(422, { erro: "Não entendi os testes: recarregue a página." });
      }
    }
    const cpu = extrairLimite(form, "tempo_cpu_ms", 2000);
    const total = extrairLimite(form, "tempo_total_ms", 5000);
    const mem = extrairLimite(form, "memoria_mb", 256);

    const r = await criarTarefa(evento.request, token, {
      nome,
      enunciado,
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
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    redirect(303, `/professor/tarefas/${(r.body as { id?: number } | null)?.id}`);
  },
};
