import type { AtribuicaoDoAluno, TurmaDoAluno } from "#lib/server/ensino";
import { chaveDia } from "#lib/agenda-aluno";

export function turmasMock(): TurmaDoAluno[] {
  return [
    {
      id: 1,
      titulo: "Turma A",
      materia_codigo: "MAT101",
      materia_nome: "Cálculo I",
      trimestre: 4,
      ano: 2026,
      professor: "Helena Costa",
      professor_email: "helena@ufpa.br",
      ativa: true,
      created_at: "2026-09-01T12:00:00Z",
    },
    {
      id: 2,
      titulo: "T2",
      materia_codigo: "PRG101",
      materia_nome: "Programação I",
      trimestre: 4,
      ano: 2026,
      professor: "Carlos Silva",
      professor_email: "carlos@ufpa.br",
      ativa: true,
      created_at: "2026-09-01T12:00:00Z",
    },
    {
      id: 3,
      titulo: "Noite",
      materia_codigo: "FIS201",
      materia_nome: "Física Experimental",
      trimestre: 4,
      ano: 2026,
      professor: "Marcos Moraes",
      professor_email: "marcos@ufpa.br",
      ativa: true,
      created_at: "2026-09-01T12:00:00Z",
    },
    {
      id: 4,
      titulo: "T1",
      materia_codigo: "EST301",
      materia_nome: "Estatística Aplicada",
      trimestre: 4,
      ano: 2026,
      professor: "Ana Ribeiro",
      professor_email: "ana@ufpa.br",
      ativa: true,
      created_at: "2026-09-01T12:00:00Z",
    },
    {
      id: 5,
      titulo: "Antiga",
      materia_codigo: "INATIVA",
      materia_nome: "Disciplina Encerrada",
      trimestre: 2,
      ano: 2026,
      professor: "João Souza",
      professor_email: "joao@ufpa.br",
      ativa: false,
      created_at: "2026-03-01T12:00:00Z",
    },
  ];
}

/**
 * Mock para inspecionar visualmente todas as combinações da agenda.
 * Ative em dev com `/aluno?mock`. Datas são relativas a "agora" (Belém).
 */
export function atribuicoesMock(agora = new Date()): AtribuicaoDoAluno[] {
  const meiaNoite = new Date(`${chaveDia(agora)}T00:00:00-03:00`).getTime();
  const H = 3_600_000;
  const iso = (dias: number, hora: number) =>
    new Date(meiaNoite + dias * 24 * H + hora * H).toISOString();
  const agoraH = (agora.getTime() - meiaNoite) / H; // hora atual do dia

  const turmaPorMateria: Record<string, { id: number; titulo: string }> = {
    MAT101: { id: 1, titulo: "Turma A" },
    PRG101: { id: 2, titulo: "T2" },
    FIS201: { id: 3, titulo: "Noite" },
    EST301: { id: 4, titulo: "T1" },
    INATIVA: { id: 5, titulo: "Antiga" },
  };

  let id = 0;
  const mk = (
    prazo: string,
    materia: string,
    o: Partial<AtribuicaoDoAluno> = {},
  ): AtribuicaoDoAluno => {
    const t = turmaPorMateria[materia] ?? { id: 1, titulo: "Turma A" };
    return {
      id: ++id,
      turma_id: t.id,
      turma_titulo: t.titulo,
      atividade_id: id,
      autocomplete: false,
      prazo,
      pode_atrasado: false,
      materia_codigo: materia,
      turma_ativa: true,
      atividade_nome: `Lista de exercícios ${id}`,
      em_andamento: false,
      entregue: false,
      atrasada: false,
      ...o,
    };
  };
  const noPrazo = { entregue: true, atrasada: false };
  const comAtraso = { entregue: true, atrasada: true };

  return [
    // -5d: tudo entregue (esmaecida) + correção publicada
    mk(iso(-5, 10), "MAT101", {
      ...noPrazo,
      atividade_nome: "Limites e Continuidade",
      publicada_em: iso(-3, 14),
      nota_final: 9.5,
      submissao_id: 101,
    }),
    mk(iso(-5, 14), "FIS201", {
      ...comAtraso,
      atividade_nome: "Relatório de Cinemática",
      publicada_em: iso(-2, 11),
      nota_final: 7.8,
      submissao_id: 102,
    }),
    // -4d: só perdidas (esmaecida, sem laranja)
    mk(iso(-4, 9), "MAT101", { atividade_nome: "Derivadas Parciais" }),
    mk(iso(-4, 23), "PRG101", {
      ...noPrazo,
      atividade_nome: "Estruturas Condicionais",
      publicada_em: iso(-1, 16),
      nota_final: 10,
      submissao_id: 103,
    }),
    // -3d: pendente ainda entregável com atraso (esmaecida alaranjada)
    mk(iso(-3, 8), "PRG101", {
      pode_atrasado: true,
      atividade_nome: "Laços e Vetores",
    }),
    mk(iso(-3, 15), "FIS201", noPrazo),
    mk(iso(-3, 18), "MAT101"),
    // -2d: atraso aceito + perdida + entregue com atraso
    mk(iso(-2, 12), "EST301", {
      pode_atrasado: true,
      em_andamento: true,
      iniciada_em: iso(-2, 10),
      atividade_nome: "Probabilidade Condicional",
    }),
    mk(iso(-2, 13), "EST301", comAtraso),
    mk(iso(-2, 20), "PRG101"),
    // -1d: um só item perdido
    mk(iso(-1, 17), "FIS201"),
    // hoje: todos os estados possíveis
    mk(iso(0, Math.max(0, agoraH - 3)), "MAT101", noPrazo),
    mk(iso(0, Math.max(0, agoraH - 2)), "PRG101", comAtraso),
    mk(iso(0, Math.max(0, agoraH - 1.5)), "FIS201"), // perdida hoje
    mk(iso(0, Math.max(0, agoraH - 1)), "EST301", {
      pode_atrasado: true,
      atividade_nome: "Distribuição Normal",
    }), // pendente atrasado
    mk(iso(0, Math.min(23.9, agoraH + 1)), "MAT101", {
      em_andamento: true,
      iniciada_em: iso(0, Math.max(0, agoraH - 0.5)),
      duracao_seg: 5400,
      atividade_nome: "Prova Prática — Integrais",
    }), // ! hoje
    mk(iso(0, Math.min(23.9, agoraH + 2)), "PRG101", noPrazo), // entregue antecipado
    // +1d: pendentes e entregue antecipada
    mk(iso(1, 9), "MAT101", { atividade_nome: "Séries de Taylor" }),
    mk(iso(1, 11), "FIS201", noPrazo),
    mk(iso(1, 23.5), "EST301", { atividade_nome: "Teste de Hipóteses" }),
    // +2d: um item
    mk(iso(2, 15), "PRG101", { atividade_nome: "Recursão e Busca Binária" }),
    // +4d: muitas atividades no mesmo dia
    ...["MAT101", "FIS201", "PRG101", "EST301", "BIO110", "QUI120"].map((m, i) =>
      mk(iso(4, 8 + i), m, i % 2 ? noPrazo : {}),
    ),
    // turma inativa: não deve aparecer
    mk(iso(1, 10), "INATIVA", { turma_ativa: false }),
    // sem prazo: não deve aparecer na agenda por data
    mk(undefined as unknown as string, "SEMPRAZO"),
  ];
}
