"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { BTC_RESEARCH_STRATEGIES } from "@/lib/btcResearchStrategyRegistry";
import type { MarketRegime } from "@/lib/marketRegimeClassifier";
import type { MockTrade } from "@/lib/mockTradingEngine";
import { computeStrategyPerformance } from "@/lib/strategyPerformanceEngine";
import { scoreAllStrategies, type StrategyScore } from "@/lib/strategyScoringEngine";

const DEFAULT_REGIME: MarketRegime = "RANGING";

export function useStrategyScoring(deps: {
  trades: readonly MockTrade[];
  currentRegime: MarketRegime | null;
  newCandleReady: boolean;
  topNCount: number;
}): {
  scores: StrategyScore[];
  rankings: StrategyScore[];
  topN: (n: number) => StrategyScore[];
  bottomN: (n: number) => StrategyScore[];
  approvedIds: ReadonlySet<number>;
  approvedProfitIds: ReadonlySet<number>;
  approvedRegimeIds: ReadonlySet<number>;
  lastScoredAt: number | null;
} {
  const [scores, setScores] = useState<StrategyScore[]>([]);
  const [lastScoredAt, setLastScoredAt] = useState<number | null>(null);
  const ref = useRef(deps);
  ref.current = deps;

  useEffect(() => {
    if (!deps.newCandleReady && scores.length > 0) return;
    const id = setTimeout(() => {
      const { trades, currentRegime } = ref.current;
      const grouped = new Map<number, MockTrade[]>();
      for (const trade of trades) {
        const bucket = grouped.get(trade.strategyId) ?? [];
        bucket.push(trade);
        grouped.set(trade.strategyId, bucket);
      }

      const performances = new Map<number, ReturnType<typeof computeStrategyPerformance>>();
      for (const [strategyId, strategyTrades] of grouped) {
        performances.set(strategyId, computeStrategyPerformance(strategyTrades));
      }

      setScores(scoreAllStrategies(performances, currentRegime ?? DEFAULT_REGIME));
      setLastScoredAt(Date.now());
    }, 0);
    return () => clearTimeout(id);
    // Trade array identity churns on every price tick; length is enough to trigger
    // material rescoring as trades open/close while candle pulses refresh cadence.
  }, [deps.newCandleReady, deps.trades.length, deps.currentRegime, scores.length]);

  const topN = useCallback((n: number) => scores.slice(0, Math.max(0, n)), [scores]);
  const bottomN = useCallback(
    (n: number) =>
      [...scores]
        .filter((score) => score.metrics.sampleSizeConfidence >= 40)
        .sort((a, b) => a.overallScore - b.overallScore)
        .slice(0, Math.max(0, n)),
    [scores],
  );

  const approvedIds = useMemo(() => {
    const limit = Math.max(1, deps.topNCount);
    const rankedResearchIds = scores
      .filter((score) => score.strategyId >= 2000 && score.strategyId <= 2059)
      .slice(0, limit)
      .map((score) => score.strategyId);

    if (rankedResearchIds.length > 0) return new Set(rankedResearchIds);

    return new Set(
      BTC_RESEARCH_STRATEGIES.filter((strategy) => !strategy.dataFeedRequired)
        .slice(0, limit)
        .map((strategy) => strategy.id),
    );
  }, [deps.topNCount, scores]);

  const approvedRegimeIds = useMemo(() => {
    const limit = Math.max(1, deps.topNCount);
    const regime = deps.currentRegime ?? DEFAULT_REGIME;
    return new Set(
      [...scores]
        .filter((score) => score.strategyId >= 2000 && score.strategyId <= 2059)
        .filter((score) => {
          const stats = score.metrics.regimeBreakdown[regime];
          return stats.trades >= 3 && stats.expectancy > 0 && score.currentRegimeScore >= 50;
        })
        .sort((a, b) => a.regimeRank - b.regimeRank)
        .slice(0, limit)
        .map((score) => score.strategyId),
    );
  }, [deps.currentRegime, deps.topNCount, scores]);

  return {
    scores,
    rankings: scores,
    topN,
    bottomN,
    approvedIds,
    approvedProfitIds: approvedIds,
    approvedRegimeIds,
    lastScoredAt,
  };
}
