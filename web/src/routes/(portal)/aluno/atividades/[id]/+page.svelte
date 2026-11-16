<script lang="ts">
  import { invalidateAll } from "$app/navigation";
  import { calcularTotaisCorrecao } from "#lib/correcao-aluno";
  import { dataHora, prazoRelativo } from "#lib/formato";
  // Tela da atividade do aluno: prazo, tempo limite, iniciar (com modal
  // de confirmação abrindo em nova aba), status em andamento e resumo de notas.
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const a = $derived(data.a);
  const visao = $derived(data.visao);
  const cor = $derived(visao?.publicada ? visao.correcao ?? null : null);
  const totais = $derived(cor ? calcularTotaisCorrecao(cor.tarefas) : null);

  let mostrarNota = $state(false);
  let modalConfirmarInicio = $state(false);
  let iniciadaLocal = $state<string | null>(null);

  const emAndamento = $derived(
    Boolean(a && !a.entregue && (a.em_andamento || iniciadaLocal)),
  );
  const iniciadaEm = $derived(
    a?.iniciada_em ?? a?.gravado_em ?? iniciadaLocal ?? null,
  );

  function confirmarInicioAtividade() {
    if (!a) return;
    window.open(`/aluno/monitor/${a.id}`, "_blank", "noopener");
    modalConfirmarInicio = false;
    if (!iniciadaLocal) {
      iniciadaLocal = new Date().toISOString();
    }
    setTimeout(() => {
      void invalidateAll();
    }, 800);
  }

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
    <div class="navegacao">
      <a class="voltar" href="/aluno/atividades">Voltar para Atividades</a>
    </div>

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
      {#if emAndamento}
        <div class="cartao em-andamento-card">
          <span class="subrotulo">Progresso</span>
          <p class="valor">Atividade em andamento</p>
          {#if iniciadaEm}
            <span class="nota">Iniciada em {dataHora(iniciadaEm)}</span>
          {/if}
        </div>
      {/if}
    </div>

    {#if !a.entregue}
      <div class="iniciar">
        {#if podeIniciar}
          {#if emAndamento}
            <a
              class="botao"
              href={`/aluno/monitor/${a.id}`}
              target="_blank"
              rel="noopener noreferrer"
            >
              Continuar a atividade →
            </a>
          {:else}
            <button
              type="button"
              class="botao"
              onclick={() => (modalConfirmarInicio = true)}
            >
              Iniciar atividade →
            </button>
          {/if}
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
                  ? "Detalhes da correção"
                  : "Comprovante de entrega"}
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

    {#if modalConfirmarInicio}
      <div
        class="modal-fundo"
        role="presentation"
        onclick={() => (modalConfirmarInicio = false)}
      >
        <div
          class="modal-caixa"
          role="dialog"
          aria-modal="true"
          aria-labelledby="titulo-modal-iniciar"
          tabindex="-1"
          onclick={(e) => e.stopPropagation()}
          onkeydown={(e) => e.key === "Escape" && (modalConfirmarInicio = false)}
        >
          <h3 id="titulo-modal-iniciar">Iniciar atividade?</h3>
          <p class="modal-texto">
            Ao confirmar, <strong>{a.atividade_nome}</strong> será aberta em uma nova página para realização.
          </p>
          {#if tempoLimite}
            <p class="modal-aviso">
              Esta atividade possui tempo limite de <strong>{tempoLimite}</strong>, contado a partir do início.
            </p>
          {/if}
          {#if a.prazo}
            <p class="modal-meta">Prazo de entrega: {dataHora(a.prazo)}</p>
          {/if}
          <div class="modal-acoes">
            <button
              type="button"
              class="botao-toggle"
              onclick={() => (modalConfirmarInicio = false)}
            >
              Cancelar
            </button>
            <button
              type="button"
              class="botao"
              onclick={confirmarInicioAtividade}
            >
              Confirmar e iniciar →
            </button>
          </div>
        </div>
      </div>
    {/if}
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
    border: none;
    border-radius: var(--raio-pill);
    padding: 0.55rem 1.2rem;
    text-decoration: none;
    font-weight: 600;
    font-size: 0.9rem;
    cursor: pointer;
  }
  .botao:hover {
    background: var(--seno-blue-700);
  }
  .em-andamento-card {
    background: var(--seno-blue-50);
    border-color: var(--seno-blue-200);
  }
  .modal-fundo {
    position: fixed;
    inset: 0;
    background: rgba(15, 23, 42, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    z-index: 80;
  }
  .modal-caixa {
    width: 100%;
    max-width: 440px;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.35rem 1.45rem;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    box-shadow: 0 18px 40px rgba(15, 23, 42, 0.2);
  }
  .modal-caixa h3 {
    margin: 0;
    color: var(--seno-blue-900);
    font-size: 1.15rem;
  }
  .modal-texto,
  .modal-aviso,
  .modal-meta {
    margin: 0;
    font-size: 0.9rem;
    color: var(--seno-gray-700);
    line-height: 1.45;
  }
  .modal-aviso {
    background: #fffbeb;
    border: 1px solid #fde68a;
    color: #92400e;
    border-radius: var(--raio-sm);
    padding: 0.55rem 0.75rem;
    font-size: 0.85rem;
  }
  .modal-meta {
    color: var(--seno-gray-500);
    font-size: 0.84rem;
  }
  .modal-acoes {
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
    margin-top: 0.45rem;
    flex-wrap: wrap;
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
  .navegacao {
    margin-bottom: -0.25rem;
  }
</style>
