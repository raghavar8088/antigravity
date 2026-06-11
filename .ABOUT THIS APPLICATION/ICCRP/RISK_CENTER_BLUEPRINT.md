# RISK CENTER BLUEPRINT — ICCRP V3

**Permanent Ribbon:** `RiskRibbon.tsx` embedded in `TerminalShell.tsx` L59-61  
**Full Page:** `/terminal/risk` → `RiskModule.tsx`  
**API:** `/api/risk-ribbon`

---

## Ribbon Items (Implemented)

| Label | Source | Status Logic | File |
|-------|--------|--------------|------|
| MARKET DATA | `/api/btc/price` | GREEN if price valid | `risk-ribbon/route.ts` L47-52 |
| ENGINE | Go `/health` | GREEN if 200 | L54-69 |
| EXECUTION | Engine health mirror | L70 |
| WATCHDOG | Engine health mirror | L71 |
| DATABASE | Mongo configured + state | L73-145 |
| OMS | Order freshness | L114-124 |
| RECON | State snap age | L109-110 |
| PORTFOLIO RISK | Drawdown thresholds | L105-107 |
| KILL SWITCH | `/api/killswitch/status` | L196-217 |
| TODAY PnL | `getTodayRealizedPnlUtc` | L178-187 |
| EXPOSURE | Equity | L189-194 |

Colors: GREEN / AMBER / RED — `RiskRibbon.tsx` L20-38

---

## Risk Module Page

| Section | Content | File |
|---------|---------|------|
| VaR 95/99, CVaR | `snapshot.risk` | `RiskModule.tsx` L36-40 |
| Portfolio Heat bar | `heatPct` | L42-57 |
| Correlation matrix | API fetch L21 | L59-80 |
| Reconciliation | `/api/engine/reconciliation` | L22-27 |

---

## Status: IMPLEMENTED — ribbon permanent in shell
