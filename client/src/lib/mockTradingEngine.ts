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
}

export const DEFAULT_MOCK_TRADING_CONFIG: MockTradingConfig = {
  startingBalanceUsd: 1_000_000,
  sizingMode: "fixed_pct_equity",
  fixedPctOfEquity: 1,
  fixedNotionalUsd: 10_000,
  riskPctOfEquity: 1,
  leverage: 25,
  takeProfitPct: 1.5,
  stopLossPct: 0.6,
  maxHoldMinutes: 45,
  takerFeePct: 0.0005,
  slippageBpsPerSide: 5,
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
): { takeProfitPct: number; stopLossPct: number; maxHoldMinutes: number } {
  if (!override) {
    return {
      takeProfitPct: config.takeProfitPct,
      stopLossPct: config.stopLossPct,
      maxHoldMinutes: config.maxHoldMinutes,
    };
  }
  return {
    takeProfitPct: override.takeProfitPct ?? config.takeProfitPct,
    stopLossPct: override.stopLossPct ?? config.stopLossPct,
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
    return finalizeClose({ trade, fillBeforeSlippage: price, exitReason, now, config });
  }

  // Unrealized PnL: gross + estimated round-trip fees not yet realized.
  // We surface NET unrealized so the equity card matches realized math.
  const gross = computeMockPnl(trade.side, trade.entryPrice, price, trade.quantity);
  const feesIfClosed = paperRoundTripTakerFees(trade.notional, config.takerFeePct);
  return { ...trade, currentPrice: price, unrealizedPnl: gross - feesIfClosed };
}

/** Manually close an open trade at the given mark. */
export function closeMockTrade(
  trade: MockTrade,
  price: number,
  now: number,
  config: MockTradingConfig = DEFAULT_MOCK_TRADING_CONFIG,
): MockTrade {
  if (trade.status === "CLOSED") return trade;
  return finalizeClose({ trade, fillBeforeSlippage: price, exitReason: "MANUAL", now, config });
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
  profitFactor: number | null;
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
export const MOCK_PERSIST_VERSION = 2;

const REQUIRED_TRADE_NUMERIC_FIELDS: (keyof MockTrade)[] = [
  "notional",
  "quantity",
  "leverage",
  "marginUsed",
  "entryPrice",
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
    num(c.takeProfitPct) &&
    num(c.stopLossPct) &&
    num(c.maxHoldMinutes) &&
    num(c.takerFeePct) &&
    num(c.slippageBpsPerSide)
  );
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
  notional?: number;
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
    notional: trade.notional,
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
    notional: trade.notional,
  };
}
