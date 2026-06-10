# PHASE 8 — MARKET REGIME ANALYSIS

**Generated:** 2026-06-10  
**Verdict:** FAIL — single-regime dominance; cross-regime stability unproven

---

## Regime Classification Methods

### Phase 22E (Synthetic Data)

Rolling 20-trade window (`REGIME_PERFORMANCE_REPORT.md`):
- **BULL:** Cumulative return > +5%, low vol
- **BEAR:** Cumulative return < -5%, low vol
- **RANGE:** -5% to +5%, low vol
- **VOLATILE:** ATR% > 2.5%

### Go Engine Runtime

`isCategoryAlignedWithRegime()` in `loop.go` — blocks misaligned category×regime signals.

### Client Desk

`futuresDeskPolicy.ts` — chop-disable for mean reversion, session gates, regime annotations.

---

## Portfolio Performance by Regime (Phase 22E Synthetic)

| Regime | Trades | Win Rate | PF | Expectancy | Net PnL |
|:-------|-------:|:--------:|---:|:----------:|--------:|
| BULL | 0 | — | — | — | — |
| BEAR | 0 | — | — | — | — |
| RANGE | 5 | 40.0% | 0.83 | -$8.43 | -$42.14 |
| VOLATILE | 1245 | 53.3% | 1.49 | +$20.57 | +$25,615.22 |

**Critical finding:** 99.6% of synthetic trades occurred in VOLATILE regime only. BULL and BEAR have **zero trades**.

**Go-live warning (confirmed):** `GO_LIVE_READINESS_REPORT.md:36-38` — cross-regime stability unconfirmed.

---

## Strategy Regime Stability (Synthetic)

| Strategy | Stable Regimes (of 4) | Assessment |
|:---------|:---------------------:|:-----------|
| MSS Continuation | 1/4 | **FAIL** — volatile only |
| Funding Rate Arb | 1/4 | **FAIL** |
| Bollinger Squeeze BTC | 1/4 | **FAIL** |
| RSI Oversold 30 Revert | 1/4 | **FAIL** |
| Order Block Rejection | 1/4 | **FAIL** |
| EMA Cross 21/50 BTC | 1/4 | **FAIL** (RANGE PF 0.83) |
| Delta Absorption | 0/4 | **FAIL** — no profitable regime |

**Requirement:** ≥ 2 of 4 regimes for full capital allocation. **0 of 12 synthetic strategies qualify.**

---

## Family Regime Mismatch (Code Analysis)

| Family | Best Regime | Worst Regime | Evidence |
|:-------|:------------|:-------------|:---------|
| EMA Cross (15+301) | TREND | RANGE/CHOP | `STRATEGY_QUALITY_TABLE.md` — robustness 3/10 |
| RSI/BB Mean Reversion (33+) | RANGE | TREND | Chop-disable exists in client, not Go |
| Breakout (18+) | VOLATILE/TREND | RANGE | 5 removed losers were breakout |
| Alpha microstructure (17) | VOLATILE | LOW VOL | Requires sweep/FVG events |
| MACD (10) | TREND | ALL | Composite 1.85 — weakest family |
| VWAP (10) | RANGE | TREND | Weak edge — reference price not alpha |

---

## Client Desk Regime Handling

| Feature | File | Effect |
|:--------|:-----|:-------|
| Chop disable for MR | `futuresDeskPolicy.ts` | Blocks mean reversion in chop |
| Session window gate | `futuresDeskPolicy.ts` | Time-of-day filtering |
| `regime` annotation | `buildPaperDeskStrategies()` | Tags strategies with regime |
| MTF requirements | `requiresHtf: true` on 8 strategies | HTF regime alignment |

**Client has regime gates. Go engine regime filter exists but 606 strategies still generate in all regimes — only post-signal filter applies.**

---

## Regime Analysis Verdict

| Question | Answer |
|:---------|:-------|
| Which strategies perform in each regime? | **FAIL** — only VOLATILE tested (synthetic) |
| Which strategies fail? | Delta Absorption (all regimes); RANGE PF 0.83 portfolio |
| Regime mismatch destroying profits? | **YES** — EMA/MACD deployed in all regimes without adaptation |
| Cross-regime stability? | **FAIL** — 0/12 pass 2/4 regime test |

**Production requirement:** Classify all `paper_trades` by regime at entry using ATR%/trend metrics. Recompute PF per strategy×regime.
