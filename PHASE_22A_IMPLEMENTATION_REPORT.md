# Phase 22A Implementation Report

## 1. Files Modified

| File | Purpose |
| --- | --- |
| `engine/cmd/antigravity/main.go` | Removed the 30-strategy startup truncation and load the full curated roster. |
| `engine/internal/trading/aggregator_selective.go` | Raised signal throughput, lowered selective and dominance filters, changed category suppression from 1 to 2 per category, and added funnel telemetry. |
| `engine/internal/trading/loop.go` | Lowered the executable confidence floor and instrumented final execution gates. |
| `engine/internal/trading/signal_flow_metrics.go` | Added cumulative funnel metrics snapshot types and counters. |
| `engine/internal/trading/aggregator.go` | Attached signal-flow metrics to the active aggregator. |
| `engine/internal/trading/aggregator_selective_test.go` | Updated/selectively added tests for 8-signal throughput, two-per-category logic, and rejection metrics. |
| `engine/internal/trading/loop_profit_test.go` | Added coverage for the Phase 22A confidence floor. |
| `engine/internal/strategy/curated_registry.go` | Corrected stale roster wording. |
| `engine/internal/strategy/curated_expansion_pack.go` | Corrected stale expansion-pack wording. |
| `engine/internal/strategy/curated_registry_test.go` | Added guards proving the full 589-entry roster and 301-entry expansion pack are available. |
| `SIGNAL_FLOW_METRICS.md` | Documents the new funnel counters and code-level ceilings. |

## 2. Old Values and New Values

| Area | File | Line(s) | Old Value | New Value |
| --- | --- | ---: | --- | --- |
| Strategy cap | `engine/cmd/antigravity/main.go` | 414-420 | `btcEquityStrategyCount = 30`, slice to first 30 | `btcEquityStrategyCapacity = 600`, no truncation |
| Max approved signals | `engine/internal/trading/aggregator_selective.go` | 14-18 | `2` | `8` |
| Selective score floor | `engine/internal/trading/aggregator_selective.go` | 14 | `1.30` | `1.10` |
| Dominance ratio | `engine/internal/trading/aggregator_selective.go` | 15 | `1.25` | `1.10` |
| Dominance lead | `engine/internal/trading/aggregator_selective.go` | 16 | `0.18` | `0.18` unchanged |
| Category deduplication | `engine/internal/trading/aggregator_selective.go` | 87-127 | One signal per category | Two signals per category |
| Minimum executable confidence | `engine/internal/trading/loop.go` | 36 | `0.82` | `0.74` |
| Final gate telemetry | `engine/internal/trading/loop.go` | 914-1067 | Log-only rejections | Stage counters plus reason/category counters |

## 3. Signals Unlocked

Code-derived unlocks only:

| Bottleneck | Before | After | Exact Unlock |
| --- | ---: | ---: | ---: |
| Active strategy roster | 30 | 589 | +559 strategies |
| Per-batch approvals | 2 | 8 | +6 signals per qualifying batch |
| Per-category approvals | 1 | 2 | +1 signal per category per qualifying batch |
| Startup registry starvation | 559 of 589 excluded | 0 of 589 excluded by startup cap | 94.91 percentage points removed |

The full roster count is validated by `TestBuildCuratedScalpersHasUniqueNamesAndExpectedCount`. The expansion count is validated by `TestBuildExpansionPackExpectedCount`.

## 4. Expected Active Strategies

Current code before Phase 22A:

- Active at startup: `30`.
- Registry available: `589`.
- Expansion pack available but mostly unreachable due to startup truncation: `301`.

Post-Phase 22A:

- Active at startup: `589`.
- Expansion strategies loaded: `301`.
- Startup truncation: removed.

## 5. Expected Trade Frequency

Code-derived maximum before final risk/execution gates:

- Before: at most `2` approved strategy signals per qualifying batch.
- After: at most `8` approved strategy signals per qualifying batch.
- Maximum pre-risk approval throughput increase: `4x`.

Exact trades/day cannot be derived from static code because it depends on runtime tick cadence, strategy firing rate, regime distribution, bridge-online state, risk exposure, fills, and open-position occupancy. Phase 22A adds runtime counters to measure that value without guessing.

## 6. Expected Signal Survival Rate

Code-derived ceilings:

- Startup strategy survival before Phase 22A: `30 / 589 = 5.09%`.
- Startup strategy survival after Phase 22A: `589 / 589 = 100%`.
- If every loaded strategy emitted a signal in one batch, old full-registry approval ceiling was `2 / 589 = 0.34%`.
- If every loaded strategy emitted a signal in one batch after Phase 22A, approval ceiling is `8 / 589 = 1.36%`.
- For a fixed candidate batch above 8 signals, the aggregator cap increases survival by `4x`.

Runtime signal survival is now measured by `SignalAggregator.GetSignalFlowSnapshot()`.

## 7. Profitability Impact Assessment

Exact return, profit factor, and win-rate impacts are not code-derivable without a post-change trade sample. Any numeric annual-return projection from static code would be speculation.

Code evidence does quantify starvation:

- `94.91%` of registry strategies were excluded before signal generation by the startup cap.
- Per-batch approval capacity increased from `2` to `8`, removing a hard `75%` throughput reduction relative to the new cap.
- Same-category suppression now allows two independent strategies per category instead of suppressing every second signal in the same category.

Therefore, the profitability problem demonstrably had a major signal-starvation component. The exact split between signal starvation and strategy quality requires runtime funnel counts plus realized trade outcomes; Phase 22A now captures the funnel side needed to compute that split.

## 8. Profitability Safety Check

Unchanged controls:

- `RiskEngine.Validate` remains the authoritative risk gate.
- `MaxPositionBTC = 2.0` remains configured in `engine/cmd/antigravity/main.go`.
- `MaxCapitalUSD = 1,000,000` remains configured in `engine/cmd/antigravity/main.go`.
- `MaxDailyLossPct = 0.05` remains configured in `engine/cmd/antigravity/main.go`.
- Fixed futures capital per entry remains `1%` of paper capital in `engine/internal/trading/loop.go`.
- `minExecutionWeightToTrade = 0.50` remains unchanged.
- `positions.Manager` still enforces `MaxPerStrategy = 2`.
- OMS v3/event sourcing/security/observability/reconciliation/portfolio management/infrastructure paths were not modified.

The risk package tests passed as part of targeted validation.

## 9. Final Certification

Current state from code before Phase 22A:

- Active strategies: `30`.
- Trades/day: not exactly code-derivable.
- Signal survival: startup `5.09%`; full-registry per-batch approval ceiling `0.34%` if all 589 strategies emit.

Post-Phase 22A state:

- Active strategies: `589`.
- Trades/day: not exactly code-derivable; pre-risk approval capacity is `4x` higher.
- Signal survival: startup `100%`; full-registry per-batch approval ceiling `1.36%` if all 589 strategies emit.

Profitability starvation versus strategy quality:

- Exactly quantified starvation from code: `559 / 589 = 94.91%` of strategies were blocked by startup truncation before signal generation.
- Additional hard aggregator starvation: approval cap was `2` and is now `8`.
- Strategy-quality contribution: not exactly quantifiable from code alone because realized post-filter trades, PnL, and win/loss outcomes are runtime data.

## 10. Validation

Passed:

```bash
go test -mod=mod ./internal/trading ./internal/strategy ./internal/risk ./internal/positions
```

Editor diagnostics reported no linter errors for the changed files.

Known pre-existing validation issue:

```bash
go test ./internal/trading ./internal/strategy ./internal/risk ./internal/positions
```

fails before running tests due to inconsistent vendoring between `go.mod` and `vendor/modules.txt`. Phase 22A did not modify dependency metadata or vendor files.
