# PHASE 20 — FINAL CERTIFICATION

**Audit Authority:** CIO / Head of Quantitative Research / Principal Systematic Trading Architect / Independent Auditor  
**Date:** 2026-06-10  
**Scope:** 714 defined strategies (606 Go + 108 Client)  
**Standard:** Evidence-only. No synthetic certification. No architecture credit. No placeholder scoring.

---

## Certification Questions — Evidence-Based Answers

| # | Question | Answer | Evidence |
|:-:|:---------|:------:|:---------|
| 1 | How many unique strategies truly exist? | **39 signal engines** in 714 strategy instances | Phase 2 clustering |
| 2 | How many should remain active? | **47 total** (10 Tier A + 22 Tier B + 15 Tier C) | Phase 17 |
| 3 | How many should be retired? | **~667** (93.4% of defined universe) | Phases 3, 11, 17 |
| 4 | Which strategies possess measurable edge? | **10 strategies** (positive live PnL) | Phase 5, aggregator_selective.go |
| 5 | Which strategies destroy portfolio returns? | **11 removed (-$108.81) + 5 active (-$7.38)** documented | Phases 15, 17 |
| 6 | Does the platform currently possess alpha? | **NO** — 0/17 alpha engines have live positive PnL | Phase 4 |
| 7 | Can the platform be made profitable? | **YES** — structural fixes are known and implementable | Phases 6-9, 19 |
| 8 | What exact changes are required? | See Top 25 sections below | All phases |

---

## Top 25 Profitability Killers

| Rank | Killer | Impact | Fix |
|:----:|:-------|:-------|:----|
| 1 | 301 expansion pack overfit strategies active | Consumes 49% of registry; zero edge; wastes aggregator slots | Remove immediately |
| 2 | SL 0.15% inside 1m BTC noise band (ATR avg 0.15%) | Converts correct directional trades to noise losses | ATR-based stops ≥0.30% |
| 3 | 6 alpha engines blocked by OnCandle dispatch bug | Highest-quality strategies never fire a single trade | Fix loop.go dispatch |
| 4 | Funding data feed empty | FundingMeanReversion_Alpha operationally dead | Populate from Binance API |
| 5 | Phase 22E synthetic certification creates false confidence | Certified -100% live degradation as "PASS" | Revoke; replace with real data |
| 6 | No production PnL data for 592 of 606 strategies | Cannot rank, size, or retire rationally | MongoDB export query |
| 7 | Aggregator starvation (cap 25 of 606) | 96% of strategies never execute per candle | Remove 301 XP; cap becomes meaningful |
| 8 | MACD family active (18 strategies) | 2 documented losers; lagging on 1m BTC | Retire entire MACD family |
| 9 | Liquidation cascade feed unwired | LiquidationCascade_Alpha never runs | Wire in main.go |
| 10 | Equal-weight sizing to unvalidated strategies | Capital allocation to 592 unproven strategies | Evidence-based Kelly sizing |
| 11 | CCI family active (13 strategies) | Redundant with RSI; no documented edge | Retire entire CCI family |
| 12 | Breakout strategies without regime gate | 5 removed losers totaling -$49.55 | Regime gate: block in Range |
| 13 | 5 borderline active losers still running | -$7.38 documented ongoing drag | Remove immediately |
| 14 | Williams %R family (8 strategies) | Mathematically redundant with Stochastic | Retire entirely |
| 15 | ROC family (8 strategies) | Equivalent to MACD histogram | Retire entirely |
| 16 | Parabolic SAR family (8 strategies) | Whipsaw-prone on 1m BTC | Retire entirely |
| 17 | N-bar breakout family (15 strategies) | Donchian-family proven loser | Retire entirely |
| 18 | Consecutive candles family (8 strategies) | Noise signal on 1m BTC | Retire entirely |
| 19 | Hull MA family (8 strategies) | Lagging; redundant with EMA | Retire all but 1 |
| 20 | No TIME exit in Go position manager | Stale positions tie up capital; no expiry path | Add 30-bar TIME exit |
| 21 | Range regime PF 0.83 (confirmed negative) | 30% of time platform is unprofitable | Regime gate all trend strategies |
| 22 | Go engine missing funding cost in PnL | Overstates paper edge for perpetual positions | Add funding cost calculation |
| 23 | Go paper slippage = 0 | Paper PnL higher than live by ~5-10 bps/trade | Apply 5 bps slippage model |
| 24 | Kill switch lacks auto-heal | 48-hour outage from single false positive | Grace period + auto-heal |
| 25 | 25× client leverage on unvalidated strategies | Amplifies losses if portfolio has negative EV | Reduce to 15× maximum |

---

## Top 25 Alpha Improvements

| Rank | Improvement | Expected Impact | Effort |
|:----:|:-----------|:----------------|:-------|
| 1 | Fix OnCandle dispatch → unlock 6 alpha engines | Unlock MSS, OB, FVG, LS, POC, Session simultaneously | 3 days |
| 2 | Populate funding.ndjson | Enable funding mean reversion (highest institutional theoretical edge) | 2 days |
| 3 | Wire liquidation feed | Enable cascade reversal alpha (unique institutional signal) | 3 days |
| 4 | ATR-based SL reconstruction | Convert noise-stops to trend-stops; +15-25% PF improvement | 1 week |
| 5 | Validate MSSContinuation_Alpha with real data post-fix | Confirm or reject synthetic PF 2.92 claim | 30 days paper |
| 6 | Build 3-signal Trend Confluence Engine (from TripleFilter prototype) | Extend platform's top performer into robust validated engine | 2 weeks |
| 7 | Regime gate on all strategies | Eliminate Range regime losses (PF 0.83 → target 1.0+) | 1 week |
| 8 | Statistical mean reversion with regime gate (ZScore) | Best risk-adjusted signal type in evidence | 1 week refinement |
| 9 | Order flow expansion: CVD divergence tuning | Quality gate at 70 barely passes; improve signal quality | 1 week |
| 10 | Add PROFIT_LOCK to Go position manager | Client replay shows 74/75 wins via PROFIT_LOCK; replicate in Go | 3 days |
| 11 | Multi-level TP with trailing runner | Capture extended momentum after TP1 | 1 week |
| 12 | MongoDB strategy_scores → Kelly sizing | Capital to proven winners; zero to unvalidated | 2 days |
| 13 | Extend client replay to 90-day validation set | Statistically valid sample (vs current 8.3 hours) | 1 week |
| 14 | Walk-forward validation on Tier A/B strategies | First real OOS certification of any strategy | 2 weeks |
| 15 | Build pure OrderBlock + Volume entry confluence | Institutional structure + volume = high-quality entry signal | 2 weeks |
| 16 | Add BTC options flow as alpha input (Delta Exchange data) | Options order flow precedes BTC spot moves | 2 weeks |
| 17 | Session expansion with volume profiling | Time-of-day + volume = predictable expansion signals | 1 week |
| 18 | Unify Go + Client into single strategy registry | Single PnL, single risk gate, coherent portfolio | 2-4 weeks |
| 19 | Rolling 30-day strategy health scoring | Auto-demote underperforming strategies dynamically | 2 weeks |
| 20 | Fair Value Gap with Fibonacci retracement confluence | FVG + Fib = institutional entry accuracy improvement | 2 weeks |
| 21 | Multi-timeframe trend filter (1m entry, 5m trend, 15m bias) | Reduces counter-trend 1m noise by validating against higher TF | 1 week |
| 22 | Market microstructure: bid-ask spread monitoring | Wide spread = low liquidity = skip signal | 3 days |
| 23 | Volatility-adjusted TP sizing (larger TP in high ATR) | Avoid capping winners during high-momentum moves | 3 days |
| 24 | Funding + Liquidation composite signal | When both extremes align → highest conviction signal | 2 weeks |
| 25 | Sentiment confluence from options skew (put/call ratio) | Options market sentiment precedes spot moves | 2 weeks |

---

## Top 25 Strategy Removals (Ordered by Priority)

| Rank | Strategy / Group | Count | Reason |
|:----:|:----------------|------:|:-------|
| 1 | All XP_* expansion pack | 301 | Parameter-grid overfit; zero edge; zero live PnL |
| 2 | RSI_MACD_Divergence_Scalp | 1 | Active loser -$2.06 |
| 3 | TripleTrend_Confluence_Scalp | 1 | Active loser -$1.43 |
| 4 | VWAP_RSI2_Reversion_Scalp | 1 | Active loser -$1.42 |
| 5 | SessionOpen_Momentum_Scalp | 1 | Active loser -$1.40 |
| 6 | VWAP_Bounce_Pro_Scalp | 1 | Active loser -$1.07 |
| 7 | All MACD family (Elite V2 + Intraday) | 18 | Documented losers; lagging indicator |
| 8 | All CCI family | 13 | Redundant with RSI; no documented edge |
| 9 | All Williams %R family | 8 | Mathematically redundant with Stochastic |
| 10 | All ROC family | 8 | Equivalent to MACD histogram |
| 11 | All Parabolic SAR family | 8 | Whipsaw-prone; no evidence of edge |
| 12 | All N-bar Breakout family | 15 | Donchian-family proven loser |
| 13 | All Consecutive Candles family | 8 | Noise signal on 1m BTC |
| 14 | Hull MA (7 of 8 variants) | 7 | Lagging redundancy; keep 1 representative only |
| 15 | EMA cross variants beyond 3 representatives | 67 | Parameter inflation; 70 instances of 1 engine |
| 16 | RSI threshold variants beyond 2 representatives | 41 | Parameter inflation; 43 instances of 1 engine |
| 17 | RSI slope variants beyond 1 representative | 24 | Parameter inflation |
| 18 | Bollinger variants beyond 3 representatives | 51 | Parameter inflation |
| 19 | VWAP variants beyond 3 representatives | 17 | Parameter inflation |
| 20 | Test_Execution_Dumb_Scalper | 1 | Test strategy in production registry |
| 21 | Stochastic variants beyond 1 representative | 15 | Parameter inflation |
| 22 | ATR signal variants beyond 1 representative | 9 | ATR_Breakout removed -$15.43 |
| 23 | Triple EMA variants beyond 2 | 14 | Parameter inflation |
| 24 | Client research pool (IDs 600-659) | 60 | Never executes; clean registry |
| 25 | Client stub definitions (IDs 660-759) | 40 | Empty definitions |

---

## Top 25 Implementation Priorities (Code-Level)

| Rank | Task | File | Lines/Function | Timeline |
|:----:|:-----|:-----|:--------------|:---------|
| 1 | Remove expansion pack call | `curated_registry.go:L_end` | Remove `buildExpansionPack()` | Day 1 |
| 2 | Remove 5 borderline losers | `curated_registry.go` | Remove 5 strategy entries | Day 1 |
| 3 | Fix OnCandle dispatch | `engine/internal/trading/loop.go` | Add `onCandleClose()` loop | Day 2-4 |
| 4 | Wire funding feed goroutine | `main.go` + new `funding_feed.go` | `go startFundingFeed(ctx, ch)` | Day 5-6 |
| 5 | Wire liquidation feed | `main.go` + new `liquidation_feed.go` | `go startLiquidationFeed(ctx, ch)` | Day 7-9 |
| 6 | Retire D families | `curated_registry.go` | Remove ~170 entries | Day 2 |
| 7 | MongoDB trade export | MongoDB shell | Single aggregate query | Day 1-2 |
| 8 | ATR stop in positions.Manager | `engine/internal/trading/positions/manager.go` | `computeStop()` + StopConfig | Week 2 |
| 9 | TIME exit in positions.Manager | Same file | `MaxHoldBars` + tick counter | Week 2 |
| 10 | Regime classifier | `engine/internal/regime/classifier.go` | `ClassifyRegime()` function | Week 3 |
| 11 | Wire regime to aggregator | `aggregator_selective.go` | `gateByRegime()` call | Week 3 |
| 12 | Kelly sizing from strategy_scores | `engine/internal/trading/sizer.go` | `computeKellySize()` | Week 3-4 |
| 13 | PROFIT_LOCK in Go position manager | `positions/manager.go` | `ProfitLockPct` field + check | Week 2 |
| 14 | Multi-level TP | `positions/manager.go` | TP1/TP2/trail config | Week 2 |
| 15 | Update strategy registry test | `curated_registry_test.go` | Update expected count | After #1 |
| 16 | Kill switch grace period | `engine/internal/killswitch/` | `GracePeriod` config | Week 4 |
| 17 | Kill switch auto-heal | Same | `AutoHealThreshold` + timer | Week 4 |
| 18 | Reduce client leverage | Replay config + worker config | Change 25 → 15 | Day 3 |
| 19 | Funding cost in Go PnL | `engine/internal/ledger/` | Add funding accrual | Week 4 |
| 20 | Go slippage model | `engine/internal/omsv3/` | 5 bps fill adjustment | Week 4 |
| 21 | Phase 22E real data replacement | `engine/phase22e_reports/` | Replace `syntheticTrades()` | Week 8 |
| 22 | Walk-forward backtest mode | `engine/cmd/backtest/main.go` | `--walk-forward` flag | Week 5-6 |
| 23 | Strategy health auto-demotion | `aggregator_selective.go` | Read `strategy_health` collection | Week 6 |
| 24 | Multi-TF trend filter | New helper in `indicators.go` | `MultiTFTrend()` | Week 6-7 |
| 25 | Unify Go + Client registry | Architecture task | Full refactor | Month 2-3 |

---

## FINAL VERDICT

---

# VERDICT 3 — LIMITED EDGE, MAJOR RECONSTRUCTION REQUIRED

---

**Rationale for VERDICT 3 (not 4):**
- 10 strategies have documented positive live PnL
- Client replay shows +$0.91/trade on 8.3-hour sample
- Infrastructure for institutional alpha exists (MSS, FVG, OB, Funding — all implemented)
- Dispatch fixes required are simple and specific (not architectural)
- The losses are systematic and explainable (SL geometry, overfit XP pack, broken dispatch)
- A clear, actionable fix path exists for every identified problem

**Rationale for VERDICT 3 (not 2):**
- No single strategy has OOS-validated PnL with n≥100 trades
- Net documented PnL is negative (-$62)
- All institutional alpha engines are operationally dead
- Overfitting is systemic (301 XP strategies + synthetic certification)
- No 90-day track record exists
- Multiple infrastructure failures (kill switch, funding feed, liquidation feed)

---

## Production Profitability Scores

| Dimension | Score /10 | Evidence | Weight |
|:----------|:---------:|:---------|:------:|
| **Production Profitability** | **2.5** | Net documented PnL negative; 10/714 strategies positive; no annual track record | 30% |
| **Alpha Score** | **2.0** | 0/17 alpha engines produce live PnL; infrastructure broken; theoretical sources excellent | 25% |
| **Strategy Quality** | **2.5** | 447 strategies Tier D; 301 overfit XP; 10 proven; 22 alpha post-fix potential | 20% |
| **Portfolio Construction** | **2.5** | 714 names, 39 signal engines; dual stacks; no regime gating; equal-weight unvalidated | 15% |
| **Risk Efficiency** | **4.5** | Kill switch exists; risk gate exists; undermined by false positive outage and noise SL | 10% |

**Composite Score: 2.65 / 10**

**Confidence Level:** MEDIUM  
- HIGH confidence: Code analysis, alpha plumbing findings, SL geometry math, overfitting detection
- LOW confidence: Portfolio-level PnL (MongoDB inaccessible in audit environment)
- MEDIUM confidence: Strategy-level PnL inference (25 hardcoded + 11 removed + replay sample)

---

## What Changes Are Required for Recertification

| Requirement | Test | Target |
|:-----------|:-----|:-------|
| MongoDB `paper_trades` export | `db.paper_trades.aggregate(...)` | Available |
| Per-strategy PF/WR with n≥50 | Compute from MongoDB | All Tier A/B strategies |
| OnCandle dispatch fix verified | Signal count > 0 for each alpha | All 6 affected engines |
| Funding feed active | `funding.ndjson` non-empty, updating | 8h interval |
| ATR stops deployed | Live SL ≥ 1.5× ATR(14) for all strategies | Registry-wide |
| 90-day OOS walk-forward | Backtest on unseen data | Tier A strategies |
| Kill switch auto-heal tested | Simulate false-positive; confirm auto-recovery | Operational |
| Regime gate active | Log regime per candle; confirm gating | ≥30 days data |

**Re-certification verdict threshold:**  
- Achieve VERDICT 2: 5+ strategies with OOS PF≥1.30 (n≥100), net Go PnL positive, alpha engines firing  
- Achieve VERDICT 1: 10+ strategies OOS validated, portfolio Sharpe ≥1.5, 90-day track record positive  

---

## Can the Platform Be Made Profitable?

**YES. Here is the evidence:**

1. The top two strategies alone (+$36) outperform the 11 removed losers (-$109) on a per-strategy basis. If the platform had only deployed those 10 proven strategies, the documented result would be positive.

2. The institutional alpha engines (MSS, OB, FVG) are theoretically sound and architecturally complete. A 3-day fix unlocks them. Post-fix, they need validation — but the infrastructure is there.

3. The client desk architecture (0.50% SL, 1.50% TP, PROFIT_LOCK) is correct. The short-sample replay is positive. Extending this with wider timeframe data is a mechanical task, not an edge-discovery task.

4. Every documented failure has a specific, non-speculative fix:
   - SL too tight → ATR stops
   - Overfit strategies → remove expansion pack
   - Alpha broken → fix dispatch
   - Range losses → regime gate

**What the platform is NOT:** A system that has been proven to work.  
**What the platform IS:** A system with the right components, broken plumbing, and an clear repair roadmap.

The difference between the current platform and an institutional-grade system is:
- 10 days of engineering work (Tier 1 tasks)
- 30 days of paper validation
- 90 days of walk-forward backtesting

It is not a fundamental impossibility. It is a specific, bounded engineering and validation project.

---

## Certification Statement

This audit **does not certify** the platform as currently profitable.

This audit **certifies** that:
1. The platform contains 10 strategies with marginal positive live evidence
2. The platform's institutional alpha infrastructure is architecturally sound and fixable
3. The primary failure modes are identifiable and correctable
4. The path from current state to VERDICT 2 is a 4-6 week engineering task
5. The path from VERDICT 2 to VERDICT 1 requires 90-day validated track record

This audit **explicitly rejects:**
- All Phase 22E synthetic certifications (deterministic fake data)
- All synthetic PF/Sharpe metrics as evidence of profitability
- Strategy count, code quality, or institutional architecture as substitutes for trading evidence

**Minimum requirements before any live capital deployment:**
1. At least 5 strategies with n≥100 real paper trades and OOS PF≥1.25
2. Net 90-day paper trading account return: positive
3. Alpha engines firing confirmed with real signal counts
4. Kill switch false positive rate: <1 per 90 days
5. ATR-based stops deployed platform-wide

---

*This certification is based exclusively on evidence present in the repository and computed replay, audited 2026-06-10. It does not rely on architecture quality, strategy count, or synthetic reports as evidence of edge.*

*Prepared by: Claude Code Institutional Audit System*  
*Evidence standards: Evidence-only. No assumptions. No synthetic data accepted as certification.*
