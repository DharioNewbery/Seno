<script lang="ts">
  // Todas as turmas (staff): filtros por matéria, período e estado
  // (ativas/encerradas); rota de entrada dos atalhos "Turmas →" de
  // matérias e períodos.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const turmas = $derived(data.turmas);
  const pagina = $derived(data.pagina);
  const por = $derived(data.por);
  const encerradas = $derived(data.encerradas);
  const filtraMateria = $derived(data.filtroMateria);
  const filtraPeriodo = $derived(data.filtroPeriodo);
  const materias = $derived(data.materias);
  const periodos = $derived(data.periodos);

  function paginadora(delta: number): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    qs.set("pagina", String(pagina + delta));
    return `?${qs.toString()}`;
  }
</script>

<section class="o-turmas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="cabeca">
    <h2>
      Turmas do sistema
      <span class="total">({data.total})</span>
    </h2>
  </div>

  <form method="get" class="filtros">
    <select name="materia_id">
      <option value="">Todas as matérias</option>
      {#each materias as m (m.id)}
        <option value={m.id} selected={filtraMateria === m.id}>
          {m.codigo} — {m.nome}
        </option>
      {/each}
    </select>
    <select name="periodo_id">
      <option value="">Todos os períodos</option>
      {#each periodos as p (p.id)}
        <option value={p.id} selected={filtraPeriodo === p.id}>
          {p.trimestre}T/{p.ano}
        </option>
      {/each}
    </select>
    <select name="encerradas">
      <option value="all" selected={encerradas === "all"}>Ativas + encerradas</option>
      <option value="" selected={encerradas === ""}>Somente ativas</option>
      <option value="only" selected={encerradas === "only"}>Só encerradas</option>
    </select>
    <button>Aplicar</button>
  </form>

  <ul class="lista">
    {#each turmas as t (t.id)}
      <li>
        <div class="identidade">
          <strong>{t.materia.codigo} — {t.materia.nome}</strong>
          <span class="linha2">
            {t.titulo || "Sem título"} · {t.trimestre}T/{t.ano} ·
            {t.professor.first_name} {t.professor.last_name}
          </span>
        </div>
        <span class={`estado ${t.encerrado_em ? "encerrada" : "ativa"}`}>
          {t.encerrado_em ? "Encerrada" : "Ativa"}
        </span>
      </li>
    {:else}
      <li class="vazio">Nenhuma turma com estes filtros.</li>
    {/each}
  </ul>

  <div class="paginacao">
    <a href={paginadora(-1)} class:desab={pagina <= 1}>← Anterior</a>
    <span>Página {pagina}</span>
    <a href={paginadora(1)} class:desab={pagina * por >= data.total}>Próxima →</a>
  </div>
</section>

<style>
  .o-turmas {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .cabeca h2 {
    margin: 0;
  }
    .filtros {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .filtros select {
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    padding: 0.5rem 0.6rem;
    background: var(--seno-white);
    font-size: 0.88rem;
  }
  .filtros button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.5rem 0.9rem;
    font-size: 0.88rem;
    cursor: pointer;
  }
  .filtros button:hover {
    background: var(--seno-blue-700);
  }
        .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .linha2 {
    color: var(--seno-gray-500);
    font-size: 0.83rem;
  }
              </style>
