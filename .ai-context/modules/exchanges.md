# Exchanges Module

## Purpose
Isolate broker and exchange-specific behavior behind adapters and normalized internal types.

## Dependencies
- Broker credentials and sessions.
- Market data normalization.
- Order execution requests.
- Retry/fallback logic.
- Persistence and dashboard response models.

## Entry Points
- `engine/internal/marketdata/`
- `engine/internal/exchange/`
- `client/src/lib/broker/`
- `client/src/app/api/angelone/*`
- `client/src/app/api/delta-live/*`
- `client/src/app/api/nifty/*`

## Public APIs
- AngelOne routes and broker client helpers.
- Delta live routes.
- NIFTY and NSE market data routes.
- Engine market data adapters.

## Major Concepts
- Exchange adapter.
- Broker session.
- Market data provider.
- Order API client.
- Fallback source.
- Normalized quote/candle/order response.

## Files
Use scoped Graphify queries for `AngelOne`, `Delta`, `Binance`, `Coinbase`, `NSE`, and `marketdata`.
