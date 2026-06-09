# Position Sizing Report

**Audit date:** 2026-06-09

---

## Sizing Systems Identified

| System | Location | Active Path |
|--------|----------|-------------|
| Kelly sizing (Risk V2) | `engine/internal/risk/v2/kelly.go` | Go institutional path |
| Kelly cap (Risk V3) | `engine/internal/riskv3/portfolio_risk.go` | PMS layer |
| Dynamic sizing | `engine/internal/risk/v2/dynamic_sizing.go` | Regime multipliers |
| Client Kelly-lite | `client/src/lib/strategyAllocation.ts` | Paper desk |
| Equity-curve sizing | `client/src/lib/futuresPaperMath.ts:371` | Paper desk multiplier |
| Drawdown lock | `useBTCFuturesScalperEngine.ts:1882–1888` | Paper desk entries |
| PMS risk budget | `loop.go:443–451` | Go portfolio gate |

---

## 1. Kelly Sizing (Go Engine — Risk V2)

**File:** `engine/internal/risk/v2/kelly.go`

### Formula
```
b = avgWin / avgLoss
p = clamp(winRate, 0, 1)
fullKelly = (b*p - q) / b   where q = 1-p
fullKelly = clamp(fullKelly, 0, maxRiskPct/100)
halfKelly = full * 0.5
quarterKelly = full * 0.25
```

### Selection Logic
- Default: **Half Kelly**
- If `stability < 0.45` OR `ddRisk > 0.6` → **Quarter Kelly**

### Inputs
| Input | Source |
|-------|--------|
| `equityUSD` | `o.exec.GetEquityUSD()` (refreshed before sizing `loop.go:353`) |
| `maxRiskPct` | Default 2% |
| `metrics` | Strategy tracker stats (win rate, avg win/loss, Sharpe, drawdown) |
| `req.RequestedSizeBTC` | Signal `TargetSize` — capped to `min(kellySize, requested)` |

### Outputs
| Output | Field |
|--------|-------|
| Recommended BTC size | `RecommendedSizeBTC` |
| Risk USD | `RecommendedRiskUSD` |
| Selected fraction | `SelectedFraction` |

### Limits
| Limit | Value | Evidence |
|-------|-------|----------|
| Full Kelly cap | `maxRiskPct/100` (default 2%) | `kelly.go:49` |
| Size cap vs request | `min(size, req.RequestedSizeBTC)` | `kelly.go:67–68` |
| Test proof | `TestKellySizeCapsAtTwoPercentAndAvoidsFullKelly` | `engine_test.go:5` |

### Validation
| Check | Verdict | Evidence |
|-------|---------|----------|
| No negative sizing | **PASS** | `clamp(full, 0, maxRiskPct/100)` |
| No oversized positions | **PASS** | Capped at equity × maxRiskPct |
| No leverage overflow | **PARTIAL** | Kelly caps risk USD; leverage set separately in signals |
| Precision bugs | **PASS** | float64; tested |

---

## 2. Kelly Cap (Risk V3 / PMS)

**File:** `engine/internal/riskv3/portfolio_risk.go`

| Parameter | Value |
|-----------|-------|
| `MaxKellyFractionPct` | Tested at 5% cap |
| `MaxDrawdownPct` | Blocks orders when exceeded |
| `MaxHeatPct` | Portfolio heat limit |

**Test:** `TestCheckOrder_KellyCapApplied` — 70% win rate, 3:1 R:R → full Kelly ~57%, capped at 5%.

**Verdict:** **PASS**

---

## 3. PMS Portfolio Risk Budget

**File:** `engine/internal/trading/loop.go:443–451`

| Limit | Value |
|-------|-------|
| MaxHeatPct | 8% |
| MaxVaR95Pct | 6% |
| MaxCVaR95Pct | 9% |
| MaxDrawdownPct | 10% |
| MaxDailyLossPct | 3% |
| MaxGrossExpPct | 250% |
| MaxNetExpPct | 150% |

Blocks trade if `CheckPortfolioRisk` returns violations.

**Verdict:** **PASS** for gate existence; **FAIL** for proving live enforcement on Delta path (Delta uses contract count, not Kelly-sized BTC).

---

## 4. Client Paper Desk Sizing

### strategyAllocation.ts

| Rule | Value |
|------|-------|
| Min trades before sizing | `ALLOCATION_MIN_TRADES = 20` |
| Multiplier clamp | `[0.25, 3.0]` |
| Method | Half-Kelly from win rate + avg win/loss |
| Adaptive TP | Base × [0.8, 1.4] by vol regime |

### futuresPaperMath.ts — Equity-Curve Multiplier

| Condition | Multiplier |
|-----------|------------|
| ≥3 consecutive losses | 0.5× |
| ≥3 consecutive wins | 1.0× (capped, no Kelly blow-up) |

### Premium Notional

`PREMIUM_NOTIONAL_MULTIPLIER = 2.0` — fixed 2× capital for premium strategies.

### Drawdown Lock

`MAX_DRAWDOWN_LOCK_PCT` — pauses entries when session drawdown exceeded; recovery at `DRAWDOWN_LOCK_RECOVERY_FRAC`.

**Verdict:** **PASS** for paper desk sizing math with clamps.

---

## 5. Delta Sizing (Live)

**File:** `institutional_request.go:85–90`

```go
contracts := req.Contracts
if contracts < 1 { contracts = int(req.Size) }
if contracts < 1 { contracts = 1 }  // minimum 1 contract
```

Delta path uses **integer contracts**, not Kelly output. `TargetSize = float64(contracts) * 0.001` for ledger only.

**Bridge open:** `contracts = int(open.PremiumUSD / 100)` or `1` in buying mode (`institutional_request.go:191–203`).

| Check | Verdict |
|-------|---------|
| Kelly applied to Delta orders | **FAIL** |
| Contract sizing tied to wallet | **PARTIAL** (buying mode checks `bal < 1`) |
| Negative sizing | **PASS** (minimum 1 contract) |

---

## 6. Default Engine Signal Size

`engine/internal/strategy/scalpers.go`:
- `defaultQty = 0.10` BTC per signal
- Kelly reduces at institutional path — but raw signal still emits 0.10

**Verdict:** **PASS** — institutional path overrides.

---

## Phase 5 Conclusion

| Sizing Type | Go Paper Path | Client Paper | Delta Live |
|-------------|---------------|--------------|------------|
| Kelly | **PASS** | **PASS** | **FAIL** |
| Risk budget | **PASS** | Partial (drawdown lock) | **FAIL** |
| Volatility sizing | **PASS** (dynamic_sizing) | **PASS** (vol slippage) | **FAIL** |
| Max exposure | **PASS** (PMS) | Partial | **FAIL** |
| Drawdown controls | **PASS** (riskv3) | **PASS** | **FAIL** |

**Overall Phase 5:** **PASS** for Go paper institutional path; **FAIL** for Delta live capital sizing.
