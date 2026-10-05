<script lang="ts">
  import { goto } from "$app/navigation";
  import Logo from "#lib/components/Logo.svelte";
  import { NOME_SISTEMA } from "#lib/config";
  import { PORTAIS } from "#lib/portais";
  import {
    cargosDoPortal,
    portaisDisponiveis,
    ROTULO_CARGO,
  } from "#lib/tipos";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const usuario = $derived(data.usuario);
  const cartoes = $derived(
    portaisDisponiveis(usuario.roles).map((id) => ({
      def: PORTAIS[id],
      cargos: cargosDoPortal(usuario.roles, id),
    })),
  );

  async function sair() {
    await fetch("/api/logout", { method: "POST" });
    await goto("/");
  }
</script>

<main class="seletor">
  <header class="cabecalho">
    <div class="marca">
      <Logo tamanho={32} />
      <span class="nome">{NOME_SISTEMA}</span>
    </div>
    <button onclick={sair}>Sair</button>
  </header>

  <section class="corpo">
    <h1>Olá, {usuario.first_name}</h1>
    <p>Escolha o portal para acessar:</p>
    <div class="cartoes">
      {#each cartoes as { def, cargos } (def.id)}
        <a class="cartao" href={`/${def.id}`}>
          <span class="titulo">{def.titulo}</span>
          <span class="descricao">{def.descricao}</span>
          <span class="cargos">
            {#each cargos as cargo (cargo)}
              <em>{ROTULO_CARGO[cargo]}</em>
            {/each}
          </span>
        </a>
      {/each}
    </div>
  </section>
</main>

<style>
  .seletor {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }

  .cabecalho {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 2rem;
    border-bottom: 1px solid var(--seno-gray-300);
  }

  .marca {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .nome {
    font-weight: 700;
    color: var(--seno-blue-900);
    font-size: 1.1rem;
  }

  .cabecalho button {
    background: none;
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.4rem 0.9rem;
    cursor: pointer;
    color: var(--seno-gray-700);
  }

  .cabecalho button:hover {
    border-color: var(--seno-blue-500);
    color: var(--seno-blue-700);
  }

  .corpo {
    flex: 1;
    max-width: 960px;
    width: 100%;
    margin: 0 auto;
    padding: 2rem;
  }

  .cartoes {
    margin-top: 1.5rem;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 1rem;
  }

  .cartao {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 1.25rem;
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    text-decoration: none;
    color: inherit;
    background: var(--seno-white);
    transition:
      border-color 0.15s,
      box-shadow 0.15s;
  }

  .cartao:hover {
    border-color: var(--seno-blue-500);
    box-shadow: 0 4px 14px rgba(30, 108, 210, 0.12);
  }

  .titulo {
    font-weight: 700;
    color: var(--seno-blue-900);
    font-size: 1.1rem;
  }

  .descricao {
    color: var(--seno-gray-700);
    font-size: 0.92rem;
  }

  .cargos {
    display: flex;
    gap: 0.35rem;
    margin-top: 0.5rem;
  }

  .cargos em {
    font-style: normal;
    font-size: 0.75rem;
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 999px;
    padding: 0.2rem 0.6rem;
  }
</style>
