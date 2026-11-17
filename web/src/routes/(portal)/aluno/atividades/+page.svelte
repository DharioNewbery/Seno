<script lang="ts">
  import type { ComponentProps } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import ListaDados from "#lib/components/ListaDados.svelte";
  import { POR_PAGINA_ATIVIDADES_ALUNO } from "#lib/atividades-aluno";
  import { resumirEstadoAtribuicaoAluno } from "#lib/correcao-aluno";
  import { dataHora, prazoRelativo } from "#lib/formato";
  import type { AtribuicaoDoAluno } from "#lib/server/ensino";
  import type { PageProps } from "./$types";

  type ListaAtividadesProps = ComponentProps<
    typeof ListaDados<AtribuicaoDoAluno>
  >;

  let { data }: PageProps = $props();

  const turmasAtivas = $derived(data.turmas.filter((t) => t.ativa));

  const colunas: ListaAtividadesProps["columns"] = [
    {
      key: "atividade_nome",
      label: "Atividade",
      sortable: true,
      primary: true,
      value: (a) => a.atividade_nome,
    },
    {
      key: "turma",
      label: "Turma",
      sortable: true,
      value: (a) => `${a.materia_codigo} (${a.turma_titulo || `#${a.turma_id}`})`,
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

  const filtros: ListaAtividadesProps["filters"] = [
    { id: "todas", label: "Todas" },
    {
      id: "pendentes",
      label: "Pendentes",
      predicate: (a) => !a.entregue && a.turma_ativa,
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

  const acoes: ListaAtividadesProps["actions"] = [
    {
      id: "view",
      label: "Ver atividade",
      onselect: (a) => goto(`/aluno/atividades/${a.id}`),
    },
  ];

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

  function montarUrl(turmaId: number | null): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    if (turmaId === null) qs.delete("turma_id");
    else qs.set("turma_id", String(turmaId));
    qs.delete("pagina");
    const query = qs.toString();
    return query ? `/aluno/atividades?${query}` : "/aluno/atividades";
  }

  function aoMudarTurma(
    e: Event & { currentTarget: EventTarget & { value: string } },
  ) {
    const valor = e.currentTarget.value;
    const turmaId = valor ? Number(valor) : null;
    goto(montarUrl(turmaId));
  }
</script>

<section class="minhas-atividades">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <ListaDados
    title="Atividades"
    subtitle={`${data.atribuicoes.length} ${data.atribuicoes.length === 1 ? "atividade" : "atividades"}`}
    columns={colunas}
    rows={data.atribuicoes}
    rowKey={(a) => a.id}
    filters={filtros}
    actions={acoes}
    searchPlaceholder="Buscar atividade ou turma..."
    pageSize={POR_PAGINA_ATIVIDADES_ALUNO}
    onrowclick={(a) => goto(`/aluno/atividades/${a.id}`)}
    emptyTitle="Nenhuma atividade disponível"
    emptyText="As atribuições aparecem quando o professor anexa uma atividade à sua turma."
  >
    {#snippet toolbar()}
      <div class="filtro-turma">
        <label for="filtro-turma-select">Turma</label>
        <select
          id="filtro-turma-select"
          value={data.filtroTurma ? String(data.filtroTurma) : ""}
          onchange={aoMudarTurma}
        >
          <option value="">Todas as turmas</option>
          {#each turmasAtivas as t (t.id)}
            <option value={String(t.id)}>
              {t.materia_codigo} ({t.titulo || `#${t.id}`})
            </option>
          {/each}
        </select>
      </div>
    {/snippet}

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
          <span class="dl-badge" data-tone={tomEstado(est.classe, row.atrasada)}>
            {est.rotulo}
          </span>
          {#if !row.turma_ativa && !row.entregue}
            <span class="dl-badge">Turma encerrada</span>
          {/if}
        </div>
      {/if}
    {/snippet}
  </ListaDados>
</section>

<style>
  .minhas-atividades {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }

  .filtro-turma {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .filtro-turma label {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--seno-gray-500);
  }

  .filtro-turma select {
    height: 36px;
    padding: 0 0.65rem;
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-sm);
    background: var(--seno-white);
    font: inherit;
    font-size: 0.875rem;
    color: var(--seno-gray-900);
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
</style>
