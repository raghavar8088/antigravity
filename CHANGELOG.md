# Changelog

## Unreleased

### Added

- Upgraded Mock Trading into a BTC Research Strategy Lab with 60 active research-backed strategy entries, market regime classification, strategy scoring/ranking, health states, walk-forward validation, allocation dashboards, advanced analytics, and Lightweight Charts equity visualizations.
- Added Mongo-backed persistence for research signals, regime snapshots, equity curve history, daily PnL history, latest strategy scores, and strategy score/ranking history.
- Added mock-only API routes for research signals, regimes, equity history, strategy scores, and daily PnL.

### Safety

- Mock Trading remains fully simulated and isolated from live broker/exchange execution paths. No live order placement APIs are called by the research lab.
