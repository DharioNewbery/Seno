// Caller do BFF para a API: base URL vem do ambiente (SENO_API_URL).
const API_URL: string = process.env.SENO_API_URL ?? "http://api:8080";

interface ApiInit extends Omit<RequestInit, "headers"> {
  headers?: Headers;
}

/** Caminho na API, ex.: `/v1/auth/login` */
type ApiPath = string;

/**
 * Faz uma chamada server-side à API propagando o cookie de sessão do navegador.
 * @param path caminho na API, ex.: `/v1/auth/login`
 * @param request requisição do navegador (cookie de sessão)
 * @param init opções fetch
 */
export async function apiFetch(
  path: ApiPath,
  request: Request,
  init: ApiInit = {},
): Promise<Response> {
  const headers = new Headers(init.headers);
  headers.set("Origin", process.env.SENO_WEB_ORIGIN ?? "http://localhost:5173");
  headers.set("Content-Type", "application/json");
  const cookie = request.headers.get("cookie");
  if (cookie) headers.set("Cookie", cookie);

  return fetch(`${API_URL}${path}`, { ...init, headers });
}
