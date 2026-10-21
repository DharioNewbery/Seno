<script lang="ts">
  import TelaAcesso from "#lib/components/TelaAcesso.svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let senha = $state("");
  let enviando = $state(false);
  let redefinida = $state(false);
  let erroMensagem = $state("");

  async function redefinir(evento: SubmitEvent) {
    evento.preventDefault();
    enviando = true;
    erroMensagem = "";
    try {
      const res = await fetch("/api/redefinir-senha", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: data.token, senha }),
      });
      if (res.ok) {
        redefinida = true;
        return;
      }
      const corpo = (await res.json().catch(() => null)) as {
        erro?: { mensagem?: string };
      } | null;
      erroMensagem = corpo?.erro?.mensagem ?? "Não foi possível redefinir a senha.";
    } catch {
      erroMensagem = "Sem comunicação com o servidor. Tente novamente.";
    } finally {
      enviando = false;
    }
  }
</script>

<TelaAcesso>
  {#if !data.token}
    <p class="erro" role="alert">
      Token ausente neste link. Use o endereço completo indicado no e-mail de
      redefinição.
    </p>
  {:else if redefinida}
    <section class="formulario">
      <h2>Senha redefinida!</h2>
      <p>
        Todas as sessões anteriores foram encerradas. Entre com a nova senha.
      </p>
      <a class="entrar" href="/">Entrar no Seno</a>
    </section>
  {:else}
    <form class="formulario" onsubmit={redefinir}>
      <h2>Nova senha</h2>
      <p class="dica">
        O link de redefinição é de uso único e expira em 1 hora.
      </p>
      <label>
        <span>Nova senha</span>
        <input
          type="password"
          bind:value={senha}
          autocomplete="new-password"
          minlength={8}
          required
        />
      </label>
      {#if erroMensagem}
        <p class="erro" role="alert">{erroMensagem}</p>
      {/if}
      <button type="submit" disabled={enviando}>
        {enviando ? "Redefinindo..." : "Redefinir senha"}
      </button>
    </form>
  {/if}
</TelaAcesso>
