# Profitability & Accuracy Improvement Plan — Global Review

**Date:** 2026-05-22
**Scope:** BTC futures paper trading desk (strategies + signals + execution + risk + feedback loop)
**Disclaimer:** Paper-only. Nothing in this document promises profitability. Every intervention is evidence-graded and reversible via env flags or git revert.

---

## 0. Executive summary (10 bullets)

1. The architecture is **well-modularized** (signals / paper math / desk policy / hook) but suffers from **3 structural gaps**: hand-tuned signal weights, a stubbed winners-promotion loop, and zero integration tests for the live entry→close path.
2. Recent shipped fixes (commits `097de86`, `3ba2ffc`, `ec0e18a`) addressed the *cost side*: maker-first fees, honest slippage, kill-switch defaults, tighter position caps. The *signal side* (entry quality) is now the binding constraint.
3. The 20 CORE strategies use hand-coded integer weights (e.g. `add(18, "HTF bull stack")`) summed against a single threshold. No weight has ever been calibrated against realized win/loss outcomes.
4. `applyWinnersOnlyGate()` exists at [futuresDeskPolicy.ts:1004](../src/lib/futuresDeskPolicy.ts) but **has zero callers in production code paths**; `resolveBtcFtActiveStrategyIds()` ignores its `winnerIds` argument. The promotion loop is conceptual, not wired.
5. The ranking script [scripts/rank-btc-ft-strategies.ts:49](../scripts/rank-btc-ft-strategies.ts) returns **deterministic synthetic metrics**, not real backtest results. Any "winners" file it writes is fiction.
6. Regime detection uses only **15m ADX + ATR** (single timeframe). A multi-timeframe regime fusion (1m + 5m + 15m) would catch transitions earlier and reduce chop-regime entries that don't clear fees.
7. The Mongo trades pipeline is healthy (recent commits prove fills land in Atlas), but the rolling expectancy is **not** fed back into auto-kill — kill switch reads Supabase rolling stats only.
8. Data resilience is minimal: no retry, no backoff, no circuit-breaker on the Delta API path ([futuresKlinesFetch.ts:40-46](../src/lib/futuresKlinesFetch.ts)). One transient 5xx pauses the engine.
9. **Largest single test gap**: `useBTCFuturesScalperEngine.ts` (~2300 lines, the orchestration brain) has no integration test. Paper math has 100% coverage; the layer above it has 0%.
10. Recommended phasing: **P0 (signal calibration + winners-loop wiring + integration test)** → **P1 (multi-TF regime + Mongo kill switch + exit optimization)** → **P2 (vol-sized notional + correlation-aware caps + data resilience)**. P0 alone is expected to move net expectancy more than P1+P2 combined.

---

## 1. Current state (what's good, what's broken)

### 1.1 Strengths

| Area | Evidence | Why it's good |
|---|---|---|
| Modularity | [futuresSignals.ts](../src/lib/futuresSignals.ts) (pure math), [futuresPaperMath.ts](../src/lib/futuresPaperMath.ts) (accounting), [futuresDeskPolicy.ts](../src/lib/futuresDeskPolicy.ts) (gates), hook (orchestration) | Each concern is replaceable in isolation. Changing one rarely cascades. |
| Honest fees | `TAKER_FEE_PCT = 0.001` per leg, slippage 5 bps default ([futuresDeskPolicy.ts:309](../src/lib/futuresDeskPolicy.ts)) | No "free trades" — every spec must beat 0.30% RT to break even. |
| Maker-first model | Just shipped (`097de86`); blended fee ≈ 0.13% RT at p_maker=0.7 | ~35% fee drag reduction; reversible via env. |
| Strict caps | 6 max open / 3 per side / 1 per template / 2 per category | Concentration over shotgun → less correlated risk. |
| Kill switch | Default-ON with 7-day window, −$0.02 expectancy fence, 8 trades min | Bleeding strats retire fast, not after weeks of damage. |
| Profit-lock | `paperProfitLockGate()` ([futuresPaperMath.ts:579](../src/lib/futuresPaperMath.ts)) | Prevents winners from round-tripping to scratch/loss. |
| Drawdown lock | 25% / 21% resume thresholds (hook) | Hard floor on disaster scenarios. |
| Replay infra | `npm run replay` exists ([scripts/replay-paper-desk.ts](../scripts/replay-paper-desk.ts)) | Offline expectancy estimation possible. |
| Test coverage on math | 362 vitests, 100% on paper math + desk policy | High-confidence refactors of pure functions. |

### 1.2 Structural gaps (ranked by impact)

| # | Gap | Evidence | Why it hurts profit |
|---|---|---|---|
| G1 | **Signal weights are hand-tuned**, never calibrated | [futuresSignals.ts:636-665](../src/lib/futuresSignals.ts) — magic integers 6/8/10/12/16/18/20 with no provenance | False positives at threshold = 28 pay 0.20% RT fees with no edge. |
| G2 | **Winners promotion is stubbed end-to-end** | [scripts/rank-btc-ft-strategies.ts:49](../scripts/rank-btc-ft-strategies.ts) returns synthetic metrics; `applyWinnersOnlyGate` has 0 callers in production paths | Bad strategies aren't demoted; good ones aren't reinforced. |
| G3 | **No E2E integration test** for entry→signal→open→close | Glob shows tests for every dependency *except* `useBTCFuturesScalperEngine.ts` | Regressions in the orchestration layer ship undetected. |
| G4 | **Single-TF regime** (15m only) | [futuresSignals.ts:385](../src/lib/futuresSignals.ts) `classifyRegimeTagFrom15mBars` | Late detection of regime change → trades opened into wrong regime. |
| G5 | **Mongo trades not feeding kill switch** | Kill switch reads Supabase rolling stats (hook ~989-1041); Mongo is write-only | Auto-kill operates on stale/partial data. |
| G6 | **Stubbed exit decisions beyond TP/SL/time** | `paperResolveHardExit` handles TP/SL/TIME/LIQ_RISK only ([futuresPaperMath.ts](../src/lib/futuresPaperMath.ts)) | Missed exits when momentum dies but stop hasn't hit. No trailing stop, no partial exits. |
| G7 | **No data resilience** | [futuresKlinesFetch.ts:40-46](../src/lib/futuresKlinesFetch.ts) single fetch, no retry/backoff | One 5xx → 4s gap in signals → missed entries on momentum bars. |
| G8 | **No correlation-aware sizing** | Per-side cap is absolute; no notion of which strats are correlated | Two "different" strats can take same-direction same-bar entry. |
| G9 | **No per-strategy adaptive threshold** | Global threshold (default 28) for all 24 strats regardless of their realized hit rate | High-quality strats are gated by noise from low-quality ones. |
| G10 | **Limited research pool** | Only 20 CORE + 4 premium + 10 new (510-519). No alpha factor library, no ensemble | Diversification is shallow; concentration risk if a regime kills 3-4 strats at once. |

---

## 2. Improvement interventions (5 pillars, 18 levers)

Each lever is graded: **Effort** = S/M/L (≤1d / 1-3d / 1+wk), **Impact** = Low/Med/High on net expectancy, **Risk** = Low/Med/High (reversibility + blast radius).

### Pillar 1 — Signal accuracy (G1, G4, G9)

| # | Lever | Effort | Impact | Risk |
|---|---|---|---|---|
| 1.1 | **Calibrate signal weights from realized outcomes** — log every `add(N, "reason")` contribution + final P&L; offline regression to re-derive weights from 30d Mongo trade data. Output: new weight table in [futuresSignals.ts](../src/lib/futuresSignals.ts) | M | High | Med |
| 1.2 | **Per-strategy adaptive threshold** — each strat carries `dynamicThreshold = base + f(rolling7d_winRate)`. High win-rate → lower threshold (let edge run); low win-rate → higher threshold | M | Med | Low |
| 1.3 | **Multi-TF regime fusion** — extend `classifyRegimeTag` to consume 1m+5m+15m ADX/ATR; output composite tag. Reject 1m-trendHigh entries when 15m-chop disagrees | M | Med | Low |
| 1.4 | **Cross-template consensus boost** — when ≥2 independent templates fire same-direction on same bar, boost score by 10%. Already free OHLCV data | S | Med | Low |
| 1.5 | **Feature engineering: volume Z-score + funding deviation** — currently `volRatio` is point-in-time. Add 30-bar Z-score for volume; add `(funding − funding_7d_avg) / σ` as a sentiment feature | S | Med | Low |

### Pillar 2 — Execution quality (G6)

| # | Lever | Effort | Impact | Risk |
|---|---|---|---|---|
| 2.1 | **Trailing stop after profit-lock arms** — once `paperProfitLockGate` triggers, ratchet a stop at `currentBest − 0.5×ATR`. Locks more of winning trades | M | Med | Low |
| 2.2 | **Partial exit at TP1 (50% size, TP=0.4%)** — second half rides to base TP. Reduces variance, captures more wins on stop-out reversals | M | Med | Med |
| 2.3 | **Momentum-decay exit** — if 3 consecutive bars show momentum decay (`mom3 < 0.3×ATR` for longs), force exit. Saves capital from grinding losses | S | Med | Low |
| 2.4 | **Dynamic slippage by volatility** — current 5 bps flat. Make `slipBps = base × clamp(ATR_pct / ATR_pct_30d_avg, 0.5, 2.5)`. Honest math for vol spikes | S | Low | Low |

### Pillar 3 — Risk & sizing (G8)

| # | Lever | Effort | Impact | Risk |
|---|---|---|---|---|
| 3.1 | **Enable vol-sized notional by default** — `deskVolSizedNotionalEnabledFromEnv` exists but defaults off ([futuresDeskPolicy.ts:624](../src/lib/futuresDeskPolicy.ts)). Default ON at 1% risk-per-trade. Sizes inverse to ATR | S | Med | Med |
| 3.2 | **Correlation-aware position caps** — group strategies by category cluster (Trend, MeanRev, Breakout, etc.); cap concurrent opens *per cluster*, not just per category. Prevent 3 momentum strats all going long at once | M | Med | Low |
| 3.3 | **Equity-curve aware sizing** — after 2 consecutive losses, halve next notional; after 3 wins, restore. Implement Kelly-fraction lite (capped at 0.5×) | M | Med | Med |
| 3.4 | **Intraday soft drawdown lock** — pause new entries at −2% daily P&L (in addition to the 25% account lock). Resume next UTC day | S | Med | Low |

### Pillar 4 — Feedback loop (G2, G5)

| # | Lever | Effort | Impact | Risk |
|---|---|---|---|---|
| 4.1 | **Wire real `rank-btc-ft-strategies.ts`** — replace `stubReplay()` ([scripts/rank-btc-ft-strategies.ts:49](../scripts/rank-btc-ft-strategies.ts)) with actual replay engine call across last 30d klines. Output drives `winnerIds` parameter into roster resolution | M | High | Low |
| 4.2 | **Wire `applyWinnersOnlyGate` into roster resolution** — `resolveBtcFtActiveStrategyIds` currently ignores `winnerIds` ([btcFtRoster.ts:51](../src/lib/btcFtRoster.ts)). Pass through from ranking JSON + gate via the policy function | S | High | Low |
| 4.3 | **Mongo-driven kill switch** — read rolling expectancy from MongoDB (not Supabase) since Mongo is now the canonical store. Add `mongoTradesClient.rollingExpectancy(stratId, days)` helper | M | High | Low |
| 4.4 | **Daily policy snapshot** — at 00:00 UTC, write `{enabledStrats, thresholds, caps, regimeMix, equity}` to a `policy_snapshots` Mongo collection. Audit trail for "why did this trade happen?" | S | Low | Low |
| 4.5 | **Auto-promote / demote nightly cron** — scheduled job: re-rank strategies on 7d rolling expectancy; demote those below −$0.02 expectancy + 8 trades; promote those above +$0.05 expectancy + 12 trades. Output: env-overridable allowlist | M | High | Med |

### Pillar 5 — Data, resilience, observability (G3, G7)

| # | Lever | Effort | Impact | Risk |
|---|---|---|---|---|
| 5.1 | **Retry + backoff on Delta API** — wrap `futuresKlinesFetch` in `withRetry(3 attempts, jitter=200ms)`. Circuit-breaker after 5 consecutive failures (60s cool-down) | S | Med | Low |
| 5.2 | **Integration test for entry→close** — synthetic 200-bar fixture; assert at least one strategy opens + closes with expected P&L sign. Test under `client/src/hooks/__tests__/useBTCFuturesScalperEngine.e2e.test.ts` | M | Med (regression catch) | Low |
| 5.3 | **Per-strategy attribution dashboard** — new API route `/api/strategy-attribution/[id]` returning {trades, expectancy, regimeMix, holdTimeP50, feeDrag} for ops review | M | Low (visibility) | Low |
| 5.4 | **Replay snapshot CI gate** — every PR that touches `futuresSignals.ts` or `futuresPaperMath.ts` must run `npm run replay -- --ids=core --bars=1500` and report sumNet. Block merge if sumNet regresses >20% vs base | M | Med (regression prevention) | Low |

---

## 3. Prioritized roadmap

### Phase P0 (next 1–2 weeks, ship together) — *Expected to move expectancy more than P1+P2 combined*

| Lever | Why P0 |
|---|---|
| **4.1** Wire real rank-btc-ft-strategies replay | Without this, "winners gate" is fiction. Blocking all of P1.4.x. |
| **4.2** Wire `applyWinnersOnlyGate` into roster | Same — currently `winnerIds` is ignored. |
| **4.3** Mongo-driven kill switch | Kill switch on Supabase data is operating blind. |
| **1.1** Calibrate signal weights from 30d Mongo data | Highest-leverage single change. Hand-tuned weights are the largest source of fee-bleed false positives. |
| **5.2** Integration test for hook | Without this, P0 changes ship blind. Hard prerequisite for all subsequent phases. |

**P0 success criteria:**
- `rank-btc-ft-strategies.ts` runs against real 30d klines + Mongo trades; outputs deterministic ranking
- `applyWinnersOnlyGate` is called from `resolveBtcFtActiveStrategyIds` when `NEXT_PUBLIC_BTC_FT_USE_RANKED=1`
- Kill switch reads from `mongoTradesClient.rollingExpectancy()` not Supabase
- New weight table in `futuresSignals.ts` derived from regression on logged contributions (commit with R² + sample size in commit message)
- One vitest spec exercises full open→close lifecycle on synthetic fixture, asserts non-zero trades

**Verification:** `npm test` 100% pass + `npm run replay -- --ids=core --bars=4320 --slippageBps=5` reports sumNet ≥ current baseline.

---

### Phase P1 (weeks 3–4) — *Polish + adaptive signals*

| Lever | Why P1 |
|---|---|
| **1.2** Per-strategy adaptive threshold | Once weights are calibrated, the global threshold is too coarse. |
| **1.3** Multi-TF regime fusion | Single-TF regime is the largest unexplored signal feature. |
| **2.1** Trailing stop after profit-lock | Easy win on winning trades, low risk. |
| **2.3** Momentum-decay exit | Cuts the long tail of grinding losses. |
| **3.1** Vol-sized notional default ON | Risk parity across regimes. |
| **4.5** Auto-promote/demote nightly cron | Closes the feedback loop end-to-end. |

**P1 success criteria:**
- Each strat carries `dynamicThreshold` that varies ±4 around the base
- `classifyRegimeTag` accepts 1m+5m+15m inputs, returns composite
- `paperResolveHardExit` adds `TRAIL`, `MOM_DECAY` exit reasons
- `deskVolSizedNotionalEnabledFromEnv` default flipped to ON
- Nightly cron writes `winners.json` consumed by P0.4.1 ranking pipeline

---

### Phase P2 (weeks 5–6) — *Hardening*

| Lever | Why P2 |
|---|---|
| **1.4** Cross-template consensus boost | Diversification benefit; low risk. |
| **1.5** Volume Z-score + funding-deviation features | Feature engineering for next signal calibration cycle. |
| **2.2** Partial exit at TP1 | Material P&L variance reduction. |
| **2.4** Dynamic slippage by vol | Honest math, no behavior change in calm markets. |
| **3.2** Correlation-aware caps | Once vol-sized notional is on, correlation gating matters more. |
| **3.3** Equity-curve aware sizing | Kelly-lite; modest impact, modest risk. |
| **3.4** Intraday soft drawdown lock | Catastrophic-day protection. |
| **4.4** Daily policy snapshot | Audit trail; useful when something goes wrong. |
| **5.1** Retry/backoff on Delta API | Production resilience. |
| **5.3** Strategy attribution dashboard | Ops visibility. |
| **5.4** Replay snapshot CI gate | Lock in P0–P2 gains. |

---

## 4. KPIs to track (define now, measure after each phase)

| KPI | Source | Target after P0 | Target after P1 | Target after P2 |
|---|---|---|---|---|
| **Net expectancy / trade** (USD) | Mongo aggregation over rolling 7d | ≥ +$0.01 | ≥ +$0.05 | ≥ +$0.12 |
| **Win rate** (%) | Mongo win/total | ≥ 38% | ≥ 42% | ≥ 45% |
| **Avg TP/SL realized ratio** | Mongo {tpHitPct, slHitPct} | ≥ 1.8 | ≥ 2.0 | ≥ 2.2 |
| **Fee drag** (% of gross P&L) | Mongo sum(fees) / sum(grossPnl) | ≤ 35% | ≤ 28% | ≤ 22% |
| **Max daily drawdown** (%) | hook daily P&L low-water mark | ≤ −3% | ≤ −2% | ≤ −1.5% |
| **Strategies enabled** (count) | applyWinnersOnlyGate output | 14–18 | 10–14 | 8–12 (high-conviction) |
| **Trades/day** (count) | Mongo count(by openedAt UTC day) | 8–14 | 6–12 | 4–10 (concentration↑) |
| **Test coverage on hook** | vitest --coverage | ≥ 40% lines | ≥ 60% | ≥ 75% |

Track via a new ops route `/api/desk-kpis?window=7d` (P2.5.3 dashboard).

---

## 5. What NOT to do (anti-patterns)

- **Don't add more strategies until P0 ships.** More research-pool IDs (520+) without calibration just multiplies fee-bleed noise.
- **Don't lower the signal threshold further.** It's already at 28 from 22→26→28; the answer is better features, not lower bar.
- **Don't trade live with this until P0 ships AND a 30-day paper run shows sustained positive expectancy.** [LIVE_TRADING_PHASE.md](./LIVE_TRADING_PHASE.md) gates remain in force.
- **Don't claim "guaranteed profitable" anywhere.** Honest paper math is the only way to test.
- **Don't replace CORE 20 wholesale.** They're the baseline; new strategies (510-519, etc.) ride research pool until promoted by the wired (post-P0) ranking script.
- **Don't bypass the kill switch in production.** If a strat trips it, investigate before re-enabling.

---

## 6. Open research questions (defer until after P0)

1. **Order-flow proxy library?** No L2/tape, but volume + range + delta-vol can approximate. Worth one or two more research strategies post-P0.
2. **Funding-rate term structure?** Currently only spot funding. Cross-exchange funding spreads might be a signal (deferred — requires multi-venue data).
3. **Per-session calibration?** Asian / EU / US open all have different microstructure. Currently we use one set of weights — separate calibration per session might improve.
4. **Reinforcement learning for exit timing?** Once 30 days of post-P0 data exists, a small RL policy for "hold one more bar vs exit now" could be tested in shadow mode.
5. **Sentiment from news / social?** Out of scope on current data surface. Defer until P3 if ever.

---

## 7. Verification approach (per phase)

### Per-intervention checklist

- [ ] **Code change** is in a single commit, reverts cleanly via `git revert`
- [ ] **Vitest** added/updated; full suite passes (`npm test` → all green)
- [ ] **Typecheck** clean (`npx tsc --noEmit`)
- [ ] **Replay** runs with new code; sumNet reported honestly (negative is OK if explained)
- [ ] **Env flag** added so intervention can be disabled in production without a redeploy
- [ ] **Audit log entry** if intervention changes policy state (P2.4.4 snapshot)
- [ ] **PR description** cites: which gap (G1–G10), which lever (1.x–5.x), KPI delta expected

### Phase-gate checks (P0 → P1 → P2)

- **P0 → P1 gate:** 7-day paper run after P0 ships shows net expectancy ≥ +$0.01/trade with ≥ 50 trades, fee drag ≤ 35%
- **P1 → P2 gate:** 14-day paper run after P1 ships shows net expectancy ≥ +$0.05/trade, max daily DD ≤ −2%, win rate ≥ 42%
- **P2 → live consideration:** 30-day paper run after P2 ships meets all P2 KPI targets; runbook updated; live-trading checklist in [LIVE_TRADING_PHASE.md](./LIVE_TRADING_PHASE.md) re-reviewed

---

## 8. Cross-links

- [ROOT_CAUSE.md](./ROOT_CAUSE.md) — why expectancy was negative; fee/profit-lock math
- [PAPER_DESK_RUNBOOK.md](./PAPER_DESK_RUNBOOK.md) — ops workflow, env flags reference
- [SCALPING_STRATEGY_RESEARCH.md](./SCALPING_STRATEGY_RESEARCH.md) — Top-5 strategy specs (IDs 510-519)
- [SCALPING_TOP5_IMPLEMENTATION.md](./SCALPING_TOP5_IMPLEMENTATION.md) — engineering tickets for 510-519 scaffolding
- [LIVE_TRADING_PHASE.md](./LIVE_TRADING_PHASE.md) — gates before any live capital
- [SHADOW_VS_PAPER.md](./SHADOW_VS_PAPER.md) — shadow-mode vs paper-mode distinction

---

## 9. Summary table — interventions by pillar

| Pillar | Interventions | Total effort | Expected expectancy lift |
|---|---|---|---|
| 1. Signal accuracy | 5 levers (1.1–1.5) | ~2 weeks | High (root cause of fee bleed) |
| 2. Execution quality | 4 levers (2.1–2.4) | ~1 week | Medium (improves realized RR) |
| 3. Risk & sizing | 4 levers (3.1–3.4) | ~1 week | Medium (variance reduction) |
| 4. Feedback loop | 5 levers (4.1–4.5) | ~2 weeks | High (closes the loop) |
| 5. Data & resilience | 4 levers (5.1–5.4) | ~1 week | Low expectancy / high regression prevention |
| **Total** | **22 levers** | **~7 weeks** | — |

Three highest-impact levers: **1.1 (calibrate weights)**, **4.1+4.2 (wire winners gate)**, **5.2 (E2E test)**. Ship these first.

---

*End of plan. This document is a recommendation, not a commitment. Each lever ships as its own PR with its own gates. Roll back any lever that regresses KPIs by setting its env flag to `0`.*
