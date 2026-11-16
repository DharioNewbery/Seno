<script lang="ts">
  // Correção publicada da entrega do aluno: nota total, notas e
  // feedback por tarefa, código/arquivo enviado e resultados dos testes (§Visibilidade).
  import VisaoSubmissaoAluno from "#lib/components/VisaoSubmissaoAluno.svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const visao = $derived(data.visao);
  const entrega = $derived(data.entrega);
  const atribuicaoId = $derived(visao?.atribuicao_id ?? entrega?.atribuicao_id);
</script>

<section class="correcao">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if visao}
    <div class="navegacao">
      <a
        class="voltar"
        href={atribuicaoId
          ? `/aluno/atividades/${atribuicaoId}`
          : "/aluno/atividades"}
      >
        Voltar para a atividade
      </a>
    </div>
    <VisaoSubmissaoAluno
      {visao}
      {entrega}
      mostrarCabecalhoEntrega={true}
    />
  {/if}
</section>

<style>
  .correcao {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .navegacao {
    margin-bottom: -0.25rem;
  }
</style>
