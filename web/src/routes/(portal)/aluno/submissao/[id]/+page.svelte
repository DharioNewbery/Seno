<script lang="ts">
  // Correção publicada da entrega do aluno: nota total, notas e
  // feedback por tarefa e resultados dos testes (§Visibilidade).
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const visao = $derived(data.visao);
  const cor = $derived(visao?.correcao ?? null);
  const notaTotal = $derived(
    visao?.publicada && cor
      ? cor.tarefas.reduce(
          (soma, t) => soma + (t.nota_final ?? t.nota_auto ?? 0),
          0,
        )
      : null,
  );
</script>

<section class="correcao">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if visao}
    {#if !visao.publicada || !cor}
      <p class="aviso">
        Sua correção ainda não foi publicada. Quando o professor publicar as
        notas, elas aparecem aqui.
      </p>
    {:else}
      <div class="cabeca">
        <h2>Correção publicada</h2>
        <span class="meta">
          {new Date(visao.publicada_em ?? "").toLocaleString("pt-BR")}
        </span>
      </div>

      {#if cor.feedback}
        <div class="cartao">
          <span class="subrotulo">Feedback geral</span>
          <p class="texto">{cor.feedback}</p>
        </div>
      {/if}

      {#each cor.tarefas as t (t.id)}
        <div class="cartao">
          <div class="linha-tarefa">
            <strong>{t.tarefa_nome}</strong>
            <span class="nota">Nota: {t.nota_final ?? t.nota_auto ?? "—"} de {t.valor_pts} pts</span>
          </div>
          {#if t.feedback}
            <span class="subrotulo">Comentário do professor</span>
            <p class="texto">{t.feedback}</p>
          {/if}
          {#if t.resultados.length > 0}
            <ul class="resultados">
              {#each t.resultados as res (res.teste_id)}
                <li class:ok={res.status === "accepted"}>
                  <span class="veredito">{res.status ?? "—"}</span>
                  <span>teste #{res.teste_id} ({res.publico ? "público" : "privado"})</span>
                  {#if res.stderr}<details><summary>erro</summary><pre>{res.stderr}</pre></details>{/if}
                  {#if res.compile_output}
                    <details><summary>compilação</summary><pre>{res.compile_output}</pre></details>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/each}

      <p class="nota">Nota final: <strong>{notaTotal}</strong> pontos.</p>
    {/if}
    <a class="voltar" href="/aluno/atividades">← Atividades</a>
  {/if}
</section>

<style>
  .correcao {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    max-width: 820px;
  }
  .cabeca {
    display: flex;
    justify-content: space-between;
    align-items: end;
  }
  .cabeca h2 {
    margin: 0;
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    padding: 1.1rem 1.3rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .linha-tarefa {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.6rem;
  }
  .nota {
    color: var(--seno-blue-900);
    font-size: 0.86rem;
  }
  .subrotulo {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--seno-gray-500);
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }
  .texto {
    margin: 0;
    white-space: pre-wrap;
    font-size: 0.92rem;
    color: var(--seno-gray-800);
  }
  .resultados {
    list-style: none;
    padding: 0;
    margin: 0.3rem 0 0;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .resultados li {
    font-size: 0.82rem;
    display: flex;
    gap: 0.5rem;
    align-items: center;
    flex-wrap: wrap;
    color: var(--seno-gray-700);
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
  .resultados pre {
    max-width: 500px;
    white-space: pre-wrap;
    font-size: 0.78rem;
    background: var(--seno-gray-100);
    border-radius: 6px;
    padding: 0.35rem 0.5rem;
  }
  details summary {
    cursor: pointer;
    font-size: 0.76rem;
  }
  .nota {
    color: var(--seno-gray-700);
    font-size: 0.92rem;
  }
  .aviso {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
  }
</style>
