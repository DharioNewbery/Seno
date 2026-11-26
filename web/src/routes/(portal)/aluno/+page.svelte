<script lang="ts">
  import { onMount, tick } from "svelte";
  import { page } from "$app/state";
  import Icone from "#lib/components/Icone.svelte";
  import { PORTAIS } from "#lib/portais";
  import {
    horaDoPrazo,
    montarAgenda,
    resumirPainelAluno,
    type EstadoAgenda,
  } from "#lib/agenda-aluno";
  import { dataHora, prazoRelativo } from "#lib/formato";
  import {
    filtrarTurmasAtivasAluno,
    formatarSubtituloCardTurmaAluno,
    formatarTituloCardTurmaAluno,
  } from "#lib/turmas-aluno";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const portal = PORTAIS.aluno;
  const nome = $derived(page.data.usuario?.first_name as string | undefined);

  // `agora` é reativo: as colunas e os contadores se recalculam sozinhos com o tempo.
  let agora = $state(new Date());

  const colunas = $derived(montarAgenda(data.atribuicoes, agora));

  const resumo = $derived(
    resumirPainelAluno(data.atribuicoes, data.turmas, agora),
  );
  const turmasAtivas = $derived(filtrarTurmasAtivasAluno(data.turmas));

  let trilho: HTMLDivElement | undefined = $state();
  let podeRolarEsq = $state(false);
  let podeRolarDir = $state(false);

  function atualizarEstadoRolagem() {
    if (!trilho) {
      podeRolarEsq = false;
      podeRolarDir = false;
      return;
    }
    const max = Math.max(0, trilho.scrollWidth - trilho.clientWidth);
    podeRolarEsq = trilho.scrollLeft > 2;
    podeRolarDir = trilho.scrollLeft < max - 2;
  }

  function obterAlvoHoje(): HTMLElement | null {
    if (!trilho) return null;
    return (
      trilho.querySelector<HTMLElement>('[data-tom="hoje"]') ??
      trilho.querySelector<HTMLElement>('[data-tom="futuro"]') ??
      trilho.querySelector<HTMLElement>('[data-tom="passado-aberto"]')
    );
  }

  function centralizarHoje(suave = false) {
    if (!trilho) return;
    const alvo = obterAlvoHoje();
    if (!alvo) {
      atualizarEstadoRolagem();
      return;
    }
    const centroAlvo = alvo.offsetLeft - trilho.offsetLeft + alvo.offsetWidth / 2;
    const max = Math.max(0, trilho.scrollWidth - trilho.clientWidth);
    const destino = Math.min(
      max,
      Math.max(0, centroAlvo - trilho.clientWidth / 2),
    );
    trilho.scrollTo({
      left: destino,
      behavior: suave ? "smooth" : "auto",
    });
    atualizarEstadoRolagem();
  }

  function irParaHoje(suave = true) {
    if (!trilho) return;
    const alvo = obterAlvoHoje();
    if (!alvo) {
      atualizarEstadoRolagem();
      return;
    }
    const primeira = trilho.querySelector<HTMLElement>(".coluna");
    const padEsq = primeira ? primeira.offsetLeft : 0;
    const max = Math.max(0, trilho.scrollWidth - trilho.clientWidth);
    const destino = Math.min(max, Math.max(0, alvo.offsetLeft - padEsq));
    trilho.scrollTo({
      left: destino,
      behavior: suave ? "smooth" : "auto",
    });
    atualizarEstadoRolagem();
  }

  function moverColuna(direcao: -1 | 1) {
    if (!trilho) return;
    const cols = Array.from(trilho.querySelectorAll<HTMLElement>(".coluna"));
    if (cols.length === 0) return;
    const c0 = cols[0];
    const c1 = cols[1];
    const passo =
      c0 && c1
        ? c1.offsetLeft - c0.offsetLeft
        : (c0?.offsetWidth ?? 224) + 14;
    trilho.scrollBy({
      left: direcao * passo,
      behavior: "smooth",
    });
  }

  onMount(() => {
    const t = setInterval(() => (agora = new Date()), 30_000);
    centralizarHoje(false);
    return () => clearInterval(t);
  });

  $effect(() => {
    void colunas.length;
    void tick().then(() => atualizarEstadoRolagem());
  });

  const fmtCab = new Intl.DateTimeFormat("pt-BR", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    timeZone: "America/Belem",
  });

  const rotulos: Record<EstadoAgenda, string> = {
    pendente: "Pendente",
    perdida: "Atrasada e perdida",
    no_prazo: "Entregue no prazo",
    atrasada: "Entregue com atraso",
  };

  const legendaItens: { estado: EstadoAgenda; rotulo: string }[] = [
    { estado: "pendente", rotulo: "Pendente" },
    { estado: "no_prazo", rotulo: "No prazo" },
    { estado: "atrasada", rotulo: "Com atraso" },
    { estado: "perdida", rotulo: "Perdida" },
  ];
</script>

<svelte:window onresize={atualizarEstadoRolagem} />

{#snippet iconeEstado(estado: EstadoAgenda)}
  <svg
    class={`icone-estado ${estado}`}
    viewBox="0 0 24 24"
    width="18"
    height="18"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    role="img"
    aria-label={rotulos[estado]}
  >
    <circle cx="12" cy="12" r="9" />
    {#if estado === "perdida"}
      <path d="M9 9l6 6M15 9l-6 6" />
    {:else if estado === "no_prazo"}
      <path d="M8 12.5l3 3 5-6" stroke-linejoin="round" />
    {:else if estado === "atrasada"}
      <path d="M8.5 15.5l7-7" />
    {/if}
  </svg>
{/snippet}

<div class="painel-aluno">
  <header class="saudacao">
    <h1>{nome ? `Olá, ${nome}` : portal.titulo}</h1>
    <p>{portal.descricao}</p>
  </header>

  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  {#if resumo.emAndamento.length > 0}
    <section class="secao-painel em-andamento-painel" aria-label="Continuar atividade">
      <div class="cabecalho-secao">
        <div>
          <h2>Continuar de onde parou</h2>
          <p class="sub-secao">
            Você tem {resumo.emAndamento.length === 1
              ? "uma atividade iniciada aguardando"
              : `${resumo.emAndamento.length} atividades iniciadas aguardando`} conclusão.
          </p>
        </div>
      </div>

      <div class="lista-andamento">
        {#each resumo.emAndamento as a (a.id)}
          {@const rel = a.prazo ? prazoRelativo(a.prazo, agora) : null}
          <div class="item-andamento">
            <div class="andamento-info">
              <div class="andamento-badges">
                <span class="pilula-codigo">{a.materia_codigo}</span>
                {#if a.turma_titulo}
                  <span class="andamento-turma">{a.turma_titulo}</span>
                {/if}
              </div>
              <a class="andamento-titulo" href={`/aluno/atividades/${a.id}`}>
                {a.atividade_nome}
              </a>
            </div>
            <div class="andamento-acoes">
              {#if rel}
                <span class="andamento-prazo">
                  Prazo {rel.rotulo}
                </span>
              {/if}
              <a
                class="abrir"
                href={`/aluno/monitor/${a.id}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                <span>Continuar</span>
                <Icone nome="external-link" tamanho={14} />
              </a>
            </div>
          </div>
        {/each}
      </div>
    </section>
  {/if}

  <section class="secao-painel agenda" aria-label="Atividades para entregar">
    <div class="cabecalho-secao">
      <h2>Cronograma de entregas</h2>

      {#if colunas.length > 0}
        <div class="controles-agenda">
          <button
            type="button"
            class="btn-foco"
            onclick={() => irParaHoje(true)}
          >
            Ir para hoje
          </button>
        </div>
      {/if}
    </div>

    {#if colunas.length === 0}
      <div class="vazio-painel">
        <p class="vazio">
          Nenhuma atividade com prazo nas suas turmas ativas.
        </p>
      </div>
    {:else}
      <div class="carrossel-agenda">
        <button
          type="button"
          class="seta-trilho"
          aria-label="Coluna anterior"
          title="Coluna anterior"
          disabled={!podeRolarEsq}
          onclick={() => moverColuna(-1)}
        >
          <Icone nome="chevron-left" tamanho={20} />
        </button>

        <div
          class="trilho"
          bind:this={trilho}
          onscroll={atualizarEstadoRolagem}
        >
          {#each colunas as col (col.dia)}
            <section class="coluna" data-tom={col.tom}>
              <header>
                <div class="cab-data">
                  <strong>
                    {fmtCab.format(new Date(col.prazo)).replace(".", "")}
                  </strong>
                  {#if col.tom === "hoje"}
                    <span class="tag-dia hoje">Hoje</span>
                  {:else if col.tom === "passado-aberto"}
                    <span class="tag-dia atraso">Aceita atraso</span>
                  {/if}
                </div>
                <span
                  class="contagem"
                  class:completa={col.entregues === col.total}
                  title="Atividades entregues / total no dia"
                >
                  {col.entregues}/{col.total}
                </span>
              </header>
              <ul>
                {#each col.itens as item (item.atribuicao.id)}
                  <li>
                    <a
                      href={`/aluno/atividades/${item.atribuicao.id}`}
                      title={`${item.atribuicao.atividade_nome} (${item.atribuicao.materia_codigo}) — ${rotulos[item.estado]}`}
                    >
                      <div class="linha-topo">
                        {@render iconeEstado(item.estado)}
                        <span class="hora">
                          {horaDoPrazo(item.atribuicao.prazo as string)}
                        </span>
                        <span class="codigo">
                          {item.atribuicao.materia_codigo}
                        </span>
                      </div>
                      <span class="nome-atividade">
                        {item.atribuicao.atividade_nome}
                      </span>
                    </a>
                  </li>
                {/each}
              </ul>
            </section>
          {/each}
        </div>

        <button
          type="button"
          class="seta-trilho"
          aria-label="Próxima coluna"
          title="Próxima coluna"
          disabled={!podeRolarDir}
          onclick={() => moverColuna(1)}
        >
          <Icone nome="chevron-right" tamanho={20} />
        </button>
      </div>

      <footer class="legenda-agenda" aria-label="Legenda de estados">
        {#each legendaItens as item (item.estado)}
          <span class="legenda-item">
            {@render iconeEstado(item.estado)}
            <span>{item.rotulo}</span>
          </span>
        {/each}
      </footer>
    {/if}
  </section>

  <div class="grade-inferior">
    <section class="secao-painel" aria-label="Minhas turmas ativas">
      <div class="cabecalho-secao">
        <div>
          <h2>Minhas turmas</h2>
          <p class="sub-secao">
            {turmasAtivas.length}
            {turmasAtivas.length === 1 ? "turma ativa" : "turmas ativas"} neste período.
          </p>
        </div>
        <a class="link-secao" href="/aluno/turmas">Ver todas</a>
      </div>

      {#if turmasAtivas.length === 0}
        <p class="vazio">Você não está matriculado em nenhuma turma ativa.</p>
      {:else}
        <ul class="lista-compacta">
          {#each turmasAtivas.slice(0, 4) as t (t.id)}
            <li>
              <a class="item-compacto" href={`/aluno/turmas/${t.id}`}>
                <span class="pilula-codigo">{t.materia_codigo}</span>
                <div class="item-compacto-texto">
                  <strong>{formatarTituloCardTurmaAluno(t)}</strong>
                  <span>{formatarSubtituloCardTurmaAluno(t)}</span>
                </div>
                <Icone nome="chevron-right" tamanho={16} classe="seta-item" />
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section class="secao-painel" aria-label="Correções publicadas">
      <div class="cabecalho-secao">
        <div>
          <h2>Correções recentes</h2>
          <p class="sub-secao">Notas e comentários liberados pelos professores.</p>
        </div>
        <a class="link-secao" href="/aluno/atividades">Histórico</a>
      </div>

      {#if resumo.corrigidasRecentes.length === 0}
        <p class="vazio">Nenhuma correção publicada até o momento.</p>
      {:else}
        <ul class="lista-compacta">
          {#each resumo.corrigidasRecentes as a (a.id)}
            {@const destino = a.submissao_id
              ? `/aluno/submissao/${a.submissao_id}`
              : `/aluno/atividades/${a.id}`}
            <li>
              <a class="item-compacto" href={destino}>
                <span class="pilula-codigo">{a.materia_codigo}</span>
                <div class="item-compacto-texto">
                  <strong>{a.atividade_nome}</strong>
                  <span>
                    Publicada em {dataHora(a.publicada_em)}
                  </span>
                </div>
                <Icone nome="chevron-right" tamanho={16} classe="seta-item" />
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </div>
</div>

<style>
  .painel-aluno {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  /* ---- Topo e Saudação ---- */
  .saudacao h1 {
    margin: 0 0 0.25rem;
    font-size: 1.85rem;
  }

  .saudacao p {
    margin: 0;
    color: var(--seno-gray-700);
    max-width: 48ch;
  }

  /* ---- Seções do Painel ---- */
  .secao-painel {
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
    padding: 1.15rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .em-andamento-painel {
    border-color: var(--seno-blue-300);
    background: linear-gradient(180deg, var(--seno-blue-100) 0%, var(--seno-white) 100%);
  }

  .cabecalho-secao {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem 1rem;
    flex-wrap: wrap;
  }

  .cabecalho-secao h2 {
    margin: 0;
    font-size: 1.12rem;
    color: var(--seno-blue-900);
  }

  .sub-secao {
    margin: 0.15rem 0 0;
    font-size: 0.86rem;
    color: var(--seno-gray-500);
  }

  .link-secao {
    display: inline-flex;
    align-items: center;
    font-size: 0.86rem;
    font-weight: 600;
    color: var(--seno-blue-700);
    text-decoration: none;
    white-space: nowrap;
  }

  .link-secao:hover {
    color: var(--seno-blue-900);
    text-decoration: underline;
  }

  /* ---- Em Andamento ---- */
  .lista-andamento {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
  }

  .item-andamento {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem 1rem;
    flex-wrap: wrap;
    padding: 0.8rem 1rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-blue-200);
    border-radius: var(--raio-md);
  }

  .andamento-info {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    min-width: 0;
  }

  .andamento-badges {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
  }

  .andamento-turma {
    font-size: 0.8rem;
    color: var(--seno-gray-500);
  }

  .andamento-prazo {
    font-size: 0.84rem;
    color: var(--seno-gray-500);
  }

  .andamento-titulo {
    font-weight: 600;
    font-size: 0.98rem;
    color: var(--seno-blue-900);
    text-decoration: none;
  }

  .andamento-titulo:hover {
    text-decoration: underline;
  }

  .andamento-acoes {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  /* ---- Controles da Agenda ---- */
  .controles-agenda {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .btn-foco {
    background: var(--seno-gray-100);
    color: var(--seno-blue-900);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    padding: 0.32rem 0.7rem;
    font: inherit;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    transition:
      background-color 0.12s,
      border-color 0.12s;
  }

  .btn-foco:hover {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-400);
  }

  .vazio-painel {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    padding: 1rem 0;
  }

  /* ---- Carrossel, Setas e Colunas de Atividades ---- */
  .carrossel-agenda {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .seta-trilho {
    flex: none;
    display: grid;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    border-radius: var(--raio-pill);
    border: 1px solid var(--seno-gray-300);
    background: var(--seno-white);
    color: var(--seno-blue-900);
    cursor: pointer;
    transition:
      background-color 0.12s,
      border-color 0.12s,
      opacity 0.12s;
  }

  .seta-trilho:hover:not(:disabled) {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-500);
  }

  .seta-trilho:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }

  .trilho {
    flex: 1;
    min-width: 0;
    position: relative;
    display: flex;
    gap: 0.85rem;
    overflow-x: auto;
    padding: 0.45rem 0.35rem 0.45rem;
    align-items: flex-start;
    scrollbar-width: none;
    -ms-overflow-style: none;
  }

  .trilho::-webkit-scrollbar {
    display: none;
  }

  .coluna {
    flex: none;
    width: 14rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
    overflow: hidden;
    transition:
      border-color 0.15s,
      box-shadow 0.15s;
  }

  .coluna header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.4rem;
    padding: 0.65rem 0.85rem;
    background: var(--seno-gray-50);
    border-bottom: 1px solid var(--seno-line);
    text-transform: capitalize;
  }

  .cab-data {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    min-width: 0;
  }

  .cab-data strong {
    font-size: 0.9rem;
    color: var(--seno-blue-950);
    white-space: nowrap;
  }

  .tag-dia {
    font-size: 0.68rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    padding: 0.1rem 0.42rem;
    border-radius: var(--raio-pill);
    white-space: nowrap;
  }

  .tag-dia.hoje {
    background: var(--seno-blue-600);
    color: var(--seno-white);
  }

  .tag-dia.atraso {
    background: var(--seno-gray-200);
    color: var(--seno-gray-700);
  }

  .contagem {
    font-variant-numeric: tabular-nums;
    font-size: 0.78rem;
    font-weight: 600;
    padding: 0.12rem 0.48rem;
    border-radius: var(--raio-pill);
    background: var(--seno-gray-200);
    color: var(--seno-gray-700);
  }

  .contagem.completa {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }

  .coluna ul {
    list-style: none;
    margin: 0;
    padding: 0.3rem 0;
  }

  .coluna li + li {
    border-top: 1px solid var(--seno-gray-100);
  }

  .coluna li a {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    padding: 0.55rem 0.85rem;
    color: inherit;
    text-decoration: none;
    transition: background-color 0.12s;
  }

  .coluna li a:hover {
    background: var(--seno-blue-100);
  }

  .linha-topo {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .hora {
    font-variant-numeric: tabular-nums;
    font-size: 0.86rem;
    color: var(--seno-gray-700);
  }

  .codigo {
    margin-left: auto;
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-blue-900);
    background: var(--seno-blue-100);
    padding: 0.08rem 0.42rem;
    border-radius: var(--raio-sm);
  }

  .nome-atividade {
    font-size: 0.82rem;
    color: var(--seno-gray-700);
    padding-left: 1.62rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .icone-estado {
    flex: none;
    color: var(--seno-gray-500);
  }
  .icone-estado.perdida {
    color: var(--seno-red);
  }
  .icone-estado.no_prazo {
    color: var(--seno-green);
  }
  .icone-estado.atrasada {
    color: var(--seno-amber);
  }

  /* Datas passadas: esmaecidas. */
  .coluna[data-tom="passado"],
  .coluna[data-tom="passado-aberto"] {
    opacity: 0.55;
  }

  /* Hoje: outline pulsante. */
  .coluna[data-tom="hoje"] {
    border-color: var(--seno-blue-400);
    outline: 2px solid var(--seno-blue-500);
    outline-offset: 2px;
    animation: pulso 1.8s ease-in-out infinite;
  }

  .coluna[data-tom="hoje"] header {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-200);
  }

  @keyframes pulso {
    0%,
    100% {
      outline-color: var(--seno-blue-500);
      outline-offset: 2px;
    }
    50% {
      outline-color: transparent;
      outline-offset: 5px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .coluna[data-tom="hoje"] {
      animation: none;
    }
  }

  /* ---- Legenda ---- */
  .legenda-agenda {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.65rem 1.15rem;
    padding-top: 0.5rem;
    border-top: 1px solid var(--seno-gray-100);
    font-size: 0.8rem;
    color: var(--seno-gray-700);
  }

  .legenda-item {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }

  /* ---- Grade Inferior (Turmas & Correções) ---- */
  .grade-inferior {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
    gap: 1.15rem;
    align-items: stretch;
  }

  .lista-compacta {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-md);
    overflow: hidden;
  }

  .lista-compacta li + li {
    border-top: 1px solid var(--seno-line);
  }

  .item-compacto {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.7rem 0.9rem;
    color: inherit;
    text-decoration: none;
    transition: background-color 0.12s;
  }

  .item-compacto:hover {
    background: var(--seno-gray-50);
  }

  .pilula-codigo {
    flex: none;
    font-size: 0.76rem;
    font-weight: 600;
    color: var(--seno-blue-800);
    background: var(--seno-blue-100);
    padding: 0.18rem 0.5rem;
    border-radius: var(--raio-sm);
  }

  .item-compacto-texto {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .item-compacto-texto strong {
    font-size: 0.9rem;
    color: var(--seno-gray-900);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .item-compacto-texto span {
    font-size: 0.78rem;
    color: var(--seno-gray-500);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  :global(.seta-item) {
    flex: none;
    color: var(--seno-gray-500);
  }
</style>
