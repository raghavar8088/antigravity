# OBJECTIVE 4 — STRATEGY INTELLIGENCE COMMAND CENTER

## Dashboard

**Route:** `/terminal/strategies`  
**Component:** `StrategyIntelligenceDashboard.tsx`  
**API:** `/api/strategy-intelligence`

## Views

| View | Query |
|------|-------|
| Top 20 | `view=top20` |
| Top 50 | `view=top50` |
| Bottom 20 | `view=bottom20` |
| Retirement | `view=retirement` |

## Columns Displayed

strategy_id, status, enabled, total_pnl, expectancy, profit_factor, win_rate, max_drawdown, sample_size, evidence_score, allocation_tier

## Research Center

`/terminal/research` now uses live `snapshot.strategies` from REST + side panel from strategy-intelligence API (no fake tournament bars).
