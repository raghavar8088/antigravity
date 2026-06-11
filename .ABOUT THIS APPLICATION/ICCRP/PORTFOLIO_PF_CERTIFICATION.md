# PORTFOLIO PF CERTIFICATION

**Status:** PASS  
**Audited:** 2026-06-11

## Finding (Before)

| Location | Issue |
|----------|-------|
| `strategy-intelligence/route.ts:123` | `profit_factor: null` |
| `StrategyIntelligenceDashboard.tsx:164` | `fmt(null)` → `"0.00"` via `Number.isFinite` |

## Real PF Source

| Component | Location |
|-----------|----------|
| Formula | `mockExtendedMetrics.ts:162` — `grossWins / grossLosses` |
| Aggregation | `portfolioAccountingService.ts:228-241` — `getPortfolioExtendedMetrics()` |
| Persistence | Mongo `paper_trades` closed records |
| API | `strategy-intelligence/route.ts:119-125` |

## Remediation

```typescript
// strategy-intelligence/route.ts
profit_factor: extendedMetrics.profit_factor,  // real or null
sharpe: extendedMetrics.sharpe,
```

```typescript
// StrategyIntelligenceDashboard.tsx
function fmt(n: number | null | undefined) {
  if (n === null || n === undefined || !Number.isFinite(n)) return "—";
  ...
}
```

## Reachability Proof

```
Mongo paper_trades
  → getPortfolioExtendedMetrics()
  → GET /api/strategy-intelligence
  → portfolio_stats.profit_factor
  → StrategyIntelligenceDashboard ribbon tile
```

## UI Proof

- Portfolio PF tile: real value or `—`
- Never `null → 0.00`
- Per-strategy PF still from `strategy_scores.profit_factor` (Mongo authoritative)
