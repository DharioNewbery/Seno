<script lang="ts">
  // Criação de matéria (staff): código único e nome; sucesso volta à lista.
  interface Props {
    form?: { erro?: string } | null;
  }

  let { form }: Props = $props();
</script>

<section class="nova-materia">
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  <form method="POST" action="?/criar" class="cartao">
    <h2>Nova matéria</h2>
    <p class="dica">Código único (ex.: PROG1) e nome da cadeira.</p>
    <label>
      <span>Código</span>
      <input name="codigo" maxlength="32" required placeholder="PROG1" />
    </label>
    <label>
      <span>Nome</span>
      <input name="nome" required placeholder="Programação I" />
    </label>
    <button>Criar matéria</button>
  </form>
  <a class="voltar" href="/admin/materias">← Voltar às matérias</a>
</section>

<style>
  .nova-materia {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
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
    padding: 0.65rem 1.25rem;
    font-weight: 600;
    cursor: pointer;
    width: fit-content;
  }
  button:hover {
    background: var(--seno-blue-700);
  }
    </style>
