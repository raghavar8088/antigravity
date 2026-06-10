# PHASE 4 — ENTRY FORENSICS

**Generated:** 2026-06-10  
**Verdict:** FAIL — proven entry degradation mechanisms; no production fill-level dataset

---

## Data Availability

| Dataset | Status |
|:--------|:------:|
| Production signal→fill timestamp pairs | **FAIL** |
| `slippage_bps` per trade in Mongo | **FAIL** (not accessible) |
| Paper trading slippage measurement | **PASS** (0 bps by design) |
| Stale signal guard | **PASS** (code exists) |
| Missed entry quantification | **PARTIAL** (estimated) |

---

## Entry Pipeline Stages (Go Engine)

```
Signal created (OnTick/OnCandle)
  → [T+0ms] sig.CreatedAt stamped (Phase 22D)
  → FilterSignalsSelective (cooldown 15s, dominance, score)
  → Stale signal guard: signalMaxAge(timeframe)
  → Regime filter
  → sanitizeSignalForProfit (confidence ≥ 0.68)
  → risk.Validate()
  → Kill switch check
  → OMS v3 → exec.ExecuteSignal()
  → Fill at currentPrice (paper = zero slippage)
```

**Evidence:** `EXECUTION_FUNNEL_REPORT.md`, `loop.go`

---

## Entry Delay Mechanisms

### 1. Stale Signal Rejection (FIXED Phase 22D)

| Timeframe | Max Age | Impact |
|:----------|:--------|:-------|
| tick | ~500ms | High sensitivity |
| 1m | ~30s | 30-50% of 1m signals affected pre-fix |
| 5m | ~120s | Moderate |
| 15m | ~300s | Low |

**Evidence:** `MISSED_ENTRY_REPORT.md:14-17` — "30-50% of 1m signals were potentially executing outside their window."

**Quantified impact (estimated):** Converts 55% WR strategy → ~45% WR from timing alone.

**Status:** Guard exists post-22D. Production log quantification **not available** in audit environment.

### 2. Aggregator Batch Delay

606 strategies evaluated in parallel → selective filter → ≤25 approved. Strategies ranked lower may wait for next candle close.

**Estimated delay:** 0–60s (1m strategies miss optimal entry within candle).

### 3. Bridge Parking (AI Command Center)

Signals for non-trusted strategies parked in `pendingSignals` map.

**Bounded:** 5 min max, 45s offline fallback (`MISSED_ENTRY_REPORT.md:30-32`).

**Trusted bypass:** `isTrustedStrategy()` — proven winners with confidence ≥ 0.80 skip queue (`loop.go:2237+`).

### 4. Kill Switch Block (2026-06-08 → 2026-06-10)

**Entry delay:** ∞ (no fills permitted).

**Evidence:** `ROOT_CAUSE_ANALYSIS.md` — 100% entry failure during outage.

---

## Entry Slippage

| Context | Measured Slippage | Evidence |
|:--------|------------------:|:---------|
| Go paper trading | **0 bps** | `SLIPPAGE_ANALYSIS_REPORT.md:44` — ExecPrice = currentPrice |
| Client replay (5 bps configured) | **5 bps applied** | `npm run replay` config |
| Live (expected) | 1–20 bps | `SLIPPAGE_ANALYSIS_REPORT.md:50-55` |

**Impact at 1m tick scalping:** 1 bps slippage can erase ~30% of TP on 0.10% targets (`SLIPPAGE_ANALYSIS_REPORT.md:60`).

**Verdict:** Entry slippage not destroying paper profits (zero slippage). **Will destroy live edge** on tight-SL strategies.

---

## TP Distortion at Entry (Pre-Fix)

`sanitizeSignalForProfit()` previously inflated TP to 2.4× SL even when strategy set explicit TP.

**Post-fix:** Explicit strategy TPs preserved (`loop.go:2214-2225`).

**Quantified damage (pre-fix):** ~67% of signals affected. 15 additional losers per 100 trades × $150 = **-$2,250/100 trades** (`MISSED_ENTRY_REPORT.md:24-28`).

---

## Client Entry Path

```
evalMinuteSignal() → passesEntryConfirmation() → paperOms create order
```

| Gate | Threshold | Entry Impact |
|:-----|:----------|:-------------|
| Signal score | ≥ 26 (replay default) | Filters weak setups |
| Confluence min | 5–6 per strategy | Delays marginal entries |
| Cooldown | 5–10 min | Prevents re-entry |
| Session gate | Env-driven | Blocks off-hours |
| Slippage model | 5 bps (replay) | Worsens fill vs signal price |

**Replay first trade evidence:**
- Strategy: PRM_RangeBreak_Short (523)
- Signal bar → entry next bar (1 min delay inherent to bar-close model)
- Entry: $100,881.09 → Exit SL: $100,928.59 → Net: -$0.09

---

## Profitable Setups Entered Correctly?

| Stack | Answer | Evidence |
|:------|:-------|:---------|
| Go engine | **FAIL — cannot verify** | No signal/fill timestamp dataset |
| Go engine (timing) | **FAIL — pre-22D** | Stale execution documented |
| Go engine (outage) | **FAIL** | Zero entries during kill switch |
| Client replay | **PARTIAL** | 65.5% PROFIT_LOCK exits suggest entries adequate on sample |

---

## Entry Forensics Summary

| Metric | Go Engine | Client Desk |
|:-------|:----------|:------------|
| Entry delay | Unmeasured (guards exist) | 1 bar (replay model) |
| Entry slippage | 0 bps (paper) | 5 bps (replay) |
| Missed entries | Estimated 30-50% pre-22D | Not measured |
| Late entries | Aggregator + pipeline latency | Cooldown blocks |
| Premature entries | Not evidenced | Not evidenced |

**Root cause rank (entry):** Stale execution > aggregator starvation > kill switch outage > live slippage (future).
