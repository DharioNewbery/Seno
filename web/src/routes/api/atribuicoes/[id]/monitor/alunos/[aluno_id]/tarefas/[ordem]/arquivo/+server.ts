// Proxy BFF de download de arquivo de entrega no Monitor (GET /v1/atribuicoes/:id/monitor/alunos/:aluno_id/tarefas/:ordem/arquivo).
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

  const atribuicaoID = Number(params.id);
  const alunoID = Number(params.aluno_id);
  const ordem = Number(params.ordem);
  if (
    !Number.isInteger(atribuicaoID) ||
    atribuicaoID < 1 ||
    !Number.isInteger(alunoID) ||
    alunoID < 1 ||
    !Number.isInteger(ordem) ||
    ordem < 1
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
      `/v1/atribuicoes/${atribuicaoID}/monitor/alunos/${alunoID}/tarefas/${ordem}/arquivo`,
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
