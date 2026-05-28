/**
 * GET /api/research-edge-report?window_days=30
 *
 * Research-grade edge report for the BTC Futures paper desk.
 * Fetches closed paper trades from MongoDB, scores each strategy, builds
 * regime rosters, and runs walk-forward validation — all returned as JSON.
 *
 * Hard invariants:
 *   - No secrets in the response (account key is never echoed).
 *   - Read-only: no trades modified, no state changed.
 *   - Paper only: no live order recommendations.
 *   - No guaranteed profit language.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured, listTradesMongo } from "@/lib/mongoTradesClient";
import { computeResearchEdgeScores } from "@/lib/researchEdgeScore";
import { buildRegimeRosters } from "@/lib/regimeRosterBuilder";
import { runWalkForwardValidation } from "@/lib/walkForwardValidation";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  const windowDays = Math.min(
    90,
    Math.max(7, Number(url.searchParams.get("window_days") ?? "30")),
  );

  const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY ?? "";

  if (!accountKey) {
    return NextResponse.json(
      { ok: false, error: "DESK_WORKER_ACCOUNT_KEY not set" },
      { status: 503 },
    );
  }
  if (!isMongoConfigured()) {
    return NextResponse.json(
      { ok: false, error: "MongoDB not configured" },
      { status: 503 },
    );
  }

  // Fetch trades — listTradesMongo returns closed_at DESC, limit 2000
  let rawTrades;
  try {
    rawTrades = await listTradesMongo({ accountKey, limit: 2000 });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: `MongoDB read error: ${String(err)}` },
      { status: 500 },
    );
  }

  // Filter to window
  const sinceIso = new Date(Date.now() - windowDays * 86_400_000).toISOString();
  const trades = rawTrades.filter((t) => t.closed_at >= sinceIso);

  // Score strategies
  const scores = computeResearchEdgeScores(trades);

  // Build regime rosters
  const regimeRosters = buildRegimeRosters(scores);

  // Walk-forward validation (all trades in window, not per-strategy)
  const wfTrades = [...trades]
    .sort((a, b) => a.closed_at.localeCompare(b.closed_at))
    .map((t) => ({ closedAt: t.closed_at, netPnl: t.net_pnl }));
  const walkForward = runWalkForwardValidation(wfTrades);

  // Quick summary lists
  const topPromote = scores
    .filter((s) => s.status === "PROMOTE")
    .map((s) => s.strategyId);
  const topDisable = scores
    .filter((s) => s.status === "DISABLE")
    .map((s) => s.strategyId);

  // Human-readable recommendations (no guaranteed profit language)
  const recommendations: string[] = [];
  if (topPromote.length > 0) {
    recommendations.push(
      `${topPromote.length} strateg${topPromote.length === 1 ? "y" : "ies"} meet promotion criteria after fees: [${topPromote.join(", ")}]. Review before enabling.`,
    );
  }
  if (topDisable.length > 0) {
    recommendations.push(
      `${topDisable.length} strateg${topDisable.length === 1 ? "y" : "ies"} recommended for disabling (negative expectancy + high fee drag): [${topDisable.join(", ")}].`,
    );
  }
  if (walkForward.status === "FAIL") {
    recommendations.push(
      `Walk-forward validation FAILED: ${walkForward.reason}`,
    );
  }
  if (walkForward.status === "COLLECT_DATA") {
    recommendations.push(
      "Insufficient trade history for walk-forward validation. Continue collecting paper trades.",
    );
  }
  if (regimeRosters.chopRoster.length === 0 && regimeRosters.trendRoster.length === 0) {
    recommendations.push(
      "No strategies qualify for any regime roster yet. Gather more trades or lower minTradesForVerdict (not threshold).",
    );
  }

  return NextResponse.json({
    ok: true,
    windowDays,
    tradeCount: trades.length,
    generatedAt: new Date().toISOString(),
    scores,
    regimeRosters,
    walkForward,
    topPromote,
    topDisable,
    recommendations,
  });
}
