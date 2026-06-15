/**
 * GET /api/strategy-intelligence
 *
 * Strategy health pool for terminal UI — aggregates closed-trade stats per strategy.
 * Query: view=all|retirement, limit=1..600
 */

import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { buildMockStrategyIntelRows } from "@/lib/trading/mockTradingSnapshotService";
import { mongoUnconfigured } from "@/lib/trading/mockTradingApiErrors";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

type IntelRow = Awaited<ReturnType<typeof buildMockStrategyIntelRows>>[number];

function summarize(rows: IntelRow[]) {
  return {
    total: rows.length,
    healthy: rows.filter((r) => r.status === "HEALTHY").length,
    warning: rows.filter((r) => r.status === "WARNING").length,
    critical: rows.filter((r) => r.status === "CRITICAL").length,
    insufficient_data: rows.filter((r) => r.status === "INSUFFICIENT_DATA").length,
  };
}

function filterRows(rows: IntelRow[], view: string, limit: number): IntelRow[] {
  let filtered = rows;
  if (view === "retirement") {
    filtered = rows.filter((r) => r.status === "CRITICAL" || r.allocation_tier === "F" || r.expectancy < 0);
    filtered.sort((a, b) => a.expectancy - b.expectancy);
  } else {
    filtered.sort((a, b) => b.evidence_score - a.evidence_score);
  }
  return filtered.slice(0, limit);
}

export async function GET(req: Request) {
  const { searchParams } = new URL(req.url);
  const view = searchParams.get("view") ?? "all";
  const limit = Math.min(Math.max(parseInt(searchParams.get("limit") ?? "100", 10) || 100, 1), 600);

  if (!isMongoConfigured()) return mongoUnconfigured();

  try {
    const allRows = await buildMockStrategyIntelRows(OWNER_ACCOUNT_KEY);
    const strategies = filterRows(allRows, view, limit);
    return NextResponse.json({
      ok: true,
      view,
      strategies,
      summary: summarize(allRows),
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        error: "strategy_intelligence_failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
