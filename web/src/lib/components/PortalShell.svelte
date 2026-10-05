<script lang="ts">
  // Template único dos portais: sidebar com navegação principal e
  // subopções em dropdown, topbar (voltar ao seletor, home, card do
  // usuário) e bottom bar com versões e indicador de modo DEV.
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import type { Snippet } from "svelte";

  import Logo from "./Logo.svelte";
  import { APP_VERSION, API_VERSION, modoDev, NOME_SISTEMA } from "#lib/config";
  import type { PortalDef } from "#lib/portais";
  import {
    portaisDisponiveis,
    ROTULO_CARGO,
    type Usuario,
  } from "#lib/tipos";

  let {
    portal,
    usuario,
    children,
  }: { portal: PortalDef; usuario: Usuario; children: Snippet } = $props();

  const rota = $derived(page.url.pathname);
  const outros = $derived(portaisDisponiveis(usuario.roles));
  const podeTrocar = $derived(outros.length > 1);

  /** Itens de navegação visíveis (filtro de super admin aplicado). */
  const nav = $derived(
    portal.nav.filter((item) => !item.soSuper || usuario.roles.includes("super")),
  );

  let abertos = $state<Record<string, boolean>>({});

  function alternar(rotulo: string) {
    abertos = { ...abertos, [rotulo]: !abertos[rotulo] };
  }

  function ativa(item: { href: string; sub?: { href: string }[] }): boolean {
    if (rota === item.href) return true;
    return item.sub?.some((s) => rota.startsWith(s.href)) ?? false;
  }

  async function sair() {
    await fetch("/api/logout", { method: "POST" });
    await goto("/");
  }
</script>

<div class="portal">
  <aside class="barra">
    <a class="marca" href={`/${portal.id}`} title={`Início de ${portal.titulo}`}>
      <Logo tamanho={30} />
      <span class="nome">{NOME_SISTEMA}</span>
    </a>
    <nav aria-label={`Navegação do ${portal.titulo}`}>
      <ul>
        {#each nav as item (item.rotulo)}
          {@const grupo = Boolean(item.sub)}
          {@const ativo = ativa(item)}
          <li>
            {#if grupo}
              <button
                class:ativo
                class:aberto={abertos[item.rotulo]}
                onclick={() => alternar(item.rotulo)}
                aria-expanded={Boolean(abertos[item.rotulo])}
              >
                {item.rotulo}
                <svg
                  class="seta"
                  width="12"
                  height="12"
                  viewBox="0 0 12 12"
                  aria-hidden="true"
                >
                  <path d="M2 4l4 4 4-4" stroke="currentColor" stroke-width="1.6" fill="none" stroke-linecap="round" />
                </svg>
              </button>
              {#if abertos[item.rotulo] || ativo}
                <ul class="sub">
                  {#each item.sub as subitem (subitem.href)}
                    <li>
                      <a
                        href={subitem.href}
                        class:ativo={rota === subitem.href}
                        onclick={() => alternar(item.rotulo)}
                      >
                        {subitem.rotulo}
                      </a>
                    </li>
                  {/each}
                </ul>
              {/if}
            {:else}
              <a href={item.href} class:ativo>{item.rotulo}</a>
            {/if}
          </li>
        {/each}
      </ul>
    </nav>
  </aside>

  <div class="conteudo">
    <header class="topbar">
      <h1 class="titulo">{portal.titulo}</h1>
      <div class="acoes">
        {#if podeTrocar}
          <a class="botao" href="/portais" title="Voltar ao seletor de portal">
            Trocar portal
          </a>
        {/if}
        <a
          class="botao"
          href={`/${portal.id}`}
          class:atual={rota === `/${portal.id}`}
          title="Voltar à página inicial"
        >
          Início
        </a>
        <div class="usuario" title={usuario.email}>
          <span class="avatar">{usuario.first_name.charAt(0).toUpperCase()}</span>
          <span class="quem">
            <strong>{usuario.first_name} {usuario.last_name}</strong>
            <small>
              {usuario.roles.map((c) => ROTULO_CARGO[c]).join(", ")}
            </small>
          </span>
          <button class="sair" onclick={sair} title="Encerrar sessão">Sair</button>
        </div>
      </div>
    </header>

    <main class="corpo">
      {@render children()}
    </main>

    <footer class="rodape">
      <span>{NOME_SISTEMA} v{APP_VERSION} · api v{API_VERSION}</span>
      <span class="direita">
        <span>UFPA — sistema local</span>
        {#if modoDev}
          <span class="dev" title="Build de desenvolvimento">DEV</span>
        {/if}
      </span>
    </footer>
  </div>
</div>

<style>
  .portal {
    display: grid;
    grid-template-columns: 250px 1fr;
    height: 100vh;
  }

  .barra {
    background: var(--seno-blue-900);
    color: var(--seno-blue-100);
    display: flex;
    flex-direction: column;
    padding: 1rem 0.75rem;
    gap: 1rem;
    overflow-y: auto;
  }

  .marca {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.25rem 0.5rem;
    text-decoration: none;
  }

  .nome {
    color: var(--seno-white);
    font-weight: 700;
    font-size: 1.15rem;
  }

  nav ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  nav a,
  nav button {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    background: none;
    border: none;
    color: var(--seno-blue-100);
    text-align: left;
    text-decoration: none;
    font-size: 0.95rem;
    padding: 0.55rem 0.65rem;
    border-radius: 8px;
    cursor: pointer;
  }

  nav a:hover,
  nav button:hover {
    background: var(--seno-blue-800);
  }

  nav a.ativo,
  nav button.ativo {
    background: var(--seno-blue-700);
    color: var(--seno-white);
    font-weight: 600;
  }

  .seta {
    transition: transform 0.15s;
  }

  button.aberto .seta {
    transform: rotate(180deg);
  }

  .sub {
    padding-left: 0.9rem !important;
  }

  .sub a {
    font-size: 0.87rem;
    color: var(--seno-blue-200);
  }

  .conteudo {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
  }

  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.75rem 1.5rem;
    border-bottom: 1px solid var(--seno-gray-300);
    background: var(--seno-white);
  }

  .titulo {
    margin: 0;
    font-size: 1.1rem;
  }

  .acoes {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .botao {
    color: var(--seno-gray-700);
    text-decoration: none;
    font-size: 0.88rem;
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.4rem 0.75rem;
  }

  .botao:hover,
  .botao.atual {
    border-color: var(--seno-blue-500);
    color: var(--seno-blue-700);
  }

  .usuario {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.35rem 0.6rem;
    border: 1px solid var(--seno-gray-300);
    border-radius: 999px;
  }

  .avatar {
    width: 30px;
    height: 30px;
    border-radius: 50%;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    display: grid;
    place-items: center;
    font-weight: 700;
  }

  .quem {
    display: flex;
    flex-direction: column;
    line-height: 1.15;
  }

  .quem strong {
    font-size: 0.85rem;
    color: var(--seno-gray-900);
  }

  .quem small {
    color: var(--seno-gray-500);
    font-size: 0.72rem;
  }

  .sair {
    background: none;
    border: none;
    color: var(--seno-red);
    font-size: 0.82rem;
    cursor: pointer;
    padding: 0.2rem 0.3rem;
  }

  .corpo {
    flex: 1;
    padding: 1.75rem;
    overflow-y: auto;
    background: var(--seno-gray-100);
  }

  .rodape {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.5rem 1.5rem;
    border-top: 1px solid var(--seno-gray-300);
    color: var(--seno-gray-500);
    font-size: 0.75rem;
    background: var(--seno-white);
  }

  .direita {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .dev {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border: 1px solid var(--seno-blue-300);
    border-radius: 6px;
    padding: 0.1rem 0.45rem;
    font-weight: 700;
    letter-spacing: 0.05em;
  }

  @media (max-width: 900px) {
    .portal {
      grid-template-columns: 1fr;
    }

    .barra {
      flex-direction: row;
      align-items: center;
    }

    nav ul {
      flex-direction: row;
      flex-wrap: wrap;
    }

    .sub {
      flex-direction: column;
    }
  }
</style>
