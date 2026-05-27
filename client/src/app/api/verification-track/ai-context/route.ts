/**
 * GET /api/verification-track/ai-context?window_minutes=120
 * Compact, paste-ready context for Cursor AI / future agents.
 * Never includes secrets or full account keys.
 */

import { NextResponse, type NextRequest } from "next/server";
import { listVerificationEvents } from "@/lib/verificationTrack/verificationTrackMongo";

export const dynamic = "force-dynamic";

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const windowMin = Math.min(720, Number(url.searchParams.get("window_minutes") || 120));
  const sinceMs = Date.now() - windowMin * 60_000;

  const events = await listVerificationEvents({ limit: 300, sinceMs });

  const rootCauseEv = events.find(e => e.type === "NO_TRADE_ROOT_CAUSE");
  const funnelEv = events.find(e => e.type === "ENTRY_FUNNEL");
  const workerEv = events.find(e => e.type === "WORKER_HEALTH");
  const opened = events.filter(e => e.type === "POSITION_OPENED").slice(0, 5);
  const closed = events.filter(e => e.type === "POSITION_CLOSED").slice(0, 5);
  const topGates = Object.entries(
    events.reduce((acc, e) => { if (e.gate) acc[e.gate] = (acc[e.gate] ?? 0) + 1; return acc; }, {} as Record<string, number>)
  ).sort((a, b) => b[1] - a[1]).slice(0, 5);

  const text = [
    "BTC Paper Desk Verification Summary",
    `Window: last ${windowMin} minutes`,
    `Latest root cause: ${rootCauseEv?.summary ?? "none"}`,
    `Latest funnel blocker: ${funnelEv?.dominant_blocker ?? "none"}`,
    `Worker health: ${workerEv?.summary ?? "unknown"}`,
    `Opened: ${opened.length}  Closed: ${closed.length}`,
    `Top rejected gates: ${topGates.map(g => `${g[0]}(${g[1]})`).join(", ") || "none"}`,
    "",
    "Recent opens:",
    ...opened.map(e => `  ${e.strategy_name} ${e.side} score=${e.signal_score?.toFixed(1)}`),
    "Recent closes:",
    ...closed.map(e => `  ${e.strategy_name} PnL=${e.net_pnl?.toFixed(2)}`),
    "",
    "Recommended files: runPaperDeskPollTick.ts, useBTCFuturesScalperEngine.ts, noTradeRootCause.ts, verificationTrack/*",
  ].join("\n");

  return NextResponse.json({
    ok: true,
    text,
    json: {
      latestRootCause: rootCauseEv?.summary ?? null,
      latestFunnelBlocker: funnelEv?.dominant_blocker ?? null,
      workerHealth: workerEv?.summary ?? null,
      recentOpenedTrades: opened.map(e => ({ strategy: e.strategy_name, side: e.side, score: e.signal_score })),
      recentClosedTrades: closed.map(e => ({ strategy: e.strategy_name, pnl: e.net_pnl, reason: e.exit_reason })),
      topRejectedGates: topGates,
      recommendedFiles: ["runPaperDeskPollTick.ts", "useBTCFuturesScalperEngine.ts", "noTradeRootCause.ts", "verificationTrack/*"],
    },
  });
}
