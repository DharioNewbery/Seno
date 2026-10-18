<script lang="ts">
  // Entregas da atribuição: quem entregou, em quanto e atrasadas —
  // cada entrega abre o painel de correção e permite ações do professor
  // (coletar rascunho vencido, excluir submissão, reexecutar testes).
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const atribuicao = $derived(data.atribuicao);
</script>

<section class="entregas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}

  <div class="cabeca">
    <div>
      <h2>Entregas — {atribuicao?.atividade_nome || "Atividade"}</h2>
      <span class="meta">
        Turma {atribuicao?.turma_titulo || atribuicao?.turma_id} ·
        ({data.entregas.length} registros)
      </span>
    </div>
    <a class="voltar" href={`/professor/atribuicoes/${data.atribuicao?.id ?? ""}`}>
      ← Configuração
    </a>
  </div>

  <ul class="lista">
    {#each data.entregas as e, i (i)}
      <li>
        <div class="identidade">
          <strong>{e.aluno}</strong>
          <span class="meta">
            {e.aluno_email} ·
            {#if e.comecou_em}
              com. {new Date(e.comecou_em).toLocaleString("pt-BR")}
            {/if}
            {#if e.estado === "rascunho"}
              · <span class="badge rascunho">Rascunho não entregue</span>
            {:else if e.entregue_em}
              → ent. {new Date(e.entregue_em).toLocaleString("pt-BR")}
            {/if}
            {#if e.atrasada}· <span style:color="#c0392b">atrasada</span>{/if}
            {#if e.do_monitor && e.nota_monitor != null}
              · nota (auto) {e.nota_monitor}
            {/if}
          </span>
        </div>

        <div class="acoes-linha">
          {#if e.estado === "rascunho"}
            {#if e.pode_coletar}
              <form method="POST" action="?/coletar">
                <input type="hidden" name="aluno_id" value={e.aluno_id} />
                <button class="botao coletar">Coletar rascunho</button>
              </form>
            {:else}
              <span class="meta">Em andamento</span>
            {/if}
          {:else if e.do_monitor}
            <span class="meta">
              Monitor
              {#if e.correcao_monitor}· {e.correcao_monitor}{/if}
            </span>
            <a class="abrir" href={`/professor/atribuicoes/${atribuicao?.id}/entregas/monitor/${e.aluno_id}`}>Ver entrega →</a>
            <form method="POST" action="?/reexecutar_monitor">
              <input type="hidden" name="aluno_id" value={e.aluno_id} />
              <button class="botao neutro" title="Reexecutar correção">Reexecutar</button>
            </form>
            <form
              method="POST"
              action="?/excluir"
              onsubmit={(ev) => {
                if (typeof window !== "undefined" && !window.confirm("Tem certeza que deseja excluir esta entrega? O aluno receberá uma nova tentativa pré-preenchida.")) {
                  ev.preventDefault();
                }
              }}
            >
              <input type="hidden" name="aluno_id" value={e.aluno_id} />
              <input type="hidden" name="do_monitor" value="true" />
              <button class="botao perigo" title="Excluir entrega">Excluir</button>
            </form>
          {:else}
            <a class="abrir" href={`/professor/submissoes/${e.id}`}>Corrigir →</a>
            <form
              method="POST"
              action="?/excluir"
              onsubmit={(ev) => {
                if (typeof window !== "undefined" && !window.confirm("Tem certeza que deseja excluir esta entrega? O aluno receberá uma nova tentativa pré-preenchida.")) {
                  ev.preventDefault();
                }
              }}
            >
              <input type="hidden" name="submissao_id" value={e.id} />
              <button class="botao perigo" title="Excluir entrega">Excluir</button>
            </form>
          {/if}
        </div>
      </li>
    {:else}
      <li class="vazio">Nenhum aluno iniciou ou entregou ainda.</li>
    {/each}
  </ul>
</section>


<style>
  .entregas {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    align-items: end;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .cabeca h2 {
    margin: 0 0 0.2rem;
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .lista {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  .lista li {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 0.8rem 1rem;
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
  .abrir {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: 8px;
    padding: 0.4rem 0.8rem;
    text-decoration: none;
    font-size: 0.86rem;
  }
  .abrir:hover {
    background: var(--seno-blue-700);
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
  }
  .aviso.ok {
    background: #e6f4ea;
    color: var(--seno-green);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .acoes-linha {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .acoes-linha form {
    display: contents;
  }
  .botao {
    border: none;
    border-radius: 8px;
    padding: 0.4rem 0.75rem;
    font-size: 0.84rem;
    cursor: pointer;
    font-weight: 500;
  }
  .botao.coletar {
    background: #d97706;
    color: var(--seno-white);
  }
  .botao.coletar:hover {
    background: #b45309;
  }
  .botao.perigo {
    background: #fdecea;
    color: var(--seno-red);
  }
  .botao.perigo:hover {
    background: #fad2cf;
  }
  .botao.neutro {
    background: var(--seno-gray-200);
    color: var(--seno-gray-800);
  }
  .botao.neutro:hover {
    background: var(--seno-gray-300);
  }
  .badge.rascunho {
    background: #fef3c7;
    color: #92400e;
    padding: 0.15rem 0.45rem;
    border-radius: 999px;
    font-size: 0.76rem;
    font-weight: 500;
  }
  .vazio {
    color: var(--seno-gray-500);
    justify-content: center !important;
  }
</style>
