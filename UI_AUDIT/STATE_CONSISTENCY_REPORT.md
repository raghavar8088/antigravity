# PHASE 11 — STATE CONSISTENCY REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Do displayed states match backend states?

---

## STATE SOURCES INVENTORY

| State Layer | Source | Sync Mechanism |
|-------------|--------|---------------|
| Terminal snapshot | `initialTerminalSnapshot` | WebSocket (NOT CONNECTED) |
| Paper desk state | MongoDB Atlas | 5s HTTP poll |
| Paper positions | MongoDB Atlas | 5s HTTP poll |
| Paper trades | MongoDB Atlas | 5s HTTP poll + lazy load |
| Paper orders (OMS) | MongoDB Atlas | Lazy load (on tab open) |
| In-browser signal engine | Local state machine | 5s poll signals API |
| Mock trading | Local state + 15s persist | Polling |
| Go engine state | SQLite + MongoDB | Not directly polled by UI |

---

## CONSISTENCY ANALYSIS

### Terminal State vs Backend State
**Verdict: INCONSISTENT — always diverged**

Evidence:
- Terminal snapshot never updates (WS not connected)
- `initialTerminalSnapshot.connected: true` — falsely claims connection
- Any backend state change (new position opened, risk level changed, strategy disabled) is invisible on terminal pages
- The displayed state and backend state are structurally disconnected

### Paper Desk State vs MongoDB State
**Verdict: CONSISTENT (with 5s lag)**

Evidence:
- `usePaperDesk()` polls `/api/paper-desk/snapshot` every 5s
- Server-side handler queries MongoDB `paper_state` and `paper_positions` collections
- 5s staleness is acceptable for non-HFT monitoring
- The connection state badge (`live/stale/error`) correctly reflects actual connectivity

**Caveat**: OMS orders are lazy-loaded. If the trader never opens the OMS tab, OMS state is not displayed. An order that sits pending for hours would not be noticed unless the tab was clicked.

### Signal Engine State vs Go Engine State
**Verdict: UNKNOWN — two separate engines**

Evidence:
- In-browser engine (`useBTCFuturesScalperEngine`) generates signals from TypeScript logic
- Go engine (`engine/internal/strategy/`) generates signals from Go logic
- These are independent implementations of similar strategy logic
- There is no mechanism to verify they agree
- Signals displayed in Signal Trace Panel come from the in-browser engine, not the Go engine
- **A trader sees in-browser signals but the Go engine may be generating different signals or none at all**

### Risk State vs Go Engine Risk Gate
**Verdict: INCONSISTENT — completely separate**

Evidence:
- Terminal Risk Module: mock data
- Go engine risk gate (`engine/internal/risk/gate/`): real risk calculations
- These two never communicate
- **A trader monitoring the terminal risk page has zero information about Go engine's actual risk state**

### Kill Switch State
**Verdict: INCONSISTENT — UI cannot determine actual state**

Evidence:
- Kill switch button triggers a POST to Go engine
- No component polls kill switch status after triggering
- No component polls `/api/admin/ks/status` periodically
- If the engine crashes and restarts, kill switch state could reset without UI knowing
- UI can be in state "I pressed kill" while engine is in state "not killed" (restart cleared it)

### Position State Across Systems
**Verdict: INCONSISTENT — multiple sources of truth**

Evidence:
- MongoDB `paper_positions` collection (paper desk)
- In-browser state machine position tracking (scalper engines)
- Go engine SQLite (`SQLITE_PATH=./data/engine.db`)
- AngelOne broker positions (not fetched to UI)
- Delta Exchange positions (not fetched to UI)

No single source of truth is displayed. A "net portfolio position" calculation could produce different answers from each data source.

---

## STATE SYNCHRONIZATION GAPS

| State Pair | Synchronized? | Max Lag |
|-----------|--------------|---------|
| Paper desk state ↔ MongoDB | YES | 5s |
| Terminal state ↔ Backend | NO | Infinite (never updates) |
| Signal state ↔ Go engine | NO | Undefined |
| Risk state ↔ Go risk gate | NO | Undefined |
| Kill switch UI ↔ Engine | NO | Undefined |
| OMS state ↔ MongoDB | PARTIAL | Lazy-load only |
| Position state ↔ Broker | NO | No polling |

---

## STATE CONSISTENCY SCORECARD

| Dimension | Score |
|-----------|-------|
| Terminal-backend sync | 0/10 — WS not wired |
| Paper desk-MongoDB sync | 8/10 — 5s lag acceptable |
| Signal state agreement | 0/10 — two independent engines |
| Risk state agreement | 0/10 — mock vs real |
| Kill switch state sync | 0/10 — fire-and-forget |
| Cross-system consistency | 1/10 — no unified state layer |

**Overall Score: 1.5/10 — State consistency is critically deficient except for paper desk**

**Verdict: The terminal (which is the primary/home UI) shows permanently stale state. The paper desk is the only subsystem with genuine state consistency. All other state pairs are either disconnected or unknown.**
