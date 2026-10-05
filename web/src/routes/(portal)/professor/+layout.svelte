<script lang="ts">
  import PortalShell from "#lib/components/PortalShell.svelte";
  import { PORTAIS } from "#lib/portais";
  import type { LayoutProps } from "./$types";

  let { data, children }: LayoutProps = $props();
</script>

<PortalShell portal={PORTAIS.professor} usuario={data.usuario}>
  {@render children()}
</PortalShell>
