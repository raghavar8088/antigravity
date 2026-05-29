import type { Collection, Db, Filter, Sort } from "mongodb";
import { getDb } from "@/lib/mongoTradesClient";
import {
  computeAccountState,
  computeAnalytics,
  closeMockTrade,
  logForMockTradeClosed,
  normalizeMockTradingConfig,
  withMockFixedDollarExitFields,
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

export const MOCK_TRADES_COLLECTION = "mock_trades";
export const MOCK_ACCOUNT_SNAPSHOTS_COLLECTION = "mock_account_snapshots";
export const MOCK_STRATEGY_ANALYTICS_COLLECTION = "mock_strategy_analytics";
export const MOCK_TRADE_LOGS_COLLECTION = "mock_trade_logs";
export const MOCK_ENGINE_CONFIG_COLLECTION = "mock_engine_config";

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
  slippage_bps_per_side: number;
  take_profit_usd: number;
  stop_loss_usd: number;
  risk_reward_ratio: number;
  blockers_ignored: string[];
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

export type MockAccountSnapshotDoc = {
  account_key: string;
  starting_balance: number;
  cash_balance: number;
  equity: number;
  realized_pnl: number;
  unrealized_pnl: number;
  total_exposure: number;
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

  await Promise.all([
    trades.createIndex({ trade_id: 1 }, { unique: true, name: "uniq_mock_trade_id" }),
    trades.createIndex({ account_key: 1, status: 1, opened_at: -1 }, { name: "by_account_status_opened" }),
    trades.createIndex({ account_key: 1, strategy_id: 1, opened_at: -1 }, { name: "by_account_strategy_opened" }),
    trades.createIndex({ account_key: 1, strategy_family: 1, opened_at: -1 }, { name: "by_account_family_opened" }),
    trades.createIndex({ account_key: 1, opened_at: -1 }, { name: "by_account_opened" }),
    trades.createIndex({ account_key: 1, closed_at: -1 }, { name: "by_account_closed" }),
    trades.createIndex({ account_key: 1, side: 1, opened_at: -1 }, { name: "by_account_side_opened" }),
    trades.createIndex({ account_key: 1, blockers_ignored: 1, opened_at: -1 }, { name: "by_account_blocker_opened" }),
    snapshots.createIndex({ account_key: 1, timestamp: -1 }, { name: "by_account_snapshot_time" }),
    analytics.createIndex({ account_key: 1, generated_at: -1 }, { name: "by_account_analytics_time" }),
    logs.createIndex({ account_key: 1, ts: -1 }, { name: "by_account_log_time" }),
    logs.createIndex({ account_key: 1, event: 1, ts: -1 }, { name: "by_account_log_event" }),
    config.createIndex({ account_key: 1 }, { unique: true, name: "uniq_mock_config_account" }),
  ]);
  indexesEnsured = true;
}

async function collections(): Promise<{
  trades: Collection<MockTradeDoc>;
  snapshots: Collection<MockAccountSnapshotDoc>;
  analytics: Collection<MockStrategyAnalyticsDoc>;
  logs: Collection<MockTradeLogDoc>;
  config: Collection<MockEngineConfigDoc>;
}> {
  const db = await getDb();
  await ensureMockIndexes(db);
  return {
    trades: db.collection<MockTradeDoc>(MOCK_TRADES_COLLECTION),
    snapshots: db.collection<MockAccountSnapshotDoc>(MOCK_ACCOUNT_SNAPSHOTS_COLLECTION),
    analytics: db.collection<MockStrategyAnalyticsDoc>(MOCK_STRATEGY_ANALYTICS_COLLECTION),
    logs: db.collection<MockTradeLogDoc>(MOCK_TRADE_LOGS_COLLECTION),
    config: db.collection<MockEngineConfigDoc>(MOCK_ENGINE_CONFIG_COLLECTION),
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
    slippage_bps_per_side: config.slippageBpsPerSide,
    take_profit_usd: trade.takeProfitUsd,
    stop_loss_usd: trade.stopLossUsd,
    risk_reward_ratio: trade.riskRewardRatio,
    blockers_ignored: trade.blockers.map((blocker) => blocker.gate),
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
  return withMockFixedDollarExitFields(doc.raw_trade, normalizeMockTradingConfig(doc.parameters_used));
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
      doc.event === "MOCK_TRADE_LIMIT_REACHED"
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
  const result = await trades.updateOne(
    { trade_id: trade.id },
    { $set: doc, $setOnInsert: { created_at: doc.created_at } },
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
        ignoredBlockers: trade.blockers.map((blocker) => blocker.gate),
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

function filterForQuery(query: MockTradeListQuery): Filter<MockTradeDoc> {
  const filter: Filter<MockTradeDoc> = { account_key: query.account_key };
  if (query.status) filter.status = query.status;
  if (query.side) filter.side = query.side;
  if (query.strategy_id != null) filter.strategy_id = query.strategy_id;
  if (query.strategy_family) filter.strategy_family = query.strategy_family;
  if (query.blocker_gate) filter.blockers_ignored = query.blocker_gate;
  if (query.profitability === "profit") filter.pnl_value = { $gt: 0 };
  if (query.profitability === "loss") filter.pnl_value = { $lt: 0 };
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
  const filter = filterForQuery(query);
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

export async function resetMockTradingState(accountKey: string): Promise<{
  tradesDeleted: number;
  snapshotsDeleted: number;
  analyticsDeleted: number;
  logsDeleted: number;
}> {
  const { trades, snapshots, analytics, logs, config } = await collections();
  const [tradeRes, snapshotRes, analyticsRes, logRes] = await Promise.all([
    trades.deleteMany({ account_key: accountKey }),
    snapshots.deleteMany({ account_key: accountKey }),
    analytics.deleteMany({ account_key: accountKey }),
    logs.deleteMany({ account_key: accountKey }),
    config.deleteOne({ account_key: accountKey }),
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
