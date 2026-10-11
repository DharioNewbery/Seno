# deploy/dev.ps1 — utilitário de desenvolvimento: sobe, para e consulta
# o status da API (Go), do web (SvelteKit) e do monitor (Fase 9).
# Processos 100% desacoplados do terminal (WScript.Shell.Run, janela oculta,
# stdout para arquivo), então chamar este script NUNCA trava.
#
# Uso:
#   & deploy\dev.ps1 -Modo all      # (padrão) sobe API + web + monitor
#   & deploy\dev.ps1 -Modo api      # só API
#   & deploy\dev.ps1 -Modo web      # só web
#   & deploy\dev.ps1 -Modo monitor  # só monitor
#   & deploy\dev.ps1 -Modo stop     # para todos
#   & deploy\dev.ps1 -Modo status   # lista portas e PIDs
#
# Logs em %TEMP%\opencode\ (seno-api.log/err, seno-web.log/err, seno-monitor.log/err).
# Pré-requisito: docker compose -f compose.dev.yaml up -d (PostgreSQL) —
# e, para ensaios: docker compose -f judge0/compose.yaml up -d (Judge0).

param(
  [ValidateSet("all", "api", "web", "monitor", "stop", "status")]
  $Modo = "all"
)

$ErrorActionPreference = "Stop"
$raiz = Split-Path -Parent $PSScriptRoot
$logDir = Join-Path $env:TEMP "opencode"
New-Item -ItemType Directory -Force $logDir | Out-Null

$NL = [string][char]13 + [char]10
$Q = [string][char]34

function Ler-Env {
  $arquivo = Join-Path $raiz "deploy\.env"
  Get-Content $arquivo | ForEach-Object {
    if ($_ -match '^\s*([A-Z_0-9]+)=(.*)$') {
      [Environment]::SetEnvironmentVariable($matches[1], $matches[2].Trim(), "Process")
    }
  }
}

function Matar-Porta($porta) {
  $conn = Get-NetTCPConnection -LocalPort $porta -State Listen -ErrorAction SilentlyContinue |
    Select-Object -First 1
  if ($conn) {
    Stop-Process -Id $conn.OwningProcess -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
  }
}

# Lançamento desacoplado: um .cmd temporário (cd + comando com
# redirecionamento para arquivos) roda via wscript COM — sem herdar o
# stdout/stderr do terminal, nada fica segurando o shell.
function Iniciar($diretorio, $comando, $prefixo) {
  $out = Join-Path $logDir ($prefixo + ".log")
  $err = Join-Path $logDir ($prefixo + ".err.log")
  Remove-Item -LiteralPath $out -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $err -ErrorAction SilentlyContinue
  $conteudo = "@echo off${NL}cd /d ${Q}${diretorio}${Q}${NL}${comando} > ${Q}${out}${Q} 2> ${Q}${err}${Q}${NL}"
  $bat = Join-Path $logDir ($prefixo + "-start.cmd")
  [IO.File]::WriteAllText($bat, $conteudo)
  $sh = New-Object -ComObject WScript.Shell
  $linha = $Q + $bat + $Q
  $sh.Run($linha, 0, $false) | Out-Null
}

switch ($Modo) {
  "api" {
    Ler-Env
    Matar-Porta 8080
    Iniciar (Join-Path $raiz "api") "go run ./cmd/api" "seno-api"
    "API lancada (logs em $logDir\seno-api.log)"
  }
  "web" {
    Ler-Env
    [Environment]::SetEnvironmentVariable("SENO_API_URL", "http://localhost:8080", "Process")
    [Environment]::SetEnvironmentVariable("SENO_WEB_ORIGIN", "http://localhost:5173", "Process")
    Matar-Porta 5173
    Iniciar (Join-Path $raiz "web") "npm run dev" "seno-web"
    "Web lancado (logs em $logDir\seno-web.log)"
  }
  "monitor" {
    Ler-Env
    # Banco próprio do Monitor (seno_monitor no mesmo servidor de dev).
    [Environment]::SetEnvironmentVariable("SENO_DATABASE_URL",
      "postgres://seno:seno@localhost:5432/seno_monitor?sslmode=disable", "Process")
    Matar-Porta 8081
    Iniciar (Join-Path $raiz "monitor") "go run ./cmd/monitor" "seno-monitor"
    "Monitor lancado (logs em $logDir\seno-monitor.log)"
  }
  "all" {
    Ler-Env
    Matar-Porta 8080
    Iniciar (Join-Path $raiz "api") "go run ./cmd/api" "seno-api"
    Matar-Porta 5173
    [Environment]::SetEnvironmentVariable("SENO_API_URL", "http://localhost:8080", "Process")
    [Environment]::SetEnvironmentVariable("SENO_WEB_ORIGIN", "http://localhost:5173", "Process")
    Iniciar (Join-Path $raiz "web") "npm run dev" "seno-web"
    # Monitor nativo (banco seno_monitor, :8081) — Fase 9.
    [Environment]::SetEnvironmentVariable("SENO_DATABASE_URL",
      "postgres://seno:seno@localhost:5432/seno_monitor?sslmode=disable", "Process")
    Matar-Porta 8081
    Iniciar (Join-Path $raiz "monitor") "go run ./cmd/monitor" "seno-monitor"
    "API, web e monitor lancados (logs em ${logDir})"
  }
  "stop" {
    Matar-Porta 8080
    Matar-Porta 5173
    Matar-Porta 8081
    "Processos de dev parados."
  }
  "status" {
    foreach ($item in @(@("API", 8080), @("Web", 5173), @("Monitor", 8081))) {
      $porta = Get-NetTCPConnection -LocalPort $item[1] -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
      if ($porta) {
        $txt = $item[0] + ": no ar (PID " + $porta.OwningProcess + ", porta " + $item[1] + ")"
        Write-Output $txt
      } else {
        $txt = $item[0] + ": fora do ar (porta " + $item[1] + ")"
        Write-Output $txt
      }
    }
  }
}
