<script lang="ts">
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const metricas = $derived(data.metricas);
</script>

<section class="admin-dashboard">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <header class="cabecalho">
    <h1>Painel de Controle e Métricas Globais</h1>
    <span class="sub">Visão consolidada da infraestrutura, usuários e execuções</span>
  </header>

  {#if metricas}
    <div class="grade-destaques">
      <div class="cartao-metrica">
        <span class="rotulo">Turmas Ativas</span>
        <strong class="valor">{metricas.total_turmas_ativas}</strong>
      </div>
      <div class="cartao-metrica alerta" class:tem-falhas={metricas.falhas_ultimos_7_dias > 0}>
        <span class="rotulo">Falhas (Últimos 7 dias)</span>
        <strong class="valor">{metricas.falhas_ultimos_7_dias}</strong>
      </div>
    </div>

    <div class="grade-paineis">
      <section class="painel">
        <h2>Usuários por Cargo</h2>
        <ul class="lista-metricas">
          {#each Object.entries(metricas.usuarios_por_cargo || {}) as [cargo, qtd] (cargo)}
            <li>
              <span class="tag-chave">{cargo}</span>
              <strong class="tag-valor">{qtd}</strong>
            </li>
          {:else}
            <li class="vazio">Sem dados</li>
          {/each}
        </ul>
      </section>

      <section class="painel">
        <h2>Usuários por Status</h2>
        <ul class="lista-metricas">
          {#each Object.entries(metricas.usuarios_por_status || {}) as [status, qtd] (status)}
            <li>
              <span class="tag-chave">{status}</span>
              <strong class="tag-valor">{qtd}</strong>
            </li>
          {:else}
            <li class="vazio">Sem dados</li>
          {/each}
        </ul>
      </section>

      <section class="painel">
        <h2>Correções por Status</h2>
        <ul class="lista-metricas">
          {#each Object.entries(metricas.submissoes_por_status || {}) as [status, qtd] (status)}
            <li>
              <span class="tag-chave">{status}</span>
              <strong class="tag-valor">{qtd}</strong>
            </li>
          {:else}
            <li class="vazio">Sem dados</li>
          {/each}
        </ul>
      </section>

      <section class="painel">
        <h2>Fila de Execuções (Judge0 / Jobs)</h2>
        <ul class="lista-metricas">
          {#each Object.entries(metricas.fila_judge0 || {}) as [status, qtd] (status)}
            <li>
              <span class="tag-chave">{status}</span>
              <strong class="tag-valor">{qtd}</strong>
            </li>
          {:else}
            <li class="vazio">Fila vazia (sem jobs pendentes)</li>
          {/each}
        </ul>
      </section>
    </div>
  {/if}
</section>

<style>
  .admin-dashboard {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .cabecalho h1 {
    margin: 0;
    font-size: 1.4rem;
  }
  .sub {
    font-size: 0.88rem;
    color: var(--seno-gray-500, #666);
  }
  .grade-destaques {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1rem;
  }
  .cartao-metrica {
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 12px;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .cartao-metrica .rotulo {
    font-size: 0.8rem;
    color: var(--seno-gray-500, #666);
    text-transform: uppercase;
    font-weight: 600;
  }
  .cartao-metrica .valor {
    font-size: 2rem;
    color: var(--seno-primary, #0056b3);
  }
  .cartao-metrica.alerta.tem-falhas .valor {
    color: var(--seno-red, #c00);
  }
  .grade-paineis {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 1.25rem;
  }
  .painel {
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 12px;
    padding: 1.2rem;
  }
  .painel h2 {
    margin: 0 0 1rem;
    font-size: 1.05rem;
    border-bottom: 1px solid #eee;
    padding-bottom: 0.5rem;
  }
  .lista-metricas {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .lista-metricas li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.4rem 0.6rem;
    border-radius: 6px;
    background: #f8f9fa;
    font-size: 0.9rem;
  }
  .tag-chave {
    text-transform: capitalize;
    color: var(--seno-gray-700, #444);
  }
  .tag-valor {
    font-size: 0.95rem;
    color: var(--seno-primary, #0056b3);
  }
  .vazio {
    padding: 1rem;
    text-align: center;
    color: var(--seno-gray-500, #777);
    font-size: 0.85rem;
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red, #c00);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    margin: 0;
  }
</style>
