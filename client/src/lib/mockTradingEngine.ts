/**
 * Mock Trading Engine — analysis-only $1,000,000 paper account that mirrors
 * the production BTC FT desk's sizing, fees, slippage, and PnL math but
 * deliberately removes every strategy gate so we can answer "which strategies
 * would be profitable if blockers were removed?".
 *
 * Mirrored from `futuresPaperMath`:
 *   - Linear gross PnL (paperLinearGrossPnl)
 *   - Round-trip taker fees (paperRoundTripTakerFees)
 *   - Entry / exit slippage in bps (paperApply{Entry,Exit}Slippage)
 *   - Margin required = notional / leverage (paperMarginRequired)
 *   - Fixed-%-of-equity sizing (deskFixedNotionalPctOfEquity)
 *   - Vol-aware risk sizing (paperNotionalForTargetRisk)
 *
 * NEVER imports paperOms, broker, Delta, or Angel One code paths — this is a
 * pure simulation. All inputs are observability data (signal trace rows + a
 * live Binance mark). All outputs stay in-memory + localStorage.
 *
 * Pure functions; no React, no I/O. Tested in mockTradingEngine.test.ts.
 */

import {
  paperApplyEntrySlippage,
  paperApplyExitSlippage,
  paperLinearGrossPnl,
  paperMarginRequired,
  paperRoundTripTakerFees,
} from "@/lib/futuresPaperMath";
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
export type MockSizingMode = "fixed_pct_equity" | "fixed_notional" | "risk_pct_equity";
export const DEFAULT_MAX_OPEN_MOCK_TRADES = 5_000;

export interface MockTradeBlocker {
  /** Gate the strategy hit (e.g. "REGIME"). */
  gate: string;
  /** Human-readable reason captured by the trace. */
  reason: string;
}

/**
 * Mock Trading account + sizing + execution-cost config.
 *
 * Production-default values mirror the live BTC FT desk:
 *   - 1% of equity per trade (deskFixedNotionalPctOfEquity, when set)
 *   - 25x leverage (BTC FT)
 *   - 0.05% per-leg taker fee (Delta India futures)
 *   - 5 bps per-leg slippage (DESK_SLIPPAGE_BPS_DEFAULT)
 */
export interface MockTradingConfig {
  /** Simulated starting balance in USD. Default $1,000,000. */
  startingBalanceUsd: number;
  /** Sizing strategy. */
  sizingMode: MockSizingMode;
  /** % of current equity for "fixed_pct_equity" mode (1 = 1%). */
  fixedPctOfEquity: number;
  /** USD notional for "fixed_notional" mode. */
  fixedNotionalUsd: number;
  /** % of equity to risk per trade for "risk_pct_equity" mode (1 = 1%). */
  riskPctOfEquity: number;
  /** Position leverage. Used for margin sizing. Defaults to 25x (BTC FT). */
  leverage: number;
  /** Fixed net profit target per mock trade in USD. */
  takeProfitUsd: number;
  /** Fixed net loss target per mock trade in USD. */
  stopLossUsd: number;
  /** Take profit percent of entry price. */
  takeProfitPct: number;
  /** Stop loss percent of entry price. */
  stopLossPct: number;
  /** Max hold time in minutes before forced close. */
  maxHoldMinutes: number;
  /** Per-leg taker fee fraction (0.0005 = 0.05%). */
  takerFeePct: number;
  /** Per-leg slippage in bps (5 = 0.05%). */
  slippageBpsPerSide: number;
  /** Mock-only cap for simultaneous OPEN mock trades. */
  maxOpenMockTrades: number;
}

export const DEFAULT_MOCK_TRADING_CONFIG: MockTradingConfig = {
  startingBalanceUsd: 1_000_000,
  sizingMode: "fixed_pct_equity",
  fixedPctOfEquity: 1,
  fixedNotionalUsd: 10_000,
  riskPctOfEquity: 1,
  leverage: 25,
  takeProfitUsd: 10,
  stopLossUsd: 5,
  takeProfitPct: 1.5,
  stopLossPct: 0.6,
  maxHoldMinutes: 45,
  takerFeePct: 0.0005,
  slippageBpsPerSide: 5,
  maxOpenMockTrades: DEFAULT_MAX_OPEN_MOCK_TRADES,
};

/**
 * Per-strategy override of TP/SL (mirrors the production strategy def's
 * slPct/tpPct/holdMinutes when known).
 */
export interface StrategyExitOverride {
  strategyId: number;
  takeProfitPct?: number;
  stopLossPct?: number;
  maxHoldMinutes?: number;
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
  /** Notional in USD at entry. */
  notional: number;
  /** Effective quantity in base currency at entry (notional / entryPrice). */
  quantity: number;
  /** Leverage at open. Margin = notional / leverage. */
  leverage: number;
  /** Initial margin reserved at open. */
  marginUsed: number;
  /** Mark price used for sizing — pre-slippage. */
  signalPrice: number;
  /** Fill price after entry slippage. */
  entryPrice: number;
  /** Live mark price that realizes the fixed TP amount after exit slippage and fees. */
  takeProfitPrice: number;
  /** Live mark price that realizes the fixed SL amount after exit slippage and fees. */
  stopLossPrice: number;
  /** Fixed net profit target in USD captured at entry. */
  takeProfitUsd: number;
  /** Fixed net loss target in USD captured at entry. */
  stopLossUsd: number;
  /** Reward divided by risk, based on the fixed dollar TP/SL amounts. */
  riskRewardRatio: number;
  signalScore: number;
  requiredThreshold: number;
  /** Blocker(s) that would have stopped the real entry. */
  blockers: MockTradeBlocker[];
  status: MockTradeStatus;
  openedAt: number;
  closedAt: number | null;
  /** Live mark — updated on each price tick. */
  currentPrice: number;
  /** Net unrealized PnL (gross − accrued fee debt) in USD for OPEN trades. */
  unrealizedPnl: number;
  /** Net realized PnL in USD once CLOSED. */
  realizedPnl: number;
  /** Round-trip fees in USD once CLOSED. */
  fees: number;
  exitReason: MockExitReason | null;
  /** Fill price after exit slippage. */
  exitPrice: number | null;
  /** Strategy family (e.g. "TrendFollowing") — set only for research pack trades. */
  strategyFamily?: string;
  /** Signal confidence 0–100 — set only for research pack trades. */
  confidenceScore?: number;
  /** Strategy parameters used to create the signal — set only for research pack trades. */
  strategyParams?: Record<string, number | string>;
  /** True for trades produced by the mock research runner (IDs 1000–1499). */
  researchPack?: boolean;
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
  return true;
}

/** Map LONG/SHORT trace side → BUY/SELL order side. */
export function traceSideToOrderSide(side: "LONG" | "SHORT"): MockSide {
  return side === "LONG" ? "BUY" : "SELL";
}

/** Mock-side ↔ paper-math side. */
function toPaperSide(side: MockSide): "LONG" | "SHORT" {
  return side === "BUY" ? "LONG" : "SHORT";
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
  config: MockTradingConfig,
  override: StrategyExitOverride | undefined,
): { takeProfitUsd: number; stopLossUsd: number; stopLossPct: number; maxHoldMinutes: number } {
  const takeProfitUsd = Number.isFinite(config.takeProfitUsd) && config.takeProfitUsd > 0
    ? config.takeProfitUsd
    : DEFAULT_MOCK_TRADING_CONFIG.takeProfitUsd;
  const stopLossUsd = Number.isFinite(config.stopLossUsd) && config.stopLossUsd > 0
    ? config.stopLossUsd
    : DEFAULT_MOCK_TRADING_CONFIG.stopLossUsd;
  const stopLossPct = Number.isFinite(config.stopLossPct) && config.stopLossPct > 0
    ? config.stopLossPct
    : DEFAULT_MOCK_TRADING_CONFIG.stopLossPct;
  if (!override) {
    return {
      takeProfitUsd,
      stopLossUsd,
      stopLossPct,
      maxHoldMinutes: config.maxHoldMinutes,
    };
  }
  return {
    takeProfitUsd,
    stopLossUsd,
    stopLossPct,
    maxHoldMinutes: override.maxHoldMinutes ?? config.maxHoldMinutes,
  };
}

// ── Sizing ───────────────────────────────────────────────────────────────────

/**
 * Compute the notional in USD for a new mock trade given the current equity
 * and config. Mirrors the production sizing helpers in `futuresPaperMath`.
 *
 * "fixed_pct_equity": `equity * fixedPctOfEquity / 100`
 * "fixed_notional":   `fixedNotionalUsd`
 * "risk_pct_equity":  budget = `equity * riskPctOfEquity / 100`,
 *                     notional = budget / (slPct/100 + 2 * takerFeePct)
 *                     so that maximum loss at SL is roughly the budget.
 */
export function computeMockNotional(args: {
  config: MockTradingConfig;
  equity: number;
  slPct?: number;
}): number {
  const { config, equity } = args;
  const eq = Number.isFinite(equity) && equity > 0 ? equity : config.startingBalanceUsd;
  const minNotional = 50;
  const maxNotional = Math.max(minNotional, eq * 10); // cap at 10× equity for sanity
  let raw = 0;
  switch (config.sizingMode) {
    case "fixed_pct_equity":
      raw = eq * (config.fixedPctOfEquity / 100);
      break;
    case "fixed_notional":
      raw = config.fixedNotionalUsd;
      break;
    case "risk_pct_equity": {
      const sl = Number.isFinite(args.slPct) && (args.slPct ?? 0) > 0 ? (args.slPct as number) : config.stopLossPct;
      const lossPerDollar = sl / 100 + 2 * config.takerFeePct;
      const budget = eq * (config.riskPctOfEquity / 100);
      raw = lossPerDollar > 0 ? budget / lossPerDollar : 0;
      break;
    }
  }
  if (!Number.isFinite(raw) || raw <= 0) return minNotional;
  return Math.min(maxNotional, Math.max(minNotional, raw));
}

export function maxOpenMockTradesFromConfig(config: MockTradingConfig): number {
  const raw = config.maxOpenMockTrades;
  if (!Number.isFinite(raw) || raw <= 0) return DEFAULT_MAX_OPEN_MOCK_TRADES;
  return Math.max(1, Math.floor(raw));
}

export function countOpenMockTrades(trades: readonly MockTrade[]): number {
  let open = 0;
  for (const trade of trades) {
    if (trade.status === "OPEN") open++;
  }
  return open;
}

export function canOpenAdditionalMockTrade(args: {
  trades: readonly MockTrade[];
  config: MockTradingConfig;
  pendingOpenTrades?: number;
}): boolean {
  const pending = Math.max(0, Math.floor(args.pendingOpenTrades ?? 0));
  return countOpenMockTrades(args.trades) + pending < maxOpenMockTradesFromConfig(args.config);
}

/**
 * Build a MockTrade from a raised signal trace row + a live price quote.
 * Returns null when the row did not raise a signal (no side or zero score).
 *
 * `equity` is used for percent-of-equity sizing; defaults to startingBalance.
 */
export function buildMockTradeFromTrace(args: {
  row: StrategySignalTraceRow;
  currentPrice: number;
  config: MockTradingConfig;
  now: number;
  equity?: number;
  override?: StrategyExitOverride;
}): MockTrade | null {
  const { row, currentPrice, config, now, override } = args;
  if (!isStrategySignalRaised(row)) return null;
  if (!Number.isFinite(currentPrice) || currentPrice <= 0) return null;
  const equity = args.equity ?? config.startingBalanceUsd;
  const side = traceSideToOrderSide(row.side as "LONG" | "SHORT");
  const paperSide = toPaperSide(side);
  const exit = resolveExit(config, override);
  const notional = computeMockNotional({
    config,
    equity,
    slPct: exit.stopLossPct,
  });
  const lev = Math.max(1, Math.min(125, Math.floor(config.leverage)));
  const marginUsed = paperMarginRequired(notional, lev);
  const fillPrice = paperApplyEntrySlippage(paperSide, currentPrice, config.slippageBpsPerSide);
  const quantity = fillPrice > 0 ? notional / fillPrice : 0;
  const fixedDollarExits = computeMockFixedDollarExitLevels({
    side,
    entryPrice: fillPrice,
    quantity,
    notional,
    config,
    takeProfitUsd: exit.takeProfitUsd,
    stopLossUsd: exit.stopLossUsd,
  });
  return {
    id: `mock-${row.traceId}`,
    traceId: row.traceId,
    strategyId: row.strategyId,
    strategyName: row.strategyName,
    symbol: row.symbol,
    side,
    notional,
    quantity,
    leverage: lev,
    marginUsed,
    signalPrice: currentPrice,
    entryPrice: fillPrice,
    ...fixedDollarExits,
    signalScore: row.signalScore,
    requiredThreshold: row.requiredThreshold,
    blockers: blockersFromTraceRow(row),
    status: "OPEN",
    openedAt: now,
    closedAt: null,
    currentPrice: fillPrice,
    unrealizedPnl: 0,
    realizedPnl: 0,
    fees: 0,
    exitReason: null,
    exitPrice: null,
  };
}

export interface MockResearchSignalInput {
  strategyId: number;
  strategyName: string;
  strategyFamily: string;
  side: MockSide;
  confidenceScore: number;
  params: Record<string, number | string>;
  evaluatedAt: number;
}

/**
 * Build a mock-only trade from the 500-strategy research runner.
 *
 * This path deliberately avoids StrategySignalTraceRow and every production
 * broker/order API. The research runner feeds only metadata and a live BTC mark;
 * the output is the same MockTrade shape used by the mock engine's PnL lifecycle.
 */
export function buildMockTradeFromResearchSignal(args: {
  signal: MockResearchSignalInput;
  currentPrice: number;
  config: MockTradingConfig;
  now: number;
  equity?: number;
}): MockTrade | null {
  const { signal, currentPrice, config, now } = args;
  if (!Number.isFinite(currentPrice) || currentPrice <= 0) return null;
  if (signal.side !== "BUY" && signal.side !== "SELL") return null;
  if (!Number.isFinite(signal.confidenceScore) || signal.confidenceScore <= 0) return null;

  const equity = args.equity ?? config.startingBalanceUsd;
  const paperSide = toPaperSide(signal.side);
  const exit = resolveExit(config, undefined);
  const notional = computeMockNotional({
    config,
    equity,
    slPct: exit.stopLossPct,
  });
  const lev = Math.max(1, Math.min(125, Math.floor(config.leverage)));
  const marginUsed = paperMarginRequired(notional, lev);
  const fillPrice = paperApplyEntrySlippage(paperSide, currentPrice, config.slippageBpsPerSide);
  const quantity = fillPrice > 0 ? notional / fillPrice : 0;
  const fixedDollarExits = computeMockFixedDollarExitLevels({
    side: signal.side,
    entryPrice: fillPrice,
    quantity,
    notional,
    config,
    takeProfitUsd: exit.takeProfitUsd,
    stopLossUsd: exit.stopLossUsd,
  });
  const safeConfidence = Math.max(0, Math.min(100, signal.confidenceScore));
  const traceId = `mock-research-${signal.strategyId}-${signal.evaluatedAt}-${signal.side}`;

  return {
    id: `mock-${traceId}`,
    traceId,
    strategyId: signal.strategyId,
    strategyName: signal.strategyName,
    symbol: "BTCUSD",
    side: signal.side,
    notional,
    quantity,
    leverage: lev,
    marginUsed,
    signalPrice: currentPrice,
    entryPrice: fillPrice,
    ...fixedDollarExits,
    signalScore: safeConfidence,
    requiredThreshold: 0,
    blockers: [],
    status: "OPEN",
    openedAt: now,
    closedAt: null,
    currentPrice: fillPrice,
    unrealizedPnl: 0,
    realizedPnl: 0,
    fees: 0,
    exitReason: null,
    exitPrice: null,
    strategyFamily: signal.strategyFamily,
    confidenceScore: safeConfidence,
    strategyParams: signal.params,
    researchPack: true,
  };
}

// ── PnL maths ────────────────────────────────────────────────────────────────

/**
 * Compute linear gross PnL in USD using the production paper-math helper.
 * Sign convention: positive = trade is in profit at the given exit price.
 */
export function computeMockPnl(
  side: MockSide,
  entryPrice: number,
  exitPrice: number,
  quantity: number,
): number {
  if (!Number.isFinite(entryPrice) || !Number.isFinite(exitPrice)) return 0;
  if (!Number.isFinite(quantity) || quantity <= 0) return 0;
  // Quantity × entry = notional (BUY) — use paperLinearGrossPnl with notional.
  const notional = quantity * entryPrice;
  return paperLinearGrossPnl(entryPrice, exitPrice, notional, toPaperSide(side));
}

export function computeMockExitMarkForNetPnl(args: {
  side: MockSide;
  entryPrice: number;
  quantity: number;
  notional: number;
  targetNetPnl: number;
  config: MockTradingConfig;
}): number {
  const { side, entryPrice, quantity, notional, targetNetPnl, config } = args;
  if (!Number.isFinite(entryPrice) || entryPrice <= 0) return 0;
  if (!Number.isFinite(quantity) || quantity <= 0) return 0;
  if (!Number.isFinite(notional) || notional <= 0) return 0;
  if (!Number.isFinite(targetNetPnl)) return 0;

  const fees = paperRoundTripTakerFees(notional, config.takerFeePct);
  const grossTarget = targetNetPnl + fees;
  const slippage = Number.isFinite(config.slippageBpsPerSide) && config.slippageBpsPerSide > 0
    ? config.slippageBpsPerSide / 10_000
    : 0;

  if (side === "BUY") {
    const exitFill = entryPrice + grossTarget / quantity;
    const mark = slippage < 1 ? exitFill / (1 - slippage) : exitFill;
    return Number.isFinite(mark) && mark > 0 ? mark : 0;
  }

  const exitFill = entryPrice - grossTarget / quantity;
  const mark = exitFill / (1 + slippage);
  return Number.isFinite(mark) && mark > 0 ? mark : 0;
}

export function computeMockFixedDollarExitLevels(args: {
  side: MockSide;
  entryPrice: number;
  quantity: number;
  notional: number;
  config: MockTradingConfig;
  takeProfitUsd: number;
  stopLossUsd: number;
}): {
  takeProfitPrice: number;
  stopLossPrice: number;
  takeProfitUsd: number;
  stopLossUsd: number;
  riskRewardRatio: number;
} {
  const takeProfitUsd = Number.isFinite(args.takeProfitUsd) && args.takeProfitUsd > 0
    ? args.takeProfitUsd
    : DEFAULT_MOCK_TRADING_CONFIG.takeProfitUsd;
  const stopLossUsd = Number.isFinite(args.stopLossUsd) && args.stopLossUsd > 0
    ? args.stopLossUsd
    : DEFAULT_MOCK_TRADING_CONFIG.stopLossUsd;

  return {
    takeProfitPrice: computeMockExitMarkForNetPnl({
      side: args.side,
      entryPrice: args.entryPrice,
      quantity: args.quantity,
      notional: args.notional,
      targetNetPnl: takeProfitUsd,
      config: args.config,
    }),
    stopLossPrice: computeMockExitMarkForNetPnl({
      side: args.side,
      entryPrice: args.entryPrice,
      quantity: args.quantity,
      notional: args.notional,
      targetNetPnl: -stopLossUsd,
      config: args.config,
    }),
    takeProfitUsd,
    stopLossUsd,
    riskRewardRatio: stopLossUsd > 0 ? takeProfitUsd / stopLossUsd : 0,
  };
}

export function withMockFixedDollarExitFields(
  trade: MockTrade,
  config: MockTradingConfig,
): MockTrade {
  const existing = trade as MockTrade & Partial<Pick<
    MockTrade,
    "takeProfitPrice" | "stopLossPrice" | "takeProfitUsd" | "stopLossUsd" | "riskRewardRatio"
  >>;
  if (
    Number.isFinite(existing.takeProfitPrice) &&
    Number.isFinite(existing.stopLossPrice) &&
    Number.isFinite(existing.takeProfitUsd) &&
    Number.isFinite(existing.stopLossUsd) &&
    Number.isFinite(existing.riskRewardRatio)
  ) {
    return trade;
  }
  const exits = computeMockFixedDollarExitLevels({
    side: trade.side,
    entryPrice: trade.entryPrice,
    quantity: trade.quantity,
    notional: trade.notional,
    config,
    takeProfitUsd: existing.takeProfitUsd ?? config.takeProfitUsd,
    stopLossUsd: existing.stopLossUsd ?? config.stopLossUsd,
  });
  return { ...trade, ...exits };
}

function reachedFixedDollarTarget(args: {
  trade: MockTrade;
  price: number;
  targetPrice: number;
  targetNetPnl: number;
}): boolean {
  const { trade, price, targetPrice, targetNetPnl } = args;
  if (!Number.isFinite(targetPrice) || targetPrice <= 0) return false;
  if (targetNetPnl >= 0) {
    return trade.side === "BUY" ? price >= targetPrice : price <= targetPrice;
  }
  return trade.side === "BUY" ? price <= targetPrice : price >= targetPrice;
}

/**
 * Apply a price tick to a single trade and (optionally) exit it if TP/SL/maxHold
 * conditions are met. Returns a new trade object — caller should replace the
 * old one in their store. Idempotent for CLOSED trades.
 *
 * Unrealized PnL is gross only (no fees deducted); realized PnL on close
 * deducts the full round-trip fee at the current config's `takerFeePct`.
 */
export function applyPriceTickToTrade(args: {
  trade: MockTrade;
  price: number;
  config: MockTradingConfig;
  override?: StrategyExitOverride;
  now: number;
}): MockTrade {
  const { trade, price, config, now, override } = args;
  if (trade.status === "CLOSED") return trade;
  if (!Number.isFinite(price) || price <= 0) return trade;

  const exit = resolveExit(config, override);
  const activeTrade = withMockFixedDollarExitFields(trade, config);
  const ageMin = (now - trade.openedAt) / 60_000;

  let exitReason: MockExitReason | null = null;
  let exitPrice = price;
  if (
    reachedFixedDollarTarget({
      trade: activeTrade,
      price,
      targetPrice: activeTrade.takeProfitPrice,
      targetNetPnl: activeTrade.takeProfitUsd,
    })
  ) {
    exitReason = "TAKE_PROFIT";
    exitPrice = activeTrade.takeProfitPrice;
  } else if (
    reachedFixedDollarTarget({
      trade: activeTrade,
      price,
      targetPrice: activeTrade.stopLossPrice,
      targetNetPnl: -activeTrade.stopLossUsd,
    })
  ) {
    exitReason = "STOP_LOSS";
    exitPrice = activeTrade.stopLossPrice;
  }
  if (!exitReason && ageMin >= exit.maxHoldMinutes) exitReason = "MAX_HOLD";

  if (exitReason) {
    return finalizeClose({ trade: activeTrade, fillBeforeSlippage: exitPrice, exitReason, now, config });
  }

  // Unrealized PnL: gross + estimated round-trip fees not yet realized.
  // We surface NET unrealized so the equity card matches realized math.
  const gross = computeMockPnl(activeTrade.side, activeTrade.entryPrice, price, activeTrade.quantity);
  const feesIfClosed = paperRoundTripTakerFees(activeTrade.notional, config.takerFeePct);
  return { ...activeTrade, currentPrice: price, unrealizedPnl: gross - feesIfClosed };
}

/** Manually close an open trade at the given mark. */
export function closeMockTrade(
  trade: MockTrade,
  price: number,
  now: number,
  config: MockTradingConfig = DEFAULT_MOCK_TRADING_CONFIG,
): MockTrade {
  if (trade.status === "CLOSED") return trade;
  return finalizeClose({
    trade: withMockFixedDollarExitFields(trade, config),
    fillBeforeSlippage: price,
    exitReason: "MANUAL",
    now,
    config,
  });
}

function finalizeClose(args: {
  trade: MockTrade;
  fillBeforeSlippage: number;
  exitReason: MockExitReason;
  now: number;
  config: MockTradingConfig;
}): MockTrade {
  const { trade, fillBeforeSlippage, exitReason, now, config } = args;
  const fillPrice = paperApplyExitSlippage(
    toPaperSide(trade.side),
    fillBeforeSlippage,
    config.slippageBpsPerSide,
  );
  const gross = computeMockPnl(trade.side, trade.entryPrice, fillPrice, trade.quantity);
  const fees = paperRoundTripTakerFees(trade.notional, config.takerFeePct);
  const net = gross - fees;
  return {
    ...trade,
    status: "CLOSED",
    closedAt: now,
    currentPrice: fillPrice,
    unrealizedPnl: 0,
    realizedPnl: net,
    fees,
    exitReason,
    exitPrice: fillPrice,
  };
}

// ── Account state ────────────────────────────────────────────────────────────

/**
 * Aggregate $1,000,000 paper account state — mirrors the main paper desk's
 * equity / margin / exposure model.
 */
export interface MockAccountState {
  /** Configured starting balance. */
  startingBalance: number;
  /** Cash available after reserving margin for open positions:
   *  startingBalance + realizedPnl − sum(marginUsed of OPEN positions). */
  cashBalance: number;
  /** Equity = startingBalance + realizedPnl + unrealizedPnl. */
  equity: number;
  /** Net realized PnL across all CLOSED trades (after fees + slippage). */
  realizedPnl: number;
  /** Net unrealized PnL across all OPEN trades (after estimated round-trip fees). */
  unrealizedPnl: number;
  /** Sum of notional on OPEN trades. */
  exposure: number;
  /** Sum of marginUsed on OPEN trades (isolated). */
  marginUsed: number;
  /** equity − marginUsed. Can be negative if exposure exceeds equity. */
  availableBalance: number;
  /** (equity − startingBalance) / startingBalance, expressed as a fraction. */
  returnPct: number;
  /** Running peak realized equity for drawdown computation. */
  peakEquity: number;
  /** Worst observed (peakEquity − point)/peakEquity, expressed as a fraction. */
  maxDrawdownPct: number;
  openCount: number;
  closedCount: number;
}

/**
 * Compute the full account snapshot from the trade list.
 *
 * Drawdown is computed by walking the realized close events ordered by
 * `closedAt`. `peakEquity` is the maximum realized-equity high-water mark,
 * including the starting balance. `maxDrawdownPct` is the worst trough from
 * that running peak. Current unrealized PnL is added on top of the latest
 * realized equity for the live mark.
 */
export function computeAccountState(
  trades: readonly MockTrade[],
  config: MockTradingConfig,
): MockAccountState {
  const startingBalance = Number.isFinite(config.startingBalanceUsd) && config.startingBalanceUsd > 0
    ? config.startingBalanceUsd
    : DEFAULT_MOCK_TRADING_CONFIG.startingBalanceUsd;

  let realizedPnl = 0;
  let unrealizedPnl = 0;
  let exposure = 0;
  let marginUsed = 0;
  let openCount = 0;
  let closedCount = 0;

  const closes: { ts: number; pnl: number }[] = [];

  for (const t of trades) {
    if (t.status === "OPEN") {
      openCount++;
      unrealizedPnl += t.unrealizedPnl;
      exposure += t.notional;
      marginUsed += t.marginUsed;
    } else {
      closedCount++;
      realizedPnl += t.realizedPnl;
      if (t.closedAt != null) closes.push({ ts: t.closedAt, pnl: t.realizedPnl });
    }
  }

  // Equity high-water mark over realized history.
  closes.sort((a, b) => a.ts - b.ts);
  let runningEquity = startingBalance;
  let peakEquity = startingBalance;
  let maxDrawdownPct = 0;
  for (const c of closes) {
    runningEquity += c.pnl;
    if (runningEquity > peakEquity) peakEquity = runningEquity;
    if (peakEquity > 0) {
      const dd = (peakEquity - runningEquity) / peakEquity;
      if (dd > maxDrawdownPct) maxDrawdownPct = dd;
    }
  }

  const equity = startingBalance + realizedPnl + unrealizedPnl;
  // Include the live unrealized leg in current drawdown comparison.
  if (peakEquity > 0 && equity < peakEquity) {
    const dd = (peakEquity - equity) / peakEquity;
    if (dd > maxDrawdownPct) maxDrawdownPct = dd;
  }
  // Open-position equity also resets the peak when in profit.
  if (equity > peakEquity) peakEquity = equity;

  const cashBalance = startingBalance + realizedPnl - marginUsed;
  const availableBalance = equity - marginUsed;
  const returnPct = startingBalance > 0 ? (equity - startingBalance) / startingBalance : 0;

  return {
    startingBalance,
    cashBalance,
    equity,
    realizedPnl,
    unrealizedPnl,
    exposure,
    marginUsed,
    availableBalance,
    returnPct,
    peakEquity,
    maxDrawdownPct,
    openCount,
    closedCount,
  };
}

// ── Analytics ────────────────────────────────────────────────────────────────
export interface MockTradeAnalytics {
  totalTrades: number;
  openTrades: number;
  closedTrades: number;
  winRate: number;
  totalPnl: number;
  realizedPnl: number;
  unrealizedPnl: number;
  averagePnl: number;
  averageRealizedPnl: number;
  takeProfitWins: number;
  stopLossLosses: number;
  takeProfitHitRate: number;
  stopLossHitRate: number;
  profitFactor: number | null;
  perStrategy: MockStrategyAggregate[];
  perBlocker: MockBlockerAggregate[];
  perFamily: MockFamilyAggregate[];
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
  /** Sum of notional on OPEN positions for this strategy. */
  exposure: number;
}

export interface MockBlockerAggregate {
  gate: string;
  total: number;
  wins: number;
  losses: number;
  winRate: number;
  totalPnl: number;
}

export interface MockFamilyAggregate {
  family: string;
  total: number;
  open: number;
  closed: number;
  wins: number;
  losses: number;
  winRate: number;
  totalPnl: number;
  realizedPnl: number;
  unrealizedPnl: number;
  tradeCount: number;
}

export function computeAnalytics(trades: readonly MockTrade[]): MockTradeAnalytics {
  let open = 0;
  let closed = 0;
  let realized = 0;
  let unrealized = 0;
  let wins = 0;
  let losses = 0;
  let takeProfitWins = 0;
  let stopLossLosses = 0;
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
      if (t.exitReason === "TAKE_PROFIT") takeProfitWins++;
      if (t.exitReason === "STOP_LOSS") stopLossLosses++;
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
        exposure: 0,
      };
      stratMap.set(t.strategyId, strat);
    }
    strat.total++;
    if (t.status === "OPEN") {
      strat.open++;
      strat.unrealizedPnl += t.unrealizedPnl;
      strat.exposure += t.notional;
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

  // ── Per-family rollup ──────────────────────────────────────────────────────
  const familyMap = new Map<string, MockFamilyAggregate>();
  for (const t of trades) {
    const fam = t.strategyFamily ?? (t.researchPack ? "Research" : undefined);
    if (!fam) continue;
    let fa = familyMap.get(fam);
    if (!fa) {
      fa = { family: fam, total: 0, open: 0, closed: 0, wins: 0, losses: 0, winRate: 0, totalPnl: 0, realizedPnl: 0, unrealizedPnl: 0, tradeCount: 0 };
      familyMap.set(fam, fa);
    }
    fa.total++;
    fa.tradeCount++;
    if (t.status === "OPEN") {
      fa.open++;
      fa.unrealizedPnl += t.unrealizedPnl;
    } else {
      fa.closed++;
      fa.realizedPnl += t.realizedPnl;
      if (t.realizedPnl > 0) fa.wins++;
      else if (t.realizedPnl < 0) fa.losses++;
    }
    fa.totalPnl = fa.realizedPnl + fa.unrealizedPnl;
    const dec = fa.wins + fa.losses;
    fa.winRate = dec > 0 ? fa.wins / dec : 0;
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
    averageRealizedPnl: closed > 0 ? realized / closed : 0,
    takeProfitWins,
    stopLossLosses,
    takeProfitHitRate: closed > 0 ? takeProfitWins / closed : 0,
    stopLossHitRate: closed > 0 ? stopLossLosses / closed : 0,
    profitFactor: grossLosses > 0 ? grossWins / grossLosses : null,
    perStrategy: [...stratMap.values()].sort((a, b) => b.totalPnl - a.totalPnl),
    perBlocker: [...blockerMap.values()].sort((a, b) => b.total - a.total),
    perFamily: [...familyMap.values()].sort((a, b) => b.totalPnl - a.totalPnl),
  };
}

// ── Filtering ────────────────────────────────────────────────────────────────
export interface MockTradeFilter {
  strategyId?: number | null;
  side?: MockSide | null;
  status?: MockTradeStatus | null;
  blockerGate?: string | null;
  profitability?: "profit" | "loss" | null;
  strategyFamily?: string | null;
  /** When true, only show trades from the research pack (IDs 1000–1499). */
  researchOnly?: boolean | null;
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
    if (filter.strategyFamily && t.strategyFamily !== filter.strategyFamily) return false;
    if (filter.researchOnly && !t.researchPack) return false;
    if (filter.profitability) {
      const pnl = t.realizedPnl + t.unrealizedPnl;
      if (filter.profitability === "profit" && pnl <= 0) return false;
      if (filter.profitability === "loss" && pnl >= 0) return false;
    }
    return true;
  });
}

// ── Sorting ──────────────────────────────────────────────────────────────────
export type MockTradeSortKey =
  | "most_profitable"
  | "least_profitable"
  | "newest"
  | "oldest";

export const MOCK_TRADE_SORT_OPTIONS: { value: MockTradeSortKey; label: string }[] = [
  { value: "most_profitable", label: "Most profitable first" },
  { value: "least_profitable", label: "Least profitable first" },
  { value: "newest", label: "Newest first" },
  { value: "oldest", label: "Oldest first" },
];

/**
 * The PnL used for sorting and the profitability filter.
 *
 * OPEN trades use the live unrealized PnL (already net of the round-trip fee
 * debt that will crystallize on close — see applyPriceTickToTrade). CLOSED
 * trades use realized PnL (net of fees + slippage). This is the single
 * authoritative "what is this trade worth right now?" number.
 */
export function mockTradePnl(trade: MockTrade): number {
  return trade.status === "OPEN" ? trade.unrealizedPnl : trade.realizedPnl;
}

/**
 * Return a new array of trades ordered by the given sort key. Pure — input is
 * not mutated. Ties are broken deterministically by `openedAt` (newer first)
 * so the order is stable across re-renders.
 */
export function sortMockTrades(
  trades: readonly MockTrade[],
  sortKey: MockTradeSortKey,
): MockTrade[] {
  const next = trades.slice();
  switch (sortKey) {
    case "most_profitable":
      next.sort((a, b) => mockTradePnl(b) - mockTradePnl(a) || b.openedAt - a.openedAt);
      break;
    case "least_profitable":
      next.sort((a, b) => mockTradePnl(a) - mockTradePnl(b) || b.openedAt - a.openedAt);
      break;
    case "newest":
      next.sort((a, b) => b.openedAt - a.openedAt);
      break;
    case "oldest":
      next.sort((a, b) => a.openedAt - b.openedAt);
      break;
  }
  return next;
}

// ── Persistence validators ──────────────────────────────────────────────────
/** Current persisted shape version. Bump when MockTrade fields change. */
export const MOCK_PERSIST_VERSION = 3;

const REQUIRED_TRADE_NUMERIC_FIELDS: (keyof MockTrade)[] = [
  "notional",
  "quantity",
  "leverage",
  "marginUsed",
  "entryPrice",
  "takeProfitPrice",
  "stopLossPrice",
  "takeProfitUsd",
  "stopLossUsd",
  "riskRewardRatio",
  "signalPrice",
  "currentPrice",
  "signalScore",
  "requiredThreshold",
  "openedAt",
  "unrealizedPnl",
  "realizedPnl",
  "fees",
];

/** Strict shape guard for a persisted trade — rejects anything missing or non-numeric. */
export function isValidMockTrade(value: unknown): value is MockTrade {
  if (!value || typeof value !== "object") return false;
  const t = value as Record<string, unknown>;
  if (typeof t.id !== "string") return false;
  if (typeof t.traceId !== "string") return false;
  if (typeof t.strategyId !== "number") return false;
  if (typeof t.strategyName !== "string") return false;
  if (typeof t.symbol !== "string") return false;
  if (t.side !== "BUY" && t.side !== "SELL") return false;
  if (t.status !== "OPEN" && t.status !== "CLOSED") return false;
  for (const k of REQUIRED_TRADE_NUMERIC_FIELDS) {
    const v = t[k];
    if (typeof v !== "number" || !Number.isFinite(v)) return false;
  }
  if (!Array.isArray(t.blockers)) return false;
  for (const b of t.blockers as unknown[]) {
    if (!b || typeof b !== "object") return false;
    const br = b as Record<string, unknown>;
    if (typeof br.gate !== "string") return false;
    if (typeof br.reason !== "string") return false;
  }
  // closedAt / exitPrice / exitReason can be null when status === "OPEN".
  if (t.status === "CLOSED") {
    if (typeof t.closedAt !== "number" || !Number.isFinite(t.closedAt)) return false;
    if (typeof t.exitPrice !== "number" || !Number.isFinite(t.exitPrice)) return false;
    if (
      t.exitReason !== "TAKE_PROFIT" &&
      t.exitReason !== "STOP_LOSS" &&
      t.exitReason !== "MAX_HOLD" &&
      t.exitReason !== "MANUAL"
    ) {
      return false;
    }
  }
  return true;
}

export function isValidMockConfig(value: unknown): value is MockTradingConfig {
  if (!value || typeof value !== "object") return false;
  const c = value as Record<string, unknown>;
  const num = (v: unknown) => typeof v === "number" && Number.isFinite(v);
  if (!num(c.startingBalanceUsd) || (c.startingBalanceUsd as number) <= 0) return false;
  if (c.sizingMode !== "fixed_pct_equity" && c.sizingMode !== "fixed_notional" && c.sizingMode !== "risk_pct_equity") return false;
  return (
    num(c.fixedPctOfEquity) &&
    num(c.fixedNotionalUsd) &&
    num(c.riskPctOfEquity) &&
    num(c.leverage) &&
    num(c.takeProfitUsd) &&
    num(c.stopLossUsd) &&
    (c.takeProfitUsd as number) > 0 &&
    (c.stopLossUsd as number) > 0 &&
    num(c.takeProfitPct) &&
    num(c.stopLossPct) &&
    num(c.maxHoldMinutes) &&
    num(c.takerFeePct) &&
    num(c.slippageBpsPerSide) &&
    num(c.maxOpenMockTrades) &&
    (c.maxOpenMockTrades as number) >= 1
  );
}

export function normalizeMockTradingConfig(value: unknown): MockTradingConfig {
  const source = value && typeof value === "object"
    ? value as Partial<Record<keyof MockTradingConfig, unknown>>
    : {};
  const num = (v: unknown, fallback: number, positive = false): number => {
    if (typeof v !== "number" || !Number.isFinite(v)) return fallback;
    if (positive && v <= 0) return fallback;
    return v;
  };
  const sizingMode = source.sizingMode === "fixed_pct_equity" ||
    source.sizingMode === "fixed_notional" ||
    source.sizingMode === "risk_pct_equity"
    ? source.sizingMode
    : DEFAULT_MOCK_TRADING_CONFIG.sizingMode;
  const next: MockTradingConfig = {
    startingBalanceUsd: num(source.startingBalanceUsd, DEFAULT_MOCK_TRADING_CONFIG.startingBalanceUsd, true),
    sizingMode,
    fixedPctOfEquity: Math.max(0, num(source.fixedPctOfEquity, DEFAULT_MOCK_TRADING_CONFIG.fixedPctOfEquity)),
    fixedNotionalUsd: Math.max(0, num(source.fixedNotionalUsd, DEFAULT_MOCK_TRADING_CONFIG.fixedNotionalUsd)),
    riskPctOfEquity: Math.max(0, num(source.riskPctOfEquity, DEFAULT_MOCK_TRADING_CONFIG.riskPctOfEquity)),
    leverage: Math.max(1, Math.min(125, num(source.leverage, DEFAULT_MOCK_TRADING_CONFIG.leverage))),
    takeProfitUsd: num(source.takeProfitUsd, DEFAULT_MOCK_TRADING_CONFIG.takeProfitUsd, true),
    stopLossUsd: num(source.stopLossUsd, DEFAULT_MOCK_TRADING_CONFIG.stopLossUsd, true),
    takeProfitPct: Math.max(0, num(source.takeProfitPct, DEFAULT_MOCK_TRADING_CONFIG.takeProfitPct)),
    stopLossPct: Math.max(0, num(source.stopLossPct, DEFAULT_MOCK_TRADING_CONFIG.stopLossPct)),
    maxHoldMinutes: Math.max(0, num(source.maxHoldMinutes, DEFAULT_MOCK_TRADING_CONFIG.maxHoldMinutes)),
    takerFeePct: Math.max(0, num(source.takerFeePct, DEFAULT_MOCK_TRADING_CONFIG.takerFeePct)),
    slippageBpsPerSide: Math.max(0, num(source.slippageBpsPerSide, DEFAULT_MOCK_TRADING_CONFIG.slippageBpsPerSide)),
    maxOpenMockTrades: maxOpenMockTradesFromConfig({
      ...DEFAULT_MOCK_TRADING_CONFIG,
      maxOpenMockTrades: num(source.maxOpenMockTrades, DEFAULT_MOCK_TRADING_CONFIG.maxOpenMockTrades, true),
    }),
  };
  return next;
}

// ── Logging helper ───────────────────────────────────────────────────────────
export interface MockTradeLog {
  ts: number;
  event:
    | "MOCK_TRADE_CREATED"
    | "MOCK_TRADE_UPDATED"
    | "MOCK_TRADE_CLOSED"
    | "MOCK_TRADE_TP_HIT"
    | "MOCK_TRADE_SL_HIT"
    | "MOCK_TRADE_LIMIT_REACHED"
    | "MOCK_ACCOUNT_UPDATED"
    | "MOCK_STRATEGY_RUNNER_EVENT"
    | "MOCK_STRATEGY_RUNNER_ERROR"
    | "MOCK_TRADING_RESET";
  tradeId?: string;
  strategyId: number;
  strategyName: string;
  side: MockSide;
  price: number;
  entryPrice?: number;
  exitPrice?: number;
  exitReason?: MockExitReason | null;
  ignoredBlockers: string[];
  pnl?: number;
  notional?: number;
  message?: string;
}

export function logForMockTradeCreated(trade: MockTrade): MockTradeLog {
  return {
    ts: trade.openedAt,
    event: "MOCK_TRADE_CREATED",
    tradeId: trade.id,
    strategyId: trade.strategyId,
    strategyName: trade.strategyName,
    side: trade.side,
    price: trade.entryPrice,
    ignoredBlockers: trade.blockers.map((b) => b.gate),
    notional: trade.notional,
  };
}

export function logForMockTradeLimitReached(args: {
  ts: number;
  strategyId: number;
  strategyName: string;
  side: MockSide;
  price: number;
  openCount: number;
  maxOpenMockTrades: number;
  ignoredBlockers?: string[];
}): MockTradeLog {
  return {
    ts: args.ts,
    event: "MOCK_TRADE_LIMIT_REACHED",
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    side: args.side,
    price: args.price,
    ignoredBlockers: args.ignoredBlockers ?? [],
    message: `Open mock trade limit reached (${args.openCount}/${args.maxOpenMockTrades})`,
  };
}

export function logForMockTradeClosed(trade: MockTrade): MockTradeLog {
  const event = trade.exitReason === "TAKE_PROFIT"
    ? "MOCK_TRADE_TP_HIT"
    : trade.exitReason === "STOP_LOSS"
      ? "MOCK_TRADE_SL_HIT"
      : "MOCK_TRADE_CLOSED";
  return {
    ts: trade.closedAt ?? Date.now(),
    event,
    tradeId: trade.id,
    strategyId: trade.strategyId,
    strategyName: trade.strategyName,
    side: trade.side,
    price: trade.exitPrice ?? trade.currentPrice,
    entryPrice: trade.entryPrice,
    exitPrice: trade.exitPrice ?? trade.currentPrice,
    exitReason: trade.exitReason,
    ignoredBlockers: trade.blockers.map((b) => b.gate),
    pnl: trade.realizedPnl,
    notional: trade.notional,
  };
}
