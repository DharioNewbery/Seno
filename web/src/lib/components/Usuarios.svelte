<script lang="ts">
  // Tela de listagem de usuários: filtros de busca (nome/e-mail), estado e
  // cargo; o próprio perfil aparece em evidência, como primeiro item. Ações
  // via form actions do server (+page.server.ts), preservando os filtros.
  // A gestão de cargos/bloqueio/reseta-senha vive na inspeção: clique no
  // usuário para navegar até ela.
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { Usuario } from "#lib/tipos";
  import { ROTULO_CARGO, type Cargo } from "#lib/tipos";

  interface Props {
    usuarios: Usuario[];
    total: number;
    atual: number;
    por: number;
    busca: string;
    filtroStatus: string;
    filtroCargo: string;
    erro: string | null;
    form?: { ok?: string; erro?: string } | null;
    usuarioId: number;
  }

  let {
    usuarios,
    total,
    atual,
    por,
    busca,
    filtroStatus,
    filtroCargo,
    erro,
    form,
    usuarioId,
  }: Props = $props();

  const ROTULO_STATUS: Record<string, string> = {
    active: "Ativo",
    pending: "Pendente",
    disabled: "Bloqueado",
  };

  // Próprio perfil em evidência, primeiro item da lista.
  const ordenados = $derived([
    ...usuarios.filter((u) => u.id === usuarioId),
    ...usuarios.filter((u) => u.id !== usuarioId),
  ]);

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
    <h2>Usuários</h2>
    <span class="total">{total} conta(s)</span>
    <a class="ir-criar" href="/admin/usuarios/novo">Criar usuário →</a>
  </div>

  <form method="get" class="busca">
    <input
      name="busca"
      value={busca}
      placeholder="Buscar por nome ou e-mail"
      aria-label="Buscar"
    />
    <select name="status" aria-label="Filtrar por estado">
      <option value="">Todos os estados</option>
      <option value="active" selected={filtroStatus === "active"}>
        Ativos
      </option>
      <option value="pending" selected={filtroStatus === "pending"}>
        Pendentes
      </option>
      <option value="disabled" selected={filtroStatus === "disabled"}>
        Bloqueados
      </option>
    </select>
    <select name="cargo" aria-label="Filtrar por cargo">
      <option value="">Todos os cargos</option>
      <option value="student" selected={filtroCargo === "student"}>
        Aluno
      </option>
      <option value="professor" selected={filtroCargo === "professor"}>
        Professor
      </option>
      <option value="admin" selected={filtroCargo === "admin"}>
        Admin
      </option>
    </select>
    <button>Buscar</button>
  </form>

  <ul class="lista">
    {#each ordenados as u (u.id)}
      <li class:destaque={u.id === usuarioId}>
        <a
          class="card"
          href={`/admin/usuarios/${u.id}`}
          aria-label={`Inspecionar ${u.first_name} ${u.last_name}`}
        >
          <div class="identidade">
            <strong>
              {u.first_name} {u.last_name}
              {#if u.id === usuarioId}
                <em class="vc">(você)</em>
              {/if}
            </strong>
            <span class="email">{u.email}</span>
            <span class="chips">
              {#each u.roles as cargo (cargo)}
                <em>{ROTULO_CARGO[cargo as Cargo] ?? cargo}</em>
              {/each}
            </span>
          </div>
          <span class={`estado ${u.status}`}>
            {ROTULO_STATUS[u.status] ?? u.status}
          </span>
        </a>
      </li>
    {:else}
      <li class="vazio">Nenhum usuário com estes filtros.</li>
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
    align-items: center;
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

  .busca {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .busca input,
  .busca select {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.65rem;
    background: var(--seno-white);
    font-size: 0.9rem;
  }

  .busca button,
  .acao {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    cursor: pointer;
  }

  .busca button:hover {
    background: var(--seno-blue-700);
  }

  .ir-criar {
    margin-left: auto;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    text-decoration: none;
  }

  .ir-criar:hover {
    background: var(--seno-blue-700);
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
  }

  .lista li.destaque {
    border: 2px solid var(--seno-blue-500);
    background: var(--seno-blue-100);
  }

  .card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
    padding: 0.9rem 1.1rem;
    text-decoration: none;
    color: inherit;
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

  .vc {
    font-style: normal;
    font-size: 0.75rem;
    color: var(--seno-blue-700);
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

  .vazio {
    color: var(--seno-gray-500);
    text-align: center;
    padding: 1rem;
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
</style>
