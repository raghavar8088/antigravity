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

/**
 * Desk coherence: `buildPaperDeskStrategies` may **widen** `tpPct` (min TP/SL vs fees). The
 * `scalp_aggro_v1` profile shortens time exits via `holdTimeMul` below 1, which otherwise fights
 * wider targets (more TIME exits before TP). For **widened** strats under **scalp_aggro_v1** only,
 * `deskEffectiveHoldMinutesAtOpen` lengthens base `holdMinutes` by `HOLD_MUL_AFTER_TP_WIDEN` before
 * the existing `holdTimeMul` is applied in `paperResolveHardExit` — do not stack ad-hoc extra
 * `holdTimeMul` cuts on top of that without review.
 */
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
    /**
     * Earlier time exits vs baseline (`×0.85` in `paperResolveHardExit`). Widened-TP strats get
     * a compensating **base** hold bump in the hook — see module doc above.
     */
    holdTimeMul: 0.85,
  },
};

export function resolveStrategyProfile(raw: string | undefined): FuturesStrategyProfile {
  return raw === "scalp_aggro_v1" ? "scalp_aggro_v1" : "baseline";
}

/** Closed trade with optional exit reason (paper desk). */
export type SessionClosedTradeWithReason = SessionTradeLike & { exitReason?: string };

export type ExitReasonSessionRow = {
  reason: string;
  count: number;
  /** Mean net PnL for this exit reason in the window (expectancy slice). */
  avgNet: number;
};

/**
 * Last-N closed trades: counts and mean net per `exitReason` (which exit types pay vs bleed).
 * Not a Pearson correlation — grouped expectancy by bucket vs histogram counts.
 */
export function computeSessionExitReasonAnalytics(
  trades: readonly SessionClosedTradeWithReason[],
  lastN = 400,
): { rows: ExitReasonSessionRow[]; totalInWindow: number } {
  const window = trades.length <= lastN ? [...trades] : trades.slice(-lastN);
  const by = new Map<string, { sum: number; n: number }>();
  for (const t of window) {
    const r = (t.exitReason ?? "UNKNOWN").trim() || "UNKNOWN";
    const cur = by.get(r) ?? { sum: 0, n: 0 };
    cur.sum += t.netPnl;
    cur.n += 1;
    by.set(r, cur);
  }
  const rows = [...by.entries()]
    .map(([reason, v]) => ({ reason, count: v.n, avgNet: v.n ? v.sum / v.n : 0 }))
    .sort((a, b) => b.count - a.count);
  return { rows, totalInWindow: window.length };
}

export function formatExitReasonSessionSummary(rows: readonly ExitReasonSessionRow[], maxReasons = 8): string {
  if (!rows.length) return "—";
  return rows
    .slice(0, maxReasons)
    .map((r) => `${r.reason}×${r.count} ${r.avgNet >= 0 ? "avg+" : "avg"}${r.avgNet.toFixed(2)}`)
    .join(" · ");
}
