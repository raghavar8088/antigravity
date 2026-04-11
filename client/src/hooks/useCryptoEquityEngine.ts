"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { CRYPTO_TOP_20, type CryptoAsset } from "@/lib/cryptoTop20";
import type { CryptoMarketItem } from "@/app/api/crypto/markets/route";

const INITIAL_BALANCE = 1_000_000;
const MAX_OPEN_POSITIONS = 22;        // ↑ more concurrent trades = more PnL
const MAX_BARS = 120;
const MIN_BARS_FAST = 18;
const MIN_BARS_SLOW = 28;
const SIGNAL_THRESHOLD = 61;
const POLL_MS = 3_000;
const MAX_TRADES = 500;
const ALLOCATION_USD = 50_000;        // 5% of the initial $1,000,000 capital per trade
const PROFIT_LOCK_PROGRESS = 0.24;    // ↑ lock profits earlier in the hold
const PROFIT_LOCK_SHARE = 0.35;       // ↑ lock at lower threshold
const LATE_EXIT_PROGRESS = 0.54;
const LATE_EXIT_MIN_GAIN = 0.05;
const GRIND_EXIT_PROGRESS = 0.38;     // ↑ exit grinders earlier
const GRIND_EXIT_SHARE = 0.18;        // ↑ lower threshold to exit grinders
const TRAIL_ACTIVATION_PCT = 0.85;    // ↑ trailing protection kicks in sooner
const TRAIL_GIVEBACK_SHARE = 0.26;    // ↓ give back less profit before trail exit
const MIN_SIZE_MULTIPLIER = 0.5;
const MAX_SIZE_MULTIPLIER = 2.25;     // ↑ allow larger size on hot strategies
const LOSS_COOLDOWN_PENALTY = 0.35;
const UNDERPERFORMING_MIN_TRADES = 8;
const UNDERPERFORMING_MAX_WINRATE = 35;
const UNDERPERFORMING_PAUSE_MS = 90 * 60 * 1000; // ↓ 90 min instead of 6h
// Volume confirmation thresholds
const VOL_SPIKE_RATIO = 1.6;          // volume must be 1.6x avg for boost
const VOL_BOOST_POINTS = 5;           // extra score points on volume spike
const VOL_HISTORY = 30;               // bars of volume history for avg

type Side = "LONG" | "SHORT";
type Status = "WARMING" | "READY" | "IN_POSITION" | "COOLING";

type SignalInputs = {
  price: number;
  prevPrice: number;
  fast: number;
  slow: number;
  trend: number;
  prevFast: number;
  prevSlow: number;
  mean20: number;
  std20: number;
  rsi14: number;
  high20: number;
  low20: number;
  momentum3: number;
  momentum6: number;
  volRatio: number; // currentVolume / rolling avg — 1.0 if unknown
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
  asset: CryptoAsset;
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
  currentSymbol: string;
  lastSignalSymbol: string;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  consecutiveLosses: number;
}

interface EngineRef {
  bars: Record<string, number[]>;
  volBars: Record<string, number[]>; // rolling volume history per symbol
  quotes: Record<string, CryptoMarketItem>;
  strategies: InternalStrategyState[];
  positions: Map<string, InternalPosition>;
  trades: InternalTrade[];
  balance: number;
  seq: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
  lastError: string;
}

export type CryptoQuoteDisplay = {
  symbol: string;
  name: string;
  sector: string;
  ltp: number;
  changePct: number;
  volume: number;
  signalScore: number;
  hasPosition: boolean;
  strategyLabel?: string;
  sparkline: number[];
};

export type CryptoPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  name: string;
  sector: string;
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

export type CryptoTrade = {
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

export type CryptoStrategyStatus = {
  id: number;
  name: string;
  category: string;
  side: Side;
  status: Status;
  currentSymbol: string;
  score: number;
  allocationUSD: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  cooldownUntil?: string;
};

export type CryptoEngineStats = {
  equity: number;
  balance: number;
  sessionPnl: number;
  unrealizedPnl: number;
  realizedPnl: number;
  totalTrades: number;
  openPositions: number;
  winRate: number;
  activeStrategies: number;
  warmingUp: boolean;
  liveSymbols: number;
  lastUpdateAt: number;
  diagnostics: string;
};

type CryptoDbPosition = {
  id: string;
  strategyId: number;
  symbol: string;
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

type CryptoDbTrade = {
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

type CryptoDbStrategy = {
  id: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  cooldownUntil: number;
  consecutiveLosses: number;
};

type CryptoDbPayload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  tradeSeq: number;
  positions: CryptoDbPosition[];
  trades: CryptoDbTrade[];
  strategies: CryptoDbStrategy[];
};

type CategoryProfile = { minTp: number; maxTp: number; minSl: number; maxSl: number; holdMins: number };

const CATEGORY_PROFILES: Record<string, CategoryProfile> = {
  Momentum:        { minTp: 3.8, maxTp: 5.8, minSl: 1.4, maxSl: 2.2, holdMins: 95 },  // ↑ TP +20%
  Breakout:        { minTp: 4.4, maxTp: 6.5, minSl: 1.5, maxSl: 2.3, holdMins: 110 }, // ↑ TP +20%
  Trend:           { minTp: 4.2, maxTp: 6.2, minSl: 1.4, maxSl: 2.2, holdMins: 115 }, // ↑ TP +20%
  "Mean Reversion":{ minTp: 2.8, maxTp: 4.4, minSl: 1.2, maxSl: 2.0, holdMins: 70 },  // ↑ TP +20%
  VWAP:            { minTp: 3.2, maxTp: 4.8, minSl: 1.2, maxSl: 2.0, holdMins: 85 },  // ↑ TP +20%
  Surge:           { minTp: 4.0, maxTp: 6.0, minSl: 1.5, maxSl: 2.2, holdMins: 90 },  // new
};

const BASE_SIGNALS = [
  { signal: "BREAKOUT",      category: "Breakout",        longName: "Range_Breakout",          shortName: "Range_Breakdown",           tpPct: 5.2, slPct: 2.1, cooldownMinutes: 14, minBars: MIN_BARS_SLOW },
  { signal: "EMA_CROSS",     category: "Momentum",        longName: "EMA_Cross_Impulse",        shortName: "EMA_Cross_Fade",            tpPct: 4.5, slPct: 1.8, cooldownMinutes: 11, minBars: MIN_BARS_SLOW },
  { signal: "RSI_BOUNCE",    category: "Mean Reversion",  longName: "RSI_Compression_Reclaim",  shortName: "RSI_Compression_Fade",      tpPct: 3.8, slPct: 1.6, cooldownMinutes: 10, minBars: MIN_BARS_FAST },
  { signal: "VWAP_RECLAIM",  category: "VWAP",            longName: "VWAP_Reclaim",             shortName: "VWAP_Reject",               tpPct: 4.0, slPct: 1.6, cooldownMinutes: 10, minBars: MIN_BARS_SLOW },
  { signal: "TREND_CONT",    category: "Trend",           longName: "Trend_Continuation",       shortName: "Trend_Reversal_Pressure",   tpPct: 5.0, slPct: 2.0, cooldownMinutes: 16, minBars: MIN_BARS_SLOW },
  { signal: "MOMENTUM_SURGE",category: "Surge",           longName: "Momentum_Surge_Long",      shortName: "Momentum_Surge_Short",      tpPct: 5.0, slPct: 1.8, cooldownMinutes: 12, minBars: MIN_BARS_FAST },
];

const VARIANTS = [
  { suffix: "Scalp", tpBump: -0.4, slBump: -0.2, cooldown: -2 },
  { suffix: "Pulse", tpBump: -0.15, slBump: 0, cooldown: -1 },
  { suffix: "Core", tpBump: 0, slBump: 0, cooldown: 0 },
  { suffix: "Flow", tpBump: 0.25, slBump: 0.1, cooldown: 1 },
  { suffix: "Pro", tpBump: 0.45, slBump: 0.15, cooldown: 2 },
];

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function profileFor(category: string): CategoryProfile {
  return CATEGORY_PROFILES[category] ?? { minTp: 2.8, maxTp: 4.2, minSl: 1.4, maxSl: 2.4, holdMins: 85 };
}

function tuneStrategy(def: StratDef): StratDef {
  const profile = profileFor(def.category);
  return {
    ...def,
    tpPct: clamp(def.tpPct, profile.minTp, profile.maxTp),
    slPct: clamp(def.slPct, profile.minSl, profile.maxSl),
    holdMinutes: profile.holdMins,
  };
}

const STRAT_DEFS: StratDef[] = (() => {
  const defs: StratDef[] = [];
  let id = 1;
  for (const base of BASE_SIGNALS) {
    for (const variant of VARIANTS) {
      defs.push(tuneStrategy({ id: id++, name: `${base.longName}_${variant.suffix}_LONG`, category: base.category, side: "LONG", signal: base.signal, tpPct: base.tpPct + variant.tpBump, slPct: base.slPct + variant.slBump, cooldownMinutes: Math.max(6, base.cooldownMinutes + variant.cooldown), minBars: base.minBars, holdMinutes: profileFor(base.category).holdMins }));
      defs.push(tuneStrategy({ id: id++, name: `${base.shortName}_${variant.suffix}_SHORT`, category: base.category, side: "SHORT", signal: `${base.signal}_SHORT`, tpPct: base.tpPct + variant.tpBump, slPct: base.slPct + variant.slBump, cooldownMinutes: Math.max(6, base.cooldownMinutes + variant.cooldown), minBars: base.minBars, holdMinutes: profileFor(base.category).holdMins }));
    }
  }
  return defs;
})();

const CRYPTO_ASSET_BY_SYMBOL = new Map(CRYPTO_TOP_20.map((asset) => [asset.symbol, asset]));

function sma(values: number[], period: number): number {
  const slice = values.slice(-period);
  return slice.length ? slice.reduce((sum, value) => sum + value, 0) / slice.length : 0;
}

function ema(values: number[], period: number): number {
  if (!values.length) return 0;
  const k = 2 / (period + 1);
  let current = values[0];
  for (let index = 1; index < values.length; index++) current = values[index] * k + current * (1 - k);
  return current;
}

function stdDev(values: number[], period: number): number {
  const slice = values.slice(-period);
  if (!slice.length) return 0;
  const avg = slice.reduce((sum, value) => sum + value, 0) / slice.length;
  return Math.sqrt(slice.reduce((sum, value) => sum + (value - avg) ** 2, 0) / slice.length);
}

function rsi(values: number[], period: number): number {
  if (values.length < 2) return 50;
  const start = Math.max(1, values.length - period);
  let gains = 0;
  let losses = 0;
  for (let index = start; index < values.length; index++) {
    const diff = values[index] - values[index - 1];
    if (diff > 0) gains += diff;
    else losses -= diff;
  }
  if (losses === 0) return 100;
  return 100 - 100 / (1 + gains / losses);
}

function scoreClamp(value: number, min = 0, max = 99): number {
  return Math.max(min, Math.min(max, value));
}

function buildSignalInputs(bars: number[], volRatio = 1.0): SignalInputs {
  const last = bars.length - 1;
  const price = bars[last];
  const previous = bars.slice(0, -1);
  return {
    price,
    prevPrice: last > 0 ? bars[last - 1] : price,
    fast: ema(bars, 6),
    slow: ema(bars, 15),
    trend: ema(bars, 28),
    prevFast: ema(previous, 6),
    prevSlow: ema(previous, 15),
    mean20: sma(bars, 20),
    std20: stdDev(bars, 20),
    rsi14: rsi(bars, 14),
    high20: Math.max(...bars.slice(-20)),
    low20: Math.min(...bars.slice(-20)),
    momentum3: last >= 3 ? ((price - bars[last - 3]) / bars[last - 3]) * 100 : 0,
    momentum6: last >= 6 ? ((price - bars[last - 6]) / bars[last - 6]) * 100 : 0,
    volRatio,
  };
}

// NOTE: bars are 3-second sampled price ticks from Binance lastPrice.
// At 3s resolution, BTC moves ~0.001–0.02% per tick on average.
// All momentum thresholds are calibrated for this sub-minute bar frequency.
function evalSignal(signal: string, input: SignalInputs): number {
  const { price, prevPrice, fast, slow, trend, prevFast, prevSlow, mean20, rsi14, high20, low20, momentum3, momentum6 } = input;
  switch (signal) {
    case "BREAKOUT":
      // 0.04% momentum over 9s is achievable on any active crypto tick
      return price > high20 * 1.0004 && fast > slow && rsi14 >= 54 && momentum3 > 0.04
        ? scoreClamp(72 + (price / high20 - 1) * 8000 + momentum3 * 40) : 0;
    case "BREAKOUT_SHORT":
      return price < low20 * 0.9996 && fast < slow && rsi14 <= 46 && momentum3 < -0.04
        ? scoreClamp(72 + (low20 / price - 1) * 8000 + Math.abs(momentum3) * 40) : 0;
    case "EMA_CROSS":
      return prevFast <= prevSlow && fast > slow && price > trend && rsi14 >= 52
        ? scoreClamp(70 + (fast / slow - 1) * 18000 + (rsi14 - 50) * 0.4) : 0;
    case "EMA_CROSS_SHORT":
      return prevFast >= prevSlow && fast < slow && price < trend && rsi14 <= 48
        ? scoreClamp(70 + (slow / fast - 1) * 18000 + (50 - rsi14) * 0.4) : 0;
    case "RSI_BOUNCE":
      return rsi14 <= 33 && price >= prevPrice && momentum3 > -0.25
        ? scoreClamp(67 + (35 - rsi14) * 1.4) : 0;
    case "RSI_BOUNCE_SHORT":
      return rsi14 >= 67 && price <= prevPrice && momentum3 < 0.25
        ? scoreClamp(67 + (rsi14 - 65) * 1.4) : 0;
    case "VWAP_RECLAIM":
      // Price crossing above mean20 — crossover on 3s ticks
      return price > mean20 && prevPrice <= mean20 * 1.0003 && momentum3 > 0.02
        ? scoreClamp(68 + (price / mean20 - 1) * 9000 + momentum3 * 30) : 0;
    case "VWAP_RECLAIM_SHORT":
      return price < mean20 && prevPrice >= mean20 * 0.9997 && momentum3 < -0.02
        ? scoreClamp(68 + (mean20 / price - 1) * 9000 + Math.abs(momentum3) * 30) : 0;
    case "TREND_CONT":
      // 0.12% over 18s is a real directional push at 3s resolution
      return fast > slow && slow > trend && momentum6 > 0.12 && rsi14 >= 54 && rsi14 <= 78
        ? scoreClamp(73 + momentum6 * 60 + (rsi14 - 54) * 0.3) : 0;
    case "TREND_CONT_SHORT":
      return fast < slow && slow < trend && momentum6 < -0.12 && rsi14 >= 22 && rsi14 <= 46
        ? scoreClamp(73 + Math.abs(momentum6) * 60 + (46 - rsi14) * 0.3) : 0;
    // Volume-spike surge: uses incremental volume delta ratio
    case "MOMENTUM_SURGE":
      return input.volRatio >= VOL_SPIKE_RATIO && momentum3 > 0.08 && fast > slow && rsi14 >= 52 && rsi14 <= 78
        ? scoreClamp(74 + momentum3 * 50 + (input.volRatio - 1) * 8 + (rsi14 - 50) * 0.3)
        : 0;
    case "MOMENTUM_SURGE_SHORT":
      return input.volRatio >= VOL_SPIKE_RATIO && momentum3 < -0.08 && fast < slow && rsi14 >= 22 && rsi14 <= 48
        ? scoreClamp(74 + Math.abs(momentum3) * 50 + (input.volRatio - 1) * 8 + (50 - rsi14) * 0.3)
        : 0;
    default:
      return 0;
  }
}

function classifyRegime(input: SignalInputs): string {
  const trendGapPct = input.price > 0 ? Math.abs(input.fast - input.slow) / input.price * 100 : 0;
  const volPct = input.price > 0 ? input.std20 / input.price * 100 : 0;
  // Thresholds scaled for 3-second bars (momentum is tiny vs minute bars)
  if (input.fast > input.slow && input.slow > input.trend && input.momentum6 > 0.08) return "TRENDING_BULL";
  if (input.fast < input.slow && input.slow < input.trend && input.momentum6 < -0.08) return "TRENDING_BEAR";
  if (volPct >= 0.3 || trendGapPct >= 0.12) return "HIGH_VOL";
  return "RANGE";
}

function isCategoryAligned(category: string, regime: string): boolean {
  if (regime === "HIGH_VOL") return category !== "Mean Reversion";
  if (regime === "RANGE") return category !== "Trend";
  return true;
}

function passesEntryConfirmation(def: StratDef, input: SignalInputs, regime: string): boolean {
  if (!isCategoryAligned(def.category, regime)) return false;
  if (def.side === "LONG") {
    if (def.category === "VWAP") return input.price >= input.mean20 && input.rsi14 >= 48;
    if (def.category === "Mean Reversion") return input.rsi14 <= 45 && input.price >= input.prevPrice;
    if (def.category === "Surge") return input.fast >= input.slow && input.momentum3 > 0.01;
    // 0.015% momentum over 9s is a genuine directional push at 3s bar frequency
    return input.price >= input.fast && input.fast >= input.slow && input.momentum3 > 0.015;
  }
  if (def.category === "VWAP") return input.price <= input.mean20 && input.rsi14 <= 52;
  if (def.category === "Mean Reversion") return input.rsi14 >= 55 && input.price <= input.prevPrice;
  if (def.category === "Surge") return input.fast <= input.slow && input.momentum3 < -0.01;
  return input.price <= input.fast && input.fast <= input.slow && input.momentum3 < -0.015;
}

function computeQuantity(price: number): number {
  return price > 0 ? Math.max(0.0001, ALLOCATION_USD / price) : 0;
}

function sizeMultiplierFor(strategy: InternalStrategyState): number {
  let multiplier = 1;
  if (strategy.totalTrades >= 6 && strategy.winRate >= 55) multiplier += 0.20;
  if (strategy.totalTrades >= 12 && strategy.winRate >= 60) multiplier += 0.25;
  if (strategy.totalTrades >= 20 && strategy.winRate >= 65) multiplier += 0.30; // hot streak bonus
  if (strategy.totalTrades > 0) {
    const avgPnlRatio = strategy.totalPnl / (strategy.totalTrades * ALLOCATION_USD);
    if (avgPnlRatio > 0.025) multiplier += 0.25;
    if (avgPnlRatio > 0.05)  multiplier += 0.20; // extra bonus for high avg profit
    if (avgPnlRatio < -0.015) multiplier -= 0.15;
  }
  multiplier -= strategy.consecutiveLosses * LOSS_COOLDOWN_PENALTY * 0.2;
  return clamp(multiplier, MIN_SIZE_MULTIPLIER, MAX_SIZE_MULTIPLIER);
}

function cooldownMsFor(strategy: InternalStrategyState, won: boolean): number {
  const base = strategy.def.cooldownMinutes * 60 * 1000;
  if (won) return base;
  return Math.round(base * (1 + strategy.consecutiveLosses * LOSS_COOLDOWN_PENALTY));
}

function shouldPauseStrategy(strategy: InternalStrategyState): boolean {
  return strategy.totalTrades >= UNDERPERFORMING_MIN_TRADES && strategy.totalPnl < 0 && strategy.winRate < UNDERPERFORMING_MAX_WINRATE;
}

function calcPnl(side: Side, entryPrice: number, currentPrice: number, quantity: number): number {
  return (currentPrice - entryPrice) * quantity * (side === "LONG" ? 1 : -1);
}

function resolveExit(position: InternalPosition, def: StratDef, currentPrice: number, now: number): { reason: string; exitPrice: number } | null {
  const returnPct = position.notional > 0 ? (calcPnl(position.side, position.entryPrice, currentPrice, position.quantity) / position.notional) * 100 : 0;
  const maxHoldMs = def.holdMinutes * 60 * 1000;
  const timeProgress = maxHoldMs > 0 ? Math.min(1, (now - position.entryTime) / maxHoldMs) : 0;
  const profitLockThreshold = Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * PROFIT_LOCK_SHARE);
  if (position.side === "LONG") {
    if (currentPrice >= position.tpPrice) return { reason: "TP", exitPrice: position.tpPrice };
    if (currentPrice <= position.slPrice) return { reason: "SL", exitPrice: position.slPrice };
  } else {
    if (currentPrice <= position.tpPrice) return { reason: "TP", exitPrice: position.tpPrice };
    if (currentPrice >= position.slPrice) return { reason: "SL", exitPrice: position.slPrice };
  }
  if (timeProgress >= GRIND_EXIT_PROGRESS && returnPct >= Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * GRIND_EXIT_SHARE)) return { reason: "PROFIT_LOCK", exitPrice: currentPrice };
  if (timeProgress >= PROFIT_LOCK_PROGRESS && returnPct >= profitLockThreshold) return { reason: "PROFIT_LOCK", exitPrice: currentPrice };
  if (position.peakReturnPct >= Math.max(TRAIL_ACTIVATION_PCT, def.tpPct * 0.4) && returnPct > 0 && returnPct <= position.peakReturnPct * (1 - TRAIL_GIVEBACK_SHARE)) return { reason: "TRAIL_STOP", exitPrice: currentPrice };
  if (timeProgress >= LATE_EXIT_PROGRESS && returnPct >= LATE_EXIT_MIN_GAIN) return { reason: "LATE_EXIT", exitPrice: currentPrice };
  if (maxHoldMs > 0 && now - position.entryTime >= maxHoldMs) return { reason: "TIME_EXIT", exitPrice: currentPrice };
  return null;
}

function initEngine(): EngineRef {
  return {
    bars: {},
    volBars: {},
    quotes: {},
    strategies: STRAT_DEFS.map((def) => ({ def, position: null, status: "WARMING", cooldownUntil: 0, score: 0, currentSymbol: "", lastSignalSymbol: "", totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0, consecutiveLosses: 0 })),
    positions: new Map(),
    trades: [],
    balance: INITIAL_BALANCE,
    seq: 0,
    totalWins: 0,
    totalLosses: 0,
    totalRealizedPnl: 0,
    lastError: "",
  };
}

const EMPTY_STATS: CryptoEngineStats = {
  equity: INITIAL_BALANCE,
  balance: INITIAL_BALANCE,
  sessionPnl: 0,
  unrealizedPnl: 0,
  realizedPnl: 0,
  totalTrades: 0,
  openPositions: 0,
  winRate: 0,
  activeStrategies: 0,
  warmingUp: true,
  liveSymbols: 0,
  lastUpdateAt: 0,
  diagnostics: "Bootstrapping 40-coin crypto feed.",
};

function buildPersistedPayload(engine: EngineRef): CryptoDbPayload {
  return {
    balance: engine.balance,
    totalWins: engine.totalWins,
    totalLosses: engine.totalLosses,
    totalPnl: engine.totalRealizedPnl,
    tradeSeq: engine.seq,
    positions: [...engine.positions.values()].map((position) => ({
      id: position.id,
      strategyId: position.strategyId,
      symbol: position.asset.symbol,
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
    trades: engine.trades.slice(0, MAX_TRADES).map((trade) => ({
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
      entryTime: trade.entryTime,
      exitTime: trade.exitTime,
      exitReason: trade.exitReason,
      holdSeconds: trade.holdSeconds,
    })),
    strategies: engine.strategies.map((strategy) => ({
      id: strategy.def.id,
      totalTrades: strategy.totalTrades,
      wins: strategy.wins,
      losses: strategy.losses,
      totalPnl: strategy.totalPnl,
      winRate: strategy.winRate,
      cooldownUntil: strategy.cooldownUntil,
      consecutiveLosses: strategy.consecutiveLosses,
    })),
  };
}

const LS_KEY = "crypto_equity_state_v1";

function saveToLocalStorage(engine: EngineRef): void {
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(buildPersistedPayload(engine)));
  } catch {
    // quota exceeded or SSR — ignore
  }
}

function loadFromLocalStorage(): CryptoDbPayload | null {
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as CryptoDbPayload;
  } catch {
    return null;
  }
}

async function saveCryptoState(engine: EngineRef): Promise<void> {
  saveToLocalStorage(engine);
  try {
    await fetch("/api/crypto/equity-state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildPersistedPayload(engine)),
    });
  } catch {
    // non-critical
  }
}

async function loadCryptoState(engine: EngineRef): Promise<boolean> {
  // Try localStorage first (always available, no network needed)
  const lsState = loadFromLocalStorage();

  // Also try DB (may have newer data if another tab saved)
  let dbState: CryptoDbPayload | null = null;
  try {
    const response = await fetch("/api/crypto/equity-state");
    if (response.ok) {
      const data = await response.json() as { ok: boolean; found: boolean; disabled?: boolean; state?: CryptoDbPayload };
      if (data.ok && data.found && data.state) dbState = data.state;
    }
  } catch {
    // DB unavailable, use localStorage
  }

  // Pick whichever has more trades (most up-to-date)
  let saved: CryptoDbPayload | null = null;
  if (dbState && lsState) {
    saved = (dbState.tradeSeq ?? 0) >= (lsState.tradeSeq ?? 0) ? dbState : lsState;
  } else {
    saved = dbState ?? lsState;
  }

  if (!saved) return false;

  try {
    engine.balance = saved.balance;
    engine.totalWins = saved.totalWins;
    engine.totalLosses = saved.totalLosses;
    engine.totalRealizedPnl = saved.totalPnl;
    engine.seq = saved.tradeSeq;
    engine.trades = (saved.trades ?? []).slice(0, MAX_TRADES);
    engine.positions.clear();

    for (const persisted of saved.positions ?? []) {
      const asset = CRYPTO_ASSET_BY_SYMBOL.get(persisted.symbol);
      if (!asset) continue;
      const position: InternalPosition = {
        id: persisted.id,
        strategyId: persisted.strategyId,
        strategyName: STRAT_DEFS.find((def) => def.id === persisted.strategyId)?.name ?? persisted.id,
        asset,
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
        strategy.currentSymbol = asset.symbol;
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
      if (!strategy.position) {
        strategy.status = strategy.cooldownUntil > Date.now() ? "COOLING" : "WARMING";
      }
    }
    return true;
  } catch {
    return false;
  }
}

function openPosition(engine: EngineRef, strategy: InternalStrategyState, asset: CryptoAsset, entryPrice: number, now: number): boolean {
  const quantity = computeQuantity(entryPrice) * sizeMultiplierFor(strategy);
  const notional = quantity * entryPrice;
  if (notional <= 0 || engine.balance < notional) return false;
  engine.seq++;
  engine.balance -= notional;
  const tpMultiplier = 1 + strategy.def.tpPct / 100;
  const slMultiplier = 1 - strategy.def.slPct / 100;
  const position: InternalPosition = {
    id: `CRYPTO-${Date.now()}-${engine.seq}`,
    strategyId: strategy.def.id,
    strategyName: strategy.def.name,
    asset,
    side: strategy.def.side,
    entryPrice,
    currentPrice: entryPrice,
    tpPrice: strategy.def.side === "LONG" ? entryPrice * tpMultiplier : entryPrice * (2 - tpMultiplier),
    slPrice: strategy.def.side === "LONG" ? entryPrice * slMultiplier : entryPrice * (2 - slMultiplier),
    quantity,
    notional,
    entryTime: now,
    unrealizedPnl: 0,
    returnPct: 0,
    peakReturnPct: 0,
  };
  strategy.position = position;
  strategy.status = "IN_POSITION";
  strategy.currentSymbol = asset.symbol;
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
    symbol: position.asset.symbol,
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
    strategy.consecutiveLosses += 1;
  }
  strategy.totalPnl += netPnl;
  strategy.winRate = strategy.totalTrades > 0 ? (strategy.wins / strategy.totalTrades) * 100 : 0;
  strategy.cooldownUntil = now + cooldownMsFor(strategy, netPnl >= 0);
  strategy.status = "COOLING";
  strategy.currentSymbol = "";
  strategy.position = null;
  engine.positions.delete(position.id);
}

export default function useCryptoEquityEngine() {
  const engineRef = useRef<EngineRef>(initEngine());
  const lastSavedSignatureRef = useRef("");
  const dbLoadedRef = useRef(false);
  const [quotes, setQuotes] = useState<CryptoQuoteDisplay[]>(CRYPTO_TOP_20.map((asset) => ({ symbol: asset.symbol, name: asset.name, sector: asset.sector, ltp: 0, changePct: 0, volume: 0, signalScore: 0, hasPosition: false, sparkline: [] })));
  const [positions, setPositions] = useState<CryptoPosition[]>([]);
  const [trades, setTrades] = useState<CryptoTrade[]>([]);
  const [strategies, setStrategies] = useState<CryptoStrategyStatus[]>(STRAT_DEFS.map((def) => ({ id: def.id, name: def.name, category: def.category, side: def.side, status: "WARMING", currentSymbol: "", score: 0, allocationUSD: ALLOCATION_USD, totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0 })));
  const [stats, setStats] = useState<CryptoEngineStats>(EMPTY_STATS);

  const pushDisplayState = useCallback((errorMessage = "") => {
    const engine = engineRef.current;
    const now = Date.now();

    if (errorMessage) engine.lastError = errorMessage;

    const positionSides: Record<string, Set<Side>> = {};
    const symbolScores: Record<string, number> = {};
    for (const strategy of engine.strategies) {
      const symbol = strategy.position?.asset.symbol ?? strategy.lastSignalSymbol;
      if (symbol && strategy.score > 0) symbolScores[symbol] = Math.max(symbolScores[symbol] ?? 0, strategy.score);
      if (strategy.position) {
        if (!positionSides[strategy.position.asset.symbol]) positionSides[strategy.position.asset.symbol] = new Set();
        positionSides[strategy.position.asset.symbol].add(strategy.def.side);
      }
    }

    setQuotes(CRYPTO_TOP_20.map((asset) => {
      const quote = engine.quotes[asset.symbol];
      const sides = positionSides[asset.symbol];
      const sparkline = (engine.bars[asset.symbol] ?? []).slice(-24);
      return { symbol: asset.symbol, name: asset.name, sector: asset.sector, ltp: quote?.price ?? 0, changePct: quote?.changePct ?? 0, volume: quote?.volume ?? 0, signalScore: symbolScores[asset.symbol] ?? 0, hasPosition: Boolean(sides?.size), strategyLabel: sides ? [...sides].join("+") : undefined, sparkline };
    }));

    setPositions([...engine.positions.values()].map((position) => ({ id: position.id, strategyId: position.strategyId, strategyName: position.strategyName, symbol: position.asset.symbol, name: position.asset.name, sector: position.asset.sector, side: position.side, quantity: position.quantity, entryPrice: position.entryPrice, currentPrice: position.currentPrice, tpPrice: position.tpPrice, slPrice: position.slPrice, notional: position.notional, entryTime: new Date(position.entryTime).toISOString(), unrealizedPnl: position.unrealizedPnl, returnPct: position.returnPct })));

    setTrades(engine.trades.slice(0, 120).map((trade) => ({ id: trade.id, strategyId: trade.strategyId, strategyName: trade.strategyName, symbol: trade.symbol, side: trade.side, quantity: trade.quantity, entryPrice: trade.entryPrice, exitPrice: trade.exitPrice, netPnl: trade.netPnl, returnPct: trade.returnPct, entryTime: new Date(trade.entryTime).toISOString(), exitTime: new Date(trade.exitTime).toISOString(), exitReason: trade.exitReason, holdSeconds: trade.holdSeconds })));

    setStrategies(engine.strategies.map((strategy) => ({ id: strategy.def.id, name: strategy.def.name, category: strategy.def.category, side: strategy.def.side, status: strategy.status, currentSymbol: strategy.currentSymbol || strategy.lastSignalSymbol, score: strategy.score, allocationUSD: Math.round(ALLOCATION_USD * sizeMultiplierFor(strategy)), totalTrades: strategy.totalTrades, wins: strategy.wins, losses: strategy.losses, totalPnl: strategy.totalPnl, winRate: strategy.winRate, cooldownUntil: strategy.cooldownUntil > 0 ? new Date(strategy.cooldownUntil).toISOString() : undefined })));

    const unrealizedPnl = [...engine.positions.values()].reduce((sum, position) => sum + position.unrealizedPnl, 0);
    const openNotional = [...engine.positions.values()].reduce((sum, position) => sum + position.notional, 0);
    const equity = engine.balance + openNotional + unrealizedPnl;
    const liveSymbols = CRYPTO_TOP_20.filter((asset) => (engine.quotes[asset.symbol]?.price ?? 0) > 0).length;
    const totalTrades = engine.strategies.reduce((sum, strategy) => sum + strategy.totalTrades, 0);
    const winRate = engine.totalWins + engine.totalLosses > 0 ? (engine.totalWins / (engine.totalWins + engine.totalLosses)) * 100 : 0;
    setStats({ equity, balance: engine.balance, sessionPnl: equity - INITIAL_BALANCE, unrealizedPnl, realizedPnl: engine.totalRealizedPnl, totalTrades, openPositions: engine.positions.size, winRate, activeStrategies: engine.strategies.filter((strategy) => strategy.status !== "WARMING").length, warmingUp: CRYPTO_TOP_20.every((asset) => (engine.bars[asset.symbol]?.length ?? 0) < MIN_BARS_SLOW), liveSymbols, lastUpdateAt: now, diagnostics: engine.lastError || (liveSymbols > 0 ? `Tracking ${liveSymbols}/40 crypto symbols live.` : "Waiting for crypto market quotes.") });
  }, []);

  const processTick = useCallback((items: CryptoMarketItem[], errorMessage = "") => {
    const engine = engineRef.current;
    const now = Date.now();
    if (errorMessage) engine.lastError = errorMessage;
    else if (items.length) engine.lastError = "";

    for (const item of items) {
      if (item.price <= 0) continue;
      engine.quotes[item.symbol] = item;
      const bars = engine.bars[item.symbol] ?? [];
      bars.push(item.price);
      if (bars.length > MAX_BARS) bars.splice(0, bars.length - MAX_BARS);
      engine.bars[item.symbol] = bars;
      // Track INCREMENTAL volume delta per tick (not cumulative 24hr volume)
      // Binance 24hr ticker gives cumulative volume — compute the delta each tick
      if (item.volume > 0) {
        const vb = engine.volBars[item.symbol] ?? [];
        const prevCumVol = engine.quotes[item.symbol]?.volume ?? 0;
        const delta = prevCumVol > 0 ? Math.max(0, item.volume - prevCumVol) : 0;
        if (delta > 0) {
          vb.push(delta);
          if (vb.length > VOL_HISTORY) vb.splice(0, vb.length - VOL_HISTORY);
          engine.volBars[item.symbol] = vb;
        }
      }
    }

    for (const strategy of engine.strategies) {
      if (strategy.position) {
        const latest = engine.quotes[strategy.position.asset.symbol];
        if (latest?.price > 0) {
          strategy.position.currentPrice = latest.price;
          strategy.position.unrealizedPnl = calcPnl(strategy.position.side, strategy.position.entryPrice, latest.price, strategy.position.quantity);
          strategy.position.returnPct = strategy.position.notional > 0 ? (strategy.position.unrealizedPnl / strategy.position.notional) * 100 : 0;
          strategy.position.peakReturnPct = Math.max(strategy.position.peakReturnPct, strategy.position.returnPct);
          const exit = resolveExit(strategy.position, strategy.def, latest.price, now);
          if (exit) closePosition(engine, strategy, exit.exitPrice, exit.reason, now);
        }
        continue;
      }
      if (shouldPauseStrategy(strategy)) {
        strategy.cooldownUntil = Math.max(strategy.cooldownUntil, now + UNDERPERFORMING_PAUSE_MS);
        strategy.status = "COOLING";
        strategy.score = 0;
        continue;
      }
      if (strategy.cooldownUntil > now) {
        strategy.status = "COOLING";
        strategy.score = 0;
        continue;
      }
      strategy.status = "READY";
      strategy.score = 0;
      strategy.lastSignalSymbol = "";
      if (engine.positions.size >= MAX_OPEN_POSITIONS) continue;

      let bestAsset: CryptoAsset | null = null;
      let bestScore = 0;
      for (const asset of CRYPTO_TOP_20) {
        const bars = engine.bars[asset.symbol];
        if (!bars || bars.length < strategy.def.minBars) continue;
        const vb = engine.volBars[asset.symbol] ?? [];
        const avgVol = vb.length >= 3 ? vb.slice(0, -1).reduce((s, v) => s + v, 0) / (vb.length - 1) : 0;
        const curVol = vb.length > 0 ? vb[vb.length - 1] : 0;
        const volRatio = avgVol > 0 ? curVol / avgVol : 1.0;
        const input = buildSignalInputs(bars, volRatio);
        const rawScore = evalSignal(strategy.def.signal, input);
        // boost score when volume is spiking — confirms conviction
        const score = rawScore > 0 && volRatio >= VOL_SPIKE_RATIO ? Math.min(99, rawScore + VOL_BOOST_POINTS) : rawScore;
        const confirmed = score >= SIGNAL_THRESHOLD && passesEntryConfirmation(strategy.def, input, classifyRegime(input));
        const displayScore = confirmed ? score : Math.min(score, SIGNAL_THRESHOLD - 1);
        if (displayScore > strategy.score) {
          strategy.score = displayScore;
          strategy.lastSignalSymbol = asset.symbol;
        }
        if (confirmed && score > bestScore) {
          bestScore = score;
          bestAsset = asset;
        }
      }
      if (bestAsset) {
        const price = engine.quotes[bestAsset.symbol]?.price ?? 0;
        if (price > 0) openPosition(engine, strategy, bestAsset, price, now);
      } else if (CRYPTO_TOP_20.every((asset) => (engine.bars[asset.symbol]?.length ?? 0) < strategy.def.minBars)) {
        strategy.status = "WARMING";
      }
    }

    pushDisplayState();

    if (!dbLoadedRef.current) return;
    const signature = JSON.stringify({
      balance: engine.balance,
      totalWins: engine.totalWins,
      totalLosses: engine.totalLosses,
      totalPnl: engine.totalRealizedPnl,
      tradeSeq: engine.seq,
      openPositions: [...engine.positions.keys()],
      tradeIds: engine.trades.slice(0, MAX_TRADES).map((trade) => trade.id),
    });
    if (signature !== lastSavedSignatureRef.current) {
      lastSavedSignatureRef.current = signature;
      void saveCryptoState(engine);
    }
  }, [pushDisplayState]);

  const reset = useCallback(() => {
    engineRef.current = initEngine();
    lastSavedSignatureRef.current = "";
    if (dbLoadedRef.current) void saveCryptoState(engineRef.current);
    setQuotes(CRYPTO_TOP_20.map((asset) => ({ symbol: asset.symbol, name: asset.name, sector: asset.sector, ltp: 0, changePct: 0, volume: 0, signalScore: 0, hasPosition: false, sparkline: [] })));
    setPositions([]);
    setTrades([]);
    setStrategies(STRAT_DEFS.map((def) => ({ id: def.id, name: def.name, category: def.category, side: def.side, status: "WARMING", currentSymbol: "", score: 0, allocationUSD: ALLOCATION_USD, totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0 })));
    setStats(EMPTY_STATS);
  }, []);

  useEffect(() => {
    void loadCryptoState(engineRef.current).then(() => {
      pushDisplayState();
      dbLoadedRef.current = true;
      lastSavedSignatureRef.current = JSON.stringify({
        balance: engineRef.current.balance,
        totalWins: engineRef.current.totalWins,
        totalLosses: engineRef.current.totalLosses,
        totalPnl: engineRef.current.totalRealizedPnl,
        tradeSeq: engineRef.current.seq,
        openPositions: [...engineRef.current.positions.keys()],
        tradeIds: engineRef.current.trades.slice(0, MAX_TRADES).map((trade) => trade.id),
      });
    });
  }, [pushDisplayState]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (dbLoadedRef.current) void saveCryptoState(engineRef.current);
    }, 15_000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const onUnload = () => {
      if (!dbLoadedRef.current) return;
      // localStorage is synchronous — always succeeds before tab closes
      saveToLocalStorage(engineRef.current);
      const payload = JSON.stringify(buildPersistedPayload(engineRef.current));
      navigator.sendBeacon("/api/crypto/equity-state", new Blob([payload], { type: "application/json" }));
    };
    window.addEventListener("beforeunload", onUnload);
    return () => window.removeEventListener("beforeunload", onUnload);
  }, []);

  const fetchDirectBinanceMarkets = useCallback(async (): Promise<{ ok: boolean; data: CryptoMarketItem[]; error?: string }> => {
    const symbols = JSON.stringify(CRYPTO_TOP_20.map((asset) => asset.pair));
    try {
      const response = await fetch(
        `https://api.binance.com/api/v3/ticker/24hr?symbols=${encodeURIComponent(symbols)}`,
        { cache: "no-store" },
      );
      if (!response.ok) {
        return { ok: false, data: [], error: `Binance fallback returned ${response.status}` };
      }
      const raw = await response.json() as Array<{
        symbol?: string;
        lastPrice?: string;
        openPrice?: string;
        highPrice?: string;
        lowPrice?: string;
        prevClosePrice?: string;
        volume?: string;
        priceChangePercent?: string;
      }>;
      const byPair = new Map(raw.map((item) => [item.symbol ?? "", item]));
      return {
        ok: true,
        data: CRYPTO_TOP_20.map((asset) => {
          const ticker = byPair.get(asset.pair);
          return {
            symbol: asset.symbol,
            pair: asset.pair,
            price: Number(ticker?.lastPrice ?? 0),
            open: Number(ticker?.openPrice ?? 0),
            high: Number(ticker?.highPrice ?? 0),
            low: Number(ticker?.lowPrice ?? 0),
            prevClose: Number(ticker?.prevClosePrice ?? 0),
            volume: Number(ticker?.volume ?? 0),
            changePct: Number(ticker?.priceChangePercent ?? 0),
          };
        }),
      };
    } catch {
      return { ok: false, data: [], error: "Direct Binance fallback failed." };
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      if (cancelled) return;
      try {
        const response = await fetch("/api/crypto/markets", { next: { revalidate: 0 } } as RequestInit);
        const payload = await response.json() as { ok?: boolean; data?: CryptoMarketItem[]; error?: string };
        if (!response.ok || !payload.ok || !payload.data?.length) {
          const fallback = await fetchDirectBinanceMarkets();
          if (fallback.ok && fallback.data.length) {
            processTick(fallback.data, payload.error ? `${payload.error} | using direct Binance fallback.` : "Using direct Binance fallback.");
            return;
          }
          processTick([], payload.error || fallback.error || `Crypto market API returned ${response.status}`);
          return;
        }
        processTick(payload.data, "");
      } catch {
        const fallback = await fetchDirectBinanceMarkets();
        if (fallback.ok && fallback.data.length) {
          processTick(fallback.data, "Internal crypto API unavailable | using direct Binance fallback.");
          return;
        }
        processTick([], "Unable to fetch crypto market data.");
      }
    };
    void tick();
    const interval = setInterval(() => void tick(), POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [fetchDirectBinanceMarkets, processTick]);

  return { quotes, positions, trades, strategies, stats, reset };
}
