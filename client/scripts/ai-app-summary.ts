#!/usr/bin/env tsx
/**
 * AI App Summary — "read this first" CLI for future AI agents.
 *
 * Usage:
 *   cd client
 *   npm run ai:summary
 *   # or directly:
 *   npx tsx scripts/ai-app-summary.ts
 *
 * Prints:
 *   - App mind map summary
 *   - Latest tracker report
 *   - Current worker health
 *   - Current no-trade blocker
 *   - Top recommended next action
 *   - Files future AI should read first
 *
 * Designed to output a compact block an operator can paste into an AI
 * session instead of sending the entire app context.
 */

// Env: locally run with `npx tsx --env-file=.env.local scripts/ai-app-summary.ts`
// On VPS: env vars already set in pm2 ecosystem config.
import { join } from "path";
import { readFileSync } from "fs";
import type { AiAppTrackerSnapshot } from "../src/lib/aiAppTracker/types";

// Dynamic import so the script works even without a compiled build
async function main() {
  const { MongoClient } = await import("mongodb");
  const uri = process.env.MONGODB_URI;
  if (!uri) {
    console.error("[ai-app-summary] MONGODB_URI not set. Add it to client/.env.local");
    process.exit(1);
  }

  const dbName = process.env.MONGODB_DB?.trim() || "loop_trades";
  const client = new MongoClient(uri, { serverSelectionTimeoutMS: 8_000 });

  try {
    await client.connect();
    const db = client.db(dbName);

    // ── 1. Latest tracker report ─────────────────────────────────────────────
    const trackerCol = db.collection("ai_app_tracker_reports");
    const latestReport = await trackerCol
      .find({ module: "btc_future_trading" })
      .sort({ created_at: -1 })
      .limit(1)
      .next() as Record<string, unknown> | null;

    // ── 2. Current paper_state ───────────────────────────────────────────────
    const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY?.trim();
    const stateCol = db.collection("paper_state");
    const state = accountKey
      ? await stateCol.findOne({ account_key: accountKey }) as Record<string, unknown> | null
      : null;

    const workerLastPollAt = state?.worker_last_poll_at as number | null ?? null;
    const workerAgeS = workerLastPollAt
      ? Math.floor((Date.now() - workerLastPollAt) / 1000)
      : null;
    const workerHealthy = workerAgeS != null && workerAgeS < 90;
    const workerStatus = workerAgeS != null
      ? `${workerHealthy ? "LIVE" : "STALE"} (${workerAgeS}s ago)`
      : "UNKNOWN";

    const funnel = state?.entry_funnel_snapshot as Record<string, unknown> | null ?? null;
    const dominantBlocker = funnel?.dominantBlocker as string ?? "unknown";
    const recommendation = funnel?.recommendation as string ?? "—";
    const openPositions = Array.isArray(state?.positions) ? state!.positions.length : 0;

    // ── 3. Render ────────────────────────────────────────────────────────────
    const SEP = "─".repeat(60);

    console.log(`\n${SEP}`);
    console.log("  BTC Paper Futures Desk — AI App Summary");
    console.log(`  Generated: ${new Date().toISOString()}`);
    console.log(SEP);

    console.log("\n[Worker Health]");
    console.log(`  Status        : ${workerStatus}`);
    console.log(`  Open positions: ${openPositions}`);
    console.log(`  Owner         : ${state?.worker_owner ?? "unknown"}`);

    console.log("\n[Entry Funnel (last tick)]");
    console.log(`  Dominant blocker : ${dominantBlocker}`);
    console.log(`  Recommendation   : ${recommendation}`);
    console.log(`  Active strategies: ${funnel?.activeStrategies ?? "?"}`);
    console.log(`  Opened last tick : ${funnel?.opened ?? 0}`);

    // Signal trace
    const traceRaw = state?.signal_trace_latest as Record<string, unknown> | null ?? null;
    const traceSummary = traceRaw?.summary as Record<string, unknown> | null ?? null;
    if (traceSummary) {
      const traceAgeS = traceRaw?.tickAt ? Math.floor((Date.now() - Number(traceRaw.tickAt)) / 1000) : null;
      console.log("\n[Signal Trace (last tick)]");
      console.log(`  Signal trace: evaluated=${traceSummary.totalEvaluated} fired=${traceSummary.fired} candidates=${traceSummary.candidates} opened=${traceSummary.opened} topReject=${traceSummary.topRejectedGate ?? "none"}${traceAgeS != null ? ` (${traceAgeS}s ago, ${traceRaw?.mode ?? "?"})` : ""}`);
    }

    if (latestReport) {
      const r = latestReport;
      const ageMin = Math.floor(
        (Date.now() - new Date(r.created_at as string).getTime()) / 60_000,
      );
      console.log("\n[Latest Tracker Report]");
      console.log(`  Report ID : ${r.report_id}`);
      console.log(`  Created   : ${r.created_at} (${ageMin}m ago)`);
      console.log(`  Severity  : ${(r.severity as string).toUpperCase()}`);
      console.log(`  Summary   : ${r.summary}`);
      const recs = (r.recommendations as string[]) ?? [];
      if (recs.length) {
        console.log("\n[Top Recommended Next Action]");
        console.log(`  ${recs[0]}`);
        if (recs.length > 1) {
          console.log("\n[Additional Recommendations]");
          recs.slice(1).forEach((rec, i) => console.log(`  ${i + 2}. ${rec}`));
        }
      }

      // Self-healing actions derived from the snapshot
      const snapshot = r.snapshot as AiAppTrackerSnapshot | undefined;
      if (snapshot) {
        const { recommendHealingActions } = await import("../src/lib/deskSelfHealing");
        const healing = recommendHealingActions(snapshot);
        const actionable = healing.filter((a) => a.type !== "NO_ACTION");
        if (actionable.length > 0) {
          console.log(`\n[Self-Healing Actions (${actionable.length})]`);
          actionable.slice(0, 4).forEach((a, i) => {
            const tag = a.safeToAutomate ? " [safe-to-automate]" : "";
            console.log(`  ${i + 1}. [${a.severity.toUpperCase()}] [${a.type}]${tag} ${a.title}`);
            console.log(`     reason : ${a.reason}`);
            console.log(`     action : ${a.operatorAction}`);
            if (a.copyableCommand) console.log(`     cmd    : ${a.copyableCommand}`);
          });
        }
      }

      // Auto-heal log from the stored report (server-side executor results)
      const healingExecuted = (r.healingExecuted as Array<{
        actionType: string; title: string; status: string;
        reason?: string; durationMs?: number;
      }> | undefined) ?? [];
      if (healingExecuted.length > 0) {
        const executedCount = healingExecuted.filter((h) => h.status === "executed").length;
        console.log(
          `\n[Auto-Heal Log] (${executedCount}/${healingExecuted.length} executed; env DESK_SELF_HEAL_AUTO=${process.env.DESK_SELF_HEAL_AUTO ?? "0"})`,
        );
        healingExecuted.slice(0, 4).forEach((h, i) => {
          const ms = h.durationMs != null ? ` ${h.durationMs}ms` : "";
          console.log(`  ${i + 1}. [${h.status.toUpperCase()}${ms}] ${h.actionType} — ${h.title}`);
          if (h.reason) console.log(`     reason: ${h.reason}`);
        });
      }
    } else {
      console.log("\n[Tracker Report]");
      console.log("  No reports yet. Run: curl -X POST http://localhost:3000/api/ai-app-tracker/capture");
    }

    // ── 4. Mind map pointer ──────────────────────────────────────────────────
    console.log("\n[Mind Map]");
    console.log("  Markdown : client/docs/AI_APPLICATION_MINDMAP.md");
    console.log("  JSON     : client/docs/ai-application-mindmap.json");

    // ── 5. Files AI should read first ────────────────────────────────────────
    console.log("\n[Files AI Should Read First]");
    const keyFiles = [
      "client/AI_README.md                          — start here",
      "client/docs/AI_APPLICATION_MINDMAP.md        — full system topology",
      "client/src/lib/deskEntryFunnelSnapshot.ts    — pure funnel logic",
      "client/src/lib/futuresSessionMetrics.ts      — probe exclusion",
      "client/src/lib/futuresDeskPolicy.ts          — entry gates",
      "client/src/hooks/useBTCFuturesScalperEngine.ts — browser engine",
      "client/scripts/btc-ft-paper-worker.ts        — VPS worker",
    ];
    keyFiles.forEach((f) => console.log(`  ${f}`));

    // ── 6. Compact copy block ─────────────────────────────────────────────────
    const copyBlock = [
      `[BTC Paper Desk AI Context — ${new Date().toISOString()}]`,
      `Worker: ${workerStatus}`,
      `Blocker: ${dominantBlocker}`,
      `Action: ${recommendation}`,
      latestReport
        ? `Last report (${(latestReport.severity as string).toUpperCase()}): ${latestReport.summary}`
        : "No tracker report yet.",
      `Read first: client/AI_README.md | client/docs/AI_APPLICATION_MINDMAP.md`,
    ].join("\n");

    console.log(`\n${SEP}`);
    console.log("  COMPACT AI CONTEXT (copy-paste to AI session):");
    console.log(SEP);
    console.log(copyBlock);
    console.log(`${SEP}\n`);

  } finally {
    await client.close();
  }
}

main().catch((err) => {
  console.error("[ai-app-summary] Error:", err instanceof Error ? err.message : err);
  process.exit(1);
});
