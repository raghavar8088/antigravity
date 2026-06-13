# Execution Flow

## BTC Paper Desk
```text
market data / signal tick
-> signal trace
-> strategy policy gate
-> risk and sizing logic
-> paper order/open position
-> mark price updates
-> exit logic
-> PnL, fees, liquidation/funding checks
-> MongoDB persistence
-> API route
-> hook
-> dashboard component
```

## Go Engine
```text
market data adapter
-> normalized tick/bar state
-> strategy registry
-> signal decision
-> risk gate
-> OMS v3 command
-> execution adapter
-> fill event
-> ledger update
-> reconciliation
-> kill switch check
-> persistence and observability
```

## Broker Flow
```text
broker credentials/session
-> market data or order API client
-> normalized internal type
-> risk/session validation
-> execution or persistence action
-> dashboard/API response
```

## Debugging Checklist
- Confirm the signal exists and is fresh.
- Confirm the gate did not reject the signal.
- Confirm sizing and account state are valid.
- Confirm OMS/order state changed as expected.
- Confirm fill/position/ledger math is consistent.
- Confirm persistence matches UI/API output.
