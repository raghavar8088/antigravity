# PHASE 14 — CAPITAL EFFICIENCY REPORT

**Date:** 2026-06-10

---

## Capital Utilization Analysis

### Go Engine — Paper Account ($1M)

| Metric | Value | Evidence |
|:-------|:-----:|:---------|
| Account size | ~$1M | BTC paper desk config |
| Default position size | 0.10 BTC × ~$100k/BTC = ~$10k | `defaultQty = 0.10` |
| Max concurrent positions | Unknown | Position manager limit not found in evidence |
| Typical deployed capital | Unknown | MongoDB not accessible |
| Idle capital | Unknown | Not computed |

**Documented net PnL on $1M account: ~-$62 (known trades)**  
**Return on capital (known trades): -0.006%** — essentially flat, statistically indistinguishable from zero.

### Capital Concentration Issue

With 25 signals per batch and 0.10 BTC position:
- Maximum concurrent deployment: 25 × 0.10 BTC × ~$100k = $250k
- On a $1M account: maximum 25% deployed at any time
- In practice: far fewer concurrent positions (most signals don't result in simultaneous fills)
- **Estimated average capital utilization: 5-10%**

A 5-10% capital utilization on a $1M paper account with near-zero return means the platform is deploying $50-100k effectively while 90-95% sits idle. This is extremely capital-inefficient.

### Client Desk — Leverage Utilization

| Metric | Value |
|:-------|:-----:|
| Leverage | 25× |
| Account (implied) | Starting from $1,000 in replay |
| Effective exposure per trade | 25× position notional |
| Max drawdown risk at 25× | 4% adverse move = 100% position loss |

25× leverage with 0.50% SL = 12.5% position loss per SL hit. This is aggressive for unvalidated strategies but survivable with the 3:1 RR structure.

---

## Risk Concentration Analysis

### Current Risk Concentration Points

| Concentration | Risk Level | Evidence |
|:-------------|:----------:|:---------|
| 100% BTC-USD exposure | HIGH | All strategies trade single asset |
| 1m scalping only (Go) | HIGH | 85%+ of strategies are 1m |
| Long bias (most strategies) | MEDIUM | Most entries are buy-side scalps |
| Single market (crypto only) | HIGH | No equity, FX, or other asset |
| Single exchange (price feed) | MEDIUM | Coinbase primary with Binance fallback |
| Single strategy type (scalping) | HIGH | 95%+ of strategies are scalpers |

The platform has **extremely concentrated risk** — it is a BTC scalping operation that happens to have 714 strategy names.

### Kill Switch as Capital Concentration Risk

The June 2026 48-hour kill switch outage demonstrated that a single infrastructure failure can block 100% of capital deployment simultaneously. With diversified exchange connectivity and capital allocation, a single broker failure affects only 20-30% of capital.

**Risk concentration in the execution infrastructure is as dangerous as risk concentration in positions.**

---

## Leverage Efficiency

### Go Engine

At 0.10 BTC position on a $1M account:
- Position notional: $10,000 (1% of capital)
- No leverage applied (paper spot or futures at 1:1)
- Capital efficiency: 1% per position, 25% maximum at 25 concurrent

This is actually **under-levered** for a scalping operation. Institutional scalpers typically deploy 50-70% of capital in concurrent positions.

### Client Desk

At 25× leverage:
- The effective capital utilization per trade is 1/25 = 4% of notional
- This IS capital-efficient in terms of margin usage
- But 25× on unvalidated strategies is leverage applied before validation — the wrong order of operations

---

## Idle Capital Problem

At any given moment, approximately 75-90% of the $1M paper account is idle. This capital:
- Earns no return (paper environment, no interest)
- Contributes to opportunity cost

**Recommendation:** In a live account, idle capital should be deployed in:
1. Short-duration treasury positions (risk-free rate)
2. Stablecoin yield instruments (low counterparty risk)
3. Basis trade (delta-neutral BTC/futures carry)

For paper trading, this is irrelevant — but for live deployment, a $1M account generating 5% risk-free on idle capital earns $50k/year before any trading strategy contributes. This baseline return should be the floor target.

---

## Capital Efficiency Metrics (Proposed)

| Metric | Formula | Target |
|:-------|:--------|:------:|
| Capital utilization | Deployed capital / Total capital | 40-60% |
| Return on deployed | Net PnL / Average deployed capital | ≥ 30% annualized |
| Return on equity | Net PnL / Total capital | ≥ 15% annualized |
| Win rate × RR product | WR × (avg_win / avg_loss) | > 1.0 |
| Sharpe on deployed | Return_deployed / Std_deployed | ≥ 2.0 |

---

## Phase 14 Verdict

**FAIL — capital efficiency is unmeasurable due to missing data, and the structural deployment model is inefficient.**

**Three immediate capital efficiency improvements:**
1. Remove 389 retire-priority strategies → reduce noise, free aggregator capacity for quality signals
2. Increase allocation to top 5 proven strategies from 0.10 BTC to 0.15-0.20 BTC
3. After alpha fix: route all capital freed from retired strategies to the 10-strategy validated portfolio
