<script lang="ts">
  // Correção da entrega (professor): resultados automáticos por teste,
  // ajuste de nota/feedback por tarefa, confirmação e publicação.
  import type { PageProps } from "./$types";

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
          {new Date(visao.submissao.entregue_em).toLocaleString("pt-BR")}
          {#if visao.submissao.atrasada}· <span style:color="#c0392b">atrasada</span>{/if}
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
            <input type="hidden" name="ct_ids" value={t.id} />
            <div class="linha">
              <label class="mini">
                <span>Nota final</span>
                <input
                  type="number"
                  name={`nota_${t.id}`}
                  min="0"
                  value={notaAtual(t)}
                  disabled={publicada}
                />
              </label>
              <label class="mini grow">
                <span>Feedback da tarefa</span>
                <input name={`fb_${t.id}`} value={t.feedback ?? ""} disabled={publicada} />
              </label>
            </div>
            <ul class="resultados">
              {#each t.resultados as res (res.teste_id)}
                <li class:ok={res.status === "accepted"}>
                  <span class="veredito">{res.status ?? "—"}</span>
                  <span>teste #{res.teste_id} {res.publico ? "(público)" : "(privado)"}</span>
                  {#if res.tempo}<span>{res.tempo}s</span>{/if}
                  {#if res.memoria}<span>{res.memoria}KB</span>{/if}
                  {#if res.stderr}<details><summary>stderr</summary><pre>{res.stderr}</pre></details>{/if}
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
            <button>Salvar correção</button>
          </div>
        {/if}
      </form>

      {#if !publicada}
        <div class="trava">
          <form method="POST" action="?/confirmar">
            <button class="acao">Confirmar</button>
          </form>
          <form method="POST" action="?/publicar">
            <button class="acao verde">Publicar notas</button>
          </form>
          <span class="meta">Publicar exige confirmação; depois disso congela.</span>
        </div>
      {/if}
    {/if}

    <a class="voltar" href={`/professor/atribuicoes/${data.submissaoID}/entregas`}>
      ← Entregas
    </a>
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
    gap: 1rem;
    flex-wrap: wrap;
  }
  .cabeca h2 {
    margin: 0 0 0.2rem;
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }
  .estado-area {
    display: flex;
    gap: 0.4rem;
  }
  .estado {
    font-size: 0.75rem;
    border-radius: 999px;
    padding: 0.2rem 0.6rem;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .estado.publicada {
    background: #e6f4ea;
    color: var(--seno-green);
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
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
  input,
  textarea {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.45rem 0.6rem;
    font-size: 0.9rem;
    background: var(--seno-gray-100);
    font-family: inherit;
  }
  fieldset input {
    background: var(--seno-white);
  }
  fieldset {
    border: 1px solid var(--seno-gray-200);
    border-radius: 10px;
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
    width: 90px;
  }
  .mini.grow {
    flex: 1;
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
  .sem {
    color: var(--seno-gray-500);
  }
  .botoes button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.6rem 1.2rem;
    font-weight: 600;
    cursor: pointer;
  }
  .botoes button:hover {
    background: var(--seno-blue-700);
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
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .acao:hover {
    background: var(--seno-blue-700);
  }
  .acao.verde {
    background: #2f855a;
  }
  .acao.verde:hover {
    background: #276749;
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .aviso.ok {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
  }
</style>
