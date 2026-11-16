<script lang="ts">
  import { LUCIDE_ICONS, type NomeIcone } from "#lib/vendor/lucide-icons";

  interface Props {
    nome: NomeIcone;
    tamanho?: number;
    strokeWidth?: number;
    classe?: string;
    titulo?: string;
  }

  let {
    nome,
    tamanho = 18,
    strokeWidth = 1.85,
    classe = "",
    titulo,
  }: Props = $props();

  const nodes = $derived(LUCIDE_ICONS[nome] ?? []);
</script>

<svg
  xmlns="http://www.w3.org/2000/svg"
  width={tamanho}
  height={tamanho}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width={strokeWidth}
  stroke-linecap="round"
  stroke-linejoin="round"
  class={classe || undefined}
  aria-hidden={titulo ? undefined : "true"}
  role={titulo ? "img" : undefined}
  aria-label={titulo}
>
  {#if titulo}
    <title>{titulo}</title>
  {/if}
  {#each nodes as [tag, attrs], i (i)}
    <svelte:element this={tag} {...attrs} />
  {/each}
</svg>
