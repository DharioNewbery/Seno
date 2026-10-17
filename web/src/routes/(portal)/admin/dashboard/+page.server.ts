import {
  obterAdminMetricas,
  mensagemDaApi,
  type AdminMetricas,
} from "#lib/server/ensino";
import { error, redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request }) => {
  if (!locals.usuario) redirect(303, "/");
  const roles = locals.usuario.roles ?? [];
  const isStaff = roles.includes("admin") || roles.includes("super");
  if (!isStaff) redirect(303, "/");

  const token = tokenDaSessao(cookies);
  try {
    const r = await obterAdminMetricas(request, token);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 403) redirect(303, "/admin");
    if (!r.ok) {
      return {
        metricas: null,
        erro: mensagemDaApi(r.body),
      };
    }
    return {
      metricas: r.body as AdminMetricas,
      erro: null,
    };
  } catch {
    return {
      metricas: null,
      erro: "Sem comunicação com a API.",
    };
  }
};
