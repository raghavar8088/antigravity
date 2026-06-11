# STRATEGY INTELLIGENCE CERTIFICATION

**Status:** PASS  
**Surface:** `/terminal/strategies` → `StrategyIntelligenceDashboard.tsx`

---

## Data Authority

| Field | Source | File:Line |
|-------|--------|-----------|
| Expectancy | Mongo `strategy_scores.expectancy` | `strategy-intelligence/route.ts:80` |
| PF (per strategy) | Mongo `strategy_scores.profit_factor` | route.ts:81 |
| PF (portfolio) | `getPortfolioExtendedMetrics()` | route.ts:125 |
| Win rate | Mongo aggregated | route.ts:82 / closedStats |
| Trade count | `strategy_scores.sample_size` | route.ts:84 |
| Max drawdown | `strategy_scores.max_drawdown` | route.ts:83 |
| Status | `strategy_health.health_status` | route.ts:57-58 |
| Evidence score | Computed from real PF/WR/exp/sample | route.ts:68-73 |

---

## Operator Tasks Without Logs

| Task | Mechanism | Evidence |
|------|-----------|----------|
| Identify winners | Sort by expectancy; tier A/B | view=top20/50, route.ts:97-101 |
| Identify losers | Sort bottom20; negative expectancy | route.ts:102-104 |
| Retirement candidates | view=retirement filter CRITICAL/tier F | route.ts:106-109 |
| Health summary | summary ribbon HEALTHY/WARNING/CRITICAL | dashboard lines 156-174 |

---

## Guard + Error States

- Wrapped in `TerminalAuthorityGuard` via layout
- API failure → `BACKEND AUTHORITY UNAVAILABLE` (dashboard line 222-225)
- Empty data → `NO DATA AVAILABLE` (line 228-231)
- Portfolio PF null → `—` not 0.00 (`fmt()` lines 74-77)

---

## Reachability

```
GET /api/strategy-intelligence
  → listStrategyScores + listStrategyHealth + getPortfolioExtendedMetrics
  → StrategyIntelligenceDashboard load() [line 97]
  → Table + summary ribbon
```

**Certification:** Operator can rank, filter, and retire strategies without reading logs.
