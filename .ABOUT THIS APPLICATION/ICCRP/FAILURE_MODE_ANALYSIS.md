# FAILURE MODE ANALYSIS

**Status:** PASS (ICC path)

---

## Simulated Failures

### WebSocket loss

| Step | User sees |
|------|-----------|
| WS closes | `terminalStore.tsx:146` → WS_CLOSE |
| REST poll | `fetchRestAuthority()` every 3s |
| Success | Badge: REST AUTHORITY |
| 3 REST fails | `REST_UNAVAILABLE` → guard blocks content |
| Ribbon | Still polls independently every 5s |

**Misleading UI?** No — guard + badge + ribbon.

---

### REST failure

Same as above. Shell shows `—` for price (`TerminalShell.tsx:46`).

---

### Mongo outage

| Step | User sees |
|------|-----------|
| getPaperState throws | risk-ribbon DATABASE RED (`route.ts:117-121`) |
| snapshot API fails | Terminal guard: BACKEND AUTHORITY UNAVAILABLE |
| platformEvents | Empty array; event center empty |
| Portfolio page | Error banner line 137-144 |

**Misleading UI?** No.

---

### SEP outage

SEP not rendered in ICC shell nav. No operator impact on `/terminal/*`.

---

### OMS outage

| Step | User sees |
|------|-----------|
| Engine down | OMS RED (route.ts:116-117) |
| Engine up, stale orders + open positions | OMS AMBER (route.ts:119-120) |
| Events | ORDER stream stops updating |

**Misleading UI?** No (after OMS ribbon fix).

---

### Market data outage

Ribbon MARKET DATA RED OFFLINE (`route.ts:47-51`).

---

### Reconciliation failure

| Step | User sees |
|------|-----------|
| State stale >120s | RECON STALE + RECONCILIATION WARNING event |
| No state | RECON AMBER + CRITICAL reconciliation event |

---

## Synthetic Value Reachability

| Scenario | Synthetic displayed? |
|----------|---------------------|
| Pre-authority ICC | No — guard blocks |
| Post-authority, metric missing | `—` |
| Legacy `/` home | **Redirected** to ICC (`page.tsx:3-4`) |

**Certification:** UI cannot mislead operator on ICC path during failures.
