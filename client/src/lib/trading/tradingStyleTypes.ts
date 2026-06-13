/**
 * Trading style and playbook types shared across the multi-style futures desk.
 *
 * Time-horizon modules (scalp / day / swing / position) each own a distinct bar
 * interval, poll cadence, leverage band, and hold-time window. They are separate
 * desks with isolated paper accounts and Mongo namespaces.
 *
 * Playbooks (trend / range / breakout / momentum) are OVERLAYS — they filter the
 * active roster and tighten/loosen entry confirmation within a desk. They are not
 * separate desks.
 */

export type { TradingStyleId, PlaybookId } from "@/lib/trading/futuresStratTypes";

/** Full profile for a time-horizon desk module. */
export type TradingStyleProfile = {
  id: import("@/lib/trading/futuresStratTypes").TradingStyleId;
  label: string;
  /** OHLCV bar granularity fetched from the klines API. */
  barInterval: "1m" | "5m" | "15m" | "4h" | "1d";
  /** Milliseconds between poll ticks. */
  pollIntervalMs: number;
  defaultLeverage: number;
  maxLeverage: number;
  /** Minimum wall-clock hold in minutes before TIME exit may fire. */
  holdMinutesMin: number;
  /** Maximum wall-clock hold before TIME exit always fires. */
  holdMinutesMax: number;
  slPctMin: number;
  slPctMax: number;
  /** Minimum TP/SL ratio enforced after desk widen. */
  tpSlRatioMin: number;
  maxOpenPositions: number;
  maxNewEntriesPerPoll: number;
  signalThresholdDefault: number;
  confluenceMinDefault: number;
  /** Close all open positions at `EOD_UTC_HOUR` when true. Day desk only. */
  requiresEodFlat: boolean;
};

/** Profile for a playbook overlay (trend / range / breakout / momentum). */
export type PlaybookProfile = {
  id: import("@/lib/trading/futuresStratTypes").PlaybookId;
  label: string;
  /** Strategy categories eligible when this playbook is active. */
  categoryAllowlist: readonly string[];
  /** Added to (or subtracted from) the desk's base signal threshold. */
  signalDelta: number;
  /** Multiplier applied to strat `holdMinutes` (after style clamp). */
  holdMul: number;
  /** Multiplier applied to strat `cooldownMin`. */
  cooldownMul: number;
  description: string;
};
