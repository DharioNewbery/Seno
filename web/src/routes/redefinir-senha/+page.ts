import type { PageLoad } from "./$types";

// O link do e-mail chega como /redefinir-senha?token=xxx (recuper.go).
export const load: PageLoad = ({ url }) => ({
  token: url.searchParams.get("token") ?? "",
});
