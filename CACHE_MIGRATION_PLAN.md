# Cache Migration Plan

Generated from repository discovery on 2026-06-02.

## Cache Inventory

| Cache | Location | Class | Purpose | Migration Decision |
| --- | --- | --- | --- | --- |
| Redis live market cache | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Rebuildable | Latest market snapshots | Rebuild from exchange feed / projections |
| Redis live position cache | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Rebuildable | Low-latency position reads | Rebuild from ledger/projections |
| Redis PnL/risk cache | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Rebuildable | Dashboard/risk speed | Rebuild from projections |
| Redis strategy rankings | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Rebuildable with source | Ranking cache | Rebuild from Mongo `paper_trades` / fixture |
| Redis order cache | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Rebuildable with source | Recent order status | Rebuild from ledger/OMS projections |
| Redis idempotency keys | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Semi-persistent | Duplicate suppression | Export during warm/live clone; can expire on cold clone |
| Redis rate-limit keys | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Optional | Exchange endpoint throttling | Do not migrate for cold clone |
| Redis health keys | `infrastructure/REDIS_PHASE14_SCHEMA.md` | Optional | Service health | Rebuild |
| Node module-scope Mongo client cache | `client/src/lib/mongoTradesClient.ts` | Runtime only | Connection reuse | Do not migrate |
| Browser localStorage caches | Client hooks/components | Persistent user cache | UI/state fallback | Export per browser profile if exact clone required |
| Options snapshot cache | `client/src/lib/optionsSnapshotCache.ts` | Browser persistent | Options buy/sell UI state | Export if operator-profile parity required |
| Go in-memory ledger/projections | `engine/internal/ledger/store.go`, `omsv3/authority.go` | Runtime only | Paper/test projections | Rebuild from durable event store |
| AI framework instance | `infrastructure/ai/strategy_service/api.py` | Runtime only | Strategy service status/journal | Persist separately if production use adds durable journal |
| Go build/test cache | `.gocache-test/`, `.gotmp-test/` | Rebuildable | Local developer cache | Optional |

## Redis Keyspace

Documented keys:

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

TTL policy:

- `live:market:*`: 3 seconds
- `live:position:*`: 10 seconds
- `live:pnl:*`: 10 seconds
- `live:risk:*`: 10 seconds
- `live:strategy_rankings:*`: 60 seconds
- `live:order:*`: 24 hours after terminal state
- `dedupe:idempotency:*`: 24 hours
- `rate:exchange:*`: request window plus 5 seconds
- `health:service:*`: 15 seconds

## Classification

### Persistent / Copy If Warm Or Live Clone

- `dedupe:idempotency:*` if original and clone run near-simultaneously.
- `live:order:*` if orders can be active during migration.
- Redis AOF/RDB if exact runtime cache reconstruction is required.

### Rebuildable

- `live:position:*`
- `live:pnl:*`
- `live:risk:*`
- `live:strategy_rankings:*`
- Go in-memory projections
- Node connection caches

### Optional / Do Not Migrate

- `rate:exchange:*`
- `health:service:*`
- Go build/test caches
- Promtail positions for fresh clone ingestion

## Cold Clone Procedure

1. Do not migrate Redis.
2. Restore source-of-truth databases.
3. Start engine with kill switch active.
4. Replay ledger and rebuild projections.
5. Warm Redis:
   - active positions
   - open orders
   - latest PnL
   - risk summary
   - strategy rankings
6. Run reconciliation.
7. Release kill switch only after validation.

## Warm / Live Clone Procedure

1. Capture Redis metadata:

```bash
redis-cli INFO keyspace > redis_info.txt
redis-cli --scan > redis_keys.txt
redis-cli --rdb redis_dump.rdb
```

2. Restore into isolated Redis.
3. Immediately rewrite any environment/account namespace if the clone must be independent.
4. Validate TTLs:

```bash
redis-cli --scan | while read key; do redis-cli ttl "$key"; done
```

5. Run ledger replay anyway and compare rebuilt cache values to restored cache values.

## Cache Validation

Required checks after clone:

- Redis key count by prefix matches expected migration class.
- Rebuildable keys can be regenerated from DB/ledger.
- No clone Redis URL points to original Redis.
- No original worker and clone worker share `DESK_WORKER_ACCOUNT_KEY` unless original is stopped.
- Strategy ranking cache matches `client/fixtures/replay/btc_ft_strategy_rankings.json` or Mongo-derived ranking output.

## Recommendation

For the first dry run, treat Redis as rebuildable and do not copy it. For warm/live clone, copy only if active orders/idempotency windows must survive with no duplicate submissions.
