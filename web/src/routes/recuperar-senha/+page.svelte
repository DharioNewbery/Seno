<script lang="ts">
  import TelaAcesso from "#lib/components/TelaAcesso.svelte";

  let email = $state("");
  let enviando = $state(false);
  let enviado = $state(false);
  let erroMensagem = $state("");

  async function pedir(evento: SubmitEvent) {
    evento.preventDefault();
    enviando = true;
    erroMensagem = "";
    try {
      const res = await fetch("/api/recuperar-senha", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      // 202 sempre (silencioso: não revela se a conta existe).
      if (res.ok) {
        enviado = true;
        return;
      }
      const corpo = (await res.json().catch(() => null)) as {
        erro?: { mensagem?: string };
      } | null;
      erroMensagem = corpo?.erro?.mensagem ?? "Não foi possível enviar o pedido.";
    } catch {
      erroMensagem = "Sem comunicação com o servidor. Tente novamente.";
    } finally {
      enviando = false;
    }
  }
</script>

<TelaAcesso>
  {#if enviado}
    <section class="formulario">
      <h2>Pedido enviado</h2>
      <p>
        Se o e-mail estiver cadastrado, você receberá um link para redefinir a
        senha (válido por 1 hora). Verifique sua caixa de entrada.
      </p>
      <a class="voltar" href="/">Voltar ao login</a>
    </section>
  {:else}
    <form class="formulario" onsubmit={pedir}>
      <h2>Recuperar senha</h2>
      <p class="dica">
        Enviaremos um link de redefinição (uso único) para o seu e-mail.
      </p>
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
      {#if erroMensagem}
        <p class="erro" role="alert">{erroMensagem}</p>
      {/if}
      <button type="submit" disabled={enviando}>
        {enviando ? "Enviando..." : "Enviar link de redefinição"}
      </button>
      <a class="voltar" href="/">Voltar ao login</a>
    </form>
  {/if}
</TelaAcesso>
