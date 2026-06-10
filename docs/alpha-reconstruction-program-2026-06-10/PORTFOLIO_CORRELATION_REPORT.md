# PHASE 9 — PORTFOLIO CORRELATION REPORT

**Date:** 2026-06-10

---

## Correlation Risk Framework

In a multi-strategy portfolio, the primary risk beyond individual strategy performance is **signal correlation** — when multiple strategies fire in the same direction simultaneously, creating:
1. Hidden leverage (multiple positions = multiple times the intended risk)
2. Correlated drawdowns (all strategies lose at once in adverse regime)
3. Capital concentration in a single market view

**Institutional requirement:** Portfolio correlation matrix, max pairwise strategy correlation, max directionality.

---

## Why This Portfolio Has Extreme Correlation Risk

The 714-strategy registry contains 657 parameter variants of ~39 signal engines. Even after deduplication to 57 candidates, the following correlation structure remains:

### Level 1: Within-Family Correlation

Strategies using the same indicator share 100% correlation on whether to signal:
- All 70+ EMA variants → LONG on any EMA bullish crossover bar
- All 43+ RSI variants → LONG when RSI exits oversold
- All 55+ BB variants → LONG when price touches lower band

**In a falling market with RSI oversold + BB lower band touch (common):**
- ~40 strategies simultaneously generate BUY signals
- Aggregator approves up to 25
- Up to 25 positions opened simultaneously
- ALL will lose if the bearish trend continues

### Level 2: Cross-Family Correlation

EMA cross, RSI oversold, BB bounce, and MACD cross all tend to fire at the same time in a strong directional move because:
- EMA crosses down → bearish signal (or up for RSI oversold entry after decline)
- RSI enters oversold → price has declined
- BB price < lower band → price has declined
- MACD crosses down → price declining

All four events occur simultaneously during fast moves. The families are not independent.

**Estimated cross-family correlation (BTC 1m):**
- EMA vs RSI: ~0.45-0.55 (fire in similar but different conditions)
- EMA vs BB Bounce: ~0.35-0.45
- RSI vs BB Bounce: ~0.60-0.70 (very high: both fire on price decline)
- MACD vs EMA: ~0.70-0.80 (very high: both trend-following, near-identical signal timing)
- Statistical vs EMA: ~0.20-0.30 (lower: Z-score is regime-agnostic)
- Multi-confluence vs EMA: ~0.55-0.65 (confluence requires EMA as a component)

---

## Correlation-Adjusted Portfolio Analysis

### Actual Independent Signal Generators (Low Correlation Groups)

**Group A — Trend Following:** EMA, Triple EMA, Multi-confluence (all correlated ≥0.55)
**Group B — Mean Reversion:** RSI, BB Bounce, Z-Score, LinReg (correlated ≥0.45)
**Group C — Order Flow:** OFP, CVD, Delta, Liquidity Sweep (correlation unknown but mechanically independent)
**Group D — Structure:** FVG, OB, MSS (mechanically independent of A/B)
**Group E — Timing:** Session Expansion, Liquidation Cascade (timing/event driven)

**Between groups:** A vs B: ~0.10-0.20 (opposite signals on same move), A vs C: ~0.30, A vs D: ~0.20

**Effective independent bets in the portfolio:**
- Group A (trend): 1 independent bet
- Group B (mean rev): 1 independent bet
- Group C (order flow): 2-3 independent bets
- Group D (structure): 3 independent bets
- Group E (timing): 2 independent bets

**Total effective independent bets: ~10** (not 714, not 57 — 10)

---

## Hidden Leverage Analysis

When 25 signals fire simultaneously (aggregator cap), and each takes 1% of $1M capital:

```
Position size per strategy: 1% × $1M = $10,000 notional
25 simultaneous positions: $250,000 notional = 25% of portfolio exposed
All correlated direction: effective leverage = 25× on the directional view
```

If all 25 are BUY signals and BTC drops 0.20% (a small move):
- Each position hits SL: -0.20% × $10,000 = -$20 per position
- 25 positions × -$20 = **-$500 simultaneous loss**
- On $1M portfolio: -0.05% in one batch

This is manageable. But if 25 positions open across 3 batches (75 positions):
- $750,000 notional (75% of portfolio)
- BTC drops 0.50%: -0.50% × $750,000 = **-$3,750 drawdown**
- On $1M portfolio: -0.375% in one event

**The MaxPerStrategy = 2 limit partially controls this** — each strategy can hold max 2 positions simultaneously. But 25 different strategies × 2 positions = 50 possible simultaneous positions = $500,000 notional = 50% of portfolio in correlated positions.

---

## Correlation Matrix (Estimated, 10 Families)

| | EMA | RSI | StatZ | Conf | OFP | FVG | OB | MSS | Session | Liq |
|:--|:--:|:--:|:-----:|:----:|:---:|:---:|:--:|:---:|:-------:|:---:|
| EMA | 1.0 | .45 | .25 | .60 | .30 | .20 | .15 | .20 | .25 | .10 |
| RSI | .45 | 1.0 | .35 | .45 | .25 | .15 | .15 | .10 | .15 | .05 |
| StatZ | .25 | .35 | 1.0 | .30 | .20 | .10 | .10 | .10 | .10 | .05 |
| Conf | .60 | .45 | .30 | 1.0 | .35 | .25 | .20 | .25 | .25 | .10 |
| OFP | .30 | .25 | .20 | .35 | 1.0 | .40 | .40 | .35 | .20 | .30 |
| FVG | .20 | .15 | .10 | .25 | .40 | 1.0 | .60 | .55 | .30 | .25 |
| OB | .15 | .15 | .10 | .20 | .40 | .60 | 1.0 | .55 | .25 | .20 |
| MSS | .20 | .10 | .10 | .25 | .35 | .55 | .55 | 1.0 | .30 | .20 |
| Session | .25 | .15 | .10 | .25 | .20 | .30 | .25 | .30 | 1.0 | .15 |
| Liq | .10 | .05 | .05 | .10 | .30 | .25 | .20 | .20 | .15 | 1.0 |

**Highest correlations (>0.50):**
- EMA ↔ Multi-confluence: 0.60 (because confluence contains EMA component)
- FVG ↔ OB: 0.60 (both structural, fire in similar conditions)
- FVG ↔ MSS: 0.55 (MSS and FVG often form together)
- OB ↔ MSS: 0.55 (same as above)

**Lowest correlations (near independent):**
- Liquidation ↔ RSI: 0.05 (event-driven vs oscillator)
- StatZ ↔ Liq: 0.05 (mathematical vs event)
- EMA ↔ Liquidation: 0.10 (trend-following vs cascade event)

---

## Portfolio Diversification Score

**Effective number of independent strategies (using eigenvalue decomposition approximation):**

With 10 families having average pairwise correlation of ~0.25:
```
Effective N ≈ n / (1 + (n-1) × avg_correlation)
Effective N ≈ 10 / (1 + 9 × 0.25)
Effective N ≈ 10 / 3.25 ≈ 3.1
```

**The 714-strategy portfolio has approximately 3 effective independent bets.** This is a highly concentrated portfolio despite the large strategy count.

---

## Redundant Strategy Clusters (Post-Deduplication)

Even after reducing to 57 candidates, correlation risks persist:

### Redundant Group 1: Trend Concentration
- TripleFilter_Alpha_Scalp (winner, keep)
- VolumeWeighted_Trend_Scalp (winner, keep)
- EMA_Cross_Scalp (winner, keep)
- EMA_5_13 (representative, redundant with EMA_Cross)
- Triple_EMA_8_21_55 (redundant with trend group)

**Correlation:** These 5 strategies will simultaneously BUY in uptrend and SELL in downtrend. They are 4 copies of 1 directional bet amplified 5×.

**Recommended:** Keep TripleFilter + VolumeWeighted only. Remove EMA_Cross_Scalp as redundant with TripleFilter (which contains EMA component). Keep one representative EMA strategy for the EMA signal family.

### Redundant Group 2: Mean Reversion Concentration
- ZScoreBand_MeanRev_Scalp (winner, keep)
- LinReg_Statistical_Scalp (winner, keep)
- RSI_BB_Confluence_Scalp (winner, keep)
- BB_Bounce_20_2 (representative)
- RSI_Oversold30 (representative)

**Correlation:** Z-Score and RSI+BB all fire when price is extended/oversold. Three independent approaches but all long when price is down.

**Recommended:** Reduce to ZScore + LinReg (both winners) + RSI_BB_Confluence as the third distinct input. Remove redundant BB Bounce and RSI Oversold representatives.

---

## Hidden Leverage Correction

**Current risk:** When RSI oversold + BB bounce + Z-score all fire simultaneously:
- 3+ strategies take LONG positions at same time
- Effective directional exposure = 3% of portfolio instead of 1%

**Fix:** Portfolio-level net exposure limit:
```go
type ExposureGuard struct {
    MaxNetLongPct  float64  // max 5% net long
    MaxNetShortPct float64  // max 5% net short
    MaxGrossPct    float64  // max 10% gross exposure
}
```

This caps total portfolio directionality regardless of how many strategies agree.

---

## Phase 9 Verdict

**Correlation risk is the most underappreciated structural risk in this portfolio.**

Key findings:
1. **714 strategies → ~3 effective independent bets** (eigenvalue decomposition)
2. **EMA-RSI-BB correlation: 0.45-0.70** — these families are not independent
3. **Aggregator allows 25 correlated signals per batch** — creating potential 25% directional exposure
4. **MaxPerStrategy = 2 provides partial protection** but not portfolio-level correlation control
5. **Structural alpha (FVG/OB/MSS) is the most independent group** — lowest correlation to other families

**Recommended maximum portfolio correlation:** Any two strategies in the approved portfolio should have correlation < 0.40. Under this constraint, the approved portfolio shrinks to ~8-10 strategies:
- TripleFilter or VolumeWeighted (1, not both — too correlated)
- ZScoreBand (2)
- LinReg (3)
- OrderFlow (4)
- FVG (5)
- OB (6)
- MSS (7)
- Session or Liquidation (8)
- Funding Rate (9, post-fix)
- One wildcard (10)
