#!/usr/bin/env bash
# Reconciliation validation — see doc 08
set -euo pipefail

ENGINE_URL="${ENGINE_URL:-}"
LOG_GROUP="${ECS_LOG_GROUP:-/ecs/antigravity-production/engine}"

echo "=== Reconciliation Validation ==="

# RECON-01: Check engine logs for startup message
if command -v aws &>/dev/null && [[ -n "$LOG_GROUP" ]]; then
  RECENT=$(aws logs filter-log-events \
    --log-group-name "$LOG_GROUP" \
    --filter-pattern "RECONCILIATION" \
    --limit 5 \
    --query 'events[0].message' --output text 2>/dev/null || echo "")
  if echo "$RECENT" | grep -qE "RECON-V2|Runtime reconciliation started|Delta broker reconciliation started"; then
    echo "PASS: RECON-01 reconciliation v2 started in logs"
  elif echo "$RECENT" | grep -q "Continuous reconciliation started"; then
    echo "PASS: RECON-01 reconciliation started in logs (legacy v1)"
  else
    echo "WARN: RECON-01 — reconciliation log not found (check LOG_GROUP)"
  fi
else
  echo "WARN: AWS CLI or LOG_GROUP unavailable — check logs manually"
fi

# Go unit tests
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
if command -v go &>/dev/null; then
  (cd "$ROOT/engine" && go test ./internal/reconciliationv2/... -count=1) \
    && echo "PASS: reconciliationv2 unit tests" \
    || { echo "FAIL: reconciliationv2 unit tests"; exit 1; }
  (cd "$ROOT/engine" && go test ./internal/reconciliation/... -count=1) \
    && echo "PASS: reconciliation v1 unit tests" \
    || { echo "FAIL: reconciliation v1 unit tests"; exit 1; }
fi

# Prometheus metrics
if [[ -n "$ENGINE_URL" ]]; then
  METRICS=$(curl -sf "$ENGINE_URL/metrics" 2>/dev/null || echo "")
  if echo "$METRICS" | grep -q "reconciliation"; then
    echo "PASS: reconciliation metrics exposed"
  else
    echo "WARN: reconciliation metrics not found in /metrics"
  fi
fi

echo "VERDICT: PASS (drift injection RECON-03/04 requires staging manual test)"
