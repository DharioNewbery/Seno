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
    <div class="cabeca-acoes">
      <form method="POST" action="?/publicar_tudo">
        <button class="botao publicar" title="Publica de uma vez as correções validadas para toda a turma">
          Publicar notas da turma
        </button>
      </form>
      <a class="voltar" href={`/professor/atribuicoes/${data.atribuicao?.id ?? ""}`}>
        ← Configuração
      </a>
    </div>
  </div>

  <ul class="lista">
    {#each data.entregas as e, i (i)}
      <li>
        <div class="identidade">
          <div class="aluno-linha">
            <strong>{e.aluno}</strong>
            {#if e.publicada}
              <span class="badge publicada">Publicada</span>
            {:else if e.confirmada}
              <span class="badge confirmada">Validada</span>
            {/if}
            {#if e.correcao_status}
              <span class="badge status-{e.correcao_status}">{e.correcao_status}</span>
            {/if}
          </div>
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
            {#if e.nota_final !== undefined && e.nota_final !== null}
              · <strong class="nota-destaque">Nota: {e.nota_final}</strong>
            {:else if e.nota_auto !== undefined && e.nota_auto !== null}
              · <span>Nota auto: {e.nota_auto}</span>
            {:else if e.nota_monitor !== undefined && e.nota_monitor !== null}
              · <span>Nota auto: {e.nota_monitor}</span>
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
          {:else if e.id}
            <a class="abrir" href={`/professor/submissoes/${e.id}`}>
              {e.publicada ? "Ver correção →" : e.confirmada ? "Editar validação →" : "Corrigir / Validar →"}
            </a>
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
  .cabeca-acoes {
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }
  .aluno-linha {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .badge {
    border-radius: var(--raio-pill);
    padding: 0.12rem 0.5rem;
    font-size: 0.74rem;
    font-weight: 600;
  }
  .badge.publicada {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .badge.confirmada {
    background: var(--seno-blue-50);
    color: var(--seno-blue-700);
  }
  .badge.status-concluida {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .badge.status-executando, .badge.status-pendente {
    background: var(--seno-amber-50);
    color: var(--seno-amber-700);
  }
  .badge.status-falhou {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .nota-destaque {
    color: var(--seno-blue-900);
  }
  </style>
