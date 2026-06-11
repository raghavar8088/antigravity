# EVENT STREAM CERTIFICATION

**Status:** PASS  
**Pipeline:** `platformEvents.ts` → `/api/event-center` → `/api/engine/events` (SSE) → `EventCenter.tsx`

---

## Event Type Emission Proof

| Type | Emitter | File:Line | Trigger |
|------|---------|-----------|---------|
| **FILL** | Closed trades | `platformEvents.ts:64-74` | Each paper_trade |
| **POSITION_CLOSE** | Closed trades | `platformEvents.ts:76-86` | Each paper_trade |
| **POSITION_OPEN** | Open positions | `platformEvents.ts:89-99` | Each paper_position |
| **ORDER** | OMS orders | `platformEvents.ts:119-128` | Each paper_order |
| **SIGNAL** | OMS transitions | `platformEvents.ts:106-117` | transition contains SIGNAL |
| **RISK_EVENT** | Drawdown | `platformEvents.ts:131-151` | DD thresholds |
| **RECONCILIATION** | State staleness | `platformEvents.ts:153-182` | snap age / missing state |
| **KILL_SWITCH** | Engine probe | `platformEvents.ts:189-197` | killswitch active |
| **SYSTEM** | Engine probe fail | `platformEvents.ts:199-217` | HTTP error / timeout |

---

## Silent Failure Paths

| Path | Behavior | Misleading? |
|------|----------|-------------|
| Mongo not configured | Returns `[]` | EventCenter shows empty — ribbon shows DB status |
| Engine probe fails | SYSTEM WARNING event emitted | No |
| No SIGNAL transitions in orders | No SIGNAL events | Honest absence |
| Malformed WS frame | Ignored in store | N/A for events |

---

## Reachability

```
buildPlatformEvents(accountKey)
  → GET /api/event-center [event-center/route.ts:15]
  → EventCenter poll 5s [EventCenter.tsx:65-66]
  → SSE /api/engine/events [engine/events/route.ts:20] (parallel path)
```

---

## UI Filter Coverage

`EventCenter.tsx:46` — all 9 types in filter including ORDER.

**Certification:** Full event taxonomy wired; no silent drop of critical types.
