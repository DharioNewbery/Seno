<script lang="ts">
  import Usuarios from "#lib/components/Usuarios.svelte";
  import type { PageProps } from "./$types";

  let { data, form }: PageProps = $props();
</script>

<Usuarios
  usuarios={data.usuarios}
  total={data.total}
  atual={data.pagina}
  por={data.por}
  busca={data.busca}
  filtroStatus={data.status}
  filtroCargo={data.cargo}
  erro={data.erro}
  {form}
  usuarioId={data.usuario.id}
/>
