import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  listStrategyScores,
  listStrategyHealth,
  getStrategyHealthSummary,
  getClosedTradeStats,
} from "@/lib/paperDeskClient";
import { readSepStrategyEvidence, sepReportsAvailable, type SepStrategyRow } from "@/lib/sep/sepPipeline";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

function filterSepRows(rows: SepStrategyRow[], view: string, limit: number) {
  let sorted = [...rows];
  switch (view) {
    case "top":
    case "rankings":
      sorted.sort((a, b) => b.expectancy - a.expectancy);
      break;
    case "bottom":
      sorted.sort((a, b) => a.expectancy - b.expectancy);
      break;
    case "retirement":
      sorted = rows.filter((r) => r.status === "FAIL" || r.tier === "F" || r.expectancy < 0);
      sorted.sort((a, b) => a.expectancy - b.expectancy);
      break;
    default:
      sorted.sort((a, b) => b.evidence_score - a.evidence_score);
  }
  return sorted.slice(0, limit);
}

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  const { searchParams } = new URL(req.url);
  const limit = Math.min(parseInt(searchParams.get("limit") ?? "50", 10) || 50, 600);
  const view = searchParams.get("view") ?? "rankings";

  const sepRows = readSepStrategyEvidence();
  if (sepRows?.length) {
    return NextResponse.json({
      ok: true,
      source: "sep_filesystem",
      sep_available: true,
      view,
      strategies: filterSepRows(sepRows, view, limit),
      server_time: new Date().toISOString(),
    });
  }

  if (!isMongoConfigured()) return mongoUnconfigured();

  try {
    const [scores, health, summary, closedStats] = await Promise.all([
      listStrategyScores(auth.ctx.userId, limit),
      listStrategyHealth(auth.ctx.userId, undefined, limit),
      getStrategyHealthSummary(auth.ctx.userId),
      getClosedTradeStats(auth.ctx.userId),
    ]);

    const healthMap = new Map(health.map((h) => [h.strategy_id, h]));
    const rows = scores.map((s) => {
      const h = healthMap.get(s.strategy_id);
      return {
        strategy_id: s.strategy_id,
        status: h?.health_status ?? "INSUFFICIENT_DATA",
        enabled: h?.health_status === "HEALTHY" || h?.health_status === "WARNING",
        total_pnl: s.total_pnl,
        expectancy: s.expectancy,
        profit_factor: s.profit_factor,
        win_rate: s.win_rate,
        max_drawdown: s.max_drawdown,
        sample_size: s.sample_size,
        evidence_score: Math.round(Math.min(s.profit_factor / 2, 1) * 100),
        allocation_tier: s.profit_factor >= 1.5 ? "A" : s.profit_factor >= 1.25 ? "B" : s.profit_factor >= 1.1 ? "C" : "D",
        sharpe_ratio: null,
        sortino_ratio: null,
        regime: null,
        last_signal: h?.computed_at ?? s.computed_at,
      };
    });

    let filtered = rows;
    if (view === "top" || view === "rankings") filtered = [...rows].sort((a, b) => b.expectancy - a.expectancy).slice(0, limit);
    else if (view === "bottom") filtered = [...rows].sort((a, b) => a.expectancy - b.expectancy).slice(0, limit);
    else if (view === "retirement") filtered = rows.filter((r) => r.status === "CRITICAL" || r.expectancy < 0);

    return NextResponse.json({
      ok: true,
      source: "mongo_strategy_intelligence",
      sep_available: sepReportsAvailable(),
      view,
      summary,
      portfolio_stats: {
        total_realized_pnl: closedStats.realized_pnl,
        total_trades: closedStats.total_trades,
        win_rate: closedStats.win_rate,
      },
      strategies: filtered,
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
