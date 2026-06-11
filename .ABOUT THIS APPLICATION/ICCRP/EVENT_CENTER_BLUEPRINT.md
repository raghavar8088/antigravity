# EVENT CENTER BLUEPRINT — ICCRP V3

**Route:** `/terminal/events`  
**Component:** `client/src/components/EventCenter.tsx`  
**API:** `/api/event-center`

---

## Event Types (Implemented)

| Type | Color | Line |
|------|-------|------|
| FILL | `#3b82f6` | L37 |
| SIGNAL | `#8b5cf6` | L38 |
| ORDER | `#6366f1` | L39 |
| POSITION_OPEN | `#22c55e` | L40 |
| POSITION_CLOSE | `#94a3b8` | L41 |
| RISK_EVENT | `#ef4444` | L42 |
| KILL_SWITCH | `#dc2626` | L43 |
| RECONCILIATION | `#f59e0b` | L44 |
| SYSTEM | `#64748b` | L45 |

---

## Features

| Feature | Implementation | Lines |
|---------|----------------|-------|
| Search | `search` state filter | L55-56 |
| Type filter | `filterType` | L56 |
| Severity filter | `filterSeverity` | L57 |
| Realtime poll | 5s interval | L62-80 |
| Auto-scroll | `autoScroll` toggle | L60 |

---

## Data Source

`/api/event-center` aggregates platform events from MongoDB/engine — see `platformEvents.ts`.

---

## Status: IMPLEMENTED — P3 style migration to TerminalCard tokens
