// BFF de entrada no Monitor: pede o `st` na API e redireciona (Fase 9.4).
// NOTE: redirect() do SvelteKit v2 recusa URL externa
// (redirect_external_not_allowed) — o Monitor é outra origem, então o
// 302 é construído manualmente.
// Resiliência: quando o Monitor está configurado na instalação mas fora do ar
// (503 SERVICO_INDISPONIVEL) ou quando a própria API está indisponível, volta
// à página da atividade informando o erro ao aluno (impedindo split-brain no editor legado).
import type { RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";
import { comBearer, tokenDaSessao } from "#lib/server/sessao";

function para(location: string): Response {
  return new Response(null, { status: 302, headers: { Location: location } });
}

export const GET: RequestHandler = async (event) => {
  const { params, cookies, request } = event;
  const token = tokenDaSessao(cookies);
  if (!token) return para("/");

  const editor = `/aluno/atividade/${params.id}`; // editor local apenas quando Monitor está desligado na instalação
  const telaAtividade = `/aluno/atividades/${params.id}`;
  try {
    const res = await apiFetch(`/v1/atribuicoes/${params.id}/entrar`, request, {
      method: "GET",
      headers: comBearer(token),
    });
    if (res.status === 401 || res.status === 403) return para("/");
    if (res.status === 404) return para("/aluno/atividades");
    if (res.status === 409) return para(telaAtividade);

    const body = (await res.json().catch(() => ({}))) as {
      url?: string;
      erro?: { codigo?: string; mensagem?: string };
    };

    if (res.status === 503) {
      // Monitor explicitamente desligado na instalação (CodeConflict): usa o editor local da API.
      if (body.erro?.codigo === "CONFLITO_DE_ESTADO") {
        return para(editor);
      }
      // Monitor ligado na instalação, porém temporariamente fora do ar: informa o aluno sem criar rascunho paralelo.
      return para(`${telaAtividade}?erro=monitor_indisponivel`);
    }

    if (!res.ok || !body.url) {
      return para(`${telaAtividade}?erro=monitor_indisponivel`);
    }
    return para(body.url);
  } catch {
    return para(`${telaAtividade}?erro=api_indisponivel`);
  }
};
