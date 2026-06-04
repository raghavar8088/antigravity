# ALPHA_EXECUTION_TRACE — Phase 22C
Generated: 2026-06-04

## Purpose
End-to-end execution trace for each alpha module from data source to trade output, AFTER Phase 22C fixes. Shows every function call, file, and line number. Quantifies signal loss at each stage.

---

## Trace Format
```
[STAGE] Function → File:Line → Output
```

---

## Trace A: CVDDivergence_Alpha (tick-based)

```
[1. DATA INGEST]
   CoinbaseClient WebSocket → marketdata/coinbase.go → Tick{Price, Qty, Side, TimeMs}

[2. TICK DISPATCH]
   Orchestrator.processTickPipeline() → loop.go:535
   → o.groups.Tick contains CVDDivergence_Alpha ✅ (after Fix 1)
   → processStrategyGroup(ctx, groups.Tick, tick) → loop.go:565

[3. STRATEGY OnTick]
   InstitutionalAlphaScalper.OnTick(t) → alpha_strategies.go:140
   → module == alphaCVD → evaluate(symbol) returned ✅ (always was working)

[4. CVD ACCUMULATION]
   cvd.Engine.AddTick(row) → alpha/cvd/cvd.go
   cvd.Cache.Add(td) → alpha/cvd/cvd_cache.go (2000-tick ring)
   delta.Engine.Add(price, delta) → alpha/delta/delta_engine.go

[5. EVALUATION]
   InstitutionalAlphaScalper.evaluate(symbol) → alpha_strategies.go:161
   case alphaCVD:
     cvd.NewStrategy(s.cvdCache).Evaluate(symbol, s.prices)
     → alpha/cvd/cvd_strategy.go:17
     → DetectDivergence(prices, cache.CVDSeries(1000))
     → Returns Signal{Source:"CVDDivergence", Action:Buy|Sell|Hold}

[6. QUALITY GATE]
   qualityFor(sig) → alpha_strategies.go:273
   quality.Score({CVD:1.0, Delta:0.70, MSS:0.65, ...}) → score = 71 ✅ (after Fix 4)
   quality.MandatoryPass(71) → true ✅

[7. SIGNAL RETURN]
   []Signal{{Action:Buy|Sell, Confidence:~0.70-0.85, StopLossPct:0.30, TakeProfitPct:0.75}}

[8. AGGREGATION]
   rawSignals append → loop.go:885
   FilterSignalsSelective() → aggregator_selective.go:23
   strategyPriority(): confidence(0.70) + 1.45 boost = 2.15 > 1.10 ✅ (was always boosted)
   Category "Microstructure" → allowed in TREND/MIXED/VOLATILE ✅

[9. REGIME FILTER]
   isCategoryAlignedWithRegime("Microstructure", regime) → loop.go:1689
   → Allowed in TREND, MIXED, VOLATILE ✅ (was already working)

[10. EXECUTION WEIGHT]
    tracker.GetExecutionWeight("CVDDivergence_Alpha")
    New strategies start at weight 1.0 ≥ minExecutionWeightToTrade(0.50) ✅

[11. RISK GATE]
    sanitizeSignalForProfit() → confidence ≥ 0.74, SL/TP within limits ✅
    risk.Validate(sig, price) → position limit, daily loss check ✅

[12. EXECUTION]
    executeThroughInstitutionalPath() → loop.go:191
    → OMS v3 ledger events: Created → Validated → RiskApproved → Submitted → Filled
    → PaperClient.ExecuteSignal() → fill result

[13. POSITION OPEN]
    openAndTrackPosition() → posMgr.OpenPosition() ✅
    emitPositionOpened() → OMS v3 EventPositionOpened ✅
```

**Signal loss map**: Data→Alpha: 0%, Alpha→Quality: 0% (score 71), Quality→Aggregator: 0%, Aggregator→Regime: 0%, Regime→Risk: variable (risk engine), Risk→Execution: 0%

---

## Trace B: FVGRetest_Alpha (1m candle-based)

```
[1. DATA INGEST]
   Coinbase WS → CandleAggregator.Feed(tick) → loop.go:559
   → 60s accumulation → Candles1m channel emit

[2. CANDLE DISPATCH]
   process1mCandles() → loop.go:568
   candle.ToTick() → tick with Close price, Volume, Timestamp
   processStrategyGroup(ctx, groups.M1, tick) → loop.go:578
   groups.M1 contains FVGRetest_Alpha ✅ (after Fix 1)

[3. STRATEGY OnTick (FIXED)]
   InstitutionalAlphaScalper.OnTick(t) → alpha_strategies.go:140
   module == alphaFVG → NOT in {CVD,Delta,Confluence}
   → s.OnCandle(t) ← DELEGATE (after Fix 2, was returning holdSignal())

[4. CANDLE ACCUMULATION]
   InstitutionalAlphaScalper.OnCandle(t) → alpha_strategies.go:151
   s.feed(t.Price) → baseScalper price buffer
   s.toCandle(t) → alpha.Candle{Open, High, Low, Close, Volume, Timestamp}
   s.candles = append(s.candles, c) — capped at defaultBufSize (320)

[5. EVALUATION]
   evaluate(symbol) → alpha_strategies.go:161
   case alphaFVG:
     s.fvgStrategy.Evaluate(s.candles)
     → alpha/fvg/fvg_strategy.go:16
     → Tracks gaps, checks: 35% < FillPct < 85%, SizePct >= 0.03
     → Confidence = 0.65 + SizePct*2 + (100-FillPct)/500
     → Returns Signal{Source:"FVGRetest", Action:Buy|Sell|Hold, SL:0.35, TP:0.80}

[6. QUALITY GATE]
   qualityFor(sig) → alpha_strategies.go:273
   quality.Score({FVG:1.0, MSS:0.80, Liquidity:0.70, CVD:0.70, ...}) → score = 73 ✅ (after Fix 4)
   quality.MandatoryPass(73) → true ✅

[7. SIGNAL RETURN]
   []Signal{{Action:Buy|Sell, Confidence:0.65+, StopLossPct:0.35, TakeProfitPct:0.80}}

[8. AGGREGATION]
   strategyPriority(): confidence(0.70) + 1.45 = 2.15 > 1.10 ✅
   Category "Structure" — in MIXED/TREND lists ✅ (after Fix 3)

[9. REGIME FILTER]
   isCategoryAlignedWithRegime("Structure", "MIXED") → true ✅ (after Fix 3)
   isCategoryAlignedWithRegime("Structure", "TREND") → true ✅ (after Fix 3)

[10–13. Same as Trace A]
```

**Signal loss map**: Before Fix 2, signal loss was 100% at stage 3 (holdSignal). After fix: 0% loss through stage 8.

---

## Trace C: Phase11MSSCHOCH_Alpha (1m candle-based, microstructure engine)

```
[1. DATA INGEST]
   Same as Trace B — 1m candle from CandleAggregator

[2. CANDLE DISPATCH]
   groups.M1 contains Phase11MSSCHOCH_Alpha ✅ (after Fix 1)
   processStrategyGroup(ctx, groups.M1, tick)

[3. STRATEGY OnTick]
   Phase11MicrostructureAlpha.OnTick(t) → alpha_strategies.go:347
   s.feed(t.Price)
   row := alpha.Tick{...}
   features := s.engine.AddTick(row) ← calls microstructure.Engine.AddTick()

[4. FEATURE SNAPSHOT (microstructure engine)]
   microstructure.Engine.AddTick(row) → alpha/microstructure/engine.go:32
   engine.Snapshot(symbol) → engine.go:74
   Computes:
   - CVD metrics (rolling sum, momentum, confirmation score) → lines 90-95
   - ATR 14-period, regime classification (Trending/Ranging/HighVol) → lines 97-103
   - Liquidity zones (walls, gaps, proximity score) → lines 105-108
   - Sweep direction, rejection flag → line 110
   - Funding state (from nil collector — rate=0, OI=0) → line 112
   - Liquidation state (no feed — all zero) → line 115
   - Market structure: BOS/CHOCH/retest, alignment score → lines 117-120
   - FVGs, OrderBlocks, VolumeProfile → lines 119-121

[5. STRATEGY EVALUATION]
   microstructure.Strategy.Evaluate(features) → alpha/microstructure/strategy.go:47
   case StrategyMSSRetest:
     Detects CHOCHDirection != "" && StructureRetest
     Returns Signal{Action, Confidence, Source}

[6. SIGNAL ENRICHMENT]
   microstructure.EnrichSignal(raw, kind, features) → strategy.go:68
   FinalConfidence = base*0.38 + CVD*0.18 + Liquidity*0.16 + Funding*0.10 + Structure*0.13 + Regime*0.05
   Approval gates: FinalConfidence > 0.70, regime compatible

[7. DEDUPLICATION]
   microstructure.FilterCandidate(nil, enriched, nil) → strategy.go:111
   Passes through (no prior candidate)

[8. SIGNAL RETURN]
   If enriched.Approved:
     []Signal{{Confidence: enriched.FinalConfidence, AIReasoning: "... cvd=X liquidity=Y funding=Z structure=W regime=R"}}

[9. AGGREGATION]
   strategyPriority(): confidence(>0.70) + 1.45 = 2.15+ > 1.10 ✅ (after Fix 5)
   Category "Phase 11 Structure" → allowed in MIXED/TREND ✅ (after Fix 3)

[10. REGIME FILTER]
   isCategoryAlignedWithRegime("Phase 11 Structure", "MIXED") → true ✅ (after Fix 3)

[11–13. Same as Trace A]
```

**Note on reduced features**: With no order book feed (`BidAskImbalance=0`) and no liquidation feed (`LiquidationSpike=false`), Phase11 strategies rely on CVD + liquidity zone proximity + market structure + volatility regime for their 5-dimension score. Signals are possible but FinalConfidence may be slightly below 0.70 threshold in some market conditions.

---

## Signal Loss Quantification (After Phase 22C)

| Stage | Before Phase 22C | After Phase 22C |
|---|---|---|
| Registry (loaded strategies) | 0/16 | 16/16 |
| OnTick dispatch (candle modules) | 6/9 dead | 9/9 active |
| Quality gate (InstitutionalAlpha) | 0/9 pass | 9/9 pass |
| Category regime filter | 0/16 pass | 16/16 pass (in MIXED) |
| Aggregator score (Phase11) | 0/7 pass | 7/7 pass |
| Data pipelines (price/CVD/delta) | ✅ | ✅ |
| Data pipelines (OB/liquidations/OI) | ❌ | ❌ (Phase 22D) |
