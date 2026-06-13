# Recent Decisions

## Token Optimization Direction
- Keep Graphify as the primary code relationship graph.
- Use Repomix only as a compact metadata/directory summary by default.
- Avoid adding duplicate graph engines, syntax parsers, or repository indexing tools.
- Use `.ai-context/` as the stable AI orientation layer.

## Runtime Data
- Runtime trading data lives under `runtime/`.
- `runtime/` is ignored by git and Cursor.
- Historical candles, ticks, backtests, logs, reports, exports, and snapshots should not be indexed as source.

## Protected Trading Boundaries
- BTC futures trading safety modules are protected.
- Risk gates must remain before execution.
- Kill switch wiring must remain in production paths.
- Fee, funding, liquidation, PnL, sizing, and ledger math require focused tests and explicit approval for behavior changes.

## Auto Refresh
- `npm run ai-context:refresh` rebuilds graph context, maps, and Repomix summary.
- `npm run ai-context:install-hooks` installs a local pre-commit hook that refreshes and stages AI context.
- GitHub workflow `.github/workflows/ai-context.yml` checks context freshness.
