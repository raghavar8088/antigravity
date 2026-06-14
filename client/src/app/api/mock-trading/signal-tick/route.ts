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
import {
  evaluateMockTradingSignals,
  MOCK_TRADING_MIN_BARS,
} from "@/lib/trading/mockTradingSignalEvaluator";
import {
  fetchMockTradingKlines,
  sanitizeMockTradingSymbol,
} from "@/lib/trading/mockTradingMarketData";
import { STRATEGY_CATALOG } from "@/lib/strategyAuthority/strategyCatalog";
import { fanOutGrade5CatalogSignals } from "@/lib/strategyAuthority/grade5CatalogSignals";
import { capTraceRows, summarizeSignalTrace } from "@/lib/ai/strategySignalTrace";

export const dynamic = "force-dynamic";
export const maxDuration = 15;

export async function GET(request: NextRequest): Promise<Response> {
  const url = new URL(request.url);
  const symbol = sanitizeMockTradingSymbol(url.searchParams.get("symbol"));
  const accountKey = url.searchParams.get("account_key")?.trim() || OWNER_ACCOUNT_KEY;

  try {
    const { bars, markPrice } = await fetchMockTradingKlines(symbol);
    const baseResult = evaluateMockTradingSignals({ bars, markPrice, symbol });

    // Trade Engine is the single active surface. When the legacy catalog exists,
    // fan out to catalog rows; otherwise use the active production roster rows.
    const rows = capTraceRows(
      (STRATEGY_CATALOG.length > 0
        ? fanOutGrade5CatalogSignals({
            catalog: STRATEGY_CATALOG,
            baseRows: baseResult.rows,
            tickAt: baseResult.tickAt,
            symbol: baseResult.symbol,
            regime: baseResult.regime,
          })
        : baseResult.rows
      ).map((row) => ({
        ...row,
        traceId: row.traceId.replace(/^grade5-/, "trade-engine-"),
        pipelineStage: "MAIN_ENGINE",
      })),
      500,
    );
    const candidateCount = rows.filter((row) => row.status === "CANDIDATE").length;
    const summary = summarizeSignalTrace(rows);

    if (isMongoConfigured()) {
      await upsertSignalTrace(accountKey, {
        tickAt: baseResult.tickAt,
        mode: "browser",
        symbol: baseResult.symbol,
        summary,
        rows,
      });
    }

    const ageSeconds = 0;
    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      status: "TRADE_ENGINE",
      ageSeconds,
      markPrice: baseResult.markPrice,
      regime: baseResult.regime,
      bars: baseResult.bars,
      minBars: MOCK_TRADING_MIN_BARS,
      activeStrategies: STRATEGY_CATALOG.length > 0 ? STRATEGY_CATALOG.length : baseResult.activeStrategies,
      evaluatedStrategies: baseResult.evaluatedStrategies,
      candidateCount,
      summary,
      rows,
      error: baseResult.error,
      fetchedAt: new Date(baseResult.tickAt).toISOString(),
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
