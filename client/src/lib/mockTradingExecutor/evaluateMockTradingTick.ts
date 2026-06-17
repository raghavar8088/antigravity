/**
 * Shared signal evaluation for browser signal-tick and backend executor.
 * Keeps strategy logic in mockTradingSignalEvaluator — this module only
 * applies Trade Engine catalog fan-out and trace caps.
 */

import { capTraceRows, summarizeSignalTrace, type StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";
import { fanOutGrade5CatalogSignals } from "@/lib/strategyAuthority/grade5CatalogSignals";
import { STRATEGY_CATALOG } from "@/lib/strategyAuthority/strategyCatalog";
import {
  evaluateMockTradingSignals,
  type MockTradingSignalEvalResult,
} from "@/lib/trading/mockTradingSignalEvaluator";
import type { MockTradingBar } from "@/lib/trading/mockTradingMarketData";

export type MockTradingTickEval = MockTradingSignalEvalResult & {
  rows: StrategySignalTraceRow[];
  summary: ReturnType<typeof summarizeSignalTrace>;
  candidateCount: number;
};

export function buildMockTradingTraceRows(
  baseResult: MockTradingSignalEvalResult,
): StrategySignalTraceRow[] {
  return capTraceRows(
    (STRATEGY_CATALOG.length > 0
      ? fanOutGrade5CatalogSignals({
          catalog: STRATEGY_CATALOG,
          baseRows: baseResult.rows,
          tickAt: baseResult.tickAt,
          symbol: baseResult.symbol,
          regime: baseResult.regime,
        })
      : baseResult.rows
    ).map((row) => ({
      ...row,
      traceId: row.traceId.replace(/^grade5-/, "trade-engine-"),
      pipelineStage: "MAIN_ENGINE",
      mode: "worker" as const,
    })),
    500,
  );
}

export function evaluateMockTradingTick(args: {
  bars: readonly MockTradingBar[];
  markPrice: number;
  symbol: string;
  tickAt?: number;
  signalThreshold?: number;
}): MockTradingTickEval {
  const baseResult = evaluateMockTradingSignals(args);
  const rows = buildMockTradingTraceRows(baseResult);
  const summary = summarizeSignalTrace(rows);
  const candidateCount = rows.filter((row) => row.status === "CANDIDATE").length;
  return {
    ...baseResult,
    rows,
    summary,
    candidateCount,
  };
}
