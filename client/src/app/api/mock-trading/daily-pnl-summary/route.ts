import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { listMockTrades } from "@/lib/trading/mockTradingMongo";
import { mockTradeListQuerySchema } from "@/lib/trading/mockTradingPersistenceTypes";
import { DEFAULT_MOCK_TRADING_CONFIG, computeDailyPnl } from "@/lib/trading/mockTradingEngine";

export const dynamic = "force-dynamic";

function mongoNotConfigured() {
  return NextResponse.json(
    {
      ok: false,
      code: "MONGO_NOT_CONFIGURED",
      error: "MongoDB not configured",
      hint: "Set MONGODB_URI and MONGODB_DB_NAME in the server environment",
    },
    { status: 503 },
  );
}

/**
 * Per-day PnL breakdown for the Daily PnL dashboard panel.
 *
 * Named "-summary" (not "/daily-pnl") because /api/mock-trading/daily-pnl
 * already exists as a raw read/write endpoint on the daily_pnl_history
 * collection (MockTradingDashboard.tsx POSTs per-tick points there) — reusing
 * that path would have broken that contract.
 *
 * Derives buckets from the same merged trade source as
 * /api/mock-trading/trades (mock_trades + Go-engine paper_trades), so totals
 * here always agree with what the Closed Trades table shows.
 */
export async function GET(req: Request) {
  const url = new URL(req.url);
  const parsed = mockTradeListQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    status: "CLOSED",
    limit: 50_000,
    sort: "oldest",
  });
  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, code: "VALIDATION_FAILED", error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  if (!isMongoConfigured()) return mongoNotConfigured();

  try {
    const { trades } = await listMockTrades(parsed.data);
    const result = computeDailyPnl(trades, DEFAULT_MOCK_TRADING_CONFIG.startingBalanceUsd);
    return NextResponse.json({
      ok: true,
      days: result.days,
      summary: result.summary,
      source: "mongo",
      storage: "mongo",
    });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_READ_FAILED",
        error: "Mongo read failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
