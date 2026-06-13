import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { buildMockStrategyIntelRows } from "@/lib/trading/mockTradingSnapshotService";
import { listMockTrades } from "@/lib/trading/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const accountKey = OWNER_ACCOUNT_KEY;

  if (!isMongoConfigured()) {
    return NextResponse.json(
      { ok: false, code: "MONGO_NOT_CONFIGURED", error: "MongoDB not configured" },
      { status: 503 },
    );
  }

  const { searchParams } = new URL(req.url);
  const view = searchParams.get("view") ?? "all";

  try {
    const rows = await buildMockStrategyIntelRows(accountKey);
    const closed = await listMockTrades({
      account_key: accountKey,
      status: "CLOSED",
      page: 1,
      limit: 50_000,
      sort: "oldest",
    });

    const totalPnl = closed.trades.reduce((sum, t) => sum + t.realizedPnl, 0);
    const wins = closed.trades.filter((t) => t.realizedPnl >= 0).length;
    const winRate = closed.trades.length > 0 ? wins / closed.trades.length : 0;
    const grossWin = closed.trades.filter((t) => t.realizedPnl >= 0).reduce((s, t) => s + t.realizedPnl, 0);
    const grossLoss = closed.trades.filter((t) => t.realizedPnl < 0).reduce((s, t) => s + Math.abs(t.realizedPnl), 0);
    const profitFactor = grossLoss > 0 ? grossWin / grossLoss : grossWin > 0 ? 99 : 0;

    type IntelRow = (typeof rows)[number] & {
      avg_win: number;
      avg_loss: number;
      health_reasons: string[];
      last_computed: string;
    };

    const enriched: IntelRow[] = rows.map((row) => ({
      ...row,
      avg_win: 0,
      avg_loss: 0,
      health_reasons: [],
      last_computed: new Date().toISOString(),
    }));

    let filtered: IntelRow[];
    switch (view) {
      case "top20":
        filtered = [...enriched].sort((a, b) => b.expectancy - a.expectancy).slice(0, 20);
        break;
      case "top50":
        filtered = [...enriched].sort((a, b) => b.expectancy - a.expectancy).slice(0, 50);
        break;
      case "bottom20":
        filtered = [...enriched].sort((a, b) => a.expectancy - b.expectancy).slice(0, 20);
        break;
      case "retirement":
        filtered = enriched.filter(
          (r) => r.status === "CRITICAL" || r.allocation_tier === "F" || r.expectancy < 0,
        );
        break;
      default:
        filtered = enriched;
    }

    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      view,
      execution_authority: "mock-trading",
      total_strategies: enriched.length,
      summary: {
        healthy: enriched.filter((r) => r.status === "HEALTHY").length,
        warning: enriched.filter((r) => r.status === "WARNING").length,
        critical: enriched.filter((r) => r.status === "CRITICAL").length,
        insufficient_data: enriched.filter((r) => r.status === "INSUFFICIENT_DATA").length,
      },
      portfolio_stats: {
        total_realized_pnl: totalPnl,
        total_trades: closed.trades.length,
        win_rate: winRate,
        profit_factor: profitFactor,
        sharpe: null,
      },
      strategies: filtered,
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    return NextResponse.json(
      {
        ok: false,
        code: "MONGO_READ_FAILED",
        error: "Strategy intelligence read failed",
        detail: err instanceof Error ? err.message : "unknown",
      },
      { status: 500 },
    );
  }
}
