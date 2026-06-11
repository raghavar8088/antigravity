# STRATEGY AUTHORITY REPORT
**Phase 6 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**FAIL — Strategies execute in both Go engine AND browser. Dual strategy authority.**

---

## STRATEGY EXECUTION LOCATIONS

### Location S1 — Go Strategy Registry (AUTHORITATIVE)

| Property | Value |
|----------|-------|
| File | `engine/internal/strategy/curated_registry.go` |
| Count | 600+ strategies |
| Families | EMA Cross, RSI, Bollinger Band, Funding/CVD, Liquidity Sweep, FVG, MSS, Microstructure, Volume Profile |
| Interface | `Strategy.OnTick(tick)` / `Strategy.OnCandle(candle)` |
| Execution | Called by orchestrator loop every tick/candle |
| Risk gate | ALL signals pass through PMS + pre-trade pipeline + kill switch |
| Authority | PRIMARY |

### Location S2 — Client-Side Strategy Registry (UNAUTHORIZED DUPLICATE)

| Property | Value |
|----------|-------|
| File | `client/src/lib/futuresStrategies.ts` |
| Purpose | Browser-side strategy definitions for scalper engine |
| Used by | `useBTCFuturesScalperEngine.ts` (E17), paper desk worker (E19) |
| Risk gate | Client-side confidence threshold only (does NOT use Go risk gates) |
| Authority | NONE — unauthorized duplicate |

### Location S3 — Mock Trading Signal Pipeline (UNAUTHORIZED)

| Property | Value |
|----------|-------|
| File | `client/src/lib/mockTradingEngine.ts` → `executeMockSignalThroughFunnel()` |
| Purpose | Simulates strategy execution from signal traces |
| Risk gate | Mock confidence + RR gates (not Go engine gates) |
| Authority | NONE — simulation only |

---

## WHERE STRATEGIES EXECUTE

| System | Strategy Source | Execution Gate | Status |
|--------|----------------|----------------|--------|
| Go engine (S1) | `curated_registry.go` | Full institutional gate | AUTHORIZED |
| Browser scalper (S2) | `futuresStrategies.ts` | Client-side threshold only | UNAUTHORIZED |
| Paper desk worker (S2) | `futuresStrategies.ts` | Client-side threshold only | UNAUTHORIZED |
| Mock engine (S3) | Signal trace replay | Mock gates only | UNAUTHORIZED |

---

## SIGNAL GENERATION IN BROWSER

### `client/src/lib/futuresSignals.ts`
- Generates BUY/SELL signals from kline data
- Called by `useBTCFuturesScalperEngine.ts` on every 4s poll
- Produces signals that directly open positions in browser state
- **Does NOT pass through Go engine risk gates**

### `client/src/lib/futuresDeskPolicy.ts`
- Resolves desk parameters from env vars (thresholds, caps, fees)
- Applied to browser-side signal evaluation
- Separate from Go engine strategy parameters

### `client/src/lib/strategySignalTrace.ts`
- Creates signal trace rows for every browser-generated signal
- Trace consumed by mock trading engine → additional unauthorized trades

---

## BROWSER INDICATORS

The browser scalper engine fetches raw klines and calculates:
- EMA (multiple periods)
- RSI
- Bollinger Bands
- Volume/CVD metrics
- Funding rates

All computed in-browser on every 4-second poll. This is full strategy execution in the browser, not just display.

---

## REQUIRED ACTION

S2 and S3 must be removed. The Go engine (S1) is the sole strategy execution authority. The client must only consume strategy status/results via the engine API — it must never generate signals, evaluate indicators, or execute strategy logic.
