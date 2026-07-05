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

## 4. Run the backtest — ALWAYS two windows (train + validation)

A single-window backtest is not evidence — with 300+ strategy variants tested
against one sample, dozens qualify by chance. Run the same command twice:

```
# TRAIN window
go run ./cmd/run_backtest --symbol BTCUSDT --from 2021-06-26 --to 2024-06-30 --cache-dir data/historical --out data/backtest_results_<label>_train.json
# VALIDATION window (out-of-sample)
go run ./cmd/run_backtest --symbol BTCUSDT --from 2024-07-01 --to 2026-06-26 --cache-dir data/historical --out data/backtest_results_<label>_validate.json
```
- Use `--hw-only` if only testing hand-written strategies added since the last full run.
- Name the `--out` file descriptively (e.g. `backtest_results_v7_train.json`) — never overwrite `backtest_results_prelive.json` directly; that file is the canonical "last decision" snapshot.
- The runner automatically restricts to `OHLCVCompatible` strategies and calls `EvaluatePromotion` per strategy.

**Cost model & metrics (fixed 2026-07-05, do not regress):**
- Commissions are Binance **taker** both legs (`TierStandard`, 8 bps round trip) —
  the live engine sends market orders; the old `TierMaker` (4 bps) assumption
  overstated every strategy's edge.
- Sharpe is annualised by the strategy's **actual trade frequency**
  (`√(trades/year)`), not `√35040`. The old formula inflated Sharpe ~10-40×
  and made the `Sharpe ≥ 1.0` gate pass nearly everything. Honest Sharpe for a
  good scalper lands roughly in 1.0–2.5; if you see 20+, the metric is broken again.

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

## 5b. Out-of-sample confirmation (the actual qualification bar)

A strategy is **qualified** only if it:
1. has `meets_promotion: true` in the **train** window, AND
2. confirms in the **validation** window: ideally `meets_promotion: true` there
   too; at minimum trades ≥ 30, positive return, profit factor ≥ 1.2,
   win rate ≥ 45%, Sharpe > 0.5.

Strategies that qualify in train but fail validation are curve-fit survivors —
route them to `tradeEngineShadow` (see §7), never to the live whitelist.
Benchmark: on 2026-07-05 this bar reduced 303 strategies → 19 train-qualified
→ **2 fully confirmed OOS** (`BB_Squeeze_EFI_ADX_Short`, `WMA_Bear_Cross_Short`).

## 6. Promote qualified strategies

Strategies with `meets_promotion: true` are candidates for `PromoteFromResult()`
(`promotion.go:67+`), which writes `Performance{Active:true}` into the global
performance registry via `scalers.UpdatePerformance`. This is **not automatic** —
it must be explicitly invoked/reviewed per strategy after inspecting the backtest output.

## 7. Add to the live trade-engine whitelist (final manual gate)

The registry is two-tier (`curated_registry.go`):
- `tradeEngineEnabled` — LIVE. Only strategies that passed §5b (train-qualified
  AND OOS-confirmed). Never add a name from a single-window backtest or from
  short-window live leaderboard rankings — that is exactly the selection-on-noise
  mistake that put 6 of the worst 5-year losers live on 2026-07-03.
- `tradeEngineShadow` — SHADOW tier, pinned via `withForcedShadow()`: evaluates
  every cycle, records to the shadow ledger, never reaches the live OMS. For
  (a) feed-dependent strategies that cannot be backtested offline and
  (b) train-qualifiers that failed OOS. Promotion: shadow track record must
  clear `ShadowPromoter.CanPromote`, then move the name to `tradeEngineEnabled`.

Update the comment/date on the maps when adding entries.

Sequence enforced in `BuildCuratedScalpers()`: all builders → `filterTradeEngineEnabled()` (keeps live + wraps shadow tier) → `FilterWinnersOnly()` (drops strategies with ≥30 trades and `Active=false`).

## 8. Demotion (ongoing, automatic once live)

`DemotionCriteria` (`promotion.go:29-35`): trades ≥ 30 and win rate < 40% → `Active` flips
to `false` → `FilterWinnersOnly()` excludes it from future `BuildCuratedScalpers()` calls.
No action needed here unless investigating a live regression.

## Known gap (as of 2026-07-05)

There is no automated code path from backtest output to the `tradeEngineEnabled`
whitelist — applying §5b and editing the two maps is a manual operator decision,
not a pipeline stage. When asked to "add N strategies to the live engine," steps
5b, 6 and 7 must be done explicitly, one strategy at a time, based on reviewed
train + validation output.

Historical note: results produced before 2026-07-05 (all `backtest_results_*.json`
except the `*_honest_*` files) used the inflated √35040 Sharpe and maker-fee cost
model — their `meets_promotion` flags are unreliable and must not be used for
whitelist decisions.
