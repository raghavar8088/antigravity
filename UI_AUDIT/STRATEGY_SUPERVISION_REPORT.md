# PHASE 4 — STRATEGY SUPERVISION REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Can a trader supervise 600+ strategies?

---

## WHAT THE BACKEND HAS

- 600+ strategies in `engine/internal/strategy/curated_registry.go`
- Strategy families: EMA Cross, RSI threshold, RSI slope, Bollinger Band, Funding/CVD, Delta absorption, Liquidity sweep, FVG retest, Order block, MSS continuation, Microstructure, Volume profile
- Per-strategy health scoring in `lib/strategyHealthEngine.ts`: HEALTHY / WARNING / CRITICAL / INSUFFICIENT_DATA
- Per-strategy Sharpe, expectancy, win rate, profit factor, max drawdown in `lib/strategyScoringEngine.ts`
- WINNERS_ONLY gate active — strategies auto-disabled on rolling performance
- Regime routing: strategy activation depends on market regime

---

## WHAT THE UI SHOWS

### Strategy Enable/Disabled Status
**Verdict: PARTIAL**

Evidence:
- Paper Desk `HealthSummary`: `{healthy: N, warning: N, critical: N, insufficient_data: N}` aggregated counts only
- `client/src/hooks/usePaperDesk.ts:36-42`
- Strategy health tab in Paper Desk: fetched via `/api/paper-desk/strategy-health` (lazy load)
- Terminal Research Center: 5 hardcoded strategies with health ACTIVE/WATCHLIST/DISABLED
- **Gap**: No UI lists all 600+ strategies with individual enable/disable status. Trader sees aggregate health counts, not which specific strategies are in what state.

### Signals Generated
**Verdict: PARTIAL**

Evidence:
- BTC Futures Dashboard includes `SignalTracePanel.tsx` (22KB) — funnel analysis showing signal generation, confidence pass, scoring gate, risk gate, family conflicts
- `client/src/components/SignalTracePanel.tsx` — shows dominant blocker, % of evaluations at each funnel stage
- Mock Trading: `/api/mock-trading/signals` fetched every 5s, signals displayed in `MockTradingDashboard`
- **Gap**: Signal trace is for the in-browser engine only. The Go engine's signal generation is not visible in any live dashboard.

### Trades Taken
**Verdict: PARTIAL**

Evidence:
- Paper Desk: recent trades visible with strategy name, entry/exit, PnL — live from MongoDB
- BTC Futures Desk: recent trades shown in table
- **Gap**: No per-strategy trade count visible in primary views. Strategy trade history requires navigating to separate analytics

### Win Rate
**Verdict: PARTIAL**

Evidence:
- `client/src/lib/strategyScoringEngine.ts` — calculates win rate per strategy
- Paper trades leaderboard: `/api/paper-trades/leaderboard` provides win rate per strategy
- Mock Trading: strategy leaderboard shows win rate
- **Gap**: Not prominently displayed per-strategy in primary operator view. Requires navigating to leaderboard tab.

### Expectancy
**Verdict: PARTIAL**

Evidence:
- Terminal Research Center snapshot: `expectancy: 32.4` per strategy (hardcoded demo)
- `lib/strategyScoringEngine.ts` computes expectancy
- `/api/paper-trades/strategy-stats` returns per-strategy expectancy
- **Gap**: Live expectancy data from Go engine not surfaced in primary dashboard

### PnL Per Strategy
**Verdict: PARTIAL**

Evidence:
- `AttributionPanel.tsx` (5KB) — PnL by strategy, PnL by family
- `/api/strategy-attribution/[id]` exists
- **Gap**: Attribution panel placement in navigation unclear; not confirmed in primary view

### Drawdown Per Strategy
**Verdict: ABSENT (in primary view)**

Evidence:
- `lib/strategyScoringEngine.ts` computes max drawdown
- `/api/paper-trades/strategy-stats` includes drawdown
- **Gap**: No primary view displays per-strategy drawdown. Only aggregate portfolio drawdown shown.

### Risk Per Strategy
**Verdict: ABSENT**

No UI component renders per-strategy risk contribution in any live dashboard.

### Regime
**Verdict: PARTIAL**

Evidence:
- `DashboardHeader.tsx` renders `RegimeBadge` with: TRENDING_BULL, TRENDING_BEAR, RANGE, HIGH_VOL
- Regime chip shown in header
- **Gap**: No breakdown of which strategies are active in current regime vs blocked. The regime indicator is a single badge, not a strategy-regime mapping.

### Alpha Contribution
**Verdict: ABSENT**

Evidence:
- `lib/futuresAttribution.ts` computes alpha contribution per strategy family
- `/api/strategy-attribution/[id]` exists
- **Gap**: Not surfaced in any primary dashboard that traders regularly see

---

## CRITICAL FINDING: 600 STRATEGIES, ~5 VISIBLE AT A TIME

The Go engine runs 600+ strategies simultaneously. The UI shows:
- A health count badge (e.g., "42 healthy, 3 critical") in Paper Desk
- 5 hardcoded strategies in Terminal Research Center (not live)
- Strategy names in individual trade records
- A leaderboard (requires tab navigation, lazy-loaded)

There is no master strategy supervision panel where a trader can see all active strategies with current status, PnL, health, regime eligibility, and kill state simultaneously.

**For 72-hour autonomous operation this is a material deficiency.** A strategy could enter "CRITICAL" health and a trader might only notice when the aggregate counter changes — with no indication of which strategy, why, or what action to take.

---

## STRATEGY SUPERVISION SCORECARD

| Capability | Status | Evidence |
|-----------|--------|----------|
| See all 600 strategies | ABSENT | No such panel |
| Enable/disable per strategy | ABSENT | No UI control |
| See strategy signals live | PARTIAL | Signal trace (in-browser only) |
| See per-strategy win rate | PARTIAL | Leaderboard tab, lazy-loaded |
| See per-strategy drawdown | ABSENT | No primary view |
| See per-strategy PnL | PARTIAL | Attribution panel (navigation unclear) |
| See strategy health | PARTIAL | Aggregate counts only |
| See regime routing | PARTIAL | Single regime badge, no per-strategy mapping |
| Override strategy | ABSENT | No UI |
| See alpha contribution | ABSENT | Not surfaced |

**Score: 3/10 — Insufficient for institutional strategy supervision**
