<script lang="ts">
  // Banco de atividades: listagem com ações (abrir, duplicar, excluir).
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const atividades = $derived(data.atividades);
  let confirmando = $state<number | null>(null);
</script>

<section class="atividades">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}

  <div class="cabeca">
    <h2>
      Banco de atividades
      <span class="total">({atividades.length})</span>
    </h2>
    <a class="nova" href="/professor/atividades/nova">Criar atividade +</a>
  </div>

  <ul class="lista">
    {#each atividades as a (a.id)}
      <li>
        <div class="identidade">
          <a class="nome" href={`/professor/atividades/${a.id}`}>{a.nome}</a>
        </div>
        <div class="acoes">
          <a class="link" href={`/professor/atividades/${a.id}`}>Abrir →</a>
          <form method="POST" action="?/duplicar">
            <input type="hidden" name="id" value={a.id} />
            <button class="peq">Duplicar</button>
          </form>
          {#if confirmando === a.id}
            <form method="POST" action="?/excluir" class="confirma">
              <span>Excluir?</span>
              <input type="hidden" name="id" value={a.id} />
              <button class="peq vermelho">Sim</button>
              <button type="button" class="peq" onclick={() => (confirmando = null)}>
                Não
              </button>
            </form>
          {:else}
            <button class="peq vermelho" onclick={() => (confirmando = a.id)}>
              Excluir
            </button>
          {/if}
        </div>
      </li>
    {:else}
      <li class="vazio">Nenhuma atividade. Envelop suas tarefas em uma prova/atividade.</li>
    {/each}
  </ul>
</section>

<style>
  .atividades {
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
  }
  .nome {
    color: var(--seno-blue-900);
    text-decoration: none;
    font-weight: 600;
  }
  .nome:hover {
    text-decoration: underline;
  }
  .acoes {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .acoes form {
    display: contents;
  }
  .confirma {
    align-items: center;
    gap: 0.3rem;
    color: var(--seno-red);
    font-size: 0.82rem;
  }
  .link {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.86rem;
  }
          </style>
