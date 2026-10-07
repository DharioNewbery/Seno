import {
  listarMinhasAtribuicoes,
  listarMinhasTurmas,
  mensagemDaApi,
  type AtribuicaoDoAluno,
  type TurmaDoAluno,
} from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, url }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const turmaParam = Number(url.searchParams.get("turma_id")) || undefined;
  try {
    const [ra, rt] = await Promise.all([
      listarMinhasAtribuicoes(request, token, turmaParam),
      listarMinhasTurmas(request, token),
    ]);
    if (ra.status === 401 || rt.status === 401) redirect(303, "/");
    if (ra.status === 403) redirect(303, "/");
    return {
      atribuicoes:
        (ra.body as { atribuicoes?: AtribuicaoDoAluno[] } | null)?.atribuicoes ?? [],
      turmas: (rt.body as { turmas?: TurmaDoAluno[] } | null)?.turmas ?? [],
      filtroTurma: turmaParam,
      erro: ra.ok ? null : mensagemDaApi(ra.body),
    };
  } catch {
    return {
      atribuicoes: [],
      turmas: [],
      filtroTurma: turmaParam,
      erro: "Sem comunicação com a API.",
    };
  }
};
