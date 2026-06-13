import {
  DEFAULT_MOCK_TRADING_CONFIG,
  normalizeMockTradingConfig,
  type MockTradingConfig,
} from "@/lib/trading/mockTradingEngine";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAccountKey";
import type { StrategyStatus } from "./types";

/** Grade 5 discovery — unlimited concurrent positions, no risk gates. */
export const GRADE_5_DISCOVERY_CONFIG: MockTradingConfig = normalizeMockTradingConfig({
  ...DEFAULT_MOCK_TRADING_CONFIG,
  pipelineStage: "GRADE_5",
  maxOpenMockTrades: 50_000,
  maxOpenLongTrades: 50_000,
  maxOpenShortTrades: 50_000,
  tradeCooldownMinutes: 0,
  minSignalScore: 0,
  maxSignalsPerBatch: 305,
  minRiskRewardRatio: 0,
  dailyLossLimitPct: 0,
  weeklyLossLimitPct: 0,
  maxDrawdownPct: 0,
});

export function mockAccountKeyForStage(status: StrategyStatus): string {
  if (status === "MAIN_ENGINE") return OWNER_ACCOUNT_KEY;
  return `mock_trading_${status.toLowerCase()}`;
}

export function getMockConfigForPipelineStage(status: StrategyStatus): MockTradingConfig {
  if (status === "GRADE_5") return GRADE_5_DISCOVERY_CONFIG;
  return DEFAULT_MOCK_TRADING_CONFIG;
}
