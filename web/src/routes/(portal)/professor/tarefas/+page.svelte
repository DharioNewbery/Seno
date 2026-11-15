<script lang="ts">
  // Banco de tarefas: listagem com cards expansíveis, modal de criação de tarefa
  // com visualização unitária de testes (setas esquerda/direita) e modal de edição de testes.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import {
    adicionarTesteNaLista,
    ajustarIndiceTeste,
    atualizarTesteNaLista,
    navegarIndiceTeste,
    removerTesteDaLista,
    type TesteFormulario,
  } from "#lib/tarefas-testes";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const tarefas = $derived(data.tarefas);

  let confirmando = $state<number | null>(null);
  let expandidos = $state<Record<number, boolean>>({});

  // Modal de criação de tarefa
  let modalCriarAberto = $state(false);
  let tipo = $state<"programacao" | "arquivo_submissao">("programacao");
  let extensoesPermitidas = $state(".pdf, .zip, .txt, .md, .py, .c, .cpp, .png, .jpg");
  let maxMb = $state(10);
  let tempoCpu = $state(2000);
  let tempoTotal = $state(5000);
  let memoria = $state(256);

  // Estado dos testes na criação (visualização unitária + modal de criação/edição de teste)
  let testes = $state<TesteFormulario[]>([]);
  let indiceTeste = $state(0);
  let testesJSON = $derived(JSON.stringify(testes));
  const testeAtual = $derived(testes[ajustarIndiceTeste(indiceTeste, testes.length)]);

  let modalTesteAberto = $state(false);
  let indiceTesteEditando = $state<number | null>(null);
  let rascunhoStdin = $state("");
  let rascunhoStdout = $state("");
  let rascunhoPublico = $state(true);

  $effect(() => {
    if (form?.acao === "criar" && form?.erro) {
      modalCriarAberto = true;
    }
  });

  function alternar(id: number) {
    expandidos[id] = !expandidos[id];
  }

  function paginadora(delta: number): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    qs.set("pagina", String(data.pagina + delta));
    return `?${qs.toString()}`;
  }

  function abrirModalCriar() {
    modalCriarAberto = true;
  }

  function fecharModalCriar() {
    modalCriarAberto = false;
    modalTesteAberto = false;
  }

  function irTesteAnterior() {
    indiceTeste = navegarIndiceTeste(indiceTeste, -1, testes.length);
  }

  function irProximoTeste() {
    indiceTeste = navegarIndiceTeste(indiceTeste, 1, testes.length);
  }

  function abrirModalNovoTeste() {
    indiceTesteEditando = null;
    rascunhoStdin = "";
    rascunhoStdout = "";
    rascunhoPublico = true;
    modalTesteAberto = true;
  }

  function abrirModalEditarTeste(idx: number) {
    const alvo = testes[idx];
    if (!alvo) return;
    indiceTesteEditando = idx;
    rascunhoStdin = alvo.stdin;
    rascunhoStdout = alvo.stdout;
    rascunhoPublico = alvo.publico;
    modalTesteAberto = true;
  }

  function fecharModalTeste() {
    modalTesteAberto = false;
    indiceTesteEditando = null;
  }

  function salvarModalTeste() {
    const item: TesteFormulario = {
      stdin: rascunhoStdin,
      stdout: rascunhoStdout,
      publico: rascunhoPublico,
    };
    if (indiceTesteEditando === null) {
      const res = adicionarTesteNaLista(testes, item);
      testes = res.lista;
      indiceTeste = res.indice;
    } else {
      const res = atualizarTesteNaLista(testes, indiceTesteEditando, item);
      testes = res.lista;
      indiceTeste = res.indice;
    }
    fecharModalTeste();
  }

  function removerTeste(idx: number) {
    const res = removerTesteDaLista(testes, idx);
    testes = res.lista;
    indiceTeste = res.indice;
  }
</script>

<section class="tarefas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro && form?.acao !== "criar"}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}

  <div class="cabeca">
    <h2>Banco de tarefas <span class="total">({data.total})</span></h2>
    <button type="button" class="nova" onclick={abrirModalCriar}>Criar tarefa +</button>
  </div>

  <ul class="lista">
    {#each tarefas as t (t.id)}
      <li class="item-tarefa" class:expandido={expandidos[t.id]}>
        <div
          class="card-topo"
          role="button"
          tabindex="0"
          aria-expanded={Boolean(expandidos[t.id])}
          onclick={() => alternar(t.id)}
          onkeydown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              alternar(t.id);
            }
          }}
        >
          <div class="identidade">
            <svg
              class="chevron"
              class:rotacionado={expandidos[t.id]}
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <polyline points="9 6 15 12 9 18" />
            </svg>
            <span class="nome">{t.nome}</span>
            {#if t.tipo === "arquivo_submissao"}
              <span class="badge-tipo arquivo">Submissão de arquivo</span>
            {:else}
              <span class="badge-tipo prog">Programação</span>
            {/if}
          </div>

          <div
            class="acoes"
            onclick={(e) => e.stopPropagation()}
            onkeydown={(e) => e.stopPropagation()}
            role="toolbar"
            aria-label="Ações da tarefa"
            tabindex="-1"
          >
            <form method="POST" action="?/duplicar">
              <input type="hidden" name="id" value={t.id} />
              <button
                type="submit"
                class="btn-icone"
                title="Duplicar tarefa"
                aria-label="Duplicar tarefa"
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                </svg>
              </button>
            </form>

            {#if confirmando === t.id}
              <form method="POST" action="?/excluir" class="confirma">
                <span>Excluir?</span>
                <input type="hidden" name="id" value={t.id} />
                <button type="submit" class="peq vermelho">Sim</button>
                <button
                  type="button"
                  class="peq"
                  onclick={() => (confirmando = null)}
                >
                  Não
                </button>
              </form>
            {:else}
              <button
                type="button"
                class="btn-icone vermelho"
                title="Excluir tarefa"
                aria-label="Excluir tarefa"
                onclick={() => (confirmando = t.id)}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M3 6h18" />
                  <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                  <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                  <line x1="10" y1="11" x2="10" y2="17" />
                  <line x1="14" y1="11" x2="14" y2="17" />
                </svg>
              </button>
            {/if}
          </div>
        </div>

        {#if expandidos[t.id]}
          <div class="card-detalhes">
            <div class="enunciado-bloco">
              <span class="enunciado-rotulo">Enunciado</span>
              {#if t.enunciado && t.enunciado.trim()}
                <p class="enunciado-texto">{t.enunciado}</p>
              {:else}
                <p class="enunciado-vazio">Sem enunciado cadastrado.</p>
              {/if}
            </div>

            <div class="rodape-detalhes">
              <a class="btn-detalhes" href={`/professor/tarefas/${t.id}`}>
                Ver detalhes →
              </a>
            </div>
          </div>
        {/if}
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

{#if modalCriarAberto}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModalCriar();
    }}
  >
    <div class="modal-dialog modal-largo" role="dialog" aria-modal="true" aria-labelledby="titulo-modal-criar-tarefa">
      <div class="modal-topo">
        <h3 id="titulo-modal-criar-tarefa">Nova tarefa</h3>
        <button type="button" class="btn-fechar" aria-label="Fechar" onclick={fecharModalCriar}>×</button>
      </div>

      {#if form?.acao === "criar" && form?.erro}
        <p class="aviso erro" role="alert">{form.erro}</p>
      {/if}

      <form method="POST" action="?/criar" class="form-modal">
        <div class="linha-dupla">
          <label class="col-maior">
            <span>Nome</span>
            <input name="nome" required maxlength="128" placeholder="Sequência de Fibonacci" />
          </label>

          <label class="col-menor">
            <span>Tipo de tarefa</span>
            <select name="tipo" bind:value={tipo}>
              <option value="programacao">Programação (código e testes)</option>
              <option value="arquivo_submissao">Submissão de arquivo (upload)</option>
            </select>
          </label>
        </div>

        <label>
          <span>Enunciado</span>
          <textarea
            name="enunciado"
            rows="4"
            required
            placeholder="Descrição completa da tarefa para o aluno"
          ></textarea>
        </label>

        {#if tipo === "arquivo_submissao"}
          <fieldset class="secao-fieldset">
            <legend>Restrições do Arquivo Submetido</legend>
            <div class="limites">
              <label style="flex: 2 1 260px;">
                <span>Extensões permitidas (separadas por vírgula)</span>
                <input
                  name="extensoes_permitidas"
                  bind:value={extensoesPermitidas}
                  placeholder=".pdf, .zip, .py, .png"
                  required
                />
              </label>
              <label style="flex: 1 1 140px;">
                <span>Tamanho máximo (MB, até 50 MB)</span>
                <input
                  type="number"
                  name="max_mb"
                  min="1"
                  max="50"
                  step="1"
                  bind:value={maxMb}
                  required
                />
              </label>
            </div>
            <p class="dica-arquivo">
              Extensões aceitas pelo verificador de assinatura: <code>.pdf</code>, <code>.zip</code>, <code>.png</code>, <code>.jpg</code>, <code>.jpeg</code>, <code>.txt</code>, <code>.md</code>, <code>.py</code>, <code>.c</code>, <code>.cpp</code>, <code>.h</code>, <code>.hpp</code>, <code>.csv</code>, <code>.json</code>.
            </p>
          </fieldset>
        {:else}
          <details class="secao-avancadas">
            <summary class="btn-avancadas">
              <svg
                class="seta"
                width="12"
                height="12"
                viewBox="0 0 12 12"
                aria-hidden="true"
              >
                <path
                  d="M4 2l4 4-4 4"
                  stroke="currentColor"
                  stroke-width="1.6"
                  fill="none"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              <span>Opções avançadas</span>
            </summary>

            <div class="painel-avancadas">
              <fieldset class="secao-fieldset">
                <legend>Limites de execução (vazio ou 0 = padrão do executor)</legend>
                <div class="limites">
                  <label>
                    <span>Tempo de CPU (ms)</span>
                    <input
                      type="number"
                      name="tempo_cpu_ms"
                      min="0"
                      max="600000"
                      placeholder="2000"
                      bind:value={tempoCpu}
                    />
                  </label>
                  <label>
                    <span>Tempo total (ms)</span>
                    <input
                      type="number"
                      name="tempo_total_ms"
                      min="0"
                      max="600000"
                      placeholder="5000"
                      bind:value={tempoTotal}
                    />
                  </label>
                  <label>
                    <span>Memória (MB)</span>
                    <input
                      type="number"
                      name="memoria_mb"
                      min="0"
                      max="4096"
                      placeholder="256"
                      bind:value={memoria}
                    />
                  </label>
                </div>
              </fieldset>
            </div>
          </details>

          <fieldset class="secao-fieldset secao-testes">
            <div class="topo-testes">
              <div class="titulo-testes">
                <span class="rotulo-secao">Testes Automáticos</span>
                <span class="contador-testes">{testes.length}</span>
              </div>
              <button type="button" class="btn-add-teste" onclick={abrirModalNovoTeste}>
                + Adicionar teste
              </button>
            </div>

            {#if testes.length > 0 && testeAtual}
              <div class="visor-teste-unitario">
                <div class="visor-barra">
                  <div class="nav-setas">
                    <button
                      type="button"
                      class="btn-seta"
                      onclick={irTesteAnterior}
                      disabled={indiceTeste <= 0}
                      aria-label="Teste anterior"
                      title="Teste anterior"
                    >
                      ←
                    </button>
                    <span class="indicador-teste">
                      Teste <strong>{indiceTeste + 1}</strong> de <strong>{testes.length}</strong>
                    </span>
                    <button
                      type="button"
                      class="btn-seta"
                      onclick={irProximoTeste}
                      disabled={indiceTeste >= testes.length - 1}
                      aria-label="Próximo teste"
                      title="Próximo teste"
                    >
                      →
                    </button>
                  </div>

                  <div class="acoes-teste-unitario">
                    <span class="badge-vis" class:publico={testeAtual.publico}>
                      {testeAtual.publico ? "Público" : "Privado"}
                    </span>
                    <button
                      type="button"
                      class="btn-acao-teste"
                      onclick={() => abrirModalEditarTeste(indiceTeste)}
                    >
                      Editar
                    </button>
                    <button
                      type="button"
                      class="btn-acao-teste vermelho"
                      onclick={() => removerTeste(indiceTeste)}
                    >
                      Remover
                    </button>
                  </div>
                </div>

                <div class="grade-io-compacta">
                  <div class="coluna-io">
                    <span class="rotulo-io">Entrada (stdin)</span>
                    <pre class="bloco-io">{testeAtual.stdin || "(vazio)"}</pre>
                  </div>
                  <div class="coluna-io">
                    <span class="rotulo-io">Saída esperada (stdout)</span>
                    <pre class="bloco-io">{testeAtual.stdout || "(vazio)"}</pre>
                  </div>
                </div>
              </div>
            {:else}
              <div class="vazio-testes">
                <span>Nenhum teste automático adicionado (tarefa aberta / nota manual).</span>
              </div>
            {/if}

            <input type="hidden" name="testes" value={testesJSON} />
          </fieldset>
        {/if}

        <div class="modal-acoes">
          <button type="button" class="btn-secundario" onclick={fecharModalCriar}>
            Cancelar
          </button>
          <button type="submit" class="btn-primario">Criar tarefa</button>
        </div>
      </form>
    </div>
  </div>
{/if}

{#if modalTesteAberto}
  <div
    class="modal-backdrop camada-superior"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModalTeste();
    }}
  >
    <div class="modal-dialog modal-compacto" role="dialog" aria-modal="true" aria-labelledby="titulo-modal-teste">
      <div class="modal-topo">
        <h3 id="titulo-modal-teste">
          {indiceTesteEditando === null ? "Adicionar teste" : `Editar teste #${indiceTesteEditando + 1}`}
        </h3>
        <button type="button" class="btn-fechar" aria-label="Fechar" onclick={fecharModalTeste}>×</button>
      </div>

      <div class="form-modal">
        <label>
          <span>Entrada (stdin)</span>
          <textarea
            class="textarea-mono"
            rows="3"
            bind:value={rascunhoStdin}
            placeholder="Entrada fornecida ao programa (stdin)"
          ></textarea>
        </label>

        <label>
          <span>Saída esperada (stdout)</span>
          <textarea
            class="textarea-mono"
            rows="3"
            bind:value={rascunhoStdout}
            placeholder="Saída exata esperada (stdout)"
          ></textarea>
        </label>

        <label class="linha-pequena">
          <input type="checkbox" bind:checked={rascunhoPublico} />
          <span>Público (visível como exemplo para o aluno)</span>
        </label>

        <div class="modal-acoes">
          <button type="button" class="btn-secundario" onclick={fecharModalTeste}>
            Cancelar
          </button>
          <button type="button" class="btn-primario" onclick={salvarModalTeste}>
            {indiceTesteEditando === null ? "Adicionar" : "Salvar teste"}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

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
  button.nova {
    border: none;
    cursor: pointer;
    font: inherit;
  }

  .item-tarefa {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    padding: 0;
    transition: background-color 0.15s;
  }
  .item-tarefa.expandido {
    background: var(--seno-white);
  }

  .card-topo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.85rem 1.1rem;
    width: 100%;
    box-sizing: border-box;
    cursor: pointer;
    user-select: none;
    outline: none;
    transition: background-color 0.12s;
  }
  .card-topo:focus-visible {
    outline: 2px solid var(--seno-blue-600);
    outline-offset: -2px;
  }
  .item-tarefa:not(.expandido) .card-topo:hover {
    background: var(--seno-gray-50);
  }

  .identidade {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    flex: 1;
    min-width: 0;
  }
  .chevron {
    color: var(--seno-gray-500);
    transition: transform 0.2s ease, color 0.15s;
    flex-shrink: 0;
  }
  .chevron.rotacionado {
    transform: rotate(90deg);
    color: var(--seno-blue-600);
  }
  .nome {
    color: var(--seno-blue-900);
    font-weight: 600;
    font-size: 0.95rem;
    line-height: 1.35;
    word-break: break-word;
  }
  .badge-tipo {
    font-size: 0.74rem;
    font-weight: 600;
    padding: 0.15rem 0.55rem;
    border-radius: var(--raio-pill);
    flex-shrink: 0;
  }
  .badge-tipo.prog {
    background: var(--seno-blue-50);
    color: var(--seno-blue-700);
    border: 1px solid var(--seno-blue-100);
  }
  .badge-tipo.arquivo {
    background: #faf5ff;
    color: #7e22ce;
    border: 1px solid #f3e8ff;
  }

  .acoes {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-shrink: 0;
  }
  .acoes form {
    display: contents;
  }

  .btn-icone {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    padding: 0;
    border-radius: var(--raio-md);
    border: 1px solid var(--seno-line);
    background: var(--seno-white);
    color: var(--seno-gray-600);
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .btn-icone:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
    border-color: var(--seno-gray-300);
  }
  .btn-icone.vermelho {
    color: var(--seno-gray-600);
  }
  .btn-icone.vermelho:hover {
    background: #fef2f2;
    color: var(--seno-red);
    border-color: #fecaca;
  }

  .confirma {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--seno-red);
    font-size: 0.82rem;
    background: #fef2f2;
    padding: 0.2rem 0.5rem;
    border-radius: var(--raio-md);
    border: 1px solid #fecaca;
  }

  .card-detalhes {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    padding: 0.85rem 1.1rem 1.1rem 1.1rem;
    border-top: 1px dashed var(--seno-line);
    cursor: default;
    user-select: text;
    background: var(--seno-gray-50);
  }

  .enunciado-bloco {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .enunciado-rotulo {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--seno-gray-500);
  }
  .enunciado-texto {
    margin: 0;
    font-size: 0.9rem;
    color: var(--seno-gray-800);
    white-space: pre-wrap;
    line-height: 1.55;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-md);
    padding: 0.75rem 0.95rem;
  }
  .enunciado-vazio {
    margin: 0;
    font-size: 0.88rem;
    color: var(--seno-gray-500);
    font-style: italic;
  }

  .rodape-detalhes {
    display: flex;
    justify-content: flex-end;
  }
  .btn-detalhes {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.45rem 0.95rem;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: var(--raio-md);
    font-size: 0.86rem;
    font-weight: 500;
    text-decoration: none;
    transition: background-color 0.15s ease;
  }
  .btn-detalhes:hover {
    background: var(--seno-blue-700);
    color: var(--seno-white);
    text-decoration: none;
  }

  /* Modais */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(15, 23, 42, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    z-index: 100;
  }
  .modal-backdrop.camada-superior {
    z-index: 120;
    background: rgba(15, 23, 42, 0.55);
  }
  .modal-dialog {
    background: var(--seno-white);
    border-radius: var(--raio-lg);
    border: 1px solid var(--seno-gray-200);
    box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.15);
    width: 100%;
    max-height: 90vh;
    overflow-y: auto;
    padding: 1.35rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.95rem;
  }
  .modal-largo {
    max-width: 680px;
  }
  .modal-compacto {
    max-width: 480px;
  }
  .modal-topo {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .modal-topo h3 {
    margin: 0;
    font-size: 1.15rem;
    color: var(--seno-blue-900);
  }
  .btn-fechar {
    background: none;
    border: none;
    font-size: 1.4rem;
    line-height: 1;
    color: var(--seno-gray-500);
    cursor: pointer;
    padding: 0.2rem 0.4rem;
    border-radius: var(--raio-sm);
  }
  .btn-fechar:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-800);
  }
  .form-modal {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }
  .linha-dupla {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .col-maior {
    flex: 2 1 240px;
  }
  .col-menor {
    flex: 1 1 200px;
  }
  .form-modal label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .form-modal label span {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .textarea-mono {
    font-family: monospace;
    font-size: 0.85rem;
  }
  .secao-fieldset {
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-lg);
    padding: 0.75rem 0.95rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    margin: 0;
  }
  .secao-fieldset legend {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .limites {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .limites label {
    flex: 1 1 140px;
  }
  .dica-arquivo {
    margin: 0;
    font-size: 0.79rem;
    color: var(--seno-gray-600);
  }

  /* Opções avançadas */
  details.secao-avancadas {
    display: block;
  }
  summary.btn-avancadas {
    list-style: none;
    background: none;
    border: none;
    padding: 0.15rem 0;
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--seno-blue-600);
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    width: fit-content;
    border-radius: var(--raio-sm);
    user-select: none;
  }
  summary.btn-avancadas::-webkit-details-marker {
    display: none;
  }
  summary.btn-avancadas:hover {
    color: var(--seno-blue-800);
    text-decoration: underline;
  }
  summary.btn-avancadas .seta {
    transform-origin: center;
    transition: transform 0.15s ease;
  }
  details[open] > summary.btn-avancadas .seta {
    transform: rotate(90deg);
  }
  .painel-avancadas {
    margin-top: 0.45rem;
  }

  /* Visor unitário de testes */
  .secao-testes {
    background: var(--seno-gray-50);
  }
  .topo-testes {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .titulo-testes {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .rotulo-secao {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--seno-gray-800);
  }
  .contador-testes {
    font-size: 0.74rem;
    font-weight: 700;
    background: var(--seno-blue-50);
    color: var(--seno-blue-700);
    border: 1px solid var(--seno-blue-100);
    padding: 0.05rem 0.45rem;
    border-radius: var(--raio-pill);
  }
  .btn-add-teste {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.32rem 0.7rem;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-add-teste:hover {
    background: var(--seno-blue-700);
  }
  .visor-teste-unitario {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-md);
    padding: 0.65rem 0.8rem;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  .visor-barra {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    flex-wrap: wrap;
    padding-bottom: 0.45rem;
    border-bottom: 1px solid var(--seno-gray-100);
  }
  .nav-setas {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .btn-seta {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: var(--raio-sm);
    border: 1px solid var(--seno-gray-300);
    background: var(--seno-white);
    color: var(--seno-gray-800);
    font-weight: 700;
    font-size: 0.88rem;
    cursor: pointer;
    padding: 0;
  }
  .btn-seta:hover:not(:disabled) {
    background: var(--seno-gray-100);
    border-color: var(--seno-gray-400);
  }
  .btn-seta:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .indicador-teste {
    font-size: 0.82rem;
    color: var(--seno-gray-700);
  }
  .acoes-teste-unitario {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .badge-vis {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.1rem 0.5rem;
    border-radius: var(--raio-pill);
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .badge-vis.publico {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .btn-acao-teste {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    color: var(--seno-gray-700);
    border-radius: var(--raio-sm);
    padding: 0.2rem 0.55rem;
    font-size: 0.76rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-acao-teste:hover {
    background: var(--seno-gray-100);
  }
  .btn-acao-teste.vermelho {
    color: var(--seno-red);
    border-color: #fecaca;
  }
  .btn-acao-teste.vermelho:hover {
    background: #fef2f2;
  }
  .grade-io-compacta {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.6rem;
  }
  .coluna-io {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    min-width: 0;
  }
  .rotulo-io {
    font-size: 0.72rem;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--seno-gray-500);
  }
  .bloco-io {
    margin: 0;
    padding: 0.45rem 0.6rem;
    background: var(--seno-gray-50);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-sm);
    font-family: monospace;
    font-size: 0.8rem;
    color: var(--seno-gray-800);
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 4.8rem;
    overflow-y: auto;
  }
  .vazio-testes {
    font-size: 0.82rem;
    color: var(--seno-gray-500);
    padding: 0.45rem 0.2rem;
  }
  .linha-pequena {
    flex-direction: row !important;
    gap: 0.4rem !important;
    align-items: center;
    width: fit-content;
  }
  .linha-pequena span {
    font-weight: 500 !important;
  }
  .modal-acoes {
    display: flex;
    justify-content: flex-end;
    gap: 0.55rem;
    margin-top: 0.2rem;
  }
  .btn-secundario {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    color: var(--seno-gray-700);
    border-radius: var(--raio-md);
    padding: 0.5rem 0.95rem;
    font-size: 0.86rem;
    font-weight: 500;
    cursor: pointer;
  }
  .btn-secundario:hover {
    background: var(--seno-gray-50);
  }
  .btn-primario {
    background: var(--seno-blue-600);
    border: none;
    color: var(--seno-white);
    border-radius: var(--raio-md);
    padding: 0.5rem 1.05rem;
    font-size: 0.86rem;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-primario:hover {
    background: var(--seno-blue-700);
  }
</style>
