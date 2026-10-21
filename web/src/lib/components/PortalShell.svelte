<script lang="ts">
  // Template único dos portais: barra lateral com navegação (grupos com
  // subopções), cartão do usuário e versões no rodapé da barra. Em telas
  // estreitas a barra vira uma gaveta aberta por um botão no topo.
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import type { Snippet } from "svelte";

  import Logo from "./Logo.svelte";
  import NavIcon from "./NavIcon.svelte";
  import { APP_VERSION, API_VERSION, modoDev, NOME_SISTEMA } from "#lib/config";
  import { iconeDoItem, type NavItem, type PortalDef } from "#lib/portais";
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
  const raiz = $derived(`/${portal.id}`);

  /** Itens de navegação visíveis (filtro de super admin aplicado). */
  const nav = $derived(
    portal.nav.filter((item) => !item.soSuper || usuario.roles.includes("super")),
  );

  // Grupos: abertos por padrão quando a rota atual está dentro deles; o clique
  // do usuário sempre prevalece sobre esse padrão.
  let abertos = $state<Record<string, boolean>>({});
  let menuAberto = $state(false);

  function dentro(href: string): boolean {
    return rota === href || rota.startsWith(`${href}/`);
  }

  /** Subitem que melhor casa com a rota (maior prefixo), para realçar em páginas de detalhe. */
  function subAtivo(item: NavItem): string | undefined {
    const casam = (item.sub ?? []).filter((s) => dentro(s.href));
    casam.sort((a, b) => b.href.length - a.href.length);
    return casam[0]?.href;
  }

  function ativo(item: NavItem): boolean {
    if (item.href === raiz) return rota === raiz;
    return dentro(item.href) || subAtivo(item) !== undefined;
  }

  function aberto(item: NavItem): boolean {
    return abertos[item.rotulo] ?? ativo(item);
  }

  function alternar(item: NavItem) {
    abertos = { ...abertos, [item.rotulo]: !aberto(item) };
  }

  // Ao navegar, a gaveta (mobile) se fecha.
  $effect(() => {
    void rota;
    menuAberto = false;
  });

  async function sair() {
    await fetch("/api/logout", { method: "POST" });
    await goto("/");
  }
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && (menuAberto = false)} />

<a class="pular" href="#conteudo">Ir para o conteúdo</a>

<div class="portal" class:menu-aberto={menuAberto}>
  <header class="topo-movel">
    <button
      class="hamburguer"
      onclick={() => (menuAberto = !menuAberto)}
      aria-label={menuAberto ? "Fechar menu" : "Abrir menu"}
      aria-expanded={menuAberto}
      aria-controls="menu-lateral"
    >
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path d="M4 7h16M4 12h16M4 17h16" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      </svg>
    </button>
    <a class="marca-movel" href={raiz}>
      <Logo tamanho={26} />
      <span>{NOME_SISTEMA}</span>
    </a>
    <span class="portal-movel">{portal.titulo}</span>
  </header>

  <button
    class="veu"
    tabindex="-1"
    aria-label="Fechar menu"
    onclick={() => (menuAberto = false)}
  ></button>

  <aside class="barra" id="menu-lateral">
    <a class="marca" href={raiz} title={`Início de ${portal.titulo}`}>
      <Logo tamanho={32} />
      <span class="textos">
        <span class="nome">{NOME_SISTEMA}</span>
        <span class="portal-nome">{portal.titulo}</span>
      </span>
    </a>

    <nav aria-label={`Navegação do ${portal.titulo}`}>
      <ul>
        {#each nav as item (item.rotulo)}
          {@const icone = iconeDoItem(portal.id, item.href)}
          {@const atual = ativo(item)}
          <li>
            {#if item.sub}
              <button
                class:ativo={atual}
                class:aberto={aberto(item)}
                onclick={() => alternar(item)}
                aria-expanded={aberto(item)}
                aria-controls={`sub-${icone}`}
              >
                <NavIcon nome={icone} />
                <span class="rotulo">{item.rotulo}</span>
                <svg class="seta" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
                  <path d="M2 4l4 4 4-4" stroke="currentColor" stroke-width="1.6" fill="none" stroke-linecap="round" />
                </svg>
              </button>
              {#if aberto(item)}
                {@const subAtual = subAtivo(item)}
                <ul class="sub" id={`sub-${icone}`}>
                  {#each item.sub as subitem (subitem.href)}
                    <li>
                      <a
                        href={subitem.href}
                        class:ativo={subitem.href === subAtual}
                        aria-current={subitem.href === subAtual ? "page" : undefined}
                      >
                        {subitem.rotulo}
                      </a>
                    </li>
                  {/each}
                </ul>
              {/if}
            {:else}
              <a
                href={item.href}
                class:ativo={atual}
                aria-current={atual ? "page" : undefined}
              >
                <NavIcon nome={icone} />
                <span class="rotulo">{item.rotulo}</span>
              </a>
            {/if}
          </li>
        {/each}
      </ul>
    </nav>

    <div class="rodape-barra">
      <div class="usuario" title={usuario.email}>
        <span class="avatar" aria-hidden="true">{usuario.first_name.charAt(0).toUpperCase()}</span>
        <span class="quem">
          <strong>{usuario.first_name} {usuario.last_name}</strong>
          <small>{usuario.roles.map((c) => ROTULO_CARGO[c]).join(", ")}</small>
        </span>
      </div>
      <div class="acoes">
        {#if podeTrocar}
          <a class="acao" href="/portais" title="Voltar ao seletor de portal">Trocar portal</a>
        {/if}
        <button class="acao" onclick={sair} title="Encerrar sessão">Sair</button>
      </div>
      <p class="versao">
        <span>{NOME_SISTEMA} v{APP_VERSION} · API v{API_VERSION}</span>
        {#if modoDev}
          <span class="dev" title="Build de desenvolvimento">DEV</span>
        {/if}
      </p>
    </div>
  </aside>

  <main class="corpo" id="conteudo" tabindex="-1">
    <div class="pagina">
      {@render children()}
    </div>
  </main>
</div>

<style>
  .pular {
    position: fixed;
    top: 0.5rem;
    left: 0.5rem;
    z-index: 100;
    padding: 0.5rem 0.9rem;
    background: var(--seno-white);
    color: var(--seno-blue-900);
    border-radius: var(--raio-md);
    box-shadow: var(--sombra-flutuante);
    transform: translateY(-200%);
  }

  .pular:focus {
    transform: none;
  }

  .portal {
    display: grid;
    grid-template-columns: 16.5rem minmax(0, 1fr);
    height: 100vh;
    height: 100dvh;
    background: var(--seno-bg);
  }

  /* ---- Barra lateral ---- */
  .barra {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--seno-blue-950);
    color: var(--seno-blue-200);
  }

  .marca {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    padding: 1.15rem 1.1rem 1rem;
    text-decoration: none;
  }

  .textos {
    display: flex;
    flex-direction: column;
    line-height: 1.2;
  }

  .nome {
    color: var(--seno-white);
    font-weight: 600;
    font-size: 1.15rem;
    letter-spacing: -0.01em;
  }

  .portal-nome {
    color: var(--seno-blue-300);
    font-size: 0.78rem;
  }

  nav {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0.35rem 0.75rem 1rem;
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
    position: relative;
    width: 100%;
    display: flex;
    align-items: center;
    gap: 0.7rem;
    background: none;
    border: none;
    color: var(--seno-blue-200);
    text-align: left;
    text-decoration: none;
    font: inherit;
    font-size: 0.93rem;
    font-weight: 500;
    padding: 0.55rem 0.7rem;
    border-radius: var(--raio-md);
    cursor: pointer;
    transition:
      background-color 0.12s,
      color 0.12s;
  }

  nav a:hover,
  nav button:hover {
    background: rgb(255 255 255 / 0.07);
    color: var(--seno-white);
  }

  nav a.ativo,
  nav button.ativo {
    background: rgb(255 255 255 / 0.12);
    color: var(--seno-white);
  }

  nav :global(svg:first-child) {
    flex: none;
    opacity: 0.85;
  }

  .rotulo {
    flex: 1;
    min-width: 0;
  }

  .seta {
    flex: none;
    transition: transform 0.15s;
  }

  button.aberto .seta {
    transform: rotate(180deg);
  }

  /* Subitens: uma linha-guia à esquerda; o item atual a sobrepõe. */
  nav .sub {
    margin: 0.15rem 0 0.35rem 1.4rem;
    padding-left: 0.7rem;
    gap: 0;
    border-left: 1px solid rgb(255 255 255 / 0.16);
  }

  .sub a {
    padding: 0.4rem 0.65rem;
    font-size: 0.88rem;
    font-weight: 400;
  }

  .sub a.ativo {
    background: none;
    font-weight: 600;
  }

  .sub a.ativo::before {
    content: "";
    position: absolute;
    left: calc(-0.7rem - 1px);
    top: 0.3rem;
    bottom: 0.3rem;
    width: 2px;
    border-radius: 2px;
    background: var(--seno-blue-300);
  }

  /* ---- Rodapé da barra: usuário, ações, versão ---- */
  .rodape-barra {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    padding: 0.9rem 1rem 1rem;
    border-top: 1px solid rgb(255 255 255 / 0.1);
  }

  .usuario {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    min-width: 0;
  }

  .avatar {
    flex: none;
    width: 2.2rem;
    height: 2.2rem;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    font-weight: 600;
  }

  .quem {
    min-width: 0;
    display: flex;
    flex-direction: column;
    line-height: 1.25;
  }

  .quem strong {
    color: var(--seno-white);
    font-size: 0.9rem;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .quem small {
    color: var(--seno-blue-300);
    font-size: 0.78rem;
  }

  .acoes {
    display: flex;
    gap: 0.5rem;
  }

  .acao {
    flex: 1;
    padding: 0.42rem 0.5rem;
    background: none;
    border: 1px solid rgb(255 255 255 / 0.2);
    border-radius: var(--raio-md);
    color: var(--seno-blue-100);
    font: inherit;
    font-size: 0.84rem;
    text-align: center;
    text-decoration: none;
    cursor: pointer;
    transition: background-color 0.12s;
  }

  .acao:hover {
    background: rgb(255 255 255 / 0.1);
    color: var(--seno-white);
  }

  .versao {
    margin: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    color: var(--seno-blue-300);
    font-size: 0.72rem;
  }

  .dev {
    padding: 0.05rem 0.4rem;
    border: 1px solid var(--seno-blue-400);
    border-radius: var(--raio-sm);
    color: var(--seno-blue-100);
    font-weight: 600;
    letter-spacing: 0.04em;
  }

  /* ---- Conteúdo ---- */
  .corpo {
    min-width: 0;
    overflow-y: auto;
  }

  .corpo:focus {
    outline: none;
  }

  .pagina {
    max-width: 74rem;
    margin: 0 auto;
    padding: 2rem 2rem 3rem;
  }

  /* ---- Barra superior e gaveta (telas estreitas) ---- */
  .topo-movel,
  .veu {
    display: none;
  }

  @media (max-width: 900px) {
    .portal {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: auto minmax(0, 1fr);
    }

    .topo-movel {
      display: flex;
      align-items: center;
      gap: 0.6rem;
      padding: 0.5rem 0.75rem;
      background: var(--seno-blue-950);
      color: var(--seno-white);
    }

    .hamburguer {
      display: grid;
      place-items: center;
      width: 2.5rem;
      height: 2.5rem;
      background: none;
      border: none;
      border-radius: var(--raio-md);
      color: inherit;
      cursor: pointer;
    }

    .hamburguer:hover {
      background: rgb(255 255 255 / 0.1);
    }

    .marca-movel {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      color: var(--seno-white);
      font-weight: 600;
      text-decoration: none;
    }

    .portal-movel {
      margin-left: auto;
      color: var(--seno-blue-200);
      font-size: 0.82rem;
    }

    .barra {
      position: fixed;
      inset: 0 auto 0 0;
      z-index: 30;
      width: min(18rem, 86vw);
      visibility: hidden;
      transform: translateX(-100%);
      transition:
        transform 0.2s ease,
        visibility 0s linear 0.2s;
    }

    .menu-aberto .barra {
      visibility: visible;
      transform: none;
      box-shadow: var(--sombra-flutuante);
      transition-delay: 0s;
    }

    .menu-aberto .veu {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 20;
      border: none;
      background: rgb(8 42 90 / 0.5);
    }

    .pagina {
      padding: 1.25rem 1rem 2.5rem;
    }
  }
</style>
