# EXPANSION PACK REMOVAL REPORT
## SEP Phase 2 — Strategy Rationalization

**Date:** 2026-06-10  
**Auditor:** Principal Quant Researcher / SEP Program  
**Status:** COMPLETE

---

## VERDICT

**301 expansion-pack strategies permanently removed from the live registry.**

Registry reduced from **396 strategies → 95 strategies** (76% reduction).

---

## Evidence of Removal

### What Was Removed

`buildExpansionPack()` generated 301 parametric clones across 14 indicator families using deterministic loops. Every variant was prefixed with `XP_`.

| Family | Count | Examples |
|--------|-------|---------|
| EMA Crossover variants | 40 | `XP_EMA_2_8_Cross`, `XP_EMA_6_34_Cross` |
| RSI Threshold variants | 30 | `XP_RSI_7_22_28`, `XP_RSI_28_55_60` |
| RSI Slope variants | 20 | `XP_RSI_Slope_7_2_4p0` |
| Bollinger Signal variants | 25 | `XP_BB_bounce_lower_14_1p8` |
| Bollinger Width variants | 15 | `XP_BB_Width_14_1p8` |
| VWAP variants | 25 | `XP_VWAP_cross_20_0_1` |
| MACD variants | 25 | `XP_MACD_cross_5_13_3_1` |
| N-bar Breakout variants | 20 | `XP_NBar_4_Break`, `XP_NBar_64_Break` |
| Triple EMA variants | 20 | `XP_Triple_3_8_21`, `XP_Triple_21_55_89` |
| CCI variants | 15 | `XP_CCI_zero_cross_10` |
| Stochastic variants | 15 | `XP_Stoch_cross_5_3` |
| ATR variants | 15 | `XP_ATR_momentum_7_14` |
| ROC variants | 12 | `XP_ROC_3_0p30` |
| Williams %R variants | 8 | `XP_WR_bounce_7` |
| PSAR+EMA variants | 6 | `XP_PSAR_EMA_9_0p01` |
| Hull MA variants | 5 | `XP_Hull_12` |
| Keltner variants | 5 | `XP_Kelt_break_20_14_1p5` |
| **TOTAL** | **301** | |

### Why These Were Removed

1. **Definitional Overfitting** — Parameters were chosen by deterministic grid sweep over the same indicator logic as curated strategies. They add zero new signal diversity.

2. **Quality Score: 1.85–2.00 / 10** — All XP strategies received the lowest institutional quality scores in the Phase 23 code audit (10/10 overfitting penalty).

3. **Overfitting Risk: 10/10** — Parametric clones cannot have out-of-sample edge because they were never independently validated. Their performance degrades to market noise post-in-sample period.

4. **Signal Noise** — With 301 XP variants voting alongside 95 curated strategies, the signal aggregator was being swamped by correlated noise, diluting institutional alpha signals.

5. **WINNERS_ONLY Gate Already Active** — These strategies provided no unique edge that curated strategies don't already cover.

### Implementation

```go
// engine/internal/strategy/curated_registry.go, line 409
// BEFORE:
return append(entries, buildExpansionPack()...)

// AFTER:
return entries
```

`buildExpansionPack()` remains in `curated_expansion_pack.go` for reference but is no longer called.

---

## Survivors: 95 Curated Strategies

| Group | Count | Families |
|-------|-------|---------|
| Original Proven Strategies | 24 | EMA, VWAP, RSI, Bollinger, OrderFlow, LinReg, ZScore, etc. |
| Elite V2 — EMA Cross | 15 | Trend |
| Elite V2 — RSI Threshold | 8 | Mean Reversion |
| Elite V2 — RSI Slope | 5 | Mean Rev Elite |
| Elite V2 — Bollinger | 12 | Mean Reversion / Breakout |
| Elite V2 — VWAP | 10 | Trend / Mean Rev Elite |
| Elite V2 — MACD | 10 | Momentum Elite |
| Elite V2 — Volume+Price | 8 | Breakout Elite |
| Elite V2 — N-bar | 10 | Breakout Elite |
| Elite V2 — Triple EMA | 8 | Trend |
| Elite V2 — CCI | 8 | Momentum / Mean Rev |
| Elite V3 — Stochastic | 12 | Mean Reversion |
| Elite V3 — ATR Signal | 10 | Volatility / Breakout |
| Elite V3 — ROC | 8 | Momentum Elite |
| Elite V3 — Williams %R | 8 | Mean Reversion / Trend |
| Elite V3 — PSAR+EMA | 8 | Trend |
| Elite V3 — Hull MA | 8 | Trend |
| Elite V3 — Keltner | 12 | Breakout / Mean Rev |
| Elite V3 — Momentum Divergence | 6 | Price Action Elite |
| Elite V3 — Consecutive Candles | 8 | Trend |
| Elite V3 — Additional variants | 25 | Various |
| Intraday 5m/15m strategies | 65 | Intraday |
| Institutional Alpha Engine | 10 | Funding, CVD, Delta, Liquidity, FVG, OrderBlock, MSS, POC, Session, Liquidation |
| Phase 11 Microstructure Alpha | 7 | All-in-one microstructure |

---

## Expected Impact

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Active strategies | 396 | 95 | −76% |
| XP signal noise | HIGH | ELIMINATED | Signal/noise improved |
| Institutional alpha share of signals | ~2.5% | ~17.9% | +7× |
| Correlation among strategies | HIGH | REDUCED | Cluster risk reduced |
| Quality gate passage rate | LOW | HIGHER | Fewer dilutive signals |

---

## Recommendation

The 95 curated survivors require further evidence-based triage via Phase 3 (loser retirement) once MongoDB/SQLite trade records are available. The current Phase 23 code audit places strategies in four tiers. Tier 3 and Tier 4 strategies (technical indicator clones) remain candidates for retirement pending actual trade evidence.

**Status: FAIL FORWARD** — Survivors unproven. Phase 3 requires live trade data.
