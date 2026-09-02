# Diretrizes de desenvolvimento — Seno API

Projeto: API RESTful em Go para autenticação, usuários e controle de acesso baseado em papéis (RBAC).

## Stack

- **Linguagem:** Go 1.26+
- **Roteador:** `github.com/go-chi/chi/v5`
- **Banco de dados:** PostgreSQL 17 (via Docker)
- **Acesso a dados:** `github.com/jmoiron/sqlx` + driver `github.com/jackc/pgx/v5/stdlib` (nome do driver: `pgx`)
- **Autenticação:** JWT (`github.com/golang-jwt/jwt/v5`) + bcrypt (`golang.org/x/crypto/bcrypt`)
- **UUIDs:** `github.com/google/uuid`
- **Correção automática:** cliente Docker `github.com/moby/moby/client` (API em `github.com/moby/moby/api`)

## Regra de idioma (OBRIGATÓRIA)

- **Inglês:** nomes de variáveis, funções, tipos, structs, tabelas, colunas, pacotes, mensagens de log estruturado/erro interno (`fmt.Errorf`).
- **Português:** mensagens de erro de domínio expostas ao usuário, respostas da API, mensagens em `log.Printf` voltadas ao operador, comentários.

Exemplo:
```go
// CORRETO
func (r *UserRepository) GetByEmail(...) error {
    return fmt.Errorf("erro ao buscar usuário por email: %w", err) // msg interna em PT, wrapper em EN
}
// Erro de domínio (exposto ao usuário) em PT:
var ErrUserNotFound = errors.New("usuário não encontrado")
```

## Estrutura de camadas (desacoplamento)

Fluxo obrigatório: **handler → service → repository**. Nunca pular camadas.

```
cmd/api/main.go            # Composição de dependências (DI) e bootstrap
internal/
  config/                  # Carregamento de configuração (.env)
  database/                # Pool de conexão + runner de migrações (embed)
    migrations/*.sql       # Scripts SQL versionados
  models/                  # Entidades de domínio (structs + tags db/json)
  repositories/            # Acesso a dados (sqlx). Implementam interfaces de services.
  services/                # Regras de negócio. Definem as interfaces (ports.go).
    ports.go               # Interfaces de repositórios (consumer-defined)
  handlers/                # Camada HTTP: parse, validação leve, mapeamento de erros
  middleware/              # Auth (RequireAuth), RBAC (RequireRole/RequirePermission)
  server/                  # Montagem do chi router + middlewares + rotas
  utils/                   # jwt, password (bcrypt), hash (sha256)
pkg/response/              # Helper de resposta JSON padrão (Body{success,message,data,error})
web/                       # Interface web estática (embed em web.go) + STYLEGUIDE.md
```

### Princípios

- **Services dependem de interfaces** definidas em `services/ports.go`, nunca de structs concretos de `repositories`.
- **Handlers não acessam repositórios** nem o banco diretamente; só services.
- **Repositories não conhecem HTTP** nem regras de negócio.
- **Models** são compartilhados entre camadas (sem dependência de infraestrutura).
- Erros sentinelas ficam em `repositories/errors.go` (dados) e `services/errors.go` (negócio).

## Convenções de código

- Pacote `internal` para tudo que não deve ser importado externamente; `pkg/` só para utilitários reutilizáveis.
- Receptor ponteiro (`*Type`) em métodos que mutam estado ou acessam `db`.
- Nomes: `UserRepository`, `NewUserRepository`, `GetByEmail`, `toUserResponse`.
- Funções de conversão DTO: `toXxxResponse`. DTOs em `handlers/dto.go`.
- Sem comentários explicativos óbvios; comentar apenas decisões não triviais (em PT).
- Sem `panic` em camadas de domínio; usar `log.Fatalf` só em `main.go`.
- Sem segredos no código; carregar de variáveis de ambiente (`.env` local via godotenv).

## Banco de dados

- Chaves primárias: `UUID DEFAULT gen_random_uuid()`.
- Timestamps: `TIMESTAMPTZ NOT NULL DEFAULT NOW()`. Soft delete via `deleted_at TIMESTAMPTZ`.
- Nomes de tabela em plural inglês: `users`, `user_credentials`, `roles`, `permissions`, `role_permissions`, `user_roles`, `refresh_tokens`.
- Colunas em snake_case inglês: `full_name`, `created_at`, `failed_login_attempts`.
- Queries em arquivos `.sql` com indentação consistente; usar `$1, $2...` para placeholders.
### Migrações

- Arquivos em `internal/database/migrations/`, nomeados `NNN_descricao.sql` (ex.: `001_init.sql`).
- Aplicadas automaticamente no startup via `database.Migrate` (embed + tabela `schema_migrations`).
- O runner divide o script por `;` e remove comentários `--` (suficiente para DDL; **não usar funções PL/pgSQL** com `;` internos nessas migrações).
- Para adicionar uma migration: criar novo arquivo `NNN_xxx.sql` seguindo a numeração. Não editar migrations já aplicadas em produção.

## Autenticação e RBAC

- `Register`: autocadastro público de **aluno** — usuário + credencial (bcrypt) + vínculo em `students` + papel `student`, em transação única (`StudentRepository.CreateWithAccount`). Professores são criados pelo SUPER.
- `Login`: valida senha, controla `failed_login_attempts` (bloqueia após 5), emite par de tokens (access + refresh).
- `Refresh`: valida o refresh token (JWT + tabela `refresh_tokens`), revoga o antigo e emite novo par (rotação).
- Refresh tokens são armazenados como **hash SHA-256** (revogáveis sem expor o token).
- Middlewares: `RequireAuth` (Bearer), `RequireRole(roleChecker, "admin")`, `RequirePermission(roleChecker, "users:read")`.
- **Superusuário:** `SeedService.EnsureSuperUser` roda no startup (após `Migrate`) e garante o usuário SUPER (login via `SUPER_LOGIN`, senha temporária via `SUPER_PASSWORD`). Idempotente: não sobrescreve senha.
- Login aceita **email ou username** (`users.username`, normalizado lowercase). SUPER é a única conta com username no MVP.
- `ChangePassword`: verifica senha atual, atualiza e **revoga todos os refresh tokens** (outras sessões caem).
- Cadastro de professores: transação única em `professors` (composição 1:1 com `users`) — `ProfessorRepository.CreateWithAccount`.
- Criação de contas com perfil (professores/alunos) compartilha o helper transacional `repositories/account.go` (`createProfiledAccount`).
- **Turmas:** professor cria (`POST /classes`) com `join_code` gerado pelo serviço (6 caracteres, sem ambíguos, retry em colisão); aluno ingressa via `POST /classes/join` (valida papel `student`, idempotente); listagens por papel (`GET /classes` do professor, `GET /classes/mine` do aluno). Associação N:M em `class_members`.
- **Tarefas e submissões:** professor publica em `POST /classes/{id}/assignments` (título, enunciado, linguagem `python|c|cpp`, prazo opcional RFC3339 e **casos de teste** — entrada + saída esperada, 1..20 por tarefa, IO ≤8KB); aluno vê feed (`GET /assignments/mine`), detalhe (`GET /assignments/{id}` — casos de teste e submissões embutidas conforme papel) e submete código pela IDE web (`POST /assignments/{id}/submissions`, linguagem herdada da tarefa, status inicial `pending`). IDE: CodeMirror 5 embutido em `web/static/vendor` (sem build).
- **Correção automática (milestone 6):** um worker interno (`internal/worker`, ticker + batch, config `RUNNER_*`) consome submissões `pending` e executa o código em containers efêmeros (`internal/runner`, cliente Docker `moby/moby/client`). Cada caso de teste roda isolado: sem rede, `128MB` RAM, 1 CPU, 64 pids, `CapDrop ALL`, `no-new-privileges`, usuário `nobody` (65534), timeout por linguagem (`python` 10s, `c`/`cpp` 30s incluindo compilação). Fonte e entrada são injetadas via `CopyToContainer` (tar) e a saída é capturada por logs (stdout/stderr, 64KB cada), sem bind mounts. Imagens `python:3.12-alpine` e `gcc:13-alpine` são baixadas no primeiro uso (pull com timeout de 5min, fora do timeout da execução). Comparação normalizada (CRLF→LF, trim de espaços finais por linha e de linhas vazias). Resultado gravado como JSON em `submissions.result` (`{tests: [{position, passed, input, expected, got, stderr, timed_out, exit_code, duration_ms}], summary: {passed, total}}`); status final `passed`/`failed`/`error`. Erros de infraestrutura mantêm a submissão `pending` (nova tentativa no próximo tick). O worker só inicia com `RUNNER_ENABLED=true` (requer daemon Docker acessível).
- **Editor com backup em tempo real:** rascunho do aluno salvado em duas camadas — `localStorage` (`seno.draft.<id>`, debounce 800ms, formato `{code, savedAt}`) e servidor (`PUT /assignments/{id}/draft`, upsert em `drafts`, sync debounced 3s + `keepalive` no `pagehide`). Restauração usa o mais recente das duas fontes; "Carregar no editor" também sincroniza.

## Padrão de resposta HTTP

Todo endpoint usa `pkg/response`:
```go
{ "success": true, "message": "...", "data": {...} }   // 2xx
{ "success": false, "error": "..." }                   // 4xx/5xx
```
Mapeamento de erro de domínio → status HTTP centralized em `handlers/errors.go` (`mapError`).

## Comandos

```bash
# Dependências
go mod tidy

# Formatar e validar
gofmt -w .
go build ./...
go vet ./...

# Subir o banco (requer Docker Desktop em execução)
docker compose up -d postgres

# Rodar a API (carrega .env se existir; há defaults de dev)
go run ./cmd/api

# Build do binário
go build -o bin/seno-api ./cmd/api
```

## Endpoints iniciais

| Método | Rota               | Auth  | Descrição                          |
|--------|--------------------|-------|-----------------------------------|
| GET    | /health            | —     | Healthcheck                       |
| GET    | /routes            | —     | Lista todas as rotas registradas  |
| GET    | /                  | —     | Interface web (login)             |
| POST   | /api/v1/auth/register | —  | Cadastrar aluno (autocadastro)    |
| POST   | /api/v1/auth/login    | —  | Autenticar (email ou username)    |
| POST   | /api/v1/auth/refresh  | —  | Renovar tokens                    |
| GET    | /api/v1/auth/me       | Bearer | Dados do usuário autenticado   |
| POST   | /api/v1/auth/change-password | Bearer | Trocar a própria senha (revoga sessões) |
| GET    | /api/v1/users         | Bearer | Listar usuários                 |
| GET    | /api/v1/users/{id}    | Bearer | Obter usuário por id            |
| POST   | /api/v1/professors   | Bearer (super) | Cadastrar professor (senha temporária) |
| GET    | /api/v1/professors   | Bearer (super) | Listar professores              |
| POST   | /api/v1/classes      | Bearer (professor) | Criar turma (gera join_code) |
| GET    | /api/v1/classes      | Bearer (professor) | Listar turmas do professor   |
| POST   | /api/v1/classes/join | Bearer | Ingressar em turma por código     |
| GET    | /api/v1/classes/mine | Bearer | Turmas do aluno autenticado       |
| POST   | /api/v1/classes/{id}/assignments | Bearer (professor) | Publicar tarefa na turma |
| GET    | /api/v1/classes/{id}/assignments | Bearer (professor) | Listar tarefas da turma   |
| GET    | /api/v1/assignments/mine | Bearer | Feed de tarefas do aluno     |
| GET    | /api/v1/assignments/{id} | Bearer | Detalhe da tarefa (submissões conforme papel) |
| PUT    | /api/v1/assignments/{id}/draft | Bearer | Salvar rascunho do editor (backup) |
| POST   | /api/v1/assignments/{id}/submissions | Bearer | Submeter código (IDE web) |

## Como adicionar uma feature (ex.: entidade Product)

1. **Migration:** `internal/database/migrations/002_products.sql`.
2. **Model:** `internal/models/product.go` (struct com tags `db`/`json`).
3. **Repository:** `internal/repositories/product_repository.go` + sentinelas em `repositories/errors.go` se necessário.
4. **Interface:** adicionar `ProductRepository` em `services/ports.go`.
5. **Service:** `internal/services/product_service.go` com regras de negócio + DTOs de entrada (`XxxInput`).
6. **Handler:** `internal/handlers/product_handler.go` + DTOs em `handlers/dto.go`; mapear erros em `handlers/errors.go`.
7. **Rotas:** registrar em `internal/server/server.go` (grupo público ou autenticado).
8. **Wiring:** instanciar repo/service/handler em `cmd/api/main.go`.
9. **Validar:** `gofmt -w . && go build ./... && go vet ./...`.

## Variáveis de ambiente

Ver `.env.example`. Defaults de desenvolvimento permitem `go run` sem `.env`, exceto que `JWT_SECRET` usa um valor de dev que **deve** ser redefinido em produção (`APP_ENV=production` valida isso).

| Variável          | Default     | Descrição                                            |
|-------------------|-------------|------------------------------------------------------|
| `RUNNER_ENABLED`  | `true`      | Liga o worker de correção automática (requer Docker) |
| `RUNNER_INTERVAL` | `5s`        | Intervalo do tick do worker                          |
| `RUNNER_BATCH`    | `5`         | Máximo de submissões corrigidas por tick             |
