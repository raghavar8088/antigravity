import type { StrategyStatus } from "./types";
import {
  DEFAULT_MOCK_TRADING_CONFIG,
  type MockTradingConfig,
} from "@/lib/trading/mockTradingEngine";

/** Single Trade Engine config — all active strategies use this. */
export const TRADE_ENGINE_CONFIG: Partial<MockTradingConfig> = {
  maxOpenMockTrades: 20,
  fixedNotionalUsd: 5000,
  takeProfitPct: 1.5,
  stopLossPct: 0.4,
  pipelineStage: "MAIN_ENGINE",
  dailyLossLimitPct: 2,
};

/** Always returns the trade engine account key. */
export function mockAccountKeyForStage(_stage?: StrategyStatus): string {
  return "mock_trading_main";
}

/** Always returns the trade engine config. */
export function mockConfigForStage(_stage?: StrategyStatus): MockTradingConfig {
  return { ...DEFAULT_MOCK_TRADING_CONFIG, ...TRADE_ENGINE_CONFIG };
}

/** @deprecated All stages map to Trade Engine. */
export function getMockConfigForPipelineStage(stage?: StrategyStatus): MockTradingConfig {
  return mockConfigForStage(stage);
}

/** @deprecated All stages map to Trade Engine. */
export const STAGE_ACCOUNT_KEY: Record<StrategyStatus, string> = {
  TRADE_ENGINE: "mock_trading_main",
  MAIN_ENGINE: "mock_trading_main",
  GRADE_5: "mock_trading_main",
  GRADE_4: "mock_trading_main",
  GRADE_3: "mock_trading_main",
  GRADE_2: "mock_trading_main",
  GRADE_1: "mock_trading_main",
  RETIRED: "mock_trading_retired",
};
