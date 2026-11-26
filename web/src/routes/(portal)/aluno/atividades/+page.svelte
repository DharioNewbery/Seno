<script lang="ts">
  import {
    agruparAtividadesPorData,
    chaveDataBelem,
    gerarDiasCalendario,
    indiceDiaMaisAtual,
    nomeMateriaAtividade,
    timelineAtividadeAluno,
  } from "#lib/atividades-aluno";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const hojeChave = $derived(chaveDataBelem(new Date()) ?? "2026-01-01");

  let mostrarEntregues = $state(false);
  let minimizado = $state(false);
  let indiceManual = $state<number | null>(null);
  let suprimirAnimacao = $state(false);

  const setores = $derived(
    agruparAtividadesPorData(data.atribuicoes, mostrarEntregues),
  );
  const indicePadrao = $derived(indiceDiaMaisAtual(setores, hojeChave));
  const indiceAtual = $derived.by(() => {
    if (setores.length === 0) return 0;
    const base = indiceManual ?? indicePadrao;
    return Math.min(Math.max(0, base), setores.length - 1);
  });

  const setorAtual = $derived(setores[indiceAtual] ?? null);

  // Datas que possuem atividades na aba atual (para destacar no calendário).
  const datasComAtividade = $derived(new Set(setores.map((s) => s.chave)));

  // Mês exibido no calendário (inicia no mês do setor atual ou de hoje).
  let calAno = $state(new Date().getFullYear());
  let calMes = $state(new Date().getMonth());

  function sincronizarMesComChave(chave: string | undefined) {
    const alvo = chave && chave !== "sem-data" ? chave : hojeChave;
    const [y, m] = alvo.split("-").map(Number);
    if (y && m) {
      calAno = y;
      calMes = m - 1;
    }
  }

  const diasCalendario = $derived(gerarDiasCalendario(calAno, calMes));

  const NOME_MESES = [
    "Janeiro",
    "Fevereiro",
    "Março",
    "Abril",
    "Maio",
    "Junho",
    "Julho",
    "Agosto",
    "Setembro",
    "Outubro",
    "Novembro",
    "Dezembro",
  ];
  const DIAS_SEMANA = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];

  function irParaSetor(novoIndice: number) {
    if (setores.length === 0) return;
    const limitado = Math.min(Math.max(0, novoIndice), setores.length - 1);
    if (limitado === indiceAtual) return;
    indiceManual = limitado;
  }

  function alternarEntregues() {
    const chaveAtual = setorAtual?.chave;
    mostrarEntregues = !mostrarEntregues;
    if (chaveAtual) {
      const novos = agruparAtividadesPorData(data.atribuicoes, mostrarEntregues);
      const idx = novos.findIndex((s) => s.chave === chaveAtual);
      if (idx !== -1) {
        suprimirAnimacao = true;
        indiceManual = idx;
        globalThis.requestAnimationFrame(() => {
          globalThis.requestAnimationFrame(() => {
            suprimirAnimacao = false;
          });
        });
      } else {
        indiceManual = null;
      }
    } else {
      indiceManual = null;
    }
    if (minimizado) {
      sincronizarMesComChave(chaveAtual ?? hojeChave);
    }
  }

  function alternarMinimizar() {
    if (!minimizado) {
      sincronizarMesComChave(setorAtual?.chave);
      minimizado = true;
    } else {
      minimizado = false;
    }
  }

  function maximizarNaData(chave: string) {
    const idx = setores.findIndex((s) => s.chave === chave);
    if (idx === -1) return;
    indiceManual = idx;
    minimizado = false;
  }

  function mudarMesCalendario(delta: number) {
    const d = new Date(calAno, calMes + delta, 1);
    calAno = d.getFullYear();
    calMes = d.getMonth();
  }

  // ---- Navegação lateral por arrasto / swipe / wheel com intenção do usuário ----
  let viewportEl = $state<HTMLDivElement | null>(null);
  let arrastoPx = $state(0);
  let arrastando = $state(false);
  let suprimirClique = false;

  const SLOT_DATA_REM = 7.25;
  const progressoArrasto = $derived.by(() => {
    const largura = viewportEl?.clientWidth || 960;
    return Math.max(-1, Math.min(1, arrastoPx / largura));
  });
  const posicaoContinua = $derived(indiceAtual - progressoArrasto);

  let pointerIdAtivo: number | null = null;
  let inicioX = 0;
  let inicioY = 0;
  let inicioTempo = 0;
  let eixoTravado: "x" | "y" | null = null;
  let wheelTimer: ReturnType<typeof setTimeout> | null = null;

  function calcularDeslocamentoComResistencia(dx: number): number {
    const noInicio = indiceAtual === 0 && dx > 0;
    const noFim = indiceAtual >= setores.length - 1 && dx < 0;
    return noInicio || noFim ? dx * 0.28 : dx;
  }

  function finalizarIntencao(dx: number, velocidadePxMs: number) {
    const largura = viewportEl?.clientWidth || 600;
    const limiarDistancia = largura * 0.14;
    const querProximo =
      (dx < -limiarDistancia || (dx < -28 && velocidadePxMs < -0.25)) &&
      indiceAtual < setores.length - 1;
    const querAnterior =
      (dx > limiarDistancia || (dx > 28 && velocidadePxMs > 0.25)) &&
      indiceAtual > 0;

    if (querProximo) {
      irParaSetor(indiceAtual + 1);
    } else if (querAnterior) {
      irParaSetor(indiceAtual - 1);
    }
    arrastoPx = 0;
    arrastando = false;
  }

  function aoPointerDown(e: {
    button: number;
    pointerId: number;
    clientX: number;
    clientY: number;
  }) {
    if (e.button !== 0 || setores.length <= 1) return;
    pointerIdAtivo = e.pointerId;
    inicioX = e.clientX;
    inicioY = e.clientY;
    inicioTempo = Date.now();
    eixoTravado = null;
    suprimirClique = false;
  }

  function aoPointerMove(e: {
    pointerId: number;
    clientX: number;
    clientY: number;
  }) {
    if (pointerIdAtivo !== e.pointerId) return;
    const dx = e.clientX - inicioX;
    const dy = e.clientY - inicioY;

    if (!eixoTravado) {
      if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return;
      eixoTravado = Math.abs(dx) >= Math.abs(dy) ? "x" : "y";
      if (eixoTravado === "x") {
        arrastando = true;
        suprimirClique = true;
        viewportEl?.setPointerCapture?.(e.pointerId);
      } else {
        pointerIdAtivo = null;
        return;
      }
    }

    if (eixoTravado === "x") {
      arrastoPx = calcularDeslocamentoComResistencia(dx);
    }
  }

  function aoPointerUp(e: { pointerId: number; clientX: number }) {
    if (pointerIdAtivo !== e.pointerId) return;
    pointerIdAtivo = null;
    if (eixoTravado !== "x") {
      arrastando = false;
      arrastoPx = 0;
      return;
    }
    const dx = e.clientX - inicioX;
    const dt = Math.max(1, Date.now() - inicioTempo);
    finalizarIntencao(dx, dx / dt);
  }

  function aoWheel(e: {
    deltaX: number;
    deltaY: number;
    preventDefault(): void;
  }) {
    if (minimizado || setores.length <= 1) return;
    if (Math.abs(e.deltaX) <= Math.abs(e.deltaY) || Math.abs(e.deltaX) < 3) {
      return;
    }
    e.preventDefault();
    arrastando = true;
    const bruto = arrastoPx - e.deltaX;
    arrastoPx = calcularDeslocamentoComResistencia(bruto);

    if (wheelTimer) globalThis.clearTimeout(wheelTimer);
    wheelTimer = globalThis.setTimeout(() => {
      finalizarIntencao(arrastoPx, 0);
    }, 110);
  }

  function aoKeyDownViewport(e: { key: string; preventDefault(): void }) {
    if (minimizado || setores.length <= 1) return;
    if (e.key === "ArrowLeft") {
      e.preventDefault();
      irParaSetor(indiceAtual - 1);
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      irParaSetor(indiceAtual + 1);
    }
  }

  function aoClicarCard(e: { preventDefault(): void }) {
    if (suprimirClique) {
      e.preventDefault();
      suprimirClique = false;
    }
  }

  // ---- Setas laterais nas bordas ao passar o mouse ----
  let palcoEl = $state<HTMLDivElement | null>(null);
  let bordaHover = $state<"esq" | "dir" | null>(null);

  function aoMouseMovePalco(e: { clientX: number }) {
    if (minimizado || setores.length <= 1 || !palcoEl) {
      bordaHover = null;
      return;
    }
    const rect = palcoEl.getBoundingClientRect();
    const zona = Math.min(150, rect.width * 0.16);
    if (e.clientX <= rect.left + zona && indiceAtual > 0) {
      bordaHover = "esq";
    } else if (
      e.clientX >= rect.right - zona &&
      indiceAtual < setores.length - 1
    ) {
      bordaHover = "dir";
    } else {
      bordaHover = null;
    }
  }
</script>

<svelte:window onkeydown={aoKeyDownViewport} />

<section class="atividades-board">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="topo-board">
    <div class="topo-esq" aria-hidden="true"></div>

    <div class="centro-data">
      <button
        type="button"
        class="btn-modo"
        class:ativo={minimizado}
        onclick={alternarMinimizar}
        aria-label={minimizado
          ? "Maximizar no dia mais atual"
          : "Minimizar para calendário"}
        title={minimizado ? "Maximizar no dia mais atual" : "Abrir calendário"}
      >
        <svg
          width="17"
          height="17"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
          <line x1="16" y1="2" x2="16" y2="6" />
          <line x1="8" y1="2" x2="8" y2="6" />
          <line x1="3" y1="10" x2="21" y2="10" />
        </svg>
      </button>

      <div class="data-nav" class:oculto={minimizado || setores.length === 0}>
        <div
          class="data-trilho"
          class:animando={!arrastando && !suprimirAnimacao}
          style:transform={`translateX(${-posicaoContinua * SLOT_DATA_REM}rem)`}
        >
          {#each setores as setor, idx (setor.chave)}
            {@const dist = Math.min(1.5, Math.abs(idx - posicaoContinua))}
            {@const escala = 1 - Math.min(1, dist) * 0.22}
            {@const opacidade = Math.max(0, 1 - dist * 0.56)}
            <button
              type="button"
              class="data-item"
              class:ativa={idx === indiceAtual}
              class:animando={!arrastando && !suprimirAnimacao}
              style:transform={`scale(${escala})`}
              style:opacity={opacidade}
              onclick={() => irParaSetor(idx)}
              tabindex={Math.abs(idx - indiceAtual) <= 1 ? 0 : -1}
              aria-current={idx === indiceAtual ? "date" : undefined}
            >
              {setor.rotulo}
            </button>
          {/each}
        </div>
      </div>
    </div>

    <div class="controles-dir">
      <button
        type="button"
        role="switch"
        aria-checked={mostrarEntregues}
        class="filtro-entregues"
        class:ativo={mostrarEntregues}
        onclick={alternarEntregues}
      >
        <span class="switch-rotulo">Mostrar entregues</span>
        <span class="switch-trilho" aria-hidden="true">
          <span class="switch-thumb"></span>
        </span>
      </button>
    </div>
  </div>

  <div
    class="palco-principal"
    class:minimizado
    bind:this={palcoEl}
    onmousemove={aoMouseMovePalco}
    onmouseleave={() => (bordaHover = null)}
    role="presentation"
  >
    <div
      class="calendario-palco"
      class:visivel={minimizado}
      aria-hidden={!minimizado}
    >
      <div class="calendario-card">
        <div class="cal-cabeca">
          <button
            type="button"
            class="cal-seta"
            tabindex={minimizado ? 0 : -1}
            onclick={() => mudarMesCalendario(-1)}
            aria-label="Mês anterior"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <polyline points="15 18 9 12 15 6" />
            </svg>
          </button>
          <span class="cal-mes">{NOME_MESES[calMes]} {calAno}</span>
          <button
            type="button"
            class="cal-seta"
            tabindex={minimizado ? 0 : -1}
            onclick={() => mudarMesCalendario(1)}
            aria-label="Próximo mês"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>
        </div>

        <div class="cal-semana" aria-hidden="true">
          {#each DIAS_SEMANA as ds (ds)}
            <span>{ds}</span>
          {/each}
        </div>

        <div class="cal-grade">
          {#each diasCalendario as cel (cel.chave)}
            {@const temAtividade = datasComAtividade.has(cel.chave)}
            {@const ehHoje = cel.chave === hojeChave}
            {#if temAtividade}
              <button
                type="button"
                class="cal-dia com-atividade"
                class:fora-mes={!cel.doMes}
                class:hoje={ehHoje}
                tabindex={minimizado ? 0 : -1}
                onclick={() => maximizarNaData(cel.chave)}
              >
                {cel.dia}
              </button>
            {:else}
              <span
                class="cal-dia sem-atividade"
                class:fora-mes={!cel.doMes}
                class:hoje={ehHoje}
              >
                {cel.dia}
              </span>
            {/if}
          {/each}
        </div>
      </div>
    </div>

    {#if setores.length === 0}
      <div class="vazio-board" class:escondido={minimizado} aria-hidden={minimizado}>
        <p class="vazio-titulo">Nenhuma atividade encontrada</p>
        <p class="vazio-texto">
          {mostrarEntregues
            ? "Você não possui atividades registradas no momento."
            : "Você não possui atividades pendentes no momento."}
        </p>
      </div>
    {:else}
      <div
        class="setores-viewport"
        class:arrastando
        class:escondido={minimizado}
        bind:this={viewportEl}
        role="region"
        aria-label="Setores de atividades por data"
        aria-hidden={minimizado}
        onpointerdown={aoPointerDown}
        onpointermove={aoPointerMove}
        onpointerup={aoPointerUp}
        onpointercancel={aoPointerUp}
        onwheel={aoWheel}
      >
        <div
          class="setores-trilho"
          class:animando={!arrastando && !suprimirAnimacao}
          style:transform={`translateX(calc(-${indiceAtual} * (100% + 2.5rem) + ${arrastoPx}px))`}
        >
          {#each setores as setor, idx (setor.chave)}
            {@const distIdx = Math.abs(idx - indiceAtual)}
            {@const relPos = idx === indiceAtual ? 0 : idx < indiceAtual ? -1 : 1}
            <div
              class="setor"
              class:ativo={relPos === 0}
              class:antes={relPos === -1}
              class:depois={relPos === 1}
              class:distante={distIdx > 1}
              class:sem-transicao={suprimirAnimacao}
              aria-hidden={minimizado || relPos !== 0}
            >
              <div class="notice-board">
                {#each setor.itens as a (a.id)}
                  {@const etapas = timelineAtividadeAluno(a)}
                  {@const corrigida = Boolean(a.entregue && a.publicada_em)}
                  {@const expiradaEmProgresso =
                    etapas[1]?.estado === "expirada"}
                  <a
                    class="notice-card"
                    href={`/aluno/atividades/${a.id}`}
                    tabindex={!minimizado && relPos === 0 ? 0 : -1}
                    draggable="false"
                    onclick={aoClicarCard}
                  >
                    <div class="notice-cabeca">
                      <h3 class="notice-titulo">{a.atividade_nome}</h3>
                      <p class="notice-materia">{nomeMateriaAtividade(a)}</p>
                    </div>

                    <ol
                      class="timeline"
                      class:corrigida
                      class:expirada-em-progresso={expiradaEmProgresso}
                      aria-label="Progresso da atividade"
                    >
                      {#each etapas as etapa (etapa.id)}
                        {@const ehCheckCorrigida =
                          corrigida && etapa.id === "corrigida"}
                        {@const ehXExpirada = etapa.estado === "expirada"}
                        <li
                          class="etapa"
                          class:concluida={etapa.estado === "concluida"}
                          class:atual={etapa.estado === "atual"}
                          class:expirada={ehXExpirada}
                          class:em-progresso-etapa={etapa.id === "em_progresso" &&
                            etapa.estado === "atual"}
                        >
                          <span class="etapa-marcador" aria-hidden="true">
                            <span
                              class="etapa-ponto"
                              class:com-icone={ehCheckCorrigida || ehXExpirada}
                            >
                              {#if ehCheckCorrigida}
                                <svg
                                  width="10"
                                  height="10"
                                  viewBox="0 0 12 12"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2.2"
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                >
                                  <polyline points="2.5 6.5 5 9 9.5 3.5" />
                                </svg>
                              {:else if ehXExpirada}
                                <svg
                                  width="9"
                                  height="9"
                                  viewBox="0 0 12 12"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2.2"
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                >
                                  <line x1="3" y1="3" x2="9" y2="9" />
                                  <line x1="9" y1="3" x2="3" y2="9" />
                                </svg>
                              {/if}
                            </span>
                          </span>
                          <span class="etapa-rotulo">{etapa.rotulo}</span>
                        </li>
                      {/each}
                    </ol>
                  </a>
                {/each}
              </div>
            </div>
          {/each}
        </div>
      </div>

      {#if indiceAtual > 0}
        <button
          type="button"
          class="seta-lateral esq"
          class:visivel={!minimizado && bordaHover === "esq"}
          onclick={() => irParaSetor(indiceAtual - 1)}
          tabindex={minimizado ? -1 : 0}
          aria-label="Dia anterior"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <polyline points="15 18 9 12 15 6" />
          </svg>
        </button>
      {/if}

      {#if indiceAtual < setores.length - 1}
        <button
          type="button"
          class="seta-lateral dir"
          class:visivel={!minimizado && bordaHover === "dir"}
          onclick={() => irParaSetor(indiceAtual + 1)}
          tabindex={minimizado ? -1 : 0}
          aria-label="Próximo dia"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </button>
      {/if}
    {/if}
  </div>
</section>

<style>
  .atividades-board {
    display: flex;
    flex-direction: column;
    gap: 1.75rem;
  }

  /* ---- Topo: carrossel de datas centralizado + controles à direita ---- */
  .topo-board {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    gap: 1rem;
  }

  .centro-data {
    justify-self: center;
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
  }

  .data-nav {
    width: 21.75rem;
    overflow: hidden;
    position: relative;
    mask-image: linear-gradient(
      to right,
      transparent 0%,
      black 14%,
      black 86%,
      transparent 100%
    );
    transition:
      transform 0.44s cubic-bezier(0.22, 1, 0.36, 1),
      opacity 0.34s ease;
  }

  .data-nav.oculto {
    opacity: 0;
    transform: translateY(34px);
    pointer-events: none;
  }

  .data-trilho {
    display: flex;
    align-items: center;
    padding-left: 7.25rem;
    will-change: transform;
  }

  .data-trilho.animando {
    transition: transform 0.42s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .data-item {
    flex: 0 0 7.25rem;
    width: 7.25rem;
    padding: 0.25rem 0.2rem;
    background: none;
    border: none;
    font: inherit;
    font-size: 1.15rem;
    font-weight: 600;
    color: var(--seno-gray-900);
    letter-spacing: -0.01em;
    text-align: center;
    text-transform: capitalize;
    white-space: nowrap;
    cursor: pointer;
    transform-origin: center;
  }

  .data-item.animando {
    transition:
      transform 0.42s cubic-bezier(0.22, 1, 0.36, 1),
      opacity 0.35s ease,
      color 0.2s ease;
  }

  .data-item:not(.ativa) {
    color: var(--seno-gray-500);
    font-weight: 500;
  }

  .data-item:not(.ativa):hover {
    color: var(--seno-gray-900);
  }

  .controles-dir {
    justify-self: end;
    display: inline-flex;
    align-items: center;
    gap: 0.6rem;
  }

  /* ---- Switch true/false para mostrar entregues ---- */
  .filtro-entregues {
    display: inline-flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.35rem 0.45rem 0.35rem 0.85rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-pill);
    color: var(--seno-gray-700);
    font: inherit;
    font-size: 0.86rem;
    font-weight: 500;
    cursor: pointer;
    transition:
      border-color 0.16s ease,
      color 0.16s ease,
      background-color 0.16s ease;
  }

  .filtro-entregues:hover {
    border-color: var(--seno-gray-300);
    color: var(--seno-gray-900);
  }

  .filtro-entregues.ativo {
    border-color: var(--seno-blue-200);
    background: var(--seno-blue-100);
    color: var(--seno-blue-700);
  }

  .switch-trilho {
    position: relative;
    width: 2.1rem;
    height: 1.2rem;
    border-radius: var(--raio-pill);
    background: var(--seno-gray-300);
    transition: background-color 0.2s ease;
    flex: none;
  }

  .filtro-entregues.ativo .switch-trilho {
    background: var(--seno-primary);
  }

  .switch-thumb {
    position: absolute;
    top: 2px;
    left: 2px;
    width: calc(1.2rem - 4px);
    height: calc(1.2rem - 4px);
    border-radius: 50%;
    background: var(--seno-white);
    box-shadow: 0 1px 2px rgb(15 23 42 / 0.2);
    transition: transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .filtro-entregues.ativo .switch-thumb {
    transform: translateX(0.9rem);
  }

  .btn-modo {
    width: 2.25rem;
    height: 2.25rem;
    display: grid;
    place-items: center;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: 50%;
    color: var(--seno-gray-700);
    cursor: pointer;
    transition:
      background-color 0.14s ease,
      color 0.14s ease,
      transform 0.14s ease;
  }

  .btn-modo:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
    transform: scale(1.05);
  }

  .btn-modo.ativo {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-300);
    color: var(--seno-primary);
  }

  /* ---- Palco empilhado (cards + calendário) ---- */
  .palco-principal {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    position: relative;
    align-items: start;
  }

  /* ---- Setores horizontais com laterais borradas e mescladas ao fundo ---- */
  .setores-viewport {
    grid-area: 1 / 1;
    position: relative;
    z-index: 5;
    overflow: visible;
    margin: -0.75rem;
    padding: 0.75rem;
    touch-action: pan-y;
    cursor: grab;
    outline: none;
  }

  .setores-viewport.escondido {
    z-index: 1;
    pointer-events: none;
  }

  .setores-viewport.arrastando {
    cursor: grabbing;
    user-select: none;
  }

  .setores-trilho {
    display: flex;
    align-items: flex-start;
    gap: 2.5rem;
    will-change: transform;
  }

  .setores-trilho.animando {
    transition: transform 0.42s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .setor {
    flex: 0 0 100%;
    width: 100%;
    min-width: 0;
    transition:
      filter 0.38s ease,
      opacity 0.36s ease,
      transform 0.46s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .setor.antes {
    filter: blur(4px);
    opacity: 0.55;
    transform: scale(0.96);
    pointer-events: none;
    mask-image: linear-gradient(
      to right,
      transparent 45%,
      rgb(0 0 0 / 0.35) 78%,
      black 100%
    );
  }

  .setor.depois {
    filter: blur(4px);
    opacity: 0.55;
    transform: scale(0.96);
    pointer-events: none;
    mask-image: linear-gradient(
      to left,
      transparent 45%,
      rgb(0 0 0 / 0.35) 78%,
      black 100%
    );
  }

  .setor.distante {
    opacity: 0;
    pointer-events: none;
  }

  .setor.sem-transicao {
    transition: none;
  }

  /* Ao minimizar: cards do meio descem e laterais saem para os lados */
  .setores-viewport.escondido .setor.ativo {
    transform: translateY(115px);
    opacity: 0;
  }

  .setores-viewport.escondido .setor.antes {
    transform: scale(0.96) translateX(-125px);
    opacity: 0;
  }

  .setores-viewport.escondido .setor.depois {
    transform: scale(0.96) translateX(125px);
    opacity: 0;
  }

  /* ---- Painel Notice Board (3 colunas) ---- */
  .notice-board {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1.25rem;
    align-items: stretch;
  }

  .notice-card {
    position: relative;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 1.5rem;
    padding: 1.35rem 1.35rem 1.25rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
    box-shadow: 0 2px 6px rgb(15 23 42 / 0.04);
    text-decoration: none;
    color: inherit;
    transition:
      transform 0.22s cubic-bezier(0.22, 1, 0.36, 1),
      box-shadow 0.18s ease,
      border-color 0.18s ease;
  }

  .setor.ativo .notice-card:hover,
  .setor.ativo .notice-card:focus-visible {
    transform: scale(1.025) translateY(-2px);
    box-shadow: 0 12px 26px rgb(15 23 42 / 0.1);
    border-color: var(--seno-gray-300);
  }

  .notice-cabeca {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .notice-titulo {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 600;
    line-height: 1.3;
    color: var(--seno-gray-900);
  }

  .notice-materia {
    margin: 0;
    font-size: 0.84rem;
    font-weight: 500;
    color: var(--seno-gray-500);
  }

  /* ---- Timeline (4 marcos) ---- */
  .timeline {
    list-style: none;
    margin: 0;
    padding: 0.9rem 0 0;
    border-top: 1px solid var(--seno-gray-100);
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0;
  }

  .etapa {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.35rem;
    padding-inline: 0.15rem;
    text-align: center;
  }

  .etapa::before {
    content: "";
    position: absolute;
    top: 7px;
    left: -50%;
    right: 50%;
    height: 2px;
    background: var(--seno-gray-200);
  }

  .etapa:first-child::before {
    display: none;
  }

  .etapa.concluida::before,
  .etapa.atual::before {
    background: var(--seno-primary);
  }

  .etapa.em-progresso-etapa::before {
    background: linear-gradient(
      to right,
      var(--seno-primary),
      var(--seno-amber-500)
    );
  }

  .etapa-marcador {
    position: relative;
    z-index: 1;
    height: 16px;
    display: grid;
    place-items: center;
  }

  .etapa-ponto {
    position: relative;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--seno-gray-300);
    display: grid;
    place-items: center;
    transition: transform 0.15s ease;
  }

  .etapa-ponto.com-icone {
    width: 16px;
    height: 16px;
    color: var(--seno-white);
  }

  .etapa.concluida .etapa-ponto {
    background: var(--seno-primary);
  }

  .etapa.atual .etapa-ponto {
    background: var(--seno-primary);
    box-shadow: 0 0 0 3px var(--seno-blue-100);
  }

  .etapa.em-progresso-etapa .etapa-ponto {
    background: var(--seno-amber-500);
    box-shadow: 0 0 0 3px var(--seno-amber-200);
  }

  .etapa.expirada .etapa-ponto {
    background: var(--seno-red);
    box-shadow: 0 0 0 3px var(--seno-red-50);
  }

  .etapa.expirada .etapa-rotulo {
    color: var(--seno-red-700);
    font-weight: 600;
  }

  .timeline.expirada-em-progresso .etapa.concluida .etapa-ponto {
    background: var(--seno-red);
  }

  .timeline.expirada-em-progresso .etapa.expirada::before {
    background: var(--seno-red);
  }

  .timeline.corrigida .etapa::before {
    background: var(--seno-green);
  }

  .timeline.corrigida .etapa .etapa-ponto {
    background: var(--seno-green);
  }

  /* ---- Setas laterais nas bordas ---- */
  .seta-lateral {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    z-index: 8;
    width: 2.5rem;
    height: 2.5rem;
    display: grid;
    place-items: center;
    border-radius: 50%;
    border: 1px solid var(--seno-line);
    background: var(--seno-white);
    color: var(--seno-gray-900);
    box-shadow: 0 4px 12px rgb(15 23 42 / 0.12);
    cursor: pointer;
    opacity: 0;
    pointer-events: none;
    transition:
      opacity 0.25s ease,
      transform 0.2s ease,
      box-shadow 0.2s ease;
  }

  .seta-lateral.esq {
    left: -1.25rem;
  }

  .seta-lateral.dir {
    right: -1.25rem;
  }

  .seta-lateral.visivel {
    opacity: 0.4;
    pointer-events: auto;
  }

  .seta-lateral.visivel:hover {
    opacity: 1;
    transform: translateY(-50%) scale(1.06);
    box-shadow: 0 6px 16px rgb(15 23 42 / 0.18);
  }

  .etapa.em-progresso-etapa .etapa-ponto::after {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: 50%;
    background: var(--seno-amber-500);
    animation: radar-pulso 1.8s cubic-bezier(0.22, 1, 0.36, 1) infinite;
    pointer-events: none;
  }

  @keyframes radar-pulso {
    0% {
      transform: scale(1);
      opacity: 0.7;
    }
    70%,
    100% {
      transform: scale(2.8);
      opacity: 0;
    }
  }

  .etapa-rotulo {
    font-size: 0.7rem;
    line-height: 1.15;
    font-weight: 500;
    color: var(--seno-gray-500);
  }

  .etapa.concluida .etapa-rotulo,
  .etapa.atual .etapa-rotulo {
    color: var(--seno-gray-900);
    font-weight: 600;
  }

  .etapa.em-progresso-etapa .etapa-rotulo {
    color: var(--seno-amber);
  }

  /* ---- Calendário estilizado (modo minimizado) ---- */
  .calendario-palco {
    grid-area: 1 / 1;
    position: relative;
    z-index: 1;
    display: flex;
    justify-content: center;
    transform: translateY(115px);
    opacity: 0;
    pointer-events: none;
    transition:
      transform 0.46s cubic-bezier(0.22, 1, 0.36, 1),
      opacity 0.34s ease;
  }

  .calendario-palco.visivel {
    z-index: 5;
    transform: translateY(0);
    opacity: 1;
    pointer-events: auto;
  }

  .calendario-card {
    width: 100%;
    max-width: 28rem;
    padding: 1.5rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
    box-shadow: 0 8px 24px rgb(15 23 42 / 0.06);
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .cal-cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .cal-mes {
    font-size: 1.05rem;
    font-weight: 600;
    color: var(--seno-gray-900);
  }

  .cal-seta {
    width: 2rem;
    height: 2rem;
    display: grid;
    place-items: center;
    background: transparent;
    border: 1px solid var(--seno-line);
    border-radius: 50%;
    color: var(--seno-gray-700);
    cursor: pointer;
    transition: background-color 0.14s ease;
  }

  .cal-seta:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
  }

  .cal-semana {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    text-align: center;
    font-size: 0.76rem;
    font-weight: 600;
    color: var(--seno-gray-500);
  }

  .cal-grade {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    gap: 0.4rem;
  }

  .cal-dia {
    aspect-ratio: 1;
    display: grid;
    place-items: center;
    border-radius: 50%;
    font: inherit;
    font-size: 0.88rem;
    border: none;
    background: transparent;
  }

  .cal-dia.sem-atividade {
    color: var(--seno-gray-500);
    background: var(--seno-gray-100);
  }

  .cal-dia.sem-atividade.fora-mes {
    background: transparent;
    color: var(--seno-gray-300);
  }

  .cal-dia.com-atividade {
    background: var(--seno-blue-100);
    color: var(--seno-blue-700);
    font-weight: 600;
    cursor: pointer;
    transition:
      transform 0.14s ease,
      background-color 0.14s ease,
      color 0.14s ease;
  }

  .cal-dia.com-atividade:hover {
    background: var(--seno-primary);
    color: var(--seno-white);
    transform: scale(1.08);
  }

  .cal-dia.hoje {
    outline: 2px solid var(--seno-blue-300);
    outline-offset: -2px;
  }

  /* ---- Estado vazio ---- */
  .vazio-board {
    grid-area: 1 / 1;
    position: relative;
    z-index: 5;
    padding: 3.5rem 1.5rem;
    text-align: center;
    background: var(--seno-white);
    border: 1px dashed var(--seno-gray-300);
    border-radius: var(--raio-lg);
    transition:
      transform 0.46s cubic-bezier(0.22, 1, 0.36, 1),
      opacity 0.34s ease;
  }

  .vazio-board.escondido {
    z-index: 1;
    transform: translateY(115px);
    opacity: 0;
    pointer-events: none;
  }

  .vazio-titulo {
    margin: 0 0 0.35rem;
    font-size: 1.05rem;
    font-weight: 600;
    color: var(--seno-gray-900);
  }

  .vazio-texto {
    margin: 0;
    font-size: 0.9rem;
    color: var(--seno-gray-500);
  }

  @media (max-width: 960px) {
    .notice-board {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 720px) {
    .topo-board {
      grid-template-columns: 1fr;
      justify-items: center;
    }

    .topo-esq {
      display: none;
    }

    .controles-dir {
      justify-self: center;
    }
  }

  @media (max-width: 620px) {
    .notice-board {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
