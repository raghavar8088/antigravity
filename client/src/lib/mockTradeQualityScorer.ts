/**
 * Trade Quality Scoring Engine for the BTC futures mock trading module.
 *
 * For every closed (or open) trade, produces a composite 0–100 quality score
 * across five dimensions:
 *
 *   1. Entry quality   (0–25): How good was the entry signal and timing?
 *   2. Exit quality    (0–25): How well was the position exited?
 *   3. Risk quality    (0–25): Was position sizing and R:R appropriate?
 *   4. Timing quality  (0–15): Was the trade placed in a favourable session/regime?
 *   5. Execution quality (0–10): How close to ideal were fills and latency?
 *
 * Overall score = sum of five components (0–100).
 *
 * Pure functions — no React, no I/O.
 */

import type { MockTrade } from "@/lib/mockTradingEngine";

// ── Component scores ──────────────────────────────────────────────────────────

export interface TradeQualityComponents {
  entryScore: number;   // 0–25
  exitScore: number;    // 0–25
  riskScore: number;    // 0–25
  timingScore: number;  // 0–15
  executionScore: number; // 0–10
}

export interface TradeQualityResult extends TradeQualityComponents {
  /** Composite 0–100. */
  overallScore: number;
  /** Grade: A (≥80), B (60–79), C (40–59), D (20–39), F (<20). */
  grade: "A" | "B" | "C" | "D" | "F";
  /** Human-readable breakdown for the diagnostics panel. */
  breakdown: TradeQualityBreakdown;
}

export interface TradeQualityBreakdown {
  entryReason: string;
  exitReason: string;
  riskReason: string;
  timingReason: string;
  executionReason: string;
}

// ── Scoring context ───────────────────────────────────────────────────────────

export interface TradeQualityContext {
  /** Best possible signal score for this strategy (used for normalisation). */
  maxSignalScore?: number;
  /** Minimum R:R ratio configured for this engine. */
  minRiskReward?: number;
  /** Ideal max hold time in minutes. */
  idealMaxHoldMinutes?: number;
  /** Simulated execution latency in ms (from market simulator). */
  executionLatencyMs?: number;
  /** Actual fill slippage in bps at entry (compared to signal price). */
  entrySlippageBps?: number;
  /** Actual fill slippage in bps at exit. */
  exitSlippageBps?: number;
}

// ── Main scorer ───────────────────────────────────────────────────────────────

/**
 * Score a trade on all five quality dimensions.
 * Works for both OPEN (partial score using unrealized) and CLOSED trades.
 */
export function scoreTrade(trade: MockTrade, ctx: TradeQualityContext = {}): TradeQualityResult {
  const entry = _scoreEntry(trade, ctx);
  const exit = _scoreExit(trade, ctx);
  const risk = _scoreRisk(trade, ctx);
  const timing = _scoreTiming(trade, ctx);
  const execution = _scoreExecution(trade, ctx);

  const overall = entry.score + exit.score + risk.score + timing.score + execution.score;

  return {
    entryScore: entry.score,
    exitScore: exit.score,
    riskScore: risk.score,
    timingScore: timing.score,
    executionScore: execution.score,
    overallScore: Math.min(100, Math.max(0, Math.round(overall))),
    grade: _grade(overall),
    breakdown: {
      entryReason: entry.reason,
      exitReason: exit.reason,
      riskReason: risk.reason,
      timingReason: timing.reason,
      executionReason: execution.reason,
    },
  };
}

/**
 * Score a batch of trades. Returns results in the same order as input.
 */
export function scoreAllTrades(
  trades: readonly MockTrade[],
  ctx: TradeQualityContext = {},
): TradeQualityResult[] {
  return trades.map((t) => scoreTrade(t, ctx));
}

// ── Diagnostics helpers ───────────────────────────────────────────────────────

export interface TradeDiagnostics {
  whyEntered: string;
  whyExited: string | null;
  whyRejected?: string;
  filtersPassed: string[];
  filtersFailed: string[];
  confidenceScore: number;
  signalScore: number;
  riskScore: number;
}

/**
 * Generate a human-readable diagnostic for a trade.
 * `filtersPassed` and `filtersFailed` mirror the mock engine's blocker pipeline.
 */
export function buildTradeDiagnostics(trade: MockTrade): TradeDiagnostics {
  const filtersPassed: string[] = [];
  const filtersFailed: string[] = [];

  // Signal filter
  if (trade.signalScore >= trade.requiredThreshold) {
    filtersPassed.push(`Signal score (${trade.signalScore.toFixed(1)} ≥ ${trade.requiredThreshold.toFixed(1)})`);
  } else {
    filtersFailed.push(`Signal score (${trade.signalScore.toFixed(1)} < ${trade.requiredThreshold.toFixed(1)})`);
  }

  // Regime filter
  if (trade.regimeAtEntry) {
    filtersPassed.push(`Regime: ${trade.regimeAtEntry}`);
  }

  // R:R filter
  if (trade.riskRewardRatio >= 1.5) {
    filtersPassed.push(`R:R ratio (${trade.riskRewardRatio.toFixed(2)})`);
  } else {
    filtersFailed.push(`R:R ratio too low (${trade.riskRewardRatio.toFixed(2)})`);
  }

  // Blocker filters
  for (const b of trade.blockers) {
    filtersFailed.push(`${b.gate}: ${b.reason}`);
  }

  const whyEntered = [
    `Strategy ${trade.strategyName} generated a ${trade.side} signal`,
    `Signal score: ${trade.signalScore.toFixed(1)}`,
    trade.regimeAtEntry ? `Regime: ${trade.regimeAtEntry}` : null,
    trade.strategyFamily ? `Family: ${trade.strategyFamily}` : null,
  ].filter(Boolean).join(". ");

  let whyExited: string | null = null;
  if (trade.status === "CLOSED" && trade.exitReason) {
    const exitMap: Record<string, string> = {
      TAKE_PROFIT: `Take profit hit at ${trade.exitPrice?.toFixed(2) ?? "—"} (TP level: ${trade.takeProfitPrice.toFixed(2)})`,
      STOP_LOSS: `Stop loss hit at ${trade.exitPrice?.toFixed(2) ?? "—"} (SL level: ${trade.stopLossPrice.toFixed(2)})`,
      MAX_HOLD: `Maximum hold time reached (${((trade.closedAt ?? trade.openedAt) - trade.openedAt) / 60_000 | 0} minutes)`,
      MANUAL: "Manually closed",
    };
    whyExited = exitMap[trade.exitReason] ?? trade.exitReason;
  }

  return {
    whyEntered,
    whyExited,
    filtersPassed,
    filtersFailed,
    confidenceScore: trade.confidenceScore ?? trade.signalScore,
    signalScore: trade.signalScore,
    riskScore: Math.min(100, trade.riskRewardRatio * 30),
  };
}

// ── Aggregate quality stats ───────────────────────────────────────────────────

export interface QualityAggregate {
  averageOverall: number;
  averageEntry: number;
  averageExit: number;
  averageRisk: number;
  averageTiming: number;
  averageExecution: number;
  gradeDistribution: Record<"A" | "B" | "C" | "D" | "F", number>;
  bestTrades: { tradeId: string; score: number }[];
  worstTrades: { tradeId: string; score: number }[];
}

export function computeQualityAggregate(
  trades: readonly MockTrade[],
  ctx: TradeQualityContext = {},
): QualityAggregate {
  if (trades.length === 0) {
    return {
      averageOverall: 0,
      averageEntry: 0,
      averageExit: 0,
      averageRisk: 0,
      averageTiming: 0,
      averageExecution: 0,
      gradeDistribution: { A: 0, B: 0, C: 0, D: 0, F: 0 },
      bestTrades: [],
      worstTrades: [],
    };
  }

  const scores = trades.map((t) => ({ trade: t, result: scoreTrade(t, ctx) }));
  const n = scores.length;

  const dist: Record<"A" | "B" | "C" | "D" | "F", number> = { A: 0, B: 0, C: 0, D: 0, F: 0 };
  let sumOverall = 0, sumEntry = 0, sumExit = 0, sumRisk = 0, sumTiming = 0, sumExec = 0;

  for (const { result } of scores) {
    sumOverall += result.overallScore;
    sumEntry += result.entryScore;
    sumExit += result.exitScore;
    sumRisk += result.riskScore;
    sumTiming += result.timingScore;
    sumExec += result.executionScore;
    dist[result.grade]++;
  }

  const sorted = [...scores].sort((a, b) => b.result.overallScore - a.result.overallScore);
  const bestTrades = sorted.slice(0, 5).map(({ trade, result }) => ({ tradeId: trade.id, score: result.overallScore }));
  const worstTrades = sorted.slice(-5).reverse().map(({ trade, result }) => ({ tradeId: trade.id, score: result.overallScore }));

  return {
    averageOverall: sumOverall / n,
    averageEntry: sumEntry / n,
    averageExit: sumExit / n,
    averageRisk: sumRisk / n,
    averageTiming: sumTiming / n,
    averageExecution: sumExec / n,
    gradeDistribution: dist,
    bestTrades,
    worstTrades,
  };
}

// ── Internal dimension scorers ────────────────────────────────────────────────

interface DimScore { score: number; reason: string; }

function _scoreEntry(trade: MockTrade, ctx: TradeQualityContext): DimScore {
  // Signal strength relative to threshold (0–12.5 pts)
  const maxScore = ctx.maxSignalScore ?? 100;
  const threshold = Math.max(1, trade.requiredThreshold);
  const signalRatio = trade.signalScore / Math.max(1, maxScore);
  const signalPts = _clamp(signalRatio * 12.5, 0, 12.5);

  // Confidence score bonus (0–7.5 pts)
  const confidence = trade.confidenceScore ?? trade.signalScore;
  const confPts = _clamp((confidence / 100) * 7.5, 0, 7.5);

  // Regime alignment bonus (0–5 pts)
  const regimePts = trade.regimeAtEntry ? 3 : 0;

  const score = signalPts + confPts + regimePts;
  const reason = `Signal strength ${(signalRatio * 100).toFixed(0)}% of max (${signalPts.toFixed(1)}/12.5), confidence ${confidence.toFixed(0)} (${confPts.toFixed(1)}/7.5), regime ${trade.regimeAtEntry ?? "unknown"} (${regimePts}/5)`;

  return { score: _clamp(score, 0, 25), reason };
}

function _scoreExit(trade: MockTrade, ctx: TradeQualityContext): DimScore {
  if (trade.status === "OPEN") {
    return { score: 12.5, reason: "Trade still open — partial score (12.5/25)" };
  }

  // TP hit = best (25 pts), SL hit = poor (5 pts), max-hold = mediocre (12 pts), manual = neutral (15 pts)
  const reasonMap: Record<string, number> = {
    TAKE_PROFIT: 25,
    STOP_LOSS: 5,
    MAX_HOLD: 12,
    MANUAL: 15,
  };
  const exitPts = reasonMap[trade.exitReason ?? "MANUAL"] ?? 15;

  // Penalize for PnL below expected (max −5)
  const expectedTpPnl = trade.takeProfitUsd;
  let pnlPenalty = 0;
  if (expectedTpPnl > 0 && trade.realizedPnl < expectedTpPnl * 0.8) {
    pnlPenalty = Math.min(5, (1 - trade.realizedPnl / expectedTpPnl) * 5);
  }

  const score = _clamp(exitPts - pnlPenalty, 0, 25);
  const reason = `Exit reason: ${trade.exitReason ?? "MANUAL"} (${exitPts}/25 base, −${pnlPenalty.toFixed(1)} PnL penalty)`;
  return { score, reason };
}

function _scoreRisk(trade: MockTrade, ctx: TradeQualityContext): DimScore {
  // R:R ratio (0–15 pts): ideal ≥ 3:1
  const minRr = ctx.minRiskReward ?? 1.5;
  const rr = trade.riskRewardRatio;
  const rrPts = rr <= 0 ? 0 : _clamp(Math.min(rr / 3, 1) * 15, 0, 15);

  // Blockers penalty (−3 per blocker, max −9)
  const blockerPenalty = Math.min(9, trade.blockers.length * 3);

  // Margin efficiency (0–10 pts): lower leverage = more conservative = higher score
  const lev = Math.max(1, trade.leverage);
  const leveragePts = _clamp((1 - (lev - 1) / 124) * 10, 0, 10);

  const score = _clamp(rrPts - blockerPenalty + leveragePts, 0, 25);
  const reason = `R:R ${rr.toFixed(2)} (${rrPts.toFixed(1)}/15), blockers −${blockerPenalty}, leverage ${lev}× (${leveragePts.toFixed(1)}/10)`;
  return { score, reason };
}

function _scoreTiming(trade: MockTrade, ctx: TradeQualityContext): DimScore {
  const ideal = ctx.idealMaxHoldMinutes ?? 120;
  const actualMin = trade.closedAt != null
    ? (trade.closedAt - trade.openedAt) / 60_000
    : (Date.now() - trade.openedAt) / 60_000;

  // Time efficiency: shorter hold that hits TP = better
  let timePts = 7.5;
  if (trade.exitReason === "TAKE_PROFIT") {
    const efficiency = Math.min(1, (ideal - actualMin) / ideal);
    timePts = _clamp(7.5 + efficiency * 7.5, 0, 15);
  } else if (trade.exitReason === "MAX_HOLD") {
    timePts = 3;
  }

  const reason = `Hold time ${actualMin.toFixed(0)}min vs ideal ${ideal}min (${timePts.toFixed(1)}/15)`;
  return { score: _clamp(timePts, 0, 15), reason };
}

function _scoreExecution(trade: MockTrade, ctx: TradeQualityContext): DimScore {
  // Latency score (0–5): ≤50ms = 5pts, ≥500ms = 0pts
  const latency = ctx.executionLatencyMs ?? 100;
  const latencyPts = _clamp((1 - (latency - 50) / 450) * 5, 0, 5);

  // Entry slippage (0–5): 0 bps = 5pts, ≥10 bps = 0pts
  const entrySlippage = ctx.entrySlippageBps ?? 5;
  const slippagePts = _clamp((1 - entrySlippage / 10) * 5, 0, 5);

  const score = _clamp(latencyPts + slippagePts, 0, 10);
  const reason = `Latency ${latency}ms (${latencyPts.toFixed(1)}/5), entry slippage ${entrySlippage.toFixed(1)}bps (${slippagePts.toFixed(1)}/5)`;
  return { score, reason };
}

// ── Utilities ─────────────────────────────────────────────────────────────────

function _clamp(v: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, Number.isFinite(v) ? v : min));
}

function _grade(score: number): "A" | "B" | "C" | "D" | "F" {
  if (score >= 80) return "A";
  if (score >= 60) return "B";
  if (score >= 40) return "C";
  if (score >= 20) return "D";
  return "F";
}
