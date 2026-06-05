# SLIPPAGE_ANALYSIS_REPORT.md
## Phase 22D — Slippage Intelligence

**Method:** Slippage is now captured at fill time from the paper desk's deterministic
execution model and attributed across six dimensions. Numbers below are the
**deterministic model values** (from code) plus **test-harness measurements**
(from `internal/execintel/evidence_test.go`). No production runtime is claimed —
there is no live trading session in this audit.

---

## How slippage is captured (code evidence)

- `PaperClient.executionPrice` (paper.go:54-80) applies a fixed, mode-dependent
  execution offset to the mark price:

  | Order mode | BUY offset | SELL offset | Adverse slippage |
  |---|---|---|---|
  | `MARKET` (default) | ×1.0001 | ×0.9999 | **+1.00 bps** |
  | `IOC` | ×1.00012 | ×0.99988 | **+1.20 bps** |
  | `POST_ONLY` (maker) | ×0.99995 | ×1.00005 | **−0.50 bps** (price improvement) |

- `ExecuteSignal` (paper.go:127) now records `RequestedPrice` (mark at submit) and
  `SlippageBps = signedSlippageBps(requested, filled, action)` into `FillResult`
  (routing.go:15). Positive = adverse for the trade's direction.

- `loop.go:1195` forwards each fill to `execIntel.RecordSlippage` with attribution
  by **strategy, alpha source, session (UTC-derived), regime, and direction**.

- `slippage.go` aggregates per dimension: count, avg bps, median bps, worst bps,
  total USD cost, avg % move.

---

## Measured results (test harness — 100 MARKET BUY fills)

From `TestEvidenceSnapshotDump` (`go test ./internal/execintel -run Evidence -v`):

```
Slippage avg = 1.000 bps   median = 1.000 bps   worst = 1.000 bps
```

This matches the MARKET model exactly (1.00 bps), confirming the capture path is
faithful end-to-end.

### Direction sign correctness (test: `TestSlippageBpsDirectionSign`)
- BUY filled **above** reference → adverse **positive** ✓
- SELL filled **below** reference → adverse **positive** ✓ (sign-flipped, slippage.go:36)

### Attribution dimensions verified (test: `TestSlippageAttribution`)
`byStrategy`, `byAlpha`, `bySession`, `byRegime`, `byDirection` all populated.

---

## Success criterion

| Target | Result | Status |
|---|---|---|
| Average slippage < 0.05% (5 bps) | 1.00 bps (0.01%) on MARKET | **PASS** (model) |

> Note: real-market slippage will exceed the paper model under thin liquidity.
> The infrastructure now records it per dimension so high-slippage strategies,
> sessions, and regimes surface automatically in `/api/execution/intelligence`.

---

## High-slippage detectors (live once trading)
`SlippageReport.ByStrategy / BySession / ByRegime` expose `WorstBps` and `TotalCost`
per bucket. A strategy or session whose `AvgBps` exceeds the 5 bps target will rank
itself; no separate alerting code is needed beyond the Prometheus gauge
`trading_execintel_slippage_avg_bps`.
