// Tela da atividade do aluno: inspeção antes de iniciar e visualização
// completa das próprias respostas e notas quando entregue e publicada.
import {
  listarMinhasAtribuicoes,
  mensagemDaApi,
  minhaCorrecao,
  minhaSubmissao,
  type AtribuicaoDoAluno,
  type Resultado,
  type SubmissaoAluno,
  type VisaoCorrecaoAluno,
} from "#lib/server/ensino";
import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";
import { tokenDaSessao } from "#lib/server/sessao";

export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  params,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) redirect(303, "/aluno/atividades");
  const erroParam = url.searchParams.get("erro");
  const avisoMonitor =
    erroParam === "monitor_indisponivel"
      ? "O ambiente de realização da atividade (Monitor) está temporariamente indisponível. Seus dados estão seguros — tente abrir novamente em instantes."
      : erroParam === "api_indisponivel"
        ? "Não foi possível comunicar com o servidor no momento. Tente novamente em instantes."
        : null;
  let r: Resultado;
  try {
    r = await listarMinhasAtribuicoes(request, token);
  } catch {
    return {
      a: null as AtribuicaoDoAluno | null,
      visao: null as VisaoCorrecaoAluno | null,
      entrega: null as SubmissaoAluno | null,
      aviso: avisoMonitor,
      erro: "Sem comunicação com a API.",
    };
  }
  if (r.status === 401 || r.status === 403) redirect(303, "/");
  const lista =
    (r.body as { atribuicoes?: AtribuicaoDoAluno[] } | null)?.atribuicoes ?? [];
  const a = lista.find((x) => x.id === id) ?? null;
  if (!a) redirect(303, "/aluno/atividades");

  let visao: VisaoCorrecaoAluno | null = null;
  let entrega: SubmissaoAluno | null = null;
  if (a.submissao_id) {
    try {
      const [rc, rs] = await Promise.all([
        minhaCorrecao(request, token, a.submissao_id),
        minhaSubmissao(request, token, a.submissao_id),
      ]);
      if (rc.ok) visao = rc.body as VisaoCorrecaoAluno;
      if (rs.ok) entrega = rs.body as SubmissaoAluno;
    } catch {
      // Mantém os metadados da atribuição caso falhe o detalhe da submissão.
    }
  }

  return {
    a,
    visao,
    entrega,
    aviso: avisoMonitor,
    erro: r.ok ? null : mensagemDaApi(r.body),
  };
};
