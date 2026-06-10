# PHASE 18 — STRATEGY RANKING

**Generated:** 2026-06-10  
**Verdict:** PARTIAL — ranking possible only for ~25 strategies with evidence

---

## Ranking Methodology

**Composite score (when data available):**
- 35% Profit Factor
- 25% Sharpe Ratio
- 20% Win Rate
- 10% Expectancy per trade
- 10% Drawdown penalty

**Data constraint:** Full ranking of 714 strategies is **impossible** — only ~25 have any PnL evidence.

---

## Tier Definitions

| Tier | Criteria | Action |
|:-----|:---------|:-------|
| **A** | Live PnL > $0, multi-signal confluence, n≥5 | Full capital |
| **B** | Live PnL > $0 or synthetic PF ≥ 1.30, needs validation | Reduced capital |
| **C** | No data or marginal PF 1.0-1.30 | Paper-only |
| **D** | Proven loser or overfit grid | Retire immediately |

---

## Go Engine Rankings (Evidence-Based)

### Tier A — Proven Live Winners

| Rank | Strategy | Live PnL | Family | Expectancy | PF | Sharpe |
|:----:|:---------|:---------|:-------|:-----------|---:|:-------|
| 1 | TripleFilter_Alpha_Scalp | +$20.00 | Multi-Signal | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 2 | VolumeWeighted_Trend_Scalp | +$16.00 | Volume | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 3 | EMA_Cross_Scalp | +$4.51 | Trend | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 4 | ZScoreBand_MeanRev_Scalp | +$4.32 | Statistical | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 5 | RSI_BB_Confluence_Scalp | +$3.00 | Multi-Signal | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 6 | OrderFlow_Pressure_Pro_Scalp | +$2.00 | Order Flow | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 7 | Stochastic_Range_Scalp | +$1.77 | Mean Reversion | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 8 | Chart_DoubleTap_Reversal_Scalp | +$1.63 | Price Action | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 9 | BollingerWalk_Trend_Scalp | positive | Bollinger | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |
| 10 | TrendMomentum_Score_Scalp | boosted | Momentum | **UNKNOWN** | **UNKNOWN** | **UNKNOWN** |

### Tier B — Boosted, Needs Validation

| Strategy | Basis |
|:---------|:------|
| OpeningRange_Breakout_Scalp | Aggregator boost +1.35 |
| VolSqueeze_Explosion_Scalp | Aggregator boost +1.35 |
| LinReg_Statistical_Scalp | +$0.56 live |
| Bollinger_RSI_Fade_Scalp | Aggregator boost +1.30 |
| All 17 alpha strategies | Boosted +1.45 but plumbing broken |

### Tier C — No Data (592 strategies)

All Elite V2/V3, intraday, and expansion pack strategies without live PnL:
- 301 `XP_*` expansion pack
- 65 `ID_*` intraday
- ~226 elite/hand-crafted without evidence

### Tier D — Proven Losers / Retire

| Rank | Strategy | Loss | PF (synthetic) |
|:----:|:---------|-----:|:--------------:|
| 1 | ATR_Volume_Impulse_Scalp | -$19.65 | — (removed) |
| 2 | ATR_Breakout | -$15.43 | — (removed) |
| 3 | KAMA_Adaptive | -$14.36 | — (removed) |
| 4 | MACD_VWAP_Flip | -$10.90 | — (removed) |
| 5 | PriceChannel_Breakout | -$11.29 | — (removed) |
| 6 | DeltaAbsorption_Alpha | — | 0.91 (synthetic) |
| 7 | RSI_MACD_Divergence_Scalp | -$2.06 | — (active) |
| 8 | All MACD family (10) | — | ≤1.26 |
| 9 | All CCI family (8) | — | RETIRE per quality table |
| 10 | All Williams %R (8) | — | RETIRE per quality table |

---

## Client Rankings

### Tier A — Active Production (48 strategies)

All 48 default strategies are Tier B minimum (active but unvalidated individually).

**Portfolio-level replay:** +$0.91/trade expectancy → Tier B as a pool.

### Tier C — Research Only (60 strategies)

IDs 600-659: `researchOnly: true`. Never executed in default worker.

### Tier D — Stubs (40 strategies)

IDs 660-759: Empty definition arrays.

---

## Phase 22E Synthetic Rankings (INVALID — Reference Only)

| Rank | Strategy | PF | Sharpe | Status |
|:----:|:---------|---:|:-------|:-------|
| 1 | MSS Continuation | 2.92 | 4.17 | Synthetic approved |
| 2 | Funding Rate Arb | 2.09 | 3.48 | Synthetic approved |
| 3 | Bollinger Squeeze BTC | 1.88 | 3.51 | Synthetic approved |
| 7 | FVG Retest Long | 1.48 | 2.00 | Synthetic approved |
| 10 | Delta Absorption | 0.91 | -0.42 | Synthetic rejected |

**Do not use for capital allocation decisions.**

---

## Strategy Quality Tiers (Code Analysis)

From `STRATEGY_QUALITY_TABLE.md`:

| Tier | Families | Count |
|:-----|:---------|------:|
| TIER 1 | Phase 11 Alpha, Institutional Alpha, Funding MR | 17 |
| TIER 2 | Momentum Div, OrderFlow, ZScore, LinReg, TripleFilter | ~12 |
| TIER 3 | Volume Breakout, N-bar, Triple EMA, ATR, Keltner, VWAP, Intraday | ~130 |
| TIER 4 / RETIRE | EMA Cross, RSI, BB, MACD, Stochastic, PSAR, Hull, CCI, Williams, ROC, Consecutive, XP_* | ~447 |

---

## Ranking Verdict

| Tier | Go Count | Client Count | Total |
|:-----|:---------|:-------------|------:|
| A (proven) | 10 | 0 (individual) | 10 |
| B (promising) | 17+ | 48 | 65+ |
| C (unvalidated) | 568 | 60 | 628 |
| D (retire) | 11+447 | 0 | 458+ |

**Only 10 of 714 strategies (1.4%) have positive live PnL evidence.**
