# Seno

Envio e correção automática de provas de programação — UFPA. Turmas, tarefas
e atividades por professor; editor de código; correção automática com Judge0
(nota por tarefa) e "prova" pelo serviço Monitor. Stack: SvelteKit (BFF) +
Go (`api`, `monitor`, `contract`) + PostgreSQL.

Documentação de produto e arquitetura em [`docs/`](docs/) (PROJETO,
ARQUITETURA, PLANO_MONITOR_FLUXO_ALUNO).

## Deploy em dev

Dois caminhos **mutuamente exclusivos** (não rode os dois ao mesmo tempo:
briga de portas e de volumes). Antes de trocar de método, pare o método
atual. Dados de banco sobrevivem à troca — o volume só é apagado com `-v`.

### Pré-requisitos (uma vez)

```powershell
# 1) segredos/ambientes (os defaults servem para dev)
Copy-Item deploy\.env.example deploy\.env
Copy-Item deploy\judge0\judge0.conf.example deploy\judge0\judge0.conf

# 2) Docker Desktop ligado (WSL2). Para o método A, também:
#    Go 1.26+ e Node 22+ / npm instalados.
```

O `.env` carrega **todos** os valores de dev — inclusive os serviços nativos
(método A): nada precisa ser setado no terminal. Em dev os valores do
exemplo já funcionam, **mas confira**:

| Chave | Valor para o dev |
|---|---|
| `SENO_DATABASE_URL` (API) | `postgres://…@localhost:5432/seno` (partida do PostgreSQL de dev) |
| `SENO_MONITOR_DSN` (Monitor) | `postgres://…@localhost:5432/seno_monitor` (chave própria — nunca duplique `SENO_DATABASE_URL`) |
| `SENO_JUDGE0_URL` (API e Monitor) | `http://localhost:2358` |
| `SENO_JUDGE0_CALLBACK_URL` (API) | `http://host.docker.internal:8080` — Judge0 → api nativa |
| `SENO_JUDGE0_CALLBACK_KEY` (API) | qualquer segredo (ex.: valor do exemplo) |
| `SENO_JUDGE0_CALLBACK_SECRET` (Monitor) | qualquer segredo (ex.: valor do exemplo) |
| `SENO_MONITOR_URL` / `SENO_MONITOR_PUBLIC_URL` (API) | `http://localhost:8081` |
| `SENO_MONITOR_CALLBACK_URL` (Monitor) | `http://host.docker.internal:8081` — Judge0 → monitor nativo |
| `SENO_WEB_URL` (Monitor) | `http://localhost:5173` |
| `SENO_WEB_ORIGIN` (API e Web/BFF) | `http://localhost:5173` |
| `SENO_API_URL` (Web/BFF) | `http://localhost:8080` |
| `SENO_MONITOR_SECRET` (API e Monitor) | mesmo valor nos dois, ≥ 32 bytes |
| `SENO_COOKIE_SECURE` (Monitor) | `false` (HTTP localhost) |

Nos containers (método B) o `compose.dev.yaml` sobrepõe as chaves acima
com os nomes internos do compose — o `.env` nativo continua válido porque
os overrides ganham.

Migrações, banco `seno_monitor` e o super admin (`SENO_SUPER_EMAIL` /
`SENO_SUPER_PASSWORD`) são criados **automaticamente no boot** da API.

### Método A — terminal nativo (sem build de imagem, com hot reload)

API, web e monitor rodam no Windows via `go run` / `vite dev`; só a
infraestrutura fica em containers.

```powershell
# 1) pare o que estiver de ambos os métodos
& deploy\dev.ps1 -Modo stop
docker compose -f deploy/compose.dev.yaml down   # sem -v para manter os dados

# 2) infra de banco + e-mail + Judge0
docker compose -f deploy/compose.dev.yaml up -d postgres mailpit
docker compose -f deploy/judge0/compose.yaml up -d   # porta 2358

# 3) api (:8080) + web dev (:5173) + monitor (:8081) nativos
& deploy\dev.ps1 -Modo all
```

- **Acesso:** web `http://localhost:5173` · monitor `http://localhost:8081` ·
  MailPit `http://localhost:8025`
- **Login inicial:** `SENO_SUPER_EMAIL` / `SENO_SUPER_PASSWORD` do `.env`
- **Logs nativos:** `%TEMP%\opencode\seno-{api,web,monitor}.log` (+ `.err.log`)
- **Status / fim de sessão:** `& deploy\dev.ps1 -Modo status` /
  `& deploy\dev.ps1 -Modo stop`
- Mudanças em Go: reinicie (`stop` + `all`); o front recarrega sozinho.

### Método A2 — nativo manual (um terminal por serviço, sem `dev.ps1`)

Os serviços leem `deploy/.env` sozinhos no boot (nada de variável no
terminal): Go via `platform.LoadDotEnv` na inicialização e o vite via
`node --env-file`. A lógica: preenche só o que não existe no ambiente
(o compose do método B injeta os valores dele — quem seta antes vence)
e o arquivo ausente é silencioso (nas imagens de container ele não
está presente). A lógica é idêntica à do `dev.ps1`, mas com logs ao
vivo no terminal.

```powershell
# 0) uma vez por sessão (repita sempre que reiniciar o Docker):
docker compose -f deploy/compose.dev.yaml up -d postgres mailpit
docker compose -f deploy/judge0/compose.yaml up -d
```

**Terminal da API** (porta :8080; protagoniza migrações e super admin):

```powershell
cd C:\Users\dhario\UFPA\Seno\api
go run ./cmd/api
```

**Terminal do Web/BFF** (vite dev, porta :5173):

```powershell
cd C:\Users\dhario\UFPA\Seno\web
npm run dev
```

**Terminal do Monitor** (porta :8081; cria o banco `seno_monitor` se faltar):

```powershell
cd C:\Users\dhario\UFPA\Seno\monitor
go run ./cmd/monitor
```

- **Acesso:** web `http://localhost:5173` · monitor `http://localhost:8081` ·
  MailPit `http://localhost:8025` · Judge0 `http://localhost:2358`
- **Parar:** `Ctrl+C` em cada terminal (o `dev.ps1 -Modo stop` também mata
  esses processos, por porta).
- Para o arquivo ser lido, rode os comandos **dentro do monorepo**
  (`api/`, `web/`, `monitor/` — a busca sobe as pastas até achar
  `deploy/.env`). Fora dele os serviços vão dar erro de configuração.
- Reiniciou o Docker? Repita só o passo 0 — os processos nativos
  continuam rodando.

### Método B — tudo em containers

Um comando sobe o pacote completo: postgres (+ banco do monitor), MailPit,
**Judge0, API, monitor e web** já configurados entre si (callbacks internos,
SSO, origem do CSRF, MailPit SMTP).

```powershell
# 1) pare os processos nativos (ocupam 8080/5173/8081)
& deploy\dev.ps1 -Modo stop

# 2) deploy limpo OPCIONAL (apaga dados de todos os bancos!)
docker compose -f deploy/compose.dev.yaml down -v --remove-orphans

# 3) build das imagens (api, monitor, web; judge0 usa imagem pronta)
docker compose -f deploy/compose.dev.yaml build

# 4) subir tudo
docker compose -f deploy/compose.dev.yaml up -d

# 5) conferir
docker compose -f deploy/compose.dev.yaml ps
curl.exe http://localhost:3000/healthz   # web/BFF
curl.exe http://localhost:8081/healthz   # monitor
```

- **Acesso:** web `http://localhost:3000` (única entrada da parte API) ·
  monitor `http://localhost:8081` · MailPit `http://localhost:8025` ·
  Judge0 `http://localhost:2358`
- **Logs:** `docker compose -f deploy/compose.dev.yaml logs -f api monitor`
- **Parar mantendo dados:** `down`; reinício após mudança de código:
  `build api` (ou `web`/`monitor`) + `up -d <serviço>`

### O que muda entre os métodos

| | A (nativo) | B (containers) |
|---|---|---|
| URL do web | `:5173` (dev server) | `:3000` (build de produção) |
| Recarga | Go reinicia manual; front recarrega sozinho | rebuild da imagem |
| Apagar os dados | `down -v` no compose (`-Modo stop` nunca apaga) | idem |

**Limitações comuns:** use sempre `localhost` — cookies e origens não são
válidos por IP de rede local (para LAN entra TLS + Caddy, Fase 9). O Judge0
roda `privileged` com a imagem `cgv2` por causa do cgroups v2 do
Docker Desktop/WSL2; no host Linux real da UFPA reavalie (nota no
`deploy/judge0/compose.yaml`).

## Desenvolvimento

```powershell
go vet ./... ; go test ./...          # em api/, monitor/ e contract/ (testes usam Postgres de dev)
cd web ; npm run check ; npm test     # svelte-check + vitest
sqlc generate                          # em api/ quando queries/*.sql mudar
```

Rotas de produção (`deploy/compose.yaml`: monitor + judge0 + backup,
com segredos do `.env`) entram aqui na Fase 9 do PLANO_MONITOR; o Caddy
com TLS real da UFPA fecha o acesso público.
