<script lang="ts">
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import Icone from "#lib/components/Icone.svelte";
  import {
    POR_PAGINA_ATIVIDADES_ALUNO,
    paginarAtribuicoesAluno,
  } from "#lib/atividades-aluno";
  import { resumirEstadoAtribuicaoAluno } from "#lib/correcao-aluno";
  import { dataHora, prazoRelativo } from "#lib/formato";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const paginacao = $derived(
    paginarAtribuicoesAluno(
      data.atribuicoes,
      data.pagina ?? 1,
      POR_PAGINA_ATIVIDADES_ALUNO,
    ),
  );
  const atribuicoesVisiveis = $derived(paginacao.itens);
  const turmasAtivas = $derived(data.turmas.filter((t) => t.ativa));

  function montarUrl(turmaId: number | null, pagina = 1): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    if (turmaId === null) qs.delete("turma_id");
    else qs.set("turma_id", String(turmaId));
    if (pagina <= 1) qs.delete("pagina");
    else qs.set("pagina", String(pagina));
    const query = qs.toString();
    return query ? `/aluno/atividades?${query}` : "/aluno/atividades";
  }

  function aoMudarTurma(e: Event) {
    const valor = (e.currentTarget as HTMLSelectElement).value;
    const turmaId = valor ? Number(valor) : null;
    goto(montarUrl(turmaId, 1));
  }

  function paraTexto(iso: string | undefined): string {
    if (!iso) return "Sem prazo";
    return `Prazo ${dataHora(iso)}`;
  }

  const turmaFiltrada = $derived(
    data.turmas.find((t) => t.id === data.filtroTurma),
  );
</script>

<section class="minhas-atividades">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="cabeca">
    <h2>
      Atividades
      <span class="total">({paginacao.total})</span>
      {#if turmaFiltrada}
        <span class="filtro">— {turmaFiltrada.materia_codigo} {turmaFiltrada.titulo || ""}</span>
      {/if}
    </h2>

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
            {t.materia_codigo} · {t.titulo || "Sem título"}
          </option>
        {/each}
      </select>
    </div>
  </div>

  <ul class="lista">
    {#each atribuicoesVisiveis as a (a.id)}
      {@const rel = a.entregue ? null : prazoRelativo(a.prazo)}
      {@const est = resumirEstadoAtribuicaoAluno(a)}
      <li class="item-atividade">
        <a class="link-atividade" href={`/aluno/atividades/${a.id}`}>
          <div class="identidade">
            <strong class="titulo-atividade">{a.atividade_nome}</strong>
            <span class="meta">
              {a.materia_codigo} · {a.turma_titulo || `Turma ${a.turma_id}`}
            </span>
            <span class="meta">
              {paraTexto(a.prazo)}
              {#if rel}<b class="rel">{rel.rotulo}</b>{/if}
              {#if a.pode_atrasado}· atraso permitido{/if}
            </span>
          </div>
          <div class="acao-area">
            <span class="estado">{est.rotulo}</span>
            {#if !a.turma_ativa && !a.entregue}
              <span class="estado">Turma encerrada</span>
            {/if}
          </div>
        </a>
      </li>
    {:else}
      <li class="vazio">
        Nenhuma atividade disponível. As atribuições aparecem quando o
        professor anexa uma atividade à sua turma.
      </li>
    {/each}
  </ul>

  {#if paginacao.totalPaginas > 1}
    <nav class="paginacao" aria-label="Paginação de atividades">
      <a
        class="btn-pag"
        class:desabilitado={paginacao.paginaAtual <= 1}
        aria-disabled={paginacao.paginaAtual <= 1}
        href={paginacao.paginaAtual <= 1
          ? undefined
          : montarUrl(data.filtroTurma ?? null, paginacao.paginaAtual - 1)}
      >
        <Icone nome="chevron-left" tamanho={16} />
        <span>Anterior</span>
      </a>

      <span class="pag-info">
        Página <strong>{paginacao.paginaAtual}</strong> de
        <strong>{paginacao.totalPaginas}</strong>
      </span>

      <a
        class="btn-pag"
        class:desabilitado={paginacao.paginaAtual >= paginacao.totalPaginas}
        aria-disabled={paginacao.paginaAtual >= paginacao.totalPaginas}
        href={paginacao.paginaAtual >= paginacao.totalPaginas
          ? undefined
          : montarUrl(data.filtroTurma ?? null, paginacao.paginaAtual + 1)}
      >
        <span>Próxima</span>
        <Icone nome="chevron-right" tamanho={16} />
      </a>
    </nav>
  {/if}
</section>

<style>
  .minhas-atividades {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.75rem;
  }
  .cabeca h2 {
    margin: 0;
  }
  .filtro {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .filtro-turma {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .filtro-turma label {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .filtro-turma select {
    padding: 0.42rem 0.7rem;
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    background: var(--seno-white);
    color: var(--seno-text);
    font-size: 0.88rem;
  }
  .lista > li.item-atividade {
    padding: 0;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  .lista > li.item-atividade:hover {
    border-color: var(--seno-blue-300, #93c5fd);
    box-shadow: 0 2px 8px rgba(15, 23, 42, 0.05);
  }
  .link-atividade {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.6rem 1rem;
    width: 100%;
    padding: 0.9rem 1.15rem;
    color: inherit;
    text-decoration: none;
  }
  .link-atividade:hover .titulo-atividade {
    color: var(--seno-blue-700);
  }
  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
  .rel {
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .estado {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
    font-weight: 600;
  }
  .paginacao {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.85rem;
    margin-top: 0.25rem;
  }
  .btn-pag {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.4rem 0.8rem;
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    background: var(--seno-white);
    color: var(--seno-gray-700);
    font-size: 0.85rem;
    font-weight: 600;
    text-decoration: none;
  }
  .btn-pag:hover:not(.desabilitado) {
    background: var(--seno-gray-100);
    color: var(--seno-blue-900);
  }
  .btn-pag.desabilitado {
    opacity: 0.45;
    pointer-events: none;
  }
  .pag-info {
    font-size: 0.85rem;
    color: var(--seno-gray-600);
  }
</style>
