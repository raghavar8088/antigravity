#!/usr/bin/env bash
# Pre-start health check — verifies DB connections before the engine boots.
# Exits 0 on success, non-zero on failure (systemd will not start the service).
set -euo pipefail

ENV_FILE="${ENV_FILE:-/home/ubuntu/btc-pilot/engine/.env}"
[[ -f "$ENV_FILE" ]] && export $(grep -v '^#' "$ENV_FILE" | xargs)

MAX_RETRIES=5
SLEEP_SEC=3

# ── MongoDB ────────────────────────────────────────────────────────────────────
if [[ -n "${MONGODB_URI:-}" ]]; then
  for i in $(seq 1 $MAX_RETRIES); do
    if mongosh --quiet --eval "db.runCommand({ping:1})" "$MONGODB_URI" > /dev/null 2>&1; then
      echo "[pre-start] MongoDB OK"
      break
    fi
    echo "[pre-start] MongoDB attempt $i/$MAX_RETRIES failed, retrying in ${SLEEP_SEC}s..."
    sleep $SLEEP_SEC
    if [[ $i -eq $MAX_RETRIES ]]; then
      echo "[pre-start] WARNING: MongoDB unreachable — engine will run in degraded mode"
    fi
  done
fi

# ── PostgreSQL (Neon) ──────────────────────────────────────────────────────────
if [[ -n "${DATABASE_URL:-}" ]]; then
  for i in $(seq 1 $MAX_RETRIES); do
    if psql "$DATABASE_URL" -c "SELECT 1" > /dev/null 2>&1; then
      echo "[pre-start] PostgreSQL OK"
      break
    fi
    echo "[pre-start] PostgreSQL attempt $i/$MAX_RETRIES failed, retrying in ${SLEEP_SEC}s..."
    sleep $SLEEP_SEC
    if [[ $i -eq $MAX_RETRIES ]]; then
      echo "[pre-start] WARNING: PostgreSQL unreachable — ledger persistence degraded"
    fi
  done
fi

echo "[pre-start] Pre-start health checks complete"
exit 0
