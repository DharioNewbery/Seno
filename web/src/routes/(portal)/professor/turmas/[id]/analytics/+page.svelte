<script lang="ts">
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();

  const turma = $derived(data.turma);
  const analytics = $derived(data.analytics);
  const matriculas = $derived(data.matriculas);
  const aba = $derived(data.aba);
  const atvAnalytics = $derived(data.atribuicaoAnalytics);
  const alunoAnalytics = $derived(data.alunoAnalytics);

  function formatarMinutos(segundos?: number): string {
    if (segundos === undefined || segundos === null) return "—";
    const min = Math.round(segundos / 60);
    if (min < 60) return `${min} min`;
    const horas = Math.floor(min / 60);
    const resto = min % 60;
    return `${horas}h ${resto}m`;
  }
</script>

<section class="analytics-turma">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  {#if turma}
    <div class="navegacao">
      <a class="voltar" href={`/professor/turmas/${turma.id}`}>← Voltar para a Turma</a>
    </div>

    <header class="cabeca-analytics">
      <div class="identidade">
        <span class="materia">{turma.materia.codigo} — {turma.materia.nome}</span>
        <h1>Analytics da Turma {turma.titulo ? `· ${turma.titulo}` : ""}</h1>
      </div>

      <nav class="abas">
        <a class="aba" class:ativa={aba === "turma"} href="?aba=turma">Visão da Turma</a>
        <a class="aba" class:ativa={aba === "atividade"} href="?aba=atividade">Por Atividade</a>
        <a class="aba" class:ativa={aba === "aluno"} href="?aba=aluno">Por Aluno</a>
      </nav>
    </header>

    {#if aba === "turma"}
      <div class="grade-resumo">
        <div class="card-metrica">
          <span class="rotulo">Alunos Matriculados</span>
          <strong class="valor">{analytics?.total_matriculados ?? 0}</strong>
        </div>
        <div class="card-metrica">
          <span class="rotulo">Atividades Atribuídas</span>
          <strong class="valor">{analytics?.atribuicoes.length ?? 0}</strong>
        </div>
      </div>

      <section class="tabela-secao">
        <h2>Desempenho por Atribuição</h2>
        {#if analytics && analytics.atribuicoes.length > 0}
          <div class="tabela-responsiva">
            <table class="tabela">
              <thead>
                <tr>
                  <th>Atividade</th>
                  <th>Prazo</th>
                  <th>Entregas</th>
                  <th>Taxa de Atraso</th>
                  <th>Nota Média</th>
                  <th>Nota Mediana</th>
                  <th>Tempo Médio</th>
                </tr>
              </thead>
              <tbody>
                {#each analytics.atribuicoes as a (a.atribuicao_id)}
                  <tr>
                    <td>
                      <a href={`?aba=atividade&atribuicao_id=${a.atribuicao_id}`} class="link-destaque">
                        <strong>{a.atividade_nome}</strong>
                      </a>
                    </td>
                    <td>{a.prazo ? new Date(a.prazo).toLocaleDateString("pt-BR") : "Sem prazo"}</td>
                    <td>
                      <div class="barra-container">
                        <div class="barra" style={`width: ${Math.min(100, a.taxa_entrega)}%`}></div>
                        <span class="taxa">{a.total_entregas} ({Math.round(a.taxa_entrega)}%)</span>
                      </div>
                    </td>
                    <td>
                      {#if a.total_entregas > 0}
                        <span class:atraso-alerta={a.taxa_atraso > 20}>
                          {Math.round(a.taxa_atraso)}% ({a.total_atrasos})
                        </span>
                      {:else}
                        —
                      {/if}
                    </td>
                    <td>{a.nota_media !== undefined && a.nota_media !== null ? a.nota_media.toFixed(1) : "—"}</td>
                    <td>{a.nota_mediana !== undefined && a.nota_mediana !== null ? a.nota_mediana.toFixed(1) : "—"}</td>
                    <td>{formatarMinutos(a.tempo_medio_seg)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else}
          <p class="vazio">Nenhuma atribuição encontrada nesta turma.</p>
        {/if}
      </section>

    {:else if aba === "atividade"}
      <div class="seletor-container">
        <label for="sel-atrib">Selecionar Atribuição:</label>
        <select
          id="sel-atrib"
          value={data.atribuicaoSelecionada}
          onchange={(e) => {
            window.location.search = `?aba=atividade&atribuicao_id=${e.currentTarget.value}`;
          }}
        >
          {#each analytics?.atribuicoes ?? [] as at (at.atribuicao_id)}
            <option value={at.atribuicao_id}>{at.atividade_nome}</option>
          {/each}
        </select>
      </div>

      {#if atvAnalytics}
        <section class="tarefas-analytics">
          <h2>Tarefas da Atividade: {atvAnalytics.atividade_nome}</h2>
          <div class="grade-tarefas">
            {#each atvAnalytics.tarefas as t (t.tarefa_id)}
              <div class="card-tarefa">
                <header class="card-tarefa-header">
                  <strong>{t.ordem}. {t.tarefa_nome || `Tarefa ${t.tarefa_id}`}</strong>
                  <span class="pts">{t.valor_pts} pts</span>
                </header>
                <div class="metricas-tarefa">
                  <div class="linha-m">
                    <span>Taxa de Acerto Pleno:</span>
                    <strong class="taxa-acerto">{Math.round(t.taxa_acerto)}%</strong>
                  </div>
                  <div class="linha-m">
                    <span>Nota Média:</span>
                    <strong>{t.nota_media !== undefined && t.nota_media !== null ? t.nota_media.toFixed(1) : "—"} / {t.valor_pts}</strong>
                  </div>
                  <div class="linha-m">
                    <span>Total de Avaliações:</span>
                    <span>{t.total_avaliadas}</span>
                  </div>
                </div>

                {#if t.vereditos_comuns && t.vereditos_comuns.length > 0}
                  <div class="vereditos">
                    <span class="titulo-v">Vereditos Judge0 mais comuns:</span>
                    <ul>
                      {#each t.vereditos_comuns as vc (vc.status)}
                        <li>
                          <span class="status-nome">{vc.status}:</span>
                          <span class="status-qtd">{vc.contagem}</span>
                        </li>
                      {/each}
                    </ul>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </section>
      {:else}
        <p class="vazio">Selecione uma atribuição acima para visualizar o detalhamento das tarefas.</p>
      {/if}

    {:else if aba === "aluno"}
      <div class="seletor-container">
        <label for="sel-aluno">Selecionar Aluno:</label>
        <select
          id="sel-aluno"
          value={data.alunoSelecionado}
          onchange={(e) => {
            window.location.search = `?aba=aluno&aluno_id=${e.currentTarget.value}`;
          }}
        >
          {#each matriculas as m (m.aluno.id)}
            <option value={m.aluno.id}>{m.aluno.first_name} {m.aluno.last_name} ({m.aluno.email})</option>
          {/each}
        </select>
      </div>

      {#if alunoAnalytics}
        <section class="aluno-linha-tempo">
          <h2>Desempenho de {alunoAnalytics.nome_completo}</h2>
          <ul class="timeline">
            {#each alunoAnalytics.atribuicoes as it (it.atribuicao_id)}
              <li class="timeline-item {it.estado}">
                <div class="item-topo">
                  <strong>{it.atividade_nome}</strong>
                  <span class="status-pill {it.estado}">
                    {#if it.estado === "entregue"}
                      {it.atrasada ? "Entregue Atrasada" : "Entregue"}
                    {:else if it.estado === "rascunho"}
                      Rascunho aberto
                    {:else}
                      Pendente
                    {/if}
                  </span>
                </div>
                <div class="item-detalhes">
                  <span>Prazo: {it.prazo ? new Date(it.prazo).toLocaleString("pt-BR") : "Sem prazo"}</span>
                  {#if it.entregue_em}
                    <span>Entregue em: {new Date(it.entregue_em).toLocaleString("pt-BR")}</span>
                  {/if}
                  {#if it.tempo_gasto_seg}
                    <span>Tempo de execução: {formatarMinutos(it.tempo_gasto_seg)}</span>
                  {/if}
                  {#if it.nota_publicada !== undefined && it.nota_publicada !== null}
                    <strong class="nota-pub">Nota final: {it.nota_publicada} pts</strong>
                  {/if}
                </div>
              </li>
            {/each}
          </ul>
        </section>
      {:else}
        <p class="vazio">Nenhum aluno selecionado ou turma sem matrículas.</p>
      {/if}
    {/if}
  {/if}
</section>

<style>
  .analytics-turma {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .voltar {
    color: var(--seno-primary, #0056b3);
    text-decoration: none;
    font-size: 0.9rem;
    font-weight: 500;
  }
  .voltar:hover {
    text-decoration: underline;
  }
  .cabeca-analytics {
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .materia {
    font-size: 0.85rem;
    color: var(--seno-gray-500, #666);
    font-weight: 600;
    text-transform: uppercase;
  }
  .cabeca-analytics h1 {
    margin: 0.2rem 0 0;
    font-size: 1.4rem;
  }
  .abas {
    display: flex;
    gap: 0.5rem;
  }
  .aba {
    padding: 0.45rem 0.9rem;
    border-radius: 8px;
    text-decoration: none;
    color: var(--seno-gray-700, #444);
    font-weight: 500;
    font-size: 0.9rem;
    background: #f1f3f4;
  }
  .aba.ativa {
    background: var(--seno-primary, #0056b3);
    color: #fff;
  }
  .grade-resumo {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 1rem;
  }
  .card-metrica {
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 10px;
    padding: 1.2rem;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .card-metrica .rotulo {
    font-size: 0.8rem;
    color: var(--seno-gray-500, #666);
    text-transform: uppercase;
    font-weight: 600;
  }
  .card-metrica .valor {
    font-size: 1.8rem;
    color: var(--seno-primary, #0056b3);
  }
  .tabela-secao {
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 12px;
    padding: 1.25rem;
  }
  .tabela-secao h2 {
    margin: 0 0 1rem;
    font-size: 1.15rem;
  }
  .tabela-responsiva {
    overflow-x: auto;
  }
  .tabela {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  .tabela th, .tabela td {
    padding: 0.75rem 0.9rem;
    border-bottom: 1px solid #eee;
    text-align: left;
  }
  .tabela th {
    background: #f8f9fa;
    color: var(--seno-gray-600, #555);
    font-weight: 600;
  }
  .barra-container {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-width: 130px;
  }
  .barra {
    height: 8px;
    background: #34a853;
    border-radius: 4px;
    max-width: 60px;
  }
  .taxa {
    font-size: 0.82rem;
    color: #444;
  }
  .atraso-alerta {
    color: #d93025;
    font-weight: 600;
  }
  .link-destaque {
    color: inherit;
    text-decoration: none;
  }
  .link-destaque:hover {
    text-decoration: underline;
    color: var(--seno-primary, #0056b3);
  }
  .seletor-container {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 10px;
    padding: 0.9rem 1.2rem;
    font-size: 0.95rem;
  }
  .seletor-container select {
    padding: 0.4rem 0.8rem;
    border-radius: 6px;
    border: 1px solid var(--seno-gray-300, #ccc);
    font-size: 0.9rem;
  }
  .grade-tarefas {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 1rem;
    margin-top: 1rem;
  }
  .card-tarefa {
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 10px;
    padding: 1.1rem;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .card-tarefa-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    border-bottom: 1px solid #eee;
    padding-bottom: 0.5rem;
  }
  .card-tarefa-header .pts {
    font-size: 0.85rem;
    color: var(--seno-gray-500, #666);
  }
  .metricas-tarefa {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    font-size: 0.9rem;
  }
  .linha-m {
    display: flex;
    justify-content: space-between;
  }
  .taxa-acerto {
    color: #137333;
  }
  .vereditos {
    border-top: 1px dashed #ddd;
    padding-top: 0.6rem;
    font-size: 0.82rem;
  }
  .titulo-v {
    font-weight: 600;
    color: var(--seno-gray-600, #555);
  }
  .vereditos ul {
    list-style: none;
    padding: 0;
    margin: 0.3rem 0 0;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .vereditos li {
    display: flex;
    justify-content: space-between;
  }
  .timeline {
    list-style: none;
    padding: 0;
    margin: 1rem 0 0;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .timeline-item {
    background: var(--seno-white, #fff);
    border: 1px solid var(--seno-gray-300, #ddd);
    border-radius: 10px;
    padding: 1rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .item-topo {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .item-detalhes {
    display: flex;
    gap: 1.5rem;
    flex-wrap: wrap;
    font-size: 0.85rem;
    color: var(--seno-gray-600, #666);
  }
  .status-pill {
    padding: 0.2rem 0.55rem;
    border-radius: 999px;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .status-pill.entregue {
    background: #e6f4ea;
    color: #137333;
  }
  .status-pill.rascunho {
    background: #fef7e0;
    color: #b06000;
  }
  .status-pill.pendente {
    background: #f1f3f4;
    color: #5f6368;
  }
  .nota-pub {
    color: #1a73e8;
  }
  .vazio {
    padding: 2rem;
    text-align: center;
    color: var(--seno-gray-500, #666);
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red, #c00);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
  }
</style>
