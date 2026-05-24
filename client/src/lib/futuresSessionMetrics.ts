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
  /** Optional strategy name — used to exclude probe/bootstrap trades from metrics. */
  strategyName?: string;
};

/**
 * Returns true for synthetic probe/bootstrap trades that must be excluded from
 * session metrics (they have no signal edge and distort expectancy + fee ratios).
 * Handles both camelCase (BTCFuturesTrade) and snake_case (PaperTradeDbRow) fields.
 */
export function isProbeOrBootstrapTrade(
  t: { strategy_name?: string | null; strategyName?: string | null },
): boolean {
  const name = (t.strategy_name ?? t.strategyName ?? "").toUpperCase();
  return name.includes("BOOTSTRAP") || name.includes("PROBE") || name.includes("DEV_FORCE");
}

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
  // Exclude synthetic probe/bootstrap trades — they have no signal edge and
  // inflate/deflate expectancy, fee-ratio, and hold-time metrics.
  const productionTrades = trades.filter(
    (t) => !isProbeOrBootstrapTrade({ strategyName: t.strategyName }),
  );

  if (process.env.NODE_ENV === "development") {
    const probeCount = trades.filter((t) =>
      isProbeOrBootstrapTrade({ strategyName: t.strategyName }),
    ).length;
    if (probeCount > 0) {
      console.error(
        `[Metrics] WARNING: ${probeCount} probe trades reached metrics. ` +
          "isProbeOrBootstrapTrade filter may not be applied upstream.",
      );
    }
  }

  if (!productionTrades.length) {
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

  for (const t of productionTrades) {
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
  const tradesPerHour = productionTrades.length / sessionHours;
  const expectancyPerTrade = sumNet / productionTrades.length;

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

/**
 * Production-aligned session equity — excludes probe/bootstrap balance mutations.
 * Use this instead of `equity - baseBalance` when probe trades may have corrupted balance.
 */
export function computeSessionEquityFromProduction(params: {
  initialBalance: number;
  productionClosedNetPnl: number;
  productionUnrealizedPnl: number;
}): { sessionPnL: number; equity: number; totalReturnPct: number } {
  const { initialBalance, productionClosedNetPnl, productionUnrealizedPnl } = params;
  const sessionPnL = productionClosedNetPnl + productionUnrealizedPnl;
  const equity = initialBalance + sessionPnL;
  const totalReturnPct = initialBalance > 0 ? (sessionPnL / initialBalance) * 100 : 0;
  return { sessionPnL, equity, totalReturnPct };
}

export type FuturesStrategyProfile = "baseline" | "scalp_aggro_v1" | "fee_aware_v1";

/**
 * Desk coherence: `buildPaperDeskStrategies` may **widen** `tpPct` (min TP/SL vs fees). The
 * `scalp_aggro_v1` profile shortens time exits via `holdTimeMul` below 1, which otherwise fights
 * wider targets (more TIME exits before TP). For **widened** strats under **scalp_aggro_v1** only,
 * `deskEffectiveHoldMinutesAtOpen` lengthens base `holdMinutes` by `HOLD_MUL_AFTER_TP_WIDEN` before
 * the existing `holdTimeMul` is applied in `paperResolveHardExit` — do not stack ad-hoc extra
 * `holdTimeMul` cuts on top of that without review.
 */
export type FuturesStrategyProfileConfig = {
  label: string;
  signalThresholdDelta: number;
  cooldownMul: number;
  holdTimeMul: number;
  /** Multiplier on desk `minExpectedMoveSafetyK` (env + profile). */
  minExpectedMoveSafetyKMul: number;
};

export const FUTURES_STRATEGY_PROFILES: Record<FuturesStrategyProfile, FuturesStrategyProfileConfig> = {
  baseline: {
    label: "Baseline",
    signalThresholdDelta: 0,
    cooldownMul: 1,
    holdTimeMul: 1,
    minExpectedMoveSafetyKMul: 1,
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
    minExpectedMoveSafetyKMul: 1,
  },
  fee_aware_v1: {
    label: "FeeAware v1",
    /** Stricter entries vs baseline (higher bar score required). */
    signalThresholdDelta: 6,
    cooldownMul: 1,
    holdTimeMul: 1,
    /** Tighter ATR$ vs fee hurdle (`deskMinExpectedMoveSafetyK × 1.25`). */
    minExpectedMoveSafetyKMul: 1.25,
  },
};

export function resolveStrategyProfile(raw: string | undefined): FuturesStrategyProfile {
  if (raw === "scalp_aggro_v1") return "scalp_aggro_v1";
  if (raw === "fee_aware_v1") return "fee_aware_v1";
  return "baseline";
}

export function effectiveMinExpectedMoveSafetyK(
  baseK: number,
  profileCfg: Pick<FuturesStrategyProfileConfig, "minExpectedMoveSafetyKMul">,
): number {
  return baseK * profileCfg.minExpectedMoveSafetyKMul;
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
