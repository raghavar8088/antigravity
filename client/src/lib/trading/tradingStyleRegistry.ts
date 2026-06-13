/**
 * Registry of the four time-horizon trading-style profiles.
 *
 * Values chosen to match the multi-style desk spec:
 *   scalp    — 1m bars, 4s poll, 25x, hold 5–45 min
 *   day      — 5m bars, 15s poll, 15x, hold 30 min–8h, EOD flat
 *   swing    — 4h bars, 60s poll, 7x, hold 1–14 days
 *   position — 1d bars, 5m poll, 3x, hold 1 week–90 days
 */

import type { TradingStyleId } from "@/lib/trading/futuresStratTypes";
import type { TradingStyleProfile } from "@/lib/trading/tradingStyleTypes";

export const TRADING_STYLE_PROFILES: Readonly<Record<TradingStyleId, TradingStyleProfile>> = {
  scalp: {
    id: "scalp",
    label: "Scalp Desk",
    barInterval: "1m",
    pollIntervalMs: 4_000,
    defaultLeverage: 25,
    maxLeverage: 25,
    holdMinutesMin: 5,
    holdMinutesMax: 45,
    slPctMin: 0.3,
    slPctMax: 0.7,
    tpSlRatioMin: 2.0,
    maxOpenPositions: 8,
    maxNewEntriesPerPoll: 2,
    signalThresholdDefault: 26,
    confluenceMinDefault: 5,
    requiresEodFlat: false,
  },
  day: {
    id: "day",
    label: "Day Desk",
    barInterval: "5m",
    pollIntervalMs: 15_000,
    defaultLeverage: 15,
    maxLeverage: 20,
    holdMinutesMin: 30,
    holdMinutesMax: 480,
    slPctMin: 0.5,
    slPctMax: 1.5,
    tpSlRatioMin: 1.8,
    maxOpenPositions: 6,
    maxNewEntriesPerPoll: 2,
    signalThresholdDefault: 24,
    confluenceMinDefault: 5,
    requiresEodFlat: true,
  },
  swing: {
    id: "swing",
    label: "Swing Desk",
    barInterval: "4h",
    pollIntervalMs: 60_000,
    defaultLeverage: 7,
    maxLeverage: 10,
    holdMinutesMin: 1_440,   // 1 day
    holdMinutesMax: 20_160,  // 14 days
    slPctMin: 1.0,
    slPctMax: 4.0,
    tpSlRatioMin: 2.0,
    maxOpenPositions: 4,
    maxNewEntriesPerPoll: 1,
    signalThresholdDefault: 22,
    confluenceMinDefault: 5,
    requiresEodFlat: false,
  },
  position: {
    id: "position",
    label: "Position Desk",
    barInterval: "1d",
    pollIntervalMs: 300_000,  // 5 min
    defaultLeverage: 3,
    maxLeverage: 5,
    holdMinutesMin: 10_080,  // 1 week
    holdMinutesMax: 129_600, // 90 days
    slPctMin: 2.0,
    slPctMax: 8.0,
    tpSlRatioMin: 2.5,
    maxOpenPositions: 2,
    maxNewEntriesPerPoll: 1,
    signalThresholdDefault: 20,
    confluenceMinDefault: 6,
    requiresEodFlat: false,
  },
};

export function getTradingStyleProfile(id: TradingStyleId): TradingStyleProfile {
  return TRADING_STYLE_PROFILES[id];
}

/** All four style IDs in display order. */
export const TRADING_STYLE_IDS: readonly TradingStyleId[] = [
  "scalp",
  "day",
  "swing",
  "position",
];

/**
 * Clamp a hold-minutes value to the style's min/max window.
 * Used by the engine when building effective hold time per style.
 */
export function clampHoldMinutesToStyle(
  holdMinutes: number,
  profile: Pick<TradingStyleProfile, "holdMinutesMin" | "holdMinutesMax">,
): number {
  if (!Number.isFinite(holdMinutes) || holdMinutes <= 0) return profile.holdMinutesMin;
  return Math.min(profile.holdMinutesMax, Math.max(profile.holdMinutesMin, holdMinutes));
}
