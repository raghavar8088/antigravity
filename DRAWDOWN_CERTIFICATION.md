# DRAWDOWN CERTIFICATION — Phase 22E

---

Generated: 2026-06-04T03:21:01Z

## Methodology

**Max Drawdown = (Peak Cumulative P&L − Trough) / Peak × 100**
Computed on the cumulative net P&L series across all trades.
Must remain below 10% to qualify for live deployment.

## Drawdown Metrics

| Metric                  | Value    |
|:------------------------|:--------:|
| **Max Drawdown**        | **0.14%** |
| Required (< limit)      | < 10%   |
| 95% VaR per trade       | $112.32   |
| 95% CVaR per trade      | $132.45   |
| Return Skewness         | -0.07     |
| **Certification**       | **✅ PASS** |

## Drawdown Controls Validated

- Kill switch engaged if intraday loss > 2% of capital
- Position sizing uses ½-Kelly to limit individual position risk
- Portfolio heat limit: maximum 6 concurrent positions
- Capital preservation: no new positions when drawdown > 5%

## Certification Verdict

**✅ Max Drawdown CERTIFIED: 0.14% (limit < 10%)**
