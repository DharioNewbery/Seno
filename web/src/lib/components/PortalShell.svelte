<script lang="ts">
  // Template único dos portais: barra superior limpa (sem fundo azul) com logo,
  // navegação centralizada (incluindo dropdowns de subitens) e perfil à direita
  // em coluna central única. Em telas estreitas, a navegação abre em gaveta.
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import type { Snippet } from "svelte";

  import Logo from "./Logo.svelte";
  import { iniciaisUsuario } from "#lib/atividades-aluno";
  import type { NavItem, PortalDef } from "#lib/portais";
  import { portaisDisponiveis, type Usuario } from "#lib/tipos";

  let {
    portal,
    usuario,
    children,
  }: { portal: PortalDef; usuario: Usuario; children: Snippet } = $props();

  const rota = $derived(page.url.pathname);
  const outros = $derived(portaisDisponiveis(usuario.roles));
  const podeTrocar = $derived(outros.length > 1);
  const raiz = $derived(portal.nav[0]?.href ?? `/${portal.id}`);
  const sigla = $derived(iniciaisUsuario(usuario.first_name, usuario.last_name));

  /** Itens de navegação visíveis (filtro de super admin aplicado). */
  const nav = $derived(
    portal.nav.filter((item) => !item.soSuper || usuario.roles.includes("super")),
  );

  let subAberto = $state<string | null>(null);
  let perfilAberto = $state(false);
  let menuMovelAberto = $state(false);
  let navRef = $state<HTMLElement | null>(null);
  let perfilRef = $state<HTMLDivElement | null>(null);

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
    if (item.href === `/${portal.id}`) return rota === `/${portal.id}`;
    return dentro(item.href) || subAtivo(item) !== undefined;
  }

  $effect(() => {
    void rota;
    subAberto = null;
    perfilAberto = false;
    menuMovelAberto = false;
  });

  function aoClicarFora(e: Event) {
    const alvo = e.target as Parameters<HTMLDivElement["contains"]>[0];
    if (perfilAberto && perfilRef && alvo && !perfilRef.contains(alvo)) {
      perfilAberto = false;
    }
    if (subAberto && navRef && alvo && !navRef.contains(alvo)) {
      subAberto = null;
    }
  }

  function aoTecla(e: { key: string }) {
    if (e.key === "Escape") {
      subAberto = null;
      perfilAberto = false;
      menuMovelAberto = false;
    }
  }

  async function sair() {
    perfilAberto = false;
    await fetch("/api/logout", { method: "POST" });
    await goto("/");
  }
</script>

<svelte:window onclick={aoClicarFora} onkeydown={aoTecla} />

<a class="pular" href="#conteudo">Ir para o conteúdo</a>

<div class="portal-shell">
  <header class="topbar">
    <div class="topbar-inner">
      <div class="topbar-esq">
        <button
          type="button"
          class="hamburguer"
          onclick={() => (menuMovelAberto = !menuMovelAberto)}
          aria-label={menuMovelAberto ? "Fechar menu" : "Abrir menu"}
          aria-expanded={menuMovelAberto}
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            aria-hidden="true"
          >
            <path
              d="M4 7h16M4 12h16M4 17h16"
              stroke="currentColor"
              stroke-width="1.9"
              stroke-linecap="round"
            />
          </svg>
        </button>

        <a class="marca" href={raiz} aria-label={portal.titulo}>
          <Logo tamanho={32} />
        </a>
      </div>

      <nav
        class="topbar-nav"
        bind:this={navRef}
        aria-label={`Navegação do ${portal.titulo}`}
      >
        {#each nav as item (item.rotulo)}
          {@const atual = ativo(item)}
          {#if item.sub}
            {@const aberto = subAberto === item.rotulo}
            {@const subAtual = subAtivo(item)}
            <div class="nav-grupo">
              <button
                type="button"
                class="nav-btn"
                class:ativo={atual}
                aria-expanded={aberto}
                onclick={() => (subAberto = aberto ? null : item.rotulo)}
              >
                <span>{item.rotulo}</span>
                <svg
                  class="seta"
                  class:girada={aberto}
                  width="12"
                  height="12"
                  viewBox="0 0 12 12"
                  aria-hidden="true"
                >
                  <path
                    d="M2.5 4.5l3.5 3.5 3.5-3.5"
                    stroke="currentColor"
                    stroke-width="1.6"
                    fill="none"
                    stroke-linecap="round"
                  />
                </svg>
              </button>

              {#if aberto}
                <div class="balao nav-dropdown" role="menu">
                  {#each item.sub as subitem (subitem.href)}
                    <a
                      href={subitem.href}
                      class="balao-item"
                      class:ativo-sub={subitem.href === subAtual}
                      role="menuitem"
                      aria-current={subitem.href === subAtual ? "page" : undefined}
                    >
                      {subitem.rotulo}
                    </a>
                  {/each}
                </div>
              {/if}
            </div>
          {:else}
            <a
              href={item.href}
              class="nav-btn"
              class:ativo={atual}
              aria-current={atual ? "page" : undefined}
            >
              {item.rotulo}
            </a>
          {/if}
        {/each}
      </nav>

      <div class="topbar-dir" bind:this={perfilRef}>
        <button
          type="button"
          class="perfil-btn"
          aria-expanded={perfilAberto}
          aria-haspopup="menu"
          onclick={() => (perfilAberto = !perfilAberto)}
        >
          <span class="avatar" aria-hidden="true">{sigla}</span>
          <span class="primeiro-nome">{usuario.first_name}</span>
        </button>

        {#if perfilAberto}
          <div class="balao balao-perfil" role="menu">
            {#if podeTrocar}
              <a class="balao-item" role="menuitem" href="/portais">
                Trocar portal
              </a>
            {/if}
            <button
              type="button"
              class="balao-item sair"
              role="menuitem"
              onclick={sair}
            >
              Sair
            </button>
          </div>
        {/if}
      </div>
    </div>

    {#if menuMovelAberto}
      <nav
        class="menu-movel"
        aria-label={`Navegação móvel do ${portal.titulo}`}
      >
        <div class="menu-movel-inner">
          {#each nav as item (item.rotulo)}
            {@const atual = ativo(item)}
            {#if item.sub}
              {@const subAtual = subAtivo(item)}
              <div class="movel-grupo">
                <span class="movel-titulo" class:ativo={atual}>
                  {item.rotulo}
                </span>
                <div class="movel-sub">
                  {#each item.sub as subitem (subitem.href)}
                    <a
                      href={subitem.href}
                      class="movel-link"
                      class:ativo={subitem.href === subAtual}
                      aria-current={subitem.href === subAtual ? "page" : undefined}
                    >
                      {subitem.rotulo}
                    </a>
                  {/each}
                </div>
              </div>
            {:else}
              <a
                href={item.href}
                class="movel-link"
                class:ativo={atual}
                aria-current={atual ? "page" : undefined}
              >
                {item.rotulo}
              </a>
            {/if}
          {/each}
        </div>
      </nav>
    {/if}
  </header>

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
    color: var(--seno-gray-900);
    border-radius: var(--raio-md);
    box-shadow: var(--sombra-flutuante);
    transform: translateY(-200%);
  }

  .pular:focus {
    transform: none;
  }

  .portal-shell {
    min-height: 100vh;
    min-height: 100dvh;
    display: flex;
    flex-direction: column;
    background: var(--seno-bg);
    overflow-x: clip;
  }

  /* ---- Top bar ---- */
  .topbar {
    position: sticky;
    top: 0;
    z-index: 20;
    background: var(--seno-white);
    border-bottom: 1px solid var(--seno-line);
  }

  .topbar-inner {
    width: 100%;
    max-width: var(--seno-largura-conteudo);
    margin: 0 auto;
    padding: 0.75rem 2rem;
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    gap: 1rem;
  }

  .topbar-esq {
    justify-self: start;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .hamburguer {
    display: none;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    background: transparent;
    border: none;
    border-radius: var(--raio-md);
    color: var(--seno-gray-700);
    cursor: pointer;
  }

  .hamburguer:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
  }

  .marca {
    display: inline-flex;
    align-items: center;
    border-radius: var(--raio-md);
    text-decoration: none;
  }

  /* ---- Navegação centralizada ---- */
  .topbar-nav {
    justify-self: center;
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  .nav-grupo {
    position: relative;
  }

  .nav-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.45rem 0.9rem;
    border: none;
    border-radius: var(--raio-pill);
    background: transparent;
    text-decoration: none;
    font: inherit;
    font-size: 0.95rem;
    font-weight: 500;
    color: var(--seno-gray-700);
    white-space: nowrap;
    cursor: pointer;
    transition: color 0.14s ease;
  }

  .nav-btn:hover {
    background: transparent;
    color: var(--seno-blue-800);
  }

  .nav-btn.ativo {
    background: transparent;
    color: var(--seno-primary);
    font-weight: 600;
  }

  .seta {
    transition: transform 0.16s ease;
  }

  .seta.girada {
    transform: rotate(180deg);
  }

  /* ---- Perfil à direita ---- */
  .topbar-dir {
    justify-self: end;
    position: relative;
  }

  .perfil-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.55rem;
    padding: 0.3rem 0.65rem 0.3rem 0.35rem;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--raio-pill);
    color: var(--seno-gray-900);
    font: inherit;
    font-size: 0.92rem;
    font-weight: 500;
    cursor: pointer;
    transition:
      background-color 0.14s ease,
      border-color 0.14s ease;
  }

  .perfil-btn:hover,
  .perfil-btn[aria-expanded="true"] {
    background: var(--seno-gray-100);
    border-color: var(--seno-line);
  }

  .avatar {
    width: 2rem;
    height: 2rem;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: var(--seno-blue-100);
    color: var(--seno-blue-700);
    font-size: 0.78rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    flex: none;
  }

  .primeiro-nome {
    max-width: 10rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ---- Balões (dropdowns de subitens e perfil) ---- */
  .balao {
    position: absolute;
    top: calc(100% + 0.5rem);
    min-width: 11rem;
    padding: 0.35rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-md);
    box-shadow: var(--sombra-flutuante);
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    z-index: 30;
  }

  .nav-dropdown {
    left: 50%;
    transform: translateX(-50%);
  }

  .balao-perfil {
    right: 0;
  }

  .balao-perfil::before {
    content: "";
    position: absolute;
    top: -5px;
    right: 1rem;
    width: 9px;
    height: 9px;
    background: var(--seno-white);
    border-left: 1px solid var(--seno-line);
    border-top: 1px solid var(--seno-line);
    transform: rotate(45deg);
  }

  .balao-item {
    width: 100%;
    padding: 0.5rem 0.75rem;
    background: none;
    border: none;
    border-radius: var(--raio-sm);
    color: var(--seno-gray-900);
    font: inherit;
    font-size: 0.88rem;
    font-weight: 500;
    text-align: left;
    text-decoration: none;
    cursor: pointer;
    transition: background-color 0.12s ease;
  }

  .balao-item:hover {
    background: var(--seno-gray-100);
  }

  .balao-item.ativo-sub {
    color: var(--seno-primary);
    font-weight: 600;
  }

  .balao-item.sair {
    color: var(--seno-red-700);
  }

  .balao-item.sair:hover {
    background: var(--seno-red-50);
  }

  /* ---- Menu móvel ---- */
  .menu-movel {
    display: none;
    border-top: 1px solid var(--seno-line);
    background: var(--seno-white);
  }

  .menu-movel-inner {
    max-width: var(--seno-largura-conteudo);
    margin: 0 auto;
    padding: 0.75rem 1.25rem 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .movel-grupo {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    padding-top: 0.25rem;
  }

  .movel-titulo {
    padding: 0.3rem 0.5rem;
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--seno-gray-500);
  }

  .movel-titulo.ativo {
    color: var(--seno-primary);
  }

  .movel-sub {
    padding-left: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    border-left: 2px solid var(--seno-line);
  }

  .movel-link {
    padding: 0.45rem 0.65rem;
    border-radius: var(--raio-sm);
    text-decoration: none;
    font-size: 0.92rem;
    font-weight: 500;
    color: var(--seno-gray-700);
  }

  .movel-link:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
  }

  .movel-link.ativo {
    color: var(--seno-primary);
    font-weight: 600;
  }

  /* ---- Conteúdo em coluna central ---- */
  .corpo {
    flex: 1;
    min-width: 0;
  }

  .corpo:focus {
    outline: none;
  }

  .pagina {
    width: 100%;
    max-width: var(--seno-largura-conteudo);
    margin: 0 auto;
    padding: 2rem 2rem 3.5rem;
  }

  @media (max-width: 900px) {
    .topbar-inner {
      grid-template-columns: 1fr auto;
      padding: 0.65rem 1rem;
    }

    .hamburguer {
      display: grid;
    }

    .topbar-nav {
      display: none;
    }

    .menu-movel {
      display: block;
    }

    .pagina {
      padding: 1.25rem 1rem 2.5rem;
    }
  }
</style>
