import { fail, redirect, type Actions } from "@sveltejs/kit";

import {
  criarTurma,
  listaDeMaterias,
  listaDePeriodos,
  listarMaterias,
  listarPeriodos,
  listarTurmas,
  mensagemDaApi,
  type ListaTurmas,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import {
  ordenarPeriodosRecentes,
  resolverMateriaSelecionada,
  resolverPeriodoSelecionado,
} from "#lib/turmas-professor";
import type { PageServerLoad } from "./$types";

// Lista as turmas do professor logado filtradas por período (padrão: mais
// recente; não há "todos os períodos") e matéria (padrão: todas). Carrega
// separadamente as ativas e as encerradas para exibição sob demanda.
export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const pagina = Math.max(1, Number(url.searchParams.get("pagina")) || 1);
  const mostrarEncerradas =
    url.searchParams.get("encerradas") === "1" ||
    url.searchParams.get("encerradas") === "all" ||
    url.searchParams.get("encerradas") === "only";

  const token = tokenDaSessao(cookies);
  try {
    const [rm, rp] = await Promise.all([
      listarMaterias(request, token),
      listarPeriodos(request, token),
    ]);
    if (rm.status === 401 || rp.status === 401) redirect(303, "/");

    const materias = rm.ok ? listaDeMaterias(rm) : [];
    const periodos = rp.ok ? ordenarPeriodosRecentes(listaDePeriodos(rp)) : [];
    const erroCatalogo = !rm.ok
      ? mensagemDaApi(rm.body)
      : !rp.ok
        ? mensagemDaApi(rp.body)
        : null;

    const filtroMateria = resolverMateriaSelecionada(
      materias,
      url.searchParams.get("materia_id"),
    );
    const filtroPeriodo = resolverPeriodoSelecionado(
      periodos,
      url.searchParams.get("periodo_id"),
    );

    if (filtroPeriodo === undefined) {
      return {
        total: 0,
        totalEncerradas: 0,
        pagina: 1,
        por: 50,
        turmas: [],
        turmasEncerradas: [],
        materias,
        periodos,
        filtroMateria,
        filtroPeriodo: undefined,
        semPeriodos: true,
        mostrarEncerradas,
        erro: erroCatalogo,
      };
    }

    const [rAtivas, rEncerradas] = await Promise.all([
      listarTurmas(request, token, {
        encerradas: "",
        materia_id: filtroMateria,
        periodo_id: filtroPeriodo,
        pagina,
      }),
      listarTurmas(request, token, {
        encerradas: "only",
        materia_id: filtroMateria,
        periodo_id: filtroPeriodo,
        pagina: 1,
      }),
    ]);

    if (rAtivas.status === 401 || rEncerradas.status === 401) {
      redirect(303, "/");
    }

    if (!rAtivas.ok) {
      return {
        total: 0,
        totalEncerradas: 0,
        pagina: 1,
        por: 50,
        turmas: [],
        turmasEncerradas: [],
        materias,
        periodos,
        filtroMateria,
        filtroPeriodo,
        semPeriodos: false,
        mostrarEncerradas,
        erro: mensagemDaApi(rAtivas.body),
      };
    }

    const dadosAtivas = rAtivas.body as ListaTurmas;
    const dadosEncerradas = rEncerradas.ok
      ? (rEncerradas.body as ListaTurmas)
      : null;

    return {
      total: dadosAtivas.total,
      totalEncerradas: dadosEncerradas?.total ?? 0,
      pagina: dadosAtivas.pagina,
      por: dadosAtivas.por,
      turmas: dadosAtivas.turmas,
      turmasEncerradas: dadosEncerradas?.turmas ?? [],
      materias,
      periodos,
      filtroMateria,
      filtroPeriodo,
      semPeriodos: false,
      mostrarEncerradas,
      erro: erroCatalogo,
    };
  } catch {
    return {
      total: 0,
      totalEncerradas: 0,
      pagina: 1,
      por: 50,
      turmas: [],
      turmasEncerradas: [],
      materias: [],
      periodos: [],
      filtroMateria: undefined,
      filtroPeriodo: undefined,
      semPeriodos: false,
      mostrarEncerradas,
      erro: "Sem comunicação com a API.",
    };
  }
};

export const actions: Actions = {
  criar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) {
      return fail(401, {
        erro: "Sessão encerrada.",
        materia_id: "",
        periodo_id: "",
        titulo: "",
      });
    }
    const form = await evento.request.formData();
    const materia_id = Number(form.get("materia_id"));
    const periodo_id = Number(form.get("periodo_id"));
    const titulo = String(form.get("titulo") ?? "").trim();

    if (!Number.isInteger(materia_id) || materia_id < 1) {
      return fail(422, {
        erro: "Selecione uma matéria.",
        materia_id: "",
        periodo_id: Number.isInteger(periodo_id) && periodo_id > 0 ? periodo_id : "",
        titulo,
      });
    }
    if (!Number.isInteger(periodo_id) || periodo_id < 1) {
      return fail(422, {
        erro: "Selecione o período letivo.",
        materia_id,
        periodo_id: "",
        titulo,
      });
    }

    const r = await criarTurma(evento.request, token, {
      materia_id,
      periodo_id,
      titulo,
    });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, {
        erro: mensagemDaApi(r.body),
        materia_id,
        periodo_id,
        titulo,
      });
    }
    redirect(
      303,
      `/professor/turmas/${(r.body as { id?: number } | null)?.id}`,
    );
  },
};
