import { redirect } from "@sveltejs/kit";

import { gestaoDeCriacao } from "#lib/server/gestao";
import type { PageServerLoad } from "./$types";

// Gestão de admins é exclusiva do super (ARQUITETURA §5.4).
export const load: PageServerLoad = async ({ locals }) => {
  if (!locals.usuario) redirect(303, "/");
  if (!locals.usuario.roles.includes("super")) redirect(303, "/admin");
  return {};
};

export const actions = gestaoDeCriacao("admin").actions;
