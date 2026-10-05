import { redirect } from "@sveltejs/kit";
import type { LayoutServerLoad } from "./$types";

// Guarda dos portais: sem sessão válida (revalidada na API pelos hooks),
// volta ao login antes de enviar qualquer HTML (ARQUITETURA §4.1).
export const load: LayoutServerLoad = async ({ locals }) => {
  if (!locals.usuario) redirect(303, "/");
  return { usuario: locals.usuario };
};
