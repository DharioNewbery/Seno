<script lang="ts">
  import { dataHora, prazoRelativo } from "#lib/formato";
  // Atividades do aluno (atribuições das minhas turmas): filtro por
  // turma, estado do rascunho e prazo; correção publicada no portal.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const atribuicoes = $derived(data.atribuicoes);

  function filtroTurma(id: number | null): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    if (id === null) qs.delete("turma_id");
    else qs.set("turma_id", String(id));
    const query = qs.toString();
    return query ? `?${query}` : "";
  }

  function estado(
    a: (typeof atribuicoes)[number] | undefined,
  ): { rotulo: string; classe: string } {
    if (a?.entregue)
      return a?.atrasada
        ? { rotulo: "Entregue atrasada", classe: "entregue" }
        : { rotulo: "Entregue", classe: "entregue" };
    if (a?.em_andamento) return { rotulo: "Em andamento", classe: "andamento" };
    if (a?.prazo && new Date(a.prazo) < new Date() && !a.pode_atrasado)
      return { rotulo: "Prazo encerrado", classe: "encerrada" };
    return { rotulo: "Não começou", classe: "nao" };
  }

  function paraTexto(iso: string | undefined): string {
    if (!iso) return "Sem prazo";
    return `Prazo ${dataHora(iso)}`;
  }

  const turmaFiltrada = $derived(
    data.turmas.find((t) => t.id === data.filtroTurma),
  );
</script>

<section class="minhas-atividades">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="cabeca">
    <h2>
      Atividades
      <span class="total">({atribuicoes.length})</span>
      {#if turmaFiltrada}
        <span class="filtro">— {turmaFiltrada.materia_codigo} {turmaFiltrada.titulo || ""}</span>
      {/if}
    </h2>
    {#if data.filtroTurma}
      <a class="limpar" href={filtroTurma(null)}>Ver de todas as turmas</a>
    {/if}
  </div>

  <nav class="turmas" aria-label="Filtrar por turma">
    <a href={filtroTurma(null)} class:y={data.filtroTurma == null}>Todas</a>
    {#each data.turmas.filter((t) => t.ativa) as t (t.id)}
      <a href={filtroTurma(t.id)} class:y={data.filtroTurma === t.id}>
        {t.materia_codigo} · {t.titulo || "Sem título"}
      </a>
    {/each}
  </nav>

  <ul class="lista">
    {#each atribuicoes as a (a.id)}
      {@const rel = a.entregue ? null : prazoRelativo(a.prazo)}
      <li>
        <div class="identidade">
          <strong>{a.atividade_nome}</strong>
          <span class="meta">
            {a.materia_codigo} · {a.turma_titulo || `Turma ${a.turma_id}`}
          </span>
          <span class="meta">
            {paraTexto(a.prazo)}
            {#if rel}<b class="rel {rel.tom}">{rel.rotulo}</b>{/if}
            {#if a.pode_atrasado}· atraso permitido{/if}
          </span>
        </div>
        <div class="acao-area">
          <span class="estado {estado(a).classe}">{estado(a).rotulo}</span>
          {#if a.turma_ativa && !a.entregue}
            <!-- Fase 9.4: entra pelo BFF (st → Monitor); cai no editor se a
                 integração estiver desligada -->
            <a class="abrir" href={`/aluno/monitor/${a.id}`}>
              {a.em_andamento ? `Continuar (rev. ${a.minha_revisao})` : "Abrir"} →
            </a>
          {:else if a.entregue}
            <span class="meta">Entregue em {a.entregue_em ? dataHora(a.entregue_em) : ""}</span>
            {#if a.publicada_em}
              <a class="corrigido" href={`/aluno/submissao/${a.submissao_id}`}>Correção publicada →</a>
            {/if}
          {:else}
            <span class="estado encerrada">Turma encerrada</span>
          {/if}
        </div>
      </li>
    {:else}
      <li class="vazio">
        Nenhuma atividade disponível. As atribuições aparecem quando o
        professor anexa uma atividade à sua turma.
      </li>
    {/each}
  </ul>

  <p class="nota">
    Quando o professor publicar as notas, a correção completa (feedback e
    resultados por teste) fica disponível em "Correção publicada".
  </p>
</section>

<style>
  .minhas-atividades {
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
    .filtro {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .limpar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
    .turmas {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .turmas a {
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-pill);
    padding: 0.25rem 0.8rem;
    color: var(--seno-gray-700);
    font-size: 0.83rem;
    text-decoration: none;
    background: var(--seno-white);
  }
  .turmas a.y {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-500);
    color: var(--seno-blue-800);
    font-weight: 600;
  }
      .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
    .rel {
    font-weight: 600;
  }
  .rel.folga {
    color: var(--seno-gray-700);
  }
  .rel.breve {
    color: var(--seno-amber);
  }
  .rel.vencido {
    color: var(--seno-red);
  }
  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
    .estado.nao {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .estado.andamento {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .estado.entregue {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }
  .estado.encerrada {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
      .corrigido {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.86rem;
  }
    .nota {
    color: var(--seno-gray-500);
    font-size: 0.82rem;
    margin: 0;
  }
</style>
