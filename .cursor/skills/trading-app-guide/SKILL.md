---
name: trading-app-guide
description: Helps agents work on this BTC futures trading app, including the paper desk, signal trace, paper worker, replay, PnL math, strategy policy, and Next.js client. Use when debugging or changing BTC Future Trading, paper-desk execution, strategy gates, closed trades, worker/cron behavior, or UI display.
---

# Trading App Guide

## Quick Context

This repo is a BTC futures trading application. The main app is under `client/`.

Stack:
- Next 16
- React 19
- TypeScript 5
- Vitest
- MongoDB-backed paper state and trade history

There is no root `package.json`. Run app commands from `client/`.

Default validation:

```bash
cd client
npm run test
npm run build
```

Avoid rereading the whole repo. Start from the file map below and follow imports only when needed.

For deeper details on gate order, worker/browser parity, PnL formulas, env vars, and debugging paths, read [REFERENCE.md](REFERENCE.md).

## Fast File Map

Core:
- `client/package.json` - scripts, dependencies, versions.
- `client/src/components/BTCFuturesScalper.tsx` - BTC Future Trading UI, stats, tables, controls.
- `client/src/hooks/useBTCFuturesScalperEngine.ts` - browser paper engine loop and position lifecycle.

Policy and math:
- `client/src/lib/futuresDeskPolicy.ts` - desk constants, env parsing, gates, strategy build policy.
- `client/src/lib/futuresPaperMath.ts` - PnL, fees, funding, margin, liquidation, slippage, price move math.
- `client/src/lib/futuresSignals.ts` - signal input building, signal evaluation, confirmation, regime classification.
- `client/src/lib/futuresStrategies.ts` - strategy definitions and category metadata.

Worker and cron:
- `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` - headless worker tick path.
- `client/scripts/btc-ft-paper-worker.ts` - long-running paper worker process.
- `client/src/app/api/cron/paper-desk-tick/route.ts` - cron/failover tick endpoint.

Diagnostics:
- `client/src/lib/strategySignalTrace.ts` - signal trace rows and summaries.
- `client/src/lib/noTradeRootCause.ts` - no-trade diagnosis.
- `client/src/components/SignalTracePanel.tsx` - signal trace UI.
- `client/src/app/api/strategy-signal-trace/route.ts` - signal trace API.

Important tests:
- `client/src/lib/futuresDeskPolicy.test.ts`
- `client/src/lib/futuresPaperMath.test.ts`
- `client/src/lib/futuresSignals.test.ts`
- `client/src/lib/tests/paperDeskWorker.test.ts`
- `client/src/lib/tests/strategySignalTrace.test.ts`
- `client/src/lib/tests/noTradeRootCause.test.ts`
- `client/src/lib/tests/regressionGuard.test.ts`

## Debug flow for desk bugs

Trace in this order:

1. Signal
2. Gate/policy
3. Open position
4. Mark/update/exit
5. Paper math booking
6. UI table display

Usual path:
`futuresSignals.ts` -> `futuresDeskPolicy.ts` -> `useBTCFuturesScalperEngine.ts` or `runPaperDeskPollTick.ts` -> `futuresPaperMath.ts` -> `BTCFuturesScalper.tsx`.

For UI issues, start with `BTCFuturesScalper.tsx` and nearby components.

For worker/cron issues, start with `runPaperDeskPollTick.ts`, `btc-ft-paper-worker.ts`, and `/api/cron/paper-desk-tick`.

## Invariants

Do not change these unless the user explicitly asks:

- Funding accrual semantics.
- `lastFundingAppliedAt` behavior.
- Liquidation only on true cross.
- Taker fee and round-trip fee accounting.
- `paperNetPnlOnClose` booking math.
- Entry/exit price consistency between booked PnL and displayed trade rows.
- No synthetic PnL bumps.
- No preemptive liquidation exits.
- Existing widen/skip, fake-diversity, hold-multiplier, regime, same-dir, and min-move gate semantics.
- `MAX_OPEN_POSITIONS` unless explicitly requested.

When changing display labels, keep booking math unchanged unless the task is explicitly about accounting.

## Testing Checklist

Use targeted tests first, then full checks:

```bash
cd client
npm run test
npm run build
```

For PnL/fees bugs:
- Add or update `futuresPaperMath.test.ts`.
- Include hand-calculated gross, fees, net, and return cases.

For policy/gate bugs:
- Add or update `futuresDeskPolicy.test.ts`.

For worker/cron bugs:
- Add or update `client/src/lib/tests/paperDeskWorker.test.ts`.
- Confirm browser and worker paths use the same helpers where possible.

For signal trace or no-trade bugs:
- Add or update `strategySignalTrace.test.ts`.
- Add or update `noTradeRootCause.test.ts`.

## Conventions

- Keep changes narrow.
- Prefer existing helpers and local patterns over new abstractions.
- Pure logic belongs in `client/src/lib/`; React state belongs in hooks/components.
- Client-visible env vars use `NEXT_PUBLIC_*`; server secrets must stay server-side.
- If a bug looks like a UI issue, first confirm whether it is display-only, booking/math, exit price mismatch, replay/worker fork, or stale persisted state.
- Document root cause clearly in PR-style summaries.
