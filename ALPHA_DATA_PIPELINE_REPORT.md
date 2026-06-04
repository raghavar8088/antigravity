# ALPHA_DATA_PIPELINE_REPORT — Phase 22C
Generated: 2026-06-04

## Purpose
End-to-end audit of every data pipeline feeding institutional alpha modules: price/tick, CVD, funding, liquidations, order book, open interest. Documents what flows, what is broken, and what is missing.

---

## Pipeline 1: Price / Tick Data

**Source**: Coinbase WebSocket → `marketdata.CoinbaseClient`  
**Path**: `CoinbaseClient.GetTickChannel()` → `Orchestrator.Run()` → `processTickPipeline()`

| Stage | Status | Notes |
|---|---|---|
| Coinbase WS connect | ✅ Active | main.go:399 `coinbaseClient.Connect(ctx, ["BTC-USD"])` |
| Tick channel | ✅ Active | `ticks := o.client.GetTickChannel()` loop.go:497 |
| Candle aggregation | ✅ Active | `o.candleAgg.Feed(t)` loop.go:559 |
| 1m candle emit | ✅ Active | `candleAgg.Candles1m` channel → `process1mCandles()` |
| 5m candle emit | ✅ Active | `candleAgg.Candles5m` channel → `process5mCandles()` |
| Alpha OnTick (tick) | ✅ Fixed | CVD, Delta, Phase11CVD receive raw ticks |
| Alpha OnTick (1m) | ✅ Fixed | FVG, MSS, OrderBlock, etc. receive 1m candle ticks |

**Data quality**: Tick contains `Price`, `Quantity`, `Side`, `TimeMs`. All fields used by alpha engines.

---

## Pipeline 2: CVD (Cumulative Volume Delta)

**Source**: Raw ticks (Coinbase WS)  
**Engine**: `engine/internal/alpha/cvd/`

| Stage | Status | Notes |
|---|---|---|
| Tick ingestion | ✅ Active | `s.cvdEngine.AddTick(row)` — called on EVERY OnTick |
| CVD accumulation | ✅ Active | `cvd.Engine` maintains running delta in cache |
| Cache | ✅ Active | `cvd.Cache` 2000-tick ring buffer |
| Divergence detection | ✅ Active | `cvd_divergence.go` DetectDivergence() |
| Strategy evaluation | ✅ Fixed | CVDDivergence_Alpha and Phase11CVDDivergence_Alpha both active |

**Gap**: `Side` field in Coinbase ticks may not always be populated (BTC-USD public feed). If `Side == ""`, the CVD engine treats the tick as passive. This may reduce divergence signal frequency but does not break the pipeline.

---

## Pipeline 3: Delta Absorption

**Source**: Raw ticks (price + CVD delta)  
**Engine**: `engine/internal/alpha/delta/`

| Stage | Status | Notes |
|---|---|---|
| Delta feed | ✅ Active | `s.deltaEngine.Add(t.Price, td.Delta)` — every OnTick |
| Accumulation/distribution | ✅ Active | Detects price-down+delta-up (accumulation), price-up+delta-down (absorption) |
| Strategy evaluation | ✅ Fixed | DeltaAbsorption_Alpha active |

---

## Pipeline 4: Funding Rate

**Source**: Local NDJSON file + optional live collector  
**Engine**: `engine/internal/alpha/funding/`

| Stage | Status | Notes |
|---|---|---|
| Cache file load | ✅ Active | `funding.NewCache("data/alpha/funding.ndjson")` alpha_strategies.go:56 |
| Live collector | ⚠️ Partial | `funding.NewEngine(fundingCache, nil)` — nil collector means no live fetch |
| Historical data | ✅ If present | Cache reads NDJSON if file exists at engine startup |
| Strategy evaluation | ✅ Fixed | FundingMeanReversion_Alpha and Phase11FundingMeanReversion_Alpha active |

**Gap**: The funding collector is `nil` (second arg to `NewEngine`). The funding engine can still evaluate using cached historical data, but won't fetch live rates from Binance/Delta. If `data/alpha/funding.ndjson` is absent, the funding confluence path falls back to CVD/MSS voting without funding component. **No crash, just reduced confluence precision.**

---

## Pipeline 5: Order Book

**Source**: Not currently ingested  
**Engine**: `engine/internal/alpha/microstructure/` (Phase11)

| Stage | Status | Notes |
|---|---|---|
| AddOrderBook() method | ✅ Exists | Phase11MicrostructureAlpha.AddOrderBook() alpha_strategies.go:360 |
| Caller | ❌ ABSENT | No code in trading loop or main.go calls AddOrderBook on any strategy |
| BidAskImbalance feature | ⚠️ Zero | Always 0.0 in FeatureSnapshot without OB feed |
| LiquidityWalls feature | ⚠️ Partial | Inferred from price data, not OB depth |

**Impact**: Phase11 strategies' `BidAskImbalance` field is always 0. `FundingPressureScore` and `LiquidityConfirmation` are computed from price/candle data only, not depth. Phase11 signals can still fire via the other 4 dimensions (CVD, liquidity zones, market structure, volatility regime). **This is a known remaining blocker documented in Phase 22C.**

---

## Pipeline 6: Liquidations

**Source**: Not currently ingested  
**Engine**: `engine/internal/alpha/liquidations/` and `engine/internal/alpha/microstructure/`

| Stage | Status | Notes |
|---|---|---|
| AddLiquidation() method | ✅ Exists | Phase11MicrostructureAlpha.AddLiquidation() alpha_strategies.go:368 |
| Caller | ❌ ABSENT | No code calls AddLiquidation in production |
| LiquidationCascade strategy | ⚠️ Zero data | `liquidations_engine.go` Signal() requires liquidation events to be fed |
| LiquidationSpike feature | ⚠️ Zero | Always false without liquidation feed |

**Impact**: `Phase11LiquidationCascadeReversal_Alpha` and `LiquidationCascade` standalone strategy will not fire unless liquidation data is injected. The liquidations engine has the complete implementation — just needs a data source wired in. **Remaining blocker for Phase 22D.**

---

## Pipeline 7: Open Interest

**Source**: Not currently ingested  
**Engine**: `engine/internal/alpha/microstructure/` (field `OpenInterestDelta`)

| Stage | Status | Notes |
|---|---|---|
| OI tracking | ⚠️ Zero | `OpenInterestDelta` defaults to 0 in FeatureSnapshot |
| Funding feed | ⚠️ Partial | `AddFunding(FundingSnapshot{FundingRate, OpenInterest})` exists |
| Caller | ❌ ABSENT | No code calls AddFunding on Phase11 strategies |

**Impact**: OI delta is 0 in all Phase11 features. Funding pressure score uses only rate without OI confirmation. **Remaining blocker for Phase 22D.**

---

## Pipeline 8: Warmup Data

**Source**: Coinbase REST API  
**Engine**: `engine/cmd/antigravity/main.go:617`

| Stage | Status | Notes |
|---|---|---|
| Historical 1m candles | ✅ Active | `marketdata.FetchWarmupCandles("BTC-USD")` — up to 3 retries |
| Strategy buffer pre-fill | ✅ Active | `orchestrator.WarmupStrategies(warmupData)` loop.go:452 |
| Alpha strategy warmup | ✅ Fixed | Alpha strategies now in groups.M1 and groups.Tick, receive warmup ticks |

---

## Summary

| Pipeline | Status | Priority |
|---|---|---|
| Price/Tick (Coinbase WS) | ✅ Flowing | — |
| CVD accumulation | ✅ Flowing | — |
| Delta absorption | ✅ Flowing | — |
| Funding (cache-based) | ✅ Partial | Low — works with cached data |
| Order Book depth | ❌ Missing | Phase 22D |
| Liquidations | ❌ Missing | Phase 22D |
| Open Interest | ❌ Missing | Phase 22D |
| Warmup candles | ✅ Flowing | — |
