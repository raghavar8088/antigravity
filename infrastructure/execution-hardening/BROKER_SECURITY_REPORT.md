# Broker Security Report

## BrokerExecutionGateway model

Brokers are reachable only from Go engine after institutional gates:

| Broker | Adapter | Direct frontend? | Direct Next route? |
|--------|---------|------------------|-------------------|
| Paper BTC | `PaperClient.ExecuteSignal` | NO | NO |
| Delta | `delta.Bridge.SubmitOrder` | NO (410 routes) | NO |
| Angel One | not wired in ETP | NO (410) | NO (orders read-only) |
| Binance live | `BinanceLiveClient` | NO | NOT VERIFIED in prod loop |

## Delta bridge

- `OnOpen` → `WireDeltaBridge` → `executeThroughInstitutionalPathWithFill` → `SubmitOrder`
- `PlaceManualOrder` → disabled (error directing to gateway)
- `OnClose` → **RESIDUAL**: kill switch check added; full ETP not yet applied to reduce-only closes

## Blocked paths (evidence)

```typescript
// client/src/lib/blockedExecutionRoute.ts
return NextResponse.json({ code: "EXECUTION_ROUTE_RETIRED" }, { status: 410 });
```

## Allowed broker call chain

```
ExecutionGateway → ProcessExecutionRequest → ETP → Bridge.SubmitOrder → delta/client.PlaceOrder
```
