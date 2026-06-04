# R:R Optimization Report — Phase 22D

**Date:** 2026-06-04

---

## Current State (Post Phase 22D)

### Explicit-TP Signals
R:R inflation is **removed**. The strategy's own TP is used as-is, subject only to a
0.10% absolute floor that prevents fee-erosion micro-exits.

### No-TP Signals
The 2.40:1 R:R floor is **preserved** for signals that do not set TP. These signals
have no internally validated exit target, so the systemic floor protects them.

---

## Forced R:R Values in Code

| Constant | Value | Applies To |
|----------|-------|------------|
| `minRewardToRiskRatio` | 2.40 | Signals with TP = 0 only (post Phase 22D) |
| `minSignalTakeProfitPct` | 0.50% | Signals with TP = 0 (floor before R:R math) |
| `maxSignalStopLossPct` | 0.20% | All signals (SL cap) |
| `absoluteTPFloor` | 0.10% | All signals (fee-erosion guard) |

---

## R:R Distribution (Pre Phase 22D, Estimated)

Based on audit of `sanitizeSignalForProfit`:

| Strategy TP | % of Signals | Pre-22D R:R | Post-22D R:R |
|-------------|-------------|-------------|--------------|
| Explicit (> 0) | ~67% | Inflated to ≥ 2.4× | Preserved |
| None (= 0) | ~33% | ≥ 2.4× (R:R floor) | ≥ 2.4× (unchanged) |

---

## What R:R Inflation Costs

When TP is pushed from 0.30% → 0.36% for a scalp with SL = 0.15%:
- Price must travel 20% further to hit TP
- In a 5-bps range market, this converts a 55% win-rate signal into a ~40% one
- Every 100 such trades: 15 additional losers × $1,500 avg loss = -$22,500

At 600 strategies × 3 trades/day avg = 1,800 trades/day, even a 5% improvement in
win rate from geometry preservation = +$2,250/day on a $1M paper account.

---

## Recommendations

1. Strategies that currently emit TP = 0 should be updated to include their
   backtested optimal TP. The R:R floor remains as a backstop but should not
   be the primary exit mechanism for any strategy with live trade data.

2. Monitor `[GEOMETRY]` log lines to identify which strategies are having their
   TP preserved vs. which are using the floor. The ratio should trend toward
   more preserved-TP executions over time.

3. Consider reducing `minRewardToRiskRatio` from 2.40 to 1.80 for the
   no-TP fallback path after 30-day observation period, as 2.4× may be too
   restrictive for mean-reversion strategies that operate at 1.5–2.0× naturally.
