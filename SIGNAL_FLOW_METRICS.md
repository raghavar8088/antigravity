# Signal Flow Metrics

Phase 22A adds in-engine funnel telemetry for the active BTC futures strategy path:

`strategy.OnTick` -> `FilterSignalsSelective` -> regime filter -> execution-weight filter -> confidence filter -> risk engine -> execution.

## Instrumented Stages

| Stage | Code Source | Input | Output | Rejection Count | Rejection % |
| --- | --- | ---: | ---: | ---: | ---: |
| Generated | `engine/internal/trading/aggregator_selective.go` | Runtime `rawSignals` count | Runtime `rawSignals` count | Runtime | Runtime |
| Aggregator | `engine/internal/trading/aggregator_selective.go` | Runtime `rawSignals` count | Runtime approved count | Runtime | Runtime |
| Signal Cooldown | `engine/internal/trading/aggregator_selective.go` | Runtime `rawSignals` count | Runtime cooldown-eligible count | Runtime | Runtime |
| Dominance Filter | `engine/internal/trading/aggregator_selective.go` | Runtime eligible count | Runtime dominant-side count | Runtime | Runtime |
| Score Filter | `engine/internal/trading/aggregator_selective.go` | Runtime dominant-side count | Runtime score-passing count | Runtime | Runtime |
| Category Deduplication | `engine/internal/trading/aggregator_selective.go` | Runtime score-passing count | Runtime category-cap-passing count | Runtime | Runtime |
| Throughput Cap | `engine/internal/trading/aggregator_selective.go` | Runtime category-cap-passing count | Runtime approved count | Runtime | Runtime |
| Regime Filter | `engine/internal/trading/loop.go` | Runtime approved count | Runtime regime-aligned count | Runtime | Runtime |
| Execution Weight Filter | `engine/internal/trading/loop.go` | Runtime regime-aligned count | Runtime quality-weight-passing count | Runtime | Runtime |
| Confidence Filter | `engine/internal/trading/loop.go` | Runtime quality-weight-passing count | Runtime confidence-passing count | Runtime | Runtime |
| Risk Filter | `engine/internal/trading/loop.go` | Runtime confidence-passing count | Runtime risk-approved count | Runtime | Runtime |
| Execution | `engine/internal/trading/loop.go` | Runtime risk-approved count | Runtime filled count | Runtime | Runtime |

Runtime counters are exposed through `SignalAggregator.GetSignalFlowSnapshot()`, returning:

- `Stages`: ordered stage-level input/output/rejection counts and rejection percentage.
- `RejectedByReason`: rejection reason counts such as `category_batch_cap`, `batch_approval_cap`, `score_below_selective_floor`, `category_not_aligned_with_regime`, and risk errors.
- `RejectedByCategory`: category-level rejection counts for the category deduplication audit.

## Code-Level Funnel Ceilings

| Funnel Point | Before Phase 22A | After Phase 22A | Code Evidence |
| --- | ---: | ---: | --- |
| Strategy loading cap | 30 strategies | No truncation; 600 capacity marker | `engine/cmd/antigravity/main.go` |
| Curated runtime roster | 589 strategies available in registry | 589 strategies loaded | `engine/internal/strategy/curated_registry_test.go` |
| Expansion pack | 301 strategies available | 301 strategies loaded | `engine/internal/strategy/curated_registry_test.go` |
| Max approved signals per batch | 2 | 8 | `engine/internal/trading/aggregator_selective.go` |
| Max approved per category per batch | 1 | 2 | `engine/internal/trading/aggregator_selective.go` |
| Selective score floor | 1.30 | 1.10 | `engine/internal/trading/aggregator_selective.go` |
| Dominance ratio | 1.25 | 1.10 | `engine/internal/trading/aggregator_selective.go` |
| Minimum executable confidence | 0.82 | 0.74 | `engine/internal/trading/loop.go` |

## Rejection Reasons Now Captured

- `Signal Cooldown: strategy_cooldown`
- `Dominance Filter: weak_directional_consensus`
- `Dominance Filter: non_dominant_side`
- `Score Filter: score_below_selective_floor`
- `Category Deduplication: category_batch_cap`
- `Throughput Cap: batch_approval_cap`
- `Regime Filter: category_not_aligned_with_regime`
- `Execution Weight Filter: execution_weight_below_floor`
- `Confidence Filter: <sanitizeSignalForProfit reason>`
- `Risk Filter: <RiskEngine.Validate error>`
- `Execution: position_limit`
- `Execution: size_below_minimum`
- `Execution: parked_for_command_center_bridge`
- `Execution: <execution error>`

## Validation

Targeted validation passed with:

```bash
go test -mod=mod ./internal/trading ./internal/strategy ./internal/risk ./internal/positions
```

The plain `go test` command without `-mod=mod` currently fails before tests run because `go.mod` and `vendor/modules.txt` are already inconsistent. Phase 22A did not modify vendor files.
