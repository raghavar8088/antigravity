# Strategies Module

## Purpose
Generate, rank, filter, and evaluate trading signals across BTC, options, NIFTY, and strategy research workflows.

## Dependencies
- Market data adapters and normalized price/candle state.
- Indicator/trading helpers in `client/src/lib/trading/`.
- Go strategy registry in `engine/internal/strategy/`.
- Risk gates and session gates.
- Persistence for signal traces and performance analytics.

## Entry Points
- `engine/internal/strategy/`
- `client/src/lib/trading/`
- `client/src/lib/strategyAuthority/`
- `client/src/app/api/strategy-*`

## Public APIs
- Strategy signal routes under `client/src/app/api/strategy-*`.
- Strategy authority and analysis helpers under `client/src/lib/strategyAuthority/`.
- Go registry and strategy package APIs under `engine/internal/strategy/`.

## Major Concepts
- Strategy registry.
- Signal trace.
- Confidence/scoring.
- Policy gate.
- WINNERS_ONLY filtering.
- Replay and profitability analysis.

## Files
Use Graphify or `.ai-context/symbols.json` for exact symbols before reading raw strategy files.
