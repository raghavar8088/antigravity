# STRATEGY CENTER BLUEPRINT — ICCRP V3

**Route:** `/terminal/strategies`  
**Component:** `client/src/components/StrategyIntelligenceDashboard.tsx`  
**API:** `/api/strategy-intelligence`

---

## Display Fields (Implemented)

| Field | API Key | File Reference |
|-------|---------|----------------|
| Strategy | `strategy_id` | `StrategyIntelligenceDashboard.tsx` L8-9 |
| Status | `status` (HEALTHY/WARNING/CRITICAL) | L10, L60-65 |
| PnL | `total_pnl` | L13 |
| Profit Factor | `profit_factor` | L14 |
| Expectancy | `expectancy` | L13 |
| Drawdown | `max_drawdown` | L16 |
| Trade Count | `sample_size` | L17 |
| Win Rate | `win_rate` | L15 |
| Evidence Score | `evidence_score` | L21 |
| Allocation Tier | `allocation_tier` A-F | L22, L67-73 |

**Not yet in UI:** Sharpe per strategy (Mongo has no Sharpe field — `mapSnapshotToTerminalDelta.ts` L114 sets `sharpe: null`), Regime, Last Signal, Last Trade.

---

## Filters (Implemented)

| Filter | View Key | Lines |
|--------|----------|-------|
| Top 20 | `top20` | L50-58 |
| Top 50 | `top50` | same |
| Bottom 20 | `bottom20` | same |
| Retirement Candidates | `retirement` | same |
| All / Survivors | `all` | same |

API: `/api/strategy-intelligence?view={view}&limit=100`

---

## Gaps

| Gap | Priority | Recommendation |
|-----|----------|----------------|
| Wrap in `TerminalCard` design system | P2 | Refactor styles to institutional tokens |
| Last signal / last trade columns | P2 | Join engine events API |
| Regime column | P3 | Add from SEP pipeline |

---

## Status: OPERATIONAL (minor UI polish remaining)
