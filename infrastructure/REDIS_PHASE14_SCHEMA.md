# Phase 14 Redis Schema

Redis is a hot cache and coordination layer only. It is never the source of truth.

## Keys

```text
live:position:{account}:{symbol}
live:pnl:{account}
live:risk:{account}
live:market:{exchange}:{symbol}
live:strategy_rankings:{account}
live:order:{account}:{client_order_id}
dedupe:idempotency:{idempotency_key}
rate:exchange:{exchange}:{endpoint}:{window}
health:service:{service_name}
```

## TTL Strategy

- `live:market:*`: 3 seconds. Expiry means stale data and blocks trading.
- `live:position:*`: 10 seconds, refreshed from projections.
- `live:pnl:*`: 10 seconds.
- `live:risk:*`: 10 seconds.
- `live:strategy_rankings:*`: 60 seconds.
- `live:order:*`: 24 hours after terminal state.
- `dedupe:idempotency:*`: 24 hours.
- `rate:exchange:*`: request-window plus 5 seconds.
- `health:service:*`: 15 seconds.

## Warm Restart

1. Replay event ledger into Postgres projections.
2. Load active positions, open orders, risk state, and latest market snapshots.
3. Populate Redis.
4. Reconcile exchange.
5. Keep kill switch active until reconciliation passes.
