/**
 * Institutional Research Engine
 *
 * Walk-forward validation, regime-specific analysis, strategy health
 * classification, and multi-metric ranking for the 10 institutional BTC
 * futures strategy modules.
 *
 * All computation is pure (no React, no I/O).  The engine consumes MockTrade[]
 * from the mock trading engine and produces structured research reports.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import { REGIME_LABELS as ALL_REGIME_LABELS, type MarketRegime } from "@/lib/ai/marketRegimeClassifier";
import {
  computeStrategyPerformance,
  type StrategyPerformanceMetrics,
} from "@/lib/ai/strategyPerformanceEngine";
import { computeStrategyHealth, type StrategyHealthRow } from "@/lib/ai/strategyHealthEngine";
import { scoreAllStrategies, type StrategyScore } from "@/lib/ai/strategyScoringEngine";
import { computeMockWalkForwardRows, type MockWalkForwardRow } from "@/lib/ai/mockResearchWalkForward";
import {
  INSTITUTIONAL_STRATEGIES,
  INSTITUTIONAL_STRATEGY_IDS,
  type InstitutionalStrategy,
} from "@/lib/trading/btcInstitutionalStrategies";

// ── Constants ──────────────────────────────────────────────────────────────────

/** Minimum closed trades before a strategy can leave WATCHLIST. */
const MIN_SAMPLE_SIZE = 20;

/** Minimum trade count to compute walk-forward windows. */
const MIN_WF_TRADES = 10;

/** Strategies are considered viable if profit factor ≥ this threshold. */
const MIN_VIABLE_PROFIT_FACTOR = 0.90;

// ── Per-strategy extended metrics ─────────────────────────────────────────────

export interface InstitutionalStrategyMetrics extends StrategyPerformanceMetrics {
  strategyTier: 1 | 2 | 3;
  staticConfidenceScore: number;
  staticRiskScore: number;
  staticEstimatedWinRate: number;
  staticNetExpectancy: number;
  liveNetExpectancy: number;
  /** Expectancy computed from live closed trades (may differ from static estimate). */
  expectedProfitAfterFees: number;
  expectedLossAfterFees: number;
  healthState: "ACTIVE" | "WATCHLIST" | "DISABLED";
  walkForwardScore: number;
  walkForwardStatus: "PASS" | "FAIL" | "COLLECT_DATA";
  /** Ranked position among all institutional strategies (1 = best). */
  overallRank: number;
}

export interface RegimeLeaderboardRow {
  regime: MarketRegime;
  strategyId: number;
  strategyName: string;
  regimeWinRate: number;
  regimeExpectancy: number;
  regimeTrades: number;
  regimeNetPnl: number;
}

export interface InstitutionalResearchReport {
  generatedAt: number;
  totalTrades: number;
  totalClosedTrades: number;
  strategies: InstitutionalStrategyMetrics[];
  /** Ranked by overall composite score. */
  rankByScore: InstitutionalStrategyMetrics[];
  /** Ranked by net PnL. */
  rankByPnl: InstitutionalStrategyMetrics[];
  /** Ranked by profit factor. */
  rankByProfitFactor: InstitutionalStrategyMetrics[];
  /** Ranked by Sharpe ratio. */
  rankBySharpe: InstitutionalStrategyMetrics[];
  /** Ranked by live expectancy. */
  rankByExpectancy: InstitutionalStrategyMetrics[];
  /** Ranked by win rate. */
  rankByWinRate: InstitutionalStrategyMetrics[];
  /** Ranked by lowest drawdown (ascending drawdown = best). */
  rankByLowestDrawdown: InstitutionalStrategyMetrics[];
  /** Best strategy per regime. */
  regimeLeaderboard: RegimeLeaderboardRow[];
  /** Walk-forward validation rows per strategy. */
  walkForward: MockWalkForwardRow[];
  /** Health state rows. */
  health: StrategyHealthRow[];
  /** ACTIVE strategies. */
  activeStrategies: InstitutionalStrategyMetrics[];
  /** WATCHLIST strategies (collecting sample). */
  watchlistStrategies: InstitutionalStrategyMetrics[];
  /** DISABLED strategies (below performance threshold). */
  disabledStrategies: InstitutionalStrategyMetrics[];
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function institutionalTrades(trades: readonly MockTrade[]): MockTrade[] {
  const idSet = new Set(INSTITUTIONAL_STRATEGY_IDS);
  return trades.filter((t) => idSet.has(t.strategyId));
}

function tradesByStrategy(trades: readonly MockTrade[]): Map<number, MockTrade[]> {
  const map = new Map<number, MockTrade[]>();
  for (const t of trades) {
    const bucket = map.get(t.strategyId) ?? [];
    bucket.push(t);
    map.set(t.strategyId, bucket);
  }
  return map;
}

function safeExpectancy(trades: readonly MockTrade[]): number {
  const closed = trades.filter((t) => t.status === "CLOSED");
  if (closed.length === 0) return 0;
  return closed.reduce((s, t) => s + t.realizedPnl, 0) / closed.length;
}

function safeAvgWin(trades: readonly MockTrade[]): number {
  const wins = trades.filter((t) => t.status === "CLOSED" && t.realizedPnl > 0);
  if (wins.length === 0) return 0;
  return wins.reduce((s, t) => s + t.realizedPnl, 0) / wins.length;
}

function safeAvgLoss(trades: readonly MockTrade[]): number {
  const losses = trades.filter((t) => t.status === "CLOSED" && t.realizedPnl < 0);
  if (losses.length === 0) return 0;
  return losses.reduce((s, t) => s + t.realizedPnl, 0) / losses.length;
}

// Use the legacy regime keys that strategyPerformanceEngine.ts still populates
const ALL_REGIMES: MarketRegime[] = [
  "TRENDING",
  "RANGING",
  "HIGH_VOLATILITY_BREAKOUT",
  "LOW_VOLATILITY_CHOP",
];

// ── Core report builder ────────────────────────────────────────────────────────

/**
 * Build a complete institutional research report from the current mock trade history.
 *
 * @param allTrades    All MockTrade[] from the mock trading engine (open + closed).
 * @param currentRegime  Current market regime — used for regime-weighted ranking.
 * @param now          Current timestamp in epoch ms (injectable for testing).
 */
export function buildInstitutionalReport(
  allTrades: readonly MockTrade[],
  currentRegime: MarketRegime = "RANGING",
  now = Date.now(),
): InstitutionalResearchReport {
  const instTrades = institutionalTrades(allTrades);
  const byStrategy = tradesByStrategy(instTrades);

  // 1. Compute StrategyPerformanceMetrics for each strategy
  const perfMap = new Map<number, StrategyPerformanceMetrics>();
  for (const strat of INSTITUTIONAL_STRATEGIES) {
    const stratTrades = byStrategy.get(strat.id) ?? [];
    perfMap.set(strat.id, computeStrategyPerformance(stratTrades, now));
  }

  // 2. Score & rank
  const scores = scoreAllStrategies(perfMap, currentRegime);
  const scoreByStratId = new Map<number, StrategyScore>(scores.map((s) => [s.strategyId, s]));

  // 3. Walk-forward
  const wfRows = computeMockWalkForwardRows(instTrades, {
    trainDays: 14,
    validationDays: 7,
    minTrainTrades: MIN_WF_TRADES,
    minValidationTrades: 3,
  });
  const wfByStratId = new Map<number, MockWalkForwardRow>(wfRows.map((r) => [r.strategyId, r]));

  // 4. Health
  const healthRows = computeStrategyHealth(scores, instTrades, {
    minSampleSize: MIN_SAMPLE_SIZE,
    disableProfitFactorBelow: MIN_VIABLE_PROFIT_FACTOR,
    disableConsecutiveLosses: 5,
    watchlistSampleSize: 10,
  });
  const healthByStratId = new Map<number, StrategyHealthRow>(
    healthRows.map((r) => [r.strategyId, r]),
  );

  // 5. Build extended metrics
  const strategies: InstitutionalStrategyMetrics[] = INSTITUTIONAL_STRATEGIES.map((strat) => {
    const perf = perfMap.get(strat.id)!;
    const score = scoreByStratId.get(strat.id);
    const wf = wfByStratId.get(strat.id);
    const health = healthByStratId.get(strat.id);

    return {
      ...perf,
      strategyId: strat.id,
      strategyName: strat.name,
      strategyTier: strat.meta.tier,
      staticConfidenceScore: strat.meta.confidenceScore,
      staticRiskScore: strat.meta.riskScore,
      staticEstimatedWinRate: strat.meta.estimatedWinRate,
      staticNetExpectancy: strat.meta.netExpectancyEstimate,
      liveNetExpectancy: safeExpectancy(byStrategy.get(strat.id) ?? []),
      expectedProfitAfterFees: strat.meta.expectedProfitAfterFees,
      expectedLossAfterFees: strat.meta.expectedLossAfterFees,
      healthState: health?.state ?? "WATCHLIST",
      walkForwardScore: wf?.walkForwardScore ?? 0,
      walkForwardStatus: wf?.status ?? "COLLECT_DATA",
      overallRank: score?.rank ?? 999,
    };
  });

  // 6. Regime leaderboard — best strategy per regime by regimeExpectancy
  const regimeLeaderboard: RegimeLeaderboardRow[] = ALL_REGIMES.map((regime) => {
    let bestId = INSTITUTIONAL_STRATEGIES[0]?.id ?? 2100;
    let bestExpectancy = -Infinity;
    let bestRow: RegimeLeaderboardRow = {
      regime,
      strategyId: bestId,
      strategyName: "",
      regimeWinRate: 0,
      regimeExpectancy: 0,
      regimeTrades: 0,
      regimeNetPnl: 0,
    };

    for (const m of strategies) {
      const stats = m.regimeBreakdown[regime];
      if (!stats || stats.trades === 0) continue;
      if (stats.expectancy > bestExpectancy) {
        bestExpectancy = stats.expectancy;
        bestId = m.strategyId;
        bestRow = {
          regime,
          strategyId: m.strategyId,
          strategyName: m.strategyName,
          regimeWinRate: stats.winRate,
          regimeExpectancy: stats.expectancy,
          regimeTrades: stats.trades,
          regimeNetPnl: stats.netPnl,
        };
      }
    }
    return bestRow;
  });

  // 7. Ranked views
  const sorted = <K extends keyof InstitutionalStrategyMetrics>(
    key: K,
    ascending = false,
  ): InstitutionalStrategyMetrics[] =>
    [...strategies].sort((a, b) => {
      const va = a[key] as number;
      const vb = b[key] as number;
      return ascending ? va - vb : vb - va;
    });

  const active = strategies.filter((s) => s.healthState === "ACTIVE");
  const watchlist = strategies.filter((s) => s.healthState === "WATCHLIST");
  const disabled = strategies.filter((s) => s.healthState === "DISABLED");

  const totalTrades = instTrades.length;
  const totalClosedTrades = instTrades.filter((t) => t.status === "CLOSED").length;

  return {
    generatedAt: now,
    totalTrades,
    totalClosedTrades,
    strategies,
    rankByScore: sorted("overallRank", true),
    rankByPnl: sorted("netPnl"),
    rankByProfitFactor: sorted("profitFactor"),
    rankBySharpe: sorted("sharpeRatio"),
    rankByExpectancy: sorted("liveNetExpectancy"),
    rankByWinRate: sorted("winRate"),
    rankByLowestDrawdown: sorted("maxDrawdownPct", true), // ascending = lower drawdown is better
    regimeLeaderboard,
    walkForward: wfRows,
    health: healthRows,
    activeStrategies: active,
    watchlistStrategies: watchlist,
    disabledStrategies: disabled,
  };
}

// ── Walk-forward summary helper ────────────────────────────────────────────────

export interface WalkForwardSummary {
  strategyId: number;
  strategyName: string;
  tier: 1 | 2 | 3;
  status: "PASS" | "FAIL" | "COLLECT_DATA";
  walkForwardScore: number;
  outOfSampleExpectancy: number;
  outOfSampleTrades: number;
  inSampleExpectancy: number;
  windows: number;
  healthState: "ACTIVE" | "WATCHLIST" | "DISABLED";
}

export function buildWalkForwardSummary(
  report: InstitutionalResearchReport,
): WalkForwardSummary[] {
  return INSTITUTIONAL_STRATEGIES.map((strat) => {
    const wf = report.walkForward.find((row) => row.strategyId === strat.id);
    const m = report.strategies.find((row) => row.strategyId === strat.id);
    return {
      strategyId: strat.id,
      strategyName: strat.name,
      tier: strat.meta.tier,
      status: wf?.status ?? "COLLECT_DATA",
      walkForwardScore: wf?.walkForwardScore ?? 0,
      outOfSampleExpectancy: wf?.outOfSampleExpectancy ?? 0,
      outOfSampleTrades: wf?.outOfSampleTrades ?? 0,
      inSampleExpectancy: wf?.inSampleExpectancy ?? 0,
      windows: wf?.windows ?? 0,
      healthState: m?.healthState ?? "WATCHLIST",
    };
  });
}

// ── Regime label helper ────────────────────────────────────────────────────────

export const REGIME_LABELS: Record<MarketRegime, string> = ALL_REGIME_LABELS;

// ── Static strategy metadata table ────────────────────────────────────────────

export interface StaticStrategyRow {
  id: number;
  name: string;
  tier: 1 | 2 | 3;
  family: string;
  side: "LONG" | "SHORT";
  tpPct: number;
  slPct: number;
  rr: number;
  confidenceScore: number;
  riskScore: number;
  estimatedWinRate: number;
  staticNetExpectancy: number;
  bestRegimes: string;
  worstRegimes: string;
}

export function getStaticStrategyTable(): StaticStrategyRow[] {
  return INSTITUTIONAL_STRATEGIES.map((s) => ({
    id: s.id,
    name: s.name,
    tier: s.meta.tier,
    family: s.family,
    side: s.side as "LONG" | "SHORT",
    tpPct: s.meta.tpPct,
    slPct: s.meta.slPct,
    rr: s.meta.slPct > 0 ? +(s.meta.tpPct / s.meta.slPct).toFixed(2) : 0,
    confidenceScore: s.meta.confidenceScore,
    riskScore: s.meta.riskScore,
    estimatedWinRate: s.meta.estimatedWinRate,
    staticNetExpectancy: s.meta.netExpectancyEstimate,
    bestRegimes: s.bestRegime.join(", "),
    worstRegimes: s.worstRegime.join(", "),
  }));
}
