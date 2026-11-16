<script lang="ts">
  import { calcularTotaisCorrecao } from "#lib/correcao-aluno";
  import { dataHora, prazoRelativo } from "#lib/formato";
  // Tela da atividade do aluno: prazo, tempo limite, iniciar (se pendente)
  // e resumo da entrega/publicação com notas sob demanda e link para os detalhes.
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const a = $derived(data.a);
  const visao = $derived(data.visao);
  const cor = $derived(visao?.publicada ? visao.correcao ?? null : null);
  const totais = $derived(cor ? calcularTotaisCorrecao(cor.tarefas) : null);

  let mostrarNota = $state(false);

  const relPrazo = $derived(
    a && !a.entregue ? prazoRelativo(a.prazo) : null,
  );

  // Tempo limite (duração da tentativa): "1 h 30" | "90 min".
  const tempoLimite = $derived.by(() => {
    if (!a?.duracao_seg) return null;
    const min = Math.round(a.duracao_seg / 60);
    if (min % 60 === 0) {
      const h = min / 60;
      return h === 1 ? "1 hora" : `${h} horas`;
    }
    return `${min} min`;
  });

  // Iniciar: turma ativa, sem entrega e dentro da janela (prazo encerrado
  // só bloqueia sem atraso permitido; começar antes do `inicio`, nunca).
  const encerrada = $derived(
    !!a?.prazo && new Date(a.prazo) < new Date() && !a.pode_atrasado,
  );
  const podeIniciar = $derived(
    !!a &&
      a.turma_ativa &&
      !a.entregue &&
      !encerrada &&
      (!a.inicio || new Date(a.inicio) <= new Date()),
  );
</script>

<section class="pre">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if a}
    <div class="cabeca">
      <h2>{a.atividade_nome}</h2>
      <span class="meta">
        {a.materia_codigo} · {a.turma_titulo || `Turma ${a.turma_id}`}
      </span>
    </div>

    <div class="informacoes">
      <div class="cartao">
        <span class="subrotulo">Prazo</span>
        <p class="valor">{a.prazo ? dataHora(a.prazo) : "Sem prazo"}</p>
        {#if relPrazo}<b class="rel {relPrazo.tom}">{relPrazo.rotulo}</b>{/if}
        {#if a.inicio}
          <span class="nota">Abre em {dataHora(a.inicio)}</span>
        {/if}
        {#if a.pode_atrasado}
          <span class="nota">Entrega atrasada permitida</span>
        {/if}
      </div>
      <div class="cartao">
        <span class="subrotulo">Tempo limite</span>
        <p class="valor">{tempoLimite ?? "Sem tempo limite"}</p>
        {#if tempoLimite}
          <span class="nota">Após abrir, você tem esse tempo para entregar</span>
        {/if}
      </div>
    </div>

    {#if !a.entregue}
      <div class="iniciar">
        {#if podeIniciar}
          <a class="botao" href={`/aluno/monitor/${a.id}`}>
            {a.em_andamento ? `Continuar (rev. ${a.minha_revisao})` : "Iniciar atividade"} →
          </a>
        {:else if encerrada}
          <span class="estado encerrada">Prazo encerrado</span>
        {:else if a.inicio && new Date(a.inicio) > new Date()}
          <span class="estado nao">Atividade ainda não começou</span>
        {:else}
          <span class="estado encerrada">Turma encerrada</span>
        {/if}
      </div>
    {/if}

    <div class="cartao entregas">
      <span class="subrotulo">Sua entrega</span>
      {#if a.entregue}
        <div class="entrega">
          <div class="entrega-texto">
            <span>
              Entregue em {a.entregue_em ? dataHora(a.entregue_em) : ""}
              {#if a.atrasada}<b class="rel vencido">(atrasada)</b>{/if}
            </span>
            {#if a.publicada_em || visao?.publicada}
              <span class="publicacao-info">
                Correção publicada em {dataHora((visao?.publicada_em ?? a.publicada_em) || "")}
              </span>
            {:else if visao?.confirmada}
              <span class="nota">Atividade corrigida — aguardando publicação.</span>
            {:else}
              <span class="nota">Aguardando correção e publicação pelo professor.</span>
            {/if}
          </div>

          <div class="entrega-acoes">
            {#if (a.publicada_em || visao?.publicada) && cor}
              <button
                type="button"
                class="botao-toggle"
                onclick={() => (mostrarNota = !mostrarNota)}
              >
                {mostrarNota ? "Ocultar nota" : "Mostrar nota"}
              </button>
            {/if}

            {#if a.submissao_id}
              <a class="botao secundario" href={`/aluno/submissao/${a.submissao_id}`}>
                {a.publicada_em || visao?.publicada
                  ? "Mostrar detalhes da correção →"
                  : "Abrir comprovante de entrega →"}
              </a>
            {/if}
          </div>
        </div>

        {#if mostrarNota && cor}
          <div class="resumo-notas">
            <ul class="lista-notas">
              {#each cor.tarefas ?? [] as t, idx (t.id)}
                {@const notaQ = t.nota_final ?? t.nota_auto ?? 0}
                <li>
                  <span class="questao-nome">
                    Questão {t.ordem ?? idx + 1}: {t.tarefa_nome}
                  </span>
                  <strong class="questao-nota">{notaQ}/{t.valor_pts}</strong>
                </li>
              {/each}
            </ul>
            {#if totais}
              <div class="nota-total">
                <span>Nota total</span>
                <strong>{totais.notaObtida}/{totais.valorTotal}</strong>
              </div>
            {/if}
          </div>
        {/if}
      {:else}
        <span class="nota">Nenhuma entrega ainda.</span>
      {/if}
    </div>

    <a class="voltar" href="/aluno/atividades">← Atividades</a>
  {/if}
</section>

<style>
  .pre {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }
  .cabeca {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .cabeca h2 {
    margin: 0;
    color: var(--seno-blue-900);
  }
  .meta {
    color: var(--seno-gray-500);
    font-size: 0.88rem;
  }
  .informacoes {
    display: flex;
    gap: 0.9rem;
    flex-wrap: wrap;
  }
  .informacoes .cartao {
    flex: 1 1 240px;
  }
  .cartao {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.1rem 1.3rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .subrotulo {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--seno-gray-500);
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }
  .valor {
    margin: 0;
    font-size: 1.05rem;
    color: var(--seno-gray-800);
  }
  .nota {
    color: var(--seno-gray-500);
    font-size: 0.82rem;
  }
  .rel {
    font-weight: 600;
    font-size: 0.86rem;
  }
  .rel.folga {
    color: var(--seno-gray-700);
  }
  .rel.breve {
    color: var(--seno-amber);
  }
  .rel.vencido {
    color: var(--seno-red);
  }
  .iniciar {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .botao {
    display: inline-block;
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border-radius: var(--raio-pill);
    padding: 0.55rem 1.2rem;
    text-decoration: none;
    font-weight: 600;
    font-size: 0.9rem;
  }
  .botao.secundario {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }
  .botao-toggle {
    background: var(--seno-gray-100);
    color: var(--seno-gray-800);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-pill);
    padding: 0.5rem 1.05rem;
    font-weight: 600;
    font-size: 0.88rem;
    cursor: pointer;
  }
  .botao-toggle:hover {
    background: var(--seno-gray-200);
  }
  .estado {
    border-radius: var(--raio-pill);
    padding: 0.3rem 0.8rem;
    font-size: 0.85rem;
  }
  .estado.nao {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .estado.encerrada {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .entregas .entrega {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
    font-size: 0.92rem;
    color: var(--seno-gray-800);
  }
  .entrega-texto {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .entrega-acoes {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    flex-wrap: wrap;
  }
  .publicacao-info {
    font-weight: 600;
    color: var(--seno-green);
    font-size: 0.88rem;
  }
  .resumo-notas {
    margin-top: 0.65rem;
    padding-top: 0.65rem;
    border-top: 1px solid var(--seno-gray-200);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .lista-notas {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .lista-notas li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    padding: 0.35rem 0.6rem;
    background: var(--seno-gray-50);
    border: 1px solid var(--seno-gray-200);
    border-radius: var(--raio-sm);
    font-size: 0.88rem;
  }
  .questao-nome {
    color: var(--seno-gray-800);
  }
  .questao-nota {
    color: var(--seno-blue-900);
    font-variant-numeric: tabular-nums;
  }
  .nota-total {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.45rem 0.6rem;
    border-top: 1px dashed var(--seno-gray-300);
    font-size: 0.94rem;
    color: var(--seno-blue-900);
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
</style>
