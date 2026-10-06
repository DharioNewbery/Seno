import { redirect } from "@sveltejs/kit";

import {
  listaDeMaterias,
  listaDePeriodos,
  listarMaterias,
  listarPeriodos,
  listarTurmas,
  mensagemDaApi,
  turmasDe,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Todas as turmas (staff): entra por "turmas de uma matéria" ou "turmas do
// período" (query params), com filtro de estado e paginação.
export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const token = tokenDaSessao(cookies);

  const materiaF = Number(url.searchParams.get("materia_id")) || undefined;
  const periodoF = Number(url.searchParams.get("periodo_id")) || undefined;
  const encParam = url.searchParams.get("encerradas") ?? "all";
  const encerradas = ["only", "all"].includes(encParam) ? encParam : "all";
  const pagina = Math.max(1, Number(url.searchParams.get("pagina")) || 1);

  try {
    const [t, m, p] = await Promise.all([
      listarTurmas(request, token, {
        encerradas,
        materia_id: materiaF,
        periodo_id: periodoF,
        pagina,
      }),
      listarMaterias(request, token),
      listarPeriodos(request, token),
    ]);
    if (t.status === 401) redirect(303, "/");
    const dados = t.ok ? turmasDe(t) : null;
    return {
      turmas: dados?.turmas ?? [],
      total: dados?.total ?? 0,
      pagina: dados?.pagina ?? 1,
      por: dados?.por ?? 50,
      encerradas,
      filtroMateria: materiaF,
      filtroPeriodo: periodoF,
      materias: m.ok ? listaDeMaterias(m) : [],
      periodos: p.ok ? listaDePeriodos(p) : [],
      erro: t.ok ? null : mensagemDaApi(t.body),
    };
  } catch {
    return {
      turmas: [],
      total: 0,
      pagina: 1,
      por: 50,
      encerradas,
      filtroMateria: materiaF,
      filtroPeriodo: periodoF,
      materias: [],
      periodos: [],
      erro: "Sem comunicação com a API.",
    };
  }
};
