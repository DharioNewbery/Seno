<script lang="ts" generics="T extends object">
  import type { Snippet } from "svelte";
  import type { SvgNode } from "#lib/vendor/lucide-icons";

  type ActionId = "view" | "edit" | "duplicate" | "delete" | "open";

  interface Column {
    key: string;
    label: string;
    width?: string; // ex.: '160px', '20%'
    align?: "left" | "center" | "right";
    sortable?: boolean;
    primary?: boolean; // texto em destaque (nome do item)
    custom?: boolean; // renderiza via snippet `cell`
    value?: (row: T) => string | number | null | undefined;
    sub?: (row: T) => string | null | undefined; // 2ª linha, mais discreta
  }

  interface Filter {
    id: string;
    label: string;
    predicate?: (row: T) => boolean; // sem predicate = "todos"
  }

  interface RowAction {
    id: ActionId;
    label?: string;
    onselect: (row: T) => void;
    hidden?: (row: T) => boolean;
  }

  interface Props {
    title?: string;
    subtitle?: string;
    columns: Column[];
    rows: T[];
    rowKey?: (row: T) => string | number;
    filters?: Filter[];
    actions?: RowAction[];
    searchable?: boolean;
    searchPlaceholder?: string;
    pageSize?: number;
    onrowclick?: (row: T) => void;
    emptyTitle?: string;
    emptyText?: string;
    header?: Snippet; // botões do cabeçalho (ex.: "Criar turma")
    toolbar?: Snippet; // controles extras ao lado da busca (selects etc.)
    cell?: Snippet<[T, Column]>; // células das colunas com `custom: true`
  }

  let {
    title,
    subtitle,
    columns,
    rows,
    rowKey,
    filters = [],
    actions = [],
    searchable = true,
    searchPlaceholder = "Buscar",
    pageSize = 10,
    onrowclick,
    emptyTitle = "Nada por aqui ainda",
    emptyText,
    header,
    toolbar,
    cell,
  }: Props = $props();

  const ACTIONS: Record<
    ActionId,
    { label: string; danger?: boolean; nodes: readonly SvgNode[] }
  > = {
    view: {
      label: "Ver",
      nodes: [
        ["path", { d: "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" }],
        ["circle", { cx: "12", cy: "12", r: "3" }],
      ],
    },
    open: {
      label: "Abrir",
      nodes: [
        ["path", { d: "M7 17 17 7" }],
        ["path", { d: "M8 7h9v9" }],
      ],
    },
    edit: {
      label: "Editar",
      nodes: [
        ["path", { d: "M12 20h9" }],
        [
          "path",
          { d: "M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" },
        ],
      ],
    },
    duplicate: {
      label: "Duplicar",
      nodes: [
        ["rect", { x: "9", y: "9", width: "13", height: "13", rx: "2" }],
        [
          "path",
          {
            d: "M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1",
          },
        ],
      ],
    },
    delete: {
      label: "Excluir",
      danger: true,
      nodes: [
        ["path", { d: "M3 6h18" }],
        ["path", { d: "m19 6-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" }],
        ["path", { d: "M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" }],
      ],
    },
  };

  let query = $state("");
  let activeFilter = $state<string | null>(null);
  let sortKey = $state<string | null>(null);
  let sortDir = $state<"asc" | "desc">("asc");
  let page = $state(1);

  const currentFilterId = $derived(activeFilter ?? filters[0]?.id ?? null);

  const norm = (v: unknown) =>
    String(v ?? "")
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "")
      .toLowerCase();

  const getValue = (row: T, col: Column): unknown =>
    col.value ? col.value(row) : (row as Record<string, unknown>)[col.key];

  const filtered = $derived.by(() => {
    const filter = filters.find((f) => f.id === currentFilterId);
    const q = norm(query.trim());
    return rows.filter((row) => {
      if (filter?.predicate && !filter.predicate(row)) return false;
      if (!q) return true;
      return columns.some(
        (col) =>
          norm(getValue(row, col)).includes(q) || norm(col.sub?.(row)).includes(q),
      );
    });
  });

  const sorted = $derived.by(() => {
    const col = columns.find((c) => c.key === sortKey);
    if (!col) return filtered;
    const dir = sortDir === "asc" ? 1 : -1;
    return [...filtered].sort((a, b) => {
      const va = getValue(a, col);
      const vb = getValue(b, col);
      if (typeof va === "number" && typeof vb === "number") {
        return (va - vb) * dir;
      }
      return (
        String(va ?? "").localeCompare(String(vb ?? ""), "pt-BR", {
          numeric: true,
        }) * dir
      );
    });
  });

  const pages = $derived(Math.max(1, Math.ceil(sorted.length / pageSize)));
  const safePage = $derived(Math.min(page, pages));
  const start = $derived((safePage - 1) * pageSize);
  const visible = $derived(sorted.slice(start, start + pageSize));
  const hasFilters = $derived(
    query.trim() !== "" ||
      (activeFilter !== null && activeFilter !== filters[0]?.id),
  );
  const colCount = $derived(columns.length + (actions.length ? 1 : 0));

  function toggleSort(col: Column) {
    if (sortKey !== col.key) {
      sortKey = col.key;
      sortDir = "asc";
    } else if (sortDir === "asc") {
      sortDir = "desc";
    } else {
      sortKey = null;
      sortDir = "asc";
    }
    page = 1;
  }

  function clearFilters() {
    query = "";
    activeFilter = null;
    page = 1;
  }

  function onRowKey(e: KeyboardEvent, row: T) {
    if (e.target !== e.currentTarget) return;
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onrowclick?.(row);
    }
  }
</script>

<section class="dl">
  {#if title || header}
    <header class="dl-head">
      <div class="dl-titles">
        {#if title}<h2>{title}</h2>{/if}
        {#if subtitle}<p>{subtitle}</p>{/if}
      </div>
      {#if header}<div class="dl-head-actions">{@render header()}</div>{/if}
    </header>
  {/if}

  {#if searchable || filters.length > 1 || toolbar}
    <div class="dl-toolbar">
      {#if searchable}
        <label class="dl-search">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          <input
            type="search"
            placeholder={searchPlaceholder}
            aria-label={searchPlaceholder}
            bind:value={query}
            oninput={() => (page = 1)}
          />
        </label>
      {/if}

      {#if filters.length > 1}
        <div class="dl-tabs" role="group" aria-label="Filtros">
          {#each filters as f (f.id)}
            <button
              type="button"
              class="dl-tab"
              aria-pressed={currentFilterId === f.id}
              onclick={() => {
                activeFilter = f.id;
                page = 1;
              }}>{f.label}</button
            >
          {/each}
        </div>
      {/if}

      {#if toolbar}<div class="dl-toolbar-extra">{@render toolbar()}</div>{/if}
    </div>
  {/if}

  <div class="dl-scroll">
    <table>
      <thead>
        <tr>
          {#each columns as col (col.key)}
            <th
              scope="col"
              style:width={col.width}
              style:text-align={col.align ?? "left"}
              aria-sort={sortKey === col.key
                ? sortDir === "asc"
                  ? "ascending"
                  : "descending"
                : undefined}
            >
              {#if col.sortable}
                <button
                  type="button"
                  class="dl-sort"
                  onclick={() => toggleSort(col)}
                >
                  {col.label}
                  <svg
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    class:on={sortKey === col.key}
                    class:desc={sortKey === col.key && sortDir === "desc"}
                  >
                    <path d="m6 15 6-6 6 6" />
                  </svg>
                </button>
              {:else}
                {col.label}
              {/if}
            </th>
          {/each}
          {#if actions.length}
            <th scope="col" class="dl-actions-col">
              <span class="sr">Ações</span>
            </th>
          {/if}
        </tr>
      </thead>

      <tbody>
        {#each visible as row, i (rowKey ? rowKey(row) : ((row as { id?: string | number }).id ?? start + i))}
          <tr
            class:clickable={!!onrowclick}
            tabindex={onrowclick ? 0 : undefined}
            onclick={() => onrowclick?.(row)}
            onkeydown={(e) => onRowKey(e, row)}
          >
            {#each columns as col (col.key)}
              <td
                style:text-align={col.align ?? "left"}
                class:primary={col.primary}
              >
                {#if col.custom && cell}
                  {@render cell(row, col)}
                {:else}
                  <span class="dl-main">{getValue(row, col) ?? "—"}</span>
                  {#if col.sub?.(row)}
                    <span class="dl-sub">{col.sub(row)}</span>
                  {/if}
                {/if}
              </td>
            {/each}

            {#if actions.length}
              <td class="dl-actions-col">
                <div class="dl-actions">
                  {#each actions as a (a.id)}
                    {#if !a.hidden?.(row)}
                      {@const meta = ACTIONS[a.id]}
                      {@const label = a.label ?? meta.label}
                      <button
                        type="button"
                        class="dl-action"
                        class:danger={meta.danger}
                        title={label}
                        aria-label={label}
                        onclick={(e) => {
                          e.stopPropagation();
                          a.onselect(row);
                        }}
                        onkeydown={(e) => e.stopPropagation()}
                      >
                        <svg viewBox="0 0 24 24" aria-hidden="true">
                          {#each meta.nodes as [tag, attrs], idx (idx)}
                            <svelte:element this={tag} {...attrs} />
                          {/each}
                        </svg>
                      </button>
                    {/if}
                  {/each}
                </div>
              </td>
            {/if}
          </tr>
        {:else}
          <tr class="dl-empty-row">
            <td colspan={colCount}>
              <div class="dl-empty">
                {#if rows.length === 0}
                  <strong>{emptyTitle}</strong>
                  {#if emptyText}<span>{emptyText}</span>{/if}
                {:else}
                  <strong>Nenhum resultado</strong>
                  <span>Tente outro termo ou remova os filtros.</span>
                  {#if hasFilters}
                    <button
                      type="button"
                      class="dl-link"
                      onclick={clearFilters}
                    >
                      Limpar filtros
                    </button>
                  {/if}
                {/if}
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  {#if pages > 1}
    <footer class="dl-foot">
      <span class="dl-count">
        {start + 1}–{start + visible.length} de {sorted.length}
      </span>
      <nav class="dl-pager" aria-label="Paginação">
        <button
          type="button"
          disabled={safePage === 1}
          onclick={() => (page = safePage - 1)}
        >
          Anterior
        </button>
        <span aria-current="page">Página {safePage} de {pages}</span>
        <button
          type="button"
          disabled={safePage === pages}
          onclick={() => (page = safePage + 1)}
        >
          Próxima
        </button>
      </nav>
    </footer>
  {/if}
</section>

<style>
  .dl {
    --dl-accent: var(--seno-primary, #1e6cd2);
    --dl-accent-soft: var(--seno-blue-100, #e8f0fd);
    --dl-ink: var(--seno-blue-900, #0d3b7a);
    --dl-text: var(--seno-gray-900, #1a2332);
    --dl-muted: var(--seno-gray-500, #5b6779);
    --dl-line: var(--seno-line, #e2e8f0);
    --dl-surface: var(--seno-white, #ffffff);
    --dl-hover: var(--seno-gray-50, #f9fafc);
    --dl-danger: var(--seno-red, #c53030);

    background: var(--dl-surface);
    border: 1px solid var(--dl-line);
    border-radius: var(--raio-lg, 12px);
    color: var(--dl-text);
    overflow: hidden;
  }

  /* Cabeçalho */
  .dl-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--dl-line);
  }
  .dl-titles h2 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 600;
    color: var(--dl-ink);
  }
  .dl-titles p {
    margin: 2px 0 0;
    font-size: 0.8rem;
    color: var(--dl-muted);
  }
  .dl-head-actions {
    display: flex;
    gap: 8px;
  }

  /* Barra de busca e filtros */
  .dl-toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px 16px;
    padding: 12px 20px;
    border-bottom: 1px solid var(--dl-line);
  }
  .dl-toolbar-extra {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
  }
  .dl-search {
    display: flex;
    align-items: center;
    gap: 8px;
    width: min(100%, 240px);
    height: 36px;
    padding: 0 10px;
    border: 1px solid var(--dl-line);
    border-radius: var(--raio-sm, 6px);
    background: var(--dl-surface);
    color: var(--dl-muted);
  }
  .dl-search:focus-within {
    border-color: var(--dl-accent);
    outline: 2px solid var(--dl-accent-soft);
  }
  .dl-search input {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: 0;
    background: transparent;
    font: inherit;
    font-size: 0.875rem;
    color: var(--dl-text);
  }
  .dl-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
  }
  .dl-tab {
    height: 32px;
    padding: 0 12px;
    border: 0;
    border-radius: var(--raio-sm, 6px);
    background: transparent;
    font: inherit;
    font-size: 0.8125rem;
    color: var(--dl-muted);
    cursor: pointer;
  }
  .dl-tab:hover {
    color: var(--dl-ink);
  }
  .dl-tab[aria-pressed="true"] {
    background: var(--dl-accent-soft);
    color: var(--dl-accent);
    font-weight: 600;
  }

  /* Tabela */
  .dl-scroll {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
  }
  thead th {
    padding: 10px 20px;
    border-bottom: 1px solid var(--dl-line);
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--dl-muted);
    white-space: nowrap;
  }
  tbody td {
    padding: 14px 20px;
    border-bottom: 1px solid var(--dl-line);
    font-size: 0.875rem;
    vertical-align: middle;
  }
  tbody tr:last-child td {
    border-bottom: 0;
  }
  tbody tr.clickable {
    cursor: pointer;
  }
  tbody tr:hover {
    background: var(--dl-hover);
  }
  tbody tr:focus-visible {
    outline: 2px solid var(--dl-accent);
    outline-offset: -2px;
  }
  .dl-main {
    display: block;
  }
  td.primary .dl-main {
    font-weight: 600;
    color: var(--dl-ink);
  }
  .dl-sub {
    display: block;
    margin-top: 2px;
    font-size: 0.78rem;
    color: var(--dl-muted);
  }

  .dl-sort {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    letter-spacing: inherit;
    text-transform: inherit;
    color: inherit;
    cursor: pointer;
  }
  .dl-sort:hover,
  .dl-sort:focus-visible {
    color: var(--dl-ink);
  }
  .dl-sort svg {
    width: 12px;
    height: 12px;
    opacity: 0;
    transition: transform 0.15s;
  }
  .dl-sort:hover svg,
  .dl-sort svg.on {
    opacity: 1;
  }
  .dl-sort svg.desc {
    transform: rotate(180deg);
  }

  /* Ações */
  .dl-actions-col {
    width: 1%;
    white-space: nowrap;
  }
  .dl-actions {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }
  .dl-action {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border: 1px solid var(--dl-line);
    border-radius: var(--raio-sm, 6px);
    background: var(--dl-surface);
    color: var(--dl-muted);
    cursor: pointer;
  }
  .dl-action:hover,
  .dl-action:focus-visible {
    border-color: var(--dl-accent);
    color: var(--dl-accent);
  }
  .dl-action.danger:hover,
  .dl-action.danger:focus-visible {
    border-color: var(--dl-danger);
    color: var(--dl-danger);
  }

  /* Estado vazio */
  .dl-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 40px 16px;
    text-align: center;
    color: var(--dl-muted);
  }
  .dl-empty strong {
    color: var(--dl-ink);
  }
  .dl-link {
    margin-top: 8px;
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    font-weight: 600;
    color: var(--dl-accent);
    cursor: pointer;
  }
  tr.dl-empty-row:hover {
    background: transparent;
  }

  /* Rodapé / paginação */
  .dl-foot {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 8px 16px;
    padding: 10px 20px;
    border-top: 1px solid var(--dl-line);
    font-size: 0.8125rem;
    color: var(--dl-muted);
  }
  .dl-pager {
    display: flex;
    align-items: center;
    gap: 14px;
  }
  .dl-pager button {
    padding: 4px 0;
    border: 0;
    background: none;
    font: inherit;
    font-weight: 600;
    color: var(--dl-accent);
    cursor: pointer;
  }
  .dl-pager button:disabled {
    color: var(--dl-muted);
    opacity: 0.5;
    cursor: default;
  }

  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }

  svg {
    width: 16px;
    height: 16px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
    flex: none;
  }

  /* Utilitários opcionais para quem usa o componente */
  :global(.dl-btn) {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 36px;
    padding: 0 14px;
    border: 0;
    border-radius: var(--raio-sm, 6px);
    background: var(--seno-primary, #1e6cd2);
    font: inherit;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--seno-white, #ffffff);
    cursor: pointer;
  }
  :global(.dl-btn:hover) {
    background: var(--seno-blue-700, #175bb0);
  }
  :global(.dl-badge) {
    display: inline-block;
    padding: 2px 8px;
    border-radius: var(--raio-sm, 4px);
    background: var(--seno-gray-100, #f4f6f9);
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--seno-gray-700, #44505f);
  }
  :global(.dl-badge[data-tone="success"]) {
    background: var(--seno-green-50, #e6f4ea);
    color: var(--seno-green, #276749);
  }
  :global(.dl-badge[data-tone="info"]) {
    background: var(--seno-blue-100, #e8f0fd);
    color: var(--seno-blue-700, #175bb0);
  }
  :global(.dl-badge[data-tone="warning"]) {
    background: var(--seno-amber-50, #fdf1d8);
    color: var(--seno-amber, #8a4b08);
  }
  :global(.dl-badge[data-tone="danger"]) {
    background: var(--seno-red-50, #fdecea);
    color: var(--seno-red-700, #a32424);
  }

  @media (max-width: 640px) {
    .dl-head,
    .dl-toolbar,
    thead th,
    tbody td,
    .dl-foot {
      padding-left: 14px;
      padding-right: 14px;
    }
    .dl-toolbar-extra {
      margin-left: 0;
    }
  }
</style>
