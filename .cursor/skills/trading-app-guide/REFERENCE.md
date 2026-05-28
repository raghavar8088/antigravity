# Trading App Reference

Read this only when the short `SKILL.md` is not enough. Prefer targeted searches and narrow file ranges over opening large files fully.

## Entry Pipeline Gate Order

Use Signal Trace first, then inspect the gate that rejected the candidate.

Typical desk flow:

```text
DATA
DISABLED
SUSPENDED
ROTATION
COOLDOWN
OCCUPIED
REGIME
SIGNAL
CONFIRM
QUALITY
MTF
ATR_FEES
SPREAD
SESSION
CATEGORY
SAME_SIDE
MARGIN
MAX_OPEN
OPENED
```

Primary files:

- `client/src/lib/futuresSignals.ts` - signal inputs, signal evaluation, confirmation, regime classification.
- `client/src/lib/futuresDeskPolicy.ts` - env parsing, strategy build policy, gates and caps.
- `client/src/lib/strategySignalTrace.ts` - trace rows, statuses, gate summaries.
- `client/src/components/SignalTracePanel.tsx` - UI view of per-strategy gates.

## Browser Engine vs Worker Engine

Browser engine:

- File: `client/src/hooks/useBTCFuturesScalperEngine.ts`
- Runs while the BTC Future Trading UI is mounted.
- Owns browser paper state, stats, UI-facing positions/trades, and local analysis helpers.
- Uses `futuresSignals.ts`, `futuresDeskPolicy.ts`, and `futuresPaperMath.ts`.

Worker engine:

- Files: `client/scripts/btc-ft-paper-worker.ts` and `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts`
- Runs headless for paper desk polling and writes account state/trades.
- Cron fallback route: `client/src/app/api/cron/paper-desk-tick/route.ts`
- Should reuse the same signal, policy, and paper math helpers where practical.

When a bug appears in only one path, compare:

- candidate selection and gate order
- notional/margin sizing
- ATR-fee gate
- exit price source
- fee/funding/PnL booking
- state persistence shape

## PnL, Fees, Funding, Return

Core paper math lives in `client/src/lib/futuresPaperMath.ts`.

Gross linear PnL:

```text
LONG gross = ((exitPrice - entryPrice) / entryPrice) * notional
SHORT gross = ((entryPrice - exitPrice) / entryPrice) * notional
```

Round-trip taker fees:

```text
fees = notional * takerFeePct * 2
```

Net PnL on close:

```text
netPnl = grossPnl - roundTripFees - fundingCosts
```

Price move percent:

```text
LONG priceMovePct = ((exitPrice - entryPrice) / entryPrice) * 100
SHORT priceMovePct = ((entryPrice - exitPrice) / entryPrice) * 100
```

Return on margin:

```text
netPnlPct = (netPnl / marginUsed) * 100
```

This is leverage-amplified and can look much larger than price move percent.

Minimum expected move vs fees:

```text
expectedMoveUsd = (atr14 / markPrice) * notional
thresholdUsd = safetyK * paperRoundTripTakerFees(notional, takerFeePct)
gate passes when expectedMoveUsd >= thresholdUsd
```

Funding invariant:

- Delta-style `funding_rate` is per funding interval, not per poll.
- Scale funding by elapsed time over the funding interval.
- Preserve `lastFundingAppliedAt` semantics.

Liquidation invariant:

- Liquidation only on a true modeled cross.
- No preemptive liquidation exits.

## Key Env Vars

Desk sizing and entry quality:

- `NEXT_PUBLIC_DESK_INITIAL_BALANCE`
- `NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY`
- `NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL`
- `NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY`
- `NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K`
- `NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY`
- `NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT`

Position caps and sessions:

- `NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY`
- `NEXT_PUBLIC_DESK_MAX_OPEN_PER_SIDE`
- `NEXT_PUBLIC_DESK_MAX_OPEN_PER_TEMPLATE`
- `NEXT_PUBLIC_DESK_ENTRY_UTC_START`
- `NEXT_PUBLIC_DESK_ENTRY_UTC_END`
- `NEXT_PUBLIC_DESK_SESSION_GATE`

BTC Future Trading route:

- `NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD`
- `NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM`
- `NEXT_PUBLIC_DESK_FORCE_PROBE_OPEN`

Regime and hold tuning analysis:

- `NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE`
- `NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS`
- `NEXT_PUBLIC_DESK_REGIME_WATCH_MS`
- `NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW`
- `NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST`

Profit mode:

- `NEXT_PUBLIC_DESK_PROFIT_MODE`
- `NEXT_PUBLIC_DESK_PROFIT_MIN_QUALITY`
- `NEXT_PUBLIC_DESK_PROFIT_MIN_MTF`
- `NEXT_PUBLIC_DESK_PROFIT_CHOP_THRESHOLD_BOOST`
- `NEXT_PUBLIC_DESK_PROFIT_MIN_MOVE_K`
- `NEXT_PUBLIC_DESK_PROFIT_MAX_OPEN`
- `NEXT_PUBLIC_DESK_PROFIT_ROTATION_STRICT`

Worker and server:

- `DESK_WORKER_ACCOUNT_KEY`
- `DESK_WORKER_STORAGE_NAMESPACE`
- `DESK_WORKER_STRATEGY_IDS`
- `DELTA_API_BASE_URL`
- `MONGODB_URI`

## Common Debugging Paths

No trades:

1. Check Signal Trace summary and dominant rejected gate.
2. Check no-trade diagnosis in `client/src/lib/noTradeRootCause.ts`.
3. Inspect only the gate file/function involved.

Wrong PnL or return display:

1. Confirm `entryPrice`, `exitPrice`, `side`, `notional`, `marginUsed`, `fees`, `fundingCosts`.
2. Compare `paperNetPnlOnClose` with the displayed trade row.
3. Distinguish price move percent from return on margin.

Worker stale or cron mismatch:

1. Start with `runPaperDeskPollTick.ts`.
2. Compare worker state shape with browser engine state shape.
3. Confirm signal/policy/math helpers are shared instead of duplicated.

UI data mismatch:

1. Start with `BTCFuturesScalper.tsx`.
2. Confirm whether data comes from engine state, API state, Signal Trace, or persisted paper state.
3. Fix labels/tooltips separately from booking math.

Policy or gate change:

1. Update or add `futuresDeskPolicy.test.ts`.
2. Preserve invariants unless the user explicitly requested behavior changes.
3. Run targeted tests, then `npm run test` and `npm run build` from `client/`.
