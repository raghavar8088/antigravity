/**
 * Mock strategy ranking engine — produces a leaderboard of strategies from
 * mock trade history. Used by the MockStageTradingSuite terminal panel.
 *
 * Pure computation, no I/O or React.
 */

export type RankClassification = "ACTIVE" | "WATCHLIST" | "DISABLED";

/** Minimal trade shape needed to compute a leaderboard — MockTrade satisfies this structurally. */
export interface RankableTrade {
  strategyId: number;
  strategyName: string;
  status: "OPEN" | "CLOSED";
  realizedPnl: number;
  closedAt: number | null;
}

export interface StrategyRankRow {
  rank: number;
  strategyId: number;
  strategyName: string;
  strategyFamily?: string;
  score: number;
  winRate: number;
  totalTrades: number;
  closedTrades: number;
  profitFactor: number | null;
  netPnl: number;
  classification: RankClassification;
}

export interface RankStrategiesResult {
  rows: StrategyRankRow[];
  rankedAt: number;
}

function deriveFamily(name: string): string {
  const lower = name.toLowerCase();
  if (lower.includes("ema")) return "EMA";
  if (lower.includes("rsi")) return "RSI";
  if (lower.includes("bb") || lower.includes("bollinger")) return "Bollinger";
  if (lower.includes("vwap")) return "VWAP";
  if (lower.includes("cvd") || lower.includes("delta")) return "CVD";
  if (lower.includes("liquidity") || lower.includes("sweep")) return "Liquidity";
  if (lower.includes("funding")) return "Funding";
  if (lower.includes("adx") || lower.includes("momentum")) return "Momentum";
  if (lower.includes("orb") || lower.includes("opening")) return "ORB";
  if (lower.includes("mss") || lower.includes("microstructure")) return "Microstructure";
  return "Research";
}

function classifyFromMetrics(
  closedTrades: number,
  profitFactor: number | null,
  consecutiveLosses: number,
): RankClassification {
  if (closedTrades < 10) return "WATCHLIST";
  if (profitFactor != null && profitFactor < 0.8) return "DISABLED";
  if (consecutiveLosses >= 5) return "DISABLED";
  return "ACTIVE";
}

export function rankStrategies(args: {
  trades: readonly RankableTrade[];
  topN?: number;
}): RankStrategiesResult {
  const byStrategy = new Map<number, RankableTrade[]>();

  for (const t of args.trades) {
    const arr = byStrategy.get(t.strategyId) ?? [];
    arr.push(t);
    byStrategy.set(t.strategyId, arr);
  }

  const rows: (StrategyRankRow & { _score: number })[] = [];

  for (const [strategyId, stTrades] of byStrategy) {
    const closed = stTrades.filter((t) => t.status === "CLOSED");
    const closedCount = closed.length;
    const netPnl = closed.reduce((s, t) => s + t.realizedPnl, 0);

    const wins = closed.filter((t) => t.realizedPnl > 0);
    const losses = closed.filter((t) => t.realizedPnl <= 0);
    const winRate = closedCount > 0 ? wins.length / closedCount : 0;
    const grossWin = wins.reduce((s, t) => s + t.realizedPnl, 0);
    const grossLoss = Math.abs(losses.reduce((s, t) => s + t.realizedPnl, 0));
    const profitFactor = grossLoss > 0 ? grossWin / grossLoss : closedCount > 0 ? null : null;

    const sorted = [...closed].sort((a, b) => (b.closedAt ?? 0) - (a.closedAt ?? 0));
    let consecutiveLosses = 0;
    for (const t of sorted) {
      if (t.realizedPnl < 0) consecutiveLosses++;
      else break;
    }

    const strategyName = stTrades[0]?.strategyName ?? `Strategy ${strategyId}`;
    const classification = classifyFromMetrics(closedCount, profitFactor, consecutiveLosses);

    const pfScore = profitFactor != null ? Math.min(1, profitFactor / 3) * 40 : 0;
    const wrScore = winRate * 35;
    const sampleScore = Math.min(1, closedCount / 50) * 15;
    const pnlScore = netPnl > 0 ? Math.min(10, (netPnl / 500)) : 0;
    const _score = pfScore + wrScore + sampleScore + pnlScore;

    rows.push({
      rank: 0,
      strategyId,
      strategyName,
      strategyFamily: deriveFamily(strategyName),
      score: Math.min(100, _score),
      winRate,
      totalTrades: stTrades.length,
      closedTrades: closedCount,
      profitFactor,
      netPnl,
      classification,
      _score,
    });
  }

  rows.sort((a, b) => b._score - a._score);

  return {
    rows: rows.map((r, i) => {
      const { _score: _, ...row } = r;
      return { ...row, rank: i + 1 };
    }),
    rankedAt: Date.now(),
  };
}
