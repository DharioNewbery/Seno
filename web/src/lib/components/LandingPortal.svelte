<script lang="ts">
  // Landing comum dos portais: hero + atalhos das seções principais.
  // Serve de esqueleto para popular quando os módulos ficarem concretos.
  import type { PortalDef } from "#lib/portais";

  let { portal }: { portal: PortalDef } = $props();

  const atalhos = $derived(portal.nav.filter((item) => item.href !== `/${portal.id}`));
</script>

<section class="hero">
  <h1>{portal.titulo}</h1>
  <p>{portal.descricao}</p>
  <p class="aviso">
    Painel em construção — estatísticas e resumos entram com os módulos de
    domínio.
  </p>
</section>

<section class="atalhos" aria-label="Atalhos do portal">
  {#each atalhos as item (item.rotulo)}
    <a class="atalho" href={item.href}>
      <span class="rotulo">{item.rotulo}</span>
      <span class="caminho">{item.href}</span>
    </a>
  {/each}
</section>

<style>
  .hero {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    padding: 1.75rem;
  }

  .hero h1 {
    margin: 0 0 0.5rem;
  }

  .hero p {
    margin: 0;
    color: var(--seno-gray-700);
  }

  .aviso {
    margin-top: 0.75rem !important;
    color: var(--seno-blue-600) !important;
    font-weight: 600;
    font-size: 0.9rem;
  }

  .atalhos {
    margin-top: 1.25rem;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
    gap: 0.75rem;
  }

  .atalho {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 0.9rem 1rem;
    text-decoration: none;
    color: inherit;
  }

  .atalho:hover {
    border-color: var(--seno-blue-500);
  }

  .rotulo {
    font-weight: 600;
    color: var(--seno-blue-900);
  }

  .caminho {
    font-size: 0.78rem;
    color: var(--seno-gray-500);
  }
</style>
