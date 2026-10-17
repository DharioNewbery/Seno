import {
  obterMinhaTurma,
  mensagemDaApi,
  type DetalhesTurmaAluno,
} from "#lib/server/ensino";
import { error, redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const turmaID = Number(params.id);
  if (!Number.isInteger(turmaID) || turmaID <= 0) {
    throw error(404, "Turma não encontrada.");
  }

  const token = tokenDaSessao(cookies);
  try {
    const r = await obterMinhaTurma(request, token, turmaID);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 403) redirect(303, "/aluno");
    if (r.status === 404) {
      throw error(404, "Turma não encontrada ou você não possui matrícula ativa.");
    }
    if (!r.ok) {
      return {
        detalhes: null,
        erro: mensagemDaApi(r.body),
      };
    }
    return {
      detalhes: r.body as DetalhesTurmaAluno,
      erro: null,
    };
  } catch (err: unknown) {
    if (err && typeof err === "object" && "status" in err) {
      throw err;
    }
    return { detalhes: null, erro: "Sem comunicação com a API." };
  }
};
