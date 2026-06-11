# OBSERVABILITY CENTER BLUEPRINT — ICCRP V3

**Route:** `/terminal/observability`  
**Component:** `client/src/components/terminal/institutional/ObservabilityCenter.tsx`  
**Related:** `/terminal/health`, `/terminal/diagnostics`

---

## Display (Implemented)

| Metric | API | Component Lines |
|--------|-----|-----------------|
| API Health | `/api/system/health` | L44-48 |
| Engine Latency | `health.engine.ping_ms` | L49 |
| Mongo Latency | `health.mongo.ping_ms` | L50 |
| Worker Heartbeat | `/api/health/desk-worker` | L51-55 |
| Feed Health | `/api/risk-ribbon` items | L59-74 |
| OMS / Recon / Kill Switch | risk-ribbon | L76-82 |

Poll interval: 10s — `ObservabilityCenter.tsx` L34

---

## Health Center (`/terminal/health`)

- Environment blockers/warnings
- Mongo ping
- Engine reachability
- OMS authority chain documentation

**File:** `HealthCenter.tsx`

---

## Diagnostics Center (`/terminal/diagnostics`)

- Engine diagnostics proxy: `/api/paper-desk/diagnostics`
- Env blockers + worker cron reference

**File:** `DiagnosticsCenter.tsx`

---

## Status: IMPLEMENTED
