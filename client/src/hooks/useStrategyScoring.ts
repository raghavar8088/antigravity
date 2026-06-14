"use client";

import { useMemo, useRef } from "react";
import { scoreStrategiesFromTrades, type StrategyScore } from "@/lib/ai/strategyScoringEngine";
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
  /** Strategy IDs approved based on overall profit score (PROFIT_MODE). */
  approvedProfitIds: Set<number>;
  /** Strategy IDs approved based on current-regime score (REGIME_MODE). */
  approvedRegimeIds: Set<number>;
  rankedAt: number;
  lastScoredAt: number;
}

const APPROVAL_OVERALL_THRESHOLD = 40;
const APPROVAL_REGIME_THRESHOLD = 35;

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

  const lastScoredAtRef = useRef<number>(Date.now());

  const scores = useMemo<StrategyScore[]>(() => {
    lastScoredAtRef.current = Date.now();
    return scoreStrategiesFromTrades(trades, currentRegime);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [closedCount, currentRegime]);

  const topN = useMemo(() => scores.slice(0, topNCount), [scores, topNCount]);

  const approvedProfitIds = useMemo<Set<number>>(
    () => new Set(scores.filter((s) => s.overallScore >= APPROVAL_OVERALL_THRESHOLD).map((s) => s.strategyId)),
    [scores],
  );

  const approvedRegimeIds = useMemo<Set<number>>(
    () => new Set(scores.filter((s) => s.currentRegimeScore >= APPROVAL_REGIME_THRESHOLD).map((s) => s.strategyId)),
    [scores],
  );

  return {
    scores,
    topN,
    approvedProfitIds,
    approvedRegimeIds,
    rankedAt: lastScoredAtRef.current,
    lastScoredAt: lastScoredAtRef.current,
  };
}
