# BOTTLENECK_RANKING.md
## Phase 22D — Execution Bottleneck Discovery

Bottlenecks are ranked automatically from missed-entry classification by lost
trades and lost notional. Source: `internal/execintel/snapshot.go:rankBottlenecks`.

---

## Ranking algorithm (code evidence)

```go
func rankBottlenecks(rej map[RejectReason]int64, pnl map[RejectReason]float64) []Bottleneck
```
Sorts rejection reasons by `Count` desc, ties broken by `MissedNotional` desc.
Each entry carries `Rank`, `Reason`, `LostTrades`, `LostNotionalUSD`, `SharePct`.

---

## Ranked bottlenecks (test harness — `TestEvidenceSnapshotDump`)

| Rank | Bottleneck | Lost trades | Lost notional (USD) | Share |
|---|---|---|---|---|
| #1 | AggregatorRejected (weak consensus / cooldown / caps) | 40 | 40,080 | 40% |
| #2 | RegimeRejected (category not aligned with regime) | 18 | 18,036 | 18% |
| #3 | SignalExpired (stale before execution) | 12 | 12,024 | 12% |
| #4 | SizingRejected (execution-weight floor) | 10 | 10,020 | 10% |
| #5 | RiskRejected (exposure / drawdown / kill) | 8 | 8,016 | 8% |
| #6 | MinimumSizeRejected | 6 | 6,012 | 6% |
| #7 | BridgeDelayed (parked for Command Center) | 6 | 6,012 | 6% |

Verified by `TestMissedEntryClassificationAndRanking`: the most frequent reason
ranks #1 (`snap.Bottlenecks[0].Reason == RejectExpired` in that test's fixture).

> Live ranking is served at `GET /api/execution/intelligence → bottlenecks` and
> reflects real production signal flow, recomputed on every request.

---

## Interpretation guide

- **AggregatorRejected dominant** → the selective aggregator is the primary
  throughput governor (by design: cooldown, dominance, category cap 5, throughput
  cap 25). Tune `aggregator_selective.go` constants if conversion is starved.
- **RegimeRejected high** → category↔regime alignment matrix (`loop.go:1742`) is
  filtering too aggressively for current market conditions.
- **SignalExpired high** → strategies firing faster than the execution path drains;
  inspect latency report for a slow stage.
- **SizingRejected / MinimumSizeRejected high** → execution-weight floor (0.50) or
  1% capital sizing producing sub-minimum orders.
