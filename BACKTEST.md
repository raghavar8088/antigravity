# Backtest → Qualification → Live Pipeline (Reference Procedure)

This document is the authoritative, step-by-step procedure for adding new strategies,
backtesting them, and determining whether they qualify for the pre-live/live trade
engine. Follow these steps exactly whenever asked to "add N strategies and backtest them."

## 1. Architecture map

| Purpose | Path |
|---|---|
| Real backtest CLI entrypoint | `engine/cmd/run_backtest/main.go` |
| Backtest engine core | `engine/internal/backtest/` (`promotion.go`, `v2/` portfolio/regime, `v3/backtest_validation.go` stress replay) |
| Historical data fetcher | `engine/cmd/fetch_historical` |
| Historical cached OHLCV data | `engine/data/historical/BTCUSDT_{1m,5m,15m,1h,4h,1d,funding}.json` |
| Alpha/funding feed | `engine/data/alpha/funding.ndjson` |
| Strategy registry (where strategies are added) | `engine/internal/strategy/scalpers/curated_registry.go` |
| Live/pre-live engine consumer | `engine/cmd/antigravity/main.go:523` (`strategy.BuildCuratedScalpers()`) |
| Result output files | `engine/data/backtest_*.json`, `engine/data/backtest_results_*.json` |

⚠️ `engine/cmd/backtest/main.go` is a **stale toy/demo** (mock sine-wave data). Never use it — always use `engine/cmd/run_backtest/main.go`.

## 2. Adding new strategies

1. Implement the strategy satisfying the `OHLCVCompatible` strategy interface used by the registry builders.
2. Add it inside one of the existing builder functions in `curated_registry.go`
   (`BuildAllScalpers`, `BuildPortedStrategies`, `buildExpansionPack`,
   `buildVolatilityFamily`, `buildMicrostructureFamily`, `buildMacroFamily`,
   `buildStatisticalFamily`, `buildEventFamily`, `buildFamily1Momentum` … `buildFamily5DerivativesMacro`,
   `buildImmortalEditionPack`), or create a new builder function following the same
   pattern and wire it into `BuildCuratedScalpers()` (lines ~91-148).
3. Each builder returns `[]RegistryEntry{Name, Strategy, OHLCVCompatible, ...}`.
4. Do **NOT** add the new strategy names to `tradeEngineEnabled` (lines ~45-67) yet —
   that whitelist is the final manual gate, applied only *after* backtest qualification (step 5).

## 3. Fetch/verify historical data (only if date range not already cached)

```
go run ./cmd/fetch_historical
```
This populates/refreshes `engine/data/historical/`. Skip if the required symbol/date range is already cached.

## 4. Run the backtest

```
go run ./cmd/run_backtest --symbol BTCUSDT --from <start-date> --to <end-date> --cache-dir data/historical --out data/backtest_results_<label>.json
```
- Use `--hw-only` if only testing hand-written strategies added since the last full run.
- Name the `--out` file descriptively (e.g. `backtest_results_v7.json`) — never overwrite `backtest_results_prelive.json` directly; that file is the canonical "last decision" snapshot.
- The runner automatically restricts to `OHLCVCompatible` strategies and calls `EvaluatePromotion` per strategy.

## 5. Read qualification results

Output schema (`ResultRow`), one entry per strategy:
```
{
  "elapsed_s": ..., "from": ..., "ran_at": ...,
  "strategies": [
    {
      "rank": ..., "strategy": "...",
      "total_trades": ..., "win_rate_pct": ..., "sharpe": ...,
      "max_drawdown_pct": ..., "profit_factor": ..., "total_return_pct": ...,
      "avg_win_pct": ..., "avg_loss_pct": ...,
      "meets_promotion": true/false,
      "demote_reason": "..."   // present only if failed
    }
  ]
}
```

**Promotion criteria** (`engine/internal/backtest/promotion.go:12-26`) — a strategy is "qualified" (`meets_promotion: true`) only if ALL of:
- Sharpe ≥ 1.0
- Win rate ≥ 45%
- Max drawdown ≤ 20%
- Profit factor ≥ 1.3
- Total trades ≥ 50
- Stress passes ≥ 3 (skipped/not enforced when no v3 stress-test result is attached — the plain CLI run does not attach one)

For each newly added strategy, report: pass/fail against each individual criterion, not just the final boolean — the user wants to know *why* a strategy failed, not just that it did.

## 6. Promote qualified strategies

Strategies with `meets_promotion: true` are candidates for `PromoteFromResult()`
(`promotion.go:67+`), which writes `Performance{Active:true}` into the global
performance registry via `scalers.UpdatePerformance`. This is **not automatic** —
it must be explicitly invoked/reviewed per strategy after inspecting the backtest output.

## 7. Add to the live trade-engine whitelist (final manual gate)

Only after a strategy is qualified (step 5) AND promoted (step 6) should its name be
added to the `tradeEngineEnabled` map in `curated_registry.go:45-67`. This is a
manually curated whitelist ("Selected from the Strategy Leadership Board") — it is
the actual gate for what runs in the live/pre-live engine, independent of the
`Active` flag from promotion. Update the comment/date on the map when adding entries.

Sequence enforced in `BuildCuratedScalpers()`: all builders → `filterTradeEngineEnabled()` → `FilterWinnersOnly()` (drops strategies with ≥30 trades and `Active=false`).

## 8. Demotion (ongoing, automatic once live)

`DemotionCriteria` (`promotion.go:29-35`): trades ≥ 30 and win rate < 40% → `Active` flips
to `false` → `FilterWinnersOnly()` excludes it from future `BuildCuratedScalpers()` calls.
No action needed here unless investigating a live regression.

## Known gap (as of 2026-07-03)

There is no automated code path from "100 backtest-qualified strategies" to the
21-name `tradeEngineEnabled` whitelist — narrowing the qualified pool down to the
live whitelist is a manual operator decision, not a pipeline stage. When asked to
"add N strategies to the live engine," steps 6 and 7 must be done explicitly, one
strategy at a time, based on reviewed backtest output.
