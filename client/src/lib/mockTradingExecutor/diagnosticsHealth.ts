import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { diagnoseNoTradeFromExecutorState } from "@/lib/mockTradingExecutor/diagnoseExecutor";
import {
  executorStateHealth,
  loadExecutorState,
} from "@/lib/mockTradingExecutor/persistExecutorState";
import { collectMockSignalTraceDiagnostic } from "@/lib/mockTradingExecutor/signalTraceDiagnostic";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export type HealthCheckResult = {
  healthy: boolean;
  reason: string;
  details?: Record<string, unknown>;
};

export type DiagnosticsHealthReport = {
  healthy: boolean;
  timestamp: number;
  accountKey: string;
  checks: {
    mongo: HealthCheckResult;
    executor: HealthCheckResult;
    marketData: HealthCheckResult;
    signalPipeline: HealthCheckResult;
  };
  noTradeDiagnosis: ReturnType<typeof diagnoseNoTradeFromExecutorState>;
};

export async function buildDiagnosticsHealthReport(args?: {
  accountKey?: string;
  symbol?: string;
}): Promise<DiagnosticsHealthReport> {
  const accountKey = args?.accountKey?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const timestamp = Date.now();

  const mongo: HealthCheckResult = isMongoConfigured()
    ? { healthy: true, reason: "OK" }
    : { healthy: false, reason: "MONGODB_NOT_CONFIGURED" };

  const state = mongo.healthy ? await loadExecutorState(accountKey) : null;
  const execFlags = executorStateHealth(state, timestamp);
  const executor: HealthCheckResult = {
    healthy: execFlags.healthy,
    reason: execFlags.stale ? "EXECUTOR_STALE" : state ? "OK" : "EXECUTOR_NEVER_RAN",
    details: {
      ageSeconds: execFlags.ageSeconds,
      lastMode: state?.last_mode ?? null,
      lastTickAt: state?.last_tick_at ?? null,
      dominantBlocker: state?.last_dominant_blocker ?? null,
    },
  };

  const trace = await collectMockSignalTraceDiagnostic({ accountKey, symbol: args?.symbol });
  const marketData: HealthCheckResult = {
    healthy: trace.dataFresh && trace.markPrice > 0,
    reason: trace.dataFresh ? "OK" : trace.marketError ?? "MARKET_DATA_STALE",
    details: {
      markPrice: trace.markPrice,
      bars: trace.bars,
      regime: trace.regime,
    },
  };

  const signalPipeline: HealthCheckResult = {
    healthy: trace.ok,
    reason: trace.dominantBlocker === "none" ? "OK" : trace.dominantBlocker,
    details: {
      candidateCount: trace.candidateCount,
      executableCount: trace.executableCount,
      blockerSummary: trace.blockerSummary,
    },
  };

  const checks = { mongo, executor, marketData, signalPipeline };
  const healthy = Object.values(checks).every((c) => c.healthy);

  return {
    healthy,
    timestamp,
    accountKey,
    checks,
    noTradeDiagnosis: diagnoseNoTradeFromExecutorState(state, timestamp),
  };
}
