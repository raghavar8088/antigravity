# TP_OVERRIDE_AUDIT.md
## Phase 22D — Take-Profit Override Audit

There are **two** TP-override sites in the execution path. Both are now audited.

---

## TP override sites (code evidence)

### 1. Profit sanitizer geometry — `sanitizeSignalForProfit` (loop.go:1056)
Adjusts SL/TP to enforce R:R ≥ 2.40 and the TP floor `minSignalTakeProfitPct = 0.50%`.
Recorded at loop.go:1073:
```go
if sig.TakeProfitPct != baseTakeProfitPct {
    o.execIntel.RecordTPOverride(execintel.TPOverrideSample{
        Strategy: aggSig.StrategyName, Source: "sanitize",
        OriginalTP: baseTakeProfitPct, AdjustedTP: sig.TakeProfitPct,
    })
}
```

### 2. Position TP floor — `Manager.OpenPosition` (manager.go:123)
```go
if takeProfitPct < m.config.MinTakeProfitPct {   // 0.30%
    takeProfitPct = m.config.MinTakeProfitPct
}
```
This raises (widens) a too-tight TP to the 0.30% floor.

---

## Audit mechanism (code evidence)

`internal/execintel/tpaudit.go`:
- `add` classifies each override as **tightened** (`AdjustedTP < OriginalTP`),
  **widened** (`AdjustedTP > OriginalTP`), or **unchanged**.
- `updateOutcome` (called from `finalizeExecIntelClose`, loop.go:1213, via
  `RecordTPOutcome`) attributes realized PnL when the trade closes:
  - winner exited at a **tightened** TP → counts as `ReductionUSD` (profit given up)
  - winner exited at a **widened** TP → counts as `ImprovementUSD`
- `Verdict` field reports HELPING / HURTING / NEUTRAL from `NetImpactUSD =
  ImprovementUSD − ReductionUSD`.

---

## Per-trade record (TPOverrideSample)

| Field | Meaning |
|---|---|
| `OriginalTP` | TP % before override |
| `AdjustedTP` | TP % after override |
| `RealizedPnL` | filled in at close |
| `WasWinner` | net PnL > 0 |
| `ExitReason` | TAKE_PROFIT / STOP_LOSS / MANUAL |

---

## Verification (test: `TestTPOverrideAuditTighteningHurtsWinner`)

A winner whose TP was tightened 0.9% → 0.5% and exited at TP:
```
tightened     = 1
reductionUSD  > 0      (profit given up correctly attributed)
netImpactUSD  < 0      (verdict: HURTING)
```
Test passes — the audit correctly detects when TP overrides shrink winners.

---

## How many winners became smaller winners?

This is a **runtime** quantity. The audit now computes it continuously:
`tpOverride.tightened` (count) and `tpOverride.tpReductionUSD` (dollars given up),
served at `GET /api/execution/intelligence → tpOverride`. Prometheus gauge:
`trading_execintel_tp_override_net_impact_usd`.

**Pre-22D this was unknowable** — overrides were applied silently with no outcome
attribution. Phase 22D makes the helping/hurting verdict measurable from real fills.
