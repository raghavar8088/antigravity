/**
 * GET /api/mock-trading/signal-tick
 *
 * Fetches live klines, evaluates the mock-trading strategy roster, persists the
 * latest signal trace snapshot, and returns CANDIDATE rows for the browser mock
 * engine to ingest into mock_trades.
 */

import { NextResponse, type NextRequest } from "next/server";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";
import { isMongoConfigured, upsertSignalTrace } from "@/lib/mongoTradesClient";
import {
  evaluateMockTradingSignals,
  MOCK_TRADING_MIN_BARS,
} from "@/lib/mockTradingSignalEvaluator";
import {
  fetchMockTradingKlines,
  sanitizeMockTradingSymbol,
} from "@/lib/mockTradingMarketData";

export const dynamic = "force-dynamic";
export const maxDuration = 15;

export async function GET(request: NextRequest): Promise<Response> {
  const url = new URL(request.url);
  const symbol = sanitizeMockTradingSymbol(url.searchParams.get("symbol"));
  const accountKey = OWNER_ACCOUNT_KEY;

  try {
    const { bars, markPrice } = await fetchMockTradingKlines(symbol);
    const result = evaluateMockTradingSignals({ bars, markPrice, symbol });

    if (isMongoConfigured()) {
      await upsertSignalTrace(accountKey, {
        tickAt: result.tickAt,
        mode: "browser",
        symbol: result.symbol,
        summary: result.summary,
        rows: result.rows,
      });
    }

    const ageSeconds = 0;
    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      ageSeconds,
      markPrice: result.markPrice,
      regime: result.regime,
      bars: result.bars,
      minBars: MOCK_TRADING_MIN_BARS,
      activeStrategies: result.activeStrategies,
      evaluatedStrategies: result.evaluatedStrategies,
      candidateCount: result.candidateCount,
      summary: result.summary,
      rows: result.rows,
      error: result.error,
      fetchedAt: new Date(result.tickAt).toISOString(),
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "signal tick failed";
    return NextResponse.json(
      {
        ok: false,
        error: message,
        account_key: accountKey,
        rows: [],
        summary: null,
        markPrice: 0,
      },
      { status: 502 },
    );
  }
}
