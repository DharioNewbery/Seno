<script lang="ts">
  // Detalhe da tarefa: campos editáveis + testes (substituição inteira
  // ao salvar). Tarefa usada em atividade → erro 409 do servidor.
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  interface Teste {
    stdin: string;
    stdout: string;
    publico: boolean;
  }

  const tarefa = $derived(data.tarefa);
  let nome = $state("");
  let enunciado = $state("");
  let cpu = $state<number | undefined>(undefined);
  let total = $state<number | undefined>(undefined);
  let mem = $state<number | undefined>(undefined);
  let testes = $state<Teste[]>([]);
  let testesJSON = $state("");
  let carregado = $state(false);

  $effect(() => {
    if (carregado || !tarefa) return;
    nome = tarefa.nome;
    enunciado = tarefa.enunciado ?? "";
    cpu = tarefa.tempo_cpu_ms || undefined;
    total = tarefa.tempo_total_ms || undefined;
    mem = tarefa.memoria_mb || undefined;
    testes = data.testes.map((t) => ({
      stdin: t.stdin,
      stdout: t.stdout_esperado,
      publico: t.publico,
    }));
    carregado = true;
  });
  $effect(() => {
    testesJSON = JSON.stringify(testes);
  });

  function addTeste() {
    testes = [...testes, { stdin: "", stdout: "", publico: true }];
  }
  function delTeste(i: number) {
    testes = testes.filter((_, idx) => idx !== i);
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

    <form method="POST" action="?/salvar" class="cartao">
      <label>
        <span>Nome</span>
        <input name="nome" bind:value={nome} required maxlength="128" />
      </label>
      <label>
        <span>Enunciado</span>
        <textarea name="enunciado" rows="6" bind:value={enunciado} required></textarea>
      </label>

      <div class="limites">
        <label>
          <span>Tempo de CPU (ms)</span>
          <input type="number" name="tempo_cpu_ms" bind:value={cpu} min="0" max="600000" />
        </label>
        <label>
          <span>Tempo total (ms)</span>
          <input type="number" name="tempo_total_ms" bind:value={total} min="0" max="600000" />
        </label>
        <label>
          <span>Memória (MB)</span>
          <input type="number" name="memoria_mb" bind:value={mem} min="0" max="4096" />
        </label>
      </div>

      <fieldset>
        <legend>Testes (salvar substitui a lista inteira)</legend>
        {#each testes as t, i (i)}
          <div class="teste">
            <textarea rows="2" bind:value={t.stdin} placeholder="stdin"></textarea>
            <textarea rows="2" bind:value={t.stdout} placeholder="Saída esperada"></textarea>
            <label class="linha-pequena">
              <input type="checkbox" bind:value={t.publico} />
              <span>Público</span>
            </label>
            <button type="button" class="peq vermelho" onclick={() => delTeste(i)}>
              Remover
            </button>
          </div>
        {:else}
          <p class="dica">Sem testes = tarefa aberta (nota manual).</p>
        {/each}
        <button type="button" class="peq" onclick={addTeste}>+ Teste</button>
        <input type="hidden" name="testes" value={testesJSON} />
      </fieldset>

      <button>Salvar tarefa</button>
    </form>
  {/if}
</section>

<style>
  .tarefa {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    max-width: 760px;
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
  .peq {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.35rem 0.75rem;
    font-size: 0.83rem;
    cursor: pointer;
  }
  .peq.vermelho {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }
  .peq.vermelho:hover {
    background: #fdecea;
  }
  .peq:hover {
    background: var(--seno-blue-700);
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
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
  input,
  textarea {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.65rem;
    font-size: 0.9rem;
    font-family: inherit;
    background: var(--seno-gray-100);
  }
  fieldset input,
  fieldset textarea {
    background: var(--seno-white);
  }
  fieldset textarea {
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
  .aviso.ok {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
</style>
