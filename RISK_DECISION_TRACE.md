# RISK DECISION TRACE — Phase 22B

**Date:** 2026-06-04  
**Scope:** Every field recorded in a trade decision, and where it comes from

---

## Complete Decision Record Per Trade

For every trade that passes through `executeThroughInstitutionalPath`, the following is recorded:

### Strategy Identity
| Field | Source | Where Recorded |
|---|---|---|
| `Strategy` | `strategyName` (loop.go) | `RiskDecision.Kelly.SelectedFraction` log |
| `WinRate` | `StrategyStats.Wins/TotalTrades` | `BuildRiskMetrics` → `StrategyMetrics.WinRate` |
| `ProfitFactor` | `GrossWinUSD/GrossLossUSD` | `BuildRiskMetrics` → `StrategyMetrics.ProfitFactor` |
| `Sharpe` | `trackerAnnualizedSharpe(recentReturns)` | `BuildRiskMetrics` → `StrategyMetrics.Sharpe` |
| `Expectancy` | `WR×AvgWin – (1-WR)×AvgLoss` | `BuildRiskMetrics` → `StrategyMetrics.ExpectancyUSD` |
| `HealthScore` | `trackerHealthScore(stats)` | `BuildRiskMetrics` → `StrategyMetrics.HealthScore` |
| `TotalTrades` | `StrategyStats.TotalTrades` | `BuildRiskMetrics` → `StrategyMetrics.TotalTrades` |
| `MaxDrawdownPct` | Live peak-to-trough per strategy | `BuildRiskMetrics` → `StrategyMetrics.MaxDrawdownPct` |

### Kelly Decision
| Field | Source | Where Recorded |
|---|---|---|
| `FullKellyFraction` | `(b×p–q)/b` | `RiskDecision.Kelly.FullKellyFraction` |
| `HalfKellyFraction` | `full × 0.5` | `RiskDecision.Kelly.HalfKellyFraction` |
| `QuarterKellyFraction` | `full × 0.25` | `RiskDecision.Kelly.QuarterKellyFraction` |
| `SelectedFraction` | Half or Quarter | `RiskDecision.Kelly.SelectedFraction` |
| `KellyStability` | `(TotalTrades/100) × (PF/1.5)` | `RiskDecision.Kelly.KellyStability` |
| `KellyConfidence` | `(Sharpe/2 + OOS/2)/2` | `RiskDecision.Kelly.KellyConfidence` |
| `KellyDrawdownRisk` | `MaxDrawdownPct/10` | `RiskDecision.Kelly.KellyDrawdownRisk` |

### Sizing Decision Log
Each `DynamicSize` multiplier layer is recorded in `RiskDecision.Logs []SizingDecisionLog`:
```go
type SizingDecisionLog struct {
    Layer      string   // e.g. "strategy_health", "sharpe", "profit_factor"
    BeforeBTC  float64
    AfterBTC   float64
    Multiplier float64
    Reason     string
}
```

### Drawdown Decision
| Field | Source |
|---|---|
| `DrawdownPct` | `(HWM – Equity) / HWM × 100` |
| `SizeMultiplier` | Tier lookup (0, 0.10, 0.25, 0.50, 0.75, 1.0) |
| `TradingHalt` | DD ≥ 10% |
| `Severity` | NORMAL / CAUTION / WARNING / CRITICAL / ELITE_ONLY / HALT |

### Allocation Decision
| Field | Source |
|---|---|
| `RecommendedCapitalPct` | `5 + composite×20` |
| `Scores` | map with sharpe/sortino/profitFactor/expectancy/drawdown/oos/health components |
| `Allowed` | `stratPct ≤ 20% AND familyPct ≤ 30%` |

### Final Decision
| Field | Source |
|---|---|
| `RecommendedSizeBTC` | `min(Kelly.size, Dynamic.size)` |
| `RecommendedRiskUSD` | `PositionRiskUSD(req, finalSize)` |
| `RecommendedLeverage` | `RecommendLeverage(req, market, heat, drawdown, corr, limits)` |
| `RiskScore` | `100 – penalties(heat, VaR, CVaR, corr, exposure, drawdown, tail)` |
| `Approved` | All gates passed AND RiskScore ≥ MinRiskScore |

---

## Ledger Events Emitted Per Trade

```
EventOrderCreated   → order placed in ledger
EventOrderValidated → OMS v3 validated
EventRiskApproved   → risk gate approved (or EventRiskBlocked)
EventOrderSubmitted → sent to execution
EventOrderAcked     → exchange acknowledged
EventOrderFilled    → fill received (or EventOrderRejected)
EventPositionOpened → positions manager notified
EventPositionClosed → on trade close
```

All events carry `StrategyID = strategyName` and `AggregateID = clientOrderID`, forming a replayable audit trail for every trade decision.

---

## Dashboard Access

The full `RiskDashboard` is available at `GET /api/engine/risk/dashboard` and contains:
- Per-strategy metrics (once `StrategyMetrics` are populated via `Engine.ValidateTrade`)
- Heat history (last 1,500 snapshots)
- VaR/CVaR history
- Recent risk decisions (last 2,000)
- Portfolio state
