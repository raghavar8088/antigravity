#!/usr/bin/env bash
# Authentication validation — see doc 06
set -euo pipefail

APP_URL="${APP_URL:?Set APP_URL}"
PASS=0
FAIL=0

check() {
  local name="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    echo "PASS: $name (HTTP $actual)"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name — expected $expected, got $actual"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== Authentication Validation ==="
echo "Target: $APP_URL"

# AUTH-01: Engine proxy no session
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$APP_URL/api/engine/api/admin/ks/block")
check "AUTH-01 engine proxy no session" "401" "$CODE"
[[ "$CODE" == "403" ]] && PASS=$((PASS + 1)) && FAIL=$((FAIL - 1))

# AUTH-02: Blocked path
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$APP_URL/api/engine/api/nifty/seed-engine")
check "AUTH-02 blocked path" "403" "$CODE"

# AUTH-03: Fake cookie
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Cookie: raig_session=fake.not.real" "$APP_URL/dashboard")
if [[ "$CODE" == "307" || "$CODE" == "302" || "$CODE" == "401" ]]; then
  echo "PASS: AUTH-03 fake cookie rejected ($CODE)"
  PASS=$((PASS + 1))
else
  echo "FAIL: AUTH-03 fake cookie — got $CODE"
  FAIL=$((FAIL + 1))
fi

# AUTH-08: Cron no secret
if [[ -n "${CRON_SECRET:-}" ]]; then
  CODE=$(curl -s -o /dev/null -w "%{http_code}" "$APP_URL/api/cron/rank-strategies")
  check "AUTH-08 cron no secret" "401" "$CODE"
  [[ "$CODE" == "403" ]] && PASS=$((PASS + 1)) && FAIL=$((FAIL - 1))
else
  echo "WARN: CRON_SECRET not set — skip AUTH-08"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
