# PHASE 3 — SIGNAL QUALITY AUDIT

**Generated:** 2026-06-10  
**Verdict:** FAIL — production per-strategy metrics unavailable

---

## Data Availability

| Metric Source | Status | Notes |
|:--------------|:------:|:------|
| MongoDB `paper_trades` per-strategy aggregation | **FAIL** | Not accessible in audit environment |
| `/api/paper-trades/analytics` | **FAIL** | Requires live Mongo |
| `/api/paper-trades/strategy-stats` | **FAIL** | Requires live Mongo |
| `fixtures/replay/btc_ft_strategy_rankings.json` | **FAIL** | Empty `[]` |
| `fixtures/research/btc_ft_verdicts.json` | **FAIL** | Empty `[]` |
| Phase 22E synthetic certification | **PARTIAL** | 12 abstract strategies only |
| Aggregator hardcoded live PnL | **PARTIAL** | ~25 strategies, micro-dollar |
| Client replay (48 strats, 500 bars) | **PASS** | Computed below |

---

## Available Evidence — Go Engine Live PnL (Hardcoded)

From `engine/internal/trading/aggregator_selective.go:183-253`:

| Strategy | Documented Live PnL | Signal Quality Inference |
|:---------|--------------------:|:-------------------------|
| TripleFilter_Alpha_Scalp | +$20.00 | **Positive edge (micro sample)** |
| VolumeWeighted_Trend_Scalp | +$16.00 | **Positive edge (micro sample)** |
| EMA_Cross_Scalp | +$4.51 | Marginal positive |
| ZScoreBand_MeanRev_Scalp | +$4.32 | Marginal positive |
| RSI_BB_Confluence_Scalp | +$3.00 | Marginal positive |
| OrderFlow_Pressure_Pro_Scalp | +$2.00 | Marginal positive |
| Stochastic_Range_Scalp | +$1.77 | Marginal positive |
| Chart_DoubleTap_Reversal_Scalp | +$1.63 | Marginal positive |
| BollingerWalk_Trend_Scalp | positive | Marginal positive |
| LinReg_Statistical_Scalp | +$0.56 | Near breakeven |
| VWAP_Bounce_Pro_Scalp | -$1.07 | **Negative** |
| VWAP_RSI2_Reversion_Scalp | -$1.42 | **Negative** |
| SessionOpen_Momentum_Scalp | -$1.40 | **Negative** |
| TripleTrend_Confluence_Scalp | -$1.43 | **Negative** |
| RSI_MACD_Divergence_Scalp | -$2.06 | **Negative** |
| ATR_Volume_Impulse_Scalp | -$19.65 (removed) | **Destroying returns** |
| RangeCompress_Breakout_Scalp | loser (penalized) | **Negative** |
| Exhaustion_Reversal_Scalp | loser (penalized) | **Negative** |

**Critical limitation:** These are **dollar amounts without trade counts, win rates, or profit factors**. Cannot compute expectancy or Sharpe from this data alone.

---

## Phase 22E Synthetic Metrics (NOT Production)

**WARNING:** Generated from `syntheticTrades()` in `phase22e_test.go:39`. **Not 606-strategy production data.**

| Strategy (synthetic) | Trades | PF | WR | Sharpe | Expectancy |
|:---------------------|-------:|---:|---:|-------:|:-----------|
| MSS Continuation | 60 | 2.92 | 65% | 4.17 | $57.45 |
| Funding Rate Arb | 90 | 2.09 | 60% | 3.48 | $24.11 |
| Bollinger Squeeze BTC | 130 | 1.88 | 53% | 3.51 | $20.96 |
| RSI Oversold 30 Revert | 120 | 1.80 | 58% | 3.19 | $25.90 |
| Order Block Rejection | 100 | 1.79 | 58% | 2.89 | $33.12 |
| EMA Cross 21/50 BTC | 180 | 1.65 | 53% | 3.29 | $20.83 |
| FVG Retest Long | 110 | 1.48 | 51% | 2.00 | $22.16 |
| EMA Cross 9/21 BTC | 150 | 1.26 | 51% | 1.40 | $11.39 |
| RSI Slope Mean Rev | 100 | 1.02 | 50% | 0.08 | $0.66 |
| Delta Absorption | 80 | 0.91 | 44% | -0.42 | -$7.14 |
| Liquidity Sweep | 70 | 1.02 | 44% | 0.06 | $0.84 |
| Volume Profile VWAP | 60 | 1.19 | 53% | 0.67 | $7.76 |

**Portfolio synthetic:** PF 1.49, WR 53.2%, Sharpe 6.81, 1250 trades.

**Invalidity proof:** Monte Carlo p5=p50=p95=$25,573 (`MONTE_CARLO_REPORT.md`) — deterministic synthetic input, not real variance.

---

## Client Replay Signal Quality (48 strategies, 500 bars)

**Command:** `npm run replay`  
**Period:** ~8.3 hours (500 × 1m bars, Nov 2023 sample)  
**Initial balance:** $1,000

| Metric | Value |
|:-------|------:|
| Total trades | 113 |
| Net PnL | +$102.29 |
| Expectancy/trade | +$0.91 |
| ROI | +10.2% |
| Exit: PROFIT_LOCK | 74 (65.5%) |
| Exit: SL | 38 (33.6%) |
| Exit: MOM_DECAY | 1 (0.9%) |

**Inference:** Client 48-strategy pool shows **positive expectancy on short sample**. Not statistically significant (n=113, single regime window). Cannot extrapolate to 90/180/365 days.

---

## Signal Statistical Edge Determination

| Universe | Edge Proven? | Evidence |
|:---------|:------------:|:---------|
| Go 606 (full) | **FAIL** | No per-strategy production metrics |
| Go top 14 winners | **PARTIAL** | Positive micro-PnL, no PF/WR |
| Go 301 XP expansion | **FAIL** | No metrics; overfit by design |
| Go 17 alpha | **FAIL** | 8/17 broken (dispatch/data) |
| Client 48 | **PARTIAL** | Replay +$0.91/trade, n=113 |
| Client 108 | **FAIL** | 60 research never executed |
| Phase 22E 12 | **INVALID** | Synthetic data |

---

## Do Signals Possess Statistical Edge?

**Answer: UNPROVEN at portfolio scale.**

- **14 Go strategies** show marginal positive live PnL (largest +$20 total).
- **11 Go strategies** proven losers (removed, -$108.81 cumulative).
- **592 Go strategies** have **zero documented trade outcomes**.
- Phase 22E certification **cannot be cited** as edge proof — synthetic generator confirmed in source code.

**Required to pass:** Export MongoDB `paper_trades` grouped by `strategy_name` with n≥30 per strategy, compute PF, WR, expectancy, binomial significance.
