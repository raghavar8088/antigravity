/**
 * Pure session analytics for the browser futures paper desk (closed trades only).
 * Used for expectancy / fee drag / hold-time diagnostics — not live trading advice.
 */

export type SessionTradeLike = {
  openedAt: string;
  closedAt: string;
  netPnl: number;
  fees: number;
  realizedPnl: number;
};

export type SessionTradingMetrics = {
  /** Closed trades / elapsed session hours (from earliest open to now). */
  tradesPerHour: number;
  /** Mean net PnL per closed trade (expectancy proxy). */
  expectancyPerTrade: number;
  /** sum(fees) / sum(|gross|) × 100 — fee drag vs absolute gross. */
  feePctOfAbsGross: number;
  avgHoldMinutes: number;
  medianHoldMinutes: number;
  holdP95Minutes: number;
};

function percentileSorted(sorted: number[], p: number): number {
  if (sorted.length === 0) return 0;
  const idx = Math.min(sorted.length - 1, Math.max(0, Math.floor((sorted.length - 1) * p)));
  return sorted[idx] ?? 0;
}

export function computeSessionTradingMetrics(
  trades: ReadonlyArray<SessionTradeLike>,
  nowMs: number = Date.now(),
): SessionTradingMetrics {
  if (!trades.length) {
    return {
      tradesPerHour: 0,
      expectancyPerTrade: 0,
      feePctOfAbsGross: 0,
      avgHoldMinutes: 0,
      medianHoldMinutes: 0,
      holdP95Minutes: 0,
    };
  }

  const holds: number[] = [];
  let sumNet = 0;
  let sumFees = 0;
  let sumAbsGross = 0;
  let earliest = Infinity;

  for (const t of trades) {
    const o = new Date(t.openedAt).getTime();
    const c = new Date(t.closedAt).getTime();
    if (Number.isFinite(o)) earliest = Math.min(earliest, o);
    if (Number.isFinite(o) && Number.isFinite(c)) {
      holds.push(Math.max(0, (c - o) / 60_000));
    }
    sumNet += t.netPnl;
    sumFees += t.fees;
    sumAbsGross += Math.abs(t.realizedPnl);
  }

  if (!Number.isFinite(earliest)) earliest = nowMs;
  const sessionHours = Math.max(1 / 60, (nowMs - earliest) / 3_600_000);
  const tradesPerHour = trades.length / sessionHours;
  const expectancyPerTrade = sumNet / trades.length;

  const feePctOfAbsGross = sumAbsGross > 1e-12 ? (sumFees / sumAbsGross) * 100 : 0;

  holds.sort((a, b) => a - b);
  const avgHoldMinutes = holds.length ? holds.reduce((s, x) => s + x, 0) / holds.length : 0;
  const medianHoldMinutes = holds.length ? percentileSorted(holds, 0.5) : 0;
  const holdP95Minutes = holds.length ? percentileSorted(holds, 0.95) : 0;

  return {
    tradesPerHour,
    expectancyPerTrade,
    feePctOfAbsGross,
    avgHoldMinutes,
    medianHoldMinutes,
    holdP95Minutes,
  };
}

export type FuturesStrategyProfile = "baseline" | "scalp_aggro_v1";

export const FUTURES_STRATEGY_PROFILES: Record<
  FuturesStrategyProfile,
  { label: string; signalThresholdDelta: number; cooldownMul: number; holdTimeMul: number }
> = {
  baseline: {
    label: "Baseline",
    signalThresholdDelta: 0,
    cooldownMul: 1,
    holdTimeMul: 1,
  },
  scalp_aggro_v1: {
    label: "ScalpAggro v1",
    /** Slightly easier entries (fewer points required vs bar). */
    signalThresholdDelta: -4,
    /** Shorter post-exit lockout → more re-entries (whipsaw risk). */
    cooldownMul: 0.65,
    /** Earlier time exits → higher turnover, more fee events. */
    holdTimeMul: 0.85,
  },
};

export function resolveStrategyProfile(raw: string | undefined): FuturesStrategyProfile {
  return raw === "scalp_aggro_v1" ? "scalp_aggro_v1" : "baseline";
}
