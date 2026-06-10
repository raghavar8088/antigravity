# PHASE 16 — ALPHA RECONSTRUCTION PLAN

**Date:** 2026-06-10

---

## Alpha Reconstruction Scope

This phase outlines specific repair plans for each broken or underperforming alpha engine. The target is to move each engine from $0 PnL (broken or untested) to validated, deployable signal generators.

**Engines requiring reconstruction:**
1. Funding Rate Mean Reversion — data feed broken
2. CVD Divergence — quality gate too close to threshold, weak synthetic PF
3. Delta Absorption — weak synthetic PF (0.91)
4. Liquidity Sweep Reversal — unknown quality score
5. Fair Value Gap (FVG) — quality gate + 1m timeframe mismatch
6. Order Block Retest — quality gate + 1m timeframe mismatch
7. Market Structure Shift (MSS) — highest synthetic PF (2.92), quality gate blocking
8. POC Bounce — volume profile stability on 1m insufficient
9. Session Expansion — timezone alignment uncertain
10. Liquidation Cascade — heuristic insufficient vs real feed

---

## Engine #1: Funding Rate Mean Reversion

**Status:** DEAD (funding.ndjson empty)  
**Synthetic PF:** 2.09 (second highest — valid pattern)  
**Repair complexity:** LOW  

### Root Cause
`data/alpha/funding.ndjson` is the hardcoded data source. The file exists but contains zero records. The funding engine has nothing to read.

### Reconstruction Plan

**Step 1: Create funding data poller**
```go
// cmd/funding_poller/main.go
// Calls Binance API /fapi/v1/premiumIndex every 30 minutes
// Appends to data/alpha/funding.ndjson in NDJSON format:
// {"timestamp":1750000000,"symbol":"BTCUSDT","rate":0.000123}
```

**Step 2: Backfill 30 days of historical funding data**
- Binance API: `/fapi/v1/fundingRate?symbol=BTCUSDT&limit=1000`
- Returns up to 1,000 historical 8-hour funding periods
- Load ~120 records to seed the cache

**Step 3: Verify engine activates**
- Add log line: `log.Printf("[FUNDING] rate=%.6f, signal=%s", rate, sig.Action)`
- Run for 24 hours, verify signals appear

**Expected outcome:** 2-5 signals per week during extreme funding conditions. Expected PF: 1.5-2.5 based on synthetic data.

**Estimated effort:** 4 hours of development + 24 hours of monitoring.

---

## Engine #2: CVD Divergence

**Status:** Running but weak (quality score ~71/70 threshold, synthetic PF 0.91)  
**Repair complexity:** MEDIUM  

### Root Cause Analysis
CVD is firing (tick-rate) but the quality score barely exceeds the gate (71/70). Synthetic PF of 0.91 means the backtest of this signal loses money.

### Issue: CVD Signal on 1m BTC
CVD (Cumulative Volume Delta) measures net buying vs selling pressure. The divergence signal fires when price makes a new low but CVD does not (bullish divergence) or price new high but CVD doesn't follow (bearish).

On 1m BTC, CVD divergence is common because:
- Volume is noisy on 1-minute bars
- Many large orders cross multiple 1m candles
- True divergence patterns need 5-15m bars to be meaningful

### Reconstruction Plan

**Option A: Move CVD to 5m candles**
- Accumulate 5× 1m CVD bars before evaluating divergence
- Requires a 5m CVD buffer in `cvd.Engine`
- Signal frequency drops to ~1-3/hour but quality improves

**Option B: Strengthen divergence criteria**
- Require consecutive 3+ bars of divergence (not just 1 bar)
- Add volume threshold: only fire when bar volume > 1.5× 20-bar average
- This raises quality score above 75 naturally

**Option C: Pair CVD with price structure**
- Require CVD divergence to coincide with a support/resistance level
- Reduces false divergences in open air

**Recommendation:** Option B (no architectural change, only parameter adjustment).

---

## Engine #3: Delta Absorption

**Status:** Running, synthetic PF 0.91 (below 1.0)  
**Repair complexity:** HIGH (may not have edge)  

### Assessment
Delta absorption measures order flow imbalance at specific price levels. A synthetic PF of 0.91 means even on historical data the strategy loses. This is the most concerning alpha engine — the edge may simply not exist at the signal construction level.

**Reconstruction decision:** DOWNGRADE to research/monitoring status. Do not allocate capital. Re-evaluate after CVD and FVG/OB fixes reveal whether order flow data quality is sufficient for alpha.

---

## Engine #4: Liquidity Sweep Reversal

**Status:** Running (candle-based), unknown quality score  
**Synthetic PF:** 1.02 (marginally positive)  
**Repair complexity:** MEDIUM  

### Reconstruction Plan

**Liquidity sweep definition:** Price moves below a recent swing low (hunting stops), then reverses sharply back above.

**Current issue:** The 1m timeframe has many mini-sweeps that are noise (price ticks below a level, immediately recovers). Real liquidity sweeps are more visible on 5m/15m.

**Plan:**
1. Add `MinSweepDepth` parameter: require price to move at least 0.15% below swing low before reversal qualifies
2. Require reversal candle to close above the sweep level (confirmation)
3. Add volume filter: sweep candle volume must exceed 2× 20-bar average

**Expected improvement:** Reduce false sweeps, improve signal quality from ~1.02 PF to 1.30+.

---

## Engine #5: Fair Value Gap (FVG)

**Status:** Running but 0 live trades, synthetic PF 1.48  
**Repair complexity:** MEDIUM-HIGH  

### Root Cause
FVG requires 3-candle gap pattern: candle N body + candle N+1 gap + candle N+2 no-fill. On 1m BTC, true FVGs (unfilled price areas) are visible but often quickly filled within minutes (BTC liquidity is deep).

**More productive approach:** FVG on 15m or 1h charts creates structural gaps that persist for hours and attract price back to fill.

### Reconstruction Plan

**Phase 1 (short-term):** Keep FVG on 1m but add minimum gap size requirement:
- Gap width must be ≥ 0.30% (not micro-gaps from 1-tick misses)
- Reduces signal frequency but improves quality

**Phase 2 (medium-term):** Move FVG to 5m candle aggregation:
- Accumulate 5 × 1m candles, feed to 5m FVG engine
- FVG on 5m represents ~25-minute structural areas worth trading

**Phase 3 (long-term):** Multi-timeframe FVG: only trade 1m entry if there's a 15m FVG confluence.

---

## Engine #6: Order Block Retest

**Status:** Running, 0 live trades, synthetic PF 1.79  
**Repair complexity:** MEDIUM  

### Root Cause
Order blocks (institutional accumulation/distribution zones) are more meaningful on 5m-1h timeframes where institutional orders are placed. On 1m, many "order blocks" are noise.

### Reconstruction Plan

**Minimum order block quality criteria:**
1. Block candle must have volume > 2.5× 20-bar average (high participation)
2. Block must be > 0.25% of price range
3. Retest must be within 0.15% of block midpoint
4. Retest candle must show rejection (close significantly above/below block)

**Expected improvement:** Reduces from many low-quality OB signals to fewer high-conviction signals. Synthetic PF 1.79 suggests the mechanism works — the issue is signal precision, not direction.

---

## Engine #7: Market Structure Shift (MSS)

**Status:** Running, 0 live trades, HIGHEST synthetic PF = 2.92  
**Repair complexity:** HIGH (architectural change needed)  
**Priority: CRITICAL**  

### Root Cause
MSS identifies when a market breaks a structural level (higher high in uptrend breaks → potential trend change). This is one of the most reliable institutional signals on higher timeframes.

**On 1m BTC:** Market structure shifts happen every few minutes as micro-highs and lows are constantly broken. True structural shifts (the ones that precede meaningful moves) require at minimum 15m structure.

### Reconstruction Plan

**This is the highest-priority alpha fix because synthetic PF = 2.92 is extraordinary.**

**Phase 1: 5m MSS (2-3 days)**
- Feed 5m candles to MSS engine (accumulate 5× 1m)
- 5m structure shifts represent ~30-minute trend changes — meaningful
- Expected signal frequency: 3-8 per day (vs dozens on 1m)

**Phase 2: Structural level quality gate (2-3 days)**
- Structural high/low must have held for ≥ 3 bars to be a "real" structure level
- Break must be confirmed by 2 consecutive closes beyond the level
- Volume at break must be above 20-bar average

**Phase 3: Multi-timeframe confirmation (1 week)**
- MSS on 5m confirmed by 1h trend direction
- Entry on 1m after 5m MSS is confirmed

**Expected outcome after Phase 1:** MSS generates 3-8 high-quality signals/day with PF >1.80 (conservative degradation from synthetic 2.92).

---

## Engine #8: POC Bounce (Volume Profile)

**Status:** Running, 0 live trades, synthetic PF 1.19  
**Repair complexity:** MEDIUM  

### Reconstruction Plan
POC (Point of Control) is the price with highest traded volume. On 1m with a 500-bar buffer, the POC represents the most traded price over the last ~8 hours.

**Issue:** 500 1m bars = 8.3 hours of history. The POC can change as new volume enters, making it unstable for short-term trading.

**Fix:** Use longer lookback for POC calculation (1,000+ bars = ~16 hours, or entire session). The VWAP-POC is more stable than short-window POC.

---

## Engine #9: Session Expansion

**Status:** Running (time-based), 0 live trades  
**Repair complexity:** LOW  

### Reconstruction Plan
Session expansion fires at London/NY opens when price typically breaks from overnight range.

**Verification needed:**
```go
// Verify UTC offset in session_engine.go
// London open: 08:00 UTC
// NY open: 13:30 UTC
// Expected bias: breakout in direction of prior session's close
```

If the engine is using local time rather than UTC, the windows are wrong. Add explicit UTC conversion.

---

## Engine #10: Liquidation Cascade

**Status:** Running via heuristic (tick notional ≥ $50k), 0 live trades  
**Repair complexity:** HIGH (requires real feed)  
**Heuristic adequacy:** LOW  

### Reconstruction Plan

**Current heuristic:** `if notional >= 50000` on any single tick. This is a proxy for a large order, not a liquidation.

**Real liquidation data:** Binance provides a WebSocket endpoint for forced liquidations:
```
wss://fstream.binance.com/ws/!forceOrder@arr
```

This sends real-time liquidation events with symbol, side, price, quantity.

**Implementation:**
1. Add Binance liquidation WebSocket listener in bridge layer
2. Feed liquidation events to `liquidationEngine.Add()`
3. Strategy fires when large liquidations create momentum

**Alternative (lower effort):** Use funding rate extremes as a proxy for liquidation risk (crowded positions approaching liquidation). This pairs naturally with the Funding Rate engine (Engine #1).

---

## Reconstruction Priority Matrix

| Engine | Expected PF (post-fix) | Effort (days) | ROI Score |
|:-------|:---------------------:|:-------------:|:---------:|
| Funding Rate Mean Rev | 1.5-2.5 | **1** | **HIGH** |
| MSS Continuation | 1.8-2.5 | **3** | **CRITICAL** |
| FVG Retest (5m) | 1.3-1.7 | **2** | HIGH |
| Order Block (5m) | 1.4-1.9 | **2** | HIGH |
| Session Expansion | 1.2-1.6 | **0.5** | MEDIUM |
| Liquidity Sweep | 1.2-1.5 | **2** | MEDIUM |
| CVD (strengthened criteria) | 1.2-1.4 | **1** | MEDIUM |
| POC Bounce (longer window) | 1.1-1.3 | **1** | LOW-MEDIUM |
| Liquidation Cascade (real feed) | 1.3-1.8 | **5** | MEDIUM |
| Delta Absorption | 0.9-1.1 | **3** | LOW (may not have edge) |

**Recommended execution order:**
1. **Day 1:** Funding Rate (backfill + poller — 4 hours)
2. **Days 2-4:** MSS 5m reconstruction (highest potential)
3. **Days 5-6:** FVG + OB 5m candle feeds
4. **Day 7:** Session UTC verification
5. **Days 8-9:** CVD criteria strengthening
6. **Future sprint:** Liquidation real feed, Delta research

---

## Phase 16 Verdict

**The alpha engine portfolio can be reconstructed in 8-10 days of focused development.**

The most critical reconstruction is **MSS on 5m candles** — synthetic PF of 2.92 is extraordinary, and if even half of that survives real trading, MSS becomes the strongest signal in the portfolio.

The quickest fix is **Funding Rate** — the data file is empty and needs a simple poller. This engine may contribute 2-5 high-quality signals per week.

**FVG and OB on 5m** represent a realistic 10-15% improvement to portfolio Sharpe once their pattern quality improves with longer bars.

The institutional alpha engines, when working, would represent the 4 most independent signals in the portfolio — genuinely different from EMA/RSI/statistical families and providing true diversification.
