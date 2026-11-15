<script lang="ts">
  import { dataHora, formatarBytes } from "#lib/formato";
  import {
    calcularTotaisCorrecao,
    extrairCodigoRespostaAluno,
  } from "#lib/correcao-aluno";
  import { ajustarIndiceTeste, navegarIndiceTeste } from "#lib/tarefas-testes";
  import EditorCodigo from "#lib/components/EditorCodigo.svelte";
  import type {
    CorrecaoTarefa,
    SubmissaoAluno,
    VisaoCorrecaoAluno,
  } from "#lib/server/ensino";

  interface Props {
    visao: VisaoCorrecaoAluno;
    entrega?: SubmissaoAluno | null;
    mostrarCabecalhoEntrega?: boolean;
  }

  let { visao, entrega = null, mostrarCabecalhoEntrega = false }: Props = $props();

  const cor = $derived(visao?.correcao ?? null);
  const totais = $derived(
    visao?.publicada && cor ? calcularTotaisCorrecao(cor.tarefas) : null,
  );

  // Índice ativo do visor unitário de resultados de teste por tarefa (id da correcao_tarefa -> idx)
  let indicesResultado = $state<Record<number, number>>({});

  function indiceAtualResultado(ctId: number, total: number): number {
    return ajustarIndiceTeste(indicesResultado[ctId] ?? 0, total);
  }

  function navegarResultado(ctId: number, delta: -1 | 1, total: number) {
    const atual = indiceAtualResultado(ctId, total);
    indicesResultado[ctId] = navegarIndiceTeste(atual, delta, total);
  }

  function contarAprovados(t: CorrecaoTarefa): number {
    return (t.resultados ?? []).filter((r) => r.status === "accepted").length;
  }

  function classeNotaTarefa(obtida: number, maximo: number): string {
    if (maximo <= 0) return "neutra";
    if (obtida >= maximo) return "plena";
    if (obtida > 0) return "parcial";
    return "zero";
  }
</script>

<div class="visao-submissao-aluno">
  {#if mostrarCabecalhoEntrega && entrega}
    <div class="cabeca-entrega">
      <div>
        <h2>{visao.atividade_nome || "Sua entrega"}</h2>
        <span class="meta">
          Entregue em {dataHora(entrega.entregue_em)}
          {#if entrega.atrasada}
            <span class="pilula atrasada">atrasada</span>
          {/if}
        </span>
      </div>
      {#if visao.atribuicao_id}
        <a class="link-atividade" href={`/aluno/atividades/${visao.atribuicao_id}`}>
          Ir para a página da atividade →
        </a>
      {/if}
    </div>
  {/if}

  {#if entrega?.observacao}
    <div class="cartao">
      <span class="subrotulo">Observação enviada na entrega</span>
      <p class="texto">{entrega.observacao}</p>
    </div>
  {/if}

  {#if !visao.publicada || !cor}
    <div class="cartao pendente-box">
      <h3>
        {visao.confirmada
          ? "Atividade corrigida — aguardando publicação"
          : "Atividade entregue com sucesso!"}
      </h3>
      <p class="texto">
        {#if visao.confirmada}
          O professor já validou sua entrega. Suas respostas detalhadas, notas por questão e resultados dos testes ficarão visíveis assim que as notas da turma forem publicadas.
        {:else}
          Sua submissão foi registrada e está aguardando a finalização da correção e publicação pelo professor. Assim que publicada, você poderá consultar aqui suas respostas e notas de cada questão.
        {/if}
      </p>
    </div>
  {:else}
    <div class="cartao resumo-publicada">
      <div class="resumo-topo">
        <div class="resumo-info">
          <span class="badge-publicada">✓ Correção publicada</span>
          <span class="meta">
            Publicada em {dataHora(visao.publicada_em ?? "")}
          </span>
        </div>

        {#if totais}
          <div class="placar-nota">
            <span class="placar-rotulo">Nota Final</span>
            <strong class="placar-valor">
              {totais.notaObtida}
              <span class="placar-total">/ {totais.valorTotal} pts</span>
            </strong>
          </div>
        {/if}
      </div>

      {#if cor.feedback}
        <div class="feedback-geral">
          <span class="subrotulo">Feedback geral do professor</span>
          <p class="texto">{cor.feedback}</p>
        </div>
      {/if}
    </div>

    <div class="lista-tarefas-corrigidas">
      {#each cor.tarefas ?? [] as t, idx (t.id)}
        {@const notaTarefa = t.nota_final ?? t.nota_auto ?? 0}
        {@const codigoEnviado = extrairCodigoRespostaAluno(t, entrega)}
        {@const isArq = t.tarefa_tipo === "arquivo_submissao" || Boolean(t.arquivo)}
        {@const resultados = t.resultados ?? []}
        {@const idxRes = indiceAtualResultado(t.id, resultados.length)}
        {@const resAtual = resultados[idxRes]}

        <article class="cartao cartao-questao">
          <header class="questao-topo">
            <div class="questao-identidade">
              <h3>Questão {t.ordem ?? idx + 1}: {t.tarefa_nome}</h3>
              <div class="questao-tags">
                {#if isArq}
                  <span class="tag-tipo arquivo">Submissão de arquivo</span>
                {:else}
                  <span class="tag-tipo prog">
                    Programação{#if t.linguagem} · {t.linguagem}{/if}
                  </span>
                {/if}
              </div>
            </div>

            <div class="nota-questao-badge {classeNotaTarefa(notaTarefa, t.valor_pts)}">
              <span>Nota:</span>
              <strong>{notaTarefa} / {t.valor_pts} pts</strong>
            </div>
          </header>

          {#if t.enunciado}
            <details class="detalhe-enunciado">
              <summary>Ver enunciado da questão</summary>
              <p class="texto-enunciado">{t.enunciado}</p>
            </details>
          {/if}

          <div class="bloco-resposta">
            <span class="subrotulo">
              {isArq ? "Sua resposta (arquivo enviado)" : "Sua resposta (código enviado)"}
            </span>

            {#if isArq}
              {#if t.arquivo}
                <div class="arquivo-box">
                  <div class="arquivo-meta">
                    <strong>{t.arquivo.nome_arquivo}</strong>
                    <span class="meta">
                      {formatarBytes(t.arquivo.tamanho_bytes)} · {t.arquivo.mime_type}
                    </span>
                  </div>
                  <a
                    class="botao-download"
                    href={`/api/submissoes/${cor.submissao_id}/tarefas/${t.tarefa_id}/arquivo`}
                    download
                  >
                    Baixar meu arquivo ↓
                  </a>
                </div>
              {:else}
                <p class="resposta-vazia">Nenhum arquivo foi anexado nesta questão.</p>
              {/if}
            {:else if codigoEnviado.trim().length > 0}
              <EditorCodigo
                valor={codigoEnviado}
                linguagem={t.linguagem || "python"}
                somenteLeitura={true}
              />
            {:else}
              <p class="resposta-vazia">Nenhum código foi enviado nesta questão.</p>
            {/if}
          </div>

          {#if t.feedback}
            <div class="feedback-tarefa">
              <span class="subrotulo">Comentário do professor</span>
              <p class="texto">{t.feedback}</p>
            </div>
          {/if}

          {#if !isArq && resultados.length > 0 && resAtual}
            <div class="bloco-resultados">
              <div class="topo-resultados">
                <div class="titulo-resultados">
                  <span class="subrotulo">Resultados dos casos de teste</span>
                  <span class="pill-aprovados">
                    {contarAprovados(t)} / {resultados.length} aprovados
                  </span>
                </div>

                <div class="nav-setas">
                  <button
                    type="button"
                    class="btn-seta"
                    onclick={() => navegarResultado(t.id, -1, resultados.length)}
                    disabled={idxRes <= 0}
                    aria-label="Teste anterior"
                    title="Teste anterior"
                  >
                    ←
                  </button>
                  <span class="indicador-teste">
                    Teste <strong>{idxRes + 1}</strong> de <strong>{resultados.length}</strong>
                  </span>
                  <button
                    type="button"
                    class="btn-seta"
                    onclick={() => navegarResultado(t.id, 1, resultados.length)}
                    disabled={idxRes >= resultados.length - 1}
                    aria-label="Próximo teste"
                    title="Próximo teste"
                  >
                    →
                  </button>
                </div>
              </div>

              <div class="visor-resultado" class:ok={resAtual.status === "accepted"}>
                <div class="res-meta-barra">
                  <div class="res-badges">
                    <span class="veredito" class:ok={resAtual.status === "accepted"}>
                      {resAtual.status ?? "—"}
                    </span>
                    <span class="badge-vis" class:publico={resAtual.publico}>
                      {resAtual.publico ? "Público" : "Privado"}
                    </span>
                  </div>
                  <div class="res-metricas">
                    {#if resAtual.tempo}
                      <span>Tempo: <code>{resAtual.tempo}s</code></span>
                    {/if}
                    {#if resAtual.memoria}
                      <span>Memória: <code>{resAtual.memoria}KB</code></span>
                    {/if}
                  </div>
                </div>

                <div class="grade-io-resultado">
                  {#if resAtual.stdin !== undefined}
                    <div class="col-io">
                      <span class="rotulo-io">Entrada (stdin)</span>
                      <pre class="bloco-io">{resAtual.stdin || "(vazio)"}</pre>
                    </div>
                  {/if}
                  {#if resAtual.stdout_esperado !== undefined}
                    <div class="col-io">
                      <span class="rotulo-io">Saída esperada</span>
                      <pre class="bloco-io">{resAtual.stdout_esperado || "(vazio)"}</pre>
                    </div>
                  {/if}
                  <div class="col-io">
                    <span class="rotulo-io">Saída do seu código (stdout)</span>
                    <pre class="bloco-io">{resAtual.stdout || "(vazio)"}</pre>
                  </div>
                </div>

                {#if resAtual.stderr}
                  <div class="col-io erro-io">
                    <span class="rotulo-io">Erro em execução (stderr)</span>
                    <pre class="bloco-io">{resAtual.stderr}</pre>
                  </div>
                {/if}
                {#if resAtual.compile_output}
                  <div class="col-io erro-io">
                    <span class="rotulo-io">Saída de compilação</span>
                    <pre class="bloco-io">{resAtual.compile_output}</pre>
                  </div>
                {/if}
              </div>
            </div>
          {/if}
        </article>
      {/each}
    </div>
  {/if}
</div>

<style>
  .visao-submissao-aluno {
    display: flex;
    flex-direction: column;
    gap: 0.95rem;
    width: 100%;
  }
  .cabeca-entrega {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .cabeca-entrega h2 {
    margin: 0 0 0.2rem;
    color: var(--seno-blue-900);
  }
  .link-atividade {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
    font-weight: 500;
  }
  .link-atividade:hover {
    text-decoration: underline;
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.86rem;
  }
  .pilula {
    border-radius: var(--raio-pill);
    padding: 0.1rem 0.5rem;
    font-size: 0.74rem;
    font-weight: 600;
  }
  .pilula.atrasada {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.15rem 1.35rem;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
  }
  .subrotulo {
    font-size: 0.76rem;
    font-weight: 700;
    color: var(--seno-gray-500);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .texto {
    margin: 0;
    white-space: pre-wrap;
    font-size: 0.92rem;
    color: var(--seno-gray-800);
    line-height: 1.5;
  }
  .pendente-box {
    background: var(--seno-blue-50);
    border-color: var(--seno-blue-200);
  }
  .pendente-box h3 {
    margin: 0;
    color: var(--seno-blue-900);
    font-size: 1.05rem;
  }

  /* Resumo da correção publicada */
  .resumo-publicada {
    background: linear-gradient(135deg, #f0fdf4 0%, #ffffff 100%);
    border-color: #86efac;
  }
  .resumo-topo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .resumo-info {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .badge-publicada {
    display: inline-flex;
    align-items: center;
    width: fit-content;
    background: var(--seno-green-50);
    color: var(--seno-green);
    border: 1px solid #bbf7d0;
    border-radius: var(--raio-pill);
    padding: 0.2rem 0.65rem;
    font-size: 0.8rem;
    font-weight: 700;
  }
  .placar-nota {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    background: var(--seno-white);
    border: 1px solid #bbf7d0;
    border-radius: var(--raio-md);
    padding: 0.5rem 0.95rem;
  }
  .placar-rotulo {
    font-size: 0.72rem;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--seno-gray-500);
  }
  .placar-valor {
    font-size: 1.35rem;
    color: var(--seno-green);
    line-height: 1.2;
  }
  .placar-total {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--seno-gray-600);
  }
  .feedback-geral {
    padding-top: 0.6rem;
    border-top: 1px dashed #bbf7d0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  /* Lista de questões */
  .lista-tarefas-corrigidas {
    display: flex;
    flex-direction: column;
    gap: 0.95rem;
  }
  .cartao-questao {
    gap: 0.85rem;
  }
  .questao-topo {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.8rem;
    flex-wrap: wrap;
    padding-bottom: 0.65rem;
    border-bottom: 1px solid var(--seno-gray-200);
  }
  .questao-identidade {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .questao-identidade h3 {
    margin: 0;
    font-size: 1.05rem;
    color: var(--seno-blue-900);
  }
  .questao-tags {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .tag-tipo {
    font-size: 0.73rem;
    font-weight: 600;
    padding: 0.12rem 0.55rem;
    border-radius: var(--raio-pill);
  }
  .tag-tipo.prog {
    background: var(--seno-blue-50);
    color: var(--seno-blue-700);
    border: 1px solid var(--seno-blue-100);
  }
  .tag-tipo.arquivo {
    background: #faf5ff;
    color: #7e22ce;
    border: 1px solid #f3e8ff;
  }
  .nota-questao-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.32rem 0.75rem;
    border-radius: var(--raio-pill);
    font-size: 0.85rem;
  }
  .nota-questao-badge.plena {
    background: var(--seno-green-50);
    color: var(--seno-green);
    border: 1px solid #bbf7d0;
  }
  .nota-questao-badge.parcial {
    background: #fffbeb;
    color: #b45309;
    border: 1px solid #fde68a;
  }
  .nota-questao-badge.zero {
    background: var(--seno-red-50);
    color: var(--seno-red);
    border: 1px solid #fecaca;
  }
  .nota-questao-badge.neutra {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }

  /* Enunciado e resposta */
  .detalhe-enunciado {
    background: var(--seno-gray-50);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-md);
    padding: 0.55rem 0.85rem;
  }
  .detalhe-enunciado summary {
    cursor: pointer;
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-blue-700);
    user-select: none;
  }
  .texto-enunciado {
    margin: 0.5rem 0 0;
    white-space: pre-wrap;
    font-size: 0.88rem;
    color: var(--seno-gray-800);
    line-height: 1.5;
  }
  .bloco-resposta {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .resposta-vazia {
    margin: 0;
    font-size: 0.86rem;
    font-style: italic;
    color: var(--seno-gray-500);
    background: var(--seno-gray-50);
    border: 1px dashed var(--seno-gray-300);
    border-radius: var(--raio-sm);
    padding: 0.6rem 0.85rem;
  }
  .feedback-tarefa {
    background: var(--seno-blue-50);
    border: 1px solid var(--seno-blue-100);
    border-radius: var(--raio-md);
    padding: 0.65rem 0.9rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  /* Arquivo enviado */
  .arquivo-box {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.75rem;
    padding: 0.7rem 0.95rem;
    background: var(--seno-blue-50);
    border: 1px solid var(--seno-blue-100);
    border-radius: var(--raio-md);
  }
  .arquivo-meta {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .botao-download {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    text-decoration: none;
    padding: 0.4rem 0.85rem;
    border-radius: var(--raio-md);
    font-size: 0.84rem;
    font-weight: 600;
  }
  .botao-download:hover {
    background: var(--seno-blue-700);
  }

  /* Visor unitário de resultados dos testes */
  .bloco-resultados {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding-top: 0.4rem;
    border-top: 1px solid var(--seno-gray-100);
  }
  .topo-resultados {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .titulo-resultados {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .pill-aprovados {
    font-size: 0.74rem;
    font-weight: 700;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    padding: 0.1rem 0.5rem;
    border-radius: var(--raio-pill);
  }
  .nav-setas {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .btn-seta {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: var(--raio-sm);
    border: 1px solid var(--seno-gray-300);
    background: var(--seno-white);
    color: var(--seno-gray-800);
    font-weight: 700;
    font-size: 0.88rem;
    cursor: pointer;
    padding: 0;
  }
  .btn-seta:hover:not(:disabled) {
    background: var(--seno-gray-100);
  }
  .btn-seta:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .indicador-teste {
    font-size: 0.8rem;
    color: var(--seno-gray-700);
  }
  .visor-resultado {
    background: var(--seno-gray-50);
    border: 1px solid var(--seno-gray-200);
    border-left: 4px solid var(--seno-red);
    border-radius: var(--raio-md);
    padding: 0.65rem 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  .visor-resultado.ok {
    border-left-color: var(--seno-green);
  }
  .res-meta-barra {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .res-badges {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .veredito {
    border-radius: var(--raio-pill);
    padding: 0.12rem 0.55rem;
    background: var(--seno-red-50);
    color: var(--seno-red);
    font-size: 0.74rem;
    font-weight: 700;
    text-transform: uppercase;
  }
  .veredito.ok {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .badge-vis {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.1rem 0.5rem;
    border-radius: var(--raio-pill);
    background: var(--seno-gray-200);
    color: var(--seno-gray-700);
  }
  .badge-vis.publico {
    background: var(--seno-blue-50);
    color: var(--seno-blue-700);
  }
  .res-metricas {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    font-size: 0.78rem;
    color: var(--seno-gray-600);
  }
  .grade-io-resultado {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 0.55rem;
  }
  .col-io {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    min-width: 0;
  }
  .rotulo-io {
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--seno-gray-500);
  }
  .bloco-io {
    margin: 0;
    padding: 0.4rem 0.55rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-sm);
    font-family: monospace;
    font-size: 0.78rem;
    color: var(--seno-gray-800);
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 5rem;
    overflow-y: auto;
  }
  .erro-io .bloco-io {
    background: #fef2f2;
    border-color: #fecaca;
    color: #991b1b;
  }
</style>
