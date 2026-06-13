/**
 * Strategy Ranking Engine for the BTC futures mock trading module.
 *
 * Ranks strategies using a composite weighted score across:
 *   - Profitability       30%  (net PnL + expectancy)
 *   - Consistency         20%  (Sharpe ratio)
 *   - Drawdown            20%  (max drawdown inverted)
 *   - Trade sample size   15%  (log-scaled trade count)
 *   - Risk-adjusted       15%  (Calmar ratio)
 *
 * Recent performance is exponentially weighted (half-life = 30 days).
 *
 * Auto-classifies each strategy as:
 *   ACTIVE     — composite score ≥ 60 and ≥ MIN_QUALIFYING_TRADES
 *   WATCHLIST  — composite score 35–59 or insufficient data
 *   DISABLED   — composite score < 35 or failed walk-forward validation
 *
 * Pure functions — no React, no I/O.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import { computeExtendedMetrics, computeRollingMetrics } from "@/lib/analytics/mockExtendedMetrics";

// ── Constants ─────────────────────────────────────────────────────────────────

const WEIGHT_PROFITABILITY = 0.30;
const WEIGHT_CONSISTENCY   = 0.20;
const WEIGHT_DRAWDOWN      = 0.20;
const WEIGHT_SAMPLE_SIZE   = 0.15;
const WEIGHT_RISK_ADJUSTED = 0.15;

const MIN_QUALIFYING_TRADES = 10;
const ACTIVE_THRESHOLD      = 60;
const WATCHLIST_MIN         = 35;

/** Half-life for exponential time-weighting (ms). 30 days. */
const RECENT_HALF_LIFE_MS = 30 * 24 * 60 * 60 * 1000;

// ── Types ─────────────────────────────────────────────────────────────────────

export type StrategyClassification = "ACTIVE" | "WATCHLIST" | "DISABLED";

export interface StrategyRankRow {
  strategyId: number;
  strategyName: string;
  strategyFamily?: string;

  /** Composite rank score (0–100). */
  score: number;
  classification: StrategyClassification;
  rank: number;

  /** Score components (each 0–100 before weighting). */
  components: {
    profitability: number;
    consistency: number;
    drawdown: number;
    sampleSize: number;
    riskAdjusted: number;
  };

  /** Core metrics. */
  totalTrades: number;
  winRate: number;
  profitFactor: number | null;
  expectancy: number;
  sharpeRatio: number | null;
  calmarRatio: number | null;
  maxDrawdownPct: number;
  netPnl: number;

  /** Recent (30-day) vs all-time performance. */
  recentNetPnl: number;
  recentWinRate: number;
  recentTrades: number;

  /** Walk-forward validation status (if available). */
  walkForwardStatus?: "PASS" | "FAIL" | "COLLECT_DATA";

  /** Regime breakdown: best/worst regime by expectancy. */
  bestRegime?: string;
  worstRegime?: string;

  lastTradeAt: number | null;
  disabledReason?: string;
}

export interface StrategyRankingResult {
  rows: StrategyRankRow[];
  rankedAt: number;
  totalStrategies: number;
  activeCount: number;
  watchlistCount: number;
  disabledCount: number;
}

// ── Main ranking function ─────────────────────────────────────────────────────

/**
 * Rank all strategies that appear in the trade list.
 */
export function rankStrategies(args: {
  trades: readonly MockTrade[];
  startingEquityUsd?: number;
  now?: number;
  /** Map of strategyId → walk-forward status. Optional. */
  walkForwardStatuses?: Map<number, "PASS" | "FAIL" | "COLLECT_DATA">;
}): StrategyRankingResult {
  const now = args.now ?? Date.now();
  const startingEquity = args.startingEquityUsd ?? 1_000_000;
  const wfStatuses = args.walkForwardStatuses ?? new Map();

  // Group trades by strategy
  const byStrategy = new Map<number, MockTrade[]>();
  for (const trade of args.trades) {
    const bucket = byStrategy.get(trade.strategyId) ?? [];
    bucket.push(trade);
    byStrategy.set(trade.strategyId, bucket);
  }

  const rows: StrategyRankRow[] = [];

  for (const [strategyId, stratTrades] of byStrategy.entries()) {
    const row = _buildRankRow(strategyId, stratTrades, startingEquity, now, wfStatuses);
    rows.push(row);
  }

  // Sort by score descending
  rows.sort((a, b) => b.score - a.score);
  rows.forEach((r, i) => { r.rank = i + 1; });

  const activeCount = rows.filter((r) => r.classification === "ACTIVE").length;
  const watchlistCount = rows.filter((r) => r.classification === "WATCHLIST").length;
  const disabledCount = rows.filter((r) => r.classification === "DISABLED").length;

  return {
    rows,
    rankedAt: now,
    totalStrategies: rows.length,
    activeCount,
    watchlistCount,
    disabledCount,
  };
}

// ── Walk-forward override ────────────────────────────────────────────────────

/**
 * Apply walk-forward results to an existing ranking, downgrading FAIL strategies
 * to DISABLED regardless of their composite score.
 */
export function applyWalkForwardOverrides(
  result: StrategyRankingResult,
  wfMap: Map<number, "PASS" | "FAIL" | "COLLECT_DATA">,
): StrategyRankingResult {
  const updated = result.rows.map((row) => {
    const wfStatus = wfMap.get(row.strategyId);
    if (wfStatus === "FAIL") {
      return {
        ...row,
        walkForwardStatus: "FAIL" as const,
        classification: "DISABLED" as StrategyClassification,
        disabledReason: "Failed walk-forward validation",
      };
    }
    return { ...row, walkForwardStatus: wfStatus };
  });

  const activeCount = updated.filter((r) => r.classification === "ACTIVE").length;
  const watchlistCount = updated.filter((r) => r.classification === "WATCHLIST").length;
  const disabledCount = updated.filter((r) => r.classification === "DISABLED").length;

  return {
    ...result,
    rows: updated,
    activeCount,
    watchlistCount,
    disabledCount,
  };
}

// ── Score normalisation helpers ───────────────────────────────────────────────

/** Clip a value to [0, 100]. */
function _clamp100(v: number): number {
  return Math.min(100, Math.max(0, Number.isFinite(v) ? v : 0));
}

/** Normalise PnL to [0, 100] using a sigmoid-like shape centred at 0. */
function _normalisePnl(pnl: number, scale: number = 50_000): number {
  return _clamp100(50 + (pnl / scale) * 50);
}

/** Normalise Sharpe to [0, 100]. Sharpe of 2 → 100, 0 → 50, <0 → <50. */
function _normaliseSharpe(sharpe: number | null): number {
  if (sharpe == null) return 30; // insufficient data penalty
  return _clamp100(50 + sharpe * 25);
}

/** Normalise drawdown (lower = better). 0% drawdown = 100, 50%+ = 0. */
function _normaliseDrawdown(ddPct: number): number {
  return _clamp100(100 - ddPct * 200);
}

/** Normalise Calmar to [0, 100]. Calmar 1 → 70, 0 → 50, 2+ → 100. */
function _normaliseCalmar(calmar: number | null): number {
  if (calmar == null) return 30;
  return _clamp100(50 + calmar * 25);
}

/** Normalise trade count to [0, 100] with log scaling (saturates at 200). */
function _normaliseSampleSize(n: number): number {
  return _clamp100((Math.log2(Math.max(1, n)) / Math.log2(200)) * 100);
}

// ── Internal row builder ──────────────────────────────────────────────────────

function _buildRankRow(
  strategyId: number,
  trades: MockTrade[],
  startingEquity: number,
  now: number,
  wfStatuses: Map<number, "PASS" | "FAIL" | "COLLECT_DATA">,
): StrategyRankRow {
  const allMetrics = computeExtendedMetrics(trades, startingEquity);
  const recentMetrics = computeRollingMetrics({ trades, windowDays: 30, now, startingEquityUsd: startingEquity });

  // Time-weighted expectancy boost for recent trades
  const timeWeightedExpectancy = _timeWeightedExpectancy(trades, now);

  const closedTrades = trades.filter((t) => t.status === "CLOSED");
  const lastTradeAt = closedTrades.length > 0
    ? Math.max(...closedTrades.map((t) => t.closedAt ?? 0))
    : null;

  // Score components (0–100 each)
  const profitability = _clamp100(
    _normalisePnl(allMetrics.netPnl) * 0.6 +
    _normalisePnl(timeWeightedExpectancy, 1000) * 0.4,
  );
  const consistency = _normaliseSharpe(allMetrics.sharpeRatio);
  const drawdown = _normaliseDrawdown(allMetrics.maxDrawdownPct * 100);
  const sampleSize = _normaliseSampleSize(allMetrics.totalTrades);
  const riskAdjusted = _normaliseCalmar(allMetrics.calmarRatio);

  const compositeScore =
    profitability  * WEIGHT_PROFITABILITY +
    consistency    * WEIGHT_CONSISTENCY +
    drawdown       * WEIGHT_DRAWDOWN +
    sampleSize     * WEIGHT_SAMPLE_SIZE +
    riskAdjusted   * WEIGHT_RISK_ADJUSTED;

  // Regime breakdown
  const { bestRegime, worstRegime } = _regimeBreakdown(trades);

  // Walk-forward status
  const walkForwardStatus = wfStatuses.get(strategyId);

  // Classification
  const { classification, disabledReason } = _classify(
    compositeScore,
    allMetrics.totalTrades,
    walkForwardStatus,
  );

  return {
    strategyId,
    strategyName: trades[0]?.strategyName ?? `Strategy ${strategyId}`,
    strategyFamily: trades[0]?.strategyFamily,
    score: Math.round(compositeScore),
    classification,
    rank: 0, // set after sort
    components: {
      profitability: Math.round(profitability),
      consistency: Math.round(consistency),
      drawdown: Math.round(drawdown),
      sampleSize: Math.round(sampleSize),
      riskAdjusted: Math.round(riskAdjusted),
    },
    totalTrades: allMetrics.totalTrades,
    winRate: allMetrics.winRate,
    profitFactor: allMetrics.profitFactor,
    expectancy: allMetrics.expectancy,
    sharpeRatio: allMetrics.sharpeRatio,
    calmarRatio: allMetrics.calmarRatio,
    maxDrawdownPct: allMetrics.maxDrawdownPct,
    netPnl: allMetrics.netPnl,
    recentNetPnl: recentMetrics.netPnl,
    recentWinRate: recentMetrics.winRate,
    recentTrades: recentMetrics.totalTrades,
    walkForwardStatus,
    bestRegime,
    worstRegime,
    lastTradeAt,
    disabledReason,
  };
}

function _classify(
  score: number,
  totalTrades: number,
  wfStatus: "PASS" | "FAIL" | "COLLECT_DATA" | undefined,
): { classification: StrategyClassification; disabledReason?: string } {
  if (wfStatus === "FAIL") {
    return { classification: "DISABLED", disabledReason: "Failed walk-forward validation" };
  }
  if (totalTrades < MIN_QUALIFYING_TRADES) {
    return { classification: "WATCHLIST", disabledReason: `Needs ${MIN_QUALIFYING_TRADES - totalTrades} more closed trades` };
  }
  if (score >= ACTIVE_THRESHOLD) return { classification: "ACTIVE" };
  if (score >= WATCHLIST_MIN) return { classification: "WATCHLIST" };
  return { classification: "DISABLED", disabledReason: `Composite score ${score.toFixed(0)} below threshold ${WATCHLIST_MIN}` };
}

function _timeWeightedExpectancy(trades: MockTrade[], now: number): number {
  const closed = trades.filter((t) => t.status === "CLOSED" && t.closedAt != null);
  if (closed.length === 0) return 0;
  let weightSum = 0;
  let weightedPnl = 0;
  for (const t of closed) {
    const ageMs = now - (t.closedAt ?? now);
    const weight = Math.exp(-ageMs / RECENT_HALF_LIFE_MS);
    weightSum += weight;
    weightedPnl += t.realizedPnl * weight;
  }
  return weightSum > 0 ? weightedPnl / weightSum : 0;
}

function _regimeBreakdown(trades: MockTrade[]): { bestRegime?: string; worstRegime?: string } {
  const closed = trades.filter((t) => t.status === "CLOSED" && t.regimeAtEntry);
  if (closed.length === 0) return {};

  const byRegime = new Map<string, number[]>();
  for (const t of closed) {
    const r = t.regimeAtEntry as string;
    const bucket = byRegime.get(r) ?? [];
    bucket.push(t.realizedPnl);
    byRegime.set(r, bucket);
  }

  let bestRegime: string | undefined;
  let bestAvg = -Infinity;
  let worstRegime: string | undefined;
  let worstAvg = Infinity;

  for (const [regime, pnls] of byRegime) {
    const avg = pnls.reduce((s, p) => s + p, 0) / pnls.length;
    if (avg > bestAvg) { bestAvg = avg; bestRegime = regime; }
    if (avg < worstAvg) { worstAvg = avg; worstRegime = regime; }
  }

  return { bestRegime, worstRegime };
}

// ── Leaderboard helpers ───────────────────────────────────────────────────────

/** Return strategies classified as ACTIVE, sorted by score descending. */
export function activeStrategies(result: StrategyRankingResult): StrategyRankRow[] {
  return result.rows.filter((r) => r.classification === "ACTIVE");
}

/** Return strategies that should be on the watchlist. */
export function watchlistStrategies(result: StrategyRankingResult): StrategyRankRow[] {
  return result.rows.filter((r) => r.classification === "WATCHLIST");
}

/** Return disabled strategies with reasons. */
export function disabledStrategies(result: StrategyRankingResult): StrategyRankRow[] {
  return result.rows.filter((r) => r.classification === "DISABLED");
}

/** Return top-N strategies by composite score. */
export function topNStrategies(result: StrategyRankingResult, n: number): StrategyRankRow[] {
  return result.rows.slice(0, Math.max(1, n));
}

/** Compute rank delta (positive = improved, negative = fell) for two consecutive rankings. */
export function computeRankDeltas(
  previous: StrategyRankingResult,
  current: StrategyRankingResult,
): Map<number, number> {
  const prevRanks = new Map(previous.rows.map((r) => [r.strategyId, r.rank]));
  const deltas = new Map<number, number>();
  for (const row of current.rows) {
    const prev = prevRanks.get(row.strategyId);
    deltas.set(row.strategyId, prev != null ? prev - row.rank : 0);
  }
  return deltas;
}
