/**
 * Grade-stage mock trading configuration.
 *
 * Each pipeline stage (Grade 5 → Main Engine) runs the mock engine with
 * slightly different risk parameters so performance is measured consistently
 * at each level of the promotion ladder.
 */

import type { StrategyStatus } from "./types";
import {
  DEFAULT_MOCK_TRADING_CONFIG,
  type MockTradingConfig,
} from "@/lib/trading/mockTradingEngine";

// ── Account key helpers ───────────────────────────────────────────────────────

const STAGE_ACCOUNT_KEY: Record<StrategyStatus, string> = {
  GRADE_5: "mock_trading_grade_5",
  GRADE_4: "mock_trading_grade_4",
  GRADE_3: "mock_trading_grade_3",
  GRADE_2: "mock_trading_grade_2",
  GRADE_1: "mock_trading_grade_1",
  MAIN_ENGINE: "mock_trading_main",
  RETIRED: "mock_trading_retired",
};

export function mockAccountKeyForStage(stage: StrategyStatus): string {
  return STAGE_ACCOUNT_KEY[stage] ?? "mock_trading_grade_5";
}

// ── Per-stage config overrides ────────────────────────────────────────────────

const STAGE_OVERRIDES: Record<StrategyStatus, Partial<MockTradingConfig>> = {
  GRADE_5: {
    maxOpenMockTrades: 3,
    fixedNotionalUsd: 300,
    takeProfitPct: 1.5,
    stopLossPct: 0.4,
    pipelineStage: "GRADE_5",
    dailyLossLimitPct: 5,
  },
  GRADE_4: {
    maxOpenMockTrades: 5,
    fixedNotionalUsd: 500,
    takeProfitPct: 1.5,
    stopLossPct: 0.4,
    pipelineStage: "GRADE_4",
    dailyLossLimitPct: 4,
  },
  GRADE_3: {
    maxOpenMockTrades: 8,
    fixedNotionalUsd: 750,
    takeProfitPct: 1.5,
    stopLossPct: 0.4,
    pipelineStage: "GRADE_3",
    dailyLossLimitPct: 3.5,
  },
  GRADE_2: {
    maxOpenMockTrades: 10,
    fixedNotionalUsd: 1000,
    takeProfitPct: 1.5,
    stopLossPct: 0.4,
    pipelineStage: "GRADE_2",
    dailyLossLimitPct: 3,
  },
  GRADE_1: {
    maxOpenMockTrades: 12,
    fixedNotionalUsd: 1500,
    takeProfitPct: 1.5,
    stopLossPct: 0.4,
    pipelineStage: "GRADE_1",
    dailyLossLimitPct: 3,
  },
  MAIN_ENGINE: {
    maxOpenMockTrades: 20,
    fixedNotionalUsd: 5000,
    takeProfitPct: 1.5,
    stopLossPct: 0.4,
    pipelineStage: "MAIN_ENGINE",
    dailyLossLimitPct: 3,
  },
  RETIRED: {
    maxOpenMockTrades: 0,
    fixedNotionalUsd: 0,
    pipelineStage: "RETIRED",
  },
};

/** Returns the full mock trading config for a given pipeline stage. */
export function getMockConfigForPipelineStage(stage: StrategyStatus): MockTradingConfig {
  const overrides = STAGE_OVERRIDES[stage] ?? STAGE_OVERRIDES.GRADE_5;
  return { ...DEFAULT_MOCK_TRADING_CONFIG, ...overrides };
}
