import { error, fail, redirect, type Actions } from "@sveltejs/kit";

import {
  acaoTurma,
  criarAtribuicao,
  excluirAtribuicao,
  listarAtribuicoes,
  listarAtividades,
  mensagemDaApi,
  renomearTurma,
  verAtribuicao,
  verTurma,
  type Atribuicao,
  type Turma as TurmaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Painel de controle da turma: dados + atalhos; escopo no serviço
// (professor dono; admin/super; outsider → 404 → tela de erro do Kit).
export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  params,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) {
    error(404, "Turma não encontrada.");
  }
  const token = tokenDaSessao(cookies);
  try {
    const r = await verTurma(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) error(404, "Turma não encontrada.");
    if (!r.ok) {
      return { turma: null, atribuicoes: [], atividades: [], erro: mensagemDaApi(r.body) };
    }
    // Atribuições da turma + banco de atividades (para attribuir).
    const [ra, rtv] = await Promise.all([
      listarAtribuicoes(request, token, { turma_id: id }),
      listarAtividades(request, token),
    ]);
    const atribuicoes =
      (ra.body as { atribuicoes?: Atribuicao[] } | null)?.atribuicoes ?? [];
    return {
      turma: r.body as TurmaApi,
      atribuicoes,
      atividades:
        (rtv.body as { atividades?: { id: number; nome: string }[] } | null)
          ?.atividades ?? [],
      erro: null,
    };
  } catch (e) {
    if (e && (e as { status?: number }).status) throw e;
    return { turma: null, atribuicoes: [], atividades: [], erro: "Sem comunicação com a API." };
  }
};

export const actions: Actions = {
  renomear: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const titulo = String(
      (await evento.request.formData()).get("titulo") ?? "",
    ).trim();
    const r = await renomearTurma(evento.request, token, id, titulo);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Título atualizado." };
  },

  encerrar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const r = await acaoTurma(evento.request, token, id, "encerrar");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Turma encerrada." };
  },

  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const r = await acaoTurma(evento.request, token, id, "excluir");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    redirect(303, "/professor/turmas");
  },

  // Attribui a atividade à turma (POST /v1/atribuicoes); campos de tempo
  // vazios = padrão §Atribuição.
  atribuir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const turmaID = Number(evento.params.id);
    const form = await evento.request.formData();
    const atividadeID = Number(form.get("atividade_id"));
    if (!Number.isInteger(atividadeID) || atividadeID < 1) {
      return fail(422, { erro: "Selecione a atividade." });
    }
    const cad: {
      turma_id: number;
      atividade_id: number;
      autocomplete?: boolean;
      inicio?: string;
      prazo?: string;
      duracao_seg?: number;
      pode_atrasado?: boolean;
    } = { turma_id: turmaID, atividade_id: atividadeID };
    const prazo = String(form.get("prazo") ?? "").trim();
    if (prazo !== "") cad.prazo = new Date(prazo).toISOString();
    const duracaoHoras = Number(form.get("duracao_horas"));
    if (Number.isFinite(duracaoHoras) && duracaoHoras > 0) {
      cad.duracao_seg = Math.floor(duracaoHoras * 3600);
    }
    if (form.get("pode_atrasado") === "on") cad.pode_atrasado = true;
    const autocerto = String(form.get("autocomplete") ?? "");
    if (autocerto === "on") cad.autocomplete = true;
    if (autocerto === "off") cad.autocomplete = false;
    const r = await criarAtribuicao(evento.request, token, cad);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Atividade atribuída à turma." };
  },

  // Desligar: excluir atribuição.
  desligar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const atribuicaoID = Number(form.get("atribuicao_id"));
    const r = await excluirAtribuicao(evento.request, token, atribuicaoID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Atribuição removida." };
  },
  // Editar config direto pelo painel (vai para página dedicada):
  editarConfig: async (evento) => {
    const form = await evento.request.formData();
    const atribuicaoID = Number(form.get("atribuicao_id"));
    redirect(303, `/professor/atribuicoes/${atribuicaoID}`);
  },
};
