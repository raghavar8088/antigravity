/**
 * Per-category parameter registry for the 160-strategy research pool.
 *
 * These values govern: bar interval, poll cadence, leverage caps, hold windows,
 * SL/TP bands, signal threshold delta, confluence defaults, and position limits.
 *
 * IMPORTANT — leverage caps:
 *   - scalping / day / trend / range / breakout / momentum: ≤25×
 *   - swing_trading: ≤10× (never 25×)
 *   - position_trading: ≤5× (never 25×)
 *
 * signalThresholdDelta is ADDED to the desk base threshold. Positive = tighter gate.
 */

import type { TradingCategoryId } from "@/lib/futuresStratTypes";

export type CategoryProfile = {
  label: string;
  primaryBarInterval: "1m" | "5m" | "15m" | "4h" | "1d";
  pollIntervalMs: number;
  /** Warm-up bars needed before indicators are valid. */
  minBars: number;
  defaultLeverage: number;
  maxLeverage: number;
  holdMinutesMin: number;
  holdMinutesMax: number;
  slPctMin: number;
  slPctMax: number;
  tpSlRatioMin: number;
  /** Added to desk base signal threshold (positive = harder to enter). */
  signalThresholdDelta: number;
  confluenceMinDefault: number;
  maxOpenPositions: number;
  maxNewEntriesPerPoll: number;
  requiresEodFlat?: boolean;
};

export const CATEGORY_REGISTRY: Readonly<Record<TradingCategoryId, CategoryProfile>> = {
  scalping: {
    label: "Scalping",
    primaryBarInterval: "1m",
    pollIntervalMs: 4_000,
    minBars: 18,
    defaultLeverage: 25,
    maxLeverage: 25,
    holdMinutesMin: 5,
    holdMinutesMax: 45,
    slPctMin: 0.35,
    slPctMax: 0.65,
    tpSlRatioMin: 2.5,
    signalThresholdDelta: 0,
    confluenceMinDefault: 5,
    maxOpenPositions: 8,
    maxNewEntriesPerPoll: 2,
  },
  day_trading: {
    label: "Day Trading",
    primaryBarInterval: "5m",
    pollIntervalMs: 15_000,
    minBars: 24,
    defaultLeverage: 15,
    maxLeverage: 20,
    holdMinutesMin: 30,
    holdMinutesMax: 480,
    slPctMin: 0.50,
    slPctMax: 1.00,
    tpSlRatioMin: 2.0,
    signalThresholdDelta: 2,
    confluenceMinDefault: 5,
    maxOpenPositions: 6,
    maxNewEntriesPerPoll: 1,
    requiresEodFlat: true,
  },
  swing_trading: {
    label: "Swing Trading",
    primaryBarInterval: "4h",
    pollIntervalMs: 60_000,
    minBars: 20,
    defaultLeverage: 8,
    maxLeverage: 10,
    holdMinutesMin: 1_440,   // 1 day
    holdMinutesMax: 20_160,  // 14 days
    slPctMin: 1.0,
    slPctMax: 2.5,
    tpSlRatioMin: 2.0,
    signalThresholdDelta: 3,
    confluenceMinDefault: 6,
    maxOpenPositions: 4,
    maxNewEntriesPerPoll: 1,
  },
  position_trading: {
    label: "Position Trading",
    primaryBarInterval: "1d",
    pollIntervalMs: 300_000,
    minBars: 30,
    defaultLeverage: 3,
    maxLeverage: 5,
    holdMinutesMin: 10_080,  // 1 week
    holdMinutesMax: 86_400,  // 60 days
    slPctMin: 2.0,
    slPctMax: 5.0,
    tpSlRatioMin: 1.8,
    signalThresholdDelta: 4,
    confluenceMinDefault: 6,
    maxOpenPositions: 2,
    maxNewEntriesPerPoll: 1,
  },
  trend_trading: {
    label: "Trend Trading",
    primaryBarInterval: "15m",
    pollIntervalMs: 15_000,
    minBars: 24,
    defaultLeverage: 15,
    maxLeverage: 20,
    holdMinutesMin: 60,
    holdMinutesMax: 2_880,   // 2 days
    slPctMin: 0.60,
    slPctMax: 2.0,
    tpSlRatioMin: 2.0,
    signalThresholdDelta: 1,
    confluenceMinDefault: 5,
    maxOpenPositions: 6,
    maxNewEntriesPerPoll: 1,
  },
  range_trading: {
    label: "Range Trading",
    primaryBarInterval: "15m",
    pollIntervalMs: 15_000,
    minBars: 24,
    defaultLeverage: 12,
    maxLeverage: 15,
    holdMinutesMin: 30,
    holdMinutesMax: 1_440,   // 1 day
    slPctMin: 0.45,
    slPctMax: 1.2,
    tpSlRatioMin: 2.0,
    signalThresholdDelta: 2,
    confluenceMinDefault: 6,
    maxOpenPositions: 5,
    maxNewEntriesPerPoll: 1,
  },
  breakout_trading: {
    label: "Breakout Trading",
    primaryBarInterval: "5m",
    pollIntervalMs: 15_000,
    minBars: 24,
    defaultLeverage: 15,
    maxLeverage: 20,
    holdMinutesMin: 15,
    holdMinutesMax: 480,
    slPctMin: 0.50,
    slPctMax: 1.5,
    tpSlRatioMin: 2.5,
    signalThresholdDelta: 1,
    confluenceMinDefault: 5,
    maxOpenPositions: 6,
    maxNewEntriesPerPoll: 1,
  },
  momentum_trading: {
    label: "Momentum Trading",
    primaryBarInterval: "5m",
    pollIntervalMs: 15_000,
    minBars: 24,
    defaultLeverage: 15,
    maxLeverage: 20,
    holdMinutesMin: 15,
    holdMinutesMax: 720,
    slPctMin: 0.50,
    slPctMax: 1.8,
    tpSlRatioMin: 2.0,
    signalThresholdDelta: 1,
    confluenceMinDefault: 5,
    maxOpenPositions: 6,
    maxNewEntriesPerPoll: 1,
  },
};

export function getCategoryProfile(id: TradingCategoryId): CategoryProfile {
  return CATEGORY_REGISTRY[id];
}

export const TRADING_CATEGORY_IDS: readonly TradingCategoryId[] = [
  "scalping",
  "day_trading",
  "swing_trading",
  "position_trading",
  "trend_trading",
  "range_trading",
  "breakout_trading",
  "momentum_trading",
];

/** ID range for each category block — used for collision detection. */
export const CATEGORY_ID_RANGES: Readonly<Record<TradingCategoryId, [number, number]>> = {
  scalping:          [600, 619],
  day_trading:       [620, 639],
  swing_trading:     [640, 659],
  position_trading:  [660, 679],
  trend_trading:     [680, 699],
  range_trading:     [700, 719],
  breakout_trading:  [720, 739],
  momentum_trading:  [740, 759],
};

/** Returns true if `id` falls within the allocated block for `category`. */
export function idInCategoryBlock(id: number, category: TradingCategoryId): boolean {
  const [lo, hi] = CATEGORY_ID_RANGES[category];
  return id >= lo && id <= hi;
}
