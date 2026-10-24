import { redirect } from "@sveltejs/kit";
import type { LayoutServerLoad } from "./$types";

// Guarda RBAC do portal do aluno: exige cargo 'student'.
export const load: LayoutServerLoad = async ({ locals }) => {
  if (!locals.usuario) {
    redirect(303, "/");
  }
  if (!locals.usuario.roles.includes("student")) {
    redirect(303, "/portais");
  }
  return { usuario: locals.usuario };
};
