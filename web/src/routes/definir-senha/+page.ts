import type { PageLoad } from "./$types";

// O link do e-mail chega como /definir-senha?token=xxx (ARQUITETURA §5.3).
export const load: PageLoad = ({ url }) => ({
  token: url.searchParams.get("token") ?? "",
});
