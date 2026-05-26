/**
 * futuresEdgeCandidates.ts — Edge Candidate Engine
 * Pure functions, no side effects, fully testable.
 *
 * Combines rotation scoring with raw diagnostic data to produce an
 * operator-readable verdict per strategy: what deserves paper capital,
 * what should sit out, and how confident we are in each verdict.
 *
 * NEVER auto-applies anything. All output is read-only recommendations.
 * NEVER uses probe/bootstrap trades (filtered by StrategyDiagnosticRow.isProbe).
 */

import type { StrategyDiagnosticRow } from "./futuresStrategyDiagnostics";
import type { RotationReport, RotationStatus } from "./futuresStrategyRotation";

// ── Types ──────────────────────────────────────────────────────────────────

export type EdgeCandidateStatus =
  | "KEEP"
  | "WATCH"
  | "DISABLE_SUGGESTED"
  | "PROMOTE_SUGGESTED"
  | "INSUFFICIENT_DATA";

export interface EdgeCandidateRow {
  strategyId: number;
  strategyName: string;
  status: EdgeCandidateStatus;
  /** 0–100: how reliable the verdict is. Grows with trade count and signal clarity. */
  confidence: number;
  trades: number;
  expectancy: number;
  winRate: number;
  profitFactor: number;
  feePctOfAbsGross: number;
  slCount: number;
  tpCount: number;
  rotationStatus: RotationStatus | null;
  rotationScore: number | null;
  lastSeenAt?: string;
  reason: string;
}

export interface NarrowRosterResult {
  /** KEEP + PROMOTE_SUGGESTED — deserving of live paper capital. */
  keepIds: number[];
  /** WATCH — not immediately removed but under review. */
  watchIds: number[];
  /** DISABLE_SUGGESTED — suggest removing from DESK_WORKER_STRATEGY_IDS. */
  disableIds: number[];
  /** Copyable env line: only keepIds in DESK_WORKER_STRATEGY_IDS. */
  envLine: string;
  summary: string;
}

// ── Constants ──────────────────────────────────────────────────────────────

/** Minimum production closes required for any verdict other than INSUFFICIENT_DATA. */
const MIN_TRADES = 5;

/** Promote suggested threshold: strong positive expectancy with low fee drag. */
const PROMOTE_MIN_TRADES = 10;
const PROMOTE_MIN_EXPECTANCY = 2;
const PROMOTE_MAX_FEE_RATIO = 0.5;

/** Disable threshold: sustained negative expectancy AND fees eat all gross. */
const DISABLE_MIN_TRADES = 5;
const DISABLE_MAX_EXPECTANCY = 0;
const DISABLE_MIN_FEE_RATIO = 1.0;

// ── Confidence scoring ────────────────────────────────────────────────────

/**
 * Confidence is 0–100 and grows logarithmically with trade count (data volume)
 * plus a signal-clarity bonus (large expectancy gap from 0, low fees = cleaner read).
 *
 * At 5 trades:    ~30–40  (border of scoreable)
 * At 10 trades:   ~40–55  (early verdict, moderate confidence)
 * At 20 trades:   ~50–65  (reasonable confidence)
 * At 50 trades:   ~65–80  (solid confidence)
 * At 100+ trades: ~80–100 (high confidence)
 */
function computeConfidence(
  trades: number,
  expectancy: number,
  feePctOfAbsGross: number,
): number {
  // Data volume: 0–70 (logarithmic — each additional trade matters less)
  const dataConf = Math.min(
    (Math.log(Math.max(trades, 1) + 1) / Math.log(101)) * 70,
    70,
  );

  // Signal strength: how far expectancy is from 0 (positive or negative both give signal)
  const expStrength = Math.min((Math.abs(expectancy) / 10) * 15, 15);

  // Fee clarity: low fee/gross means the gross signal is real
  const feeClarity = Math.max(0, (1 - Math.min(feePctOfAbsGross, 2) / 2) * 15);

  return Math.round(Math.min(dataConf + expStrength + feeClarity, 100));
}

// ── Status derivation ─────────────────────────────────────────────────────

function deriveEdgeStatus(
  row: StrategyDiagnosticRow,
  rotationStatus: RotationStatus | null,
): { status: EdgeCandidateStatus; reason: string } {
  // Always INSUFFICIENT when under threshold — no verdict possible
  if (row.totalTrades < MIN_TRADES) {
    return {
      status: "INSUFFICIENT_DATA",
      reason: `Only ${row.totalTrades}/${MIN_TRADES} production closes — collect more data before judging.`,
    };
  }

  // PROMOTE_SUGGESTED: rotation already scored it highly AND diagnostics confirm strong edge
  if (
    (rotationStatus === "PROMOTED" || rotationStatus === "ACTIVE") &&
    row.totalTrades >= PROMOTE_MIN_TRADES &&
    row.avgNetPnl >= PROMOTE_MIN_EXPECTANCY &&
    row.feePctOfAbsGross <= PROMOTE_MAX_FEE_RATIO
  ) {
    const feePct = (row.feePctOfAbsGross * 100).toFixed(0);
    return {
      status: "PROMOTE_SUGGESTED",
      reason: `${row.totalTrades}t · E $${row.avgNetPnl.toFixed(2)} · WR ${(row.winRate * 100).toFixed(0)}% · fee/gross ${feePct}% — strong after-fee edge confirmed.`,
    };
  }

  // DISABLE_SUGGESTED: rotation SUSPENDED or diagnostics confirm sustained losses
  if (rotationStatus === "SUSPENDED") {
    return {
      status: "DISABLE_SUGGESTED",
      reason: `Rotation SUSPENDED (score < 25). ${row.totalTrades}t · E $${row.avgNetPnl.toFixed(2)} · fee/gross ${(row.feePctOfAbsGross * 100).toFixed(0)}%.`,
    };
  }

  if (
    row.totalTrades >= DISABLE_MIN_TRADES &&
    row.avgNetPnl <= DISABLE_MAX_EXPECTANCY &&
    row.feePctOfAbsGross >= DISABLE_MIN_FEE_RATIO
  ) {
    const feePct = (row.feePctOfAbsGross * 100).toFixed(0);
    return {
      status: "DISABLE_SUGGESTED",
      reason: `${row.totalTrades}t · E $${row.avgNetPnl.toFixed(2)} (≤$0) · fee/gross ${feePct}% (≥100%) — fees consume all gross profit.`,
    };
  }

  // WATCH: on probation, or marginal positive, or negative but not yet disqualified
  if (
    rotationStatus === "PROBATION" ||
    (row.avgNetPnl < 0 && row.totalTrades >= MIN_TRADES)
  ) {
    return {
      status: "WATCH",
      reason: `${rotationStatus === "PROBATION" ? "Rotation PROBATION. " : ""}${row.totalTrades}t · E $${row.avgNetPnl.toFixed(2)} · WR ${(row.winRate * 100).toFixed(0)}% — monitor for improvement.`,
    };
  }

  // KEEP: active in rotation, positive expectancy, acceptable fees
  const feePct = (row.feePctOfAbsGross * 100).toFixed(0);
  return {
    status: "KEEP",
    reason: `${row.totalTrades}t · E $${row.avgNetPnl.toFixed(2)} · WR ${(row.winRate * 100).toFixed(0)}% · fee/gross ${feePct}% — keep running.`,
  };
}

// ── Main export ───────────────────────────────────────────────────────────

/**
 * Compute edge candidates for all strategies that have production closes.
 *
 * @param diagnosticRows  Per-strategy aggregated performance (probe rows already filtered inside)
 * @param rotationReport  Optional rotation report to cross-reference rotation scores
 */
export function computeEdgeCandidates(
  diagnosticRows: StrategyDiagnosticRow[],
  rotationReport: RotationReport | null,
): EdgeCandidateRow[] {
  const prodRows = diagnosticRows.filter((r) => !r.isProbe);
  if (prodRows.length === 0) return [];

  const rotationByStratId = new Map<
    number,
    { status: RotationStatus; score: number }
  >();
  if (rotationReport) {
    for (const s of rotationReport.scores) {
      rotationByStratId.set(s.strategyId, { status: s.status, score: s.score });
    }
  }

  return prodRows.map((row) => {
    const rot = rotationByStratId.get(row.strategyId) ?? null;
    const { status, reason } = deriveEdgeStatus(row, rot?.status ?? null);
    const confidence = computeConfidence(
      row.totalTrades,
      row.avgNetPnl,
      row.feePctOfAbsGross,
    );

    return {
      strategyId: row.strategyId,
      strategyName: row.strategyName,
      status,
      confidence,
      trades: row.totalTrades,
      expectancy: row.avgNetPnl,
      winRate: row.winRate,
      profitFactor: row.profitFactor,
      feePctOfAbsGross: row.feePctOfAbsGross,
      slCount: row.slCount,
      tpCount: row.tpCount,
      rotationStatus: rot?.status ?? null,
      rotationScore: rot?.score ?? null,
      lastSeenAt: row.lastTradeAt ?? undefined,
      reason,
    };
  });
}

// ── Roster narrowing ──────────────────────────────────────────────────────

/**
 * Given edge candidates, produce a narrow roster recommendation.
 * keepIds = strategies worth running (KEEP + PROMOTE_SUGGESTED).
 * disableIds = strategies to remove from DESK_WORKER_STRATEGY_IDS.
 * watchIds = borderline (WATCH) — keep for now but flag.
 *
 * Returns a copyable env line for the operator to apply manually.
 */
export function computeNarrowRoster(
  candidates: EdgeCandidateRow[],
  currentEnabledIds: readonly number[] = [],
): NarrowRosterResult {
  const keepSet = candidates.filter(
    (c) => c.status === "KEEP" || c.status === "PROMOTE_SUGGESTED",
  );
  const watchSet = candidates.filter((c) => c.status === "WATCH");
  const disableSet = candidates.filter((c) => c.status === "DISABLE_SUGGESTED");
  const insufficientSet = candidates.filter(
    (c) => c.status === "INSUFFICIENT_DATA",
  );

  const keepIds = keepSet.map((c) => c.strategyId).sort((a, b) => a - b);
  const watchIds = watchSet.map((c) => c.strategyId).sort((a, b) => a - b);
  const disableIds = disableSet.map((c) => c.strategyId).sort((a, b) => a - b);

  // Env line: all keepIds + watchIds (keep WATCH running for data), minus explicit disables
  const disableSet_ = new Set(disableIds);
  const rosterIds =
    currentEnabledIds.length > 0
      ? [...currentEnabledIds].filter((id) => !disableSet_.has(id))
      : [...keepIds, ...watchIds];
  const rosterSorted = [...new Set(rosterIds)].sort((a, b) => a - b);
  const envLine =
    rosterSorted.length > 0
      ? `DESK_WORKER_STRATEGY_IDS=${rosterSorted.join(",")}`
      : "# No strategies qualify yet — collect more data";

  const summary = [
    `${keepIds.length} KEEP`,
    `${watchIds.length} WATCH`,
    `${disableIds.length} DISABLE_SUGGESTED`,
    `${insufficientSet.length} INSUFFICIENT_DATA`,
  ].join(" · ");

  return { keepIds, watchIds, disableIds, envLine, summary };
}

// ── Status display helpers ────────────────────────────────────────────────

export const EDGE_STATUS_COLOR: Record<EdgeCandidateStatus, string> = {
  PROMOTE_SUGGESTED: "#3fb950",
  KEEP: "#58a6ff",
  WATCH: "#d29922",
  DISABLE_SUGGESTED: "#f85149",
  INSUFFICIENT_DATA: "#8b949e",
};

export const EDGE_STATUS_LABEL: Record<EdgeCandidateStatus, string> = {
  PROMOTE_SUGGESTED: "Promote",
  KEEP: "Keep",
  WATCH: "Watch",
  DISABLE_SUGGESTED: "Disable?",
  INSUFFICIENT_DATA: "Pending",
};
