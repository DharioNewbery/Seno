import { redirect } from "@sveltejs/kit";
import type { LayoutServerLoad } from "./$types";

// Guarda RBAC do portal do professor: exige cargo 'professor'.
export const load: LayoutServerLoad = async ({ locals }) => {
  if (!locals.usuario) {
    redirect(303, "/");
  }
  if (!locals.usuario.roles.includes("professor")) {
    redirect(303, "/portais");
  }
  return { usuario: locals.usuario };
};
