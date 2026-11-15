<script lang="ts">
  // Detalhe da tarefa: campos editáveis, opções avançadas recolhidas por padrão,
  // visualização unitária e compacta de testes com setas esquerda/direita e modal de edição.
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

  const tarefa = $derived(data.tarefa);
  let nome = $state("");
  let enunciado = $state("");
  let tipo = $state<"programacao" | "arquivo_submissao">("programacao");
  let extensoesPermitidas = $state(".pdf, .zip, .txt, .md, .py, .c, .cpp, .png, .jpg");
  let maxMb = $state(10);
  let cpu = $state<number | undefined>(undefined);
  let total = $state<number | undefined>(undefined);
  let mem = $state<number | undefined>(undefined);

  let testes = $state<TesteFormulario[]>([]);
  let indiceTeste = $state(0);
  let testesJSON = $derived(JSON.stringify(testes));
  const testeAtual = $derived(testes[ajustarIndiceTeste(indiceTeste, testes.length)]);

  let modalTesteAberto = $state(false);
  let indiceTesteEditando = $state<number | null>(null);
  let rascunhoStdin = $state("");
  let rascunhoStdout = $state("");
  let rascunhoPublico = $state(true);

  let carregado = $state(false);

  $effect(() => {
    if (carregado || !tarefa) return;
    nome = tarefa.nome;
    enunciado = tarefa.enunciado ?? "";
    tipo = tarefa.tipo === "arquivo_submissao" ? "arquivo_submissao" : "programacao";
    if (tarefa.config_arquivo) {
      if (tarefa.config_arquivo.extensoes_permitidas?.length) {
        extensoesPermitidas = tarefa.config_arquivo.extensoes_permitidas.join(", ");
      }
      if (tarefa.config_arquivo.max_bytes) {
        maxMb = Math.max(1, Math.round(tarefa.config_arquivo.max_bytes / (1024 * 1024)));
      }
    }
    cpu = tarefa.tempo_cpu_ms || undefined;
    total = tarefa.tempo_total_ms || undefined;
    mem = tarefa.memoria_mb || undefined;
    testes = data.testes.map((t) => ({
      stdin: t.stdin,
      stdout: t.stdout_esperado,
      publico: t.publico,
    }));
    indiceTeste = 0;
    carregado = true;
  });

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

  function delTeste(i: number) {
    const res = removerTesteDaLista(testes, i);
    testes = res.lista;
    indiceTeste = res.indice;
  }

  let confirmando = $state(false);
</script>

<section class="tarefa">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if tarefa}
    {#if form?.erro}
      <p class="aviso erro" role="alert">{form.erro}</p>
    {/if}
    {#if form?.ok}
      <p class="aviso ok">{form.ok}</p>
    {/if}

    <div class="cabeca">
      <h2>{tarefa.nome}</h2>
      <div class="acoes">
        <a class="link" href="/professor/tarefas">← Banco</a>
        <form method="POST" action="?/duplicar">
          <button class="peq">Duplicar</button>
        </form>
        {#if confirmando}
          <form method="POST" action="?/excluir" class="confirma">
            <span>Excluir definitivamente?</span>
            <button class="peq vermelho">Sim</button>
            <button type="button" class="peq" onclick={() => (confirmando = false)}>Não</button>
          </form>
        {:else}
          <button class="peq vermelho" onclick={() => (confirmando = true)}>
            Excluir
          </button>
        {/if}
      </div>
    </div>

    {#if data.analytics}
      <section class="metricas-tarefa-cartao">
        <h3>Métricas de Uso e Desempenho</h3>
        <div class="grade-metricas-tarefa">
          <div class="item-m">
            <span class="rotulo">Atividades que usam</span>
            <strong>{data.analytics.total_atividades}</strong>
          </div>
          <div class="item-m">
            <span class="rotulo">Atribuições em turmas</span>
            <strong>{data.analytics.total_atribuicoes}</strong>
          </div>
          <div class="item-m">
            <span class="rotulo">Submissões avaliadas</span>
            <strong>{data.analytics.total_submissoes_avaliadas}</strong>
          </div>
          <div class="item-m">
            <span class="rotulo">Taxa de Acerto Pleno</span>
            <strong class="destaque-verde">{Math.round(data.analytics.taxa_acerto_geral)}%</strong>
          </div>
        </div>
      </section>
    {/if}

    <form method="POST" action="?/salvar" class="cartao">
      <label>
        <span>Nome</span>
        <input name="nome" bind:value={nome} required maxlength="128" />
      </label>
      <label>
        <span>Tipo de tarefa</span>
        <select name="tipo" bind:value={tipo}>
          <option value="programacao">Programação (editor de código e casos de teste)</option>
          <option value="arquivo_submissao">Submissão de arquivo (upload com validação de tipo e tamanho)</option>
        </select>
      </label>
      <label>
        <span>Enunciado</span>
        <textarea name="enunciado" rows="6" bind:value={enunciado} required></textarea>
      </label>

      {#if tipo === "arquivo_submissao"}
        <fieldset>
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
          <p class="dica" style="margin:0;font-size:0.8rem;color:var(--seno-gray-600);">
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
            <fieldset>
              <legend>Limites de execução (vazio ou 0 = padrão do executor)</legend>
              <div class="limites">
                <label>
                  <span>Tempo de CPU (ms)</span>
                  <input type="number" name="tempo_cpu_ms" bind:value={cpu} min="0" max="600000" placeholder="2000" />
                </label>
                <label>
                  <span>Tempo total (ms)</span>
                  <input type="number" name="tempo_total_ms" bind:value={total} min="0" max="600000" placeholder="5000" />
                </label>
                <label>
                  <span>Memória (MB)</span>
                  <input type="number" name="memoria_mb" bind:value={mem} min="0" max="4096" placeholder="256" />
                </label>
              </div>
            </fieldset>
          </div>
        </details>

        <fieldset class="secao-testes">
          <div class="topo-testes">
            <div class="titulo-testes">
              <span class="rotulo-secao">Testes (salvar substitui a lista inteira)</span>
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
                    onclick={() => delTeste(indiceTeste)}
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
            <p class="dica">Sem testes = tarefa aberta (nota manual).</p>
          {/if}

          <input type="hidden" name="testes" value={testesJSON} />
        </fieldset>
      {/if}

      <button>Salvar tarefa</button>
    </form>
  {/if}
</section>

{#if modalTesteAberto}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModalTeste();
    }}
  >
    <div class="modal-dialog modal-compacto" role="dialog" aria-modal="true" aria-labelledby="titulo-modal-teste-detalhe">
      <div class="modal-topo">
        <h3 id="titulo-modal-teste-detalhe">
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
  .tarefa {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .cabeca h2 {
    margin: 0;
  }
  .acoes {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
  }
  .acoes form {
    display: contents;
  }
  .link {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
  .confirma {
    align-items: center;
    gap: 0.35rem;
    color: var(--seno-red);
    font-size: 0.83rem;
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.4rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  label span {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  fieldset input {
    background: var(--seno-white);
  }
  .textarea-mono {
    font-family: monospace;
    font-size: 0.85rem;
  }
  .limites {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .limites label {
    flex: 1 1 140px;
  }
  fieldset {
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-lg);
    padding: 0.8rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    margin: 0;
  }
  legend {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .linha-pequena {
    flex-direction: row !important;
    gap: 0.35rem !important;
    align-items: center;
    width: fit-content;
  }
  .linha-pequena span {
    font-weight: 500;
  }
  .cartao > button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.65rem;
    font-weight: 600;
    cursor: pointer;
  }
  .cartao > button:hover {
    background: var(--seno-blue-700);
  }

  /* Opções avançadas */
  details.secao-avancadas {
    display: block;
  }
  summary.btn-avancadas {
    list-style: none;
    background: none;
    border: none;
    padding: 0.2rem 0;
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    color: var(--seno-blue-600);
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    width: fit-content;
    border-radius: var(--raio-sm);
    transition: color 0.12s ease;
    user-select: none;
  }
  summary.btn-avancadas::-webkit-details-marker {
    display: none;
  }
  summary.btn-avancadas:hover {
    color: var(--seno-blue-800);
    text-decoration: underline;
  }
  summary.btn-avancadas:focus-visible {
    outline: 2px solid var(--seno-blue-600);
    outline-offset: 2px;
  }
  summary.btn-avancadas .seta {
    transform-origin: center;
    transition: transform 0.15s ease;
  }
  details[open] > summary.btn-avancadas .seta {
    transform: rotate(90deg);
  }
  .painel-avancadas {
    margin-top: 0.5rem;
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
    max-height: 5.5rem;
    overflow-y: auto;
  }
  .dica {
    color: var(--seno-gray-500);
    font-size: 0.84rem;
    margin: 0;
  }

  /* Métricas */
  .metricas-tarefa-cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.1rem 1.3rem;
  }
  .metricas-tarefa-cartao h3 {
    margin: 0 0 0.8rem;
    font-size: 1rem;
    color: var(--seno-gray-700);
  }
  .grade-metricas-tarefa {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 0.9rem;
  }
  .item-m {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .item-m .rotulo {
    font-size: 0.75rem;
    color: var(--seno-gray-500);
    text-transform: uppercase;
    font-weight: 600;
  }
  .item-m strong {
    font-size: 1.25rem;
    color: var(--seno-blue-900);
  }
  .destaque-verde {
    color: var(--seno-green) !important;
  }

  /* Modal de Teste */
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
