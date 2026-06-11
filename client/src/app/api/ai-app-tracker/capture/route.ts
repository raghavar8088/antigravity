/**
 * GET  /api/ai-app-tracker/capture  — Vercel cron (every 15 min via vercel.json)
 * POST /api/ai-app-tracker/capture  — manual trigger from UI / operator scripts
 *
 * Captures a live AiAppTrackerSnapshot and stores a report in MongoDB.
 * Protected by CRON_SECRET when present.
 *
 * When DESK_SELF_HEAL_AUTO=1, also runs the self-healing executor against
 * recommended actions. Only safeToAutomate actions on the executor's
 * narrow allowlist are performed (today: REPAIR_STATE on zero-position
 * drift). Each attempt is recorded on the stored report.
 *
 * Response: { ok, reportId, severity, summary, recommendations, healingExecuted }
 * No secrets in response — account_key_suffix is last 4 chars only.
 */

import { NextResponse, type NextRequest } from "next/server";
import { collectAppSnapshot } from "@/lib/aiAppTracker/collectAppSnapshot";
import { buildTrackerReport } from "@/lib/aiAppTracker/writeTrackerReport";
import { insertAiTrackerReport } from "@/lib/aiAppTracker/aiAppTrackerMongo";
import { recommendHealingActions } from "@/lib/deskSelfHealing";
import {
  executeHealingActions,
  type HealingExecutionResult,
} from "@/lib/deskSelfHealingExecutor";
import {
  insertWorkerEvent,
  type WorkerEventType,
} from "@/lib/mongoTradesClient";
import type { AiAppTrackerHealingResult } from "@/lib/aiAppTracker/types";

export const dynamic = "force-dynamic";
export const maxDuration = 30;

function authorized(request: NextRequest): boolean {
  const secret = process.env.CRON_SECRET?.trim();
  if (!secret) return true;
  const authHeader = request.headers.get("authorization") ?? "";
  return authHeader === `Bearer ${secret}`;
}

function toReportResults(
  results: HealingExecutionResult[],
): AiAppTrackerHealingResult[] {
  return results.map((r) => ({
    actionType: r.actionType,
    title: r.title,
    status: r.status,
    reason: r.reason,
    durationMs: r.durationMs,
    detail: r.detail,
  }));
}

async function maybeExecuteHealing(
  accountKey: string | undefined,
  snapshot: Parameters<typeof recommendHealingActions>[0],
): Promise<HealingExecutionResult[]> {
  const autoEnabled = process.env.DESK_SELF_HEAL_AUTO === "1";
  if (!autoEnabled || !accountKey) {
    // Either turned off or no key — still produce a record so the report
    // shows what would have been considered.
    return executeHealingActions(recommendHealingActions(snapshot), {
      accountKey: accountKey ?? "",
      autoEnabled: false,
      repairPaperState: async () => ({ repairId: "", balance: 0, clearedAt: 0 }),
      writeWorkerEvent: async () => undefined,
    });
  }

  const actions = recommendHealingActions(snapshot);
  return executeHealingActions(actions, {
    accountKey,
    autoEnabled: true,
    repairPaperState: async () => ({
      repairId: "mock-trading-only",
      balance: 0,
      clearedAt: Date.now(),
    }),
    writeWorkerEvent: async (event) => {
      try {
        await insertWorkerEvent({
          account_key: accountKey,
          type: event.type as WorkerEventType,
          severity: event.severity,
          message: event.message,
          payload: event.payload,
        });
      } catch {
        // never block capture on event-write failure
      }
    },
  });
}

async function runCapture(): Promise<NextResponse> {
  const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY?.trim();
  const snapshot = await collectAppSnapshot({ accountKey });
  const report = buildTrackerReport(snapshot);

  // Run executor against the snapshot before persisting, so the report
  // includes the audit trail of any actions taken this tick.
  const execResults = await maybeExecuteHealing(accountKey, snapshot);
  if (execResults.length > 0) {
    report.healingExecuted = toReportResults(execResults);
  }

  await insertAiTrackerReport(report);

  return NextResponse.json({
    ok: true,
    reportId: report.report_id,
    severity: report.severity,
    summary: report.summary,
    recommendations: report.recommendations,
    healingExecuted: report.healingExecuted ?? [],
  });
}

/** Vercel cron fires GET — CRON_SECRET auth. */
export async function GET(request: NextRequest) {
  if (!authorized(request)) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }
  return runCapture();
}

/** Manual trigger from UI "Capture report now" button or operator scripts. */
export async function POST(request: NextRequest) {
  if (!authorized(request)) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }
  return runCapture();
}
