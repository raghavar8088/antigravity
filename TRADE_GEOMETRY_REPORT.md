# Trade Geometry Report — Phase 22D

**Date:** 2026-06-04  
**Status:** IMPLEMENTED — strategy-defined TP now respected

---

## Problem Statement

`sanitizeSignalForProfit` (loop.go) previously enforced a blanket 2.4:1 R:R override
on **every** signal, regardless of whether the strategy explicitly provided a TP:

```
Strategy sets TP=0.30%, SL=0.15%  → R:R = 2.0
Forced override: TP = 0.15 * 2.40 = 0.36%
Actual entry exit: further from target → higher miss rate
```

This inflates TP for scalping strategies whose edge relies on tight, frequently-hit
targets, converting a high-win-rate strategy into a low-fill rate one.

---

## Root Cause

`minRewardToRiskRatio = 2.40` was applied universally in `sanitizeSignalForProfit`
without checking whether the strategy had explicitly encoded its own geometry.

When `sig.TakeProfitPct` is already set (> 0), the strategy's own expectancy model
determined that target. Overriding it destroys the signal's alpha.

---

## Phase 22D Fix

### Before
```go
minTakeProfitByRR := adjusted.StopLossPct * minRewardToRiskRatio
if adjusted.TakeProfitPct < minTakeProfitByRR {
    adjusted.TakeProfitPct = minTakeProfitByRR  // always overrides
}
```

### After
```go
tpExplicit := adjusted.TakeProfitPct > 0

if !tpExplicit {
    // Strategy did not define TP → apply floor + R:R baseline
    adjusted.TakeProfitPct = minSignalTakeProfitPct
    minTakeProfitByRR := adjusted.StopLossPct * minRewardToRiskRatio
    if adjusted.TakeProfitPct < minTakeProfitByRR {
        adjusted.TakeProfitPct = minTakeProfitByRR
    }
}

// Absolute floor: 0.10% — prevents fee-level micro exits even on explicit TPs
const absoluteTPFloor = 0.10
if adjusted.TakeProfitPct < absoluteTPFloor {
    adjusted.TakeProfitPct = absoluteTPFloor
}
```

### Rules After Fix

| Signal TP | SL | Behaviour |
|-----------|-----|-----------|
| 0 (not set) | any | Floor to 0.50%, then R:R floor (2.4×SL) applied |
| > 0 (explicit) | any | TP preserved exactly. Only 0.10% absolute floor enforced |
| > 0 but < 0.10% | any | TP raised to 0.10% (fee protection) |

---

## Geometry Logging

Every adjusted trade geometry is now logged with the actual R:R ratio achieved:
```
[GEOMETRY] TripleFilter_Alpha_Scalp SL/TP 0.15%/0.30% -> 0.15%/0.30% (R:R 2.00)
```
When no change occurs, no log line is emitted (zero noise on clean signals).

---

## Impact Assessment

### Strategies Affected
- All strategies that set `TakeProfitPct > 0` in their `OnTick` / `OnCandle` output.
- Estimated: ~400 of the 600+ curated strategies set explicit TP.

### Expected Outcome
| Metric | Before | After |
|--------|--------|-------|
| TP hit rate (scalps) | Lower — TP inflated beyond target zone | Higher — strategy's own TP preserved |
| Win rate (explicit TP strategies) | Suppressed by inflation | Restored to strategy-tested level |
| R:R for no-TP strategies | 2.4:1 enforced | 2.4:1 enforced (unchanged) |
| Absolute TP floor | Implicit via minSignalTakeProfitPct | Explicit 0.10% guard |
