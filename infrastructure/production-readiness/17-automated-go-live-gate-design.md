# 17 — Automated Go-Live Gate Design

**Implementation:** `scripts/production-readiness/go-live-gate.sh` + `engine/internal/validation/production/gate.go`

---

## Gate Philosophy

**Fail-closed:** Any CRITICAL check failure blocks release. No waivers for capital-protection controls.

---

## Gate Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    go-live-gate.sh (CI/CD)                   │
├─────────────────────────────────────────────────────────────┤
│  Phase 1: Static checks (no network)                        │
│  Phase 2: Build & test (go test, npm test)                  │
│  Phase 3: Environment validation (secrets present)          │
│  Phase 4: Infrastructure health (AWS CLI)                   │
│  Phase 5: Security smoke (HTTP probes)                      │
│  Phase 6: Trading safety (kill switch, recon, replay)       │
└─────────────────────────────────────────────────────────────┘
         │ PASS                          │ FAIL
         ▼                               ▼
   Deploy proceeds              Pipeline blocked + alert
```

---

## Hard Blockers (Release MUST FAIL)

| ID | Check | Method |
|----|-------|--------|
| G-01 | `ENGINE_ADMIN_SECRET` missing | Env var / Secrets Manager |
| G-02 | `DATABASE_URL` missing (production) | Env var / Secrets Manager |
| G-03 | Secrets in plaintext in repo | `rg` scan |
| G-04 | Reconciliation not in main.go | `rg reconSvc.Run` |
| G-05 | Event replay tests fail | `go test ./internal/omsv3/...` |
| G-06 | Kill switch persistence test fail | `validate-kill-switch.sh` |
| G-07 | Broker auth test fail | `validate-broker-security.sh` |
| G-08 | Aurora backup exists | AWS CLI snapshot check |
| G-09 | ECS health checks pass | `runningCount >= 2`, ALB healthy |
| G-10 | Leader election wired (if ECS count > 1) | `rg ha.NewCluster main.go` |

---

## CI/CD Integration

### GitHub Actions (Pre-Deploy)

```yaml
name: Production Go-Live Gate

on:
  workflow_dispatch:
  push:
    branches: [main]
    paths: ['engine/**', 'client/**', 'infrastructure/**']

jobs:
  gate:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4

      - name: Static + build gate
        run: bash scripts/production-readiness/go-live-gate.sh --phase static,build

      - name: Configure AWS
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
          aws-region: ap-south-1

      - name: Infrastructure + security gate
        run: bash scripts/production-readiness/go-live-gate.sh --phase infra,security
        env:
          APP_URL: ${{ secrets.PRODUCTION_APP_URL }}
          ENGINE_URL: ${{ secrets.ENGINE_URL }}
          ENGINE_ADMIN_SECRET: ${{ secrets.ENGINE_ADMIN_SECRET }}
          CRON_SECRET: ${{ secrets.CRON_SECRET }}
```

### Engine Boot Gate

`gate.go` runs at engine startup when `SECURITY_ENFORCE_AUTH=true`:

```go
result := production.RunBootGate(production.BootGateConfig{...})
if !result.Passed {
    log.Fatalf("[BOOT GATE] FATAL: %v", result.Blockers)
}
```

---

## Output Format

```json
{
  "passed": false,
  "timestamp": "2026-06-09T12:00:00Z",
  "blockers": [
    "G-02: DATABASE_URL not set in production",
    "G-10: Leader election not wired — dual-writer risk with ECS count=2"
  ],
  "warnings": [
    "G-08: Aurora snapshot older than 24h"
  ],
  "scores": {
    "security": 68,
    "reliability": 58,
    "capital_protection": 72,
    "production_readiness": 59
  }
}
```

---

## Waiver Process

| Blocker Type | Waiver Authority | Max Duration |
|--------------|------------------|--------------|
| CRITICAL | None — no waivers | N/A |
| HIGH | CISO + CTO joint | 7 days |
| MEDIUM | Engineering Lead | 30 days |

---

## Usage

```bash
# Full gate
bash scripts/production-readiness/go-live-gate.sh

# Specific phases
bash scripts/production-readiness/go-live-gate.sh --phase static,build
bash scripts/production-readiness/go-live-gate.sh --phase security
bash scripts/production-readiness/go-live-gate.sh --phase infra
```

**Gate Design Readiness:** 90/100 (implemented, pending CI wiring)
