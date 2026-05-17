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
  /** Optional: open→close duration in minutes. */
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

export const DESK_KILL_MIN_TRADES_DEFAULT = 5;
export const DESK_KILL_MAX_EXPECTANCY_USD_DEFAULT = -0.05;
export const DESK_KILL_MAX_SUM_NET_USD_DEFAULT = -1;

const DEFAULT_DISABLE_OPTS: BuildStratDisableOpts = {
  minTrades: DESK_KILL_MIN_TRADES_DEFAULT,
  maxExpectancyUsd: DESK_KILL_MAX_EXPECTANCY_USD_DEFAULT,
  maxSumNetUsd: DESK_KILL_MAX_SUM_NET_USD_DEFAULT,
};

/** Aggregate closed trades by `strategyId` (expectancy = sumNet / tradeCount). */
export function aggregateStrategyStats(rows: StratTradeRow[]): StrategyStatRow[] {
  const map = new Map<number, { count: number; sum: number }>();
  for (const row of rows) {
    if (!Number.isFinite(row.strategyId) || !Number.isFinite(row.netPnl)) continue;
    const cur = map.get(row.strategyId) ?? { count: 0, sum: 0 };
    cur.count += 1;
    cur.sum += row.netPnl;
    map.set(row.strategyId, cur);
  }
  return [...map.entries()]
    .map(([strategyId, { count, sum }]) => ({
      strategyId,
      tradeCount: count,
      sumNet: sum,
      expectancy: count > 0 ? sum / count : 0,
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
  if (s.length > maxLen) s = `${s.slice(0, maxLen - 1)}…`;
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
  const map = new Map<number, { name: string; count: number; sum: number; wins: number }>();
  for (const row of rows) {
    if (!Number.isFinite(row.strategyId) || !Number.isFinite(row.netPnl)) continue;
    const cur = map.get(row.strategyId) ?? {
      name: row.strategyName?.trim() || `Strat ${row.strategyId}`,
      count: 0,
      sum: 0,
      wins: 0,
    };
    cur.count += 1;
    cur.sum += row.netPnl;
    if (row.netPnl > 0) cur.wins += 1;
    if (row.strategyName?.trim()) cur.name = row.strategyName.trim();
    map.set(row.strategyId, cur);
  }
  return [...map.entries()]
    .map(([strategyId, { name, count, sum, wins }]) => ({
      strategyId,
      strategyName: name,
      tradeCount: count,
      sumNet: sum,
      expectancy: count > 0 ? sum / count : 0,
      winRate: count > 0 ? wins / count : 0,
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
};

export type ResearchAggRow = {
  strategyId: number;
  strategyName: string;
  tradeCount: number;
  sumNet: number;
  expectancy: number;
  winRate: number;
  /** sum(fees) / sum(|gross_pnl|) * 100 — null when gross is 0. */
  feePctOfGross: number | null;
  avgHoldMin: number | null;
  lastTradeAt: string | null;
};

/** Aggregate research tournament stats from raw DB rows (includes hold time + fee ratio). */
export function aggregateResearchStratStats(rows: ResearchDbRow[]): ResearchAggRow[] {
  const map = new Map<
    number,
    {
      name: string;
      count: number;
      sumNet: number;
      wins: number;
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
      count: 0,
      sumNet: 0,
      wins: 0,
      sumGross: 0,
      sumFees: 0,
      sumHoldMin: 0,
      holdMinCount: 0,
      lastAt: null,
    };

    cur.count += 1;
    cur.sumNet += r.net_pnl;
    if (r.net_pnl > 0) cur.wins += 1;
    if (typeof r.gross_pnl === "number" && Number.isFinite(r.gross_pnl)) cur.sumGross += Math.abs(r.gross_pnl);
    if (typeof r.fees === "number" && Number.isFinite(r.fees)) cur.sumFees += r.fees;
    if (r.strategy_name?.trim()) cur.name = r.strategy_name.trim();

    // Hold minutes from opened_at + closed_at
    if (r.opened_at && r.closed_at) {
      const openMs = Date.parse(r.opened_at);
      const closeMs = Date.parse(r.closed_at);
      if (Number.isFinite(openMs) && Number.isFinite(closeMs) && closeMs > openMs) {
        cur.sumHoldMin += (closeMs - openMs) / 60_000;
        cur.holdMinCount += 1;
      }
    }

    // Latest closed trade
    if (r.closed_at) {
      if (!cur.lastAt || r.closed_at > cur.lastAt) cur.lastAt = r.closed_at;
    }

    map.set(r.strategy_id, cur);
  }

  return [...map.entries()]
    .map(([strategyId, s]) => ({
      strategyId,
      strategyName: s.name,
      tradeCount: s.count,
      sumNet: Math.round(s.sumNet * 100) / 100,
      expectancy: s.count > 0 ? Math.round((s.sumNet / s.count) * 1000) / 1000 : 0,
      winRate: s.count > 0 ? Math.round((s.wins / s.count) * 1000) / 1000 : 0,
      feePctOfGross: s.sumGross > 0 ? Math.round((s.sumFees / s.sumGross) * 10000) / 100 : null,
      avgHoldMin: s.holdMinCount > 0 ? Math.round((s.sumHoldMin / s.holdMinCount) * 10) / 10 : null,
      lastTradeAt: s.lastAt,
    }))
    .sort((a, b) => b.sumNet - a.sumNet);
}

