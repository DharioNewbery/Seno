import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

// Todas as páginas do portal do aluno convergem para Atividades.
export const load: PageServerLoad = async () => {
  redirect(303, "/aluno/atividades");
};
