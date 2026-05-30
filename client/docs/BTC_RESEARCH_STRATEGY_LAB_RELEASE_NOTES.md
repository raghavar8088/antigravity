# BTC Research Strategy Lab Release Notes

## Commit-Ready Summary

This upgrade transforms the existing Mock Trading module into a mock-only BTC Research Strategy Lab. The lab discovers, evaluates, ranks, and monitors BTC intraday strategies under simulated execution costs while preserving full separation from paper/live order execution.

Suggested commit title:

```text
feat(mock-trading): transform mock trading into BTC research strategy lab
```

## Architecture Additions

- `btcResearchStrategyRegistry.ts`: research-backed BTC strategy registry with active OHLCV strategies and external-data stubs.
- `marketRegimeClassifier.ts`: ADX/ATR/Bollinger/EMA/realized-volatility regime engine.
- `strategyPerformanceEngine.ts`: net/gross PnL, win rate, profit factor, expectancy, Sharpe, Sortino, drawdown, recency, and regime breakdown metrics.
- `strategyScoringEngine.ts`: weighted strategy ranking and current-regime scoring.
- `strategyHealthEngine.ts`: `ACTIVE`, `WATCHLIST`, and `DISABLED` states with sample-size, expectancy, profit factor, and losing-streak controls.
- `mockResearchWalkForward.ts`: rolling training/validation windows and out-of-sample walk-forward score.
- `mockResearchPortfolioAllocation.ts`: score-weighted strategy/family allocation for healthy strategies.
- `mockResearchAnalytics.ts`: equity, drawdown, daily PnL, family analytics, strategy correlations, exposure, long/short bias, and warning generation.
- `MockResearchChartsPanel.tsx`: Lightweight Charts v5 dashboard for equity curve, drawdown, daily PnL, cumulative PnL, strategy comparison, and family comparison.
- `useMarketRegime.ts` and `useStrategyScoring.ts`: React hooks for live regime and strategy rankings.

## Strategy Registry Expansion

- Active research-backed strategies: `60` entries (`2000-2059`) from `30` distinct long/short base strategies.
- Data-pending advanced strategies: `40` entries (`2060-2099`) for CVD, OI/funding, and liquidation concepts that require unavailable external feeds.
- Active strategies are tagged with:
  - strategy id
  - name
  - family
  - timeframe
  - entry/exit/SL/TP rules
  - required indicators/data
  - best/worst regimes
  - research source document
  - confidence score
  - enabled state
- External-feed strategies always return `NO_SIGNAL` until the required data exists.

## New Mongo Collections

- `strategy_signals`
  - Stores emitted research signals with strategy, family, symbol, side, confidence, timestamp, and market regime.
  - Indexes: account/time, account/strategy/time, account/family/time, account/regime/time.

- `regime_snapshots`
  - Stores regime, confidence, ADX, ATR%, Bollinger width, EMA slope, realized volatility, and timestamp.
  - Indexes: unique account/timestamp, account/regime/time.

- `strategy_scores`
  - Stores latest score/rank per strategy.
  - Indexes: unique account/strategy, account/rank, account/current-regime-score.

- `strategy_score_history`
  - Append-only score and ranking history for restart reconstruction and historical ranking analysis.
  - Indexes: unique history id, account/strategy/scored time, account/rank/scored time.

- `equity_curve`
  - Stores equity, realized PnL, unrealized PnL, drawdown, daily PnL, regime, and timestamp.
  - Indexes: unique account/timestamp, account/regime/time.

- `daily_pnl_history`
  - Stores UTC daily PnL and trade count with optional regime tag.
  - Indexes: unique account/day, account/regime/day.

Existing mock collections remain in use:

- `mock_trades`
- `mock_account_snapshots`
- `mock_strategy_analytics`
- `mock_trade_logs`
- `mock_engine_config`

## New API Routes

- `GET/POST /api/mock-trading/signals`
- `GET/POST /api/mock-trading/regime`
- `GET/POST /api/mock-trading/equity`
- `GET/POST /api/mock-trading/scores`
- `GET/POST /api/mock-trading/daily-pnl`

Existing mock routes remain:

- `/api/mock-trading/trades`
- `/api/mock-trading/account`
- `/api/mock-trading/analytics`
- `/api/mock-trading/logs`
- `/api/mock-trading/reset`

## Analytics Additions

- Current regime card with confidence and indicator snapshot.
- Best/worst strategy panels.
- Full scoring table with health state.
- Strategy health dashboard.
- Walk-forward validation dashboard.
- Strategy and family allocation dashboards.
- Equity curve, drawdown, daily PnL, cumulative PnL, strategy comparison, and family comparison charts.
- Family analytics with best/worst regime.
- Strategy correlation matrix.
- Exposure dashboard.
- Long/short bias dashboard.
- Strategy confidence/sample-size dashboard.
- Risk and quality warnings for low sample size, excessive drawdown, high exposure, and correlated strategies.

## Selection Modes

- `RESEARCH_MODE`: accepts all valid enabled mock research signals.
- `PROFIT_MODE`: restricts entries to approved top-ranked research-backed strategies.
- `REGIME_MODE`: restricts entries to strategies with positive current-regime expectancy, adequate current-regime sample size, and sufficient current-regime score.

## Risk Controls

- Duplicate open research entries are suppressed by `strategyId + side`.
- Signal emissions remain capped by `maxSignalsPerMinute`.
- Strategy health can disable negative expectancy, low profit factor, and persistent losing strategies.
- Portfolio allocation only allocates to `ACTIVE` strategies.
- Allocation is advisory/dashboard-only and does not alter execution sizing.

## Safety Guarantees

- This is mock trading only.
- No broker orders are placed.
- No exchange orders are placed.
- No live trading APIs are called by the research lab.
- The mock research runner feeds only metadata into the mock engine.
- Live/paper execution modules are not imported by mock research code paths.
- Advanced strategies requiring order book, CVD, open interest, funding, or liquidation data remain inactive stubs until such feeds are explicitly added.

## Validation

Final checks run before commit:

- `npx tsc --noEmit`
- `npm run test`

Expected full-suite result at time of writing:

- `96` test files passed
- `1361` tests passed
