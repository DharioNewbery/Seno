# deploy/stress/rodar-estresse.ps1
# Executa o teste de estresse completo no Seno (seed SQL + painel k6 ao vivo + relatórios HTML/JSON + auditoria das filas SQL).
#
# Uso rápido (com a stack rodando em compose.dev.yaml ou dev.ps1):
#   .\deploy\stress\rodar-estresse.ps1
#   .\deploy\stress\rodar-estresse.ps1 -TotalAlunos 100 -DuracaoProvaSeg 180 -AbrirRelatorio

param(
  [int]$TotalAlunos = 100,
  [int]$RampUpSeg = 25,
  [int]$DuracaoProvaSeg = 120,
  [int]$JanelaEntregaSeg = 20,
  [string]$ApiUrl = "http://localhost:8080",
  [string]$MonitorUrl = "http://localhost:8081",
  [string]$WebOrigin = "",
  [string]$SenhaPadrao = "102938",
  [bool]$ExecutarEnsaios = $true,
  [int]$PortaDashboard = 5665,
  [switch]$PularSeed,
  [switch]$AbrirRelatorio
)

$ErrorActionPreference = "Stop"
$stressDir = $PSScriptRoot
$raiz = Split-Path -Parent (Split-Path -Parent $stressDir)
$resultadosHost = Join-Path $stressDir "resultados"
New-Item -ItemType Directory -Force $resultadosHost | Out-Null

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$dashboardHtmlRel = "deploy/stress/resultados/dashboard-tempo-real-$stamp.html"

Write-Host "========================================================================" -ForegroundColor Cyan
Write-Host " SENO — TESTE DE ESTRESSE ($TotalAlunos ALUNOS + 1 PROFESSOR)" -ForegroundColor Cyan
Write-Host "========================================================================" -ForegroundColor Cyan

# 1. Verifica saúde da API e do Monitor
foreach ($alvo in @(@("API", "$ApiUrl/healthz"), @("Monitor", "$MonitorUrl/healthz"))) {
  try {
    $resp = Invoke-WebRequest -Uri $alvo[1] -UseBasicParsing -TimeoutSec 5
    if ($resp.StatusCode -ne 200) { throw "Status $($resp.StatusCode)" }
    Write-Host "[OK] $($alvo[0]) respondendo em $($alvo[1])" -ForegroundColor Green
  } catch {
    Write-Error "$($alvo[0]) não está acessível em $($alvo[1]). Suba a stack (docker compose -f deploy/compose.dev.yaml up -d ou .\deploy\dev.ps1) antes do teste."
    exit 1
  }
}

# Descobre container PostgreSQL (se estiver rodando via Docker)
$pgContainer = $null
if (Get-Command docker -ErrorAction SilentlyContinue) {
  $pgContainer = docker ps --filter "ancestor=postgres:17-alpine" --format "{{.Names}}" | Select-Object -First 1
  if (-not $pgContainer) {
    $pgContainer = docker ps --filter "name=postgres" --format "{{.Names}}" | Select-Object -First 1
  }
}

function Invoke-Psql([string]$Database, [string]$SqlFile = "", [string]$Command = "") {
  if ($pgContainer) {
    if ($SqlFile) {
      Get-Content $SqlFile -Raw | docker exec -i $pgContainer psql -v ON_ERROR_STOP=1 -v "total_alunos=$TotalAlunos" -U seno -d $Database -f -
    } else {
      docker exec -i $pgContainer psql -U seno -d $Database -c $Command
    }
  } elseif (Get-Command psql -ErrorAction SilentlyContinue) {
    $env:PGPASSWORD = "seno"
    if ($SqlFile) {
      psql -h localhost -U seno -d $Database -v ON_ERROR_STOP=1 -v "total_alunos=$TotalAlunos" -f $SqlFile
    } else {
      psql -h localhost -U seno -d $Database -c $Command
    }
  } else {
    throw "Não foi encontrado container postgres ativo nem cliente psql local."
  }
}

# 2. Aplica seed-stress.sql (idempotente)
if (-not $PularSeed) {
  Write-Host "`n[1/3] Provisionando professor + $TotalAlunos alunos matriculados (seed-stress.sql)..." -ForegroundColor Yellow
  $seedPath = Join-Path $stressDir "seed-stress.sql"
  Invoke-Psql -Database "seno" -SqlFile $seedPath | Out-Null
  Write-Host "[OK] Seed aplicado com sucesso." -ForegroundColor Green
}

# 3. Executa k6 com Dashboard Web em Tempo Real (:5665) + Exportação de Relatórios
Write-Host "`n[2/3] Iniciando simulação no Grafana k6..." -ForegroundColor Yellow
Write-Host "  -> Acompanhe EM TEMPO REAL no navegador: http://localhost:$PortaDashboard" -ForegroundColor Magenta
Write-Host ""

$temK6Local = [bool](Get-Command k6 -ErrorAction SilentlyContinue)

Push-Location $raiz
try {
  if ($temK6Local) {
    $env:K6_WEB_DASHBOARD = "true"
    $env:K6_WEB_DASHBOARD_HOST = "127.0.0.1"
    $env:K6_WEB_DASHBOARD_PORT = "$PortaDashboard"
    $env:K6_WEB_DASHBOARD_PERIOD = "2s"
    $env:K6_WEB_DASHBOARD_EXPORT = $dashboardHtmlRel

    $env:API_URL = $ApiUrl
    $env:MONITOR_URL = $MonitorUrl
    $env:WEB_ORIGIN = $WebOrigin
    $env:SENHA_PADRAO = $SenhaPadrao
    $env:TOTAL_ALUNOS = "$TotalAlunos"
    $env:RAMP_UP_S = "$RampUpSeg"
    $env:DURACAO_PROVA_S = "$DuracaoProvaSeg"
    $env:JANELA_ENTREGA_S = "$JanelaEntregaSeg"
    $env:EXECUTAR_ENSAIOS = if ($ExecutarEnsaios) { "true" } else { "false" }
    $env:RESULTADOS_DIR = "deploy/stress/resultados"

    k6 run "deploy/stress/prova-100-alunos.k6.js"
  } elseif (Get-Command docker -ErrorAction SilentlyContinue) {
    Write-Host "  (k6 local não encontrado; executando via imagem oficial grafana/k6 no Docker)" -ForegroundColor DarkGray
    $apiDocker = $ApiUrl -replace "localhost|127\.0\.0\.1", "host.docker.internal"
    $monDocker = $MonitorUrl -replace "localhost|127\.0\.0\.1", "host.docker.internal"

    docker run --rm -i -u 0 `
      -p "${PortaDashboard}:${PortaDashboard}" `
      -v "${raiz}:/work" -w /work `
      -e K6_WEB_DASHBOARD=true `
      -e K6_WEB_DASHBOARD_HOST=0.0.0.0 `
      -e K6_WEB_DASHBOARD_PORT="$PortaDashboard" `
      -e K6_WEB_DASHBOARD_PERIOD=2s `
      -e K6_WEB_DASHBOARD_EXPORT="$dashboardHtmlRel" `
      -e API_URL="$apiDocker" `
      -e MONITOR_URL="$monDocker" `
      -e WEB_ORIGIN="$WebOrigin" `
      -e SENHA_PADRAO="$SenhaPadrao" `
      -e TOTAL_ALUNOS="$TotalAlunos" `
      -e RAMP_UP_S="$RampUpSeg" `
      -e DURACAO_PROVA_S="$DuracaoProvaSeg" `
      -e JANELA_ENTREGA_S="$JanelaEntregaSeg" `
      -e EXECUTAR_ENSAIOS="$(if ($ExecutarEnsaios) { 'true' } else { 'false' })" `
      -e RESULTADOS_DIR="deploy/stress/resultados" `
      grafana/k6:latest run "deploy/stress/prova-100-alunos.k6.js"
  } else {
    throw "Instale o Grafana k6 (winget install k6) ou o Docker para executar o teste."
  }
} finally {
  Pop-Location
}

# 4. Auditoria final nos bancos (filas jobs + consistência Monitor <-> API)
Write-Host "`n[3/3] Estado pós-teste das filas e persistência no PostgreSQL:" -ForegroundColor Yellow
try {
  Write-Host "--- Filas do Monitor (seno_monitor.jobs) ---" -ForegroundColor Cyan
  Invoke-Psql -Database "seno_monitor" -Command "SELECT kind, status, count(*) AS qtd FROM jobs GROUP BY 1, 2 ORDER BY 1, 2;"
  Write-Host "--- Filas da API (seno.jobs) ---" -ForegroundColor Cyan
  Invoke-Psql -Database "seno" -Command "SELECT kind, status, count(*) AS qtd FROM jobs GROUP BY 1, 2 ORDER BY 1, 2;"
} catch {
  Write-Host "(Auditoria SQL opcional ignorada: $_)" -ForegroundColor DarkGray
}

$relatorioFinal = Join-Path $resultadosHost "relatorio-seno.html"
$dashboardExport = Join-Path $raiz $dashboardHtmlRel
Write-Host "`nRelatórios gerados:" -ForegroundColor Green
Write-Host "  1. Relatório Executivo Seno : $relatorioFinal"
Write-Host "  2. Dashboard Séries k6 HTML : $dashboardExport"
Write-Host "  3. Métricas Brutas JSON     : $(Join-Path $resultadosHost 'resumo.json')"

if ($AbrirRelatorio -and (Test-Path $relatorioFinal)) {
  Start-Process $relatorioFinal
}
