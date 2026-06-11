# RISK COMMAND CENTER CERTIFICATION — ICCF-LDAP Phase 9

---

## Surfaces

1. `/terminal/risk` — `RiskModule.tsx`
2. Global — `RiskRibbon.tsx` via `layout.tsx:41`
3. `/api/risk-ribbon` — server aggregation

---

## Indicator Matrix

| Required Indicator | Terminal Risk Page | Risk Ribbon (Global) | Backend Source | Verdict |
|--------------------|-------------------|----------------------|----------------|---------|
| Market Data | **NO** | YES — `/api/btc/price` | `risk-ribbon/route.ts:45-51` | PARTIAL |
| OMS | **NO** | YES (weak) | `listPaperOrders` L91-95,115 | PARTIAL |
| Reconciliation | YES | YES | `/api/engine/reconciliation` | PASS |
| Kill Switch | **NO** | YES | Go `/api/killswitch/status` L172-189 | PARTIAL |
| Watchdog | **NO** | YES (mirrors engine health) | L70 | PARTIAL |
| Execution Health | **NO** | YES (mirrors engine) | L69 | PARTIAL |
| Portfolio Risk | YES (heat, DD) | YES | Mongo state L97-107 | PASS |
| Daily Drawdown | YES | YES | `state.current_drawdown` | PASS |
| Max Drawdown | Partial (current only in heat card) | YES | state + portfolio | PARTIAL |
| Exposure | YES | YES (equity label) | portfolio exposure | PASS |
| VaR/CVaR | YES | NO | portfolio fields (often 0) | PARTIAL |
| Correlation | YES | NO | correlation-matrix API | PASS |

---

## Risk Module Detail (`RiskModule.tsx`)

| Component | API | Lines |
|-----------|-----|-------|
| VaR 95/99, CVaR | snapshot.risk (from portfolio) | L37-39 |
| Portfolio heat | snapshot.risk.heatPct | L42-57 |
| Correlation matrix | `/api/paper-trades/correlation-matrix` | L18-29, 59-91 |
| Reconciliation panel | `/api/engine/reconciliation` | L92, 117-147 |
| Open position funding risk | snapshot.positions | L98-110 |

**hasRisk gate:** `drawdownPct !== 0 || grossExposureUsd > 0` (L31) — flat account shows NO DATA (acceptable).

---

## Risk Ribbon Detail (`risk-ribbon/route.ts`)

All items server-computed with GREEN/AMBER/RED semantics.

**Issues found:**

| Issue | Lines | Severity |
|-------|-------|----------|
| TODAY PnL uses lifetime `realized_pnl` | L103, 154-158 | HIGH — mislabeled |
| OMS always GREEN if mongoOk | L115 | MEDIUM — false confidence |
| Watchdog = engine health proxy | L70 | LOW — not independent watchdog |
| Kill switch only queried if engine GREEN | L170 | MEDIUM — hides KS when engine down |

---

## Phase 9 Verdict

**FAIL for `/terminal/risk` as standalone command center** — missing 5 of 10 required live indicators on-page.

**PASS for global RiskRibbon** with noted mislabeling issues.

Operators must rely on **top-of-screen ribbon** for kill switch, OMS, market data — not the Risk page itself.

---

## Remediation

1. **P0** — Embed RiskRibbon indicators (or equivalent) inside `RiskModule.tsx`.
2. **P0** — Fix TODAY PnL: filter trades by `closed_at >= startOfUTCDay`.
3. **P1** — OMS status from `/api/paper-oms/summary` not order count heuristic.
4. **P1** — Query kill switch even when engine health fails (show UNREACHABLE explicitly).
