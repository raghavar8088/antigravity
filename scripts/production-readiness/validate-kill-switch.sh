#!/usr/bin/env bash
# Kill switch validation — see doc 10
set -euo pipefail

ENGINE_URL="${ENGINE_URL:?Set ENGINE_URL}"
SECRET="${ENGINE_ADMIN_SECRET:?Set ENGINE_ADMIN_SECRET}"

hdr=(-H "X-Engine-Admin-Secret: $SECRET" -H "Content-Type: application/json")

echo "=== Kill Switch Validation ==="

# KS-11: No secret
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ENGINE_URL/api/admin/ks/block")
if [[ "$CODE" == "401" || "$CODE" == "403" ]]; then
  echo "PASS: KS-11 block without secret ($CODE)"
else
  echo "FAIL: KS-11 — got $CODE"
  exit 1
fi

# Status check
STATUS=$(curl -s "${hdr[@]}" "$ENGINE_URL/api/admin/ks/status")
echo "Initial status: $STATUS"

# KS-01: Activate block
curl -sf -X POST "${hdr[@]}" "$ENGINE_URL/api/admin/ks/block" -d '{"reason":"go-live-gate test"}' \
  && echo "PASS: KS-01 block activated" || { echo "FAIL: KS-01"; exit 1; }

ACTIVE=$(curl -s "${hdr[@]}" "$ENGINE_URL/api/admin/ks/status")
echo "Active status: $ACTIVE"
echo "$ACTIVE" | grep -qi '"active":true' && echo "PASS: KS-01 confirmed active" \
  || { echo "FAIL: kill switch not active"; exit 1; }

# KS-07: Release
curl -sf -X POST "${hdr[@]}" "$ENGINE_URL/api/admin/ks/release" \
  && echo "PASS: KS-07 released" || { echo "FAIL: KS-07 release"; exit 1; }

echo "VERDICT: PASS (restart persistence test KS-03 manual — restart engine and re-run)"
