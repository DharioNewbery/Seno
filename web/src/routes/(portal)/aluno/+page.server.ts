import {
  listarMinhasAtribuicoes,
  listarMinhasTurmas,
  mensagemDaApi,
  type AtribuicaoDoAluno,
  type TurmaDoAluno,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import { atribuicoesMock, turmasMock } from "#lib/agenda-aluno-mock";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");
  // Só em desenvolvimento: /aluno?mock mostra todas as combinações de estado.
  if (import.meta.env.DEV && url.searchParams.has("mock")) {
    return {
      atribuicoes: atribuicoesMock(),
      turmas: turmasMock(),
      erro: null,
    };
  }
  const token = tokenDaSessao(cookies);
  try {
    const [ra, rt] = await Promise.all([
      listarMinhasAtribuicoes(request, token),
      listarMinhasTurmas(request, token),
    ]);
    if (
      ra.status === 401 ||
      rt.status === 401 ||
      ra.status === 403 ||
      rt.status === 403
    ) {
      redirect(303, "/");
    }
    return {
      atribuicoes:
        (ra.body as { atribuicoes?: AtribuicaoDoAluno[] } | null)
          ?.atribuicoes ?? [],
      turmas: (rt.body as { turmas?: TurmaDoAluno[] } | null)?.turmas ?? [],
      erro: ra.ok ? null : mensagemDaApi(ra.body),
    };
  } catch (e) {
    if (e && typeof e === "object" && "location" in e) throw e; // redirect
    return {
      atribuicoes: [],
      turmas: [],
      erro: "Sem comunicação com a API.",
    };
  }
};
