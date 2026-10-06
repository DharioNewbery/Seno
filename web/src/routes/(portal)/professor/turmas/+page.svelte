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
  <span>El: {pagina}</span>
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
  .filtros {
    display: flex;
    gap: 0.4rem;
  }
  .filtros a {
    border: 1px solid var(--seno-gray-300);
    border-radius: 999px;
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
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .lista {
    list-style: none;
    padding: 0;
    margin: 0.5rem 0 0;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .lista li {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 0.9rem 1.1rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.8rem;
    flex-wrap: wrap;
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
  .estado {
    font-size: 0.73rem;
    border-radius: 999px;
    padding: 0.18rem 0.55rem;
  }
  .estado.ativa {
    background: #e6f4ea;
    color: var(--seno-green);
  }
  .estado.encerrada {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .abrir {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: 8px;
    padding: 0.4rem 0.8rem;
    text-decoration: none;
    font-size: 0.86rem;
  }
  .abrir:hover {
    background: var(--seno-blue-700);
  }
  .vazio {
    color: var(--seno-gray-500);
    justify-content: center !important;
  }
  .paginacao {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }
  .paginacao a {
    color: var(--seno-blue-600);
    text-decoration: none;
  }
  .paginacao a.desab {
    color: var(--seno-gray-300);
    pointer-events: none;
  }
</style>
