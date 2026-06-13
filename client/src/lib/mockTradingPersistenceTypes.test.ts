import { describe, expect, it } from "vitest";
import {
  DEFAULT_MOCK_TRADING_CONFIG,
  computeAccountState,
  type MockTrade,
} from "@/lib/trading/mockTradingEngine";
import {
  mergeHydratedMockTrades,
  mergePortfolioTrades,
  mockAccountStateSchema,
  mockTradeListQuerySchema,
  mockTradeSchema,
  mockTradeWriteBodySchema,
  mockTradingConfigSchema,
  strategyFamilyForTrade,
} from "@/lib/trading/mockTradingPersistenceTypes";
import { getMockConfigForPipelineStage } from "@/lib/strategyAuthority/gradeStageMockConfig";
import { mockTradeToDoc } from "@/lib/trading/mockTradingMongo";

const trade: MockTrade = {
  id: "mock-trace-1",
  traceId: "trace-1",
  strategyId: 91,
  strategyName: "Trend_Continuation_Long",
  symbol: "BTCUSD",
  side: "BUY",
  notional: 10_000,
  quantity: 0.166,
  leverage: 25,
  marginUsed: 400,
  signalPrice: 60_000,
  entryPrice: 60_030,
  takeProfitPrice: 60_600,
  stopLossPrice: 59_700,
  takeProfitUsd: 10,
  stopLossUsd: 5,
  riskRewardRatio: 2,
  signalScore: 28,
  requiredThreshold: 20,
  blockers: [{ gate: "REGIME", reason: "regime blocked" }],
  status: "OPEN",
  openedAt: 1_700_000_000_000,
  closedAt: null,
  currentPrice: 60_050,
  unrealizedPnl: 3.2,
  realizedPnl: 0,
  fees: 0,
  fundingCosts: 0,
  exitReason: null,
  exitPrice: null,
};

describe("Mock Trading persistence schemas", () => {
  it("accepts a valid raw mock trade and rejects malformed close state", () => {
    expect(mockTradeSchema.safeParse(trade).success).toBe(true);
    expect(mockTradeSchema.safeParse({ ...trade, status: "CLOSED" }).success).toBe(false);
  });

  it("accepts computed account snapshots", () => {
    const account = computeAccountState([trade], DEFAULT_MOCK_TRADING_CONFIG);
    expect(mockAccountStateSchema.safeParse(account).success).toBe(true);
  });

  it("defaults mock config to 5 max open mock trades", () => {
    const parsed = mockTradingConfigSchema.parse({
      ...DEFAULT_MOCK_TRADING_CONFIG,
      maxOpenMockTrades: undefined,
    });
    expect(parsed.maxOpenMockTrades).toBe(5);
  });

  it("sanitizes pagination, filters, and sorting", () => {
    const parsed = mockTradeListQuerySchema.parse({
      page: "2",
      limit: "25",
      status: "OPEN",
      side: "BUY",
      strategy_id: "91",
      blocker_gate: "REGIME",
      profitability: "profit",
      sort: "most_profitable",
    });
    expect(parsed.page).toBe(2);
    expect(parsed.limit).toBe(25);
    expect(parsed.strategy_id).toBe(91);
    expect(parsed.sort).toBe("most_profitable");
  });

  it("allows mock trade hydration pages larger than the open-position cap", () => {
    const parsed = mockTradeListQuerySchema.parse({ limit: "100", status: "OPEN" });
    expect(parsed.limit).toBe(100);
  });

  it("accepts grade-stage mock config and catalog trades for Mongo writes", () => {
    const config = getMockConfigForPipelineStage("GRADE_5");
    expect(mockTradingConfigSchema.safeParse(config).success).toBe(true);

    const gradeTrade = {
      ...trade,
      ispapStrategyId: "ema-cross-fast",
      pipelineStage: "GRADE_5",
      regimeAtEntry: "TREND",
    };
    expect(mockTradeSchema.safeParse(gradeTrade).success).toBe(true);
    expect(
      mockTradeWriteBodySchema.safeParse({
        accountKey: "mock_trading_grade_5",
        trade: gradeTrade,
        config,
      }).success,
    ).toBe(true);
  });

  it("maps raw trades to Mongo documents with required searchable fields", () => {
    const doc = mockTradeToDoc("mock_trading_default", trade, DEFAULT_MOCK_TRADING_CONFIG);
    expect(doc.trade_id).toBe(trade.id);
    expect(doc.strategy_family).toBe(strategyFamilyForTrade(trade));
    expect(doc.blockers_rejected).toEqual(["REGIME"]);
    expect(doc.parameters_used.leverage).toBe(DEFAULT_MOCK_TRADING_CONFIG.leverage);
    expect(doc.raw_trade).toEqual(trade);
  });

  it("merges Mongo hydration over local cache by trade id", () => {
    const remote = { ...trade, currentPrice: 61_000, unrealizedPnl: 120 };
    const merged = mergeHydratedMockTrades([trade], [remote]);
    expect(merged).toHaveLength(1);
    expect(merged[0].currentPrice).toBe(61_000);
  });

  it("computes portfolio equity from persisted closed trades when live cache is empty", () => {
    const closed = {
      ...trade,
      status: "CLOSED" as const,
      closedAt: 1_700_000_100_000,
      exitPrice: 60_100,
      exitReason: "TAKE_PROFIT" as const,
      unrealizedPnl: 0,
      realizedPnl: -250,
    };
    const portfolio = mergePortfolioTrades([], [closed]);
    const account = computeAccountState(portfolio, DEFAULT_MOCK_TRADING_CONFIG);
    expect(account.realizedPnl).toBe(-250);
    expect(account.equity).toBe(DEFAULT_MOCK_TRADING_CONFIG.startingBalanceUsd - 250);
  });
});
