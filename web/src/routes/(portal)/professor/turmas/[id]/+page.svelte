<script lang="ts">
  // Painel de controle da turma: dados + navegação gerencial (alunos),
  // placeholders de seções futuras e ações de dono (renomear, encerrar,
  // excluir — com confirmação).
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const turma = $derived(data.turma);
  const encerrada = $derived(Boolean(turma?.encerrado_em));
  let titulo = $state("");
  let editandoTitulo = $state(false);
  let confirmouExcluir = $state(false);
</script>

<section class="painel">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if turma}
    {#if form?.erro}
      <p class="aviso erro" role="alert">{form.erro}</p>
    {/if}
    {#if form?.ok}
      <p class="aviso ok">{form.ok}</p>
    {/if}

    <div class="cabeca">
      <div>
        <h2 class="nomes">
          {turma.materia.codigo} — {turma.materia.nome}
        </h2>
        <span class="linha2">
          {turma.titulo || "Sem título"} · {turma.trimestre}T/{turma.ano} ·
          {turma.professor.first_name} {turma.professor.last_name}
        </span>
      </div>
      {#if encerrada}
        <span class="estado encerrada">Encerrada</span>
      {:else}
        <span class="estado ativa">Ativa</span>
      {/if}
    </div>

    <nav class="opcoes" aria-label="Seções da turma">
      <a class="opcao" href={`/professor/turmas/${turma.id}/alunos`}>
        <strong>Alunos da turma</strong>
        <span>Matrículas, convites e remoção de alunos</span>
      </a>
      <a class="opcao" href="#atribuicoes">
        <strong>Atividades ({data.atribuicoes.length})</strong>
        <span>Atribuições da turma e suas configurações</span>
      </a>
      <a class="opcao desativada" href={`/professor/turmas/${turma.id}`} aria-disabled="true">
        <strong>Analytics</strong>
        <span>Métricas por tarefa/aluno (em construção)</span>
      </a>
    </nav>

    <div class="gestao">
      {#if editandoTitulo}
        <form method="POST" action="?/renomear" class="renomear">
          <input name="titulo" bind:value={titulo} placeholder="T1, noturna..." />
          <button>Salvar</button>
          <button
            type="button"
            class="secundario"
            onclick={() => (editandoTitulo = false)}>
            Cancelar
          </button>
        </form>
      {:else}
        <button class="acao" onclick={() => { titulo = turma?.titulo ?? ""; editandoTitulo = true; }}>
          Renomear
        </button>
      {/if}
      {#if !encerrada}
        <form method="POST" action="?/encerrar">
          <button class="acao amarela">Encerrar turma</button>
        </form>
      {/if}
      {#if confirmouExcluir}
        <form method="POST" action="?/excluir" class="linha-excluir">
          <span>Excluir definitivamente? Matrículas encerradas somem.</span>
          <button class="acao vermelha">Sim, excluir</button>
          <button type="button" class="secundario" onclick={() => (confirmouExcluir = false)}>
            Cancelar
          </button>
        </form>
      {:else}
        <button class="acao vermelha" onclick={() => (confirmouExcluir = true)}>
          Excluir turma
        </button>
      {/if}
    </div>

    <section id="atribuicoes" class="atribuicoes">
      <div class="cabeca-atrib">
        <h3>Atividades atribuídas <span class="total">({data.atribuicoes.length})</span></h3>
      </div>

      {#if !encerrada && !turma.encerrado_em}
        <form method="POST" action="?/atribuir" class="atribuir">
          <select name="atividade_id" required>
            <option value="" disabled selected>Atribuir atividade do banco…</option>
            {#each data.atividades as a (a.id)}
              <option value={a.id}>{a.nome}</option>
            {/each}
          </select>
          <label class="mini">
            <span>Prazo</span>
            <input type="datetime-local" name="prazo" />
          </label>
          <label class="mini">
            <span>Duração (h)</span>
            <input type="number" name="duracao_horas" min="0.25" max="168" step="0.25" />
          </label>
          <label class="mini linha">
            <input type="checkbox" name="pode_atrasado" />
            <span>Atraso ok</span>
          </label>
          <label class="mini linha">
            <select name="autocomplete" class="mini-select">
              <option value="">Auto (padrão do §)</option>
              <option value="on">Autocomplete ligado</option>
              <option value="off">Desligado</option>
            </select>
          </label>
          <button class="acao">Atribuir</button>
        </form>
      {/if}

      <ul class="lista-atrib">
        {#each data.atribuicoes as a (a.id)}
          <li>
            <div class="identidade">
              <strong>{a.atividade_nome || "Atividade"}</strong>
              <span class="meta">
                {a.prazo
                  ? `Prazo ${new Date(a.prazo).toLocaleString("pt-BR")}`
                  : "Sem prazo"} ·
                {a.duracao_seg
                  ? `${Math.round(a.duracao_seg / 3600 * 10) / 10} h de duração`
                  : "sem duração"} ·
                autocomplete {a.autocomplete ? "ligado" : "desligado"} ·
                {a.pode_atrasado ? "atraso permitido" : "sem atraso"}
              </span>
            </div>
            <div class="acao-area">
              <form method="POST" action="?/editarConfig">
                <input type="hidden" name="atribuicao_id" value={a.id} />
                <button class="peq">Configurar</button>
              </form>
              <form method="POST" action="?/desligar">
                <input type="hidden" name="atribuicao_id" value={a.id} />
                <button class="peq vermelho">Desligar</button>
              </form>
            </div>
          </li>
        {:else}
          <li class="vazio">Nenhuma atividade atribuída ainda.</li>
        {/each}
      </ul>
    </section>
  {/if}
</section>

<style>
  .painel {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
  }
  .cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .nomes {
    margin: 0;
  }
  .linha2 {
    color: var(--seno-gray-500);
    font-size: 0.88rem;
  }
  .estado {
    font-size: 0.75rem;
    border-radius: 999px;
    padding: 0.2rem 0.6rem;
  }
  .estado.ativa {
    background: #e6f4ea;
    color: var(--seno-green);
  }
  .estado.encerrada {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .opcoes {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
    gap: 0.75rem;
  }
  .opcao {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 1rem 1.1rem;
    text-decoration: none;
    color: inherit;
  }
  .opcao strong {
    color: var(--seno-blue-900);
  }
  .opcao span {
    color: var(--seno-gray-500);
    font-size: 0.82rem;
  }
  .opcao:hover {
    border-color: var(--seno-blue-500);
  }
  .opcao.desativada {
    opacity: 0.55;
  }
  .gestao {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .gestao form {
    display: contents;
  }
  .acao {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    cursor: pointer;
  }
  .acao:hover {
    background: var(--seno-blue-700);
  }
  .acao.amarela {
    background: #b7791f;
  }
  .acao.amarela:hover {
    background: #975a16;
  }
  .acao.vermelha {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }
  .acao.vermelha:hover {
    background: #fdecea;
  }
  .secundario {
    background: none;
    border: 1px solid var(--seno-gray-300);
    color: var(--seno-gray-700);
    border-radius: 8px;
    padding: 0.45rem 0.7rem;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .renomear {
    display: flex;
    gap: 0.4rem;
    align-items: center;
  }
  .renomear input {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.45rem 0.6rem;
    font-size: 0.9rem;
  }
  .linha-excluir {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.85rem;
    color: var(--seno-gray-700);
    background: var(--seno-gray-100);
    border-radius: 8px;
    padding: 0.5rem 0.7rem;
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
  .atribuicoes {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
  }
  .cabeca-atrib h3 {
    margin: 0;
  }
  .total {
    color: var(--seno-gray-500);
    font-size: 0.82rem;
    font-weight: 400;
  }
  .atribuir {
    display: flex;
    align-items: end;
    gap: 0.55rem;
    flex-wrap: wrap;
    background: var(--seno-gray-100);
    border-radius: 10px;
    padding: 0.7rem 0.85rem;
  }
  .atribuir select,
  .atribuir input {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.4rem 0.55rem;
    font-size: 0.85rem;
    background: var(--seno-white);
  }
  .atribuir select:first-of-type {
    min-width: 200px;
  }
  .mini {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .mini span {
    font-size: 0.74rem;
    color: var(--seno-gray-700);
    font-weight: 600;
  }
  .mini.linha {
    flex-direction: row;
    gap: 0.3rem;
    align-items: center;
  }
  .lista-atrib {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .lista-atrib li {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 0.7rem 0.9rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.7rem;
    flex-wrap: wrap;
  }
  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.79rem;
  }
  .acao-area {
    display: flex;
    gap: 0.4rem;
  }
  .acao-area form {
    display: contents;
  }
  .peq {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.3rem 0.65rem;
    font-size: 0.8rem;
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
  .vazio {
    color: var(--seno-gray-500);
    text-align: center;
  }
</style>
