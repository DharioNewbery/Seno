<script lang="ts">
  // Detalhe/edição da atividade: nome + tarefas (ordem, pontos,
  // linguagem) com substituição inteira ao salvar, como na API.
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  interface Atributo {
    tarefa_id: number | null;
    valor_pts: number;
    linguagem: string;
  }

  const atividade = $derived(
    data.atividade as
      | { nome?: string; tarefas?: { tarefa_id: number; valor_pts: number; linguagem: string }[] }
      | null,
  );
  let nome = $state("");
  let atributos = $state<Atributo[]>([]);
  let atributosJSON = $state("");
  let carregado = $state(false);

  $effect(() => {
    if (carregado || !atividade) return;
    nome = atividade.nome ?? "";
    atributos = (atividade.tarefas ?? []).map((t) => ({
      tarefa_id: t.tarefa_id,
      valor_pts: t.valor_pts,
      linguagem: t.linguagem,
    }));
    carregado = true;
  });
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

  let confirmando = $state(false);
</script>

<section class="atividade">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if atividade}
    {#if form?.erro}
      <p class="aviso erro" role="alert">{form.erro}</p>
    {/if}
    {#if form?.ok}
      <p class="aviso ok">{form.ok}</p>
    {/if}

    <div class="cabeca">
      <h2>{nome || "Atividade"}</h2>
      <div class="acoes">
        <a class="link" href="/professor/atividades">← Banco</a>
        <form method="POST" action="?/duplicar">
          <button class="peq">Duplicar</button>
        </form>
        {#if confirmando}
          <form method="POST" action="?/excluir" class="confirma">
            <span>Excluir definitivamente?</span>
            <button class="peq vermelho">Sim</button>
            <button type="button" class="peq" onclick={() => (confirmando = false)}>
              Não
            </button>
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

      <fieldset>
        <legend>Tarefas (ordem = ordem das linhas)</legend>
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
        {:else}
          <p class="dica">Sem tarefas. Adicione as questões do banco.</p>
        {/each}
        <button type="button" class="peq" onclick={addTarefa}>+ Tarefa</button>
        <input type="hidden" name="tarefas" value={atributosJSON} />
      </fieldset>

      <button>Salvar atividade</button>
    </form>
  {/if}
</section>

<style>
  .atividade {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    max-width: 780px;
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
    width: fit-content;
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
  select {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.65rem;
    font-size: 0.9rem;
    background: var(--seno-gray-100);
    font-family: inherit;
  }
  fieldset input,
  fieldset select {
    background: var(--seno-white);
  }
  fieldset {
    border: 1px solid var(--seno-gray-200);
    border-radius: 10px;
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
  .dica {
    color: var(--seno-gray-500);
    font-size: 0.84rem;
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
