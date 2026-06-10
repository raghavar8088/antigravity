# PHASE 17 — CAPITAL EFFICIENCY

**Generated:** 2026-06-10  
**Verdict:** FAIL — capital deployed on unvalidated strategies; idle capital vs overdeployment coexist

---

## Capital Configuration

| Parameter | Go Engine | Client Desk |
|:----------|:----------|:------------|
| Paper account equity | $1,000,000 | $1,000 (replay) / configurable |
| Per-trade size | 0.10 BTC (~$6,500-$10,000) | 1% equity risk |
| Leverage | Implicit (spot-style PnL) | 25× |
| Max concurrent positions | 2/strategy × 25 approved | 12 (replay) |
| Kelly sizing | Yes (institutional path) | Half-Kelly |
| Family concentration cap | 30% | 35% same-direction |

---

## ROI Analysis

### Go Engine (Documented)

| Metric | Value | Assessment |
|:-------|------:|:-----------|
| Top strategy ROI | +$20 / $1M = **0.002%** | Negligible |
| Documented net (winners - losers) | -$61.96 | **Negative** |
| Portfolio ROI | **UNMEASURED** | No accessible trade history |
| Annualized ROI | **FAIL** | Cannot compute |

### Client Replay (500 bars)

| Metric | Value |
|:-------|------:|
| ROI (8.3 hours) | +10.2% |
| Annualized (extrapolated) | **MEANINGLESS** — 8.3h sample |
| Risk-adjusted ROI (Sharpe) | **FAIL** — not computed |

---

## Capital Utilization

### Go Engine

| Factor | Utilization | Evidence |
|:-------|:-----------|:---------|
| Strategies generating signals | 606 (100%) | All wired |
| Strategies receiving fills | ≤25/batch (≤4.1%) | Aggregator cap |
| Strategies with known PnL | 25 (4.1%) | Aggregator hardcoded |
| Capital per unvalidated strategy | 0.10 BTC each if approved | Equal weight |
| Kill switch utilization | **0%** during outage | 48+ hours zero deployment |

**Paradox:** 95.9% of strategies are starved of capital (good for risk) but the 4.1% that receive capital are **mostly unvalidated** (bad for return).

### Client Desk

| Factor | Utilization |
|:-------|:-----------|
| Active strategies | 48 of 108 (44%) |
| Research pool capital | 0% (disabled) |
| Max positions | 12 concurrent |
| Per-trade risk | 1% of equity |
| Premium 2× multiplier | 28 strategies get double |

---

## Idle Capital

| Scenario | Idle % | Evidence |
|:---------|:-------|:---------|
| Go: aggregator rejects batch | ~100% for that minute | Dominance/score filter |
| Go: kill switch active | **100%** | Outage 2026-06-08-10 |
| Go: normal operation | ~60-80% estimated | Max 25 positions of 606 strategies |
| Client: cooldown periods | ~40-60% estimated | 5-10 min cooldowns |
| Client: no signal above threshold | Variable | Score < 26 |

---

## Risk-Adjusted ROI

| Metric | Go | Client | Status |
|:-------|:--:|:------:|:------:|
| Sharpe ratio | **UNKNOWN** | **UNKNOWN** | **FAIL** |
| Sortino ratio | **UNKNOWN** | **UNKNOWN** | **FAIL** |
| Calmar ratio | **UNKNOWN** | **UNKNOWN** | **FAIL** |
| Max drawdown | **UNKNOWN** live | **UNKNOWN** | **FAIL** |
| Phase 22E synthetic Sharpe | 6.81 | — | **INVALID** |

---

## Capital Efficiency Score

| Dimension | Score /10 | Rationale |
|:----------|:---------:|:----------|
| ROI on deployed capital | 1 | +$20 best strategy on $1M |
| Capital allocation to winners | 2 | Winners get boost but equal base size |
| Idle capital management | 5 | Aggregator prevents overdeployment |
| Risk-adjusted return | 1 | Unmeasured |
| Scaling efficiency | 1 | 606 strategies, 25 active |

**Capital Efficiency Score: 2/10**

---

## Capital Efficiency Verdict

The platform **deploys capital inefficiently** by:
1. Running 606 strategies when ~15 have any evidence of edge
2. Equal-weighting unvalidated strategies with proven micro-winners
3. Maintaining 108 client definitions when 48 are active
4. Suffering 100% capital idle during kill switch outages

**$1M paper account generating $20 best-strategy profit is 0.002% return — not capital efficient by any institutional standard.**
