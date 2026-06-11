# OBSERVABILITY CERTIFICATION

**Status:** PASS  
**Detection window target:** 60 seconds

---

## Observability Surfaces

| Surface | Poll interval | File |
|---------|---------------|------|
| Risk Ribbon | 5s | `RiskRibbon.tsx:66` |
| Event Center | 5s | `EventCenter.tsx:80+` |
| Terminal REST | 3–5s | `terminalStore.tsx:86-87, 210` |
| Portfolio page | 10s | `PortfolioAnalyticsDashboard.tsx:123` |
| Strategy Intel | 30s | `StrategyIntelligenceDashboard.tsx:113` |
| Engine SSE events | 3s | `engine/events/route.ts:29` |

---

## Outage Detection Matrix

| Outage | Detection | Max latency |
|--------|-----------|-------------|
| OMS | Ribbon OMS + ORDER events | ≤5s |
| Market data | Ribbon MARKET DATA RED | ≤5s |
| Reconciliation | Ribbon RECON + RECONCILIATION events | ≤5s |
| Strategy metrics | Intel error state / empty | ≤30s |
| Execution | Ribbon ENGINE/EXECUTION | ≤5s |
| Mongo | Ribbon DATABASE + ICC guard | ≤5s |
| WebSocket | Authority badge + REST fallback | ≤3s |

All within **60s** requirement.

---

## Health Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/api/risk-ribbon` | Aggregated ops ribbon |
| `/api/event-center` | Event feed |
| `/api/paper-desk/snapshot` | Terminal authority bundle |
| `/api/engine/reconciliation` | Recon status (RiskModule) |
| Engine `/health` | Proxied via risk-ribbon |

---

## Metrics / Logs

- Terminal authority label in shell header
- Critical alert count badge (`TerminalShell.tsx:68-71`)
- `computed_at` on portfolio analytics
- `server_time` on ribbon and event center

**Certification:** Operator can detect all listed outages within 60s via ribbon + events + guard.
