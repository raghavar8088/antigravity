# Business Rules

These are system-level rules AI agents should preserve unless the user explicitly asks to change them.

## Position Rules
- Never open duplicate positions for the same account, symbol, side, and strategy unless the strategy explicitly supports scaling.
- Position size must be validated before order creation.
- Position state must include entry price, quantity, side, mark price, unrealized PnL, fees, and exit state where applicable.
- Closed positions must preserve realized PnL and exit reason.
- Liquidation risk checks must not be skipped for futures-like products.

## Risk Rules
- Risk gate approval is required before paper or live execution.
- Stop loss or explicit risk exit logic is required for automated strategies.
- Daily loss, exposure, and position size limits must be enforced before order submission.
- Kill switch state must be checked in production execution paths.
- NSE/BSE trades must respect market hours and holidays.
- Crypto flows may run 24/7 but still require stale-data checks.

## Order Rules
- Orders must pass validation before entering OMS.
- OMS state transitions must be explicit and auditable.
- Cancelled/rejected orders must not mutate positions as filled orders.
- Partial fills must update position and ledger state consistently.
- Live broker responses must be normalized before internal state updates.

## Signal Rules
- A signal must be fresh enough for its market and timeframe.
- Signals rejected by gates should expose a rejection reason.
- Strategy scoring must not hide hard risk rejections.
- WINNERS_ONLY filtering remains active for production strategy selection.

## PnL And Accounting Rules
- Preserve fee, funding, liquidation, and PnL invariants.
- Paper trading math should remain consistent with live execution assumptions where possible.
- Ledger updates must reflect fills, fees, funding, realized PnL, and balance-impacting events.
- Reconciliation should detect mismatches rather than silently overwrite authoritative state.

## Data Rules
- Historical market data, tick dumps, candles, logs, reports, screenshots, and binaries should not be loaded into AI context by default.
- Use `.cursorignore`, Graphify, Repomix summaries, and `.ai-context` maps for orientation.
- Source files should be opened only after the graph or maps identify the exact target.
