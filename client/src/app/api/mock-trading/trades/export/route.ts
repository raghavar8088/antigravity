/**
 * FEAT 2 — CSV Export of closed mock trades (RFC4180).
 */
import { getDb } from "@/lib/broker/mongoTradesClient";
import { MOCK_TRADES_COLLECTION } from "@/lib/trading/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

const CSV_HEADERS = [
  "trade_id",
  "strategy_name",
  "strategy_family",
  "side",
  "entry_price",
  "exit_price",
  "quantity",
  "notional",
  "leverage",
  "gross_pnl",
  "realized_pnl",
  "fees",
  "funding_costs",
  "opened_at",
  "closed_at",
  "exit_reason",
  "hold_minutes",
  "confidence_score",
  "regime_at_entry",
].join(",");

function escapeCsv(value: unknown): string {
  const str = value == null ? "" : String(value);
  if (str.includes(",") || str.includes('"') || str.includes("\n")) {
    return `"${str.replace(/"/g, '""')}"`;
  }
  return str;
}

export async function GET() {
  const accountKey = OWNER_ACCOUNT_KEY;

  try {
    const db = await getDb();
    const col = db.collection(MOCK_TRADES_COLLECTION);
    const trades = await col
      .find({ account_key: accountKey, status: "CLOSED" })
      .sort({ closed_at: -1 })
      .limit(50_000)
      .toArray();

    const rows = trades.map((t) => {
      const openedAt = t.opened_at ? new Date(t.opened_at as number).toISOString() : "";
      const closedAt = t.closed_at ? new Date(t.closed_at as number).toISOString() : "";
      const holdMinutes =
        t.opened_at && t.closed_at
          ? Math.round(((t.closed_at as number) - (t.opened_at as number)) / 60000)
          : 0;
      // gross_pnl = realized_pnl before fees
      const grossPnl = (t.realized_pnl as number ?? 0) + (t.fees as number ?? 0) + (t.funding_costs as number ?? 0);
      return [
        t.trade_id,
        t.strategy_name,
        t.strategy_family,
        t.side,
        t.entry_price,
        t.close_price,
        t.quantity,
        t.notional,
        t.leverage,
        grossPnl,
        t.realized_pnl,
        t.fees,
        t.funding_costs,
        openedAt,
        closedAt,
        t.exit_reason,
        holdMinutes,
        t.confidence_score,
        t.regime_at_entry,
      ]
        .map(escapeCsv)
        .join(",");
    });

    const csv = [CSV_HEADERS, ...rows].join("\r\n");

    return new Response(csv, {
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": `attachment; filename="mock_trades_${Date.now()}.csv"`,
      },
    });
  } catch (err) {
    return new Response(`error: ${err instanceof Error ? err.message : "unknown"}`, { status: 500 });
  }
}
