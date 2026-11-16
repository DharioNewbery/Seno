# Deploy via GitHub (GHCR), Microsserviços e Serviço de E-mail

Este guia descreve como publicar versões do **Seno** usando GitHub Actions + GitHub Container Registry (GHCR) e como configurar todos os microsserviços e o serviço de e-mail (SMTP) no servidor de produção.

---

## 1. Arquitetura dos Microsserviços em Produção

O arquivo [`deploy/compose.yaml`](../deploy/compose.yaml) orquestra os serviços abaixo na mesma rede Docker:

| Serviço | Imagem / Origem | Porta Interna | Responsabilidade |
| --- | --- | --- | --- |
| `caddy` | `caddy:2.9-alpine` | `80`, `443` | Terminação TLS e proxy reverso único ([`Caddyfile`](../deploy/caddy/Caddyfile)) |
| `web` | `seno-web` (SvelteKit / Node) | `3000` | BFF e interface web do Portal (Aluno, Professor, Admin) |
| `api` | `seno-api` (Go) | `8080` | Domínio acadêmico, autenticação, correções e disparo de e-mails |
| `monitor` | `seno-monitor` (Go + assets embebidos) | `8081` | Ambiente isolado de realização de provas/atividades e ensaios |
| `postgres` | `postgres:17-alpine` | `5432` | Armazena os bancos `seno` (API) e `seno_monitor` (Monitor) |
| `server` / `workers` / `db` / `redis` | `judge0` ([`deploy/judge0/compose.yaml`](../deploy/judge0/compose.yaml)) | `2358` | Sandbox de compilação e execução de código |
| `backup` | `postgres:17-alpine` | — | Sidecar de backup diário (`pg_dump` de `seno` e `seno_monitor`, retenção de 120 dias) |

### Roteamento do Proxy Reverso (`caddy`)

Conforme [`deploy/caddy/Caddyfile`](../deploy/caddy/Caddyfile):
- `/monitor*`, `/a/*` e `/entrar*` → `monitor:8081`
- Demais rotas (`/*`) → `web:3000` (que se comunica internamente com `api:8080` via `SENO_API_URL`)

Os certificados TLS devem estar montados em `deploy/caddy/certs/fullchain.pem` e `deploy/caddy/certs/privkey.pem`.

---

## 2. Pipeline de Build e Release no GitHub

O workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml) é disparado automaticamente ao enviar uma tag `v*` (por exemplo, `v0.2.0`):

1. **Build & Push no GHCR** de três imagens:
   - `ghcr.io/<owner>/seno-api` (contexto na raiz `.` usando `api/Dockerfile`, pois importa `contract/`)
   - `ghcr.io/<owner>/seno-monitor` (contexto na raiz `.` usando `monitor/Dockerfile`, pois importa `contract/`)
   - `ghcr.io/<owner>/seno-web` (contexto em `web/` usando `web/Dockerfile` e `SENO_SITE_ORIGIN`)
2. **Tags semânticas geradas** automaticamente: `0.2.0`, `0.2`, `0` e `latest`.
3. **GitHub Release** criada automaticamente com notas geradas (`softprops/action-gh-release@v2`).

### Variável recomendada no repositório GitHub

Em **Settings → Secrets and variables → Actions → Variables**, configure:
- `SENO_SITE_ORIGIN`: origem pública da aplicação (ex.: `https://seno.ufpa.br`).
  - Importante porque o SvelteKit (`adapter-node`) grava a origem no build para validação de CSRF em *form actions*. Caso não seja definida, o workflow usa `https://seno.ufpa.br`.

---

## 3. Configuração dos Microsserviços (`deploy/.env`)

Copie [`deploy/.env.example`](../deploy/.env.example) para `deploy/.env` no servidor e substitua todos os segredos de desenvolvimento.

### 3.1 Banco de Dados (`postgres`)
```env
POSTGRES_DB=seno
POSTGRES_USER=seno
POSTGRES_PASSWORD=TROQUE-POR-SENHA-FORTE
```
> [!IMPORTANT]
> O banco `seno` é criado automaticamente pelo container `postgres`. Na primeira subida em produção, garanta também a criação do banco `seno_monitor` caso ainda não exista:
> ```bash
> docker compose -f deploy/compose.yaml --env-file deploy/.env exec postgres \
>   psql -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE seno_monitor;"
> ```
> As migrações SQL de `seno` e `seno_monitor` são executadas automaticamente no boot de `api` e `monitor`.

### 3.2 API (`api`) e Integração com Monitor / Judge0
```env
SENO_ADDR=:8080
SENO_SESSION_TTL=12h
SENO_SESSION_MAX_TTL=168h
SENO_SUPER_EMAIL=admin@seudominio.edu.br
SENO_SUPER_PASSWORD=TROQUE-POR-SENHA-FORTE

SENO_DOMAIN=seno.ufpa.br
SENO_WEB_ORIGIN=https://seno.ufpa.br
SENO_MONITOR_PUBLIC_URL=https://seno.ufpa.br/monitor
SENO_POLL_ESTADO_S=30

# Segredo HMAC compartilhado entre API e Monitor (>= 32 bytes)
SENO_MONITOR_SECRET=TROQUE-POR-SEGREDO-ALEATORIO-DE-32-BYTES-OU-MAIS

# Chaves de callback do Judge0
SENO_JUDGE0_CALLBACK_KEY=TROQUE-POR-CHAVE-API-JUDGE0
SENO_JUDGE0_CALLBACK_SECRET=TROQUE-POR-CHAVE-MONITOR-JUDGE0
```
No `compose.yaml`, os endereços internos entre containers já são fixados automaticamente:
- `api` → `monitor`: `http://monitor:8081`
- `api` / `monitor` → `judge0`: `http://server:2358`
- `judge0` → `api`: `http://api:8080`
- `judge0` → `monitor`: `http://monitor:8081`
- `web` → `api`: `http://api:8080`

### 3.3 Monitor (`monitor`)
```env
SENO_MONITOR_ADDR=:8081
SENO_COOKIE_SECURE=true
SENO_WEB_URL=https://seno.ufpa.br
SENO_MONITOR_HMAC_JANELA_S=60
```
*(Em produção atrás do Caddy com HTTPS, `SENO_COOKIE_SECURE` deve permanecer `"true"`.)*

### 3.4 Versão das Imagens GHCR
```env
GHCR_OWNER=dharionewbery
SENO_VERSION=0.2.0
```

---

## 4. Configuração do Serviço de E-mail (SMTP)

O serviço de e-mail da API ([`api/internal/mail/mail.go`](../api/internal/mail/mail.go)) é usado para **convites de matrícula/cadastro** e **recuperação de senha**.

### Variáveis de ambiente

| Variável | Obrigatória | Padrão | Descrição |
| --- | --- | --- | --- |
| `SENO_SMTP_HOST` | Sim (prod) | `""` | Endereço do servidor SMTP (ex.: `smtp.ufpa.br`, `smtp.gmail.com`, `smtp.resend.com`) |
| `SENO_SMTP_PORT` | Não | `587` | Porta SMTP (`587` com STARTTLS é o padrão suportado por `net/smtp`) |
| `SENO_SMTP_USER` | Condicional | `""` | Usuário para autenticação `AUTH PLAIN` (se vazio, envia sem autenticação — útil em relays internos confiáveis) |
| `SENO_SMTP_PASSWORD` | Condicional | `""` | Senha ou App Password do usuário SMTP |
| `SENO_SMTP_FROM` | Recomendado | `seno@ufpa.br` | Endereço remetente (`From:`) exibido nos e-mails |

### Comportamento e Fallback Operacional

A seleção do canal em [`mail.Choose`](../api/internal/mail/mail.go#L79-L87) segue esta ordem:
1. **`SENO_SMTP_HOST` preenchido**: usa `SMTPSender` (`net/smtp` com STARTTLS automático quando suportado pelo servidor e `AUTH PLAIN` se `SENO_SMTP_USER` estiver definido).
2. **`MAIL_DEV=true` (apenas desenvolvimento)**: envia para `localhost:1025` (MailPit).
3. **Sem SMTP configurado ou erro de envio**: aciona automaticamente `LogSender` (`SendWithFallback`), gravando o assunto e o link de convite/recuperação no log de auditoria (`kind = mail.fallback`) para que a operação não falhe silenciosamente nem bloqueie o cadastro.

### Exemplo para Produção

**Com autenticação SMTP (porta 587 / STARTTLS):**
```env
SENO_SMTP_HOST=smtp.seuprovedor.com
SENO_SMTP_PORT=587
SENO_SMTP_USER=no-reply@seno.ufpa.br
SENO_SMTP_PASSWORD=sua-senha-ou-token-smtp
SENO_SMTP_FROM=no-reply@seno.ufpa.br
```

**Com relay SMTP institucional sem autenticação na rede interna (porta 25 ou 587):**
```env
SENO_SMTP_HOST=relay.ufpa.br
SENO_SMTP_PORT=25
SENO_SMTP_USER=
SENO_SMTP_PASSWORD=
SENO_SMTP_FROM=seno@ufpa.br
```

---

## 5. Passo a Passo do Deploy no Servidor

1. **Clonar o repositório** (contém `deploy/` e `caddy/Caddyfile`):
   ```bash
   git clone https://github.com/DharioNewbery/Seno.git
   cd Seno
   ```

2. **Autenticar no GHCR** (necessário se os pacotes estiverem privados):
   ```bash
   echo "$GHCR_PAT" | docker login ghcr.io -u DharioNewbery --password-stdin
   ```

3. **Configurar `deploy/.env` e certificados TLS** em `deploy/caddy/certs/`.

4. **Baixar as imagens da release e subir a stack**:
   ```bash
   docker compose -f deploy/compose.yaml -f deploy/compose.ghcr.yaml --env-file deploy/.env pull
   docker compose -f deploy/compose.yaml -f deploy/compose.ghcr.yaml --env-file deploy/.env up -d
   ```

5. **Verificar saúde dos serviços**:
   ```bash
   docker compose -f deploy/compose.yaml -f deploy/compose.ghcr.yaml ps
   docker compose -f deploy/compose.yaml -f deploy/compose.ghcr.yaml logs -f api monitor web caddy
   ```

### Como publicar uma nova versão no GitHub

```bash
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```
Após a conclusão do workflow **Release** no GitHub Actions, atualize `SENO_VERSION=0.2.0` no `deploy/.env` do servidor e rode `pull` + `up -d`.
