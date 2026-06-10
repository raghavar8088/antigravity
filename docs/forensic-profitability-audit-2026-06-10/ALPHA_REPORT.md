# PHASE 15 — ALPHA DETECTION

**Generated:** 2026-06-10  
**Verdict:** FAIL — alpha plumbing broken; measurable alpha unproven at scale

---

## Alpha Engine Assessment

### Institutional Alpha Sources (17 Go strategies)

| Alpha Engine | Implemented | Data Available | Actively Trading | PF (synthetic) | Measurable Alpha? |
|:-------------|:-----------:|:--------------:|:----------------:|:--------------:|:-----------------:|
| Market Structure Shift | ✅ | ⚠️ Dispatch bug | ❌ | 2.92 | **FAIL** |
| Funding Mean Reversion | ✅ | ❌ Empty file | ❌ | 2.09 | **FAIL** |
| Order Block | ✅ | ⚠️ Dispatch bug | ❌ | 1.79 | **FAIL** |
| Statistical Mean Reversion | ✅ | ✅ | ✅ | 1.53 | **PARTIAL** |
| Fair Value Gap | ✅ | ⚠️ Dispatch bug | ❌ | 1.48 | **FAIL** |
| Market Profile / POC | ✅ | ⚠️ Dispatch bug | ❌ | 1.19 | **FAIL** |
| Liquidity Sweep | ✅ | ⚠️ Dispatch bug | ❌ | 1.02 | **FAIL** |
| Order Flow (CVD/Delta) | ✅ | ✅ | ⚠️ Partial | 0.91 | **FAIL** |
| Liquidation Cascade | ✅ | ❌ Feed unwired | ❌ | — | **FAIL** |
| Session Expansion | ✅ | ⚠️ Dispatch bug | ❌ | — | **FAIL** |
| Phase 11 (7 alphas) | ✅ | ⚠️ Mixed | ❌ | — | **FAIL** |

**5 of 8 alpha engines pass synthetic PF ≥ 1.30** — but synthetic data is invalid for certification.

**3 of 8 fail even synthetically:** Market Profile (1.19), Liquidity Sweep (1.02), Order Flow (0.91).

---

## Alpha Metrics (Where Available)

### Synthetic Only (Phase 22E — INVALID)

| Alpha Engine | Expectancy | Sharpe | Sortino | Calmar | PF |
|:-------------|:-----------|:-------|:--------|:-------|---:|
| MSS | $57.45 | 4.17 | — | — | 2.92 |
| Funding | $24.11 | 3.48 | — | — | 2.09 |
| Order Block | $33.12 | 2.89 | — | — | 1.79 |
| Stat Mean Rev | $20.96 | 5.40 | — | — | 1.53 |
| FVG | $22.16 | 2.00 | — | — | 1.48 |
| Market Profile | $7.76 | 0.67 | — | — | 1.19 |
| Liquidity Sweep | $0.84 | 0.06 | — | — | 1.02 |
| Order Flow | -$7.14 | -0.42 | — | — | 0.91 |

### Live Evidence (Go Aggregator — PARTIAL)

| Strategy | Live PnL | Alpha Type |
|:---------|:---------|:-----------|
| TripleFilter_Alpha_Scalp | +$20 | Multi-signal (not pure alpha) |
| OrderFlow_Pressure_Pro_Scalp | +$2 | Order flow |
| ZScoreBand_MeanRev_Scalp | +$4.32 | Statistical |
| LinReg_Statistical_Scalp | +$0.56 | Statistical |

**No pure alpha strategy (FVG, MSS, OB, Funding) has positive LIVE PnL evidence.**

---

## Alpha Plumbing Failures

| Failure | Impact | Fix Required |
|:--------|:-------|:-------------|
| OnCandle dispatch bug | 6 alpha modules never evaluate | Route candle alphas through OnCandle in loop |
| `funding.ndjson` empty | Funding MR cannot fire | Populate funding data feed |
| Liquidation feed unwired | Cascade alpha dead | Wire liquidation proxy in main.go |
| Quality gate at 70 | CVD scores ~71 — barely pass | Not a bug but limits throughput |
| Aggregator starvation | Alpha boosted +1.45 but still capped at 25/batch | Increase cap for alpha-only batches |

---

## Does Each Strategy Have Measurable Alpha?

| Universe | Strategies with Proven Alpha | % |
|:---------|:----------------------------:|--:|
| Go 606 | 14 (marginal live PnL) | 2.3% |
| Go 17 alpha | 0 (live) | 0% |
| Go 301 XP | 0 | 0% |
| Client 48 | 1 (portfolio replay positive) | 2.1% |
| Client 108 | 1 (portfolio level) | 0.9% |

---

## Alpha Score

| Dimension | Score /10 | Evidence |
|:----------|:---------:|:---------|
| Theoretical alpha sources | 7 | Funding, CVD, FVG, MSS are real institutional tools |
| Implementation completeness | 2 | 8/17 broken |
| Data pipeline | 1 | Funding empty, liquidation unwired |
| Measured live alpha | 1 | +$20 max on $1M account |
| Portfolio alpha | 1 | Net documented Go PnL is negative |

**Alpha Score: 2/10**

---

## Alpha Verdict

**The platform has theoretical alpha sources but zero proven live alpha from pure alpha engines.**

Profits (where they exist) come from multi-signal confluence scalpers, not from institutional alpha modules. The alpha infrastructure is **architecturally present but operationally dead.**
