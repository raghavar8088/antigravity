import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  listStrategyScores,
  listStrategyHealth,
  getStrategyHealthSummary,
  getClosedTradeStats,
} from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  const { searchParams } = new URL(req.url);
  const view = searchParams.get("view") ?? "all"; // top20 | top50 | bottom20 | retirement | all
  const limit = Math.min(parseInt(searchParams.get("limit") ?? "600", 10) || 600, 600);

  try {
    const [scores, health, summary, closedStats] = await Promise.all([
      listStrategyScores(accountKey, limit),
      listStrategyHealth(accountKey, undefined, limit),
      getStrategyHealthSummary(accountKey),
      getClosedTradeStats(accountKey),
    ]);

    // Build a merged row per strategy, keying off strategy_id
    const healthMap = new Map(health.map((h) => [h.strategy_id, h]));

    type IntelRow = {
      strategy_id: string;
      status: "HEALTHY" | "WARNING" | "CRITICAL" | "INSUFFICIENT_DATA";
      enabled: boolean;
      total_pnl: number;
      expectancy: number;
      profit_factor: number;
      win_rate: number;
      max_drawdown: number;
      sample_size: number;
      avg_win: number;
      avg_loss: number;
      health_reasons: string[];
      evidence_score: number;
      allocation_tier: "A" | "B" | "C" | "D" | "F";
      last_computed: string;
    };

    const rows: IntelRow[] = scores.map((s) => {
      const h = healthMap.get(s.strategy_id);
      const status = h?.health_status ?? "INSUFFICIENT_DATA";
      const enabled = status === "HEALTHY" || status === "WARNING";

      // Allocation tier by profit factor
      let tier: IntelRow["allocation_tier"] = "F";
      if (s.profit_factor >= 1.5) tier = "A";
      else if (s.profit_factor >= 1.25) tier = "B";
      else if (s.profit_factor >= 1.1) tier = "C";
      else if (s.profit_factor >= 1.0) tier = "D";

      // Evidence score 0–100: weighted composite
      const pfScore = Math.min(s.profit_factor / 2.0, 1) * 30;
      const wrScore = Math.min((s.win_rate - 0.4) / 0.3, 1) * 20;
      const expScore = Math.min(s.expectancy / 30.0, 1) * 25;
      const sampleScore = Math.min(s.sample_size / 200, 1) * 25;
      const evidenceScore = Math.max(0, Math.round(pfScore + wrScore + expScore + sampleScore));

      return {
        strategy_id: s.strategy_id,
        status,
        enabled,
        total_pnl: s.total_pnl,
        expectancy: s.expectancy,
        profit_factor: s.profit_factor,
        win_rate: s.win_rate,
        max_drawdown: s.max_drawdown,
        sample_size: s.sample_size,
        avg_win: s.avg_win,
        avg_loss: s.avg_loss,
        health_reasons: h?.health_reasons ?? [],
        evidence_score: evidenceScore,
        allocation_tier: tier,
        last_computed: h?.computed_at ?? s.computed_at,
      };
    });

    // Apply view filter
    let filtered: IntelRow[];
    switch (view) {
      case "top20":
        filtered = [...rows].sort((a, b) => b.expectancy - a.expectancy).slice(0, 20);
        break;
      case "top50":
        filtered = [...rows].sort((a, b) => b.expectancy - a.expectancy).slice(0, 50);
        break;
      case "bottom20":
        filtered = [...rows].sort((a, b) => a.expectancy - b.expectancy).slice(0, 20);
        break;
      case "retirement":
        filtered = rows.filter(
          (r) => r.status === "CRITICAL" || r.allocation_tier === "F" || r.expectancy < 0,
        );
        break;
      default:
        filtered = rows;
    }

    return NextResponse.json({
      ok: true,
      account_key: accountKey,
      view,
      total_strategies: rows.length,
      summary,
      portfolio_stats: {
        total_realized_pnl: closedStats.realized_pnl,
        total_trades: closedStats.total_trades,
        win_rate: closedStats.win_rate,
        profit_factor: closedStats.profit_factor,
      },
      strategies: filtered,
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
