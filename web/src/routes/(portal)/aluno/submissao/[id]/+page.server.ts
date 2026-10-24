// "Minha correção": correção publicada da própria submissão
// (GET /v1/submissoes/:id/minha; antes de publicar, só o estado) +
// a própria entrega (GET /v1/submissoes/:id).
import {
  mensagemDaApi,
  minhaCorrecao,
  minhaSubmissao,
  type SubmissaoAluno,
  type VisaoCorrecaoAluno,
} from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/aluno/atividades");
  try {
    const r = await minhaCorrecao(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) redirect(303, "/aluno/atividades");
    // A entrega (o que o aluno enviou) é dele: visível mesmo antes de
    // a correção ser publicada.
    const s = await minhaSubmissao(request, token, id);
    return {
      visao: r.ok ? (r.body as VisaoCorrecaoAluno) : null,
      entrega: s.ok ? (s.body as SubmissaoAluno) : null,
      erro: r.ok ? null : mensagemDaApi(r.body),
    };
  } catch {
    return { visao: null, entrega: null, erro: "Sem comunicação com a API." };
  }
};
