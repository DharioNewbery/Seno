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

  function aoMudarTurma(
    e: Event & { currentTarget: EventTarget & { value: string } },
  ) {
    const valor = e.currentTarget.value;
    const turmaId = valor ? Number(valor) : null;
    goto(montarUrl(turmaId, 1));
  }
</script>

<section class="minhas-atividades">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="cabeca">
    <h2>
      Atividades
      <span class="total">({paginacao.total})</span>
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
            {t.materia_codigo} ({t.titulo || `#${t.id}`})
          </option>
        {/each}
      </select>
    </div>
  </div>

  <ul class="lista">
    {#each atribuicoesVisiveis as a (a.id)}
      {@const est = resumirEstadoAtribuicaoAluno(a)}
      <li class="item-atividade">
        <div class="identidade">
          <strong class="titulo-atividade">{a.atividade_nome}</strong>
          <span class="meta">
            {a.materia_codigo} ({a.turma_titulo || `#${a.turma_id}`})
          </span>
        </div>
        <div class="acao-area">
          <span class="estado">{est.rotulo}</span>
          {#if !a.turma_ativa && !a.entregue}
            <span class="estado">Turma encerrada</span>
          {/if}
          <a
            class="btn-icone primario"
            href={`/aluno/atividades/${a.id}`}
            title="Ver atividade"
            aria-label="Ver atividade"
          >
            <Icone nome="eye" tamanho={18} />
          </a>
        </div>
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
    flex-direction: column;
    align-items: flex-start;
    gap: 0.75rem;
  }
  .cabeca h2 {
    margin: 0;
  }
  .filtro-turma {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
  }
  .filtro-turma label {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .filtro-turma select {
    min-width: 12rem;
    padding: 0.45rem 0.65rem;
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    background: var(--seno-white);
    color: var(--seno-text);
    font-size: 0.88rem;
  }
  .lista > li.item-atividade {
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  .lista > li.item-atividade:hover {
    border-color: var(--seno-blue-300, #93c5fd);
    box-shadow: 0 2px 8px rgba(15, 23, 42, 0.05);
  }
  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .btn-icone {
    width: 2.15rem;
    height: 2.15rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--raio-md);
    text-decoration: none;
    transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
  }
  .btn-icone.primario {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: 1px solid var(--seno-blue-600);
  }
  .btn-icone.primario:hover {
    background: var(--seno-blue-700);
    border-color: var(--seno-blue-700);
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
