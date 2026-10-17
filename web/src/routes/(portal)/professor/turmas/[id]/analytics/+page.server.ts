import {
  obterAnalyticsTurma,
  obterAnalyticsAtribuicao,
  obterAnalyticsAlunoTurma,
  verTurma,
  listarMatriculas,
  mensagemDaApi,
  type TurmaAnalytics,
  type Turma,
  type Matricula,
  type AtribuicaoAnalytics,
  type AlunoTurmaAnalytics,
} from "#lib/server/ensino";
import { error, redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params, url }) => {
  if (!locals.usuario) redirect(303, "/");
  const turmaID = Number(params.id);
  if (!Number.isInteger(turmaID) || turmaID <= 0) {
    throw error(404, "Turma não encontrada.");
  }

  const token = tokenDaSessao(cookies);
  const aba = url.searchParams.get("aba") || "turma";
  const atribuicaoIDParam = Number(url.searchParams.get("atribuicao_id"));
  const alunoIDParam = Number(url.searchParams.get("aluno_id"));

  try {
    const [resTurma, resAnalytics, resMatriculas] = await Promise.all([
      verTurma(request, token, turmaID),
      obterAnalyticsTurma(request, token, turmaID),
      listarMatriculas(request, token, turmaID, false),
    ]);

    if (resTurma.status === 401 || resAnalytics.status === 401) redirect(303, "/");
    if (resTurma.status === 403 || resAnalytics.status === 403) redirect(303, "/professor");
    if (resTurma.status === 404 || resAnalytics.status === 404) {
      throw error(404, "Turma não encontrada.");
    }

    const turma = (resTurma.body as { turma?: Turma } | null)?.turma ?? null;
    const analytics = (resAnalytics.body as TurmaAnalytics | null) ?? null;
    const matriculas = (resMatriculas.body as { matriculas?: Matricula[] } | null)?.matriculas ?? [];

    let atribuicaoAnalytics: AtribuicaoAnalytics | null = null;
    if (aba === "atividade" && atribuicaoIDParam > 0) {
      const resAtrib = await obterAnalyticsAtribuicao(request, token, atribuicaoIDParam);
      if (resAtrib.ok) {
        atribuicaoAnalytics = resAtrib.body as AtribuicaoAnalytics;
      }
    } else if (aba === "atividade" && analytics?.atribuicoes && analytics.atribuicoes.length > 0) {
      const primeiroId = analytics.atribuicoes[0]?.atribuicao_id;
      if (primeiroId) {
        const resAtrib = await obterAnalyticsAtribuicao(request, token, primeiroId);
        if (resAtrib.ok) {
          atribuicaoAnalytics = resAtrib.body as AtribuicaoAnalytics;
        }
      }
    }

    let alunoAnalytics: AlunoTurmaAnalytics | null = null;
    if (aba === "aluno" && alunoIDParam > 0) {
      const resAluno = await obterAnalyticsAlunoTurma(request, token, turmaID, alunoIDParam);
      if (resAluno.ok) {
        alunoAnalytics = resAluno.body as AlunoTurmaAnalytics;
      }
    } else if (aba === "aluno" && matriculas.length > 0) {
      const primeiroAlunoId = matriculas[0]?.aluno.id;
      if (primeiroAlunoId) {
        const resAluno = await obterAnalyticsAlunoTurma(request, token, turmaID, primeiroAlunoId);
        if (resAluno.ok) {
          alunoAnalytics = resAluno.body as AlunoTurmaAnalytics;
        }
      }
    }

    return {
      aba,
      turma,
      analytics,
      matriculas,
      atribuicaoAnalytics,
      alunoAnalytics,
      atribuicaoSelecionada: atribuicaoAnalytics?.atribuicao_id ?? atribuicaoIDParam,
      alunoSelecionado: alunoAnalytics?.aluno_id ?? alunoIDParam,
      erro: resAnalytics.ok ? null : mensagemDaApi(resAnalytics.body),
    };
  } catch (err: unknown) {
    if (err && typeof err === "object" && "status" in err) throw err;
    return {
      aba,
      turma: null,
      analytics: null,
      matriculas: [],
      atribuicaoAnalytics: null,
      alunoAnalytics: null,
      atribuicaoSelecionada: 0,
      alunoSelecionado: 0,
      erro: "Sem comunicação com a API.",
    };
  }
};
