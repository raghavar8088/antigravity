# PHASE 1 — STRATEGY INVENTORY

**Generated:** 2026-06-10  
**Verdict:** PASS (enumeration complete) | FAIL (expected edge unproven for 95%+ of strategies)

---

## Summary Counts

| Stack | Defined | Production-Active | Asset | Timeframes |
|:------|--------:|:-----------------:|:------|:-----------|
| Go `BuildCuratedScalpers()` | **606** | 606 wired, ≤25 execute/batch | BTC-USD | tick, 1m, 5m, 15m |
| Client `FUTURES_STRAT_DEFS` | **108** | **48** default (worker excludes research) | BTCUSD futures | 1m primary |
| Go legacy `BuildAllScalpers()` | ~108+ | Not live | BTC-USD | varies |
| Mock research registry | 500 | Never executes | synthetic | — |
| BTC research registry | 100 | UI only | BTC | — |

**Total audited definitions:** 714 (606 + 108). **Total production-reachable:** 654 (606 Go + 48 client default).

---

## Go Engine — 606 Strategies

**Registry:** `engine/internal/strategy/curated_registry.go`  
**Test proof:** `curated_registry_test.go` — `len(BuildCuratedScalpers()) == 606`, unique names enforced.

### Composition

| Section | Count | Source File | Entry Pattern | Default SL/TP |
|:--------|------:|:------------|:--------------|:--------------|
| Base scalpers (post-removal) | 24 | `scalpers.go`, `scalpers_elite*.go` | EMA/RSI/BB/VWAP cross | 0.15% / 0.25% |
| Elite V2 EMA cross | 15 | `elite_v2.go` | EMA cross + ADX + RSI band | Per-instance, R:R ≥ 1.5 |
| Elite V2 RSI threshold | 8 | `elite_v2.go` | RSI band entry | 0.17–0.19% / 0.40–0.44% |
| Elite V2 RSI slope | 5 | `elite_v2.go` | RSI slope reversal | Per-instance |
| Elite V2 Bollinger | 12 | `elite_v2.go` | BB touch/break | Per-instance |
| Elite V2 VWAP | 10 | `elite_v2.go` | VWAP deviation | Per-instance |
| Elite V2 MACD | 10 | `elite_v2.go` | MACD signal cross | Per-instance |
| Elite V2 Volume+Price | 8 | `elite_v2.go` | Volume impulse | Per-instance |
| Elite V2 N-bar breakout | 10 | `elite_v2.go` | Donchian-style break | Per-instance |
| Elite V2 Triple EMA | 8 | `elite_v2.go` | 3-EMA alignment | Per-instance |
| Elite V2 CCI | 8 | `elite_v2.go` | CCI threshold | Per-instance |
| Elite V3 Stochastic | 12 | `elite_v3.go` | Stoch cross | Per-instance |
| Elite V3 ATR | 10 | `elite_v3.go` | ATR expansion | Per-instance |
| Elite V3 ROC | 8 | `elite_v3.go` | ROC momentum | Per-instance |
| Elite V3 Williams %R | 8 | `elite_v3.go` | %R extreme | Per-instance |
| Elite V3 PSAR+EMA | 8 | `elite_v3.go` | PSAR flip + EMA | Per-instance |
| Elite V3 Hull MA | 8 | `elite_v3.go` | Hull cross | Per-instance |
| Elite V3 Keltner | 12 | `elite_v3.go` | Keltner break | Per-instance |
| Elite V3 Momentum divergence | 6 | `elite_v3.go` | Price/osc divergence | Per-instance |
| Elite V3 Consecutive candles | 8 | `elite_v3.go` | N-candle streak | Per-instance |
| Elite V3 Additional | 25 | `elite_v3.go` | Mixed | Per-instance |
| Intraday 5m/15m | 65 | `intraday_strategies.go` | `ID_*` EMA/RSI/ADX | 0.22% / 0.55% typical |
| Institutional alpha | 10 | `alpha_strategies.go` | Funding, CVD, Delta, FVG, OB, MSS, POC, Session, Liq | 0.30–0.35% / 0.75–0.85% |
| Phase 11 alpha | 7 | `alpha_strategies.go` | Microstructure multi-feature | 0.30–0.35% / 0.75–0.85% |
| Expansion pack `XP_*` | 301 | `curated_expansion_pack.go` | Parameter-grid clones | Loop-generated |

### Permanently Removed (11) — Live Loss Evidence

| Strategy | Documented Loss | Evidence |
|:---------|----------------:|:---------|
| ATR_Volume_Impulse_Scalp | -$19.65 | `curated_registry.go:19` (worst) |
| ATR_Breakout | -$15.43 | `curated_registry.go:18` |
| KAMA_Adaptive | -$14.36 | `curated_registry.go:35` |
| MACD_VWAP_Flip | -$10.90 | `curated_registry.go:15` |
| PriceChannel_Breakout | -$11.29 | `curated_registry.go:22` |
| Donchian_Breakout | -$7.84 | `curated_registry.go:17` |
| ADX_Trend_Scalp | -$7.86 | `curated_registry.go:10` |
| VolumeBreakout_Impulse | -$5.34 | `curated_registry.go:24` |
| Pullback_Continuation_Pro | -$4.27 | `curated_registry.go:12` |
| MACD_ZeroCross_Confluence | -$3.71 | `curated_registry.go:42` |
| VolumeDelta_Spike | -$3.44 | `curated_registry.go:41` |

**Total documented removals:** -$108.81

### Representative Strategy Records

#### `EMA_Cross_Scalp` (Go)
| Field | Value |
|:------|:------|
| Location | `engine/internal/strategy/scalpers.go` |
| Asset | BTC-USD |
| Timeframe | 1m |
| Indicators | EMA(fast), EMA(slow) |
| Entry | Fast EMA crosses above/below slow |
| Exit | Position manager SL/TP (not in-strategy) |
| Stop Loss | 0.15% default via `baseScalper` |
| Take Profit | 0.25% default |
| Position Sizing | `defaultQty` 0.10 BTC → Kelly in institutional path |
| Expected Edge | **UNPROVEN** — live +$4.51 per `aggregator_selective.go:189` |

#### `TripleFilter_Alpha_Scalp` (Go)
| Field | Value |
|:------|:------|
| Location | `engine/internal/strategy/scalpers_elite2.go` |
| Asset | BTC-USD |
| Timeframe | 1m |
| Indicators | EMA(20), MACD histogram, ADX |
| Entry | Price > EMA20 + MACD hist > 0 + ADX > 25 |
| Exit | SL/TP via position manager |
| Stop Loss | Strategy-defined |
| Take Profit | Strategy-defined |
| Position Sizing | 0.10 BTC default |
| Expected Edge | **PARTIAL** — +$20 live (#1 winner), `aggregator_selective.go:185` |

#### `XP_EMA_3_8_Cross` (Go Expansion)
| Field | Value |
|:------|:------|
| Location | `engine/internal/strategy/curated_expansion_pack.go` |
| Asset | BTC-USD |
| Timeframe | 1m or 5m |
| Indicators | EMA(3,8), ADX, RSI |
| Entry | EMA cross + ADX gate + RSI band |
| Exit | SL/TP per loop-generated params |
| Stop Loss | 0.15–0.18% (generated) |
| Take Profit | SL × 2.2 |
| Position Sizing | 0.10 BTC |
| Expected Edge | **FAIL** — definitionally overfit parameter grid (`STRATEGY_QUALITY_TABLE.md`: composite ≤2.00) |

#### `FundingMeanReversion_Alpha` (Go)
| Field | Value |
|:------|:------|
| Location | `engine/internal/strategy/alpha_strategies.go` |
| Asset | BTC-USD perpetual |
| Timeframe | 1m (OnTick) |
| Indicators | Funding rate, CVD, MSS, POC confluence |
| Entry | ≥2 confluence votes + funding extreme |
| Exit | 0.35% SL / 0.85% TP |
| Position Sizing | 0.10 BTC |
| Expected Edge | **FAIL (data)** — `data/alpha/funding.ndjson` empty; signals cannot fire |

---

## Client — 108 Strategies

**Registry:** `client/src/lib/futuresStrategies.ts`

### CORE 20 (IDs 91–152) — Always Production

| ID | Name | Category | SL% | TP% | Hold (min) | Indicators (scoring) |
|---:|:-----|:---------|----:|----:|-----------:|:---------------------|
| 91 | Trend_Continuation_Long | Trend | 0.50 | 1.50 | 40 | EMA, momentum3, ADX, RSI |
| 92 | Trend_Continuation_Short | Trend | 0.50 | 1.50 | 40 | Same |
| 95 | Breakout_Long | Breakout | 0.55 | 1.65 | 32 | Range break, volume |
| 96 | Breakout_Short | Breakout | 0.55 | 1.65 | 32 | Same |
| 111 | MTF_Trend_Align_Long | MTF Trend | 0.50 | 1.55 | 50 | HTF + LTF EMA |
| 112 | MTF_Trend_Align_Short | MTF Trend | 0.50 | 1.55 | 50 | Same |
| 117 | MTF_MACD_Align_Long | MTF MACD | 0.52 | 1.56 | 45 | HTF MACD |
| 118 | MTF_MACD_Align_Short | MTF MACD | 0.52 | 1.56 | 45 | Same |
| 123 | MTF_ADX_Power_Long | MTF ADX | 0.52 | 1.56 | 52 | ADX power |
| 124 | MTF_ADX_Power_Short | MTF ADX | 0.52 | 1.56 | 52 | Same |
| 125 | MTF_Breakout_Long | MTF Break | 0.55 | 1.65 | 45 | HTF breakout |
| 126 | MTF_Breakout_Short | MTF Break | 0.55 | 1.65 | 45 | Same |
| 131 | SmartMoney_Accum_Long | Smart Money | 0.50 | 1.55 | 50 | Accumulation proxy |
| 132 | SmartMoney_Distrib_Short | Smart Money | 0.50 | 1.55 | 50 | Distribution proxy |
| 133 | OrderFlow_Break_Long | Order Flow | 0.52 | 1.56 | 38 | OF break |
| 134 | OrderFlow_Break_Short | Order Flow | 0.52 | 1.56 | 38 | Same |
| 139 | Wyckoff_Spring_Long | Wyckoff | 0.55 | 1.65 | 60 | Spring pattern |
| 140 | Wyckoff_Upthrust_Short | Wyckoff | 0.55 | 1.65 | 60 | Upthrust pattern |
| 151 | OpeningDrive_Long | Session | 0.50 | 1.50 | 30 | Session open momentum |
| 152 | OpeningDrive_Short | Session | 0.50 | 1.50 | 30 | Same |

### Premium 28 (IDs 500–527)

Template-driven via `btcFtPremiumStrategies.ts`. Examples: `PRM_VWAP_SessionReject`, `PRM_FundingFade`, `PRM_LiquiditySweep_Scalp`. 2× notional multiplier. SL/TP vary per template.

### Research Pool 60 (IDs 600–659) — `researchOnly: true`

| Sub-pool | IDs | Count | Entry Scoring |
|:---------|:----|------:|:--------------|
| Scalping `SCP_*` | 600–619 | 20 | `scoreScalping()` |
| Day `DAY_*` | 620–639 | 20 | `scoreDay()` |
| Swing `SWG_*` | 640–659 | 20 | `scoreSwing()` |

**Not active in default worker** — excluded by `resolveWorkerStrategyRoster()`.

---

## Dead Code (Not Registered)

| File | Contents | Status |
|:-----|:---------|:-------|
| `research_scalpers.go` | Research prototypes | Dead |
| `external_ai.go` | AI signal bridge | Dead |
| `profit_composites.go` | Composite strategies | Dead |
| `moving_average.go` | Standalone MA | Dead |

---

## Cross-Stack Inventory Gap

| Property | Go 606 | Client 48 |
|:---------|:-------|:----------|
| Strategy names | `*_Scalp`, `XP_*`, `*_Alpha` | `Trend_Continuation_Long`, `PRM_*` |
| Overlap | **ZERO** name overlap | **ZERO** |
| SL range | 0.10–0.55% (sanitized) | 0.50–0.55% |
| TP range | 0.10–0.85% | 1.50–1.65% |
| Exit engine | `positions.Manager` | `paperResolveHardExit` |

**Verdict:** Two independent strategy universes with no code-proven synchronization (`STRATEGY_VALIDATION_REPORT.md`).
