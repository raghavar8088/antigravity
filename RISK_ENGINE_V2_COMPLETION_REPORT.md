# Phase 5: Institutional Risk Engine, Portfolio Management & Capital Allocation System

## Summary

Implemented a new `engine/internal/risk/v2` institutional risk package and wired it into the existing Go `RiskEngine.Validate` approval path. The V2 system evaluates every proposed trade through layered portfolio risk controls before execution approval.

## Architecture Added

Risk V2 approval flow:

`Strategy -> Position Risk -> Portfolio Heat -> VaR/CVaR -> Correlation -> Exposure -> Allocation -> Regime -> Leverage -> Tail Risk -> Execution Approval`

New package:

- `engine/internal/risk/v2/engine.go`
- `portfolio.go`
- `allocation.go`
- `sizing.go`
- `kelly.go`
- `dynamic_sizing.go`
- `heat.go`
- `var.go`
- `cvar.go`
- `correlation.go`
- `exposure.go`
- `optimizer.go`
- `drawdown.go`
- `regime_risk.go`
- `family_risk.go`
- `risk_budget.go`
- `leverage.go`
- `tail_risk.go`
- `attribution.go`
- `forecast.go`
- `tournament.go`
- `health.go`
- `alerts.go`
- `dashboard.go`

## Institutional Controls

Implemented:

- Kelly sizing with full, half, and quarter Kelly, while never selecting full Kelly.
- Maximum 2% account risk per trade.
- Dynamic position sizing across strategy health, Sharpe, profit factor, drawdown, regime, volatility, funding, correlation, and tail risk.
- Portfolio heat engine with normal, reduce-size, block-new-trades, and force-reduction states.
- Historical, parametric, and Monte Carlo VaR.
- CVaR / expected shortfall.
- Strategy, position, and portfolio correlation analysis with hidden concentration detection.
- Exposure management by strategy, family, regime, and signal type.
- Capital allocation scoring by Sharpe, Sortino, PF, expectancy, drawdown, OOS performance, and health.
- Portfolio optimizer modes: equal weight, risk parity, Sharpe optimization, minimum variance, maximum diversification, and Kelly portfolio.
- Drawdown ladder from 2% size reduction to 10% trading halt.
- Regime-aware sizing and leverage reduction.
- Strategy family controls with 30% allocation cap.
- Risk budgeting with institutional family budgets.
- Dynamic leverage control.
- Tail risk scoring for volatility shocks, flash crashes, liquidity collapse, liquidation cascades, and funding shock.
- Risk attribution by strategy, family, funding, volatility, execution, correlation, and regime.
- Risk forecasting using rolling windows, EWMA volatility, and Monte Carlo VaR.
- Tournament and promotion risk scoring.
- Strategy health risk categories: Elite, Healthy, Watchlist, Restricted, Disabled.
- Dashboard, Telegram, email, and webhook alert metadata.

## Integration

- `engine/internal/risk/engine.go` now owns a `risk/v2.Engine`.
- `RiskEngine.Validate` calls V2 after legacy checks and Phase 2 portfolio checks.
- `RiskEngine.V2()` exposes the V2 engine for dashboards, analytics, or execution services.
- Added `client/src/components/InstitutionalRiskCenter.tsx` for dashboard rendering of heat, VaR/CVaR, correlation, allocation, risk budgets, drawdown, leverage, tail risk, attribution, exposure, forecasts, Kelly sizing, and alerts.

## Tests

Added:

- `engine/internal/risk/v2/engine_test.go`

Coverage includes:

- Kelly sizing capped at 2% risk and fractional Kelly selection.
- Portfolio heat breach blocking.
- Tail-risk halt blocking.
- Promotion requirements for OOS and Sharpe/risk score.

## Validation

Passed:

- `go test -mod=mod ./internal/risk/v2 ./internal/risk`
- `go test -mod=mod ./internal/risk/v2 ./internal/risk ./internal/backtest/v2 ./internal/strategy`
- `npm run lint -- src/components/InstitutionalRiskCenter.tsx`

Repo-wide note:

- `go test -mod=mod ./...` still fails on the pre-existing `engine/internal/marketdata/angelone.go` vet errors for non-constant `fmt.Errorf` format strings.

## Outcome

Risk management has moved from mostly rule-based protection toward a portfolio-level capital preservation and optimization system with institutional controls for position sizing, heat, VaR/CVaR, correlation, exposure, allocation, drawdown, leverage, tail risk, attribution, forecasting, promotion quality, and dashboard visibility.
