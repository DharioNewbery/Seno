<script lang="ts">
  // Períodos letivos: fila por ano/trimestre; cada período abre as turmas
  // dele (/admin/turmas?periodo_id=id); excluir com confirmação inline.
  import { SvelteMap } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const periodos = $derived(data.periodos);

  const grupos = $derived.by(() => {
    const mapa = new SvelteMap<number, (typeof periodos)[number][]>();
    for (const p of periodos) {
      mapa.set(p.ano, [...(mapa.get(p.ano) ?? []), p]);
    }
    return [...mapa.entries()].sort((a, b) => b[0] - a[0]);
  });
</script>

<section class="periodos">
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
    <h2>Períodos letivos <span class="total">({periodos.length})</span></h2>
    <a class="nova" href="/admin/periodos/nova">Novo período +</a>
  </div>

  {#each grupos as [ano, lista] (ano)}
    <div class="ano-grupo">
      <h3>{ano}</h3>
      <ul class="lista">
        {#each lista as p (p.id)}
          <li>
            <span class="rotulo">{p.trimestre}T/{p.ano}</span>
            <div class="acoes">
              <a class="link" href={`/admin/turmas?periodo_id=${p.id}&encerradas=all`}>
                Turmas →
              </a>
              <form method="POST" action="?/excluir">
                <input type="hidden" name="id" value={p.id} />
                <button class="peq vermelho">Excluir</button>
              </form>
            </div>
          </li>
        {/each}
      </ul>
    </div>
  {:else}
    <p class="vazio">Nenhum período registrado.</p>
  {/each}
</section>

<style>
  .periodos {
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
  .total {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .nova {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: 8px;
    padding: 0.45rem 0.9rem;
    text-decoration: none;
    font-size: 0.9rem;
    font-weight: 600;
  }
  .nova:hover {
    background: var(--seno-blue-700);
  }
  .ano-grupo h3 {
    color: var(--seno-blue-900);
    margin: 0.6rem 0 0.4rem;
    font-size: 1rem;
  }
  .lista {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .lista li {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 0.7rem 1rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }
  .rotulo {
    font-weight: 600;
    color: var(--seno-blue-900);
  }
  .acoes {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .acoes form {
    display: contents;
  }
  .link {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
  .peq {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.35rem 0.75rem;
    font-size: 0.82rem;
    cursor: pointer;
  }
  .peq.vermelho {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }
  .peq.vermelho:hover {
    background: #fdecea;
  }
  .vazio {
    color: var(--seno-gray-500);
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .aviso.ok {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
</style>
