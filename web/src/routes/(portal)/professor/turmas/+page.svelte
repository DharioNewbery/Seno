<script lang="ts">
  // Turmas do professor: filtro por período (padrão: mais recente; sem opção
  // "todos os períodos") e matéria (padrão: todas). Exibe apenas ativas por
  // padrão, com botão de texto sutil para mostrar/ocultar encerradas. Criação
  // de turma via modal na própria tela.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import {
    formatarQuantidadeAlunos,
    tituloCardTurma,
  } from "#lib/turmas-professor";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const turmasAtivas = $derived(data.turmas);
  const turmasEncerradas = $derived(data.turmasEncerradas);
  const totalEncerradas = $derived(data.totalEncerradas);
  const materias = $derived(data.materias);
  const periodos = $derived(data.periodos);
  const filtroMateria = $derived(data.filtroMateria);
  const filtroPeriodo = $derived(data.filtroPeriodo);
  const semPeriodos = $derived(data.semPeriodos);
  const pagina = $derived(data.pagina);
  const por = $derived(data.por);

  let exibirEncerradas = $state(false);
  let modalCriarAberto = $state(false);

  $effect(() => {
    exibirEncerradas = Boolean(data.mostrarEncerradas);
  });

  $effect(() => {
    if (form?.erro) {
      modalCriarAberto = true;
    }
  });

  const turmasVisiveis = $derived(
    exibirEncerradas
      ? [...turmasAtivas, ...turmasEncerradas]
      : turmasAtivas,
  );

  function abrirModalCriar() {
    if (semPeriodos) return;
    modalCriarAberto = true;
  }

  function fecharModalCriar() {
    modalCriarAberto = false;
  }

  function alternarEncerradas() {
    exibirEncerradas = !exibirEncerradas;
  }

  function paginadora(delta: number): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    if (filtroPeriodo) qs.set("periodo_id", String(filtroPeriodo));
    if (filtroMateria) qs.set("materia_id", String(filtroMateria));
    if (exibirEncerradas) qs.set("encerradas", "1");
    else qs.delete("encerradas");
    qs.set("pagina", String(pagina + delta));
    return `?${qs.toString()}`;
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === "Escape" && modalCriarAberto) fecharModalCriar();
  }}
/>

<section class="turmas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="cabecalho">
    <h2>Turmas <span class="total">({turmasVisiveis.length})</span></h2>
    <button
      type="button"
      class="nova btn-criar-turma"
      onclick={abrirModalCriar}
      disabled={semPeriodos}
      title={semPeriodos
        ? "Cadastre um período letivo antes de criar uma turma"
        : "Criar nova turma"}
    >
      Criar turma +
    </button>
  </div>

  {#if semPeriodos}
    <p class="aviso" role="status">
      Nenhum período letivo cadastrado pelo administrador. Assim que um período
      for cadastrado, suas turmas aparecerão aqui.
    </p>
  {:else}
    <div class="controles-listagem">
      <form method="GET" class="filtros" aria-label="Filtrar turmas">
        <label class="campo-filtro">
          <span class="rotulo-filtro">Período</span>
          <select
            name="periodo_id"
            onchange={(e) => e.currentTarget.form?.requestSubmit()}
          >
            {#each periodos as p (p.id)}
              <option value={p.id} selected={filtroPeriodo === p.id}>
                {p.trimestre}T/{p.ano}
              </option>
            {/each}
          </select>
        </label>

        <label class="campo-filtro">
          <span class="rotulo-filtro">Matéria</span>
          <select
            name="materia_id"
            onchange={(e) => e.currentTarget.form?.requestSubmit()}
          >
            <option value="" selected={!filtroMateria}>Todas as matérias</option>
            {#each materias as m (m.id)}
              <option value={m.id} selected={filtroMateria === m.id}>
                {m.codigo} — {m.nome}
              </option>
            {/each}
          </select>
        </label>

        {#if exibirEncerradas}
          <input type="hidden" name="encerradas" value="1" />
        {/if}
      </form>

      <button
        type="button"
        class="btn-encerradas"
        onclick={alternarEncerradas}
        aria-pressed={exibirEncerradas}
      >
        {#if exibirEncerradas}
          Ocultar encerradas ({totalEncerradas})
        {:else if totalEncerradas > 0}
          Mostrar encerradas ({totalEncerradas})
        {:else}
          Mostrar encerradas
        {/if}
      </button>
    </div>

    <ul class="lista">
      {#each turmasVisiveis as t (t.id)}
        {@const encerrada = Boolean(t.encerrado_em)}
        <li class="item-turma" class:encerrada>
          <a class="link-turma" href={`/professor/turmas/${t.id}`}>
            <div class="identidade">
              <strong class="titulo-turma">{tituloCardTurma(t)}</strong>
              <span class="detalhes-turma">
                {t.materia.codigo} — {t.materia.nome} · {t.trimestre}T/{t.ano} ·
                {formatarQuantidadeAlunos(t.total_alunos)}
              </span>
            </div>
            {#if encerrada}
              <div class="acao-area">
                <span class="estado encerrada">Encerrada</span>
              </div>
            {/if}
          </a>
        </li>
      {:else}
        <li class="vazio">
          {#if exibirEncerradas}
            Nenhuma turma encontrada neste período.
          {:else if totalEncerradas > 0}
            Nenhuma turma ativa neste período ({totalEncerradas}
            {totalEncerradas === 1 ? "encerrada oculta" : "encerradas ocultas"}).
          {:else}
            Nenhuma turma ativa neste período.
          {/if}
        </li>
      {/each}
    </ul>

    {#if data.total > por || pagina > 1}
      <div class="paginacao">
        <a href={paginadora(-1)} class:desab={pagina <= 1}>← Anterior</a>
        <span>Página {pagina}</span>
        <a href={paginadora(1)} class:desab={pagina * por >= data.total}>
          Próxima →
        </a>
      </div>
    {/if}
  {/if}
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
      class="modal-janela"
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-criar-turma-titulo"
    >
      <div class="modal-cabecalho">
        <div>
          <h3 id="modal-criar-turma-titulo">Criar turma</h3>
          <p class="dica">Você será o professor dono da turma.</p>
        </div>
        <button
          type="button"
          class="btn-fechar-modal"
          onclick={fecharModalCriar}
          aria-label="Fechar janela"
        >
          ✕
        </button>
      </div>

      <form method="POST" action="?/criar" class="modal-form">
        {#if form?.erro}
          <p class="aviso erro" role="alert">{form.erro}</p>
        {/if}

        <label>
          <span>Matéria</span>
          <select name="materia_id" required>
            <option
              value=""
              disabled
              selected={!form?.materia_id && !filtroMateria}
            >
              Escolha a matéria
            </option>
            {#each materias as m (m.id)}
              <option
                value={m.id}
                selected={Number(form?.materia_id || filtroMateria) === m.id}
              >
                {m.codigo} — {m.nome}
              </option>
            {/each}
          </select>
        </label>

        <label>
          <span>Período letivo</span>
          <select name="periodo_id" required>
            {#each periodos as p (p.id)}
              <option
                value={p.id}
                selected={Number(form?.periodo_id || filtroPeriodo) === p.id}
              >
                {p.trimestre}T/{p.ano}
              </option>
            {/each}
          </select>
        </label>

        <label>
          <span>Título (opcional)</span>
          <input
            name="titulo"
            value={form?.titulo ?? ""}
            placeholder="T1, noturna..."
          />
        </label>

        <div class="modal-rodape">
          <button
            type="button"
            class="btn-cancelar"
            onclick={fecharModalCriar}
          >
            Cancelar
          </button>
          <button
            type="submit"
            class="btn-confirmar"
            disabled={materias.length === 0 || periodos.length === 0}
          >
            Criar turma
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .turmas {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .cabecalho {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }

  .cabecalho h2 {
    margin: 0;
  }

  .btn-criar-turma {
    border: none;
    cursor: pointer;
    font-family: inherit;
  }

  .btn-criar-turma:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .controles-listagem {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 0.75rem 1rem;
    flex-wrap: wrap;
  }

  .filtros {
    display: flex;
    align-items: flex-end;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .campo-filtro {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .rotulo-filtro {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  .campo-filtro select {
    min-width: 10rem;
    font-size: 0.88rem;
    padding: 0.45rem 0.65rem;
  }

  .btn-encerradas {
    background: none;
    border: none;
    padding: 0.35rem 0.5rem;
    font: inherit;
    font-size: 0.83rem;
    color: var(--seno-gray-500);
    cursor: pointer;
    border-radius: var(--raio-sm);
    transition:
      color 0.12s,
      background-color 0.12s;
  }

  .btn-encerradas:hover {
    color: var(--seno-blue-800);
    background: var(--seno-gray-100);
  }

  .lista > li.item-turma {
    padding: 0;
  }

  .link-turma {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    width: 100%;
    padding: 0.9rem 1.15rem;
    color: inherit;
    text-decoration: none;
    transition: background-color 0.12s;
  }

  .link-turma:hover {
    color: inherit;
  }

  .link-turma:hover .titulo-turma {
    color: var(--seno-blue-700);
  }

  .item-turma.encerrada {
    background: var(--seno-gray-100);
  }

  .item-turma.encerrada:hover {
    background: var(--seno-gray-200);
  }

  .item-turma.encerrada .titulo-turma {
    color: var(--seno-gray-700);
  }

  .item-turma.encerrada .detalhes-turma {
    color: var(--seno-gray-500);
  }

  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .titulo-turma {
    color: var(--seno-blue-900);
    font-size: 0.98rem;
    transition: color 0.12s;
  }

  .detalhes-turma {
    color: var(--seno-gray-500);
    font-size: 0.84rem;
  }

  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  /* Modal Criar Turma */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgb(8 42 90 / 0.45);
    backdrop-filter: blur(2px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 1rem;
  }

  .modal-janela {
    background: var(--seno-white);
    border-radius: var(--raio-lg);
    box-shadow: var(--sombra-flutuante);
    width: 100%;
    max-width: 28rem;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--seno-gray-200);
  }

  .modal-cabecalho {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    padding: 1.1rem 1.35rem 0.9rem;
    border-bottom: 1px solid var(--seno-gray-200);
  }

  .modal-cabecalho h3 {
    margin: 0 0 0.15rem;
    font-size: 1.15rem;
    color: var(--seno-blue-900);
  }

  .btn-fechar-modal {
    background: transparent;
    border: none;
    font-size: 1.05rem;
    color: var(--seno-gray-500);
    cursor: pointer;
    padding: 0.2rem 0.45rem;
    border-radius: var(--raio-sm);
  }

  .btn-fechar-modal:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
  }

  .modal-form {
    padding: 1.2rem 1.35rem 1.35rem;
    display: flex;
    flex-direction: column;
    gap: 0.95rem;
  }

  .modal-form label {
    display: flex;
    flex-direction: column;
    gap: 0.28rem;
  }

  .modal-form label span {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  .modal-rodape {
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
    margin-top: 0.35rem;
  }

  .btn-cancelar {
    background: var(--seno-white);
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    padding: 0.55rem 1rem;
    font: inherit;
    font-size: 0.9rem;
    font-weight: 500;
    cursor: pointer;
  }

  .btn-cancelar:hover {
    background: var(--seno-gray-100);
  }

  .btn-confirmar {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.55rem 1.15rem;
    font: inherit;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
  }

  .btn-confirmar:hover:not(:disabled) {
    background: var(--seno-blue-700);
  }

  .btn-confirmar:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
</style>
