<script lang="ts">
  import Logo from "#lib/components/Logo.svelte";
  import { NOME_SISTEMA } from "#lib/config";
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

<main class="definir">
  <div class="marca">
    <Logo tamanho={32} />
    <h1>{NOME_SISTEMA}</h1>
  </div>

  {#if !data.token}
    <p class="erro" role="alert">
      Token ausente neste link. Use o endereço completo indicado no e-mail de
      convite.
    </p>
  {:else if definido}
    <section class="cartao">
      <h2>Senha definida!</h2>
      <p>Sua conta está ativa. Entre com seu e-mail e a senha que você criou.</p>
      <a class="entrar" href="/">Entrar no Seno</a>
    </section>
  {:else}
    <form class="cartao" onsubmit={definir}>
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
</main>

<style>
  .definir {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.25rem;
    padding: 2rem;
  }

  .marca {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .marca h1 {
    margin: 0;
    font-size: 1.6rem;
  }

  .cartao {
    width: 100%;
    max-width: 400px;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    padding: 2rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .cartao h2 {
    margin: 0;
  }

  .dica {
    margin: 0;
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  label span {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  input {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.65rem 0.75rem;
    font-size: 1rem;
    background: var(--seno-gray-100);
  }

  input:focus {
    outline: 2px solid var(--seno-blue-500);
    outline-offset: 1px;
    border-color: var(--seno-blue-500);
  }

  button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.75rem;
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
  }

  button:hover {
    background: var(--seno-blue-700);
  }

  button:disabled {
    opacity: 0.6;
    cursor: wait;
  }

  .erro {
    margin: 0;
    color: var(--seno-red);
    font-size: 0.9rem;
  }

  .entrar {
    color: var(--seno-blue-600);
    font-weight: 600;
    text-align: center;
  }
</style>
