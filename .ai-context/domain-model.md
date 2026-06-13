# Domain Model

This file captures the trading concepts AI agents should understand before reading source.

## Core Chain
```text
MarketData
-> SignalEngine
-> StrategyEngine
-> RiskManager
-> OrderManager / OMS
-> ExchangeAdapter
-> PortfolioManager
-> Persistence
-> Dashboard / API
```

## MarketData
Market data includes ticks, candles, index quotes, option chains, funding, order book state, and broker/account state. Sources include Coinbase, Binance, Delta Exchange, AngelOne, Yahoo Finance, and NSE fallbacks.

Important rules:
- Crypto data is 24/7.
- NSE/BSE data must respect exchange sessions and holidays.
- Stale data must not create fresh trading signals.

## Signal Lifecycle
```text
raw market data
-> indicator calculation
-> strategy condition
-> signal candidate
-> confidence/scoring
-> policy gate
-> risk gate
-> executable decision or rejection reason
```

Signals should carry enough trace information to explain why a trade did or did not happen.

## Strategy Lifecycle
```text
strategy registry
-> active strategy selection
-> signal evaluation
-> rank/score
-> policy filtering
-> risk approval
-> execution request
-> performance tracking
```

WINNERS_ONLY filtering is active. Treat removed losing strategies as unavailable unless the user asks for research-only analysis.

## Risk Flow
```text
account state
-> exposure check
-> position size check
-> daily loss check
-> session/market gate
-> kill switch check
-> approve, resize, or reject
```

Risk must precede execution. Do not move execution before gates.

## Order Lifecycle
```text
intent
-> validation
-> OMS command
-> submitted/paper-submitted
-> accepted or rejected
-> partial/full fill
-> position update
-> ledger event
-> reconciliation
-> closed/cancelled/expired
```

Paper and live flows should preserve the same accounting invariants even when adapters differ.

## Position Lifecycle
```text
open request
-> entry fill
-> mark updates
-> unrealized PnL
-> funding/fee/liquidation checks
-> exit signal or risk exit
-> close fill
-> realized PnL
-> persisted closed trade
```

Preserve fee, funding, liquidation, and PnL math.

## Exchange Flow
```text
credentials/session
-> market data or order request
-> exchange response
-> normalization
-> retry/fallback handling
-> internal event/state update
```

Broker-specific behavior belongs behind adapter or broker-client boundaries.

## Persistence Flow
```text
domain event/state
-> validation
-> database write
-> read model/API projection
-> dashboard hook
-> UI state
```

Do not silently paper over persistence failures in trading-critical paths.
