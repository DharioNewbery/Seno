<script lang="ts">
  import { dataHora } from "#lib/formato";
  // Fazer atividade (aluno): enunciados da atividade + Editor de código
  // com autosave local (IndexedDB) e remoto (15 s ou ao parar de digitar).
  import EditorCodigo from "#lib/components/EditorCodigo.svelte";
  import {
    gravarRascunhoLocal,
    obterRascunhoLocal,
    sincronizarRascunho,
    type Snapshot,
  } from "#lib/autosave";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const visao = $derived(data.visao);
  const tarefas = $derived(visao?.atividade.tarefas.filter((t) => !t.excluida) ?? []);
  const alunoId = $derived(data.alunoId ?? 0);

  let textos = $state<Record<number, string>>({});
  let revisao = $state(1);
  let estadoSave = $state<"nada" | "salvando" | "salvo" | "erro" | "offline">("nada");
  let conflitoAviso = $state(false);
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

    // Verifica se há rascunho local mais recente pendente no IndexedDB
    if (alunoId && visao.atribuicao.id) {
      obterRascunhoLocal(visao.atribuicao.id, alunoId).then((local) => {
        if (local && local.pendente && local.revisao_base >= revisao) {
          for (const item of local.snapshot.tasks ?? []) {
            textos[item.id] = item.text;
          }
          revisao = local.revisao_base;
        }
      });
    }

    carregado = true;
  });

  async function enviarSnapshotParaServidor(
    rev: number,
    snapshot: Snapshot,
  ): Promise<{ ok: boolean; status: number; revisao?: number }> {
    if (!visao) return { ok: false, status: 500 };
    const params = new URLSearchParams();
    params.set("revisao", String(rev));
    params.set("snapshot", JSON.stringify(snapshot));
    const res = await fetch(`/aluno/atividade/${visao.atribuicao.id}?/gravar`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: params.toString(),
    });
    const saida = (await res.json().catch(() => ({}))) as { revisao?: number };
    return {
      ok: res.ok,
      status: res.status,
      revisao: saida.revisao,
    };
  }

  async function tentarSincronizar() {
    if (!visao || !alunoId) return;
    const res = await sincronizarRascunho(
      visao.atribuicao.id,
      alunoId,
      enviarSnapshotParaServidor,
    );
    if (res.conflito) {
      conflitoAviso = true;
    } else if (res.sincronizado && res.novaRevisao) {
      revisao = res.novaRevisao;
      estadoSave = "salvo";
    }
  }

  async function salvar() {
    if (!visao || carro) return;
    carro = true;
    estadoSave = "salvando";
    const snapshotAtual: Snapshot = {
      tasks: tarefas.map((t) => ({ id: t.id, text: textos[t.id] ?? "" })),
    };

    // 1. Sempre salva localmente primeiro (IndexedDB)
    if (alunoId) {
      await gravarRascunhoLocal(
        visao.atribuicao.id,
        alunoId,
        snapshotAtual,
        revisao,
        true,
      );
    }

    // 2. Envia para o servidor
    try {
      const res = await enviarSnapshotParaServidor(revisao, snapshotAtual);
      if (res.status === 409) {
        conflitoAviso = true;
        estadoSave = "erro";
      } else if (res.ok && typeof res.revisao === "number") {
        revisao = res.revisao;
        estadoSave = "salvo";
        conflitoAviso = false;
        if (alunoId) {
          await gravarRascunhoLocal(
            visao.atribuicao.id,
            alunoId,
            snapshotAtual,
            revisao,
            false,
          );
        }
      } else {
        estadoSave = "erro";
      }
    } catch {
      // Offline: mantém salvo no IndexedDB como pendente
      estadoSave = "offline";
    } finally {
      carro = false;
    }
  }

  $effect(() => {
    const pulso = globalThis.setInterval(salvar, 15000);
    const onOnline = () => {
      tentarSincronizar();
    };
    if (typeof window !== "undefined") {
      window.addEventListener("online", onOnline);
    }
    return () => {
      globalThis.clearInterval(pulso);
      if (typeof window !== "undefined") {
        window.removeEventListener("online", onOnline);
      }
    };
  });

  function aoDigitar(id: number, valor: string) {
    textos[id] = valor;
    if (alunoId && visao) {
      const snapshotAtual: Snapshot = {
        tasks: tarefas.map((t) => ({
          id: t.id,
          text: t.id === id ? valor : (textos[t.id] ?? ""),
        })),
      };
      gravarRascunhoLocal(
        visao.atribuicao.id,
        alunoId,
        snapshotAtual,
        revisao,
        true,
      );
    }
  }
</script>

<section class="atividade-aluno">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if visao}
    {#if conflitoAviso}
      <p class="aviso erro" role="alert">
        Atenção: outra aba ou dispositivo gravou uma versão mais recente deste rascunho (conflito 409).
        Seu código local foi preservado. Recarregue a página se desejar puxar a versão do servidor.
      </p>
    {/if}

    <div class="cabeca">
      <div>
        <h2>{visao.atividade.nome}</h2>
        <span class="meta">
          {visao.atribuicao.prazo
            ? `Prazo: ${dataHora(visao.atribuicao.prazo)}`
            : "Sem prazo"}
          — atrasado: {visao.atribuicao.pode_atrasado ? "permitido" : "não permitido"}
        </span>
      </div>
      <span class="estado-save" data-estado={estadoSave}>
        {#if estadoSave === "salvando"}Salvando…{:else if estadoSave === "salvo"}
          Rascunho salvo (revisão {revisao}){:else if estadoSave === "offline"}
          Salvo localmente (offline){:else if estadoSave === "erro"}Falhou — tente de novo{:else}Rascunho abre no primeiro save{/if}
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

        <EditorCodigo
          linguagem={t.linguagem}
          valor={textos[t.id] ?? ""}
          autocomplete={visao.atribuicao.autocomplete ?? false}
          onchange={(val) => aoDigitar(t.id, val)}
        />


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
    width: 100%;
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
    .estado-save {
    font-size: 0.8rem;
    border-radius: var(--raio-pill);
    padding: 0.25rem 0.7rem;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .estado-save[data-estado="salvo"] {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .estado-save[data-estado="erro"] {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .tarefa {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
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
    border-radius: var(--raio-pill);
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
    border-radius: var(--raio-pill);
    padding: 0.15rem 0.6rem;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .enunciado {
    margin: 0;
    color: var(--seno-gray-700);
    white-space: pre-wrap;
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
    border-radius: var(--raio-md);
    padding: 0.35rem 0.8rem;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .testar button:hover {
    background: var(--seno-blue-100);
  }
  .testes {
    background: var(--seno-gray-100);
    border-radius: var(--raio-md);
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
  .testes pre {
    max-width: 100%;
    white-space: pre-wrap;
    font-size: 0.78rem;
    background: var(--seno-white);
    border-radius: var(--raio-sm);
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
    border-radius: var(--raio-md);
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
    background: var(--seno-red-50);
  }
  .confirma-entrega {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    flex-wrap: wrap;
    background: var(--seno-gray-100);
    border-radius: var(--raio-md);
    padding: 0.5rem 0.7rem;
    font-size: 0.85rem;
    color: var(--seno-gray-700);
  }
  .confirma-entrega button {
    background: var(--seno-red);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.4rem 0.8rem;
    cursor: pointer;
  }
  .confirma-entrega .cancelar {
    background: none;
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
  }
  </style>
