/**
 * Per-strategy hourly statistics for time-of-day session gating (#4).
 *
 * A strategy is "in its proven session" at UTC hour `H` when, over a rolling
 * window of recent trades:
 *   - total sample is large enough to be statistically meaningful, AND
 *   - the strategy's win rate at hour `H` is at least {@link MIN_HOURLY_WIN_RATE}, AND
 *   - the strategy's expectancy at hour `H` is non-negative
 *
 * Below the sample-size floor we DO NOT filter — premature filtering would
 * starve the strategy of data and lock it into whatever its first few trades
 * happened to be.
 *
 * Honesty invariants:
 *   - Insufficient sample → allow trade (no fake filtering)
 *   - Stats are computed from `closedAt` timestamps; no look-ahead bias
 */

export const SESSION_MIN_TOTAL_TRADES = 50;
export const SESSION_MIN_HOUR_TRADES = 5;
export const MIN_HOURLY_WIN_RATE = 0.35; // sub-35% in a window → block that hour

export type HourlyBucket = {
  hour: number; // 0..23
  trades: number;
  wins: number;
  winRate: number; // 0..1
  netSum: number; // sum of net PnL (USD)
  expectancy: number; // netSum / trades
};

export type StrategySessionStats = {
  strategyId: number;
  totalTrades: number;
  byHour: HourlyBucket[]; // length 24, index = UTC hour
};

export type SessionTradeSample = {
  strategyId: number;
  netPnl: number;
  closedAtMs: number;
};

/** Build hourly buckets for a single strategy from its closed trades. */
export function computeStrategyHourlyStats(
  strategyId: number,
  trades: ReadonlyArray<SessionTradeSample>,
): StrategySessionStats {
  const byHour: HourlyBucket[] = Array.from({ length: 24 }, (_, h) => ({
    hour: h,
    trades: 0,
    wins: 0,
    winRate: 0,
    netSum: 0,
    expectancy: 0,
  }));

  let totalTrades = 0;
  for (const t of trades) {
    if (t.strategyId !== strategyId) continue;
    if (!Number.isFinite(t.closedAtMs) || !Number.isFinite(t.netPnl)) continue;
    const hour = new Date(t.closedAtMs).getUTCHours();
    if (hour < 0 || hour > 23) continue;
    const b = byHour[hour]!;
    b.trades += 1;
    b.netSum += t.netPnl;
    if (t.netPnl > 0) b.wins += 1;
    totalTrades += 1;
  }

  for (const b of byHour) {
    b.winRate = b.trades > 0 ? b.wins / b.trades : 0;
    b.expectancy = b.trades > 0 ? b.netSum / b.trades : 0;
  }

  return { strategyId, totalTrades, byHour };
}

export type SessionGateOpts = {
  /** Minimum total trades across all hours before the gate engages. Default 50. */
  minTotalTrades?: number;
  /** Minimum trades inside the queried hour before that hour is judged. Default 5. */
  minHourTrades?: number;
  /** Below this winRate, the hour is considered "outside proven session". Default 0.35. */
  minWinRate?: number;
};

/**
 * Return `true` when the strategy is allowed to trade at this UTC hour, `false`
 * when it should be blocked by the session gate. Defaults err toward ALLOW
 * unless we have enough data to confidently say "this hour loses money".
 */
export function isStrategyInProvenSession(
  stats: StrategySessionStats,
  utcHour: number,
  opts: SessionGateOpts = {},
): boolean {
  const minTotal = opts.minTotalTrades ?? SESSION_MIN_TOTAL_TRADES;
  const minHour = opts.minHourTrades ?? SESSION_MIN_HOUR_TRADES;
  const minWr = opts.minWinRate ?? MIN_HOURLY_WIN_RATE;

  if (stats.totalTrades < minTotal) return true; // not enough data overall → allow
  const h = Math.floor(utcHour);
  if (h < 0 || h > 23) return true;
  const bucket = stats.byHour[h];
  if (!bucket || bucket.trades < minHour) return true; // sparse hour → allow

  // Both gates must fail for us to block: bad win rate AND bad expectancy.
  // A high-win-rate but negative-expectancy hour is rare; require both to err safe.
  const badWinRate = bucket.winRate < minWr;
  const badExpectancy = bucket.expectancy < 0;
  return !(badWinRate && badExpectancy);
}
