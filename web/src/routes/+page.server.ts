import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

// Sem sessão, a raiz é a landing pública com o formulário de login;
// com sessão, o usuário vai direto ao seletor de portais.
export const load: PageServerLoad = async ({ locals }) => {
  if (locals.usuario) redirect(303, "/portais");
  return {};
};
