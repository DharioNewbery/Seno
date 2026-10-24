import { redirect } from "@sveltejs/kit";
import type { LayoutServerLoad } from "./$types";

// Guarda RBAC do portal do admin: exige cargo 'admin' ou 'super'.
export const load: LayoutServerLoad = async ({ locals }) => {
  if (!locals.usuario) {
    redirect(303, "/");
  }
  if (!locals.usuario.roles.some((r) => r === "admin" || r === "super")) {
    redirect(303, "/portais");
  }
  return { usuario: locals.usuario };
};
