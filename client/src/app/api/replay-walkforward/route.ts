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
 *   - Refuses to run if candle coverage is < 80% of the requested window.
 */

import { NextResponse } from "next/server";
import {
  computeCoverageDays,
  hasSufficientCoverage,
  loadReplayFixtureForDays,
} from "@/lib/analytics/futuresReplayFixtures";
import { runPaperDeskReplay, summarizeReplayTrades } from "@/lib/analytics/futuresReplayEngine";
import { rankReplayStrategies } from "@/lib/ai/replayWalkForwardRanker";

export const dynamic = "force-dynamic";

const MAX_DAYS = 90;

export async function GET(req: Request): Promise<NextResponse> {
  try {
    const url = new URL(req.url);
    const days = Math.min(MAX_DAYS, Math.max(1, Number(url.searchParams.get("days") ?? "30")));
    const strategyIdsParam = url.searchParams.get("strategy_ids") ?? "";
    const strategyIds = strategyIdsParam
      ? strategyIdsParam
          .split(",")
          .map((s) => Number(s.trim()))
          .filter((n) => Number.isFinite(n) && n > 0)
      : undefined;

    // Load candles — prefer btcusd_1m_{N}d.json, fall back to live
    let candleFile;
    let usedFixturePath = "";
    try {
      const loaded = loadReplayFixtureForDays(days);
      candleFile = loaded.file;
      usedFixturePath = loaded.fixturePath;
    } catch (e) {
      return NextResponse.json(
        {
          ok: false,
          error: `No replay fixture found for ${days}d. Run: npm run replay:fetch -- --days=${days}`,
          fetchCommand: `npm run replay:fetch -- --days=${days}`,
        },
        { status: 422 },
      );
    }

    const candles = candleFile.candles;
    const candlesLoaded = candles.length;
    const coverageDays = computeCoverageDays(candlesLoaded);
    const sufficient = hasSufficientCoverage(candlesLoaded, days);

    if (!sufficient) {
      return NextResponse.json(
        {
          ok: false,
          error:
            `Insufficient replay data: ${candlesLoaded} candles = ${coverageDays.toFixed(1)} days ` +
            `(need ≥80% of ${days}d). Run: npm run replay:fetch -- --days=${days}`,
          candlesLoaded,
          coverageDays,
          requestedDays: days,
          fetchCommand: `npm run replay:fetch -- --days=${days}`,
        },
        { status: 422 },
      );
    }

    if (candlesLoaded < 100) {
      return NextResponse.json(
        { ok: false, error: `Too few candles (${candlesLoaded}). Run: npm run replay:fetch -- --days=${days}` },
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
      candlesLoaded,
      coverageDays,
      requestedDays: days,
      generatedAt: new Date().toISOString(),
      fixturePath: usedFixturePath,
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
