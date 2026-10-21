<script lang="ts">
  import TelaAcesso from "#lib/components/TelaAcesso.svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  let senha = $state("");
  let enviando = $state(false);
  let definido = $state(false);
  let erroMensagem = $state("");

  async function definir(evento: SubmitEvent) {
    evento.preventDefault();
    enviando = true;
    erroMensagem = "";
    try {
      const res = await fetch("/api/convite", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: data.token, senha }),
      });
      if (res.ok) {
        definido = true;
        return;
      }
      const corpo = (await res.json().catch(() => null)) as {
        erro?: { mensagem?: string };
      } | null;
      erroMensagem = corpo?.erro?.mensagem ?? "Não foi possível definir a senha.";
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
      convite.
    </p>
  {:else if definido}
    <section class="formulario">
      <h2>Senha definida!</h2>
      <p>Sua conta está ativa. Entre com seu e-mail e a senha que você criou.</p>
      <a class="entrar" href="/">Entrar no Seno</a>
    </section>
  {:else}
    <form class="formulario" onsubmit={definir}>
      <h2>Defina sua senha</h2>
      <p class="dica">
        Este link de convite é de uso único e expira em 7 dias.
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
        {enviando ? "Definindo..." : "Definir senha"}
      </button>
    </form>
  {/if}
</TelaAcesso>
