// Tela pré-atividade: inspeção da atribuição pelo aluno antes de abrir o
// editor/Monitor. Dados do portal (GET /v1/me/atribuicoes), filtrado aqui
// pelo id da rota; atribuição inexistente ou fora do portal → lista.
import {
  listarMinhasAtribuicoes,
  mensagemDaApi,
  type AtribuicaoDoAluno,
  type Resultado,
} from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({ locals, cookies, request, params }) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/aluno/atividades");
  let r: Resultado;
  try {
    r = await listarMinhasAtribuicoes(request, token);
  } catch {
    return { a: null as AtribuicaoDoAluno | null, erro: "Sem comunicação com a API." };
  }
  if (r.status === 401 || r.status === 403) redirect(303, "/");
  const lista = (r.body as { atribuicoes?: AtribuicaoDoAluno[] } | null)?.atribuicoes ?? [];
  const a = lista.find((x) => x.id === id) ?? null;
  if (!a) redirect(303, "/aluno/atividades");
  return { a, erro: r.ok ? null : mensagemDaApi(r.body) };
};
