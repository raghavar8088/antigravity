"use client";

import { useMemo } from "react";
import { scoreAllStrategies, type StrategyScore } from "@/lib/ai/strategyScoringEngine";
import type { MockTrade } from "@/lib/trading/mockTradingEngine";

export interface UseStrategyScoringArgs {
  trades: readonly MockTrade[];
  currentRegime: string;
  newCandleReady: boolean;
  topNCount?: number;
}

export interface UseStrategyScoringResult {
  scores: StrategyScore[];
  topN: StrategyScore[];
  rankedAt: number;
}

/**
 * Scores and ranks mock-engine strategies from the live portfolio trade history.
 * Re-ranks whenever the closed trade count changes (cheapest signal for updates).
 */
export function useStrategyScoring({
  trades,
  currentRegime,
  topNCount = 10,
}: UseStrategyScoringArgs): UseStrategyScoringResult {
  const closedCount = useMemo(
    () => trades.filter((t) => t.status === "CLOSED").length,
    [trades],
  );

  const scores = useMemo<StrategyScore[]>(
    () => scoreAllStrategies(trades, currentRegime),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [closedCount, currentRegime],
  );

  const topN = useMemo(() => scores.slice(0, topNCount), [scores, topNCount]);

  return { scores, topN, rankedAt: Date.now() };
}
