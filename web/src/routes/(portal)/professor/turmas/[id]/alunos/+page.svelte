<script lang="ts">
  // Alunos da turma: matrículas ativas (toggle de encerradas), matrícula
  // individual, matrícula em lote (linhas email,nome,sobrenome), busca de
  // aluno existente por e-mail/nome para adicionar, e remoção (saída).
  import { page } from "$app/state";
  import { SvelteURLSearchParams } from "svelte/reactivity";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();

  const turma = $derived(data.turma);
  const matriculas = $derived(data.matriculas);
  const busca = $derived(data.busca);
  const incluir = $derived(data.incluir);
  const resultados = $derived(data.resultados);
  const encerrada = $derived(Boolean(turma?.encerrado_em));
  let lote = $state("");
  const modoLote = $derived(page.url.searchParams.get("lote") === "1");

  const idRota = $derived(page.params.id);

  function nav(over: Record<string, string> = {}): string {
    const qs = new SvelteURLSearchParams(page.url.searchParams.toString());
    for (const [k, v] of Object.entries(over)) {
      if (v) qs.set(k, v);
      else qs.delete(k);
    }
    const query = qs.toString();
    return query ? `?${query}` : "";
  }

  function encerradaDe(m: (typeof matriculas)[number]): boolean {
    return Boolean(m.saida_em);
  }
</script>

<section class="alunos">
  <div class="cabeca">
    <div>
      <h2 class="nomes">
        {#if turma}
          {turma.materia.codigo} · {turma.titulo || "Sem título"} ·
          {turma.trimestre}T/{turma.ano}
        {/if}
      </h2>
      <a class="voltar" href={`/professor/turmas/${idRota}`}>← Voltar ao painel</a>
    </div>
  </div>

  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}
  {#if form?.erro}
    <p class="aviso erro" role="alert">{form.erro}</p>
  {/if}
  {#if form?.ok}
    <p class="aviso ok">{form.ok}</p>
  {/if}
  {#if form?.falhas?.length}
    <ul class="falhas">
      {#each form.falhas as falha (falha)}
        <li>{falha}</li>
      {/each}
    </ul>
  {/if}

  <div class="uteis">
    <nav class="mini-filtro" aria-label="Filtrar matrículas">
      <a href={nav({ incluir: "" })} class:y={!incluir}>Ativas</a>
      <a href={nav({ incluir: "true" })} class:y={incluir}>
        Todas (incl. encerradas)
      </a>
    </nav>

    {#if !encerrada}
      <div class="acoes-criacao">
        <a class="modo" href={nav({ lote: "" })} class:y={!modoLote}>Individual</a>
        <a class="modo" href={nav({ lote: "1" })} class:y={modoLote}>
          Vários de uma vez
        </a>
      </div>
    {/if}
  </div>

  {#if !encerrada}
    {#if modoLote}
      <form method="POST" action="?/matricular_lote" class="lote">
        <label>
          <span>
            Uma linha por aluno: email,nome,sobrenome — alunos existentes
            podem ter só o e-mail
          </span>
          <textarea
            name="lote"
            rows="5"
            bind:value={lote}
            placeholder="carla@ufpa.br,Carla,Souza e joao@ufpa.br (uma linha por aluno)"
          ></textarea>
        </label>
        <button>Matricular todos</button>
      </form>
    {:else}
      <form method="POST" action="?/matricular" class="individual">
        <input name="email" type="email" placeholder="E-mail do aluno" required />
        <input name="first_name" placeholder="Nome (para aluna/o novo)" />
        <input name="last_name" placeholder="Sobrenome (para aluna/o novo)" />
        <button>Adicionar</button>
      </form>
    {/if}
  {/if}

  <form class="pesquisa" method="get">
    <input name="q" value={busca} placeholder="Pesquisar aluno por e-mail ou nome" />
    <button>Pesquisar</button>
  </form>
  {#if busca}
    <div class="resultados">
      {#each resultados as a (a.id)}
        <div class="res">
          <span class="quem">
            <strong>{a.first_name} {a.last_name}</strong>
            <small>{a.email}</small>
          </span>
          <span class={`estado ${a.status}`}>
            {a.status === "active" ? "Ativo" : a.status === "pending" ? "Pendente" : "Bloqueado"}
          </span>
          {#if !encerrada}
            <form method="POST" action="?/matricular">
              <input type="hidden" name="email" value={a.email} />
              <button class="pequeno">Adicionar</button>
            </form>
          {/if}
        </div>
      {:else}
        <p class="dica">Nenhum aluno encontrado para “{busca}”.</p>
      {/each}
    </div>
  {/if}

  <table class="tabela">
    <thead>
      <tr>
        <th>Aluna/o</th>
        <th>E-mail</th>
        <th>Estado</th>
        <th>Entrada</th>
        {#if incluir}
          <th>Saída</th>
        {/if}
        <th aria-hidden="true"></th>
      </tr>
    </thead>
    <tbody>
      {#each matriculas as m (m.id)}
        <tr>
          <td>{m.aluno.first_name} {m.aluno.last_name}</td>
          <td class="email">{m.aluno.email}</td>
          <td>
            <span class={`estado ${m.aluno.status}`}>
              {m.aluno.status === "active" ? "Ativo" : m.aluno.status === "pending" ? "Pendente" : "Bloqueado"}
            </span>
            {#if encerradaDe(m)}
              <span class="estado encerrada">Matrícula encerrada</span>
            {/if}
          </td>
          <td>{new Date(m.entrada_em).toLocaleDateString("pt-BR")}</td>
          {#if incluir}
            <td>
              {m.saida_em ? new Date(m.saida_em).toLocaleDateString("pt-BR") : "—"}
            </td>
          {/if}
          <td class="acao-cell">
            {#if !encerradaDe(m) && !encerrada}
              <form method="POST" action="?/remover">
                <input type="hidden" name="matricula_id" value={m.id} />
                <button class="pequeno vermelho">Remover</button>
              </form>
            {/if}
          </td>
        </tr>
      {:else}
        <tr>
          <td colspan={incluir ? 6 : 5} class="vazio">
            Nenhuma matrícula com este filtro.
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>

<style>
  .alunos {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .nomes {
    margin: 0;
  }
  .voltar {
    color: var(--seno-blue-600);
    text-decoration: none;
    font-size: 0.88rem;
  }
  .aviso.erro {
    background: #fdecea;
    color: var(--seno-red);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .aviso.ok {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
    border-radius: 8px;
    padding: 0.6rem 0.9rem;
    font-size: 0.9rem;
    margin: 0;
  }
  .falhas {
    background: var(--seno-gray-100);
    border-radius: 8px;
    padding: 0.6rem 1rem 0.6rem 1.8rem;
    font-size: 0.85rem;
    color: var(--seno-gray-700);
    margin: 0;
  }
  .uteis {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.8rem;
    flex-wrap: wrap;
  }
  .mini-filtro {
    display: flex;
    gap: 0.4rem;
  }
  .mini-filtro a {
    border: 1px solid var(--seno-gray-300);
    border-radius: 999px;
    padding: 0.25rem 0.8rem;
    color: var(--seno-gray-700);
    font-size: 0.82rem;
    text-decoration: none;
    background: var(--seno-white);
  }
  .mini-filtro a.y {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-500);
    color: var(--seno-blue-800);
    font-weight: 600;
  }
  .acoes-criacao {
    display: flex;
    gap: 0.4rem;
  }
  .acoes-criacao .modo {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.35rem 0.8rem;
    color: var(--seno-gray-700);
    text-decoration: none;
    font-size: 0.84rem;
    cursor: pointer;
    background: var(--seno-white);
  }

  .acoes-criacao .modo.y {
    background: var(--seno-blue-100);
    border-color: var(--seno-blue-500);
    color: var(--seno-blue-800);
    font-weight: 600;
  }
  .individual,
  .lote {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    padding: 1rem;
  }
  .individual {
    flex-direction: row;
    flex-wrap: wrap;
  }
  .individual input,
  .pesquisa input {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.6rem;
    font-size: 0.9rem;
    background: var(--seno-white);
  }
  .individual input {
    flex: 1;
    min-width: 130px;
  }
  .lote textarea {
    border: 1px solid var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.6rem;
    font-size: 0.9rem;
    font-family: inherit;
    background: var(--seno-white);
    resize: vertical;
  }
  .lote label span {
    font-size: 0.8rem;
    color: var(--seno-gray-500);
  }
  button {
    background: var(--seno-blue-600);
    color: var(--seno-white);
    border: none;
    border-radius: 8px;
    padding: 0.5rem 0.9rem;
    font-size: 0.9rem;
    cursor: pointer;
  }
  button:hover {
    background: var(--seno-blue-700);
  }
  button.pequeno {
    padding: 0.3rem 0.7rem;
    font-size: 0.8rem;
  }
  button.vermelho {
    background: none;
    color: var(--seno-red);
    border: 1px solid var(--seno-red);
  }
  button.vermelho:hover {
    background: #fdecea;
  }
  .pesquisa {
    display: flex;
    gap: 0.4rem;
  }
  .pesquisa input {
    flex: 1;
    max-width: 420px;
  }
  .resultados {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .res {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    background: var(--seno-white);
    border: 1px dashed var(--seno-gray-300);
    border-radius: 8px;
    padding: 0.5rem 0.8rem;
  }
  .quem {
    display: flex;
    flex-direction: column;
  }
  .estado {
    font-size: 0.72rem;
    border-radius: 999px;
    padding: 0.15rem 0.5rem;
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
  }
  .estado.active {
    background: #e6f4ea;
    color: var(--seno-green);
  }
  .estado.pending {
    background: var(--seno-blue-100);
    color: var(--seno-blue-800);
  }
  .estado.disabled,
  .estado.encerrada {
    background: #fdecea;
    color: var(--seno-red);
  }
  .tabela {
    width: 100%;
    border-collapse: collapse;
    background: var(--seno-white);
    border: 1px solid var(--seno-gray-300);
    border-radius: 10px;
    overflow: hidden;
  }
  .tabela th {
    background: var(--seno-gray-100);
    color: var(--seno-gray-700);
    font-size: 0.8rem;
    text-align: left;
    padding: 0.55rem 0.8rem;
  }
  .tabela td {
    padding: 0.55rem 0.8rem;
    border-top: 1px solid var(--seno-gray-300);
    font-size: 0.9rem;
  }
  .email {
    color: var(--seno-gray-500);
    font-size: 0.84rem;
  }
  .acao-cell {
    text-align: right;
  }
  .vazio {
    text-align: center;
    color: var(--seno-gray-500);
  }
  .dica {
    color: var(--seno-gray-500);
    font-size: 0.85rem;
  }
</style>
