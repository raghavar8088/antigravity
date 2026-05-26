/**
 * POST /api/ai-app-tracker/capture
 *
 * Captures a live snapshot of the desk state and stores a tracker report in MongoDB.
 * Protected by CRON_SECRET when present.
 *
 * Response: { ok, reportId, severity, summary, recommendations }
 * No secrets (full account key, MONGODB_URI) ever appear in the response.
 */

import { NextResponse, type NextRequest } from "next/server";
import { isMongoConfigured, getAccountState, listTradesMongo } from "@/lib/mongoTradesClient";
import { insertAiTrackerReport } from "@/lib/aiAppTrackerMongo";
import { collectAppSnapshot } from "@/lib/aiAppTracker/collectAppSnapshot";
import { buildTrackerReport } from "@/lib/aiAppTracker/writeTrackerReport";
import { BTC_FT_DESK_BUILD } from "@/lib/btcFtDeskBuild";
import type { EntryFunnelSnapshot } from "@/lib/deskEntryFunnelSnapshot";

export const dynamic = "force-dynamic";
// Allow up to 30s for the Mongo queries on cold start
export const maxDuration = 30;

function authorized(request: NextRequest): boolean {
  const secret = process.env.CRON_SECRET?.trim();
  if (!secret) return true; // no secret configured — open (internal only)
  const authHeader = request.headers.get("authorization") ?? "";
  return authHeader === `Bearer ${secret}`;
}

export async function POST(request: NextRequest) {
  if (!authorized(request)) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY?.trim();
  if (!accountKey) {
    return NextResponse.json(
      { ok: false, error: "DESK_WORKER_ACCOUNT_KEY env not set — cannot read desk state" },
      { status: 503 },
    );
  }

  // Fetch paper_state
  const state = await getAccountState(accountKey);

  // Count closed trades (last 500 to avoid large scans)
  const closedTrades = await listTradesMongo({
    accountKey,
    limit: 500,
    moduleKey: "btc_future_trading",
  }).catch(() => []);

  const closedTradesCount = closedTrades.length;
  const lastTradeRaw = closedTrades[0]?.closed_at
    ? new Date(closedTrades[0].closed_at).getTime()
    : null;

  const openPositions = Array.isArray(state?.positions) ? state!.positions : [];
  const openPositionsCount = openPositions.length;

  const funnelSnapshot = (state?.entry_funnel_snapshot as EntryFunnelSnapshot | null) ?? null;
  const rotationReport = (state as Record<string, unknown> | null)?.rotation_report ?? null;

  // Detect probe-dominant: check if last 5 closed trades are all probes
  let probeDominant = false;
  if (closedTrades.length >= 5) {
    const last5 = closedTrades.slice(0, 5);
    const probeNames = ["BOOTSTRAP", "PROBE", "DEV_FORCE", "CANARY"];
    probeDominant = last5.every((t) => {
      const name = (t.strategy_name ?? "").toUpperCase();
      return probeNames.some((p) => name.includes(p));
    });
  }

  const snapshot = collectAppSnapshot({
    accountKey,
    buildSha: BTC_FT_DESK_BUILD ?? null,
    balance: state?.balance ?? null,
    dayStartBalance: state?.day_start_balance ?? null,
    workerLastPollAt: state?.worker_last_poll_at ?? null,
    workerOwner: state?.worker_owner ?? null,
    openPositionsCount,
    closedTradesCount,
    lastTradeAt: lastTradeRaw,
    funnelSnapshot,
    rotationReport,
    probeDominant,
  });

  const report = buildTrackerReport(snapshot);
  await insertAiTrackerReport(report);

  return NextResponse.json({
    ok: true,
    reportId: report.report_id,
    severity: report.severity,
    summary: report.summary,
    recommendations: report.recommendations,
  });
}
