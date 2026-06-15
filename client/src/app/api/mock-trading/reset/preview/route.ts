/**
 * FEAT 6 — Reset Impact Preview
 * GET ?account_key=... — returns stats without deleting anything.
 */
import { NextResponse } from "next/server";
import { isMongoConfigured, getDb } from "@/lib/broker/mongoTradesClient";
import { MOCK_TRADES_COLLECTION, MOCK_EQUITY_CURVE_COLLECTION } from "@/lib/trading/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key") ?? OWNER_ACCOUNT_KEY;

  try {
    const db = await getDb();
    const tradesColl = db.collection(MOCK_TRADES_COLLECTION);
    const equityColl = db.collection(MOCK_EQUITY_CURVE_COLLECTION);

    const [allTrades, equityDocs] = await Promise.all([
      tradesColl.find({ account_key: accountKey }).toArray(),
      equityColl
        .find({ account_key: accountKey })
        .sort({ timestamp: 1 })
        .limit(1)
        .toArray(),
    ]);

    const openTrades = allTrades.filter((t) => t.status === "OPEN");
    const closedTrades = allTrades.filter((t) => t.status === "CLOSED");

    const realizedPnl = closedTrades.reduce((acc, t) => acc + ((t.realized_pnl as number) ?? 0), 0);
    const unrealizedPnl = openTrades.reduce((acc, t) => acc + ((t.unrealized_pnl as number) ?? 0), 0);
    const feesPaid = allTrades.reduce((acc, t) => acc + ((t.fees as number) ?? 0), 0);

    const sessionStart = equityDocs[0]?.timestamp
      ? new Date(equityDocs[0].timestamp as number).toISOString()
      : null;

    const sessionDays = sessionStart
      ? Math.round((Date.now() - new Date(sessionStart).getTime()) / 86400000)
      : 0;

    // Latest equity from most recent equity point.
    const latestEquity = await equityColl
      .find({ account_key: accountKey })
      .sort({ timestamp: -1 })
      .limit(1)
      .toArray();

    const currentEquity = (latestEquity[0]?.equity as number) ?? 1_000_000;

    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      trade_count: allTrades.length,
      open_positions: openTrades.length,
      realized_pnl: realizedPnl,
      unrealized_pnl: unrealizedPnl,
      current_equity: currentEquity,
      session_start: sessionStart,
      session_days: sessionDays,
      fees_paid: feesPaid,
    });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
