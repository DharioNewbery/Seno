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
  .total {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .nova {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: 8px;
    padding: 0.45rem 0.9rem;
    text-decoration: none;
    font-size: 0.9rem;
    font-weight: 600;
  }
  .nova:hover {
    background: var(--seno-blue-700);
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
    padding: 0.8rem 1rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.7rem;
    flex-wrap: wrap;
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
  .peq {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.32rem 0.7rem;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .peq.vermelho {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }
  .peq.vermelho:hover {
    background: #fdecea;
  }
  .peq:hover {
    background: var(--seno-blue-700);
  }
  .vazio {
    color: var(--seno-gray-500);
    justify-content: center !important;
  }
</style>
