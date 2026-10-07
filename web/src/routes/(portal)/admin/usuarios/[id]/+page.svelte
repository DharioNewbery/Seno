<script lang="ts">
  import UsuarioDetalhe from "#lib/components/UsuarioDetalhe.svelte";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();
</script>

<UsuarioDetalhe
  usuario={data.alvo}
  usuarioId={data.usuario.id}
  eSuper={data.usuario.roles.includes("super")}
  {form}
/>
