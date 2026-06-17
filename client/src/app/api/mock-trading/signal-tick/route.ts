/**
 * GET /api/mock-trading/signal-tick
 *
 * Fetches live klines, evaluates the mock-trading strategy roster, persists the
 * latest signal trace snapshot, and returns CANDIDATE rows for the browser mock
 * engine to ingest into mock_trades.
 */

import { NextResponse, type NextRequest } from "next/server";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";
import { isMongoConfigured, upsertSignalTrace } from "@/lib/broker/mongoTradesClient";
import { evaluateMockTradingTick } from "@/lib/mockTradingExecutor/evaluateMockTradingTick";
import { MOCK_TRADING_MIN_BARS } from "@/lib/trading/mockTradingSignalEvaluator";
import {
  fetchMockTradingKlines,
  sanitizeMockTradingSymbol,
} from "@/lib/trading/mockTradingMarketData";
import { STRATEGY_CATALOG } from "@/lib/strategyAuthority/strategyCatalog";

export const dynamic = "force-dynamic";
export const maxDuration = 15;

export async function GET(request: NextRequest): Promise<Response> {
  const url = new URL(request.url);
  const symbol = sanitizeMockTradingSymbol(url.searchParams.get("symbol"));
  const accountKey = url.searchParams.get("account_key")?.trim() || OWNER_ACCOUNT_KEY;

  try {
    const { bars, markPrice } = await fetchMockTradingKlines(symbol);
    const evalResult = evaluateMockTradingTick({ bars, markPrice, symbol });
    const rows = evalResult.rows.map((row) => ({ ...row, mode: "browser" as const }));

    if (isMongoConfigured()) {
      await upsertSignalTrace(accountKey, {
        tickAt: evalResult.tickAt,
        mode: "browser",
        symbol: evalResult.symbol,
        summary: evalResult.summary,
        rows,
      });
    }

    const ageSeconds = 0;
    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      status: "TRADE_ENGINE",
      ageSeconds,
      markPrice: evalResult.markPrice,
      regime: evalResult.regime,
      bars: evalResult.bars,
      minBars: MOCK_TRADING_MIN_BARS,
      activeStrategies: STRATEGY_CATALOG.length > 0 ? STRATEGY_CATALOG.length : evalResult.activeStrategies,
      evaluatedStrategies: evalResult.evaluatedStrategies,
      candidateCount: evalResult.candidateCount,
      summary: evalResult.summary,
      rows,
      error: evalResult.error,
      fetchedAt: new Date(evalResult.tickAt).toISOString(),
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
