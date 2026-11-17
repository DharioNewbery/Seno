<script lang="ts">
  import type { ComponentProps } from "svelte";
  import { goto } from "$app/navigation";
  import Icone from "#lib/components/Icone.svelte";
  import ListaDados from "#lib/components/ListaDados.svelte";
  import { resumirEstadoAtribuicaoAluno } from "#lib/correcao-aluno";
  import { dataHora, prazoRelativo } from "#lib/formato";
  import type { AtribuicaoDoAluno } from "#lib/server/ensino";
  import { formatarTituloCardTurmaAluno } from "#lib/turmas-aluno";
  import type { PageProps } from "./$types";

  interface ColegaLinha {
    id: number;
    nome: string;
    iniciais: string;
  }

  type ListaAtividadesProps = ComponentProps<
    typeof ListaDados<AtribuicaoDoAluno>
  >;
  type ListaColegasProps = ComponentProps<typeof ListaDados<ColegaLinha>>;

  let { data }: PageProps = $props();

  const detalhes = $derived(data.detalhes);
  const turma = $derived(detalhes?.turma);
  const atribuicoes = $derived(detalhes?.atribuicoes ?? []);
  const colegas = $derived(detalhes?.colegas ?? []);
  const colegasLinhas = $derived<ColegaLinha[]>(
    colegas.map((c, idx) => {
      const nome = `${c.first_name} ${c.last_name}`.trim();
      const iniciais = `${c.first_name?.[0] ?? ""}${c.last_name?.[0] ?? ""}`
        .trim()
        .toUpperCase();
      return { id: idx + 1, nome, iniciais: iniciais || "?" };
    }),
  );

  function subtituloAtividade(a: AtribuicaoDoAluno): string | null {
    const partes: string[] = [];
    if (a.duracao_seg) partes.push(`${Math.round(a.duracao_seg / 60)} min`);
    if (a.pode_atrasado) partes.push("atraso permitido");
    return partes.length > 0 ? partes.join(" · ") : null;
  }

  function tomEstado(
    classe: ReturnType<typeof resumirEstadoAtribuicaoAluno>["classe"],
    atrasada: boolean,
  ): "success" | "info" | "warning" | "danger" | undefined {
    if (classe === "nota") return "success";
    if (classe === "entregue") return atrasada ? "warning" : "info";
    if (classe === "andamento") return "warning";
    if (classe === "encerrada") return "danger";
    return undefined;
  }

  const colunasAtividades: ListaAtividadesProps["columns"] = [
    {
      key: "atividade_nome",
      label: "Atividade",
      sortable: true,
      primary: true,
      value: (a) => a.atividade_nome,
      sub: (a) => subtituloAtividade(a),
    },
    {
      key: "prazo",
      label: "Prazo",
      sortable: true,
      custom: true,
      value: (a) => (a.prazo ? dataHora(a.prazo) : "Sem prazo"),
    },
    {
      key: "estado",
      label: "Situação",
      sortable: true,
      custom: true,
      value: (a) => resumirEstadoAtribuicaoAluno(a).rotulo,
    },
  ];

  const filtrosAtividades: ListaAtividadesProps["filters"] = [
    { id: "todas", label: "Todas" },
    {
      id: "pendentes",
      label: "Pendentes",
      predicate: (a) => !a.entregue && Boolean(turma?.ativa),
    },
    {
      id: "entregues",
      label: "Entregues",
      predicate: (a) => a.entregue,
    },
    {
      id: "corrigidas",
      label: "Corrigidas",
      predicate: (a) => Boolean(a.entregue && a.publicada_em),
    },
  ];

  const acoesAtividades: ListaAtividadesProps["actions"] = [
    {
      id: "view",
      label: "Ver atividade",
      onselect: (a) => goto(`/aluno/atividades/${a.id}`),
    },
    {
      id: "open",
      label: "Ver submissão",
      onselect: (a) => {
        if (a.submissao_id) goto(`/aluno/submissao/${a.submissao_id}`);
      },
      hidden: (a) => !a.entregue || !a.submissao_id,
    },
  ];

  const colunasColegas: ListaColegasProps["columns"] = [
    {
      key: "nome",
      label: "Nome",
      sortable: true,
      primary: true,
      custom: true,
      value: (c) => c.nome,
    },
  ];
</script>

<section class="detalhes-turma">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  {#if turma}
    <div class="navegacao">
      <a class="voltar" href="/aluno/turmas">
        <Icone nome="chevron-left" tamanho={16} />
        <span>Voltar para Minhas turmas</span>
      </a>
    </div>

    <section class="dl-card sobre-painel">
      <header class="dl-card-head">
        <div class="sobre-titulo">
          <div class="sobre-badges">
            <span class="dl-badge" data-tone="info">{turma.materia_codigo}</span>
            <span
              class="dl-badge"
              data-tone={turma.ativa ? "success" : undefined}
            >
              {turma.ativa ? "Ativa" : "Encerrada"}
            </span>
          </div>
          <h1>{formatarTituloCardTurmaAluno(turma)}</h1>
        </div>
        <div class="sobre-acoes">
          <a
            class="dl-btn link-btn"
            href={`/aluno/atividades?turma_id=${turma.id}`}
          >
            <Icone nome="clipboard-list" tamanho={16} />
            <span>Atividades da turma</span>
          </a>
        </div>
      </header>

      <div class="metadados">
        <div class="dado">
          <span class="rotulo">Período</span>
          <span class="valor">{turma.trimestre}º Trimestre / {turma.ano}</span>
        </div>
        <div class="dado">
          <span class="rotulo">Professor(a)</span>
          <span class="valor">{turma.professor}</span>
          {#if turma.professor_email}
            <span class="email">{turma.professor_email}</span>
          {/if}
        </div>
        <div class="dado">
          <span class="rotulo">Atividades</span>
          <span class="valor">{atribuicoes.length}</span>
        </div>
        <div class="dado">
          <span class="rotulo">Colegas</span>
          <span class="valor">{colegas.length}</span>
        </div>
      </div>
    </section>

    <div class="grade-conteudo">
      <ListaDados
        title="Atividades da turma"
        subtitle={`${atribuicoes.length} ${atribuicoes.length === 1 ? "atividade atribuída" : "atividades atribuídas"}`}
        columns={colunasAtividades}
        rows={atribuicoes}
        rowKey={(a) => a.id}
        filters={filtrosAtividades}
        actions={acoesAtividades}
        searchable={atribuicoes.length > 5}
        searchPlaceholder="Buscar atividade..."
        onrowclick={(a) => goto(`/aluno/atividades/${a.id}`)}
        emptyTitle="Nenhuma atividade atribuída"
        emptyText="Nenhuma atividade atribuída a esta turma até o momento."
      >
        {#snippet cell(row, col)}
          {#if col.key === "prazo"}
            {#if row.prazo}
              {@const rel = !row.entregue ? prazoRelativo(row.prazo) : null}
              <span class="dl-main">{dataHora(row.prazo)}</span>
              {#if rel}
                <span
                  class="dl-sub"
                  class:prazo-breve={rel.tom === "breve"}
                  class:prazo-vencido={rel.tom === "vencido"}
                >
                  {rel.rotulo}
                </span>
              {/if}
            {:else}
              <span class="dl-main">Sem prazo</span>
            {/if}
          {:else if col.key === "estado"}
            {@const est = resumirEstadoAtribuicaoAluno(row)}
            <div class="estados-celula">
              <span
                class="dl-badge"
                data-tone={tomEstado(est.classe, row.atrasada)}
              >
                {est.rotulo}
              </span>
              {#if !turma.ativa && !row.entregue}
                <span class="dl-badge">Turma encerrada</span>
              {/if}
            </div>
            {#if row.entregue && row.entregue_em}
              <span class="dl-sub">Entregue em {dataHora(row.entregue_em)}</span>
            {/if}
          {/if}
        {/snippet}
      </ListaDados>

      <ListaDados
        title="Colegas de turma"
        subtitle={`${colegas.length} ${colegas.length === 1 ? "colega" : "colegas"}`}
        columns={colunasColegas}
        rows={colegasLinhas}
        rowKey={(c) => c.id}
        searchable={colegasLinhas.length > 5}
        searchPlaceholder="Buscar colega..."
        pageSize={10}
        emptyTitle="Nenhum colega matriculado"
        emptyText="Ainda não há outros alunos matriculados nesta turma."
      >
        {#snippet cell(row, col)}
          {#if col.key === "nome"}
            <div class="colega-celula">
              <span class="avatar-iniciais" aria-hidden="true">
                {row.iniciais}
              </span>
              <span class="dl-main">{row.nome}</span>
            </div>
          {/if}
        {/snippet}
      </ListaDados>
    </div>
  {/if}
</section>

<style>
  .detalhes-turma {
    display: flex;
    flex-direction: column;
    gap: 1.15rem;
  }

  .navegacao {
    margin-bottom: -0.25rem;
  }

  .dl-card {
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
    overflow: hidden;
  }

  .dl-card-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    flex-wrap: wrap;
    padding: 16px 20px;
    border-bottom: 1px solid var(--seno-line);
  }

  .sobre-titulo {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .sobre-badges {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .sobre-titulo h1 {
    margin: 0;
    font-size: 1.3rem;
    font-weight: 600;
    color: var(--seno-blue-900);
  }

  .sobre-acoes {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .link-btn {
    text-decoration: none;
  }

  .link-btn:hover {
    color: var(--seno-white);
  }

  .metadados {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 16px 24px;
    padding: 16px 20px;
    background: var(--seno-gray-50);
  }

  .dado {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .dado .rotulo {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--seno-gray-500);
  }

  .dado .valor {
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--seno-gray-900);
  }

  .dado .email {
    font-size: 0.78rem;
    color: var(--seno-gray-500);
  }

  .grade-conteudo {
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
    gap: 1.15rem;
    align-items: start;
  }

  @media (max-width: 900px) {
    .grade-conteudo {
      grid-template-columns: 1fr;
    }
  }

  .estados-celula {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .prazo-breve {
    color: var(--seno-amber);
    font-weight: 500;
  }

  .prazo-vencido {
    color: var(--seno-red);
    font-weight: 500;
  }

  .colega-celula {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .avatar-iniciais {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: 999px;
    background: var(--seno-blue-100);
    color: var(--seno-blue-700);
    font-size: 0.72rem;
    font-weight: 600;
    flex: none;
  }
</style>
