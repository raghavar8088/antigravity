/**
 * Extended strategy evaluation metrics for the BTC futures mock trading module.
 *
 * Computes institutional-grade performance statistics from a set of closed trades:
 *
 *   Core
 *   - Win rate, profit factor, expectancy
 *   - Average trade PnL, average win, average loss
 *   - Average holding time
 *
 *   Risk-adjusted
 *   - Sharpe ratio   (mean return / std-dev of returns × √N)
 *   - Sortino ratio  (mean return / downside std-dev × √N)
 *   - Calmar ratio   (annualized return / max drawdown)
 *
 *   Drawdown
 *   - Maximum drawdown (peak-to-trough on running equity)
 *   - Recovery factor (net profit / max drawdown)
 *
 * All inputs and outputs are plain TypeScript — pure functions, no React, no I/O.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface ClosedTradeRecord {
  realizedPnl: number;
  openedAt: number;
  closedAt: number;
}

export interface ExtendedMetrics {
  // Trade counts
  totalTrades: number;
  winningTrades: number;
  losingTrades: number;
  breakEvenTrades: number;

  // Core
  winRate: number;
  lossRate: number;
  profitFactor: number | null;
  expectancy: number;
  averageTrade: number;
  averageWin: number;
  averageLoss: number;

  // Time
  averageHoldingTimeMinutes: number;
  medianHoldingTimeMinutes: number;

  // Risk-adjusted (null when insufficient data)
  sharpeRatio: number | null;
  sortinoRatio: number | null;
  calmarRatio: number | null;

  // Drawdown
  maxDrawdownUsd: number;
  maxDrawdownPct: number;
  recoveryFactor: number | null;

  // Summary
  netPnl: number;
  grossWins: number;
  grossLosses: number;

  // Consistency (standard deviation of trade PnL)
  pnlStdDev: number;
  pnlDownsideStdDev: number;
}

export const EMPTY_EXTENDED_METRICS: ExtendedMetrics = {
  totalTrades: 0,
  winningTrades: 0,
  losingTrades: 0,
  breakEvenTrades: 0,
  winRate: 0,
  lossRate: 0,
  profitFactor: null,
  expectancy: 0,
  averageTrade: 0,
  averageWin: 0,
  averageLoss: 0,
  averageHoldingTimeMinutes: 0,
  medianHoldingTimeMinutes: 0,
  sharpeRatio: null,
  sortinoRatio: null,
  calmarRatio: null,
  maxDrawdownUsd: 0,
  maxDrawdownPct: 0,
  recoveryFactor: null,
  netPnl: 0,
  grossWins: 0,
  grossLosses: 0,
  pnlStdDev: 0,
  pnlDownsideStdDev: 0,
};

// ── Main computation ──────────────────────────────────────────────────────────

/**
 * Compute all extended metrics from an array of closed MockTrade records.
 * Trades that are still OPEN are ignored.
 */
export function computeExtendedMetrics(
  trades: readonly MockTrade[],
  startingEquityUsd: number = 1_000_000,
): ExtendedMetrics {
  const closed = trades
    .filter((t) => t.status === "CLOSED" && t.closedAt != null)
    .sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0));

  if (closed.length === 0) return { ...EMPTY_EXTENDED_METRICS };

  return computeExtendedMetricsFromRecords(
    closed.map((t) => ({
      realizedPnl: t.realizedPnl,
      openedAt: t.openedAt,
      closedAt: t.closedAt as number,
    })),
    startingEquityUsd,
  );
}

/**
 * Compute extended metrics from raw ClosedTradeRecord objects.
 * Useful when records come from sources other than MockTrade.
 */
export function computeExtendedMetricsFromRecords(
  records: readonly ClosedTradeRecord[],
  startingEquityUsd: number = 1_000_000,
): ExtendedMetrics {
  const sorted = [...records].sort((a, b) => a.closedAt - b.closedAt);
  const n = sorted.length;
  if (n === 0) return { ...EMPTY_EXTENDED_METRICS };

  // ── Basic counts ───────────────────────────────────────────────────────────
  let grossWins = 0;
  let grossLosses = 0;
  let wins = 0;
  let losses = 0;
  let breakEven = 0;
  const pnls: number[] = [];
  const holdingMinutes: number[] = [];

  for (const r of sorted) {
    const pnl = r.realizedPnl;
    pnls.push(pnl);
    holdingMinutes.push(Math.max(0, (r.closedAt - r.openedAt) / 60_000));
    if (pnl > 0) { wins++; grossWins += pnl; }
    else if (pnl < 0) { losses++; grossLosses += Math.abs(pnl); }
    else breakEven++;
  }

  const netPnl = pnls.reduce((s, p) => s + p, 0);
  const winRate = wins / n;
  const lossRate = losses / n;
  const averageTrade = netPnl / n;
  const averageWin = wins > 0 ? grossWins / wins : 0;
  const averageLoss = losses > 0 ? -grossLosses / losses : 0;
  const profitFactor = grossLosses > 0 ? grossWins / grossLosses : null;
  const expectancy = winRate * averageWin + lossRate * averageLoss;

  // ── Holding time ───────────────────────────────────────────────────────────
  const averageHoldingTimeMinutes = holdingMinutes.reduce((s, h) => s + h, 0) / n;
  const sortedHolding = [...holdingMinutes].sort((a, b) => a - b);
  const medianHoldingTimeMinutes = _median(sortedHolding);

  // ── Std-dev (trade-level, not return-level) ────────────────────────────────
  const pnlStdDev = _stdDev(pnls, averageTrade);
  const downsidePnls = pnls.filter((p) => p < averageTrade);
  const pnlDownsideStdDev = _stdDev(downsidePnls, averageTrade);

  // ── Sharpe ────────────────────────────────────────────────────────────────
  // Using per-trade returns (PnL / starting equity) and annualising by √(trades/year)
  // Assume BTC futures ~1500 trades/year for scaling; adjusted per actual trade count.
  const sharpeRatio = pnlStdDev > 0
    ? (averageTrade / pnlStdDev) * Math.sqrt(n)
    : null;

  // ── Sortino ───────────────────────────────────────────────────────────────
  const sortinoRatio = pnlDownsideStdDev > 0
    ? (averageTrade / pnlDownsideStdDev) * Math.sqrt(n)
    : null;

  // ── Drawdown ──────────────────────────────────────────────────────────────
  const { maxDrawdownUsd, maxDrawdownPct } = _computeMaxDrawdown(sorted, startingEquityUsd);

  // ── Calmar ────────────────────────────────────────────────────────────────
  let calmarRatio: number | null = null;
  if (maxDrawdownUsd > 0 && sorted.length >= 2) {
    const spanMs = sorted[sorted.length - 1].closedAt - sorted[0].openedAt;
    const spanYears = Math.max(0.01, spanMs / (365.25 * 24 * 60 * 60 * 1000));
    const annualizedReturn = netPnl / spanYears;
    calmarRatio = annualizedReturn / maxDrawdownUsd;
  }

  // ── Recovery factor ───────────────────────────────────────────────────────
  const recoveryFactor = maxDrawdownUsd > 0 ? netPnl / maxDrawdownUsd : null;

  return {
    totalTrades: n,
    winningTrades: wins,
    losingTrades: losses,
    breakEvenTrades: breakEven,
    winRate,
    lossRate,
    profitFactor,
    expectancy,
    averageTrade,
    averageWin,
    averageLoss,
    averageHoldingTimeMinutes,
    medianHoldingTimeMinutes,
    sharpeRatio,
    sortinoRatio,
    calmarRatio,
    maxDrawdownUsd,
    maxDrawdownPct,
    recoveryFactor,
    netPnl,
    grossWins,
    grossLosses,
    pnlStdDev,
    pnlDownsideStdDev,
  };
}

// ── Drawdown ──────────────────────────────────────────────────────────────────

function _computeMaxDrawdown(
  sorted: readonly ClosedTradeRecord[],
  startingEquityUsd: number,
): { maxDrawdownUsd: number; maxDrawdownPct: number } {
  let equity = startingEquityUsd;
  let peak = startingEquityUsd;
  let maxDdUsd = 0;
  let maxDdPct = 0;

  for (const r of sorted) {
    equity += r.realizedPnl;
    if (equity > peak) peak = equity;
    const ddUsd = peak - equity;
    const ddPct = peak > 0 ? ddUsd / peak : 0;
    if (ddUsd > maxDdUsd) maxDdUsd = ddUsd;
    if (ddPct > maxDdPct) maxDdPct = ddPct;
  }
  return { maxDrawdownUsd: maxDdUsd, maxDrawdownPct: maxDdPct };
}

// ── Utility math ──────────────────────────────────────────────────────────────

function _stdDev(values: readonly number[], mean: number): number {
  if (values.length < 2) return 0;
  const variance = values.reduce((sum, v) => sum + (v - mean) ** 2, 0) / (values.length - 1);
  return Math.sqrt(Math.max(0, variance));
}

function _median(sorted: readonly number[]): number {
  if (sorted.length === 0) return 0;
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 0
    ? ((sorted[mid - 1] ?? 0) + (sorted[mid] ?? 0)) / 2
    : (sorted[mid] ?? 0);
}

// ── Per-strategy metrics ──────────────────────────────────────────────────────

export interface StrategyExtendedMetrics extends ExtendedMetrics {
  strategyId: number;
  strategyName: string;
  strategyFamily?: string;
}

/**
 * Compute extended metrics grouped by strategyId.
 */
export function computePerStrategyMetrics(
  trades: readonly MockTrade[],
  startingEquityUsd: number = 1_000_000,
): StrategyExtendedMetrics[] {
  const byStrategy = new Map<number, MockTrade[]>();
  for (const trade of trades) {
    if (trade.status !== "CLOSED" || trade.closedAt == null) continue;
    const bucket = byStrategy.get(trade.strategyId) ?? [];
    bucket.push(trade);
    byStrategy.set(trade.strategyId, bucket);
  }

  return [...byStrategy.entries()].map(([strategyId, stratTrades]) => {
    const metrics = computeExtendedMetrics(stratTrades, startingEquityUsd);
    return {
      ...metrics,
      strategyId,
      strategyName: stratTrades[0]?.strategyName ?? `Strategy ${strategyId}`,
      strategyFamily: stratTrades[0]?.strategyFamily,
    };
  });
}

// ── Regime-sliced metrics ─────────────────────────────────────────────────────

export interface RegimeMetrics extends ExtendedMetrics {
  regime: string;
}

/**
 * Compute extended metrics for each market regime at entry.
 */
export function computePerRegimeMetrics(
  trades: readonly MockTrade[],
  startingEquityUsd: number = 1_000_000,
): RegimeMetrics[] {
  const byRegime = new Map<string, MockTrade[]>();
  for (const trade of trades) {
    if (trade.status !== "CLOSED" || trade.closedAt == null) continue;
    const regime = trade.regimeAtEntry ?? "UNKNOWN";
    const bucket = byRegime.get(regime) ?? [];
    bucket.push(trade);
    byRegime.set(regime, bucket);
  }

  return [...byRegime.entries()].map(([regime, regimeTrades]) => {
    const metrics = computeExtendedMetrics(regimeTrades, startingEquityUsd);
    return { ...metrics, regime };
  });
}

// ── Rolling window metrics ────────────────────────────────────────────────────

/**
 * Compute metrics over a rolling N-day window ending at `now`.
 * Useful for recent-performance weighting in ranking engines.
 */
export function computeRollingMetrics(args: {
  trades: readonly MockTrade[];
  windowDays: number;
  now: number;
  startingEquityUsd?: number;
}): ExtendedMetrics {
  const windowMs = args.windowDays * 24 * 60 * 60 * 1000;
  const since = args.now - windowMs;
  const recent = args.trades.filter(
    (t) => t.status === "CLOSED" && t.closedAt != null && (t.closedAt as number) >= since,
  );
  return computeExtendedMetrics(recent, args.startingEquityUsd ?? 1_000_000);
}

// ── Equity curve ─────────────────────────────────────────────────────────────

export interface EquityCurvePoint {
  timestamp: number;
  equity: number;
  drawdownPct: number;
  cumulativePnl: number;
}

/**
 * Build a time-series equity curve from sorted closed trade records.
 */
export function buildEquityCurve(
  trades: readonly MockTrade[],
  startingEquityUsd: number,
): EquityCurvePoint[] {
  const sorted = [...trades]
    .filter((t) => t.status === "CLOSED" && t.closedAt != null)
    .sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0));

  const curve: EquityCurvePoint[] = [];
  let equity = startingEquityUsd;
  let peak = startingEquityUsd;

  for (const t of sorted) {
    equity += t.realizedPnl;
    if (equity > peak) peak = equity;
    const drawdownPct = peak > 0 ? (peak - equity) / peak : 0;
    curve.push({
      timestamp: t.closedAt as number,
      equity,
      drawdownPct,
      cumulativePnl: equity - startingEquityUsd,
    });
  }
  return curve;
}

// ── Daily / monthly PnL ───────────────────────────────────────────────────────

export interface DailyPnlRecord {
  dateStr: string;
  timestamp: number;
  pnl: number;
  trades: number;
  wins: number;
  losses: number;
}

export interface MonthlyPnlRecord {
  monthStr: string;
  timestamp: number;
  pnl: number;
  trades: number;
  winRate: number;
}

export function computeDailyPnl(trades: readonly MockTrade[]): DailyPnlRecord[] {
  const byDay = new Map<string, DailyPnlRecord>();
  for (const t of trades) {
    if (t.status !== "CLOSED" || t.closedAt == null) continue;
    const d = new Date(t.closedAt);
    const dateStr = `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}-${String(d.getUTCDate()).padStart(2, "0")}`;
    const rec = byDay.get(dateStr) ?? {
      dateStr,
      timestamp: d.setUTCHours(0, 0, 0, 0),
      pnl: 0,
      trades: 0,
      wins: 0,
      losses: 0,
    };
    rec.pnl += t.realizedPnl;
    rec.trades++;
    if (t.realizedPnl > 0) rec.wins++;
    else if (t.realizedPnl < 0) rec.losses++;
    byDay.set(dateStr, rec);
  }
  return [...byDay.values()].sort((a, b) => a.timestamp - b.timestamp);
}

export function computeMonthlyPnl(trades: readonly MockTrade[]): MonthlyPnlRecord[] {
  const byMonth = new Map<string, { pnl: number; trades: number; wins: number; ts: number }>();
  for (const t of trades) {
    if (t.status !== "CLOSED" || t.closedAt == null) continue;
    const d = new Date(t.closedAt);
    const monthStr = `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
    const rec = byMonth.get(monthStr) ?? { pnl: 0, trades: 0, wins: 0, ts: new Date(d.getUTCFullYear(), d.getUTCMonth(), 1).getTime() };
    rec.pnl += t.realizedPnl;
    rec.trades++;
    if (t.realizedPnl > 0) rec.wins++;
    byMonth.set(monthStr, rec);
  }
  return [...byMonth.entries()]
    .map(([monthStr, v]) => ({
      monthStr,
      timestamp: v.ts,
      pnl: v.pnl,
      trades: v.trades,
      winRate: v.trades > 0 ? v.wins / v.trades : 0,
    }))
    .sort((a, b) => a.timestamp - b.timestamp);
}
