<script lang="ts">
  // Minhas turmas: padrão só ativas; pílulas de filtro (Ativas/Encerradas/
  // Todas) preservando paginação; cada turma abre o painel de controle.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const turmas = $derived(data.turmas);
  const pagina = $derived(data.pagina);
  const por = $derived(data.por);
  const encerradas = $derived(data.encerradas);

  const rotulos: { v: string; texto: string }[] = [
    { v: "", texto: "Ativas" },
    { v: "only", texto: "Encerradas" },
    { v: "all", texto: "Todas" },
  ];
  const tituloOu = (t: (typeof turmas)[number]) => t.titulo || "Sem título";

  function filtro(rota: string): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    if (rota) {
      qs.set("encerradas", rota);
    } else {
      qs.delete("encerradas");
    }
    qs.delete("pagina");
    const query = qs.toString();
    return query ? `?${query}` : "";
  }

  function paginadora(delta: number): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    qs.set("pagina", String(pagina + delta));
    return `?${qs.toString()}`;
  }
</script>

<section class="turmas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="cabecalho">
    <h2>Minhas turmas <span class="total">({data.total})</span></h2>
    <a class="nova" href="/professor/turmas/nova">Criar turma +</a>
  </div>

  <nav class="filtros" aria-label="Filtrar por estado">
    {#each rotulos as { v, texto } (v)}
      <a
        href={filtro(v)}
        class:y={encerradas === v}
        aria-current={encerradas === v ? "true" : undefined}
      >
        {texto}
      </a>
    {/each}
  </nav>
</section>

<ul class="lista">
  {#each turmas as t (t.id)}
    <li>
      <div class="identidade">
        <strong>{t.materia.codigo} — {t.materia.nome}</strong>
        <span class="periodo">
          {tituloOu(t)} · {t.trimestre}T/{t.ano}
        </span>
      </div>
      <div class="acao-area">
        {#if t.encerrado_em}
          <span class="estado encerrada">Encerrada</span>
        {:else}
          <span class="estado ativa">Ativa</span>
        {/if}
        <a class="abrir" href={`/professor/turmas/${t.id}`}>Painel →</a>
      </div>
    </li>
  {:else}
    <li class="vazio">Nenhuma turma com este filtro.</li>
  {/each}
</ul>

<div class="paginacao">
  <a href={paginadora(-1)} class:desab={pagina <= 1}>← Anterior</a>
  <span>Página {pagina}</span>
  <a href={paginadora(1)} class:desab={pagina * por >= data.total}>Próxima →</a>
</div>

<style>
  .turmas {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabecalho {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .cabecalho h2 {
    margin: 0;
  }
        .filtros {
    display: flex;
    gap: 0.4rem;
  }
  .filtros a {
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-pill);
    padding: 0.25rem 0.8rem;
    color: var(--seno-gray-700);
    font-size: 0.83rem;
    text-decoration: none;
    background: var(--seno-white);
  }
  .filtros a.y {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-500);
    color: var(--seno-blue-800);
    font-weight: 600;
  }
        .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .periodo {
    color: var(--seno-gray-500);
    font-size: 0.83rem;
  }
  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
                  </style>
