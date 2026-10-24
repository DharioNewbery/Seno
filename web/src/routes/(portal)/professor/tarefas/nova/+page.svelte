<script lang="ts">
  // Criação de tarefa: nome, enunciado, limites opcionais (0 = padrão
  // global) e uma lista dinâmica de testes (stdin → stdout esperado;
  // comparação exata). Testes são enviados via campo JSON oculto.
  import type { PageProps } from "./$types";

  let { form }: PageProps = $props();

  interface Teste {
    stdin: string;
    stdout: string;
    publico: boolean;
  }

  let testes = $state<Teste[]>([]);
  let testesJSON = $state("");
  let tempoCpu = $state(2000);
  let tempoTotal = $state(5000);
  let memoria = $state(256);

  $effect(() => {
    testesJSON = JSON.stringify(testes);
  });

  function addTeste() {
    testes = [...testes, { stdin: "", stdout: "", publico: true }];
  }
  function delTeste(i: number) {
    testes = testes.filter((_, idx) => idx !== i);
    if (testes.length === 0) testesJSON = "[]";
  }
</script>

<section class="nova-tarefa">
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  <form method="POST" action="?/criar" class="cartao">
    <h2>Nova tarefa</h2>

    <label>
      <span>Nome</span>
      <input name="nome" required maxlength="128" placeholder="Prova 1 — questão 2" />
    </label>

    <label>
      <span>Enunciado</span>
      <textarea name="enunciado" rows="5" required
        placeholder="Descrição completa da tarefa para o aluno"></textarea>
    </label>

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

    <fieldset>
      <legend>Teste (stdin → saída esperada; comparação exata)</legend>
      {#each testes as t, i (i)}
        <div class="teste">
          <textarea name="stdin" rows="2" bind:value={t.stdin}
            placeholder="stdin (entrada)"></textarea>
          <textarea name="stdout" rows="2" bind:value={t.stdout}
            placeholder="Saída esperada"></textarea>
          <label class="linha-pequena">
            <input type="checkbox" bind:checked={t.publico} />
            <span>Público</span>
          </label>
          <button type="button" class="pequeno vermelho" onclick={() => delTeste(i)}>
            Remover
          </button>
        </div>
      {:else}
        <p class="dica">Sem testes = tarefa aberta (correção manual).</p>
      {/each}
      <button type="button" class="pequeno" onclick={addTeste}>+ Teste</button>
      <input type="hidden" name="testes" value={testesJSON} />
    </fieldset>

    <button type="submit" class="btn-submeter">Criar tarefa</button>
  </form>
  <a class="voltar" href="/professor/tarefas">← Voltar ao banco</a>
</section>

<style>
  .nova-tarefa {
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
    gap: 0.9rem;
  }
  .cartao h2 {
    margin: 0;
  }
  fieldset {
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-lg);
    padding: 0.8rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  legend {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--seno-gray-700);
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
      fieldset input,
  fieldset textarea {
    background: var(--seno-white);
  }
  .limites {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .limites label {
    flex: 1 1 140px;
  }
  .teste {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.4rem;
    align-items: start;
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
  .pequeno {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.35rem 0.75rem;
    font-size: 0.83rem;
    cursor: pointer;
    width: fit-content;
  }
  .pequeno.vermelho {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }
  .pequeno.vermelho:hover {
    background: var(--seno-red-50);
  }
  .pequeno:hover {
    background: var(--seno-blue-700);
  }
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
  .cartao > button.btn-submeter {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.65rem 1.25rem;
    font-weight: 600;
    cursor: pointer;
    width: fit-content;
  }
  .cartao > button.btn-submeter:hover {
    background: var(--seno-blue-700);
  }
    </style>
