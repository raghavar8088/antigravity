---
name: trading-app-guide
description: Helps agents work on this BTC futures trading app — paper desk, signal trace, paper worker, replay, PnL math, strategy policy, kill switch, Go engine, and Next.js client. Use when debugging or changing BTC futures trading, paper-desk execution, strategy gates, closed trades, worker/cron behavior, UI display, backtest engine, or risk/OMS modules.
---

# Trading App Guide

## What This Repo Is

Institutional-grade algo trading platform. BTC futures paper trading + live Indian equity (AngelOne/NSE).

| Layer | Tech | Root |
|---|---|---|
| Frontend + API | Next.js 16 + React 19 + TypeScript | `client/` |
| Execution Engine | Go | `engine/` |
| AI/Strategy Brain | Python | `brain/` |

Run all client commands from `client/`. There is no root `package.json`.

```bash
cd client && npm run test   # targeted tests first
cd client && npm run build  # type + build check
```

---

## Step 1 — Before Opening Any Source File

Always consult these first. Each costs far fewer tokens than raw source reads:

```bash
# Broad question
npm run graphify:query -- "how does X connect to Y?"

# Subsystem question
python scripts/graphify_workflow.py query --scope client "where is gate X enforced?"
python scripts/graphify_workflow.py query --scope engine-internal "how does risk gate connect to OMS?"

# Dependency path
graphify path "SymbolA" "SymbolB"

# Single symbol
graphify explain "functionName"
```

Session state (read these before source):
- `.ai-context/session/current-work.md`
- `.ai-context/session/recent-decisions.md`
- `.ai-context/session/known-issues.md`
- `.ai-context/README_FOR_AI.md`

Protected modules (require explicit approval to change):
- `.ai-context/protected-modules.md`

---

## Fast File Map — Client

**Core UI + engine loop:**
- `client/src/components/BTCFuturesScalper.tsx` — BTC desk UI, stats, tables, controls
- `client/src/hooks/useBTCFuturesScalperEngine.ts` — browser paper engine, position lifecycle

**Policy and math (pure logic — no React):**
- `client/src/lib/futuresDeskPolicy.ts` — desk constants, env parsing, gates, strategy build policy
- `client/src/lib/futuresPaperMath.ts` — PnL, fees, funding, margin, liquidation, slippage
- `client/src/lib/futuresSignals.ts` — signal input building, evaluation, confirmation, regime
- `client/src/lib/futuresStrategies.ts` — strategy definitions and category metadata

**Worker and cron:**
- `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` — headless worker tick
- `client/scripts/btc-ft-paper-worker.ts` — long-running worker process
- `client/src/app/api/cron/paper-desk-tick/route.ts` — cron / failover tick endpoint

**Diagnostics:**
- `client/src/lib/strategySignalTrace.ts` — signal trace rows and gate summaries
- `client/src/lib/noTradeRootCause.ts` — no-trade diagnosis
- `client/src/components/SignalTracePanel.tsx` — signal trace UI
- `client/src/app/api/strategy-signal-trace/route.ts` — signal trace API

**Tests (run these first for any change):**
- `client/src/lib/futuresDeskPolicy.test.ts`
- `client/src/lib/futuresPaperMath.test.ts`
- `client/src/lib/futuresSignals.test.ts`
- `client/src/lib/tests/paperDeskWorker.test.ts`
- `client/src/lib/tests/strategySignalTrace.test.ts`
- `client/src/lib/tests/noTradeRootCause.test.ts`
- `client/src/lib/tests/regressionGuard.test.ts`

---

## Fast File Map — Go Engine

**Entry points:**
- `engine/cmd/antigravity/main.go` — main engine (600+ strategies, BTC + NIFTY paper, AI, risk, kill switch)
- `engine/cmd/backtest/main.go` — offline backtesting
- `engine/cmd/perfbench/main.go` — performance benchmarking

**Critical internal modules:**
- `engine/internal/killswitch/` — kill switch (must stay wired in all prod paths)
- `engine/internal/risk/gate/` — risk gates (must precede execution)
- `engine/internal/omsv3/` — OMS v3
- `engine/internal/ledger/` — ledger
- `engine/internal/reconciliation/` — reconciliation
- `engine/internal/strategy/` — 600+ strategies, curated_registry.go, scalpers/
- `engine/internal/backtest/` — backtest engine (v3, commission, context builder, scaler)

**Execution data flow:**
```
Market Data → Strategy Registry → Risk Gate → OMS v3 → Execution
→ Fill → Position → Ledger → Reconciliation → Kill Switch check
→ Persistence → Next.js API → Dashboard
```

---

## Debug Flow for Desk Bugs

Trace in this exact order — stop at the first failing step:

1. **Signal** — `futuresSignals.ts`
2. **Gate/policy** — `futuresDeskPolicy.ts`
3. **Open position** — `useBTCFuturesScalperEngine.ts` or `runPaperDeskPollTick.ts`
4. **Mark/update/exit** — same files above
5. **Paper math booking** — `futuresPaperMath.ts`
6. **UI display** — `BTCFuturesScalper.tsx`

**Entry gate order** (see REFERENCE.md for full list):
`DATA → DISABLED → SUSPENDED → ROTATION → COOLDOWN → OCCUPIED → REGIME → SIGNAL → CONFIRM → QUALITY → MTF → ATR_FEES → SPREAD → SESSION → CATEGORY → SAME_SIDE → MARGIN → MAX_OPEN → OPENED`

**No trades?** → Check Signal Trace summary for dominant rejected gate → inspect that gate's file only.

**Wrong PnL/return?** → See PnL formulas in REFERENCE.md. Distinguish price-move% from return-on-margin.

**Worker/cron mismatch?** → Start at `runPaperDeskPollTick.ts`, compare state shape with browser engine.

**UI data mismatch?** → Start at `BTCFuturesScalper.tsx`, determine data source before editing.

---

## Hard Invariants — Never Change Without Explicit User Approval

- Funding accrual and `lastFundingAppliedAt` semantics
- Liquidation only on true modeled cross — no preemptive exits
- Taker fee and round-trip fee accounting
- `paperNetPnlOnClose` booking math
- Entry/exit price consistency between booked PnL and trade row display
- No synthetic PnL bumps
- Existing widen/skip, fake-diversity, hold-multiplier, regime, same-dir, min-move gate semantics
- `MAX_OPEN_POSITIONS` cap
- Kill switch wiring in all prod paths
- Risk gates before execution (never bypass)
- WINNERS_ONLY gate — do not re-add losing strategies
- NSE/BSE strategies must be gated by market session

---

## PnL Quick Reference

```text
LONG  gross = ((exitPrice - entryPrice) / entryPrice) * notional
SHORT gross = ((entryPrice - exitPrice) / entryPrice) * notional
fees        = notional * takerFeePct * 2   (round-trip taker)
netPnl      = grossPnl - fees - fundingCosts
netPnlPct   = (netPnl / marginUsed) * 100  (leverage-amplified)
```

Full formulas, funding scaling, and ATR-fee gate math → see [REFERENCE.md](REFERENCE.md).

---

## Strategy Registry

- Location: `engine/internal/strategy/curated_registry.go`
- Count: 600+ live strategies
- Pre-live registry: `engine/internal/strategy/scalpers/pre_live_registry.go`
- Research strategies: `engine/internal/strategy/scalpers/research_registry.go` + `research_strategies_*.go`
- Families: EMA Cross, RSI threshold/slope, Bollinger Band, Funding/CVD, Delta absorption, Liquidity sweep, FVG retest, Order block, MSS continuation, Microstructure, Volume profile

---

## Deployment

| Target | Platform |
|---|---|
| `client/` | Vercel |
| `engine/` | AWS Lightsail |
| MongoDB Atlas, PostgreSQL Neon, Redis | Cloud-managed |

Max **2 Vercel cron jobs** (Hobby plan). Count before adding any new cron.

---

## Testing Checklist

```bash
cd client
npm run test    # run targeted test file first, then all
npm run build
```

- PnL/fee bug → update `futuresPaperMath.test.ts` with hand-calculated cases
- Policy/gate bug → update `futuresDeskPolicy.test.ts`
- Worker/cron bug → update `paperDeskWorker.test.ts`
- Signal trace / no-trade → update `strategySignalTrace.test.ts` / `noTradeRootCause.test.ts`

---

## Conventions

- Pure logic → `client/src/lib/`. React state → hooks/components. Never mix.
- Client env vars use `NEXT_PUBLIC_*`. Server secrets stay server-side only.
- Prefer existing helpers over new abstractions.
- Keep changes narrow. One concern per PR.
- For UI label changes: fix display only, do not touch booking math unless explicitly asked.
- After source changes: `npm run graphify:update` (small) or `npm run graphify:rebuild` (broad structural).

For deeper details on env vars, gate order, browser vs worker comparison, and common debugging paths → [REFERENCE.md](REFERENCE.md).
