<script lang="ts">
  // Tela de criação de usuário: o usuário + pessoa nascem pendentes, SEM
  // cargos — eles são atribuídos depois, na tela de inspeção. O convite por
  // e-mail (uso único, 7 dias) define a senha. Falhas voltam com o aviso.
  interface Props {
    form?: { erro?: string } | null;
  }

  let { form }: Props = $props();
</script>

<section class="criacao">
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}

  <form method="POST" action="?/criar" class="cartao">
    <h2>Criar usuário</h2>
    <p class="dica">
      O novo usuário nasce <strong>pendente</strong>, sem cargos, e recebe um
      e-mail de convite para definir a própria senha (uso único, 7 dias). Os
      cargos são atribuídos depois, na inspeção do usuário.
    </p>
    <label>
      <span>Nome</span>
      <input name="first_name" required />
    </label>
    <label>
      <span>Sobrenome</span>
      <input name="last_name" required />
    </label>
    <label>
      <span>E-mail</span>
      <input name="email" type="email" required placeholder="voce@ufpa.br" />
    </label>
    <button>Criar e convidar</button>
  </form>
</section>

<style>
  .criacao {
    max-width: 420px;
  }

  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .cartao h2 {
    margin: 0;
  }

    label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  label span {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

      button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.65rem;
    font-weight: 600;
    cursor: pointer;
  }

  button:hover {
    background: var(--seno-blue-700);
  }

  </style>
