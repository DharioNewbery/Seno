<script lang="ts">
  // Criação de atividade: nome + lista de tarefas (do banco) com ordem
  // crescente, pontuação e linguagem por tarefa. O JSON (schema_version
  // 1) é montado no servidor a partir desta lista.
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  interface Atributo {
    tarefa_id: number | null;
    valor_pts: number;
    linguagem: string;
  }

  let atributos = $state<Atributo[]>([]);
  let atributosJSON = $state("");
  $effect(() => {
    atributosJSON = JSON.stringify(
      atributos.filter((a) => a.tarefa_id != null),
    );
  });

  function addTarefa() {
    atributos = [
      ...atributos,
      { tarefa_id: null, valor_pts: 10, linguagem: "python" },
    ];
  }
  function delAtributo(i: number) {
    atributos = atributos.filter((_, idx) => idx !== i);
  }
  const nomeDe = (id: number | null): string => {
    const t = data.tarefas.find((x) => x.id === id);
    return t ? `${t.id}: ${t.nome}` : "—";
  };
</script>

<section class="nova-atividade">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}

  <form method="POST" action="?/criar" class="cartao">
    <h2>Nova atividade</h2>

    <label>
      <span>Nome</span>
      <input name="nome" required maxlength="128" placeholder="Prova 1" />
    </label>

    <fieldset>
      <legend>Tarefas (ordem = linha; ordem definitiva = ordem de gravação)</legend>
      {#each atributos as a, i (i)}
        <div class="tarefa">
          <span class="num">{i + 1}</span>
          <select required bind:value={a.tarefa_id}>
            <option value={null} disabled selected>Escolha a tarefa</option>
            {#each data.tarefas as t (t.id)}
              <option value={t.id}>#{t.id} — {t.nome}</option>
            {/each}
          </select>
          <input type="number" min="0" bind:value={a.valor_pts} title="Valor (pontos)" />
          <select bind:value={a.linguagem} title="Linguagem">
            <option value="python">Python</option>
            <option value="c">C</option>
            <option value="cpp">C++</option>
          </select>
          <button type="button" class="peq vermelho" onclick={() => delAtributo(i)}>
            Remover
          </button>
        </div>
        <span class="rotulo">{nomeDe(a.tarefa_id)}</span>
      {:else}
        <p class="dica">
          Nenhuma tarefa ainda. Adicione as questões do banco.
        </p>
      {/each}
      <button type="button" class="peq" onclick={addTarefa}>+ Tarefa</button>
      <input type="hidden" name="tarefas" value={atributosJSON} />
    </fieldset>

    <button>Criar atividade</button>
  </form>
  <a class="voltar" href="/professor/atividades">← Voltar ao banco</a>
</section>

<style>
  .nova-atividade {
    max-width: 760px;
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
  fieldset select {
    background: var(--seno-white);
  }
  fieldset {
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-lg);
    padding: 0.8rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  legend {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  .tarefa {
    display: grid;
    grid-template-columns: 1.6rem 3fr 5.5rem 6.5rem auto;
    gap: 0.4rem;
    align-items: center;
  }
  .num {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    text-align: right;
  }
  .rotulo {
    color: var(--seno-gray-500);
    font-size: 0.78rem;
    margin-left: 2rem;
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
    </style>
