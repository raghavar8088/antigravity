# PHASE 19 — REMEDIATION PLAN

**Generated:** 2026-06-10  
**Priority:** Ranked by expected return improvement

---

## Immediate Actions (Week 1)

### R1. Retire 301 Expansion Pack Strategies

| Field | Value |
|:------|:------|
| **Root cause** | Parameter-grid overfitting with zero OOS validation |
| **Evidence** | `curated_expansion_pack.go` nested loops; `STRATEGY_QUALITY_TABLE.md` overfitting 10/10 |
| **Fix** | Remove `buildExpansionPack()` from `BuildCuratedScalpers()` |
| **Expected improvement** | Eliminate ~50% of false signals; reduce aggregator noise |
| **Severity** | **CRITICAL** |

### R2. Fix Alpha OnCandle Dispatch Bug

| Field | Value |
|:------|:------|
| **Root cause** | `loop.go` calls `OnTick()` for all strategies; 6 alphas require `OnCandle()` |
| **Evidence** | `STRATEGY_QUALITY_TABLE.md` Part I |
| **Fix** | In `processStrategyGroup`, route candle-timeframe strategies through candle buffer before evaluation |
| **Affected** | FVG, MSS, OB, POC, Session, Liquidity Sweep (6+ strategies) |
| **Expected improvement** | Unlock highest-theoretical-edge alpha sources |
| **Severity** | **CRITICAL** |

### R3. Populate Funding Data Feed

| Field | Value |
|:------|:------|
| **Root cause** | `data/alpha/funding.ndjson` empty |
| **Evidence** | `STRATEGY_QUALITY_TABLE.md:29` |
| **Fix** | Wire Binance/Delta funding rate polling into `engine/data/alpha/funding.ndjson` |
| **Expected improvement** | Enable FundingMeanReversion_Alpha (highest theoretical edge) |
| **Severity** | **HIGH** |

### R4. Raise Go Engine Minimum SL to ATR-Scaled

| Field | Value |
|:------|:------|
| **Root cause** | `sanitizeSignalForProfit` sets SL 0.10-0.20% inside 1m BTC noise |
| **Evidence** | `STOPLOSS_REPORT.md`; 1m ATR ~0.5% |
| **Fix** | `minSL = max(0.40%, 0.8 × ATR%)` in `sanitizeSignalForProfit` |
| **Expected improvement** | Reduce noise-stop losses by est. 30-50% |
| **Severity** | **CRITICAL** |

### R5. Retire Active Borderline Losers

| Field | Value |
|:------|:------|
| **Root cause** | 4 strategies with documented negative live PnL still active |
| **Evidence** | `aggregator_selective.go:237-246` |
| **Fix** | Remove from `BuildCuratedScalpers()`: RSI_MACD_Divergence, TripleTrend_Confluence, SessionOpen_Momentum, VWAP_RSI2_Reversion, VWAP_Bounce_Pro |
| **Expected improvement** | +$7.38 documented + prevent future losses |
| **Severity** | **MEDIUM** |

---

## Short-Term Actions (Week 2-4)

### R6. Unify Strategy Stacks (Go + Client)

| Root cause | Dual independent stacks trading same asset |
| Evidence | `STRATEGY_VALIDATION_REPORT.md` — zero name overlap |
| Fix | Single strategy registry; client becomes UI layer over Go engine |
| Expected improvement | Eliminate PnL attribution confusion; single source of truth |

### R7. Export and Analyze MongoDB Production Trades

| Root cause | 592 strategies have zero performance data |
| Evidence | This audit — no accessible `paper_trades` |
| Fix | Run `db.paper_trades.aggregate` per strategy; populate `strategy_scores` |
| Expected improvement | Enable data-driven ranking for all 606 strategies |

### R8. Replace Phase 22E Synthetic Certification

| Root cause | `syntheticTrades()` generates fake certification |
| Evidence | `phase22e_test.go:39`; Monte Carlo p5=p50=p95 |
| Fix | Re-run Phase 22E against real MongoDB trade exports |
| Expected improvement | Valid go-live decisions |

### R9. Add Funding Cost to Go Engine PnL

| Root cause | Go close path ignores funding |
| Evidence | `PNL_VALIDATION_REPORT.md:62-64` |
| Fix | Wire `applyFundingAccrual` equivalent in `processCloseEvents` |
| Expected improvement | Accurate PnL → correct Kelly sizing |

### R10. Wire Liquidation Feed

| Root cause | `LiquidationCascade_Alpha` has no data |
| Evidence | `STRATEGY_QUALITY_TABLE.md:35` |
| Fix | Connect liquidation proxy in `main.go` |
| Expected improvement | Enable cascade reversal alpha |

---

## Medium-Term Actions (Month 2-3)

### R11. Retire Tier 4 Indicator Families

| Families | Count | Evidence |
|:---------|------:|:---------|
| MACD | 10 | Composite 1.85 |
| CCI | 8 | Composite 1.85 |
| Williams %R | 8 | Composite 1.85 |
| Consecutive Candles | 8 | Composite 1.90 |
| ROC | 8 | Composite 2.45 |
| Hull MA | 8 | EMA variant, no edge |

**Total retirement:** ~50 additional strategies

### R12. Implement Regime-Aware Strategy Gating

| Root cause | 99.6% of trades in VOLATILE regime only |
| Evidence | `REGIME_PERFORMANCE_REPORT.md` |
| Fix | Classify regime at entry; disable misaligned families |
| Expected improvement | Reduce RANGE-regime losses (PF 0.83) |

### R13. Run 90/180/365-Day Walk-Forward Replay

| Root cause | Only 8.3-hour replay sample exists |
| Evidence | `REPLAY_REPORT.md` |
| Fix | `npm run replay-fetch` + walk-forward on 48 client strategies; Go V3 backtest on Tier A |
| Expected improvement | Statistical significance for remaining strategies |

### R14. Size by Validated Edge (Not Fixed 0.10 BTC)

| Root cause | Kelly data-starved; equal weight on all |
| Evidence | `POSITION_SIZING_REPORT.md` |
| Fix | Zero size for unvalidated; Kelly proportional to validated PF |
| Expected improvement | Capital flows to winners |

### R15. Add TIME Exit to Go Engine

| Root cause | Go positions have no time-based exit |
| Evidence | `EXIT_FORENSICS_REPORT.md` |
| Fix | Add `holdMinutes` to Go position manager |
| Expected improvement | Align Go/client lifecycle; reduce stale position risk |

---

## Per-Failing-Strategy Remediation Summary

| Strategy Group | Count | Root Cause | Fix | Priority |
|:---------------|------:|:-----------|:----|:--------:|
| XP_* expansion | 301 | Overfit grid | Retire all | P0 |
| Alpha OnCandle | 6 | Dispatch bug | Fix loop.go routing | P0 |
| Alpha data-blocked | 2 | Empty feeds | Wire funding + liquidation | P0 |
| Tight SL scalpers | ~500 | 0.10-0.20% SL in noise | ATR-scale SL | P0 |
| Breakout losers | 11 | False breakouts + tight SL | Already removed; don't re-add | P1 |
| MACD/CCI/Williams | 26 | Redundant indicators | Retire families | P1 |
| Borderline losers | 5 | Weak edge on 1m BTC | Remove from registry | P1 |
| Client research 60 | 60 | Never validated | Keep disabled until OOS pass | P2 |
| EMA cross elite | 55 | Crowded, no edge | Reduce to 3-5 validated pairs | P2 |

---

## Expected Cumulative Improvement

| Action | Est. Impact on Portfolio Expectancy |
|:-------|:------------------------------------|
| Retire 301 XP + 50 tier-4 | Eliminate ~60% of losing signals |
| Fix alpha plumbing (3 fixes) | Unlock 5-8 uncorrelated alpha sources |
| ATR-scale SL | +30-50% win rate on remaining strategies |
| Data-driven ranking | Allocate capital to top 15 strategies only |
| Unified stack | Eliminate dual-PnL confusion |

**Conservative estimate:** If top 15 strategies average +$5/trade at 50 trades/day = +$250/day on $1M = **9% annualized** (still below institutional hurdle of 15-20%).
