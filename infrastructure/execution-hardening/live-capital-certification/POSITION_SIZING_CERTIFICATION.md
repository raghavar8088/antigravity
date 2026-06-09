# POSITION_SIZING_CERTIFICATION.md
## Phase 8 — Position Sizing Audit

**Audit Date:** 2026-06-09  
**Verdict: PARTIAL**

---

## Engine Risk V2 — Authoritative Institutional Sizing

### Kelly Formula

`engine/internal/risk/v2/kelly.go:39–69`:

```
b = avgWin / avgLoss
p = clamp(winRate, 0, 1)
q = 1 - p
full = (b*p - q) / b          // clamped to [0, maxRiskPct/100]
half = full × 0.5
quarter-Kelly if stability < 0.45 OR ddRisk > 0.6
riskUSD = min(equity × selected, equity × maxRiskPct/100)
sizeBTC = riskUSD / perBTC       // perBTC = PositionRiskUSD(req, 1)
sizeBTC = min(sizeBTC, RequestedSizeBTC)
```

### Position Risk (Stop Distance)

`engine/internal/risk/v2/portfolio.go:78–89`:
```
PositionRiskUSD = |EntryPrice - StopLossPrice| × sizeBTC
```

### Dynamic Sizing Multipliers

`engine/internal/risk/v2/dynamic_sizing.go`:

| Condition | Multiplier |
|-----------|------------|
| HealthScore < 50 | ×0.50 |
| Sharpe < 1.0 (≥5 trades) | ×0.75 |
| ProfitFactor < 1.2 | ×0.50 |
| Drawdown mult < 1 | ×drawdown |
| Portfolio heat mult < 1 | ×heat |
| VolatilityPct ≥ 3 | ×0.50 |
| Extreme funding | ×0.75 |
| MaxCorrelation ≥ 0.70 | ×0.50 |
| Tail risk reduce/halt | ×0.25 / ×0 |

### Final Size Merge

`engine/internal/risk/v2/engine.go:197–200`:
```
size = minPositive(kelly.RecommendedSizeBTC, dynamic.RecommendedSizeBTC)
```

### Execution Floor

`engine/internal/risk/v2/sizing.go:18–20, 51–59`:
```
MinExecutionSizeBTC = 0.01
Reject below floor via EnforceExecutionFloor
```

### Applied in Loop

`loop.go:592–637` — Risk-approved size replaces `sig.TargetSize` before submit.

---

## Exposure & Risk Limits

`engine/internal/risk/v2/limits.go:7–25`:

| Limit | Default |
|-------|---------|
| MaxRiskPerTradePct | 2% |
| MaxPortfolioHeatPct | 6% |
| BlockHeatPct | 6% |
| MaxNetExposurePct | 150% |
| MaxGrossExposurePct | 250% |
| MaxFamilyAllocationPct | 30% |
| MaxVaRPct | 6% |
| MaxCVaRPct | 9% |
| MinRiskScore | 70 |

### Heat Formula

`engine/internal/risk/v2/heat.go:27–32`:
```
heat = (Σ PositionRiskFromPosition + PositionRiskUSD(proposed)) / equity × 100
```
Block at >6%, force-reduce at >8%.

### Exposure Formula

`engine/internal/risk/v2/exposure.go:43–45`:
```
NetExposurePct  = |NetExposureUSD| / equity × 100
GrossExposurePct = GrossExposureUSD / equity × 100
```

---

## PMS Portfolio Gate (Pre-Risk V2)

`loop.go:435–481` + `pms/portfolio_risk_budget.go:118–177`:

| Cap | Value |
|-----|-------|
| MaxHeatPct | 8% |
| MaxVaR95Pct | 6% |
| MaxCVaR95Pct | 9% |
| MaxDrawdownPct | 10% |
| MaxDailyLossPct | 3% |
| MaxGrossExpPct | 250% |
| MaxNetExpPct | 150% |

```
proposedHeat = TotalHeatPct + (proposedDollarRisk / equity) × 100
```

---

## Client Paper Desk Sizing (Parallel Path)

`client/src/lib/futuresPaperMath.ts:427–434`:
```
lossPerUnit = paperEstimatedMaxLossAtStopSl(entry, sl, 1, side, takerFeePct)
riskBudget  = equityUsd × riskPctOfEquity
size        = clamp(riskBudget / lossPerUnit, minNotional, maxNotional)
```

Kelly-lite: `strategyAllocation.ts:97–105` — defensive half-Kelly capped at 25%.

Exposure cap: `useBTCFuturesScalperEngine.ts:298–307` — `MAX_POSITION_NOTIONAL_DEFAULT = 500 USD`.

**This path does NOT use engine Risk V2.**

---

## Delta Options Sizing (Divergent)

`engine/internal/delta/bridge_buy_sizing.go:89–96`:
```
spend = walletUSD × riskPct (tiered 7–12%)
contracts = clamp(round(spend / estPremium), 1, maxContracts)
```

Institutional open handler (`institutional_request.go:191–204`):
```
contracts = int(PremiumUSD / 100)  // selling mode
contracts = 1                       // buying mode (hardcoded)
```

Risk V2 sees: `TargetSize = PremiumUSD / 100000` (L174) — **not contract count**.

---

## Sizing Bypass Paths

| Path | Bypass | Evidence |
|------|--------|----------|
| Emergency flatten | All PMS/RiskV2/Kelly gates | `loop.go:409–422`, `kellyFraction=1.0` |
| Delta contract sizing | RiskV2 BTC semantics | `institutional_request.go:174, 191–204` |
| Browser paper desk | Engine sizing entirely | `useBTCFuturesScalperEngine.ts` |
| Gateway delta request | `contracts = int(req.Size)` fallback | `institutional_request.go:85–90` |

---

## Maximum Loss Limits

| Limit | Enforced? | Where |
|-------|-----------|-------|
| Per-trade risk 2% | **YES** | Risk V2 Kelly cap |
| Portfolio heat 6–8% | **YES** | Risk V2 + PMS |
| Daily loss 3% | **YES** | PMS gate |
| Drawdown 10% | **YES** | PMS gate |
| Min size 0.01 BTC | **YES** | EnforceExecutionFloor |
| Client notional $500 | **YES** (browser only) | useBTCFuturesScalperEngine |
| Delta wallet tier | **YES** (options only) | bridge_buy_sizing.go |

---

## Position Sizing Verdict

| Question | Verdict |
|----------|---------|
| Kelly sizing correct (engine)? | **PASS** |
| Risk budget enforced? | **PASS** (institutional path) |
| Exposure caps enforced? | **PASS** (institutional path) |
| Drawdown sizing? | **PASS** |
| Delta live sizing aligned with Risk V2? | **FAIL** |
| Browser desk sizing aligned? | **PARTIAL** (separate math, tested) |
| Emergency flatten bypass? | **PARTIAL** (intentional, documented) |

**Overall: PARTIAL** — Engine BTC paper sizing is institutionally sound. Delta live and browser paths diverge.
