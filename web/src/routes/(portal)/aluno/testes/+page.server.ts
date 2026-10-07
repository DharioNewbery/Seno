import { execucoesDe, historicoTestes, mensagemDaApi } from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  try {
    const r = await historicoTestes(request, token);
    if (r.status === 401) redirect(303, "/");
    return {
      execucoes: r.ok ? execucoesDe(r) : [],
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { execucoes: [], erro: "Sem comunicação com a API." };
  }
};
