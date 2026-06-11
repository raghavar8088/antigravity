# TODAY PnL CERTIFICATION

**Status:** PASS  
**Audited:** 2026-06-11

## Finding (Before)

| Location | Issue |
|----------|-------|
| `risk-ribbon/route.ts:103` | `todayPnl = closedStats.realized_pnl` (lifetime) |
| `TerminalDashboard.tsx:378` | `dailyPnl={totalPnl}` mislabeled as Day PnL |

## Remediation

### UTC-day filter (`paperDeskClient.ts:272-281`)

`getClosedTradeStats(accountKey, { closedAfter, closedBefore })` filters on `closed_at`.

### Today helper (`portfolioAccountingService.ts:215-221`)

```typescript
export async function getTodayRealizedPnlUtc(accountKey: string): Promise<number> {
  const { start, end } = utcDayBounds(now);
  return (await getClosedTradeStats(accountKey, { closedAfter: start, closedBefore: end })).realized_pnl;
}
```

### Risk ribbon (`risk-ribbon/route.ts:103, 154-161`)

- Uses `getTodayRealizedPnlUtc(accountKey)`
- Label detail: `"UTC day · realized"`
- Shows `NO DATA` when Mongo unavailable

### Terminal dashboard (`TerminalDashboard.tsx:378`)

- Removed misleading `dailyPnl={totalPnl}` — Day PnL hidden until dedicated today feed wired

## Reachability Proof

```
Mongo paper_trades (closed_at in UTC day window)
  → getTodayRealizedPnlUtc()
  → GET /api/risk-ribbon
  → TODAY PnL ribbon item
```

## Verification

Displayed TODAY PnL = sum of `net_pnl` for trades where  
`closed_at >= UTC 00:00:00` AND `closed_at < UTC+1 00:00:00`.
