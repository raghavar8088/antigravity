/**
 * GET /api/strategy-rankings
 *
 * Returns the rankings JSON produced by `npm run rank:btc-ft`.
 * The hook fetches this on mount when `NEXT_PUBLIC_BTC_FT_USE_RANKED=1`
 * and passes the winner IDs to `resolveBtcFtActiveStrategyIds`.
 *
 * Response: { ok: true, rankings: BtcFtStrategyRankingRow[], winners: number[], source: string }
 */

import { NextResponse } from "next/server";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import type { BtcFtStrategyRankingRow } from "@/lib/trading/btcFtRoster";
import { winnerIdsFromRankings } from "@/lib/trading/btcFtRoster";

export const dynamic = "force-dynamic";

const RANKINGS_PATH = join(process.cwd(), "..", "fixtures", "replay", "btc_ft_strategy_rankings.json");

function loadRankings(): BtcFtStrategyRankingRow[] {
  try {
    if (!existsSync(RANKINGS_PATH)) return [];
    const raw = readFileSync(RANKINGS_PATH, "utf-8");
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (r) => typeof r === "object" && typeof r.id === "number" && typeof r.expectancy === "number",
    ) as BtcFtStrategyRankingRow[];
  } catch {
    return [];
  }
}

export async function GET(req: Request) {
  const url = new URL(req.url);
  const minExpectancy = Number(url.searchParams.get("minExpectancy") ?? "0");
  const minTrades = Math.max(1, Number(url.searchParams.get("minTrades") ?? "5"));

  const rankings = loadRankings();
  const winnerSet = winnerIdsFromRankings(rankings, minExpectancy, minTrades);
  const winners = [...winnerSet].sort((a, b) => a - b);

  return NextResponse.json({
    ok: true,
    rankings,
    winners,
    winnerCount: winners.length,
    totalRanked: rankings.length,
    source: rankings.length > 0 ? "replay" : "empty",
  });
}
