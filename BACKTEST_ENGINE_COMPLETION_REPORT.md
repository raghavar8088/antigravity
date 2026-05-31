# Phase 4: Institutional Backtesting, Simulation & Validation Engine

## Summary

Implemented a V2 Go backtesting stack for BTC futures research under `engine/internal/backtest/v2` and a dedicated execution realism layer under `engine/internal/backtest/execution`. The legacy `engine/internal/backtest` simulator remains intact for compatibility, while V2 provides realistic execution assumptions, out-of-sample validation, portfolio analytics, and dashboard-ready reporting.

## Core Systems Added

- Backtest V2 engine with tick, candle, and hybrid simulation modes.
- Portfolio accounting with long/short positions, lifecycle exits, fees, funding, spread, slippage, latency, and impact attribution.
- Strict chronological bias detection and mandatory warmup enforcement.
- Dynamic regime classification and regime-specific performance metrics.
- Walk-forward validation V2 with rolling, expanding, and anchored windows using strategy factories to avoid state leakage.
- Monte Carlo analysis over trade order, execution quality, slippage, funding, and spread shocks.
- Robustness testing for higher fees, wider spreads, slippage shocks, funding shocks, volatility shocks, latency shocks, and missing data.
- Portfolio backtesting with capital allocations, portfolio drawdown, VaR, CVaR, correlation, heat, and Sharpe.
- Benchmark comparison against BTC buy-and-hold, BTC perpetual hold, VWAP, and funding carry.
- Research tournament scoring and promotion rules for execution, robustness, OOS, Monte Carlo, regimes, and benchmarks.
- Dashboard-ready Go report struct and a typed Next.js `InstitutionalBacktestDashboard` component.

## Execution Realism

The new fill path no longer executes at signal midpoint. Fill price is modeled as:

`signal price + bid/ask spread + slippage + latency decay + market impact`

The execution layer now includes:

- `SpreadModel` with volatility, session, liquidity, and time-of-day inputs.
- `SlippageEngine` with base, volatility, liquidity, order-size, and momentum components.
- `LatencyEngine` supporting 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, and 1000ms tiers.
- `ImpactModel` with temporary and permanent impact cost attribution.
- `FillModel` with market, limit, post-only, reduce-only, and partial fill behavior.
- `FundingModel` applying long/short funding cashflows over 8-hour funding windows.

## Validation Results

Passed:

- `go test -mod=mod ./internal/backtest/...`
- `go test -mod=mod ./internal/backtest/... ./internal/risk ./internal/strategy`
- `npm run lint -- src/components/InstitutionalBacktestDashboard.tsx`

Repo-wide validation note:

- `go test -mod=mod ./...` still fails on the pre-existing `engine/internal/marketdata/angelone.go` vet errors for non-constant `fmt.Errorf` format strings.
- The same full run also hit a Windows Application Control block while launching `internal/positions` test binary from the Go temp build directory.

## Key Files

- `engine/internal/backtest/execution/`
- `engine/internal/backtest/v2/`
- `engine/internal/backtest/execution/execution_model_test.go`
- `engine/internal/backtest/v2/engine_test.go`
- `client/src/components/InstitutionalBacktestDashboard.tsx`

## Outcome

The platform now has the core infrastructure needed to move backtesting reliability from a naive midpoint-fill PnL simulator toward an institutional research environment with realistic BTC futures execution, OOS controls, Monte Carlo survival checks, robustness gates, benchmark alpha, portfolio risk analytics, and promotion-quality evidence.
