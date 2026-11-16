<script lang="ts">
  // Minhas turmas (aluno): apenas turmas ativas, sem tag de status e com
  // botões de ação por ícone.
  import Icone from "#lib/components/Icone.svelte";
  import {
    filtrarTurmasAtivasAluno,
    formatarSubtituloCardTurmaAluno,
    formatarTituloCardTurmaAluno,
  } from "#lib/turmas-aluno";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const turmas = $derived(filtrarTurmasAtivasAluno(data.turmas));
</script>

<section class="minhas-turmas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <div class="cabeca">
    <h2>Minhas turmas <span class="total">({turmas.length})</span></h2>
  </div>

  <ul class="lista">
    {#each turmas as t (t.id)}
      <li class="card-turma">
        <div class="identidade">
          <a class="titulo-link" href={`/aluno/turmas/${t.id}`}>
            <strong>{formatarTituloCardTurmaAluno(t)}</strong>
          </a>
          <span class="meta">
            {formatarSubtituloCardTurmaAluno(t)}
          </span>
        </div>
        <div class="acao-area">
          <a
            class="btn-icone secundario"
            href={`/aluno/turmas/${t.id}`}
            title="Ver turma"
            aria-label="Ver turma"
          >
            <Icone nome="eye" tamanho={18} />
          </a>
          <a
            class="btn-icone primario"
            href={`/aluno/atividades?turma_id=${t.id}`}
            title="Atividades"
            aria-label="Atividades"
          >
            <Icone nome="clipboard-list" tamanho={18} />
          </a>
        </div>
      </li>
    {:else}
      <li class="vazio">
        Você ainda não está matriculado em nenhuma turma ativa.
      </li>
    {/each}
  </ul>
</section>

<style>
  .minhas-turmas {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .cabeca h2 {
    margin: 0;
  }
  .card-turma {
    transition: border-color 0.16s ease, box-shadow 0.16s ease;
  }
  .card-turma:hover {
    border-color: var(--seno-blue-300, #93c5fd);
    box-shadow: 0 2px 8px rgba(15, 23, 42, 0.05);
  }
  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.22rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
  .titulo-link {
    text-decoration: none;
    color: inherit;
  }
  .titulo-link:hover strong {
    text-decoration: underline;
    color: var(--seno-blue-700);
  }
  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .btn-icone {
    width: 2.15rem;
    height: 2.15rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--raio-md);
    text-decoration: none;
    transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
  }
  .btn-icone.secundario {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
  }
  .btn-icone.secundario:hover {
    background: var(--seno-gray-200);
    color: var(--seno-blue-900);
  }
  .btn-icone.primario {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: 1px solid var(--seno-blue-600);
  }
  .btn-icone.primario:hover {
    background: var(--seno-blue-700);
    border-color: var(--seno-blue-700);
  }
</style>
