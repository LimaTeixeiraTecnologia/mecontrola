#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

COMPOSE="deployment/compose/compose.swarm.yml"
FAIL=0

if [ ! -f "$COMPOSE" ]; then
  echo "FAIL: $COMPOSE ausente"
  exit 1
fi

OTELCOL="deployment/telemetry/grafana/otelcol-config.yaml"

echo "==> [1/6] RF-13: order: stop-first presente"
if [ "$(grep -c 'order: stop-first' "$COMPOSE")" -ge 1 ]; then
  echo "OK"
else
  echo "FAIL: order: stop-first ausente"
  FAIL=1
fi

echo "==> [2/6] RF-13: order: start-first proibido (ADR-004)"
if grep -q 'start-first' "$COMPOSE"; then
  echo "FAIL: order: start-first encontrado — proibido no no unico"
  FAIL=1
else
  echo "OK"
fi

echo "==> [3/6] RF-13: stop_grace_period: 30s nos 4 servicos de aplicacao"
if [ "$(grep -c 'stop_grace_period: 30s' "$COMPOSE")" -ge 4 ]; then
  echo "OK"
else
  echo "FAIL: stop_grace_period: 30s ausente em algum servico de aplicacao"
  FAIL=1
fi

echo "==> [4/6] RF-20: OTEL_SERVICE_VERSION cabeado ao IMAGE_TAG"
if grep -q 'OTEL_SERVICE_VERSION: ${IMAGE_TAG}' "$COMPOSE"; then
  echo "OK"
else
  echo "FAIL: OTEL_SERVICE_VERSION nao cabeado a \${IMAGE_TAG}"
  FAIL=1
fi

echo "==> [5/6] RF-11: OTEL_TRACE_SAMPLE_RATE=1 no caminho inbound (SDK envia tudo; coletor amostra via tail_sampling)"
if [ "$(grep 'OTEL_TRACE_SAMPLE_RATE' "$COMPOSE" | grep -c '"1"')" -ge 4 ]; then
  echo "OK"
else
  echo "FAIL: OTEL_TRACE_SAMPLE_RATE deve ser \"1\" nos 4 servicos (tail_sampling no coletor realiza a reducao)"
  FAIL=1
fi

echo "==> [6/6] RF-11/RF-12: tail_sampling no otelcol com policy de erros (100%) e probabilistica"
if [ ! -f "$OTELCOL" ]; then
  echo "FAIL: $OTELCOL ausente"
  FAIL=1
elif grep -q 'tail_sampling' "$OTELCOL" && grep -q 'errors-policy' "$OTELCOL" && grep -q 'probabilistic-policy' "$OTELCOL"; then
  echo "OK"
else
  echo "FAIL: tail_sampling com errors-policy e probabilistic-policy ausentes em $OTELCOL"
  FAIL=1
fi

if [ "$FAIL" -ne 0 ]; then
  echo "==> Gate anti-storm FALHOU"
  exit 1
fi
echo "==> Gate anti-storm OK"
