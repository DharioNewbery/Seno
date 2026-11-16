// Proxy BFF de download de arquivo submetido (GET /v1/submissoes/:id/tarefas/:tarefa_id/arquivo).
// Preserva Content-Type, Content-Disposition, Content-Length e X-Content-Type-Options.
import type { RequestHandler } from "@sveltejs/kit";

import { apiFetch } from "#lib/server/api";
import { comBearer, tokenDaSessao } from "#lib/server/sessao";

export const GET: RequestHandler = async ({ params, cookies, request }) => {
  const token = tokenDaSessao(cookies);
  if (!token) {
    return new Response(
      JSON.stringify({
        erro: { codigo: "SESSAO_INVALIDA", mensagem: "Sessão encerrada." },
      }),
      { status: 401, headers: { "Content-Type": "application/json" } },
    );
  }

  const submissaoID = Number(params.id);
  const tarefaID = Number(params.tarefa_id);
  if (
    !Number.isInteger(submissaoID) ||
    submissaoID < 1 ||
    !Number.isInteger(tarefaID) ||
    tarefaID < 1
  ) {
    return new Response(
      JSON.stringify({
        erro: {
          codigo: "PARAMETRO_INVALIDO",
          mensagem: "Identificador inválido.",
        },
      }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    );
  }

  try {
    const res = await apiFetch(
      `/v1/submissoes/${submissaoID}/tarefas/${tarefaID}/arquivo`,
      request,
      {
        method: "GET",
        headers: comBearer(token),
      },
    );

    const headers = new Headers();
    for (const h of [
      "content-type",
      "content-disposition",
      "content-length",
      "x-content-type-options",
    ]) {
      const v = res.headers.get(h);
      if (v) headers.set(h, v);
    }

    return new Response(res.body, {
      status: res.status,
      headers,
    });
  } catch {
    return new Response(
      JSON.stringify({
        erro: { codigo: "REDE", mensagem: "Sem comunicação com a API." },
      }),
      { status: 502, headers: { "Content-Type": "application/json" } },
    );
  }
};
