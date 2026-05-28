/**
 * GET /api/replay-walkforward?days=30&strategy_ids=91,92,95
 *
 * Runs a deterministic bar-by-bar replay on the stored fixture candles then
 * ranks strategies by expectancy + walk-forward efficiency.
 *
 * Hard constraints:
 *   - Paper/research only. No live Delta order placement.
 *   - No forced trades, no threshold lowering, no gate bypassing.
 *   - Recommendations only — operator decides whether to apply.
 *   - Max 90 days of candles (129,600 bars at 1-minute resolution).
 */

import { NextResponse } from "next/server";
import {
  loadReplayFixture,
  replayFixturePath,
} from "@/lib/futuresReplayFixtures";
import { runPaperDeskReplay, summarizeReplayTrades } from "@/lib/futuresReplayEngine";
import { rankReplayStrategies } from "@/lib/replayWalkForwardRanker";

export const dynamic = "force-dynamic";

const MAX_DAYS = 90;
const BARS_PER_DAY = 1440; // 1-minute bars

export async function GET(req: Request): Promise<NextResponse> {
  try {
    const url = new URL(req.url);
    const days = Math.min(MAX_DAYS, Math.max(1, Number(url.searchParams.get("days") ?? "30")));
    const maxBars = days * BARS_PER_DAY;
    const strategyIdsParam = url.searchParams.get("strategy_ids") ?? "";
    const strategyIds = strategyIdsParam
      ? strategyIdsParam
          .split(",")
          .map((s) => Number(s.trim()))
          .filter((n) => Number.isFinite(n) && n > 0)
      : undefined;

    // Load candles — prefer live fixture, fall back to sample
    let candleFile;
    let fixtureKind: "live" | "sample" = "live";
    try {
      candleFile = loadReplayFixture("live", { maxBars });
    } catch {
      try {
        candleFile = loadReplayFixture("sample", { maxBars });
        fixtureKind = "sample";
      } catch {
        return NextResponse.json(
          {
            ok: false,
            error: "No replay fixture found. Run `npm run replay:fetch` first.",
            fixturePaths: {
              live: replayFixturePath("live"),
              sample: replayFixturePath("sample"),
            },
          },
          { status: 422 },
        );
      }
    }

    const candles = candleFile.candles;
    if (candles.length < 100) {
      return NextResponse.json(
        { ok: false, error: `Too few candles (${candles.length}). Run replay:fetch first.` },
        { status: 422 },
      );
    }

    const result = runPaperDeskReplay(candles, {
      initialBalance: 1000,
      leverage: 25,
      slippageBps: 5,
      maxPositions: 60,
      barMs: 60_000,
      symbol: "BTCUSD",
      signalThreshold: 26,
      minExpectedMoveSafetyK: 1.1,
      maxSameDirFracOfEquity: 0.35,
      minTpSlRatio: 2,
      fundingRate: candleFile.fundingRate ?? 0,
      strategyIds,
    });

    const summary = summarizeReplayTrades(result.trades);
    const rankings = rankReplayStrategies(result.trades);
    const promoted = rankings
      .filter((r) => r.recommendation === "PROMOTE")
      .map((r) => r.strategyId);

    return NextResponse.json({
      ok: true,
      days,
      barsProcessed: result.barsProcessed,
      generatedAt: new Date().toISOString(),
      fixtureKind,
      summary: {
        totalTrades: summary.count,
        sumNet: summary.sumNet,
        expectancy: summary.expectancy,
        finalBalance: result.finalBalance,
        exitReasonCounts: summary.exitReasonCounts,
      },
      rankings,
      promoted,
      canvasNote: "Recommendations only — operator decides whether to apply to the paper roster.",
    });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "Internal error" },
      { status: 500 },
    );
  }
}
