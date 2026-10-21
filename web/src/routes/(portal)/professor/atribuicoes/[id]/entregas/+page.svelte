<script lang="ts">
  import { dataHora } from "#lib/formato";
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
    <form method="POST" action="?/publicar_tudo">
      <button class="botao publicar" title="Publica de uma vez as correções já confirmadas">Publicar confirmadas</button>
    </form>
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
              com. {dataHora(e.comecou_em)}
            {/if}
            {#if e.estado === "rascunho"}
              · <span class="badge rascunho">Rascunho não entregue</span>
            {:else if e.entregue_em}
              → ent. {dataHora(e.entregue_em)}
            {/if}
            {#if e.atrasada}· <span style:color="var(--seno-red)">atrasada</span>{/if}
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
          .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
        .botao.publicar {
    background: var(--seno-green);
    color: var(--seno-white);
  }
  .botao.publicar:hover {
    background: var(--seno-green);
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
    border-radius: var(--raio-md);
    padding: 0.4rem 0.75rem;
    font-size: 0.84rem;
    cursor: pointer;
    font-weight: 500;
  }
  .botao.coletar {
    background: var(--seno-amber-500);
    color: var(--seno-white);
  }
  .botao.coletar:hover {
    background: var(--seno-amber);
  }
  .botao.perigo {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .botao.perigo:hover {
    background: var(--seno-red-200);
  }
  .botao.neutro {
    background: var(--seno-gray-200);
    color: var(--seno-gray-800);
  }
  .botao.neutro:hover {
    background: var(--seno-gray-300);
  }
  .badge.rascunho {
    background: var(--seno-amber-50);
    color: #92400e;
    padding: 0.15rem 0.45rem;
    border-radius: var(--raio-pill);
    font-size: 0.76rem;
    font-weight: 500;
  }
  </style>
