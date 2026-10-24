<script lang="ts">
  // Criação de atividade: nome + lista de tarefas (do banco) com ordem
  // definida visualmente (drag-and-drop e click & add), pontuação e linguagem por tarefa.
  import type { PageProps } from "./$types";
  import {
    adicionarAoWorkspace,
    removerDoWorkspace,
    moverWorkspaceParaAtividade,
    removerDaAtividade,
    reordenarAtividade,
    serializarAtributos,
    filtrarTarefas,
    paginar,
    type AtributoAtividade,
  } from "#lib/atividade-workspace";

  let { data, form }: PageProps = $props();

  // Lista de tarefas na atividade (ordenada e configurada)
  let atributos = $state<AtributoAtividade[]>([]);

  // Linha inferior: tarefas importadas para o workspace e ainda não adicionadas à atividade
  let workspaceTarefas = $state<number[]>([]);

  // Tarefa selecionada no workspace (ao clicar, mostra opções de remoção e adição)
  let cardSelecionadoId = $state<number | null>(null);

  // JSON final para gravação no servidor
  let atributosJSON = $derived(serializarAtributos(atributos));

  // Estado do modal de importação de tarefas
  let modalAberto = $state(false);
  let buscaModal = $state("");
  let paginaModal = $state(1);
  let selecionadasModal = $state<number[]>([]);
  const porPagina = 6;

  // Estado do Drag and Drop
  let dragging = $state<
    | { tipo: "workspace"; id: number }
    | { tipo: "atividade"; index: number }
    | null
  >(null);
  let dropTargetIndex = $state<number | null>(null);
  let dropOverWorkspace = $state(false);

  function tarefaDe(id: number | null) {
    if (id == null) return null;
    return (data.tarefas ?? []).find((x) => x.id === id) ?? null;
  }

  // ---- Modal ----
  function abrirModal() {
    modalAberto = true;
    buscaModal = "";
    paginaModal = 1;
    selecionadasModal = [];
  }

  function fecharModal() {
    modalAberto = false;
    selecionadasModal = [];
  }

  const tarefasFiltradas = $derived(
    filtrarTarefas(data.tarefas ?? [], buscaModal),
  );

  const paginacao = $derived(
    paginar(tarefasFiltradas, paginaModal, porPagina),
  );

  function toggleSelecaoModal(id: number) {
    if (selecionadasModal.includes(id)) {
      selecionadasModal = selecionadasModal.filter((x) => x !== id);
    } else {
      selecionadasModal = [...selecionadasModal, id];
    }
  }

  function selecionarTodasPagina() {
    const idsPagina = paginacao.itens
      .map((t) => t.id)
      .filter(
        (id) =>
          !workspaceTarefas.includes(id) &&
          !atributos.some((a) => a.tarefa_id === id),
      );
    const novoSet = new Set([...selecionadasModal, ...idsPagina]);
    selecionadasModal = Array.from(novoSet);
  }

  function desmarcarTodasModal() {
    selecionadasModal = [];
  }

  function confirmarImportacao() {
    const atvIds = atributos.map((a) => a.tarefa_id);
    workspaceTarefas = adicionarAoWorkspace(
      workspaceTarefas,
      selecionadasModal,
      atvIds,
    );
    fecharModal();
  }

  // ---- Ações do Workspace (Click & Add / Remover) ----
  function toggleCardWorkspace(id: number) {
    cardSelecionadoId = cardSelecionadoId === id ? null : id;
  }

  function addTarefaDoWorkspace(id: number) {
    const res = moverWorkspaceParaAtividade(
      workspaceTarefas,
      atributos,
      id,
      atributos.length,
    );
    workspaceTarefas = res.workspace;
    atributos = res.atributos;
    if (cardSelecionadoId === id) cardSelecionadoId = null;
  }

  function removerDoWorkspaceClick(id: number) {
    workspaceTarefas = removerDoWorkspace(workspaceTarefas, id);
    if (cardSelecionadoId === id) cardSelecionadoId = null;
  }

  // ---- Ações da Atividade ----
  function removerAtividadeClick(index: number) {
    const res = removerDaAtividade(atributos, index, workspaceTarefas);
    workspaceTarefas = res.workspace;
    atributos = res.atributos;
  }

  function toggleExpandida(index: number) {
    if (atributos[index]) {
      atributos[index].expandida = !atributos[index].expandida;
    }
  }

  // ---- Drag & Drop ----
  function onDragStartWorkspace(e: DragEvent, id: number) {
    dragging = { tipo: "workspace", id };
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", String(id));
    }
  }

  function onDragStartAtividade(e: DragEvent, index: number) {
    dragging = { tipo: "atividade", index };
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", String(index));
    }
  }

  function onDragEnd() {
    dragging = null;
    dropTargetIndex = null;
    dropOverWorkspace = false;
  }

  function onDragOverContainer(e: DragEvent) {
    e.preventDefault();
    if (!dragging) return;
    if (atributos.length === 0) {
      dropTargetIndex = 0;
    } else {
      const target = e.target as HTMLElement | null;
      if (
        target &&
        (target.classList.contains("secao-atividade") ||
          target.classList.contains("lista-tarefas") ||
          target.classList.contains("badge-total"))
      ) {
        dropTargetIndex = atributos.length;
      }
    }
  }

  function onDragLeaveContainer(e: DragEvent) {
    const related = e.relatedTarget as HTMLElement | null;
    const current = e.currentTarget as HTMLElement | null;
    if (current && !current.contains(related)) {
      dropTargetIndex = null;
    }
  }

  function onDragOverItem(e: DragEvent, index: number) {
    e.preventDefault();
    e.stopPropagation();
    if (!dragging) return;

    const targetEl = e.currentTarget as HTMLElement | null;
    if (!targetEl) return;
    const rect = targetEl.getBoundingClientRect();
    const offsetY = e.clientY - rect.top;

    // Metade superior insere antes do item; metade inferior insere depois
    if (offsetY < rect.height / 2) {
      dropTargetIndex = index;
    } else {
      dropTargetIndex = index + 1;
    }
  }

  function onDropActivity(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (!dragging) return;

    const targetIdx = dropTargetIndex ?? atributos.length;

    if (dragging.tipo === "workspace") {
      const res = moverWorkspaceParaAtividade(
        workspaceTarefas,
        atributos,
        dragging.id,
        targetIdx,
      );
      workspaceTarefas = res.workspace;
      atributos = res.atributos;
      if (cardSelecionadoId === dragging.id) cardSelecionadoId = null;
    } else if (dragging.tipo === "atividade") {
      atributos = reordenarAtividade(atributos, dragging.index, targetIdx);
    }

    onDragEnd();
  }

  function onDragOverWorkspaceZone(e: DragEvent) {
    if (dragging && dragging.tipo === "atividade") {
      e.preventDefault();
      dropOverWorkspace = true;
    }
  }

  function onDragLeaveWorkspaceZone() {
    dropOverWorkspace = false;
  }

  function onDropWorkspaceZone(e: DragEvent) {
    e.preventDefault();
    if (dragging && dragging.tipo === "atividade") {
      removerAtividadeClick(dragging.index);
    }
    onDragEnd();
  }

  function lingLabel(l: string) {
    switch (l) {
      case "python":
        return "Python";
      case "c":
        return "C";
      case "cpp":
        return "C++";
      default:
        return l;
    }
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === "Escape" && modalAberto) fecharModal();
  }}
/>

<section class="nova-atividade">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}

  <form method="POST" action="?/criar" class="cartao">
    <div class="cartao-cabecalho">
      <h2>Nova atividade</h2>
      <span class="dica">
        Arraste ou clique nas tarefas do workspace para estruturar a atividade.
      </span>
    </div>

    <label class="campo-nome">
      <span>Nome da atividade</span>
      <input name="nome" required maxlength="128" placeholder="Ex: Prova 1" />
    </label>

    <!-- Lista de Tarefas da Atividade -->
    <fieldset
      class="secao-atividade"
      ondragover={onDragOverContainer}
      ondragleave={onDragLeaveContainer}
      ondrop={onDropActivity}
    >
      <legend>
        Tarefas da atividade
        {#if atributos.length > 0}
          <span class="badge-total">({atributos.length})</span>
        {/if}
      </legend>

      {#if atributos.length === 0}
        <div
          class="zona-vazia"
          class:drop-ativo={dropTargetIndex === 0}
        >
          <span class="icone-alvo">🎯</span>
          <strong>Nenhuma tarefa na atividade ainda</strong>
          <p class="dica">
            Importe tarefas no workspace abaixo e arraste ou clique em "+ Adicionar" para incluí-las.
          </p>
        </div>
      {:else}
        <div class="lista-tarefas" role="list">
          {#each atributos as a, i (a.tarefa_id)}
            {@const t = tarefaDe(a.tarefa_id)}
            <!-- Linha indicadora de drop antes do item -->
            {#if dropTargetIndex === i}
              <div class="indicador-drop" role="presentation"></div>
            {/if}

            <div
              class="tarefa-item"
              class:expandida={a.expandida}
              role="listitem"
              draggable="true"
              ondragstart={(e) => onDragStartAtividade(e, i)}
              ondragend={onDragEnd}
              ondragover={(e) => onDragOverItem(e, i)}
            >
              <!-- Linha clicável superior / cabeçalho -->
              <div class="tarefa-linha-container">
                <button
                  type="button"
                  class="tarefa-linha-clicavel"
                  onclick={() => toggleExpandida(i)}
                  title={a.expandida ? "Clique para recolher opções" : "Clique para alterar pontuação e linguagem"}
                >
                  <div class="esquerda">
                    <span class="puxador" title="Arraste para reposicionar">⋮⋮</span>
                    <span class="ordem-numero" title="Ordem da tarefa">#{i + 1}</span>
                    <div class="identidade-tarefa">
                      <span class="nome-tarefa">{t ? t.nome : `Tarefa #${a.tarefa_id}`}</span>
                      <span class="id-subtexto">ID #{a.tarefa_id}</span>
                    </div>
                  </div>

                  <div class="direita">
                    <span class="badge-pontos">{a.valor_pts} pts</span>
                    <span class="badge-ling">{lingLabel(a.linguagem)}</span>
                    <span class="chevron" class:rotacionado={a.expandida} title="Expandir/Recolher">▼</span>
                  </div>
                </button>

                <button
                  type="button"
                  class="btn-remover-linha"
                  title="Remover da atividade (volta para o workspace)"
                  onclick={() => removerAtividadeClick(i)}
                  onkeydown={(e) => e.stopPropagation()}
                >
                  ✕
                </button>
              </div>

              <!-- Painel expandido com opções de pontuação e linguagem -->
              {#if a.expandida}
                <div class="painel-opcoes" draggable="false">
                  <div class="campos-config">
                    <label class="campo-pts">
                      <span>Pontuação (pts)</span>
                      <input
                        type="number"
                        min="0"
                        bind:value={a.valor_pts}
                        title="Valor em pontos desta questão"
                      />
                    </label>

                    <label class="campo-ling">
                      <span>Linguagem permitida</span>
                      <select
                        bind:value={a.linguagem}
                      >
                        <option value="python">Python</option>
                        <option value="c">C</option>
                        <option value="cpp">C++</option>
                      </select>
                    </label>
                  </div>

                  <div class="acoes-linha">
                    <button
                      type="button"
                      class="peq vermelho"
                      onclick={() => removerAtividadeClick(i)}
                    >
                      Remover da atividade
                    </button>
                    <span class="nota-retorno">A tarefa retornará ao workspace de opções</span>
                  </div>
                </div>
              {/if}
            </div>

            <!-- Linha indicadora de drop após o último item -->
            {#if i === atributos.length - 1 && dropTargetIndex === atributos.length}
              <div class="indicador-drop" role="presentation"></div>
            {/if}
          {/each}
        </div>
      {/if}

      <input type="hidden" name="tarefas" value={atributosJSON} />
    </fieldset>

    <!-- Linha inferior: Workspace de tarefas importadas -->
    <div
      class="workspace-container"
      class:drop-alvo={dropOverWorkspace}
      role="region"
      aria-label="Workspace de tarefas"
      ondragover={onDragOverWorkspaceZone}
      ondragleave={onDragLeaveWorkspaceZone}
      ondrop={onDropWorkspaceZone}
    >
      <div class="workspace-topo">
        <div class="workspace-titulos">
          <span class="workspace-label">Workspace de tarefas</span>
          <span class="workspace-contagem">
            ({workspaceTarefas.length} {workspaceTarefas.length === 1 ? "disponível" : "disponíveis"})
          </span>
        </div>
        <span class="workspace-dica">
          Clique em uma tarefa para ver opções ou arraste para a atividade acima.
        </span>
      </div>

      <div class="workspace-trilha" role="list">
        <!-- Botão (+) para abrir o modal de importação -->
        <button
          type="button"
          class="card-adicionar"
          onclick={abrirModal}
          title="Importar tarefas do banco para o workspace"
        >
          <span class="mais-icone">+</span>
          <span class="mais-texto">Importar tarefas</span>
        </button>

        <!-- Cards das tarefas importadas para o workspace -->
        {#each workspaceTarefas as id (id)}
          {@const t = tarefaDe(id)}
          <div
            class="card-tarefa"
            role="listitem"
            class:selecionado={cardSelecionadoId === id}
            draggable="true"
            ondragstart={(e) => onDragStartWorkspace(e, id)}
            ondragend={onDragEnd}
          >
            <button
              type="button"
              class="card-corpo"
              onclick={() => toggleCardWorkspace(id)}
              ondblclick={() => addTarefaDoWorkspace(id)}
              title="Clique para ver opções, duplo-clique para adicionar, ou arraste para a atividade"
            >
              <span class="card-puxador" title="Arraste para a atividade">⋮⋮</span>
              <div class="card-info">
                <span class="card-id">#{id}</span>
                <strong class="card-nome">{t ? t.nome : `Tarefa #${id}`}</strong>
              </div>
            </button>

            <!-- Ao clicar na tarefa do workspace, mostra a opção de removê-la e click & add -->
            {#if cardSelecionadoId === id}
              <div class="card-acoes" draggable="false">
                <button
                  type="button"
                  class="btn-card-add"
                  onclick={(e) => {
                    e.stopPropagation();
                    addTarefaDoWorkspace(id);
                  }}
                  onkeydown={(e) => e.stopPropagation()}
                  title="Adicionar à atividade"
                >
                  + Adicionar
                </button>
                <button
                  type="button"
                  class="btn-card-del"
                  onclick={(e) => {
                    e.stopPropagation();
                    removerDoWorkspaceClick(id);
                  }}
                  onkeydown={(e) => e.stopPropagation()}
                  title="Remover do workspace"
                >
                  Remover
                </button>
              </div>
            {/if}
          </div>
        {:else}
          <div class="workspace-vazio">
            <span>Nenhuma tarefa no workspace. Pressione <strong>(+) Importar tarefas</strong> para buscar no banco.</span>
          </div>
        {/each}
      </div>
    </div>

    <button class="btn-submeter" type="submit" disabled={atributos.length === 0}>
      Criar atividade
    </button>
  </form>

  <a class="voltar" href="/professor/atividades">← Voltar ao banco</a>
</section>

<!-- Modal de Importação de Tarefas -->
{#if modalAberto}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModal();
    }}
  >
    <div
      class="modal-janela"
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-titulo"
    >
      <div class="modal-cabecalho">
        <h3 id="modal-titulo">Importar tarefas do banco</h3>
        <button
          type="button"
          class="btn-fechar-modal"
          onclick={fecharModal}
          aria-label="Fechar janela"
        >
          ✕
        </button>
      </div>

      <!-- Barra de Busca -->
      <div class="modal-busca">
        <input
          type="search"
          placeholder="Buscar por nome ou ID..."
          bind:value={buscaModal}
          oninput={() => {
            paginaModal = 1;
          }}
        />
      </div>

      <!-- Controles de Múltipla Seleção -->
      <div class="modal-barra-selecao">
        <div class="modal-contadores">
          <span>{tarefasFiltradas.length} encontrada{tarefasFiltradas.length === 1 ? "" : "s"}</span>
          {#if selecionadasModal.length > 0}
            <span class="badge-selecionadas">
              {selecionadasModal.length} selecionada{selecionadasModal.length === 1 ? "" : "s"}
            </span>
          {/if}
        </div>
        <div class="modal-botoes-atalho">
          <button type="button" class="btn-atalho" onclick={selecionarTodasPagina}>
            Selecionar página
          </button>
          {#if selecionadasModal.length > 0}
            <button type="button" class="btn-atalho" onclick={desmarcarTodasModal}>
              Desmarcar todas
            </button>
          {/if}
        </div>
      </div>

      <!-- Lista Paginada -->
      <div class="modal-lista">
        {#each paginacao.itens as t (t.id)}
          {@const jaNoWorkspace = workspaceTarefas.includes(t.id)}
          {@const jaNaAtividade = atributos.some((a) => a.tarefa_id === t.id)}
          {@const jaImportada = jaNoWorkspace || jaNaAtividade}
          {@const estaSelecionada = selecionadasModal.includes(t.id)}

          <label
            class="modal-item"
            class:item-selecionado={estaSelecionada}
            class:item-bloqueado={jaImportada}
          >
            <input
              type="checkbox"
              checked={estaSelecionada}
              disabled={jaImportada}
              onchange={() => toggleSelecaoModal(t.id)}
            />
            <span class="item-id">#{t.id}</span>
            <span class="item-nome">{t.nome}</span>
            {#if jaNaAtividade}
              <span class="badge-status na-atividade">Na atividade</span>
            {:else if jaNoWorkspace}
              <span class="badge-status no-workspace">No workspace</span>
            {/if}
          </label>
        {:else}
          <p class="modal-vazio">Nenhuma tarefa encontrada para a busca realizada.</p>
        {/each}
      </div>

      <!-- Paginação -->
      {#if paginacao.totalPaginas > 1}
        <div class="modal-paginacao">
          <button
            type="button"
            class="peq"
            disabled={paginacao.paginaAtual <= 1}
            onclick={() => {
              paginaModal = paginacao.paginaAtual - 1;
            }}
          >
            ← Anterior
          </button>
          <span class="texto-paginacao">
            Página {paginacao.paginaAtual} de {paginacao.totalPaginas}
          </span>
          <button
            type="button"
            class="peq"
            disabled={paginacao.paginaAtual >= paginacao.totalPaginas}
            onclick={() => {
              paginaModal = paginacao.paginaAtual + 1;
            }}
          >
            Próxima →
          </button>
        </div>
      {/if}

      <!-- Rodapé do Modal -->
      <div class="modal-rodape">
        <button type="button" class="peq" onclick={fecharModal}>
          Cancelar
        </button>
        <button
          type="button"
          class="peq azul"
          disabled={selecionadasModal.length === 0}
          onclick={confirmarImportacao}
        >
          Confirmar ({selecionadasModal.length})
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .nova-atividade {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .cartao-cabecalho {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .cartao h2 {
    margin: 0;
  }
  .campo-nome {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .campo-nome span {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  /* Seção de Tarefas da Atividade */
  .secao-atividade {
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-lg);
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    background: var(--seno-gray-50);
  }
  legend {
    font-size: 0.88rem;
    font-weight: 600;
    color: var(--seno-gray-900);
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0 0.3rem;
  }
  .badge-total {
    color: var(--seno-blue-700);
    font-size: 0.82rem;
    font-weight: 600;
  }

  .zona-vazia {
    padding: 2.2rem 1rem;
    text-align: center;
    border: 2px dashed var(--seno-gray-300);
    border-radius: var(--raio-md);
    background: var(--seno-white);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.4rem;
    color: var(--seno-gray-700);
    transition: all 0.15s ease;
  }
  .zona-vazia.drop-ativo {
    border-color: var(--seno-blue-500);
    background: var(--seno-blue-100);
  }
  .icone-alvo {
    font-size: 1.6rem;
  }

  .lista-tarefas {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }

  .indicador-drop {
    height: 4px;
    background: var(--seno-blue-500);
    border-radius: var(--raio-pill);
    box-shadow: 0 0 8px rgba(47, 128, 237, 0.6);
    margin: 2px 0;
    pointer-events: none;
  }

  .tarefa-item {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    overflow: hidden;
    transition: box-shadow 0.15s ease, border-color 0.15s ease, transform 0.15s ease;
    animation: expandirParaLinha 0.22s ease-out;
  }
  @keyframes expandirParaLinha {
    from {
      opacity: 0.6;
      transform: scaleY(0.75);
    }
    to {
      opacity: 1;
      transform: scaleY(1);
    }
  }
  .tarefa-item:hover {
    border-color: var(--seno-blue-400);
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.04);
  }
  .tarefa-item.expandida {
    border-color: var(--seno-blue-500);
    box-shadow: 0 2px 8px rgba(47, 128, 237, 0.12);
  }

  .tarefa-linha-container {
    display: flex;
    align-items: center;
    width: 100%;
    padding-right: 0.4rem;
  }

  .tarefa-linha-clicavel {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.65rem 0.85rem;
    cursor: pointer;
    user-select: none;
    gap: 0.8rem;
    background: transparent;
    border: none;
    flex: 1;
    min-width: 0;
    text-align: left;
    font-family: inherit;
    color: inherit;
  }
  .tarefa-linha-clicavel:hover {
    background: var(--seno-gray-50);
  }
  .esquerda {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
  }
  .puxador {
    color: var(--seno-gray-400);
    font-size: 1rem;
    cursor: grab;
    line-height: 1;
    letter-spacing: -2px;
  }
  .puxador:active {
    cursor: grabbing;
  }
  .ordem-numero {
    font-weight: 700;
    font-size: 0.9rem;
    color: var(--seno-blue-700);
    background: var(--seno-blue-100);
    padding: 0.15rem 0.45rem;
    border-radius: var(--raio-sm);
    min-width: 2rem;
    text-align: center;
  }
  .identidade-tarefa {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .nome-tarefa {
    font-weight: 600;
    font-size: 0.92rem;
    color: var(--seno-gray-900);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .id-subtexto {
    font-size: 0.74rem;
    color: var(--seno-gray-500);
  }

  .direita {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
  }
  .badge-pontos {
    font-size: 0.78rem;
    font-weight: 600;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    padding: 0.2rem 0.5rem;
    border-radius: var(--raio-pill);
    border: 1px solid var(--seno-gray-200);
  }
  .badge-ling {
    font-size: 0.78rem;
    font-weight: 600;
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    padding: 0.2rem 0.5rem;
    border-radius: var(--raio-pill);
    text-transform: uppercase;
  }
  .chevron {
    font-size: 0.65rem;
    color: var(--seno-gray-500);
    transition: transform 0.2s ease;
    margin-left: 0.2rem;
  }
  .chevron.rotacionado {
    transform: rotate(180deg);
  }
  .btn-remover-linha {
    background: transparent;
    border: none;
    color: var(--seno-gray-500);
    font-size: 0.95rem;
    cursor: pointer;
    padding: 0.2rem 0.4rem;
    border-radius: var(--raio-sm);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .btn-remover-linha:hover {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }

  /* Painel de Opções Expandido */
  .painel-opcoes {
    border-top: 1px solid var(--seno-gray-200);
    background: var(--seno-gray-50);
    padding: 0.85rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .campos-config {
    display: flex;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .campo-pts,
  .campo-ling {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    flex: 1;
    min-width: 140px;
  }
  .campo-pts span,
  .campo-ling span {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .campo-pts input,
  .campo-ling select {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-sm);
    padding: 0.45rem 0.6rem;
    font-size: 0.88rem;
  }
  .acoes-linha {
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }
  .nota-retorno {
    font-size: 0.78rem;
    color: var(--seno-gray-500);
  }

  /* Linha inferior: Workspace de Tarefas */
  .workspace-container {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    transition: all 0.15s ease;
  }
  .workspace-container.drop-alvo {
    border-color: var(--seno-blue-500);
    background: var(--seno-blue-100);
  }
  .workspace-topo {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .workspace-titulos {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .workspace-label {
    font-weight: 600;
    font-size: 0.95rem;
    color: var(--seno-blue-900);
  }
  .workspace-contagem {
    font-size: 0.82rem;
    color: var(--seno-gray-500);
  }
  .workspace-dica {
    font-size: 0.78rem;
    color: var(--seno-gray-500);
  }

  .workspace-trilha {
    display: flex;
    gap: 0.65rem;
    overflow-x: auto;
    padding-bottom: 0.4rem;
    align-items: stretch;
  }
  .workspace-trilha::-webkit-scrollbar {
    height: 6px;
  }
  .workspace-trilha::-webkit-scrollbar-thumb {
    background: var(--seno-gray-300);
    border-radius: var(--raio-pill);
  }

  .card-adicionar {
    min-width: 130px;
    background: var(--seno-blue-100);
    border: 2px dashed var(--seno-blue-400);
    border-radius: var(--raio-md);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 0.75rem 0.5rem;
    gap: 0.3rem;
    cursor: pointer;
    color: var(--seno-blue-700);
    transition: all 0.15s ease;
    flex-shrink: 0;
  }
  .card-adicionar:hover {
    background: var(--seno-blue-200);
    border-color: var(--seno-blue-600);
  }
  .mais-icone {
    font-size: 1.4rem;
    font-weight: bold;
    line-height: 1;
  }
  .mais-texto {
    font-size: 0.78rem;
    font-weight: 600;
  }

  .card-tarefa {
    min-width: 170px;
    max-width: 220px;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    padding: 0.65rem 0.75rem;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 0.5rem;
    cursor: pointer;
    user-select: none;
    flex-shrink: 0;
    transition: all 0.15s ease;
  }
  .card-tarefa:hover {
    border-color: var(--seno-blue-500);
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.05);
  }
  .card-tarefa.selecionado {
    border-color: var(--seno-blue-600);
    box-shadow: 0 0 0 2px var(--seno-blue-200);
    background: var(--seno-blue-100);
  }
  .card-corpo {
    display: flex;
    align-items: flex-start;
    gap: 0.4rem;
    background: transparent;
    border: none;
    padding: 0;
    width: 100%;
    text-align: left;
    font-family: inherit;
    cursor: pointer;
    color: inherit;
  }
  .card-puxador {
    color: var(--seno-gray-400);
    font-size: 0.9rem;
    cursor: grab;
    line-height: 1.2;
    letter-spacing: -2px;
  }
  .card-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .card-id {
    font-size: 0.72rem;
    color: var(--seno-gray-500);
  }
  .card-nome {
    font-size: 0.85rem;
    color: var(--seno-gray-900);
    line-height: 1.25;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .card-acoes {
    display: flex;
    gap: 0.35rem;
    margin-top: 0.2rem;
  }
  .btn-card-add {
    flex: 1;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-sm);
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.3rem 0.4rem;
    cursor: pointer;
  }
  .btn-card-add:hover {
    background: var(--seno-blue-700);
  }
  .btn-card-del {
    background: var(--seno-red-50);
    color: var(--seno-red-700);
    border: 1px solid var(--seno-red-200);
    border-radius: var(--raio-sm);
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.3rem 0.4rem;
    cursor: pointer;
  }
  .btn-card-del:hover {
    background: var(--seno-red);
    color: var(--seno-white);
  }

  .workspace-vazio {
    display: flex;
    align-items: center;
    padding: 0.6rem 1rem;
    color: var(--seno-gray-500);
    font-size: 0.82rem;
  }

  .btn-submeter {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.75rem;
    font-weight: 600;
    font-size: 0.95rem;
    cursor: pointer;
    transition: background 0.15s ease;
  }
  .btn-submeter:hover:not(:disabled) {
    background: var(--seno-blue-700);
  }
  .btn-submeter:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  /* Modal de Importação */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(13, 37, 74, 0.45);
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
    max-width: 580px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--seno-gray-200);
  }
  .modal-cabecalho {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--seno-gray-200);
  }
  .modal-cabecalho h3 {
    margin: 0;
    font-size: 1.15rem;
    color: var(--seno-blue-900);
  }
  .btn-fechar-modal {
    background: transparent;
    border: none;
    font-size: 1.1rem;
    color: var(--seno-gray-500);
    cursor: pointer;
    padding: 0.2rem 0.4rem;
    border-radius: var(--raio-sm);
  }
  .btn-fechar-modal:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
  }

  .modal-busca {
    padding: 0.75rem 1.25rem 0.5rem;
  }
  .modal-busca input {
    width: 100%;
    padding: 0.55rem 0.75rem;
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    font-size: 0.9rem;
    background: var(--seno-gray-50);
  }
  .modal-busca input:focus {
    background: var(--seno-white);
  }

  .modal-barra-selecao {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 1.25rem 0.6rem;
    font-size: 0.8rem;
    color: var(--seno-gray-500);
  }
  .modal-contadores {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .badge-selecionadas {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    font-weight: 600;
    padding: 0.15rem 0.4rem;
    border-radius: var(--raio-pill);
  }
  .modal-botoes-atalho {
    display: flex;
    gap: 0.5rem;
  }
  .btn-atalho {
    background: transparent;
    border: none;
    color: var(--seno-blue-700);
    font-size: 0.78rem;
    cursor: pointer;
    padding: 0;
    text-decoration: underline;
  }
  .btn-atalho:hover {
    color: var(--seno-blue-900);
  }

  .modal-lista {
    padding: 0.25rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    overflow-y: auto;
    max-height: 320px;
  }
  .modal-item {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.55rem 0.75rem;
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-md);
    background: var(--seno-white);
    cursor: pointer;
    transition: background 0.1s ease, border-color 0.1s ease;
  }
  .modal-item:hover:not(.item-bloqueado) {
    background: var(--seno-gray-50);
    border-color: var(--seno-blue-400);
  }
  .modal-item.item-selecionado {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-500);
  }
  .modal-item.item-bloqueado {
    background: var(--seno-gray-100);
    opacity: 0.65;
    cursor: not-allowed;
  }
  .item-id {
    font-size: 0.8rem;
    color: var(--seno-gray-500);
    min-width: 2.2rem;
  }
  .item-nome {
    font-size: 0.88rem;
    color: var(--seno-gray-900);
    font-weight: 500;
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .badge-status {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.15rem 0.45rem;
    border-radius: var(--raio-pill);
    flex-shrink: 0;
  }
  .badge-status.na-atividade {
    background: var(--seno-green-50);
    color: var(--seno-green);
    border: 1px solid var(--seno-green-200);
  }
  .badge-status.no-workspace {
    background: var(--seno-amber-50);
    color: var(--seno-amber);
    border: 1px solid var(--seno-amber-200);
  }

  .modal-vazio {
    text-align: center;
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    padding: 1.5rem;
  }

  .modal-paginacao {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.8rem;
    padding: 0.6rem 1.25rem;
    border-top: 1px solid var(--seno-gray-100);
  }
  .texto-paginacao {
    font-size: 0.82rem;
    color: var(--seno-gray-700);
  }

  .modal-rodape {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.6rem;
    padding: 0.85rem 1.25rem;
    border-top: 1px solid var(--seno-gray-200);
    background: var(--seno-gray-50);
  }
  .peq.azul {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-color: var(--seno-blue-600);
  }
  .peq.azul:hover:not(:disabled) {
    background: var(--seno-blue-700);
  }
  .peq.azul:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
</style>
