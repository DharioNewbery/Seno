#!/usr/bin/env bash
# ==============================================================================
# deploy/deploy.sh — Deploy completo do Seno em produção (Linux)
# ==============================================================================
# - Gera automaticamente segredos fortes (API, Monitor, PostgreSQL, Judge0).
# - Verifica certificado SSL em deploy/caddy/certs/ e oferece gerar um
#   certificado autoassinado quando ausente.
# - Solicita via terminal apenas as configurações ainda não preenchidas no .env
#   (controlado pela flag SENO_DEPLOY_CONFIGURED=true no deploy/.env).
# - Suporta qualquer IP, domínio, porta e múltiplas camadas de proxy reverso.
# - Sobe toda a stack atrás do Caddy (único ponto de entrada público) e valida
#   todas as rotas essenciais ao final.
# ==============================================================================

set -euo pipefail

# --- Caminhos do repositório --------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_EXAMPLE="${SCRIPT_DIR}/.env.example"
JUDGE0_CONF="${SCRIPT_DIR}/judge0/judge0.conf"
JUDGE0_CONF_EXAMPLE="${SCRIPT_DIR}/judge0/judge0.conf.example"
CERTS_DIR="${SCRIPT_DIR}/caddy/certs"
CERT_FULL="${CERTS_DIR}/fullchain.pem"
CERT_KEY="${CERTS_DIR}/privkey.pem"

# --- Flags de CLI -------------------------------------------------------------
RECONFIGURE=false
ROTATE_SECRETS=false
USE_GHCR=false
NO_BUILD=false

usage() {
  cat <<'EOF'
Uso: ./deploy/deploy.sh [opções]

Opções:
  -r, --reconfigure      Reconfigura interativamente as opções do usuário
                         (mantém os segredos existentes, salvo com --rotate-secrets).
      --rotate-secrets   Regenera as chaves secretas da aplicação e do Judge0.
      --ghcr             Usa imagens pré-compiladas do GHCR (deploy/compose.ghcr.yaml)
                         em vez de compilar localmente.
      --no-build         Não executa --build ao subir os containers.
  -h, --help             Exibe esta ajuda.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -r|--reconfigure)
      RECONFIGURE=true
      shift
      ;;
    --rotate-secrets)
      ROTATE_SECRETS=true
      shift
      ;;
    --ghcr)
      USE_GHCR=true
      shift
      ;;
    --no-build)
      NO_BUILD=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Opção desconhecida: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

# --- Utilitários de saída -----------------------------------------------------
if [[ -t 1 ]]; then
  C_RESET=$'\033[0m'
  C_BOLD=$'\033[1m'
  C_CYAN=$'\033[36m'
  C_GREEN=$'\033[32m'
  C_YELLOW=$'\033[33m'
  C_RED=$'\033[31m'
  C_DIM=$'\033[2m'
else
  C_RESET="" C_BOLD="" C_CYAN="" C_GREEN="" C_YELLOW="" C_RED="" C_DIM=""
fi

info()    { printf "%s[INFO]%s %s\n" "${C_CYAN}" "${C_RESET}" "$*"; }
ok()      { printf "%s[OK]%s   %s\n" "${C_GREEN}" "${C_RESET}" "$*"; }
warn()    { printf "%s[AVISO]%s %s\n" "${C_YELLOW}" "${C_RESET}" "$*"; }
erro()    { printf "%s[ERRO]%s %s\n" "${C_RED}" "${C_RESET}" "$*" >&2; }
titulo()  { printf "\n%s==> %s%s\n" "${C_BOLD}${C_CYAN}" "$*" "${C_RESET}"; }

# --- Pré-requisitos (Linux apenas) --------------------------------------------
verificar_ambiente_linux() {
  if [[ "$(uname -s)" != "Linux" ]]; then
    erro "Este script é exclusivo para sistemas Linux (detectado: $(uname -s))."
    exit 1
  fi

  local missing=()
  for cmd in docker openssl curl; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    erro "Ferramentas obrigatórias ausentes: ${missing[*]}"
    exit 1
  fi

  if ! docker compose version >/dev/null 2>&1; then
    erro "Plugin 'docker compose' (v2) não está disponível."
    exit 1
  fi

  if ! docker info >/dev/null 2>&1; then
    erro "Não foi possível conectar ao daemon do Docker."
    erro "Verifique se o serviço 'docker' está ativo e se o usuário atual tem permissão (grupo docker ou sudo)."
    exit 1
  fi
}

# --- Leitura segura de arquivos chave=valor -----------------------------------
declare -A ENV_MAP=()
declare -A JUDGE0_MAP=()

ler_env_arquivo() {
  local arquivo="$1"
  local -n destino="$2"
  [[ -f "$arquivo" ]] || return 0

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    if [[ "$line" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
      local key="${BASH_REMATCH[1]}"
      local val="${BASH_REMATCH[2]}"
      # Remove espaços externos apenas se não houver aspas
      if [[ "$val" =~ ^\"(.*)\"[[:space:]]*$ ]]; then
        val="${BASH_REMATCH[1]}"
      elif [[ "$val" =~ ^\'(.*)\'[[:space:]]*$ ]]; then
        val="${BASH_REMATCH[1]}"
      else
        # Remove comentário inline precedido por espaço (ex.: "1048576   # desc")
        val="${val%%[[:space:]]#*}"
        val="${val%"${val##*[![:space:]]}"}"
        val="${val#"${val%%[![:space:]]*}"}"
      fi
      destino["$key"]="$val"
    fi
  done < "$arquivo"
}

e_placeholder() {
  local key="$1"
  local val="$2"
  [[ -z "$val" ]] && return 0
  case "$val" in
    troque-por-*|dev-callback-*|troque-isto-*|"<"*">") return 0 ;;
  esac
  case "$key" in
    POSTGRES_PASSWORD) [[ "$val" == "seno" ]] && return 0 ;;
    SENO_SUPER_PASSWORD) [[ "$val" == "102938" || "$val" == "troque-essa-senha-agora" ]] && return 0 ;;
    SENO_SUPER_EMAIL) [[ "$val" == "super@seno.dev" && "${ENV_MAP[SENO_DEPLOY_CONFIGURED]:-false}" != "true" ]] && return 0 ;;
    SENO_DOMAIN) [[ "$val" == "seno.ufpa.br" && "${ENV_MAP[SENO_DEPLOY_CONFIGURED]:-false}" != "true" ]] && return 0 ;;
    SENO_WEB_ORIGIN) [[ "$val" == http://localhost* && "${ENV_MAP[SENO_DEPLOY_CONFIGURED]:-false}" != "true" ]] && return 0 ;;
    SENO_SMTP_HOST) [[ "$val" == "localhost" ]] && return 0 ;;
  esac
  return 1
}

valor_configurado() {
  local key="$1"
  local val="${ENV_MAP[$key]:-}"
  [[ -n "$val" ]] && ! e_placeholder "$key" "$val"
}

gerar_hex() {
  local bytes="$1"
  openssl rand -hex "$bytes"
}

# --- Descoberta de rede / IP / portas -----------------------------------------
detectar_ip_local() {
  local ip=""
  if command -v ip >/dev/null 2>&1; then
    ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") {print $(i+1); exit}}' || true)"
    if [[ -z "$ip" ]]; then
      ip="$(ip -4 -o addr show scope global 2>/dev/null | awk '{split($4,a,"/"); print a[1]; exit}' || true)"
    fi
  fi
  if [[ -z "$ip" ]] && command -v hostname >/dev/null 2>&1; then
    ip="$(hostname -I 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i !~ /^127\./ && $i !~ /:/) {print $i; exit}}' || true)"
  fi
  printf '%s' "${ip:-127.0.0.1}"
}

listar_ips_locais() {
  if command -v ip >/dev/null 2>&1; then
    ip -4 -o addr show scope global 2>/dev/null | awk '{split($4,a,"/"); print a[1]}' | sort -u
  elif command -v hostname >/dev/null 2>&1; then
    hostname -I 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | grep -v '^127\.' | sort -u || true
  fi
}

e_ipv4() {
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

e_ipv6() {
  [[ "$1" == *:* ]]
}

formatar_host_url() {
  local host="$1"
  if e_ipv6 "$host" && [[ "$host" != \[*\] ]]; then
    printf '[%s]' "$host"
  else
    printf '%s' "$host"
  fi
}

porta_valida() {
  local p="$1"
  [[ "$p" =~ ^[0-9]+$ ]] && (( p >= 1 && p <= 65535 ))
}

porta_em_uso() {
  local porta="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -tlnH "sport = :${porta}" 2>/dev/null | grep -q .
    return $?
  fi
  return 1
}

# --- Prompts interativos ------------------------------------------------------
perguntar_sim_nao() {
  local prompt="$1"
  local padrao="$2" # S ou N
  local sufixo="[s/N]"
  [[ "$padrao" == "S" ]] && sufixo="[S/n]"

  while true; do
    local resp=""
    read -r -p "${prompt} ${sufixo}: " resp
    resp="${resp:-$padrao}"
    case "${resp,,}" in
      s|sim|y|yes) return 0 ;;
      n|nao|não|no) return 1 ;;
      *) warn "Responda com 's' (sim) ou 'n' (não)." ;;
    esac
  done
}

perguntar_texto() {
  local prompt="$1"
  local padrao="$2"
  local obrigatorio="${3:-true}"
  local resp=""

  while true; do
    if [[ -n "$padrao" ]]; then
      read -r -p "${prompt} [${padrao}]: " resp
      resp="${resp:-$padrao}"
    else
      read -r -p "${prompt}: " resp
    fi
    resp="${resp#"${resp%%[![:space:]]*}"}"
    resp="${resp%"${resp##*[![:space:]]}"}"
    if [[ "$obrigatorio" == "true" && -z "$resp" ]]; then
      warn "Este campo não pode ficar vazio."
      continue
    fi
    printf '%s' "$resp"
    return 0
  done
}

perguntar_porta() {
  local prompt="$1"
  local padrao="$2"
  local porta=""

  while true; do
    porta="$(perguntar_texto "$prompt" "$padrao" true)"
    if ! porta_valida "$porta"; then
      warn "Porta inválida: '$porta'. Informe um número entre 1 e 65535."
      continue
    fi
    if porta_em_uso "$porta"; then
      warn "A porta ${porta}/tcp parece já estar em uso nesta máquina (talvez pelo próprio Caddy do Seno ou por Nginx/Apache)."
      if ! perguntar_sim_nao "Deseja manter a porta ${porta} mesmo assim?" "S"; then
        continue
      fi
    fi
    printf '%s' "$porta"
    return 0
  done
}

perguntar_email() {
  local prompt="$1"
  local padrao="$2"
  local email=""
  while true; do
    email="$(perguntar_texto "$prompt" "$padrao" true)"
    if [[ "$email" =~ ^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$ ]]; then
      printf '%s' "${email,,}"
      return 0
    fi
    warn "Informe um endereço de e-mail válido (ex.: admin@ufpa.br)."
  done
}

ADMIN_PASSWORD_GENERATED=false

perguntar_senha_admin() {
  local s1="" s2=""
  while true; do
    read -r -s -p "Senha do Super Admin (mín. 8 chars; Enter gera automaticamente): " s1
    printf '\n' >&2
    if [[ -z "$s1" ]]; then
      s1="$(gerar_hex 12)"
      ADMIN_PASSWORD_GENERATED=true
      ok "Senha inicial do Super Admin gerada automaticamente." >&2
      printf '%s' "$s1"
      return 0
    fi
    if [[ ${#s1} -lt 8 ]]; then
      warn "A senha precisa ter pelo menos 8 caracteres."
      continue
    fi
    read -r -s -p "Confirme a senha do Super Admin: " s2
    printf '\n' >&2
    if [[ "$s1" != "$s2" ]]; then
      warn "As senhas não conferem. Tente novamente."
      continue
    fi
    printf '%s' "$s1"
    return 0
  done
}

# --- Geração automática de segredos -------------------------------------------
garantir_segredos() {
  titulo "Verificando e gerando chaves secretas"
  local gerados=()

  if [[ "$ROTATE_SECRETS" == "true" && -n "${ENV_MAP[POSTGRES_PASSWORD]:-}" && ! $(e_placeholder "POSTGRES_PASSWORD" "${ENV_MAP[POSTGRES_PASSWORD]}") ]]; then
    warn "Rotacionando POSTGRES_PASSWORD. Se o volume 'pgdata' já existir, o PostgreSQL manterá a senha antiga até ser recriado."
  fi

  local secret_keys=(
    "POSTGRES_PASSWORD:24"
    "SENO_MONITOR_SECRET:32"
    "SENO_JUDGE0_CALLBACK_KEY:24"
    "SENO_JUDGE0_CALLBACK_SECRET:24"
    "SENO_JUDGE0_TOKEN:24"
  )

  for spec in "${secret_keys[@]}"; do
    local k="${spec%%:*}"
    local bytes="${spec##*:}"
    if [[ "$ROTATE_SECRETS" == "true" ]] || ! valor_configurado "$k"; then
      ENV_MAP["$k"]="$(gerar_hex "$bytes")"
      gerados+=("$k")
    fi
  done

  if [[ ${#gerados[@]} -gt 0 ]]; then
    ok "Segredos gerados em deploy/.env: ${gerados[*]}"
  else
    ok "Todas as chaves secretas já estão preenchidas."
  fi
}

garantir_judge0_conf() {
  titulo "Configurando Judge0 (deploy/judge0/judge0.conf)"
  if [[ ! -f "$JUDGE0_CONF" ]]; then
    if [[ ! -f "$JUDGE0_CONF_EXAMPLE" ]]; then
      erro "Arquivo modelo não encontrado: ${JUDGE0_CONF_EXAMPLE}"
      exit 1
    fi
    cp "$JUDGE0_CONF_EXAMPLE" "$JUDGE0_CONF"
    info "Criado deploy/judge0/judge0.conf a partir do modelo."
  fi

  ler_env_arquivo "$JUDGE0_CONF" JUDGE0_MAP

  local j_redis="${JUDGE0_MAP[REDIS_PASSWORD]:-}"
  local j_pg="${JUDGE0_MAP[POSTGRES_PASSWORD]:-}"
  local j_secret="${JUDGE0_MAP[SECRET_KEY_BASE]:-}"

  if [[ "$ROTATE_SECRETS" == "true" || -z "$j_redis" || "$j_redis" == troque-isto-* ]]; then
    j_redis="$(gerar_hex 24)"
  fi
  if [[ "$ROTATE_SECRETS" == "true" || -z "$j_pg" || "$j_pg" == troque-isto-* ]]; then
    j_pg="$(gerar_hex 24)"
  fi
  if [[ "$ROTATE_SECRETS" == "true" || -z "$j_secret" || ${#j_secret} -lt 32 ]]; then
    j_secret="$(gerar_hex 64)"
  fi

  local j_token="${ENV_MAP[SENO_JUDGE0_TOKEN]}"

  cat > "$JUDGE0_CONF" <<EOF
# Gerado/atualizado automaticamente por deploy/deploy.sh. NUNCA comitar este arquivo.

# Acesso / rede (produção: rede interna Docker)
ALLOW_ORIGIN=${JUDGE0_MAP[ALLOW_ORIGIN]:-}

# Autenticação interna da API do Judge0
AUTHN_HEADER=X-Auth-Token
AUTHN_TOKEN=${j_token}

# Redis / PostgreSQL exclusivos do Judge0
REDIS_HOST=redis
REDIS_PASSWORD=${j_redis}
POSTGRES_HOST=db
POSTGRES_DB=${JUDGE0_MAP[POSTGRES_DB]:-judge0}
POSTGRES_USER=${JUDGE0_MAP[POSTGRES_USER]:-judge0}
POSTGRES_PASSWORD=${j_pg}

# Limites do sandbox
CPU_TIME_LIMIT=${JUDGE0_MAP[CPU_TIME_LIMIT]:-}
WALL_TIME_LIMIT=${JUDGE0_MAP[WALL_TIME_LIMIT]:-}
MEMORY_LIMIT=${JUDGE0_MAP[MEMORY_LIMIT]:-}
MAX_CPU_TIME_LIMIT=${JUDGE0_MAP[MAX_CPU_TIME_LIMIT]:-15}
MAX_WALL_TIME_LIMIT=${JUDGE0_MAP[MAX_WALL_TIME_LIMIT]:-20}
MAX_MEMORY_LIMIT=${JUDGE0_MAP[MAX_MEMORY_LIMIT]:-2097152}

# Rails
RAILS_ENV=production
SECRET_KEY_BASE=${j_secret}
EOF
  chmod 600 "$JUDGE0_CONF"
  ok "deploy/judge0/judge0.conf pronto (AUTHN_TOKEN sincronizado e segredos protegidos)."
}

# --- Certificados SSL ---------------------------------------------------------
gerar_certificado_autoassinado() {
  local host="$1"
  mkdir -p "$CERTS_DIR"

  local san_items=()
  local -A seen=()

  add_san() {
    local entry="$1"
    if [[ -z "${seen[$entry]:-}" ]]; then
      seen["$entry"]=1
      san_items+=("$entry")
    fi
  }

  if e_ipv4 "$host" || e_ipv6 "$host"; then
    add_san "IP:${host//[\[\]]/}"
  else
    add_san "DNS:${host}"
  fi

  while IFS= read -r lip; do
    [[ -n "$lip" ]] && add_san "IP:${lip}"
  done < <(listar_ips_locais)

  add_san "IP:127.0.0.1"
  add_san "DNS:localhost"

  local san_csv
  san_csv="$(IFS=,; echo "${san_items[*]}")"

  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -keyout "$CERT_KEY" \
    -out "$CERT_FULL" \
    -subj "/CN=${host}" \
    -addext "subjectAltName=${san_csv}" >/dev/null 2>&1

  chmod 644 "$CERT_FULL"
  chmod 600 "$CERT_KEY"
  ok "Certificado SSL autoassinado gerado para: ${san_csv}"
}

verificar_ou_gerar_ssl() {
  local host="$1"
  local force_check="${2:-false}"

  # Se já está em modo HTTP puro e não pediu reconfiguração, não exige cert.
  if [[ "${ENV_MAP[SENO_TLS_ENABLED]:-true}" == "false" && "$force_check" != "true" ]]; then
    info "Modo HTTP puro ativo (SENO_TLS_ENABLED=false)."
    return 0
  fi

  titulo "Verificação de Certificado SSL (HTTPS)"

  if [[ -f "$CERT_FULL" && -f "$CERT_KEY" ]]; then
    if openssl x509 -in "$CERT_FULL" -noout >/dev/null 2>&1; then
      local subj exp
      subj="$(openssl x509 -in "$CERT_FULL" -noout -subject 2>/dev/null | sed 's/^subject=//')"
      exp="$(openssl x509 -in "$CERT_FULL" -noout -enddate 2>/dev/null | sed 's/^notAfter=//')"
      ok "Certificado SSL encontrado em deploy/caddy/certs/ (${subj}; válido até ${exp})."
      if [[ "$force_check" == "true" && "${ENV_MAP[SENO_TLS_ENABLED]:-true}" == "false" ]]; then
        if perguntar_sim_nao "Deseja ativar HTTPS no Caddy usando este certificado?" "S"; then
          ENV_MAP["SENO_TLS_ENABLED"]="true"
        fi
      else
        ENV_MAP["SENO_TLS_ENABLED"]="true"
      fi
      return 0
    else
      warn "Arquivo $CERT_FULL existe, mas não é um certificado X.509 válido."
    fi
  fi

  warn "Nenhum certificado SSL válido encontrado em ${CERTS_DIR}/ (esperado: fullchain.pem e privkey.pem)."
  echo "  Sem um certificado válido, o Caddy não consegue servir HTTPS diretamente."
  echo "  Você pode gerar agora um certificado autoassinado (ideal para LAN ou IP direto;"
  echo "  o navegador exibirá apenas o aviso inicial de certificado não confiável)."

  if perguntar_sim_nao "Deseja gerar um certificado SSL autoassinado (válido por 365 dias) agora?" "S"; then
    gerar_certificado_autoassinado "$host"
    ENV_MAP["SENO_TLS_ENABLED"]="true"
    return 0
  fi

  echo ""
  info "Caso já possua arquivos .pem em outro caminho neste servidor, informe-os abaixo para copiá-los"
  info "(ou pressione Enter em branco para continuar sem certificado local)."
  local src_full="" src_key=""
  read -r -p "Caminho para fullchain.pem (opcional): " src_full
  if [[ -n "$src_full" ]]; then
    read -r -p "Caminho para privkey.pem: " src_key
    if [[ -f "$src_full" && -f "$src_key" ]]; then
      mkdir -p "$CERTS_DIR"
      cp "$src_full" "$CERT_FULL"
      cp "$src_key" "$CERT_KEY"
      chmod 644 "$CERT_FULL"
      chmod 600 "$CERT_KEY"
      if openssl x509 -in "$CERT_FULL" -noout >/dev/null 2>&1; then
        ok "Certificado copiado para ${CERTS_DIR}/."
        ENV_MAP["SENO_TLS_ENABLED"]="true"
        return 0
      fi
      erro "O arquivo copiado não é um certificado X.509 válido."
      exit 1
    else
      erro "Arquivo(s) não encontrado(s): '${src_full}' / '${src_key}'."
      exit 1
    fi
  fi

  warn "Sem certificado SSL configurado, o HTTPS direto no Caddy ficará desabilitado."
  if perguntar_sim_nao "Deseja subir em modo HTTP puro (recomendado apenas atrás de um proxy externo que já faça HTTPS)?" "N"; then
    ENV_MAP["SENO_TLS_ENABLED"]="false"
    return 0
  fi

  erro "Deploy cancelado. Adicione fullchain.pem e privkey.pem em ${CERTS_DIR}/ ou execute novamente aceitando o certificado autoassinado."
  exit 1
}

# --- Coleta interativa de configuração ----------------------------------------
coletar_configuracao() {
  local precisa_config=false
  local flag_config="${ENV_MAP[SENO_DEPLOY_CONFIGURED]:-false}"

  if [[ "$RECONFIGURE" == "true" ]]; then
    precisa_config=true
  elif [[ "$flag_config" != "true" ]]; then
    precisa_config=true
  else
    # Mesmo com SENO_DEPLOY_CONFIGURED=true, confere se algum campo crítico está ausente.
    local obrigatorias=(SENO_DOMAIN SENO_WEB_ORIGIN SENO_HTTP_PORT SENO_HTTPS_PORT SENO_SUPER_EMAIL SENO_SUPER_PASSWORD)
    for k in "${obrigatorias[@]}"; do
      if [[ -z "${ENV_MAP[$k]:-}" ]]; then
        precisa_config=true
        break
      fi
    done
  fi

  if [[ "$precisa_config" == "false" ]]; then
    ok "Configuração já preenchida (SENO_DEPLOY_CONFIGURED=true). Pulando perguntas interativas."
    info "Use './deploy/deploy.sh --reconfigure' se desejar alterar host, portas, proxy, admin ou SMTP."
    # Ainda verifica o SSL quando TLS está habilitado (caso os certs tenham sido apagados).
    verificar_ou_gerar_ssl "${ENV_MAP[SENO_DOMAIN]}" "false"
    return 0
  fi

  titulo "Configuração inicial do ambiente de produção"
  echo "Preencha apenas os dados necessários abaixo (pressione Enter para aceitar o padrão entre [])."

  local ip_padrao
  ip_padrao="$(detectar_ip_local)"

  # 1. Host / Domínio / IP
  if [[ "$RECONFIGURE" == "true" ]] || ! valor_configurado "SENO_DOMAIN"; then
    local def_domain="${ENV_MAP[SENO_DOMAIN]:-$ip_padrao}"
    if e_placeholder "SENO_DOMAIN" "$def_domain"; then
      def_domain="$ip_padrao"
    fi
    echo ""
    echo "${C_BOLD}1. Endereço do servidor (Domínio ou IP)${C_RESET}"
    echo "   Informe o domínio (ex.: seno.ufpa.br) ou o IP desta máquina (detectado: ${ip_padrao})."
    echo "   Observação: se ainda não houver DNS configurado, use o IP; tudo funcionará normalmente."
    ENV_MAP["SENO_DOMAIN"]="$(perguntar_texto "Domínio ou IP de acesso" "$def_domain" true)"
  else
    ok "SENO_DOMAIN já configurado: ${ENV_MAP[SENO_DOMAIN]}"
  fi

  # 2. Certificado SSL
  verificar_ou_gerar_ssl "${ENV_MAP[SENO_DOMAIN]}" "$RECONFIGURE"

  # 3. Portas HTTP / HTTPS do Caddy
  local tls_enabled="${ENV_MAP[SENO_TLS_ENABLED]:-true}"
  if [[ "$RECONFIGURE" == "true" ]] || ! valor_configurado "SENO_HTTP_PORT" || { [[ "$tls_enabled" == "true" ]] && ! valor_configurado "SENO_HTTPS_PORT"; }; then
    echo ""
    echo "${C_BOLD}2. Portas de exposição do Caddy (único ponto de entrada)${C_RESET}"
    echo "   O padrão web é 80 (HTTP) e 443 (HTTPS). Use outras portas se já houver outro proxy no host."
    local def_http="${ENV_MAP[SENO_HTTP_PORT]:-80}"
    local def_https="${ENV_MAP[SENO_HTTPS_PORT]:-443}"
    if [[ "$tls_enabled" == "true" ]]; then
      ENV_MAP["SENO_HTTPS_PORT"]="$(perguntar_porta "Porta HTTPS pública" "$def_https")"
      ENV_MAP["SENO_HTTP_PORT"]="$(perguntar_porta "Porta HTTP pública (redireciona para HTTPS)" "$def_http")"
    else
      ENV_MAP["SENO_HTTP_PORT"]="$(perguntar_porta "Porta HTTP pública" "$def_http")"
      ENV_MAP["SENO_HTTPS_PORT"]="$def_https"
    fi
  else
    ok "Portas já configuradas: HTTP=${ENV_MAP[SENO_HTTP_PORT]}, HTTPS=${ENV_MAP[SENO_HTTPS_PORT]}"
  fi

  local http_port="${ENV_MAP[SENO_HTTP_PORT]:-80}"
  local https_port="${ENV_MAP[SENO_HTTPS_PORT]:-443}"
  if [[ "$https_port" == "443" ]]; then
    ENV_MAP["SENO_HTTPS_PORT_SUFFIX"]=""
  else
    ENV_MAP["SENO_HTTPS_PORT_SUFFIX"]=":${https_port}"
  fi

  # 4. URL pública canônica e camadas de proxy reverso externas
  local host_fmt default_origin
  host_fmt="$(formatar_host_url "${ENV_MAP[SENO_DOMAIN]}")"
  if [[ "$tls_enabled" == "true" ]]; then
    default_origin="https://${host_fmt}${ENV_MAP[SENO_HTTPS_PORT_SUFFIX]}"
  else
    if [[ "$http_port" == "80" ]]; then
      default_origin="http://${host_fmt}"
    else
      default_origin="http://${host_fmt}:${http_port}"
    fi
  fi

  if [[ "$RECONFIGURE" == "true" ]] || ! valor_configurado "SENO_WEB_ORIGIN"; then
    echo ""
    echo "${C_BOLD}3. Camadas de proxy e URL pública (e-mails e CSRF)${C_RESET}"
    echo "   URL calculada para links de e-mail e verificação de origem: ${default_origin}"
    if perguntar_sim_nao "O Seno ficará atrás de outro proxy reverso externo (ex.: Nginx/SETIC) com URL pública diferente?" "N"; then
      local cur_origin="${ENV_MAP[SENO_WEB_ORIGIN]:-$default_origin}"
      e_placeholder "SENO_WEB_ORIGIN" "$cur_origin" && cur_origin="$default_origin"
      local web_origin
      while true; do
        web_origin="$(perguntar_texto "URL pública completa vista pelo navegador (ex.: https://seno.ufpa.br)" "$cur_origin" true)"
        web_origin="${web_origin%/}"
        if [[ "$web_origin" =~ ^https?://[^/]+$ ]]; then
          break
        fi
        warn "Informe apenas esquema + host (+ porta opcional), sem caminho no final (ex.: https://seno.ufpa.br)."
      done
      ENV_MAP["SENO_WEB_ORIGIN"]="$web_origin"

      local cur_trusted="${ENV_MAP[SENO_TRUSTED_PROXIES]:-private_ranges}"
      echo "   O Caddy já confia automaticamente em redes privadas (private_ranges: 10/8, 172.16/12, 192.168/16, 127/8)."
      ENV_MAP["SENO_TRUSTED_PROXIES"]="$(perguntar_texto "CIDRs/proxies confiáveis (separados por espaço)" "$cur_trusted" true)"

      if [[ "$web_origin" == https://* ]]; then
        ENV_MAP["SENO_COOKIE_SECURE"]="true"
      else
        if perguntar_sim_nao "O acesso final dos usuários no proxy externo será via HTTPS?" "S"; then
          ENV_MAP["SENO_COOKIE_SECURE"]="true"
        else
          ENV_MAP["SENO_COOKIE_SECURE"]="false"
        fi
      fi
    else
      ENV_MAP["SENO_WEB_ORIGIN"]="$default_origin"
      ENV_MAP["SENO_TRUSTED_PROXIES"]="${ENV_MAP[SENO_TRUSTED_PROXIES]:-private_ranges}"
      if [[ "$tls_enabled" == "true" ]]; then
        ENV_MAP["SENO_COOKIE_SECURE"]="true"
      else
        ENV_MAP["SENO_COOKIE_SECURE"]="false"
      fi
    fi
  else
    ok "SENO_WEB_ORIGIN já configurado: ${ENV_MAP[SENO_WEB_ORIGIN]}"
  fi

  # 5. Super Admin inicial
  if [[ "$RECONFIGURE" == "true" ]] || ! valor_configurado "SENO_SUPER_EMAIL" || ! valor_configurado "SENO_SUPER_PASSWORD"; then
    echo ""
    echo "${C_BOLD}4. Conta inicial do Super Administrador${C_RESET}"
    echo "   Usada no primeiro login para cadastrar matérias, períodos, professores e alunos."
    if [[ "$RECONFIGURE" == "true" ]] || ! valor_configurado "SENO_SUPER_EMAIL"; then
      local def_email="${ENV_MAP[SENO_SUPER_EMAIL]:-}"
      e_placeholder "SENO_SUPER_EMAIL" "$def_email" && def_email=""
      ENV_MAP["SENO_SUPER_EMAIL"]="$(perguntar_email "E-mail do Super Admin" "$def_email")"
    else
      ok "SENO_SUPER_EMAIL já configurado: ${ENV_MAP[SENO_SUPER_EMAIL]}"
    fi

    if [[ "$RECONFIGURE" == "true" ]] || ! valor_configurado "SENO_SUPER_PASSWORD"; then
      if [[ "$RECONFIGURE" == "true" ]] && valor_configurado "SENO_SUPER_PASSWORD"; then
        if perguntar_sim_nao "Deseja alterar SENO_SUPER_PASSWORD?" "N"; then
          ENV_MAP["SENO_SUPER_PASSWORD"]="$(perguntar_senha_admin)"
        fi
      else
        ENV_MAP["SENO_SUPER_PASSWORD"]="$(perguntar_senha_admin)"
      fi
    fi
  else
    ok "Super Admin já configurado (${ENV_MAP[SENO_SUPER_EMAIL]})."
  fi

  # 6. SMTP (opcional)
  local smtp_definido=false
  if [[ -n "${ENV_MAP[SENO_SMTP_HOST]:-}" && "${ENV_MAP[SENO_SMTP_HOST]}" != "localhost" ]]; then
    smtp_definido=true
  fi
  if [[ "$RECONFIGURE" == "true" ]] || [[ "$flag_config" != "true" && "$smtp_definido" == "false" ]]; then
    echo ""
    echo "${C_BOLD}5. Serviço de E-mail (SMTP — opcional)${C_RESET}"
    echo "   Usado para enviar convites e recuperação de senha. Se não configurar agora,"
    echo "   o Seno grava os links no Log de auditoria (fallback operacional seguro)."
    local padrao_smtp="N"
    [[ "$smtp_definido" == "true" ]] && padrao_smtp="S"
    if perguntar_sim_nao "Deseja configurar um servidor SMTP agora?" "$padrao_smtp"; then
      local def_host="${ENV_MAP[SENO_SMTP_HOST]:-}"
      [[ "$def_host" == "localhost" ]] && def_host=""
      ENV_MAP["SENO_SMTP_HOST"]="$(perguntar_texto "Host SMTP (ex.: smtp.ufpa.br)" "$def_host" true)"
      local def_sport="${ENV_MAP[SENO_SMTP_PORT]:-587}"
      [[ "$def_sport" == "1025" ]] && def_sport="587"
      ENV_MAP["SENO_SMTP_PORT"]="$(perguntar_porta "Porta SMTP" "$def_sport")"
      ENV_MAP["SENO_SMTP_USER"]="$(perguntar_texto "Usuário SMTP (deixe vazio para relay sem autenticação)" "${ENV_MAP[SENO_SMTP_USER]:-}" false)"
      if [[ -n "${ENV_MAP[SENO_SMTP_USER]}" ]]; then
        local spw=""
        read -r -s -p "Senha SMTP (Enter mantém a atual se já existir): " spw
        printf '\n'
        if [[ -n "$spw" ]]; then
          ENV_MAP["SENO_SMTP_PASSWORD"]="$spw"
        fi
      else
        ENV_MAP["SENO_SMTP_PASSWORD"]=""
      fi
      ENV_MAP["SENO_SMTP_FROM"]="$(perguntar_email "E-mail remetente (From)" "${ENV_MAP[SENO_SMTP_FROM]:-seno@ufpa.br}")"
    else
      ENV_MAP["SENO_SMTP_HOST"]=""
      ENV_MAP["SENO_SMTP_PORT"]="587"
      ENV_MAP["SENO_SMTP_USER"]=""
      ENV_MAP["SENO_SMTP_PASSWORD"]=""
      ENV_MAP["SENO_SMTP_FROM"]="${ENV_MAP[SENO_SMTP_FROM]:-seno@ufpa.br}"
    fi
  else
    ok "Configuração SMTP preservada."
  fi
}

# --- Escrita de deploy/.env ---------------------------------------------------
gravar_env() {
  titulo "Salvando deploy/.env"

  local tls_enabled="${ENV_MAP[SENO_TLS_ENABLED]:-true}"
  local caddyfile_rel="./caddy/Caddyfile"
  [[ "$tls_enabled" == "false" ]] && caddyfile_rel="./caddy/Caddyfile.http"

  # Normaliza valores críticos de produção
  ENV_MAP["SENO_ADDR"]="${ENV_MAP[SENO_ADDR]:-:8080}"
  ENV_MAP["SENO_MONITOR_ADDR"]="${ENV_MAP[SENO_MONITOR_ADDR]:-:8081}"
  ENV_MAP["SENO_SESSION_TTL"]="${ENV_MAP[SENO_SESSION_TTL]:-12h}"
  ENV_MAP["SENO_SESSION_MAX_TTL"]="${ENV_MAP[SENO_SESSION_MAX_TTL]:-168h}"
  ENV_MAP["SENO_POLL_ESTADO_S"]="${ENV_MAP[SENO_POLL_ESTADO_S]:-30}"
  ENV_MAP["SENO_MONITOR_HMAC_JANELA_S"]="${ENV_MAP[SENO_MONITOR_HMAC_JANELA_S]:-60}"
  ENV_MAP["SENO_MONITOR_MAX_PAYLOAD_BYTES"]="${ENV_MAP[SENO_MONITOR_MAX_PAYLOAD_BYTES]:-1048576}"
  ENV_MAP["SENO_SNAPSHOT_INTERVALO_S"]="${ENV_MAP[SENO_SNAPSHOT_INTERVALO_S]:-60}"
  ENV_MAP["SENO_MONITOR_MAX_SNAPSHOT_BYTES"]="${ENV_MAP[SENO_MONITOR_MAX_SNAPSHOT_BYTES]:-262144}"
  ENV_MAP["SENO_TOKEN_SSO_TTL_S"]="${ENV_MAP[SENO_TOKEN_SSO_TTL_S]:-300}"
  ENV_MAP["SENO_PRE_AVA_TTL_S"]="${ENV_MAP[SENO_PRE_AVA_TTL_S]:-900}"
  ENV_MAP["SENO_ALUNO_EXEC_MAX_PER_MIN"]="${ENV_MAP[SENO_ALUNO_EXEC_MAX_PER_MIN]:-5}"
  ENV_MAP["SENO_EXECUCAO_RETENCAO_D"]="${ENV_MAP[SENO_EXECUCAO_RETENCAO_D]:-7}"
  ENV_MAP["SENO_RETENCAO_DADOS_D"]="${ENV_MAP[SENO_RETENCAO_DADOS_D]:-730}"
  ENV_MAP["SENO_WEB_BODY_LIMIT"]="${ENV_MAP[SENO_WEB_BODY_LIMIT]:-16M}"
  ENV_MAP["POSTGRES_DB"]="${ENV_MAP[POSTGRES_DB]:-seno}"
  ENV_MAP["POSTGRES_USER"]="${ENV_MAP[POSTGRES_USER]:-seno}"
  ENV_MAP["SENO_MONITOR_PUBLIC_URL"]="/monitor"
  ENV_MAP["SENO_WEB_URL"]=""
  ENV_MAP["SENO_CADDYFILE"]="$caddyfile_rel"
  ENV_MAP["SENO_DEPLOY_CONFIGURED"]="true"
  ENV_MAP["PORT"]="3000"

  # Mantém também DSNs coerentes caso algum utilitário local os leia fora do compose
  local pg_user="${ENV_MAP[POSTGRES_USER]}"
  local pg_pass="${ENV_MAP[POSTGRES_PASSWORD]}"
  local pg_db="${ENV_MAP[POSTGRES_DB]}"

  cat > "$ENV_FILE" <<EOF
# ============================================================================
# Seno — Configuração de Produção (gerado por deploy/deploy.sh)
# NUNCA comite este arquivo no Git.
# ============================================================================

# --- Flag de configuração concluída ---
SENO_DEPLOY_CONFIGURED=${ENV_MAP[SENO_DEPLOY_CONFIGURED]}

# --- Rede pública, Proxy / Caddy e TLS ---
SENO_DOMAIN=${ENV_MAP[SENO_DOMAIN]}
SENO_HTTP_PORT=${ENV_MAP[SENO_HTTP_PORT]:-80}
SENO_HTTPS_PORT=${ENV_MAP[SENO_HTTPS_PORT]:-443}
SENO_HTTPS_PORT_SUFFIX=${ENV_MAP[SENO_HTTPS_PORT_SUFFIX]:-}
SENO_TRUSTED_PROXIES=${ENV_MAP[SENO_TRUSTED_PROXIES]:-private_ranges}
SENO_TLS_ENABLED=${tls_enabled}
SENO_CADDYFILE=${ENV_MAP[SENO_CADDYFILE]}
SENO_WEB_ORIGIN=${ENV_MAP[SENO_WEB_ORIGIN]}
SENO_COOKIE_SECURE=${ENV_MAP[SENO_COOKIE_SECURE]:-true}

# --- Rotas entre serviços (dinâmicas em produção atrás do Caddy) ---
SENO_MONITOR_PUBLIC_URL=${ENV_MAP[SENO_MONITOR_PUBLIC_URL]}
SENO_WEB_URL=${ENV_MAP[SENO_WEB_URL]}
SENO_API_URL=http://api:8080
SENO_MONITOR_URL=http://monitor:8081
SENO_JUDGE0_URL=http://server:2358
SENO_JUDGE0_CALLBACK_URL=http://api:8080
SENO_MONITOR_CALLBACK_URL=http://monitor:8081

# --- Segredos gerados ---
POSTGRES_DB=${pg_db}
POSTGRES_USER=${pg_user}
POSTGRES_PASSWORD=${pg_pass}
SENO_DATABASE_URL=postgres://${pg_user}:${pg_pass}@postgres:5432/${pg_db}?sslmode=disable
SENO_MONITOR_DSN=postgres://${pg_user}:${pg_pass}@postgres:5432/seno_monitor?sslmode=disable
SENO_MONITOR_SECRET=${ENV_MAP[SENO_MONITOR_SECRET]}
SENO_JUDGE0_CALLBACK_KEY=${ENV_MAP[SENO_JUDGE0_CALLBACK_KEY]}
SENO_JUDGE0_CALLBACK_SECRET=${ENV_MAP[SENO_JUDGE0_CALLBACK_SECRET]}
SENO_JUDGE0_TOKEN=${ENV_MAP[SENO_JUDGE0_TOKEN]}

# --- Super Admin inicial ---
SENO_SUPER_EMAIL=${ENV_MAP[SENO_SUPER_EMAIL]}
SENO_SUPER_PASSWORD=${ENV_MAP[SENO_SUPER_PASSWORD]}

# --- Serviço de E-mail (SMTP) ---
SENO_SMTP_HOST=${ENV_MAP[SENO_SMTP_HOST]:-}
SENO_SMTP_PORT=${ENV_MAP[SENO_SMTP_PORT]:-587}
SENO_SMTP_USER=${ENV_MAP[SENO_SMTP_USER]:-}
SENO_SMTP_PASSWORD=${ENV_MAP[SENO_SMTP_PASSWORD]:-}
SENO_SMTP_FROM=${ENV_MAP[SENO_SMTP_FROM]:-seno@ufpa.br}

# --- Ajustes operacionais ---
SENO_ADDR=${ENV_MAP[SENO_ADDR]}
SENO_MONITOR_ADDR=${ENV_MAP[SENO_MONITOR_ADDR]}
SENO_SESSION_TTL=${ENV_MAP[SENO_SESSION_TTL]}
SENO_SESSION_MAX_TTL=${ENV_MAP[SENO_SESSION_MAX_TTL]}
SENO_POLL_ESTADO_S=${ENV_MAP[SENO_POLL_ESTADO_S]}
SENO_MONITOR_HMAC_JANELA_S=${ENV_MAP[SENO_MONITOR_HMAC_JANELA_S]}
SENO_MONITOR_MAX_PAYLOAD_BYTES=${ENV_MAP[SENO_MONITOR_MAX_PAYLOAD_BYTES]}
SENO_SNAPSHOT_INTERVALO_S=${ENV_MAP[SENO_SNAPSHOT_INTERVALO_S]}
SENO_MONITOR_MAX_SNAPSHOT_BYTES=${ENV_MAP[SENO_MONITOR_MAX_SNAPSHOT_BYTES]}
SENO_TOKEN_SSO_TTL_S=${ENV_MAP[SENO_TOKEN_SSO_TTL_S]}
SENO_PRE_AVA_TTL_S=${ENV_MAP[SENO_PRE_AVA_TTL_S]}
SENO_ALUNO_EXEC_MAX_PER_MIN=${ENV_MAP[SENO_ALUNO_EXEC_MAX_PER_MIN]}
SENO_EXECUCAO_RETENCAO_D=${ENV_MAP[SENO_EXECUCAO_RETENCAO_D]}
SENO_RETENCAO_DADOS_D=${ENV_MAP[SENO_RETENCAO_DADOS_D]}
SENO_WEB_BODY_LIMIT=${ENV_MAP[SENO_WEB_BODY_LIMIT]}
PORT=${ENV_MAP[PORT]}
EOF

  # Preserva variáveis extras opcionais (ex.: GHCR_OWNER, SENO_VERSION) se já existiam
  for extra in GHCR_OWNER SENO_VERSION SENO_ARQUIVO_MAX_BYTES; do
    if [[ -n "${ENV_MAP[$extra]:-}" ]]; then
      printf '%s=%s\n' "$extra" "${ENV_MAP[$extra]}" >> "$ENV_FILE"
    fi
  done

  chmod 600 "$ENV_FILE"
  ok "Arquivo deploy/.env salvo com permissão 600."
}

# --- Subida da stack e smoke checks -------------------------------------------
subir_stack() {
  titulo "Subindo containers do Seno (Docker Compose)"

  local compose_args=(-f "${SCRIPT_DIR}/compose.yaml")
  if [[ "${ENV_MAP[SENO_TLS_ENABLED]:-true}" == "false" ]]; then
    compose_args+=(-f "${SCRIPT_DIR}/compose.http.yaml")
  fi
  if [[ "$USE_GHCR" == "true" ]]; then
    compose_args+=(-f "${SCRIPT_DIR}/compose.ghcr.yaml")
  fi
  compose_args+=(--env-file "${ENV_FILE}")

  if [[ "$USE_GHCR" == "true" ]]; then
    info "Baixando imagens do GHCR..."
    docker compose "${compose_args[@]}" pull
    docker compose "${compose_args[@]}" up -d --remove-orphans
  elif [[ "$NO_BUILD" == "true" ]]; then
    docker compose "${compose_args[@]}" up -d --remove-orphans
  else
    info "Construindo e subindo serviços (primeira execução pode levar alguns minutos)..."
    docker compose "${compose_args[@]}" up -d --build --remove-orphans
  fi

  titulo "Aguardando saúde dos serviços"
  local cid status deadline=$((SECONDS + 120))
  local svc
  for svc in postgres monitor web; do
    while (( SECONDS < deadline )); do
      cid="$(docker compose "${compose_args[@]}" ps -q "$svc" 2>/dev/null || true)"
      if [[ -n "$cid" ]]; then
        status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$cid" 2>/dev/null || echo "unknown")"
        if [[ "$status" == "healthy" || "$status" == "running" ]]; then
          ok "Serviço ${svc}: ${status}"
          break
        fi
      fi
      sleep 2
    done
    if (( SECONDS >= deadline )); then
      warn "Serviço ${svc} demorou para responder como healthy (status atual: ${status:-desconhecido})."
    fi
  done

  titulo "Validando rotas através do único endereço público (Caddy)"
  local tls_enabled="${ENV_MAP[SENO_TLS_ENABLED]:-true}"
  local host="${ENV_MAP[SENO_DOMAIN]}"
  local host_fmt base_local host_hdr
  host_fmt="$(formatar_host_url "$host")"

  if [[ "$tls_enabled" == "true" ]]; then
    local https_port="${ENV_MAP[SENO_HTTPS_PORT]:-443}"
    base_local="https://127.0.0.1:${https_port}"
    host_hdr="${host_fmt}${ENV_MAP[SENO_HTTPS_PORT_SUFFIX]:-}"
  else
    local http_port="${ENV_MAP[SENO_HTTP_PORT]:-80}"
    base_local="http://127.0.0.1:${http_port}"
    if [[ "$http_port" == "80" ]]; then
      host_hdr="${host_fmt}"
    else
      host_hdr="${host_fmt}:${http_port}"
    fi
  fi

  local curl_common=(-ksS -o /dev/null -w "%{http_code}" -H "Host: ${host_hdr}" --max-time 5)
  local tent=0
  local code=""
  while (( tent < 20 )); do
    code="$(curl "${curl_common[@]}" "${base_local}/healthz" 2>/dev/null || echo "000")"
    [[ "$code" == "200" ]] && break
    tent=$((tent + 1))
    sleep 2
  done

  verificar_rota() {
    local rota="$1"
    local esperado="$2"
    local desc="$3"
    local got
    got="$(curl "${curl_common[@]}" "${base_local}${rota}" 2>/dev/null || echo "000")"
    if [[ "$got" == "$esperado" ]]; then
      ok "Rota ${rota} (${desc}) -> HTTP ${got}"
    else
      warn "Rota ${rota} (${desc}) retornou HTTP ${got} (esperado ${esperado})"
    fi
  }

  verificar_rota "/" "200" "Portal Web / Login"
  verificar_rota "/healthz" "200" "Healthcheck Web"
  verificar_rota "/monitor/healthz" "200" "Monitor via /monitor"
  verificar_rota "/a/app" "200" "Tela de realização do Monitor"
  verificar_rota "/monitor/interna/v1/atividades" "404" "Bloqueio de rota interna do Monitor"

  titulo "Deploy concluído!"
  echo "  • Endereço público (Caddy) : ${ENV_MAP[SENO_WEB_ORIGIN]}"
  echo "  • Host configurado         : ${ENV_MAP[SENO_DOMAIN]}"
  echo "  • Super Admin              : ${ENV_MAP[SENO_SUPER_EMAIL]}"
  if [[ "$ADMIN_PASSWORD_GENERATED" == "true" ]]; then
    echo "  • Senha gerada Super Admin : ${ENV_MAP[SENO_SUPER_PASSWORD]}"
    warn "Anote a senha gerada acima e altere-a após o primeiro acesso."
  fi
  if [[ "$tls_enabled" == "true" && -f "$CERT_FULL" ]]; then
    if openssl x509 -in "$CERT_FULL" -noout -issuer -subject 2>/dev/null | grep -qE "issuer=.*${ENV_MAP[SENO_DOMAIN]}"; then
      info "Certificado autoassinado ativo: na primeira visita, aceite o aviso do navegador (Avançado → Continuar)."
    fi
  fi
  if [[ -z "${ENV_MAP[SENO_SMTP_HOST]:-}" ]]; then
    info "SMTP não configurado: links de convite/recuperação são gravados no Log operacional da API."
  fi
  echo ""
  echo "Comandos úteis:"
  echo "  docker compose ${compose_args[*]} ps"
  echo "  docker compose ${compose_args[*]} logs -f caddy web api monitor"
}

main() {
  verificar_ambiente_linux
  cd "$REPO_ROOT"

  if [[ -f "$ENV_FILE" ]]; then
    ler_env_arquivo "$ENV_FILE" ENV_MAP
  elif [[ -f "$ENV_EXAMPLE" ]]; then
    ler_env_arquivo "$ENV_EXAMPLE" ENV_MAP
  fi

  garantir_segredos
  garantir_judge0_conf
  coletar_configuracao
  gravar_env
  subir_stack
}

main "$@"
