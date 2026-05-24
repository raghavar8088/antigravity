/**
 * futuresWinnersRefresh.ts
 * Suggested promote/demote lists from rolling diagnostics — read-only.
 */

import type { StrategyDiagnosticRow } from "./futuresStrategyDiagnostics";

export interface WinnersRefreshSuggestion {
  promote: number[];
  demote: number[];
  rationale: string;
  /** Copy-paste env for manual roster refresh (never auto-applied). */
  strategyIdsEnvLine: string;
}

export function suggestWinnersFromScorecard(
  diagnosticRows: StrategyDiagnosticRow[],
  minTradesPromote = 8,
  minTradesDemote = 5,
): WinnersRefreshSuggestion {
  const rows = diagnosticRows.filter((r) => !r.isProbe && r.totalTrades > 0);

  const promoteCandidates = rows
    .filter(
      (r) =>
        r.totalTrades >= minTradesPromote &&
        r.avgNetPnl > 0 &&
        r.feePctOfAbsGross < 0.5,
    )
    .sort((a, b) => b.avgNetPnl - a.avgNetPnl)
    .slice(0, 5);

  const demoteCandidates = rows
    .filter((r) => r.totalTrades >= minTradesDemote && r.avgNetPnl < -5)
    .sort((a, b) => a.avgNetPnl - b.avgNetPnl)
    .slice(0, 5);

  const promote = promoteCandidates.map((r) => r.strategyId);
  const demote = demoteCandidates.map((r) => r.strategyId);

  const rationale =
    promote.length || demote.length
      ? `Promote ${promote.length} (E>0, fee<50%, ≥${minTradesPromote}t). ` +
        `Demote ${demote.length} (E<-5, ≥${minTradesDemote}t). Manual only.`
      : `Need more per-strategy trades (promote ≥${minTradesPromote}, demote ≥${minTradesDemote}).`;

  const rosterIds = [...new Set([...promote, ...rows.map((r) => r.strategyId)])].filter(
    (id) => !demote.includes(id),
  );

  const strategyIdsEnvLine = rosterIds.length
    ? `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=${rosterIds.join(",")}`
    : "# Insufficient data for roster env line";

  return {
    promote,
    demote,
    rationale,
    strategyIdsEnvLine,
  };
}
