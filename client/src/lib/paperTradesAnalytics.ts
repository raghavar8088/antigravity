/**
 * Rolling strategy expectancy analytics for paper-trade kill-switch (Supabase `paper_trades`).
 */

export type StratTradeRow = {
  strategyId: number;
  netPnl: number;
  closedAt?: string;
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
