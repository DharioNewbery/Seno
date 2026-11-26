import { atribuicoesMockAluno } from "#lib/atividades-aluno";
import {
  listarMinhasAtribuicoes,
  mensagemDaApi,
  type AtribuicaoDoAluno,
} from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");

  if (url.searchParams.has("mock")) {
    return {
      atribuicoes: atribuicoesMockAluno(),
      erro: null,
    };
  }

  const token = tokenDaSessao(cookies);
  try {
    const ra = await listarMinhasAtribuicoes(request, token);
    if (ra.status === 401) redirect(303, "/");
    if (ra.status === 403) redirect(303, "/");
    return {
      atribuicoes:
        (ra.body as { atribuicoes?: AtribuicaoDoAluno[] } | null)
          ?.atribuicoes ?? [],
      erro: ra.ok ? null : mensagemDaApi(ra.body),
    };
  } catch {
    return {
      atribuicoes: [],
      erro: "Sem comunicação com a API.",
    };
  }
};
