<script lang="ts">
  // Correção publicada da entrega do aluno: nota total, notas e
  // feedback por tarefa, código/arquivo enviado e resultados dos testes (§Visibilidade).
  import VisaoSubmissaoAluno from "#lib/components/VisaoSubmissaoAluno.svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const visao = $derived(data.visao);
  const entrega = $derived(data.entrega);
</script>

<section class="correcao">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if visao}
    <VisaoSubmissaoAluno
      {visao}
      {entrega}
      mostrarCabecalhoEntrega={true}
    />
    <a class="voltar" href="/aluno/atividades">← Atividades</a>
  {/if}
</section>

<style>
  .correcao {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
</style>
