import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

// Seletor exige sessão válida (hooks já revalidaram na API).
export const load: PageServerLoad = async ({ locals }) => {
  if (!locals.usuario) redirect(303, "/");
  return { usuario: locals.usuario };
};
