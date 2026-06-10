# PHASE 2 — STRATEGY REACHABILITY

**Generated:** 2026-06-10  
**Verdict:** FAIL — mass signal starvation; alpha plumbing broken; dual-stack desync

---

## Reachability Matrix

| Category | Count | Signal Gen | Aggregator | Risk Gate | OMS Fill | Status |
|:---------|------:|:----------:|:----------:|:---------:|:--------:|:-------|
| Go curated (all 606) | 606 | ✅ Wired | ⚠️ ≤25/batch | ✅ | ⚠️ Kill-switch risk | **PARTIALLY WIRED** |
| Go expansion `XP_*` | 301 | ✅ | ⚠️ Low priority | ✅ | ⚠️ | **REACHABLE BUT STARVED** |
| Go institutional alpha | 17 | ⚠️ 6 broken dispatch | ⚠️ Boosted +1.45 | ✅ | ⚠️ | **PARTIALLY WIRED** |
| Go removed losers | 11 | ❌ | ❌ | ❌ | ❌ | **DEAD (registry)** |
| Go research files | 4 files | ❌ Not registered | ❌ | ❌ | ❌ | **DEAD CODE** |
| Client CORE 20 | 20 | ✅ | N/A (desk policy) | ✅ | ✅ Worker/browser | **FULLY WIRED** |
| Client Premium 28 | 28 | ✅ | N/A | ✅ | ✅ | **FULLY WIRED** |
| Client Research 60 | 60 | ✅ Code exists | ❌ Worker excludes | ✅ | ❌ Default | **DISABLED** |
| Mock research 500 | 500 | ✅ Mock only | ❌ | ❌ | ❌ | **UNREACHABLE** |
| BTC research 100 | 100 | ✅ UI only | ❌ | ❌ | ❌ | **UNREACHABLE** |

---

## Go Engine Execution Path

```
BuildCuratedScalpers() [606]
  → NewOrchestrator (main.go:433-449)
    → GroupByTimeframe (tick/1m/5m/15m/1h)
      → processStrategyGroup → Strategy.OnTick()
        → FilterSignalsSelective (max 25 approved)
          → regime filter, confidence ≥ 0.68, risk.Validate()
            → kill switch check
              → OMS v3 → exec.ExecuteSignal()
```

**Evidence:** All 606 are wired to signal generation. Execution is throughput-gated, not registry-gated.

### Aggregator Starvation Constants

| Gate | Value | Effect | File |
|:-----|------:|:-------|:-----|
| `minSelectiveScore` | 0.80 | Blocks low-confidence strategies | `aggregator_selective.go:29` |
| `maxApprovedSignals` | 25 | ≤3.7% of 606 can execute per batch | `aggregator_selective.go:32` |
| `maxApprovedPerCategory` | 5 | Category concentration cap | `aggregator_selective.go:33` |
| `minDominanceRatio` | 1.10 | Blocks ambiguous direction | `aggregator_selective.go:30` |
| Cooldown | 15s/strategy | Rate limit | `aggregator_selective.go:58-64` |
| `minExecutableConfidence` | 0.68 | Post-aggregator block | `loop.go:42` |

**Throughput math:** 606 strategies → ≤25 approved = **≤4.1% reachability per candle close**.

---

## Strategies That Never Generate Trades

### Alpha Modules — OnCandle Dispatch Bug

| Strategy | Module | Dispatch Required | Actual Dispatch | Status |
|:---------|:-------|:------------------|:----------------|:-------|
| LiquiditySweepReversal_Alpha | `alpha/liquidity/` | OnCandle | OnTick only | **NEVER TRADES** |
| FVGRetest_Alpha | `alpha/fvg/` | OnCandle | OnTick only | **NEVER TRADES** |
| OrderBlockRetest_Alpha | `alpha/orderblock/` | OnCandle | OnTick only | **NEVER TRADES** |
| MSSContinuation_Alpha | `alpha/mss/` | OnCandle | OnTick only | **NEVER TRADES** |
| POCBounce_Alpha | `alpha/poc/` | OnCandle | OnTick only | **NEVER TRADES** |
| SessionExpansion_Alpha | `alpha/session/` | OnCandle | OnTick only | **NEVER TRADES** |

**Evidence:** `STRATEGY_QUALITY_TABLE.md` Part I — "OnCandle ONLY — dispatch bug".

### Data-Blocked Alphas

| Strategy | Missing Data | Evidence |
|:---------|:-------------|:---------|
| FundingMeanReversion_Alpha | `data/alpha/funding.ndjson` empty | `STRATEGY_QUALITY_TABLE.md:29` |
| LiquidationCascade_Alpha | Liquidation feed not wired in `main.go` | `STRATEGY_QUALITY_TABLE.md:35` |

### Working Alpha (OnTick)

| Strategy | Status |
|:---------|:-------|
| CVDDivergence_Alpha | PARTIAL — quality gate 70, scores ~71 |
| DeltaAbsorption_Alpha | YES — passes quality gate |

---

## Runtime Disable (Strategy Tracker)

`engine/internal/risk/strategy_tracker.go` — checked at `loop.go:1384`:

| Trigger | Threshold | Effect |
|:--------|:----------|:-------|
| Consecutive losses | 5 | 10-min cooldown |
| Daily loss limit | 5% of per-strategy capital | Disabled |
| Poor performance | ≥6 trades, negative PnL, WR < 35% | Disabled |

**Impact:** Strategies with early losing streak never recover reachability.

---

## Client Desk Reachability

| Mode | Active IDs | Mechanism |
|:-----|:-----------|:----------|
| Default production | 48 | `CORE_BTC_FT_STRATEGY_IDS` in `btcFtRoster.ts` |
| Research mode | 108 | `NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1` |
| Worker default | 48 | Excludes `researchOnly` IDs ≥ 600 |
| Winners only | Subset | `NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1` + Mongo promotion |

### Client Strategies Never Executing (Default)

- All 60 research pool (600–659): `researchOnly: true`, worker exclusion
- 100 stub category slots (660–759): empty arrays in `futuresCategoryStrategies.ts`

---

## Kill Switch — System-Wide Block (2026-06-08 → 2026-06-10)

**Effect:** ALL 606 Go strategies blocked at `PreTradeRiskPipeline` when kill switch active.

**Evidence:** `ROOT_CAUSE_ANALYSIS.md` — reconciliation v2 false CRITICAL drift → `OMS_DESYNC` → zero fills.

**Conclusion:** Reachability was **0%** for entire Go stack during outage regardless of signal quality.

---

## Reachability Verdict

| Question | Answer | Evidence |
|:---------|:-------|:---------|
| Which strategies execute? | ≤25 Go + 48 Client per cycle | Aggregator cap + roster |
| Which are dead code? | 4 Go files + 11 removed + 500 mock + 100 research UI | Registry analysis |
| Which are partially wired? | 6 alpha OnCandle + 2 data-blocked alphas | Quality table |
| Which never generate trades? | ~8 alpha strategies + 301 XP (starved) | Dispatch bug + aggregator |
| Which are disabled? | Runtime tracker + 60 client research | Tracker + worker filter |

**Overall: FAIL** — majority of defined strategies are either unreachable, starved, or unproven.
