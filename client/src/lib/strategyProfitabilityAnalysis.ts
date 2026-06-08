/**
 * Strategy-level profitability analysis from MongoDB paper_trades.
 */

import { getDb } from "@/lib/mongoTradesClient";
import { computeExtendedMetricsFromRecords } from "@/lib/mockExtendedMetrics";
import { PAPER_DESK_STARTING_BALANCE } from "@/lib/portfolioAccountingTypes";

export type StrategyProfitabilityRow = {
  strategy_id: string;
  trade_count: number;
  win_rate: number;
  gross_pnl: number;
  net_pnl: number;
  fees: number;
  profit_factor: number | null;
  sharpe: number | null;
  expectancy: number;
  max_drawdown_pct: number;
  exposure_btc: number;
  capital_efficiency: number;
  rank: number;
  recommendation: "scale" | "hold" | "disable" | "insufficient_data";
};

export type StrategyProfitabilityReport = {
  computed_at: string;
  total_strategies: number;
  top_20: StrategyProfitabilityRow[];
  bottom_20: StrategyProfitabilityRow[];
  to_disable: string[];
  to_scale: string[];
};

const MIN_SAMPLE = 20;

function recommendationFor(row: StrategyProfitabilityRow): StrategyProfitabilityRow["recommendation"] {
  if (row.trade_count < MIN_SAMPLE) return "insufficient_data";
  if (row.net_pnl < 0 && (row.profit_factor ?? 0) < 0.9) return "disable";
  if (row.net_pnl > 0 && (row.profit_factor ?? 0) >= 1.2 && row.expectancy > 0) return "scale";
  return "hold";
}

export async function analyzeStrategyProfitability(
  accountKey: string,
): Promise<StrategyProfitabilityReport> {
  const db = await getDb();
  const trades = await db
    .collection("paper_trades")
    .find({ account_key: accountKey })
    .project({
      strategy_id: 1,
      gross_pnl: 1,
      net_pnl: 1,
      fees: 1,
      quantity: 1,
      entry_at: 1,
      closed_at: 1,
    })
    .toArray();

  const byStrategy = new Map<string, typeof trades>();
  for (const t of trades) {
    const sid = String(t.strategy_id ?? "unknown");
    const bucket = byStrategy.get(sid) ?? [];
    bucket.push(t);
    byStrategy.set(sid, bucket);
  }

  const rows: StrategyProfitabilityRow[] = [];
  for (const [strategy_id, bucket] of byStrategy) {
    const records = bucket.map((t) => ({
      realizedPnl: Number(t.net_pnl) || 0,
      openedAt: new Date(String(t.entry_at ?? t.closed_at)).getTime(),
      closedAt: new Date(String(t.closed_at)).getTime(),
    }));
    const ext = computeExtendedMetricsFromRecords(records, PAPER_DESK_STARTING_BALANCE);
    const gross = bucket.reduce((s, t) => s + (Number(t.gross_pnl) || 0), 0);
    const net = bucket.reduce((s, t) => s + (Number(t.net_pnl) || 0), 0);
    const fees = bucket.reduce((s, t) => s + (Number(t.fees) || 0), 0);
    const exposure = bucket.reduce((s, t) => s + Math.abs(Number(t.quantity) || 0), 0);

    const row: StrategyProfitabilityRow = {
      strategy_id,
      trade_count: bucket.length,
      win_rate: ext.winRate,
      gross_pnl: gross,
      net_pnl: net,
      fees,
      profit_factor: ext.profitFactor,
      sharpe: ext.sharpeRatio,
      expectancy: ext.expectancy,
      max_drawdown_pct: ext.maxDrawdownPct,
      exposure_btc: exposure / Math.max(1, bucket.length),
      capital_efficiency: net / Math.max(1, exposure),
      rank: 0,
      recommendation: "hold",
    };
    row.recommendation = recommendationFor(row);
    rows.push(row);
  }

  rows.sort((a, b) => b.net_pnl - a.net_pnl);
  rows.forEach((r, i) => { r.rank = i + 1; });

  const top20 = rows.slice(0, 20);
  const bottom20 = [...rows].sort((a, b) => a.net_pnl - b.net_pnl).slice(0, 20);

  return {
    computed_at: new Date().toISOString(),
    total_strategies: rows.length,
    top_20: top20,
    bottom_20: bottom20,
    to_disable: rows.filter((r) => r.recommendation === "disable").map((r) => r.strategy_id),
    to_scale: rows.filter((r) => r.recommendation === "scale").map((r) => r.strategy_id),
  };
}
