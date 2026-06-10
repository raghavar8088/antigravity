# PHASE 12 — WALK-FORWARD VALIDATION REPORT

**Date:** 2026-06-10

---

## Walk-Forward Framework

**Walk-forward analysis (WFA)** is the institutional standard for validating strategy robustness. It tests whether a strategy's performance generalizes beyond its optimization period.

### Standard Protocol
```
1. Split historical data: 70% in-sample (IS), 30% out-of-sample (OOS)
2. Optimize parameters on IS data
3. Validate on OOS data (never used in optimization)
4. Roll window forward: new IS window, new OOS window
5. Repeat across full history
6. Report: IS Sharpe vs OOS Sharpe, IS PF vs OOS PF
```

**Institutional acceptance criteria:**
- OOS Sharpe ≥ 0.70 × IS Sharpe (strategy survives OOS degradation)
- OOS PF ≥ 0.80 × IS PF
- OOS win rate within 8 percentage points of IS win rate
- Consistent across ≥3 OOS windows

---

## Current Validation Status

### What Has Been Done

| Validation Type | Status | Evidence Available |
|:----------------|:------:|:------------------:|
| In-sample optimization | ❓ Unknown | Parameters hard-coded — may be optimized or default |
| Out-of-sample testing | ❌ NOT DONE | No OOS period defined or tested |
| Walk-forward analysis | ❌ NOT DONE | No rolling window structure |
| Monte Carlo simulation | ❌ NOT DONE | No distribution sampling |
| Cross-validation | ❌ NOT DONE | No k-fold or expanding window |
| Paper trading | ✅ Partial | $1M paper desk running, but no per-strategy split |

### What Data Is Available

| Dataset | Period | Quality |
|:--------|:------:|:-------:|
| Client replay | 8.3 hours, Nov 2023 | Insufficient (single session) |
| Paper trades (MongoDB) | ~6+ months | Not accessible in audit |
| Synthetic backtest data | 22 engine scenarios | Invalid (labeled synthetic) |
| Audit NDJSON | 2026-06-10 | 0 TRADE_CLOSED events |

**Walk-forward analysis has NOT been performed on any strategy in this registry.**

---

## Parameter Sensitivity Analysis

Without walk-forward data, parameter sensitivity can be assessed theoretically:

### EMA Period Sensitivity

The registry contains EMA variants across many period pairs:
- Very fast: EMA(3,8) — 3 extreme crossovers/hour, very sensitive to noise
- Standard: EMA(5,13) — balanced
- Slow: EMA(20,50) — few signals, more regime-dependent

**If EMA(5,13) is the selected best performer:** Is performance specific to these exact periods?

Empirical finding from literature: EMA crossover performance on intraday crypto is not period-sensitive — the edge (or lack thereof) exists across many periods at similar levels. This suggests low overfitting risk for EMA parameters.

**Period sensitivity: LOW** — EMA parameters are not fragile.

### RSI Threshold Sensitivity

Registry contains RSI variants at different overbought/oversold thresholds (25/75, 30/70, 35/65):

Performance should degrade smoothly as thresholds tighten (fewer signals) or widen (more false signals). If performance peaks sharply at one specific threshold, overfitting is likely.

**Threshold sensitivity: MEDIUM** — RSI extremes at 30/70 are the natural statistical boundary (2σ equivalent), so they're less likely to be overfit than arbitrary thresholds like 27/73.

### Bollinger Band Width Sensitivity

BB(20, 2.0) vs BB(20, 1.5) vs BB(20, 2.5): similar pattern to RSI — 2.0 standard deviations is the natural statistical reference. Not likely overfit.

**Width sensitivity: LOW for 2.0σ** — natural statistical boundary.

### Z-Score Window Sensitivity

Z-score uses 30-bar window. Sensitivity to window size:
- 15 bars: noisier, more false extremes
- 30 bars: roughly 30 minutes of 1m context
- 60 bars: smoother, fewer signals, more regime-dependent

If ZScoreBand(30) was specifically chosen over ZScoreBand(20) or ZScoreBand(45): moderate overfitting concern.

**Window sensitivity: MEDIUM** — needs OOS validation to confirm.

---

## Institutional Validation Requirements

### Minimum Data Requirements for Certification

| Requirement | Current Status | Gap |
|:------------|:--------------:|:---:|
| 200+ trades per strategy OOS | ❌ 0 strategies | Critical |
| 90+ day OOS validation period | ❌ Never tested | Critical |
| 3+ regime types in OOS data | ❌ Never tested | Critical |
| IS Sharpe computed | ❌ No Sharpe data | Critical |
| OOS Sharpe computed | ❌ No OOS data | Critical |
| WFA efficiency ratio (OOS/IS) | ❌ N/A | Critical |
| Maximum drawdown in OOS | ❌ No data | High |
| CAGR in OOS | ❌ No data | High |

**All 9 critical validation requirements are unmet for all strategies.**

---

## Walk-Forward Design for This Platform

### Proposed WFA Protocol

Given available infrastructure (Go backtester at `engine/cmd/backtest/main.go`):

**Step 1: Data preparation**
- Source BTC 1m OHLCV from Binance API: 2022-01-01 to 2026-06-10 (4+ years)
- Store in PostgreSQL TimescaleDB with timestamp, open, high, low, close, volume
- Separate into folds:

```
Fold 1: IS = 2022-01-01 to 2022-12-31, OOS = 2023-01-01 to 2023-06-30
Fold 2: IS = 2022-07-01 to 2023-06-30, OOS = 2023-07-01 to 2023-12-31
Fold 3: IS = 2023-01-01 to 2023-12-31, OOS = 2024-01-01 to 2024-06-30
Fold 4: IS = 2023-07-01 to 2024-06-30, OOS = 2024-07-01 to 2024-12-31
Fold 5: IS = 2024-01-01 to 2024-12-31, OOS = 2025-01-01 to 2025-06-30
Fold 6: IS = 2024-07-01 to 2025-06-30, OOS = 2025-07-01 to 2025-12-31
```

**Step 2: IS optimization (only for strategies with parameters)**
- For EMA family: optimize period pairs within range (fast=3-8, slow=8-25)
- For RSI: optimize threshold (26-34 buy, 66-74 sell range)
- For BB: optimize width (1.5-2.5σ range)
- Lock parameters after IS optimization. Do NOT re-optimize for each fold.

**Step 3: OOS evaluation**
- Run each strategy with IS-optimized parameters on OOS data
- Compute: trade count, WR, avg win, avg loss, PF, Sharpe, max drawdown

**Step 4: WFA efficiency computation**
```
WFA_efficiency = OOS_Sharpe / IS_Sharpe
Accept if WFA_efficiency >= 0.70
```

**Step 5: Identify survivors**
- Must have WFA_efficiency ≥ 0.70 across ≥ 4 of 6 folds
- Must have OOS PF ≥ 1.30 across ≥ 4 of 6 folds
- Must have OOS trade count ≥ 50 per fold

---

## Expected WFA Results (Predicted)

Based on the signal mechanism analysis:

### Strategies Likely to Pass WFA

**Multi-Signal Confluence:** TripleFilter type strategies have multiple independent conditions that must simultaneously agree. Over-fitting is structurally harder. Expected WFA efficiency: 0.75-0.90.

**Statistical (Z-Score):** Z-score at ±2σ is mathematically derived, not parameter-optimized. Expected WFA efficiency: 0.70-0.85.

**Order Flow (OFP):** If real order flow data is present, this is forward-looking information. Expected WFA efficiency: 0.65-0.80.

### Strategies Likely to Fail WFA

**EMA family:** 70 parameter variants → the specific best-performing parameters in IS are likely noise. The family may survive WFA but specific period-pair selections are likely overfit. Expected WFA efficiency: 0.40-0.65 for individual variants.

**Bollinger Breakout (already removed):** Breakout strategies are highly regime-dependent. Expected WFA efficiency: 0.20-0.45.

**Expansion pack (301 variants):** Pure parameter search across a grid. Maximum overfitting. Expected WFA efficiency: 0.10-0.30.

### Alpha Strategies (Cannot Predict)

All 17 alpha strategies have $0 live PnL. Walk-forward is impossible without live data. Walk-forward can be run on backtested data only after fixing the data/dispatch issues.

---

## Overfitting Risk Assessment

**Overfitting risk = (number of optimization degrees of freedom) / (number of independent OOS trades)**

For a strategy with 3 parameters optimized and 100 IS trades:
- Degrees of freedom: 3
- IS trades: 100
- Overfitting risk: LOW (3/100 = 3%)

For the expansion pack (301 variants, each with 2-4 parameters):
- Degrees of freedom per variant: 3
- But implicit selection bias: best of 301 variants is selected
- Effective degrees of freedom: log(301) × avg_params ≈ 8 × 3 = 24
- Required OOS trades for low overfitting: 24 × 50 = 1,200
- Available OOS trades: Unknown (paper desk may have far fewer)

**The expansion pack 301 variants constitute extreme multiple-testing overfitting.**

---

## Walk-Forward Implementation Estimate

**Prerequisites:**
1. BTC 1m historical data 2022-2026 (~2M rows) — 1-2 days to source and load
2. Backtest engine supports parameterized runs (`backtest/main.go`) — needs validation
3. Parallel run infrastructure — Go concurrency can run 300+ strategy backtests in minutes

**Timeline:**
- Data sourcing and preprocessing: 2 days
- WFA framework implementation: 3-5 days
- Initial results for top 20 strategies: 1 additional day
- Full 57-strategy WFA: 2-3 days

**Total WFA implementation time: 8-12 days** (assuming one developer).

---

## Phase 12 Verdict

**Walk-forward analysis has NOT been performed on any strategy. This is the most critical gap in the validation framework.**

Without WFA, there is no way to distinguish:
1. Strategies with genuine edge that generalizes OOS
2. Strategies that looked good in IS due to curve-fitting
3. Strategies whose parameters happened to work in the paper trading period

**Current validation confidence for all 714 strategies: 0/10**

**Remediation priority: HIGH** — WFA is the minimum institutional standard. The Go backtest engine exists (`backtest/main.go`), historical data can be sourced from Binance API, and the implementation is feasible within 2 weeks. This is the highest ROI infrastructure investment available.

**After WFA, expected survivor count:** 20-30 of 57 candidates (based on typical WFA filter rates in systematic trading research). The remaining 20-30 are genuinely robust, not just well-optimized IS performers.
