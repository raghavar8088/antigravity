# STRATEGY MONITORING REPORT
**Phase 10 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code verification + MongoDB collection structure analysis

---

## VERDICT: PER-STRATEGY MONITORING IS SHALLOW — AGGREGATE ONLY

The engine runs 600+ strategies. The UI can show aggregate PnL and a leaderboard. It cannot answer "what is strategy X doing right now, why, and is it healthy?"

---

## WHAT EXISTS

### Strategy Leaderboard (`StrategyLeaderboard.tsx`)

**Data source:** MongoDB `strategy_health` collection + `paper_trades` aggregation
**API:** `/api/paper-desk/strategy-health` + `/api/paper-trades/strategy-stats`

**Per-strategy fields available:**

| Field | Source | Present in UI? |
|-------|--------|----------------|
| Strategy ID | paper_trades | YES |
| Strategy Name | registry | YES |
| Win rate (%) | Aggregated from paper_trades | YES |
| Total realized PnL | Aggregated from paper_trades | YES |
| Trade count | Aggregated from paper_trades | YES |
| Avg PnL per trade | Aggregated | YES |
| Last trade timestamp | paper_trades | YES |
| Strategy family | registry | PARTIAL (some views) |

### Strategy Registry API (`/api/engine/strategies`)

**Source:** Go engine in-memory registry
**Returns:** Array of `{id, family, description, parameters, isActive}` per strategy

**Per-strategy fields from registry:**

| Field | Present |
|-------|---------|
| Strategy ID | YES |
| Strategy family | YES |
| Is active (enabled) | YES |
| Parameters (window lengths, thresholds) | YES |
| Description | YES |

---

## WHAT IS MISSING

### Per-Strategy Runtime Metrics (Not Exposed)

| Metric | Backend Has It? | API? | UI? |
|--------|----------------|------|-----|
| Sharpe ratio per strategy | Computable from paper_trades | NO | NO |
| Expectancy ($ per trade) | Computable from paper_trades | NO | NO |
| Profit factor (gross_win/gross_loss) | Computable from paper_trades | NO | NO |
| Max drawdown per strategy | Computable from paper_trades | NO | NO |
| Win rate by regime | Computable from paper_trades + RegimeAtEntry | NO | NO |
| Signal frequency (signals/hour) | Not persisted (ephemeral) | NO | NO |
| Last signal timestamp | Not persisted | NO | NO |
| Current open positions count | Computable from paper_positions | NO | NO |
| Trade frequency (trades/day) | Computable | NO | NO |
| Consecutive losses (current streak) | Computable from last N trades | NO | NO |

### Per-Strategy Health Monitoring (Not Implemented)

| Check | What It Would Detect |
|-------|---------------------|
| Performance degradation alert | Trailing 7-day Sharpe drops below threshold |
| Unusual silence alert | Strategy hasn't traded in > 24h during active session |
| Concentration alert | Strategy has > 3 open positions simultaneously |
| Correlation alert | Strategy is +0.9 correlated with another running strategy |
| Regime mismatch alert | Trend-following strategy active during CHOP regime |

None of these monitoring capabilities exist.

---

## REGIME PERFORMANCE (CRITICAL GAP)

`ClosedTrade.RegimeAtEntry` field exists in Go structs and is stored in `paper_trades`. No aggregation or display of regime-conditional performance has been implemented.

This means:
- A strategy might show 65% win rate overall
- But in TREND regime: 82% win rate
- In CHOP regime: 31% win rate
- Net: should ONLY be active during TREND
- Current system: runs in both regimes, dragging performance

**The engine has regime classification. The paper_trades records have regime context. The UI ignores it entirely.**

---

## STRATEGY FAMILY MONITORING

**Current state:**
- 600+ strategies across 12 families (EMA Cross, RSI threshold, RSI slope, Bollinger, Funding/CVD, Delta absorption, Liquidity sweep, FVG retest, Order block, MSS, Microstructure, Volume profile)
- Leaderboard shows individual strategies with family label
- No family-level aggregation view

**What's needed:**
- Family-level performance dashboard (family → total PnL, Sharpe, win rate, trade count)
- Cross-family correlation (how correlated is EMA Cross family to RSI family?)
- Family heat map (quick visual: which family is performing this week?)

---

## WINNERS_ONLY GATE VISIBILITY

**Backend:** WINNERS_ONLY gate is active since May 2026. Losing strategies are removed from the registry.
**UI:** No indication that strategies are being removed. No historical view of removed strategies.
**Gap:** PM cannot see which strategies were removed or why. Audit trail is unavailable from UI.

---

## ACTIVE STRATEGY COUNT DISPLAY

The engine reports `activeStrategyCount` on the `/api/health` endpoint. This is displayed in the engine status section but not in the strategy monitoring area.

The displayed count (600+) does not distinguish between:
- Strategies that have traded in the last 24h (genuinely active)
- Strategies that haven't generated a signal in days (regime-inactive)
- Strategies that are active in registry but have no historical trades

**Recommendation:** Categorize active strategies as ACTIVE_TRADING (traded in last 24h), ACTIVE_SILENT (in registry but no recent signals), REMOVED (WINNERS_ONLY gate).

---

## RECOMMENDATIONS

1. **Add per-strategy aggregation API** — extend `/api/paper-trades/strategy-stats` to include: expectancy, profit factor, max drawdown, consecutive losses, regime breakdown
2. **Add regime performance tab** to Strategy Leaderboard — filter by `RegimeAtEntry` from `paper_trades`
3. **Add family performance rollup** — aggregate leaderboard data by strategy family
4. **Strategy health alerts** — threshold-based alerts when trailing Sharpe drops or silence exceeds threshold
5. **WINNERS_ONLY audit log** — expose which strategies were removed and when
