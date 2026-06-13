# Strategy Index

## Strategy Layers
- Strategy registry: curated active strategies and families.
- Signal generation: market data and indicators converted into entries/exits.
- Policy gates: session, market regime, correlation, confidence, and feature-flag checks.
- Risk gate: sizing, loss limits, exposure limits, and kill switch awareness.
- Execution: OMS and adapter handoff.
- Analytics: replay, ranking, profitability, and verification.

## Strategy Families
Active strategy families include EMA crosses, RSI thresholds, RSI slope, Bollinger Band variants, funding/CVD, Delta absorption, liquidity sweeps, fair value gap retests, order blocks, market structure shifts, microstructure, and volume profile logic.

## Important Rule
WINNERS_ONLY filtering is active. Do not reintroduce strategies that were removed for negative expectancy unless the user explicitly asks for research-only restoration.

## Where To Look
- `engine/internal/strategy/`: Go registry and strategy logic.
- `client/src/lib/trading/`: TypeScript trading helpers, strategy analytics, paper desk calculations, and replay utilities.
- `client/src/lib/strategyAuthority/`: portfolio intelligence, ranking, strategy authority, and Mongo-backed analysis.
- `client/src/app/api/strategy-*`: route families that expose strategy analysis and traces.

## AI Query Pattern
For strategy questions:

```bash
npm run graphify:query -- "which modules evaluate BTC futures strategy signals?"
python scripts/graphify_workflow.py query --scope engine-internal "where is strategy registry connected to risk?"
```
