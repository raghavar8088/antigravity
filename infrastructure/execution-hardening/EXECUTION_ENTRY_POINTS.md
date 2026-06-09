# Execution Entry Points (Post-Hardening Scan)

Generated: 2026-06-09. Source-only evidence.

## Authoritative institutional path

| File | Line | Function | Caller | Callee | Risk | OMS | Broker |
|------|------|----------|--------|--------|------|-----|--------|
| `engine/internal/trading/loop.go` | 296 | `executeThroughInstitutionalPath` | strategy loop, AI, ConfirmSignal | `executeThroughInstitutionalPathWithFill` | PMS+KillSwitch+RiskV2+Elite+Floor | omsv3+ledger | paper `ExecuteSignal` |
| `engine/internal/trading/loop.go` | 305 | `executeThroughInstitutionalPathWithFill` | above + gateway | `fillFn` | same | same | adapter via fillFn |
| `engine/internal/executiongateway/handler.go` | 27 | `ServeHTTP` | POST `/api/execution/request` | `ProcessExecutionRequest` | via ETP | via ETP | via fillFn |
| `engine/internal/trading/institutional_request.go` | 16 | `ProcessExecutionRequest` | gateway, delta-live/order | ETP / WithFill | full stack | full stack | venue-specific |

## Signal confirmation (backend-only execution trigger)

| File | Line | Function | Risk path |
|------|------|----------|-----------|
| `engine/internal/trading/loop.go` | 1855 | `ConfirmSignal` | → ETP line 1889 |
| `engine/internal/trading/loop.go` | 1903 | `ConfirmSignalFromBridge` | → ETP line 1961 |
| `engine/cmd/antigravity/main.go` | 1824 | `/api/ai/submit` | ConfirmSignal |
| `engine/cmd/antigravity/main.go` | 1864 | `/api/ai/bridge-result` | ConfirmSignalFromBridge |

## Delta broker (engine-only, gated)

| File | Line | Function | Gate |
|------|------|----------|------|
| `engine/internal/delta/live_bridge.go` | 116 | `SubmitOrder` | callable only from institutional fillFn |
| `engine/internal/delta/live_bridge.go` | 200 | `OnOpen` | `institutionalOpen` → ETP |
| `engine/internal/delta/live_bridge.go` | 374 | `OnClose` | killCheck + direct close (residual — see BROKER_SECURITY_REPORT) |
| `engine/internal/delta/client.go` | 182 | `PlaceOrder` | only via Bridge.SubmitOrder |

## Retired / blocked paths

| Path | Status |
|------|--------|
| `client/src/app/api/angelone/order` | 410 — retired |
| `client/src/app/api/delta/mirror` | 410 — retired |
| `client/src/app/api/delta/spot` POST | 410 — retired |
| `client/src/app/api/delta/testnet/place-order` | 410 — retired |
| `engine/internal/delta/live_bridge.go` PlaceManualOrder | returns error — use gateway |

## Call graph (production)

```
Frontend submitExecutionRequest()
  → POST /api/execution/request (Next.js)
    → POST /api/execution/request (Go gateway)
      → ProcessExecutionRequest
        → executeThroughInstitutionalPath[WithFill]
          → PMS → PreTradeRiskPipeline → OMS → fillFn → Broker
```
