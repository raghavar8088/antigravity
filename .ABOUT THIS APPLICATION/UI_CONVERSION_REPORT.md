# UI CONVERSION REPORT
**Phase 9 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**CONVERTED — UI is now a read-only execution consumer.**

---

## UI RESPONSIBILITY MATRIX

| Responsibility | Required State | Evidence |
|----------------|---------------|---------|
| Display positions | READ | `usePositions.ts` polls `/api/positions` → Go engine proxy |
| Display trades | READ | `useTrades.ts` polls `/api/trades` → Go engine proxy |
| Display PnL | READ | `usePaperDesk.ts` polls `/api/paper-desk/snapshot` → MongoDB read |
| Display strategy status | READ | `useStrategies.ts` polls `/api/engine/strategies` → Go engine proxy |
| Display OMS status | READ | `/api/paper-desk/orders` → MongoDB read |
| Display risk status | READ | `useEngineState.ts` polls `/api/engine/state` → Go engine proxy |
| Display kill switch | READ | `/api/admin/killswitch` GET → Go engine |
| Display analytics | READ | `portfolioAccountingService.ts` reads MongoDB aggregations |

---

## PROHIBITED CAPABILITIES (Confirmed Absent)

| Prohibited Action | Previous State | Current State | Proof |
|-------------------|---------------|---------------|-------|
| Generate signals | ACTIVE | DISABLED | `useBTCFuturesScalperEngine.ts` poll() → returns immediately |
| Generate orders | ACTIVE | DISABLED | poll() → returns immediately |
| Calculate authoritative positions | ACTIVE | DISABLED | all client position state is empty (no poll) |
| Calculate authoritative portfolio | ACTIVE | DISABLED | `portfolioAccountingService.ts` is READ from MongoDB only |
| Calculate authoritative PnL | ACTIVE | DISABLED | `persistenceDisabled=true` in mock engine; scalper returns empty |
| Write to MongoDB | ACTIVE | DISABLED | all POST routes return HTTP 410 |

---

## DATA FLOW: UI AS CONSUMER

```
Go Engine (AWS Lightsail)
  ↓ [writes]
MongoDB Atlas (paper_trades, paper_state, positions, equity_curve)
  ↓ [reads via Next.js API routes]
Dashboard (read-only display)
  → positions panel
  → trade history table
  → PnL metrics
  → strategy health
  → equity curve chart
  → kill switch status
```

---

## HOOKS THAT REMAIN VALID (Read-Only)

| Hook | Source | Type |
|------|--------|------|
| `usePositions.ts` | `/api/positions` → Go engine | READ |
| `useTrades.ts` | `/api/trades` → Go engine | READ |
| `useStrategies.ts` | `/api/engine/strategies` | READ |
| `useEngineState.ts` | `/api/engine/state` | READ |
| `usePaperDesk.ts` | `/api/paper-desk/snapshot` → MongoDB | READ |
| `useEngineLogs.ts` | `/api/engine/logs` | READ |
| `useDeltaLive.ts` | `/api/delta-live/stats` | READ |
| `useOptions.ts` | `/api/options/*` | READ |

---

## HOOKS THAT ARE NOW DEAD CODE (No-Op)

| Hook | Reason | Status |
|------|--------|--------|
| `useBTCFuturesScalperEngine.ts` | poll() returns immediately | Dead code — returns empty state |
| `useMockTradingEngine.ts` | disablePolling=true | Dead code — returns empty state |

These hooks remain in the codebase but execute no logic. They can be deprecated and removed in a future cleanup cycle.

---

## CONCLUSION

The UI is a read-only consumer. No component generates signals, calculates positions, or writes to any datastore. All authoritative state flows from the Go engine through MongoDB to the Next.js display layer.
