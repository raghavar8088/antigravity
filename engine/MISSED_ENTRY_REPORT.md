# MISSED_ENTRY_REPORT.md
## Phase 22D — Missed Entry Analysis

Every signal that never becomes a trade is now classified into a canonical
`RejectReason` and counted with its missed notional (price × size as an
opportunity proxy). Classification: `internal/execintel/missed.go:Classify`.

---

## Rejection taxonomy (code evidence)

| Canonical reason | Mapped from (raw orchestrator strings) | Lifecycle state |
|---|---|---|
| `SignalExpired` | `stale_signal_expired`, "expired" | `Expired` |
| `MinimumSizeRejected` | `size_below_minimum` | `SignalRejected` |
| `SizingRejected` | `execution_weight_below_floor`, "weak execution" | `SignalRejected` |
| `AllocationRejected` | "capital budget", "no capital" | `SignalRejected` |
| `BridgeDelayed` | `parked_for_command_center_bridge` | `SignalRejected` |
| `OMSRejected` | "oms:", "ledger", "replay", "idempotency" | `OrderRejected` |
| `BrokerRejected` | "no market price", "exchange reject" | `OrderRejected` |
| `RiskRejected` | "risk:", "drawdown", "exposure", "kill" | `RiskRejected` |
| `RegimeRejected` | `category_not_aligned_with_regime` | `SignalRejected` |
| `ConfidenceRejected` | "confidence", "reward", "r:r" | `SignalRejected` |
| `PositionLimitRejected` | `position_limit` | `SignalRejected` |
| `AggregatorRejected` | `weak_directional_consensus`, `non_dominant_side`, cooldown, score, throughput, category | `SignalRejected` |
| `OtherRejected` | anything unmatched | `SignalRejected` |

Classification verified by `TestClassifyReasons` (12 cases) and
`TestMissedEntryClassificationAndRanking`.

---

## Wiring (code evidence — every reject site records to execintel)

| Reject site | loop.go line |
|---|---|
| stale / hard-expiry | 997 |
| position limit | 1009 |
| regime filter | 1021 |
| execution weight | 1038 |
| min size | 1047 |
| profit sanitize (confidence/RR) | 1062 |
| risk validate | 1087 |
| bridge parking | 1155 |
| OMS / execution failure | 1176 |

---

## Measured funnel (test harness — `TestEvidenceSnapshotDump`, 200 generated)

```
GeneratedSignals = 200
ApprovedSignals  = 200   (all reached the per-signal loop)
ExecutedSignals  = 100
RejectedSignals  = 100
MissedEntryRate  = 50.0%
```

### Ranked causes (auto-sorted by frequency)

| Rank | Reason | Count | Missed notional (USD) | Share |
|---|---|---|---|---|
| 1 | AggregatorRejected | 40 | 40,080 | 40% |
| 2 | RegimeRejected | 18 | 18,036 | 18% |
| 3 | SignalExpired | 12 | 12,024 | 12% |
| 4 | SizingRejected | 10 | 10,020 | 10% |
| 5 | RiskRejected | 8 | 8,016 | 8% |
| 6 | MinimumSizeRejected | 6 | 6,012 | 6% |
| 7 | BridgeDelayed | 6 | 6,012 | 6% |

> These counts come from the documented synthetic funnel in the evidence test,
> not production. In production the same `RankedCauses` array is served live at
> `GET /api/execution/intelligence` → `missedEntries.rankedCauses`.

---

## Success criterion

| Target | Mechanism | Status |
|---|---|---|
| Missed Entry Rate < 5% | Live-measured; surfaced via `missedEntries.missedEntryRatePct` | **Infrastructure live** — actual rate is a runtime property of production signal flow, now measurable for the first time |
