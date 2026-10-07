<script lang="ts">
  // Tela de inspeção de usuário: detalhes completos + edição de cargos via
  // checkboxes (o super vê admin; o admin comum vê aluno/professor) e botão
  // de reset de senha. As regras continuam garantidas pela API: ninguém
  // remove o próprio cargo de admin/super; admins não alteram outros
  // admins; o super só é visível a si mesmo.
  import type { Usuario } from "#lib/tipos";
  import { ROTULO_CARGO, type Cargo } from "#lib/tipos";

  interface Props {
    usuario: Usuario;
    usuarioId: number;
    eSuper: boolean;
    form?: { ok?: string; erro?: string } | null;
  }

  let { usuario, usuarioId, eSuper, form }: Props = $props();

  const ROTULO_STATUS: Record<string, string> = {
    active: "Ativo",
    pending: "Pendente",
    disabled: "Bloqueado",
  };

  const superAlvo = $derived(usuario.roles.includes("super"));
  const staffAlvo = $derived(
    usuario.roles.includes("admin") || usuario.roles.includes("super"),
  );
  const alvoÉOPróprio = $derived(usuario.id === usuarioId);

  // Edição de cargos: super altera todos; admin altera não-admins e a si
  // mesmo (mantendo o próprio admin, garantido pela API).
  const podeEditarCargos = $derived(
    eSuper || !staffAlvo || alvoÉOPróprio,
  );

  // O checkbox de admin aparece para o super (atribuição) e fica travado
  // marcado quando o alvo é conta admin/super editada pelo dono.
  const checkboxAdmin = $derived(eSuper || (alvoÉOPróprio && staffAlvo));
  const adminTravado = $derived(superAlvo || (alvoÉOPróprio && staffAlvo));

  let editandoCargos = $state(false);
  let cargoStudent = $state(false);
  let cargoProfessor = $state(false);
  let cargoAdmin = $state(false);

  function comecarEdicaoCargos() {
    cargoStudent = usuario.roles.includes("student");
    cargoProfessor = usuario.roles.includes("professor");
    cargoAdmin = usuario.roles.includes("admin");
    editandoCargos = true;
  }

  function cancelarEdicaoCargos() {
    editandoCargos = false;
  }

  function data(iso: string | undefined | null): string {
    if (!iso) return "—";
    return new Date(iso).toLocaleDateString("pt-BR", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  }
</script>

<section class="detalhe">
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}

  <div class="cabecalho">
    <h2>{usuario.first_name} {usuario.last_name}</h2>
    <span class={`estado ${usuario.status}`}>
      {ROTULO_STATUS[usuario.status] ?? usuario.status}
    </span>
  </div>

  <dl class="ficha">
    <div>
      <dt>E-mail</dt>
      <dd>{usuario.email}</dd>
    </div>
    <div>
      <dt>Identificador</dt>
      <dd>{usuario.id}</dd>
    </div>
    <div>
      <dt>Criado em</dt>
      <dd>{data(usuario.created_at)}</dd>
    </div>
    <div>
      <dt>Atualizado em</dt>
      <dd>{data(usuario.updated_at)}</dd>
    </div>
  </dl>

  <div class="cartao">
    <div class="cabecalho-cargo">
      <h3>Cargos</h3>
      {#if podeEditarCargos && !editandoCargos}
        <button type="button" class="acao" onclick={comecarEdicaoCargos}>
          Alterar cargos
        </button>
      {/if}
    </div>

    {#if !podeEditarCargos}
      <p class="nota">Só o super admin altera cargos de contas de admin.</p>
    {/if}

    {#if editandoCargos}
      <form method="POST" action="?/cargos" class="form-cargos">
        <input type="hidden" name="id" value={usuario.id} />
        {#if checkboxAdmin}
          <label class="opcao">
            <input
              type="checkbox"
              name="cargo_admin"
              bind:checked={cargoAdmin}
              disabled={adminTravado}
            />
            {ROTULO_CARGO.admin}
            {#if adminTravado}
              <em class="trava">(não removível)</em>
            {/if}
          </label>
        {/if}
        <label class="opcao">
          <input
            type="checkbox"
            name="cargo_professor"
            bind:checked={cargoProfessor}
          />
          {ROTULO_CARGO.professor}
        </label>
        <label class="opcao">
          <input
            type="checkbox"
            name="cargo_student"
            bind:checked={cargoStudent}
          />
          {ROTULO_CARGO.student}
        </label>
        <div class="botoes">
          <button>Salvar cargos</button>
          <button type="button" class="acao neutro" onclick={cancelarEdicaoCargos}>
            Cancelar
          </button>
        </div>
      </form>
    {:else}
      <div class="chips">
        {#if usuario.roles.length === 0}
          <span class="nota">Sem cargos atribuídos.</span>
        {/if}
        {#each usuario.roles as cargo (cargo)}
          <em>{ROTULO_CARGO[cargo as Cargo] ?? cargo}</em>
        {/each}
      </div>
    {/if}
  </div>

  <div class="acoes">
    {#if usuario.status !== "pending"}
      {#if (!staffAlvo || eSuper) && usuario.status !== "disabled" && !alvoÉOPróprio}
        <form method="POST" action="?/bloquear" class="inline">
          <input type="hidden" name="id" value={usuario.id} />
          <button class="acao ruim">Bloquear</button>
        </form>
      {/if}
      {#if (!staffAlvo || eSuper) && usuario.status === "disabled"}
        <form method="POST" action="?/desbloquear" class="inline">
          <input type="hidden" name="id" value={usuario.id} />
          <button class="acao">Desbloquear</button>
        </form>
      {/if}
      {#if !staffAlvo || eSuper}
        <form method="POST" action="?/resetar" class="inline">
          <input type="hidden" name="id" value={usuario.id} />
          <button class="acao">Resetar senha</button>
        </form>
      {/if}
    {:else}
      <p class="nota">Pendente: aguardando aceite do convite por e-mail.</p>
    {/if}
  </div>
</section>

<style>
  .detalhe {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
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

  .cabecalho {
    display: flex;
    align-items: center;
    gap: 0.9rem;
  }

  .cabecalho h2 {
    margin: 0;
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

  .ficha {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 1rem 1.2rem;
    margin: 0;
  }

  .ficha dt {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--seno-gray-500);
  }

  .ficha dd {
    margin: 0.15rem 0 0;
    font-size: 0.95rem;
  }

  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 1rem 1.2rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .cabecalho-cargo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .cabecalho-cargo h3 {
    margin: 0;
  }

  .chips {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .chips em {
    font-style: normal;
    font-size: 0.78rem;
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 999px;
    padding: 0.2rem 0.7rem;
  }

  .form-cargos {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }

  .opcao {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.92rem;
  }

  .form-cargos input[type="checkbox"] {
    accent-color: var(--seno-blue-600);
    width: 1rem;
    height: 1rem;
  }

  .form-cargos input[type="checkbox"]:disabled {
    opacity: 0.55;
  }

  .trava {
    font-size: 0.75rem;
    color: var(--seno-gray-500);
  }

  .botoes {
    display: flex;
    gap: 0.5rem;
  }

  .acao {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    cursor: pointer;
  }

  .acao:hover {
    background: var(--seno-blue-700);
  }

  .acao.neutro {
    background: none;
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
  }

  .acao.neutro:hover {
    background: var(--seno-gray-100);
  }

  .acao.ruim {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }

  .acao.ruim:hover {
    background: #fdecea;
  }

  .form-cargos button:not(.neutro) {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    cursor: pointer;
  }

  .form-cargos button:not(.neutro):hover {
    background: var(--seno-blue-700);
  }

  .acoes {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }

  .inline {
    display: inline;
  }

  .nota {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    margin: 0;
  }
</style>
