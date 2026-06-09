# Strategy Validation Report

**Audit date:** 2026-06-09  
**Scope:** Source-code evidence only  
**Auditor roles:** Principal Quant Architect, Institutional Trading Systems Auditor

---

## Executive Summary

| Stack | Strategy Count (proven) | Overall Verdict |
|-------|------------------------|-----------------|
| Go engine (`BuildCuratedScalpers`) | 606 | **PASS** (pattern-level) with family-specific caveats |
| Next.js paper desk (`FUTURES_STRAT_DEFS`) | 108 (48 live + 60 research) | **PASS** (pattern-level) |
| Mock/research registries | 120 | **FAIL** for capital (not wired to execution) |

**Cross-cutting verdict:** Signal generation logic is provably correct at the pattern level (warmup, crossover-only, cooldown). **FAIL** for end-to-end capital safety because two independent strategy stacks (Go engine vs Next.js paper desk) execute different strategy sets on different runtimes with no code-proven synchronization.

---

## Strategy Inventory

### Go Engine — 606 Live Strategies

**Registry:** `engine/internal/strategy/curated_registry.go`  
**Test proof:** `curated_registry_test.go` asserts `len(BuildCuratedScalpers()) == 606` with unique names.

| Family | Count | Primary file | Entry pattern | SL% / TP% |
|--------|-------|--------------|---------------|-----------|
| Base scalpers | ~35 | `scalpers.go` | EMA cross, RSI, BB, VWAP | 0.15 / 0.25 default |
| Elite V2 | ~95 | `elite_v2.go` | EMA cross + ADX + RSI band | Per-instance (R:R ≥ 1.5) |
| Elite V3 | ~80 | `elite_v3.go` | Stoch, ATR, ROC, PSAR, Hull | Per-instance |
| Intraday 5m/15m | 65 | `intraday_strategies.go` | Wider SL/TP for TF | e.g. 0.22 / 0.55 |
| Institutional alpha | 16 | `alpha_strategies.go` | Funding, CVD, FVG, liquidity | 0.30–0.35 / 0.75–0.85 |
| Expansion pack | 301 | `curated_expansion_pack.go` | XP_EMA_*, XP_RSI_* variants | Per-instance |

**Exit logic:** Strategies attach `StopLossPct` / `TakeProfitPct` to entry signals only. Exits are enforced by `positions.Manager.CheckStopLossAndTakeProfit` (`manager.go:190–258`), not in-strategy.

**Emergency exits:** Kill switch → `KillSwitchExecutor.FlattenPositions` → `ExecuteEmergencyFlatten` (`killswitch_executor.go:50–81`).

**Cooldown:** `aggregator_selective.go` — per-strategy `lastSignal` map vs `cooldownSec`; dominance filter (ratio ≥ 1.10); batch cap 25 signals.

**Risk controls:** Risk V2 Kelly sizing (`risk/v2/kelly.go`); family cap 30% (`risk/v2/limits.go`); PMS portfolio gate (`loop.go:435–452`).

**Capital allocation:** `TargetSize: defaultQty` (0.10 BTC) in signals; scaled by Kelly in institutional path (`loop.go:635–637`).

---

### Next.js Paper Desk — 108 Strategies

**Registry:** `client/src/lib/futuresStrategies.ts`  
**Roster:** 20 CORE (`btcFutureTradingRoster.ts`) + 28 Premium (`btcFtPremiumStrategies.ts`) + 60 research (`futuresCategoryStrategies.ts`).

| Pool | IDs | SL% range | TP% range | Cooldown (min) | Hold (min) |
|------|-----|-----------|-----------|----------------|------------|
| CORE | 91–152 | 0.50–0.55 | 1.50–1.65 | 5–10 | 30–60 |
| Premium | 500–527 | varies | varies | 8–30 | varies |
| Research scalping | 600–659 | 0.35–0.50 | TP:SL ≥ 2.5 | 4–8 | varies |

**Exit logic:** `useBTCFuturesScalperEngine.ts` + `paperResolveHardExit` in `futuresPaperMath.ts` — SL, TP, TIME, LIQUIDATION_RISK, TRAIL, BREAKEVEN, PROFIT_LOCK.

**Capital allocation:** `strategyAllocation.ts` — half-Kelly from win rate; multiplier clamp [0.25, 3.0]; premium 2× notional (`PREMIUM_NOTIONAL_MULTIPLIER`).

---

## Cross-Cutting Validation Matrix

| Check | Go Engine | Paper Desk | Evidence | Verdict |
|-------|-----------|------------|----------|---------|
| A. Signals generate correctly | Yes | Yes | `OnCandle`/`OnTick` return `[]Signal`; `evalMinuteSignal` scores | **PASS** |
| B. Indicators calculate correctly | Yes | Yes | `indicators.go`; `buildSignalInputs` | **PASS** |
| C. Indicator warmup handled | Yes | Yes | `len(prices) < N` gates; `MIN_BARS = 15` in hook | **PASS** |
| D. No lookahead bias | Yes | Yes | `prevFast`/`prevSlow` crossover-only; `idx = closes.length - 1` | **PASS** |
| E. No future candle leakage | Yes | Yes | `GroupByTimeframe()` tick vs candle separation (`registry.go:208`) | **PASS** |
| F. No duplicate signals | Yes | Yes | Aggregator cooldown; `occupied.has(symbol:stratId)` | **PASS** |
| G. No signal race conditions | Partial | Partial | `sync.Mutex` on positions; no proven lock on signal eval in Go parallel group | **FAIL** |
| H. Position state synchronized | Partial | Partial | Single `positions.Manager` in Go; Mongo state in client — separate stores | **FAIL** |

---

## Representative Strategy Records (Pattern Samples)

### Go: `EMA_Cross_Scalp`

| Field | Value |
|-------|-------|
| **Strategy Name** | EMA_Cross_Scalp |
| **Signal Logic** | Fast EMA crosses above slow → BUY; below → SELL |
| **Indicators Used** | EMA(fast), EMA(slow) |
| **Entry** | Crossover with `prevFast`/`prevSlow` |
| **Exit** | SL 0.15%, TP 0.25% via position manager |
| **Warmup** | `len(prices) < slowPeriod+2` → HOLD |
| **Potential Bugs** | Fixed 0.10 BTC size ignores Kelly until institutional path |
| **PASS / FAIL** | **PASS** |

### Go: `ID_EMA5_20_5m`

| Field | Value |
|-------|-------|
| **Strategy Name** | ID_EMA5_20_5m |
| **Signal Logic** | EMA(5,20) cross + ADX≥20 + RSI 44–72 |
| **Indicators Used** | EMA, ADX, RSI |
| **SL/TP** | 0.22% / 0.55% |
| **Potential Bugs** | 5m candle path may use stale tick price for sizing if `SyncEquity` not called |
| **PASS / FAIL** | **PASS** |

### Go: `FundingMeanReversion_Alpha`

| Field | Value |
|-------|-------|
| **Strategy Name** | FundingMeanReversion_Alpha |
| **Signal Logic** | ≥2 confluence votes (CVD/MSS/POC) + optional funding |
| **Quality gate** | `quality.MandatoryPass(score.Score)` or HOLD |
| **SL/TP** | 0.35% / 0.85% |
| **Potential Bugs** | Alpha modules may lack sufficient warmup on cold start |
| **PASS / FAIL** | **PASS** (with warmup caveat) |

### Client: `Trend_Continuation_Long` (ID 91)

| Field | Value |
|-------|-------|
| **Strategy Name** | Trend_Continuation_Long |
| **Signal Logic** | EMA bullish + momentum3>0 + ADX>25 scoring |
| **Indicators Used** | EMA fast/slow, momentum3, ADX proxy, RSI |
| **SL/TP** | 0.50% / 1.50% |
| **Cooldown** | 8 min |
| **Hold** | 40 min TIME exit |
| **Potential Bugs** | Worker vs browser hook may diverge if env flags differ |
| **PASS / FAIL** | **PASS** |

### Client: `PRM_VWAP_REJECT` (Premium)

| Field | Value |
|-------|-------|
| **Strategy Name** | PRM_VWAP_REJECT (template) |
| **Signal Logic** | `evalBtcFtTemplateSignal` VWAP rejection pattern |
| **Capital** | 2× notional multiplier |
| **Potential Bugs** | Premium pool not in Go engine — no live parity |
| **PASS / FAIL** | **PASS** (paper only) |

### Expansion Pack: `XP_EMA_*` (301 strategies)

| Field | Value |
|-------|-------|
| **Signal Logic** | Generated parameter grids via `curated_expansion_pack.go` |
| **Indicators** | EMA cross variants with ADX/RSI guards |
| **Potential Bugs** | High strategy count increases aggregator race window; category cap limits to 5/category |
| **PASS / FAIL** | **PASS** (pattern); **FAIL** for race-condition guarantee |

---

## Full 606-Strategy Enumeration Policy

Enumerating all 606 strategies individually would be redundant — they are factory-generated from ~15 archetypes in `elite_v2.go`, `elite_v3.go`, and `curated_expansion_pack.go`. Each archetype shares:

- Warmup gate (`slow+16` or equivalent)
- Crossover-only signal emission (`prevSet`/`prevAbove`)
- Per-instance SL/TP parameters
- HOLD default when guards fail

**Verdict for all 606:** **PASS** at signal-generation pattern level. **FAIL** for capital deployment without proving execution-path parity (see EXECUTION_TRACE_REPORT.md).

---

## Mock / Research Strategies (Not Capital-Safe)

| Registry | Count | File | Verdict |
|----------|-------|------|---------|
| `BTC_RESEARCH_STRATEGIES` | 100 | `btcResearchStrategyRegistry.ts` | **FAIL** — offline research only |
| `INSTITUTIONAL_STRATEGIES` | 20 | `btcInstitutionalStrategies.ts` | **FAIL** — mock `signal(candles)` only |
| 40 stubs (2060–2099) | 40 | same | **FAIL** — `dataFeedRequired: true`, no signal |

---

## Phase 1 Conclusion

| Dimension | Verdict |
|-----------|---------|
| Signal logic correctness | **PASS** |
| Indicator math | **PASS** |
| Warmup / no-lookahead | **PASS** |
| Duplicate prevention | **PASS** |
| Race-free signal→position | **FAIL** |
| Single source of truth across stacks | **FAIL** |

**Overall Phase 1:** **PASS** for isolated strategy correctness; **FAIL** for institutional capital deployment due to dual-runtime divergence and unproven concurrency safety at 606-strategy scale.
