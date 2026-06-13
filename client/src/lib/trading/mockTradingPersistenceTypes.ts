import { z } from "zod";
import { FUTURES_STRAT_DEFS } from "@/lib/trading/futuresStrategies";
import {
  DEFAULT_MOCK_TRADING_CONFIG,
  type MockAccountState,
  type MockTrade,
  type MockTradeLog,
  type MockTradingConfig,
} from "@/lib/trading/mockTradingEngine";

export const DEFAULT_MOCK_ACCOUNT_KEY = "mock_trading_default";
export const MOCK_RESET_CONFIRMATION = "RESET_MOCK_TRADING";

const finiteNumber = z.number().finite();
const nonNegativeNumber = finiteNumber.min(0);
const optionalNullableFinite = finiteNumber.nullable();
const strategyParamValueSchema = z.union([finiteNumber, z.string().trim().max(180)]);

export const mockAccountKeySchema = z
  .string()
  .trim()
  .min(1)
  .max(128)
  .regex(/^[a-zA-Z0-9_.:-]+$/);

export const mockTradeBlockerSchema = z
  .object({
    gate: z.string().trim().min(1).max(64),
    reason: z.string().trim().max(500),
  })
  .strict();

export const mockTradingConfigSchema = z
  .object({
    startingBalanceUsd: finiteNumber.positive(),
    sizingMode: z.enum(["fixed_pct_equity", "fixed_notional", "risk_pct_equity"]),
    fixedPctOfEquity: nonNegativeNumber,
    fixedNotionalUsd: nonNegativeNumber,
    riskPctOfEquity: nonNegativeNumber,
    leverage: finiteNumber.min(1).max(125),
    takeProfitUsd: nonNegativeNumber,
    stopLossUsd: nonNegativeNumber,
    takeProfitPct: nonNegativeNumber,
    stopLossPct: nonNegativeNumber,
    maxHoldMinutes: nonNegativeNumber,
    takerFeePct: nonNegativeNumber,
    slippageBpsPerSide: nonNegativeNumber,
    maxOpenMockTrades: z.coerce.number().int().min(1).max(50_000).default(DEFAULT_MOCK_TRADING_CONFIG.maxOpenMockTrades),
    maxOpenLongTrades: z.coerce.number().int().min(1).max(50_000).default(DEFAULT_MOCK_TRADING_CONFIG.maxOpenLongTrades),
    maxOpenShortTrades: z.coerce.number().int().min(1).max(50_000).default(DEFAULT_MOCK_TRADING_CONFIG.maxOpenShortTrades),
    tradeCooldownMinutes: nonNegativeNumber.default(DEFAULT_MOCK_TRADING_CONFIG.tradeCooldownMinutes),
    minSignalScore: finiteNumber.min(0).max(100).default(DEFAULT_MOCK_TRADING_CONFIG.minSignalScore),
    maxSignalsPerBatch: z.coerce.number().int().min(1).max(500).default(DEFAULT_MOCK_TRADING_CONFIG.maxSignalsPerBatch),
    minRiskRewardRatio: nonNegativeNumber.default(DEFAULT_MOCK_TRADING_CONFIG.minRiskRewardRatio),
    dailyLossLimitPct: nonNegativeNumber.default(DEFAULT_MOCK_TRADING_CONFIG.dailyLossLimitPct),
    weeklyLossLimitPct: nonNegativeNumber.default(DEFAULT_MOCK_TRADING_CONFIG.weeklyLossLimitPct),
    maxDrawdownPct: nonNegativeNumber.default(DEFAULT_MOCK_TRADING_CONFIG.maxDrawdownPct),
    fundingRatePctPer8h: finiteNumber.default(DEFAULT_MOCK_TRADING_CONFIG.fundingRatePctPer8h),
    fundingIntervalHours: finiteNumber.positive().default(DEFAULT_MOCK_TRADING_CONFIG.fundingIntervalHours),
    pipelineStage: z.string().trim().max(32).nullable().optional(),
  })
  .strict();

export const mockTradeSchema = z
  .object({
    id: z.string().trim().min(1).max(180),
    traceId: z.string().trim().min(1).max(180),
    strategyId: z.number().int().nonnegative(),
    strategyName: z.string().trim().min(1).max(180),
    symbol: z.string().trim().min(1).max(40),
    side: z.enum(["BUY", "SELL"]),
    notional: nonNegativeNumber,
    quantity: nonNegativeNumber,
    leverage: finiteNumber.min(1).max(125),
    marginUsed: nonNegativeNumber,
    signalPrice: nonNegativeNumber,
    entryPrice: finiteNumber.positive(),
    takeProfitPrice: finiteNumber.positive(),
    stopLossPrice: finiteNumber.positive(),
    takeProfitUsd: nonNegativeNumber,
    stopLossUsd: nonNegativeNumber,
    riskRewardRatio: nonNegativeNumber,
    signalScore: finiteNumber,
    requiredThreshold: finiteNumber,
    blockers: z.array(mockTradeBlockerSchema).max(32),
    status: z.enum(["OPEN", "CLOSED"]),
    openedAt: nonNegativeNumber,
    closedAt: optionalNullableFinite,
    currentPrice: finiteNumber.positive(),
    unrealizedPnl: finiteNumber,
    realizedPnl: finiteNumber,
    fees: nonNegativeNumber,
    fundingCosts: finiteNumber.default(0),
    exitReason: z.enum(["TAKE_PROFIT", "STOP_LOSS", "MAX_HOLD", "MANUAL"]).nullable(),
    exitPrice: optionalNullableFinite,
    strategyFamily: z.string().trim().min(1).max(120).optional(),
    confidenceScore: finiteNumber.min(0).max(100).optional(),
    strategyParams: z.record(z.string().trim().min(1).max(80), strategyParamValueSchema).optional(),
    researchPack: z.boolean().optional(),
    regimeAtEntry: z.string().trim().min(1).max(64).optional(),
    ispapStrategyId: z.string().trim().min(1).max(180).optional(),
    pipelineStage: z.string().trim().min(1).max(32).optional(),
  })
  .strict()
  .superRefine((trade, ctx) => {
    if (trade.status === "OPEN") {
      if (trade.closedAt !== null) {
        ctx.addIssue({ code: "custom", path: ["closedAt"], message: "OPEN trades must not have closedAt" });
      }
      if (trade.exitReason !== null) {
        ctx.addIssue({ code: "custom", path: ["exitReason"], message: "OPEN trades must not have exitReason" });
      }
      if (trade.exitPrice !== null) {
        ctx.addIssue({ code: "custom", path: ["exitPrice"], message: "OPEN trades must not have exitPrice" });
      }
    }
    if (trade.status === "CLOSED") {
      if (trade.closedAt == null) {
        ctx.addIssue({ code: "custom", path: ["closedAt"], message: "CLOSED trades require closedAt" });
      }
      if (trade.exitReason == null) {
        ctx.addIssue({ code: "custom", path: ["exitReason"], message: "CLOSED trades require exitReason" });
      }
      if (trade.exitPrice == null) {
        ctx.addIssue({ code: "custom", path: ["exitPrice"], message: "CLOSED trades require exitPrice" });
      }
    }
  });

export const mockAccountStateSchema = z
  .object({
    startingBalance: finiteNumber.positive(),
    cashBalance: finiteNumber,
    equity: finiteNumber,
    realizedPnl: finiteNumber,
    unrealizedPnl: finiteNumber,
    exposure: nonNegativeNumber,
    marginUsed: nonNegativeNumber,
    availableBalance: finiteNumber,
    returnPct: finiteNumber,
    peakEquity: finiteNumber,
    maxDrawdownPct: nonNegativeNumber,
    longExposure: nonNegativeNumber.default(0),
    shortExposure: nonNegativeNumber.default(0),
    openCount: z.number().int().nonnegative(),
    closedCount: z.number().int().nonnegative(),
  })
  .strict();

export const mockTradeLogEventSchema = z.enum([
  "MOCK_TRADE_CREATED",
  "MOCK_TRADE_CLOSED",
  "MOCK_TRADE_TP_HIT",
  "MOCK_TRADE_SL_HIT",
  "MOCK_TRADE_UPDATED",
  "MOCK_ACCOUNT_UPDATED",
  "MOCK_STRATEGY_RUNNER_EVENT",
  "MOCK_STRATEGY_RUNNER_ERROR",
  "MOCK_TRADE_LIMIT_REACHED",
  "MOCK_TRADE_REJECTED",
  "MOCK_TRADING_RESET",
]);

export const mockTradeLogSchema = z
  .object({
    ts: nonNegativeNumber,
    event: mockTradeLogEventSchema,
    tradeId: z.string().trim().min(1).max(180).optional(),
    strategyId: z.number().int().nonnegative().optional(),
    strategyName: z.string().trim().max(180).optional(),
    side: z.enum(["BUY", "SELL"]).optional(),
    price: finiteNumber.optional(),
    entryPrice: finiteNumber.optional(),
    exitPrice: finiteNumber.optional(),
    exitReason: z.enum(["TAKE_PROFIT", "STOP_LOSS", "MAX_HOLD", "MANUAL"]).nullable().optional(),
    ignoredBlockers: z.array(z.string().trim().min(1).max(64)).max(32).default([]),
    rejectionCategory: z.string().trim().min(1).max(64).optional(),
    signalScore: finiteNumber.optional(),
    confidenceScore: finiteNumber.optional(),
    riskRewardRatio: finiteNumber.optional(),
    pnl: finiteNumber.optional(),
    notional: nonNegativeNumber.optional(),
    message: z.string().trim().max(500).optional(),
  })
  .strict();

export const mockTradeSortSchema = z.enum(["most_profitable", "least_profitable", "newest", "oldest"]);
export const mockTradeAgeModeSchema = z.enum(["less", "more", "between"]);
const ageMinutesQuerySchema = z.coerce.number().finite().min(0).max(10_000_000);

export const mockTradeListQuerySchema = z
  .object({
    account_key: mockAccountKeySchema.default(DEFAULT_MOCK_ACCOUNT_KEY),
    page: z.coerce.number().int().min(1).max(10_000).default(1),
    limit: z.coerce.number().int().min(1).max(50_000).default(100),
    status: z.enum(["OPEN", "CLOSED"]).optional(),
    side: z.enum(["BUY", "SELL"]).optional(),
    strategy_id: z.coerce.number().int().nonnegative().optional(),
    strategy_family: z.string().trim().min(1).max(120).optional(),
    blocker_gate: z.string().trim().min(1).max(64).optional(),
    profitability: z.enum(["profit", "loss"]).optional(),
    age_mode: mockTradeAgeModeSchema.optional(),
    age_min_minutes: ageMinutesQuerySchema.optional(),
    age_max_minutes: ageMinutesQuerySchema.optional(),
    sort: mockTradeSortSchema.default("newest"),
  })
  .strict()
  .superRefine((query, ctx) => {
    if (query.age_mode === "less" && query.age_max_minutes == null) {
      ctx.addIssue({ code: "custom", path: ["age_max_minutes"], message: "Less-than age filter requires age_max_minutes" });
    }
    if (query.age_mode === "more" && query.age_min_minutes == null) {
      ctx.addIssue({ code: "custom", path: ["age_min_minutes"], message: "More-than age filter requires age_min_minutes" });
    }
    if (query.age_mode === "between") {
      if (query.age_min_minutes == null) {
        ctx.addIssue({ code: "custom", path: ["age_min_minutes"], message: "Between age filter requires age_min_minutes" });
      }
      if (query.age_max_minutes == null) {
        ctx.addIssue({ code: "custom", path: ["age_max_minutes"], message: "Between age filter requires age_max_minutes" });
      }
      if (
        query.age_min_minutes != null &&
        query.age_max_minutes != null &&
        query.age_min_minutes > query.age_max_minutes
      ) {
        ctx.addIssue({ code: "custom", path: ["age_max_minutes"], message: "age_max_minutes must be greater than or equal to age_min_minutes" });
      }
    }
  });

export const mockLogListQuerySchema = z
  .object({
    account_key: mockAccountKeySchema.default(DEFAULT_MOCK_ACCOUNT_KEY),
    page: z.coerce.number().int().min(1).max(10_000).default(1),
    limit: z.coerce.number().int().min(1).max(500).default(100),
    event: mockTradeLogEventSchema.optional(),
  })
  .strict();

export const mockTradeWriteBodySchema = z
  .object({
    accountKey: mockAccountKeySchema.default(DEFAULT_MOCK_ACCOUNT_KEY),
    trade: mockTradeSchema,
    config: mockTradingConfigSchema.default(DEFAULT_MOCK_TRADING_CONFIG),
  })
  .strict();

export const mockTradePatchBodySchema = z
  .object({
    accountKey: mockAccountKeySchema.default(DEFAULT_MOCK_ACCOUNT_KEY),
    trade: mockTradeSchema,
    config: mockTradingConfigSchema.default(DEFAULT_MOCK_TRADING_CONFIG),
  })
  .strict();

export const mockTradeCloseBodySchema = z
  .object({
    accountKey: mockAccountKeySchema.default(DEFAULT_MOCK_ACCOUNT_KEY),
    trade: mockTradeSchema.optional(),
    price: finiteNumber.positive().optional(),
    closedAt: nonNegativeNumber.optional(),
    config: mockTradingConfigSchema.default(DEFAULT_MOCK_TRADING_CONFIG),
  })
  .strict();

export const mockAccountSnapshotBodySchema = z
  .object({
    accountKey: mockAccountKeySchema.default(DEFAULT_MOCK_ACCOUNT_KEY),
    account: mockAccountStateSchema,
    config: mockTradingConfigSchema.default(DEFAULT_MOCK_TRADING_CONFIG),
  })
  .strict();

export const mockResetBodySchema = z
  .object({
    accountKey: mockAccountKeySchema.default(DEFAULT_MOCK_ACCOUNT_KEY),
    confirmation: z.literal(MOCK_RESET_CONFIRMATION),
  })
  .strict();

export type MockTradeListQuery = z.infer<typeof mockTradeListQuerySchema>;
export type MockLogListQuery = z.infer<typeof mockLogListQuerySchema>;
export type MockTradeLogEvent = z.infer<typeof mockTradeLogEventSchema>;

const strategyFamilyById = new Map(
  FUTURES_STRAT_DEFS.map((def) => [def.id, def.templateFamily ?? def.category ?? "UNKNOWN"]),
);

export function strategyFamilyForTrade(trade: Pick<MockTrade, "strategyId" | "strategyFamily">): string {
  return trade.strategyFamily ?? strategyFamilyById.get(trade.strategyId) ?? "UNKNOWN";
}

export function mergeHydratedMockTrades(localTrades: readonly MockTrade[], remoteTrades: readonly MockTrade[]): MockTrade[] {
  const byId = new Map<string, MockTrade>();
  for (const trade of localTrades) byId.set(trade.id, trade);
  for (const trade of remoteTrades) byId.set(trade.id, trade);
  return [...byId.values()].sort((a, b) => a.openedAt - b.openedAt);
}

/** Merge persisted history with the live engine cache; live rows win on id conflicts. */
export function mergePortfolioTrades(
  liveTrades: readonly MockTrade[],
  persistedTrades: readonly MockTrade[],
): MockTrade[] {
  const byId = new Map<string, MockTrade>();
  for (const trade of persistedTrades) byId.set(trade.id, trade);
  for (const trade of liveTrades) byId.set(trade.id, trade);
  return [...byId.values()].sort((a, b) => a.openedAt - b.openedAt);
}

export type MockTradingHydrateResponse = {
  ok: true;
  trades: MockTrade[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
  source: "mongo";
};

export type MockAccountSnapshotResponse = {
  ok: true;
  snapshot: MockAccountState | null;
  config: MockTradingConfig | null;
  source: "mongo";
};

export type MockLogsResponse = {
  ok: true;
  logs: MockTradeLog[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
  source: "mongo";
};
