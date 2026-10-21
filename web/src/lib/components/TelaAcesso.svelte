<script lang="ts">
  // Moldura das telas sem sessão (login, convite, recuperação de senha):
  // painel da marca à esquerda, formulário à direita. Os elementos do
  // formulário (label, input, button[type=submit], .erro, .dica, .voltar,
  // .entrar) recebem o estilo daqui — as páginas só fornecem o conteúdo.
  import type { Snippet } from "svelte";
  import Logo from "./Logo.svelte";
  import { DESCRICAO_SISTEMA, NOME_SISTEMA } from "#lib/config";

  let { destaques = false, children }: { destaques?: boolean; children: Snippet } = $props();

  // Duas senoides sobre papel quadriculado: a marca do Seno, desenhada em grande.
  const L = 800;
  const Y0 = 420;
  function onda(amp: number, fase: number): string {
    return Array.from({ length: 161 }, (_, i) => {
      const x = i * 5;
      const y = Y0 - amp * Math.sin((x / L) * Math.PI * 4 + fase);
      return `${i ? "L" : "M"}${x} ${y.toFixed(1)}`;
    }).join(" ");
  }
  const principal = onda(105, 0);
  const secundaria = onda(70, Math.PI / 2);
  const zeros = [200, 400, 600];
</script>

<main class="acesso">
  <section class="arte">
    <svg class="grafico" viewBox="0 0 800 600" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
      <defs>
        <pattern id="papel" width="50" height="50" patternUnits="userSpaceOnUse">
          <path d="M50 0H0V50" fill="none" stroke="var(--seno-blue-800)" stroke-width="1" opacity="0.5" />
        </pattern>
      </defs>
      <rect width="800" height="600" fill="url(#papel)" />
      <line x1="0" y1={Y0} x2="800" y2={Y0} stroke="var(--seno-blue-400)" stroke-width="1" stroke-dasharray="4 6" opacity="0.6" />
      <path d={secundaria} fill="none" stroke="var(--seno-blue-500)" stroke-width="2" opacity="0.55" />
      <path class="onda" pathLength="1" d={principal} fill="none" stroke="var(--seno-blue-300)" stroke-width="3.5" stroke-linecap="round" />
      {#each zeros as x (x)}
        <circle cx={x} cy={Y0} r="6" fill="var(--seno-blue-950)" stroke="var(--seno-blue-300)" stroke-width="2.5" />
      {/each}
    </svg>

    <div class="texto">
      <div class="marca">
        <Logo tamanho={40} />
        <h1>{NOME_SISTEMA}</h1>
      </div>
      <p class="lema">{DESCRICAO_SISTEMA}</p>
      {#if destaques}
        <ul class="pontos">
          <li>Turmas, tarefas e atividades por professor</li>
          <li>Editor de código com backup automático</li>
          <li>Correção automática e notas na hora</li>
        </ul>
      {/if}
    </div>
  </section>

  <section class="painel">
    <div class="conteudo">
      {@render children()}
    </div>
  </section>
</main>

<style>
  .acesso {
    min-height: 100vh;
    min-height: 100dvh;
    display: grid;
    grid-template-columns: minmax(0, 1.05fr) minmax(0, 1fr);
  }

  /* ---- Painel da marca ---- */
  .arte {
    position: relative;
    overflow: hidden;
    display: flex;
    background: var(--seno-blue-950);
    color: var(--seno-blue-100);
  }

  .grafico {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .onda {
    stroke-dasharray: 1;
  }

  @media (prefers-reduced-motion: no-preference) {
    .onda {
      animation: tracar 1.8s ease-out both;
    }
  }

  @keyframes tracar {
    from {
      stroke-dashoffset: 1;
    }
    to {
      stroke-dashoffset: 0;
    }
  }

  .texto {
    position: relative;
    align-self: flex-start;
    max-width: 30rem;
    padding: 4rem 3.5rem;
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
  }

  .marca {
    display: flex;
    align-items: center;
    gap: 0.8rem;
  }

  .marca h1 {
    margin: 0;
    color: var(--seno-white);
    font-size: 2.4rem;
    letter-spacing: -0.02em;
  }

  .lema {
    margin: 0;
    font-size: 1.15rem;
    line-height: 1.45;
    color: var(--seno-blue-100);
  }

  .pontos {
    margin: 0.4rem 0 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    color: var(--seno-blue-200);
  }

  .pontos li {
    padding-left: 1.4rem;
    position: relative;
  }

  .pontos li::before {
    content: "";
    position: absolute;
    left: 0;
    top: 0.5em;
    width: 0.6rem;
    height: 0.35rem;
    border-left: 2px solid var(--seno-blue-300);
    border-bottom: 2px solid var(--seno-blue-300);
    transform: rotate(-45deg);
  }

  /* ---- Formulário ---- */
  .painel {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2rem;
    background: var(--seno-white);
  }

  .conteudo {
    width: 100%;
    max-width: 22rem;
  }

  .acesso :global(.formulario) {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .acesso :global(.formulario h2),
  .acesso :global(.conteudo > .erro) {
    margin: 0;
  }

  .acesso :global(.formulario h2) {
    font-size: 1.6rem;
  }

  .acesso :global(.formulario p) {
    margin: 0;
  }

  .acesso :global(label) {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .acesso :global(label > span) {
    font-size: 0.86rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  .acesso :global(input) {
    width: 100%;
  }

  .acesso :global(button[type="submit"]),
  .acesso :global(a.entrar) {
    display: block;
    padding: 0.7rem 1rem;
    border: none;
    border-radius: var(--raio-md);
    background: var(--seno-blue-600);
    color: var(--seno-white);
    font: inherit;
    font-weight: 600;
    text-align: center;
    text-decoration: none;
    cursor: pointer;
    transition: background-color 0.12s;
  }

  .acesso :global(button[type="submit"]:hover),
  .acesso :global(a.entrar:hover) {
    background: var(--seno-blue-700);
    color: var(--seno-white);
  }

  .acesso :global(button[type="submit"]:disabled) {
    opacity: 0.6;
    cursor: wait;
  }

  .acesso :global(.erro) {
    padding: 0.65rem 0.9rem;
    border: 1px solid var(--seno-red-200);
    border-radius: var(--raio-md);
    background: var(--seno-red-50);
    color: var(--seno-red-700);
    font-size: 0.9rem;
  }

  .acesso :global(.nota) {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }

  .acesso :global(.campo-senha) {
    position: relative;
  }

  .acesso :global(.campo-senha input) {
    padding-right: 4.5rem;
  }

  .acesso :global(.mostrar) {
    position: absolute;
    top: 50%;
    right: 0.4rem;
    transform: translateY(-50%);
    padding: 0.3rem 0.5rem;
    background: none;
    border: none;
    border-radius: var(--raio-sm);
    color: var(--seno-blue-700);
    font: inherit;
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
  }

  /* ---- Telas estreitas: a marca vira uma faixa no topo ---- */
  @media (max-width: 860px) {
    .acesso {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: auto 1fr;
    }

    .grafico {
      opacity: 0.3;
    }

    .texto {
      padding: 1.5rem 1.25rem 2.25rem;
      gap: 0.6rem;
    }

    .marca h1 {
      font-size: 1.7rem;
    }

    .lema {
      font-size: 1rem;
    }

    .pontos {
      display: none;
    }

    .painel {
      align-items: flex-start;
      padding: 1.75rem 1.25rem 2.5rem;
    }
  }
</style>
