// Caller do BFF para a API: base URL vem do ambiente (SENO_API_URL).
const API_URL: string = process.env.SENO_API_URL ?? "http://api:8080";

interface ApiInit extends Omit<RequestInit, "headers"> {
  headers?: Headers;
}

/** Caminho na API, ex.: `/v1/auth/login` */
type ApiPath = string;

/**
 * Faz uma chamada server-side à API encaminhando a requisição do navegador:
 * método, corpo e cookie de sessão (o token nunca sai do BFF além disso).
 * @param path caminho na API, ex.: `/v1/auth/login`
 * @param request requisição do navegador (método/corpo/cookie)
 * @param init opções fetch (headers extras, ex.: Authorization)
 */
export async function apiFetch(
  path: ApiPath,
  request: Request,
  init: ApiInit = {},
): Promise<Response> {
  const headers = new Headers(init.headers);
  const origin =
    process.env.SENO_WEB_ORIGIN?.trim().replace(/\/+$/, "") ||
    new URL(request.url).origin;
  headers.set("Origin", origin);

  for (const h of [
    "x-forwarded-for",
    "x-forwarded-proto",
    "x-forwarded-host",
    "user-agent",
  ]) {
    if (!headers.has(h)) {
      const val = request.headers.get(h);
      if (val) headers.set(h, val);
    }
  }

  const cookie = request.headers.get("cookie");
  if (cookie) headers.set("Cookie", cookie);

  const method = init.method ?? request.method;
  const corpo =
    init.body ??
    (method === "GET" || method === "HEAD" ? undefined : await request.text());
  if (corpo !== undefined) headers.set("Content-Type", "application/json");

  return fetch(`${API_URL}${path}`, { ...init, method, body: corpo, headers });
}
