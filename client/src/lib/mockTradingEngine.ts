/**
 * Mock Trading Engine — analysis-only twin of the live BTC FT pipeline.
 *
 * Purpose: turn EVERY raised strategy signal into a "what if" mock trade,
 * ignoring the gates that normally block entries in the production paper desk
 * (REGIME, SIGNAL, ATR_FEES, CONFIRM/QUALITY/MTF/COOLDOWN/SAME_SIDE/MAX_OPEN, …).
 * The blocker reason that *would* have stopped the trade is recorded on the
 * mock trade so we can later identify which strategies are profitable when
 * those filters are removed.
 *
 * This module is intentionally isolated from the real entry pipeline — it
 * never opens broker orders and never mutates the production paper desk.
 *
 * Pure functions; no React, no I/O. Tested in mockTradingEngine.test.ts.
 */

import type { StrategySignalTraceRow } from "@/lib/strategySignalTrace";

// ── Blocker gates that mock trading explicitly ignores ───────────────────────
// These are the production gates that the user wants to bypass for analysis.
// REJECTED rows whose gate is in this set still produce mock trades; the gate
// is recorded only as informational metadata on the mock trade.
export const MOCK_IGNORED_GATES = [
  "REGIME",        // REGIME_BLOCKING
  "SIGNAL",        // signal threshold not met
  "ATR_FEES",      // ATR vs fee hurdle
  "CONFIRM",       // confluence confirmation
  "QUALITY",       // signal quality score
  "MTF",           // multi-timeframe confluence
  "COOLDOWN",      // per-strategy cooldown
  "OCCUPIED",      // strategy already in a position
  "CATEGORY",      // per-category cap
  "SAME_SIDE",     // same-side cap
  "MAX_OPEN",      // global open-positions cap
  "MARGIN",        // margin sufficiency
  "SPREAD",        // spread filter
  "SESSION",       // UTC session window
] as const;

export type MockIgnoredGate = (typeof MOCK_IGNORED_GATES)[number];

// ── Types ────────────────────────────────────────────────────────────────────
export type MockTradeStatus = "OPEN" | "CLOSED";
export type MockSide = "BUY" | "SELL";
export type MockExitReason = "TAKE_PROFIT" | "STOP_LOSS" | "MAX_HOLD" | "MANUAL";

export interface MockTradeBlocker {
  /** Gate the strategy hit (e.g. "REGIME"). */
  gate: string;
  /** Human-readable reason captured by the trace. */
  reason: string;
}

export interface MockTrade {
  id: string;
  /** Trace row that produced this mock trade — dedup key. */
  traceId: string;
  /** Strategy ID from the production roster. */
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: MockSide;
  /** Notional in quote currency (USD) at entry. */
  notional: number;
  /** Effective quantity in base currency at entry. */
  quantity: number;
  /** Price the live ticker showed when the mock trade was created. */
  entryPrice: number;
  /** Price recorded in the trace row (signalScore tick). */
  signalPrice: number;
  signalScore: number;
  requiredThreshold: number;
  /** Blocker(s) that would have stopped the real entry. */
  blockers: MockTradeBlocker[];
  /** Status of the mock trade. */
  status: MockTradeStatus;
  /** Open / close timestamps, ms epoch. */
  openedAt: number;
  closedAt: number | null;
  /** Live mark used to compute unrealized PnL. Updated on each price tick. */
  currentPrice: number;
  /** Unrealized PnL in USD for OPEN trades, 0 once CLOSED. */
  unrealizedPnl: number;
  /** Realized PnL in USD once CLOSED. */
  realizedPnl: number;
  /** Exit reason once CLOSED. */
  exitReason: MockExitReason | null;
  /** Exit price once CLOSED. */
  exitPrice: number | null;
}

/** Configurable mock exit settings. */
export interface MockExitConfig {
  /** Take-profit percent of entry price (positive number, e.g. 1.5 = 1.5%). */
  takeProfitPct: number;
  /** Stop-loss percent of entry price (positive number, e.g. 1.0 = 1.0%). */
  stopLossPct: number;
  /** Max hold time in minutes before forced close. */
  maxHoldMinutes: number;
  /** Notional size per mock trade in USD. */
  notionalUsd: number;
}

export const DEFAULT_MOCK_EXIT: MockExitConfig = {
  takeProfitPct: 1.5,
  stopLossPct: 0.6,
  maxHoldMinutes: 45,
  notionalUsd: 100,
};

/** Pre-strategy override of TP/SL (e.g. mirror the production strategy def). */
export interface StrategyExitOverride {
  strategyId: number;
  takeProfitPct?: number;
  stopLossPct?: number;
  maxHoldMinutes?: number;
}

// ── Trace → mock trade ───────────────────────────────────────────────────────

/**
 * Did this trace row raise a directional strategy signal?
 *
 * A signal is "raised" when the strategy produced a side (LONG/SHORT) AND a
 * positive signal score — regardless of whether it later passed downstream
 * gates. Mock trading creates a trade for every raised signal.
 */
export function isStrategySignalRaised(row: StrategySignalTraceRow): boolean {
  if (!row.side) return false;
  if (!Number.isFinite(row.signalScore) || row.signalScore <= 0) return false;
  if (row.status === "EVALUATED") {
    // EVALUATED rows reached scoring but did not fire — only treat as a raised
    // signal if the score is non-trivial (> 0). The user's explicit ask is that
    // ANY raised signal is mocked, including those blocked by SIGNAL threshold.
    return true;
  }
  return true;
}

/** Map LONG/SHORT trace side → BUY/SELL order side. */
export function traceSideToOrderSide(side: "LONG" | "SHORT"): MockSide {
  return side === "LONG" ? "BUY" : "SELL";
}

/**
 * Extract blocker metadata from a trace row.
 * Returns an empty array if the row was OPENED in the real pipeline.
 */
export function blockersFromTraceRow(row: StrategySignalTraceRow): MockTradeBlocker[] {
  if (row.status === "OPENED") return [];
  if (row.status === "CANDIDATE") return [];
  return [{ gate: row.gate, reason: row.reason }];
}

function resolveExit(
  config: MockExitConfig,
  override: StrategyExitOverride | undefined,
): MockExitConfig {
  if (!override) return config;
  return {
    takeProfitPct: override.takeProfitPct ?? config.takeProfitPct,
    stopLossPct: override.stopLossPct ?? config.stopLossPct,
    maxHoldMinutes: override.maxHoldMinutes ?? config.maxHoldMinutes,
    notionalUsd: config.notionalUsd,
  };
}

/**
 * Build a MockTrade from a raised signal trace row + a live price quote.
 * Returns null when the row did not raise a signal (no side or zero score).
 */
export function buildMockTradeFromTrace(args: {
  row: StrategySignalTraceRow;
  currentPrice: number;
  config: MockExitConfig;
  now: number;
  override?: StrategyExitOverride;
}): MockTrade | null {
  const { row, currentPrice, config, now } = args;
  if (!isStrategySignalRaised(row)) return null;
  if (!Number.isFinite(currentPrice) || currentPrice <= 0) return null;
  const side = traceSideToOrderSide(row.side as "LONG" | "SHORT");
  const exit = resolveExit(config, args.override);
  const quantity = exit.notionalUsd / currentPrice;
  return {
    id: `mock-${row.traceId}`,
    traceId: row.traceId,
    strategyId: row.strategyId,
    strategyName: row.strategyName,
    symbol: row.symbol,
    side,
    notional: exit.notionalUsd,
    quantity,
    entryPrice: currentPrice,
    signalPrice: currentPrice,
    signalScore: row.signalScore,
    requiredThreshold: row.requiredThreshold,
    blockers: blockersFromTraceRow(row),
    status: "OPEN",
    openedAt: now,
    closedAt: null,
    currentPrice,
    unrealizedPnl: 0,
    realizedPnl: 0,
    exitReason: null,
    exitPrice: null,
  };
}

// ── PnL maths ────────────────────────────────────────────────────────────────

/** Compute PnL in USD for a long/short position vs current price. */
export function computeMockPnl(
  side: MockSide,
  entryPrice: number,
  exitPrice: number,
  quantity: number,
): number {
  if (!Number.isFinite(entryPrice) || !Number.isFinite(exitPrice)) return 0;
  if (!Number.isFinite(quantity) || quantity <= 0) return 0;
  const delta = side === "BUY" ? exitPrice - entryPrice : entryPrice - exitPrice;
  return delta * quantity;
}

/**
 * Apply a price tick to a single trade and (optionally) exit it if TP/SL/maxHold
 * conditions are met. Returns a new trade object — caller should replace the
 * old one in their store. Idempotent for CLOSED trades.
 */
export function applyPriceTickToTrade(args: {
  trade: MockTrade;
  price: number;
  config: MockExitConfig;
  override?: StrategyExitOverride;
  now: number;
}): MockTrade {
  const { trade, price, config, now, override } = args;
  if (trade.status === "CLOSED") return trade;
  if (!Number.isFinite(price) || price <= 0) return trade;

  const exit = resolveExit(config, override);
  const ageMin = (now - trade.openedAt) / 60_000;
  const tpThresholdLong = trade.entryPrice * (1 + exit.takeProfitPct / 100);
  const tpThresholdShort = trade.entryPrice * (1 - exit.takeProfitPct / 100);
  const slThresholdLong = trade.entryPrice * (1 - exit.stopLossPct / 100);
  const slThresholdShort = trade.entryPrice * (1 + exit.stopLossPct / 100);

  let exitReason: MockExitReason | null = null;
  if (trade.side === "BUY") {
    if (price >= tpThresholdLong) exitReason = "TAKE_PROFIT";
    else if (price <= slThresholdLong) exitReason = "STOP_LOSS";
  } else {
    if (price <= tpThresholdShort) exitReason = "TAKE_PROFIT";
    else if (price >= slThresholdShort) exitReason = "STOP_LOSS";
  }
  if (!exitReason && ageMin >= exit.maxHoldMinutes) exitReason = "MAX_HOLD";

  if (exitReason) {
    const realized = computeMockPnl(trade.side, trade.entryPrice, price, trade.quantity);
    return {
      ...trade,
      status: "CLOSED",
      closedAt: now,
      currentPrice: price,
      unrealizedPnl: 0,
      realizedPnl: realized,
      exitReason,
      exitPrice: price,
    };
  }

  const unrealized = computeMockPnl(trade.side, trade.entryPrice, price, trade.quantity);
  return { ...trade, currentPrice: price, unrealizedPnl: unrealized };
}

/** Manually close an open trade at the given price. */
export function closeMockTrade(trade: MockTrade, price: number, now: number): MockTrade {
  if (trade.status === "CLOSED") return trade;
  const realized = computeMockPnl(trade.side, trade.entryPrice, price, trade.quantity);
  return {
    ...trade,
    status: "CLOSED",
    closedAt: now,
    currentPrice: price,
    unrealizedPnl: 0,
    realizedPnl: realized,
    exitReason: "MANUAL",
    exitPrice: price,
  };
}

// ── Analytics ────────────────────────────────────────────────────────────────
export interface MockTradeAnalytics {
  totalTrades: number;
  openTrades: number;
  closedTrades: number;
  winRate: number;          // 0..1, over CLOSED trades only
  totalPnl: number;         // realized + unrealized
  realizedPnl: number;
  unrealizedPnl: number;
  averagePnl: number;       // realized PnL / closed count
  profitFactor: number | null; // sum(wins) / sum(losses). null if no losers.
  perStrategy: MockStrategyAggregate[];
  perBlocker: MockBlockerAggregate[];
}

export interface MockStrategyAggregate {
  strategyId: number;
  strategyName: string;
  total: number;
  open: number;
  closed: number;
  wins: number;
  losses: number;
  winRate: number;
  totalPnl: number;
  realizedPnl: number;
  unrealizedPnl: number;
}

export interface MockBlockerAggregate {
  gate: string;
  total: number;
  wins: number;
  losses: number;
  winRate: number;
  totalPnl: number;
}

export function computeAnalytics(trades: readonly MockTrade[]): MockTradeAnalytics {
  let open = 0;
  let closed = 0;
  let realized = 0;
  let unrealized = 0;
  let wins = 0;
  let losses = 0;
  let grossWins = 0;
  let grossLosses = 0;

  const stratMap = new Map<number, MockStrategyAggregate>();
  const blockerMap = new Map<string, MockBlockerAggregate>();

  for (const t of trades) {
    if (t.status === "OPEN") {
      open++;
      unrealized += t.unrealizedPnl;
    } else {
      closed++;
      realized += t.realizedPnl;
      if (t.realizedPnl > 0) {
        wins++;
        grossWins += t.realizedPnl;
      } else if (t.realizedPnl < 0) {
        losses++;
        grossLosses += Math.abs(t.realizedPnl);
      }
    }

    let strat = stratMap.get(t.strategyId);
    if (!strat) {
      strat = {
        strategyId: t.strategyId,
        strategyName: t.strategyName,
        total: 0,
        open: 0,
        closed: 0,
        wins: 0,
        losses: 0,
        winRate: 0,
        totalPnl: 0,
        realizedPnl: 0,
        unrealizedPnl: 0,
      };
      stratMap.set(t.strategyId, strat);
    }
    strat.total++;
    if (t.status === "OPEN") {
      strat.open++;
      strat.unrealizedPnl += t.unrealizedPnl;
    } else {
      strat.closed++;
      strat.realizedPnl += t.realizedPnl;
      if (t.realizedPnl > 0) strat.wins++;
      else if (t.realizedPnl < 0) strat.losses++;
    }
    strat.totalPnl = strat.realizedPnl + strat.unrealizedPnl;
    strat.winRate = strat.closed > 0 ? strat.wins / strat.closed : 0;

    for (const b of t.blockers) {
      let bk = blockerMap.get(b.gate);
      if (!bk) {
        bk = { gate: b.gate, total: 0, wins: 0, losses: 0, winRate: 0, totalPnl: 0 };
        blockerMap.set(b.gate, bk);
      }
      bk.total++;
      bk.totalPnl += t.realizedPnl + t.unrealizedPnl;
      if (t.status === "CLOSED") {
        if (t.realizedPnl > 0) bk.wins++;
        else if (t.realizedPnl < 0) bk.losses++;
      }
      const decided = bk.wins + bk.losses;
      bk.winRate = decided > 0 ? bk.wins / decided : 0;
    }
  }

  const decided = wins + losses;
  return {
    totalTrades: trades.length,
    openTrades: open,
    closedTrades: closed,
    winRate: decided > 0 ? wins / decided : 0,
    totalPnl: realized + unrealized,
    realizedPnl: realized,
    unrealizedPnl: unrealized,
    averagePnl: closed > 0 ? realized / closed : 0,
    profitFactor: grossLosses > 0 ? grossWins / grossLosses : null,
    perStrategy: [...stratMap.values()].sort((a, b) => b.totalPnl - a.totalPnl),
    perBlocker: [...blockerMap.values()].sort((a, b) => b.total - a.total),
  };
}

// ── Filtering ────────────────────────────────────────────────────────────────
export interface MockTradeFilter {
  strategyId?: number | null;
  side?: MockSide | null;
  status?: MockTradeStatus | null;
  blockerGate?: string | null;
  /** "profit" → realizedPnl + unrealizedPnl > 0, "loss" → < 0. */
  profitability?: "profit" | "loss" | null;
}

export function filterMockTrades(
  trades: readonly MockTrade[],
  filter: MockTradeFilter,
): MockTrade[] {
  return trades.filter((t) => {
    if (filter.strategyId != null && t.strategyId !== filter.strategyId) return false;
    if (filter.side && t.side !== filter.side) return false;
    if (filter.status && t.status !== filter.status) return false;
    if (filter.blockerGate && !t.blockers.some((b) => b.gate === filter.blockerGate)) return false;
    if (filter.profitability) {
      const pnl = t.realizedPnl + t.unrealizedPnl;
      if (filter.profitability === "profit" && pnl <= 0) return false;
      if (filter.profitability === "loss" && pnl >= 0) return false;
    }
    return true;
  });
}

// ── Logging helper ───────────────────────────────────────────────────────────
export interface MockTradeLog {
  ts: number;
  event: "MOCK_TRADE_CREATED" | "MOCK_TRADE_CLOSED";
  strategyId: number;
  strategyName: string;
  side: MockSide;
  price: number;
  ignoredBlockers: string[];
  pnl?: number;
}

export function logForMockTradeCreated(trade: MockTrade): MockTradeLog {
  return {
    ts: trade.openedAt,
    event: "MOCK_TRADE_CREATED",
    strategyId: trade.strategyId,
    strategyName: trade.strategyName,
    side: trade.side,
    price: trade.entryPrice,
    ignoredBlockers: trade.blockers.map((b) => b.gate),
  };
}

export function logForMockTradeClosed(trade: MockTrade): MockTradeLog {
  return {
    ts: trade.closedAt ?? Date.now(),
    event: "MOCK_TRADE_CLOSED",
    strategyId: trade.strategyId,
    strategyName: trade.strategyName,
    side: trade.side,
    price: trade.exitPrice ?? trade.currentPrice,
    ignoredBlockers: trade.blockers.map((b) => b.gate),
    pnl: trade.realizedPnl,
  };
}
