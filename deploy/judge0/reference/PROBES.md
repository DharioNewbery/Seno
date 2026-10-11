# Judge0 — extração ao vivo da instância (Fase 5)

Fonte: instância rodando em dev (compose em `deploy/judge0/compose.yaml`).
**Não havia `/openapi.json` no Judge0 1.13.0** — a documentação oficial é a
página HTML pré-renderizada em `/docs` (snapshot em `docs-1.13.0.html`,
capturado da instância). O que segue foi verificado com probes reais na
instância (hoje 2026-10-09; imagens: `mrkushalsm/judge0:cgv2`).

## Endpoints verificados

| Verbo | Rota | Comportamento confirmado |
|---|---|---|
| POST | `/submissions?wait=true&base64_encoded=false` | **executa e devolve resultado inline** |
| POST | `/submissions/batch?wait=true` (envelope `{"submissions":[…]}`) | **devolve SÓ TOKENS** — não executa inline (diferente do que o nome sugere; o cliente antigo `api/internal/judge0.EnviarBatchSincrono` assume inline e está errado para esta versão) |
| POST | `/submissions/batch` (async) | tokens; resultado via polling/callback |
| GET  | `/submissions/{token}` | status objeto `{id, description}`; `stdout` cru com `base64_encoded=false` |
| GET  | `/system_info`, `/config_info`, `/languages` | diagnósticos/configs vivos |
| PUT  | `callback_url` | **PUT** com corpo=submissão completa, query preservada, **sem header de autenticação** — segredo deve ir embutido na própria URL (`?key=…`) |

## Corpo do callback (capturado ao vivo)

```json
{"stdout":"NDIK\n","time":"0.011","memory":3240,"stderr":null,
 "token":"…","compile_output":null,"message":null,
 "status":{"id":3,"description":"Accepted"}}
```
Atenção: no callback o `stdout` veio **base64** ("NDIK\n" = "42\n") mesmo
enviando `base64_encoded=false` na criação - o Monitor decodifica no callback.
Atenção 2: `memory` (p.ex. 3240) chega como número no GET e no callback -
não como string. O struct `Resposta` do cliente Go decodifica os dois
(tipo `NumStr`); antes disto o polling do fallback engolia
`json: cannot unmarshal number into ... memory of type string` silenciosamente
e o run nunca fechava - bug real preso no e2e, achado e consertado 2026-10-09.

## Limites/config relevantes (config_info.json no mesmo diretório)

- `enable_callbacks: true`, `callbacks_max_tries: 3`, `callbacks_timeout: 5s`
- `max_submission_batch_size: 20`, `enable_batched_submissions: true`
- limites default: `cpu 5s (max 15)`, `wall 10s (max 20)`, `memória 128 MB (max 512)`,
  `stack 64 MB`, `max_processes 60`, `max_file_size 1 MB`
- resposta de pré-avaliação do Monitor usa comparação exata via
  `expected_output` (status 3 = accepted)

## Imagem (dev)

- Oficial `judge0/judge0:1.13.0` é **cgroups v1 only** (isolate 1.8.1) e
  falha em Docker Desktop/WSL2 (`/box/script.py` → status 13).
- Dev usa `mrkushalsm/judge0:cgv2` (fork da comunidade, judge0 #549) com
  `/sys/fs/cgroup:/sys/fs/cgroup:rw`. Produção: revisar no host Linux real
  (PLANO_MONITOR Fase 8) — ideal oficial com cgroups v1, ou fork auditado.

## Probe bruto (registro)

```
POST /submissions?wait=true&base64_encoded=false
{"language_id":71,"source_code":"print(int(input())*2)","stdin":"3",
 "expected_output":"6","cpu_time_limit":2}
→ {"stdout":"6\n","time":"0.02","memory":20404,...,"status":{"id":3,"description":"Accepted"}}

POST /submissions/batch?wait=true
{"submissions":[{...},{...}]}
→ [{"token":"..."},{"token":"..."}]   # NÃO inline
```
