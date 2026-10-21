<script lang="ts">
  import { goto } from "$app/navigation";
  import TelaAcesso from "#lib/components/TelaAcesso.svelte";
  import { portalPorCargo, type Cargo } from "#lib/tipos";

  let email = $state("");
  let senha = $state("");
  let mostrar = $state(false);
  let enviando = $state(false);
  let erroMensagem = $state("");

  async function entrar(evento: SubmitEvent) {
    evento.preventDefault();
    enviando = true;
    erroMensagem = "";
    try {
      const res = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, senha }),
      });
      if (res.ok) {
        const { cargos } = (await res.json()) as { cargos: Cargo[] };
        const destino =
          cargos.length > 1
            ? "/portais"
            : `/${portalPorCargo(cargos[0] ?? "student")}`;
        await goto(destino);
        return;
      }
      const corpo = (await res.json().catch(() => null)) as {
        erro?: { mensagem?: string };
      } | null;
      erroMensagem = corpo?.erro?.mensagem ?? "Não foi possível entrar.";
    } catch {
      erroMensagem = "Sem comunicação com o servidor. Tente novamente.";
    } finally {
      enviando = false;
    }
  }
</script>

<svelte:head><title>Entrar — Seno</title></svelte:head>

<TelaAcesso destaques>
  <form class="formulario" onsubmit={entrar}>
    <h2>Entrar</h2>
    <label>
      <span>E-mail</span>
      <input
        type="email"
        bind:value={email}
        autocomplete="email"
        required
        placeholder="voce@ufpa.br"
      />
    </label>
    <label>
      <span>Senha</span>
      <span class="campo-senha">
        <input
          type={mostrar ? "text" : "password"}
          bind:value={senha}
          autocomplete="current-password"
          required
        />
        <button
          type="button"
          class="mostrar"
          onclick={() => (mostrar = !mostrar)}
          aria-pressed={mostrar}
        >
          {mostrar ? "Ocultar" : "Mostrar"}
        </button>
      </span>
    </label>
    {#if erroMensagem}
      <p class="erro" role="alert">{erroMensagem}</p>
    {/if}
    <button type="submit" disabled={enviando}>
      {enviando ? "Entrando..." : "Entrar"}
    </button>
    <a class="voltar" href="/recuperar-senha">Esqueci minha senha</a>
    <p class="nota">Não há autocadastro: a conta é criada pela coordenação.</p>
  </form>
</TelaAcesso>
