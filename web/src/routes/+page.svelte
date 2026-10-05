<script lang="ts">
  import { goto } from "$app/navigation";
  import Logo from "#lib/components/Logo.svelte";
  import { DESCRICAO_SISTEMA, NOME_SISTEMA } from "#lib/config";
  import { portalPorCargo, type Cargo } from "#lib/tipos";

  let email = $state("");
  let senha = $state("");
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

<main class="entrada">
  <section class="apresentacao">
    <div class="marca">
      <Logo tamanho={40} />
      <h1>{NOME_SISTEMA}</h1>
    </div>
    <p class="lema">{DESCRICAO_SISTEMA}</p>
    <ul class="pontos">
      <li>Turmas, tarefas e atividades por professor</li>
      <li>Editor de código com backup automático</li>
      <li>Correção automática e notas na hora</li>
    </ul>
  </section>

  <section class="painel">
    <form class="cartao" onsubmit={entrar}>
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
        <input
          type="password"
          bind:value={senha}
          autocomplete="current-password"
          required
        />
      </label>
      {#if erroMensagem}
        <p class="erro" role="alert">{erroMensagem}</p>
      {/if}
      <button type="submit" disabled={enviando}>
        {enviando ? "Entrando..." : "Entrar"}
      </button>
      <a class="recuperar" href="/recuperar-senha">Esqueci minha senha</a>
      <p class="nota">Não há autocadastro: a conta é criada pela coordenação.</p>
    </form>
  </section>
</main>

<style>
  .entrada {
    min-height: 100vh;
    display: grid;
    grid-template-columns: 1.1fr 1fr;
    align-items: center;
    gap: 3rem;
    max-width: 1080px;
    margin: 0 auto;
    padding: 2rem;
  }

  .apresentacao {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .marca {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .marca h1 {
    margin: 0;
    font-size: 2.5rem;
  }

  .lema {
    color: var(--seno-gray-700);
    font-size: 1.15rem;
    margin: 0;
  }

  .pontos {
    margin: 1rem 0 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    color: var(--seno-gray-500);
  }

  .pontos li::before {
    content: "•";
    color: var(--seno-blue-500);
    font-weight: 700;
    margin-right: 0.5rem;
  }

  .painel {
    display: flex;
    justify-content: center;
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
    margin: 0 0 0.5rem;
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

  .recuperar {
    color: var(--seno-blue-600);
    font-size: 0.9rem;
    text-align: center;
  }

  .nota {
    margin: 0;
    color: var(--seno-gray-500);
    font-size: 0.8rem;
    text-align: center;
  }

  @media (max-width: 800px) {
    .entrada {
      grid-template-columns: 1fr;
      gap: 1.5rem;
      padding: 1.5rem;
    }
  }
</style>
