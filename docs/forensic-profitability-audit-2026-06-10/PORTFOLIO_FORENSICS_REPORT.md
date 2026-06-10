# PHASE 10 — PORTFOLIO FORENSICS

**Generated:** 2026-06-10  
**Verdict:** FAIL — extreme overlap, dual stacks, concentration risk

---

## Strategy Overlap

### Go Engine Internal Overlap

| Cluster | Strategies | Correlation | Evidence |
|:--------|:-----------|:------------|:---------|
| EMA cross variants | 55 (15 elite + 40 XP) | **>0.85 estimated** | Same signal, different periods |
| RSI threshold variants | 38 (8 + 30 XP) | **>0.80** | Same oscillator, different bands |
| Bollinger variants | 37 (12 + 25 XP) | **>0.75** | Same band logic |
| MACD variants | 35 (10 + 25 XP) | **>0.80** | Same histogram signal |
| VWAP variants | 35 (10 + 25 XP) | **>0.70** | Same reference price |

**301 expansion pack duplicates 288 hand-curated families with different parameters** — not independent alpha sources.

### Cross-Stack Overlap (Go vs Client)

| Dimension | Overlap |
|:----------|:--------|
| Strategy names | **0%** — completely different universes |
| Signal logic (EMA trend) | **~30% conceptual** — both have trend/breakout/MR |
| Asset | 100% BTC |
| Timeframe | 100% 1m primary |
| Execution path | **0%** — separate runtimes |

**Two portfolios trade the same asset on the same timeframe with different strategies and different SL/TP geometry.**

---

## Signal Correlation

### Aggregator Dominance Filter

`FilterSignalsSelective` forces **single-direction consensus** per batch:
- All 606 strategies vote BUY or SELL
- Only dominant side proceeds
- Max 25 approved

**Effect:** Portfolio acts as **single-direction basket** per candle — not diversified long/short book.

### Phase 22E Correlation (Synthetic, 12 strategies)

**Portfolio Diversification Score: 65.8/100** (`PORTFOLIO_CORRELATION_REPORT.md`)

High-correlation clusters:
- Cluster 1 (r=0.73): s03, s05
- Cluster 2 (r=0.86): s06, s07, s08
- Cluster 3 (r=0.81): s11, s12

**Family correlation highlights:**
- Funding ↔ Price Action: r=0.73
- Bollinger ↔ Price Action: r=0.64
- EMA Cross ↔ EMA Cross: r=0.69 (internal)

---

## Position Correlation

| Constraint | Go Engine | Client Desk |
|:-----------|:----------|:------------|
| Max positions per strategy | 2 | Configurable (12 in replay) |
| Max same-direction equity | Risk V2 family cap 30% | 35% (`maxSameDirFracOfEquity`) |
| Max approved signals/batch | 25 | All passing strategies |
| Category cap | 5/category | Playbook filter |

**Go:** 25 correlated same-direction positions possible per minute.  
**Client:** Up to 12 concurrent positions in replay.

---

## Risk Concentration

| Risk Factor | Concentration | Evidence |
|:------------|:-------------|:---------|
| Asset | **100% BTC-USD** | Single underlying |
| Venue | **100% Coinbase WS** | Single price feed |
| Direction | **1 per batch** | Dominance filter |
| Strategy family | EMA/RSI dominate approvals | Category bonuses favor Trend/Multi-Signal |
| Alpha source | 6/17 broken | Concentration in non-functional alphas |

---

## Portfolio Diversification Score

| Method | Score | Valid? |
|:-------|------:|:------:|
| Phase 22E Pearson (12 synthetic) | 65.8/100 | **INVALID** — synthetic |
| Effective independent strategies (Go) | **~15-20 estimated** | 606 names, ~15 unique signal types |
| Effective independent strategies (Client) | **~12-15 estimated** | 48 templates, ~8 families |
| Combined platform | **~25-30 unique signals** | Massive name inflation |

---

## Portfolio Forensics Verdict

| Question | Answer |
|:---------|:-------|
| Strategy overlap destroying diversification? | **YES** — 301 XP clones + dominance filter |
| Signal correlation too high? | **YES** — same-direction batch approval |
| Position correlation managed? | **PARTIAL** — caps exist but 25/batch |
| Portfolio construction correct? | **FAIL** — dual stacks, single asset, name inflation |

**Effective portfolio:** ~25 independent signals masquerading as 714 strategies.
