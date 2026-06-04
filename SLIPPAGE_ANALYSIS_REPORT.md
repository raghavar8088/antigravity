# Slippage Analysis Report — Phase 22D

**Date:** 2026-06-04

---

## Slippage Infrastructure

### Existing Components (Pre Phase 22D)
- `engine/internal/sor/slippage_engine.go` — Full EWMA slippage tracker per venue/symbol.
  - `Expected(ctx, venue, symbol, side, qty)` — square-root market-impact model.
  - `RecordRealized(ctx, venue, symbol, side, qty, expectedBps, realizedBps)` — EWMA update.
  - `VenueScore()` — maps EWMA slippage to 0–1 score for SOR routing.

### Gap Before Phase 22D
The `SlippageEngine` was defined and wired into the SOR, but **slippage was never surfaced
at the individual trade level** in `processStrategyGroup` or in fill result logging.

---

## Phase 22D: Entry Slippage Measurement

### What Is Measured
**Entry slippage** = difference between expected fill price (last observed price at
signal time) and actual fill price (ExecPrice from `execution.FillResult`).

```go
slippageBps := math.Abs(execPrice-currentPrice) / currentPrice * 10000
log.Printf("[SLIPPAGE] %s %s entry slippage %.2f bps (expected $%.2f, filled $%.2f)",
    aggSig.StrategyName, sig.Timeframe, slippageBps, currentPrice, execPrice)
```

### Logging Format
Every executed trade now emits:
```
[SLIPPAGE] TripleFilter_Alpha_Scalp 1m entry slippage 2.14 bps (expected $68450.00, filled $68451.47)
```

---

## Slippage by Context

### Paper Trading (Current)
Paper trading uses `currentPrice` as ExecPrice, so slippage = 0 bps by definition.
The measurement infrastructure is live-ready: once live execution is wired, real
fill prices will diverge from signal prices and the log will show true slippage.

### Expected Live Slippage Ranges

| Market | Regime | Expected Slippage |
|--------|--------|-------------------|
| BTC-USD (Coinbase) | Normal | 1–5 bps |
| BTC-USD (Coinbase) | Volatile | 5–20 bps |
| BTC-USD (Binance) | Normal | 0.5–3 bps |
| BTC Futures (Delta) | Normal | 2–8 bps |

### Slippage per Timeframe (Expected)
| Timeframe | Slippage Sensitivity | Notes |
|-----------|---------------------|-------|
| tick | High — 0.5 ms matters | 1 bps slippage can erase 30% of TP |
| 1m | Medium | 2–5 bps acceptable |
| 5m | Low | 5–10 bps acceptable |
| 15m | Very Low | 10–20 bps acceptable |

---

## Slippage per Market Regime (Framework)

The `SlippageEngine` in `sor/slippage_engine.go` already captures regime-adjusted
estimates via its EWMA model. The `ImpactCoefficient` (0.5) and `Alpha` (0.2) can
be tuned per regime:

| Regime | Recommended `ImpactCoefficient` | Rationale |
|--------|-------------------------------|-----------|
| TREND | 0.4 | Deeper liquidity on one side |
| RANGE | 0.5 | Balanced book depth |
| VOLATILE | 0.8 | Wider spreads, thinner book |
| MIXED | 0.6 | Cautious default |

---

## Missed-Entry Loss from Slippage

For a 1m strategy with TP = 0.30%, SL = 0.15%:
- 5 bps slippage = 1.67% of TP distance consumed at entry
- 10 bps slippage = 3.33% of TP consumed
- 20 bps slippage = 6.67% of TP consumed (structural edge at risk)

At 1,800 trades/day, 5 bps average slippage on $1,000 position = $0.50/trade
= **$900/day** in friction on the $1M paper account.

---

## Next Steps
1. Wire `SlippageEngine.RecordRealized` into post-fill path using actual `fill.ExecPrice`.
2. Add `SlippageBps` field to `PositionOpenedPayload` for per-position tracking.
3. Surface per-strategy average slippage in the strategy tracker stats.
