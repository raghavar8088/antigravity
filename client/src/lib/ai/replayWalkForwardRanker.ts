/**
 * Replay + Walk-Forward ranker — pure, no I/O.
 *
 * Takes BTCFuturesTrade[] from a replay run and ranks each strategy by
 * expectancy + walk-forward efficiency. Returns promotion/watch/disable
 * recommendations. Operator decides whether to apply.
 *
 * Hard invariants:
 *   - Never promote if replayTrades < 20, WFE < 50%, or fee/gross > 50%.
 *   - Disable if expectancy < 0 and fee/gross > 100%.
 *   - No threshold lowering, no gate bypassing, no guaranteed profit claims.
 */

import type { BTCFuturesTrade } from "@/lib/trading/btcFuturesTrade.types";
import { runWalkForwardValidation } from "@/lib/analytics/walkForwardValidation";
import type { WalkForwardResult } from "@/lib/analytics/walkForwardValidation";

// ─── Public types ─────────────────────────────────────────────────────────────

export type ReplayRankRecommendation = "PROMOTE" | "KEEP" | "WATCH" | "DISABLE" | "INSUFFICIENT";

export interface ReplayWalkForwardRank {
  strategyId: number;
  strategyName: string;
  replayTrades: number;
  replayExpectancy: number;
  replayWinRate: number;
  replaySumNet: number;
  replayFeePctOfAbsGross: number;
  walkForward: WalkForwardResult;
  recommendation: ReplayRankRecommendation;
  recommendationReason: string;
}

// ─── Constants ────────────────────────────────────────────────────────────────

const MIN_TRADES_FOR_ANY_VERDICT = 5;
const MIN_TRADES_TO_PROMOTE = 20;
const MAX_FEE_PCT = 50;
const HIGH_FEE_DISABLE_PCT = 100;

// ─── Helpers ─────────────────────────────────────────────────────────────────

function computeFeePct(trades: BTCFuturesTrade[]): number {
  const totalFees = trades.reduce((s, t) => s + t.fees, 0);
  const absGross = trades.reduce((s, t) => s + Math.abs(t.realizedPnl), 0);
  if (absGross === 0) return 0;
  return (totalFees / absGross) * 100;
}

function classify(
  n: number,
  expectancy: number,
  feePctOfAbsGross: number,
  walkForwardPass: boolean,
): { recommendation: ReplayRankRecommendation; reason: string } {
  if (n < MIN_TRADES_FOR_ANY_VERDICT) {
    return {
      recommendation: "INSUFFICIENT",
      reason: `Only ${n} trade${n !== 1 ? "s" : ""} — need ≥${MIN_TRADES_FOR_ANY_VERDICT} for any verdict.`,
    };
  }

  if (expectancy < 0 && feePctOfAbsGross > HIGH_FEE_DISABLE_PCT) {
    return {
      recommendation: "DISABLE",
      reason: `Negative expectancy ($${expectancy.toFixed(2)}/trade) and fee/gross ${feePctOfAbsGross.toFixed(0)}% — fee drag destroying edge.`,
    };
  }

  if (n < MIN_TRADES_TO_PROMOTE) {
    return {
      recommendation: "WATCH",
      reason: `${n}/${MIN_TRADES_TO_PROMOTE} trades needed to promote. Gathering data.`,
    };
  }

  if (!walkForwardPass) {
    return {
      recommendation: expectancy > 0 ? "KEEP" : "WATCH",
      reason: `Walk-forward failed (WFE < 50%). Cannot promote until out-of-sample performance improves.`,
    };
  }

  if (expectancy > 0 && feePctOfAbsGross <= MAX_FEE_PCT) {
    return {
      recommendation: "PROMOTE",
      reason: `${n} trades, expectancy $${expectancy.toFixed(2)}/trade, fee/gross ${feePctOfAbsGross.toFixed(0)}%, walk-forward PASS — all criteria met.`,
    };
  }

  if (expectancy <= 0) {
    return {
      recommendation: "WATCH",
      reason: `Negative expectancy ($${expectancy.toFixed(2)}/trade) over ${n} trades.`,
    };
  }

  return {
    recommendation: "KEEP",
    reason: `Positive expectancy but fee/gross ${feePctOfAbsGross.toFixed(0)}% exceeds ${MAX_FEE_PCT}% cap. Observe further.`,
  };
}

const REC_ORDER: Record<ReplayRankRecommendation, number> = {
  PROMOTE: 0,
  KEEP: 1,
  WATCH: 2,
  DISABLE: 3,
  INSUFFICIENT: 4,
};

// ─── Main export ──────────────────────────────────────────────────────────────

export function rankReplayStrategies(trades: BTCFuturesTrade[]): ReplayWalkForwardRank[] {
  const byId = new Map<number, { name: string; trades: BTCFuturesTrade[] }>();
  for (const t of trades) {
    if (!byId.has(t.strategyId)) byId.set(t.strategyId, { name: t.strategyName, trades: [] });
    byId.get(t.strategyId)!.trades.push(t);
  }

  const ranks: ReplayWalkForwardRank[] = [];

  for (const [strategyId, { name, trades: stratTrades }] of byId) {
    const n = stratTrades.length;
    const sumNet = stratTrades.reduce((s, t) => s + t.netPnl, 0);
    const expectancy = n > 0 ? sumNet / n : 0;
    const wins = stratTrades.filter((t) => t.netPnl > 0).length;
    const winRate = n > 0 ? wins / n : 0;
    const feePctOfAbsGross = computeFeePct(stratTrades);

    const wfTrades = stratTrades.map((t) => ({ closedAt: t.closedAt, netPnl: t.netPnl }));
    const walkForward = runWalkForwardValidation(wfTrades);

    const { recommendation, reason } = classify(n, expectancy, feePctOfAbsGross, walkForward.aggregatePass);

    ranks.push({
      strategyId,
      strategyName: name,
      replayTrades: n,
      replayExpectancy: expectancy,
      replayWinRate: winRate,
      replaySumNet: sumNet,
      replayFeePctOfAbsGross: feePctOfAbsGross,
      walkForward,
      recommendation,
      recommendationReason: reason,
    });
  }

  return ranks.sort((a, b) => {
    const ro = REC_ORDER[a.recommendation] - REC_ORDER[b.recommendation];
    if (ro !== 0) return ro;
    return b.replayExpectancy - a.replayExpectancy;
  });
}
