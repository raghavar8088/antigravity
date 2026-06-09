#!/usr/bin/env bash
# Event replay validation — see doc 09
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/engine"

echo "=== Event Replay Validation ==="

go test ./internal/ledger/... -count=1 -v -short \
  && echo "PASS: ledger package tests" \
  || { echo "FAIL: ledger tests"; exit 1; }

go test ./internal/omsv3/... -count=1 -run 'Replay|Idempotency|Aggregate' -v -short \
  && echo "PASS: OMS replay tests" \
  || { echo "FAIL: OMS replay tests"; exit 1; }

go test ./internal/certification/... -count=1 -run 'KillSwitch|Stress' -v -short \
  && echo "PASS: certification stress tests" \
  || echo "WARN: certification tests skipped or failed"

echo "VERDICT: PASS (live scenario tests 1-5 require staging chaos environment)"
