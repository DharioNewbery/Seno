<script lang="ts">
  import { dataHora } from "#lib/formato";
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
            {e.linguagem} · {dataHora(e.criado_em)}
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
        .lista li {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
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
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .veredito {
    border-radius: var(--raio-pill);
    padding: 0.1rem 0.5rem;
    background: var(--seno-red-50);
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
  </style>
