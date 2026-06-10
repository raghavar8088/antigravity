# PHASE 4 — ALPHA ENGINE FORENSICS

**Date:** 2026-06-10  
**Scope:** All 10 institutional + 7 Phase 11 = 17 alpha engines  
**Standard:** Operational status verified from source code and data files.

---

## Alpha Engine Architecture

All institutional alpha engines share one Go struct: `InstitutionalAlphaScalper` in `alpha_strategies.go`.

```go
type InstitutionalAlphaScalper struct {
    baseScalper
    module alphaModule    // determines which alpha is evaluated
    candles []alpha.Candle
    cvdEngine    *cvd.Engine
    deltaEngine  *alphadelta.Engine
    liquidityEngine *liquidity.Engine
    fvgStrategy  *fvg.Strategy
    orderBlocks  *orderblock.Engine
    mssEngine    *mss.Engine
    sessionEngine *session.Engine
    profileStrategy *volumeprofile.Strategy
    fundingCache *funding.Cache
    fundingEngine *funding.Engine
    liquidationEngine *liquidations.Engine
}
```

All engines are initialized in `newInstitutionalAlphaScalper()`. Internal packages are wired. Data feeds and dispatch are the failure points.

---

## Dispatch Architecture (Critical)

From `alpha_strategies.go:OnTick()`:

```go
// Tick-rate modules: CVD, DELTA, CONFLUENCE
if s.module == alphaCVD || s.module == alphaDelta || s.module == alphaConfluence {
    return s.evaluate(t.Symbol)
}
// Candle-based modules: delegates to OnCandle
return s.OnCandle(t)
```

**The tick-based modules (CVD, Delta, Funding/Confluence) evaluate on every tick — they ARE being called.**

**The candle-based modules (FVG, MSS, OB, Liquidity, POC, Session, Liquidation) delegate to `OnCandle()` which calls `s.evaluate()` — they also ARE being called through this path.**

**Updated Finding:** The previous "dispatch bug" diagnosis needs revision. The code routes candle-based alphas through `OnCandle()` which does call `evaluate()`. However, the `evaluate()` function itself uses a quality gate:

```go
score := s.qualityFor(sig)
if !quality.MandatoryPass(score.Score) {
    return holdSignal()
}
```

The engines are being called but **signals may be suppressed by the quality gate** rather than the dispatch. This is a different (and potentially more nuanced) failure mode. CVD scores ~71 (gate = 70) — barely passing. Other modules' quality scores are unknown.

---

## Alpha Engine #1: Funding Rate Mean Reversion

| Property | Status |
|:---------|:------:|
| Data source | `data/alpha/funding.ndjson` |
| Data present | **❌ FILE EMPTY** |
| Engine initialized | ✅ `funding.NewCache()` |
| Strategy type | `alphaConfluence` — tick-rate evaluation |
| Quality gate | Applied |
| Live signals | **0** |
| Live trades | **0** |
| Live PnL | **$0** |
| Synthetic PF | 2.09 (invalid) |

**Root cause:** `funding.NewCache("data/alpha/funding.ndjson")` reads an empty file. `fundingEngine` has no data to compute rates. Signals cannot generate.

**Fix:** Populate `data/alpha/funding.ndjson` with real 8-hour funding rate snapshots from Binance futures API. The file path is hardcoded — a background goroutine appending NDJSON records is all that is needed.

**Expected alpha after fix:** Funding mean reversion is a legitimate institutional signal. When funding rate is extreme (>+0.03% or <-0.03% per 8h), the crowded side is being squeezed. This is one of the clearest edge signals in perpetual futures markets.

---

## Alpha Engine #2: CVD Divergence

| Property | Status |
|:---------|:------:|
| Data source | Tick-level data (real-time) |
| Data present | ✅ Processing ticks |
| Engine | `cvd.NewEngine()` + `cvd.NewCache(2000)` |
| Strategy type | `alphaCVD` — tick-rate evaluation |
| Quality score | ~71 (gate = 70) — BARELY PASSES |
| Live signals | Unknown (marginal) |
| Live trades | Unknown |
| Live PnL | $0 (no positive PnL documented) |
| Synthetic PF | 0.91 (fails even synthetically) |

**Assessment:** CVD is firing (tick-rate, passes quality gate marginally). Synthetic PF 0.91 means even on synthetic data this engine underperforms. The quality score barely clearing 70 limits signal frequency. **May not have edge — requires validation post other fixes.**

---

## Alpha Engine #3: Delta Absorption

| Property | Status |
|:---------|:------:|
| Data source | Tick-level delta accumulation |
| Data present | ✅ `deltaEngine.Add()` on every tick |
| Strategy type | `alphaDelta` — tick-rate evaluation |
| Quality gate | Applied |
| Live PnL | $0 |
| Synthetic PF | **0.91** — fails synthetically |

**Assessment:** Tick-rate, data present. Synthetic PF below 1.0. **Lowest priority alpha to fix — may not have edge.**

---

## Alpha Engine #4: Liquidity Sweep Reversal

| Property | Status |
|:---------|:------:|
| Data source | Price candles via `liquidityEngine.Add()` |
| Data present | ✅ Candle data feeds through `OnCandle()` |
| Strategy type | `alphaLiquidity` — candle-based |
| Quality gate | Applied |
| Live PnL | $0 |
| Synthetic PF | 1.02 |

**Updated status:** Not blocked by dispatch. Quality gate may be suppressing signals. Signal count unknown.

---

## Alpha Engine #5: Fair Value Gap (FVG)

| Property | Status |
|:---------|:------:|
| Data source | Price candles via `fvgStrategy.Evaluate()` |
| Data present | ✅ 20+ candles accumulate before evaluation |
| Strategy type | `alphaFVG` — candle-based |
| Quality gate | Applied |
| Live PnL | $0 |
| Synthetic PF | **1.48** |

**Updated status:** FVG evaluation requires `len(s.candles) >= 20`. After warmup, FVG evaluates on every candle. The failure mode is quality gate + insufficient signal frequency, not dispatch. FVG signals require specific 3-candle gap patterns — may be rare on 1m BTC.

---

## Alpha Engine #6: Order Block Retest

| Property | Status |
|:---------|:------:|
| Data source | Price candles via `orderBlocks.Evaluate()` |
| Data present | ✅ Candle-based |
| Strategy type | `alphaOrderBlock` — candle-based |
| Quality gate | Applied |
| Live PnL | $0 |
| Synthetic PF | **1.79** |

**Assessment:** Same status as FVG. Order block identification requires specific consolidation patterns. May produce infrequent signals on 1m BTC. Quality gate may be the binding constraint.

---

## Alpha Engine #7: Market Structure Shift (MSS/MSSCHOCH)

| Property | Status |
|:---------|:------:|
| Data source | Price candles via `mssEngine.Evaluate()` |
| Data present | ✅ Candle-based |
| Strategy type | `alphaMSS` — candle-based |
| Quality gate | Applied |
| Live PnL | $0 |
| Synthetic PF | **2.92** (highest of all strategies) |

**Assessment:** Highest synthetic PF (2.92). Quality gate may be the barrier if MSS score < 70. Alternatively, MSS on 1m BTC requires sufficient structure — may need 5m or 15m candles for meaningful structure breaks. This is worth investigating in detail.

---

## Alpha Engine #8: POC Bounce (Volume Profile)

| Property | Status |
|:---------|:------:|
| Data source | Price candles via `profileStrategy.Evaluate()` |
| Data present | ✅ Candle-based |
| Quality gate | Applied |
| Live PnL | $0 |
| Synthetic PF | 1.19 |

**Assessment:** Volume profile POC requires many candles to build a meaningful profile. On 1m BTC with 500-bar buffer, the POC may not be stable enough to generate reliable signals. Needs investigation.

---

## Alpha Engine #9: Session Expansion

| Property | Status |
|:---------|:------:|
| Data source | Price candles + time via `sessionEngine.Analyze()` |
| Data present | ✅ Time-aware |
| Signal condition | `snap.Expansion && snap.Bias != ActionHold` |
| SL/TP | 0.35% / 0.75% |
| Live PnL | $0 |

**Assessment:** Session expansion requires specific time windows (London/NY open). If the engine is running in a timezone where session windows don't align, it may never trigger. UTC alignment should be verified.

---

## Alpha Engine #10: Liquidation Cascade

| Property | Status |
|:---------|:------:|
| Data source | Tick-level notional proxy (≥$50k single tick) |
| Data present | ✅ In `OnTick()` via tick heuristic |
| Strategy type | `alphaLiquidation` — tick-rate |
| Feed status | Heuristic active in OnTick; real feed unwired |
| Live PnL | $0 |

**Heuristic detail:** `if s.module == alphaLiquidation && t.Quantity > 0 && notional >= 50000`

The liquidation heuristic IS running in OnTick. Large ticks are being detected. However, whether this heuristic fires often enough and with sufficient accuracy to generate profitable signals is unknown. Real liquidation feed from Binance WebSocket (`!forceOrder@arr`) would be more reliable.

---

## Alpha Engine Summary (Revised)

| Engine | Data | Fires? | Quality Gate | Live PnL | Root Cause | Priority |
|:-------|:----:|:------:|:------------:|:--------:|:-----------|:---------|
| Funding Mean Rev | ❌ Empty | ❌ | N/A | $0 | Missing data feed | **CRITICAL** |
| CVD Divergence | ✅ | ⚠️ Partial | Barely (71/70) | $0 | Low signal quality | MEDIUM |
| Delta Absorption | ✅ | ⚠️ Partial | Unknown | $0 | Low synth PF (0.91) | LOW |
| Liquidity Sweep | ✅ | ⚠️ Unknown | Unknown | $0 | Quality gate / signal rarity | MEDIUM |
| FVG Retest | ✅ | ⚠️ Unknown | Unknown | $0 | Pattern rarity on 1m | HIGH |
| Order Block | ✅ | ⚠️ Unknown | Unknown | $0 | Pattern rarity on 1m | HIGH |
| MSS Continuation | ✅ | ⚠️ Unknown | Unknown | $0 | Quality gate / 1m structure | **CRITICAL** |
| POC Bounce | ✅ | ⚠️ Unknown | Unknown | $0 | Profile stability on 1m | MEDIUM |
| Session Expansion | ✅ | ⚠️ Unknown | Unknown | $0 | Timezone/window alignment | MEDIUM |
| Liquidation Cascade | ✅ (heuristic) | ⚠️ Partial | Unknown | $0 | Heuristic may miss real events | MEDIUM |

**Revised finding:** The dispatch is not the primary failure. The failure mode is a combination of:
1. Quality gate suppressing signals that score marginally (CVD at 71/70)
2. Signal rarity — structural alpha patterns (FVG, OB, MSS) are rare on 1m BTC
3. Missing data (Funding feed empty — only clearly dead engine)

**Recommendation:** Add signal count logging to each alpha module. Run 7 days and count how many times each engine generates a raw signal before quality gate filtering. This will reveal whether the problem is quality gate or signal rarity.

---

## Alpha Signal Count Logging (Required)

Add to `evaluate()` in `alpha_strategies.go`:

```go
// Before quality gate:
if sig.Action != "" && sig.Action != alpha.ActionHold {
    log.Printf("[ALPHA-SIGNAL] %s generated raw signal: %s confidence=%.2f",
        s.baseScalper.name, sig.Action, sig.Confidence)
}

// After quality gate:
score := s.qualityFor(sig)
if !quality.MandatoryPass(score.Score) {
    log.Printf("[ALPHA-GATE] %s BLOCKED: score=%.2f threshold=70",
        s.baseScalper.name, score.Score)
    return holdSignal()
}
log.Printf("[ALPHA-PASS] %s PASSED gate: score=%.2f",
    s.baseScalper.name, score.Score)
```

This will reveal within 24 hours whether the quality gate or signal rarity is the binding constraint.

---

## Phase 4 Verdict

**All 10 institutional alpha engines: $0 live PnL.**

**Revised root cause:** Not primarily a dispatch bug. The architecture routes all modules correctly through evaluate(). The failure modes are:
1. **Funding feed empty** (only 100% dead engine — clear fix)
2. **Quality gate suppressing low-confidence signals** (CVD barely passes at 71/70)
3. **Signal rarity on 1m BTC** — structural patterns (MSS, FVG, OB) may require longer timeframes
4. **Unknown quality scores** — need logging to determine which engines are scoring above/below 70

**Immediate actions:**
1. Populate funding.ndjson
2. Add signal count + quality score logging
3. Consider running MSS/FVG/OB on 5m candles instead of 1m (richer structure)
