<script lang="ts">
  // Marca do Seno: eixos do plano cartesiano e função seno de 0 até pi/2.
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
  <line
    x1="9"
    y1="6"
    x2="9"
    y2="26"
    stroke="var(--seno-blue-200)"
    stroke-width="1.5"
    stroke-linecap="round"
  />
  <line
    x1="6"
    y1="23"
    x2="26"
    y2="23"
    stroke="var(--seno-blue-200)"
    stroke-width="1.5"
    stroke-linecap="round"
  />
  <path
    d="M9 23 C14.46 15, 18.54 9, 24 9"
    stroke="var(--seno-white)"
    stroke-width="2.4"
    stroke-linecap="round"
    fill="none"
  />
</svg>
