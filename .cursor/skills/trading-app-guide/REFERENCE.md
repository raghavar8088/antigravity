# Trading App — Reference

Read this only when `SKILL.md` is insufficient. Use targeted search and narrow line ranges — do not read large files in full.

---

## Entry Pipeline Gate Order (Full)

```text
DATA          → price/candle data available
DISABLED      → strategy explicitly disabled
SUSPENDED     → strategy suspended by policy
ROTATION      → rotation policy blocking entry
COOLDOWN      → post-trade cooldown active
OCCUPIED      → slot already used by this strategy
REGIME        → market regime rejects this direction
SIGNAL        → signal score below threshold
CONFIRM       → confirmation check failed
QUALITY       → quality score below threshold
MTF           → multi-timeframe filter rejected
ATR_FEES      → expected move < safety-K × round-trip fees
SPREAD        → mark/last spread too wide
SESSION       → outside allowed UTC entry window
CATEGORY      → category cap reached
SAME_SIDE     → same-direction equity cap reached
MARGIN        → insufficient margin
MAX_OPEN      → global open position cap reached
OPENED        → position opened successfully
```

Primary files for gate tracing:
- `client/src/lib/futuresSignals.ts` — signal inputs, evaluation, confirmation, regime
- `client/src/lib/futuresDeskPolicy.ts` — env parsing, gates, caps
- `client/src/lib/strategySignalTrace.ts` — trace rows and gate summaries
- `client/src/components/SignalTracePanel.tsx` — UI view

---

## Browser Engine vs Worker Engine

| Concern | Browser | Worker |
|---|---|---|
| File | `useBTCFuturesScalperEngine.ts` | `runPaperDeskPollTick.ts` + `btc-ft-paper-worker.ts` |
| Trigger | UI mounted | Headless polling / cron |
| State | Browser paper state, UI positions/trades | Account state, persisted trades |
| Cron fallback | — | `api/cron/paper-desk-tick/route.ts` |

When a bug appears in only one path, compare these dimensions side-by-side:
1. Candidate selection and gate order
2. Notional/margin sizing
3. ATR-fee gate threshold
4. Exit price source
5. Fee/funding/PnL booking
6. State persistence shape

Both paths **must** share the same helpers from `futuresSignals.ts`, `futuresDeskPolicy.ts`, and `futuresPaperMath.ts`. If they have diverged, that is the bug.

---

## PnL, Fees, Funding, Return Formulas

All paper math lives in `client/src/lib/futuresPaperMath.ts`.

**Gross PnL:**
```text
LONG  gross = ((exitPrice - entryPrice) / entryPrice) * notional
SHORT gross = ((entryPrice - exitPrice) / entryPrice) * notional
```

**Round-trip taker fees:**
```text
fees = notional * takerFeePct * 2
```

**Net PnL on close:**
```text
netPnl = grossPnl - fees - fundingCosts
```

**Price move percent vs return on margin:**
```text
priceMovePct  LONG  = ((exitPrice - entryPrice) / entryPrice) * 100
priceMovePct  SHORT = ((entryPrice - exitPrice) / entryPrice) * 100
netPnlPct           = (netPnl / marginUsed) * 100   ← leverage-amplified
```

`netPnlPct` can look much larger than `priceMovePct`. Do not confuse them in UI labels.

**ATR-fee gate:**
```text
expectedMoveUsd = (atr14 / markPrice) * notional
thresholdUsd    = safetyK * paperRoundTripTakerFees(notional, takerFeePct)
gate passes when expectedMoveUsd >= thresholdUsd
```

**Funding invariant:**
- `funding_rate` is per funding interval, not per poll tick.
- Scale by elapsed time over the funding interval.
- Preserve `lastFundingAppliedAt` semantics exactly.

**Liquidation invariant:**
- Only on a true modeled cross.
- No preemptive liquidation exits.

---

## Key Environment Variables

**Desk sizing and entry quality:**
```
NEXT_PUBLIC_DESK_INITIAL_BALANCE
NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY
NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL
NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY
NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K
NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY
NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT
```

**Position caps and sessions:**
```
NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY
NEXT_PUBLIC_DESK_MAX_OPEN_PER_SIDE
NEXT_PUBLIC_DESK_MAX_OPEN_PER_TEMPLATE
NEXT_PUBLIC_DESK_ENTRY_UTC_START
NEXT_PUBLIC_DESK_ENTRY_UTC_END
NEXT_PUBLIC_DESK_SESSION_GATE
```

**BTC futures signal tuning:**
```
NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD
NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM
NEXT_PUBLIC_DESK_FORCE_PROBE_OPEN
```

**Regime and hold tuning:**
```
NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE
NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS
NEXT_PUBLIC_DESK_REGIME_WATCH_MS
NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW
NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST
```

**Profit mode:**
```
NEXT_PUBLIC_DESK_PROFIT_MODE
NEXT_PUBLIC_DESK_PROFIT_MIN_QUALITY
NEXT_PUBLIC_DESK_PROFIT_MIN_MTF
NEXT_PUBLIC_DESK_PROFIT_CHOP_THRESHOLD_BOOST
NEXT_PUBLIC_DESK_PROFIT_MIN_MOVE_K
NEXT_PUBLIC_DESK_PROFIT_MAX_OPEN
NEXT_PUBLIC_DESK_PROFIT_ROTATION_STRICT
```

**Worker and server:**
```
DESK_WORKER_ACCOUNT_KEY
DESK_WORKER_STORAGE_NAMESPACE
DESK_WORKER_STRATEGY_IDS
DELTA_API_BASE_URL
MONGODB_URI
```

**Engine (Go):**
```
PORT                  (default 8080, AWS Lightsail)
INTERNAL_API_URL      (Next.js → Go engine proxy)
SQLITE_PATH           (./data/engine.db)
MAX_POSITION_BTC
MAX_DAILY_LOSS_PCT
```

---

## Backtest Engine

Files:
- `engine/internal/backtest/runner.go` — main runner
- `engine/internal/backtest/context_builder.go` — context setup
- `engine/internal/backtest/execution/fills.go` — fill simulation
- `engine/internal/backtest/scaler_v3_adapter.go` — scaler adapter
- `engine/internal/backtest/v3/engine.go` — v3 engine
- `engine/internal/backtest/v3/commission_engine.go` — commission model
- `engine/internal/backtest/v3/config.go` — config
- `engine/internal/backtest/v3/types.go` — types
- `engine/cmd/run_backtest/main.go` — CLI entry
- `engine/data/backtest_*.json` — results (do not index as source)

---

## Common Debugging Paths

**No trades opening:**
1. Open Signal Trace panel or call `/api/strategy-signal-trace`.
2. Find the dominant rejected gate in the summary.
3. Open only the one file responsible for that gate.
4. Check `noTradeRootCause.ts` for structured diagnosis.

**Wrong PnL or return displayed:**
1. Confirm `entryPrice`, `exitPrice`, `side`, `notional`, `marginUsed`, `fees`, `fundingCosts` are correct.
2. Compare `paperNetPnlOnClose` with the displayed trade row value.
3. Distinguish `priceMovePct` from `netPnlPct` (leverage-amplified).
4. Fix label only vs fix booking math — these are separate changes.

**Worker stale or cron mismatch:**
1. Start at `runPaperDeskPollTick.ts`.
2. Compare worker state shape against browser engine state shape field by field.
3. Confirm signal/policy/math helpers are shared, not duplicated.

**UI data mismatch (wrong values displayed):**
1. Start at `BTCFuturesScalper.tsx`.
2. Identify whether data comes from: engine state, API state, Signal Trace, or persisted paper state.
3. Fix display labels and tooltips independently from booking math.

**Policy or gate change:**
1. Add/update `futuresDeskPolicy.test.ts` before touching source.
2. Preserve all invariants unless the user explicitly approved behavior changes.
3. Run targeted test → `npm run test` → `npm run build`.

**Go engine bug:**
1. Check execution flow: Market Data → Strategy Registry → Risk Gate → OMS v3 → Execution → Fill → Ledger.
2. Risk gates must precede execution — never bypass.
3. Kill switch (`engine/internal/killswitch/`) must remain wired.
4. Run `cd engine && go test ./...` after changes.

---

## AI Context Refresh

After any structural code change:
```bash
npm run graphify:update        # small changes
npm run graphify:rebuild       # broad structural changes
npm run ai-context:refresh     # rebuilds maps + Repomix summary
```
