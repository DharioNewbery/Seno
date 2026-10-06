<script lang="ts">
  // Entregas da atribuição: quem entregou, em quanto e atrasadas —
  // cada entrega abre o painel de correção.
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const atribuicao = $derived(data.atribuicao);
</script>

<section class="entregas">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  <div class="cabeca">
    <div>
      <h2>Entregas — {atribuicao?.atividade_nome || "Atividade"}</h2>
      <span class="meta">
        Turma {atribuicao?.turma_titulo || atribuicao?.turma_id} ·
        ({data.entregas.length} entregas)
      </span>
    </div>
    <a class="voltar" href={`/professor/atribuicoes/${data.atribuicao?.id ?? ""}`}>
      ← Configuração
    </a>
  </div>

  <ul class="lista">
    {#each data.entregas as e (e.id)}
      <li>
        <div class="identidade">
          <strong>{e.aluno}</strong>
          <span class="meta">
            {e.aluno_email} · com. {new Date(e.comecou_em).toLocaleString("pt-BR")} →
            ent. {new Date(e.entregue_em).toLocaleString("pt-BR")}
            {#if e.atrasada}· <span style:color="#c0392b">atrasada</span>{/if}
          </span>
        </div>
        <a class="abrir" href={`/professor/submissoes/${e.id}`}>Corrigir →</a>
      </li>
    {:else}
      <li class="vazio">Nenhuma entrega ainda.</li>
    {/each}
  </ul>
</section>

<style>
  .entregas {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
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
    margin: 0;
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
    gap: 0.15rem;
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
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
  }
  .vazio {
    color: var(--seno-gray-500);
    justify-content: center !important;
  }
</style>
