<script lang="ts">
  import EmConstrucao from "#lib/components/EmConstrucao.svelte";
  import { PORTAIS } from "#lib/portais";
  import type { PageProps } from "./$types";

  let { params }: PageProps = $props();

  const caminho = $derived(
    (params.caminho ?? "").split("/").join(" — ").replace(/-/g, " "),
  );
</script>

<EmConstrucao portal={PORTAIS.professor} caminho={caminho} />
