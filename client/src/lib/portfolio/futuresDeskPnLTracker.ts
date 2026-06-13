/**
 * futuresDeskPnLTracker.ts
 * Rolling closed-trade scorecard for paper desk edge verification (48h window).
 */

import { computeSessionTradingMetrics, isProbeOrBootstrapTrade } from "../analytics/futuresSessionMetrics";

export const DESK_PNL_TRACKER_48H_MS = 48 * 60 * 60 * 1000;
export const DESK_PNL_LAST_N_SHORT = 20;
export const DESK_PNL_LAST_N_LONG = 50;

export type DeskPnLSliceMetrics = {
  tradeCount: number;
  expectancy: number;
  winRate: number;
  profitFactor: number;
  feePctOfAbsGross: number;
  sumNet: number;
};

export type DeskPnLTargets = {
  feePctMax: number;
  expectancyMin: number;
  winRateMin: number;
  profitFactorMin: number;
};

export type DeskRollingPnLScorecard = {
  computedAt: number;
  window48hMs: number;
  closes48h: number;
  last20: DeskPnLSliceMetrics;
  last50: DeskPnLSliceMetrics;
  targets: DeskPnLTargets;
  passesFeeTarget50: boolean;
  passesExpectancyTarget50: boolean;
  passesWinRateOrPf50: boolean;
  paperReadyHint: "COLLECT_DATA" | "REVIEW" | "ON_TRACK";
};

export type DeskPnLTradeLike = {
  closedAt: string;
  netPnl: number;
  grossPnl?: number;
  fees?: number;
  strategyName?: string;
};

function productionCloses(trades: DeskPnLTradeLike[]): DeskPnLTradeLike[] {
  return trades.filter(
    (t) =>
      Boolean(t.closedAt) &&
      !isProbeOrBootstrapTrade({ strategyName: t.strategyName ?? "" }),
  );
}

function profitFactorFromNets(nets: number[]): number {
  let wins = 0;
  let losses = 0;
  for (const n of nets) {
    if (n > 0) wins += n;
    else losses += Math.abs(n);
  }
  if (losses < 1e-12) return wins > 0 ? Infinity : 0;
  return wins / losses;
}

function metricsFromSlice(slice: DeskPnLTradeLike[]): DeskPnLSliceMetrics {
  if (!slice.length) {
    return {
      tradeCount: 0,
      expectancy: 0,
      winRate: 0,
      profitFactor: 0,
      feePctOfAbsGross: 0,
      sumNet: 0,
    };
  }
  const nets = slice.map((t) => t.netPnl ?? 0);
  const sumNet = nets.reduce((a, b) => a + b, 0);
  const wins = nets.filter((n) => n > 0).length;
  const session = computeSessionTradingMetrics(
    slice.map((t) => ({
      openedAt: t.closedAt,
      closedAt: t.closedAt,
      netPnl: t.netPnl ?? 0,
      fees: t.fees ?? 0,
      realizedPnl: t.grossPnl ?? t.netPnl ?? 0,
      strategyName: t.strategyName,
    })),
  );
  return {
    tradeCount: slice.length,
    expectancy: sumNet / slice.length,
    winRate: wins / slice.length,
    profitFactor: profitFactorFromNets(nets),
    feePctOfAbsGross: session.feePctOfAbsGross,
    sumNet,
  };
}

const DEFAULT_TARGETS: DeskPnLTargets = {
  feePctMax: 50,
  expectancyMin: 0,
  winRateMin: 0.35,
  profitFactorMin: 1.0,
};

export function computeDeskRollingPnLScorecard(
  trades: DeskPnLTradeLike[],
  nowMs: number = Date.now(),
  targets: DeskPnLTargets = DEFAULT_TARGETS,
): DeskRollingPnLScorecard {
  const prod = productionCloses(trades);
  const sorted = [...prod].sort(
    (a, b) => new Date(b.closedAt).getTime() - new Date(a.closedAt).getTime(),
  );
  const since48h = nowMs - DESK_PNL_TRACKER_48H_MS;
  const in48h = sorted.filter((t) => new Date(t.closedAt).getTime() >= since48h);
  const last20 = metricsFromSlice(sorted.slice(0, DESK_PNL_LAST_N_SHORT));
  const last50 = metricsFromSlice(sorted.slice(0, DESK_PNL_LAST_N_LONG));

  const passesFeeTarget50 = last50.tradeCount >= 10 && last50.feePctOfAbsGross < targets.feePctMax;
  const passesExpectancyTarget50 =
    last50.tradeCount >= 10 && last50.expectancy > targets.expectancyMin;
  const passesWinRateOrPf50 =
    last50.tradeCount >= 10 &&
    (last50.winRate >= targets.winRateMin || last50.profitFactor >= targets.profitFactorMin);

  let paperReadyHint: DeskRollingPnLScorecard["paperReadyHint"] = "COLLECT_DATA";
  if (last50.tradeCount >= 50) {
    paperReadyHint =
      passesFeeTarget50 && passesExpectancyTarget50 && passesWinRateOrPf50
        ? "ON_TRACK"
        : "REVIEW";
  } else if (last20.tradeCount >= 20) {
    paperReadyHint = "REVIEW";
  }

  return {
    computedAt: nowMs,
    window48hMs: DESK_PNL_TRACKER_48H_MS,
    closes48h: in48h.length,
    last20,
    last50,
    targets,
    passesFeeTarget50,
    passesExpectancyTarget50,
    passesWinRateOrPf50,
    paperReadyHint,
  };
}
