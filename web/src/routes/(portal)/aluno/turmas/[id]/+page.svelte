<script lang="ts">
  import Icone from "#lib/components/Icone.svelte";
  import { dataHora } from "#lib/formato";
  import { resumirEstadoAtribuicaoAluno } from "#lib/correcao-aluno";
  import { formatarTituloCardTurmaAluno } from "#lib/turmas-aluno";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const detalhes = $derived(data.detalhes);
  const turma = $derived(detalhes?.turma);
  const atribuicoes = $derived(detalhes?.atribuicoes ?? []);
  const colegas = $derived(detalhes?.colegas ?? []);

  function estado(
    a: (typeof atribuicoes)[number],
  ): { rotulo: string; classe: string } {
    return resumirEstadoAtribuicaoAluno(a);
  }

  function paraTexto(iso: string | undefined): string {
    if (!iso) return "Sem prazo";
    return `Prazo ${dataHora(iso)}`;
  }
</script>

<section class="detalhes-turma">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  {#if turma}
    <div class="navegacao">
      <a class="voltar" href="/aluno/turmas">Voltar para Minhas Turmas</a>
    </div>

    <section class="painel sobre-painel">
      <header class="painel-topo">
        <h2>Sobre</h2>
      </header>

      <div class="cabecalho-turma">
        <div class="info-principal">
          <h1>{formatarTituloCardTurmaAluno(turma)}</h1>
        </div>

        <div class="metadados">
          <div class="dado">
            <span class="rotulo">Período</span>
            <span class="valor">{turma.trimestre}º Trimestre / {turma.ano}</span>
          </div>
          <div class="dado">
            <span class="rotulo">Professor</span>
            <span class="valor">{turma.professor}</span>
            {#if turma.professor_email}
              <span class="email">{turma.professor_email}</span>
            {/if}
          </div>
        </div>
      </div>
    </section>

    <div class="grade-conteudo">
      <section class="painel atividades-painel">
        <header class="painel-topo">
          <h2>Atividades da Turma <span class="contador">({atribuicoes.length})</span></h2>
        </header>

        <ul class="lista-atividades">
          {#each atribuicoes as a (a.id)}
            <li>
              <div class="identidade">
                <strong>{a.atividade_nome}</strong>
                <span class="meta">
                  {paraTexto(a.prazo)}{#if a.duracao_seg}, {Math.round(a.duracao_seg / 60)} min{/if}{#if a.pode_atrasado}, atraso permitido{/if}
                </span>
              </div>
              <div class="acao-area">
                <span class="estado {estado(a).classe}">{estado(a).rotulo}</span>
                {#if turma.ativa && !a.entregue}
                  <!-- Tela pré-atividade: inspeção; o início (Monitor/editor, Fase
                       9.4) acontece lá. -->
                  <a
                    class="btn-icone primario"
                    href={`/aluno/atividades/${a.id}`}
                    title={a.em_andamento ? `Continuar (rev. ${a.minha_revisao})` : "Abrir atividade"}
                    aria-label={a.em_andamento ? `Continuar (rev. ${a.minha_revisao})` : "Abrir atividade"}
                  >
                    <Icone nome="eye" tamanho={18} />
                  </a>
                {:else if a.entregue}
                  <span class="meta">
                    Entregue em {a.entregue_em ? dataHora(a.entregue_em) : ""}
                  </span>
                  {#if a.submissao_id}
                    <a
                      class="btn-icone secundario"
                      href={`/aluno/atividades/${a.id}`}
                      title="Ver atividade"
                      aria-label="Ver atividade"
                    >
                      <Icone nome="eye" tamanho={18} />
                    </a>
                    <a
                      class="btn-icone primario"
                      href={`/aluno/submissao/${a.submissao_id}`}
                      title={a.publicada_em ? "Ver respostas e nota" : "Ver submissão"}
                      aria-label={a.publicada_em ? "Ver respostas e nota" : "Ver submissão"}
                    >
                      <Icone nome="file-text" tamanho={18} />
                    </a>
                  {:else}
                    <a
                      class="btn-icone secundario"
                      href={`/aluno/atividades/${a.id}`}
                      title="Inspecionar"
                      aria-label="Inspecionar"
                    >
                      <Icone nome="eye" tamanho={18} />
                    </a>
                  {/if}
                {:else}
                  <span class="estado encerrada">Turma encerrada</span>
                {/if}
              </div>
            </li>
          {:else}
            <li class="vazio">
              Nenhuma atividade atribuída a esta turma até o momento.
            </li>
          {/each}
        </ul>
      </section>

      <section class="painel colegas-painel">
        <header class="painel-topo">
          <h2>Colegas de Turma <span class="contador">({colegas.length})</span></h2>
        </header>

        <ul class="lista-colegas">
          {#each colegas as c, idx (idx)}
            <li class="item-colega">
              <span class="nome">{c.first_name} {c.last_name}</span>
            </li>
          {:else}
            <li class="vazio">Nenhum colega matriculado.</li>
          {/each}
        </ul>
      </section>
    </div>
  {/if}
</section>

<style>
  .detalhes-turma {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .navegacao {
    margin-bottom: -0.25rem;
  }
  .cabecalho-turma {
    padding: 1.25rem 1.2rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1.5rem;
    flex-wrap: wrap;
  }
  .info-principal {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .cabecalho-turma h1 {
    margin: 0;
    font-size: 1.4rem;
    color: var(--seno-text);
  }
  .metadados {
    display: flex;
    gap: 2rem;
  }
  .dado {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .dado .rotulo {
    font-size: 0.75rem;
    color: var(--seno-gray-500);
    text-transform: uppercase;
    font-weight: 600;
  }
  .dado .valor {
    font-size: 0.95rem;
    font-weight: 600;
  }
  .dado .email {
    font-size: 0.8rem;
    color: var(--seno-gray-500);
  }
  .grade-conteudo {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: 1.25rem;
    align-items: start;
  }
  @media (max-width: 860px) {
    .grade-conteudo {
      grid-template-columns: 1fr;
    }
  }
  .painel {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    overflow: hidden;
  }
  .painel-topo {
    background: var(--seno-blue-900);
    color: var(--seno-white);
    padding: 0.85rem 1.2rem;
  }
  .painel-topo h2 {
    margin: 0;
    font-size: 1.05rem;
    color: var(--seno-white);
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .contador {
    color: var(--seno-blue-200);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .lista-atividades {
    list-style: none;
    padding: 0.75rem 1.2rem 1.2rem;
    margin: 0;
    display: flex;
    flex-direction: column;
  }
  .lista-atividades li {
    padding: 0.85rem 0.2rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.8rem;
    flex-wrap: wrap;
  }
  .lista-atividades li + li {
    border-top: 1px solid var(--seno-gray-200);
  }
  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .acao-area {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .btn-icone {
    width: 2.15rem;
    height: 2.15rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--raio-md);
    text-decoration: none;
    transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
  }
  .btn-icone.secundario {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
  }
  .btn-icone.secundario:hover {
    background: var(--seno-gray-200);
    color: var(--seno-blue-900);
  }
  .btn-icone.primario {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: 1px solid var(--seno-blue-600);
  }
  .btn-icone.primario:hover {
    background: var(--seno-blue-700);
    border-color: var(--seno-blue-700);
  }
  .estado.nota {
    background: var(--seno-blue-100);
    color: #1967d2;
  }
  .estado.entregue {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .estado.andamento {
    background: var(--seno-amber-50);
    color: var(--seno-amber);
  }
  .estado.nao {
    background: var(--seno-gray-100);
    color: var(--seno-gray-500);
  }
  .lista-colegas {
    list-style: none;
    padding: 0.75rem 1.2rem 1.2rem;
    margin: 0;
    display: flex;
    flex-direction: column;
    max-height: 400px;
    overflow-y: auto;
  }
  .item-colega {
    display: flex;
    align-items: center;
    padding: 0.6rem 0.2rem;
    font-size: 0.9rem;
  }
  .item-colega + .item-colega {
    border-top: 1px solid var(--seno-gray-200);
  }
</style>
