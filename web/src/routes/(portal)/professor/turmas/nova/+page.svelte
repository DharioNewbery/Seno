<script lang="ts">
  // Criação de turma em tela própria: matéria (select) + período
  // (trimestre/ano) + título opcional. Sucesso leva ao painel da turma.
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  let trimestre = $state(1);
  let ano = $state(2026);
</script>

<section class="nova-turma">
  {#if data.erroMaterias}
    <p class="aviso erro" role="alert">{data.erroMaterias}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}

  <form method="POST" action="?/criar" class="cartao">
    <h2>Criar turma</h2>
    <p class="dica">
      Você será o professor dono. O período letivo é registrado na primeira
      turma do trimestre/ano.
    </p>

    <label>
      <span>Matéria</span>
      <select name="materia_id" required>
        <option value="" disabled selected>Escolha a matéria</option>
        {#each data.materias as m (m.id)}
          <option value={m.id}>{m.codigo} — {m.nome}</option>
        {/each}
      </select>
    </label>

    <div class="periodo-linha">
      <label>
        <span>Trimestre</span>
        <select bind:value={trimestre}>
          <option value={1}>1</option>
          <option value={2}>2</option>
          <option value={3}>3</option>
        </select>
      </label>
      <label>
        <span>Ano</span>
        <input type="number" bind:value={ano} min={2024} max={2100} required />
      </label>
    </div>

    <label>
      <span>Título (opcional)</span>
      <input name="titulo" placeholder="T1, noturna..." />
    </label>

    <button>Criar turma</button>
  </form>
  <a class="voltar" href="/professor/turmas">← Voltar às turmas</a>
</section>

<style>
  .nova-turma {
    max-width: 460px;
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
  input,
  select {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.55rem 0.7rem;
    font-size: 0.95rem;
    background: var(--seno-gray-100);
  }
  input:focus,
  select:focus {
    outline: 2px solid var(--seno-blue-500);
    outline-offset: 1px;
    border-color: var(--seno-blue-500);
  }
  .periodo-linha {
    display: flex;
    gap: 0.75rem;
  }
  .periodo-linha label {
    flex: 1;
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
