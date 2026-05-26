# AI Agent README — BTC Paper Futures Desk

**Read this before making any changes to this codebase.**

---

## Step 1 — Read the mind map first

```
client/docs/AI_APPLICATION_MINDMAP.md   ← full system topology (10 sections)
client/docs/ai-application-mindmap.json ← machine-readable JSON (modules, flows, constraints)
```

The mind map contains: runtime architecture, critical desk flow, key files, state ownership, env vars, hard constraints, failure modes, and debug checklist. Reading it saves you 10+ unnecessary file reads.

---

## Step 2 — Run `npm run ai:summary` before broad debugging

```bash
cd client
npm run ai:summary
```

This connects to MongoDB and prints:
- Current worker health (LIVE / STALE + age)
- Current no-trade blocker
- Latest tracker report (severity + summary + top recommendation)
- Files you should read first

Paste that compact output into your AI session instead of sending the full app context. It reduces your token usage by 80%+.

---

## Step 3 — Check the latest tracker report before editing

```bash
# CLI
npm run ai:summary

# Or via API
curl https://your-vercel-url/api/ai-app-tracker/latest
```

The tracker report contains:
- `severity`: info / warning / danger
- `summary`: one-line desk status
- `recommendations`: ordered action list
- `snapshot.dominantBlocker`: the current entry gate blocking trades
- `snapshot.warnings`: active warning codes

Don't edit code before understanding the current blocker. Most "no trades" issues are the signal gate working correctly, not a bug.

---

## Hard Constraints — Do NOT violate these

1. **Paper only.** No real order placement on Delta Exchange or any exchange.
2. **Never lower the signal threshold to force trades.** Minimum effective threshold is 26.
3. **Never bypass entry gates.** fee/ATR, regime, MTF, quality, rotation, session, spread — all mandatory.
4. **Never delete historical trades.** `paper_trades` docs are append-only; close by updating status.
5. **Canary must be OFF by default.** `NEXT_PUBLIC_DESK_ENTRY_CANARY` must not be `"1"` in production.
6. **MongoDB credentials server-side only.** `MONGODB_URI` must never be in any `NEXT_PUBLIC_*` var.
7. **`isProbeOrBootstrapTrade()` must exclude all synthetic trades.** Any new probe/canary type must be added there.
8. **No live trading.** The Go engine on AWS Lightsail `:8080` is separate; don't modify `engine/`.
9. **`funnelRecommendation()` must never suggest lowering the threshold.** This is tested.
10. **`workerLease.ts` must stay server-only.** Re-exporting it to browser causes Node.js bundle errors.

---

## Common Task Patterns

### "Why are no trades opening?"

1. Read `AiAppTrackerPanel` in the browser (Advanced tab of Desk Command Center).
2. Or: `npm run ai:summary` to see `dominantBlocker`.
3. Map the blocker: signal → market choppy (wait); regime → wrong market phase; atrFees → spread too high; noStrategies → check `DESK_WORKER_STRATEGY_IDS` env.
4. **Do not lower the threshold or bypass a gate.**

### "Worker is stale"

```bash
pm2 restart btc-ft-paper-worker   # on VPS
# or wait up to 60s for Vercel cron fallback
```

### "Balance drifted unexpectedly"

1. Check if probe/bootstrap trades are being excluded: `isProbeOrBootstrapTrade()` in `futuresSessionMetrics.ts`.
2. Use `computeSessionEquityFromProduction()` for equity display.

### "Adding a new entry gate"

1. Add to `EntryFunnelBlockerKey` union in `deskEntryFunnelSnapshot.ts`.
2. Add blocker count field to `EntryFunnelBlockerCounts`.
3. Add recommendation text to `BLOCKER_RECOMMENDATIONS` — never suggest lowering threshold.
4. Update `computeDominantBlocker()` priority logic.
5. Add test for the new blocker in `deskEntryFunnel.test.ts`.

### "After major changes"

1. `cd client && npx tsc --noEmit` — must pass zero errors.
2. `cd client && npx vitest run src/lib/tests/` — all 46+ tests must pass.
3. `npm run ai:mindmap` — updates the mind map timestamps.
4. `curl -X POST .../api/ai-app-tracker/capture` — creates a fresh tracker report.

---

## Tracker Report System

New tracker reports are captured:
- Automatically every 15 minutes via Vercel cron (`/api/ai-app-tracker/capture`)
- Manually via the "Capture report now" button in the AI App Tracker panel (Advanced tab)
- Via CLI: `curl -X POST https://your-vercel-url/api/ai-app-tracker/capture`

Reports are stored in MongoDB `ai_app_tracker_reports` with a 30-day TTL. They contain a full desk state snapshot and actionable recommendations.

---

## Key Paths Quick Reference

| What | Where |
|---|---|
| Mind map (human) | `client/docs/AI_APPLICATION_MINDMAP.md` |
| Mind map (machine) | `client/docs/ai-application-mindmap.json` |
| Tracker library | `client/src/lib/aiAppTracker/` |
| Tracker Mongo helpers | `client/src/lib/aiAppTrackerMongo.ts` |
| Tracker API routes | `client/src/app/api/ai-app-tracker/` |
| Tracker UI panel | `client/src/components/AiAppTrackerPanel.tsx` |
| CLI summary | `client/scripts/ai-app-summary.ts` |
| Funnel diagnostics | `client/src/lib/deskEntryFunnelSnapshot.ts` |
| Probe/canary exclusion | `client/src/lib/futuresSessionMetrics.ts` |
| Entry gates | `client/src/lib/futuresDeskPolicy.ts` |
| Strategy definitions | `client/src/lib/futuresStrategyDefs.ts` |
| Tests | `client/src/lib/tests/deskEntryFunnel.test.ts` |

---

## Keep Reports Updated

After any major change that affects desk behavior:
1. Run `npm run ai:mindmap` to update the mind map timestamp.
2. Capture a new tracker report so the next AI session starts with accurate data.

---

*This file is for AI agents. Human operators: see `client/docs/PAPER_DESK_RUNBOOK.md`.*
