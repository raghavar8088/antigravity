/**
 * Strategy performance engine — computes per-strategy trade metrics from raw
 * MockTrade history. Pure computation, no I/O or React.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import type { MarketRegime } from "@/lib/ai/marketRegimeClassifier";

type RegimeKey = MarketRegime | "UNKNOWN";

export interface RegimeStats {
  trades: number;
  winRate: number;
  expectancy: number;
  netPnl: number;
}

export interface StrategyPerformanceMetrics {
  strategyId: number;
  strategyName: string;
  totalTrades: number;
  closedTrades: number;
  grossPnl: number;
  netPnl: number;
  winRate: number;
  profitFactor: number;
  avgWin: number;
  avgLoss: number;
  expectancy: number;
  sharpeRatio: number;
  sortinoRatio: number;
  maxDrawdown: number;
  maxDrawdownPct: number;
  recoveryFactor: number;
  avgHoldMinutes: number;
  recencyScore: number;
  last7DaysPnl: number;
  last7DaysWinRate: number;
  last7DaysTrades: number;
  regimeBreakdown: Partial<Record<RegimeKey, RegimeStats>>;
  sampleSizeConfidence: number;
}

const EMPTY_REGIME_STATS: RegimeStats = { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 };

function safeDiv(num: number, denom: number, fallback = 0): number {
  if (!Number.isFinite(denom) || denom === 0) return fallback;
  return num / denom;
}

function computeMaxDrawdown(equity: number[]): { drawdown: number; drawdownPct: number } {
  let peak = equity[0] ?? 0;
  let maxDd = 0;
  let maxDdPct = 0;
  for (const e of equity) {
    if (e > peak) peak = e;
    const dd = peak - e;
    const ddPct = peak > 0 ? (dd / peak) * 100 : 0;
    if (dd > maxDd) { maxDd = dd; maxDdPct = ddPct; }
  }
  return { drawdown: maxDd, drawdownPct: maxDdPct };
}

function computeSharpe(pnls: number[]): number {
  if (pnls.length < 2) return 0;
  const mean = pnls.reduce((s, v) => s + v, 0) / pnls.length;
  const variance = pnls.reduce((s, v) => s + (v - mean) ** 2, 0) / (pnls.length - 1);
  const std = Math.sqrt(variance);
  if (std === 0) return mean > 0 ? 3 : 0;
  return (mean / std) * Math.sqrt(252);
}

function computeSortino(pnls: number[]): number {
  if (pnls.length < 2) return 0;
  const mean = pnls.reduce((s, v) => s + v, 0) / pnls.length;
  const downside = pnls.filter((v) => v < 0);
  if (downside.length === 0) return mean > 0 ? 3 : 0;
  const variance = downside.reduce((s, v) => s + v ** 2, 0) / downside.length;
  const std = Math.sqrt(variance);
  if (std === 0) return mean > 0 ? 3 : 0;
  return (mean / std) * Math.sqrt(252);
}

export function computeStrategyPerformance(
  trades: readonly MockTrade[],
  now = Date.now(),
): StrategyPerformanceMetrics {
  const strategyId = trades[0]?.strategyId ?? 0;
  const strategyName = trades[0]?.strategyName ?? `Strategy ${strategyId}`;

  const closed = trades.filter((t) => t.status === "CLOSED");
  const closedCount = closed.length;

  if (closedCount === 0) {
    const empty: StrategyPerformanceMetrics = {
      strategyId,
      strategyName,
      totalTrades: trades.length,
      closedTrades: 0,
      grossPnl: 0,
      netPnl: 0,
      winRate: 0,
      profitFactor: 0,
      avgWin: 0,
      avgLoss: 0,
      expectancy: 0,
      sharpeRatio: 0,
      sortinoRatio: 0,
      maxDrawdown: 0,
      maxDrawdownPct: 0,
      recoveryFactor: 0,
      avgHoldMinutes: 0,
      recencyScore: 0,
      last7DaysPnl: 0,
      last7DaysWinRate: 0,
      last7DaysTrades: 0,
      regimeBreakdown: { UNKNOWN: { ...EMPTY_REGIME_STATS } },
      sampleSizeConfidence: 0,
    };
    return empty;
  }

  const wins = closed.filter((t) => t.realizedPnl > 0);
  const losses = closed.filter((t) => t.realizedPnl < 0);

  const grossWin = wins.reduce((s, t) => s + t.realizedPnl, 0);
  const grossLoss = Math.abs(losses.reduce((s, t) => s + t.realizedPnl, 0));
  const netPnl = closed.reduce((s, t) => s + t.realizedPnl, 0);
  const grossPnl = netPnl;

  const winRate = safeDiv(wins.length, closedCount);
  const profitFactor = grossLoss > 0 ? grossWin / grossLoss : grossWin > 0 ? 99 : 0;
  const avgWin = safeDiv(grossWin, wins.length || 1);
  const avgLoss = wins.length > 0 || losses.length > 0 ? safeDiv(-grossLoss, losses.length || 1) : 0;
  const expectancy = safeDiv(netPnl, closedCount);

  // Equity curve for drawdown + Sharpe
  const sortedClosed = [...closed].sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0));
  const pnls = sortedClosed.map((t) => t.realizedPnl);
  let equity = 0;
  const equityCurve = pnls.map((p) => { equity += p; return equity; });

  const { drawdown: maxDrawdown, drawdownPct: maxDrawdownPct } = computeMaxDrawdown(equityCurve);
  const sharpeRatio = computeSharpe(pnls);
  const sortinoRatio = computeSortino(pnls);
  const recoveryFactor = maxDrawdown > 0 ? safeDiv(Math.max(0, netPnl), maxDrawdown) : 0;

  // Hold times
  const holdTimes = closed
    .map((t) => {
      if (t.openedAt == null || t.closedAt == null) return null;
      return (t.closedAt - t.openedAt) / 60_000;
    })
    .filter((v): v is number => v != null);
  const avgHoldMinutes = holdTimes.length > 0
    ? holdTimes.reduce((s, v) => s + v, 0) / holdTimes.length
    : 0;

  // Recency (last 7 days)
  const cutoff7d = now - 7 * 24 * 3600 * 1000;
  const recent7d = closed.filter((t) => (t.closedAt ?? 0) >= cutoff7d);
  const last7DaysPnl = recent7d.reduce((s, t) => s + t.realizedPnl, 0);
  const last7DaysWinRate = recent7d.length > 0
    ? recent7d.filter((t) => t.realizedPnl > 0).length / recent7d.length
    : 0;
  const last7DaysTrades = recent7d.length;

  // Recency score: 0–100, based on win rate and PnL trend of last 14 trades
  const last14 = sortedClosed.slice(-14);
  const recencyWins = last14.filter((t) => t.realizedPnl > 0).length;
  const recencyScore = last14.length > 0 ? Math.round((recencyWins / last14.length) * 100) : 50;

  // Sample-size confidence: 0–100
  const sampleSizeConfidence = Math.min(100, Math.round((closedCount / 50) * 100));

  return {
    strategyId,
    strategyName,
    totalTrades: trades.length,
    closedTrades: closedCount,
    grossPnl,
    netPnl,
    winRate,
    profitFactor,
    avgWin,
    avgLoss,
    expectancy,
    sharpeRatio,
    sortinoRatio,
    maxDrawdown,
    maxDrawdownPct,
    recoveryFactor,
    avgHoldMinutes,
    recencyScore,
    last7DaysPnl,
    last7DaysWinRate,
    last7DaysTrades,
    regimeBreakdown: { UNKNOWN: { trades: closedCount, winRate, expectancy, netPnl } },
    sampleSizeConfidence,
  };
}
