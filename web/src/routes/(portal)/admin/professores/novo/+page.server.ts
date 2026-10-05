import { redirect } from "@sveltejs/kit";

import { gestaoDeCriacao } from "#lib/server/gestao";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ locals }) => {
  if (!locals.usuario) redirect(303, "/");
  return {};
};

export const actions = gestaoDeCriacao("professor").actions;
