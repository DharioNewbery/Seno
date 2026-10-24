<script lang="ts">
  import { dataHora } from "#lib/formato";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const detalhes = $derived(data.detalhes);
  const turma = $derived(detalhes?.turma);
  const atribuicoes = $derived(detalhes?.atribuicoes ?? []);
  const colegas = $derived(detalhes?.colegas ?? []);

  function estado(
    a: (typeof atribuicoes)[number],
  ): { rotulo: string; classe: string } {
    if (a.entregue) {
      if (a.nota_final !== undefined && a.nota_final !== null) {
        return { rotulo: `Nota: ${a.nota_final} pts`, classe: "nota" };
      }
      return a.atrasada
        ? { rotulo: "Entregue atrasada", classe: "entregue" }
        : { rotulo: "Entregue", classe: "entregue" };
    }
    if (a.em_andamento) return { rotulo: "Em andamento", classe: "andamento" };
    if (a.prazo && new Date(a.prazo) < new Date() && !a.pode_atrasado) {
      return { rotulo: "Prazo encerrado", classe: "encerrada" };
    }
    return { rotulo: "Não começou", classe: "nao" };
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
      <a class="voltar" href="/aluno/turmas">← Voltar para Minhas Turmas</a>
    </div>

    <header class="cabecalho-turma">
      <div class="info-principal">
        <div class="tags">
          <span class="tag codigo">{turma.materia_codigo}</span>
          {#if turma.ativa}
            <span class="tag ativa">Ativa</span>
          {:else}
            <span class="tag encerrada">Encerrada</span>
          {/if}
        </div>
        <h1>{turma.materia_nome}</h1>
        {#if turma.titulo}
          <p class="subtitulo">{turma.titulo}</p>
        {/if}
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
    </header>

    <div class="grade-conteudo">
      <section class="painel atividades-painel">
        <div class="painel-topo">
          <h2>Atividades da Turma <span class="contador">({atribuicoes.length})</span></h2>
        </div>

        <ul class="lista-atividades">
          {#each atribuicoes as a (a.id)}
            <li>
              <div class="identidade">
                <strong>{a.atividade_nome}</strong>
                <span class="meta">
                  {paraTexto(a.prazo)}
                  {#if a.duracao_seg}
                    · {Math.round(a.duracao_seg / 60)} min
                  {/if}
                  {#if a.pode_atrasado}· atraso permitido{/if}
                </span>
              </div>
              <div class="acao-area">
                <span class="estado {estado(a).classe}">{estado(a).rotulo}</span>
                {#if turma.ativa && !a.entregue}
                  <!-- Tela pré-atividade: inspeção; o início (Monitor/editor, Fase
                       9.4) acontece lá. -->
                  <a class="abrir" href={`/aluno/atividades/${a.id}`}>
                    {a.em_andamento ? `Continuar (rev. ${a.minha_revisao})` : "Abrir"} →
                  </a>
                {:else if a.entregue}
                  <span class="meta">
                    Entregue em {a.entregue_em ? dataHora(a.entregue_em) : ""}
                  </span>
                  {#if a.submissao_id}
                    <a class="abrir" href={`/aluno/submissao/${a.submissao_id}`}>
                      {a.publicada_em ? "Ver respostas e nota →" : "Ver submissão →"}
                    </a>
                  {:else}
                    <a class="abrir secundario" href={`/aluno/atividades/${a.id}`}>Inspecionar →</a>
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
        <div class="painel-topo">
          <h2>Colegas de Turma <span class="contador">({colegas.length})</span></h2>
        </div>

        <ul class="lista-colegas">
          {#each colegas as c, idx (idx)}
            <li class="item-colega">
              <span class="icone-aluno">👤</span>
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
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.25rem 1.5rem;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1.5rem;
    flex-wrap: wrap;
  }
  .info-principal {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .tags {
    display: flex;
    gap: 0.5rem;
  }
  .tag {
    font-size: 0.75rem;
    padding: 0.2rem 0.5rem;
    border-radius: var(--raio-sm);
    font-weight: 600;
  }
  .tag.codigo {
    background: var(--seno-blue-100);
    color: #3730a3;
  }
  .tag.ativa {
    background: var(--seno-green-50);
    color: var(--seno-green);
  }
  .tag.encerrada {
    background: var(--seno-gray-100);
    color: var(--seno-gray-500);
  }
  .cabecalho-turma h1 {
    margin: 0;
    font-size: 1.4rem;
    color: var(--seno-text);
  }
  .subtitulo {
    margin: 0;
    color: var(--seno-gray-500);
    font-size: 1rem;
  }
  .metadados {
    display: flex;
    gap: 2rem;
    background: var(--seno-gray-50);
    padding: 0.8rem 1.2rem;
    border-radius: var(--raio-md);
    border: 1px solid var(--seno-gray-200);
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
    padding: 1.2rem;
  }
  .painel-topo {
    margin-bottom: 0.9rem;
    border-bottom: 1px solid var(--seno-gray-200);
    padding-bottom: 0.6rem;
  }
  .painel-topo h2 {
    margin: 0;
    font-size: 1.15rem;
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .contador {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
    font-weight: 400;
  }
  .lista-atividades {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .lista-atividades li {
    border: 1px solid var(--seno-gray-200);
    background: var(--seno-gray-50);
    border-radius: var(--raio-md);
    padding: 0.85rem 1rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.8rem;
    flex-wrap: wrap;
  }
  .identidade {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
    .acao-area {
    display: flex;
    align-items: center;
    gap: 0.75rem;
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
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    max-height: 400px;
    overflow-y: auto;
  }
  .abrir.secundario {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
  }
  .abrir.secundario:hover {
    background: var(--seno-gray-200);
  }
  .item-colega {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.45rem 0.6rem;
    border-radius: var(--raio-sm);
    background: var(--seno-gray-50);
    font-size: 0.9rem;
  }
  .icone-aluno {
    font-size: 0.9rem;
    opacity: 0.7;
  }
    </style>
