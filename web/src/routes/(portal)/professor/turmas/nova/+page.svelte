<script lang="ts">
  // Criação de turma em tela própria: matéria (select), período letivo
  // (escolha entre os cadastrados pelo admin) e título opcional.
  // Sucesso leva ao painel da turma.
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();
</script>

<section class="nova-turma">
  {#if data.erroMaterias}
    <p class="aviso erro" role="alert">{data.erroMaterias}</p>
  {/if}
  {#if data.erroPeriodos}
    <p class="aviso erro" role="alert">{data.erroPeriodos}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}

  <form method="POST" action="?/criar" class="cartao">
    <h2>Criar turma</h2>
    <p class="dica">Você será o professor dono da turma.</p>

    <label>
      <span>Matéria</span>
      <select name="materia_id" required>
        <option value="" disabled selected>Escolha a matéria</option>
        {#each data.materias as m (m.id)}
          <option value={m.id}>{m.codigo} — {m.nome}</option>
        {/each}
      </select>
    </label>

    <label>
      <span>Período letivo</span>
      <select name="periodo_id" required>
        <option value="" disabled selected>Escolha o período</option>
        {#each data.periodos as p (p.id)}
          <option value={p.id}>{p.trimestre}T/{p.ano}</option>
        {/each}
      </select>
    </label>

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
