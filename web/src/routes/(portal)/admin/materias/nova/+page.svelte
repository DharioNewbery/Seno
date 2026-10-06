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
    max-width: 420px;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 12px;
    padding: 1.5rem;
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
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }
  input {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.55rem 0.7rem;
    background: var(--seno-gray-100);
  }
  button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.65rem;
    font-weight: 600;
    cursor: pointer;
  }
  button:hover {
    background: var(--seno-blue-700);
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.9rem;
  }
</style>
