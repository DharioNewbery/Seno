import { fail, redirect, type Actions } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

import {
  editarAtribuicao,
  excluirAtribuicao,
  mensagemDaApi,
  verAtribuicao,
  type Atribuicao,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/professor/turmas");
  try {
    const r = await verAtribuicao(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) redirect(303, "/professor/turmas");
    return {
      atribuicao: (r.body as Atribuicao | null) ?? null,
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { atribuicao: null, erro: "Sem comunicação com a API." };
  }
};

function texto(form: FormData, campo: string): string | null {
  const bruto = String(form.get(campo) ?? "").trim();
  return bruto === "" ? null : bruto;
}

export const actions: Actions = {
  salvar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const form = await evento.request.formData();
    const auto = String(form.get("autocomplete") ?? "");
    const cad: Parameters<typeof editarAtribuicao>[3] = {
      autocomplete: auto === "on" ? true : auto === "off" ? false : undefined,
      inicio: texto(form, "inicio") ?? null,
      prazo: texto(form, "prazo") ?? null,
      duracao_seg: null,
      pode_atrasado: form.get("pode_atrasado") === "on",
    };
    const duracaoHoras = Number(form.get("duracao_horas"));
    if (Number.isFinite(duracaoHoras) && duracaoHoras > 0) {
      cad.duracao_seg = Math.floor(duracaoHoras * 3600);
    }
    const r = await editarAtribuicao(evento.request, token, id, cad);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Configuração salva." };
  },
  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const r = await excluirAtribuicao(evento.request, token, Number(evento.params.id));
    if (r.status === 401) redirect(303, "/");
    if (r.ok) {
      const turma = evento.url.searchParams.get("turma");
      redirect(
        303,
        turma && /^\d+$/.test(turma) ? `/professor/turmas/${turma}` : "/professor/turmas",
      );
    }
    return fail(r.status || 500, {
      erro: mensagemDaApi(r.body, "Não foi possível excluir."),
    });
  },
};
