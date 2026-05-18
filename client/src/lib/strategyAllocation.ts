/**
 * Pure-math helpers for per-strategy capital allocation and adaptive TP.
 *
 * Used by `useBTCFuturesScalperEngine` to scale position notional based on
 * each strategy's observed edge (Kelly fraction + Sharpe vs cohort), and to
 * widen/tighten TP based on current volatility regime.
 *
 * **Honesty invariants** (must NOT be violated in callers):
 *   - Insufficient sample → multiplier 1.0 (no fake amplification before data exists)
 *   - Multiplier clamped to a defensive range so a single hot streak can't
 *     push notional 10× and blow up on the next regime change
 *   - Adaptive TP never crosses below entry on the wrong side; clamped to base × [0.8, 1.4]
 */

/** Minimum trades a strategy needs before allocation adjustments engage. */
export const ALLOCATION_MIN_TRADES = 20;

/** Hard clamp on the allocation multiplier returned to the sizing layer. */
export const ALLOCATION_MIN_MULTIPLIER = 0.25;
export const ALLOCATION_MAX_MULTIPLIER = 3.0;

/** Baseline Kelly fraction we expect from a strategy with marginal edge. */
const KELLY_BASELINE = 0.025;
/** Half-Kelly factor: industry-standard defensive scale-down on raw Kelly. */
const KELLY_DEFENSIVE_SHARE = 0.5;
/** Cap a single strategy's raw Kelly at this — beyond is over-fit noise. */
const KELLY_HARD_CAP = 0.25;

export type StrategyTradeSample = {
  /** Net PnL in USD (post-fees, post-funding). */
  netPnl: number;
  /** Notional sized for this trade — used to normalize returns across strategies. */
  notional: number;
  /** Hold time in minutes — needed for Sharpe annualization. */
  holdMinutes: number;
};

export type StrategyEdgeStats = {
  tradeCount: number;
  winRate: number; // 0..1
  avgWinPct: number; // average winning trade as % of notional
  avgLossPct: number; // average losing trade as % of notional (positive number)
  expectancyPct: number; // avg net PnL as % of notional, all trades
  /**
   * Annualized Sharpe ratio based on per-trade return %. Uses 365 days/year and the
   * average hold time to scale. Returns 0 when sample size is insufficient or stddev=0.
   */
  sharpe: number;
  /** Raw (full) Kelly fraction = (winRate × avgWin - lossRate × avgLoss) / avgWin. */
  rawKelly: number;
  /** Half-Kelly fraction, clamped to [0, KELLY_HARD_CAP]. */
  defensiveKelly: number;
};

/** Compute edge stats from a sample of closed trades. Pure, no env. */
export function computeStrategyEdgeStats(trades: ReadonlyArray<StrategyTradeSample>): StrategyEdgeStats {
  const valid = trades.filter(
    (t) => Number.isFinite(t.netPnl) && Number.isFinite(t.notional) && t.notional > 0,
  );
  const n = valid.length;
  if (n === 0) {
    return {
      tradeCount: 0,
      winRate: 0,
      avgWinPct: 0,
      avgLossPct: 0,
      expectancyPct: 0,
      sharpe: 0,
      rawKelly: 0,
      defensiveKelly: 0,
    };
  }

  // Per-trade return % of notional (scale-invariant across strategies with different sizing).
  const returns = valid.map((t) => (t.netPnl / t.notional) * 100);
  const wins = returns.filter((r) => r > 0);
  const losses = returns.filter((r) => r < 0);
  const winRate = wins.length / n;
  const avgWinPct = wins.length > 0 ? wins.reduce((s, r) => s + r, 0) / wins.length : 0;
  const avgLossPct = losses.length > 0 ? Math.abs(losses.reduce((s, r) => s + r, 0) / losses.length) : 0;
  const expectancyPct = returns.reduce((s, r) => s + r, 0) / n;

  // Sharpe: stddev of per-trade returns, annualized by trades-per-year estimate.
  const mean = expectancyPct;
  const variance = returns.reduce((s, r) => s + (r - mean) ** 2, 0) / Math.max(1, n - 1);
  const stddev = Math.sqrt(variance);
  let sharpe = 0;
  if (stddev > 1e-9) {
    // Estimate trades per year from observed cadence: avg hold time → trades per year.
    const avgHoldMin = valid.reduce((s, t) => s + Math.max(0, t.holdMinutes), 0) / n;
    const minutesPerYear = 365 * 24 * 60;
    // Conservative: assume ~30% trade density vs continuous (most strats sit idle).
    const tradesPerYearEst = avgHoldMin > 0 ? (minutesPerYear / avgHoldMin) * 0.3 : 0;
    sharpe = (mean / stddev) * Math.sqrt(Math.max(0, tradesPerYearEst));
  }

  // Kelly: f* = (winRate × W - lossRate × L) / W where W = avgWin, L = avgLoss.
  // When avgWinPct is 0 the strategy has no winning trades — Kelly = 0.
  let rawKelly = 0;
  if (avgWinPct > 1e-9 && avgLossPct > 1e-9) {
    rawKelly = (winRate * avgWinPct - (1 - winRate) * avgLossPct) / avgWinPct;
  } else if (avgWinPct > 1e-9 && avgLossPct <= 1e-9) {
    rawKelly = winRate; // all wins (rare, suspicious — cap below)
  }
  const defensiveKelly = Math.max(0, Math.min(KELLY_HARD_CAP, rawKelly * KELLY_DEFENSIVE_SHARE));

  return {
    tradeCount: n,
    winRate,
    avgWinPct,
    avgLossPct,
    expectancyPct,
    sharpe,
    rawKelly,
    defensiveKelly,
  };
}

export type AllocationMultiplierInput = {
  stats: StrategyEdgeStats;
  /** Sharpe values of all peers in the same cohort (active roster). */
  cohortSharpes: ReadonlyArray<number>;
};

/**
 * Combine Kelly + Sharpe-relative scaling into one multiplier in
 * [ALLOCATION_MIN_MULTIPLIER, ALLOCATION_MAX_MULTIPLIER].
 *
 * - Under-sampled strategy (< {@link ALLOCATION_MIN_TRADES}) → 1.0 (no adjustment)
 * - kellyFactor = clamp(defensiveKelly / KELLY_BASELINE, 0.25, 2.0)
 * - sharpeRelativeFactor = 1.5 when sharpe > cohortMedian × 1.5,
 *                          0.5 when sharpe < cohortMedian × 0.5,
 *                          1.0 otherwise (linear interp would over-react to noise)
 * - multiplier = kellyFactor × sharpeRelativeFactor (then clamped)
 */
export function computeAllocationMultiplier(input: AllocationMultiplierInput): number {
  const { stats, cohortSharpes } = input;
  if (stats.tradeCount < ALLOCATION_MIN_TRADES) return 1.0;

  const kellyFactor = Math.max(
    0.25,
    Math.min(2.0, stats.defensiveKelly / KELLY_BASELINE),
  );

  let sharpeRelativeFactor = 1.0;
  const peers = cohortSharpes.filter((s) => Number.isFinite(s));
  if (peers.length >= 3) {
    const sorted = [...peers].sort((a, b) => a - b);
    const median = sorted[Math.floor(sorted.length / 2)] ?? 0;
    if (median > 0) {
      if (stats.sharpe > median * 1.5) sharpeRelativeFactor = 1.5;
      else if (stats.sharpe < median * 0.5) sharpeRelativeFactor = 0.5;
    }
  }

  const raw = kellyFactor * sharpeRelativeFactor;
  return Math.max(ALLOCATION_MIN_MULTIPLIER, Math.min(ALLOCATION_MAX_MULTIPLIER, raw));
}

// ============================================================================
// Adaptive TP (#3)
// ============================================================================

/**
 * Scale TP% based on volatility regime. High vol → wider TP to let winners run;
 * low vol → tighter TP because price won't reach a wide target before reversing.
 *
 * Inputs:
 *   - baseTpPct: the strategy's nominal TP % (e.g. 0.6 for 0.6%)
 *   - atrPct: ATR14 as % of price (e.g. 0.4 for 0.4%)
 *
 * Output: adjusted TP % clamped to [baseTpPct × 0.8, baseTpPct × 1.4]
 */
export function computeAdaptiveTpPct(baseTpPct: number, atrPct: number): number {
  if (!Number.isFinite(baseTpPct) || baseTpPct <= 0) return baseTpPct;
  if (!Number.isFinite(atrPct) || atrPct <= 0) return baseTpPct;

  // Reference: typical BTC 1m ATR ~ 0.10-0.30% of price.
  // Multiplier curve (continuous, monotone):
  //   atrPct 0.10 → 0.8x   (low vol, tighten)
  //   atrPct 0.25 → 1.0x   (normal, no change)
  //   atrPct 0.50 → 1.2x   (elevated, widen)
  //   atrPct 0.80+ → 1.4x  (high vol, max widen)
  let mul: number;
  if (atrPct <= 0.10) mul = 0.8;
  else if (atrPct <= 0.25) mul = 0.8 + ((atrPct - 0.10) / 0.15) * 0.2; // 0.8 → 1.0
  else if (atrPct <= 0.50) mul = 1.0 + ((atrPct - 0.25) / 0.25) * 0.2; // 1.0 → 1.2
  else if (atrPct <= 0.80) mul = 1.2 + ((atrPct - 0.50) / 0.30) * 0.2; // 1.2 → 1.4
  else mul = 1.4;

  return baseTpPct * mul;
}

/** Convert raw ATR (in price units) to atrPct (% of price). Safe for atrPct callers. */
export function atrPctFromAtr(atr14: number, price: number): number {
  if (!Number.isFinite(atr14) || atr14 <= 0) return 0;
  if (!Number.isFinite(price) || price <= 0) return 0;
  return (atr14 / price) * 100;
}
