/**
 * Strategy scoring engine — ranks strategies by multi-metric composite score.
 *
 * Primary API: `scoreAllStrategies(perfMap, currentRegime)` — accepts a Map of
 * pre-computed StrategyPerformanceMetrics (as used by InstitutionalResearchEngine).
 *
 * Convenience API: `scoreStrategiesFromTrades(trades, currentRegime)` — builds
 * the metrics map inline from raw MockTrade history (used by useStrategyScoring).
 *
 * Pure computation, no I/O or React.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import type { StrategyPerformanceMetrics } from "@/lib/ai/strategyPerformanceEngine";
import { computeStrategyPerformance } from "@/lib/ai/strategyPerformanceEngine";

export type ConfidenceRating = "HIGH" | "MEDIUM" | "LOW" | "INSUFFICIENT";

export interface StrategyScore {
  strategyId: number;
  strategyName: string;
  metrics: Pick<
    StrategyPerformanceMetrics,
    | "totalTrades"
    | "closedTrades"
    | "netPnl"
    | "winRate"
    | "profitFactor"
    | "expectancy"
    | "sharpeRatio"
    | "maxDrawdownPct"
  >;
  pnlScore: number;
  profitFactorScore: number;
  winRateScore: number;
  drawdownScore: number;
  sharpeScore: number;
  recencyScore: number;
  sampleSizeScore: number;
  overallScore: number;
  currentRegimeScore: number;
  confidenceRating: ConfidenceRating;
  rank: number;
  regimeRank: number;
}

// ── Scoring helpers ───────────────────────────────────────────────────────────

function clamp(v: number, min = 0, max = 100): number {
  if (!Number.isFinite(v)) return min;
  return Math.min(max, Math.max(min, v));
}

function scorePnl(netPnl: number): number {
  if (netPnl >= 1000) return 100;
  if (netPnl >= 0) return clamp((netPnl / 1000) * 100);
  return clamp(50 + (netPnl / 200));
}

function scoreProfitFactor(pf: number): number {
  if (!Number.isFinite(pf) || pf <= 0) return 0;
  if (pf >= 3) return 100;
  return clamp(((pf - 0.5) / 2.5) * 100);
}

function scoreWinRate(wr: number): number {
  return clamp(wr * 120);
}

function scoreDrawdown(ddPct: number): number {
  if (ddPct <= 0) return 100;
  if (ddPct >= 30) return 0;
  return clamp(100 - (ddPct / 30) * 100);
}

function scoreSharpe(sharpe: number): number {
  if (sharpe >= 3) return 100;
  if (sharpe <= -1) return 0;
  return clamp(((sharpe + 1) / 4) * 100);
}

function confidenceFromSample(closedTrades: number): ConfidenceRating {
  if (closedTrades >= 50) return "HIGH";
  if (closedTrades >= 20) return "MEDIUM";
  if (closedTrades >= 10) return "LOW";
  return "INSUFFICIENT";
}

function metricsToScore(
  strategyId: number,
  perf: StrategyPerformanceMetrics,
  currentRegime?: string,
): Omit<StrategyScore, "rank" | "regimeRank"> {
  const pnlS = scorePnl(perf.netPnl);
  const pfS = scoreProfitFactor(perf.profitFactor);
  const wrS = scoreWinRate(perf.winRate);
  const ddS = scoreDrawdown(perf.maxDrawdownPct);
  const sharpeS = scoreSharpe(perf.sharpeRatio);
  const recencyS = clamp(perf.recencyScore ?? 50);
  const sampleS = clamp((perf.closedTrades / 50) * 100);

  const overall = clamp(
    pnlS * 0.25 + pfS * 0.25 + wrS * 0.20 + ddS * 0.10 + sharpeS * 0.10 + recencyS * 0.05 + sampleS * 0.05,
  );

  // Regime-weighted score: look up the current-regime performance if available
  let currentRegimeScore = overall;
  if (currentRegime && perf.regimeBreakdown) {
    const regimeStats = (perf.regimeBreakdown as Record<string, { trades: number; winRate: number; expectancy: number; netPnl: number } | undefined>)[currentRegime];
    if (regimeStats && regimeStats.trades >= 3) {
      const rScore = clamp(
        scoreWinRate(regimeStats.winRate) * 0.5 + scorePnl(regimeStats.netPnl) * 0.5,
      );
      currentRegimeScore = clamp(overall * 0.6 + rScore * 0.4);
    }
  }

  return {
    strategyId,
    strategyName: perf.strategyName,
    metrics: {
      totalTrades: perf.totalTrades,
      closedTrades: perf.closedTrades,
      netPnl: perf.netPnl,
      winRate: perf.winRate,
      profitFactor: perf.profitFactor,
      expectancy: perf.expectancy,
      sharpeRatio: perf.sharpeRatio,
      maxDrawdownPct: perf.maxDrawdownPct,
    },
    pnlScore: pnlS,
    profitFactorScore: pfS,
    winRateScore: wrS,
    drawdownScore: ddS,
    sharpeScore: sharpeS,
    recencyScore: recencyS,
    sampleSizeScore: sampleS,
    overallScore: overall,
    currentRegimeScore,
    confidenceRating: confidenceFromSample(perf.closedTrades),
  };
}

// ── Primary API (used by InstitutionalResearchEngine) ─────────────────────────

/**
 * Score and rank strategies from a pre-computed performance metrics map.
 * Strategies with no closed trades are omitted.
 */
export function scoreAllStrategies(
  perfMap: Map<number, StrategyPerformanceMetrics>,
  currentRegime?: string,
): StrategyScore[] {
  const scored: (Omit<StrategyScore, "rank" | "regimeRank"> & { _overall: number })[] = [];

  for (const [strategyId, perf] of perfMap) {
    if (perf.closedTrades === 0) continue;
    const s = metricsToScore(strategyId, perf, currentRegime);
    scored.push({ ...s, _overall: s.overallScore });
  }

  scored.sort((a, b) => b.currentRegimeScore - a.currentRegimeScore);

  return scored.map((s, i) => {
    const { _overall: _, ...rest } = s;
    return { ...rest, rank: i + 1, regimeRank: i + 1 };
  });
}

// ── Convenience API (used by useStrategyScoring hook) ─────────────────────────

/**
 * Score and rank strategies directly from raw MockTrade history, building the
 * performance metrics map inline. Strategies with no closed trades are omitted.
 */
export function scoreStrategiesFromTrades(
  trades: readonly MockTrade[],
  currentRegime?: string,
  now = Date.now(),
): StrategyScore[] {
  const byStrategy = new Map<number, MockTrade[]>();
  for (const t of trades) {
    const arr = byStrategy.get(t.strategyId) ?? [];
    arr.push(t);
    byStrategy.set(t.strategyId, arr);
  }

  const perfMap = new Map<number, StrategyPerformanceMetrics>();
  for (const [id, stTrades] of byStrategy) {
    perfMap.set(id, computeStrategyPerformance(stTrades, now));
  }

  return scoreAllStrategies(perfMap, currentRegime);
}
