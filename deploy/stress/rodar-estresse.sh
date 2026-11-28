#!/usr/bin/env bash
# deploy/stress/rodar-estresse.sh
# Executa o teste de estresse completo em Linux/macOS (seed SQL + painel k6 ao vivo + relatórios HTML/JSON).

set -euo pipefail

TOTAL_ALUNOS="${TOTAL_ALUNOS:-100}"
RAMP_UP_S="${RAMP_UP_S:-25}"
DURACAO_PROVA_S="${DURACAO_PROVA_S:-120}"
JANELA_ENTREGA_S="${JANELA_ENTREGA_S:-20}"
API_URL="${API_URL:-http://localhost:8080}"
MONITOR_URL="${MONITOR_URL:-http://localhost:8081}"
WEB_ORIGIN="${WEB_ORIGIN:-}"
SENHA_PADRAO="${SENHA_PADRAO:-102938}"
EXECUTAR_ENSAIOS="${EXECUTAR_ENSAIOS:-true}"
PORTA_DASHBOARD="${PORTA_DASHBOARD:-5665}"

STRESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RAIZ="$(cd "${STRESS_DIR}/../.." && pwd)"
mkdir -p "${STRESS_DIR}/resultados"

STAMP="$(date +%Y%m%d-%H%M%S)"
DASHBOARD_REL="deploy/stress/resultados/dashboard-tempo-real-${STAMP}.html"

echo "========================================================================"
echo " SENO — TESTE DE ESTRESSE (${TOTAL_ALUNOS} ALUNOS + 1 PROFESSOR)"
echo "========================================================================"

curl -fsS "${API_URL}/healthz" >/dev/null || { echo "API indisponível em ${API_URL}/healthz"; exit 1; }
curl -fsS "${MONITOR_URL}/healthz" >/dev/null || { echo "Monitor indisponível em ${MONITOR_URL}/healthz"; exit 1; }
echo "[OK] API e Monitor acessíveis."

PG_CONTAINER=""
if command -v docker >/dev/null 2>&1; then
  PG_CONTAINER="$(docker ps --filter "ancestor=postgres:17-alpine" --format "{{.Names}}" | head -n1 || true)"
  if [[ -z "${PG_CONTAINER}" ]]; then
    PG_CONTAINER="$(docker ps --filter "name=postgres" --format "{{.Names}}" | head -n1 || true)"
  fi
fi

run_psql() {
  local db="$1"
  shift
  if [[ -n "${PG_CONTAINER}" ]]; then
    docker exec -i "${PG_CONTAINER}" psql -U "${POSTGRES_USER:-seno}" -d "${db}" "$@"
  else
    PGPASSWORD="${POSTGRES_PASSWORD:-seno}" psql -h localhost -U "${POSTGRES_USER:-seno}" -d "${db}" "$@"
  fi
}

if [[ "${PULAR_SEED:-0}" != "1" ]]; then
  echo "[1/3] Aplicando seed-stress.sql (${TOTAL_ALUNOS} alunos)..."
  run_psql "${POSTGRES_DB:-seno}" -v ON_ERROR_STOP=1 -v "total_alunos=${TOTAL_ALUNOS}" < "${STRESS_DIR}/seed-stress.sql" >/dev/null
  echo "[OK] Seed concluído."
fi

echo "[2/3] Executando k6..."
echo "  -> Painel EM TEMPO REAL: http://localhost:${PORTA_DASHBOARD}"

cd "${RAIZ}"
if command -v k6 >/dev/null 2>&1; then
  K6_WEB_DASHBOARD=true \
  K6_WEB_DASHBOARD_HOST=0.0.0.0 \
  K6_WEB_DASHBOARD_PORT="${PORTA_DASHBOARD}" \
  K6_WEB_DASHBOARD_PERIOD=2s \
  K6_WEB_DASHBOARD_EXPORT="${DASHBOARD_REL}" \
  API_URL="${API_URL}" \
  MONITOR_URL="${MONITOR_URL}" \
  WEB_ORIGIN="${WEB_ORIGIN}" \
  SENHA_PADRAO="${SENHA_PADRAO}" \
  TOTAL_ALUNOS="${TOTAL_ALUNOS}" \
  RAMP_UP_S="${RAMP_UP_S}" \
  DURACAO_PROVA_S="${DURACAO_PROVA_S}" \
  JANELA_ENTREGA_S="${JANELA_ENTREGA_S}" \
  EXECUTAR_ENSAIOS="${EXECUTAR_ENSAIOS}" \
  RESULTADOS_DIR="deploy/stress/resultados" \
  k6 run "deploy/stress/prova-100-alunos.k6.js"
else
  docker run --rm -i -u 0 --network host \
    -v "${RAIZ}:/work" -w /work \
    -e K6_WEB_DASHBOARD=true \
    -e K6_WEB_DASHBOARD_HOST=0.0.0.0 \
    -e K6_WEB_DASHBOARD_PORT="${PORTA_DASHBOARD}" \
    -e K6_WEB_DASHBOARD_PERIOD=2s \
    -e K6_WEB_DASHBOARD_EXPORT="${DASHBOARD_REL}" \
    -e API_URL="${API_URL}" \
    -e MONITOR_URL="${MONITOR_URL}" \
    -e WEB_ORIGIN="${WEB_ORIGIN}" \
    -e SENHA_PADRAO="${SENHA_PADRAO}" \
    -e TOTAL_ALUNOS="${TOTAL_ALUNOS}" \
    -e RAMP_UP_S="${RAMP_UP_S}" \
    -e DURACAO_PROVA_S="${DURACAO_PROVA_S}" \
    -e JANELA_ENTREGA_S="${JANELA_ENTREGA_S}" \
    -e EXECUTAR_ENSAIOS="${EXECUTAR_ENSAIOS}" \
    -e RESULTADOS_DIR="deploy/stress/resultados" \
    grafana/k6:latest run "deploy/stress/prova-100-alunos.k6.js"
fi

echo "[3/3] Estado pós-teste das filas SQL:"
run_psql seno_monitor -c "SELECT kind, status, count(*) AS qtd FROM jobs GROUP BY 1, 2 ORDER BY 1, 2;" || true
run_psql "${POSTGRES_DB:-seno}" -c "SELECT kind, status, count(*) AS qtd FROM jobs GROUP BY 1, 2 ORDER BY 1, 2;" || true

echo ""
echo "Relatórios gerados em ${STRESS_DIR}/resultados/:"
echo "  - relatorio-seno.html"
echo "  - $(basename "${DASHBOARD_REL}")"
echo "  - resumo.json"
