# PHASE 3 — DUPLICATE ELIMINATION REPORT

**Date:** 2026-06-10  
**Original strategy count:** 714 (606 Go + 108 Client)  
**Method:** Identify parameter clones and near-duplicates sharing identical signal logic

---

## Duplication Standards Applied

A strategy is a **duplicate** of another when:
- Same signal type (indicator + condition)
- Parameters differ only by ≤15% in period values
- Same SL/TP geometry tier
- No additional entry filter that creates statistical distinction

A strategy is a **near-duplicate** when:
- Same signal type
- Different period range but same behavioral regime on BTC 1m (e.g., EMA(3,8) and EMA(5,10) both produce near-identical signals on BTC 1m due to high autocorrelation)

---

## Elimination Analysis by Family

### EMA Crossover Family: 70+ instances → 3 survivors

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 EMA (15) | -14 | Keep only EMA(5,13) as representative 1m variant |
| Elite V3 additional EMA (5) | -5 | All parameter clones of V2 |
| Intraday EMA 5m (7) | -6 | Keep only EMA(9,21) 5m as sole intraday EMA |
| Intraday EMA 15m (3) | -2 | Keep only EMA(10,30) 15m |
| Expansion pack EMA (40) | -40 | Complete removal |
| Base EMA_Cross_Scalp | KEEP | Has live PnL (+$4.51) |
| **Survivors** | **3** | Base 1m, 5m, 15m representatives |

**Eliminated: 67**

---

### RSI Threshold Family: 43+ instances → 2 survivors

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 RSI threshold (8) | -7 | Keep RSI(14) oversold-30 only |
| Elite V3 additional RSI (5) | -5 | All parameter clones |
| Expansion pack RSI threshold (30) | -30 | Complete removal |
| RSI_Reversal_Scalp (base) | KEEP | Representative 1m version |
| RSI(14) elite V2 | KEEP | Has ADX guard (distinguishing feature) |

**Eliminated: 42**

---

### RSI Slope Family: 25 instances → 1 survivor

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 RSI slope (5) | -4 | Keep RSI(14) slope version only |
| Expansion pack RSI slope (20) | -20 | Complete removal |
| **Survivor** | **1** | RSI(14) slope reversal |

**Eliminated: 24**

---

### Bollinger Bands Family: 55+ instances → 3 survivors

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 BB (12) | -10 | Keep BB_Bounce(20,2) and BB_Breakout(20,2) |
| Elite V3 additional BB (5) | -5 | Parameter clones |
| Intraday BB (8) | -6 | Keep BB_Bounce(20,2) 5m only |
| Expansion pack BB (~30) | -30 | Complete removal |
| Bollinger_Scalp (base) | KEEP | Representative |
| Bollinger_RSI_Fade_Scalp | KEEP | Multi-signal — different family |
| BollingerWalk_Trend_Scalp | KEEP | Distinct mechanism (walk vs. touch) |

**Eliminated: 51**

---

### VWAP Family: 27+ instances → 3 survivors

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 VWAP (10) | -8 | Keep VWAP_Cross(30) and VWAP_Dev(0.3) |
| Elite V3 additional VWAP (5) | -5 | Parameter clones |
| Intraday VWAP (6) | -4 | Keep VWAP_Cross 5m only |
| VWAP_Scalp (base) | KEEP | Representative |
| VWAP_RSI2_Reversion_Scalp | BORDERLINE | Has -$1.42 live; retain for monitoring |
| ZScoreBand_MeanRev_Scalp | KEEP | Statistical family, not pure VWAP |

**Eliminated: 17**

---

### MACD Family: 18+ instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 MACD (10) | -10 | 2 documented losers; lagging on 1m BTC |
| Intraday MACD (8) | -8 | 5m MACD marginally better but unvalidated |
| MACD_VWAP_Flip (removed) | REMOVED | -$10.90 |
| MACD_ZeroCross_Confluence (removed) | REMOVED | -$3.71 |

**Decision:** Retire entire MACD family from 1m. 5m MACD can be reconsidered after 90-day paper validation.  
**Eliminated: 18**

---

### Stochastic Family: 17 instances → 1 survivor

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 Stochastic (12) | -11 | Keep Stoch(14,3) cross only |
| Intraday Stochastic (5) | -4 | Keep Stoch(14,3) 5m only |
| Stochastic_Range_Scalp | KEEP | Has live PnL (+$1.77); retain |

**Eliminated: 15**

---

### CCI Family: 13 instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 CCI (8) | -8 | No live PnL; redundant with RSI on BTC |
| Intraday CCI (5) | -5 | No live PnL; redundant |

**Decision:** Retire CCI family. Mathematically similar to RSI in practice on BTC. No evidence of distinct edge.  
**Eliminated: 13**

---

### Williams %R Family: 8 instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 Williams %R (8) | -8 | Mathematically inverse of Stochastic; redundant |

**Decision:** Retire entirely. No distinct signal from Stochastic.  
**Eliminated: 8**

---

### ROC Family: 8 instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 ROC (8) | -8 | Equivalent to MACD histogram; retire with MACD |

**Eliminated: 8**

---

### Parabolic SAR Family: 8 instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 PSAR+EMA (8) | -8 | PSAR as entry generates whipsaws on 1m BTC |

**Eliminated: 8**

---

### Hull MA Family: 8 instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 Hull MA (8) | -8 | Lagging crossover; no evidence of edge over EMA |

**Eliminated: 8**

---

### ATR Signal Family: 10 instances → 1 survivor

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 ATR (10) | -9 | ATR_Breakout removed (-$15.43); keep ATR_Mom(14,20) only |
| ATR_Volume_Impulse_Scalp (removed) | REMOVED | -$19.65 (worst performer) |

**Keep:** ATR_Momentum as volatility-expansion signal pending validation.  
**Eliminated: 9**

---

### Keltner Family: 12 instances → 1 survivor

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 Keltner (12) | -11 | Keep Kelt_Break(20,14,2) only |
| Intraday Keltner (5) | -5 | Keep Kelt_Break(50,14,2) 5m only |

**Eliminated: 16**

---

### N-bar Breakout Family: 15 instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 N-bar (10) | -10 | Donchian_Breakout removed (-$7.84); N-bar is same mechanism |
| Elite V3 N-bar (5) | -5 | Same mechanism |

**Eliminated: 15**

---

### Triple EMA Family: 16 instances → 1 survivor

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V2 Triple EMA (8) | -7 | Keep Triple(8,21,55) only |
| Intraday Triple EMA (8) | -7 | Keep Triple(5,13,34) 5m only |

**Eliminated: 14**

---

### Momentum Divergence Family: 6 instances → 1 survivor

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 Momentum Divergence (6) | -5 | Keep MomDiv(14,8) only; divergence valid mechanism |

**Eliminated: 5**

---

### Consecutive Candles Family: 8 instances → 0 survivors (retire family)

| Category | Count | Reasoning |
|:---------|------:|:---------|
| Elite V3 Consecutive (8) | -8 | High noise on 1m BTC; retire |

**Eliminated: 8**

---

### Expansion Pack (301 instances → 0 survivors)

**Complete removal.** Zero distinct signal engines. Zero live PnL. Definitive parameter-grid overfit.  
**Eliminated: 301**

---

## Summary: Elimination Results

| Category | Before | After | Eliminated |
|:---------|-------:|------:|----------:|
| EMA crossover | 70+ | 3 | 67 |
| RSI threshold | 43 | 2 | 41 |
| RSI slope | 25 | 1 | 24 |
| Bollinger Bands | 55+ | 3 | 52 |
| VWAP | 27 | 3 | 24 |
| MACD | 18 | 0 | 18 |
| Stochastic | 17 | 1 | 16 |
| CCI | 13 | 0 | 13 |
| Williams %R | 8 | 0 | 8 |
| ROC | 8 | 0 | 8 |
| Parabolic SAR | 8 | 0 | 8 |
| Hull MA | 8 | 0 | 8 |
| ATR signal | 10 | 1 | 9 |
| Keltner | 17 | 2 | 15 |
| N-bar breakout | 15 | 0 | 15 |
| Triple EMA | 16 | 2 | 14 |
| Momentum divergence | 6 | 1 | 5 |
| Consecutive candles | 8 | 0 | 8 |
| Expansion pack | 301 | 0 | 301 |
| **Total** | **577+** | **19** | **558+** |

**Retained without duplication reduction (already unique):**
- Base multi-signal confluence strategies: 10
- Institutional alpha (17, all broken): keep all for fix
- Statistical strategies: 5
- Chart pattern strategies: 5
- Session strategies: 3
- Client desk strategies: 48

---

## Final Count After Duplicate Elimination

| Stack | Before | After Elimination | % Eliminated |
|:------|-------:|:-----------------:|:------------:|
| Go engine | 606 | ~55 | 90.9% |
| Client desk | 108 | 48 (unchanged) | 0% |
| **Total** | **714** | **~103** | **85.6%** |

---

## Strategies Identified for Immediate Retirement

### Priority 1: Remove All (0 exceptions)
1. **301 Expansion Pack (`XP_*`)** — zero edge, 100% parameter inflation
2. **All MACD family** (18) — documented losers, lagging on 1m BTC
3. **All CCI family** (13) — redundant, no evidence of edge
4. **All Williams %R** (8) — mathematically redundant with Stochastic
5. **All ROC family** (8) — equivalent to MACD histogram
6. **All Parabolic SAR** (8) — generates whipsaws on 1m BTC
7. **All Hull MA** (8) — lagging, redundant with EMA
8. **All N-bar Breakout** (15) — breakout family proven losers
9. **All Consecutive Candles** (8) — noise signal on 1m BTC

**Priority 1 total removal: ~389 strategies**

### Priority 2: Reduce to Single Representative (keep 1 per family)
- EMA cross: keep 3
- RSI threshold: keep 2
- RSI slope: keep 1
- Bollinger: keep 3
- VWAP: keep 3
- Stochastic: keep 1
- ATR: keep 1
- Keltner: keep 2
- Triple EMA: keep 2
- Momentum Divergence: keep 1

**Priority 2 total retained after reduction: 19 from ~200**

---

## Phase 3 Verdict

**Original count:** 714 strategies  
**After elimination:** ~103 remaining  
**Eliminated:** ~611 strategies (85.6%)

Of the ~611 eliminations:
- 301 are parameter-grid overfit (expansion pack)
- 200+ are exact-mechanism duplicates differing only by period values
- 110+ are proven loser families or mathematically redundant families

**The platform's 714-strategy universe reduces to ~103 genuinely distinct candidates.**
