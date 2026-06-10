# PHASE 14 — WINNING TRADE FORENSICS

**Generated:** 2026-06-10  
**Verdict:** PARTIAL — limited winner evidence; conditions partially identified

---

## Data Availability

| Source | Winning Trades | Status |
|:-------|:-------------:|:------:|
| MongoDB production | **0 accessible** | **FAIL** |
| Go aggregator winners | 14 strategies (aggregate $) | **PARTIAL** |
| Client replay | 75 winning trades (net > 0) | **PARTIAL** |
| Phase 22E synthetic | 663/1250 wins (53.2%) | **INVALID** |

---

## Go Engine — Winning Trade Conditions

### Proven Winners (Live PnL > $0)

| Strategy | PnL | Winning Conditions (from code analysis) |
|:---------|----:|:------------------------------------------|
| TripleFilter_Alpha_Scalp | +$20 | EMA20 + MACD hist > 0 + ADX > 25 (multi-filter) |
| VolumeWeighted_Trend_Scalp | +$16 | Volume-weighted trend confirmation |
| EMA_Cross_Scalp | +$4.51 | EMA crossover in trending 1m |
| ZScoreBand_MeanRev_Scalp | +$4.32 | Statistical z-score band reversion |
| RSI_BB_Confluence_Scalp | +$3.00 | RSI + Bollinger confluence |
| OrderFlow_Pressure_Pro_Scalp | +$2.00 | Order flow pressure imbalance |
| Stochastic_Range_Scalp | +$1.77 | Stochastic in range-bound conditions |
| Chart_DoubleTap_Reversal_Scalp | +$1.63 | Double tap reversal pattern |
| BollingerWalk_Trend_Scalp | positive | Bollinger band walk (trending) |
| LinReg_Statistical_Scalp | +$0.56 | Linear regression statistical edge |
| OpeningRange_Breakout_Scalp | boosted | Session opening range break |
| VolSqueeze_Explosion_Scalp | boosted | Volatility squeeze → expansion |
| TrendMomentum_Score_Scalp | boosted | Composite momentum score |
| Bollinger_RSI_Fade_Scalp | boosted | BB + RSI fade in range |

### Winning Pattern Analysis

| Pattern | Winners Using | Edge Mechanism |
|:--------|:-------------|:---------------|
| Multi-signal confluence | 6 strategies | Reduces false positives |
| Statistical (z-score, linreg) | 2 strategies | Non-indicator-grid edge |
| Order flow / volume | 3 strategies | Microstructure information |
| Volatility regime (squeeze) | 1 strategy | Regime-specific |
| Session timing | 1 strategy | Time-of-day edge |

**Common thread:** Winners use **≥2 independent confirmation signals** or **statistical models** — not single-indicator crossovers.

### What Does NOT Win

| Pattern | Evidence |
|:--------|:---------|
| Single EMA cross (15+301 variants) | No winner in top 14; EMA_Cross +$4.51 is base version only |
| Single RSI threshold | No winner |
| Single MACD signal | MACD families removed or penalized |
| Single Bollinger touch | Only confluence version wins |
| Breakout without volume confirm | 5 removed losers |

---

## Client Replay — Winning Trade Conditions

| Metric | Value |
|:-------|------:|
| Winning trades | 75 (66.4% of 113) |
| Primary exit | PROFIT_LOCK (74 trades) |
| Net winner contribution | +$102.29 total |
| Avg winner | ~+$1.36 estimated |

### Winning Conditions in Replay

| Factor | Observation |
|:-------|:------------|
| Exit mechanism | PROFIT_LOCK captures partial gains before full TP |
| SL geometry | 0.50% SL allows trade to breathe through noise |
| TP geometry | 1.50% TP provides 3:1 R:R |
| Leverage | 25× amplifies small moves into meaningful PnL |
| Sample period | Nov 2023, 8.3 hours — likely volatile window |

---

## Phase 22E Synthetic Winners (INVALID)

Top synthetic performers: MSS (PF 2.92), Funding (PF 2.09), Bollinger (PF 1.88).

**Cannot cite as winning conditions** — synthetic data with no market context.

---

## Conditions That Create Profits (Evidence-Based)

| Condition | Confidence | Evidence |
|:----------|:-----------|:---------|
| Multi-signal confluence (≥2 filters) | **HIGH** | 6/14 Go winners |
| Statistical models (z-score, linreg) | **MEDIUM** | 2/14 winners |
| Order flow / volume confirmation | **MEDIUM** | 3/14 winners |
| Wider SL (≥0.50%) | **MEDIUM** | Client replay profitable with 0.50% SL |
| Volatility squeeze → expansion | **LOW** | 1 winner, no trade count |
| Session open momentum | **LOW** | 1 winner, no trade count |
| Single indicator crossover | **FAIL** | 0 winners among 301 XP + most elite |

---

## Winning Trade Forensics Verdict

**Profits come from confluence and statistical strategies, not from indicator parameter grids.**

Maximum documented winner: +$20 total PnL. This is **not institutional-scale alpha** — it is marginal paper trading evidence on a $1M account (0.002% return from best strategy).
