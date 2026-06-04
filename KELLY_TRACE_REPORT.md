# KELLY TRACE REPORT — Phase 22B

**Date:** 2026-06-04  
**Scope:** End-to-end trace of the Kelly sizing path, from performance data through to executed trade size

---

## Kelly Implementation Location

**File:** `engine/internal/risk/v2/kelly.go`  
**Function:** `KellySize(req TradeRequest, metrics StrategyMetrics, equityUSD, maxRiskPct float64) KellyDecision`

### Algorithm

```
b  = AverageWin / AverageLoss          (reward/risk ratio)
p  = WinRate (clamped 0–1)
q  = 1 – p
f* = (b*p – q) / b                     (full Kelly fraction)
f* = clamp(f*, 0, maxRiskPct/100)      (cap at max risk per trade)
f½ = f* × 0.5                          (half-Kelly, default mode)
f¼ = f* × 0.25                         (quarter-Kelly, for unstable strategies)

stability  = clamp(TotalTrades/100, 0,1) × clamp(ProfitFactor/1.5, 0,1)
confidence = clamp((Sharpe/2 + OOSProfitFactor/2)/2, 0,1)
ddRisk     = clamp(MaxDrawdownPct/10, 0,1)

selected   = f½  (default)
selected   = f¼  if stability < 0.45 OR ddRisk > 0.6

riskUSD    = min(equityUSD × selected, equityUSD × maxRiskPct/100)
sizeBTC    = riskUSD / PositionRiskUSD(req, 1)
sizeBTC    = min(sizeBTC, req.RequestedSizeBTC)
```

---

## Pre-Fix State: Kelly Was Receiving Fake Inputs

```
WinRate      = 0.50  → b*p – q = b×0.5 – 0.5 → f* = 0 for any b ≤ 1
ProfitFactor = 1.20  → stability = (30/100) × (1.2/1.5) = 0.30×0.80 = 0.24
Sharpe       = 1.20  → confidence = (0.6 + 0.55)/2 = 0.575
TotalTrades  = 30    → stability factor capped at 0.30
HealthScore  = 70    → not consumed by Kelly directly
```

**Result:** With fake WinRate=0.5, AverageWin=0, AverageLoss=0:  
- `b = 1.0` (equal win/loss fallback)  
- `f* = (1×0.5 – 0.5)/1 = 0`  
- Kelly size = 0 → falls back to fallback path → size = RequestedSizeBTC (unchanged)  
- Kelly was effectively disabled — every strategy got the fixed 1% base size regardless of quality.

---

## Post-Fix State: Kelly Receives Real Inputs

### Cold Start (0 trades)
```
WinRate      = 0.5   (neutral prior)
ProfitFactor = 1.0   (no data)
AverageWin   = 0
AverageLoss  = 0
TotalTrades  = 0
Sharpe       = 0     (< 5 samples)
MaxDrawdown  = 0
```
→ `stability = 0 × 0 = 0` → selects quarter-Kelly → very small fraction  
→ Kelly naturally constrains cold-start strategies to minimal size ✓

### Proven Winner (example: 65% WR, PF 1.8)
```
WinRate      = 0.65
AverageWin   = $8.50
AverageLoss  = $5.20
b            = 8.50/5.20 = 1.635
f*           = (1.635×0.65 – 0.35)/1.635 = (1.063–0.35)/1.635 = 0.436
               capped at maxRiskPct/100 = 0.02 (2%)
f½           = 0.01  → 1% of equity = $10,000 on $1M account
stability    = (trades/100 capped) × (1.8/1.5 capped) = 0.60 × 1.0 = 0.60
selected     = f½  (stability ≥ 0.45, ddRisk < 0.6)
sizeBTC      = $10,000 / PositionRiskUSD(req,1)
```
→ Strategy gets full half-Kelly allocation ✓

### Losing Strategy (example: 38% WR, PF 0.75)
```
WinRate      = 0.38
AverageWin   = $4.00
AverageLoss  = $6.00
b            = 4.00/6.00 = 0.667
f*           = (0.667×0.38 – 0.62)/0.667 = (0.253–0.62)/0.667 = −0.55
               clamped to 0  → no Kelly allocation
```
→ `sizeBTC = 0` → riskDecision.RecommendedSizeBTC = 0 → **trade blocked** ✓

---

## Kelly Output → Execution Trace

### Pre-Fix (BROKEN)
```
KellySize() → KellyDecision.RecommendedSizeBTC
                   ↓
            RiskDecision.RecommendedSizeBTC   ← computed but NEVER APPLIED
                   ↓
            sig.TargetSize  ← unchanged (fixed 1% base)
                   ↓
            o.exec.ExecuteSignal(sig, mode)   ← always used base size
```
**Kelly output was discarded at execution.** The entire sizing computation was dead code.

### Post-Fix (CORRECT)
```
o.tracker.BuildRiskMetrics(strategyName)
        ↓  real WinRate, PF, Sharpe, MaxDD, TotalTrades
KellySize(req, metrics, equity, maxRiskPct)
        ↓  computes f*, half/quarter selection
DynamicSize(req, metrics, market, heat, drawdown, corr, tail)
        ↓  applies multipliers (health, Sharpe, PF, drawdown, heat, regime, correlation, tail)
RiskDecision.RecommendedSizeBTC = min(kelly.size, dynamic.size)
        ↓  APPLIED to sig.TargetSize ← NEW
o.exec.ExecuteSignal(sig, mode)
        ↓  executes at Kelly-approved, dynamically-adjusted size
```

---

## Kelly Activation Status

| Component | Pre-Fix | Post-Fix |
|---|---|---|
| Kelly receives real WinRate | ✗ | ✓ |
| Kelly receives real ProfitFactor | ✗ | ✓ |
| Kelly receives real AverageWin/Loss | ✗ | ✓ |
| Kelly receives real TotalTrades | ✗ | ✓ |
| Kelly receives real Sharpe | ✗ | ✓ |
| Kelly receives real MaxDrawdown | ✗ | ✓ |
| Kelly output applied to trade size | ✗ | ✓ |
| Winners get more capital via Kelly | ✗ | ✓ |
| Losing strategies blocked by Kelly | ✗ | ✓ |
