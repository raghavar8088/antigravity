# ROOT CAUSE ANALYSIS — Mock Trading Outage (2026-06-08 → 2026-06-10)

## Primary Root Cause

**Reconciliation v2 false CRITICAL drift → institutional kill switch (`OMS_DESYNC`) → all new orders blocked at `PreTradeRiskPipeline`.**

Introduced by commit **`33c614a8`** (*Wire reconciliationv2 into production with real broker snapshots and kill-switch on drift*, 2026-06-10).

## Source-Code Evidence

### Bug 1 — OMS equity projection used realized PnL instead of account equity

**File:** `engine/internal/reconciliationv2/ledger_oms_reader.go` (pre-fix lines 98–104)

```go
Balance: OMSBalanceSnapshot{
    EquityUSD:     pnl.TotalPnLUSD,  // BUG: ~$0–$500, not ~$1,000,000
    AvailableUSD:  pnl.TotalPnLUSD,
    RealizedPnL:   pnl.TotalPnLUSD,
},
```

**Runtime truth:** `engine/internal/execution/paper.go:185–189` `GetEquityUSD()` returns `balanceUSD + positionBTC * lastKnownPrice` (~$1M).

**Detector:** `engine/internal/reconciliationv2/detectors.go:74–87` — `equity_drift` with `classifySeverity()` → **CRITICAL** when drift ≥ 1% (`detectors.go:40–41`).

**Kill switch:** `engine/internal/reconciliationv2/killswitch_hook.go:24–35` triggers on `SeverityCritical`.

**Order block:** `engine/internal/risk/gate/pipeline.go:51–54` returns `DecisionBlocked` when `killSwitch.IsActive()`.

### Bug 2 — Position side key mismatch (BUY/SELL vs LONG/SHORT)

**OMS ledger:** `engine/internal/trading/loop.go:842` stores `Side: string(pos.Side)` → `"BUY"` / `"SELL"`.

**Runtime adapter:** `engine/internal/reconciliationv2/position_manager_adapter.go:78–80` maps to `"LONG"` / `"SHORT"`.

**Detector (pre-fix):** `detectors.go:160,171` keyed `symbol + "|" + side` without normalization → simultaneous **ghost_position** + **missing_position** (both CRITICAL, lines 181, 247).

### Bug 3 — Kill switch never self-released

**File:** `engine/internal/killswitch/service.go:115–116` — *"Requires manual operator action — the switch never self-releases."*

Once triggered, trading halted until `POST /api/admin/ks/release` or engine restart (in-memory only; Postgres events persisted but not replayed on boot pre-fix).

## Secondary Causes

| Cause | Evidence |
|-------|----------|
| Reconciliation runs every 10s on balances | `scheduler.go:66` `BalancesInterval: 10s` |
| Startup audit runs immediately | `wiring.go:93–97` goroutine `RunNow` |
| Client TS worker disabled | `ENGINE_EXECUTION_AUTHORITY=1` default — only Go engine executes |
| No startup grace on kill hook (pre-fix) | Hook fired on first CRITICAL cycle |

## Contributing Factors

- `BuildPnLProjection` (`omsv3/projections.go:138–168`) tracks closed-trade PnL only — correct for PnL domain, wrong when used as equity.
- Same anti-pattern in `omsv3/snapshot_provider.go:87–90` (legacy v1 path).
- Kill switch durable in Postgres but in-memory `active` flag not restored/auto-healed pre-fix.

## Architectural Weaknesses

1. Reconciliation hook can halt entire execution path without human-in-the-loop confirmation.
2. OMS reader had no integration test comparing against `$1M` paper baseline.
3. Side normalization existed (`normalizePositionSide`) but was unused in drift detector keys.

## Operational Weaknesses

1. No `[WATCHDOG] no_fills` alert existed pre-fix.
2. Production deploy of recon v2 without staged drift validation against live paper equity.

## FIRST FAILURE POINT

| Field | Value |
|-------|-------|
| **Timestamp** | ≤10s after engine boot post-`33c614a8` deploy (first balance reconciliation cycle) |
| **Function** | `BalanceDriftDetector.Detect` |
| **File** | `engine/internal/reconciliationv2/detectors.go:74–87` |
| **Reason** | `equity_drift` CRITICAL: exchange≈1,000,000 vs OMS≈TotalPnLUSD |
| **Downstream** | `CriticalDriftKillSwitchHook` → `killswitch.Service.Trigger(OMS_DESYNC)` → `PreTradeRiskPipeline` blocks |
