<script lang="ts">
  // Página inicial comum dos portais: saudação e atalhos das seções principais.
  import { page } from "$app/state";
  import NavIcon from "./NavIcon.svelte";
  import { iconeDoItem, type PortalDef } from "#lib/portais";

  let { portal }: { portal: PortalDef } = $props();

  const nome = $derived(page.data.usuario?.first_name as string | undefined);
  const atalhos = $derived(portal.nav.filter((item) => item.href !== `/${portal.id}`));
</script>

<section class="saudacao">
  <h1>{nome ? `Olá, ${nome}` : portal.titulo}</h1>
  <p>{portal.descricao}</p>
</section>

<nav class="atalhos" aria-label="Atalhos do portal">
  {#each atalhos as item (item.rotulo)}
    <a class="atalho" href={item.href}>
      <span class="icone"><NavIcon nome={iconeDoItem(portal.id, item.href)} tamanho={22} /></span>
      <span class="texto">
        <strong>{item.rotulo}</strong>
        {#if item.descricao}<span>{item.descricao}</span>{/if}
      </span>
    </a>
  {/each}
</nav>

<style>
  .saudacao h1 {
    margin: 0 0 0.35rem;
    font-size: 1.9rem;
  }

  .saudacao p {
    margin: 0;
    color: var(--seno-gray-700);
    max-width: 46ch;
  }

  .atalhos {
    margin-top: 1.75rem;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(17rem, 1fr));
    gap: 0.9rem;
  }

  .atalho {
    display: flex;
    align-items: flex-start;
    gap: 0.9rem;
    padding: 1rem 1.1rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
    color: inherit;
    text-decoration: none;
    transition: border-color 0.12s;
  }

  .atalho:hover {
    border-color: var(--seno-blue-500);
  }

  .icone {
    flex: none;
    display: grid;
    place-items: center;
    width: 2.6rem;
    height: 2.6rem;
    border-radius: var(--raio-md);
    background: var(--seno-blue-100);
    color: var(--seno-blue-700);
  }

  .texto {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .texto strong {
    color: var(--seno-blue-900);
  }

  .texto span {
    color: var(--seno-gray-500);
    font-size: 0.88rem;
  }
</style>
