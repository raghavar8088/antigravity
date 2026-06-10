# PHASE 7 — TAKE PROFIT AUDIT

**Generated:** 2026-06-10  
**Verdict:** PARTIAL — TP geometry known; capture efficiency unmeasured

---

## Take Profit Configuration

### Go Engine

| Layer | TP% Range | R:R Rule |
|:------|:----------|:---------|
| Strategy-defined | 0.25–0.85% | Per strategy |
| `sanitizeSignalForProfit` (TP=0) | min 0.50%, then 2.4× SL | `loop.go:2216-2224` |
| `sanitizeSignalForProfit` (explicit TP) | Preserved (post-22D) | `loop.go:2214` |
| Absolute TP floor | 0.10% minimum | `loop.go:2229` |

**Pre-22D damage:** TP inflation on explicit targets cut hit rate ~15% (`MISSED_ENTRY_REPORT.md:26-28`).

### Client Desk

| Pool | TP% | TP:SL Ratio |
|:-----|----:|:-----------:|
| CORE 20 | 1.50–1.65% | ~3.0× |
| Premium | varies | ≥ 2.0 enforced |
| Research | TP:SL ≥ 2.5 | `futuresCategoryStrategies.ts` |

**Client TP is 5–10× wider than Go sanitized TP** — different profit capture philosophy.

---

## Profit Target Hit Rates

| Source | TP Hits | TP Misses | Status |
|:-------|--------:|----------:|:------:|
| Go production | **UNKNOWN** | **UNKNOWN** | **FAIL** |
| Client replay | 0 raw TP | 74 PROFIT_LOCK | **PARTIAL** |
| Client replay SL | — | 38 SL (targets missed) | **PARTIAL** |

**Client replay:** No trades exited via raw `TP` reason. All winning exits via `PROFIT_LOCK` (65.5%) — profit targets are **partially captured** before full TP.

---

## Captured vs Missed Profit

| Metric | Available? | Value |
|:-------|:----------:|:------|
| Captured profit % (net/MFE) | **FAIL** | Requires MFE data |
| Missed profit % (MFE - net)/MFE | **FAIL** | Requires MFE data |
| Avg unrealized gain before exit | **FAIL** | Not in local data |
| PROFIT_LOCK vs full TP delta | **FAIL** | Not computed |

---

## TP Distortion Impact (Quantified, Pre-Fix)

For strategy with TP=0.30%, SL=0.15% (R:R=2.0):
- Inflated TP = 0.36% (20% further travel required)
- Expected hit rate reduction: ~15%
- PnL impact: -$2,250 per 100 trades (`MISSED_ENTRY_REPORT.md`)

**Status:** Fixed in Phase 22D for explicit TPs. Residual risk for TP=0 strategies still inflated to 2.4× SL.

---

## Take Profit vs Stop Loss Geometry Comparison

| Stack | Typical SL | Typical TP | R:R | Fee Impact (0.10% round trip) |
|:------|:-----------|:-----------|:----:|:-----------------------------:|
| Go sanitized | 0.15% | 0.36% (inflated) | 2.4 | Fees = 67% of SL — **unviable** |
| Go alpha | 0.35% | 0.85% | 2.4 | Fees = 29% of SL — marginal |
| Client CORE | 0.50% | 1.50% | 3.0 | Fees = 20% of SL — viable |

**Root cause:** Go tight geometry makes fees dominate expected value.

---

## Take Profit Verdict

| Question | Answer |
|:---------|:-------|
| Profit targets hit? | **UNPROVEN** (Go); **PARTIAL** (Client uses PROFIT_LOCK) |
| Profit targets missed? | 33.6% hit SL instead (client replay) |
| Captured profit % measurable? | **FAIL** |
| TP system destroying profitability? | **YES (Go)** — tight TP + fees + noise SL |
