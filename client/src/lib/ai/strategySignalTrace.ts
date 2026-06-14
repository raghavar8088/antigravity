/**
 * Strategy signal trace types and helpers.
 *
 * A "trace row" records what happened to each strategy evaluated during a
 * signal tick: whether it became a CANDIDATE, was REJECTED, EVALUATED
 * (scored but below threshold), or FIRED (scored but failed confirmation).
 */

export type TraceStatus = "CANDIDATE" | "REJECTED" | "EVALUATED" | "FIRED";

export interface StrategySignalTraceRow {
  traceId: string;
  tickAt: number;
  mode: string;
  symbol: string;
  strategyId: number;
  strategyName: string;
  category?: string;
  side?: "LONG" | "SHORT";
  status: TraceStatus;
  gate: string;
  reason: string;
  signalScore: number;
  requiredThreshold: number;
  confirmPassed: boolean;
  feeHurdlePassed?: boolean;
  openAttempted?: boolean;
  regime: string;
  regimeAllowed: boolean;
  atrPct?: number;
  contributions?: Record<string, number>;
}

export interface SignalTraceSummary {
  tickAt: number;
  totalEvaluated: number;
  candidateCount: number;
  rejectedCount: number;
  topGates: { gate: string; count: number }[];
}

/** Constructs a fully-typed trace row (identity helper for type safety). */
export function createTraceRow(args: StrategySignalTraceRow): StrategySignalTraceRow {
  return args;
}

/**
 * Slices trace rows to `limit`, prioritising CANDIDATE rows so they are never
 * truncated before non-candidate rows.
 */
export function capTraceRows(
  rows: StrategySignalTraceRow[],
  limit = 500,
): StrategySignalTraceRow[] {
  if (rows.length <= limit) return rows;
  const candidates = rows.filter((r) => r.status === "CANDIDATE");
  const rest = rows.filter((r) => r.status !== "CANDIDATE");
  return [...candidates, ...rest].slice(0, limit);
}

/** Summarises a trace row array into counts and top rejection gates. */
export function summarizeSignalTrace(rows: StrategySignalTraceRow[]): SignalTraceSummary {
  const candidates = rows.filter((r) => r.status === "CANDIDATE");
  const rejected = rows.filter((r) => r.status !== "CANDIDATE");
  const gateCounts = new Map<string, number>();
  for (const r of rejected) {
    gateCounts.set(r.gate, (gateCounts.get(r.gate) ?? 0) + 1);
  }
  const topGates = [...gateCounts.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([gate, count]) => ({ gate, count }));
  return {
    tickAt: rows[0]?.tickAt ?? Date.now(),
    totalEvaluated: rows.length,
    candidateCount: candidates.length,
    rejectedCount: rejected.length,
    topGates,
  };
}
