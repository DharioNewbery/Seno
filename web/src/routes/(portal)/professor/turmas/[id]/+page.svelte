<script lang="ts">
  import type { ComponentProps } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import ListaDados from "#lib/components/ListaDados.svelte";
  import { dataHora, formatarBytes, soData } from "#lib/formato";
  import {
    resumirAtividade,
    type AtividadeComPreview,
    type CandidatoMatricula,
  } from "#lib/turma-painel";
  import { tituloCardTurma } from "#lib/turmas-professor";
  import type {
    AlunoExistenteConferencia,
    AlunoNovoConferencia,
    Atribuicao,
    Matricula,
  } from "#lib/server/ensino";
  import type { PageProps } from "./$types";

  type ListaAtribuicoesProps = ComponentProps<typeof ListaDados<Atribuicao>>;
  type ListaMatriculasProps = ComponentProps<typeof ListaDados<Matricula>>;

  let { data, form }: PageProps = $props();

  const turma = $derived(data.turma);
  const matriculas = $derived(data.matriculas);
  const incluirEncerradas = $derived(data.incluirEncerradas);
  const atribuicoes = $derived(data.atribuicoes);
  const atividades = $derived(data.atividades);
  const encerrada = $derived(Boolean(turma?.encerrado_em));

  // Modal de Configurações da Turma
  let modalConfigAberto = $state(false);
  let tituloTurma = $state("");
  let confirmouEncerrar = $state(false);
  let confirmouExcluir = $state(false);

  // Modal de Adicionar Alunos (individual ou CSV em lote + conferência em 2 listas)
  let modalAlunosAberto = $state(false);
  let modoAlunos = $state<"individual" | "lote">("individual");
  let emailIndividual = $state("");
  let nomeIndividual = $state("");
  let sobrenomeIndividual = $state("");
  let textoCsv = $state("");
  let existentesConferencia = $state<AlunoExistenteConferencia[]>([]);
  let novosConferencia = $state<AlunoNovoConferencia[]>([]);
  let emConferencia = $state(false);

  // Modal de atribuição de atividade em 2 etapas + preview
  let modalAtribuirAberto = $state(false);
  let etapaAtribuir = $state<1 | 2>(1);
  let buscaAtividade = $state("");
  let atividadeSelecionadaId = $state<number | null>(null);
  let atividadePreview = $state<AtividadeComPreview | null>(null);

  const idsAtribuidos = $derived(
    new Set(atribuicoes.map((a) => a.atividade_id)),
  );

  const atividadeSelecionada = $derived(
    atividades.find((a) => a.id === atividadeSelecionadaId) ?? null,
  );

  const atividadesFiltradas = $derived.by(() => {
    const q = buscaAtividade.trim().toLowerCase();
    if (!q) return atividades;
    return atividades.filter(
      (a) => a.nome.toLowerCase().includes(q) || String(a.id).includes(q),
    );
  });

  // Reage a retornos das actions (conferência de alunos ou erros nos modais)
  $effect(() => {
    if (form?.abrirModalConfig) {
      tituloTurma = turma?.titulo ?? "";
      modalConfigAberto = true;
    }
    if (form?.abrirModalAlunos) {
      modalAlunosAberto = true;
      if (form.modoAlunos === "lote" || form.modoAlunos === "individual") {
        modoAlunos = form.modoAlunos;
      }
    }
    if (form?.conferencia) {
      existentesConferencia = [...(form.conferencia.existentes ?? [])];
      novosConferencia = (form.conferencia.novos ?? []).map((n) => ({ ...n }));
      emConferencia = true;
      modalAlunosAberto = true;
    } else if (form?.ok) {
      emConferencia = false;
      modalAlunosAberto = false;
      modalConfigAberto = false;
      modalAtribuirAberto = false;
      etapaAtribuir = 1;
      atividadeSelecionadaId = null;
      confirmouEncerrar = false;
      confirmouExcluir = false;
    }
  });

  // Lista final de candidatos aptos para confirmar matrícula na etapa de conferência
  const candidatosAptos = $derived.by(() => {
    const lista: CandidatoMatricula[] = [];
    for (const ex of existentesConferencia) {
      if (ex.pode_matricular) {
        lista.push({
          email: ex.email,
          first_name: ex.first_name,
          last_name: ex.last_name,
        });
      }
    }
    for (const nv of novosConferencia) {
      const fn = nv.first_name.trim();
      const ln = nv.last_name.trim();
      if (nv.email.includes("@") && fn !== "" && ln !== "") {
        lista.push({
          email: nv.email,
          first_name: fn,
          last_name: ln,
        });
      }
    }
    return lista;
  });

  const candidatosAptosJson = $derived(JSON.stringify(candidatosAptos));

  function abrirModalAlunos() {
    erroCsvLocal = "";
    emConferencia = false;
    modalAlunosAberto = true;
  }

  function fecharModalAlunos() {
    modalAlunosAberto = false;
    emConferencia = false;
  }

  function abrirModalConfig() {
    tituloTurma = turma?.titulo ?? "";
    confirmouEncerrar = false;
    confirmouExcluir = false;
    modalConfigAberto = true;
  }

  function fecharModalConfig() {
    modalConfigAberto = false;
    confirmouEncerrar = false;
    confirmouExcluir = false;
  }

  let erroCsvLocal = $state("");

  async function carregarArquivoCsv(e: Event) {
    erroCsvLocal = "";
    const input = e.currentTarget as HTMLInputElement;
    const arq = input.files?.[0];
    if (!arq) return;
    try {
      textoCsv = await arq.text();
    } catch {
      erroCsvLocal = "Não foi possível ler o arquivo CSV selecionado.";
    }
  }

  function urlIncluirEncerradas(incluir: boolean): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    if (incluir) qs.set("incluir", "true");
    else qs.delete("incluir");
    const s = qs.toString();
    return s ? `?${s}` : page.url.pathname;
  }

  function abrirModalAtribuir() {
    buscaAtividade = "";
    atividadeSelecionadaId = null;
    etapaAtribuir = 1;
    modalAtribuirAberto = true;
  }

  function fecharModalAtribuir() {
    modalAtribuirAberto = false;
    etapaAtribuir = 1;
  }

  function selecionarParaAtribuir(id: number) {
    atividadeSelecionadaId = id;
    atividadePreview = null;
    modalAtribuirAberto = true;
    etapaAtribuir = 2;
  }

  let formEditarConfig = $state<HTMLFormElement | null>(null);
  let idEditarConfig = $state<number | null>(null);
  let formDesligar = $state<HTMLFormElement | null>(null);
  let idDesligar = $state<number | null>(null);
  let formReenviar = $state<HTMLFormElement | null>(null);
  let idReenviar = $state<number | null>(null);
  let formRemoverAluno = $state<HTMLFormElement | null>(null);
  let idRemoverAluno = $state<number | null>(null);

  function dispararForm(
    formRef: HTMLFormElement | null,
    setter: () => void,
  ) {
    setter();
    queueMicrotask(() => formRef?.requestSubmit());
  }

  const duracaoLabel = (seg: number | null | undefined) =>
    seg ? `${Math.round((seg / 3600) * 10) / 10} h` : "Sem duração";

  const colunasAtribuicoes: ListaAtribuicoesProps["columns"] = [
    {
      key: "atividade_nome",
      label: "Atividade",
      primary: true,
      sortable: true,
      value: (a) => a.atividade_nome || `Atividade #${a.atividade_id}`,
    },
    {
      key: "inicio",
      label: "Início",
      sortable: true,
      value: (a) => (a.inicio ? dataHora(a.inicio) : "Imediato"),
    },
    {
      key: "prazo",
      label: "Prazo",
      sortable: true,
      value: (a) => (a.prazo ? dataHora(a.prazo) : "Sem prazo"),
    },
    {
      key: "config",
      label: "Configuração",
      custom: true,
      value: (a) =>
        `${duracaoLabel(a.duracao_seg)} · autocomplete ${a.autocomplete ? "ligado" : "desligado"} · ${a.pode_atrasado ? "atraso permitido" : "sem atraso"}`,
    },
  ];

  const acoesAtribuicoes: ListaAtribuicoesProps["actions"] = [
    {
      id: "open",
      label: "Entregas",
      onselect: (a) => goto(`/professor/atribuicoes/${a.id}/entregas`),
    },
    {
      id: "edit",
      label: "Configurar",
      onselect: (a) =>
        dispararForm(formEditarConfig, () => (idEditarConfig = a.id)),
    },
    {
      id: "delete",
      label: "Desligar",
      onselect: (a) =>
        dispararForm(formDesligar, () => (idDesligar = a.id)),
    },
  ];

  function rotuloStatusAluno(m: Matricula): string {
    if (m.aluno.status === "active") return "Ativo";
    if (m.aluno.status === "pending" && m.aluno.convite_expirado) {
      return "Convite vencido";
    }
    if (m.aluno.status === "pending") return "Convite pendente";
    return "Bloqueado";
  }

  function tomStatusAluno(
    m: Matricula,
  ): "success" | "warning" | "danger" | "info" {
    if (m.aluno.status === "active") return "success";
    if (m.aluno.status === "pending" && m.aluno.convite_expirado) {
      return "danger";
    }
    if (m.aluno.status === "pending") return "warning";
    return "danger";
  }

  const colunasMatriculas = $derived<ListaMatriculasProps["columns"]>([
    {
      key: "aluno",
      label: "Aluno",
      primary: true,
      sortable: true,
      value: (m) => `${m.aluno.first_name} ${m.aluno.last_name}`,
    },
    {
      key: "email",
      label: "E-mail",
      sortable: true,
      value: (m) => m.aluno.email,
    },
    {
      key: "status",
      label: "Status",
      sortable: true,
      custom: true,
      value: (m) =>
        m.saida_em
          ? `${rotuloStatusAluno(m)} · Matrícula encerrada`
          : rotuloStatusAluno(m),
    },
    {
      key: "entrada_em",
      label: "Entrada",
      sortable: true,
      value: (m) => soData(m.entrada_em),
    },
    ...(incluirEncerradas
      ? [
          {
            key: "saida_em",
            label: "Saída",
            sortable: true,
            value: (m: Matricula) => (m.saida_em ? soData(m.saida_em) : "—"),
          },
        ]
      : []),
  ]);

  const filtrosMatriculas: ListaMatriculasProps["filters"] = [
    { id: "todos", label: "Todos" },
    {
      id: "ativos",
      label: "Ativos",
      predicate: (m) => m.aluno.status === "active" && !m.saida_em,
    },
    {
      id: "pendentes",
      label: "Pendentes",
      predicate: (m) => m.aluno.status === "pending" && !m.saida_em,
    },
  ];

  const acoesMatriculas: ListaMatriculasProps["actions"] = [
    {
      id: "open",
      label: "Reenviar convite",
      hidden: (m) =>
        Boolean(m.saida_em) || encerrada || m.aluno.status !== "pending",
      onselect: (m) =>
        dispararForm(formReenviar, () => (idReenviar = m.aluno.id)),
    },
    {
      id: "delete",
      label: "Remover",
      hidden: (m) => Boolean(m.saida_em) || encerrada,
      onselect: (m) =>
        dispararForm(formRemoverAluno, () => (idRemoverAluno = m.id)),
    },
  ];
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === "Escape") {
      if (atividadePreview) atividadePreview = null;
      else if (modalAtribuirAberto) fecharModalAtribuir();
      else if (modalAlunosAberto) fecharModalAlunos();
      else if (modalConfigAberto) fecharModalConfig();
    }
  }}
/>

<section class="painel">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {:else if turma}
    <div class="topo-navegacao">
      <a class="voltar" href="/professor/turmas">← Voltar para Turmas</a>
    </div>

    {#if form?.erro}
      <p class="aviso erro" role="alert">{form.erro}</p>
    {/if}
    {#if form?.ok}
      <p class="aviso ok">{form.ok}</p>
    {/if}
    {#if form?.falhas?.length}
      <ul class="falhas" role="alert">
        {#each form.falhas as falha (falha)}
          <li>{falha}</li>
        {/each}
      </ul>
    {/if}

    <header class="cabeca">
      <div class="identidade-turma">
        <div class="linha-titulo">
          <h2 class="nomes">{tituloCardTurma(turma)}</h2>
          {#if encerrada}
            <span class="estado encerrada">Encerrada</span>
          {/if}
        </div>
        <span class="linha2">
          {turma.materia.codigo} — {turma.materia.nome} · {turma.trimestre}T/{turma.ano}
          · {turma.professor.first_name} {turma.professor.last_name}
        </span>
      </div>

      <div class="acoes-cabeca">
        <button
          type="button"
          class="aba config-btn"
          onclick={abrirModalConfig}
        >
          Configurações
        </button>
      </div>
    </header>

    <!-- ==================== SEÇÃO 1: ATIVIDADES ==================== -->
    <section class="secao-painel" aria-label="Atividades da turma">
      <ListaDados
        title="Atividades atribuídas"
        subtitle={`${atribuicoes.length} ${atribuicoes.length === 1 ? "atividade" : "atividades"}`}
        columns={colunasAtribuicoes}
        rows={atribuicoes}
        rowKey={(a) => a.id}
        actions={acoesAtribuicoes}
        searchPlaceholder="Buscar atividade..."
        onrowclick={(a) => goto(`/professor/atribuicoes/${a.id}/entregas`)}
        emptyTitle="Nenhuma atividade atribuída"
        emptyText="Clique em Atribuir atividade + para adicionar atividades a esta turma."
      >
        {#snippet header()}
          {#if !encerrada}
            <button
              type="button"
              class="nova btn-primario"
              onclick={abrirModalAtribuir}
            >
              Atribuir atividade +
            </button>
          {/if}
        {/snippet}

        {#snippet cell(row, col)}
          {#if col.key === "config"}
            <span class="dl-main">{duracaoLabel(row.duracao_seg)}</span>
            <span class="dl-sub">
              autocomplete {row.autocomplete ? "ligado" : "desligado"} ·
              {row.pode_atrasado ? "atraso permitido" : "sem atraso"}
            </span>
          {/if}
        {/snippet}
      </ListaDados>

      <form method="POST" action="?/editarConfig" bind:this={formEditarConfig} hidden>
        <input type="hidden" name="atribuicao_id" value={idEditarConfig ?? ""} />
      </form>
      <form method="POST" action="?/desligar" bind:this={formDesligar} hidden>
        <input type="hidden" name="atribuicao_id" value={idDesligar ?? ""} />
      </form>
    </section>

    <!-- ==================== SEÇÃO 2: ALUNOS ==================== -->
    <section class="secao-painel" aria-label="Alunos da turma">
      <ListaDados
        title="Alunos matriculados"
        subtitle={`${matriculas.length} ${matriculas.length === 1 ? "aluno" : "alunos"}`}
        columns={colunasMatriculas}
        rows={matriculas}
        rowKey={(m) => m.id}
        filters={filtrosMatriculas}
        actions={acoesMatriculas}
        searchPlaceholder="Buscar aluno por nome ou e-mail..."
        emptyTitle="Nenhum aluno matriculado"
        emptyText="Adicione alunos individualmente ou em lote (CSV)."
      >
        {#snippet header()}
          <a
            class="btn-texto-sutil"
            href={urlIncluirEncerradas(!incluirEncerradas)}
          >
            {incluirEncerradas
              ? "Ocultar matrículas encerradas"
              : "Mostrar matrículas encerradas"}
          </a>
          {#if !encerrada}
            <button
              type="button"
              class="nova btn-primario"
              onclick={abrirModalAlunos}
            >
              Adicionar alunos +
            </button>
          {/if}
        {/snippet}

        {#snippet cell(row, col)}
          {#if col.key === "status"}
            <div class="badges-status">
              <span class="dl-badge" data-tone={tomStatusAluno(row)}>
                {rotuloStatusAluno(row)}
              </span>
              {#if row.saida_em}
                <span class="dl-badge" data-tone="danger">
                  Matrícula encerrada
                </span>
              {/if}
            </div>
          {/if}
        {/snippet}
      </ListaDados>

      <form method="POST" action="?/reenviar_convite" bind:this={formReenviar} hidden>
        <input type="hidden" name="aluno_id" value={idReenviar ?? ""} />
      </form>
      <form method="POST" action="?/remover_aluno" bind:this={formRemoverAluno} hidden>
        <input type="hidden" name="matricula_id" value={idRemoverAluno ?? ""} />
      </form>
    </section>
  {/if}
</section>

<!-- ==================== MODAL: ATRIBUIR ATIVIDADE (2 ETAPAS) ==================== -->
{#if modalAtribuirAberto && turma}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModalAtribuir();
    }}
  >
    <div
      class="modal-janela modal-largo"
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-atribuir-titulo"
    >
      <div class="modal-cabecalho">
        <div>
          <span class="etapa-indicador">Etapa {etapaAtribuir} de 2</span>
          <h3 id="modal-atribuir-titulo">
            {etapaAtribuir === 1
              ? "Selecionar atividade"
              : "Configurar atribuição"}
          </h3>
          <p class="dica">
            {etapaAtribuir === 1
              ? "Escolha uma atividade do seu banco para atribuir à turma."
              : "Defina as datas de início, prazo de entrega e regras de realização."}
          </p>
        </div>
        <button
          type="button"
          class="btn-fechar-modal"
          onclick={fecharModalAtribuir}
          aria-label="Fechar janela"
        >
          ✕
        </button>
      </div>

      <div class="modal-corpo">
        {#if etapaAtribuir === 1}
          {#if atividades.length === 0}
            <p class="vazio-card">
              Nenhuma atividade cadastrada no banco. Crie uma atividade em
              <a href="/professor/atividades">Banco de atividades</a>.
            </p>
          {:else}
            <div class="busca-atribuir">
              <input
                type="search"
                placeholder="Buscar atividade por nome ou ID..."
                bind:value={buscaAtividade}
              />
            </div>

            <div class="grid-atividades">
              {#each atividadesFiltradas as atv (atv.id)}
                {@const resumo = resumirAtividade(atv)}
                {@const jaAtribuida = idsAtribuidos.has(atv.id)}
                {@const selecionada = atividadeSelecionadaId === atv.id}
                <article
                  class="card-grid-atividade"
                  class:selecionada
                  class:ja-atribuida={jaAtribuida}
                >
                  <div class="card-grid-topo">
                    <div class="card-grid-titulo">
                      <strong>{atv.nome}</strong>
                      <span class="card-id">#{atv.id}</span>
                    </div>
                    {#if jaAtribuida}
                      <span class="badge-ja-atribuida">Já atribuída</span>
                    {/if}
                  </div>

                  <div class="card-grid-metricas">
                    <span>
                      {resumo.totalTarefas}
                      {resumo.totalTarefas === 1 ? "tarefa" : "tarefas"}
                    </span>
                    <span>·</span>
                    <span>{resumo.totalPontos} pts</span>
                  </div>

                  {#if resumo.etiquetas.length > 0}
                    <div class="card-grid-tags">
                      {#each resumo.etiquetas as tag (tag)}
                        <span class="tag-item">{tag}</span>
                      {/each}
                    </div>
                  {/if}

                  <div class="card-grid-acoes">
                    <button
                      type="button"
                      class="btn-preview"
                      onclick={() => (atividadePreview = atv)}
                    >
                      Preview
                    </button>
                    <button
                      type="button"
                      class="btn-selecionar"
                      class:ativo={selecionada}
                      onclick={() => (atividadeSelecionadaId = atv.id)}
                    >
                      {selecionada ? "Selecionada ✓" : "Selecionar"}
                    </button>
                  </div>
                </article>
              {:else}
                <p class="vazio-card">Nenhuma atividade encontrada para a busca.</p>
              {/each}
            </div>
          {/if}

          <div class="modal-rodape-acoes">
            <button type="button" class="peq" onclick={fecharModalAtribuir}>
              Cancelar
            </button>
            <button
              type="button"
              class="nova btn-primario"
              disabled={!atividadeSelecionada}
              onclick={() => (etapaAtribuir = 2)}
            >
              Próximo: Configurar →
            </button>
          </div>
        {:else if atividadeSelecionada}
          <form method="POST" action="?/atribuir" class="form-atribuir">
            <input
              type="hidden"
              name="atividade_id"
              value={atividadeSelecionada.id}
            />
            <div class="selecionada-topo">
              <div>
                <span class="rotulo-selecionada">Atividade selecionada</span>
                <strong>{atividadeSelecionada.nome}</strong>
              </div>
              <button
                type="button"
                class="btn-texto-sutil"
                onclick={() => (etapaAtribuir = 1)}
              >
                Trocar atividade
              </button>
            </div>

            <div class="campos-atribuir-modal">
              <label class="mini">
                <span>Início (vazio = agora)</span>
                <input type="datetime-local" name="inicio" />
              </label>
              <label class="mini">
                <span>Prazo de entrega *</span>
                <input type="datetime-local" name="prazo" required />
              </label>
              <label class="mini">
                <span>Duração (h)</span>
                <input
                  type="number"
                  name="duracao_horas"
                  min="0.25"
                  max="168"
                  step="0.25"
                  placeholder="Ex.: 2"
                />
              </label>
              <label class="mini">
                <span>Autocomplete</span>
                <select name="autocomplete">
                  <option value="">Padrão do sistema</option>
                  <option value="on">Ligado</option>
                  <option value="off">Desligado</option>
                </select>
              </label>
              <label class="mini linha-check">
                <input type="checkbox" name="pode_atrasado" />
                <span>Permitir entrega com atraso</span>
              </label>
            </div>

            <div class="modal-rodape-acoes">
              <button
                type="button"
                class="peq"
                onclick={() => (etapaAtribuir = 1)}
              >
                ← Voltar
              </button>
              <button type="submit" class="nova btn-primario">
                Confirmar atribuição
              </button>
            </div>
          </form>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- ==================== MODAL: ADICIONAR ALUNOS ==================== -->
{#if modalAlunosAberto && turma}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModalAlunos();
    }}
  >
    <div
      class="modal-janela modal-largo"
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-alunos-titulo"
    >
      <div class="modal-cabecalho">
        <div>
          <h3 id="modal-alunos-titulo">
            {emConferencia
              ? "Conferir alunos identificados"
              : "Adicionar alunos à turma"}
          </h3>
          <p class="dica">
            {emConferencia
              ? "Confira os alunos já cadastrados no banco e os novos alunos que receberão convite por e-mail."
              : "Adicione um aluno individualmente ou envie uma lista em formato CSV."}
          </p>
        </div>
        <button
          type="button"
          class="btn-fechar-modal"
          onclick={fecharModalAlunos}
          aria-label="Fechar janela"
        >
          ✕
        </button>
      </div>

      <div class="modal-corpo">
        {#if form?.erroAlunos}
          <p class="aviso erro" role="alert">{form.erroAlunos}</p>
        {/if}
        {#if erroCsvLocal}
          <p class="aviso erro" role="alert">{erroCsvLocal}</p>
        {/if}

        {#if !emConferencia}
          <div class="seletor-modo">
            <button
              type="button"
              class:ativo={modoAlunos === "individual"}
              onclick={() => (modoAlunos = "individual")}
            >
              Individual
            </button>
            <button
              type="button"
              class:ativo={modoAlunos === "lote"}
              onclick={() => (modoAlunos = "lote")}
            >
              Lote (CSV)
            </button>
          </div>

          <form method="POST" action="?/conferir_alunos" class="form-modal">
            <input type="hidden" name="modo" value={modoAlunos} />

            {#if modoAlunos === "individual"}
              <label>
                <span>E-mail do aluno</span>
                <input
                  name="email"
                  type="email"
                  bind:value={emailIndividual}
                  placeholder="aluno@ufpa.br"
                  required
                />
              </label>
              <div class="linha-dupla">
                <label>
                  <span>Nome (obrigatório para novos alunos)</span>
                  <input
                    name="first_name"
                    bind:value={nomeIndividual}
                    placeholder="Ex.: Carla"
                  />
                </label>
                <label>
                  <span>Sobrenome (obrigatório para novos alunos)</span>
                  <input
                    name="last_name"
                    bind:value={sobrenomeIndividual}
                    placeholder="Ex.: Souza"
                  />
                </label>
              </div>
            {:else}
              <label>
                <span>Carregar arquivo CSV (opcional)</span>
                <input
                  type="file"
                  accept=".csv,text/csv,.txt"
                  onchange={carregarArquivoCsv}
                />
              </label>
              <label>
                <span>
                  Linhas CSV (<code>email,nome,sobrenome</code> — uma por linha)
                </span>
                <textarea
                  name="lote"
                  rows="6"
                  bind:value={textoCsv}
                  placeholder="carla@ufpa.br,Carla,Souza&#10;joao@ufpa.br,João,Silva"
                  required
                ></textarea>
              </label>
            {/if}

            <div class="modal-rodape">
              <button
                type="button"
                class="btn-cancelar"
                onclick={fecharModalAlunos}
              >
                Cancelar
              </button>
              <button type="submit" class="btn-confirmar">
                Conferir lista →
              </button>
            </div>
          </form>
        {:else}
          <!-- Etapa 2: Duas listas separadas (Existentes no BD vs Novos a convidar) -->
          <div class="listas-conferencia">
            <div class="bloco-conferencia">
              <h4>
                1. Alunos já existentes no sistema
                <span class="total">({existentesConferencia.length})</span>
              </h4>
              {#if existentesConferencia.length === 0}
                <p class="dica-vazia">
                  Nenhum dos e-mails informados já possui cadastro no sistema.
                </p>
              {:else}
                <ul class="lista-conf">
                  {#each existentesConferencia as ex (ex.id)}
                    <li class:bloqueado={!ex.pode_matricular}>
                      <div class="conf-info">
                        <strong>{ex.first_name} {ex.last_name}</strong>
                        <span>{ex.email}</span>
                      </div>
                      <div class="conf-badges">
                        {#if ex.status === "active"}
                          <span class="estado ativa">Ativo</span>
                        {:else if ex.status === "pending" && ex.convite_expirado}
                          <span class="estado vencido">
                            Convite vencido · Novo convite será enviado
                          </span>
                        {:else if ex.status === "pending"}
                          <span class="estado pendente">Convite pendente</span>
                        {:else}
                          <span class="estado encerrada">Bloqueado</span>
                        {/if}

                        {#if ex.motivo_bloqueio}
                          <span class="motivo-bloqueio">{ex.motivo_bloqueio}</span>
                        {/if}
                      </div>
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>

            <div class="bloco-conferencia">
              <h4>
                2. Alunos que serão convidados para o sistema
                <span class="total">({novosConferencia.length})</span>
              </h4>
              {#if novosConferencia.length === 0}
                <p class="dica-vazia">
                  Nenhum novo usuário precisará ser convidado.
                </p>
              {:else}
                <ul class="lista-conf">
                  {#each novosConferencia as nv (nv.email)}
                    {@const valido =
                      nv.email.includes("@") &&
                      nv.first_name.trim() !== "" &&
                      nv.last_name.trim() !== ""}
                    <li class="item-novo" class:bloqueado={!valido}>
                      <div class="conf-novo-topo">
                        <strong>{nv.email}</strong>
                        <span class="estado {valido ? 'pendente' : 'vencido'}">
                          {valido
                            ? "Será convidado por e-mail"
                            : "Preencha nome e sobrenome"}
                        </span>
                      </div>
                      <div class="campos-novo-inline">
                        <input
                          placeholder="Nome"
                          bind:value={nv.first_name}
                          aria-label={`Nome de ${nv.email}`}
                        />
                        <input
                          placeholder="Sobrenome"
                          bind:value={nv.last_name}
                          aria-label={`Sobrenome de ${nv.email}`}
                        />
                      </div>
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          </div>

          <form method="POST" action="?/confirmar_matriculas" class="modal-rodape">
            <input
              type="hidden"
              name="candidatos_json"
              value={candidatosAptosJson}
            />
            <button
              type="button"
              class="btn-cancelar"
              onclick={() => (emConferencia = false)}
            >
              ← Voltar e editar
            </button>
            <button
              type="submit"
              class="btn-confirmar"
              disabled={candidatosAptos.length === 0}
            >
              Confirmar matrícula ({candidatosAptos.length})
            </button>
          </form>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- ==================== MODAL: PREVIEW DA ATIVIDADE ==================== -->
{#if atividadePreview}
  {@const resumoPrev = resumirAtividade(atividadePreview)}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) atividadePreview = null;
    }}
  >
    <div
      class="modal-janela modal-largo"
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-preview-titulo"
    >
      <div class="modal-cabecalho">
        <div>
          <h3 id="modal-preview-titulo">{atividadePreview.nome}</h3>
          <p class="dica">
            {resumoPrev.totalTarefas}
            {resumoPrev.totalTarefas === 1 ? "tarefa" : "tarefas"} ·
            {resumoPrev.totalPontos} pts no total
          </p>
        </div>
        <button
          type="button"
          class="btn-fechar-modal"
          onclick={() => (atividadePreview = null)}
          aria-label="Fechar preview"
        >
          ✕
        </button>
      </div>

      <div class="modal-corpo">
        {#if atividadePreview.tarefas.length === 0}
          <p class="dica-vazia">Esta atividade não possui tarefas detalhadas.</p>
        {:else}
          <ol class="lista-preview-tarefas">
            {#each atividadePreview.tarefas as t (t.ordem)}
              <li class="item-preview-tarefa">
                <div class="preview-tarefa-topo">
                  <strong>
                    #{t.ordem} — {t.tarefa_nome}
                  </strong>
                  <div class="preview-tarefa-badges">
                    {#if t.tarefa_tipo === "arquivo_submissao"}
                      <span class="tag-item">
                        Arquivo ({(
                          t.tarefa_config_arquivo?.extensoes_permitidas ?? []
                        )
                          .map((ext) => `.${ext}`)
                          .join(", ") || "qualquer"} · máx. {formatarBytes(
                          t.tarefa_config_arquivo?.max_bytes ?? 0,
                        )})
                      </span>
                    {:else}
                      <span class="tag-item">{t.linguagem}</span>
                    {/if}
                    <span class="tag-pts">{t.valor_pts} pts</span>
                  </div>
                </div>
                {#if t.tarefa_enunciado}
                  <p class="preview-enunciado">{t.tarefa_enunciado}</p>
                {/if}
              </li>
            {/each}
          </ol>
        {/if}

        <div class="modal-rodape">
          <button
            type="button"
            class="btn-cancelar"
            onclick={() => (atividadePreview = null)}
          >
            Fechar
          </button>
          {#if !encerrada && atividadePreview}
            <button
              type="button"
              class="btn-confirmar"
              onclick={() => selecionarParaAtribuir(atividadePreview!.id)}
            >
              Selecionar para atribuir
            </button>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- ==================== MODAL: CONFIGURAÇÕES DA TURMA ==================== -->
{#if modalConfigAberto && turma}
  <div
    class="modal-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) fecharModalConfig();
    }}
  >
    <div
      class="modal-janela"
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-config-titulo"
    >
      <div class="modal-cabecalho">
        <div>
          <h3 id="modal-config-titulo">Configurações da turma</h3>
          <p class="dica">
            {turma.materia.codigo} — {turma.materia.nome} ({turma.trimestre}T/{turma.ano})
          </p>
        </div>
        <button
          type="button"
          class="btn-fechar-modal"
          onclick={fecharModalConfig}
          aria-label="Fechar configurações"
        >
          ✕
        </button>
      </div>

      <div class="modal-corpo">
        {#if form?.erroConfig}
          <p class="aviso erro" role="alert">{form.erroConfig}</p>
        {/if}

        {#if !encerrada}
          <form method="POST" action="?/renomear" class="bloco-config">
            <label>
              <span>Título da turma</span>
              <input
                name="titulo"
                bind:value={tituloTurma}
                placeholder="Ex.: T1, Noturno..."
              />
            </label>
            <button type="submit" class="btn-confirmar">Salvar título</button>
          </form>

          <hr class="divisor-config" />

          <div class="bloco-config">
            <strong>Encerrar turma</strong>
            <p class="dica">
              Bloqueia novas matrículas, edições e atribuições de atividades.
            </p>
            {#if confirmouEncerrar}
              <form method="POST" action="?/encerrar" class="confirmacao-inline">
                <span>Confirmar encerramento da turma?</span>
                <button type="submit" class="acao amarela">
                  Sim, encerrar
                </button>
                <button
                  type="button"
                  class="btn-cancelar"
                  onclick={() => (confirmouEncerrar = false)}
                >
                  Cancelar
                </button>
              </form>
            {:else}
              <button
                type="button"
                class="acao amarela"
                onclick={() => (confirmouEncerrar = true)}
              >
                Encerrar turma
              </button>
            {/if}
          </div>

          <hr class="divisor-config" />
        {/if}

        <div class="bloco-config">
          <strong>Excluir turma</strong>
          <p class="dica">
            Disponível apenas quando não há alunos com matrícula ativa na turma.
          </p>
          {#if confirmouExcluir}
            <form method="POST" action="?/excluir" class="confirmacao-inline">
              <span>Excluir definitivamente esta turma?</span>
              <button type="submit" class="acao vermelha">Sim, excluir</button>
              <button
                type="button"
                class="btn-cancelar"
                onclick={() => (confirmouExcluir = false)}
              >
                Cancelar
              </button>
            </form>
          {:else}
            <button
              type="button"
              class="acao vermelha"
              onclick={() => (confirmouExcluir = true)}
            >
              Excluir turma
            </button>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .painel {
    display: flex;
    flex-direction: column;
    gap: 1.15rem;
  }

  .topo-navegacao {
    display: flex;
    align-items: center;
  }

  .falhas {
    background: var(--seno-red-50);
    border: 1px solid var(--seno-red-200);
    border-radius: var(--raio-md);
    padding: 0.65rem 1rem 0.65rem 1.8rem;
    font-size: 0.85rem;
    color: var(--seno-red-700);
    margin: 0;
  }

  .cabeca {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 1rem;
    flex-wrap: wrap;
    padding-bottom: 0.85rem;
    border-bottom: 1px solid var(--seno-line);
  }

  .identidade-turma {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .linha-titulo {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }

  .nomes {
    margin: 0;
  }

  .linha2 {
    color: var(--seno-gray-500);
    font-size: 0.88rem;
  }

  .aba {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    padding: 0.45rem 0.9rem;
    font: inherit;
    font-size: 0.88rem;
    font-weight: 500;
    color: var(--seno-gray-700);
    cursor: pointer;
    transition:
      background-color 0.12s,
      border-color 0.12s,
      color 0.12s;
  }

  .aba:hover {
    border-color: var(--seno-blue-500);
    color: var(--seno-blue-900);
  }

  .secao-painel {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .btn-texto-sutil {
    background: none;
    border: none;
    padding: 0.35rem 0.55rem;
    font: inherit;
    font-size: 0.83rem;
    color: var(--seno-gray-500);
    text-decoration: none;
    cursor: pointer;
    border-radius: var(--raio-sm);
  }

  .btn-texto-sutil:hover {
    color: var(--seno-blue-800);
    background: var(--seno-gray-100);
  }

  .btn-primario {
    border: none;
    cursor: pointer;
    font-family: inherit;
  }

  .badges-status {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .estado.pendente {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }

  .estado.vencido {
    background: var(--seno-amber-50);
    color: var(--seno-amber);
  }

  .vazio-card {
    background: var(--seno-white);
    border: 1px dashed var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1.25rem;
    text-align: center;
    color: var(--seno-gray-500);
    margin: 0;
  }

  .form-atribuir {
    background: var(--seno-blue-100);
    border: 1px solid var(--seno-blue-300);
    border-radius: var(--raio-lg);
    padding: 1rem 1.15rem;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }

  .selecionada-topo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .rotulo-selecionada {
    display: block;
    font-size: 0.74rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--seno-blue-800);
  }

  .campos-atribuir-modal {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: 0.85rem;
    align-items: end;
  }

  .etapa-indicador {
    display: inline-block;
    font-size: 0.72rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--seno-blue-700);
    margin-bottom: 0.2rem;
  }

  .busca-atribuir {
    margin-bottom: 0.85rem;
  }

  .busca-atribuir input {
    width: 100%;
  }

  .modal-rodape-acoes {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1rem;
    padding-top: 0.85rem;
    border-top: 1px solid var(--seno-gray-200);
  }

  .acoes-cabeca {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .mini {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .mini span {
    font-size: 0.76rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  .mini.linha-check {
    flex-direction: row;
    align-items: center;
    gap: 0.4rem;
    padding-bottom: 0.5rem;
  }

  .grid-atividades {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(17.5rem, 1fr));
    gap: 0.85rem;
  }

  .card-grid-atividade {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-lg);
    padding: 1rem 1.1rem;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 0.65rem;
    transition:
      border-color 0.12s,
      box-shadow 0.12s;
  }

  .card-grid-atividade:hover {
    border-color: var(--seno-blue-400);
  }

  .card-grid-atividade.selecionada {
    border-color: var(--seno-blue-600);
    box-shadow: var(--foco);
  }

  .card-grid-topo {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .card-grid-titulo {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .card-grid-titulo strong {
    color: var(--seno-blue-900);
    font-size: 0.96rem;
  }

  .card-id {
    font-size: 0.76rem;
    color: var(--seno-gray-500);
    font-family: var(--fonte-mono);
  }

  .badge-ja-atribuida {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.15rem 0.5rem;
    border-radius: var(--raio-pill);
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    white-space: nowrap;
  }

  .card-grid-metricas {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.83rem;
    color: var(--seno-gray-700);
  }

  .card-grid-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .tag-item {
    font-size: 0.74rem;
    font-weight: 500;
    padding: 0.12rem 0.5rem;
    border-radius: var(--raio-sm);
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }

  .tag-pts {
    font-size: 0.74rem;
    font-weight: 600;
    padding: 0.12rem 0.5rem;
    border-radius: var(--raio-sm);
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }

  .card-grid-acoes {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.25rem;
  }

  .btn-preview,
  .btn-selecionar {
    flex: 1;
    padding: 0.42rem 0.65rem;
    border-radius: var(--raio-md);
    font: inherit;
    font-size: 0.83rem;
    font-weight: 500;
    cursor: pointer;
    text-align: center;
  }

  .btn-preview {
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    color: var(--seno-gray-700);
  }

  .btn-preview:hover {
    background: var(--seno-gray-50);
    border-color: var(--seno-gray-500);
  }

  .btn-selecionar {
    background: var(--seno-blue-600);
    border: 1px solid transparent;
    color: var(--seno-white);
    font-weight: 600;
  }

  .btn-selecionar:hover {
    background: var(--seno-blue-700);
  }

  .btn-selecionar.ativo {
    background: var(--seno-blue-900);
  }

  /* Modais compartilhados */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgb(8 42 90 / 0.45);
    backdrop-filter: blur(2px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 1rem;
  }

  .modal-janela {
    background: var(--seno-white);
    border-radius: var(--raio-lg);
    box-shadow: var(--sombra-flutuante);
    width: 100%;
    max-width: 30rem;
    max-height: 88vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--seno-gray-200);
  }

  .modal-janela.modal-largo {
    max-width: 42rem;
  }

  .modal-cabecalho {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    padding: 1.1rem 1.35rem 0.9rem;
    border-bottom: 1px solid var(--seno-gray-200);
  }

  .modal-cabecalho h3 {
    margin: 0 0 0.15rem;
    font-size: 1.15rem;
    color: var(--seno-blue-900);
  }

  .btn-fechar-modal {
    background: transparent;
    border: none;
    font-size: 1.05rem;
    color: var(--seno-gray-500);
    cursor: pointer;
    padding: 0.2rem 0.45rem;
    border-radius: var(--raio-sm);
  }

  .btn-fechar-modal:hover {
    background: var(--seno-gray-100);
    color: var(--seno-gray-900);
  }

  .modal-corpo {
    padding: 1.2rem 1.35rem 1.35rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .seletor-modo {
    display: inline-flex;
    gap: 0.35rem;
    background: var(--seno-gray-100);
    padding: 0.25rem;
    border-radius: var(--raio-md);
    width: fit-content;
  }

  .seletor-modo button {
    background: none;
    border: none;
    padding: 0.38rem 0.85rem;
    border-radius: var(--raio-sm);
    font: inherit;
    font-size: 0.84rem;
    font-weight: 500;
    color: var(--seno-gray-700);
    cursor: pointer;
  }

  .seletor-modo button.ativo {
    background: var(--seno-white);
    color: var(--seno-blue-900);
    font-weight: 600;
    box-shadow: 0 1px 2px rgb(0 0 0 / 0.08);
  }

  .form-modal {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }

  .form-modal label,
  .bloco-config label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .form-modal label span,
  .bloco-config label span {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--seno-gray-700);
  }

  .linha-dupla {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: 0.75rem;
  }

  /* Listas de conferência no modal de alunos */
  .listas-conferencia {
    display: flex;
    flex-direction: column;
    gap: 1.15rem;
  }

  .bloco-conferencia {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .bloco-conferencia h4 {
    margin: 0;
    font-size: 0.95rem;
  }

  .dica-vazia {
    margin: 0;
    padding: 0.75rem;
    background: var(--seno-gray-50);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-md);
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }

  .lista-conf {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-md);
    overflow: hidden;
  }

  .lista-conf li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.65rem;
    flex-wrap: wrap;
    padding: 0.65rem 0.85rem;
    background: var(--seno-white);
  }

  .lista-conf li + li {
    border-top: 1px solid var(--seno-line);
  }

  .lista-conf li.bloqueado {
    background: var(--seno-gray-50);
  }

  .conf-info {
    display: flex;
    flex-direction: column;
    font-size: 0.86rem;
  }

  .conf-info span {
    color: var(--seno-gray-500);
    font-family: var(--fonte-mono);
    font-size: 0.8rem;
  }

  .conf-badges {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .motivo-bloqueio {
    font-size: 0.78rem;
    color: var(--seno-red-700);
    font-weight: 500;
  }

  .item-novo {
    flex-direction: column;
    align-items: stretch !important;
  }

  .conf-novo-topo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    flex-wrap: wrap;
    font-size: 0.86rem;
  }

  .campos-novo-inline {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
  }

  .campos-novo-inline input {
    padding: 0.4rem 0.6rem;
    font-size: 0.85rem;
    min-height: 2.15rem;
  }

  /* Preview de tarefas no modal */
  .lista-preview-tarefas {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
  }

  .item-preview-tarefa {
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-md);
    padding: 0.8rem 0.95rem;
    background: var(--seno-gray-50);
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .preview-tarefa-topo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .preview-tarefa-badges {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  .preview-enunciado {
    margin: 0;
    font-size: 0.86rem;
    color: var(--seno-gray-700);
    white-space: pre-wrap;
  }

  /* Configurações no modal */
  .bloco-config {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }

  .divisor-config {
    border: none;
    border-top: 1px solid var(--seno-line);
    margin: 0.2rem 0;
  }

  .confirmacao-inline {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
    font-size: 0.85rem;
  }

  .acao {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.5rem 0.9rem;
    font: inherit;
    font-size: 0.88rem;
    font-weight: 500;
    cursor: pointer;
    width: fit-content;
  }

  .acao.amarela {
    background: var(--seno-amber-500);
  }

  .acao.amarela:hover {
    background: var(--seno-amber);
  }

  .acao.vermelha {
    background: var(--seno-white);
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }

  .acao.vermelha:hover {
    background: var(--seno-red-50);
  }

  .modal-rodape {
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
    margin-top: 0.35rem;
  }

  .btn-cancelar {
    background: var(--seno-white);
    color: var(--seno-gray-700);
    border: 1px solid var(--seno-gray-300);
    border-radius: var(--raio-md);
    padding: 0.5rem 0.95rem;
    font: inherit;
    font-size: 0.88rem;
    font-weight: 500;
    cursor: pointer;
  }

  .btn-cancelar:hover {
    background: var(--seno-gray-100);
  }

  .btn-confirmar {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: var(--raio-md);
    padding: 0.5rem 1.1rem;
    font: inherit;
    font-size: 0.88rem;
    font-weight: 600;
    cursor: pointer;
    width: fit-content;
  }

  .btn-confirmar:hover:not(:disabled) {
    background: var(--seno-blue-700);
  }

  .btn-confirmar:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
</style>
