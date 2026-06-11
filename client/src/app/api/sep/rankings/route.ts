import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { buildMockStrategyIntelRows } from "@/lib/mockTradingSnapshotService";
import { listMockTrades } from "@/lib/mockTradingMongo";
import { readSepStrategyEvidence, sepReportsAvailable, type SepStrategyRow } from "@/lib/sep/sepPipeline";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/mockTradingApiErrors";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

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
    const [rows, closed] = await Promise.all([
      buildMockStrategyIntelRows(OWNER_ACCOUNT_KEY),
      listMockTrades({ account_key: OWNER_ACCOUNT_KEY, status: "CLOSED", page: 1, limit: 50_000, sort: "oldest" }),
    ]);

    let filtered = rows;
    if (view === "top" || view === "rankings") filtered = [...rows].sort((a, b) => b.expectancy - a.expectancy).slice(0, limit);
    else if (view === "bottom") filtered = [...rows].sort((a, b) => a.expectancy - b.expectancy).slice(0, limit);
    else if (view === "retirement") filtered = rows.filter((r) => r.status === "CRITICAL" || r.expectancy < 0);

    const wins = closed.trades.filter((t) => t.realizedPnl >= 0).length;
    return NextResponse.json({
      ok: true,
      source: "mock_trading",
      execution_authority: "mock-trading",
      sep_available: sepReportsAvailable(),
      view,
      summary: {
        healthy: rows.filter((r) => r.status === "HEALTHY").length,
        warning: rows.filter((r) => r.status === "WARNING").length,
        critical: rows.filter((r) => r.status === "CRITICAL").length,
      },
      portfolio_stats: {
        total_realized_pnl: closed.trades.reduce((s, t) => s + t.realizedPnl, 0),
        total_trades: closed.trades.length,
        win_rate: closed.trades.length > 0 ? wins / closed.trades.length : 0,
      },
      strategies: filtered,
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
