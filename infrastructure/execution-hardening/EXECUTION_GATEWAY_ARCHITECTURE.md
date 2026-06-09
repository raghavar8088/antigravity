# Execution Gateway Architecture

## Single authoritative path

All broker submissions must pass:

```
executeThroughInstitutionalPathWithFill()
  1. Ledger EventOrderCreated
  2. omsv3.Replay validation
  3. EventOrderValidated
  4. PMS CheckPortfolioRisk (when budget set)
  5. PreTradeRiskPipeline → KillSwitch + RiskV2.ValidateTrade
  6. Elite drawdown gate
  7. EnforceExecutionFloor
  8. OMS transitions (RISK_CHECKED → ACCEPTED)
  9. fillFn(ctx, sig, clientOrderID) → broker adapter
  10. Ledger fill events
```

## HTTP entry points

| Endpoint | Permission | Handler |
|----------|------------|---------|
| `POST /api/execution/request` | `trade.request` or service | `executiongateway.Handler` |
| `POST /api/delta-live/order` | `trade.request` | `ProcessExecutionRequest(venue=delta)` |
| Strategy loop / AI confirm | internal | `executeThroughInstitutionalPath` |

## fillFn routing

| Venue | fillFn | Broker |
|-------|--------|--------|
| paper (default) | `PaperClient.ExecuteSignal` | simulated |
| delta | `Bridge.SubmitOrder` | Delta REST |
| angelone | rejected at gateway | not wired |

## Files

- `engine/internal/trading/loop.go` — core pipeline
- `engine/internal/trading/institutional_request.go` — external requests + delta bridge wiring
- `engine/internal/executiongateway/` — HTTP gateway
