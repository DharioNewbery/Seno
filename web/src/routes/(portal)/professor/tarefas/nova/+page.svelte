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

    <fieldset>
      <legend>Limites de execução (vazio = padrão global)</legend>
      <div class="limites">
        <label>
          <span>Tempo de CPU (ms)</span>
          <input type="number" name="tempo_cpu_ms" min="0" max="600000" />
        </label>
        <label>
          <span>Tempo total (ms)</span>
          <input type="number" name="tempo_total_ms" min="0" max="600000" />
        </label>
        <label>
          <span>Memória (MB)</span>
          <input type="number" name="memoria_mb" min="0" max="4096" />
        </label>
      </div>
    </fieldset>

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

    <button>Criar tarefa</button>
  </form>
  <a class="voltar" href="/professor/tarefas">← Voltar ao banco</a>
</section>

<style>
  .nova-tarefa {
    max-width: 720px;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
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
    border-radius: 10px;
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
  input,
  textarea {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.65rem;
    font-size: 0.9rem;
    font-family: inherit;
    background: var(--seno-gray-100);
  }
  textarea {
    font-family: monospace;
    font-size: 0.85rem;
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
    border-radius: 8px;
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
    background: #fdecea;
  }
  .pequeno:hover {
    background: var(--seno-blue-700);
  }
  .dica {
    color: var(--seno-gray-500);
    font-size: 0.82rem;
    margin: 0;
  }
  .cartao > button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.65rem;
    font-weight: 600;
    cursor: pointer;
  }
  .cartao > button:hover {
    background: var(--seno-blue-700);
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
  }
</style>
