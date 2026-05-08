"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { GoldMarketItem } from "@/app/api/forex/gold/markets/route";

const GOLD_SYMBOL = "GOLD";
const GOLD_PROXY_SYMBOL = "GC=F";
const GOLD_DISPLAY_NAME = "Gold / USD";
const INITIAL_BALANCE = 1_000_000;
const RISK_PER_TRADE_PCT = 0.01;
const ALLOCATION_USD = INITIAL_BALANCE * RISK_PER_TRADE_PCT;
const MAX_OPEN_POSITIONS = 20;
const MAX_BARS = 240;
const MIN_BARS_FAST = 24;
const MIN_BARS_SLOW = 40;
const SIGNAL_THRESHOLD = 64;
const POLL_MS = 5_000;
const MAX_TRADES = 20_000;
const PROFIT_LOCK_PROGRESS = 0.30;
const PROFIT_LOCK_SHARE = 0.42;
const LATE_EXIT_PROGRESS = 0.60;
const LATE_EXIT_MIN_GAIN = 0.05;
const GRIND_EXIT_PROGRESS = 0.46;
const GRIND_EXIT_SHARE = 0.24;
const TRAIL_ACTIVATION_PCT = 0.30;
const TRAIL_GIVEBACK_SHARE = 0.34;
const LOSS_COOLDOWN_PENALTY = 0.30;
const UNDERPERFORMING_PAUSE_MS = 120 * 60 * 1000;
/** Stable key — NEVER change. Old versioned keys are migrated on load. */
const LOCAL_STORAGE_KEY = "forex_gold_state";
const LS_LEGACY_KEYS = ["forex_gold_state_v1"];

type Side = "LONG" | "SHORT";
type Status = "WARMING" | "READY" | "IN_POSITION" | "COOLING";
type Regime = "UNKNOWN" | "TRENDING_BULL" | "TRENDING_BEAR" | "VOLATILE" | "RANGE" | "BREAKOUT";
type RosterState = "ACTIVE" | "WATCHLIST";

type SignalInputs = {
  price: number;
  prevPrice: number;
  fast: number;
  slow: number;
  trend: number;
  prevFast: number;
  prevSlow: number;
  mean20: number;
  mean50: number;
  weighted20: number;
  std20: number;
  std50: number;
  rsi14: number;
  high20: number;
  low20: number;
  high55: number;
  low55: number;
  breakoutHigh20: number;
  breakoutLow20: number;
  momentum3: number;
  momentum6: number;
  momentum12: number;
  macd: number;
  prevMacd: number;
  macdSignal: number;
  prevMacdSignal: number;
  stochK: number;
  prevStochK: number;
  stochD: number;
  atr14: number;
  atrPct: number;
  zScore20: number;
  trendStrength: number;
  rangeCompression: number;
};

interface StratDef {
  id: number;
  name: string;
  category: string;
  side: Side;
  signal: string;
  tpPct: number;
  slPct: number;
  cooldownMinutes: number;
  minBars: number;
  holdMinutes: number;
}

interface InternalPosition {
  id: string;
  strategyId: number;
  strategyName: string;
  side: Side;
  entryPrice: number;
  currentPrice: number;
  tpPrice: number;
  slPrice: number;
  quantity: number;
  notional: number;
  entryTime: number;
  unrealizedPnl: number;
  returnPct: number;
  peakReturnPct: number;
}

interface InternalTrade {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: Side;
  quantity: number;
  entryPrice: number;
  exitPrice: number;
  netPnl: number;
  returnPct: number;
  entryTime: number;
  exitTime: number;
  exitReason: string;
  holdSeconds: number;
}

interface InternalStrategyState {
  def: StratDef;
  position: InternalPosition | null;
  status: Status;
  cooldownUntil: number;
  score: number;
  regime: Regime;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  consecutiveLosses: number;
}

interface EngineRef {
  bars: number[];
  quote: GoldMarketItem | null;
  strategies: InternalStrategyState[];
  positions: Map<string, InternalPosition>;
  trades: InternalTrade[];
  balance: number;
  seq: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
  lastError: string;
  lastFeedAt: number;
}

export type GoldQuoteDisplay = {
  symbol: string;
  displayName: string;
  proxySymbol: string;
  ltp: number;
  changePct: number;
  dayHigh: number;
  dayLow: number;
  signalScore: number;
  regime: Regime;
  sparkline: number[];
  live: boolean;
  interval?: "1m" | "5m";
  source?: string;
};

export type GoldPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: Side;
  quantity: number;
  entryPrice: number;
  currentPrice: number;
  tpPrice: number;
  slPrice: number;
  notional: number;
  entryTime: string;
  unrealizedPnl: number;
  returnPct: number;
};

export type GoldTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: Side;
  quantity: number;
  entryPrice: number;
  exitPrice: number;
  netPnl: number;
  returnPct: number;
  entryTime: string;
  exitTime: string;
  exitReason: string;
  holdSeconds: number;
};

export type GoldStrategyStatus = {
  id: number;
  name: string;
  category: string;
  side: Side;
  status: Status;
  score: number;
  regime: Regime;
  rosterState: RosterState;
  allocationUSD: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  cooldownUntil?: string;
};

export type GoldEngineStats = {
  equity: number;
  balance: number;
  sessionPnl: number;
  unrealizedPnl: number;
  realizedPnl: number;
  totalTrades: number;
  totalWins: number;
  totalLosses: number;
  openPositions: number;
  winRate: number;
  activeStrategies: number;
  warmingUp: boolean;
  live: boolean;
  livePrice: number;
  dayHigh: number;
  dayLow: number;
  regime: Regime;
  lastUpdateAt: number;
  diagnostics: string;
};

type GoldDbPosition = {
  id: string;
  strategyId: number;
  side: Side;
  entryPrice: number;
  currentPrice: number;
  tpPrice: number;
  slPrice: number;
  quantity: number;
  notional: number;
  entryTime: number;
  unrealizedPnl: number;
  returnPct: number;
  peakReturnPct: number;
};

type GoldDbTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: Side;
  quantity: number;
  entryPrice: number;
  exitPrice: number;
  netPnl: number;
  returnPct: number;
  entryTime: number;
  exitTime: number;
  exitReason: string;
  holdSeconds: number;
};

type GoldDbStrategy = {
  id: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  cooldownUntil: number;
  consecutiveLosses: number;
  regime?: Regime;
};

type GoldDbPayload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  tradeSeq: number;
  positions: GoldDbPosition[];
  trades: GoldDbTrade[];
  strategies: GoldDbStrategy[];
};

type CategoryProfile = {
  minTp: number;
  maxTp: number;
  minSl: number;
  maxSl: number;
  holdMins: number;
};

const CATEGORY_PROFILES: Record<string, CategoryProfile> = {
  Trend: { minTp: 0.38, maxTp: 0.86, minSl: 0.20, maxSl: 0.40, holdMins: 160 },
  Momentum: { minTp: 0.42, maxTp: 0.92, minSl: 0.22, maxSl: 0.42, holdMins: 170 },
  Breakout: { minTp: 0.50, maxTp: 1.05, minSl: 0.24, maxSl: 0.46, holdMins: 210 },
  "Mean Reversion": { minTp: 0.24, maxTp: 0.58, minSl: 0.16, maxSl: 0.32, holdMins: 120 },
  Volatility: { minTp: 0.34, maxTp: 0.78, minSl: 0.20, maxSl: 0.36, holdMins: 145 },
};

const BASE_SIGNALS = [
  { signal: "EMA_TREND", category: "Trend", longName: "XAU_EMA_Trend", shortName: "XAU_EMA_Fade", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 18, minBars: MIN_BARS_SLOW },
  { signal: "DONCHIAN_BREAKOUT", category: "Breakout", longName: "XAU_Donchian_Breakout", shortName: "XAU_Donchian_Breakdown", tpPct: 0.76, slPct: 0.32, cooldownMinutes: 22, minBars: MIN_BARS_SLOW },
  { signal: "VWAP_RECLAIM", category: "Trend", longName: "XAU_VWAP_Reclaim", shortName: "XAU_VWAP_Reject", tpPct: 0.52, slPct: 0.24, cooldownMinutes: 16, minBars: MIN_BARS_FAST },
  { signal: "RSI_RECLAIM", category: "Mean Reversion", longName: "XAU_RSI_Reclaim", shortName: "XAU_RSI_Fade", tpPct: 0.40, slPct: 0.18, cooldownMinutes: 14, minBars: MIN_BARS_FAST },
  { signal: "BOLLINGER_FADE", category: "Mean Reversion", longName: "XAU_BB_Reclaim", shortName: "XAU_BB_Fade", tpPct: 0.44, slPct: 0.20, cooldownMinutes: 16, minBars: MIN_BARS_FAST },
  { signal: "MACD_MOMENTUM", category: "Momentum", longName: "XAU_MACD_Impulse", shortName: "XAU_MACD_Fade", tpPct: 0.56, slPct: 0.26, cooldownMinutes: 17, minBars: MIN_BARS_SLOW },
  { signal: "KELTNER_EXPANSION", category: "Volatility", longName: "XAU_Keltner_Expansion", shortName: "XAU_Keltner_Exhaustion", tpPct: 0.50, slPct: 0.24, cooldownMinutes: 18, minBars: MIN_BARS_SLOW },
  { signal: "ADX_PULLBACK", category: "Trend", longName: "XAU_ADX_Pullback", shortName: "XAU_ADX_Retreat", tpPct: 0.58, slPct: 0.24, cooldownMinutes: 18, minBars: MIN_BARS_SLOW },
  { signal: "STOCH_REVERSAL", category: "Mean Reversion", longName: "XAU_Stoch_Reclaim", shortName: "XAU_Stoch_Fade", tpPct: 0.38, slPct: 0.18, cooldownMinutes: 13, minBars: MIN_BARS_FAST },
  { signal: "ATR_BREAKOUT", category: "Breakout", longName: "XAU_ATR_Breakout", shortName: "XAU_ATR_Breakdown", tpPct: 0.72, slPct: 0.30, cooldownMinutes: 20, minBars: MIN_BARS_SLOW },
];

const VARIANTS = [
  { suffix: "Scalp", tpBump: -0.05, slBump: -0.02, cooldown: -2, holdScale: 0.78 },
  { suffix: "Pulse", tpBump: -0.02, slBump: -0.01, cooldown: -1, holdScale: 0.90 },
  { suffix: "Core", tpBump: 0.00, slBump: 0.00, cooldown: 0, holdScale: 1.00 },
  { suffix: "Flow", tpBump: 0.03, slBump: 0.01, cooldown: 1, holdScale: 1.12 },
  { suffix: "Pro", tpBump: 0.06, slBump: 0.02, cooldown: 2, holdScale: 1.22 },
];

function num(value: unknown, fallback = 0): number {
  const parsed = typeof value === "number" ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function nonNegativeInt(value: unknown, fallback = 0): number {
  const parsed = Math.trunc(num(value, fallback));
  return parsed >= 0 ? parsed : fallback;
}

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

function scoreClamp(value: number) {
  return clamp(value, 0, 99);
}

function profileFor(category: string): CategoryProfile {
  return CATEGORY_PROFILES[category] ?? {
    minTp: 0.30,
    maxTp: 0.80,
    minSl: 0.18,
    maxSl: 0.36,
    holdMins: 150,
  };
}

function tuneStrategy(def: StratDef): StratDef {
  const profile = profileFor(def.category);
  return {
    ...def,
    tpPct: clamp(def.tpPct, profile.minTp, profile.maxTp),
    slPct: clamp(def.slPct, profile.minSl, profile.maxSl),
    holdMinutes: Math.max(40, Math.round(profile.holdMins * (def.holdMinutes / profile.holdMins))),
  };
}

const STRAT_DEFS: StratDef[] = (() => {
  const defs: StratDef[] = [];
  let id = 1;
  for (const base of BASE_SIGNALS) {
    const profile = profileFor(base.category);
    for (const variant of VARIANTS) {
      defs.push(tuneStrategy({
        id: id++,
        name: `${base.longName}_${variant.suffix}_LONG`,
        category: base.category,
        side: "LONG",
        signal: base.signal,
        tpPct: base.tpPct + variant.tpBump,
        slPct: base.slPct + variant.slBump,
        cooldownMinutes: Math.max(6, base.cooldownMinutes + variant.cooldown),
        minBars: base.minBars,
        holdMinutes: Math.round(profile.holdMins * variant.holdScale),
      }));
      defs.push(tuneStrategy({
        id: id++,
        name: `${base.shortName}_${variant.suffix}_SHORT`,
        category: base.category,
        side: "SHORT",
        signal: `${base.signal}_SHORT`,
        tpPct: base.tpPct + variant.tpBump,
        slPct: base.slPct + variant.slBump,
        cooldownMinutes: Math.max(6, base.cooldownMinutes + variant.cooldown),
        minBars: base.minBars,
        holdMinutes: Math.round(profile.holdMins * variant.holdScale),
      }));
    }
  }
  return defs;
})();

function sma(values: number[], period: number): number {
  const sample = values.slice(-period);
  return sample.length ? sample.reduce((sum, value) => sum + value, 0) / sample.length : 0;
}

function ema(values: number[], period: number): number {
  if (!values.length) return 0;
  const k = 2 / (period + 1);
  let current = values[0];
  for (let i = 1; i < values.length; i++) current = values[i] * k + current * (1 - k);
  return current;
}

function emaSeries(values: number[], period: number): number[] {
  if (!values.length) return [];
  const k = 2 / (period + 1);
  const result: number[] = [];
  let current = values[0];
  result.push(current);
  for (let i = 1; i < values.length; i++) {
    current = values[i] * k + current * (1 - k);
    result.push(current);
  }
  return result;
}

function stdDev(values: number[], period: number): number {
  const sample = values.slice(-period);
  if (!sample.length) return 0;
  const average = sample.reduce((sum, value) => sum + value, 0) / sample.length;
  return Math.sqrt(sample.reduce((sum, value) => sum + (value - average) ** 2, 0) / sample.length);
}

function rsi(values: number[], period: number): number {
  if (values.length < 2) return 50;
  const start = Math.max(1, values.length - period);
  let gains = 0;
  let losses = 0;
  for (let i = start; i < values.length; i++) {
    const delta = values[i] - values[i - 1];
    if (delta > 0) gains += delta;
    else losses -= delta;
  }
  if (losses === 0) return gains === 0 ? 50 : 100;
  return 100 - 100 / (1 + gains / losses);
}

function weightedAverage(values: number[], period: number): number {
  const sample = values.slice(-period);
  if (!sample.length) return 0;
  let numerator = 0;
  let denominator = 0;
  for (let i = 0; i < sample.length; i++) {
    const weight = i + 1;
    numerator += sample[i] * weight;
    denominator += weight;
  }
  return denominator > 0 ? numerator / denominator : 0;
}

function averageTrueRange(values: number[], period: number): number {
  if (values.length < 2) return 0;
  const changes: number[] = [];
  for (let i = 1; i < values.length; i++) changes.push(Math.abs(values[i] - values[i - 1]));
  return sma(changes, Math.min(period, changes.length));
}

function stochasticK(values: number[], period: number): number {
  const sample = values.slice(-period);
  if (!sample.length) return 50;
  const low = Math.min(...sample);
  const high = Math.max(...sample);
  const last = sample[sample.length - 1];
  if (high === low) return 50;
  return ((last - low) / (high - low)) * 100;
}

function buildSignalInputs(bars: number[]): SignalInputs {
  const last = bars.length - 1;
  const price = bars[last];
  const prevBars = bars.slice(0, -1);
  const recent20 = bars.slice(-20);
  const recent55 = bars.slice(-55);
  const breakoutWindow20 = bars.slice(-21, -1);
  const breakoutWindow = breakoutWindow20.length > 0 ? breakoutWindow20 : recent20;
  const fastSeries = emaSeries(bars, 8);
  const slowSeries = emaSeries(bars, 21);
  const macdFastSeries = emaSeries(bars, 12);
  const macdSlowSeries = emaSeries(bars, 26);
  const macdSeries = macdFastSeries.map((value, index) => value - (macdSlowSeries[index] ?? value));
  const macdSignalSeries = emaSeries(macdSeries, 9);
  const prevPrice = last > 0 ? bars[last - 1] : price;
  const std20 = stdDev(bars, 20);
  const atr14 = averageTrueRange(bars, 14);
  const stochKCurrent = stochasticK(bars, 14);
  const stochKPrev = prevBars.length >= 14 ? stochasticK(prevBars, 14) : stochKCurrent;
  const stochD = sma([stochKCurrent, stochasticK(bars.slice(0, -1), 14), stochasticK(bars.slice(0, -2), 14)].filter((value) => Number.isFinite(value)), 3);
  const mean20 = sma(bars, 20);
  const mean50 = sma(bars, 50);
  const trendStrength = atr14 > 0 ? Math.abs((fastSeries[last] ?? 0) - (slowSeries[last] ?? 0)) / atr14 : 0;
  return {
    price,
    prevPrice,
    fast: fastSeries[last] ?? price,
    slow: slowSeries[last] ?? price,
    trend: ema(bars, 55),
    prevFast: fastSeries[last - 1] ?? price,
    prevSlow: slowSeries[last - 1] ?? price,
    mean20,
    mean50,
    weighted20: weightedAverage(bars, 20),
    std20,
    std50: stdDev(bars, 50),
    rsi14: rsi(bars, 14),
    high20: Math.max(...recent20),
    low20: Math.min(...recent20),
    high55: Math.max(...recent55),
    low55: Math.min(...recent55),
    breakoutHigh20: Math.max(...breakoutWindow),
    breakoutLow20: Math.min(...breakoutWindow),
    momentum3: last >= 3 ? ((price - bars[last - 3]) / bars[last - 3]) * 100 : 0,
    momentum6: last >= 6 ? ((price - bars[last - 6]) / bars[last - 6]) * 100 : 0,
    momentum12: last >= 12 ? ((price - bars[last - 12]) / bars[last - 12]) * 100 : 0,
    macd: macdSeries[last] ?? 0,
    prevMacd: macdSeries[last - 1] ?? 0,
    macdSignal: macdSignalSeries[last] ?? 0,
    prevMacdSignal: macdSignalSeries[last - 1] ?? 0,
    stochK: stochKCurrent,
    prevStochK: stochKPrev,
    stochD,
    atr14,
    atrPct: price > 0 ? (atr14 / price) * 100 : 0,
    zScore20: std20 > 0 ? (price - mean20) / std20 : 0,
    trendStrength,
    rangeCompression: stdDev(bars, 50) > 0 ? std20 / stdDev(bars, 50) : 1,
  };
}

function evalSignal(signal: string, input: SignalInputs): number {
  const bandUpper = input.mean20 + input.std20 * 1.5;
  const bandLower = input.mean20 - input.std20 * 1.5;
  const upperAtrBand = input.weighted20 + input.atr14 * 1.15;
  const lowerAtrBand = input.weighted20 - input.atr14 * 1.15;

  switch (signal) {
    case "EMA_TREND":
      return input.prevFast <= input.prevSlow && input.fast > input.slow && input.price > input.trend && input.rsi14 >= 52 && input.momentum6 > 0.04
        ? scoreClamp(71 + (input.fast / input.slow - 1) * 9000 + input.momentum6 * 10 + input.trendStrength * 4)
        : 0;
    case "EMA_TREND_SHORT":
      return input.prevFast >= input.prevSlow && input.fast < input.slow && input.price < input.trend && input.rsi14 <= 48 && input.momentum6 < -0.04
        ? scoreClamp(71 + (input.slow / input.fast - 1) * 9000 + Math.abs(input.momentum6) * 10 + input.trendStrength * 4)
        : 0;
    case "DONCHIAN_BREAKOUT":
      return input.price > input.breakoutHigh20 * 1.00045 && input.fast > input.slow && input.momentum3 > Math.max(0.04, input.atrPct * 0.55)
        ? scoreClamp(74 + (input.price / input.breakoutHigh20 - 1) * 7000 + input.momentum3 * 12)
        : 0;
    case "DONCHIAN_BREAKOUT_SHORT":
      return input.price < input.breakoutLow20 * 0.99955 && input.fast < input.slow && input.momentum3 < -Math.max(0.04, input.atrPct * 0.55)
        ? scoreClamp(74 + (input.breakoutLow20 / input.price - 1) * 7000 + Math.abs(input.momentum3) * 12)
        : 0;
    case "VWAP_RECLAIM":
      return input.price > input.weighted20 * 1.00035 && input.prevPrice <= input.weighted20 * 1.00025 && input.rsi14 >= 50
        ? scoreClamp(69 + (input.price / input.weighted20 - 1) * 6000 + Math.max(input.momentum3, 0) * 10)
        : 0;
    case "VWAP_RECLAIM_SHORT":
      return input.price < input.weighted20 * 0.99965 && input.prevPrice >= input.weighted20 * 0.99975 && input.rsi14 <= 50
        ? scoreClamp(69 + (input.weighted20 / input.price - 1) * 6000 + Math.max(Math.abs(input.momentum3), 0) * 10)
        : 0;
    case "RSI_RECLAIM":
      return input.rsi14 <= 34 && input.zScore20 <= -0.9 && input.price >= input.prevPrice
        ? scoreClamp(68 + (36 - input.rsi14) * 1.3 + Math.abs(input.zScore20) * 5)
        : 0;
    case "RSI_RECLAIM_SHORT":
      return input.rsi14 >= 66 && input.zScore20 >= 0.9 && input.price <= input.prevPrice
        ? scoreClamp(68 + (input.rsi14 - 64) * 1.3 + Math.abs(input.zScore20) * 5)
        : 0;
    case "BOLLINGER_FADE":
      return input.price < bandLower && input.rsi14 <= 38 && input.momentum3 > -0.24
        ? scoreClamp(67 + (bandLower - input.price) / Math.max(input.std20, 1) * 10 + (40 - input.rsi14) * 0.6)
        : 0;
    case "BOLLINGER_FADE_SHORT":
      return input.price > bandUpper && input.rsi14 >= 62 && input.momentum3 < 0.24
        ? scoreClamp(67 + (input.price - bandUpper) / Math.max(input.std20, 1) * 10 + (input.rsi14 - 60) * 0.6)
        : 0;
    case "MACD_MOMENTUM":
      return input.prevMacd <= input.prevMacdSignal && input.macd > input.macdSignal && input.price > input.mean20 && input.momentum6 > 0.03
        ? scoreClamp(70 + (input.macd - input.macdSignal) * 18 + input.momentum6 * 12)
        : 0;
    case "MACD_MOMENTUM_SHORT":
      return input.prevMacd >= input.prevMacdSignal && input.macd < input.macdSignal && input.price < input.mean20 && input.momentum6 < -0.03
        ? scoreClamp(70 + (input.macdSignal - input.macd) * 18 + Math.abs(input.momentum6) * 12)
        : 0;
    case "KELTNER_EXPANSION":
      return input.price > upperAtrBand && input.fast > input.slow && input.rangeCompression <= 1.08
        ? scoreClamp(69 + (input.price - upperAtrBand) / Math.max(input.atr14, 1) * 14 + input.trendStrength * 3)
        : 0;
    case "KELTNER_EXPANSION_SHORT":
      return input.price < lowerAtrBand && input.fast < input.slow && input.rangeCompression <= 1.08
        ? scoreClamp(69 + (lowerAtrBand - input.price) / Math.max(input.atr14, 1) * 14 + input.trendStrength * 3)
        : 0;
    case "ADX_PULLBACK":
      return input.fast > input.slow && input.slow > input.trend && input.trendStrength >= 0.45 && input.price <= input.fast * 1.0008 && input.rsi14 >= 44 && input.rsi14 <= 60 && input.momentum12 > 0.08
        ? scoreClamp(72 + input.trendStrength * 6 + input.momentum12 * 8)
        : 0;
    case "ADX_PULLBACK_SHORT":
      return input.fast < input.slow && input.slow < input.trend && input.trendStrength >= 0.45 && input.price >= input.fast * 0.9992 && input.rsi14 >= 40 && input.rsi14 <= 56 && input.momentum12 < -0.08
        ? scoreClamp(72 + input.trendStrength * 6 + Math.abs(input.momentum12) * 8)
        : 0;
    case "STOCH_REVERSAL":
      return input.prevStochK <= 20 && input.stochK > input.prevStochK && input.stochK > input.stochD && input.rsi14 <= 49
        ? scoreClamp(66 + (24 - input.prevStochK) * 0.9 + Math.max(0, input.stochK - input.stochD) * 0.4)
        : 0;
    case "STOCH_REVERSAL_SHORT":
      return input.prevStochK >= 80 && input.stochK < input.prevStochK && input.stochK < input.stochD && input.rsi14 >= 51
        ? scoreClamp(66 + (input.prevStochK - 76) * 0.9 + Math.max(0, input.stochD - input.stochK) * 0.4)
        : 0;
    case "ATR_BREAKOUT":
      return input.price > input.high55 * 1.00030 && input.atrPct >= 0.10 && input.fast > input.slow && input.momentum12 > 0.10
        ? scoreClamp(75 + input.momentum12 * 8 + input.atrPct * 18)
        : 0;
    case "ATR_BREAKOUT_SHORT":
      return input.price < input.low55 * 0.99970 && input.atrPct >= 0.10 && input.fast < input.slow && input.momentum12 < -0.10
        ? scoreClamp(75 + Math.abs(input.momentum12) * 8 + input.atrPct * 18)
        : 0;
    default:
      return 0;
  }
}

function classifyRegime(input: SignalInputs): Regime {
  if (input.price <= 0) return "UNKNOWN";
  if (input.fast > input.slow && input.slow > input.trend && input.momentum12 > 0.12) return "TRENDING_BULL";
  if (input.fast < input.slow && input.slow < input.trend && input.momentum12 < -0.12) return "TRENDING_BEAR";
  if (input.atrPct >= 0.18 || input.rangeCompression >= 1.30) return "VOLATILE";
  if (input.price > input.breakoutHigh20 * 0.9999 || input.price < input.breakoutLow20 * 1.0001) return "BREAKOUT";
  return "RANGE";
}

function passesEntryConfirmation(def: StratDef, input: SignalInputs, regime: Regime): boolean {
  if (def.side === "LONG") {
    if (def.category === "Mean Reversion") return input.price >= input.prevPrice * 0.9997 && input.rsi14 <= 58;
    if (def.category === "Breakout") return input.price >= input.fast * 0.9995 && input.momentum3 > -0.04;
    if (def.category === "Volatility") return input.atrPct >= 0.08 && input.price >= input.mean20 * 0.9995;
    if (regime === "TRENDING_BULL") return input.price >= input.trend * 0.9996 && input.momentum6 > -0.05;
    return input.price >= input.prevPrice * 0.9995;
  }
  if (def.category === "Mean Reversion") return input.price <= input.prevPrice * 1.0003 && input.rsi14 >= 42;
  if (def.category === "Breakout") return input.price <= input.fast * 1.0005 && input.momentum3 < 0.04;
  if (def.category === "Volatility") return input.atrPct >= 0.08 && input.price <= input.mean20 * 1.0005;
  if (regime === "TRENDING_BEAR") return input.price <= input.trend * 1.0004 && input.momentum6 < 0.05;
  return input.price <= input.prevPrice * 1.0005;
}

function calcPnl(side: Side, entry: number, current: number, quantity: number): number {
  return (current - entry) * quantity * (side === "LONG" ? 1 : -1);
}

function cooldownMsFor(strategy: InternalStrategyState, won: boolean): number {
  const base = strategy.def.cooldownMinutes * 60 * 1000;
  return won ? base : Math.round(base * (1 + strategy.consecutiveLosses * LOSS_COOLDOWN_PENALTY));
}

function resolveExit(position: InternalPosition, def: StratDef, price: number, now: number): { reason: string; exitPrice: number } | null {
  const returnPct = position.notional > 0 ? (calcPnl(position.side, position.entryPrice, price, position.quantity) / position.notional) * 100 : 0;
  const maxHoldMs = def.holdMinutes * 60 * 1000;
  const progress = maxHoldMs > 0 ? Math.min(1, (now - position.entryTime) / maxHoldMs) : 0;
  const lockThreshold = Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * PROFIT_LOCK_SHARE);

  if (position.side === "LONG") {
    if (price >= position.tpPrice) return { reason: "TP", exitPrice: position.tpPrice };
    if (price <= position.slPrice) return { reason: "SL", exitPrice: position.slPrice };
  } else {
    if (price <= position.tpPrice) return { reason: "TP", exitPrice: position.tpPrice };
    if (price >= position.slPrice) return { reason: "SL", exitPrice: position.slPrice };
  }

  if (progress >= GRIND_EXIT_PROGRESS && returnPct >= Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * GRIND_EXIT_SHARE)) {
    return { reason: "PROFIT_LOCK", exitPrice: price };
  }
  if (progress >= PROFIT_LOCK_PROGRESS && returnPct >= lockThreshold) {
    return { reason: "PROFIT_LOCK", exitPrice: price };
  }
  if (position.peakReturnPct >= Math.max(TRAIL_ACTIVATION_PCT, def.tpPct * 0.4) && returnPct > 0 && returnPct <= position.peakReturnPct * (1 - TRAIL_GIVEBACK_SHARE)) {
    return { reason: "TRAIL_STOP", exitPrice: price };
  }
  if (progress >= LATE_EXIT_PROGRESS && returnPct >= LATE_EXIT_MIN_GAIN) {
    return { reason: "LATE_EXIT", exitPrice: price };
  }
  if (maxHoldMs > 0 && now - position.entryTime >= maxHoldMs) {
    return { reason: "TIME_EXIT", exitPrice: price };
  }
  return null;
}

function initEngine(): EngineRef {
  return {
    bars: [],
    quote: null,
    strategies: STRAT_DEFS.map((def) => ({
      def,
      position: null,
      status: "WARMING",
      cooldownUntil: 0,
      score: 0,
      regime: "UNKNOWN",
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
      consecutiveLosses: 0,
    })),
    positions: new Map(),
    trades: [],
    balance: INITIAL_BALANCE,
    seq: 0,
    totalWins: 0,
    totalLosses: 0,
    totalRealizedPnl: 0,
    lastError: "",
    lastFeedAt: 0,
  };
}

function openPosition(engine: EngineRef, strategy: InternalStrategyState, entryPrice: number, now: number): boolean {
  const quantity = entryPrice > 0 ? ALLOCATION_USD / entryPrice : 0;
  const notional = quantity * entryPrice;
  if (quantity <= 0 || notional <= 0 || engine.balance < notional) return false;

  engine.seq++;
  engine.balance -= notional;
  const tpMul = 1 + strategy.def.tpPct / 100;
  const slMul = 1 - strategy.def.slPct / 100;
  const position: InternalPosition = {
    id: `XAU-${Date.now()}-${engine.seq}`,
    strategyId: strategy.def.id,
    strategyName: strategy.def.name,
    side: strategy.def.side,
    entryPrice,
    currentPrice: entryPrice,
    tpPrice: strategy.def.side === "LONG" ? entryPrice * tpMul : entryPrice * (2 - tpMul),
    slPrice: strategy.def.side === "LONG" ? entryPrice * slMul : entryPrice * (2 - slMul),
    quantity,
    notional,
    entryTime: now,
    unrealizedPnl: 0,
    returnPct: 0,
    peakReturnPct: 0,
  };
  strategy.position = position;
  strategy.status = "IN_POSITION";
  engine.positions.set(position.id, position);
  return true;
}

function closePosition(engine: EngineRef, strategy: InternalStrategyState, exitPrice: number, reason: string, now: number) {
  const position = strategy.position;
  if (!position) return;

  const netPnl = calcPnl(position.side, position.entryPrice, exitPrice, position.quantity);
  const trade: InternalTrade = {
    id: position.id,
    strategyId: position.strategyId,
    strategyName: position.strategyName,
    symbol: GOLD_SYMBOL,
    side: position.side,
    quantity: position.quantity,
    entryPrice: position.entryPrice,
    exitPrice,
    netPnl,
    returnPct: position.notional > 0 ? (netPnl / position.notional) * 100 : 0,
    entryTime: position.entryTime,
    exitTime: now,
    exitReason: reason,
    holdSeconds: Math.round((now - position.entryTime) / 1000),
  };

  engine.trades.unshift(trade);
  if (engine.trades.length > MAX_TRADES) engine.trades.length = MAX_TRADES;
  engine.balance += position.notional + netPnl;
  engine.totalRealizedPnl += netPnl;

  strategy.totalTrades++;
  if (netPnl >= 0) {
    strategy.wins++;
    engine.totalWins++;
    strategy.consecutiveLosses = 0;
  } else {
    strategy.losses++;
    engine.totalLosses++;
    strategy.consecutiveLosses++;
  }
  strategy.totalPnl += netPnl;
  strategy.winRate = strategy.totalTrades > 0 ? (strategy.wins / strategy.totalTrades) * 100 : 0;
  strategy.cooldownUntil = now + cooldownMsFor(strategy, netPnl >= 0);
  strategy.status = "COOLING";
  strategy.regime = "UNKNOWN";
  strategy.position = null;
  engine.positions.delete(position.id);
}

function rosterStateFor(status: Status): RosterState {
  return status === "WARMING" ? "WATCHLIST" : "ACTIVE";
}

const EMPTY_STATS: GoldEngineStats = {
  equity: INITIAL_BALANCE,
  balance: INITIAL_BALANCE,
  sessionPnl: 0,
  unrealizedPnl: 0,
  realizedPnl: 0,
  totalTrades: 0,
  totalWins: 0,
  totalLosses: 0,
  openPositions: 0,
  winRate: 0,
  activeStrategies: 0,
  warmingUp: true,
  live: false,
  livePrice: 0,
  dayHigh: 0,
  dayLow: 0,
  regime: "UNKNOWN",
  lastUpdateAt: 0,
  diagnostics: "Bootstrapping gold market feed.",
};

function normalizeEngineAccount(engine: EngineRef) {
  const openNotional = [...engine.positions.values()].reduce((sum, position) => sum + position.notional, 0);
  const unrealizedPnl = [...engine.positions.values()].reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const normalizedBalance = INITIAL_BALANCE + engine.totalRealizedPnl - openNotional;
  if (Number.isFinite(normalizedBalance) && Math.abs(engine.balance - normalizedBalance) > 0.01) {
    engine.balance = normalizedBalance;
  }
  return {
    openNotional,
    unrealizedPnl,
    balance: engine.balance,
    equity: engine.balance + openNotional + unrealizedPnl,
    sessionPnl: engine.totalRealizedPnl + unrealizedPnl,
  };
}

function buildPersistedPayload(engine: EngineRef): GoldDbPayload {
  normalizeEngineAccount(engine);
  return {
    balance: engine.balance,
    totalWins: engine.totalWins,
    totalLosses: engine.totalLosses,
    totalPnl: engine.totalRealizedPnl,
    tradeSeq: engine.seq,
    positions: [...engine.positions.values()].map((position) => ({
      id: position.id,
      strategyId: position.strategyId,
      side: position.side,
      entryPrice: position.entryPrice,
      currentPrice: position.currentPrice,
      tpPrice: position.tpPrice,
      slPrice: position.slPrice,
      quantity: position.quantity,
      notional: position.notional,
      entryTime: position.entryTime,
      unrealizedPnl: position.unrealizedPnl,
      returnPct: position.returnPct,
      peakReturnPct: position.peakReturnPct,
    })),
    trades: engine.trades.slice(0, MAX_TRADES),
    strategies: engine.strategies.map((strategy) => ({
      id: strategy.def.id,
      totalTrades: strategy.totalTrades,
      wins: strategy.wins,
      losses: strategy.losses,
      totalPnl: strategy.totalPnl,
      winRate: strategy.winRate,
      cooldownUntil: strategy.cooldownUntil,
      consecutiveLosses: strategy.consecutiveLosses,
      regime: strategy.regime,
    })),
  };
}

function buildSaveSignature(engine: EngineRef): string {
  const storedTradeCount = Math.min(engine.trades.length, MAX_TRADES);
  const oldestStoredTradeId = storedTradeCount > 0 ? engine.trades[storedTradeCount - 1]?.id ?? "" : "";
  return JSON.stringify({
    balance: engine.balance,
    totalWins: engine.totalWins,
    totalLosses: engine.totalLosses,
    totalPnl: engine.totalRealizedPnl,
    tradeSeq: engine.seq,
    openPositions: [...engine.positions.keys()],
    tradeCount: engine.trades.length,
    latestTradeId: engine.trades[0]?.id ?? "",
    oldestStoredTradeId,
  });
}

function saveToLocalStorage(engine: EngineRef): void {
  try {
    localStorage.setItem(LOCAL_STORAGE_KEY, JSON.stringify(buildPersistedPayload(engine)));
  } catch {
    // non-critical
  }
}

function loadFromLocalStorage(): GoldDbPayload | null {
  try {
    let raw = localStorage.getItem(LOCAL_STORAGE_KEY);
    if (!raw) {
      for (const legacyKey of LS_LEGACY_KEYS) {
        const legacy = localStorage.getItem(legacyKey);
        if (legacy) {
          raw = legacy;
          localStorage.setItem(LOCAL_STORAGE_KEY, legacy);
          localStorage.removeItem(legacyKey);
          break;
        }
      }
    }
    if (!raw) return null;
    return normalizeDbPayload(JSON.parse(raw) as Partial<GoldDbPayload>);
  } catch {
    return null;
  }
}

function compareSavedStates(a: GoldDbPayload, b: GoldDbPayload): number {
  const tradeSeqDiff = (a.tradeSeq ?? 0) - (b.tradeSeq ?? 0);
  if (tradeSeqDiff !== 0) return tradeSeqDiff;
  const tradeCountDiff = (a.trades?.length ?? 0) - (b.trades?.length ?? 0);
  if (tradeCountDiff !== 0) return tradeCountDiff;
  const positionCountDiff = (a.positions?.length ?? 0) - (b.positions?.length ?? 0);
  if (positionCountDiff !== 0) return positionCountDiff;
  return 0;
}

async function saveGoldState(engine: EngineRef): Promise<void> {
  saveToLocalStorage(engine);
  try {
    await fetch("/api/forex/gold/state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildPersistedPayload(engine)),
    });
  } catch {
    // non-critical
  }
}

function applySavedState(engine: EngineRef, saved: GoldDbPayload): boolean {
  engine.balance = num(saved.balance, INITIAL_BALANCE);
  engine.totalWins = nonNegativeInt(saved.totalWins, 0);
  engine.totalLosses = nonNegativeInt(saved.totalLosses, 0);
  engine.totalRealizedPnl = num(saved.totalPnl, 0);
  engine.seq = nonNegativeInt(saved.tradeSeq, 0);
  engine.trades = (saved.trades ?? []).slice(0, MAX_TRADES);
  engine.positions.clear();

  for (const persisted of saved.positions ?? []) {
    const position: InternalPosition = {
      id: persisted.id,
      strategyId: persisted.strategyId,
      strategyName: STRAT_DEFS.find((def) => def.id === persisted.strategyId)?.name ?? persisted.id,
      side: persisted.side,
      entryPrice: persisted.entryPrice,
      currentPrice: persisted.currentPrice,
      tpPrice: persisted.tpPrice,
      slPrice: persisted.slPrice,
      quantity: persisted.quantity,
      notional: persisted.notional,
      entryTime: persisted.entryTime,
      unrealizedPnl: persisted.unrealizedPnl,
      returnPct: persisted.returnPct,
      peakReturnPct: persisted.peakReturnPct,
    };
    engine.positions.set(position.id, position);
    const strategy = engine.strategies.find((item) => item.def.id === position.strategyId);
    if (strategy) {
      strategy.position = position;
      strategy.status = "IN_POSITION";
      strategy.regime = "UNKNOWN";
    }
  }

  for (const strategy of engine.strategies) {
    const savedStrategy = (saved.strategies ?? []).find((item) => item.id === strategy.def.id);
    if (!savedStrategy) continue;
    strategy.totalTrades = savedStrategy.totalTrades;
    strategy.wins = savedStrategy.wins;
    strategy.losses = savedStrategy.losses;
    strategy.totalPnl = savedStrategy.totalPnl;
    strategy.winRate = savedStrategy.winRate;
    strategy.cooldownUntil = savedStrategy.cooldownUntil;
    strategy.consecutiveLosses = savedStrategy.consecutiveLosses;
    strategy.regime = savedStrategy.regime ?? "UNKNOWN";
    if (!strategy.position) strategy.status = strategy.cooldownUntil > Date.now() ? "COOLING" : "WARMING";
  }

  normalizeEngineAccount(engine);
  return true;
}

async function loadGoldState(engine: EngineRef): Promise<boolean> {
  const local = loadFromLocalStorage();
  let dbState: GoldDbPayload | null = null;
  try {
    const response = await fetch("/api/forex/gold/state");
    if (response.ok) {
      const data = await response.json() as { ok: boolean; found: boolean; state?: GoldDbPayload };
      if (data.ok && data.found && data.state) dbState = normalizeDbPayload(data.state);
    }
  } catch {
    // fall back to local snapshot if DB is unavailable
  }

  const saved = local && dbState
    ? (compareSavedStates(local, dbState) >= 0 ? local : dbState)
    : (local ?? dbState);

  return saved ? applySavedState(engine, saved) : false;
}

function normalizeDbPayload(raw: Partial<GoldDbPayload> | null | undefined): GoldDbPayload | null {
  if (!raw) return null;
  return {
    balance: num(raw.balance, INITIAL_BALANCE),
    totalWins: nonNegativeInt(raw.totalWins, 0),
    totalLosses: nonNegativeInt(raw.totalLosses, 0),
    totalPnl: num(raw.totalPnl, 0),
    tradeSeq: nonNegativeInt(raw.tradeSeq, 0),
    positions: Array.isArray(raw.positions) ? raw.positions : [],
    trades: Array.isArray(raw.trades) ? raw.trades.slice(0, MAX_TRADES) : [],
    strategies: Array.isArray(raw.strategies) ? raw.strategies : [],
  };
}

export default function useForexGoldEngine() {
  const engineRef = useRef<EngineRef>(initEngine());
  const lastSavedSignatureRef = useRef("");
  const dbLoadedRef = useRef(false);
  const [quote, setQuote] = useState<GoldQuoteDisplay>({
    symbol: GOLD_SYMBOL,
    displayName: GOLD_DISPLAY_NAME,
    proxySymbol: GOLD_PROXY_SYMBOL,
    ltp: 0,
    changePct: 0,
    dayHigh: 0,
    dayLow: 0,
    signalScore: 0,
    regime: "UNKNOWN",
    sparkline: [],
    live: false,
  });
  const [positions, setPositions] = useState<GoldPosition[]>([]);
  const [trades, setTrades] = useState<GoldTrade[]>([]);
  const [strategies, setStrategies] = useState<GoldStrategyStatus[]>(
    STRAT_DEFS.map((def) => ({
      id: def.id,
      name: def.name,
      category: def.category,
      side: def.side,
      status: "WARMING",
      score: 0,
      regime: "UNKNOWN",
      rosterState: "WATCHLIST",
      allocationUSD: ALLOCATION_USD,
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
    })),
  );
  const [stats, setStats] = useState<GoldEngineStats>(EMPTY_STATS);

  const pushDisplayState = useCallback(() => {
    const engine = engineRef.current;
    const marketRegime = engine.bars.length >= MIN_BARS_FAST ? classifyRegime(buildSignalInputs(engine.bars)) : "UNKNOWN";
    const maxSignalScore = engine.strategies.reduce((best, strategy) => Math.max(best, strategy.score), 0);

    setQuote({
      symbol: GOLD_SYMBOL,
      displayName: GOLD_DISPLAY_NAME,
      proxySymbol: GOLD_PROXY_SYMBOL,
      ltp: engine.quote?.price ?? 0,
      changePct: engine.quote?.changePct ?? 0,
      dayHigh: engine.quote?.dayHigh ?? 0,
      dayLow: engine.quote?.dayLow ?? 0,
      signalScore: maxSignalScore,
      regime: marketRegime,
      sparkline: engine.bars.slice(-48),
      live: (engine.quote?.price ?? 0) > 0,
      interval: engine.quote?.interval,
      source: engine.quote?.source,
    });

    setPositions([...engine.positions.values()].map((position) => ({
      id: position.id,
      strategyId: position.strategyId,
      strategyName: position.strategyName,
      symbol: GOLD_SYMBOL,
      side: position.side,
      quantity: position.quantity,
      entryPrice: position.entryPrice,
      currentPrice: position.currentPrice,
      tpPrice: position.tpPrice,
      slPrice: position.slPrice,
      notional: position.notional,
      entryTime: new Date(position.entryTime).toISOString(),
      unrealizedPnl: position.unrealizedPnl,
      returnPct: position.returnPct,
    })));

    setTrades(engine.trades.slice(0, MAX_TRADES).map((trade) => ({
      id: trade.id,
      strategyId: trade.strategyId,
      strategyName: trade.strategyName,
      symbol: trade.symbol,
      side: trade.side,
      quantity: trade.quantity,
      entryPrice: trade.entryPrice,
      exitPrice: trade.exitPrice,
      netPnl: trade.netPnl,
      returnPct: trade.returnPct,
      entryTime: new Date(trade.entryTime).toISOString(),
      exitTime: new Date(trade.exitTime).toISOString(),
      exitReason: trade.exitReason,
      holdSeconds: trade.holdSeconds,
    })));

    setStrategies(engine.strategies.map((strategy) => ({
      id: strategy.def.id,
      name: strategy.def.name,
      category: strategy.def.category,
      side: strategy.def.side,
      status: strategy.status,
      score: strategy.score,
      regime: strategy.regime,
      rosterState: rosterStateFor(strategy.status),
      allocationUSD: Math.round(ALLOCATION_USD),
      totalTrades: strategy.totalTrades,
      wins: strategy.wins,
      losses: strategy.losses,
      totalPnl: strategy.totalPnl,
      winRate: strategy.winRate,
      cooldownUntil: strategy.cooldownUntil > 0 ? new Date(strategy.cooldownUntil).toISOString() : undefined,
    })));

    const { balance, equity, sessionPnl, unrealizedPnl } = normalizeEngineAccount(engine);
    const totalTrades = engine.strategies.reduce((sum, strategy) => sum + strategy.totalTrades, 0);
    const winRate = engine.totalWins + engine.totalLosses > 0 ? (engine.totalWins / (engine.totalWins + engine.totalLosses)) * 100 : 0;
    const diagnostics = engine.lastError || ((engine.quote?.price ?? 0) > 0
      ? `Tracking gold live via ${engine.quote?.proxySymbol ?? GOLD_PROXY_SYMBOL}${engine.quote?.interval ? ` on ${engine.quote.interval}` : ""}.`
      : "Waiting for gold market quotes.");

    setStats({
      equity,
      balance,
      sessionPnl,
      unrealizedPnl,
      realizedPnl: engine.totalRealizedPnl,
      totalTrades,
      totalWins: engine.totalWins,
      totalLosses: engine.totalLosses,
      openPositions: engine.positions.size,
      winRate,
      activeStrategies: engine.strategies.filter((strategy) => strategy.status !== "WARMING").length,
      warmingUp: engine.bars.length < MIN_BARS_SLOW,
      live: (engine.quote?.price ?? 0) > 0,
      livePrice: engine.quote?.price ?? 0,
      dayHigh: engine.quote?.dayHigh ?? 0,
      dayLow: engine.quote?.dayLow ?? 0,
      regime: marketRegime,
      lastUpdateAt: engine.lastFeedAt,
      diagnostics,
    });
  }, []);

  const processTick = useCallback((item: GoldMarketItem | null, errorMessage = "") => {
    const engine = engineRef.current;
    const now = Date.now();

    if (item?.price && item.price > 0) {
      engine.lastFeedAt = now;
      engine.quote = item;
      const historyBars = (item.candles ?? []).filter((bar) => bar > 0).slice(-MAX_BARS);
      const livePrice = item.price > 0 ? item.price : (historyBars.length > 0 ? historyBars[historyBars.length - 1] : 0);
      if (historyBars.length > 0) {
        engine.bars = [...historyBars];
        if (engine.bars[engine.bars.length - 1] !== livePrice) engine.bars.push(livePrice);
      } else if (livePrice > 0) {
        if (!engine.bars.length || engine.bars[engine.bars.length - 1] !== livePrice) engine.bars.push(livePrice);
      }
      if (engine.bars.length > MAX_BARS) engine.bars.splice(0, engine.bars.length - MAX_BARS);
      engine.lastError = "";
    } else if (errorMessage) {
      engine.lastError = errorMessage;
    }

    const latestPrice = engine.quote?.price ?? 0;
    const inputs = engine.bars.length >= MIN_BARS_FAST ? buildSignalInputs(engine.bars) : null;
    const marketRegime = inputs ? classifyRegime(inputs) : "UNKNOWN";

    for (const strategy of engine.strategies) {
      if (strategy.position && latestPrice > 0) {
        strategy.position.currentPrice = latestPrice;
        strategy.position.unrealizedPnl = calcPnl(strategy.position.side, strategy.position.entryPrice, latestPrice, strategy.position.quantity);
        strategy.position.returnPct = strategy.position.notional > 0 ? (strategy.position.unrealizedPnl / strategy.position.notional) * 100 : 0;
        strategy.position.peakReturnPct = Math.max(strategy.position.peakReturnPct, strategy.position.returnPct);
        strategy.regime = marketRegime;
        const exit = resolveExit(strategy.position, strategy.def, latestPrice, now);
        if (exit) closePosition(engine, strategy, exit.exitPrice, exit.reason, now);
      }
    }

    const candidates: Array<{ strategy: InternalStrategyState; score: number; regime: Regime }> = [];
    for (const strategy of engine.strategies) {
      if (strategy.position) continue;
      if (!inputs || engine.bars.length < strategy.def.minBars) {
        strategy.status = "WARMING";
        strategy.score = 0;
        strategy.regime = "UNKNOWN";
        continue;
      }
      if (strategy.totalTrades >= 8 && strategy.totalPnl < 0 && strategy.winRate < 35) {
        strategy.cooldownUntil = Math.max(strategy.cooldownUntil, now + UNDERPERFORMING_PAUSE_MS);
      }
      if (strategy.cooldownUntil > now) {
        strategy.status = "COOLING";
        strategy.score = 0;
        strategy.regime = marketRegime;
        continue;
      }

      const score = evalSignal(strategy.def.signal, inputs);
      strategy.status = "READY";
      strategy.score = score;
      strategy.regime = marketRegime;

      if (score >= SIGNAL_THRESHOLD && passesEntryConfirmation(strategy.def, inputs, marketRegime)) {
        candidates.push({ strategy, score, regime: marketRegime });
      }
    }

    candidates
      .sort((left, right) => {
        if (right.score !== left.score) return right.score - left.score;
        if (right.strategy.totalPnl !== left.strategy.totalPnl) return right.strategy.totalPnl - left.strategy.totalPnl;
        return right.strategy.winRate - left.strategy.winRate;
      });

    for (const candidate of candidates) {
      if (engine.positions.size >= MAX_OPEN_POSITIONS || latestPrice <= 0) break;
      if (candidate.strategy.position || candidate.strategy.cooldownUntil > now) continue;
      if (openPosition(engine, candidate.strategy, latestPrice, now)) candidate.strategy.regime = candidate.regime;
    }

    pushDisplayState();

    if (!dbLoadedRef.current) return;
    const signature = buildSaveSignature(engine);
    if (signature !== lastSavedSignatureRef.current) {
      lastSavedSignatureRef.current = signature;
      void saveGoldState(engine);
    }
  }, [pushDisplayState]);

  const reset = useCallback(() => {
    engineRef.current = initEngine();
    lastSavedSignatureRef.current = "";
    if (dbLoadedRef.current) void saveGoldState(engineRef.current);
    setQuote({
      symbol: GOLD_SYMBOL,
      displayName: GOLD_DISPLAY_NAME,
      proxySymbol: GOLD_PROXY_SYMBOL,
      ltp: 0,
      changePct: 0,
      dayHigh: 0,
      dayLow: 0,
      signalScore: 0,
      regime: "UNKNOWN",
      sparkline: [],
      live: false,
    });
    setPositions([]);
    setTrades([]);
    setStrategies(STRAT_DEFS.map((def) => ({
      id: def.id,
      name: def.name,
      category: def.category,
      side: def.side,
      status: "WARMING",
      score: 0,
      regime: "UNKNOWN",
      rosterState: "WATCHLIST",
      allocationUSD: ALLOCATION_USD,
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
    })));
    setStats(EMPTY_STATS);
  }, []);

  useEffect(() => {
    void loadGoldState(engineRef.current).then(() => {
      dbLoadedRef.current = true;
      lastSavedSignatureRef.current = buildSaveSignature(engineRef.current);
      pushDisplayState();
    });
  }, [pushDisplayState]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (dbLoadedRef.current) void saveGoldState(engineRef.current);
    }, 30_000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const flush = () => {
      if (!dbLoadedRef.current) return;
      saveToLocalStorage(engineRef.current);
      const payload = JSON.stringify(buildPersistedPayload(engineRef.current));
      navigator.sendBeacon("/api/forex/gold/state", new Blob([payload], { type: "application/json" }));
    };
    const onHide = () => { if (document.visibilityState === "hidden") flush(); };
    window.addEventListener("beforeunload", flush);
    document.addEventListener("visibilitychange", onHide);
    return () => {
      window.removeEventListener("beforeunload", flush);
      document.removeEventListener("visibilitychange", onHide);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      if (cancelled) return;
      try {
        const response = await fetch("/api/forex/gold/markets", { next: { revalidate: 0 } } as RequestInit);
        const payload = await response.json() as { ok?: boolean; data?: GoldMarketItem; error?: string };
        if (!response.ok || !payload.ok || !payload.data) {
          processTick(null, payload.error || `Gold API returned ${response.status}`);
          return;
        }
        processTick(payload.data, "");
      } catch {
        processTick(null, "Unable to fetch gold market data.");
      }
    };
    void tick();
    const interval = setInterval(() => void tick(), POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [processTick]);

  return { quote, positions, trades, strategies, stats, reset };
}
