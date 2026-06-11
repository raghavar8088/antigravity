# INSTITUTIONAL UI GAP REPORT
**Phase 9 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Gap analysis against institutional hedge fund dashboard standards

---

## REFERENCE STANDARD

Comparison against: Bloomberg PORT, Kensho, Two Sigma internal dashboards, Citadel risk portal, and standard multi-strategy fund operator tooling.

---

## GAP 1: RISK-ADJUSTED PERFORMANCE METRICS

### Missing from every screen

| Metric | Formula | Priority |
|--------|---------|---------|
| Sharpe Ratio | (Mean daily return − Risk-free rate) / StdDev(daily returns) | CRITICAL |
| Sortino Ratio | (Mean daily return − Risk-free rate) / StdDev(negative daily returns) | CRITICAL |
| Calmar Ratio | Annualized return / Max drawdown | HIGH |
| Maximum Drawdown | Max(equity[i] − max(equity[0..i])) | CRITICAL |
| Drawdown Duration | Days from peak to recovery | HIGH |
| Expected Shortfall (CVaR) | Average loss in worst 5% of days | HIGH |
| Information Ratio | Alpha / Tracking error | MEDIUM |

**Current state:** None of these are computed or displayed on any screen.
**Impact:** A PM cannot assess risk-adjusted performance without these. The equity curve alone is insufficient.

**Backend has the data:** All closed trades with timestamps and PnL are in MongoDB `paper_trades`. These can be computed server-side from that collection.

---

## GAP 2: STRATEGY LEADERBOARD — MISSING DEPTH METRICS

### Present
- Win rate, total PnL, trade count (per strategy)
- Sorting by PnL/win rate

### Missing

| Metric | Why It Matters |
|--------|---------------|
| Expectancy ($ per trade) | Win rate alone is misleading without avg win/loss ratio |
| Profit Factor (gross win / gross loss) | Standard metric for strategy health |
| Max Drawdown per strategy | A strategy winning 70% but with 40% drawdown is dangerous |
| Sharpe per strategy | Expected return per unit of risk |
| Trade frequency (trades/day) | Low-frequency strategies need more context |
| Regime breakdown (trend/chop/reversal) | Critical — a strategy that only works in trending markets needs to be regime-gated |
| Strategy family correlation | If 8 EMA strategies are all long, they're one trade |
| Consecutive losses | Risk monitoring — 5 consecutive losses signals possible regime change |
| Last trade timestamp | Is this strategy still active? Has it been silent for 3 days? |

**Impact:** The current leaderboard cannot answer the question "which strategies should be running right now, and why?"

---

## GAP 3: CORRELATION MATRIX

**Not present on any screen.**

What's needed:
- N×N strategy correlation matrix (daily PnL correlation between strategy pairs)
- Visual heatmap (red = high positive correlation = concentration risk)
- Portfolio correlation to BTC spot (systematic beta)
- Rolling 30-day vs 7-day correlation to spot drift

**Why critical:** If 400 of 600 strategies are correlated to BTC momentum, a BTC dump creates a single catastrophic loss event, not 400 independent events. Without seeing the matrix, the PM is flying blind on concentration.

**Backend data available:** All closed trades with timestamps. Pairwise correlation computable from daily returns.

---

## GAP 4: SLIPPAGE AND EXECUTION QUALITY DASHBOARD

**Not present on any screen.**

What's needed:

| Panel | Content |
|-------|---------|
| Average slippage at entry (bps) | By strategy family |
| Average slippage at exit (bps) | By strategy family |
| Slippage trend over time | Is model diverging from assumptions? |
| Fill rate (% orders filled at intended price) | Mock broker parity check |
| Implementation shortfall | Realized PnL vs theoretical (0 slippage) PnL |

**Backend has the data:** `ClosedTrade.SlippageBps` exists in Go struct. Not surfaced in any API response or UI.

---

## GAP 5: ALPHA DECAY ANALYSIS

**Not present on any screen.**

What's needed:
- Per-strategy alpha decay curve: plot edge (expected PnL) vs trade age
- Identify strategies where alpha decays within 30 minutes (overfit to short windows)
- Rolling 30-day vs 90-day performance comparison per strategy (detecting regime sensitivity)
- Stability score: strategies that consistently perform across regimes vs those that spike and die

**Why critical:** Paper trading alpha that doesn't survive 60+ days at consistent Sharpe ≥ 1.0 is not real alpha.

---

## GAP 6: REGIME ANALYSIS PANEL

**Not present on any screen.**

What's needed:

| Panel | Content |
|-------|---------|
| Current regime | TREND_UP / TREND_DOWN / CHOP / REVERSAL / HIGH_VOL |
| Regime history chart | Timeline of regime changes |
| Performance by regime | Which strategies perform in which regime |
| Regime-filtered P&L | Realized PnL only in X regime |
| Regime transition prediction | Momentum/volatility indicators |

**Backend has this:** `ClosedTrade.RegimeAtEntry` exists. Regime classifier runs in engine. Not exposed in any dashboard.

---

## GAP 7: TRADE ATTRIBUTION PANEL

**Not present on any screen.**

What's needed:
- P&L attribution: what % of total PnL came from which strategy family?
- P&L attribution: what % came from trend-following vs mean-reversion vs funding?
- P&L attribution by time-of-day (Asia session vs US session)
- P&L attribution by market regime
- "Which 10 trades made up 80% of our profits?" (power law check)

---

## GAP 8: DRAWDOWN MONITOR (REAL-TIME)

**Not present on any screen.**

What's needed:
- Current drawdown from peak (live, not just historical)
- Max allowed drawdown threshold displayed vs current
- Kill switch trigger level shown as a warning line on equity chart
- Time in drawdown: if in drawdown for > N days, display alert

---

## GAP 9: ENGINE HEALTH / OBSERVABILITY PANEL

**Partially present in RiskModule but incomplete.**

What's needed at the top navigation bar (always visible):
- Engine status: LIVE / DEGRADED / OFFLINE (with timestamp of last heartbeat)
- Kill switch status: ARMED / TRIGGERED
- Active strategies count: N/600
- Open positions count: N
- Today's P&L: $X (+X%)
- Today's max drawdown: -X%

A fund operator should never have to navigate to a sub-page to see engine health. It belongs in a persistent status bar.

---

## GAP 10: JOURNAL / TRADE DETAIL VIEW

**Present but shallow.**

What's missing from trade detail view:
- Entry reason: which strategy generated the signal, confidence score
- Risk gate outcome: why was this trade approved? Kelly fraction used?
- Regime at entry vs regime at exit
- Slippage: bps paid at entry and exit
- Fee: $ fee paid
- Implementation shortfall: what would theoretical PnL have been at 0 slippage?
- Attribution: did this trade add alpha or just capture market beta?

---

## PRIORITY RANKING

| Gap | Impact | Effort | Priority |
|-----|--------|--------|---------|
| Sharpe/Sortino/Max Drawdown on paper desk | CRITICAL | Low (compute from trades collection) | P0 |
| Fix BTCFuturesScalper (empty screen) | HIGH | Low (replace hook) | P0 |
| Remove hardcoded balance in useEngineState | HIGH | Low (one-line poll change) | P0 |
| Strategy leaderboard: expectancy + profit factor | HIGH | Low (add MongoDB aggregation) | P1 |
| Real-time drawdown monitor on equity chart | HIGH | Medium | P1 |
| Correlation matrix | CRITICAL | Medium (compute server-side) | P1 |
| Slippage dashboard | HIGH | Low (expose existing field) | P1 |
| Regime analysis panel | HIGH | Medium | P2 |
| Trade attribution | HIGH | Medium | P2 |
| Alpha decay curves | MEDIUM | High | P3 |

---

## VERDICT

The current UI achieves **C+ (58/100)** against institutional standards.

It is capable of monitoring trades but cannot support institutional-level questions:
- "Is our current edge statistically real?"
- "Are our strategies over-correlated?"
- "What is our current risk-adjusted return?"
- "Which strategies are degrading?"

These are answerable from existing engine data. The backend has all the raw data. The gaps are entirely in the presentation layer.
