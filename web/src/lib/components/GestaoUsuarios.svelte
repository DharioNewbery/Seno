<script lang="ts">
  // Tela de gestão de professores/admins: lista com busca e filtro de
  // estado, criação via convite (pendente), edição inline de nome,
  // bloqueio/desbloqueio e reset de senha. Ações via form actions do
  // server (+page.server.ts), preservando os filtros da página.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { Usuario } from "#lib/tipos";
  import { ROTULO_CARGO, type Cargo } from "#lib/tipos";

  interface Props {
    cargo: "professor" | "admin";
    usuarios: Usuario[];
    total: number;
    atual: number;
    por: number;
    busca: string;
    filtroStatus: string;
    erro: string | null;
    form?: { ok?: string; erro?: string } | null;
    eSuper: boolean;
    usuarioId: number;
  }

  let {
    cargo,
    usuarios,
    total,
    atual,
    por,
    busca,
    filtroStatus,
    erro,
    form,
    eSuper,
    usuarioId,
  }: Props = $props();

  let editandoId = $state<number | null>(null);
  let nomeEdicao = $state("");
  let sobrenomeEdicao = $state("");

  const ROTULO_STATUS: Record<string, string> = {
    active: "Ativo",
    pending: "Pendente",
    disabled: "Bloqueado",
  };

  /** URL da ação, preservando busca/status/página da listagem atual. */
  function urlAcao(rotulo: string): string {
    const filtro = new SvelteURLSearchParams(page.url.searchParams.toString());
    const extra = filtro.size ? `&${filtro.toString()}` : "";
    return `?/${rotulo}${extra}`;
  }

  function comecarEdicao(u: Usuario) {
    editandoId = u.id;
    nomeEdicao = u.first_name;
    sobrenomeEdicao = u.last_name;
  }

  function cancelarEdicao() {
    editandoId = null;
    nomeEdicao = "";
    sobrenomeEdicao = "";
  }

  function proximaPagina(delta: number): string {
    const filtro = new SvelteURLSearchParams(page.url.searchParams.toString());
    filtro.set("pagina", String(atual + delta));
    return `?${filtro.toString()}`;
  }
</script>

<section class="gestao">
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if erro}
    <p class="aviso erro" role="alert">{erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}

  <div class="cabecalho">
    <h2>{cargo === "professor" ? "Professores" : "Contas de admin"}</h2>
    <span class="total">{total} conta(s)</span>
  </div>

  <form method="get" class="busca">
    <input
      name="busca"
      value={busca}
      placeholder="Buscar por e-mail ou nome"
      aria-label="Buscar"
    />
    <select name="status" aria-label="Filtrar por estado">
      <option value="">Todos os estados</option>
      <option value="active" selected={filtroStatus === "active"}>Ativos</option>
      <option value="pending" selected={filtroStatus === "pending"}>
        Pendentes
      </option>
      <option value="disabled" selected={filtroStatus === "disabled"}>
        Bloqueados
      </option>
    </select>
    <button>Buscar</button>
  </form>

  <form method="POST" action={urlAcao("criar")} class="criar">
    <input name="first_name" placeholder="Nome" required />
    <input name="last_name" placeholder="Sobrenome" required />
    <input name="email" type="email" placeholder="E-mail" required />
    <button>{cargo === "professor" ? "Convidar professor" : "Criar admin"}</button>
  </form>
  {#if cargo === "admin" && !eSuper}
    <p class="nota">
      Só o super admin cria contas de admin (ARQUITETURA §5.4).
    </p>
  {/if}

  <ul class="lista">
    {#each usuarios as u (u.id)}
      <li>
        <div class="identidade">
          <strong>{u.first_name} {u.last_name}</strong>
          <span class="email">{u.email}</span>
          <span class="chips">
            {#each u.roles as cargo1 (cargo1)}
              <em>{ROTULO_CARGO[cargo1 as Cargo] ?? cargo1}</em>
            {/each}
          </span>
        </div>

        {#if editandoId === u.id}
          <form method="POST" action={urlAcao("editar")} class="edicao">
            <input type="hidden" name="id" value={u.id} />
            <input name="first_name" bind:value={nomeEdicao} required />
            <input name="last_name" bind:value={sobrenomeEdicao} required />
            <button>Salvar</button>
            <button type="button" onclick={cancelarEdicao}>Cancelar</button>
          </form>
        {:else}
          <div class="acoes">
            <span class={`estado ${u.status}`}>
              {ROTULO_STATUS[u.status] ?? u.status}
            </span>
            <button class="acao" onclick={() => comecarEdicao(u)}>Editar</button>
            {#if u.status !== "disabled" && u.id !== usuarioId && !u.roles.includes("super")}
              <form method="POST" action={urlAcao("bloquear")}>
                <input type="hidden" name="id" value={u.id} />
                <button class="acao ruim">Bloquear</button>
              </form>
            {/if}
            {#if u.status === "disabled"}
              <form method="POST" action={urlAcao("desbloquear")}>
                <input type="hidden" name="id" value={u.id} />
                <button class="acao">Desbloquear</button>
              </form>
            {/if}
            {#if u.status !== "pending"}
              <form method="POST" action={urlAcao("resetar")}>
                <input type="hidden" name="id" value={u.id} />
                <button class="acao">Resetar senha</button>
              </form>
            {/if}
          </div>
        {/if}
      </li>
    {:else}
      <li class="vazio">Nenhuma conta com estes filtros.</li>
    {/each}
  </ul>

  <div class="paginacao">
    <a
      class="botao-pag"
      href={proximaPagina(-1)}
      class:desabilitado={atual <= 1}
      aria-label="Página anterior"
    >
      ← Anterior
    </a>
    <span>Página {atual}</span>
    <a
      class="botao-pag"
      href={proximaPagina(1)}
      class:desabilitado={atual * por >= total}
      aria-label="Próxima página"
    >
      Próxima →
    </a>
  </div>
</section>

<style>
  .gestao {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .cabecalho {
    display: flex;
    align-items: baseline;
    gap: 1rem;
  }

  .cabecalho h2 {
    margin: 0;
  }

  .total {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }

  .aviso {
    padding: 0.6rem 0.9rem;
    border-radius: 8px;
    font-size: 0.9rem;
  }

  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
  }

  .aviso.ok {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }

  .busca,
  .criar {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .busca input,
  .busca select,
  .criar input,
  .edicao input {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.65rem;
    background: var(--seno-white);
    font-size: 0.9rem;
  }

  .busca button,
  .criar button,
  .edicao button,
  .acao {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    cursor: pointer;
  }

  .busca button:hover,
  .criar button:hover,
  .edicao button:hover,
  .acao:hover {
    background: var(--seno-blue-700);
  }

  .acao.ruin {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }

  .acao.ruin:hover {
    background: #fdecea;
  }

  .lista {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }

  .lista li {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 0.9rem 1.1rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .identidade .email {
    color: var(--seno-gray-700);
    font-size: 0.88rem;
  }

  .chips {
    display: flex;
    gap: 0.35rem;
  }

  .chips em {
    font-style: normal;
    font-size: 0.72rem;
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 999px;
    padding: 0.15rem 0.55rem;
  }

  .acoes {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
  }

  .acoes form {
    display: contents;
  }

  .estado {
    font-size: 0.75rem;
    border-radius: 999px;
    padding: 0.2rem 0.6rem;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }

  .estado.active {
    background: #e6f4ea;
    color: var(--seno-green);
  }

  .estado.pending {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }

  .estado.disabled {
    background: #fdecea;
    color: var(--seno-red);
  }

  .edicao {
    display: flex;
    gap: 0.4rem;
    align-items: center;
    flex-wrap: wrap;
  }

  .paginacao {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }

  .botao-pag {
    color: var(--seno-blue-600);
    text-decoration: none;
  }

  .botao-pag.desabilitado {
    color: var(--seno-gray-300);
    pointer-events: none;
  }

  .nota {
    color: var(--seno-gray-500);
    font-size: 0.8rem;
    margin: -0.5rem 0 0;
  }
</style>
