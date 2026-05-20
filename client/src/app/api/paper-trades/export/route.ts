import { NextResponse } from "next/server";
import { buildPaperTradesCsv, type PaperTradeCsvExportRow } from "@/lib/paperTradesExport";
import { paperTradeExportQuerySchema } from "@/lib/paperTradesTypes";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  const parsed = paperTradeExportQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    window_days: url.searchParams.get("window_days") ?? undefined,
  });

  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const account_key = parsed.data.account_key as string;
  if (!account_key?.trim()) {
    return NextResponse.json({ ok: false, error: "account_key is required" }, { status: 400 });
  }

  const { window_days } = parsed.data;
  const cutoffMs = Date.now() - window_days * 24 * 60 * 60 * 1000;
  const cutoff = new Date(cutoffMs).toISOString();

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "No MongoDB configured" }, { status: 503 });
  }

  try {
    const col = await getTradesCollection();
    const docs = await col
      .find({ account_key, closed_at: { $gte: cutoff } })
      .project({
        closed_at: 1,
        symbol: 1,
        strategy_name: 1,
        side: 1,
        entry_price: 1,
        exit_price: 1,
        net_pnl: 1,
        fees: 1,
        funding_costs: 1,
        exit_reason: 1,
        _id: 0,
      })
      .sort({ closed_at: -1 })
      .toArray();

    const rows = docs as PaperTradeCsvExportRow[];
    const csv = buildPaperTradesCsv(rows);
    const filename = `paper-trades-${window_days}d.csv`;

    return new NextResponse(csv, {
      status: 200,
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": `attachment; filename="${filename}"`,
        "Cache-Control": "no-store",
      },
    });
  } catch (err) {
    console.error("[paper-trades/export]", err);
    return NextResponse.json({ ok: false, error: err instanceof Error ? err.message : "Mongo read failed" }, { status: 500 });
  }
}
