# STRATEGY EVIDENCE DATABASE
## SEP Phase 1 — Forensic Trade Evidence

**Date:** 2026-06-10  
**Auditor:** Principal Quant Researcher / SEP Program  
**Evidence Base:** Code audit (Phase 23) + SQLite trade schema analysis  
**Live Trade Data:** PENDING — MongoDB/SQLite extraction required

---

## EVIDENCE STATUS

| Source | Available | Rows | Status |
|--------|-----------|------|--------|
| SQLite `trades` table | Yes (schema confirmed) | UNKNOWN | Extraction required |
| MongoDB `paper_trades` | Yes (TTL 30d) | UNKNOWN | Query required |
| MongoDB `strategy_leaderboards` | Yes (TTL 90d) | UNKNOWN | Query required |
| `engine/internal/performance/analytics.go` | StreamingAnalytics active | Real-time | In-process only |
| Phase 23 code audit | Complete | All 95 strategies | Synthetic scores only |

---

## CRITICAL FINDING

**Actual per-strategy trade PnL cannot be computed from this session.** The SQLite database (`./data/engine.db`) and MongoDB Atlas are cloud-managed and cannot be directly queried here. Evidence below is code-based quality analysis only.

**To extract live evidence**, run:
```bash
# From engine directory
go run ./cmd/sep_evidence/main.go --output STRATEGY_EVIDENCE_DATABASE_LIVE.md
```
(See SEP evidence extraction tool — Phase 1 completion requirement)

---

## CODE-BASED QUALITY EVIDENCE

### TIER 1 — INSTITUTIONAL GRADE (Score 7.35–8.25)

These strategies have theoretically sound edge based on code analysis. Actual PnL unknown.

#### Phase 11 Microstructure Alpha (7 strategies)
- **Architecture:** All-in-one feature engine blending CVD, liquidity zones, funding pressure, market structure, volatility regime
- **Data Feed:** Full tick + candle OHLCV
- **Signal Source:** Multi-factor institutional confluence
- **Code Quality Score:** 8.25/10
- **Known Issues:** None post Phase 11 implementation
- **Trade Evidence:** MISSING — requires live paper trading data

| Strategy | Kind | Regime | SL | TP | Evidence |
|----------|------|--------|----|----|---------|
| Phase11LiquiditySweepReversal | Liquidity | Trending | ATR×2 | ATR×6 | MISSING |
| Phase11FundingMeanReversion | Derivatives | All | ATR×2 | ATR×6 | MISSING |
| Phase11CVDDivergence | Order Flow | All | ATR×2 | ATR×6 | MISSING |
| Phase11LiquidationCascade | Liquidations | All | ATR×2 | ATR×6 | MISSING |
| Phase11FairValueGap | Structure | All | ATR×2 | ATR×6 | MISSING |
| Phase11OrderBlock | Smart Money | All | ATR×2 | ATR×6 | MISSING |
| Phase11MSSCHOCH | Structure | Trending | ATR×2 | ATR×6 | MISSING |

#### Core Institutional Alpha Engine (9 strategies)
- **Code Quality Score:** 7.35–7.85/10
- **Known Issues:** Funding data was empty (now fixed), dispatch was correct

| Strategy | Module | Data Feed | Status | Evidence |
|----------|--------|-----------|--------|---------|
| FundingMeanReversionAlpha | Funding | Binance/Bybit 8h | Fixed | MISSING |
| CVDDivergenceAlpha | Microstructure | Coinbase WS | Active | MISSING |
| DeltaAbsorptionAlpha | Microstructure | Tick | Active | MISSING |
| LiquiditySweepReversalAlpha | Liquidity | Candle | Active | MISSING |
| FVGRetestAlpha | Structure | Candle | Active | MISSING |
| OrderBlockRetestAlpha | Smart Money | Candle | Active | MISSING |
| MSSContinuationAlpha | Structure | Candle | Upgraded | MISSING |
| POCBounceAlpha | Market Profile | Candle | Active | MISSING |
| SessionExpansionAlpha | Session | Candle | Active | MISSING |
| LiquidationCascadeAlpha | Liquidations | Tick (proxy) | Active | MISSING |

---

### TIER 2 — SELECTIVE EDGE (Score 5.55–6.50)

Higher-quality technical strategies with multi-indicator confirmation.

| Strategy | Category | Score | Key Edge | Evidence |
|----------|----------|-------|---------|---------|
| MomentumDivergence family (6) | Price Action Elite | 6.50 | RSI divergence + price structure | MISSING |
| OrderFlowPressurePro | Microstructure | 6.25 | Tick imbalance > 80 | MISSING |
| ZScoreBand | Statistical | 5.85 | Z-score mean reversion | MISSING |
| LinReg | Statistical | 5.75 | Linear regression channel | MISSING |
| ExhaustionScalper | Price Action | 5.55 | Volume exhaustion pattern | MISSING |

---

### TIER 3 — MARGINAL EDGE (Score 4.55–4.65)

Technical indicator strategies with some signal value but no unique institutional edge.

**Count:** ~90 strategies  
**Families:** N-bar Breakout, Triple EMA, ATR Signal, Keltner, VWAP, Intraday 5m/15m variants

**Verdict:** Require LIVE TRADE EVIDENCE before capital allocation. Many may be profitable in trending conditions but lose in ranging. Regime gating has been applied where appropriate.

---

### TIER 4 — LOW EDGE (Score 1.85–3.35)

Pure indicator strategies without multi-factor confirmation.

**Count:** ~50 strategies  
**Families:** EMA Cross variants, RSI variants, Bollinger Band variants, MACD variants, CCI, Stochastic

**Verdict:** Candidate for retirement pending live evidence. No unique institutional edge. These fire on common indicator signals that are known to be marginal in BTC perpetual scalping.

---

### RETIRED (Score ≤ 2.00)

| Strategy Family | Count | Reason |
|----------------|-------|--------|
| Expansion Pack (XP_*) | 301 | Definitional overfit — removed Phase 2 |
| ADX_Trend_Scalp | 1 | −$7.86 live loss — already removed |
| Pullback_Continuation_Pro | 1 | −$4.27 live loss — already removed |
| MACD_VWAP_Flip | 1 | −$10.90 live loss — already removed |
| ATR_Breakout | 1 | −$15.43 live loss — already removed |
| ATR_Volume_Impulse | 1 | −$19.65 live loss — already removed |
| KAMA_Adaptive | 1 | −$14.36 live loss — already removed |
| Others (removed earlier) | 7 | Live losses documented in registry comments |

---

## EVIDENCE EXTRACTION TOOL (REQUIRED)

To complete Phase 1, create `engine/cmd/sep_evidence/main.go`:

```go
// Queries SQLite trades table and computes per-strategy:
// - Trade count, Win rate, Loss rate
// - Average win, Average loss
// - Profit factor (gross profit / gross loss)
// - Sharpe ratio (PnL / StdDev × √252)
// - Sortino ratio
// - Maximum drawdown
// - Expectancy (avg win × win_rate − avg_loss × loss_rate)
// - Average hold time
// - MFE (Max Favorable Excursion)
// - MAE (Max Adverse Excursion)
// - Fees paid, Slippage impact
```

---

## VERDICT

**PHASE 1 STATUS: PARTIAL**

Code-based quality audit: COMPLETE (Phase 23)  
Live trade evidence: FAIL — data extraction tool not yet built  
Strategy leaderboard (CSV): FAIL — requires live trade data  

**No UNKNOWN values will remain once the extraction tool is run.**  
**Phase 1 is not complete until live trade evidence is extracted and validated.**
