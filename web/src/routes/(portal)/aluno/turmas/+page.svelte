<script lang="ts">
  // Minhas turmas (aluno): matrículas ativas (e encerradas marcadas);
  // cada turma abre a lista de atividades dela.
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const turmas = $derived(data.turmas);
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
      <li>
        <div class="identidade">
          <a class="titulo-link" href={`/aluno/turmas/${t.id}`}>
            <strong>{t.materia_codigo} — {t.materia_nome}</strong>
          </a>
          <span class="meta">
            {t.titulo || "Sem título"} · {t.trimestre}T/{t.ano} ·
            {t.professor}
          </span>
        </div>
        <div class="acao-area">
          {#if t.ativa}
            <span class="estado ativa">Ativa</span>
          {:else}
            <span class="estado encerrada">Encerrada</span>
          {/if}
          <a class="abrir secundario" href={`/aluno/turmas/${t.id}`}>
            Ver turma
          </a>
          <a class="abrir" href={`/aluno/atividades?turma_id=${t.id}`}>
            Atividades →
          </a>
        </div>
      </li>
    {:else}
      <li class="vazio">
        Você ainda não está matriculado em nenhuma turma.
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
          .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
    .acao-area {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
        .titulo-link {
    text-decoration: none;
    color: inherit;
  }
  .titulo-link:hover strong {
    text-decoration: underline;
    color: var(--seno-blue-700);
  }
      .abrir.secundario {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
  }
  .abrir.secundario:hover {
    background: var(--seno-gray-200);
  }
  </style>
