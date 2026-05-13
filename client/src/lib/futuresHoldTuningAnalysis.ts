/**
 * Pure hold-tuning analytics: per strategy × deskTpWidened buckets, exitReason breakdown.
 * Used by session stats UI and dev-only `__deskHoldTuningDump()` (see `deskHoldTuningAnalysisModeEnabled`).
 */

export type TradeRowForHoldTuning = {
  strategyId: number;
  strategyName: string;
  category: string;
  exitReason: string;
  netPnl: number;
};

type MutableReasonAgg = { count: number; sumNet: number };

export type ReasonAgg = {
  count: number;
  sumNet: number;
  meanNet: number;
};

export type StrategyExitDeskBucket = {
  /** `${strategyId}:w|nw` */
  key: string;
  strategyId: number;
  strategyName: string;
  category: string;
  deskTpWidened: boolean;
  /** Raw counts/sums; mean derived in views / dump. */
  perReason: Record<string, MutableReasonAgg>;
};

type MutableBucket = Omit<StrategyExitDeskBucket, "perReason"> & {
  perReason: Record<string, MutableReasonAgg>;
};

function reasonAggToView(a: MutableReasonAgg | undefined): ReasonAgg {
  if (!a || a.count === 0) return { count: 0, sumNet: 0, meanNet: 0 };
  return { count: a.count, sumNet: a.sumNet, meanNet: a.sumNet / a.count };
}

function finalizeBucket(b: MutableBucket): StrategyExitDeskBucket {
  return { ...b, perReason: { ...b.perReason } };
}

/**
 * Reducer: last-N closed trades → one row per (strategyId, deskTpWidened at build time).
 */
export function reduceTradesToStrategyDeskBuckets(
  trades: readonly TradeRowForHoldTuning[],
  stratDeskWidenedById: ReadonlyMap<number, boolean>,
  stratCategoryById: ReadonlyMap<number, string>,
  lastN: number,
): StrategyExitDeskBucket[] {
  const window = trades.length <= lastN ? [...trades] : trades.slice(-lastN);
  const byKey = new Map<string, MutableBucket>();

  for (const t of window) {
    const deskTpWidened = stratDeskWidenedById.get(t.strategyId) === true;
    const key = `${t.strategyId}:${deskTpWidened ? "w" : "nw"}`;
    const reason = (t.exitReason ?? "UNKNOWN").trim() || "UNKNOWN";
    const category = t.category || stratCategoryById.get(t.strategyId) || "?";

    let row = byKey.get(key);
    if (!row) {
      row = {
        key,
        strategyId: t.strategyId,
        strategyName: t.strategyName,
        category,
        deskTpWidened,
        perReason: {},
      };
      byKey.set(key, row);
    }
    const pr = row.perReason[reason] ?? { count: 0, sumNet: 0 };
    pr.count += 1;
    pr.sumNet += t.netPnl;
    row.perReason[reason] = pr;
  }

  return [...byKey.values()]
    .map(finalizeBucket)
    .sort((a, b) => a.strategyId - b.strategyId || (a.deskTpWidened === b.deskTpWidened ? 0 : a.deskTpWidened ? 1 : -1));
}

export type WorstTimeOffenderRow = {
  strategyId: number;
  strategyName: string;
  category: string;
  deskTpWidened: boolean;
  timeCount: number;
  timeSumNet: number;
  timeMeanNet: number;
  tpCount: number;
  tpMeanNet: number;
};

/**
 * Top-K strategies by worst total net from TIME exits (sumNet ascending).
 */
export function rankWorstTimeOffenders(buckets: readonly StrategyExitDeskBucket[], topK = 5): WorstTimeOffenderRow[] {
  const rows: WorstTimeOffenderRow[] = [];
  for (const b of buckets) {
    const timeA = b.perReason.TIME;
    if (!timeA || timeA.count === 0) continue;
    const tpA = b.perReason.TP;
    rows.push({
      strategyId: b.strategyId,
      strategyName: b.strategyName,
      category: b.category,
      deskTpWidened: b.deskTpWidened,
      timeCount: timeA.count,
      timeSumNet: timeA.sumNet,
      timeMeanNet: timeA.sumNet / timeA.count,
      tpCount: tpA?.count ?? 0,
      tpMeanNet: tpA && tpA.count > 0 ? tpA.sumNet / tpA.count : 0,
    });
  }
  rows.sort((a, b) => a.timeSumNet - b.timeSumNet);
  return rows.slice(0, topK);
}

export type DeskHoldTuningDumpBucket = {
  strategyId: number;
  strategyName: string;
  category: string;
  deskTpWidened: boolean;
  exitReasons: Record<string, ReasonAgg>;
};

export function buildDeskHoldTuningDumpPayload(
  trades: readonly TradeRowForHoldTuning[],
  stratDeskWidenedById: ReadonlyMap<number, boolean>,
  stratCategoryById: ReadonlyMap<number, string>,
  lastN: number,
): {
  window: number;
  lastN: number;
  bucketCount: number;
  note: string;
  buckets: DeskHoldTuningDumpBucket[];
} {
  const buckets = reduceTradesToStrategyDeskBuckets(trades, stratDeskWidenedById, stratCategoryById, lastN);
  const window = trades.length <= lastN ? trades.length : lastN;
  return {
    window,
    lastN,
    bucketCount: buckets.length,
    note:
      "Dev: NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE=1 + NODE_ENV=development → __deskHoldTuningDump() in console.",
    buckets: buckets.map((b) => ({
      strategyId: b.strategyId,
      strategyName: b.strategyName,
      category: b.category,
      deskTpWidened: b.deskTpWidened,
      exitReasons: Object.fromEntries(
        Object.entries(b.perReason).map(([k, v]) => [k, reasonAggToView(v)]),
      ),
    })),
  };
}
