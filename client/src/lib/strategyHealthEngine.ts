import type { MockTrade } from "@/lib/mockTradingEngine";
import type { StrategyScore } from "@/lib/strategyScoringEngine";

export type StrategyHealthState = "ACTIVE" | "WATCHLIST" | "DISABLED";

export interface StrategyHealthPolicy {
  minSampleSize: number;
  disableProfitFactorBelow: number;
  disableConsecutiveLosses: number;
  watchlistSampleSize: number;
}

export interface StrategyHealthRow {
  strategyId: number;
  strategyName: string;
  state: StrategyHealthState;
  trustScore: number;
  reasons: string[];
  closedTrades: number;
  expectancy: number;
  profitFactor: number;
  consecutiveLosses: number;
}

export const DEFAULT_STRATEGY_HEALTH_POLICY: StrategyHealthPolicy = {
  minSampleSize: 20,
  disableProfitFactorBelow: 0.9,
  disableConsecutiveLosses: 5,
  watchlistSampleSize: 10,
};

function clamp(value: number, min = 0, max = 100): number {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}

function consecutiveLosses(trades: readonly MockTrade[], strategyId: number): number {
  const closed = trades
    .filter((trade) => trade.strategyId === strategyId && trade.status === "CLOSED" && trade.closedAt != null)
    .sort((a, b) => (b.closedAt ?? 0) - (a.closedAt ?? 0));

  let losses = 0;
  for (const trade of closed) {
    if (trade.realizedPnl < 0) losses++;
    else break;
  }
  return losses;
}

export function computeStrategyHealth(
  scores: readonly StrategyScore[],
  trades: readonly MockTrade[],
  policy: StrategyHealthPolicy = DEFAULT_STRATEGY_HEALTH_POLICY,
): StrategyHealthRow[] {
  return scores.map((score) => {
    const { metrics } = score;
    const losses = consecutiveLosses(trades, score.strategyId);
    const reasons: string[] = [];
    let state: StrategyHealthState = "ACTIVE";

    if (metrics.closedTrades < policy.watchlistSampleSize) {
      state = "WATCHLIST";
      reasons.push(`Collecting sample (${metrics.closedTrades}/${policy.minSampleSize})`);
    } else if (metrics.closedTrades < policy.minSampleSize) {
      state = "WATCHLIST";
      reasons.push(`Below minimum sample threshold (${metrics.closedTrades}/${policy.minSampleSize})`);
    }

    if (metrics.closedTrades >= policy.minSampleSize && metrics.expectancy < 0) {
      state = "DISABLED";
      reasons.push(`Negative expectancy (${metrics.expectancy.toFixed(2)})`);
    }
    if (metrics.closedTrades >= policy.minSampleSize && metrics.profitFactor < policy.disableProfitFactorBelow) {
      state = "DISABLED";
      reasons.push(`Profit factor below ${policy.disableProfitFactorBelow.toFixed(2)} (${metrics.profitFactor.toFixed(2)})`);
    }
    if (losses >= policy.disableConsecutiveLosses) {
      state = "DISABLED";
      reasons.push(`${losses} consecutive losing trades`);
    }

    if (state === "ACTIVE" && metrics.maxDrawdownPct > 20) {
      state = "WATCHLIST";
      reasons.push(`Elevated drawdown (${metrics.maxDrawdownPct.toFixed(1)}%)`);
    }

    const trustScore = clamp(
      metrics.sampleSizeConfidence * 0.45 +
        score.overallScore * 0.25 +
        score.currentRegimeScore * 0.15 +
        clamp(metrics.profitFactor, 0, 3) * (100 / 3) * 0.15,
    );

    return {
      strategyId: score.strategyId,
      strategyName: score.strategyName,
      state,
      trustScore,
      reasons: reasons.length > 0 ? reasons : ["Healthy sample and positive metrics"],
      closedTrades: metrics.closedTrades,
      expectancy: metrics.expectancy,
      profitFactor: metrics.profitFactor,
      consecutiveLosses: losses,
    };
  });
}

export function activeStrategyIds(healthRows: readonly StrategyHealthRow[]): ReadonlySet<number> {
  return new Set(healthRows.filter((row) => row.state === "ACTIVE").map((row) => row.strategyId));
}
