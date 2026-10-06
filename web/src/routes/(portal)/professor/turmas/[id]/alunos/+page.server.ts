import { error, fail, redirect, type Actions } from "@sveltejs/kit";

import {
  encerrarMatricula,
  listarAlunos,
  listarMatriculas,
  matricular,
  mensagemDaApi,
  verTurma,
  type Aluno,
  type ListaMatriculas,
  type Turma as TurmaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import type { PageServerLoad } from "./$types";

// Alunos da turma: matrículas (padrão: ativas), pesquisa de aluno por
// e-mail/nome para matricular, matrícula individual e em lote
// (linhas "email,nome,sobrenome"); remoção = encerrar matrícula.
export const load: PageServerLoad = async ({
  locals,
  cookies,
  request,
  params,
  url,
}) => {
  if (!locals.usuario) redirect(303, "/");
  const id = Number(params.id);
  if (!Number.isInteger(id) || id < 1) {
    error(404, "Turma não encontrada.");
  }
  const token = tokenDaSessao(cookies);
  const incluir = url.searchParams.get("incluir") === "true";
  const busca = (url.searchParams.get("q") ?? "").trim();

  try {
    const turmaR = await verTurma(request, token, id);
    if (turmaR.status === 401) redirect(303, "/");
    if (turmaR.status === 404) error(404, "Turma não encontrada.");
    if (!turmaR.ok) {
      return {
        turma: null,
        matriculas: [],
        busca,
        incluir,
        resultados: [],
        erro: mensagemDaApi(turmaR.body),
      };
    }
    const matsR = await listarMatriculas(request, token, id, incluir);
    let resultados: Aluno[] = [];
    if (busca) {
      const r = await listarAlunos(request, token, busca);
      if (r.ok) {
        resultados = (r.body as { usuarios?: Aluno[] } | null)?.usuarios ?? [];
      }
    }
    return {
      turma: turmaR.body as TurmaApi,
      matriculas: matsR.ok
        ? ((matsR.body as ListaMatriculas)?.matriculas ?? [])
        : [],
      busca,
      incluir,
      resultados,
      erro: matsR.ok ? null : mensagemDaApi(matsR.body),
    };
  } catch (e) {
    if (e && (e as { status?: number }).status) throw e;
    return {
      turma: null,
      matriculas: [],
      busca,
      incluir,
      resultados: [],
      erro: "Sem comunicação com a API.",
    };
  }
};

export const actions: Actions = {
  matricular: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const form = await evento.request.formData();
    const email = String(form.get("email") ?? "").trim();
    const first_name = String(form.get("first_name") ?? "").trim();
    const last_name = String(form.get("last_name") ?? "").trim();
    if (!email) return fail(422, { erro: "Informe o e-mail do aluno." });

    const r = await matricular(evento.request, token, id, {
      email,
      ...(first_name ? { first_name } : {}),
      ...(last_name ? { last_name } : {}),
    });
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: `${email} matriculada/o.` };
  },

  matricular_lote: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const bruto = String((await evento.request.formData()).get("lote") ?? "");
    const linhas = bruto
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean);
    if (!linhas.length) {
      return fail(422, { erro: "Cole pelo menos uma linha." });
    }

    let ok = 0;
    const falhas: string[] = [];
    for (const linha of linhas) {
      const partes = linha
        .split(/[,;\t]/)
        .map((p) => p.trim())
        .filter(Boolean);
      const email = partes[0];
      const first_name = partes[1];
      const last_name = partes[2];
      if (!email || !email.includes("@")) {
        falhas.push(`${linha}: e-mail inválido.`);
        continue;
      }
      try {
        const r = await matricular(evento.request, token, id, {
          email,
          ...(first_name ? { first_name } : {}),
          ...(last_name ? { last_name } : {}),
        });
        if (r.ok) {
          ok++;
          continue;
        }
        falhas.push(`${email}: ${mensagemDaApi(r.body)}`);
      } catch {
        falhas.push(`${email}: sem comunicação com a API.`);
      }
    }

    if (!ok) {
      return fail(422, { erro: "Nenhuma matrícula concluída.", falhas });
    }
    return {
      ok: `${ok} matriculada/o(s) com sucesso.`,
      ...(falhas.length ? { falhas } : {}),
    };
  },

  remover: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const id = Number(form.get("matricula_id"));
    if (!Number.isInteger(id) || id < 1) {
      return fail(422, { erro: "Identificador inválido." });
    }
    const r = await encerrarMatricula(evento.request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Matrícula encerrada (aluno removido)." };
  },
};
