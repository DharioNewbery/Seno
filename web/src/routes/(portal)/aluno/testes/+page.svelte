<script lang="ts">
  // Histórico de Execução de teste (últimos 7 dias, PROJETO §Execução
  // de teste); só o próprio aluno vê. Resultados ficam embutidos na
  // listagem (somente testes públicos).
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const execucoes = $derived(data.execucoes);
</script>

<section class="historico">
  <div class="cabeca">
    <h2>Execuções de teste <span class="total">({execucoes.length})</span></h2>
    <a class="voltar" href="/aluno/atividades">← Atividades</a>
  </div>

  <ul class="lista">
    {#each execucoes as e (e.id)}
      <li>
        <div class="identidade">
          <strong>{e.atividade_nome || "Atividade"} · tarefa #{e.tarefa_id}</strong>
          <span class="meta">
            {e.linguagem} · {new Date(e.criado_em).toLocaleString("pt-BR")}
          </span>
        </div>
        <ul class="resultados">
          {#each e.resultados ?? [] as res (res.teste_id)}
            <li class:ok={res.status === "accepted"}>
              <span class="veredito">{res.status ?? "—"}</span>
              <span class="mono">teste #{res.teste_id}</span>
              {#if res.tempo}<span class="mono">{res.tempo}s</span>{/if}
            </li>
          {:else}
            <li class="sem">Sem resultados registrados.</li>
          {/each}
        </ul>
      </li>
    {:else}
      <li class="vazio">
        Nenhuma execução nos últimos 7 dias (o "testar" fica na tela da
        atividade; só há resultados quando a tarefa tem testes públicos).
      </li>
    {/each}
  </ul>
</section>

<style>
  .historico {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    align-items: end;
    justify-content: space-between;
  }
  .cabeca h2 {
    margin: 0;
  }
  .total {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
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
    padding: 0.75rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.83rem;
  }
  .resultados {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .resultados li {
    font-size: 0.82rem;
    display: flex;
    gap: 0.45rem;
    align-items: center;
    flex-wrap: wrap;
  }
  .resultados li.ok .veredito {
    background: #e6f4ea;
    color: var(--seno-green);
  }
  .veredito {
    border-radius: 999px;
    padding: 0.1rem 0.5rem;
    background: #fdecea;
    color: var(--seno-red);
    font-size: 0.74rem;
  }
  .mono {
    font-family: monospace;
    font-size: 0.78rem;
  }
  .sem {
    color: var(--seno-gray-500);
  }
  .vazio {
    color: var(--seno-gray-500);
    text-align: center;
  }
</style>
