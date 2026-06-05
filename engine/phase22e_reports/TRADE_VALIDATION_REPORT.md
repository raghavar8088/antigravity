# TRADE VALIDATION REPORT — Phase 22E

---

Generated: 2026-06-05T14:40:36Z
Period: 2026-01-01 → 2026-11-09

## 1. Trade Volume

| Checkpoint         | Required | Actual | Status |
|:-------------------|:--------:|:------:|:------:|
| 500-Trade Gate     | 500      | 1250   | ✅ PASS |
| 1000-Trade Gate    | 1000     | 1250   | ✅ PASS |
| Statistical Sig.   | z > 1.96 | —      | ✅ PASS |

## 2. Trade Distribution

- Total Trades: **1250**
- Winning Trades: **665** (53.2%)
- Losing Trades: **585** (46.8%)
- Average Hold Time: **74.9 min**
## 3. Return Distribution

- Skewness: **-0.07** (near-symmetric)
- Excess Kurtosis: **-1.76** (platykurtic — thin tails)
- 95% VaR per trade: **$112.32**
- 95% CVaR (Expected Shortfall): **$132.45**
## 4. Outlier Analysis

- Top 5% of trades contribute **13.4%** of gross profit
- Bottom 5% of trades contribute **15.7%** of gross losses
## 5. Monthly Trade Distribution

| Month | Trades | Net P&L | Status |
|:------|:------:|--------:|:------:|
| Jan 2026 | 124 | $3895.62 | ✅ PASS |
| Feb 2026 | 112 | $2774.13 | ✅ PASS |
| Mar 2026 | 124 | $-90.28 | ❌ FAIL |
| Apr 2026 | 120 | $2162.98 | ✅ PASS |
| May 2026 | 124 | $2078.97 | ✅ PASS |
| Jun 2026 | 120 | $3526.87 | ✅ PASS |
| Jul 2026 | 124 | $3672.56 | ✅ PASS |
| Aug 2026 | 124 | $3922.02 | ✅ PASS |
| Sep 2026 | 120 | $-725.13 | ❌ FAIL |
| Oct 2026 | 124 | $4100.15 | ✅ PASS |
| Nov 2026 | 34 | $255.19 | ✅ PASS |

Consecutive Positive Months: **5** ✅ PASS
