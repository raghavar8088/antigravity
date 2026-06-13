# ADR 001: Exchange Abstraction

## Status
Accepted

## Context
The platform uses multiple market data and broker sources: Coinbase, Binance, Delta Exchange, AngelOne, Yahoo Finance, and NSE fallbacks. Each source has different authentication, response shapes, symbol formats, market hours, error behavior, and execution semantics.

## Decision
Keep broker and exchange-specific logic behind adapter or broker-client boundaries. Internal trading code should consume normalized quotes, candles, orders, fills, account state, and errors.

## Consequences
- Strategies and risk gates do not need broker-specific response parsing.
- Fallbacks can be added without rewriting strategy logic.
- Broker/session failures are isolated to adapter boundaries.
- Adapters must preserve enough raw context for debugging and audit trails.

## AI Guidance
When changing exchange behavior, first identify the adapter boundary. Do not leak broker-specific response shapes into strategy, risk, OMS, or UI state unless there is already a stable domain type.
