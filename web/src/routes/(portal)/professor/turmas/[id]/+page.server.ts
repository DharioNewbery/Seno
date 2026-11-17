import { error, fail, redirect, type Actions } from "@sveltejs/kit";

import {
  acaoTurma,
  conferirMatriculas,
  criarAtribuicao,
  encerrarMatricula,
  excluirAtribuicao,
  listarAtribuicoes,
  listarAtividades,
  listarMatriculas,
  matricular,
  mensagemDaApi,
  reenviarConviteAluno,
  renomearTurma,
  verAtividadePorId,
  verTurma,
  type Atribuicao,
  type ConferenciaMatriculas,
  type ListaMatriculas,
  type Turma as TurmaApi,
} from "#lib/server/ensino";
import { tokenDaSessao } from "#lib/server/sessao";
import {
  parseCsvAlunos,
  type AtividadeComPreview,
  type CandidatoMatricula,
  type TarefaPreviewAtividade,
} from "#lib/turma-painel";
import type { PageServerLoad } from "./$types";

// Painel da turma dividido em 3 seções: Alunos (listagem, reenvio de convite
// vencido e modal de matrícula individual/CSV com conferência prévia),
// Atividades (listagem de atribuídas + grid de atribuição com modal de preview)
// e Configurações (modal para renomear, encerrar ou excluir turma).
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
  const incluirEncerradas = url.searchParams.get("incluir") === "true";
  const token = tokenDaSessao(cookies);

  try {
    const r = await verTurma(request, token, id);
    if (r.status === 401) redirect(303, "/");
    if (r.status === 404) error(404, "Turma não encontrada.");
    if (!r.ok) {
      return {
        turma: null,
        matriculas: [],
        incluirEncerradas,
        atribuicoes: [],
        atividades: [],
        erro: mensagemDaApi(r.body),
      };
    }

    const [rm, ra, rtv] = await Promise.all([
      listarMatriculas(request, token, id, incluirEncerradas),
      listarAtribuicoes(request, token, { turma_id: id }),
      listarAtividades(request, token),
    ]);

    const matriculas = rm.ok
      ? ((rm.body as ListaMatriculas | null)?.matriculas ?? [])
      : [];
    const atribuicoes =
      (ra.body as { atribuicoes?: Atribuicao[] } | null)?.atribuicoes ?? [];
    const basicas =
      (
        rtv.body as {
          atividades?: { id: number; nome: string; created_at?: string }[];
        } | null
      )?.atividades ?? [];

    const atividades: AtividadeComPreview[] = await Promise.all(
      basicas.map(async (b) => {
        try {
          const det = await verAtividadePorId(request, token, b.id);
          if (det.ok && det.body) {
            const corpo = det.body as {
              id: number;
              nome: string;
              created_at?: string;
              tarefas?: TarefaPreviewAtividade[];
            };
            return {
              id: corpo.id ?? b.id,
              nome: corpo.nome ?? b.nome,
              created_at: corpo.created_at ?? b.created_at,
              tarefas: corpo.tarefas ?? [],
            };
          }
        } catch {
          // Mantém dados básicos se falhar o carregamento individual
        }
        return {
          id: b.id,
          nome: b.nome,
          created_at: b.created_at,
          tarefas: [],
        };
      }),
    );

    return {
      turma: r.body as TurmaApi,
      matriculas,
      incluirEncerradas,
      atribuicoes,
      atividades,
      erro: null,
    };
  } catch (e) {
    if (e && (e as { status?: number }).status) throw e;
    return {
      turma: null,
      matriculas: [],
      incluirEncerradas,
      atribuicoes: [],
      atividades: [],
      erro: "Sem comunicação com a API.",
    };
  }
};

export const actions: Actions = {
  conferir_alunos: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erroAlunos: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const form = await evento.request.formData();
    const modo = String(form.get("modo") ?? "individual");

    let candidatos: CandidatoMatricula[];
    if (modo === "lote") {
      const lote = String(form.get("lote") ?? "");
      candidatos = parseCsvAlunos(lote);
      if (candidatos.length === 0) {
        return fail(422, {
          erroAlunos: "Informe ou carregue pelo menos uma linha CSV válida.",
          abrirModalAlunos: true,
          modoAlunos: "lote",
        });
      }
    } else {
      const email = String(form.get("email") ?? "")
        .trim()
        .toLowerCase();
      const first_name = String(form.get("first_name") ?? "").trim();
      const last_name = String(form.get("last_name") ?? "").trim();
      if (!email || !email.includes("@")) {
        return fail(422, {
          erroAlunos: "Informe um e-mail válido para o aluno.",
          abrirModalAlunos: true,
          modoAlunos: "individual",
        });
      }
      candidatos = [{ email, first_name, last_name }];
    }

    const r = await conferirMatriculas(evento.request, token, id, candidatos);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, {
        erroAlunos: mensagemDaApi(r.body),
        abrirModalAlunos: true,
        modoAlunos: modo,
      });
    }

    return {
      conferencia: r.body as ConferenciaMatriculas,
      abrirModalAlunos: true,
      modoAlunos: modo,
    };
  },

  confirmar_matriculas: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erroAlunos: "Sessão encerrada." });
    const id = Number(evento.params.id);
    const form = await evento.request.formData();
    const bruto = String(form.get("candidatos_json") ?? "");

    let candidatos: CandidatoMatricula[];
    try {
      const lido = JSON.parse(bruto) as unknown;
      if (!Array.isArray(lido)) {
        return fail(422, {
          erroAlunos: "Lista de confirmação inválida.",
          abrirModalAlunos: true,
        });
      }
      candidatos = lido
        .map((item) => {
          const obj = item as Record<string, unknown>;
          return {
            email: String(obj.email ?? "").trim(),
            first_name: String(obj.first_name ?? "").trim(),
            last_name: String(obj.last_name ?? "").trim(),
          };
        })
        .filter((c) => c.email.length > 0);
    } catch {
      return fail(422, {
        erroAlunos: "Lista de confirmação inválida.",
        abrirModalAlunos: true,
      });
    }

    if (candidatos.length === 0) {
      return fail(422, {
        erroAlunos: "Nenhum aluno apto selecionado para matrícula.",
        abrirModalAlunos: true,
      });
    }

    let okCount = 0;
    const falhas: string[] = [];

    for (const c of candidatos) {
      try {
        const r = await matricular(evento.request, token, id, {
          email: c.email,
          ...(c.first_name ? { first_name: c.first_name } : {}),
          ...(c.last_name ? { last_name: c.last_name } : {}),
        });
        if (r.status === 401) redirect(303, "/");
        if (r.ok) {
          okCount++;
        } else {
          falhas.push(`${c.email}: ${mensagemDaApi(r.body)}`);
        }
      } catch {
        falhas.push(`${c.email}: sem comunicação com a API.`);
      }
    }

    if (okCount === 0) {
      return fail(422, {
        erroAlunos: "Nenhuma matrícula pôde ser concluída.",
        falhas,
        abrirModalAlunos: true,
      });
    }

    return {
      ok:
        okCount === 1
          ? "1 aluno matriculado com sucesso."
          : `${okCount} alunos matriculados com sucesso.`,
      ...(falhas.length ? { falhas } : {}),
    };
  },

  reenviar_convite: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const turmaID = Number(evento.params.id);
    const form = await evento.request.formData();
    const alunoID = Number(form.get("aluno_id"));
    if (!Number.isInteger(alunoID) || alunoID < 1) {
      return fail(422, { erro: "Identificador de aluno inválido." });
    }
    const r = await reenviarConviteAluno(
      evento.request,
      token,
      turmaID,
      alunoID,
    );
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Novo convite enviado para o aluno." };
  },

  remover_aluno: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const matriculaID = Number(form.get("matricula_id"));
    if (!Number.isInteger(matriculaID) || matriculaID < 1) {
      return fail(422, { erro: "Identificador de matrícula inválido." });
    }
    const r = await encerrarMatricula(evento.request, token, matriculaID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Matrícula encerrada (aluno removido)." };
  },

  renomear: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) {
      return fail(401, {
        erroConfig: "Sessão encerrada.",
        abrirModalConfig: true,
      });
    }
    const id = Number(evento.params.id);
    const titulo = String(
      (await evento.request.formData()).get("titulo") ?? "",
    ).trim();
    const r = await renomearTurma(evento.request, token, id, titulo);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, {
        erroConfig: mensagemDaApi(r.body),
        abrirModalConfig: true,
      });
    }
    return { ok: "Título da turma atualizado." };
  },

  encerrar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) {
      return fail(401, {
        erroConfig: "Sessão encerrada.",
        abrirModalConfig: true,
      });
    }
    const id = Number(evento.params.id);
    const r = await acaoTurma(evento.request, token, id, "encerrar");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, {
        erroConfig: mensagemDaApi(r.body),
        abrirModalConfig: true,
      });
    }
    return { ok: "Turma encerrada." };
  },

  excluir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) {
      return fail(401, {
        erroConfig: "Sessão encerrada.",
        abrirModalConfig: true,
      });
    }
    const id = Number(evento.params.id);
    const r = await acaoTurma(evento.request, token, id, "excluir");
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, {
        erroConfig: mensagemDaApi(r.body),
        abrirModalConfig: true,
      });
    }
    redirect(303, "/professor/turmas");
  },

  atribuir: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const turmaID = Number(evento.params.id);
    const form = await evento.request.formData();
    const atividadeID = Number(form.get("atividade_id"));
    if (!Number.isInteger(atividadeID) || atividadeID < 1) {
      return fail(422, {
        erro: "Selecione uma atividade do grid para atribuir.",
      });
    }
    const prazoBruto = String(form.get("prazo") ?? "").trim();
    const prazoData = prazoBruto !== "" ? new Date(prazoBruto) : null;
    if (!prazoData || Number.isNaN(prazoData.getTime())) {
      return fail(422, {
        erro: "Informe a data máxima de entrega.",
      });
    }
    const inicioBruto = String(form.get("inicio") ?? "").trim();
    const inicioData = inicioBruto !== "" ? new Date(inicioBruto) : null;
    if (inicioData && Number.isNaN(inicioData.getTime())) {
      return fail(422, {
        erro: "Data de início inválida.",
      });
    }
    if (inicioData && prazoData < inicioData) {
      return fail(422, {
        erro: "Prazo de entrega não pode ser anterior ao início.",
      });
    }
    const cad: {
      turma_id: number;
      atividade_id: number;
      autocomplete?: boolean;
      inicio?: string;
      prazo: string;
      duracao_seg?: number;
      pode_atrasado?: boolean;
    } = {
      turma_id: turmaID,
      atividade_id: atividadeID,
      prazo: prazoData.toISOString(),
    };
    if (inicioData) cad.inicio = inicioData.toISOString();
    const duracaoHoras = Number(form.get("duracao_horas"));
    if (Number.isFinite(duracaoHoras) && duracaoHoras > 0) {
      cad.duracao_seg = Math.floor(duracaoHoras * 3600);
    }
    if (form.get("pode_atrasado") === "on") cad.pode_atrasado = true;
    const autocerto = String(form.get("autocomplete") ?? "");
    if (autocerto === "on") cad.autocomplete = true;
    if (autocerto === "off") cad.autocomplete = false;
    const r = await criarAtribuicao(evento.request, token, cad);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Atividade atribuída à turma." };
  },

  desligar: async (evento) => {
    const token = tokenDaSessao(evento.cookies);
    if (!token) return fail(401, { erro: "Sessão encerrada." });
    const form = await evento.request.formData();
    const atribuicaoID = Number(form.get("atribuicao_id"));
    const r = await excluirAtribuicao(evento.request, token, atribuicaoID);
    if (r.status === 401) redirect(303, "/");
    if (!r.ok) {
      return fail(r.status || 500, { erro: mensagemDaApi(r.body) });
    }
    return { ok: "Atribuição removida." };
  },

  editarConfig: async (evento) => {
    const form = await evento.request.formData();
    const atribuicaoID = Number(form.get("atribuicao_id"));
    redirect(303, `/professor/atribuicoes/${atribuicaoID}`);
  },
};
