# PHASE 13 — PORTFOLIO CONSTRUCTION REPORT

**Date:** 2026-06-10

---

## Current Portfolio State

### Problems with Current Construction

1. **714 strategy names, 39 signal engines** — extreme name inflation masks concentration
2. **Aggregator cap of 25/batch** — 96% of strategies never execute; not a real portfolio
3. **Dual independent stacks** — Go 606 and Client 48 with zero name overlap, separate PnL
4. **Equal weight by default** — no risk-weighting by validated edge
5. **Single asset** — 100% BTC; no cross-asset diversification
6. **Single direction bias** — most strategies are long-biased on a 24/7 crypto asset

---

## Target Portfolio Design

### Target: 10–20 Statistically Validated Strategies

The goal is not maximum strategy count — it is maximum independent alpha with minimum correlation. For BTC scalping:

**10 strategies can be a complete portfolio if:**
- They span multiple market regimes
- They use independent signal types (not correlated families)
- They have validated OOS profit factors
- They are sized by evidence quality

---

## Proposed Portfolio Construction (Post-Fix)

### Candidate Portfolio: 15 Strategies

| Slot | Strategy | Type | Best Regime | Expected PF (OOS target) | Allocation |
|:----:|:---------|:-----|:-----------|:------------------------:|:-----------:|
| 1 | TripleFilter_Alpha_Scalp | Multi-signal trend | Trend | ≥1.5 | 12% |
| 2 | VolumeWeighted_Trend_Scalp | Volume trend | Trend | ≥1.4 | 12% |
| 3 | ZScoreBand_MeanRev_Scalp | Statistical MR | Range | ≥1.4 | 10% |
| 4 | RSI_BB_Confluence_Scalp | Multi-signal MR | Range | ≥1.3 | 8% |
| 5 | MSSContinuation_Alpha* | Smart money structure | Trend+Break | ≥1.5 | 10% |
| 6 | OrderBlockRetest_Alpha* | Smart money OB | Trend | ≥1.4 | 8% |
| 7 | FVGRetest_Alpha* | Fair value gap | Trend | ≥1.3 | 8% |
| 8 | FundingMeanReversion_Alpha* | Funding carry | All | ≥1.5 | 10% |
| 9 | LiquidationCascade_Alpha* | Liquidation event | Volatile+Panic | ≥1.3 | 6% |
| 10 | VolSqueeze_Explosion_Scalp | Volatility regime | Compression→Break | ≥1.3 | 6% |
| 11 | OrderFlow_Pressure_Pro_Scalp | Order flow | Trend+Volatile | ≥1.3 | 5% |
| 12 | OpeningRange_Breakout_Scalp | Session timing | Trend | ≥1.3 | 5% |
| 13 | Chart_DoubleTap_Reversal_Scalp | Price action | Any | ≥1.2 | 4% |
| 14 | LinReg_Statistical_Scalp | Statistical | Range | ≥1.2 | 4% |
| 15 | Stochastic_Range_Scalp | Oscillator | Range | ≥1.2 | 2% |

*Requires dispatch bug fix before inclusion

**Total portfolio allocation: 110% (correlation adjustment reduces effective exposure)**

---

## Correlation Analysis (Theoretical)

### Signal Independence Assessment

| Strategy Pairs | Signal Overlap | Correlation (Est.) |
|:--------------|:--------------|:-----------------:|
| TripleFilter vs VolumeWeighted | Different (EMA+MACD vs Volume) | ~0.4 |
| ZScoreBand vs LinReg | Similar (both statistical) | ~0.6 |
| MSS vs OrderBlock | Similar (both smart money) | ~0.5 |
| FVG vs MSS | Similar (price action structure) | ~0.4 |
| Funding vs Liquidation | Independent (different data) | ~0.1 |
| VolSqueeze vs OpeningRange | Different (regime vs session) | ~0.2 |
| RSI_BB vs ZScore | Partially similar | ~0.3 |
| OrderFlow vs VolumeWeighted | Partially similar | ~0.3 |

**Estimated average pairwise correlation: ~0.30**  
**Portfolio diversification benefit: moderate**  
**Effective N independent strategies: ~8-10 of 15**

---

## Portfolio Risk Framework

### Maximum Exposures

| Limit | Value | Rationale |
|:------|:-----:|:---------|
| Max single strategy allocation | 15% | No single strategy > 15% of capital |
| Max family exposure | 25% | No signal family > 25% combined |
| Max concurrent positions | 5 | Prevent correlated exposure |
| Max daily loss per strategy | 2% of capital | Auto-reduce sizing after hit |
| Max portfolio daily drawdown | 5% | Kill switch trigger |
| Max portfolio weekly drawdown | 10% | Manual review trigger |

### Strategy Correlation Bucketing

**Bucket A — Trend Following (max 30% combined):**
- TripleFilter_Alpha_Scalp
- VolumeWeighted_Trend_Scalp
- MSSContinuation_Alpha

**Bucket B — Mean Reversion (max 30% combined):**
- ZScoreBand_MeanRev_Scalp
- RSI_BB_Confluence_Scalp
- LinReg_Statistical_Scalp

**Bucket C — Institutional Alpha (max 35% combined):**
- FundingMeanReversion_Alpha
- OrderBlockRetest_Alpha
- FVGRetest_Alpha
- LiquidationCascade_Alpha

**Bucket D — Volatility/Event (max 20% combined):**
- VolSqueeze_Explosion_Scalp
- LiquidationCascade_Alpha

**Bucket E — Microstructure (max 15% combined):**
- OrderFlow_Pressure_Pro_Scalp
- Chart_DoubleTap_Reversal_Scalp

---

## Portfolio Metrics Targets

For the proposed 15-strategy portfolio, after walk-forward validation:

| Metric | Target | Minimum Acceptable |
|:-------|:------:|:------------------:|
| Portfolio Sharpe (annualized) | ≥ 2.0 | ≥ 1.0 |
| Portfolio Profit Factor | ≥ 1.50 | ≥ 1.30 |
| Portfolio Win Rate | ≥ 50% | ≥ 45% |
| Max Monthly Drawdown | ≤ 8% | ≤ 15% |
| Max Annual Drawdown | ≤ 20% | ≤ 30% |
| Expectancy per trade | ≥ +$0.50 | ≥ +$0.20 |
| Monthly trade count | ≥ 500 | ≥ 200 |
| Capital efficiency | ≥ 70% | ≥ 50% |

**None of these targets are currently validated. They are targets for the post-fix, post-validation portfolio.**

---

## Unify Go + Client Into Single Stack

Currently the platform runs two independent strategy universes (Go 606 + Client 48) with:
- Zero strategy name overlap
- Separate execution engines
- Separate PnL accounting
- Different SL/TP geometries

**This is a portfolio construction failure.** The combined platform cannot have a coherent portfolio view when two independent stacks trade simultaneously on the same account.

**Long-term recommendation:** Unify into a single strategy registry with:
- Client scoring logic (TypeScript) used for signal generation
- Go execution engine used for order routing
- Shared risk gate applied to all signals regardless of source
- Single PnL ledger

This is a 2-4 week engineering effort but is required for institutional-grade portfolio management.

---

## Phase 13 Verdict

**FAIL — current portfolio is not constructed; it is a registry with 714 names and no coherent allocation.**

The path to a real portfolio:
1. Retire 458+ strategies (this report, Phases 3, 11)
2. Fix alpha dispatch bugs (Phase 4)
3. Run 15 survivors through walk-forward (Phase 10)
4. Size by validated Kelly (Phase 12)
5. Apply regime gates (Phase 9)
6. Compute portfolio correlation and cap buckets
7. Deploy with hard drawdown limits

**Timeline: 6-8 weeks to first validatable 15-strategy portfolio**
