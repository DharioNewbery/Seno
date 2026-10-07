import {
  listarMinhasTurmas,
  mensagemDaApi,
  type TurmaDoAluno,
} from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const r = await listarMinhasTurmas(request, token);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 403) redirect(303, "/aluno");
    return {
      turmas: (r.body as { turmas?: TurmaDoAluno[] } | null)?.turmas ?? [],
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { turmas: [], erro: "Sem comunicação com a API." };
  }
};
