<script lang="ts">
  // Página genérica de seções ainda não implementadas nos portais.
  import type { PortalDef } from "#lib/portais";

  let { portal, caminho }: { portal: PortalDef; caminho: string } = $props();
</script>

<section class="construcao">
  <h1>{caminho}</h1>
  <p>
    Seção de <strong>{portal.titulo}</strong> ainda não implementada. O caminho
    já está reservado na navegação para os módulos seguintes.
  </p>
  <a href={`/${portal.id}`}>Voltar ao início</a>
</section>

<style>
  .construcao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    padding: 2rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .construcao h1 {
    margin: 0;
    text-transform: capitalize;
  }

  .construcao p {
    margin: 0;
    color: var(--seno-gray-700);
  }

  .construcao a {
    color: var(--seno-blue-600);
    margin-top: 0.75rem;
  }
</style>
