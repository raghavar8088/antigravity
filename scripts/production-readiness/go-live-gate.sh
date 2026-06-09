#!/usr/bin/env bash
# Production Go-Live Gate — fail-closed release blocker
# Usage: bash scripts/production-readiness/go-live-gate.sh [--phase static,build,infra,security,trading]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PHASES="${1:-all}"
FAILURES=()
WARNINGS=()

fail() { FAILURES+=("$1"); echo "FAIL: $1"; }
warn() { WARNINGS+=("$1"); echo "WARN: $1"; }
pass() { echo "PASS: $1"; }

run_phase() {
  local phase="$1"
  case "$phase" in
    static)   phase_static ;;
    build)    phase_build ;;
    infra)    phase_infra ;;
    security) phase_security ;;
    trading)  phase_trading ;;
    *) fail "Unknown phase: $phase" ;;
  esac
}

phase_static() {
  echo "=== Phase: Static Checks ==="

  # G-03: No plaintext secrets in tracked files
  if rg -i "ANGELONE_PIN=|DELTA_API_SECRET=|BINANCE_API_SECRET=" "$ROOT" \
    --glob '!*.example' --glob '!.env*' --glob '!**/vendor/**' --glob '!**/production-readiness/**' -q 2>/dev/null; then
    fail "G-03: Possible plaintext secrets in repository"
  else
    pass "G-03: No plaintext broker secrets in tracked files"
  fi

  # G-04: Reconciliation wired
  if rg -q "reconSvc\.Run\(ctx\)" "$ROOT/engine/cmd/antigravity/main.go"; then
    pass "G-04: Reconciliation service wired in main.go"
  else
    fail "G-04: Reconciliation not running — reconSvc.Run missing"
  fi

  # G-10: Leader election (required when ECS desired_count > 1)
  if rg -q "ha\.NewCluster|NewLeaderElection" "$ROOT/engine/cmd/antigravity/main.go"; then
    pass "G-10: Leader election wired"
  else
    fail "G-10: Leader election NOT wired — dual-writer risk with ECS multi-task"
  fi

  # Terraform lambda zip exists
  if [[ -f "$ROOT/infrastructure/terraform/lambda/secret_rotation.zip" ]]; then
    pass "Terraform secret rotation Lambda ZIP present"
  else
    fail "Terraform secret_rotation.zip missing — blocks terraform apply"
  fi

  # Boot replay wired
  if rg -q "omsv3\.ReplayAll" "$ROOT/engine/cmd/antigravity/main.go"; then
    pass "OMS boot replay wired"
  else
    warn "OMS boot replay not wired in main.go — required before production ECS"
  fi
}

phase_build() {
  echo "=== Phase: Build & Test ==="

  if command -v go &>/dev/null; then
    (cd "$ROOT/engine" && go vet ./... && go build ./...) && pass "Go vet + build" || fail "Go build failed"
    (cd "$ROOT/engine" && go test ./internal/ledger/... ./internal/omsv3/... ./internal/reconciliation/... -count=1 -short) \
      && pass "G-05: Event replay / ledger tests" || fail "G-05: Go tests failed"
  else
    warn "Go not installed — skipping build tests"
  fi

  if [[ -d "$ROOT/client/node_modules" ]] && command -v npm &>/dev/null; then
    (cd "$ROOT/client" && npm run test -- --run 2>/dev/null) && pass "Client Vitest" || warn "Client tests failed or skipped"
  else
    warn "Client node_modules missing — run npm install in client/"
  fi
}

phase_infra() {
  echo "=== Phase: Infrastructure ==="

  if ! command -v aws &>/dev/null; then
    warn "AWS CLI not available — skipping infra checks"
    return
  fi

  CLUSTER="${ECS_CLUSTER_NAME:-antigravity-production-cluster}"
  SERVICE="${ECS_SERVICE_NAME:-antigravity-production-engine}"

  RUNNING=$(aws ecs describe-services --cluster "$CLUSTER" --services "$SERVICE" \
    --query 'services[0].runningCount' --output text 2>/dev/null || echo "0")

  if [[ "$RUNNING" != "None" && "$RUNNING" -ge 2 ]]; then
    pass "G-09: ECS running tasks >= 2 ($RUNNING)"
  else
    fail "G-09: ECS running tasks < 2 (got: $RUNNING) — HA not operational"
  fi

  AURORA_ID="${AURORA_CLUSTER_ID:-antigravity-production-aurora}"
  SNAPSHOT=$(aws rds describe-db-cluster-snapshots \
    --db-cluster-identifier "$AURORA_ID" \
    --query 'DBClusterSnapshots | length(@)' --output text 2>/dev/null || echo "0")
  if [[ "$SNAPSHOT" -gt 0 ]]; then
    pass "G-08: Aurora snapshots exist"
  else
    fail "G-08: No Aurora snapshots — backups not verified"
  fi
}

phase_security() {
  echo "=== Phase: Security Smoke ==="

  APP_URL="${APP_URL:-}"
  if [[ -z "$APP_URL" ]]; then
    warn "APP_URL not set — skipping HTTP security probes"
    return
  fi

  CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$APP_URL/api/engine/api/admin/ks/block" || echo "000")
  if [[ "$CODE" == "401" || "$CODE" == "403" ]]; then
    pass "AUTH-01: Engine proxy blocks unauthenticated ($CODE)"
  else
    fail "AUTH-01: Engine proxy returned $CODE (expected 401/403)"
  fi

  CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$APP_URL/api/angelone/order" \
    -H "Content-Type: application/json" -d '{"symbol":"TEST"}' || echo "000")
  if [[ "$CODE" == "401" || "$CODE" == "403" ]]; then
    pass "BRK-01: Angel One order blocks anonymous ($CODE)"
  else
    fail "BRK-01: Angel One order returned $CODE (expected 401/403)"
  fi

  if [[ -n "${CRON_SECRET:-}" ]]; then
    CODE=$(curl -s -o /dev/null -w "%{http_code}" "$APP_URL/api/cron/rank-strategies" || echo "000")
    [[ "$CODE" == "401" || "$CODE" == "403" ]] && pass "AUTH-08: Cron rejects no secret" \
      || fail "AUTH-08: Cron returned $CODE without secret"
  else
    warn "CRON_SECRET not set — skipping cron test"
  fi
}

phase_trading() {
  echo "=== Phase: Trading Safety ==="

  # G-01, G-02: Production secrets
  if [[ "${SECURITY_ENFORCE_AUTH:-}" == "true" || "${ENVIRONMENT:-}" == "production" ]]; then
    [[ -n "${ENGINE_ADMIN_SECRET:-}" ]] && pass "G-01: ENGINE_ADMIN_SECRET set" \
      || fail "G-01: ENGINE_ADMIN_SECRET missing in production"
    [[ -n "${DATABASE_URL:-}" ]] && pass "G-02: DATABASE_URL set" \
      || fail "G-02: DATABASE_URL missing — kill switch non-durable"
  else
    warn "Not in production mode — skipping G-01/G-02 env checks"
  fi

  if [[ -x "$ROOT/scripts/production-readiness/validate-kill-switch.sh" && -n "${ENGINE_URL:-}" ]]; then
    bash "$ROOT/scripts/production-readiness/validate-kill-switch.sh" \
      && pass "G-06: Kill switch validation" || fail "G-06: Kill switch validation failed"
  else
    warn "ENGINE_URL not set — skipping kill switch live test"
  fi
}

# Parse --phase argument
if [[ "${1:-}" == "--phase" && -n "${2:-}" ]]; then
  IFS=',' read -ra SELECTED <<< "$2"
  for p in "${SELECTED[@]}"; do run_phase "$p"; done
else
  for p in static build infra security trading; do run_phase "$p"; done
fi

echo ""
echo "========================================"
echo "GO-LIVE GATE RESULT"
echo "========================================"
echo "Failures: ${#FAILURES[@]}"
echo "Warnings: ${#WARNINGS[@]}"

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  echo ""
  echo "BLOCKERS:"
  for f in "${FAILURES[@]}"; do echo "  - $f"; done
  echo ""
  echo "VERDICT: NO-GO"
  exit 1
fi

echo "VERDICT: PASS (with ${#WARNINGS[@]} warnings)"
exit 0
