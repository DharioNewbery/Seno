# Manual Completo de Deploy no Ubuntu Linux (Rede Local e Produção)

Este manual descreve como colocar o **Seno** em funcionamento em um servidor **Ubuntu Linux** (22.04 LTS ou 24.04 LTS) usando o script automatizado de produção ou a configuração manual passo a passo.

---

## Deploy Rápido em 1 Comando (Recomendado)

Com o **Docker Engine**, **Docker Compose v2**, **OpenSSL** e **curl** instalados no Linux:

```bash
git clone https://github.com/DharioNewbery/Seno.git
cd Seno
./deploy/deploy.sh
```

O script [`deploy/deploy.sh`](../deploy/deploy.sh) realiza automaticamente:
1. Geração das chaves secretas fortes (`POSTGRES_PASSWORD`, `SENO_MONITOR_SECRET`, chaves do Judge0 e `deploy/judge0/judge0.conf`).
2. Verificação de certificado SSL em `deploy/caddy/certs/` — caso não exista, oferece gerar um certificado autoassinado (com SAN dinâmico para o IP/domínio da máquina) ou copiar seus arquivos `.pem`.
3. Perguntas simplificadas no terminal apenas para o que ainda não estiver configurado em `deploy/.env` (controlado pela flag `SENO_DEPLOY_CONFIGURED=true`).
4. Build e subida da stack inteira atrás do Caddy em um único endereço público, com suporte a portas customizadas e múltiplas camadas de proxy reverso.

Opções úteis:
- `./deploy/deploy.sh --reconfigure`: reexibe o assistente de configuração mantendo as chaves secretas.
- `./deploy/deploy.sh --ghcr`: puxa as imagens pré-compiladas do GHCR em vez de compilar localmente.
- `./deploy/deploy.sh --rotate-secrets`: regenera todas as chaves secretas.

---

## 1. Requisitos de Hardware e Sistema

- **Sistema Operacional:** Ubuntu Server ou Desktop `22.04 LTS` / `24.04 LTS` (64 bits)
- **CPU / RAM:** mínimo recomendado de **4 vCPUs** e **8 GB de RAM** (executa PostgreSQL x2, Redis, Judge0 `server` + `workers`, API Go, Monitor Go, SvelteKit e Caddy)
- **Disco:** mínimo de **30 GB** livres (para imagens Docker, bancos de dados e backups diários)
- **Portas de rede necessárias:** `80/tcp` e `443/tcp` (Caddy); `22/tcp` se usar SSH

> [!NOTE]
> O Ubuntu moderno utiliza **cgroups v2**. O arquivo [`deploy/judge0/compose.yaml`](../deploy/judge0/compose.yaml) já utiliza a imagem compatível com `cgroups v2` (`mrkushalsm/judge0:cgv2` com `/sys/fs/cgroup:/sys/fs/cgroup:rw`), portanto **não** é necessário rebaixar o kernel para `cgroups v1`.

---

## 2. Preparação do Servidor Ubuntu

### 2.1 Atualizar o sistema e instalar pacotes básicos

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y ca-certificates curl gnupg git openssl ufw
```

### 2.2 Instalar Docker Engine e Docker Compose v2

Remova eventuais pacotes antigos e instale a versão oficial do Docker:

```bash
sudo apt remove -y docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc || true

sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

Adicione seu usuário ao grupo `docker` para rodar comandos sem `sudo` (faça logout/login ou rode `newgrp docker` depois):

```bash
sudo usermod -aG docker "$USER"
newgrp docker
```

Valide a instalação:

```bash
docker --version
docker compose version
```

---

## 3. Configuração de Rede Local e Firewall

### 3.1 Descobrir o IP da máquina na rede local

```bash
ip -4 addr show | grep -E "inet " | grep -v "127.0.0.1"
```

Anote o IP da interface conectada à rede local (por exemplo, `192.168.100.161` ou `10.0.0.50`).

> [!TIP]
> Em servidores de laboratório/faculdade, configure **IP estático** (via reserva DHCP no roteador ou pelo `/etc/netplan/*.yaml`) para que o endereço do servidor não mude após reinicializações.

### 3.2 Verificar se as portas 80 e 443 estão livres

O Caddy precisa escutar nas portas `80` e `443`. Verifique se Apache, Nginx ou outro serviço já está usando essas portas:

```bash
sudo ss -tulpn | grep -E ":80\b|:443\b"
```

Se algum serviço (ex.: `apache2` ou `nginx`) aparecer, pare e desabilite-o:

```bash
sudo systemctl stop nginx apache2 2>/dev/null || true
sudo systemctl disable nginx apache2 2>/dev/null || true
```

### 3.3 Configurar o Firewall (UFW)

Se o firewall `ufw` estiver ativo (ou for ativado), libere SSH (para não perder acesso remoto) e HTTP/HTTPS:

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

---

## 4. Baixar o Repositório do Seno

```bash
git clone https://github.com/DharioNewbery/Seno.git
cd Seno
```

*(Opcional)* Para travar exatamente na versão de release `v0.2.0`:

```bash
git checkout v0.2.0
```

---

## 5. Configurar Variáveis de Ambiente (`deploy/.env`)

Copie o arquivo de exemplo:

```bash
cp deploy/.env.example deploy/.env
```

Gere 3 segredos aleatórios fortes para copiar no `.env`:

```bash
echo "SENO_MONITOR_SECRET=$(openssl rand -hex 32)"
echo "SENO_JUDGE0_CALLBACK_KEY=$(openssl rand -hex 24)"
echo "SENO_JUDGE0_CALLBACK_SECRET=$(openssl rand -hex 24)"
echo "POSTGRES_PASSWORD=$(openssl rand -hex 16)"
```

Edite `deploy/.env` (`nano deploy/.env`) substituindo `<IP_OU_DOMINIO>` pelo IP fixo da máquina na rede local (ex.: `192.168.100.161`) ou domínio interno (ex.: `seno.ufpa.br`):

```env
# --- Domínio e URLs públicas ---
SENO_DOMAIN=<IP_OU_DOMINIO>
SENO_WEB_ORIGIN=https://<IP_OU_DOMINIO>
SENO_MONITOR_PUBLIC_URL=https://<IP_OU_DOMINIO>/monitor
SENO_WEB_URL=https://<IP_OU_DOMINIO>

# --- Segurança e cookies (HTTPS via Caddy) ---
SENO_COOKIE_SECURE=true
SENO_MONITOR_SECRET=<COLE_O_SEGREDO_GERADO_1>
SENO_JUDGE0_CALLBACK_KEY=<COLE_O_SEGREDO_GERADO_2>
SENO_JUDGE0_CALLBACK_SECRET=<COLE_O_SEGREDO_GERADO_3>

# --- Banco de dados ---
POSTGRES_DB=seno
POSTGRES_USER=seno
POSTGRES_PASSWORD=<COLE_A_SENHA_GERADA>

# --- Super Admin inicial (altere a senha após o 1º login) ---
SENO_SUPER_EMAIL=super@seno.dev
SENO_SUPER_PASSWORD=troque-essa-senha-agora

# --- Serviço de E-mail (SMTP) ---
# Preencha com o SMTP da instituição/provedor ou deixe SENO_SMTP_HOST vazio
# para usar o fallback operacional (grava os links de convite/senha no Log).
SENO_SMTP_HOST=
SENO_SMTP_PORT=587
SENO_SMTP_USER=
SENO_SMTP_PASSWORD=
SENO_SMTP_FROM=seno@ufpa.br
```

> [!IMPORTANT]
> Não é necessário alterar `SENO_DATABASE_URL`, `SENO_MONITOR_DSN`, `SENO_API_URL`, `SENO_MONITOR_URL`, `SENO_JUDGE0_URL` ou `SENO_JUDGE0_CALLBACK_URL` para uso nos containers — o [`deploy/compose.yaml`](../deploy/compose.yaml) já define esses endereços internos automaticamente na rede Docker.

Para detalhes avançados do serviço de e-mail SMTP, consulte a seção 4 de [`docs/DEPLOY_GITHUB.md`](./DEPLOY_GITHUB.md).

---

## 6. Configurar Certificados TLS (`deploy/caddy/certs/`)

O [`Caddyfile`](../deploy/caddy/Caddyfile) espera dois arquivos em `deploy/caddy/certs/`:
- `fullchain.pem`
- `privkey.pem`

### Opção A: Certificado oficial da instituição
Copie os arquivos fornecidos pela equipe de TI para `deploy/caddy/certs/fullchain.pem` e `deploy/caddy/certs/privkey.pem`.

### Opção B: Certificado autoassinado para rede local (IP ou domínio interno)

Substitua `<IP_DA_LAN>` pelo IP da máquina (ex.: `192.168.100.161`):

```bash
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout deploy/caddy/certs/privkey.pem \
  -out deploy/caddy/certs/fullchain.pem \
  -subj "/CN=<IP_DA_LAN>" \
  -addext "subjectAltName=IP:<IP_DA_LAN>,IP:127.0.0.1,DNS:localhost"
```

Garanta permissão de leitura nos certificados:

```bash
chmod 644 deploy/caddy/certs/fullchain.pem
chmod 600 deploy/caddy/certs/privkey.pem
```

---

## 7. Subir os Serviços

### 7.1 Subir o PostgreSQL e criar o banco `seno_monitor`

O container `postgres` cria automaticamente o banco `seno` (`POSTGRES_DB=seno`), mas o Monitor utiliza um banco próprio (`seno_monitor`). Na **primeira instalação**, crie-o antes de subir `api` e `monitor`:

```bash
docker compose -f deploy/compose.yaml --env-file deploy/.env up -d postgres

# Aguarde o Postgres ficar healthy e crie o banco seno_monitor:
docker compose -f deploy/compose.yaml --env-file deploy/.env exec postgres \
  psql -U seno -d postgres -c "CREATE DATABASE seno_monitor;"
```

### 7.2 Construir e subir toda a stack

> [!NOTE]
> O frontend (`web`) resolve a origem dinamicamente por requisição em produção (`PROTOCOL_HEADER` / `HOST_HEADER`), funcionando com qualquer domínio, IP, porta ou camada de proxy reverso sem precisar recompilar a imagem.

```bash
docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build
```

*(Caso esteja usando exatamente o domínio configurado no `SENO_SITE_ORIGIN` do GHCR, é possível usar também `deploy/compose.ghcr.yaml` conforme [`docs/DEPLOY_GITHUB.md`](./DEPLOY_GITHUB.md).)*

---

## 8. Verificação e Primeiro Acesso

1. **Verifique o status dos containers:**
   ```bash
   docker compose -f deploy/compose.yaml --env-file deploy/.env ps
   ```
   Todos os serviços (`caddy`, `web`, `api`, `monitor`, `postgres`, `server`, `workers`, `db`, `redis`, `backup`) devem estar `Up` (e `postgres`/`monitor` como `healthy`).

2. **Acompanhe os logs iniciais:**
   ```bash
   docker compose -f deploy/compose.yaml --env-file deploy/.env logs -f api monitor web caddy
   ```

3. **Acesse de qualquer máquina na rede local:**
   - Abra `https://<IP_OU_DOMINIO>` no navegador.
   - Se usou certificado autoassinado, aceite o aviso de segurança do navegador (*Avançado → Continuar*).
   - Entre com o e-mail e senha definidos em `SENO_SUPER_EMAIL` / `SENO_SUPER_PASSWORD` e cadastre matérias, períodos letivos e professores.

---

## 9. Operação, Backup e Solução de Problemas

### Comandos úteis do dia a dia

```bash
# Ver logs de um serviço específico
docker compose -f deploy/compose.yaml --env-file deploy/.env logs -f api

# Reiniciar um serviço após editar deploy/.env
docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --force-recreate api

# Atualizar para uma nova versão do repositório
git pull
docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build

# Parar todos os serviços (mantendo os dados nos volumes)
docker compose -f deploy/compose.yaml --env-file deploy/.env down
```

### Backups automáticos e restauração

O serviço `backup` executa `pg_dump` diário dos bancos `seno` e `seno_monitor` no volume `backups` (com retenção de 120 dias).

Para listar os dumps gerados:

```bash
docker compose -f deploy/compose.yaml --env-file deploy/.env exec backup ls -lh /backups
```

Para restaurar um dump (exemplo para `seno`):

```bash
docker compose -f deploy/compose.yaml --env-file deploy/.env exec backup \
  pg_restore -h postgres -U seno -d seno --clean --if-exists /backups/seno-AAAAMMDD.dump
```

### Problemas comuns

| Sintoma | Causa provável | Solução |
| --- | --- | --- |
| Erro ao subir `caddy` (`bind: address already in use`) | Porta `80` ou `443` ocupada por `nginx`/`apache2` | Pare e desabilite o serviço conflitante no host (`systemctl stop nginx apache2`) |
| Erro `403 Cross-site POST form submissions are forbidden` ao fazer login | `web` foi baixado do GHCR com `SENO_SITE_ORIGIN` diferente do IP/domínio acessado | Rode `docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build web` |
| Container `monitor` reiniciando (`database "seno_monitor" does not exist`) | Banco `seno_monitor` não foi criado antes do boot | Crie com `CREATE DATABASE seno_monitor;` (seção 7.1) e rode `docker compose ... restart monitor` |
| Outro PC da LAN não carrega a página | Firewall UFW ou roteador bloqueando `80`/`443` | Verifique `sudo ufw status` e garanta que os dispositivos estão na mesma sub-rede |
| Judge0 não executa código / fila travada | `workers` ou `redis` ainda inicializando ou `judge0.conf` ausente | Confira `docker compose -f deploy/compose.yaml logs workers server redis` |
