<script lang="ts">
  // Banco de tarefas: listagem com cards expansíveis e ações por ícones.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const tarefas = $derived(data.tarefas);

  let confirmando = $state<number | null>(null);
  let expandidos = $state<Record<number, boolean>>({});

  function alternar(id: number) {
    expandidos[id] = !expandidos[id];
  }

  function paginadora(delta: number): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    qs.set("pagina", String(data.pagina + delta));
    return `?${qs.toString()}`;
  }
</script>

<section class="tarefas">
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
    <h2>Banco de tarefas <span class="total">({data.total})</span></h2>
    <a class="nova" href="/professor/tarefas/nova">Criar tarefa +</a>
  </div>

  <ul class="lista">
    {#each tarefas as t (t.id)}
      <li class="item-tarefa" class:expandido={expandidos[t.id]}>
        <div
          class="card-topo"
          role="button"
          tabindex="0"
          aria-expanded={Boolean(expandidos[t.id])}
          onclick={() => alternar(t.id)}
          onkeydown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              alternar(t.id);
            }
          }}
        >
          <div class="identidade">
            <svg
              class="chevron"
              class:rotacionado={expandidos[t.id]}
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <polyline points="9 6 15 12 9 18" />
            </svg>
            <span class="nome">{t.nome}</span>
          </div>

          <div
            class="acoes"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.stopPropagation()}
            role="toolbar"
            aria-label="Ações da tarefa"
            tabindex="-1"
          >
            <form method="POST" action="?/duplicar">
              <input type="hidden" name="id" value={t.id} />
              <button
                type="submit"
                class="btn-icone"
                title="Duplicar tarefa"
                aria-label="Duplicar tarefa"
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                </svg>
              </button>
            </form>

            {#if confirmando === t.id}
              <form method="POST" action="?/excluir" class="confirma">
                <span>Excluir?</span>
                <input type="hidden" name="id" value={t.id} />
                <button type="submit" class="peq vermelho">Sim</button>
                <button
                  type="button"
                  class="peq"
                  onclick={() => (confirmando = null)}
                >
                  Não
                </button>
              </form>
            {:else}
              <button
                type="button"
                class="btn-icone vermelho"
                title="Excluir tarefa"
                aria-label="Excluir tarefa"
                onclick={() => (confirmando = t.id)}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M3 6h18" />
                  <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                  <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                  <line x1="10" y1="11" x2="10" y2="17" />
                  <line x1="14" y1="11" x2="14" y2="17" />
                </svg>
              </button>
            {/if}
          </div>
        </div>

        {#if expandidos[t.id]}
          <div class="card-detalhes">
            <div class="enunciado-bloco">
              <span class="enunciado-rotulo">Enunciado</span>
              {#if t.enunciado && t.enunciado.trim()}
                <p class="enunciado-texto">{t.enunciado}</p>
              {:else}
                <p class="enunciado-vazio">Sem enunciado cadastrado.</p>
              {/if}
            </div>

            <div class="rodape-detalhes">
              <a class="btn-detalhes" href={`/professor/tarefas/${t.id}`}>
                Ver detalhes →
              </a>
            </div>
          </div>
        {/if}
      </li>
    {:else}
      <li class="vazio">Nenhuma tarefa. Crie a primeira questão.</li>
    {/each}
  </ul>

  <div class="paginacao">
    <a href={paginadora(-1)} class:desab={data.pagina <= 1}>← Anterior</a>
    <span>Página {data.pagina}</span>
    <a href={paginadora(1)} class:desab={data.pagina * data.por >= data.total}>
      Próxima →
    </a>
  </div>
</section>

<style>
  .tarefas {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .cabeca h2 {
    margin: 0;
  }

  .item-tarefa {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    padding: 0;
    transition: background-color 0.15s;
  }
  .item-tarefa.expandido {
    background: var(--seno-white);
  }

  .card-topo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.85rem 1.1rem;
    width: 100%;
    box-sizing: border-box;
    cursor: pointer;
    user-select: none;
    outline: none;
    transition: background-color 0.12s;
  }
  .card-topo:focus-visible {
    outline: 2px solid var(--seno-blue-600);
    outline-offset: -2px;
  }
  .item-tarefa:not(.expandido) .card-topo:hover {
    background: var(--seno-gray-50);
  }

  .identidade {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    flex: 1;
    min-width: 0;
  }
  .chevron {
    color: var(--seno-gray-500);
    transition: transform 0.2s ease, color 0.15s;
    flex-shrink: 0;
  }
  .chevron.rotacionado {
    transform: rotate(90deg);
    color: var(--seno-blue-600);
  }
  .nome {
    color: var(--seno-blue-900);
    font-weight: 600;
    font-size: 0.95rem;
    line-height: 1.35;
    word-break: break-word;
  }

  .acoes {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-shrink: 0;
  }
  .acoes form {
    display: contents;
  }

  .btn-icone {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    padding: 0;
    border-radius: var(--raio-md);
    border: 1px solid var(--seno-line);
    background: var(--seno-white);
    color: var(--seno-gray-600);
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .btn-icone:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
    border-color: var(--seno-gray-300);
  }
  .btn-icone.vermelho {
    color: var(--seno-gray-600);
  }
  .btn-icone.vermelho:hover {
    background: #fef2f2;
    color: var(--seno-red);
    border-color: #fecaca;
  }

  .confirma {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--seno-red);
    font-size: 0.82rem;
    background: #fef2f2;
    padding: 0.2rem 0.5rem;
    border-radius: var(--raio-md);
    border: 1px solid #fecaca;
  }

  .card-detalhes {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    padding: 0.85rem 1.1rem 1.1rem 1.1rem;
    border-top: 1px dashed var(--seno-line);
    cursor: default;
    user-select: text;
    background: var(--seno-gray-50);
  }

  .enunciado-bloco {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .enunciado-rotulo {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--seno-gray-500);
  }
  .enunciado-texto {
    margin: 0;
    font-size: 0.9rem;
    color: var(--seno-gray-800);
    white-space: pre-wrap;
    line-height: 1.55;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-md);
    padding: 0.75rem 0.95rem;
  }
  .enunciado-vazio {
    margin: 0;
    font-size: 0.88rem;
    color: var(--seno-gray-500);
    font-style: italic;
  }

  .rodape-detalhes {
    display: flex;
    justify-content: flex-end;
  }
  .btn-detalhes {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.45rem 0.95rem;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: var(--raio-md);
    font-size: 0.86rem;
    font-weight: 500;
    text-decoration: none;
    transition: background-color 0.15s ease;
  }
  .btn-detalhes:hover {
    background: var(--seno-blue-700);
    color: var(--seno-white);
    text-decoration: none;
  }
</style>
