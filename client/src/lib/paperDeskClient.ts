/**
 * paperDeskClient.ts — MongoDB query layer for Go Engine paper trading collections.
 *
 * Reads from the 8 collections written exclusively by the Go Engine:
 *   paper_state, paper_trades, paper_positions, paper_orders,
 *   equity_curve, daily_pnl_history, strategy_scores, strategy_health
 *
 * Security: account_key filter is ALWAYS the caller-supplied key derived from
 * the JWT session (auth.ctx.userId). It is never accepted from client input.
 *
 * Connection: reuses the same MongoClient pool from mongoTradesClient via getDb().
 */

import { getDb } from "@/lib/mongoTradesClient";
import type { Db, Document } from "mongodb";

// ── Collection names (mirror Go Engine paperpersist constants) ──────────────

const COL_PAPER_STATE = "paper_state";
const COL_PAPER_TRADES = "paper_trades";
const COL_PAPER_POSITIONS = "paper_positions";
const COL_PAPER_ORDERS = "paper_orders";
const COL_EQUITY_CURVE = "equity_curve";
const COL_DAILY_PNL = "daily_pnl_history";
const COL_STRATEGY_SCORES = "strategy_scores";
const COL_STRATEGY_HEALTH = "strategy_health";

// ── TypeScript shapes (mirror Go Engine structs) ────────────────────────────

export type PaperStateDoc = {
  account_key: string;
  balance: number;
  equity: number;
  unrealized_pnl: number;
  realized_pnl: number;
  peak_equity: number;
  current_drawdown: number;
  max_drawdown: number;
  open_position_count: number;
  total_exposure_btc: number;
  long_exposure_btc: number;
  short_exposure_btc: number;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  total_fees: number;
  session_start: string;
  snapped_at: string;
  updated_at: string;
};

export type PaperTradeDoc = {
  account_key: string;
  client_trade_id: string;
  strategy_id: string;
  symbol: string;
  side: string;
  entry_price: number;
  exit_price: number;
  quantity: number;
  gross_pnl: number;
  fees: number;
  entry_fee?: number;
  exit_fee?: number;
  total_fee?: number;
  net_pnl: number;
  exit_reason: string;
  entry_at: string;
  exit_at: string;
  closed_at: string;
  slippage_bps: number;
  latency_ms: number;
  fill_quality: number;
  exec_confidence: number;
  risk: {
    kelly_fraction: number;
    recommended_size_btc: number;
    pre_trade_approved: boolean;
    drawdown_scaling: number;
    portfolio_heat: number;
  };
};

export type PaperPositionDoc = {
  account_key: string;
  position_id: string;
  order_id: string;
  strategy_id: string;
  symbol: string;
  side: string;
  entry_price: number;
  size: number;
  stop_loss: number;
  take_profit: number;
  status: "OPEN" | "CLOSED";
  opened_at: string;
  exit_price?: number;
  net_pnl?: number;
  closed_at?: string;
  exit_reason?: string;
};

export type PaperOrderDoc = {
  account_key: string;
  order_id: string;
  strategy_id: string;
  symbol: string;
  side: string;
  transition_from: string;
  transition_to: string;
  transition_at: string;
  recorded_at: string;
  risk_approved: boolean;
  fill_price?: number;
  fill_size?: number;
  position_id?: string;
  net_pnl?: number;
  reason?: string;
};

export type EquityCurveDoc = {
  account_key: string;
  ts: string;
  equity: number;
  balance: number;
  unrealized_pnl: number;
  realized_pnl: number;
  drawdown_pct: number;
  open_positions: number;
  btc_price: number;
};

export type DailyPnLDoc = {
  account_key: string;
  date: string;
  opening_balance: number;
  closing_balance: number;
  realized_pnl: number;
  fees: number;
  net_pnl: number;
  trade_count: number;
  win_count: number;
  loss_count: number;
  max_drawdown_pct: number;
  max_equity: number;
  min_equity: number;
  sealed_at: string;
};

export type StrategyScoreDoc = {
  account_key: string;
  strategy_id: string;
  win_rate: number;
  expectancy: number;
  profit_factor: number;
  max_drawdown: number;
  sample_size: number;
  total_pnl: number;
  avg_win: number;
  avg_loss: number;
  computed_at: string;
};

export type StrategyHealthDoc = {
  account_key: string;
  strategy_id: string;
  health_status: "HEALTHY" | "WARNING" | "CRITICAL" | "INSUFFICIENT_DATA";
  health_reasons: string[];
  score: StrategyScoreDoc;
  computed_at: string;
};

// ── Helper ──────────────────────────────────────────────────────────────────

function strip(doc: Document): Document {
  const rest = { ...doc };
  delete (rest as Record<string, unknown>)._id;
  return rest;
}

// ── Query functions ─────────────────────────────────────────────────────────

/** Returns the current account state snapshot (singleton per account_key). */
export async function getPaperState(accountKey: string): Promise<PaperStateDoc | null> {
  let db: Db;
  try {
    db = await getDb();
  } catch {
    return null;
  }
  const doc = await db
    .collection(COL_PAPER_STATE)
    .findOne({ account_key: accountKey });
  return doc ? (strip(doc) as PaperStateDoc) : null;
}

export type ListTradesOpts = {
  accountKey: string;
  limit?: number;
  offset?: number;
  cursor?: string; // ISO timestamp — return trades with closed_at < cursor
  strategyId?: string;
  symbol?: string;
  side?: string;
  sortBy?: "closed_at" | "net_pnl";
  sortDir?: "asc" | "desc";
};

/** Returns paginated closed trades sorted by closed_at desc (default). */
export async function listPaperTrades(opts: ListTradesOpts): Promise<PaperTradeDoc[]> {
  const limit = Math.min(opts.limit ?? 50, 200);
  const skip = opts.offset ?? 0;
  const sortField = opts.sortBy ?? "closed_at";
  const sortDir = opts.sortDir === "asc" ? 1 : -1;

  const filter: Record<string, unknown> = { account_key: opts.accountKey };
  if (opts.cursor) filter.closed_at = { $lt: new Date(opts.cursor) };
  if (opts.strategyId) filter.strategy_id = opts.strategyId;
  if (opts.symbol) filter.symbol = opts.symbol;
  if (opts.side) filter.side = opts.side;

  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }

  const docs = await db
    .collection(COL_PAPER_TRADES)
    .find(filter)
    .sort({ [sortField]: sortDir })
    .skip(skip)
    .limit(limit)
    .toArray();

  return docs.map((d) => strip(d) as PaperTradeDoc);
}

export type ClosedTradeStats = {
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  realized_pnl: number;
  gross_pnl: number;
  total_fees: number;
  entry_fees: number;
  exit_fees: number;
};

function closedTradesFilter(accountKey: string, side?: string): Record<string, unknown> {
  const filter: Record<string, unknown> = { account_key: accountKey };
  if (side) filter.side = side;
  return filter;
}

const EMPTY_CLOSED_TRADE_STATS: ClosedTradeStats = {
  total_trades: 0,
  winning_trades: 0,
  losing_trades: 0,
  win_rate: 0,
  realized_pnl: 0,
  gross_pnl: 0,
  total_fees: 0,
  entry_fees: 0,
  exit_fees: 0,
};

/** Authoritative closed-trade stats from paper_trades (matches Trade History pagination total). */
export async function getClosedTradeStats(
  accountKey: string,
  opts?: { side?: string; closedAfter?: Date; closedBefore?: Date },
): Promise<ClosedTradeStats> {
  try {
    const db = await getDb();
    const filter: Record<string, unknown> = closedTradesFilter(accountKey, opts?.side);
    if (opts?.closedAfter || opts?.closedBefore) {
      const closedAt: Record<string, string> = {};
      if (opts.closedAfter) closedAt.$gte = opts.closedAfter.toISOString();
      if (opts.closedBefore) closedAt.$lt = opts.closedBefore.toISOString();
      filter.closed_at = closedAt;
    }
    const rows = await db
      .collection(COL_PAPER_TRADES)
      .aggregate([
        { $match: filter },
        {
          $group: {
            _id: null,
            total_trades: { $sum: 1 },
            winning_trades: { $sum: { $cond: [{ $gt: ["$net_pnl", 0] }, 1, 0] } },
            losing_trades: { $sum: { $cond: [{ $lte: ["$net_pnl", 0] }, 1, 0] } },
            realized_pnl: { $sum: { $ifNull: ["$net_pnl", 0] } },
            gross_pnl: { $sum: { $ifNull: ["$gross_pnl", 0] } },
            total_fees: {
              $sum: {
                $ifNull: [
                  "$total_fee",
                  { $ifNull: ["$fees", { $add: [{ $ifNull: ["$entry_fee", 0] }, { $ifNull: ["$exit_fee", 0] }] }] },
                ],
              },
            },
            entry_fees: { $sum: { $ifNull: ["$entry_fee", 0] } },
            exit_fees: { $sum: { $ifNull: ["$exit_fee", 0] } },
          },
        },
      ])
      .toArray();

    const row = rows[0];
    if (!row) return EMPTY_CLOSED_TRADE_STATS;

    const total = Number(row.total_trades) || 0;
    const wins = Number(row.winning_trades) || 0;
    const totalFees = Number(row.total_fees) || 0;
    const entryFees = Number(row.entry_fees) || 0;
    const exitFees = Number(row.exit_fees) || 0;
    return {
      total_trades: total,
      winning_trades: wins,
      losing_trades: Number(row.losing_trades) || 0,
      win_rate: total > 0 ? wins / total : 0,
      realized_pnl: Number(row.realized_pnl) || 0,
      gross_pnl: Number(row.gross_pnl) || 0,
      total_fees: totalFees,
      entry_fees: entryFees > 0 ? entryFees : totalFees / 2,
      exit_fees: exitFees > 0 ? exitFees : totalFees / 2,
    };
  } catch {
    return EMPTY_CLOSED_TRADE_STATS;
  }
}

/**
 * Merge MongoDB-authoritative closed-trade accounting into paper_state.
 * Realized PnL, fees, and trade counts always come from paper_trades aggregation.
 */
export function mergeClosedTradeStatsIntoState(
  state: PaperStateDoc | null,
  stats: ClosedTradeStats,
  overrides?: {
    unrealized_pnl?: number;
    exposure?: { long_exposure_btc: number; short_exposure_btc: number; gross_exposure_btc: number };
    drawdown?: { peak_equity: number; current_drawdown: number; max_drawdown: number };
  },
): PaperStateDoc | null {
  if (!state) return null;
  const unrealized = overrides?.unrealized_pnl ?? state.unrealized_pnl;
  const balance = state.balance;
  return {
    ...state,
    balance,
    equity: balance + unrealized,
    unrealized_pnl: unrealized,
    realized_pnl: stats.realized_pnl,
    total_fees: stats.total_fees,
    total_trades: stats.total_trades,
    winning_trades: stats.winning_trades,
    losing_trades: stats.losing_trades,
    win_rate: stats.win_rate,
    total_exposure_btc: overrides?.exposure?.gross_exposure_btc ?? state.total_exposure_btc,
    long_exposure_btc: overrides?.exposure?.long_exposure_btc ?? state.long_exposure_btc,
    short_exposure_btc: overrides?.exposure?.short_exposure_btc ?? state.short_exposure_btc,
    peak_equity: overrides?.drawdown?.peak_equity ?? state.peak_equity,
    current_drawdown: overrides?.drawdown?.current_drawdown ?? state.current_drawdown,
    max_drawdown: overrides?.drawdown?.max_drawdown ?? state.max_drawdown,
  };
}

export async function countPaperTrades(accountKey: string, side?: string): Promise<number> {
  try {
    const db = await getDb();
    return await db.collection(COL_PAPER_TRADES).countDocuments(closedTradesFilter(accountKey, side));
  } catch {
    return 0;
  }
}

/** Returns all OPEN positions for the account. */
export async function listOpenPositions(accountKey: string): Promise<PaperPositionDoc[]> {
  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }
  const docs = await db
    .collection(COL_PAPER_POSITIONS)
    .find({ account_key: accountKey, status: "OPEN" })
    .sort({ opened_at: -1 })
    .toArray();
  return docs.map((d) => strip(d) as PaperPositionDoc);
}

/** Returns all positions (OPEN + CLOSED) with optional status filter. */
export async function listPositions(
  accountKey: string,
  status?: "OPEN" | "CLOSED",
  limit = 100,
): Promise<PaperPositionDoc[]> {
  const filter: Record<string, unknown> = { account_key: accountKey };
  if (status) filter.status = status;
  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }
  const docs = await db
    .collection(COL_PAPER_POSITIONS)
    .find(filter)
    .sort({ opened_at: -1 })
    .limit(limit)
    .toArray();
  return docs.map((d) => strip(d) as PaperPositionDoc);
}

export type ListOrdersOpts = {
  accountKey: string;
  orderId?: string;
  strategyId?: string;
  limit?: number;
};

/** Returns OMS order lifecycle history. */
export async function listPaperOrders(opts: ListOrdersOpts): Promise<PaperOrderDoc[]> {
  const limit = Math.min(opts.limit ?? 100, 500);
  const filter: Record<string, unknown> = { account_key: opts.accountKey };
  if (opts.orderId) filter.order_id = opts.orderId;
  if (opts.strategyId) filter.strategy_id = opts.strategyId;

  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }

  const docs = await db
    .collection(COL_PAPER_ORDERS)
    .find(filter)
    .sort({ recorded_at: -1 })
    .limit(limit)
    .toArray();
  return docs.map((d) => strip(d) as PaperOrderDoc);
}

/** Returns equity curve points. limitPoints defaults to 288 (24h at 5-min intervals). */
export async function getEquityCurve(
  accountKey: string,
  limitPoints = 288,
): Promise<EquityCurveDoc[]> {
  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }
  const docs = await db
    .collection(COL_EQUITY_CURVE)
    .find({ account_key: accountKey })
    .sort({ ts: -1 })
    .limit(Math.min(limitPoints, 2016)) // max 7 days at 5-min
    .toArray();
  // Return chronological order for chart rendering
  return docs.reverse().map((d) => strip(d) as EquityCurveDoc);
}

/** Returns recent daily PnL history records. */
export async function getDailyPnLHistory(
  accountKey: string,
  days = 30,
): Promise<DailyPnLDoc[]> {
  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }
  const docs = await db
    .collection(COL_DAILY_PNL)
    .find({ account_key: accountKey })
    .sort({ date: -1 })
    .limit(Math.min(days, 90))
    .toArray();
  return docs.map((d) => strip(d) as DailyPnLDoc);
}

/** Returns strategy scores sorted by total_pnl desc. */
export async function listStrategyScores(
  accountKey: string,
  limit = 100,
): Promise<StrategyScoreDoc[]> {
  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }
  const docs = await db
    .collection(COL_STRATEGY_SCORES)
    .find({ account_key: accountKey })
    .sort({ total_pnl: -1 })
    .limit(Math.min(limit, 600))
    .toArray();
  return docs.map((d) => strip(d) as StrategyScoreDoc);
}

/** Returns strategy health docs. Optional status filter. */
export async function listStrategyHealth(
  accountKey: string,
  status?: string,
  limit = 200,
): Promise<StrategyHealthDoc[]> {
  const filter: Record<string, unknown> = { account_key: accountKey };
  if (status) filter.health_status = status;

  let db: Db;
  try {
    db = await getDb();
  } catch {
    return [];
  }
  const docs = await db
    .collection(COL_STRATEGY_HEALTH)
    .find(filter)
    .sort({ computed_at: -1 })
    .limit(Math.min(limit, 600))
    .toArray();
  return docs.map((d) => strip(d) as StrategyHealthDoc);
}

/** Health summary: count by status. */
export async function getStrategyHealthSummary(accountKey: string): Promise<{
  healthy: number;
  warning: number;
  critical: number;
  insufficient_data: number;
  total: number;
}> {
  try {
    const db = await getDb();
    const agg = await db
      .collection(COL_STRATEGY_HEALTH)
      .aggregate([
        { $match: { account_key: accountKey } },
        { $group: { _id: "$health_status", count: { $sum: 1 } } },
      ])
      .toArray();

    const counts = { healthy: 0, warning: 0, critical: 0, insufficient_data: 0, total: 0 };
    for (const row of agg) {
      const n = (row.count as number) ?? 0;
      counts.total += n;
      switch (row._id) {
        case "HEALTHY": counts.healthy += n; break;
        case "WARNING": counts.warning += n; break;
        case "CRITICAL": counts.critical += n; break;
        case "INSUFFICIENT_DATA": counts.insufficient_data += n; break;
      }
    }
    return counts;
  } catch {
    return { healthy: 0, warning: 0, critical: 0, insufficient_data: 0, total: 0 };
  }
}
