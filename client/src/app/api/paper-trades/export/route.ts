import { NextResponse } from "next/server";
import { buildPaperTradesCsv, type PaperTradeCsvExportRow } from "@/lib/paperTradesExport";
import { assertCloudAccountMatchesSession } from "@/lib/paperTradesAuth";
import { getAuthenticatedPaperApiUser } from "@/lib/paperTradesApiAuth";
import { paperTradeExportQuerySchema } from "@/lib/paperTradesTypes";
import { createServiceSupabase } from "@/lib/supabase/server";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedPaperApiUser();
  if (!auth.ok) return auth.response;

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

  const match = assertCloudAccountMatchesSession(auth.ctx.userId, parsed.data.account_key);
  if (!match.ok) {
    return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const { window_days } = parsed.data;
  const account_key = match.userId;
  const cutoffMs = Date.now() - window_days * 24 * 60 * 60 * 1000;
  const cutoff = new Date(cutoffMs).toISOString();

  const { data, error } = await supabase
    .from("paper_trades")
    .select(
      "closed_at, symbol, strategy_name, side, entry_price, exit_price, net_pnl, fees, funding_costs, exit_reason",
    )
    .eq("account_key", account_key)
    .gte("closed_at", cutoff)
    .order("closed_at", { ascending: false });

  if (error) {
    console.error("[paper-trades/export]", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  const rows = (data ?? []) as PaperTradeCsvExportRow[];
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
}
