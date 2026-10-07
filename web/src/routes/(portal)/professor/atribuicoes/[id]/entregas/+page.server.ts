import {
  entregasDe,
  listarEntregas,
  mensagemDaApi,
  verAtribuicao,
  type Atribuicao,
} from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/professor/turmas");
  try {
    const ra = await verAtribuicao(request, token, id);
    if (ra.status === 401) redirect(303, "/");
    if (ra.status === 404) redirect(303, "/professor/turmas");
    const re = await listarEntregas(request, token, id);
    if (re.status === 401) redirect(303, "/");
    return {
      atribuicao: (ra.body as Atribuicao | null) ?? null,
      entregas: re.ok ? entregasDe(re) : [],
      erro: re.ok ? null : mensagemDaApi(re.body),
    };
  } catch {
    return {
      atribuicao: null,
      entregas: [],
      erro: "Sem comunicação com a API.",
    };
  }
};
