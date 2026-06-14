/**
 * Strategy health engine — classifies each strategy as ACTIVE, WATCHLIST, or
 * DISABLED based on live trade performance.
 *
 * Pure computation, no I/O or React.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import type { StrategyScore } from "@/lib/ai/strategyScoringEngine";

export type HealthState = "ACTIVE" | "WATCHLIST" | "DISABLED";

export interface StrategyHealthRow {
  strategyId: number;
  strategyName: string;
  state: HealthState;
  trustScore: number;
  reasons: string[];
  closedTrades: number;
  expectancy: number;
  profitFactor: number;
  consecutiveLosses: number;
}

export interface HealthOptions {
  minSampleSize?: number;
  disableProfitFactorBelow?: number;
  disableConsecutiveLosses?: number;
  watchlistSampleSize?: number;
}

function countConsecutiveLosses(closed: MockTrade[]): number {
  const sorted = [...closed].sort((a, b) => (b.closedAt ?? 0) - (a.closedAt ?? 0));
  let count = 0;
  for (const t of sorted) {
    if (t.realizedPnl < 0) count++;
    else break;
  }
  return count;
}

export function computeStrategyHealth(
  scores: readonly StrategyScore[],
  trades: readonly MockTrade[],
  options?: HealthOptions,
): StrategyHealthRow[] {
  const minSample = options?.minSampleSize ?? 15;
  const disablePfBelow = options?.disableProfitFactorBelow ?? 0.8;
  const disableConsecLosses = options?.disableConsecutiveLosses ?? 5;
  const watchlistSample = options?.watchlistSampleSize ?? 10;

  const byStrategy = new Map<number, MockTrade[]>();
  for (const t of trades) {
    const arr = byStrategy.get(t.strategyId) ?? [];
    arr.push(t);
    byStrategy.set(t.strategyId, arr);
  }

  return scores.map((score): StrategyHealthRow => {
    const stTrades = byStrategy.get(score.strategyId) ?? [];
    const closed = stTrades.filter((t) => t.status === "CLOSED");
    const closedCount = closed.length;

    const netPnl = closed.reduce((s, t) => s + t.realizedPnl, 0);
    const expectancy = closedCount > 0 ? netPnl / closedCount : 0;

    const wins = closed.filter((t) => t.realizedPnl > 0);
    const losses = closed.filter((t) => t.realizedPnl <= 0);
    const grossWin = wins.reduce((s, t) => s + t.realizedPnl, 0);
    const grossLoss = Math.abs(losses.reduce((s, t) => s + t.realizedPnl, 0));
    const profitFactor = grossLoss > 0 ? grossWin / grossLoss : grossWin > 0 ? 99 : 0;

    const consecutiveLosses = countConsecutiveLosses(closed);

    const reasons: string[] = [];
    let state: HealthState = "ACTIVE";

    if (closedCount < watchlistSample) {
      state = "WATCHLIST";
      reasons.push(`Collecting data (${closedCount}/${watchlistSample} trades)`);
    } else if (closedCount < minSample) {
      state = "WATCHLIST";
      reasons.push(`Below min sample (${closedCount}/${minSample})`);
    } else if (profitFactor < disablePfBelow) {
      state = "DISABLED";
      reasons.push(`Low profit factor: ${profitFactor.toFixed(2)} < ${disablePfBelow}`);
    } else if (consecutiveLosses >= disableConsecLosses) {
      state = "DISABLED";
      reasons.push(`${consecutiveLosses} consecutive losses (limit ${disableConsecLosses})`);
    }

    const baseTrust = score.overallScore ?? 50;
    const pfBoost = profitFactor > 0 ? Math.min(1, profitFactor / 2) : 0;
    const trustScore =
      state === "ACTIVE"
        ? Math.min(100, baseTrust * pfBoost)
        : state === "WATCHLIST"
          ? 50
          : 10;

    return {
      strategyId: score.strategyId,
      strategyName: score.strategyName,
      state,
      trustScore,
      reasons,
      closedTrades: closedCount,
      expectancy,
      profitFactor,
      consecutiveLosses,
    };
  });
}
