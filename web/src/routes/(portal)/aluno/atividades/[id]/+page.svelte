<script lang="ts">
  import { dataHora, prazoRelativo } from "#lib/formato";
  // Tela pré-atividade: nome, prazo, tempo limite, iniciar (se possível)
  // e as próprias entregas — com a correção, quando publicada (§Visibili-
  // dade: a entrega é do aluno; os detalhes da correção, só publicada).
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const a = $derived(data.a);
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
    !!a && a.turma_ativa && !a.entregue && !encerrada && (!a.inicio || new Date(a.inicio) <= new Date()),
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

    <div class="iniciar">
      {#if podeIniciar}
        <!-- Fase 9.4: inicia pelo BFF (st → Monitor); cai no editor se a
             integração estiver desligada -->
        <a class="botao" href={`/aluno/monitor/${a.id}`}>
          {a.em_andamento ? `Continuar (rev. ${a.minha_revisao})` : "Iniciar atividade"} →
        </a>
      {:else if encerrada}
        <span class="estado encerrada">Prazo encerrado</span>
      {:else if !a.entregue && a.inicio && new Date(a.inicio) > new Date()}
        <span class="estado nao">Atividade ainda não começou</span>
      {:else if a.entregue}
        {#if a.submissao_id}
          <a class="botao" href={`/aluno/submissao/${a.submissao_id}`}>
            {a.publicada_em ? "Ver respostas e notas →" : "Ver comprovante de entrega →"}
          </a>
        {:else}
          <span class="estado entregue">Atividade entregue</span>
        {/if}
      {:else}
        <span class="estado encerrada">Turma encerrada</span>
      {/if}
    </div>

    <div class="cartao entregas">
      <span class="subrotulo">Suas entregas</span>
      {#if a.entregue}
        <div class="entrega">
          <span>
            Entregue em {a.entregue_em ? dataHora(a.entregue_em) : ""}
            {#if a.atrasada}<b class="rel vencido">(atrasada)</b>{/if}
            {#if a.publicada_em}
              · correção publicada {a.nota_final !== null && a.nota_final !== undefined ? `— nota ${a.nota_final}` : ""}
            {/if}
          </span>
          {#if a.submissao_id}
            <a class="botao secundario" href={`/aluno/submissao/${a.submissao_id}`}>
              {a.publicada_em ? "Ver respostas e nota →" : "Ver submissão →"}
            </a>
          {/if}
        </div>
      {:else}
        <span class="nota">Nenhuma entrega ainda.</span>
      {/if}
    </div>

    <a class="voltar" href="/aluno/atividades">← Atividades</a>
  {/if}
</section>

<style>
  .pre {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    max-width: 820px;
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
  .estado {
    border-radius: var(--raio-pill);
    padding: 0.3rem 0.8rem;
    font-size: 0.85rem;
  }
  .estado.nao {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .estado.entregue {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }
  .estado.encerrada {
    background: var(--seno-red-50);
    color: var(--seno-red);
  }
  .entregas .entrega {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
    font-size: 0.92rem;
    color: var(--seno-gray-800);
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
</style>
