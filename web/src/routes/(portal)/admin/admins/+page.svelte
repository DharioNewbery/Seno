<script lang="ts">
  import GestaoUsuarios from "#lib/components/GestaoUsuarios.svelte";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();
</script>

<GestaoUsuarios
  cargo="admin"
  usuarios={data.usuarios}
  total={data.total}
  atual={data.pagina}
  por={data.por}
  busca={data.busca}
  filtroStatus={data.status}
  erro={data.erro}
  {form}
  eSuper={data.usuario.roles.includes("super")}
  usuarioId={data.usuario.id}
/>
