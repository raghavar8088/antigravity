/**
 * Strategy Signal Trace — types for end-to-end per-strategy observability.
 * Pure types only. No trading logic, no threshold changes, no gate bypasses.
 */

/** Every gate in the entry pipeline, in evaluation order. */
export type SignalTraceGate =
  | "DATA"
  | "NO_STRATEGIES"
  | "DISABLED"
  | "SUSPENDED"
  | "ROTATION"
  | "COOLDOWN"
  | "OCCUPIED"
  | "REGIME"
  | "SIGNAL"
  | "CONFIRM"
  | "QUALITY"
  | "MTF"
  | "ATR_FEES"
  | "SPREAD"
  | "SESSION"
  | "CATEGORY"
  | "SAME_SIDE"
  | "MARGIN"
  | "MAX_OPEN"
  | "OPENED";

/** Lifecycle status of a strategy during a single tick evaluation. */
export type SignalTraceStatus =
  | "EVALUATED"   // reached signal scoring; did not fire
  | "FIRED"       // signal threshold met; downstream gates pending
  | "CANDIDATE"   // passed all gates; queued for position open
  | "REJECTED"    // stopped at a specific gate
  | "OPENED";     // position successfully opened

/** One row per strategy per tick — records where and why a strategy stopped. */
export interface StrategySignalTraceRow {
  traceId: string;
  tickAt: number;
  mode: "browser" | "worker" | "cron";
  symbol: string;
  strategyId: number;
  strategyName: string;
  category?: string;
  side?: "LONG" | "SHORT";

  status: SignalTraceStatus;
  /** Gate at which evaluation stopped (or OPENED if successful). */
  gate: SignalTraceGate;
  /** Human-readable rejection reason, or "opened" / "candidate" on success. */
  reason: string;

  signalScore: number;
  requiredThreshold: number;
  confirmPassed: boolean;
  regime: string;
  regimeAllowed: boolean;

  atrPct?: number;
  feeHurdlePassed?: boolean;
  spreadPct?: number;
  qualityScore?: number;
  mtfScore?: number;

  candidateRank?: number;
  openedPositionId?: string;
  openAttempted?: boolean;

  /** Score contributions from the signal builder for debugging. */
  contributions?: Array<{ reason: string; pts: number }>;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

export interface SignalTraceSummary {
  totalEvaluated: number;
  fired: number;
  candidates: number;
  opened: number;
  rejectedByGate: Record<string, number>;
  topRejectedGate: string | null;
}

/**
 * Build a StrategySignalTraceRow with required defaults, so callers only
 * supply the fields that differ per evaluation step.
 */
export function createTraceRow(
  fields: Omit<StrategySignalTraceRow, "traceId"> & { traceId?: string },
): StrategySignalTraceRow {
  return {
    traceId: fields.traceId ?? `${fields.tickAt}-${fields.strategyId}-${Math.random().toString(36).slice(2, 7)}`,
    ...fields,
  };
}

/** Aggregate a tick's rows into summary counts. Pure — no side effects. */
export function summarizeSignalTrace(rows: StrategySignalTraceRow[]): SignalTraceSummary {
  const rejectedByGate: Record<string, number> = {};
  let fired = 0;
  let candidates = 0;
  let opened = 0;

  for (const row of rows) {
    if (row.status === "FIRED" || row.status === "CANDIDATE" || row.status === "OPENED") fired++;
    if (row.status === "CANDIDATE") candidates++;
    if (row.status === "OPENED") opened++;
    if (row.status === "REJECTED") {
      rejectedByGate[row.gate] = (rejectedByGate[row.gate] ?? 0) + 1;
    }
  }

  const topRejectedGate =
    Object.keys(rejectedByGate).sort((a, b) => rejectedByGate[b] - rejectedByGate[a])[0] ?? null;

  return { totalEvaluated: rows.length, fired, candidates, opened, rejectedByGate, topRejectedGate };
}

/**
 * Cap a tick's rows to max 500.
 * Retention order: OPENED first, then CANDIDATE, then highest signalScore.
 */
export function capTraceRows(rows: StrategySignalTraceRow[], max = 500): StrategySignalTraceRow[] {
  if (rows.length <= max) return rows;
  const priority = (r: StrategySignalTraceRow) =>
    r.status === "OPENED" ? 100_000 : r.status === "CANDIDATE" ? 10_000 : r.signalScore;
  return [...rows].sort((a, b) => priority(b) - priority(a)).slice(0, max);
}

export function signalTraceRatio(row: Pick<StrategySignalTraceRow, "signalScore" | "requiredThreshold">): number {
  const threshold = Number(row.requiredThreshold);
  if (!Number.isFinite(threshold) || threshold <= 0) return 0;
  const score = Number(row.signalScore);
  return Number.isFinite(score) ? score / threshold : 0;
}

export function closestSignalRows(
  rows: readonly StrategySignalTraceRow[],
  limit = 10,
): StrategySignalTraceRow[] {
  return [...rows]
    .sort((a, b) => signalTraceRatio(b) - signalTraceRatio(a))
    .slice(0, Math.max(0, limit));
}
