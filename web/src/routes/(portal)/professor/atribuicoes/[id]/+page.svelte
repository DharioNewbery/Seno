<script lang="ts">
  // Configuração da atribuição: autocomplete, início, prazo, duração e
  // atraso. Editar depois da 1ª submissão será bloqueado (futuro).
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const atribuicao = $derived(data.atribuicao);
  const paraCampo = (v: string | null | undefined): string =>
    v ? new Date(v).toISOString().slice(0, 16) : "";
  const horasDe = (seg: number | null | undefined): string =>
    seg ? String(Math.round((seg / 3600) * 100) / 100) : "";

  let auto = $state("");
  let inicio = $state("");
  let prazo = $state("");
  let duracao = $state("");
  let atraso = $state(false);
  let carregado = $state(false);

  $effect(() => {
    if (carregado || !atribuicao) return;
    auto = atribuicao.autocomplete ? "on" : "off";
    inicio = paraCampo(atribuicao.inicio);
    prazo = paraCampo(atribuicao.prazo);
    duracao = horasDe(atribuicao.duracao_seg);
    atraso = atribuicao.pode_atrasado;
    carregado = true;
  });
</script>

<section class="config">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if atribuicao}
    {#if form?.erro}
      <p class="aviso erro" role="alert">{form.erro}</p>
    {/if}
    {#if form?.ok}
      <p class="aviso ok">{form.ok}</p>
    {/if}

    <div class="cabeca">
      <div>
        <h2>{atribuicao.atividade_nome || "Atividade"}</h2>
        <span class="meta">Turma: {atribuicao.turma_titulo || atribuicao.turma_id}</span>
      </div>
      <a class="voltar" href={`/professor/turmas/${atribuicao.turma_id}#atribuicoes`}>
        ← Painel da turma
      </a>
    </div>

    <form method="POST" action="?/salvar" class="cartao">
      <label>
        <span>Autocomplete</span>
        <select bind:value={auto} name="autocomplete">
          <option value="on">Ligado</option>
          <option value="off">Desligado</option>
        </select>
      </label>
      <label>
        <span>Início (vazio = disponível agora)</span>
        <input type="datetime-local" name="inicio" bind:value={inicio} />
      </label>
      <label>
        <span>Prazo (vazio = aberto até encerrar a turma)</span>
        <input type="datetime-local" name="prazo" bind:value={prazo} />
      </label>
      <label>
        <span>Duração (h; vazio = só o prazo)</span>
        <input type="number" name="duracao_horas" bind:value={duracao} min="0" max="168" step="0.25" />
      </label>
      <label class="linha">
        <input type="checkbox" name="pode_atrasado" bind:value={atraso} />
        <span>Pode ser entregue atrasado (marcado)</span>
      </label>

      <div class="botoes">
        <button>Salvar</button>
        <a class="voltar" href={`/professor/turmas/${atribuicao.turma_id}#atribuicoes`}>
          Cancelar
        </a>
      </div>
    </form>

    <form method="POST" action="?/excluir" class="zona-excluir">
      <button class="acao vermelha">Desligar esta atribuição</button>
    </form>
  {/if}
</section>

<style>
  .config {
    max-width: 520px;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }
  .cabeca h2 {
    margin: 0 0 0.2rem;
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    padding: 1.4rem;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  label.linha {
    flex-direction: row;
    gap: 0.4rem;
    align-items: center;
  }
  label span {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  label.linha span {
    font-weight: 500;
  }
  input,
  select {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.45rem 0.6rem;
    font-size: 0.9rem;
    background: var(--seno-gray-100);
    font-family: inherit;
  }
  .botoes {
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }
  .botoes button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.6rem 1.2rem;
    font-weight: 600;
    cursor: pointer;
  }
  .botoes button:hover {
    background: var(--seno-blue-700);
  }
  .zona-excluir {
    display: flex;
    justify-content: flex-end;
  }
  .acao.vermelha {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
    border-radius: 8px;
    padding: 0.45rem 0.9rem;
    cursor: pointer;
    font-size: 0.86rem;
  }
  .acao.vermelha:hover {
    background: #fdecea;
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
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
