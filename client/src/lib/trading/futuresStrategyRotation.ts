/**
 * futuresStrategyRotation.ts
 * Pure functions. No side effects. Fully testable.
 *
 * Scores each strategy based on recent performance and
 * returns a ranked active roster. Never mutates state.
 * Rotation decisions are recommendations only.
 */

import type { StrategyDiagnosticRow } from "./futuresStrategyDiagnostics";

export type RotationStatus =
  | "ACTIVE"
  | "PROBATION"
  | "SUSPENDED"
  | "PROMOTED"
  | "INSUFFICIENT";

export interface StrategyRotationScore {
  strategyId: number;
  strategyName: string;
  status: RotationStatus;
  score: number;
  rank: number;
  winRateScore: number;
  expectancyScore: number;
  feeScore: number;
  consistencyScore: number;
  tradesAnalyzed: number;
  lastUpdated: number;
  reasoning: string;
}

export interface RotationReport {
  scores: StrategyRotationScore[];
  active: StrategyRotationScore[];
  promoted: StrategyRotationScore[];
  probation: StrategyRotationScore[];
  suspended: StrategyRotationScore[];
  insufficient: StrategyRotationScore[];
  topStrategyId: number | null;
  computedAt: number;
}

const MIN_TRADES_TO_SCORE = 5;

function scoreWinRate(winRate: number): number {
  if (winRate >= 0.7) return 25;
  if (winRate >= 0.5) return 20;
  if (winRate >= 0.4) return 15;
  if (winRate >= 0.3) return 10;
  if (winRate >= 0.2) return 5;
  return 0;
}

function scoreExpectancy(avgNetPnl: number): number {
  if (avgNetPnl >= 10) return 25;
  if (avgNetPnl >= 5) return 20;
  if (avgNetPnl >= 2) return 15;
  if (avgNetPnl >= 0) return 10;
  if (avgNetPnl >= -5) return 5;
  if (avgNetPnl >= -10) return 2;
  return 0;
}

function scoreFeeRatio(feePctOfAbsGross: number): number {
  if (feePctOfAbsGross <= 0.2) return 25;
  if (feePctOfAbsGross <= 0.3) return 20;
  if (feePctOfAbsGross <= 0.5) return 15;
  if (feePctOfAbsGross <= 0.75) return 8;
  if (feePctOfAbsGross <= 1.0) return 3;
  return 0;
}

function scoreConsistency(row: StrategyDiagnosticRow): number {
  let pts = 0;

  if (row.profitFactor >= 2.0) pts += 10;
  else if (row.profitFactor >= 1.5) pts += 8;
  else if (row.profitFactor >= 1.0) pts += 5;
  else if (row.profitFactor > 0) pts += 2;

  const slRate = row.slCount / Math.max(row.totalTrades, 1);
  if (slRate <= 0.3) pts += 10;
  else if (slRate <= 0.5) pts += 7;
  else if (slRate <= 0.7) pts += 3;

  const tpRate = row.tpCount / Math.max(row.totalTrades, 1);
  if (tpRate >= 0.4) pts += 5;
  else if (tpRate >= 0.2) pts += 3;

  return Math.min(pts, 25);
}

function deriveStatus(score: number, row: StrategyDiagnosticRow): RotationStatus {
  if (row.totalTrades < MIN_TRADES_TO_SCORE) return "INSUFFICIENT";
  if (score >= 75 && row.avgNetPnl > 0) return "PROMOTED";
  if (score >= 45) return "ACTIVE";
  if (score >= 25) return "PROBATION";
  return "SUSPENDED";
}

function buildReasoning(
  row: StrategyDiagnosticRow,
  score: number,
  status: RotationStatus,
): string {
  const parts: string[] = [];

  parts.push(`Score: ${score}/100`);
  parts.push(`Status: ${status}`);
  parts.push(`WR: ${(row.winRate * 100).toFixed(0)}%`);
  parts.push(`E: $${row.avgNetPnl.toFixed(2)}/trade`);
  parts.push(`fee/gross: ${(row.feePctOfAbsGross * 100).toFixed(0)}%`);
  parts.push(
    `PF: ${row.profitFactor === Infinity ? "∞" : row.profitFactor.toFixed(2)}`,
  );
  parts.push(`SL:${row.slCount} TP:${row.tpCount} (${row.totalTrades}t)`);

  return parts.join(" | ");
}

export function computeStrategyRotation(
  diagnosticRows: StrategyDiagnosticRow[],
): RotationReport {
  const prodRows = diagnosticRows.filter((r) => !r.isProbe);

  const scores: StrategyRotationScore[] = prodRows.map((row) => {
    if (row.totalTrades < MIN_TRADES_TO_SCORE) {
      return {
        strategyId: row.strategyId,
        strategyName: row.strategyName,
        status: "INSUFFICIENT" as RotationStatus,
        score: 0,
        rank: 999,
        winRateScore: 0,
        expectancyScore: 0,
        feeScore: 0,
        consistencyScore: 0,
        tradesAnalyzed: row.totalTrades,
        lastUpdated: Date.now(),
        reasoning: `Insufficient data: ${row.totalTrades}/${MIN_TRADES_TO_SCORE} trades`,
      };
    }

    const winRateScore = scoreWinRate(row.winRate);
    const expectancyScore = scoreExpectancy(row.avgNetPnl);
    const feeScore = scoreFeeRatio(row.feePctOfAbsGross);
    const consistencyScore = scoreConsistency(row);
    const score = winRateScore + expectancyScore + feeScore + consistencyScore;
    const status = deriveStatus(score, row);

    return {
      strategyId: row.strategyId,
      strategyName: row.strategyName,
      status,
      score,
      rank: 0,
      winRateScore,
      expectancyScore,
      feeScore,
      consistencyScore,
      tradesAnalyzed: row.totalTrades,
      lastUpdated: Date.now(),
      reasoning: buildReasoning(row, score, status),
    };
  });

  const ranked = [...scores]
    .sort((a, b) => b.score - a.score)
    .map((s, i) => ({ ...s, rank: i + 1 }));

  const active = ranked.filter((s) => s.status === "ACTIVE");
  const promoted = ranked.filter((s) => s.status === "PROMOTED");
  const probation = ranked.filter((s) => s.status === "PROBATION");
  const suspended = ranked.filter((s) => s.status === "SUSPENDED");
  const insufficient = ranked.filter((s) => s.status === "INSUFFICIENT");

  const topScore = [...promoted, ...active][0] ?? null;

  return {
    scores: ranked,
    active,
    promoted,
    probation,
    suspended,
    insufficient,
    topStrategyId: topScore?.strategyId ?? null,
    computedAt: Date.now(),
  };
}

/**
 * Returns strategy IDs that should be skipped this cycle.
 * Called in entry gate — suspended strategies are blocked.
 * Does NOT auto-block probation strategies.
 */
export function getSuspendedStrategyIds(report: RotationReport): Set<number> {
  return new Set(report.suspended.map((s) => s.strategyId));
}
