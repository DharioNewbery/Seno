<script lang="ts">
  import type { ComponentProps } from "svelte";
  import { goto } from "$app/navigation";
  import ListaDados from "#lib/components/ListaDados.svelte";
  import { soData } from "#lib/formato";
  import type { Atividade } from "#lib/server/ensino";
  import type { PageProps } from "./$types";

  type ListaAtividadesProps = ComponentProps<typeof ListaDados<Atividade>>;

  let { data, form }: PageProps = $props();

  const atividades = $derived(data.atividades);

  let modalCriarAberto = $state(false);
  let nomeNova = $state("");
  let excluindo = $state<Atividade | null>(null);
  let formDuplicar = $state<HTMLFormElement | null>(null);
  let idDuplicar = $state<number | null>(null);

  $effect(() => {
    if (form?.erroCriar) {
      modalCriarAberto = true;
      if ("nomeCriar" in form && typeof form.nomeCriar === "string") {
        nomeNova = form.nomeCriar;
      }
    }
  });

  function abrirModalCriar() {
    nomeNova = "";
    modalCriarAberto = true;
  }

  function fecharModalCriar() {
    modalCriarAberto = false;
  }

  function dispararDuplicar(a: Atividade) {
    idDuplicar = a.id;
    queueMicrotask(() => formDuplicar?.requestSubmit());
  }

  const colunas: ListaAtividadesProps["columns"] = [
    {
      key: "nome",
      label: "Atividade",
      sortable: true,
      primary: true,
      value: (a) => a.nome,
    },
    {
      key: "situacao",
      label: "Situação",
      sortable: true,
      custom: true,
      value: (a) => (a.bloqueada ? "Em uso · Somente leitura" : "Editável"),
    },
    {
      key: "created_at",
      label: "Criada em",
      sortable: true,
      value: (a) => soData(a.created_at),
    },
  ];

  const filtros: ListaAtividadesProps["filters"] = [
    { id: "todas", label: "Todas" },
    {
      id: "editaveis",
      label: "Editáveis",
      predicate: (a) => !a.bloqueada,
    },
    {
      id: "em_uso",
      label: "Em uso",
      predicate: (a) => Boolean(a.bloqueada),
    },
  ];

  const acoes: ListaAtividadesProps["actions"] = [
    {
      id: "view",
      label: "Abrir",
      onselect: (a) => goto(`/professor/atividades/${a.id}`),
    },
    {
      id: "duplicate",
      label: "Duplicar",
      onselect: (a) => dispararDuplicar(a),
    },
    {
      id: "delete",
      label: "Excluir",
      hidden: (a) => Boolean(a.bloqueada),
      onselect: (a) => (excluindo = a),
    },
  ];
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === "Escape") {
      if (modalCriarAberto) fecharModalCriar();
      if (excluindo) excluindo = null;
    }
  }}
/>

<section class="atividades">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}

  <ListaDados
    title="Banco de atividades"
    subtitle={`${atividades.length} ${atividades.length === 1 ? "atividade" : "atividades"}`}
    columns={colunas}
    rows={atividades}
    rowKey={(a) => a.id}
    filters={filtros}
    actions={acoes}
    searchPlaceholder="Buscar atividade..."
    onrowclick={(a) => goto(`/professor/atividades/${a.id}`)}
    emptyTitle="Nenhuma atividade criada ainda"
    emptyText="Crie sua primeira atividade para organizar tarefas e atribuir às turmas."
  >
    {#snippet header()}
      <button type="button" class="nova" onclick={abrirModalCriar}>
        Criar atividade +
      </button>
    {/snippet}

    {#snippet cell(row, col)}
      {#if col.key === "situacao"}
        <span class="dl-badge" data-tone={row.bloqueada ? "warning" : "success"}>
          {row.bloqueada ? "Em uso · Somente leitura" : "Editável"}
        </span>
      {/if}
    {/snippet}
  </ListaDados>

  <form method="POST" action="?/duplicar" bind:this={formDuplicar} hidden>
    <input type="hidden" name="id" value={idDuplicar ?? ""} />
  </form>
</section>

{#if modalCriarAberto}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModalCriar();
    }}
  >
    <div
      class="modal-card"
      role="dialog"
      aria-modal="true"
      aria-labelledby="titulo-nova-atividade"
    >
      <div class="modal-cabeca">
        <h3 id="titulo-nova-atividade">Nova atividade</h3>
        <button
          type="button"
          class="btn-fechar"
          onclick={fecharModalCriar}
          aria-label="Fechar"
        >
          ✕
        </button>
      </div>
      <form method="POST" action="?/criar" class="modal-form">
        {#if form?.erroCriar}
          <p class="aviso erro" role="alert">{form.erroCriar}</p>
        {/if}
        <label class="campo-modal">
          <span>Nome da atividade</span>
          <input
            name="nome"
            bind:value={nomeNova}
            required
            maxlength="128"
            placeholder="Ex: Prova 1"
          />
        </label>
        <p class="dica">
          A atividade será criada vazia e você irá direto para a tela de edição
          para importar e organizar as tarefas.
        </p>
        <div class="modal-acoes">
          <button type="button" class="peq" onclick={fecharModalCriar}>
            Cancelar
          </button>
          <button type="submit" class="nova">Criar e editar</button>
        </div>
      </form>
    </div>
  </div>
{/if}

{#if excluindo}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) excluindo = null;
    }}
  >
    <div
      class="modal-card"
      role="dialog"
      aria-modal="true"
      aria-labelledby="titulo-excluir-atividade"
    >
      <div class="modal-cabeca">
        <h3 id="titulo-excluir-atividade">Excluir atividade</h3>
        <button
          type="button"
          class="btn-fechar"
          onclick={() => (excluindo = null)}
          aria-label="Fechar"
        >
          ✕
        </button>
      </div>
      <form method="POST" action="?/excluir" class="modal-form">
        <input type="hidden" name="id" value={excluindo.id} />
        <p class="dica">
          Excluir definitivamente <strong>{excluindo.nome}</strong>?
        </p>
        <div class="modal-acoes">
          <button
            type="button"
            class="peq"
            onclick={() => (excluindo = null)}
          >
            Cancelar
          </button>
          <button type="submit" class="peq vermelho">Excluir</button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .atividades {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }

  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(15, 23, 42, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    z-index: 60;
  }

  .modal-card {
    width: 100%;
    max-width: 28rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-lg);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    box-shadow: 0 18px 40px rgba(15, 23, 42, 0.2);
  }

  .modal-cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .modal-cabeca h3 {
    margin: 0;
  }

  .btn-fechar {
    background: transparent;
    border: none;
    color: var(--seno-gray-600);
    font-size: 1rem;
    cursor: pointer;
    padding: 0.2rem 0.4rem;
  }

  .modal-form {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }

  .campo-modal {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .campo-modal span {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  .modal-acoes {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }
</style>
