/**
 * Portfolio Optimization Engine for the BTC futures mock trading module.
 *
 * Builds dynamic capital allocation weights across strategies using a
 * combination of:
 *   - Fractional Kelly criterion (sizing proportional to edge/variance)
 *   - Inverse volatility weighting (stability preference)
 *   - Sharpe-based tilt (higher Sharpe = higher weight)
 *   - Recent-performance recency bias (30-day window)
 *   - Per-strategy exposure caps
 *
 * The resulting weights tell the mock engine what fraction of the total
 * allocation budget to assign to each active strategy.
 *
 * Pure functions — no React, no I/O.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import { computeExtendedMetrics, computeRollingMetrics } from "@/lib/analytics/mockExtendedMetrics";
import type { StrategyRankRow } from "@/lib/ai/mockStrategyRankingEngine";

// ── Config ────────────────────────────────────────────────────────────────────

export interface PortfolioOptimizerConfig {
  /** Maximum allocation fraction for a single strategy (e.g. 0.3 = 30%). */
  maxAllocationPerStrategy: number;
  /** Minimum allocation fraction for an ACTIVE strategy (e.g. 0.02 = 2%). */
  minAllocationPerStrategy: number;
  /** Kelly fraction multiplier (full Kelly = 1, half Kelly = 0.5). Default 0.25. */
  kellyFraction: number;
  /** Weight given to Kelly vs inverse-vol vs Sharpe blend. */
  kellyWeight: number;
  inverseVolWeight: number;
  sharpeWeight: number;
  /** Recent performance recency window in days. */
  recencyWindowDays: number;
  /** Recency tilt blend (0 = all-time only, 1 = recent only). Default 0.4. */
  recencyBlend: number;
}

export const DEFAULT_OPTIMIZER_CONFIG: PortfolioOptimizerConfig = {
  maxAllocationPerStrategy: 0.30,
  minAllocationPerStrategy: 0.02,
  kellyFraction: 0.25,
  kellyWeight: 0.35,
  inverseVolWeight: 0.35,
  sharpeWeight: 0.30,
  recencyWindowDays: 30,
  recencyBlend: 0.40,
};

// ── Types ─────────────────────────────────────────────────────────────────────

export interface StrategyAllocationWeight {
  strategyId: number;
  strategyName: string;
  strategyFamily?: string;
  /** Final allocation fraction (0–1, sums to 1 across all active strategies). */
  allocationFraction: number;
  /** USD notional if applying to a budget. */
  allocationUsd?: number;
  /** Component weights before normalisation (for diagnostics). */
  rawWeights: {
    kelly: number;
    inverseVol: number;
    sharpe: number;
    recency: number;
  };
  /** Metrics used. */
  metrics: {
    expectancy: number;
    winRate: number;
    pnlStdDev: number;
    sharpeRatio: number | null;
    recentNetPnl: number;
    totalTrades: number;
  };
  /** True if this strategy hit the max cap. */
  capped: boolean;
  /** True if this strategy hit the min floor. */
  floored: boolean;
}

export interface PortfolioAllocationResult {
  weights: StrategyAllocationWeight[];
  totalAllocated: number;
  unallocatedFraction: number;
  optimizerConfig: PortfolioOptimizerConfig;
  optimizedAt: number;
}

// ── Main optimizer ────────────────────────────────────────────────────────────

/**
 * Compute dynamic allocation weights for a set of active strategies.
 *
 * @param strategyIds  IDs to allocate to (typically active strategies from ranking)
 * @param trades       Full trade history
 * @param budgetUsd    Total budget to allocate (optional — used to compute allocationUsd)
 * @param config       Optimizer configuration
 * @param now          Current timestamp (ms)
 */
export function optimizePortfolio(args: {
  strategyIds: number[];
  trades: readonly MockTrade[];
  budgetUsd?: number;
  config?: Partial<PortfolioOptimizerConfig>;
  now?: number;
  startingEquityUsd?: number;
}): PortfolioAllocationResult {
  const cfg: PortfolioOptimizerConfig = { ...DEFAULT_OPTIMIZER_CONFIG, ...args.config };
  const now = args.now ?? Date.now();
  const startingEquity = args.startingEquityUsd ?? 1_000_000;

  if (args.strategyIds.length === 0) {
    return {
      weights: [],
      totalAllocated: 0,
      unallocatedFraction: 1,
      optimizerConfig: cfg,
      optimizedAt: now,
    };
  }

  // Group trades per strategy
  const byStrategy = new Map<number, MockTrade[]>();
  for (const trade of args.trades) {
    if (!args.strategyIds.includes(trade.strategyId)) continue;
    const bucket = byStrategy.get(trade.strategyId) ?? [];
    bucket.push(trade);
    byStrategy.set(trade.strategyId, bucket);
  }

  // Compute raw weights for each strategy
  const rawEntries: { strategyId: number; rawKelly: number; rawVol: number; rawSharpe: number; rawRecency: number; allMetrics: ReturnType<typeof computeExtendedMetrics>; recentMetrics: ReturnType<typeof computeRollingMetrics>; name: string; family?: string; }[] = [];

  for (const strategyId of args.strategyIds) {
    const trades = byStrategy.get(strategyId) ?? [];
    const name = trades[0]?.strategyName ?? `Strategy ${strategyId}`;
    const family = trades[0]?.strategyFamily;

    const allMetrics = computeExtendedMetrics(trades, startingEquity);
    const recentMetrics = computeRollingMetrics({
      trades, windowDays: cfg.recencyWindowDays, now, startingEquityUsd: startingEquity,
    });

    const rawKelly = _kellyWeight(allMetrics.winRate, allMetrics.averageWin, allMetrics.averageLoss, cfg.kellyFraction);
    const rawVol = _inverseVolWeight(allMetrics.pnlStdDev);
    const rawSharpe = _sharpeWeight(allMetrics.sharpeRatio);
    const rawRecency = _recencyWeight(recentMetrics.netPnl, recentMetrics.totalTrades);

    rawEntries.push({ strategyId, rawKelly, rawVol, rawSharpe, rawRecency, allMetrics, recentMetrics, name, family });
  }

  // Blend raw weights into a single composite
  const composites = rawEntries.map((e) => {
    const allTimeWeight =
      e.rawKelly * cfg.kellyWeight +
      e.rawVol * cfg.inverseVolWeight +
      e.rawSharpe * cfg.sharpeWeight;
    const blended = allTimeWeight * (1 - cfg.recencyBlend) + e.rawRecency * cfg.recencyBlend;
    return { ...e, composite: Math.max(0, blended) };
  });

  // Normalise to [0, 1] sum
  const totalComposite = composites.reduce((s, e) => s + e.composite, 0);

  // Allocate with cap/floor
  const { allocations, totalAllocated } = _applyCapFloor(
    composites.map((e) => ({ id: e.strategyId, weight: totalComposite > 0 ? e.composite / totalComposite : 1 / composites.length })),
    cfg.maxAllocationPerStrategy,
    cfg.minAllocationPerStrategy,
  );

  const allocationMap = new Map(allocations.map((a) => [a.id, a]));

  const weights: StrategyAllocationWeight[] = composites.map((e) => {
    const alloc = allocationMap.get(e.strategyId);
    const fraction = alloc?.weight ?? 0;
    return {
      strategyId: e.strategyId,
      strategyName: e.name,
      strategyFamily: e.family,
      allocationFraction: fraction,
      allocationUsd: args.budgetUsd != null ? args.budgetUsd * fraction : undefined,
      rawWeights: { kelly: e.rawKelly, inverseVol: e.rawVol, sharpe: e.rawSharpe, recency: e.rawRecency },
      metrics: {
        expectancy: e.allMetrics.expectancy,
        winRate: e.allMetrics.winRate,
        pnlStdDev: e.allMetrics.pnlStdDev,
        sharpeRatio: e.allMetrics.sharpeRatio,
        recentNetPnl: e.recentMetrics.netPnl,
        totalTrades: e.allMetrics.totalTrades,
      },
      capped: alloc?.capped ?? false,
      floored: alloc?.floored ?? false,
    };
  });

  // Sort by allocation descending
  weights.sort((a, b) => b.allocationFraction - a.allocationFraction);

  return {
    weights,
    totalAllocated,
    unallocatedFraction: 1 - totalAllocated,
    optimizerConfig: cfg,
    optimizedAt: now,
  };
}

/**
 * Apply allocation from a strategy ranking result directly.
 * Only allocates to ACTIVE strategies.
 */
export function optimizeFromRanking(args: {
  ranking: { rows: StrategyRankRow[] };
  trades: readonly MockTrade[];
  budgetUsd?: number;
  config?: Partial<PortfolioOptimizerConfig>;
  now?: number;
  startingEquityUsd?: number;
}): PortfolioAllocationResult {
  const activeIds = args.ranking.rows
    .filter((r) => r.classification === "ACTIVE")
    .map((r) => r.strategyId);

  return optimizePortfolio({ ...args, strategyIds: activeIds });
}

// ── Internal weight functions ─────────────────────────────────────────────────

/**
 * Fractional Kelly weight based on win rate and win/loss ratio.
 * f* = (W × R - L) / R, where R = avgWin / |avgLoss|.
 * Returns 0 for negative expectation strategies.
 */
function _kellyWeight(winRate: number, avgWin: number, avgLoss: number, kellyFraction: number): number {
  if (!Number.isFinite(winRate) || !Number.isFinite(avgWin) || avgWin <= 0) return 0;
  const absLoss = Math.abs(avgLoss);
  if (absLoss <= 0) return kellyFraction; // pure winners — full Kelly fraction
  const winLossRatio = avgWin / absLoss;
  const lossRate = 1 - winRate;
  const fullKelly = (winRate * winLossRatio - lossRate) / winLossRatio;
  return Math.max(0, fullKelly * kellyFraction);
}

/**
 * Inverse-volatility weight: 1 / pnlStdDev (normalised).
 * Strategies with lower PnL variance get higher weight.
 */
function _inverseVolWeight(pnlStdDev: number): number {
  if (!Number.isFinite(pnlStdDev) || pnlStdDev <= 0) return 0;
  return 1 / pnlStdDev;
}

/**
 * Sharpe-proportional weight: max(0, sharpe).
 */
function _sharpeWeight(sharpe: number | null): number {
  if (sharpe == null || !Number.isFinite(sharpe)) return 0;
  return Math.max(0, sharpe);
}

/**
 * Recency weight: proportional to recent net PnL, floored at 0.
 */
function _recencyWeight(recentNetPnl: number, recentTrades: number): number {
  if (recentTrades < 3) return 0; // insufficient recent data
  return Math.max(0, recentNetPnl);
}

// ── Cap / floor redistribution ─────────────────────────────────────────────────

function _applyCapFloor(
  initial: { id: number; weight: number }[],
  cap: number,
  floor: number,
): { allocations: { id: number; weight: number; capped: boolean; floored: boolean }[]; totalAllocated: number } {
  const n = initial.length;
  if (n === 0) return { allocations: [], totalAllocated: 0 };

  const state = initial.map((e) => ({ ...e, capped: false, floored: false }));

  // Iterative redistribution: cap at max, floor at min, renormalise surplus
  for (let iter = 0; iter < 20; iter++) {
    const uncapped = state.filter((e) => !e.capped && !e.floored);
    if (uncapped.length === 0) break;
    let changed = false;
    for (const e of state) {
      if (e.weight > cap && !e.capped) {
        e.weight = cap; e.capped = true; changed = true;
      }
      if (e.weight < floor && !e.floored) {
        e.weight = floor; e.floored = true; changed = true;
      }
    }
    // Re-normalise unconstrained strategies
    const fixedSum = state.filter((e) => e.capped || e.floored).reduce((s, e) => s + e.weight, 0);
    const remaining = 1 - fixedSum;
    const freeEntries = state.filter((e) => !e.capped && !e.floored);
    const freeTotal = freeEntries.reduce((s, e) => s + e.weight, 0);
    if (freeTotal > 0 && freeEntries.length > 0) {
      for (const e of freeEntries) {
        e.weight = (e.weight / freeTotal) * remaining;
      }
    }
    if (!changed) break;
  }

  const totalAllocated = state.reduce((s, e) => s + e.weight, 0);
  return { allocations: state, totalAllocated: Math.min(1, totalAllocated) };
}

// ── Allocation summary ────────────────────────────────────────────────────────

export interface AllocationSummary {
  activeStrategies: number;
  totalAllocated: number;
  averageAllocation: number;
  maxAllocation: number;
  minAllocation: number;
  cappedCount: number;
  flooredCount: number;
}

export function summarizeAllocation(result: PortfolioAllocationResult): AllocationSummary {
  const n = result.weights.length;
  if (n === 0) {
    return { activeStrategies: 0, totalAllocated: 0, averageAllocation: 0, maxAllocation: 0, minAllocation: 0, cappedCount: 0, flooredCount: 0 };
  }
  const fracs = result.weights.map((w) => w.allocationFraction);
  return {
    activeStrategies: n,
    totalAllocated: result.totalAllocated,
    averageAllocation: fracs.reduce((s, f) => s + f, 0) / n,
    maxAllocation: Math.max(...fracs),
    minAllocation: Math.min(...fracs),
    cappedCount: result.weights.filter((w) => w.capped).length,
    flooredCount: result.weights.filter((w) => w.floored).length,
  };
}
