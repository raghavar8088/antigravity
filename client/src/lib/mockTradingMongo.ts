import type { Collection, Db, Filter, Sort } from "mongodb";
import type { RegimeSnapshot } from "@/lib/marketRegimeClassifier";
import { getDb } from "@/lib/mongoTradesClient";
import {
  computeAccountState,
  computeAnalytics,
  closeMockTrade,
  logForMockTradeClosed,
  normalizeMockTradingConfig,
  withMockExitFields,
  type MockAccountState,
  type MockTrade,
  type MockTradeAnalytics,
  type MockTradeLog,
  type MockTradingConfig,
} from "@/lib/mockTradingEngine";
import {
  DEFAULT_MOCK_ACCOUNT_KEY,
  strategyFamilyForTrade,
  type MockLogListQuery,
  type MockTradeListQuery,
  type MockTradeLogEvent,
} from "@/lib/mockTradingPersistenceTypes";
import type { StrategyScore } from "@/lib/strategyScoringEngine";

export const MOCK_TRADES_COLLECTION = "mock_trades";
export const MOCK_ACCOUNT_SNAPSHOTS_COLLECTION = "mock_account_snapshots";
export const MOCK_STRATEGY_ANALYTICS_COLLECTION = "mock_strategy_analytics";
export const MOCK_TRADE_LOGS_COLLECTION = "mock_trade_logs";
export const MOCK_ENGINE_CONFIG_COLLECTION = "mock_engine_config";
export const MOCK_STRATEGY_SIGNALS_COLLECTION = "strategy_signals";
export const MOCK_REGIME_SNAPSHOTS_COLLECTION = "regime_snapshots";
export const MOCK_STRATEGY_SCORES_COLLECTION = "strategy_scores";
export const MOCK_STRATEGY_SCORE_HISTORY_COLLECTION = "strategy_score_history";
export const MOCK_EQUITY_CURVE_COLLECTION = "equity_curve";
export const MOCK_DAILY_PNL_HISTORY_COLLECTION = "daily_pnl_history";

export type MockTradeDoc = {
  account_key: string;
  trade_id: string;
  trace_id: string;
  strategy_id: number;
  strategy_name: string;
  strategy_family: string;
  symbol: string;
  side: "BUY" | "SELL";
  status: "OPEN" | "CLOSED";
  opened_at: number;
  closed_at: number | null;
  entry_price: number;
  take_profit_price: number;
  stop_loss_price: number;
  close_price: number | null;
  current_price: number;
  quantity: number;
  notional: number;
  leverage: number;
  margin_used: number;
  realized_pnl: number;
  unrealized_pnl: number;
  fees: number;
  funding_costs: number;
  slippage_bps_per_side: number;
  take_profit_usd: number;
  stop_loss_usd: number;
  risk_reward_ratio: number;
  regime_at_entry?: string;
  blockers_rejected: string[];
  blocker_details: MockTrade["blockers"];
  confidence_score: number;
  required_threshold: number;
  parameters_used: MockTradingConfig;
  exit_reason: MockTrade["exitReason"];
  pnl_value: number;
  raw_trade: MockTrade;
  created_at: string;
  updated_at: string;
};

export type MockStrategySignalDoc = {
  account_key: string;
  strategy_id: number;
  strategy_name: string;
  strategy_family: string;
  symbol: string;
  side: "BUY" | "SELL" | "NO_SIGNAL";
  timeframe: string;
  entry_price: number;
  confidence_score: number;
  market_regime?: string;
  indicator_snapshot: Record<string, unknown>;
  take_profit?: number;
  stop_loss?: number;
  expected_profit_usd?: number;
  expected_loss_usd?: number;
  risk_reward?: number;
  entry_reason: string;
  timestamp: number;
  created_at: string;
};

export type MockRegimeSnapshotDoc = RegimeSnapshot & {
  account_key: string;
  created_at: string;
  updated_at: string;
};

export type MockStrategyScoreDoc = {
  account_key: string;
  strategy_id: number;
  strategy_name: string;
  strategy_family?: string;
  overall_score: number;
  current_regime_score: number;
  confidence_rating: StrategyScore["confidenceRating"];
  rank: number;
  regime_rank: number;
  metrics: StrategyScore["metrics"];
  scored_at: number;
  created_at: string;
  updated_at: string;
};

export type MockStrategyScoreHistoryDoc = MockStrategyScoreDoc & {
  history_id: string;
};

export type MockEquityCurvePointDoc = {
  account_key: string;
  timestamp: number;
  equity: number;
  realized_pnl: number;
  unrealized_pnl: number;
  drawdown_pct: number;
  daily_pnl?: number;
  regime?: string;
  created_at: string;
  updated_at: string;
};

export type MockDailyPnlHistoryDoc = {
  account_key: string;
  day: number;
  pnl: number;
  trade_count: number;
  regime?: string;
  created_at: string;
  updated_at: string;
};

export interface RecentSignalQuery {
  strategy_id?: number;
  family?: string;
  regime?: string;
  since?: number;
  limit?: number;
}

export interface EquityCurvePoint {
  timestamp: number;
  equity: number;
  realizedPnl: number;
  unrealizedPnl: number;
  drawdownPct: number;
  dailyPnl?: number;
  regime?: string;
}

export interface DailyPnlPoint {
  day: number;
  pnl: number;
  tradeCount: number;
  regime?: string;
}

export type MockAccountSnapshotDoc = {
  account_key: string;
  starting_balance: number;
  cash_balance: number;
  equity: number;
  realized_pnl: number;
  unrealized_pnl: number;
  total_exposure: number;
  long_exposure?: number;
  short_exposure?: number;
  margin_used: number;
  available_balance: number;
  return_pct: number;
  peak_equity: number;
  max_drawdown_pct: number;
  open_trade_count: number;
  closed_trade_count: number;
  timestamp: number;
  config: MockTradingConfig;
  created_at: string;
};

export type MockStrategyAnalyticsDoc = {
  account_key: string;
  generated_at: number;
  analytics: MockTradeAnalytics;
  config: MockTradingConfig;
  created_at: string;
};

export type MockTradeLogDoc = {
  account_key: string;
  ts: number;
  event: MockTradeLogEvent;
  strategy_id?: number;
  strategy_name?: string;
  side?: "BUY" | "SELL";
  price?: number;
  entry_price?: number;
  exit_price?: number;
  exit_reason?: MockTrade["exitReason"];
  ignored_blockers: string[];
  pnl?: number;
  notional?: number;
  message?: string;
  trade_id?: string;
  payload?: Record<string, unknown>;
  created_at: string;
};

export type MockEngineConfigDoc = {
  account_key: string;
  config: MockTradingConfig;
  updated_at: string;
};

let indexesEnsured = false;

async function ensureMockIndexes(db: Db): Promise<void> {
  if (indexesEnsured) return;
  const trades = db.collection<MockTradeDoc>(MOCK_TRADES_COLLECTION);
  const snapshots = db.collection<MockAccountSnapshotDoc>(MOCK_ACCOUNT_SNAPSHOTS_COLLECTION);
  const analytics = db.collection<MockStrategyAnalyticsDoc>(MOCK_STRATEGY_ANALYTICS_COLLECTION);
  const logs = db.collection<MockTradeLogDoc>(MOCK_TRADE_LOGS_COLLECTION);
  const config = db.collection<MockEngineConfigDoc>(MOCK_ENGINE_CONFIG_COLLECTION);
  const signals = db.collection<MockStrategySignalDoc>(MOCK_STRATEGY_SIGNALS_COLLECTION);
  const regimes = db.collection<MockRegimeSnapshotDoc>(MOCK_REGIME_SNAPSHOTS_COLLECTION);
  const scores = db.collection<MockStrategyScoreDoc>(MOCK_STRATEGY_SCORES_COLLECTION);
  const scoreHistory = db.collection<MockStrategyScoreHistoryDoc>(MOCK_STRATEGY_SCORE_HISTORY_COLLECTION);
  const equity = db.collection<MockEquityCurvePointDoc>(MOCK_EQUITY_CURVE_COLLECTION);
  const dailyPnl = db.collection<MockDailyPnlHistoryDoc>(MOCK_DAILY_PNL_HISTORY_COLLECTION);

  await Promise.all([
    trades.createIndex({ trade_id: 1 }, { unique: true, name: "uniq_mock_trade_id" }),
    trades.createIndex({ account_key: 1, status: 1, opened_at: -1 }, { name: "by_account_status_opened" }),
    trades.createIndex({ account_key: 1, strategy_id: 1, opened_at: -1 }, { name: "by_account_strategy_opened" }),
    trades.createIndex({ account_key: 1, strategy_family: 1, opened_at: -1 }, { name: "by_account_family_opened" }),
    trades.createIndex({ account_key: 1, opened_at: -1 }, { name: "by_account_opened" }),
    trades.createIndex({ account_key: 1, closed_at: -1 }, { name: "by_account_closed" }),
    trades.createIndex({ account_key: 1, side: 1, opened_at: -1 }, { name: "by_account_side_opened" }),
    trades.createIndex({ account_key: 1, blockers_rejected: 1, opened_at: -1 }, { name: "by_account_blocker_opened" }),
    snapshots.createIndex({ account_key: 1, timestamp: -1 }, { name: "by_account_snapshot_time" }),
    analytics.createIndex({ account_key: 1, generated_at: -1 }, { name: "by_account_analytics_time" }),
    logs.createIndex({ account_key: 1, ts: -1 }, { name: "by_account_log_time" }),
    logs.createIndex({ account_key: 1, event: 1, ts: -1 }, { name: "by_account_log_event" }),
    config.createIndex({ account_key: 1 }, { unique: true, name: "uniq_mock_config_account" }),
    signals.createIndex({ account_key: 1, timestamp: -1 }, { name: "by_account_signal_time" }),
    signals.createIndex({ account_key: 1, strategy_id: 1, timestamp: -1 }, { name: "by_account_signal_strategy" }),
    signals.createIndex({ account_key: 1, strategy_family: 1, timestamp: -1 }, { name: "by_account_signal_family" }),
    signals.createIndex({ account_key: 1, market_regime: 1, timestamp: -1 }, { name: "by_account_signal_regime" }),
    regimes.createIndex({ account_key: 1, timestamp: -1 }, { unique: true, name: "uniq_account_regime_time" }),
    regimes.createIndex({ account_key: 1, regime: 1, timestamp: -1 }, { name: "by_account_regime" }),
    scores.createIndex({ account_key: 1, strategy_id: 1 }, { unique: true, name: "uniq_account_strategy_score" }),
    scores.createIndex({ account_key: 1, rank: 1 }, { name: "by_account_score_rank" }),
    scores.createIndex({ account_key: 1, current_regime_score: -1 }, { name: "by_account_regime_score" }),
    scoreHistory.createIndex({ history_id: 1 }, { unique: true, name: "uniq_score_history_id" }),
    scoreHistory.createIndex({ account_key: 1, strategy_id: 1, scored_at: -1 }, { name: "by_account_strategy_score_history" }),
    scoreHistory.createIndex({ account_key: 1, rank: 1, scored_at: -1 }, { name: "by_account_ranking_history" }),
    equity.createIndex({ account_key: 1, timestamp: -1 }, { unique: true, name: "uniq_account_equity_time" }),
    equity.createIndex({ account_key: 1, regime: 1, timestamp: -1 }, { name: "by_account_equity_regime" }),
    dailyPnl.createIndex({ account_key: 1, day: -1 }, { unique: true, name: "uniq_account_daily_pnl" }),
    dailyPnl.createIndex({ account_key: 1, regime: 1, day: -1 }, { name: "by_account_daily_pnl_regime" }),
  ].map((p) => p.catch(() => {})));
  indexesEnsured = true;
}

async function collections(): Promise<{
  trades: Collection<MockTradeDoc>;
  snapshots: Collection<MockAccountSnapshotDoc>;
  analytics: Collection<MockStrategyAnalyticsDoc>;
  logs: Collection<MockTradeLogDoc>;
  config: Collection<MockEngineConfigDoc>;
  signals: Collection<MockStrategySignalDoc>;
  regimes: Collection<MockRegimeSnapshotDoc>;
  scores: Collection<MockStrategyScoreDoc>;
  scoreHistory: Collection<MockStrategyScoreHistoryDoc>;
  equity: Collection<MockEquityCurvePointDoc>;
  dailyPnl: Collection<MockDailyPnlHistoryDoc>;
}> {
  const db = await getDb();
  await ensureMockIndexes(db);
  return {
    trades: db.collection<MockTradeDoc>(MOCK_TRADES_COLLECTION),
    snapshots: db.collection<MockAccountSnapshotDoc>(MOCK_ACCOUNT_SNAPSHOTS_COLLECTION),
    analytics: db.collection<MockStrategyAnalyticsDoc>(MOCK_STRATEGY_ANALYTICS_COLLECTION),
    logs: db.collection<MockTradeLogDoc>(MOCK_TRADE_LOGS_COLLECTION),
    config: db.collection<MockEngineConfigDoc>(MOCK_ENGINE_CONFIG_COLLECTION),
    signals: db.collection<MockStrategySignalDoc>(MOCK_STRATEGY_SIGNALS_COLLECTION),
    regimes: db.collection<MockRegimeSnapshotDoc>(MOCK_REGIME_SNAPSHOTS_COLLECTION),
    scores: db.collection<MockStrategyScoreDoc>(MOCK_STRATEGY_SCORES_COLLECTION),
    scoreHistory: db.collection<MockStrategyScoreHistoryDoc>(MOCK_STRATEGY_SCORE_HISTORY_COLLECTION),
    equity: db.collection<MockEquityCurvePointDoc>(MOCK_EQUITY_CURVE_COLLECTION),
    dailyPnl: db.collection<MockDailyPnlHistoryDoc>(MOCK_DAILY_PNL_HISTORY_COLLECTION),
  };
}

function nowIso(): string {
  return new Date().toISOString();
}

function pnlValue(trade: MockTrade): number {
  return trade.status === "OPEN" ? trade.unrealizedPnl : trade.realizedPnl;
}

export function mockTradeToDoc(
  accountKey: string,
  trade: MockTrade,
  config: MockTradingConfig,
  createdAt = nowIso(),
): MockTradeDoc {
  return {
    account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY,
    trade_id: trade.id,
    trace_id: trade.traceId,
    strategy_id: trade.strategyId,
    strategy_name: trade.strategyName,
    strategy_family: strategyFamilyForTrade(trade),
    symbol: trade.symbol,
    side: trade.side,
    status: trade.status,
    opened_at: trade.openedAt,
    closed_at: trade.closedAt,
    entry_price: trade.entryPrice,
    take_profit_price: trade.takeProfitPrice,
    stop_loss_price: trade.stopLossPrice,
    close_price: trade.exitPrice,
    current_price: trade.currentPrice,
    quantity: trade.quantity,
    notional: trade.notional,
    leverage: trade.leverage,
    margin_used: trade.marginUsed,
    realized_pnl: trade.realizedPnl,
    unrealized_pnl: trade.unrealizedPnl,
    fees: trade.fees,
    funding_costs: trade.fundingCosts ?? 0,
    slippage_bps_per_side: config.slippageBpsPerSide,
    take_profit_usd: trade.takeProfitUsd,
    stop_loss_usd: trade.stopLossUsd,
    risk_reward_ratio: trade.riskRewardRatio,
    regime_at_entry: trade.regimeAtEntry,
    blockers_rejected: trade.blockers.map((blocker) => blocker.gate),
    blocker_details: trade.blockers,
    confidence_score: trade.signalScore,
    required_threshold: trade.requiredThreshold,
    parameters_used: config,
    exit_reason: trade.exitReason,
    pnl_value: pnlValue(trade),
    raw_trade: trade,
    created_at: createdAt,
    updated_at: createdAt,
  };
}

export function mockTradeFromDoc(doc: MockTradeDoc): MockTrade {
  return withMockExitFields(doc.raw_trade, normalizeMockTradingConfig(doc.parameters_used));
}

type PersistableMockLog = {
  event: MockTradeLogEvent;
  ts: number;
  tradeId?: string;
  strategyId?: number;
  strategyName?: string;
  side?: MockTrade["side"];
  price?: number;
  entryPrice?: number;
  exitPrice?: number;
  exitReason?: MockTrade["exitReason"];
  ignoredBlockers?: string[];
  rejectionCategory?: MockTradeLog["rejectionCategory"];
  signalScore?: number;
  confidenceScore?: number;
  riskRewardRatio?: number;
  pnl?: number;
  notional?: number;
  message?: string;
};

function mockLogToDoc(
  accountKey: string,
  log: PersistableMockLog,
  tradeId?: string,
): MockTradeLogDoc {
  return {
    account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY,
    ts: log.ts,
    event: log.event,
    strategy_id: log.strategyId,
    strategy_name: log.strategyName,
    side: log.side,
    price: log.price,
    entry_price: log.entryPrice,
    exit_price: log.exitPrice,
    exit_reason: log.exitReason,
    ignored_blockers: log.ignoredBlockers ?? [],
    payload: {
      rejection_category: log.rejectionCategory,
      signal_score: log.signalScore,
      confidence_score: log.confidenceScore,
      risk_reward_ratio: log.riskRewardRatio,
    },
    pnl: log.pnl,
    notional: log.notional,
    message: log.message,
    trade_id: log.tradeId ?? tradeId,
    created_at: nowIso(),
  };
}

function mockLogFromDoc(doc: MockTradeLogDoc): MockTradeLog {
  return {
    ts: doc.ts,
    event:
      doc.event === "MOCK_TRADE_TP_HIT" ||
      doc.event === "MOCK_TRADE_SL_HIT" ||
      doc.event === "MOCK_TRADE_CLOSED" ||
      doc.event === "MOCK_TRADE_LIMIT_REACHED" ||
      doc.event === "MOCK_TRADE_REJECTED"
        ? doc.event
        : "MOCK_TRADE_CREATED",
    tradeId: doc.trade_id,
    strategyId: doc.strategy_id ?? 0,
    strategyName: doc.strategy_name ?? "Mock Trading",
    side: doc.side ?? "BUY",
    price: doc.price ?? 0,
    entryPrice: doc.entry_price,
    exitPrice: doc.exit_price,
    exitReason: doc.exit_reason,
    ignoredBlockers: doc.ignored_blockers,
    rejectionCategory: typeof doc.payload?.rejection_category === "string" ? doc.payload.rejection_category as MockTradeLog["rejectionCategory"] : undefined,
    signalScore: typeof doc.payload?.signal_score === "number" ? doc.payload.signal_score : undefined,
    confidenceScore: typeof doc.payload?.confidence_score === "number" ? doc.payload.confidence_score : undefined,
    riskRewardRatio: typeof doc.payload?.risk_reward_ratio === "number" ? doc.payload.risk_reward_ratio : undefined,
    pnl: doc.pnl,
    notional: doc.notional,
  };
}

export async function upsertMockTrade(
  accountKey: string,
  trade: MockTrade,
  config: MockTradingConfig,
  event: "MOCK_TRADE_CREATED" | "MOCK_TRADE_UPDATED" | "MOCK_TRADE_CLOSED",
): Promise<{ upsertedCount: number; modifiedCount: number; matchedCount: number }> {
  const { trades, logs, config: configCol } = await collections();
  const existing = await trades.findOne({ trade_id: trade.id });
  const doc = mockTradeToDoc(accountKey, trade, config, existing?.created_at ?? nowIso());
  doc.updated_at = nowIso();
  const { created_at, ...docWithoutCreatedAt } = doc;
  const result = await trades.updateOne(
    { trade_id: trade.id },
    { $set: docWithoutCreatedAt, $setOnInsert: { created_at } },
    { upsert: true },
  );
  await configCol.updateOne(
    { account_key: accountKey },
    { $set: { account_key: accountKey, config, updated_at: nowIso() } },
    { upsert: true },
  );
  const log = event === "MOCK_TRADE_CLOSED"
    ? logForMockTradeClosed(trade)
    : {
        ts: event === "MOCK_TRADE_CREATED" ? trade.openedAt : Date.now(),
        event,
        strategyId: trade.strategyId,
        strategyName: trade.strategyName,
        side: trade.side,
        price: trade.currentPrice,
        ignoredBlockers: [],
        pnl: pnlValue(trade),
        notional: trade.notional,
      };
  await logs.insertOne(mockLogToDoc(accountKey, log, trade.id));
  return {
    upsertedCount: result.upsertedCount ?? 0,
    modifiedCount: result.modifiedCount ?? 0,
    matchedCount: result.matchedCount ?? 0,
  };
}

export async function getMockTrade(accountKey: string, tradeId: string): Promise<MockTrade | null> {
  const { trades } = await collections();
  const doc = await trades.findOne({ account_key: accountKey, trade_id: tradeId });
  return doc ? mockTradeFromDoc(doc) : null;
}

function sortForQuery(sort: MockTradeListQuery["sort"]): Sort {
  switch (sort) {
    case "most_profitable":
      return { pnl_value: -1, opened_at: -1 };
    case "least_profitable":
      return { pnl_value: 1, opened_at: -1 };
    case "oldest":
      return { opened_at: 1 };
    case "newest":
    default:
      return { opened_at: -1 };
  }
}

function addMongoAndCondition(filter: Filter<MockTradeDoc>, condition: Filter<MockTradeDoc>): void {
  filter.$and = [...(filter.$and ?? []), condition];
}

function mockTradeAgeDurationMsExpression(nowMs: number): Record<string, unknown> {
  return {
    $subtract: [
      {
        $cond: [
          { $eq: ["$status", "OPEN"] },
          nowMs,
          { $ifNull: ["$closed_at", nowMs] },
        ],
      },
      "$opened_at",
    ],
  };
}

function mockTradeAgeConditionForQuery(query: MockTradeListQuery, nowMs: number): Filter<MockTradeDoc> | null {
  if (!query.age_mode) return null;

  const ageMs = mockTradeAgeDurationMsExpression(nowMs);
  if (query.age_mode === "less" && query.age_max_minutes != null) {
    return { $expr: { $lt: [ageMs, query.age_max_minutes * 60_000] } } as Filter<MockTradeDoc>;
  }
  if (query.age_mode === "more" && query.age_min_minutes != null) {
    return { $expr: { $gt: [ageMs, query.age_min_minutes * 60_000] } } as Filter<MockTradeDoc>;
  }
  if (query.age_mode === "between" && query.age_min_minutes != null && query.age_max_minutes != null) {
    return {
      $expr: {
        $and: [
          { $gte: [ageMs, query.age_min_minutes * 60_000] },
          { $lte: [ageMs, query.age_max_minutes * 60_000] },
        ],
      },
    } as Filter<MockTradeDoc>;
  }
  return null;
}

export function mockTradeMongoFilterForQuery(
  query: MockTradeListQuery,
  nowMs: number = Date.now(),
): Filter<MockTradeDoc> {
  const filter: Filter<MockTradeDoc> = { account_key: query.account_key };
  if (query.status) filter.status = query.status;
  if (query.side) filter.side = query.side;
  if (query.strategy_id != null) filter.strategy_id = query.strategy_id;
  if (query.strategy_family) filter.strategy_family = query.strategy_family;
  if (query.blocker_gate) filter.blockers_rejected = query.blocker_gate;
  if (query.profitability === "profit") filter.pnl_value = { $gt: 0 };
  if (query.profitability === "loss") filter.pnl_value = { $lt: 0 };
  const ageCondition = mockTradeAgeConditionForQuery(query, nowMs);
  if (ageCondition) addMongoAndCondition(filter, ageCondition);
  return filter;
}

export async function listMockTrades(query: MockTradeListQuery): Promise<{
  trades: MockTrade[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}> {
  const { trades } = await collections();
  const filter = mockTradeMongoFilterForQuery(query);
  const skip = (query.page - 1) * query.limit;
  const [docs, total] = await Promise.all([
    trades.find(filter).sort(sortForQuery(query.sort)).skip(skip).limit(query.limit).toArray(),
    trades.countDocuments(filter),
  ]);
  return {
    trades: docs.map(mockTradeFromDoc),
    total,
    page: query.page,
    limit: query.limit,
    totalPages: Math.max(1, Math.ceil(total / query.limit)),
  };
}

export async function closeMockTradeInMongo(args: {
  accountKey: string;
  tradeId: string;
  price: number;
  closedAt: number;
  config: MockTradingConfig;
}): Promise<MockTrade | null> {
  const existing = await getMockTrade(args.accountKey, args.tradeId);
  if (!existing) return null;
  const closed = closeMockTrade(existing, args.price, args.closedAt, args.config);
  await upsertMockTrade(args.accountKey, closed, args.config, "MOCK_TRADE_CLOSED");
  return closed;
}

export async function insertMockAccountSnapshot(
  accountKey: string,
  account: MockAccountState,
  config: MockTradingConfig,
): Promise<MockAccountSnapshotDoc> {
  const { snapshots, config: configCol, logs } = await collections();
  const doc: MockAccountSnapshotDoc = {
    account_key: accountKey,
    starting_balance: account.startingBalance,
    cash_balance: account.cashBalance,
    equity: account.equity,
    realized_pnl: account.realizedPnl,
    unrealized_pnl: account.unrealizedPnl,
    total_exposure: account.exposure,
    margin_used: account.marginUsed,
    available_balance: account.availableBalance,
    return_pct: account.returnPct,
    peak_equity: account.peakEquity,
    max_drawdown_pct: account.maxDrawdownPct,
    open_trade_count: account.openCount,
    closed_trade_count: account.closedCount,
    timestamp: Date.now(),
    config,
    created_at: nowIso(),
  };
  await snapshots.insertOne(doc);
  await configCol.updateOne(
    { account_key: accountKey },
    { $set: { account_key: accountKey, config, updated_at: nowIso() } },
    { upsert: true },
  );
  await logs.insertOne({
    account_key: accountKey,
    ts: doc.timestamp,
    event: "MOCK_ACCOUNT_UPDATED",
    ignored_blockers: [],
    message: "Mock account snapshot persisted",
    created_at: doc.created_at,
  });
  return doc;
}

export async function getLatestMockAccountSnapshot(accountKey: string): Promise<{
  account: MockAccountState | null;
  config: MockTradingConfig | null;
}> {
  const { snapshots, config } = await collections();
  const [doc, cfg] = await Promise.all([
    snapshots.find({ account_key: accountKey }).sort({ timestamp: -1 }).limit(1).next(),
    config.findOne({ account_key: accountKey }),
  ]);
  if (!doc) {
    return { account: null, config: cfg?.config ? normalizeMockTradingConfig(cfg.config) : null };
  }
  return {
    account: {
      startingBalance: doc.starting_balance,
      cashBalance: doc.cash_balance,
      equity: doc.equity,
      realizedPnl: doc.realized_pnl,
      unrealizedPnl: doc.unrealized_pnl,
      exposure: doc.total_exposure,
      longExposure: doc.long_exposure ?? 0,
      shortExposure: doc.short_exposure ?? 0,
      marginUsed: doc.margin_used,
      availableBalance: doc.available_balance,
      returnPct: doc.return_pct,
      peakEquity: doc.peak_equity,
      maxDrawdownPct: doc.max_drawdown_pct,
      openCount: doc.open_trade_count,
      closedCount: doc.closed_trade_count,
    },
    config: normalizeMockTradingConfig(cfg?.config ?? doc.config),
  };
}

export async function getMockAnalyticsSummary(
  accountKey: string,
  config: MockTradingConfig,
): Promise<MockTradeAnalytics> {
  const { trades, analytics } = await collections();
  const docs = await trades.find({ account_key: accountKey }).sort({ opened_at: 1 }).limit(50_000).toArray();
  const rawTrades = docs.map(mockTradeFromDoc);
  const summary = computeAnalytics(rawTrades);
  await analytics.insertOne({
    account_key: accountKey,
    generated_at: Date.now(),
    analytics: summary,
    config,
    created_at: nowIso(),
  });
  return summary;
}

export async function listMockLogs(query: MockLogListQuery): Promise<{
  logs: MockTradeLog[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}> {
  const { logs } = await collections();
  const filter: Filter<MockTradeLogDoc> = { account_key: query.account_key };
  if (query.event) filter.event = query.event;
  const skip = (query.page - 1) * query.limit;
  const [docs, total] = await Promise.all([
    logs.find(filter).sort({ ts: -1 }).skip(skip).limit(query.limit).toArray(),
    logs.countDocuments(filter),
  ]);
  return {
    logs: docs.map(mockLogFromDoc),
    total,
    page: query.page,
    limit: query.limit,
    totalPages: Math.max(1, Math.ceil(total / query.limit)),
  };
}

export async function insertStrategySignals(
  accountKey: string,
  signalDocs: Array<Omit<MockStrategySignalDoc, "account_key" | "created_at">>,
): Promise<number> {
  if (signalDocs.length === 0) return 0;
  const { signals } = await collections();
  const createdAt = nowIso();
  const docs = signalDocs.map((doc) => ({
    ...doc,
    account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY,
    created_at: createdAt,
  }));
  const result = await signals.insertMany(docs, { ordered: false });
  return result.insertedCount;
}

export async function listRecentSignals(
  accountKey: string,
  query: RecentSignalQuery = {},
): Promise<MockStrategySignalDoc[]> {
  const { signals } = await collections();
  const filter: Filter<MockStrategySignalDoc> = { account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY };
  if (query.strategy_id != null) filter.strategy_id = query.strategy_id;
  if (query.family) filter.strategy_family = query.family;
  if (query.regime) filter.market_regime = query.regime;
  if (query.since != null) filter.timestamp = { $gte: query.since };
  return signals
    .find(filter)
    .sort({ timestamp: -1 })
    .limit(Math.max(1, Math.min(1_000, query.limit ?? 200)))
    .toArray();
}

export async function upsertRegimeSnapshot(
  accountKey: string,
  snapshot: RegimeSnapshot,
): Promise<MockRegimeSnapshotDoc> {
  const { regimes } = await collections();
  const now = nowIso();
  const doc: MockRegimeSnapshotDoc = {
    ...snapshot,
    account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY,
    created_at: now,
    updated_at: now,
  };
  const { created_at: _rca, ...regimeWithoutCreatedAt } = doc;
  await regimes.updateOne(
    { account_key: doc.account_key, timestamp: doc.timestamp },
    { $set: { ...regimeWithoutCreatedAt, updated_at: now }, $setOnInsert: { created_at: _rca } },
    { upsert: true },
  );
  return doc;
}

export async function getLatestRegimeSnapshot(accountKey: string): Promise<MockRegimeSnapshotDoc | null> {
  const { regimes } = await collections();
  return regimes
    .find({ account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY })
    .sort({ timestamp: -1 })
    .limit(1)
    .next();
}

export async function batchUpsertStrategyScores(
  accountKey: string,
  scoresInput: readonly StrategyScore[],
): Promise<number> {
  if (scoresInput.length === 0) return 0;
  const { scores, scoreHistory } = await collections();
  const now = nowIso();
  const scoredAt = Date.now();
  const normalizedAccountKey = accountKey || DEFAULT_MOCK_ACCOUNT_KEY;
  const result = await scores.bulkWrite(
    scoresInput.map((score) => ({
      updateOne: {
        filter: { account_key: normalizedAccountKey, strategy_id: score.strategyId },
        update: {
          $set: {
            account_key: normalizedAccountKey,
            strategy_id: score.strategyId,
            strategy_name: score.strategyName,
            overall_score: score.overallScore,
            current_regime_score: score.currentRegimeScore,
            confidence_rating: score.confidenceRating,
            rank: score.rank,
            regime_rank: score.regimeRank,
            metrics: score.metrics,
            scored_at: scoredAt,
            updated_at: now,
          },
          $setOnInsert: { created_at: now },
        },
        upsert: true,
      },
    })),
  );
  await scoreHistory.insertMany(
    scoresInput.map((score) => ({
      history_id: `${normalizedAccountKey}-${score.strategyId}-${scoredAt}`,
      account_key: normalizedAccountKey,
      strategy_id: score.strategyId,
      strategy_name: score.strategyName,
      overall_score: score.overallScore,
      current_regime_score: score.currentRegimeScore,
      confidence_rating: score.confidenceRating,
      rank: score.rank,
      regime_rank: score.regimeRank,
      metrics: score.metrics,
      scored_at: scoredAt,
      created_at: now,
      updated_at: now,
    })),
    { ordered: false },
  ).catch(() => {
    // History is append-only and best-effort; latest score upsert above is authoritative.
  });
  return (result.upsertedCount ?? 0) + (result.modifiedCount ?? 0);
}

export async function listStrategyScores(
  accountKey: string,
  limit = 500,
): Promise<MockStrategyScoreDoc[]> {
  const { scores } = await collections();
  return scores
    .find({ account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY })
    .sort({ rank: 1 })
    .limit(Math.max(1, Math.min(2_000, limit)))
    .toArray();
}

export async function listStrategyScoreHistory(
  accountKey: string,
  limit = 2_000,
): Promise<MockStrategyScoreHistoryDoc[]> {
  const { scoreHistory } = await collections();
  return scoreHistory
    .find({ account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY })
    .sort({ scored_at: -1, rank: 1 })
    .limit(Math.max(1, Math.min(20_000, limit)))
    .toArray();
}

export async function appendEquityCurvePoint(
  accountKey: string,
  point: EquityCurvePoint,
): Promise<MockEquityCurvePointDoc> {
  const { equity } = await collections();
  const now = nowIso();
  const doc: MockEquityCurvePointDoc = {
    account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY,
    timestamp: point.timestamp,
    equity: point.equity,
    realized_pnl: point.realizedPnl,
    unrealized_pnl: point.unrealizedPnl,
    drawdown_pct: point.drawdownPct,
    daily_pnl: point.dailyPnl,
    regime: point.regime,
    created_at: now,
    updated_at: now,
  };
  const { created_at: _eca, ...equityWithoutCreatedAt } = doc;
  await equity.updateOne(
    { account_key: doc.account_key, timestamp: doc.timestamp },
    { $set: { ...equityWithoutCreatedAt, updated_at: now }, $setOnInsert: { created_at: _eca } },
    { upsert: true },
  );
  return doc;
}

export async function listEquityCurvePoints(
  accountKey: string,
  limit = 500,
): Promise<MockEquityCurvePointDoc[]> {
  const { equity } = await collections();
  return equity
    .find({ account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY })
    .sort({ timestamp: 1 })
    .limit(Math.max(1, Math.min(5_000, limit)))
    .toArray();
}

export async function upsertDailyPnlPoint(
  accountKey: string,
  point: DailyPnlPoint,
): Promise<MockDailyPnlHistoryDoc> {
  const { dailyPnl } = await collections();
  const now = nowIso();
  const doc: MockDailyPnlHistoryDoc = {
    account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY,
    day: point.day,
    pnl: point.pnl,
    trade_count: point.tradeCount,
    regime: point.regime,
    created_at: now,
    updated_at: now,
  };
  const { created_at: _dca, ...dailyWithoutCreatedAt } = doc;
  await dailyPnl.updateOne(
    { account_key: doc.account_key, day: doc.day },
    { $set: { ...dailyWithoutCreatedAt, updated_at: now }, $setOnInsert: { created_at: _dca } },
    { upsert: true },
  );
  return doc;
}

export async function listDailyPnlHistory(
  accountKey: string,
  limit = 500,
): Promise<MockDailyPnlHistoryDoc[]> {
  const { dailyPnl } = await collections();
  return dailyPnl
    .find({ account_key: accountKey || DEFAULT_MOCK_ACCOUNT_KEY })
    .sort({ day: 1 })
    .limit(Math.max(1, Math.min(5_000, limit)))
    .toArray();
}

export async function resetMockTradingState(accountKey: string): Promise<{
  tradesDeleted: number;
  snapshotsDeleted: number;
  analyticsDeleted: number;
  logsDeleted: number;
}> {
  const { trades, snapshots, analytics, logs, config, signals, regimes, scores, scoreHistory, equity, dailyPnl } = await collections();
  const [tradeRes, snapshotRes, analyticsRes, logRes] = await Promise.all([
    trades.deleteMany({ account_key: accountKey }),
    snapshots.deleteMany({ account_key: accountKey }),
    analytics.deleteMany({ account_key: accountKey }),
    logs.deleteMany({ account_key: accountKey }),
    config.deleteOne({ account_key: accountKey }),
    signals.deleteMany({ account_key: accountKey }),
    regimes.deleteMany({ account_key: accountKey }),
    scores.deleteMany({ account_key: accountKey }),
    scoreHistory.deleteMany({ account_key: accountKey }),
    equity.deleteMany({ account_key: accountKey }),
    dailyPnl.deleteMany({ account_key: accountKey }),
  ]);
  await logs.insertOne({
    account_key: accountKey,
    ts: Date.now(),
    event: "MOCK_TRADING_RESET",
    ignored_blockers: [],
    message: "Mock trading state reset",
    created_at: nowIso(),
  });
  return {
    tradesDeleted: tradeRes.deletedCount ?? 0,
    snapshotsDeleted: snapshotRes.deletedCount ?? 0,
    analyticsDeleted: analyticsRes.deletedCount ?? 0,
    logsDeleted: logRes.deletedCount ?? 0,
  };
}

export async function computeAccountFromMongo(accountKey: string, config: MockTradingConfig): Promise<MockAccountState> {
  const { trades } = await collections();
  const docs = await trades.find({ account_key: accountKey }).sort({ opened_at: 1 }).limit(50_000).toArray();
  return computeAccountState(docs.map(mockTradeFromDoc), config);
}
