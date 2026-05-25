/**
 * Strategy safety suggestions: reads rotation report + strategy diagnostics and
 * surfaces operator suggestions — never auto-applies anything.
 *
 * Rules:
 *   Suggest disabling when ALL of:
 *     - strategy has >= 5 production closes (rotation has real signal)
 *     - avgNetPnl (expectancy) < 0
 *     - feePctOfAbsGross > 1.0 (fees exceed gross, i.e. fee/gross > 100%)
 *   OR:
 *     - rotation status === "SUSPENDED"
 *
 *   Never suggest disabling INSUFFICIENT strategies (< 5 trades — no signal yet).
 *   Never use probe/bootstrap trades in the computation (already excluded by diagnostics).
 */

import type { StrategyDiagnosticRow, DiagnosticSummary } from "./futuresStrategyDiagnostics";
import type { RotationReport } from "./futuresStrategyRotation";

export interface StrategySafetySuggestions {
  /** IDs the operator should consider disabling. */
  autoDisableCandidates: number[];
  /** IDs that look healthy and should stay enabled. */
  keepEnabledCandidates: number[];
  /** Human-readable reason keyed by strategy ID. */
  reasonByStrategyId: Record<number, string>;
}

export function computeStrategySafety(input: {
  rotationReport: RotationReport | null;
  diagnostics: DiagnosticSummary | null;
  enabledStrategyIds?: readonly number[];
}): StrategySafetySuggestions {
  const { rotationReport, diagnostics } = input;
  const reasonByStrategyId: Record<number, string> = {};
  const disableSet = new Set<number>();
  const keepSet = new Set<number>();

  // Build a lookup from rotation report
  const rotationByStratId = new Map<number, RotationReport["scores"][number]>();
  if (rotationReport) {
    for (const score of rotationReport.scores) {
      rotationByStratId.set(score.strategyId, score);
    }
  }

  // Build lookup from diagnostics
  const diagByStratId = new Map<number, StrategyDiagnosticRow>();
  if (diagnostics) {
    for (const row of diagnostics.rows) {
      if (!row.isProbe) {
        diagByStratId.set(row.strategyId, row);
      }
    }
  }

  // All strategy IDs we know about (union of rotation + diagnostics)
  const allIds = new Set<number>([
    ...rotationByStratId.keys(),
    ...diagByStratId.keys(),
  ]);

  for (const id of allIds) {
    const rotEntry = rotationByStratId.get(id);
    const diagRow = diagByStratId.get(id);

    // Skip if insufficient data from rotation perspective (< 5 trades → no signal)
    if (rotEntry?.status === "INSUFFICIENT") {
      reasonByStrategyId[id] = `Insufficient data (${diagRow?.totalTrades ?? 0} closes) — keep running to collect signal`;
      keepSet.add(id);
      continue;
    }

    // Suggest disable: rotation SUSPENDED
    if (rotEntry?.status === "SUSPENDED") {
      reasonByStrategyId[id] = `Rotation score ${rotEntry.score.toFixed(0)}/100 → SUSPENDED (score < 25). Consider disabling.`;
      disableSet.add(id);
      continue;
    }

    // Suggest disable: production diagnostics show consistently losing + fee-heavy
    if (diagRow && diagRow.totalTrades >= 5) {
      const feeExceedsGross = diagRow.feePctOfAbsGross > 1.0;
      const negativeExpectancy = diagRow.avgNetPnl < 0;
      if (feeExceedsGross && negativeExpectancy) {
        const feePct = (diagRow.feePctOfAbsGross * 100).toFixed(0);
        reasonByStrategyId[id] = `${diagRow.totalTrades} closes · expectancy $${diagRow.avgNetPnl.toFixed(2)} · fee/gross ${feePct}% (>100%) → suggest disable`;
        disableSet.add(id);
        continue;
      }
    }

    // Healthy strategies
    if (
      rotEntry?.status === "ACTIVE" ||
      rotEntry?.status === "PROMOTED"
    ) {
      const status = rotEntry.status;
      const score = rotEntry.score.toFixed(0);
      reasonByStrategyId[id] = `${status} (score ${score}/100) — keep enabled`;
      keepSet.add(id);
    }
  }

  return {
    autoDisableCandidates: [...disableSet].sort((a, b) => a - b),
    keepEnabledCandidates: [...keepSet].sort((a, b) => a - b),
    reasonByStrategyId,
  };
}

/**
 * Format the disable candidates as a DESK_WORKER_STRATEGY_IDS env line,
 * excluding the candidates from whatever is currently enabled.
 * Returns null when there's nothing to exclude.
 */
export function formatDisableListAsEnv(
  enabledIds: readonly number[],
  disableCandidates: number[],
): string | null {
  const disableSet = new Set(disableCandidates);
  const remaining = enabledIds.filter((id) => !disableSet.has(id));
  if (remaining.length === enabledIds.length) return null;
  return `DESK_WORKER_STRATEGY_IDS=${remaining.join(",")}`;
}
