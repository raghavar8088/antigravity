# Glossary

## Trading Terms
- **Signal**: Strategy output indicating possible entry, exit, or hold behavior.
- **EntrySignal**: Signal that requests opening or scaling into a position.
- **ExitSignal**: Signal that requests closing or reducing a position.
- **Position**: Current market exposure for an account, symbol, side, and strategy.
- **Exposure**: Total risk amount represented by open positions.
- **Leverage**: Notional exposure divided by account equity or margin.
- **PnL**: Profit and loss, either unrealized from marks or realized after close.
- **MarkPrice**: Current valuation price used for unrealized PnL and liquidation checks.
- **FundingRate**: Periodic futures payment rate between long and short sides.
- **Liquidation**: Forced close risk when margin is insufficient.
- **StopLoss**: Risk exit threshold to cap downside.
- **TakeProfit**: Exit threshold to capture target profit.
- **ReduceOnly**: Order flag indicating an order can only reduce exposure.
- **HedgeMode**: Mode that allows simultaneous long and short exposure on the same instrument.
- **CrossMargin**: Margin mode where collateral is shared across positions.
- **IsolatedMargin**: Margin mode where collateral is isolated to a single position.

## System Terms
- **MarketData**: Quotes, candles, ticks, option chains, order book, funding, and account/broker state.
- **SignalEngine**: Logic that turns market data and indicators into signal candidates.
- **StrategyEngine**: Registry and evaluator for active strategies.
- **RiskManager**: Gate that enforces exposure, size, loss, session, and kill switch constraints.
- **OrderManager / OMS**: Order lifecycle system that validates commands and records state transitions.
- **ExecutionAdapter**: Broker/exchange boundary that submits or simulates orders.
- **Ledger**: Accounting record for fills, fees, funding, balances, and realized events.
- **Reconciliation**: Process that compares internal state with broker or persisted state.
- **KillSwitch**: Safety control that blocks execution when triggered.
- **Persistence**: Database or filesystem storage for trading state, events, and projections.

## Repo Terms
- **Graphify**: Knowledge graph used for code relationships and low-token exploration.
- **Repomix**: Repository pack/summary generator used for compact AI orientation.
- **AI Context**: Files under `.ai-context/` that should be read before broad source exploration.
- **Scoped Graph**: Smaller Graphify graph for `client`, `engine-internal`, or `engine-cmd`.
