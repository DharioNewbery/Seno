<script lang="ts">
  import CriarUsuario from "#lib/components/CriarUsuario.svelte";
  import type { PageProps } from "./$types";

  let { form }: PageProps = $props();
</script>

<CriarUsuario cargo="admin" {form} />
