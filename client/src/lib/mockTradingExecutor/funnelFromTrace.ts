import type { StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";
import {
  buildFunnelSnapshot,
  emptyBlockerCounts,
  type EntryFunnelBlockerCounts,
} from "@/lib/trading/deskEntryFunnelSnapshot";

const GATE_TO_BLOCKER: Record<string, keyof EntryFunnelBlockerCounts> = {
  SIGNAL: "signal",
  CONFIRM: "confirm",
  REGIME: "regime",
  ATR_FEES: "atrFees",
  ROTATION: "rotation",
  SUSPENDED: "suspended",
  SPREAD: "spread",
  SESSION: "session",
  CATEGORY: "category",
  SAME_SIDE: "sameSide",
  MARGIN: "margin",
  MAX_OPEN: "maxOpen",
  COOLDOWN: "cooldown",
  NO_STRATEGIES: "noStrategies",
  DATA: "noData",
};

function blockerCountsFromTraceRows(rows: StrategySignalTraceRow[]): EntryFunnelBlockerCounts {
  const counts = emptyBlockerCounts();
  for (const row of rows) {
    if (row.status === "CANDIDATE" || row.status === "OPENED") continue;
    const key = GATE_TO_BLOCKER[String(row.gate).toUpperCase()];
    if (!key) continue;
    counts[key] += 1;
  }
  return counts;
}

export function funnelSnapshotFromTraceEval(args: {
  tickAt: number;
  symbol: string;
  markPrice: number;
  bars: number;
  activeStrategies: number;
  evaluatedStrategies: number;
  rows: StrategySignalTraceRow[];
  candidateCount: number;
  opened: number;
  workerFresh: boolean;
}) {
  const counts = blockerCountsFromTraceRows(args.rows);
  if (args.bars === 0) counts.noData = Math.max(counts.noData, 1);
  if (args.activeStrategies === 0) counts.noStrategies = Math.max(counts.noStrategies, 1);

  return buildFunnelSnapshot({
    tickAt: args.tickAt,
    workerMode: "worker",
    workerFresh: args.workerFresh,
    symbol: args.symbol,
    markPrice: args.markPrice,
    bars: args.bars,
    activeStrategies: args.activeStrategies,
    evaluatedStrategies: args.evaluatedStrategies,
    signalPassed: args.candidateCount + args.opened,
    confirmPassed: args.candidateCount + args.opened,
    candidateCount: args.candidateCount,
    openAttempts: args.candidateCount,
    opened: args.opened,
    blockerCounts: counts,
  });
}
