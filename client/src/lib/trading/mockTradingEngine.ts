/**
 * Mock Trading Engine — analysis-only $1,000,000 paper account that mirrors
 * the production BTC FT desk's sizing, fees, slippage, and PnL math but
 * keeps the strategy gate results intact so mock fills are only created for
 * executable signals that would have survived the desk's blocker pipeline.
 *
 * Mirrored from `futuresPaperMath`:
 *   - Linear gross PnL (paperLinearGrossPnl)
 *   - Round-trip taker fees (paperRoundTripTakerFees)
 *   - Entry / exit slippage in bps (paperApply{Entry,Exit}Slippage)
 *   - Margin required = notional / leverage (paperMarginRequired)
 *   - Fixed-%-of-equity sizing (deskFixedNotionalPctOfEquity)
 *   - Vol-aware risk sizing (paperNotionalForTargetRisk)
 *
 * NEVER imports paperOms, broker, or live Delta order paths — this is a
 * pure simulation. All inputs are observability data (signal trace rows + a
 * live Binance mark). All outputs stay in-memory + localStorage.
 *
 * Pure functions; no React, no I/O. Tested in mockTradingEngine.test.ts.
 */

import {
  applyFundingAccrual,
  DELTA_PAPER_FUNDING_INTERVAL_MS,
  paperApplyEntrySlippage,
  paperApplyExitSlippage,
  paperLinearGrossPnl,
  paperMarginRequired,
  paperRoundTripTakerFees,
} from "@/lib/trading/futuresPaperMath";
import type { StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";

// ── Types ────────────────────────────────────────────────────────────────────
export type MockTradeStatus = "OPEN" | "CLOSED";
export type MockSide = "BUY" | "SELL";
export type MockExitReason = "TAKE_PROFIT" | "STOP_LOSS" | "MAX_HOLD" | "MANUAL";
export type MockSizingMode = "fixed_pct_equity" | "fixed_notional" | "risk_pct_equity";
export const DEFAULT_MAX_OPEN_MOCK_TRADES = 5;
export const DEFAULT_MAX_OPEN_MOCK_LONG_TRADES = 3;
export const DEFAULT_MAX_OPEN_MOCK_SHORT_TRADES = 3;
export const DEFAULT_MOCK_TRADE_COOLDOWN_MINUTES = 15;
export const DEFAULT_MOCK_MIN_RISK_REWARD_RATIO = 1.5;
export const DEFAULT_MOCK_MAX_SIGNALS_PER_BATCH = 3;

export interface MockTradeBlocker {
  /** Gate the strategy hit (e.g. "REGIME"). */
  gate: string;
  /** Human-readable reason captured by the trace. */
  reason: string;
}

export const MOCK_REJECTION_CATEGORIES = [
  "SIGNAL",
  "CONFIRM",
  "ATR_FEES",
  "REGIME",
  "LOW_CONFIDENCE",
  "LOW_SIGNAL_SCORE",
  "RR_TOO_LOW",
  "COOLDOWN",
  "FAMILY_CONFLICT",
  "EXPOSURE_LIMIT",
  "DAILY_LOSS_LIMIT",
  "WEEKLY_LOSS_LIMIT",
  "MAX_DRAWDOWN",
  "MARGIN_CHECK",
  "DUPLICATE_POSITION",
  "NO_APPROVED_STRATEGY",
  "OTHER",
] as const;

export type MockRejectionCategory = (typeof MOCK_REJECTION_CATEGORIES)[number];
export type MockRejectionCounts = Record<MockRejectionCategory, number>;

export interface MockDiagnosticFunnel {
  signalsGenerated: number;
  confidencePassed: number;
  scorePassed: number;
  riskRewardPassed: number;
  riskPassed: number;
  cooldownPassed: number;
  familyConflictPassed: number;
  exposurePassed: number;
  tradesCreated: number;
}

export interface MockRejectionEvent {
  ts: number;
  category: MockRejectionCategory;
  strategyId: number;
  strategyName: string;
  side?: MockSide;
  signalScore?: number;
  confidenceScore?: number;
  riskRewardRatio?: number;
  message: string;
}

export interface MockTradingDiagnostics {
  funnel: MockDiagnosticFunnel;
  rejectionCounts: MockRejectionCounts;
  recentRejections: MockRejectionEvent[];
  lastUpdatedAt: number | null;
}

export function emptyMockRejectionCounts(): MockRejectionCounts {
  return MOCK_REJECTION_CATEGORIES.reduce((acc, category) => {
    acc[category] = 0;
    return acc;
  }, {} as MockRejectionCounts);
}

export function emptyMockDiagnosticFunnel(): MockDiagnosticFunnel {
  return {
    signalsGenerated: 0,
    confidencePassed: 0,
    scorePassed: 0,
    riskRewardPassed: 0,
    riskPassed: 0,
    cooldownPassed: 0,
    familyConflictPassed: 0,
    exposurePassed: 0,
    tradesCreated: 0,
  };
}

export function emptyMockTradingDiagnostics(): MockTradingDiagnostics {
  return {
    funnel: emptyMockDiagnosticFunnel(),
    rejectionCounts: emptyMockRejectionCounts(),
    recentRejections: [],
    lastUpdatedAt: null,
  };
}

export function rejectionCategoryFromTraceGate(gate: string): MockRejectionCategory {
  if (gate === "SIGNAL") return "SIGNAL";
  if (gate === "CONFIRM") return "CONFIRM";
  if (gate === "ATR_FEES") return "ATR_FEES";
  if (gate === "REGIME") return "REGIME";
  return "OTHER";
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
  /** Legacy persisted field; new TP dollar outcomes are derived from takeProfitPct. */
  takeProfitUsd: number;
  /** Legacy persisted field; new SL dollar outcomes are derived from stopLossPct. */
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
  /** Mock-only cap for simultaneous LONG/BUY positions. */
  maxOpenLongTrades: number;
  /** Mock-only cap for simultaneous SHORT/SELL positions. */
  maxOpenShortTrades: number;
  /** Minimum minutes before the same strategy can enter again. */
  tradeCooldownMinutes: number;
  /** Minimum signal score/confidence needed before risk checks. */
  minSignalScore: number;
  /** Maximum new entries selected from one signal batch. */
  maxSignalsPerBatch: number;
  /** Minimum net TP / net SL ratio required before opening. */
  minRiskRewardRatio: number;
  /** Stop new entries after this realized rolling-24h loss percentage of starting equity. */
  dailyLossLimitPct: number;
  /** Stop new entries after this realized rolling-7d loss percentage of starting equity. */
  weeklyLossLimitPct: number;
  /** Stop new entries after this max drawdown percentage of starting equity. */
  maxDrawdownPct: number;
  /** Funding rate percent for one funding interval. Positive means longs pay. */
  fundingRatePctPer8h: number;
  /** Funding interval in hours, defaults to the BTC perp 8h convention. */
  fundingIntervalHours: number;
  /** Legacy persisted stage label. Trade Engine no longer branches on it. */
  pipelineStage?: string | null;
}

export function isGradeDiscoveryStage(config: MockTradingConfig): boolean {
  void config;
  return false;
}

export const DEFAULT_MOCK_TRADING_CONFIG: MockTradingConfig = {
  startingBalanceUsd: 1_000_000,
  sizingMode: "risk_pct_equity",
  fixedPctOfEquity: 1,
  fixedNotionalUsd: 10_000,
  riskPctOfEquity: 0.25,
  leverage: 25,
  takeProfitUsd: 10,
  stopLossUsd: 5,
  takeProfitPct: 1.5,
  stopLossPct: 0.4,
  maxHoldMinutes: 120,
  takerFeePct: 0.0005,
  slippageBpsPerSide: 5,
  maxOpenMockTrades: DEFAULT_MAX_OPEN_MOCK_TRADES,
  maxOpenLongTrades: DEFAULT_MAX_OPEN_MOCK_LONG_TRADES,
  maxOpenShortTrades: DEFAULT_MAX_OPEN_MOCK_SHORT_TRADES,
  tradeCooldownMinutes: DEFAULT_MOCK_TRADE_COOLDOWN_MINUTES,
  minSignalScore: 50,
  maxSignalsPerBatch: DEFAULT_MOCK_MAX_SIGNALS_PER_BATCH,
  minRiskRewardRatio: DEFAULT_MOCK_MIN_RISK_REWARD_RATIO,
  dailyLossLimitPct: 3,
  weeklyLossLimitPct: 6,
  maxDrawdownPct: 10,
  fundingRatePctPer8h: 0,
  fundingIntervalHours: 8,
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
  /** Live mark price that triggers take-profit. */
  takeProfitPrice: number;
  /** Live mark price that triggers stop-loss. */
  stopLossPrice: number;
  /** Estimated net profit at TP, after exit slippage and round-trip fees. */
  takeProfitUsd: number;
  /** Estimated net loss magnitude at SL, after exit slippage and round-trip fees. */
  stopLossUsd: number;
  /** Estimated TP profit divided by estimated SL loss. */
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
  /** PERF 9: Gross realized PnL before fees and funding (pure price-move profit). */
  grossPnl?: number;
  /** Round-trip fees in USD once CLOSED. */
  fees: number;
  /** Futures funding paid by the position. Negative means the position received funding. */
  fundingCosts?: number;
  exitReason: MockExitReason | null;
  /** Fill price after exit slippage. */
  exitPrice: number | null;
  /** Strategy family (e.g. "TrendFollowing") — set only for research pack trades. */
  strategyFamily?: string;
  /** Signal confidence 0–100 — set only for research pack trades. */
  confidenceScore?: number;
  /** Strategy parameters used to create the signal — set only for research pack trades. */
  strategyParams?: Record<string, number | string>;
  /** Market regime observed when the research signal was converted into a mock trade. */
  regimeAtEntry?: string;
  /** True for trades produced by the mock research runner. */
  researchPack?: boolean;
  /** ISPAP catalog slug — links mock_trades to strategy_authority_profiles. */
  ispapStrategyId?: string;
  /** ISPAP pipeline stage that opened this mock trade. */
  pipelineStage?: string;
}

// ── Trace → mock trade ───────────────────────────────────────────────────────

/**
 * Did this trace row raise a directional strategy signal?
 *
 * A signal is "raised" when the strategy produced a side (LONG/SHORT) and a
 * positive signal score. Raised does not mean executable; rejected blocker rows
 * are filtered by `isExecutableTraceRow`.
 */
export function isStrategySignalRaised(row: StrategySignalTraceRow): boolean {
  if (!row.side) return false;
  if (!Number.isFinite(row.signalScore) || row.signalScore <= 0) return false;
  return true;
}

export function isExecutableTraceRow(row: StrategySignalTraceRow): boolean {
  if (!isStrategySignalRaised(row)) return false;
  if (row.status !== "OPENED" && row.status !== "CANDIDATE") return false;
  if (row.gate !== "OPENED" && row.status !== "CANDIDATE") return false;
  if (row.confirmPassed === false) return false;
  if (row.regimeAllowed === false) return false;
  if (row.feeHurdlePassed === false) return false;
  if (!Number.isFinite(row.signalScore) || !Number.isFinite(row.requiredThreshold)) return false;
  return row.requiredThreshold <= 0 || row.signalScore >= row.requiredThreshold;
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

function mockFundingRate(config: MockTradingConfig): number {
  return Number.isFinite(config.fundingRatePctPer8h) ? config.fundingRatePctPer8h / 100 : 0;
}

function mockFundingIntervalMs(config: MockTradingConfig): number {
  if (!Number.isFinite(config.fundingIntervalHours) || config.fundingIntervalHours <= 0) {
    return DELTA_PAPER_FUNDING_INTERVAL_MS;
  }
  return config.fundingIntervalHours * 60 * 60 * 1000;
}

function computeMockFundingCost(args: {
  side: MockSide;
  notional: number;
  openedAt: number;
  now: number;
  config: MockTradingConfig;
}): number {
  return applyFundingAccrual({
    side: toPaperSide(args.side),
    notional: args.notional,
    fundingRate: mockFundingRate(args.config),
    elapsedMs: Math.max(0, args.now - args.openedAt),
    fundingIntervalMs: mockFundingIntervalMs(args.config),
  });
}

function resolveExit(
  config: MockTradingConfig,
  override: StrategyExitOverride | undefined,
): { takeProfitPct: number; stopLossPct: number; maxHoldMinutes: number } {
  const takeProfitPct = Number.isFinite(override?.takeProfitPct) && (override?.takeProfitPct ?? 0) > 0
    ? override?.takeProfitPct as number
    : Number.isFinite(config.takeProfitPct) && config.takeProfitPct > 0
      ? config.takeProfitPct
      : DEFAULT_MOCK_TRADING_CONFIG.takeProfitPct;
  const stopLossPct = Number.isFinite(override?.stopLossPct) && (override?.stopLossPct ?? 0) > 0
    ? override?.stopLossPct as number
    : Number.isFinite(config.stopLossPct) && config.stopLossPct > 0
      ? config.stopLossPct
    : DEFAULT_MOCK_TRADING_CONFIG.stopLossPct;
  if (!override) {
    return {
      takeProfitPct,
      stopLossPct,
      maxHoldMinutes: config.maxHoldMinutes,
    };
  }
  return {
    takeProfitPct,
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

export function countOpenMockTradesBySide(trades: readonly MockTrade[], side: MockSide): number {
  let open = 0;
  for (const trade of trades) {
    if (trade.status === "OPEN" && trade.side === side) open++;
  }
  return open;
}

export function maxOpenLongTradesFromConfig(config: MockTradingConfig): number {
  const raw = config.maxOpenLongTrades;
  if (!Number.isFinite(raw) || raw <= 0) return DEFAULT_MAX_OPEN_MOCK_LONG_TRADES;
  return Math.max(1, Math.floor(raw));
}

export function maxOpenShortTradesFromConfig(config: MockTradingConfig): number {
  const raw = config.maxOpenShortTrades;
  if (!Number.isFinite(raw) || raw <= 0) return DEFAULT_MAX_OPEN_MOCK_SHORT_TRADES;
  return Math.max(1, Math.floor(raw));
}

export function canOpenAdditionalMockTrade(args: {
  trades: readonly MockTrade[];
  config: MockTradingConfig;
  pendingOpenTrades?: number;
}): boolean {
  const pending = Math.max(0, Math.floor(args.pendingOpenTrades ?? 0));
  return countOpenMockTrades(args.trades) + pending < maxOpenMockTradesFromConfig(args.config);
}

export function mockTradeFamilyKey(trade: Pick<MockTrade, "strategyId" | "strategyFamily">): string {
  return trade.strategyFamily?.trim() || `strategy:${trade.strategyId}`;
}

function candidateFamilyKey(candidate: Pick<MockTrade, "strategyId" | "strategyFamily">): string {
  return mockTradeFamilyKey(candidate);
}

function rollingRealizedPnl(trades: readonly MockTrade[], now: number, windowMs: number): number {
  const since = now - windowMs;
  let pnl = 0;
  for (const trade of trades) {
    if (trade.status !== "CLOSED" || trade.closedAt == null || trade.closedAt < since) continue;
    pnl += trade.realizedPnl;
  }
  return pnl;
}

function minutesSinceLastStrategyEntry(args: {
  strategyId: number;
  trades: readonly MockTrade[];
  now: number;
}): number | null {
  let latest: number | null = null;
  for (const trade of args.trades) {
    if (trade.strategyId !== args.strategyId) continue;
    if (latest == null || trade.openedAt > latest) latest = trade.openedAt;
  }
  if (latest == null) return null;
  return (args.now - latest) / 60_000;
}

export type MockTradeRejectionCode =
  | "BLOCKER"
  | "SIGNAL_SCORE"
  | "MISSING_EXIT"
  | "RISK_REWARD"
  | "MAX_OPEN"
  | "MAX_SIDE"
  | "FAMILY_CONFLICT"
  | "COOLDOWN"
  | "MARGIN"
  | "DAILY_LOSS"
  | "WEEKLY_LOSS"
  | "MAX_DRAWDOWN";

export interface MockTradeRiskDecision {
  allowed: boolean;
  code?: MockTradeRejectionCode;
  message?: string;
}

export function rejectionCategoryFromRiskCode(code: MockTradeRejectionCode | undefined): MockRejectionCategory {
  switch (code) {
    case "SIGNAL_SCORE":
      return "LOW_SIGNAL_SCORE";
    case "MISSING_EXIT":
    case "RISK_REWARD":
      return "RR_TOO_LOW";
    case "COOLDOWN":
      return "COOLDOWN";
    case "FAMILY_CONFLICT":
      return "FAMILY_CONFLICT";
    case "MAX_OPEN":
    case "MAX_SIDE":
      return "EXPOSURE_LIMIT";
    case "MARGIN":
      return "MARGIN_CHECK";
    case "DAILY_LOSS":
      return "DAILY_LOSS_LIMIT";
    case "WEEKLY_LOSS":
      return "WEEKLY_LOSS_LIMIT";
    case "MAX_DRAWDOWN":
      return "MAX_DRAWDOWN";
    case "BLOCKER":
    default:
      return "OTHER";
  }
}

export function evaluateMockTradeOpenRisk(args: {
  trade: MockTrade;
  existingTrades: readonly MockTrade[];
  pendingTrades?: readonly MockTrade[];
  config: MockTradingConfig;
  now: number;
}): MockTradeRiskDecision {
  const pendingTrades = args.pendingTrades ?? [];
  const portfolio = [...args.existingTrades, ...pendingTrades];
  const trade = args.trade;
  const config = args.config;

  if (trade.blockers.length > 0) {
    return { allowed: false, code: "BLOCKER", message: `blocked by ${trade.blockers.map((b) => b.gate).join(",")}` };
  }
  if (isGradeDiscoveryStage(config)) {
    return { allowed: true };
  }
  const minScore = Number.isFinite(config.minSignalScore) ? config.minSignalScore : DEFAULT_MOCK_TRADING_CONFIG.minSignalScore;
  if (!Number.isFinite(trade.signalScore) || trade.signalScore < minScore) {
    return { allowed: false, code: "SIGNAL_SCORE", message: `signal score ${trade.signalScore.toFixed(1)} below ${minScore.toFixed(1)}` };
  }
  if (trade.takeProfitUsd <= 0 || trade.stopLossUsd <= 0) {
    return { allowed: false, code: "MISSING_EXIT", message: "mandatory TP/SL estimate missing" };
  }
  const minRr = Number.isFinite(config.minRiskRewardRatio) && config.minRiskRewardRatio > 0
    ? config.minRiskRewardRatio
    : DEFAULT_MOCK_MIN_RISK_REWARD_RATIO;
  if (!Number.isFinite(trade.riskRewardRatio) || trade.riskRewardRatio < minRr) {
    return { allowed: false, code: "RISK_REWARD", message: `risk/reward ${trade.riskRewardRatio.toFixed(2)} below ${minRr.toFixed(2)}` };
  }

  const account = computeAccountState(args.existingTrades, config);
  const starting = account.startingBalance;
  const dailyLimit = Math.max(0, config.dailyLossLimitPct) / 100;
  if (dailyLimit > 0 && rollingRealizedPnl(args.existingTrades, args.now, 24 * 60 * 60 * 1000) <= -starting * dailyLimit) {
    return { allowed: false, code: "DAILY_LOSS", message: `rolling daily loss limit ${config.dailyLossLimitPct}% breached` };
  }
  const weeklyLimit = Math.max(0, config.weeklyLossLimitPct) / 100;
  if (weeklyLimit > 0 && rollingRealizedPnl(args.existingTrades, args.now, 7 * 24 * 60 * 60 * 1000) <= -starting * weeklyLimit) {
    return { allowed: false, code: "WEEKLY_LOSS", message: `rolling weekly loss limit ${config.weeklyLossLimitPct}% breached` };
  }
  const maxDrawdown = Math.max(0, config.maxDrawdownPct) / 100;
  if (maxDrawdown > 0 && account.maxDrawdownPct >= maxDrawdown) {
    return { allowed: false, code: "MAX_DRAWDOWN", message: `max drawdown ${config.maxDrawdownPct}% breached` };
  }

  if (countOpenMockTrades(portfolio) >= maxOpenMockTradesFromConfig(config)) {
    return { allowed: false, code: "MAX_OPEN", message: `max open positions ${maxOpenMockTradesFromConfig(config)} reached` };
  }
  const sideLimit = trade.side === "BUY" ? maxOpenLongTradesFromConfig(config) : maxOpenShortTradesFromConfig(config);
  if (countOpenMockTradesBySide(portfolio, trade.side) >= sideLimit) {
    return { allowed: false, code: "MAX_SIDE", message: `${trade.side === "BUY" ? "long" : "short"} position cap ${sideLimit} reached` };
  }

  const family = candidateFamilyKey(trade);
  const oppositeSide: MockSide = trade.side === "BUY" ? "SELL" : "BUY";
  const hasOppositeFamilyOpen = portfolio.some(
    (openTrade) =>
      openTrade.status === "OPEN" &&
      openTrade.side === oppositeSide &&
      mockTradeFamilyKey(openTrade) === family,
  );
  if (hasOppositeFamilyOpen) {
    return { allowed: false, code: "FAMILY_CONFLICT", message: `opposite ${family} position already open` };
  }

  const cooldown = isGradeDiscoveryStage(config)
    ? Math.max(0, config.tradeCooldownMinutes)
    : Math.max(DEFAULT_MOCK_TRADE_COOLDOWN_MINUTES, config.tradeCooldownMinutes);
  const minutes = minutesSinceLastStrategyEntry({ strategyId: trade.strategyId, trades: portfolio, now: args.now });
  if (minutes != null && minutes < cooldown) {
    return { allowed: false, code: "COOLDOWN", message: `strategy cooldown ${cooldown}m active (${minutes.toFixed(1)}m elapsed)` };
  }
  if (trade.marginUsed > account.availableBalance) {
    return { allowed: false, code: "MARGIN", message: "insufficient simulated margin" };
  }
  return { allowed: true };
}

export function scoreMockTraceRow(row: StrategySignalTraceRow): number {
  const threshold = Number.isFinite(row.requiredThreshold) && row.requiredThreshold > 0 ? row.requiredThreshold : 1;
  const quality = Number.isFinite(row.qualityScore) ? row.qualityScore ?? 0 : 0;
  const mtf = Number.isFinite(row.mtfScore) ? row.mtfScore ?? 0 : 0;
  return row.signalScore + Math.min(50, (row.signalScore / threshold) * 10) + quality * 0.15 + mtf * 0.1;
}

export function scoreMockResearchSignal(signal: MockResearchSignalInput): number {
  return Math.max(0, Math.min(100, signal.confidenceScore));
}

export function maxSignalsPerBatchFromConfig(config: MockTradingConfig): number {
  const raw = config.maxSignalsPerBatch;
  if (!Number.isFinite(raw) || raw <= 0) return DEFAULT_MOCK_MAX_SIGNALS_PER_BATCH;
  return Math.max(1, Math.floor(raw));
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
  if (!isExecutableTraceRow(row)) return null;
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
  const exitLevels = computeMockExitLevels({
    side,
    entryPrice: fillPrice,
    quantity,
    notional,
    config,
    takeProfitPct: exit.takeProfitPct,
    stopLossPct: exit.stopLossPct,
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
    ...exitLevels,
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
    fundingCosts: 0,
    exitReason: null,
    exitPrice: null,
    strategyFamily: row.category,
    ispapStrategyId: row.ispapStrategyId,
    pipelineStage: row.pipelineStage,
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
  regimeAtEntry?: string;
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
  const exitLevels = computeMockExitLevels({
    side: signal.side,
    entryPrice: fillPrice,
    quantity,
    notional,
    config,
    takeProfitPct: exit.takeProfitPct,
    stopLossPct: exit.stopLossPct,
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
    ...exitLevels,
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
    fundingCosts: 0,
    exitReason: null,
    exitPrice: null,
    strategyFamily: signal.strategyFamily,
    confidenceScore: safeConfidence,
    strategyParams: signal.params,
    regimeAtEntry: signal.regimeAtEntry,
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

export function computeMockNetPnlAtExitMark(args: {
  side: MockSide;
  entryPrice: number;
  exitMarkPrice: number;
  quantity: number;
  notional: number;
  config: MockTradingConfig;
  fundingCosts?: number;
}): number {
  const { side, entryPrice, exitMarkPrice, quantity, notional, config } = args;
  if (!Number.isFinite(exitMarkPrice) || exitMarkPrice <= 0) return 0;
  const fillPrice = paperApplyExitSlippage(toPaperSide(side), exitMarkPrice, config.slippageBpsPerSide);
  const gross = computeMockPnl(side, entryPrice, fillPrice, quantity);
  const fees = paperRoundTripTakerFees(notional, config.takerFeePct);
  return gross - fees - (args.fundingCosts ?? 0);
}

export function computeMockExitLevels(args: {
  side: MockSide;
  entryPrice: number;
  quantity: number;
  notional: number;
  config: MockTradingConfig;
  takeProfitPct: number;
  stopLossPct: number;
}): {
  takeProfitPrice: number;
  stopLossPrice: number;
  takeProfitUsd: number;
  stopLossUsd: number;
  riskRewardRatio: number;
} {
  const takeProfitPct = Number.isFinite(args.takeProfitPct) && args.takeProfitPct > 0
    ? args.takeProfitPct
    : DEFAULT_MOCK_TRADING_CONFIG.takeProfitPct;
  const stopLossPct = Number.isFinite(args.stopLossPct) && args.stopLossPct > 0
    ? args.stopLossPct
    : DEFAULT_MOCK_TRADING_CONFIG.stopLossPct;
  const takeProfitPrice = args.side === "BUY"
    ? args.entryPrice * (1 + takeProfitPct / 100)
    : Math.max(0.01, args.entryPrice * (1 - takeProfitPct / 100));
  const stopLossPrice = args.side === "BUY"
    ? Math.max(0.01, args.entryPrice * (1 - stopLossPct / 100))
    : args.entryPrice * (1 + stopLossPct / 100);
  const tpNet = computeMockNetPnlAtExitMark({
    side: args.side,
    entryPrice: args.entryPrice,
    exitMarkPrice: takeProfitPrice,
    quantity: args.quantity,
    notional: args.notional,
    config: args.config,
  });
  const slNet = computeMockNetPnlAtExitMark({
    side: args.side,
    entryPrice: args.entryPrice,
    exitMarkPrice: stopLossPrice,
    quantity: args.quantity,
    notional: args.notional,
    config: args.config,
  });
  const takeProfitUsd = Math.max(0, tpNet);
  const stopLossUsd = Math.max(0, -slNet);

  return {
    takeProfitPrice,
    stopLossPrice,
    takeProfitUsd,
    stopLossUsd,
    riskRewardRatio: stopLossUsd > 0 ? takeProfitUsd / stopLossUsd : 0,
  };
}

export function withMockExitFields(
  trade: MockTrade,
  config: MockTradingConfig,
  override?: StrategyExitOverride,
): MockTrade {
  const exit = resolveExit(config, override);
  const exits = computeMockExitLevels({
    side: trade.side,
    entryPrice: trade.entryPrice,
    quantity: trade.quantity,
    notional: trade.notional,
    config,
    takeProfitPct: exit.takeProfitPct,
    stopLossPct: exit.stopLossPct,
  });
  return { ...trade, ...exits };
}

function reachedExitTarget(args: {
  trade: MockTrade;
  price: number;
  targetPrice: number;
}): boolean {
  const { trade, price, targetPrice } = args;
  if (!Number.isFinite(targetPrice) || targetPrice <= 0) return false;
  return targetPrice >= trade.entryPrice ? price >= targetPrice : price <= targetPrice;
}

/**
 * Apply a price tick to a single trade and (optionally) exit it if TP/SL/maxHold
 * conditions are met. Returns a new trade object — caller should replace the
 * old one in their store. Idempotent for CLOSED trades.
 *
 * Unrealized and realized PnL are net of the estimated/full round-trip fee.
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
  const activeTrade = withMockExitFields(trade, config, override);
  const ageMin = (now - trade.openedAt) / 60_000;

  let exitReason: MockExitReason | null = null;
  let exitPrice = price;
  if (
    reachedExitTarget({
      trade: activeTrade,
      price,
      targetPrice: activeTrade.takeProfitPrice,
    })
  ) {
    exitReason = "TAKE_PROFIT";
    exitPrice = activeTrade.takeProfitPrice;
  } else if (
    reachedExitTarget({
      trade: activeTrade,
      price,
      targetPrice: activeTrade.stopLossPrice,
    })
  ) {
    exitReason = "STOP_LOSS";
    exitPrice = activeTrade.stopLossPrice;
  }
  if (!exitReason && ageMin >= exit.maxHoldMinutes) exitReason = "MAX_HOLD";

  if (exitReason) {
    return finalizeClose({ trade: activeTrade, fillBeforeSlippage: exitPrice, exitReason, now, config });
  }

  // Surface NET unrealized so the equity card matches realized math.
  const gross = computeMockPnl(activeTrade.side, activeTrade.entryPrice, price, activeTrade.quantity);
  const feesIfClosed = paperRoundTripTakerFees(activeTrade.notional, config.takerFeePct);
  const fundingCosts = computeMockFundingCost({
    side: activeTrade.side,
    notional: activeTrade.notional,
    openedAt: activeTrade.openedAt,
    now,
    config,
  });
  return { ...activeTrade, currentPrice: price, unrealizedPnl: gross - feesIfClosed - fundingCosts, fundingCosts };
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
    trade: withMockExitFields(trade, config),
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
  const fundingCosts = computeMockFundingCost({
    side: trade.side,
    notional: trade.notional,
    openedAt: trade.openedAt,
    now,
    config,
  });
  const net = gross - fees - fundingCosts;
  return {
    ...trade,
    status: "CLOSED",
    closedAt: now,
    currentPrice: fillPrice,
    unrealizedPnl: 0,
    realizedPnl: net,
    grossPnl: gross, // PERF 9: gross PnL before fees and funding
    fees,
    fundingCosts,
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
  /** Sum of notional on OPEN LONG trades. */
  longExposure: number;
  /** Sum of notional on OPEN SHORT trades. */
  shortExposure: number;
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
  let longExposure = 0;
  let shortExposure = 0;
  let marginUsed = 0;
  let openCount = 0;
  let closedCount = 0;

  const closes: { ts: number; pnl: number }[] = [];

  for (const t of trades) {
    if (t.status === "OPEN") {
      openCount++;
      unrealizedPnl += t.unrealizedPnl;
      exposure += t.notional;
      if (t.side === "BUY") longExposure += t.notional;
      else shortExposure += t.notional;
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
    longExposure,
    shortExposure,
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
  averageTrade: number;
  averageRealizedPnl: number;
  averageWin: number;
  averageLoss: number;
  takeProfitWins: number;
  stopLossLosses: number;
  takeProfitHitRate: number;
  stopLossHitRate: number;
  profitFactor: number | null;
  sharpeRatio: number | null;
  sortinoRatio: number | null;
  calmarRatio: number | null;
  maxDrawdown: number;
  expectancy: number | null;
  recoveryFactor: number | null;
  /** PERF 9: Total gross PnL before fees and funding. */
  grossPnl: number;
  /** PERF 9: Total fees and funding costs deducted from gross to arrive at realized PnL. */
  totalCosts: number;
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
  let totalGrossPnl = 0; // PERF 9
  const closedPnls: number[] = [];

  const stratMap = new Map<number, MockStrategyAggregate>();
  const blockerMap = new Map<string, MockBlockerAggregate>();

  for (const t of trades) {
    if (t.status === "OPEN") {
      open++;
      unrealized += t.unrealizedPnl;
    } else {
      closed++;
      realized += t.realizedPnl;
      closedPnls.push(t.realizedPnl);
      totalGrossPnl += t.grossPnl ?? t.realizedPnl; // PERF 9
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
  const averageTrade = closed > 0 ? realized / closed : 0;

  // PERF 8: Sharpe ratio uses daily PnL padded with zeros for inactive trading days.
  // This prevents the ratio from being inflated on sparse datasets.
  const sharpeRatio = (() => {
    const closedTrades = trades.filter(t => t.status !== "OPEN" && t.closedAt);
    if (closedTrades.length < 2) return null;
    const dayPnlMap = new Map<string, number>();
    for (const t of closedTrades) {
      const day = new Date(t.closedAt!).toISOString().slice(0, 10);
      dayPnlMap.set(day, (dayPnlMap.get(day) ?? 0) + t.realizedPnl);
    }
    if (dayPnlMap.size < 2) return null;
    const days = [...dayPnlMap.keys()].sort();
    const first = new Date(days[0]);
    const last = new Date(days[days.length - 1]);
    const dailyPnls: number[] = [];
    for (let d = new Date(first); d <= last; d.setUTCDate(d.getUTCDate() + 1)) {
      const key = d.toISOString().slice(0, 10);
      dailyPnls.push(dayPnlMap.get(key) ?? 0);
    }
    const meanDaily = dailyPnls.reduce((s, v) => s + v, 0) / dailyPnls.length;
    const varDaily = dailyPnls.reduce((s, v) => s + (v - meanDaily) ** 2, 0) / (dailyPnls.length - 1);
    const stdevDaily = Math.sqrt(varDaily);
    return stdevDaily > 0 ? (meanDaily / stdevDaily) * Math.sqrt(252) : null;
  })();

  const variance = closedPnls.length > 1
    ? closedPnls.reduce((sum, pnl) => sum + (pnl - averageTrade) ** 2, 0) / (closedPnls.length - 1)
    : 0;
  const stdev = Math.sqrt(variance);

  const downPnls = closedPnls.filter(p => p < 0);
  const downStdev = downPnls.length > 1
    ? Math.sqrt(downPnls.reduce((sum, pnl) => sum + (pnl - averageTrade) ** 2, 0) / (downPnls.length - 1))
    : 0;
  const sortinoRatio = downStdev > 0 ? (averageTrade / downStdev) * Math.sqrt(closedPnls.length) : null;

  const account = computeAccountState(trades, DEFAULT_MOCK_TRADING_CONFIG);
  const maxDrawdown = account.maxDrawdownPct * account.startingBalance;
  const calmarRatio = maxDrawdown > 0 ? realized / maxDrawdown : null;
  const recoveryFactor = maxDrawdown > 0 ? realized / maxDrawdown : null;

  const winRate = decided > 0 ? wins / decided : 0;
  const avgWin = wins > 0 ? grossWins / wins : 0;
  const avgLoss = losses > 0 ? grossLosses / losses : 0;
  const expectancy = decided > 0 ? (winRate * avgWin) - ((1 - winRate) * avgLoss) : null;

  return {
    totalTrades: trades.length,
    openTrades: open,
    closedTrades: closed,
    winRate,
    totalPnl: realized + unrealized,
    realizedPnl: realized,
    unrealizedPnl: unrealized,
    averagePnl: averageTrade,
    averageTrade,
    averageRealizedPnl: averageTrade,
    averageWin: avgWin,
    averageLoss: -avgLoss,
    takeProfitWins,
    stopLossLosses,
    takeProfitHitRate: closed > 0 ? takeProfitWins / closed : 0,
    stopLossHitRate: closed > 0 ? stopLossLosses / closed : 0,
    profitFactor: grossLosses > 0 ? grossWins / grossLosses : null,
    sharpeRatio,
    sortinoRatio,
    calmarRatio,
    maxDrawdown,
    expectancy,
    recoveryFactor,
    grossPnl: totalGrossPnl, // PERF 9
    totalCosts: totalGrossPnl - realized, // PERF 9: fees + funding
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
  ageMode?: MockTradeAgeFilterMode | null;
  ageMinMinutes?: number | null;
  ageMaxMinutes?: number | null;
  /** When true, only show trades from the research pack (IDs 1000–1499). */
  researchOnly?: boolean | null;
}

export type MockTradeAgeFilterMode = "less" | "more" | "between";

function finiteNonNegative(value: number | null | undefined): number | null {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : null;
}

export function mockTradeAgeMinutes(trade: MockTrade, nowMs: number = Date.now()): number {
  const end = trade.status === "CLOSED" && trade.closedAt != null ? trade.closedAt : nowMs;
  return Math.max(0, (end - trade.openedAt) / 60_000);
}

function passesMockTradeAgeFilter(trade: MockTrade, filter: MockTradeFilter, nowMs: number): boolean {
  if (!filter.ageMode) return true;

  const age = mockTradeAgeMinutes(trade, nowMs);
  const min = finiteNonNegative(filter.ageMinMinutes);
  const max = finiteNonNegative(filter.ageMaxMinutes);

  if (filter.ageMode === "less") return max == null || age < max;
  if (filter.ageMode === "more") return min == null || age > min;
  if (min != null && age < min) return false;
  if (max != null && age > max) return false;
  return true;
}

export function filterMockTrades(
  trades: readonly MockTrade[],
  filter: MockTradeFilter,
  nowMs: number = Date.now(),
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
    if (!passesMockTradeAgeFilter(t, filter, nowMs)) return false;
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
export const MOCK_PERSIST_VERSION = 4;

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
    (c.takeProfitUsd as number) >= 0 &&
    (c.stopLossUsd as number) >= 0 &&
    num(c.takeProfitPct) &&
    num(c.stopLossPct) &&
    num(c.maxHoldMinutes) &&
    num(c.takerFeePct) &&
    num(c.slippageBpsPerSide) &&
    num(c.maxOpenMockTrades) &&
    (c.maxOpenMockTrades as number) >= 1 &&
    num(c.maxOpenLongTrades) &&
    (c.maxOpenLongTrades as number) >= 1 &&
    num(c.maxOpenShortTrades) &&
    (c.maxOpenShortTrades as number) >= 1 &&
    num(c.tradeCooldownMinutes) &&
    (c.tradeCooldownMinutes as number) >= DEFAULT_MOCK_TRADE_COOLDOWN_MINUTES &&
    num(c.minSignalScore) &&
    num(c.maxSignalsPerBatch) &&
    (c.maxSignalsPerBatch as number) >= 1 &&
    num(c.minRiskRewardRatio) &&
    (c.minRiskRewardRatio as number) >= DEFAULT_MOCK_MIN_RISK_REWARD_RATIO &&
    num(c.dailyLossLimitPct) &&
    num(c.weeklyLossLimitPct) &&
    num(c.maxDrawdownPct) &&
    num(c.fundingRatePctPer8h) &&
    num(c.fundingIntervalHours) &&
    (c.fundingIntervalHours as number) > 0
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
    maxOpenLongTrades: maxOpenLongTradesFromConfig({
      ...DEFAULT_MOCK_TRADING_CONFIG,
      maxOpenLongTrades: num(source.maxOpenLongTrades, DEFAULT_MOCK_TRADING_CONFIG.maxOpenLongTrades, true),
    }),
    maxOpenShortTrades: maxOpenShortTradesFromConfig({
      ...DEFAULT_MOCK_TRADING_CONFIG,
      maxOpenShortTrades: num(source.maxOpenShortTrades, DEFAULT_MOCK_TRADING_CONFIG.maxOpenShortTrades, true),
    }),
    pipelineStage:
      typeof source.pipelineStage === "string" && source.pipelineStage.trim().length > 0
        ? source.pipelineStage.trim()
        : null,
    tradeCooldownMinutes: Math.max(
      DEFAULT_MOCK_TRADE_COOLDOWN_MINUTES,
      num(source.tradeCooldownMinutes, DEFAULT_MOCK_TRADING_CONFIG.tradeCooldownMinutes),
    ),
    minSignalScore: Math.max(0, Math.min(100, num(source.minSignalScore, DEFAULT_MOCK_TRADING_CONFIG.minSignalScore))),
    maxSignalsPerBatch: maxSignalsPerBatchFromConfig({
      ...DEFAULT_MOCK_TRADING_CONFIG,
      maxSignalsPerBatch: num(source.maxSignalsPerBatch, DEFAULT_MOCK_TRADING_CONFIG.maxSignalsPerBatch, true),
    }),
    minRiskRewardRatio: Math.max(
      DEFAULT_MOCK_MIN_RISK_REWARD_RATIO,
      num(source.minRiskRewardRatio, DEFAULT_MOCK_TRADING_CONFIG.minRiskRewardRatio),
    ),
    dailyLossLimitPct: Math.max(0, num(source.dailyLossLimitPct, DEFAULT_MOCK_TRADING_CONFIG.dailyLossLimitPct)),
    weeklyLossLimitPct: Math.max(0, num(source.weeklyLossLimitPct, DEFAULT_MOCK_TRADING_CONFIG.weeklyLossLimitPct)),
    maxDrawdownPct: Math.max(0, num(source.maxDrawdownPct, DEFAULT_MOCK_TRADING_CONFIG.maxDrawdownPct)),
    fundingRatePctPer8h: num(source.fundingRatePctPer8h, DEFAULT_MOCK_TRADING_CONFIG.fundingRatePctPer8h),
    fundingIntervalHours: Math.max(0.01, num(source.fundingIntervalHours, DEFAULT_MOCK_TRADING_CONFIG.fundingIntervalHours, true)),
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
    | "MOCK_TRADE_REJECTED"
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
  rejectionCategory?: MockRejectionCategory;
  signalScore?: number;
  confidenceScore?: number;
  riskRewardRatio?: number;
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
    ignoredBlockers: [],
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

export function logForMockTradeRejected(args: {
  ts: number;
  strategyId: number;
  strategyName: string;
  side: MockSide;
  price: number;
  code: MockTradeRejectionCode;
  message: string;
  category?: MockRejectionCategory;
  signalScore?: number;
  confidenceScore?: number;
  riskRewardRatio?: number;
}): MockTradeLog {
  return {
    ts: args.ts,
    event: "MOCK_TRADE_REJECTED",
    strategyId: args.strategyId,
    strategyName: args.strategyName,
    side: args.side,
    price: args.price,
    ignoredBlockers: [],
    rejectionCategory: args.category ?? rejectionCategoryFromRiskCode(args.code),
    signalScore: args.signalScore,
    confidenceScore: args.confidenceScore,
    riskRewardRatio: args.riskRewardRatio,
    message: `${args.code}: ${args.message}`,
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
    ignoredBlockers: [],
    pnl: trade.realizedPnl,
    notional: trade.notional,
  };
}
