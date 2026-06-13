# Backtesting Module

## Purpose
Replay historical data and strategy decisions to evaluate performance, risk, and execution assumptions before production use.

## Dependencies
- Historical candles/ticks.
- Strategy implementations.
- Fee, slippage, funding, liquidation, and PnL math.
- Replay and analytics helpers.
- Result persistence or report output.

## Entry Points
- `engine/cmd/backtest/`
- `engine/internal/backtest/`
- `client/src/lib/analytics/`
- `client/src/lib/trading/*replay*`
- `client/src/app/api/*replay*`

## Public APIs
- Backtest command entry point.
- Replay API routes and scripts.
- Analytics comparison helpers.

## Major Concepts
- Historical data window.
- Replay engine.
- Strategy evaluation.
- Fill simulation.
- Slippage and fee model.
- Performance metrics.
- Walk-forward validation.

## Files
Never change PnL, fee, liquidation, or funding assumptions casually. Verify with focused tests when touching backtest math.
