#!/usr/bin/env bash
# Broker security validation — see doc 07
set -euo pipefail

APP_URL="${APP_URL:?Set APP_URL}"
FAIL=0

anon_post() {
  local path="$1" name="$2"
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$APP_URL$path" \
    -H "Content-Type: application/json" -d '{}')
  if [[ "$CODE" == "401" || "$CODE" == "403" ]]; then
    echo "PASS: $name ($CODE)"
  else
    echo "FAIL: $name — expected 401/403, got $CODE"
    FAIL=$((FAIL + 1))
  fi
}

anon_get() {
  local path="$1" name="$2"
  CODE=$(curl -s -o /dev/null -w "%{http_code}" "$APP_URL$path")
  if [[ "$CODE" == "401" || "$CODE" == "403" ]]; then
    echo "PASS: $name ($CODE)"
  else
    echo "FAIL: $name — expected 401/403, got $CODE"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== Broker Security Validation ==="

anon_post "/api/angelone/order" "BRK-01 angelone order"
anon_post "/api/angelone/cancel-order" "BRK-02 angelone cancel"
anon_get  "/api/angelone/funds" "BRK-03 angelone funds"
anon_get  "/api/angelone/orders" "BRK-04 angelone orders"
anon_post "/api/delta-live/order" "BRK-05 delta order"

[[ $FAIL -eq 0 ]] && echo "VERDICT: PASS" && exit 0
echo "VERDICT: FAIL ($FAIL tests)"
exit 1
