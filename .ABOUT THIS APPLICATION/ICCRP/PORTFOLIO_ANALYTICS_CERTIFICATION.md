# PORTFOLIO ANALYTICS CERTIFICATION

**Status:** PASS  
**Surface:** `/terminal/portfolio` → `PortfolioAnalyticsDashboard.tsx`

---

## Authority Path

```
GET /api/paper-desk/portfolio
  → getPortfolioAccountingSnapshot(accountKey) [portfolio/route.ts:17]
  → portfolioAccountingService.ts:163-208
  → Mongo: paper_state, paper_trades, paper_positions, equity_curve
  → PortfolioAnalyticsDashboard setSnapshot() [line 106]
```

---

## Dual-Fetch Analysis

| Question | Answer | Evidence |
|----------|--------|----------|
| Mismatch risk? | **Low** | Same `getPortfolioAccountingSnapshot()` as snapshot API |
| Stale state risk? | **Low** | 10s poll; shows `computed_at` timestamp (line 166) |
| Dual-authority risk? | **None** | Single Mongo accounting service |

Terminal store and portfolio page may differ by seconds — not by formula.

---

## Metrics Validated

| Display | Backend field | Null handling |
|---------|---------------|---------------|
| Balance | snapshot.balance | always numeric |
| Equity | snapshot.equity | always numeric |
| Realized PnL | snapshot.realized_pnl | always numeric |
| PF | snapshot.profit_factor | `safeNum()` → `—` |
| Sharpe | snapshot.sharpe | `safeNum()` → `—` |
| Drawdown | snapshot.drawdown.* | pct format |

---

## Failure Mode

```typescript
// PortfolioAnalyticsDashboard.tsx:137-144
if (error || !snapshot) → "BACKEND AUTHORITY UNAVAILABLE"
```

No synthetic portfolio metrics on failure.

**Certification:** Portfolio dashboard trustworthy; dual-fetch is transport-only, not authority conflict.
