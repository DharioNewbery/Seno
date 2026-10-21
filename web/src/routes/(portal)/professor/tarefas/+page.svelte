<script lang="ts">
  // Banco de tarefas: listagem com ações (abrir, duplicar, excluir
  // com confirmação inline). Criação em página própria.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const tarefas = $derived(data.tarefas);

  let confirmando = $state<number | null>(null);

  function paginadora(delta: number): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    qs.set("pagina", String(data.pagina + delta));
    return `?${qs.toString()}`;
  }

  function limites(t: (typeof tarefas)[number]): string {
    const p: string[] = [];
    if (t.tempo_cpu_ms) p.push(`CPU ${t.tempo_cpu_ms}ms`);
    if (t.tempo_total_ms) p.push(`total ${t.tempo_total_ms}ms`);
    if (t.memoria_mb) p.push(`${t.memoria_mb}MB`);
    return p.length ? p.join(" · ") : "limites padrão";
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
      <li>
        <div class="identidade">
          <a class="nome" href={`/professor/tarefas/${t.id}`}>{t.nome}</a>
          <span class="meta">{limites(t)}</span>
        </div>
        <div class="acoes">
          <a class="link" href={`/professor/tarefas/${t.id}`}>Abrir →</a>
          <form method="POST" action="?/duplicar">
            <input type="hidden" name="id" value={t.id} />
            <button class="peq">Duplicar</button>
          </form>
          {#if confirmando === t.id}
            <form method="POST" action="?/excluir" class="confirma">
              <span>Excluir?</span>
              <input type="hidden" name="id" value={t.id} />
              <button class="peq vermelho">Sim</button>
              <button type="button" class="peq" onclick={() => (confirmando = null)}>
                Não
              </button>
            </form>
          {:else}
            <button class="peq vermelho" onclick={() => (confirmando = t.id)}>
              Excluir
            </button>
          {/if}
        </div>
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
                .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .nome {
    color: var(--seno-blue-900);
    text-decoration: none;
    font-weight: 600;
  }
  .nome:hover {
    text-decoration: underline;
  }
    .acoes {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .acoes form {
    display: contents;
  }
  .confirma {
    align-items: center;
    gap: 0.3rem;
    color: var(--seno-red);
    font-size: 0.82rem;
  }
  .confirma span,
  .confirma button {
    display: inline-block;
  }
  .link {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.86rem;
  }
                </style>
