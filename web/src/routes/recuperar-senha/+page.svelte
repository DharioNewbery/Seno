<script lang="ts">
  import Logo from "#lib/components/Logo.svelte";
  import { NOME_SISTEMA } from "#lib/config";
</script>

<main class="recuperar">
  <div class="marca">
    <Logo tamanho={32} />
    <h1>{NOME_SISTEMA}</h1>
  </div>
  <p>
    A recuperação de senha por e-mail entra com o fluxo de convites da API
    (token de uso único válido por 1 hora).
  </p>
  <p class="aviso">Em construção.</p>
  <a href="/">Voltar ao login</a>
</main>

<style>
  .recuperar {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    text-align: center;
    padding: 2rem;
  }

  .marca {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .aviso {
    color: var(--seno-blue-600);
    font-weight: 600;
  }
</style>
