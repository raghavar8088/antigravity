import type { EntryFunnelSnapshot } from "@/lib/trading/deskEntryFunnelSnapshot";
import type { SignalTraceSummary, StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";
import type { RotationReport } from "@/lib/trading/futuresStrategyRotation";

export type NoTradeRootCause =
  | "WORKER_STALE"
  | "NO_DATA"
  | "EMPTY_ROSTER"
  | "INVALID_ROSTER_IDS"
  | "SIGNAL_NOT_FIRING"
  | "CONFIRM_BLOCKING"
  | "REGIME_BLOCKING"
  | "ATR_FEE_BLOCKING"
  | "ROTATION_BLOCKING"
  | "STATE_DIRTY"
  | "MARGIN_OR_CAP_BLOCKING"
  | "UNKNOWN";

export interface NoTradeRootCauseResult {
  rootCause: NoTradeRootCause;
  evidence: string[];
  safeFix: string;
  canOpenIfSignalQualifies: boolean;
}

export type NoTradeWorkerHealth = {
  stale?: boolean;
  workerLastPollAt?: number | null;
  ageSeconds?: number | null;
};

export type NoTradePaperState = {
  balance?: number | null;
  positions?: unknown[] | null;
  pause_entries?: boolean | null;
  disabled_strategies?: unknown[] | null;
  cleared_at?: number | null;
  balanceDriftUsd?: number | null;
  probeDominant?: boolean | null;
};

export type NoTradeRootCauseInput = {
  funnel?: EntryFunnelSnapshot | null;
  signalTrace?: {
    summary?: SignalTraceSummary | null;
    rows?: StrategySignalTraceRow[] | null;
    ageSeconds?: number | null;
  } | null;
  workerHealth?: NoTradeWorkerHealth | null;
  paperState?: NoTradePaperState | null;
  rotationReport?: RotationReport | null;
};

const SAFE_FIX: Record<NoTradeRootCause, string> = {
  WORKER_STALE: "Restart AWS worker: pm2 restart btc-ft-worker. Browser fallback should only write after the worker is stale.",
  NO_DATA: "Check Delta kline/mark data and worker network; do not force trades without valid bars.",
  EMPTY_ROSTER: "Set a non-empty BTC futures paper roster or remove the explicit worker roster override.",
  INVALID_ROSTER_IDS: "Fix DESK_WORKER_STRATEGY_IDS, or keep DESK_WORKER_ROSTER_FALLBACK=core for paper worker fallback.",
  SIGNAL_NOT_FIRING: "Correct no-trade if scores are below threshold. Use Closest Signals; do not lower threshold.",
  CONFIRM_BLOCKING: "Signal fired but confirmation failed. Inspect confirmation reason; wait for momentum/HTF/volume alignment.",
  REGIME_BLOCKING: "Current roster is incompatible with regime. For chop, use the suggested chop-compatible paper roster.",
  ATR_FEE_BLOCKING: "Correct no-trade. Expected move is too small vs fees; do not bypass ATR/fee gate.",
  ROTATION_BLOCKING: "Only SUSPENDED strategies should block. Restore/suspend list or wait for non-suspended strategies.",
  STATE_DIRTY: "Repair state before judging entries. Historical trades stay; current session resets via cleared_at.",
  MARGIN_OR_CAP_BLOCKING: "Reduce open exposure or wait for positions to close; margin and cap gates are working.",
  UNKNOWN: "Check worker logs, funnel snapshot, signal trace, and paper_state for stale or missing diagnostics.",
};

function e(lines: string[], value: string | false | null | undefined) {
  if (value) lines.push(value);
}

function gateCount(funnel: EntryFunnelSnapshot | null | undefined, key: keyof EntryFunnelSnapshot["blockerCounts"]): number {
  return Number(funnel?.blockerCounts?.[key] ?? 0);
}

function topTraceGate(summary: SignalTraceSummary | null | undefined): string | null {
  return summary?.topRejectedGate ?? null;
}

export function diagnoseNoTradeRootCause(input: NoTradeRootCauseInput): NoTradeRootCauseResult {
  const evidence: string[] = [];
  const funnel = input.funnel ?? null;
  const summary = input.signalTrace?.summary ?? null;
  const rows = input.signalTrace?.rows ?? [];

  const workerAge =
    input.workerHealth?.ageSeconds ??
    (typeof input.workerHealth?.workerLastPollAt === "number"
      ? Math.floor((Date.now() - input.workerHealth.workerLastPollAt) / 1000)
      : null);
  if (input.workerHealth?.stale || (workerAge !== null && workerAge > 60)) {
    e(evidence, workerAge !== null ? `worker heartbeat age ${workerAge}s` : "worker heartbeat missing");
    return {
      rootCause: "WORKER_STALE",
      evidence,
      safeFix: SAFE_FIX.WORKER_STALE,
      canOpenIfSignalQualifies: false,
    };
  }

  const dirty =
    input.paperState?.probeDominant === true ||
    Math.abs(Number(input.paperState?.balanceDriftUsd ?? 0)) > 1;
  if (dirty) {
    e(evidence, input.paperState?.probeDominant ? "recent history is probe/bootstrap dominant" : null);
    e(evidence, `balance drift $${Number(input.paperState?.balanceDriftUsd ?? 0).toFixed(2)}`);
    return {
      rootCause: "STATE_DIRTY",
      evidence,
      safeFix: SAFE_FIX.STATE_DIRTY,
      canOpenIfSignalQualifies: false,
    };
  }

  if (!funnel) {
    e(evidence, "no entry funnel snapshot");
    return { rootCause: "UNKNOWN", evidence, safeFix: SAFE_FIX.UNKNOWN, canOpenIfSignalQualifies: false };
  }

  e(evidence, `funnel blocker=${funnel.dominantBlocker}`);
  e(evidence, `active=${funnel.activeStrategies} evaluated=${funnel.evaluatedStrategies} signal=${funnel.signalPassed} candidates=${funnel.candidateCount} opened=${funnel.opened}`);
  if (summary) {
    e(evidence, `trace evaluated=${summary.totalEvaluated} fired=${summary.fired} candidates=${summary.candidates} opened=${summary.opened} topReject=${topTraceGate(summary) ?? "none"}`);
  }

  if (gateCount(funnel, "noData") > 0 || funnel.bars === 0) {
    return { rootCause: "NO_DATA", evidence, safeFix: SAFE_FIX.NO_DATA, canOpenIfSignalQualifies: false };
  }

  if (funnel.activeStrategies === 0) {
    const hasInvalid = gateCount(funnel, "noStrategies") > 1 || rows.some((r) => r.gate === "DATA" && /invalid|unknown|no_strateg/i.test(r.reason));
    return {
      rootCause: hasInvalid ? "INVALID_ROSTER_IDS" : "EMPTY_ROSTER",
      evidence,
      safeFix: hasInvalid ? SAFE_FIX.INVALID_ROSTER_IDS : SAFE_FIX.EMPTY_ROSTER,
      canOpenIfSignalQualifies: false,
    };
  }

  const suspended = input.rotationReport?.suspended?.length ?? 0;
  const rotationBlocks = gateCount(funnel, "rotation") + gateCount(funnel, "suspended");
  if (rotationBlocks > 0 || (topTraceGate(summary) === "ROTATION" || topTraceGate(summary) === "SUSPENDED")) {
    e(evidence, `rotation blocks=${rotationBlocks}, suspended=${suspended}`);
    return { rootCause: "ROTATION_BLOCKING", evidence, safeFix: SAFE_FIX.ROTATION_BLOCKING, canOpenIfSignalQualifies: suspended === 0 };
  }

  if (gateCount(funnel, "maxOpen") > 0 || gateCount(funnel, "margin") > 0 || gateCount(funnel, "category") > 0 || gateCount(funnel, "sameSide") > 0) {
    e(evidence, `maxOpen=${gateCount(funnel, "maxOpen")} margin=${gateCount(funnel, "margin")} category=${gateCount(funnel, "category")} sameSide=${gateCount(funnel, "sameSide")}`);
    return { rootCause: "MARGIN_OR_CAP_BLOCKING", evidence, safeFix: SAFE_FIX.MARGIN_OR_CAP_BLOCKING, canOpenIfSignalQualifies: false };
  }

  if (gateCount(funnel, "regime") > 0 || topTraceGate(summary) === "REGIME") {
    e(evidence, `regime skips=${gateCount(funnel, "regime")}`);
    return { rootCause: "REGIME_BLOCKING", evidence, safeFix: SAFE_FIX.REGIME_BLOCKING, canOpenIfSignalQualifies: false };
  }

  if (gateCount(funnel, "atrFees") > 0 || topTraceGate(summary) === "ATR_FEES") {
    e(evidence, `atr/fee skips=${gateCount(funnel, "atrFees")}`);
    return { rootCause: "ATR_FEE_BLOCKING", evidence, safeFix: SAFE_FIX.ATR_FEE_BLOCKING, canOpenIfSignalQualifies: false };
  }

  if (gateCount(funnel, "confirm") > 0 || topTraceGate(summary) === "CONFIRM") {
    e(evidence, `confirm skips=${gateCount(funnel, "confirm")}`);
    return { rootCause: "CONFIRM_BLOCKING", evidence, safeFix: SAFE_FIX.CONFIRM_BLOCKING, canOpenIfSignalQualifies: true };
  }

  if (gateCount(funnel, "signal") > 0 || (summary && summary.fired === 0)) {
    e(evidence, `signal rejects=${gateCount(funnel, "signal")}`);
    return { rootCause: "SIGNAL_NOT_FIRING", evidence, safeFix: SAFE_FIX.SIGNAL_NOT_FIRING, canOpenIfSignalQualifies: true };
  }

  return {
    rootCause: "UNKNOWN",
    evidence,
    safeFix: SAFE_FIX.UNKNOWN,
    canOpenIfSignalQualifies: false,
  };
}
