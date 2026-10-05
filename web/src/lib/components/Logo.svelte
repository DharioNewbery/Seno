<script lang="ts">
  // Marca do Seno: SVG placeholder (onda senoidal em quadro arredondado).
  let { tamanho = 28 }: { tamanho?: number } = $props();
</script>

<svg
  width={tamanho}
  height={tamanho}
  viewBox="0 0 32 32"
  fill="none"
  role="img"
  aria-label="Logo do Seno"
>
  <rect x="1" y="1" width="30" height="30" rx="7" fill="var(--seno-blue-600)" />
  <path
    d="M5 17.5 C7.5 11, 10.5 11, 13 17.5 S18.5 24, 21 17.5 S26 11.5, 27 14"
    stroke="var(--seno-white)"
    stroke-width="2.2"
    stroke-linecap="round"
    fill="none"
  />
</svg>
