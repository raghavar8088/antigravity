import type { EntryFunnelBlockerKey } from "@/lib/trading/deskEntryFunnelSnapshot";
import { funnelRecommendation } from "@/lib/trading/deskEntryFunnelSnapshot";
import type { MockExecutorStateDoc } from "@/lib/mockTradingExecutor/types";

export type ExecutorNoTradeReason =
  | "EXECUTOR_NOT_RUNNING"
  | "EXECUTOR_STALE"
  | "MARKET_DATA_UNAVAILABLE"
  | "REGIME_BLOCKING"
  | "ATR_FEES_BLOCKING"
  | "SIGNAL_NOT_FIRING"
  | "CONFIRM_BLOCKING"
  | "MARGIN_OR_CAP_BLOCKING"
  | "NO_CANDIDATES"
  | "UNKNOWN";

export type ExecutorDiagnosisStatus =
  | "HEALTHY"
  | "MARKET_WAIT"
  | "EXECUTOR_FAULT"
  | "DATA_FAULT"
  | "CAPACITY_WAIT";

export type ExecutorNoTradeDiagnosis = {
  reason: ExecutorNoTradeReason;
  evidence: string[];
  safeFix: string;
};

export type ExecutorDiagnosis = ExecutorNoTradeDiagnosis & {
  headline: string;
  explanation: string;
  nextAction: string;
  isHealthy: boolean;
  status: ExecutorDiagnosisStatus;
  dominantBlocker: string | null;
};

const REASON_HEADLINES: Record<ExecutorNoTradeReason, string> = {
  EXECUTOR_NOT_RUNNING: "EXECUTOR_NOT_RUNNING",
  EXECUTOR_STALE: "EXECUTOR_STALE",
  MARKET_DATA_UNAVAILABLE: "MARKET_DATA_UNAVAILABLE",
  REGIME_BLOCKING: "REGIME_BLOCKING_ENTRIES",
  ATR_FEES_BLOCKING: "ATR_FEES_BLOCKING_ENTRIES",
  SIGNAL_NOT_FIRING: "SIGNAL_SCORE_BLOCKING",
  CONFIRM_BLOCKING: "CONFIRMATION_BLOCKING",
  MARGIN_OR_CAP_BLOCKING: "CAPACITY_BLOCKING",
  NO_CANDIDATES: "NO_CANDIDATES_THIS_CYCLE",
  UNKNOWN: "UNKNOWN_BLOCKER",
};

const REASON_EXPLANATIONS: Record<ExecutorNoTradeReason, string> = {
  EXECUTOR_NOT_RUNNING:
    "No backend executor cycle has been recorded. Trades cannot open until PM2 or Vercel cron runs the mock-trading loop.",
  EXECUTOR_STALE:
    "The backend executor has not completed a cycle recently. Execution may have stopped or crashed.",
  MARKET_DATA_UNAVAILABLE:
    "BTC kline/mark data is missing or stale. Signal evaluation cannot run without fresh bars.",
  REGIME_BLOCKING:
    "Market regime is ranging or incompatible with the active roster. Trend scalpers do not fire in chop — this is protective behavior.",
  ATR_FEES_BLOCKING:
    "Expected move (ATR) is too small relative to round-trip fees. Opening a trade would likely lose to costs.",
  SIGNAL_NOT_FIRING:
    "Strategies evaluated but none passed the signal score threshold on the last cycle.",
  CONFIRM_BLOCKING:
    "A strategy signal fired but secondary confirmation (support/resistance, alignment) did not pass.",
  MARGIN_OR_CAP_BLOCKING:
    "Account is at capacity — margin, max open positions, or same-side correlation limits are blocking new entries.",
  NO_CANDIDATES:
    "Executor is healthy but produced zero entry candidates this cycle. Gates or scores blocked every strategy.",
  UNKNOWN:
    "Executor ran recently but the no-trade cause could not be classified. Inspect execution logs and signal trace.",
};

const HEALTHY_REASONS = new Set<ExecutorNoTradeReason>([
  "REGIME_BLOCKING",
  "ATR_FEES_BLOCKING",
  "SIGNAL_NOT_FIRING",
  "CONFIRM_BLOCKING",
  "MARGIN_OR_CAP_BLOCKING",
  "NO_CANDIDATES",
]);

function reasonToStatus(reason: ExecutorNoTradeReason): ExecutorDiagnosisStatus {
  switch (reason) {
    case "EXECUTOR_NOT_RUNNING":
    case "EXECUTOR_STALE":
      return "EXECUTOR_FAULT";
    case "MARKET_DATA_UNAVAILABLE":
      return "DATA_FAULT";
    case "MARGIN_OR_CAP_BLOCKING":
      return "CAPACITY_WAIT";
    case "REGIME_BLOCKING":
    case "ATR_FEES_BLOCKING":
    case "SIGNAL_NOT_FIRING":
    case "CONFIRM_BLOCKING":
    case "NO_CANDIDATES":
      return "MARKET_WAIT";
    default:
      return "HEALTHY";
  }
}

function dominantBlockerFromState(state: MockExecutorStateDoc | null): string | null {
  if (!state) return null;
  return state.last_dominant_blocker ?? state.entry_funnel_snapshot?.dominantBlocker ?? null;
}

function nextActionForDiagnosis(
  reason: ExecutorNoTradeReason,
  state: MockExecutorStateDoc | null,
  base: ExecutorNoTradeDiagnosis,
): string {
  const funnelRec = state?.entry_funnel_snapshot?.recommendation;
  if (funnelRec?.trim()) return funnelRec;

  const blocker = dominantBlockerFromState(state);
  if (blocker && blocker !== "none") {
    return funnelRecommendation(blocker as EntryFunnelBlockerKey);
  }

  return base.safeFix;
}

export function diagnoseNoTradeFromExecutorState(
  state: MockExecutorStateDoc | null,
  now = Date.now(),
): ExecutorNoTradeDiagnosis {
  const evidence: string[] = [];

  if (!state?.last_tick_at) {
    return {
      reason: "EXECUTOR_NOT_RUNNING",
      evidence: ["no mock_executor_state document"],
      safeFix:
        "Start PM2 worker: pm2 start ecosystem.config.js --only mock-trading-executor. Or enable Vercel cron /api/cron/mock-trading-tick.",
    };
  }

  const ageSec = Math.floor((now - state.last_tick_at) / 1000);
  evidence.push(`last executor tick ${ageSec}s ago`);
  if (ageSec > 30) {
    return {
      reason: "EXECUTOR_STALE",
      evidence,
      safeFix:
        "Restart mock-trading executor: pm2 restart mock-trading-executor. Check VPS logs for MongoDB or market data errors.",
    };
  }

  const blocker = state.last_dominant_blocker ?? state.entry_funnel_snapshot?.dominantBlocker ?? "none";
  evidence.push(`dominant blocker=${blocker}`);
  evidence.push(`last candidates=${state.last_candidate_count ?? 0}`);

  if (state.last_error) evidence.push(`last error=${state.last_error}`);

  switch (blocker) {
    case "regime":
      return {
        reason: "REGIME_BLOCKING",
        evidence,
        safeFix: "Wait for trend regime — scalpers do not enter in ranging markets. Do not lower thresholds.",
      };
    case "atrFees":
      return {
        reason: "ATR_FEES_BLOCKING",
        evidence,
        safeFix: "Expected move too small vs fees. No trade is correct — preserve capital.",
      };
    case "signal":
      return {
        reason: "SIGNAL_NOT_FIRING",
        evidence,
        safeFix: "No strategy passed signal threshold. Monitor Closest Signals panel.",
      };
    case "confirm":
      return {
        reason: "CONFIRM_BLOCKING",
        evidence,
        safeFix: "Signals fired but confirmation failed. Wait for alignment.",
      };
    case "margin":
    case "maxOpen":
    case "sameSide":
    case "category":
      return {
        reason: "MARGIN_OR_CAP_BLOCKING",
        evidence,
        safeFix: "Desk at capacity. Wait for positions to close.",
      };
    case "noData":
      return {
        reason: "MARKET_DATA_UNAVAILABLE",
        evidence,
        safeFix: "Check Delta/Binance kline connectivity and DELTA_API_BASE_URL.",
      };
    case "none":
      if ((state.last_candidate_count ?? 0) === 0 && (state.last_opened ?? 0) === 0) {
        return {
          reason: "NO_CANDIDATES",
          evidence,
          safeFix: "Executor is healthy but no candidates this cycle. Gates or signal scores blocked all strategies.",
        };
      }
      return {
        reason: "UNKNOWN",
        evidence,
        safeFix: "Check /api/mock-trading/executor-status and execution_logs in MongoDB.",
      };
    default:
      return {
        reason: "UNKNOWN",
        evidence,
        safeFix: "Inspect execution_logs and signal trace for this account.",
      };
  }
}

/** Structured diagnosis for API + Trade Engine UI (Option A). */
export function buildExecutorDiagnosis(
  state: MockExecutorStateDoc | null,
  now = Date.now(),
): ExecutorDiagnosis {
  const base = diagnoseNoTradeFromExecutorState(state, now);
  const reason = base.reason;
  const isHealthy = HEALTHY_REASONS.has(reason);
  const status = reason === "UNKNOWN" && state?.last_tick_at ? "MARKET_WAIT" : reasonToStatus(reason);

  return {
    ...base,
    headline: REASON_HEADLINES[reason],
    explanation: REASON_EXPLANATIONS[reason],
    nextAction: nextActionForDiagnosis(reason, state, base),
    isHealthy,
    status,
    dominantBlocker: dominantBlockerFromState(state),
  };
}
