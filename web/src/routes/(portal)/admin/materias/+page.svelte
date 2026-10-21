<script lang="ts">
  // Matérias: listagem com edição inline, exclusão e atalho para as
  // turmas da matéria. Criação em página própria (/admin/materias/nova).
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const materias = $derived(data.materias);

  let editandoId = $state<number | null>(null);
  let codigo = $state("");
  let nome = $state("");

  function comecarEdicao(m: (typeof materias)[number]) {
    editandoId = m.id;
    codigo = m.codigo;
    nome = m.nome;
  }

  function cancelar() {
    editandoId = null;
    codigo = "";
    nome = "";
  }

  function turmaURL(id: number): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    qs.set("materia_id", String(id));
    qs.delete("pagina");
    return `/admin/turmas?${qs.toString()}`;
  }
</script>

<section class="materias">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}

  <div class="cabeca">
    <h2>Matérias <span class="total">({materias.length})</span></h2>
    <a class="nova" href="/admin/materias/nova">Nova matéria +</a>
  </div>

  <ul class="lista">
    {#each materias as m (m.id)}
      <li>
        {#if editandoId === m.id}
          <form method="POST" action="?/editar" class="edicao">
            <input type="hidden" name="id" value={m.id} />
            <input name="codigo" bind:value={codigo} required />
            <input name="nome" bind:value={nome} required />
            <button>Salvar</button>
            <button type="button" class="sec" onclick={cancelar}>Cancelar</button>
          </form>
        {:else}
          <div class="linha">
            <div class="identidade">
              <strong>{m.codigo}</strong>
              <span class="nome">{m.nome}</span>
            </div>
            <div class="acoes">
              <a class="link" href={turmaURL(m.id)}>Turmas →</a>
              <button class="peq" onclick={() => comecarEdicao(m)}>Editar</button>
              <form method="POST" action="?/excluir">
                <input type="hidden" name="id" value={m.id} />
                <button class="peq vermelho">Excluir</button>
              </form>
            </div>
          </div>
        {/if}
      </li>
    {:else}
      <li class="vazio">Nenhuma matéria. Comece criando a primeira.</li>
    {/each}
  </ul>
</section>

<style>
  .materias {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .cabeca h2 {
    margin: 0;
  }
                .linha {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .identidade {
    display: flex;
    align-items: baseline;
    gap: 0.7rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
  .nome {
    color: var(--seno-gray-700);
  }
  .acoes {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .acoes form {
    display: contents;
  }
  .link {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
  .peq,
  .edicao button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.35rem 0.75rem;
    font-size: 0.82rem;
    cursor: pointer;
  }
  .peq:hover,
  .edicao button:hover {
    background: var(--seno-blue-700);
  }
      .edicao {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .edicao input {
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    padding: 0.45rem 0.6rem;
    font-size: 0.88rem;
  }
  </style>
