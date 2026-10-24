<script lang="ts">
  import { dataHora } from "#lib/formato";
  // Correção da entrega (professor): resultados automáticos por teste,
  // ajuste de nota/feedback por tarefa, confirmação e publicação.
  import type { PageProps } from "./$types";
  import EditorCodigo from "#lib/components/EditorCodigo.svelte";

  let { data, form }: PageProps = $props();

  const visao = $derived(data.visao);
  const cor = $derived(visao?.correcao ?? null);
  const publicada = $derived(Boolean(cor?.publicada_em));

  function notaAtual(t: { nota_final?: number; nota_auto?: number }): string {
    return String(t.nota_final ?? t.nota_auto ?? "");
  }
</script>

<section class="correcao">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if visao}
    {#if form?.erro}
      <p class="aviso erro" role="alert">{form.erro}</p>
    {/if}
    {#if form?.ok}
      <p class="aviso ok">{form.ok}</p>
    {/if}

    <div class="cabeca">
      <div>
        <h2>{visao.submissao.aluno} — {visao.submissao.atividade_nome}</h2>
        <span class="meta">
          {dataHora(visao.submissao.entregue_em)}
          {#if visao.submissao.atrasada}· <span style:color="var(--seno-red)">atrasada</span>{/if}
        </span>
      </div>
      <div class="estado-area">
        <span class="estado">{cor?.status}</span>
        {#if publicada}
          <span class="estado publicada">Publicada</span>
        {/if}
      </div>
    </div>

    {#if cor}
      <form method="POST" action="?/salvar" class="cartao">
        <label>
          <span>Feedback geral</span>
          <textarea name="feedback" rows="2" disabled={publicada}>{cor.feedback ?? ""}</textarea>
        </label>

        {#each cor.tarefas as t (t.id)}
          <fieldset>
            <legend>
              {t.tarefa_nome} ({t.valor_pts} pts)
            </legend>
            {#if t.enunciado}
              <div class="bloco-enunciado">
                <span class="subrotulo">Enunciado:</span>
                <p class="texto-enunciado">{t.enunciado}</p>
              </div>
            {/if}
            {#if t.codigo}
              <div class="bloco-codigo">
                <span class="subrotulo">Código submetido pelo aluno:</span>
                <EditorCodigo valor={t.codigo} somenteLeitura={true} />
              </div>
            {/if}
            <input type="hidden" name="ct_ids" value={t.id} />
            <div class="linha">
              {#if t.nota_auto !== undefined && t.nota_auto !== null}
                <div class="mini auto-box">
                  <span class="rotulo-mini">Nota automática</span>
                  <span class="valor-auto">{t.nota_auto} / {t.valor_pts} pts</span>
                </div>
              {/if}
              <label class="mini">
                <span class="rotulo-mini">Nota final ({t.valor_pts} pts)</span>
                <input
                  type="number"
                  name={`nota_${t.id}`}
                  min="0"
                  max={t.valor_pts}
                  value={notaAtual(t)}
                  disabled={publicada}
                />
              </label>
              <label class="mini grow">
                <span class="rotulo-mini">Feedback da tarefa</span>
                <input name={`fb_${t.id}`} value={t.feedback ?? ""} disabled={publicada} placeholder="Comentários sobre a resposta..." />
              </label>
            </div>
            <ul class="resultados">
              {#each t.resultados as res (res.teste_id)}
                <li class:ok={res.status === "accepted"}>
                  <div class="teste-cabeca">
                    <span class="veredito">{res.status ?? "—"}</span>
                    <span>teste #{res.teste_id} {res.publico ? "(público)" : "(privado)"}</span>
                    {#if res.tempo}<span>· tempo: {res.tempo}s</span>{/if}
                    {#if res.memoria}<span>· memória: {res.memoria}KB</span>{/if}
                  </div>
                  {#if res.stdout}
                    <details><summary>stdout</summary><pre>{res.stdout}</pre></details>
                  {/if}
                  {#if res.stderr}
                    <details><summary>stderr</summary><pre>{res.stderr}</pre></details>
                  {/if}
                  {#if res.compile_output}
                    <details><summary>compilação</summary><pre>{res.compile_output}</pre></details>
                  {/if}
                </li>
              {:else}
                <li class="sem">Sem resultados (tarefa aberta ou Judge0 desligado).</li>
              {/each}
            </ul>
          </fieldset>
        {/each}

        {#if !publicada}
          <div class="botoes">
            <button type="submit" class="botao">Salvar correção</button>
            <button type="submit" formaction="?/confirmar" class="botao verde">{cor?.confirmada_em ? "Reconfirmar validação" : "Confirmar / Validar"}</button>
          </div>
        {/if}
      </form>

      {#if !publicada}
        <div class="trava">
          {#if !cor?.confirmada_em || cor?.status === "falhou"}
            <form method="POST" action="?/reexecutar">
              <button class="acao cinza">Reexecutar testes</button>
            </form>
          {/if}
          {#if cor?.confirmada_em}
            <span class="badge-confirmada">✓ Validada e pronta para publicação</span>
          {/if}
          <span class="meta">A publicação das notas é feita para toda a turma de uma vez na tela de Entregas da atividade.</span>
        </div>
      {/if}

    {/if}

    <a class="voltar" href={`/professor/atribuicoes/${visao.submissao.atribuicao_id}/entregas`}>
      ← Entregas
    </a>
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
    gap: 1rem;
    flex-wrap: wrap;
  }
  .cabeca h2 {
    margin: 0 0 0.2rem;
  }
    .estado-area {
    display: flex;
    gap: 0.4rem;
  }
    .estado.publicada {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.3rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  label span {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
    fieldset input {
    background: var(--seno-white);
  }
  fieldset {
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-lg);
    padding: 0.75rem 0.95rem;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  legend {
    font-size: 0.86rem;
    font-weight: 600;
    color: var(--seno-blue-900);
  }
  .linha {
    display: flex;
    gap: 0.6rem;
    align-items: end;
  }
  .mini {
    width: 120px;
  }
  .mini.grow {
    flex: 1;
  }
  .rotulo-mini {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--seno-gray-700);
    display: block;
    margin-bottom: 0.2rem;
  }
  .auto-box {
    background: var(--seno-blue-50);
    border: 1px solid var(--seno-blue-100);
    border-radius: var(--raio-md);
    padding: 0.35rem 0.5rem;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
  .valor-auto {
    font-weight: 700;
    color: var(--seno-blue-800);
    font-size: 0.92rem;
  }
  .teste-cabeca {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .badge-confirmada {
    background: var(--seno-green-50);
    color: var(--seno-green);
    font-weight: 600;
    font-size: 0.82rem;
    padding: 0.3rem 0.6rem;
    border-radius: var(--raio-md);
  }
  .resultados {
    list-style: none;
    padding: 0;
    margin: 0;
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
  .sem {
    color: var(--seno-gray-500);
  }
  .botoes button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.6rem 1.2rem;
    font-weight: 600;
    cursor: pointer;
  }
  .botoes button:hover {
    background: var(--seno-blue-700);
  }
  .botoes button.verde {
    background: var(--seno-green);
  }
  .botoes button.verde:hover {
    background: #15803d;
  }
  .trava {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .trava form {
    display: contents;
  }
  .acao {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.5rem 0.9rem;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .acao:hover {
    background: var(--seno-blue-700);
  }
  .acao.cinza {
    background: var(--seno-gray-600);
  }
  .acao.cinza:hover {
    background: var(--seno-gray-700);
  }

        .bloco-enunciado {
    background: var(--seno-gray-100);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-sm);
    padding: 0.75rem 1rem;
    margin-bottom: 0.75rem;
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
    margin-bottom: 0.75rem;
  }
</style>
