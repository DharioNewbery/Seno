<script lang="ts">
  // Fazer atividade (aluno): enunciados da atividade + textarea por
  // tarefa, com autosave (15 s + ao digitar parar) na tentativa.
  // 409 (outro aba gravou) recarrega a página para o rascunho atual.
  import { invalidate } from "$app/navigation";
  import { page } from "$app/state";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const visao = $derived(data.visao);
  const tarefas = $derived(visao?.atividade.tarefas.filter((t) => !t.excluida) ?? []);

  let textos = $state<Record<number, string>>({});
  let revisao = $state(1);
  let estadoSave = $state<"nada" | "salvando" | "salvo" | "erro">("nada");
  let carro = $state(false);
  let carregado = $state(false);
  let formulandoEntrega = $state(false);

  $effect(() => {
    if (carregado || !visao) return;
    if (visao.tentativa) {
      revisao = visao.tentativa.revisao;
      for (const item of visao.tentativa.snapshot.tasks ?? []) {
        textos[item.id] = item.text;
      }
    } else {
      revisao = 1;
    }
    carregado = true;
  });

  async function salvar() {
    if (!visao || carro) return;
    carro = true;
    estadoSave = "salvando";
    try {
      const quadro = {
        revisao,
        snapshot: {
          tasks: tarefas.map((t) => ({ id: t.id, text: textos[t.id] ?? "" })),
        },
      };
      const form = new URLSearchParams();
      form.set("revisao", String(quadro.revisao));
      form.set("snapshot", JSON.stringify(quadro.snapshot));
      const res = await fetch(`/aluno/atividade/${visao.atribuicao.id}?/gravar`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: form.toString(),
      });
      const saida = (await res.json()) as { revisao?: number };
      if (res.ok && typeof saida.revisao === "number") {
        revisao = saida.revisao;
        estadoSave = "salvo";
      } else {
        estadoSave = "erro";
      }
    } catch {
      estadoSave = "erro";
    } finally {
      carro = false;
    }
  }
  $effect(() => {
    const pulso = setInterval(salvar, 15000);
    return () => clearInterval(pulso);
  });

  function aoDigitar(id: number, valor: string) {
    textos[id] = valor;
  }
</script>

<section class="atividade-aluno">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if visao}
    <div class="cabeca">
      <div>
        <h2>{visao.atividade.nome}</h2>
        <span class="meta">
          {visao.atribuicao.prazo
            ? `Prazo: ${new Date(visao.atribuicao.prazo).toLocaleString("pt-BR")}`
            : "Sem prazo"}
          — atrasado: {visao.atribuicao.pode_atrasado ? "permitido" : "não permitido"}
        </span>
      </div>
      <span class="estado-save" data-estado={estadoSave}>
        {#if estadoSave === "salvando"}Salvando…{:else if estadoSave === "salvo"}
          Rascunho salvo (revisão {revisao}){:else if estadoSave === "erro"}Falhou — tente de novo{:else}Rascunho abre no primeiro save{/if}
      </span>
    </div>

    {#each tarefas as t (t.id)}
      <article class="tarefa">
        <header>
          <span class="ordem">{t.ordem}</span>
          <strong>{t.nome}</strong>
          <span class="pts">{t.valor_pts} pts</span>
          <span class="ling">{t.linguagem}</span>
        </header>
        <p class="enunciado">{t.enunciado}</p>
        <textarea
          rows="10"
          spellcheck="false"
          placeholder="Escreva sua resposta"
          value={textos[t.id] ?? ""}
          oninput={(e) => aoDigitar(t.id, e.currentTarget.value)}
          onchange={salvar}
        ></textarea>

        {#if t.teste_publico && !formulandoEntrega}
          <form method="POST" action="?/testar" class="testar">
            <input type="hidden" name="tarefa_id" value={t.id} />
            <button disabled={carro}>Testar tarefa</button>
            {#if typeof form?.testado === "number" && form.testado === t.id}
              <span class="mono nota-teste">Resultados abaixo ↓</span>
            {/if}
          </form>
          {#if typeof form?.testado === "number" && form.testado === t.id}
            <div class="testes">
              <ul>
                {#each (form.resultados ?? []) as res (res.teste_id)}
                  <li class:ok={res.status === "accepted"}>
                    <span class="veredito">{res.status ?? "—"}</span>
                    <span class="mono">teste #{res.teste_id}</span>
                    {#if res.tempo}<span class="mono">{res.tempo}s</span>{/if}
                    {#if res.memoria}<span class="mono">{res.memoria}KB</span>{/if}
                    {#if res.stderr}
                      <details><summary>stderr</summary><pre>{res.stderr}</pre></details>
                    {/if}
                  </li>
                {/each}
              </ul>
              <span class="mono dica-teste">
                Histórico nos últimos 7 dias: <a href="/aluno/testes">ver histórico</a>
              </span>
            </div>
          {/if}
        {/if}
      </article>
    {/each}

    <div class="rodape">
      <button onclick={salvar} disabled={carro}>Salvar rascunho agora</button>
      {#if formulandoEntrega}
        <form method="POST" action="?/entregar" class="confirma-entrega">
          <span>
            Entregar agora? O rascunho atual (rev. {revisao}) vira a submissão
            e não há como voltar.
          </span>
          <button>Confirmar entrega</button>
          <button type="button" class="cancelar" onclick={() => (formulandoEntrega = false)}>
            Cancelar
          </button>
        </form>
      {:else}
        <button class="entregar" onclick={() => formulandoEntrega = true}>
          Entregar atividade
        </button>
      {/if}
    </div>
  {/if}
</section>

<style>
  .atividade-aluno {
    max-width: 820px;
    display: flex;
    flex-direction: column;
    gap: 1rem;
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
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }
  .estado-save {
    font-size: 0.8rem;
    border-radius: 999px;
    padding: 0.25rem 0.7rem;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .estado-save[data-estado="salvo"] {
    background: #e6f4ea;
    color: var(--seno-green);
  }
  .estado-save[data-estado="erro"] {
    background: #fdecea;
    color: var(--seno-red);
  }
  .tarefa {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    padding: 1rem 1.2rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .tarefa header {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .ordem {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: 999px;
    width: 1.6rem;
    height: 1.6rem;
    display: grid;
    place-items: center;
    font-size: 0.8rem;
  }
  .tarefa header strong {
    color: var(--seno-blue-900);
  }
  .pts {
    color: var(--seno-gray-500);
    font-size: 0.8rem;
  }
  .ling {
    margin-left: auto;
    font-size: 0.72rem;
    border-radius: 999px;
    padding: 0.15rem 0.6rem;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .enunciado {
    margin: 0;
    color: var(--seno-gray-700);
    white-space: pre-wrap;
  }
  textarea {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.6rem 0.7rem;
    font-family: monospace;
    font-size: 0.88rem;
    background: #fbfcfe;
    resize: vertical;
  }
  .testar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .testar button {
    background: none;
    color: var(--seno-blue-600);
    border: 1px solid var(--seno-blue-600);
    border-radius: 8px;
    padding: 0.35rem 0.8rem;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .testar button:hover {
    background: #eaf2ff;
  }
  .testes {
    background: var(--seno-gray-100);
    border-radius: 8px;
    padding: 0.5rem 0.7rem;
  }
  .testes ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .testes li {
    font-size: 0.82rem;
    display: flex;
    gap: 0.45rem;
    align-items: center;
    flex-wrap: wrap;
  }
  .testes li.ok .veredito {
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
  .testes pre {
    max-width: 480px;
    white-space: pre-wrap;
    font-size: 0.78rem;
    background: var(--seno-white);
    border-radius: 6px;
    padding: 0.3rem 0.45rem;
  }
  details summary {
    cursor: pointer;
    font-size: 0.76rem;
  }
  .nota-teste,
  .dica-teste {
    color: var(--seno-gray-500);
  }
  .dica-teste a {
    color: var(--seno-blue-600);
  }
  .rodape {
    display: flex;
    align-items: center;
    gap: 0.9rem;
  }
  .rodape button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.6rem 1.1rem;
    cursor: pointer;
  }
  .rodape button:disabled {
    opacity: 0.55;
    cursor: default;
  }
  .rodape .entregar {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }
  .rodape .entregar:hover {
    background: #fdecea;
  }
  .confirma-entrega {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    flex-wrap: wrap;
    background: var(--seno-gray-100);
    border-radius: 8px;
    padding: 0.5rem 0.7rem;
    font-size: 0.85rem;
    color: var(--seno-gray-700);
  }
  .confirma-entrega button {
    background: var(--seno-red);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.4rem 0.8rem;
    cursor: pointer;
  }
  .confirma-entrega .cancelar {
    background: none;
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
</style>
