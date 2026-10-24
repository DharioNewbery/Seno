<script lang="ts">
  import { dataHora } from "#lib/formato";
  // Correção publicada da entrega do aluno: nota total, notas e
  // feedback por tarefa e resultados dos testes (§Visibilidade).
  // Além disso, a própria entrega: metadados e código enviado por tarefa.
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const visao = $derived(data.visao);
  const entrega = $derived(data.entrega);
  const cor = $derived(visao?.correcao ?? null);
  const nomesTarefa = $derived(
    cor ? new Map(cor.tarefas.map((t) => [t.tarefa_id, t.tarefa_nome])) : null,
  );
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
    {#if entrega}
      <div class="cabeca">
        <h2>Sua entrega</h2>
        <span class="meta">
          Entregue em {dataHora(entrega.entregue_em)}
          {#if entrega.atrasada}<span class="pilula atrasada">atrasada</span>{/if}
        </span>
      </div>
      {#if entrega.observacao}
        <div class="cartao">
          <span class="subrotulo">Observação</span>
          <p class="texto">{entrega.observacao}</p>
        </div>
      {/if}
    {/if}

    {#if !visao.publicada || !cor}
      <div class="cartao pendente-box">
        <h3>Atividade entregue com sucesso!</h3>
        <p class="texto">
          Sua submissão foi registrada e está aguardando a finalização da correção pelo professor.
          O código das suas respostas e as notas serão liberados assim que o professor publicar as notas da turma.
        </p>
      </div>
    {:else}
      <div class="cabeca">
        <h2>Correção publicada</h2>
        <span class="meta">
          Publicada em {dataHora(visao.publicada_em ?? "")}
        </span>
      </div>

      {#if cor.feedback}
        <div class="cartao">
          <span class="subrotulo">Feedback geral do professor</span>
          <p class="texto">{cor.feedback}</p>
        </div>
      {/if}

      {#each cor.tarefas as t (t.id)}
        <div class="cartao">
          <div class="linha-tarefa">
            <strong>{t.tarefa_nome}</strong>
            <span class="nota">Nota: <strong>{t.nota_final ?? t.nota_auto ?? "—"}</strong> de {t.valor_pts} pts</span>
          </div>

          {#if entrega?.snapshot?.tasks}
            {@const taskCode = entrega.snapshot.tasks.find((tk) => tk.id === t.tarefa_id)?.text}
            {#if taskCode}
              <span class="subrotulo">Código enviado</span>
              <pre class="codigo">{taskCode}</pre>
            {/if}
          {/if}

          {#if t.feedback}
            <span class="subrotulo">Comentário do professor</span>
            <p class="texto">{t.feedback}</p>
          {/if}

          {#if t.resultados.length > 0}
            <span class="subrotulo">Resultados dos testes</span>
            <ul class="resultados">
              {#each t.resultados as res (res.teste_id)}
                <li class:ok={res.status === "accepted"}>
                  <div class="res-topo">
                    <span class="veredito">{res.status ?? "—"}</span>
                    <span>teste #{res.teste_id} ({res.publico ? "público" : "privado"})</span>
                    {#if res.tempo}<span>· tempo: {res.tempo}s</span>{/if}
                    {#if res.memoria}<span>· memória: {res.memoria}KB</span>{/if}
                  </div>
                  {#if res.stdout}
                    <details><summary>stdout</summary><pre>{res.stdout}</pre></details>
                  {/if}
                  {#if res.stderr}
                    <details><summary>erro (stderr)</summary><pre>{res.stderr}</pre></details>
                  {/if}
                  {#if res.compile_output}
                    <details><summary>compilação</summary><pre>{res.compile_output}</pre></details>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/each}

      <div class="cartao total-box">
        <p class="nota">Nota final da atividade: <strong>{notaTotal}</strong> pontos.</p>
      </div>
    {/if}
    <a class="voltar" href="/aluno/atividades">← Atividades</a>
  {/if}
</section>

<style>
  .correcao {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    justify-content: space-between;
    align-items: end;
  }
  .cabeca h2 {
    margin: 0;
  }
  .pilula {
    border-radius: var(--raio-pill);
    padding: 0.1rem 0.5rem;
    font-size: 0.74rem;
  }
  .pilula.atrasada {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .codigo {
    margin: 0;
    white-space: pre-wrap;
    font-size: 0.82rem;
    background: var(--seno-gray-100);
    border-radius: var(--raio-sm);
    padding: 0.5rem 0.7rem;
    max-height: 20rem;
    overflow: auto;
  }
    .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
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
  .resultados pre {
    max-width: 100%;
    white-space: pre-wrap;
    font-size: 0.78rem;
    background: var(--seno-gray-100);
    border-radius: var(--raio-sm);
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
  .pendente-box {
    background: var(--seno-blue-50);
    border-color: var(--seno-blue-200);
  }
  .pendente-box h3 {
    margin: 0 0 0.4rem;
    color: var(--seno-blue-900);
    font-size: 1.05rem;
  }
  .res-topo {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .total-box {
    background: var(--seno-green-50);
    border-color: var(--seno-green);
  }
  .total-box .nota {
    font-size: 1.05rem;
    color: var(--seno-green);
    margin: 0;
  }
</style>
