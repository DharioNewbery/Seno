<script lang="ts">
  import type { ComponentProps } from "svelte";
  import { goto } from "$app/navigation";
  import ListaDados from "#lib/components/ListaDados.svelte";
  import type { TurmaDoAluno } from "#lib/server/ensino";
  import {
    filtrarTurmasAtivasAluno,
    formatarTituloCardTurmaAluno,
  } from "#lib/turmas-aluno";
  import type { PageProps } from "./$types";

  type ListaTurmasProps = ComponentProps<typeof ListaDados<TurmaDoAluno>>;

  let { data }: PageProps = $props();

  const turmas = $derived(filtrarTurmasAtivasAluno(data.turmas));

  const colunas: ListaTurmasProps["columns"] = [
    {
      key: "turma",
      label: "Turma",
      sortable: true,
      primary: true,
      value: (t) => formatarTituloCardTurmaAluno(t),
      sub: (t) => t.materia_codigo,
    },
    {
      key: "periodo",
      label: "Período",
      sortable: true,
      value: (t) => `${t.trimestre}º Trimestre / ${t.ano}`,
    },
    {
      key: "professor",
      label: "Professor(a)",
      sortable: true,
      value: (t) => t.professor,
      sub: (t) => t.professor_email || null,
    },
  ];

  const acoes: ListaTurmasProps["actions"] = [
    {
      id: "view",
      label: "Ver turma",
      onselect: (t) => goto(`/aluno/turmas/${t.id}`),
    },
    {
      id: "open",
      label: "Ver atividades",
      onselect: (t) => goto(`/aluno/atividades?turma_id=${t.id}`),
    },
  ];
</script>

<section class="minhas-turmas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <ListaDados
    title="Minhas turmas"
    subtitle={`${turmas.length} ${turmas.length === 1 ? "turma ativa" : "turmas ativas"}`}
    columns={colunas}
    rows={turmas}
    rowKey={(t) => t.id}
    actions={acoes}
    searchPlaceholder="Buscar turma ou professor..."
    onrowclick={(t) => goto(`/aluno/turmas/${t.id}`)}
    emptyTitle="Nenhuma turma ativa"
    emptyText="Você ainda não está matriculado em nenhuma turma ativa."
  />
</section>

<style>
  .minhas-turmas {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
</style>
