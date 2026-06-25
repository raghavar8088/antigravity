import type { StrategyStatus } from "./types";
import {
  DEFAULT_MOCK_TRADING_CONFIG,
  STANDARD_FIXED_TRADE_SIZE_BTC,
  type MockTradingConfig,
} from "@/lib/trading/mockTradingEngine";

/**
 * Trade Engine paper-account starting balance in USD. Mirrors the Go engine's
 * `INITIAL_PAPER_BALANCE_USD` so the dashboard equity matches the backend desk.
 * Env: `NEXT_PUBLIC_INITIAL_PAPER_BALANCE_USD`. Floors at $100 (same as the
 * engine) and defaults to $1000 when unset.
 */
export function tradeEngineStartingBalanceUsd(): number {
  const raw = process.env.NEXT_PUBLIC_INITIAL_PAPER_BALANCE_USD;
  if (raw === undefined || raw.trim() === "") return 1000;
  const n = Number(raw);
  if (!Number.isFinite(n)) return 1000;
  if (n < 100) return 100;
  return n;
}

/** Single Trade Engine config — all active strategies use this. */
export const TRADE_ENGINE_CONFIG: Partial<MockTradingConfig> = {
  startingBalanceUsd: tradeEngineStartingBalanceUsd(),
  // EQUAL FOOTING: pin every strategy to the same 0.1 BTC fixed size. Set
  // explicitly (not just inherited from DEFAULT_MOCK_TRADING_CONFIG) so the
  // Trade Engine can never silently fall back to equity/risk-relative sizing
  // that lets one strategy (e.g. MTF_Trend_Align) balloon vs the rest.
  sizingMode: "fixed_btc",
  fixedSizeBtc: STANDARD_FIXED_TRADE_SIZE_BTC,
  // High ceiling so list/cache fetch limits scale when every strategy trades
  // simultaneously (uncapped mode bypasses this as a hard gate — see
  // mockTradingUncapped / evaluateMockTradeOpenRisk).
  maxOpenMockTrades: 500,
  maxOpenLongTrades: 500,
  maxOpenShortTrades: 500,
  maxSignalsPerBatch: 500,
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
