# PHASE 5 — EXIT FORENSICS

**Generated:** 2026-06-10  
**Verdict:** PARTIAL — exit mechanics proven in code; MFE/MAE production data unavailable

---

## Exit Architecture

### Go Engine

**All exits via `positions.Manager.CheckStopLossAndTakeProfit()`** — called every tick (`manager.go:190-258`).

| Exit Type | Trigger | Code |
|:----------|:--------|:-----|
| Take Profit | Price crosses TP level | `checkLongPosition:215`, `checkShortPosition:241` |
| Stop Loss | Price crosses SL level | `checkLongPosition:225`, `checkShortPosition:251` |
| Kill switch flatten | `OMS_DESYNC` / manual | `killswitch_executor.go` |
| Time exit | **NOT IMPLEMENTED** in Go manager | Strategies have no TIME exit |

**Critical gap:** Go engine has **no time-based exit**. Client desk has `holdMinutes` TIME exits. Asymmetric lifecycle.

### Client Desk

**`paperResolveHardExit()` in `futuresPaperMath.ts`:**

| Exit Reason | Mechanism |
|:------------|:----------|
| SL | Hard stop at slPct |
| TP | Take profit at tpPct |
| TIME | holdMinutes exceeded |
| TRAIL | Trailing stop |
| BREAKEVEN | Breakeven move |
| PROFIT_LOCK | Lock partial gains |
| MOM_DECAY | Momentum decay exit |
| LIQUIDATION_RISK | Proximity to liq price |

---

## Client Replay Exit Distribution (Evidence)

**500 bars, 48 strategies, 113 trades:**

| Exit Reason | Count | % | Interpretation |
|:------------|------:|--:|:---------------|
| PROFIT_LOCK | 74 | 65.5% | Winners protected before full TP |
| SL | 38 | 33.6% | Losers stopped |
| MOM_DECAY | 1 | 0.9% | Momentum fade |

**Inference:** Exit system captures gains via PROFIT_LOCK (not raw TP). 1/3 of trades hit SL.

---

## Are Winners Cut Early?

### Go Engine

| Factor | Effect | Evidence |
|:-------|:-------|:---------|
| No trailing stop in manager | Winners run to TP only | `manager.go` — no trail logic |
| `sanitizeSignalForProfit` TP floor | Prevents exits < 0.10% | `loop.go:2229-2232` |
| Tight SL (0.10-0.20%) | Noise stops cut winners pre-TP | `loop.go:2204-2208` |
| No partial TP | Full close at TP | `manager.go:214` comment |

**Verdict:** Winners are NOT cut early by trailing — but **noise SL stops convert would-be winners to losers** on 1m scalps.

### Client Desk

PROFIT_LOCK exits (65.5% of replay trades) suggest **winners are locked before full TP** — potentially leaving profit on table but protecting gains.

---

## Are Losers Allowed to Run?

| Stack | SL Enforcement | Evidence |
|:------|:---------------|:---------|
| Go | Tick-level SL check | Every tick `CheckStopLossAndTakeProfit` |
| Client | Bar-level SL check | Per poll tick |

**Go:** SL checked every ~250ms tick — losers contained.  
**Client:** SL checked per poll (1m) — up to 60s delay on SL execution.

**Client SL% range:** 0.50-0.55% vs Go sanitized 0.10-0.20%. Client allows wider loss before stop.

---

## MFE / MAE / Exit Efficiency

| Metric | Go Production | Client Replay | Status |
|:-------|:-------------|:--------------|:------:|
| MFE (max favorable excursion) | Not stored per trade | Not computed in replay output | **FAIL** |
| MAE (max adverse excursion) | Not stored per trade | Not computed | **FAIL** |
| Exit efficiency (captured/MFE) | `/api/paper-trades/analytics` computes proxy | Not available | **FAIL** |
| Institutional schema | `core.positions.max_favorable_excursion` defined | Not populated locally | **FAIL** |

**Cannot certify exit efficiency without production trade excursion data.**

---

## Forced / Risk Exits

| Exit | Go | Client | Evidence |
|:-----|:--:|:------:|:---------|
| Kill switch flatten | ✅ | N/A | Outage 2026-06-08-10 |
| Risk gate rejection (pre-entry) | ✅ | ✅ | Not an exit but blocks entry |
| Drawdown lock | N/A | ✅ (optional) | `drawdownLock` replay flag |
| Liquidation risk | N/A | ✅ | `LIQUIDATION_RISK` exit reason |

---

## Exit Forensics Verdict

| Question | Answer |
|:---------|:-------|
| Are exits correct? | **PARTIAL** — mechanics sound; Go lacks TIME exit |
| Winners cut early? | **PARTIAL** — Go: noise SL; Client: PROFIT_LOCK |
| Losers run? | **PASS** — SL enforced both stacks |
| Exit efficiency measurable? | **FAIL** — no MFE/MAE production data |
