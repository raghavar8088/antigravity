# OMS, EXECUTION, MOCK BROKER, DATABASE, RISK, EVENT LOOP — Consolidated Forensic Reports

---

## OMS FORENSIC REPORT

**Path:** Signal → `executeThroughInstitutionalPathWithFill` (`loop.go:346`) → `EventOrderCreated` → `omsv3.Replay` → PMS → `PreTradeRiskPipeline` → `submitInstitutionalOrder`.

**Outage behavior:** Orders reached OMS event creation until kill switch active; thereafter `DecisionBlocked` at `pipeline.go:51–54` with `EventRiskBlocked`.

**Standalone PaperOMS** (`execution/paper_oms.go`) serves `/paper/` HTTP — not orchestrator fill path.

---

## EXECUTION ENGINE REPORT

**Orchestrator** runs in `safeGo` with panic restart (`main.go:2373–2390`).

**No deadlock evidence** — kill switch blocks synchronously at risk gate, not channel starvation.

**Queue:** Direct goroutine per strategy group; no external order queue.

---

## MOCK BROKER REPORT

**Primary:** `execution/paper.go` `PaperClient` — $1M virtual account (`main.go:469`).

**Fill path:** `ExecuteSignal` → `applyFill` → updates `balanceUSD`, `positionBTC`.

**Last fill:** Query Mongo `paper_trades` sorted by `closed_at` / `opened_at` — requires production DB.

**Verdict:** Broker functional; not reached when kill switch active.

---

## DATABASE FORENSIC REPORT

| Store | Purpose | Outage Impact |
|-------|---------|---------------|
| PostgreSQL | Ledger events, kill switch | `EventKillSwitchTriggered` persisted |
| MongoDB | `paper_trades`, `paper_state` | No new trades written during outage |
| SQLite | Local engine fallback | Secondary |
| Redis | Indicator cache | Not execution-critical |

**No schema corruption identified.** Stuck state: kill switch `active=true` in memory + ledger events.

---

## RISK BLOCKING REPORT

| Layer | Blocker | Active During Outage? |
|-------|---------|----------------------|
| Kill switch | `pipeline.go:51` | **YES — primary** |
| PMS | `loop.go:435` | Per-trade only |
| Risk V2 | `pipeline.go:46–79` | After kill switch check |
| Legacy risk | `loop.go:1546` | Per-signal |
| Aggregator | `aggregator_selective.go` | Reduces signal count |

---

## EVENT LOOP FORENSIC REPORT

| Loop | File | Dead? |
|------|------|-------|
| Orchestrator.Run | `loop.go:969` | NO |
| Candle processors | `loop.go:1049+` | NO |
| Recon scheduler | `scheduler.go:62` | NO — caused outage |
| State snapshotter | `main.go:770` | NO |
| Options engines | `main.go:899+` | NO (separate accounts) |

**No silent panic swallow** on orchestrator — `safeGo` logs and restarts after 5s.
