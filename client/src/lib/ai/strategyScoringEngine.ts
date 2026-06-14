/**
 * Strategy scoring engine — ranks strategies by multi-metric composite score.
 *
 * Pure computation, no I/O or React.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";

export type ConfidenceRating = "HIGH" | "MEDIUM" | "LOW" | "INSUFFICIENT";

export interface StrategyScore {
  strategyId: number;
  strategyName: string;
  /** Raw trade metrics snapshot used as score inputs. */
  metrics: {
    totalTrades: number;
    closedTrades: number;
    netPnl: number;
    winRate: number;
    profitFactor: number;
    expectancy: number;
    sharpeRatio: number;
    maxDrawdownPct: number;
  };
  pnlScore: number;
  profitFactorScore: number;
  winRateScore: number;
  drawdownScore: number;
  sharpeScore: number;
  recencyScore: number;
  sampleSizeScore: number;
  overallScore: number;
  currentRegimeScore: number;
  confidenceRating: ConfidenceRating;
  rank: number;
  regimeRank: number;
}

function clamp(v: number, min = 0, max = 100): number {
  if (!Number.isFinite(v)) return min;
  return Math.min(max, Math.max(min, v));
}

function scorePnl(netPnl: number): number {
  if (netPnl >= 1000) return 100;
  if (netPnl >= 0) return clamp((netPnl / 1000) * 100);
  return clamp(50 + (netPnl / 200));
}

function scoreProfitFactor(pf: number): number {
  if (!Number.isFinite(pf) || pf <= 0) return 0;
  if (pf >= 3) return 100;
  return clamp(((pf - 0.5) / 2.5) * 100);
}

function scoreWinRate(wr: number): number {
  return clamp(wr * 100 * 1.2);
}

function scoreDrawdown(ddPct: number): number {
  if (ddPct <= 0) return 100;
  if (ddPct >= 30) return 0;
  return clamp(100 - (ddPct / 30) * 100);
}

function scoreSharpe(sharpe: number): number {
  if (sharpe >= 3) return 100;
  if (sharpe <= -1) return 0;
  return clamp(((sharpe + 1) / 4) * 100);
}

function scoreRecency(trades: MockTrade[]): number {
  const now = Date.now();
  const recentClosed = trades.filter(
    (t) => t.status === "CLOSED" && t.closedAt != null && now - (t.closedAt ?? 0) < 7 * 24 * 3600 * 1000,
  );
  if (recentClosed.length === 0) return 40;
  const recentWins = recentClosed.filter((t) => t.realizedPnl > 0).length;
  return clamp((recentWins / recentClosed.length) * 100);
}

function confidenceFromSample(closedTrades: number): ConfidenceRating {
  if (closedTrades >= 50) return "HIGH";
  if (closedTrades >= 20) return "MEDIUM";
  if (closedTrades >= 10) return "LOW";
  return "INSUFFICIENT";
}

export function scoreAllStrategies(
  trades: readonly MockTrade[],
  currentRegime?: string,
): StrategyScore[] {
  const byStrategy = new Map<number, MockTrade[]>();
  for (const t of trades) {
    const arr = byStrategy.get(t.strategyId) ?? [];
    arr.push(t);
    byStrategy.set(t.strategyId, arr);
  }

  const raw: (StrategyScore & { _rawOverall: number })[] = [];

  for (const [strategyId, stTrades] of byStrategy) {
    const closed = stTrades.filter((t) => t.status === "CLOSED");
    if (closed.length === 0) continue;

    const netPnl = closed.reduce((s, t) => s + t.realizedPnl, 0);
    const wins = closed.filter((t) => t.realizedPnl > 0);
    const losses = closed.filter((t) => t.realizedPnl <= 0);
    const winRate = wins.length / closed.length;
    const grossWin = wins.reduce((s, t) => s + t.realizedPnl, 0);
    const grossLoss = Math.abs(losses.reduce((s, t) => s + t.realizedPnl, 0));
    const profitFactor = grossLoss > 0 ? grossWin / grossLoss : grossWin > 0 ? 99 : 0;
    const expectancy = netPnl / closed.length;
    const maxDrawdownPct = 0;

    const pnlS = scorePnl(netPnl);
    const pfS = scoreProfitFactor(profitFactor);
    const wrS = scoreWinRate(winRate);
    const ddS = scoreDrawdown(maxDrawdownPct);
    const sharpeS = scoreSharpe(0);
    const recencyS = scoreRecency(stTrades);
    const sampleS = clamp((closed.length / 50) * 100);

    const overall = clamp(pnlS * 0.25 + pfS * 0.25 + wrS * 0.20 + ddS * 0.10 + sharpeS * 0.10 + recencyS * 0.05 + sampleS * 0.05);
    const strategyName = stTrades[0]?.strategyName ?? `Strategy ${strategyId}`;

    raw.push({
      strategyId,
      strategyName,
      metrics: {
        totalTrades: stTrades.length,
        closedTrades: closed.length,
        netPnl,
        winRate,
        profitFactor,
        expectancy,
        sharpeRatio: 0,
        maxDrawdownPct,
      },
      pnlScore: pnlS,
      profitFactorScore: pfS,
      winRateScore: wrS,
      drawdownScore: ddS,
      sharpeScore: sharpeS,
      recencyScore: recencyS,
      sampleSizeScore: sampleS,
      overallScore: overall,
      currentRegimeScore: overall,
      confidenceRating: confidenceFromSample(closed.length),
      rank: 0,
      regimeRank: 0,
      _rawOverall: overall,
    });
  }

  raw.sort((a, b) => b._rawOverall - a._rawOverall);
  return raw.map((r, i) => {
    const { _rawOverall: _, ...score } = r;
    return { ...score, rank: i + 1, regimeRank: i + 1 };
  });
}
