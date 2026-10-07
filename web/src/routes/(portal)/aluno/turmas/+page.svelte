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
          <strong>{t.materia_codigo} — {t.materia_nome}</strong>
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
  .total {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .lista {
    list-style: none;
    padding: 0;
    margin: 0.4rem 0 0;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  .lista li {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 0.85rem 1.05rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.8rem;
    flex-wrap: wrap;
  }
  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .identidade strong {
    color: var(--seno-blue-900);
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.83rem;
  }
  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .estado {
    font-size: 0.73rem;
    border-radius: 999px;
    padding: 0.18rem 0.55rem;
  }
  .estado.ativa {
    background: #e6f4ea;
    color: var(--seno-green);
  }
  .estado.encerrada {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .abrir {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: 8px;
    padding: 0.4rem 0.8rem;
    text-decoration: none;
    font-size: 0.86rem;
  }
  .abrir:hover {
    background: var(--seno-blue-700);
  }
  .vazio {
    color: var(--seno-gray-500);
    justify-content: center !important;
  }
</style>
