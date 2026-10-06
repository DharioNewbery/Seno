import { redirect } from "@sveltejs/kit";

import {
  listarTurmas,
  mensagemDaApi,
  type ListaTurmas,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Lista as turmas do professor logado; padrão: só ativas (encerradas via
// ?encerradas=only|all).
export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const encParam = url.searchParams.get("encerradas") ?? "";
  const encerradas = ["only", "all"].includes(encParam) ? encParam : "";
  const pagina = Math.max(1, Number(url.searchParams.get("pagina")) || 1);

  const token = tokenDaSessao(cookies);
  try {
    const r = await listarTurmas(request, token, { encerradas, pagina });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return {
        total: 0,
        pagina: 1,
        por: 50,
        turmas: [],
        encerradas,
        erro: mensagemDaApi(r.body),
      };
    }
    const dados = r.body as ListaTurmas;
    return {
      total: dados.total,
      pagina: dados.pagina,
      por: dados.por,
      turmas: dados.turmas,
      encerradas,
      erro: null,
    };
  } catch {
    return {
      total: 0,
      pagina: 1,
      por: 50,
      turmas: [],
      encerradas,
      erro: "Sem comunicação com a API.",
    };
  }
};
