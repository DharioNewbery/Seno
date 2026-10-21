<script lang="ts">
  // Ícones de traço do menu lateral (24×24, herdam a cor do texto).
  import type { IconeNav } from "#lib/portais";

  let { nome, tamanho = 18 }: { nome: IconeNav; tamanho?: number } = $props();
</script>

<svg
  width={tamanho}
  height={tamanho}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.7"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  {#if nome === "inicio"}
    <path d="M3.5 10.5 12 3.5l8.5 7V19a1.5 1.5 0 0 1-1.5 1.5h-4v-6h-6v6H5A1.5 1.5 0 0 1 3.5 19z" />
  {:else if nome === "turmas"}
    <circle cx="9" cy="8" r="3.2" />
    <path d="M3 20v-1a4.5 4.5 0 0 1 4.5-4.5h3A4.5 4.5 0 0 1 15 19v1" />
    <path d="M16 4.9a3.2 3.2 0 0 1 0 6.2M18.5 14.7A4.5 4.5 0 0 1 21 19v1" />
  {:else if nome === "tarefas"}
    <rect x="5" y="4" width="14" height="17" rx="2" />
    <path d="M9 4V3h6v1M9 13l2 2 4-4.5" />
  {:else if nome === "atividades"}
    <path d="m8 8-4 4 4 4M16 8l4 4-4 4M13.5 5.5l-3 13" />
  {:else if nome === "dashboard"}
    <path d="M4 20v-6M10 20V5M16 20v-9M21 20H3" />
  {:else if nome === "usuarios"}
    <circle cx="12" cy="8" r="3.8" />
    <path d="M4.5 20.5a7.5 7.5 0 0 1 15 0" />
  {:else if nome === "materias"}
    <path d="M5 4.5h10.5A2.5 2.5 0 0 1 18 7v13H7.5A2.5 2.5 0 0 1 5 17.5zM5 17.5A2.5 2.5 0 0 1 7.5 15H18" />
  {:else if nome === "periodos"}
    <rect x="3.5" y="5" width="17" height="15.5" rx="2" />
    <path d="M3.5 10h17M8 3v4M16 3v4" />
  {/if}
</svg>
