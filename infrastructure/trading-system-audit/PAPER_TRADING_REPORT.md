# Paper Trading Report

**Audit date:** 2026-06-09  
**Question:** Does paper trading use the same execution path, OMS, and risk controls as live?

---

## Answer: NO — Two Independent Paper Stacks

| Dimension | Go Engine Paper | Next.js Client Paper |
|-----------|-----------------|----------------------|
| Runtime | `antigravity/main.go` | Browser hook + worker + cron |
| Strategies | 606 (`BuildCuratedScalpers`) | 108 (`FUTURES_STRAT_DEFS`) |
| Execution entry | `executeThroughInstitutionalPath` | `runPaperDeskPollTick` / hook |
| Fill model | `PaperClient.ExecuteSignal` | `markSimulatedFill` |
| OMS | Ledger events + `omsv3.Replay` + Mongo transitions | `paperOms.ts` state machine |
| Risk gates | Risk V2 Kelly + PMS portfolio budget | Desk policy gates + drawdown lock |
| Position store | `positions.Manager` (in-process) | Mongo `paper_state` + in-memory |
| Persistence | SQLite 15s saver | Mongo trades + state |
| PnL math | `CanonicalNetPnL` (no funding/slippage) | `paperNetPnlOnClose` (funding + slippage) |
| SL/TP | `CheckStopLossAndTakeProfit` per tick | `paperResolveHardExit` per poll |
| Reconciliation | `PaperSnapshotProvider` (self-compare) | `portfolioConsistencyValidation` |

**Verdict:** Paper trading does **not** use a single unified path.

---

## Go Engine Paper Path

### Execution Chain
```
Strategy.OnTick → aggregator → executeThroughInstitutionalPath
  → Risk V2 Kelly → PMS CheckPortfolioRisk
  → submitInstitutionalOrder → PaperClient.ExecuteSignal
  → positions.Manager.OpenPosition
  → [tick] CheckStopLossAndTakeProfit
  → processCloseEvents → CanonicalNetPnL
```

### OMS
- Full ledger event sequence (CREATED → VALIDATED → RISK_APPROVED → SUBMITTED → ACKED → FILLED)
- Mongo OMS transitions via `persistOMSTransition`
- `omsv3.Replay` after create

### Risk Controls
| Control | Present |
|---------|---------|
| Kelly sizing | ✅ `risk/v2/kelly.go` |
| PMS portfolio heat/VaR/drawdown | ✅ `loop.go:435–452` |
| Family concentration cap | ✅ 30% |
| Kill switch | ✅ |
| Signal aggregator cooldown | ✅ |
| Max positions per strategy | ✅ `positions/manager.go` |

**Verdict:** **PASS** as self-contained institutional paper engine.

---

## Next.js Client Paper Path

### Execution Chain
```
Delta klines fetch → evalMinuteSignal → desk policy gates
  → createPaperOmsOrder → markSimulatedFill → WorkerPosition
  → [poll] paperResolveHardExit → inline PnL booking
```

### OMS
- Simplified: NEW → RISK_CHECKED → ACCEPTED → SIMULATED_FILL → POSITION_OPENED/CLOSED
- No REJECTED, CANCELLED, PARTIAL states
- Mongo persistence via `paperOmsMongo.ts`

### Risk Controls
| Control | Present |
|---------|---------|
| Kelly sizing | ✅ `strategyAllocation.ts` (after 20 trades) |
| PMS portfolio heat/VaR | ❌ |
| Drawdown entry lock | ✅ |
| MAX_OPEN_POSITIONS | ✅ |
| Burst/family caps | ✅ |
| Session gates | ✅ |
| Slippage model | ✅ |
| Funding accrual | ✅ |

**Verdict:** **PASS** as feature-rich paper desk; **FAIL** for parity with Go institutional path.

---

## Delta Live vs Paper

| Aspect | Paper (Go) | Paper (Client) | Delta Live |
|--------|------------|----------------|------------|
| Institutional gateway | ✅ internal | ❌ (blocked routes) | ✅ `ProcessExecutionRequest` |
| Broker fill | Simulated instant | Simulated instant | REST assumed |
| Reduce-only close | N/A | N/A | ✅ |
| Bridge mirror | N/A | N/A | Paper options → Delta orders |

Delta live is a **third path** — mirrors client paper options signals to exchange.

---

## Parity Gaps (Material)

| Gap | Go Paper | Client Paper | Risk if Assumed Same |
|-----|----------|--------------|----------------------|
| Strategy set | 606 | 108 | Different signals |
| Funding in PnL | No | Yes | PnL mismatch |
| Slippage | No | Yes | Fill price mismatch |
| PMS VaR/heat | Yes | No | Risk underestimate |
| OMS partial fills | No (live) | No | Same gap |
| Persistence | SQLite | Mongo | State fork |
| Worker vs browser | N/A | Possible env divergence | Duplicate trades |

---

## What Paper Proves About Live

| Claim | Provable from Paper? |
|-------|---------------------|
| Signal logic works | **YES** (both stacks) |
| OMS state machine correct | **YES** (Go, via tests) |
| Kelly sizing works | **YES** (Go) |
| SL/TP software exits work | **YES** (Go tick path) |
| Broker fill handling works | **NO** — paper assumes instant fill |
| Reconciliation works | **NO** — self-compare |
| Delta live safe | **NO** — third path with REST assumptions |

---

## Phase 12 Conclusion

| Question | Verdict |
|----------|---------|
| Same execution path? | **FAIL** |
| Same OMS? | **FAIL** |
| Same risk controls? | **FAIL** |
| Paper results predict live behavior? | **FAIL** |

**Overall Phase 12:** **FAIL** — paper trading is valuable for signal/PnL math validation but does not prove live execution correctness.
