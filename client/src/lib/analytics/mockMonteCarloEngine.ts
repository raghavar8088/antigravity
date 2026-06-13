/**
 * Monte Carlo Analysis Engine for the BTC futures mock trading module.
 *
 * Generates:
 *   - Equity curve simulations (bootstrap resampling of trade returns)
 *   - Drawdown projections (percentile distribution of max drawdowns)
 *   - Risk-of-ruin estimates (% simulations ending below ruin threshold)
 *   - Confidence intervals for equity at each trade step
 *
 * Algorithm: block-bootstrap resampling of historical realized PnL records.
 * Block size = 5 trades (preserves short-run autocorrelation).
 *
 * Pure functions — no React, no I/O.
 * Randomness is injectable via SimRng for deterministic tests.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import { seededRng, type SimRng } from "@/lib/analytics/mockMarketSimulator";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface MonteCarloConfig {
  /** Number of simulation paths. Default 1000. */
  numSimulations: number;
  /** Number of forward trades to simulate per path. Default = historical count. */
  forwardTrades?: number;
  /** Equity is "ruined" when it falls below this fraction of starting equity. Default 0.5 (50%). */
  ruinThreshold: number;
  /** Starting equity USD. */
  startingEquityUsd: number;
  /** Block size for bootstrap resampling. Default 5. */
  blockSize?: number;
  /** Seed for deterministic RNG. Omit for random. */
  seed?: number;
}

export const DEFAULT_MC_CONFIG: MonteCarloConfig = {
  numSimulations: 1000,
  ruinThreshold: 0.5,
  startingEquityUsd: 1_000_000,
  blockSize: 5,
};

export interface MonteCarloPath {
  finalEquity: number;
  maxDrawdownPct: number;
  finalReturn: number;
}

export interface EquityPercentiles {
  p5: number;
  p25: number;
  p50: number;
  p75: number;
  p95: number;
}

export interface MonteCarloResult {
  /** Percentile equity at each forward trade step across all simulations. */
  equityPercentilesByStep: EquityPercentiles[];
  /** Distribution of final equity values. */
  finalEquityPercentiles: EquityPercentiles;
  /** Distribution of max drawdown across all paths. */
  drawdownPercentiles: EquityPercentiles;
  /** Fraction of paths ending below the ruin threshold (0–1). */
  riskOfRuin: number;
  /** Fraction of paths ending in profit. */
  profitProbability: number;
  /** Median path annualized return. */
  medianAnnualizedReturn: number;
  /** Summary of all paths. */
  paths: MonteCarloPath[];
  /** Config used. */
  config: MonteCarloConfig;
  computedAt: number;
}

// ── Main computation ──────────────────────────────────────────────────────────

/**
 * Run a Monte Carlo simulation from the historical closed trades.
 * Returns null when insufficient trade history (< 10 trades).
 */
export function runMonteCarlo(
  trades: readonly MockTrade[],
  config: Partial<MonteCarloConfig> = {},
): MonteCarloResult | null {
  const cfg: MonteCarloConfig = { ...DEFAULT_MC_CONFIG, ...config };
  const rng = cfg.seed != null ? seededRng(cfg.seed) : { next: Math.random };

  const closed = trades
    .filter((t) => t.status === "CLOSED" && t.closedAt != null)
    .sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0));

  if (closed.length < 10) return null;

  const returns = closed.map((t) => t.realizedPnl);
  const forwardN = cfg.forwardTrades ?? returns.length;
  const blockSize = Math.max(1, cfg.blockSize ?? 5);

  // Run simulations
  const allPaths: number[][] = [];
  const summaries: MonteCarloPath[] = [];

  for (let sim = 0; sim < cfg.numSimulations; sim++) {
    const path = _bootstrapPath(returns, forwardN, blockSize, cfg.startingEquityUsd, rng);
    allPaths.push(path);

    const finalEquity = path[path.length - 1];
    const maxDd = _maxDrawdown(path);
    summaries.push({
      finalEquity,
      maxDrawdownPct: maxDd,
      finalReturn: (finalEquity - cfg.startingEquityUsd) / cfg.startingEquityUsd,
    });
  }

  // Equity percentiles by step
  const equityPercentilesByStep: EquityPercentiles[] = [];
  for (let step = 0; step < forwardN; step++) {
    const vals = allPaths.map((p) => p[Math.min(step, p.length - 1)] ?? cfg.startingEquityUsd);
    equityPercentilesByStep.push(_percentiles(vals));
  }

  // Final equity percentiles
  const finalEquities = summaries.map((s) => s.finalEquity);
  const finalEquityPercentiles = _percentiles(finalEquities);

  // Drawdown distribution
  const drawdowns = summaries.map((s) => s.maxDrawdownPct);
  const drawdownPercentiles = _percentiles(drawdowns);

  // Risk of ruin
  const ruinLevel = cfg.startingEquityUsd * cfg.ruinThreshold;
  const ruinCount = summaries.filter((s) => s.finalEquity < ruinLevel).length;
  const riskOfRuin = ruinCount / cfg.numSimulations;

  // Profit probability
  const profitCount = summaries.filter((s) => s.finalEquity > cfg.startingEquityUsd).length;
  const profitProbability = profitCount / cfg.numSimulations;

  // Median annualized return
  const sortedReturns = [...summaries.map((s) => s.finalReturn)].sort((a, b) => a - b);
  const medianReturn = _medianSorted(sortedReturns);
  const spanMs = closed.length >= 2
    ? (closed[closed.length - 1].closedAt ?? 0) - (closed[0].openedAt)
    : 365.25 * 24 * 60 * 60 * 1000;
  const spanYears = Math.max(0.01, spanMs / (365.25 * 24 * 60 * 60 * 1000));
  const medianAnnualizedReturn = medianReturn / spanYears;

  return {
    equityPercentilesByStep,
    finalEquityPercentiles,
    drawdownPercentiles,
    riskOfRuin,
    profitProbability,
    medianAnnualizedReturn,
    paths: summaries,
    config: cfg,
    computedAt: Date.now(),
  };
}

// ── Confidence interval at a specific equity level ────────────────────────────

export interface EquityConfidenceInterval {
  low: number;
  mid: number;
  high: number;
  confidence: number;
}

/**
 * Get confidence interval for equity after N more trades from MC result.
 */
export function equityConfidenceAt(
  result: MonteCarloResult,
  afterTrades: number,
  confidence = 0.9,
): EquityConfidenceInterval {
  const step = Math.min(afterTrades - 1, result.equityPercentilesByStep.length - 1);
  if (step < 0) {
    const eq = result.config.startingEquityUsd;
    return { low: eq, mid: eq, high: eq, confidence };
  }
  const p = result.equityPercentilesByStep[step];
  const tailPct = ((1 - confidence) / 2) * 100;
  return {
    low: tailPct <= 5 ? p.p5 : p.p25,
    mid: p.p50,
    high: tailPct <= 5 ? p.p95 : p.p75,
    confidence,
  };
}

// ── Scenario stress metrics ───────────────────────────────────────────────────

export interface StressScenarioResult {
  scenarioName: string;
  /** Number of consecutive losing trades in the scenario. */
  worstStreak: number;
  /** Equity after worst streak. */
  equityAfterWorstStreak: number;
  /** Time to recover from worst streak (in forward trades). */
  recoveryTrades: number | null;
  survivalProbability: number;
}

/**
 * Analyse the worst-streak scenario from all MC paths.
 */
export function analyseWorstStreak(
  result: MonteCarloResult,
  trades: readonly MockTrade[],
): StressScenarioResult {
  const closed = trades.filter((t) => t.status === "CLOSED");
  if (closed.length === 0) {
    return {
      scenarioName: "Worst streak",
      worstStreak: 0,
      equityAfterWorstStreak: result.config.startingEquityUsd,
      recoveryTrades: null,
      survivalProbability: 1,
    };
  }
  const returns = closed.map((t) => t.realizedPnl);
  let maxStreak = 0, currentStreak = 0;
  for (const pnl of returns) {
    if (pnl < 0) { currentStreak++; maxStreak = Math.max(maxStreak, currentStreak); }
    else currentStreak = 0;
  }

  const avgLoss = returns.filter((r) => r < 0).reduce((s, r) => s + Math.abs(r), 0) /
    Math.max(1, returns.filter((r) => r < 0).length);
  const equityAfterWorstStreak = result.config.startingEquityUsd - maxStreak * avgLoss;
  const survivalRate = result.paths.filter(
    (p) => p.finalEquity > result.config.startingEquityUsd * result.config.ruinThreshold,
  ).length / result.paths.length;

  return {
    scenarioName: `Worst streak (${maxStreak} consecutive losses)`,
    worstStreak: maxStreak,
    equityAfterWorstStreak,
    recoveryTrades: null,
    survivalProbability: survivalRate,
  };
}

// ── Internal helpers ──────────────────────────────────────────────────────────

function _bootstrapPath(
  returns: readonly number[],
  n: number,
  blockSize: number,
  startingEquity: number,
  rng: SimRng,
): number[] {
  const path: number[] = [];
  let equity = startingEquity;
  let i = 0;
  while (i < n) {
    // Pick a random block start
    const blockStart = Math.floor(rng.next() * Math.max(1, returns.length - blockSize + 1));
    for (let b = 0; b < blockSize && i < n; b++, i++) {
      equity += returns[(blockStart + b) % returns.length] ?? 0;
      path.push(equity);
    }
  }
  return path;
}

function _maxDrawdown(path: readonly number[]): number {
  let peak = path[0] ?? 0;
  let maxDd = 0;
  for (const eq of path) {
    if (eq > peak) peak = eq;
    const dd = peak > 0 ? (peak - eq) / peak : 0;
    if (dd > maxDd) maxDd = dd;
  }
  return maxDd;
}

function _percentiles(values: readonly number[]): EquityPercentiles {
  const sorted = [...values].sort((a, b) => a - b);
  const n = sorted.length;
  if (n === 0) return { p5: 0, p25: 0, p50: 0, p75: 0, p95: 0 };
  return {
    p5: sorted[Math.floor(n * 0.05)] ?? sorted[0],
    p25: sorted[Math.floor(n * 0.25)] ?? sorted[0],
    p50: sorted[Math.floor(n * 0.5)] ?? sorted[0],
    p75: sorted[Math.floor(n * 0.75)] ?? sorted[0],
    p95: sorted[Math.floor(n * 0.95)] ?? sorted[0],
  };
}

function _medianSorted(sorted: readonly number[]): number {
  const n = sorted.length;
  if (n === 0) return 0;
  const mid = Math.floor(n / 2);
  return n % 2 === 0
    ? ((sorted[mid - 1] ?? 0) + (sorted[mid] ?? 0)) / 2
    : (sorted[mid] ?? 0);
}
