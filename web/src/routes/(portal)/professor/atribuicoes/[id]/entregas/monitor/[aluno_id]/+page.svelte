<script lang="ts">
  import { dataHora } from "#lib/formato";
  import type { PageProps } from "./$types";
  import EditorCodigo from "#lib/components/EditorCodigo.svelte";

  let { data, form }: PageProps = $props();

  const atribuicao = $derived(data.atribuicao);
  const det = $derived(data.detalhes);
  const sub = $derived(det?.submissao);
  const cor = $derived(det?.correcao);
  const atv = $derived(det?.atividade);

  function resultadoDaTarefa(ordem: number) {
    if (!cor?.resultados?.tarefas) return null;
    return cor.resultados.tarefas.find((t) => t.ordem === ordem) ?? null;
  }
</script>

<section class="detalhes-submissao">
  <div class="navegacao">
    {#if atribuicao}
      <a class="voltar" href={`/professor/atribuicoes/${atribuicao.id}/entregas`}>
        ← Voltar para Entregas
      </a>
    {/if}
  </div>

  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if det && sub && atv}
    {#if form?.erro}
      <p class="aviso erro" role="alert">{form.erro}</p>
    {/if}
    {#if form?.ok}
      <p class="aviso ok">{form.ok}</p>
    {/if}

    <header class="cabecalho">
      <div>
        <h2>Entrega do Aluno #{sub.aluno_id}</h2>
        <p class="meta">
          <span>Atividade: <strong>{atv.nome}</strong></span>
          <span>·</span>
          <span>Entregue em: {dataHora(sub.entregue_em)}</span>
          {#if sub.atrasada}
            <span class="badge perigo">Atrasada</span>
          {/if}
        </p>
      </div>
      <div class="acoes">
        <div class="status-bloco">
          <span class="status-rotulo">Correção:</span>
          <span class="badge {cor?.status === 'concluida' ? 'sucesso' : cor?.status === 'falhou' ? 'perigo' : 'neutro'}">
            {cor?.status ?? "pendente"}
          </span>
          {#if cor?.nota_auto !== undefined && cor?.nota_auto !== null}
            <span class="nota-auto">Nota: <strong>{cor.nota_auto}</strong></span>
          {/if}
        </div>
        <div class="botoes-topo">
          <form method="POST" action="?/reexecutar">
            <button class="botao neutro" type="submit">Reexecutar Correção</button>
          </form>
          <form
            method="POST"
            action="?/excluir"
            onsubmit={(ev) => {
              if (typeof window !== "undefined" && !window.confirm("Tem certeza que deseja excluir esta entrega? O aluno receberá uma nova tentativa pré-preenchida no Monitor.")) {
                ev.preventDefault();
              }
            }}
          >
            <button class="botao perigo" type="submit">Excluir Entrega</button>
          </form>
        </div>
      </div>
    </header>

    <div class="tarefas-lista">
      {#each atv.tarefas as t (t.ordem)}
        {@const res = resultadoDaTarefa(t.ordem)}
        {@const cod = sub.respostas[String(t.ordem)] ?? ""}
        <article class="tarefa-cartao">
          <header class="tarefa-cabecalho">
            <div>
              <h3>Questão {t.ordem}: {t.nome}</h3>
              <span class="tarefa-meta">
                Linguagem: <code>{t.linguagem}</code> · Valor: {t.valor_pts} pts
              </span>
            </div>
            {#if res}
              <div class="tarefa-resultado-resumo">
                <span class="badge {res.veredito === 'accepted' ? 'sucesso' : 'perigo'}">
                  {res.veredito}
                </span>
                <span class="nota-tarefa">Nota: {res.nota_auto} / {t.valor_pts}</span>
              </div>
            {/if}
          </header>

          {#if t.enunciado}
            <div class="bloco-enunciado">
              <span class="subrotulo">Enunciado:</span>
              <p class="texto-enunciado">{t.enunciado}</p>
            </div>
          {/if}

          <div class="bloco-codigo">
            <span class="subrotulo">Código submetido pelo aluno:</span>
            {#if cod.trim().length > 0}
              <EditorCodigo valor={cod} linguagem={t.linguagem} somenteLeitura={true} />
            {:else}
              <p class="codigo-vazio">Nenhum código enviado para esta tarefa.</p>
            {/if}
          </div>

          {#if res && res.testes && res.testes.length > 0}
            <div class="bloco-testes">
              <span class="subrotulo">Resultados dos casos de teste:</span>
              <ul class="lista-testes">
                {#each res.testes as teste (teste.ordem)}
                  <li class="item-teste {teste.status === 'accepted' ? 'ok' : 'falha'}">
                    <div class="teste-info">
                      <span class="badge {teste.status === 'accepted' ? 'sucesso' : 'perigo'}">
                        {teste.status}
                      </span>
                      <span class="teste-nome">Teste #{teste.ordem}</span>
                      {#if teste.privado}
                        <span class="teste-detalhe">(privado)</span>
                      {:else}
                        <span class="teste-detalhe">(público)</span>
                      {/if}
                      {#if teste.tempo !== undefined}
                        <span class="teste-detalhe">{teste.tempo}s</span>
                      {/if}
                      {#if teste.memoria !== undefined}
                        <span class="teste-detalhe">{teste.memoria}KB</span>
                      {/if}
                    </div>
                    {#if teste.stderr}
                      <details class="detalhe-saida">
                        <summary>stderr</summary>
                        <pre>{teste.stderr}</pre>
                      </details>
                    {/if}
                    {#if teste.compile_output}
                      <details class="detalhe-saida">
                        <summary>saída de compilação</summary>
                        <pre>{teste.compile_output}</pre>
                      </details>
                    {/if}
                    {#if teste.stdout}
                      <details class="detalhe-saida">
                        <summary>stdout</summary>
                        <pre>{teste.stdout}</pre>
                      </details>
                    {/if}
                  </li>
                {/each}
              </ul>
            </div>
          {:else if cor}
            <p class="sem-testes">Nenhum resultado de teste disponível para esta questão.</p>
          {/if}
        </article>
      {/each}
    </div>
  {/if}
</section>

<style>
  .detalhes-submissao {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .navegacao {
    margin-bottom: 0.25rem;
  }
      .cabecalho {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.25rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .cabecalho h2 {
    margin: 0 0 0.4rem;
    font-size: 1.4rem;
  }
    .acoes {
    display: flex;
    align-items: center;
    gap: 1.2rem;
    flex-wrap: wrap;
  }
  .status-bloco {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .status-rotulo {
    font-size: 0.85rem;
    color: var(--seno-gray-600);
  }
  .nota-auto {
    font-size: 0.95rem;
    margin-left: 0.4rem;
  }
  .botoes-topo {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .botao {
    border: none;
    border-radius: var(--raio-sm);
    padding: 0.45rem 0.85rem;
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
  }
  .botao.neutro {
    background: var(--seno-gray-200);
    color: var(--seno-gray-800);
  }
  .botao.neutro:hover {
    background: var(--seno-gray-300);
  }
  .botao.perigo {
    background: var(--seno-red-50);
    color: var(--seno-red);
    border: 1px solid var(--seno-red-200);
  }
  .botao.perigo:hover {
    background: var(--seno-red-50);
  }
  .badge {
    display: inline-block;
    padding: 0.2rem 0.55rem;
    border-radius: var(--raio-sm);
    font-size: 0.78rem;
    font-weight: 600;
    text-transform: uppercase;
  }
  .badge.sucesso {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .badge.perigo {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .badge.neutro {
    background: var(--seno-gray-200);
    color: var(--seno-gray-700);
  }
  .tarefas-lista {
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
  }
  .tarefa-cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .tarefa-cabecalho {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
    border-bottom: 1px solid var(--seno-gray-200);
    padding-bottom: 0.75rem;
  }
  .tarefa-cabecalho h3 {
    margin: 0 0 0.25rem;
    font-size: 1.15rem;
  }
  .tarefa-meta {
    font-size: 0.85rem;
    color: var(--seno-gray-600);
  }
  .tarefa-resultado-resumo {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .nota-tarefa {
    font-weight: 600;
    font-size: 0.9rem;
    color: var(--seno-gray-800);
  }
  .bloco-enunciado {
    background: var(--seno-gray-100);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-sm);
    padding: 0.75rem 1rem;
  }
  .subrotulo {
    display: block;
    font-weight: 600;
    font-size: 0.85rem;
    color: var(--seno-gray-700);
    margin-bottom: 0.35rem;
  }
  .texto-enunciado {
    margin: 0;
    white-space: pre-wrap;
    font-size: 0.9rem;
    color: var(--seno-gray-800);
    line-height: 1.5;
  }
  .bloco-codigo {
    display: flex;
    flex-direction: column;
  }
  .codigo-vazio {
    font-style: italic;
    color: var(--seno-gray-500);
    font-size: 0.9rem;
    margin: 0.4rem 0;
  }
  .bloco-testes {
    margin-top: 0.25rem;
  }
  .lista-testes {
    list-style: none;
    padding: 0;
    margin: 0.4rem 0 0;
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .item-teste {
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-sm);
    padding: 0.55rem 0.85rem;
    background: var(--seno-gray-50);
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .item-teste.ok {
    border-left: 4px solid var(--seno-green);
  }
  .item-teste.falha {
    border-left: 4px solid var(--seno-red);
  }
  .teste-info {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    font-size: 0.85rem;
  }
  .teste-nome {
    font-weight: 500;
  }
  .teste-detalhe {
    color: var(--seno-gray-600);
    font-size: 0.8rem;
  }
  .detalhe-saida {
    margin-top: 0.25rem;
    font-size: 0.82rem;
  }
  .detalhe-saida summary {
    cursor: pointer;
    color: var(--seno-blue-600);
  }
  .detalhe-saida pre {
    background: #1e1e1e;
    color: var(--seno-gray-50);
    padding: 0.6rem;
    border-radius: var(--raio-sm);
    overflow-x: auto;
    font-family: monospace;
    font-size: 0.8rem;
    margin: 0.35rem 0 0;
  }
  .sem-testes {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-style: italic;
    margin: 0.2rem 0;
  }
    </style>
