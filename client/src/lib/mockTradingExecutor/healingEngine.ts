import type { MockExecutorStateDoc } from "@/lib/mockTradingExecutor/types";
import {
  buildExecutorDiagnosis,
  type ExecutorNoTradeReason,
} from "@/lib/mockTradingExecutor/diagnoseExecutor";

export type HealingAction = {
  healed: boolean;
  action: string;
  reason: ExecutorNoTradeReason;
  safeToAutomate: boolean;
  nextAction: string;
  isHealthy: boolean;
  status: string;
};

const ACTION_BY_REASON: Partial<Record<ExecutorNoTradeReason, string>> = {
  EXECUTOR_NOT_RUNNING:
    "ALERT: start PM2 mock-trading-executor or enable Vercel cron /api/cron/mock-trading-tick",
  EXECUTOR_STALE:
    "ALERT: pm2 restart mock-trading-executor and inspect logs for MongoDB/market errors",
  MARKET_DATA_UNAVAILABLE:
    "ALERT: check Delta/Binance kline connectivity and DELTA_API_BASE_URL",
};

/**
 * Read-only healing recommendations for the mock-trading executor.
 * Does not mutate pause flags or thresholds — operator must act on alerts.
 */
export function recommendExecutorHealing(
  state: MockExecutorStateDoc | null,
  now = Date.now(),
): HealingAction {
  const diagnosis = buildExecutorDiagnosis(state, now);
  const needsOperator = diagnosis.status === "EXECUTOR_FAULT" || diagnosis.status === "DATA_FAULT";

  return {
    healed: diagnosis.isHealthy,
    action: ACTION_BY_REASON[diagnosis.reason] ?? diagnosis.nextAction,
    reason: diagnosis.reason,
    safeToAutomate: diagnosis.isHealthy,
    nextAction: diagnosis.nextAction,
    isHealthy: diagnosis.isHealthy,
    status: diagnosis.status,
  };
}
