/**
 * Rolling strategy expectancy analytics for paper-trade kill-switch (Supabase `paper_trades`).
 */

export type StratTradeRow = {
  strategyId: number;
  netPnl: number;
  closedAt?: string;
  /** Optional: gross PnL before fees (for feePctOfGross calculation). */
  grossPnl?: number;
  /** Optional: total fees on this trade. */
  fees?: number;
  /** Optional: openΓåÆclose duration in minutes. */
  holdMinutes?: number;
};

export type StrategyStatRow = {
  strategyId: number;
  tradeCount: number;
  sumNet: number;
  expectancy: number;
};

export type LeaderboardTradeRow = StratTradeRow & {
  strategyName: string;
};

export type StrategyLeaderboardRow = {
  strategyId: number;
  strategyName: string;
  tradeCount: number;
  sumNet: number;
  expectancy: number;
  /** Wins / tradeCount (netPnl > 0). */
  winRate: number;
};

export type BuildStratDisableOpts = {
  /** Minimum closed trades before a strategy can be auto-disabled. */
  minTrades: number;
  /**
   * Disable when `tradeCount >= minTrades` and **either**:
   * - mean net per trade (`expectancy`) is below this threshold, **or**
   * - cumulative `sumNet` is below `maxSumNetUsd`.
   */
  maxExpectancyUsd: number;
  maxSumNetUsd: number;
};

/**
 * Minimum closed trades before a strategy can be auto-disabled.
 * Set to 100 per the 2026 blueprint: below this sample size expectancy
 * estimates are statistically unreliable and prone to false kills.
 */
export const DESK_KILL_MIN_TRADES_DEFAULT = 100;
export const DESK_KILL_MAX_EXPECTANCY_USD_DEFAULT = -0.05;
export const DESK_KILL_MAX_SUM_NET_USD_DEFAULT = -1;

const DEFAULT_DISABLE_OPTS: BuildStratDisableOpts = {
  minTrades: DESK_KILL_MIN_TRADES_DEFAULT,
  maxExpectancyUsd: DESK_KILL_MAX_EXPECTANCY_USD_DEFAULT,
  maxSumNetUsd: DESK_KILL_MAX_SUM_NET_USD_DEFAULT,
};

/**
 * Proper quantitative expectancy: E = (W ├ù Pavg) ΓêÆ (L ├ù Lavg)
 *
 * Where:
 *  W    = win rate (wins / total)
 *  Pavg = average net PnL of winning trades
 *  L    = loss rate (1 ΓêÆ W)
 *  Lavg = average absolute net PnL of losing trades
 *
 * Degrades gracefully to sumNet/count when no wins or no losses exist.
 */
export function calcQuantExpectancy(rows: StratTradeRow[]): number {
  if (rows.length === 0) return 0;
  const wins = rows.filter((r) => r.netPnl > 0);
  const losses = rows.filter((r) => r.netPnl <= 0);
  const W = wins.length / rows.length;
  const L = losses.length / rows.length;
  const Pavg = wins.length > 0 ? wins.reduce((s, r) => s + r.netPnl, 0) / wins.length : 0;
  const Lavg = losses.length > 0 ? Math.abs(losses.reduce((s, r) => s + r.netPnl, 0) / losses.length) : 0;
  return W * Pavg - L * Lavg;
}

/** Aggregate closed trades by `strategyId` using proper W├ùPavg ΓêÆ L├ùLavg expectancy. */
export function aggregateStrategyStats(rows: StratTradeRow[]): StrategyStatRow[] {
  const map = new Map<number, StratTradeRow[]>();
  for (const row of rows) {
    if (!Number.isFinite(row.strategyId) || !Number.isFinite(row.netPnl)) continue;
    const bucket = map.get(row.strategyId) ?? [];
    bucket.push(row);
    map.set(row.strategyId, bucket);
  }
  return [...map.entries()]
    .map(([strategyId, bucket]) => ({
      strategyId,
      tradeCount: bucket.length,
      sumNet: bucket.reduce((s, r) => s + r.netPnl, 0),
      expectancy: calcQuantExpectancy(bucket),
    }))
    .sort((a, b) => a.sumNet - b.sumNet);
}

/**
 * Build the set of strategy IDs to auto-disable (no auto re-enable).
 *
 * Rule: disable `strategyId` only when `tradeCount >= minTrades` **and**
 * (`expectancy < maxExpectancyUsd` **or** `sumNet < maxSumNetUsd`).
 */
export function buildStratDisableSet(
  rows: StratTradeRow[],
  opts?: Partial<BuildStratDisableOpts>,
): Set<number> {
  return buildStratDisableSetFromStats(aggregateStrategyStats(rows), opts);
}

export function buildStratDisableSetFromStats(
  stats: StrategyStatRow[],
  opts?: Partial<BuildStratDisableOpts>,
): Set<number> {
  const o = { ...DEFAULT_DISABLE_OPTS, ...opts };
  const out = new Set<number>();
  for (const s of stats) {
    if (s.tradeCount < o.minTrades) continue;
    if (s.expectancy < o.maxExpectancyUsd || s.sumNet < o.maxSumNetUsd) {
      out.add(s.strategyId);
    }
  }
  return out;
}

/** Comma-separated strat IDs for UI/logging (sorted, truncated). */
/** Merge manual disable IDs (dedupe, stable ascending sort for persistence). */
export function mergeDisabledStrategyIds(
  current: ReadonlySet<number> | readonly number[],
  add: readonly number[],
): number[] {
  const set = new Set<number>(current instanceof Set ? current : current);
  for (const id of add) {
    if (!Number.isFinite(id)) continue;
    const n = Math.floor(id);
    if (n > 0) set.add(n);
  }
  return [...set].sort((a, b) => a - b);
}

export function formatAutoDisabledStratIds(ids: Iterable<number>, maxLen = 96): string {
  const sorted = [...ids].sort((a, b) => a - b);
  let s = sorted.join(",");
  if (s.length > maxLen) s = `${s.slice(0, maxLen - 1)}ΓÇª`;
  return s;
}

export function strategyStatsRowsFromDb(
  rows: ReadonlyArray<{ strategy_id: number; net_pnl: number; closed_at: string }>,
): StratTradeRow[] {
  return rows.map((r) => ({
    strategyId: r.strategy_id,
    netPnl: r.net_pnl,
    closedAt: r.closed_at,
  }));
}

/** Aggregate closed trades by `strategyId` (includes name + win rate for leaderboard). */
export function aggregateStrategyLeaderboard(rows: LeaderboardTradeRow[]): StrategyLeaderboardRow[] {
  const map = new Map<number, { name: string; trades: LeaderboardTradeRow[] }>();
  for (const row of rows) {
    if (!Number.isFinite(row.strategyId) || !Number.isFinite(row.netPnl)) continue;
    const cur = map.get(row.strategyId) ?? {
      name: row.strategyName?.trim() || `Strat ${row.strategyId}`,
      trades: [],
    };
    cur.trades.push(row);
    if (row.strategyName?.trim()) cur.name = row.strategyName.trim();
    map.set(row.strategyId, cur);
  }
  return [...map.entries()]
    .map(([strategyId, { name, trades }]) => ({
      strategyId,
      strategyName: name,
      tradeCount: trades.length,
      sumNet: trades.reduce((s, r) => s + r.netPnl, 0),
      expectancy: calcQuantExpectancy(trades),
      winRate: trades.length > 0 ? trades.filter((r) => r.netPnl > 0).length / trades.length : 0,
    }))
    .sort((a, b) => b.sumNet - a.sumNet);
}

/** Best `limit` by sumNet desc; worst `limit` by sumNet asc. */
export function splitLeaderboardTopBottom(
  stats: StrategyLeaderboardRow[],
  limitPerSide: number,
): { top: StrategyLeaderboardRow[]; bottom: StrategyLeaderboardRow[] } {
  const cap = Math.max(1, Math.floor(limitPerSide));
  const sortedDesc = [...stats].sort((a, b) => b.sumNet - a.sumNet);
  const top = sortedDesc.slice(0, cap);
  const bottom = [...stats].sort((a, b) => a.sumNet - b.sumNet).slice(0, cap);
  return { top, bottom };
}


export function leaderboardRowsFromDb(
  rows: ReadonlyArray<{
    strategy_id: number;
    strategy_name: string;
    net_pnl: number;
    closed_at: string;
  }>,
): LeaderboardTradeRow[] {
  return rows.map((r) => ({
    strategyId: r.strategy_id,
    strategyName: r.strategy_name,
    netPnl: r.net_pnl,
    closedAt: r.closed_at,
  }));
}

// ---------------------------------------------------------------------------
// Research tournament: richer per-strategy stats + verdict
// ---------------------------------------------------------------------------

export type ResearchDbRow = {
  strategy_id: number;
  strategy_name?: string;
  net_pnl: number;
  gross_pnl?: number;
  fees?: number;
  opened_at?: string;
  closed_at?: string;
  /** module_key for cross-module leaderboards. */
  module_key?: string;
  template_family?: string;
};

export type ResearchAggRow = {
  strategyId: number;
  strategyName: string;
  tradeCount: number;
  sumNet: number;
  /** E = W ├ù Pavg ΓêÆ L ├ù Lavg (proper quant expectancy). */
  expectancy: number;
  winRate: number;
  /** sum(fees) / sum(|gross_pnl|) * 100 ΓÇö null when gross is 0. */
  feePctOfGross: number | null;
  avgHoldMin: number | null;
  lastTradeAt: string | null;
  /** Maximum Adverse Excursion proxy: avg loss magnitude on losing trades. */
  mae: number | null;
  /** Maximum Favorable Excursion proxy: avg gross on winning trades. */
  mfe: number | null;
  /** Sharpe-like: expectancy / stddev of net PnL (null when stddev = 0). */
  sharpeProxy: number | null;
};

/** Aggregate research tournament stats from raw DB rows (proper expectancy + MAE/MFE/Sharpe). */
export function aggregateResearchStratStats(rows: ResearchDbRow[]): ResearchAggRow[] {
  const map = new Map<
    number,
    {
      name: string;
      trades: number[];
      grossWins: number[];
      grossLosses: number[];
      sumGross: number;
      sumFees: number;
      sumHoldMin: number;
      holdMinCount: number;
      lastAt: string | null;
    }
  >();

  for (const r of rows) {
    if (!Number.isFinite(r.strategy_id) || !Number.isFinite(r.net_pnl)) continue;
    const cur = map.get(r.strategy_id) ?? {
      name: r.strategy_name?.trim() || `Strat ${r.strategy_id}`,
      trades: [],
      grossWins: [],
      grossLosses: [],
      sumGross: 0,
      sumFees: 0,
      sumHoldMin: 0,
      holdMinCount: 0,
      lastAt: null,
    };

    cur.trades.push(r.net_pnl);
    if (r.net_pnl > 0) {
      cur.grossWins.push(typeof r.gross_pnl === "number" ? r.gross_pnl : r.net_pnl);
    } else {
      cur.grossLosses.push(typeof r.gross_pnl === "number" ? r.gross_pnl : r.net_pnl);
    }
    if (typeof r.gross_pnl === "number" && Number.isFinite(r.gross_pnl)) cur.sumGross += Math.abs(r.gross_pnl);
    if (typeof r.fees === "number" && Number.isFinite(r.fees)) cur.sumFees += r.fees;
    if (r.strategy_name?.trim()) cur.name = r.strategy_name.trim();

    if (r.opened_at && r.closed_at) {
      const openMs = Date.parse(r.opened_at);
      const closeMs = Date.parse(r.closed_at);
      if (Number.isFinite(openMs) && Number.isFinite(closeMs) && closeMs > openMs) {
        cur.sumHoldMin += (closeMs - openMs) / 60_000;
        cur.holdMinCount += 1;
      }
    }

    if (r.closed_at && (!cur.lastAt || r.closed_at > cur.lastAt)) cur.lastAt = r.closed_at;
    map.set(r.strategy_id, cur);
  }

  return [...map.entries()]
    .map(([strategyId, s]) => {
      const tradeRows: StratTradeRow[] = s.trades.map((netPnl) => ({ strategyId, netPnl }));
      const expectancy = calcQuantExpectancy(tradeRows);
      const sumNet = s.trades.reduce((a, b) => a + b, 0);
      const winCount = s.grossWins.length;
      const lossCount = s.grossLosses.length;
      const count = s.trades.length;

      // Sharpe proxy: expectancy / stddev
      let sharpeProxy: number | null = null;
      if (count > 1) {
        const mean = sumNet / count;
        const variance = s.trades.reduce((acc, v) => acc + (v - mean) ** 2, 0) / count;
        const stddev = Math.sqrt(variance);
        if (stddev > 0) sharpeProxy = Math.round((expectancy / stddev) * 1000) / 1000;
      }

      return {
        strategyId,
        strategyName: s.name,
        tradeCount: count,
        sumNet: Math.round(sumNet * 100) / 100,
        expectancy: Math.round(expectancy * 1000) / 1000,
        winRate: count > 0 ? Math.round((winCount / count) * 1000) / 1000 : 0,
        feePctOfGross: s.sumGross > 0 ? Math.round((s.sumFees / s.sumGross) * 10000) / 100 : null,
        avgHoldMin: s.holdMinCount > 0 ? Math.round((s.sumHoldMin / s.holdMinCount) * 10) / 10 : null,
        lastTradeAt: s.lastAt,
        // MAE: avg absolute loss on losing trades (intra-trade drawdown proxy)
        mae: lossCount > 0 ? Math.round((Math.abs(s.grossLosses.reduce((a, b) => a + b, 0)) / lossCount) * 100) / 100 : null,
        // MFE: avg gross on winning trades (TP too conservative if much > netPnl avg)
        mfe: winCount > 0 ? Math.round((s.grossWins.reduce((a, b) => a + b, 0) / winCount) * 100) / 100 : null,
        sharpeProxy,
      };
    })
    .sort((a, b) => b.sumNet - a.sumNet);
}

