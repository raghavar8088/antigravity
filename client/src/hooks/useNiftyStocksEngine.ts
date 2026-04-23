"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { NIFTY50_STOCKS, type Nifty50Stock } from "@/lib/nifty50Stocks";
import type { StockLTPItem } from "@/app/api/stocks/ltp/route";

// ─── Constants ────────────────────────────────────────────────────────────────
const INITIAL_BALANCE = 1_000_000;
const MAX_OPEN_POSITIONS = 8;
const MAX_BARS = 80; // ~4 min of price history at 3s ticks
const MIN_BARS_FAST = 15; // enough for RSI14
const MIN_BARS_SLOW = 22; // enough for EMA21 + Donchian20
const SIGNAL_THRESHOLD = 56;
const POLL_MS = 3_000;
const MAX_TRADES = 500;
const ALLOCATION_INR = 10_000; // 1% of the initial capital budget per trade
const STOCK_PROFIT_LOCK_PROGRESS = 0.28;
const STOCK_PROFIT_LOCK_SHARE = 0.44;
const STOCK_LATE_EXIT_PROGRESS = 0.56;
const STOCK_LATE_EXIT_MIN_GAIN = 0.06;
const STOCK_GRIND_EXIT_PROGRESS = 0.50;
const STOCK_GRIND_EXIT_SHARE = 0.26;
const STOCK_TRAIL_GIVEBACK_SHARE = 0.38;
const STOCK_MIN_SIZE_MULTIPLIER = 0.5;
const STOCK_MAX_SIZE_MULTIPLIER = 1.7;
const STOCK_LOSS_COOLDOWN_PENALTY = 0.3;
const STOCK_UNDERPERFORMING_MIN_TRADES = 8;
const STOCK_UNDERPERFORMING_MAX_WINRATE = 35;
const STOCK_UNDERPERFORMING_PAUSE_MS = 6 * 60 * 60 * 1000;

// ─── Strategy definitions ─────────────────────────────────────────────────────
interface StratDef {
  id: number;
  name: string;
  category: string;
  optionType: "CE" | "PE";
  signal: string;
  tpPct: number; // take-profit as % of entry premium
  slPct: number; // stop-loss as % of entry premium
  cooldownMinutes: number;
  minBars: number;
}

type StockCategoryProfile = {
  minTp: number;
  maxTp: number;
  minSl: number;
  maxSl: number;
  holdMins: number;
};

const DEFAULT_STOCK_PROFILE: StockCategoryProfile = {
  minTp: 44,
  maxTp: 60,
  minSl: 24,
  maxSl: 30,
  holdMins: 18,
};

const STOCK_CATEGORY_PROFILES: Record<string, StockCategoryProfile> = {
  Breakout: { minTp: 52, maxTp: 68, minSl: 26, maxSl: 32, holdMins: 20 },
  Momentum: { minTp: 48, maxTp: 64, minSl: 24, maxSl: 30, holdMins: 18 },
  "Mean Reversion": { minTp: 42, maxTp: 56, minSl: 22, maxSl: 28, holdMins: 14 },
  VWAP: { minTp: 40, maxTp: 54, minSl: 22, maxSl: 28, holdMins: 15 },
  "Price Action": { minTp: 48, maxTp: 62, minSl: 26, maxSl: 34, holdMins: 18 },
  Trend: { minTp: 55, maxTp: 72, minSl: 28, maxSl: 35, holdMins: 22 },
};

function stockClamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function stockProfileFor(category: string): StockCategoryProfile {
  return STOCK_CATEGORY_PROFILES[category] ?? DEFAULT_STOCK_PROFILE;
}

function tuneStockStrategy(def: StratDef): StratDef {
  const profile = stockProfileFor(def.category);
  return {
    ...def,
    tpPct: stockClamp(def.tpPct, profile.minTp, profile.maxTp),
    slPct: stockClamp(def.slPct, profile.minSl, profile.maxSl),
  };
}

function holdMinutesFor(def: StratDef): number {
  return stockProfileFor(def.category).holdMins;
}

const BASE_STRAT_DEFS: StratDef[] = [
  // ── CALL strategies (buy CE when bullish signal) ────────────────────────
  { id: 1,  name: "Breakout_Momentum_CE",    category: "Breakout",      optionType: "CE", signal: "BREAKOUT_CE",    tpPct: 85,  slPct: 38, cooldownMinutes: 3, minBars: MIN_BARS_SLOW },
  { id: 2,  name: "EMA_Cross_Bull_CE",       category: "Momentum",      optionType: "CE", signal: "EMA_CROSS_CE",   tpPct: 75,  slPct: 33, cooldownMinutes: 2, minBars: MIN_BARS_SLOW },
  { id: 3,  name: "RSI_Oversold_Bounce_CE",  category: "Mean Reversion", optionType: "CE", signal: "RSI_BOUNCE_CE",  tpPct: 70,  slPct: 32, cooldownMinutes: 2, minBars: MIN_BARS_FAST },
  { id: 4,  name: "VWAP_Reclaim_CE",         category: "VWAP",          optionType: "CE", signal: "VWAP_RECLAIM_CE",tpPct: 65,  slPct: 30, cooldownMinutes: 2, minBars: MIN_BARS_SLOW },
  { id: 5,  name: "Exhaustion_Reversal_CE",  category: "Price Action",  optionType: "CE", signal: "EXHAUSTION_CE",  tpPct: 95,  slPct: 42, cooldownMinutes: 4, minBars: MIN_BARS_SLOW },
  { id: 6,  name: "Trend_Continuation_CE",   category: "Trend",         optionType: "CE", signal: "TREND_CONT_CE",  tpPct: 100, slPct: 45, cooldownMinutes: 5, minBars: MIN_BARS_SLOW },
  // ── PUT strategies (buy PE when bearish signal) ─────────────────────────
  { id: 7,  name: "Breakdown_Momentum_PE",   category: "Breakout",      optionType: "PE", signal: "BREAKDOWN_PE",   tpPct: 85,  slPct: 38, cooldownMinutes: 3, minBars: MIN_BARS_SLOW },
  { id: 8,  name: "EMA_Cross_Bear_PE",       category: "Momentum",      optionType: "PE", signal: "EMA_CROSS_PE",   tpPct: 75,  slPct: 33, cooldownMinutes: 2, minBars: MIN_BARS_SLOW },
  { id: 9,  name: "RSI_Overbought_Fade_PE",  category: "Mean Reversion", optionType: "PE", signal: "RSI_FADE_PE",    tpPct: 70,  slPct: 32, cooldownMinutes: 2, minBars: MIN_BARS_FAST },
  { id: 10, name: "VWAP_Rejection_PE",       category: "VWAP",          optionType: "PE", signal: "VWAP_REJECT_PE", tpPct: 65,  slPct: 30, cooldownMinutes: 2, minBars: MIN_BARS_SLOW },
  { id: 11, name: "Blow_Off_Top_PE",         category: "Price Action",  optionType: "PE", signal: "EXHAUSTION_PE",  tpPct: 95,  slPct: 42, cooldownMinutes: 4, minBars: MIN_BARS_SLOW },
  { id: 12, name: "Trend_Breakdown_PE",      category: "Trend",         optionType: "PE", signal: "TREND_CONT_PE",  tpPct: 100, slPct: 45, cooldownMinutes: 5, minBars: MIN_BARS_SLOW },
];

// ─── Public display types ─────────────────────────────────────────────────────

const STRAT_DEFS: StratDef[] = BASE_STRAT_DEFS.map(tuneStockStrategy);

export type StockQuoteDisplay = {
  symbol: string;
  name: string;
  sector: string;
  ltp: number;
  prevClose: number;
  changePct: number;
  signalScore: number; // max signal score across all strategies
  hasPosition: boolean;
  strategyLabel?: string; // "CE" | "PE" | "CE+PE" if multiple
};

export type StockOptionPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  sector: string;
  optionType: "CE" | "PE";
  lotSize: number;
  numShares: number;
  entryUnderlying: number;
  currentUnderlying: number;
  entryPremium: number;
  currentPremium: number;
  tpPremium: number;
  slPremium: number;
  initialCost: number;
  entryTime: string;
  unrealizedPnl: number;
  returnPct: number;
};

export type StockOptionTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  optionType: "CE" | "PE";
  numShares: number;
  entryUnderlying: number;
  exitUnderlying: number;
  entryPremium: number;
  exitPremium: number;
  netPnl: number;
  returnPct: number;
  entryTime: string;
  exitTime: string;
  exitReason: string;
  holdSeconds: number;
};

export type StrategyDisplayStatus = {
  id: number;
  name: string;
  category: string;
  optionType: "CE" | "PE";
  status: "WARMING" | "READY" | "IN_POSITION" | "COOLING";
  currentStock: string;
  score: number;
  allocationINR: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  cooldownUntil?: string;
  lastTradeAt?: string;
};

export type StocksEngineStats = {
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
  lastUpdateAt: number;
};

export type StocksEngineState = {
  quotes: StockQuoteDisplay[];
  positions: StockOptionPosition[];
  trades: StockOptionTrade[];
  strategies: StrategyDisplayStatus[];
  stats: StocksEngineStats;
};

// ─── Internal engine types (stored in ref, no re-render) ──────────────────────

interface InternalPosition {
  id: string;
  strategyId: number;
  strategyName: string;
  stock: Nifty50Stock;
  optionType: "CE" | "PE";
  entryUnderlying: number;
  currentUnderlying: number;
  entryPremium: number;
  currentPremium: number;
  tpPremium: number;
  slPremium: number;
  numShares: number;
  initialCost: number;
  entryTime: number; // ms
  unrealizedPnl: number;
  returnPct: number;
  peakGainPct: number;
}

interface InternalTrade {
  id: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  optionType: "CE" | "PE";
  numShares: number;
  entryUnderlying: number;
  exitUnderlying: number;
  entryPremium: number;
  exitPremium: number;
  netPnl: number;
  returnPct: number;
  entryTime: number; // ms
  exitTime: number; // ms
  exitReason: string;
  holdSeconds: number;
}

interface InternalStratState {
  def: StratDef;
  position: InternalPosition | null;
  status: "WARMING" | "READY" | "IN_POSITION" | "COOLING";
  cooldownUntil: number; // ms
  score: number;
  currentStock: string;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  lastTradeAt: number; // ms
  consecutiveLosses: number;
}

interface EngineRef {
  bars: Record<string, number[]>; // symbol → last MAX_BARS closes
  quotes: Record<string, StockLTPItem>; // symbol → latest quote
  strategies: InternalStratState[];
  positions: Map<string, InternalPosition>; // id → position
  trades: InternalTrade[];
  balance: number;
  seq: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
}

// ─── Math helpers ──────────────────────────────────────────────────────────────

function sma(arr: number[], n: number): number {
  if (!arr.length) return 0;
  const slice = arr.slice(-n);
  return slice.reduce((s, v) => s + v, 0) / slice.length;
}

function ema(arr: number[], n: number): number {
  if (!arr.length) return 0;
  const k = 2 / (n + 1);
  let val = arr[0];
  for (let i = 1; i < arr.length; i++) val = arr[i] * k + val * (1 - k);
  return val;
}

function stdDev(arr: number[], n: number): number {
  const slice = arr.slice(-n);
  if (!slice.length) return 0;
  const m = slice.reduce((s, v) => s + v, 0) / slice.length;
  return Math.sqrt(slice.reduce((s, v) => s + (v - m) ** 2, 0) / slice.length);
}

function rsi(arr: number[], n: number): number {
  if (arr.length < 2) return 50;
  const start = Math.max(1, arr.length - n);
  let gains = 0, losses = 0;
  for (let i = start; i < arr.length; i++) {
    const d = arr[i] - arr[i - 1];
    if (d > 0) gains += d; else losses -= d;
  }
  if (losses === 0) return 100;
  return 100 - 100 / (1 + gains / losses);
}

function clamp(v: number, lo = 0, hi = 99): number {
  return Math.max(lo, Math.min(hi, v));
}

// ─── Option premium model ─────────────────────────────────────────────────────

/**
 * Estimate ATM option premium for a stock.
 * Uses simplified Black-Scholes ATM approximation:
 *   Premium ≈ S × σ × √(T/252) × 0.4
 * where σ=25% annualised, T=7 days to expiry (weekly options).
 */
function estimatePremium(price: number): number {
  const sigma = 0.27; // 27% IV for typical large-cap stock weekly option
  const T = 7 / 252; // 7 trading days to expiry
  const premium = price * sigma * Math.sqrt(T) * 0.4;
  // Clamp to realistic range: 0.3% – 3% of underlying
  return Math.max(price * 0.003, Math.min(price * 0.03, premium));
}

/**
 * Mark option premium to market using delta approximation.
 * ATM delta ≈ 0.5 for CE, −0.5 for PE.
 * Also apply mild Theta decay (0.5% of premium per bar ~ 15s intraday).
 */
function markPremium(
  entryPremium: number,
  entryUnderlying: number,
  currentUnderlying: number,
  optionType: "CE" | "PE",
  barsHeld: number,
): number {
  const delta = 0.5;
  const move = currentUnderlying - entryUnderlying;
  const direction = optionType === "CE" ? 1 : -1;
  const premiumDelta = delta * move * direction;
  // Theta decay: 0.3% of entry premium per bar (very mild for intraday scalping)
  const thetaDecay = entryPremium * 0.003 * barsHeld;
  const marked = entryPremium + premiumDelta - thetaDecay;
  // Floor: option can't go below 5% of entry premium (avoid negative)
  return Math.max(entryPremium * 0.05, marked);
}

// ─── Signal evaluation ─────────────────────────────────────────────────────────

interface SignalInputs {
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
  momentum3: number; // % change over last 3 bars
  momentum6: number; // % change over last 6 bars
}

function buildSignalInputs(bars: number[]): SignalInputs {
  const n = bars.length;
  const price = bars[n - 1];
  const prevBars = bars.slice(0, -1);
  const breakoutWindow = prevBars.length > 0 ? prevBars.slice(-20) : bars.slice(-20);

  return {
    price,
    prevPrice: n > 1 ? bars[n - 2] : price,
    fast: ema(bars, 5),
    slow: ema(bars, 13),
    trend: ema(bars, 21),
    prevFast: ema(prevBars, 5),
    prevSlow: ema(prevBars, 13),
    mean20: sma(bars, 20),
    std20: stdDev(bars, 20),
    rsi14: rsi(bars, 14),
    // Exclude the active bar so breakout and breakdown checks can trigger.
    high20: Math.max(...breakoutWindow),
    low20: Math.min(...breakoutWindow),
    momentum3: n >= 4 ? ((price - bars[n - 4]) / bars[n - 4]) * 100 : 0,
    momentum6: n >= 7 ? ((price - bars[n - 7]) / bars[n - 7]) * 100 : 0,
  };
}

function evalSignal(signal: string, inp: SignalInputs): number {
  const { price, prevPrice, fast, slow, trend, prevFast, prevSlow,
          mean20, rsi14, high20, low20, momentum3, momentum6 } = inp;

  switch (signal) {
    case "BREAKOUT_CE":
      if (price > high20 * 1.0007 && fast > slow && rsi14 >= 58 && rsi14 <= 76)
        return clamp(70 + (price / high20 - 1) * 2200 + (rsi14 - 58) * 0.35);
      break;

    case "BREAKDOWN_PE":
      if (price < low20 * 0.9993 && fast < slow && rsi14 >= 24 && rsi14 <= 42)
        return clamp(70 + (low20 / price - 1) * 2200 + (42 - rsi14) * 0.35);
      break;

    case "EMA_CROSS_CE":
      if (prevFast <= prevSlow && fast > slow && rsi14 > 52 && price > trend * 1.0004)
        return clamp(72 + (fast / slow - 1) * 3000 + (rsi14 - 50) * 0.2);
      break;

    case "EMA_CROSS_PE":
      if (prevFast >= prevSlow && fast < slow && rsi14 < 48 && price < trend * 0.9996)
        return clamp(72 + (slow / fast - 1) * 3000 + (50 - rsi14) * 0.2);
      break;

    case "RSI_BOUNCE_CE":
      if (rsi14 < 26 && price >= prevPrice && momentum3 > -0.35)
        return clamp(67 + (28 - rsi14) * 1.4);
      break;

    case "RSI_FADE_PE":
      if (rsi14 > 74 && price <= prevPrice && momentum3 < 0.35)
        return clamp(67 + (rsi14 - 72) * 1.4);
      break;

    case "VWAP_RECLAIM_CE":
      if (price > mean20 * 1.0008 && prevPrice <= mean20 * 1.0015 && rsi14 > 52 && momentum3 > 0.12)
        return clamp(66 + (price / mean20 - 1) * 1400 + momentum3 * 4);
      break;

    case "VWAP_REJECT_PE":
      if (price < mean20 * 0.9992 && prevPrice >= mean20 * 0.9985 && rsi14 < 48 && momentum3 < -0.12)
        return clamp(66 + (mean20 / price - 1) * 1400 + Math.abs(momentum3) * 4);
      break;

    case "EXHAUSTION_CE":
      if (momentum3 <= -1.5 && rsi14 < 24 && price > low20 * 1.0006)
        return clamp(70 + Math.abs(momentum3) * 9 + (26 - rsi14) * 1.4);
      break;

    case "EXHAUSTION_PE":
      if (momentum3 >= 1.5 && rsi14 > 76 && price < high20 * 0.9994)
        return clamp(70 + Math.abs(momentum3) * 9 + (rsi14 - 74) * 1.4);
      break;

    case "TREND_CONT_CE":
      if (fast > slow && slow > trend && momentum6 > 0.95 && rsi14 >= 56 && rsi14 <= 72)
        return clamp(74 + momentum6 * 7 + (rsi14 - 54) * 0.25);
      break;

    case "TREND_CONT_PE":
      if (fast < slow && slow < trend && momentum6 < -0.95 && rsi14 >= 28 && rsi14 <= 44)
        return clamp(74 + Math.abs(momentum6) * 7 + (46 - rsi14) * 0.25);
      break;
  }

  return 0;
}

// ─── Engine helper: compute lot shares from allocation ────────────────────────

function classifyStockRegime(inp: SignalInputs): string {
  const trendGapPct = inp.price > 0 ? Math.abs(inp.fast - inp.slow) / inp.price * 100 : 0;
  const volPct = inp.price > 0 ? inp.std20 / inp.price * 100 : 0;

  if (inp.fast > inp.slow && inp.slow > inp.trend && inp.momentum6 > 0.45 && inp.rsi14 >= 54) return "TRENDING_BULL";
  if (inp.fast < inp.slow && inp.slow < inp.trend && inp.momentum6 < -0.45 && inp.rsi14 <= 46) return "TRENDING_BEAR";
  if (volPct >= 0.9 || trendGapPct >= 0.7) return "HIGH_VOL";
  return "RANGE";
}

function isCategoryAlignedWithRegime(category: string, regime: string): boolean {
  switch (regime) {
    case "TRENDING_BULL":
    case "TRENDING_BEAR":
      return ["Breakout", "Momentum", "Trend", "VWAP", "Price Action"].includes(category);
    case "HIGH_VOL":
      return ["Breakout", "Momentum", "Price Action", "Trend"].includes(category);
    case "RANGE":
      // Allow all in range — momentum/breakout still fire on intraday swings
      return true;
    default:
      return true;
  }
}

function passesEntryConfirmation(def: StratDef, inp: SignalInputs, regime: string): boolean {
  if (!isCategoryAlignedWithRegime(def.category, regime)) return false;

  const bullish = def.optionType === "CE";
  switch (def.category) {
    case "Breakout":
    case "Momentum":
    case "Trend":
      if (regime === "TRENDING_BULL" || regime === "TRENDING_BEAR") {
        return bullish
          ? inp.price >= inp.fast && inp.fast >= inp.slow && inp.momentum3 > 0.06
          : inp.price <= inp.fast && inp.fast <= inp.slow && inp.momentum3 < -0.06;
      }
      // In range/unknown just need price direction
      return bullish ? inp.momentum3 > 0.05 : inp.momentum3 < -0.05;
    case "VWAP":
      return bullish
        ? inp.price >= inp.mean20 && inp.rsi14 >= 48
        : inp.price <= inp.mean20 && inp.rsi14 <= 52;
    case "Mean Reversion":
      return bullish
        ? inp.rsi14 <= 44 && inp.price >= inp.prevPrice
        : inp.rsi14 >= 56 && inp.price <= inp.prevPrice;
    case "Price Action":
      return bullish
        ? inp.price >= inp.prevPrice
        : inp.price <= inp.prevPrice;
    default:
      return true;
  }
}

function resolveExit(
  pos: InternalPosition,
  def: StratDef,
  currentPremium: number,
  now: number,
): { reason: string; exitPremium: number } | null {
  const gainPct = pos.entryPremium > 0 ? (currentPremium - pos.entryPremium) / pos.entryPremium : 0;
  const maxHoldMs = holdMinutesFor(def) * 60 * 1000;
  const timeProgress = maxHoldMs > 0 ? Math.min(1, (now - pos.entryTime) / maxHoldMs) : 0;
  const profitLockThreshold = Math.max(STOCK_LATE_EXIT_MIN_GAIN, (def.tpPct / 100) * STOCK_PROFIT_LOCK_SHARE);

  if (currentPremium >= pos.tpPremium) return { reason: "TP", exitPremium: pos.tpPremium };
  if (currentPremium <= pos.slPremium) return { reason: "SL", exitPremium: pos.slPremium };
  if (timeProgress >= STOCK_GRIND_EXIT_PROGRESS && gainPct >= Math.max(STOCK_LATE_EXIT_MIN_GAIN, (def.tpPct / 100) * STOCK_GRIND_EXIT_SHARE)) {
    return { reason: "PROFIT_LOCK", exitPremium: currentPremium };
  }
  if (timeProgress >= STOCK_PROFIT_LOCK_PROGRESS && gainPct >= profitLockThreshold) {
    return { reason: "PROFIT_LOCK", exitPremium: currentPremium };
  }
  if (pos.peakGainPct >= Math.max(STOCK_LATE_EXIT_MIN_GAIN, (def.tpPct / 100) * 0.4) && gainPct > 0 && gainPct <= pos.peakGainPct * (1 - STOCK_TRAIL_GIVEBACK_SHARE)) {
    return { reason: "TRAIL_STOP", exitPremium: currentPremium };
  }
  if (timeProgress >= STOCK_LATE_EXIT_PROGRESS && gainPct >= STOCK_LATE_EXIT_MIN_GAIN) {
    return { reason: "LATE_EXIT", exitPremium: currentPremium };
  }
  if (maxHoldMs > 0 && now - pos.entryTime >= maxHoldMs) {
    return { reason: "TIME_EXIT", exitPremium: currentPremium };
  }
  return null;
}

function computeShares(premium: number, lotSize: number): number {
  if (premium <= 0 || lotSize <= 0) return lotSize;
  const costPerLot = premium * lotSize;
  const numLots = Math.max(1, Math.floor(ALLOCATION_INR / costPerLot));
  return numLots * lotSize;
}

function stockSizeMultiplier(strat: InternalStratState): number {
  let multiplier = 1;
  if (strat.totalTrades >= 6 && strat.winRate >= 54) multiplier += 0.15;
  if (strat.totalTrades >= 12 && strat.winRate >= 58) multiplier += 0.2;
  if (strat.totalTrades > 0) {
    const avgPnlRatio = strat.totalPnl / (strat.totalTrades * ALLOCATION_INR);
    if (avgPnlRatio > 0.08) multiplier += 0.18;
    if (avgPnlRatio < -0.04) multiplier -= 0.15;
  }
  multiplier -= strat.consecutiveLosses * 0.1;
  return stockClamp(multiplier, STOCK_MIN_SIZE_MULTIPLIER, STOCK_MAX_SIZE_MULTIPLIER);
}

function stockCooldownMs(strat: InternalStratState, won: boolean): number {
  const base = strat.def.cooldownMinutes * 60 * 1000;
  if (won) return base;
  return Math.round(base * (1 + strat.consecutiveLosses * STOCK_LOSS_COOLDOWN_PENALTY));
}

function shouldPauseStockStrategy(strat: InternalStratState): boolean {
  return strat.totalTrades >= STOCK_UNDERPERFORMING_MIN_TRADES && strat.totalPnl < 0 && strat.winRate < STOCK_UNDERPERFORMING_MAX_WINRATE;
}

// ─── Engine: close a position ──────────────────────────────────────────────────

function closePosition(
  eng: EngineRef,
  strat: InternalStratState,
  exitPremium: number,
  exitUnderlying: number,
  reason: string,
  now: number,
): void {
  const pos = strat.position;
  if (!pos) return;

  const netPnl = (exitPremium - pos.entryPremium) * pos.numShares;
  const returnPct = ((exitPremium - pos.entryPremium) / pos.entryPremium) * 100;
  const holdSeconds = Math.round((now - pos.entryTime) / 1000);

  const trade: InternalTrade = {
    id: pos.id,
    strategyId: pos.strategyId,
    strategyName: pos.strategyName,
    symbol: pos.stock.symbol,
    optionType: pos.optionType,
    numShares: pos.numShares,
    entryUnderlying: pos.entryUnderlying,
    exitUnderlying,
    entryPremium: pos.entryPremium,
    exitPremium,
    netPnl,
    returnPct,
    entryTime: pos.entryTime,
    exitTime: now,
    exitReason: reason,
    holdSeconds,
  };

  eng.trades.unshift(trade);
  if (eng.trades.length > MAX_TRADES) eng.trades.length = MAX_TRADES;

  eng.balance += exitPremium * pos.numShares; // return premium value
  eng.totalRealizedPnl += netPnl;

  strat.totalTrades++;
  if (netPnl >= 0) {
    strat.wins++;
    eng.totalWins++;
    strat.consecutiveLosses = 0;
  } else {
    strat.losses++;
    eng.totalLosses++;
    strat.consecutiveLosses += 1;
  }
  strat.totalPnl += netPnl;
  strat.winRate = strat.totalTrades > 0 ? (strat.wins / strat.totalTrades) * 100 : 0;
  strat.lastTradeAt = now;
  strat.cooldownUntil = now + stockCooldownMs(strat, netPnl >= 0);
  strat.status = "COOLING";
  strat.currentStock = "";

  eng.positions.delete(pos.id);
  strat.position = null;
}

// ─── Engine: open a position ───────────────────────────────────────────────────

function openPosition(
  eng: EngineRef,
  strat: InternalStratState,
  stock: Nifty50Stock,
  underlying: number,
  now: number,
): boolean {
  eng.seq++;
  const premium = estimatePremium(underlying);
  const numShares = Math.max(stock.lotSize, Math.round(computeShares(premium, stock.lotSize) * stockSizeMultiplier(strat) / stock.lotSize) * stock.lotSize);
  const cost = premium * numShares;

  if (eng.balance < cost) return false; // insufficient capital

  eng.balance -= cost;

  const id = `NSTK-${Date.now()}-${eng.seq}`;
  const tpPremium = premium * (1 + strat.def.tpPct / 100);
  const slPremium = premium * (1 - strat.def.slPct / 100);

  const pos: InternalPosition = {
    id,
    strategyId: strat.def.id,
    strategyName: strat.def.name,
    stock,
    optionType: strat.def.optionType,
    entryUnderlying: underlying,
    currentUnderlying: underlying,
    entryPremium: premium,
    currentPremium: premium,
    tpPremium,
    slPremium,
    numShares,
    initialCost: cost,
    entryTime: now,
    unrealizedPnl: 0,
    returnPct: 0,
    peakGainPct: 0,
  };

  strat.position = pos;
  strat.status = "IN_POSITION";
  strat.currentStock = stock.symbol;
  eng.positions.set(id, pos);
  return true;
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

const EMPTY_STATS: StocksEngineStats = {
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
  lastUpdateAt: 0,
};

function initEngine(): EngineRef {
  return {
    bars: {},
    quotes: {},
    strategies: STRAT_DEFS.map((def) => ({
      def,
      position: null,
      status: "WARMING",
      cooldownUntil: 0,
      score: 0,
      currentStock: "",
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
      consecutiveLosses: 0,
      lastTradeAt: 0,
    })),
    positions: new Map(),
    trades: [],
    balance: INITIAL_BALANCE,
    seq: 0,
    totalWins: 0,
    totalLosses: 0,
    totalRealizedPnl: 0,
  };
}

// ─── DB persistence ───────────────────────────────────────────────────────────

type StocksDbPayload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  tradeSeq: number;
  trades: InternalTrade[];
  strategies: Array<{
    id: number; totalTrades: number; wins: number;
    losses: number; totalPnl: number; lastTradeAt: number;
  }>;
};

async function saveStocksState(eng: EngineRef): Promise<void> {
  try {
    const payload: StocksDbPayload = {
      balance:    eng.balance,
      totalWins:  eng.totalWins,
      totalLosses: eng.totalLosses,
      totalPnl:   eng.totalRealizedPnl,
      tradeSeq:   eng.seq,
      trades:     eng.trades.slice(0, 500),
      strategies: eng.strategies.map((s) => ({
        id: s.def.id, totalTrades: s.totalTrades, wins: s.wins,
        losses: s.losses, totalPnl: s.totalPnl, lastTradeAt: s.lastTradeAt,
      })),
    };
    await fetch("/api/nifty/stocks-state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  } catch { /* non-critical */ }
}

async function loadStocksState(eng: EngineRef): Promise<boolean> {
  try {
    const res = await fetch("/api/nifty/stocks-state");
    if (!res.ok) return false;
    const data = await res.json() as { ok: boolean; found?: boolean; disabled?: boolean; state?: StocksDbPayload };
    if (!data.ok) return false;
    if (data.disabled) return false;
    if (!data.found || !data.state) return true;

    const s = data.state;
    eng.balance          = s.balance;
    eng.totalWins        = s.totalWins;
    eng.totalLosses      = s.totalLosses;
    eng.totalRealizedPnl = s.totalPnl;
    eng.seq              = s.tradeSeq;
    eng.trades           = s.trades ?? [];
    for (const saved of (s.strategies ?? [])) {
      const strat = eng.strategies.find((st) => st.def.id === saved.id);
      if (!strat) continue;
      strat.totalTrades = saved.totalTrades;
      strat.wins        = saved.wins;
      strat.losses      = saved.losses;
      strat.totalPnl    = saved.totalPnl;
      strat.lastTradeAt = saved.lastTradeAt;
      strat.winRate     = saved.totalTrades > 0 ? (saved.wins / saved.totalTrades) * 100 : 0;
    }
    return true;
  } catch { return false; }
}

export default function useNiftyStocksEngine() {
  const engRef = useRef<EngineRef>(initEngine());
  const lastSavedTradeCountRef = useRef(-1); // -1 = DB not loaded yet, block saves
  const dbLoadedRef = useRef(false);

  const [state, setState] = useState<StocksEngineState>({
    quotes: NIFTY50_STOCKS.map((s) => ({
      symbol: s.symbol,
      name: s.name,
      sector: s.sector,
      ltp: 0,
      prevClose: 0,
      changePct: 0,
      signalScore: 0,
      hasPosition: false,
    })),
    positions: [],
    trades: [],
    strategies: STRAT_DEFS.map((def) => ({
      id: def.id,
      name: def.name,
      category: def.category,
      optionType: def.optionType,
      status: "WARMING" as const,
      currentStock: "",
      score: 0,
      allocationINR: ALLOCATION_INR,
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
    })),
    stats: EMPTY_STATS,
  });

  const processTick = useCallback((freshQuotes: StockLTPItem[]) => {
    const eng = engRef.current;
    const now = Date.now();

    // ── 1. Update price bars ────────────────────────────────────────────────
    for (const q of freshQuotes) {
      if (q.ltp <= 0) continue;
      eng.quotes[q.symbol] = q;
      if (!eng.bars[q.symbol]) eng.bars[q.symbol] = [];
      const b = eng.bars[q.symbol];
      // Only append if price changed (avoids duplicate bars during halts)
      if (b.length === 0 || b[b.length - 1] !== q.ltp) {
        b.push(q.ltp);
        if (b.length > MAX_BARS) b.shift();
      }
    }

    // ── 2. Mark open positions & check TP/SL ───────────────────────────────
    for (const strat of eng.strategies) {
      if (!strat.position) continue;
      const pos = strat.position;
      const currentBars = eng.bars[pos.stock.symbol];
      if (!currentBars?.length) continue;

      const currentUnderlying = currentBars[currentBars.length - 1];
      const barsHeld = Math.max(1, Math.round((now - pos.entryTime) / POLL_MS));
      const currentPremium = markPremium(
        pos.entryPremium,
        pos.entryUnderlying,
        currentUnderlying,
        pos.optionType,
        barsHeld,
      );

      pos.currentUnderlying = currentUnderlying;
      pos.currentPremium = currentPremium;
      pos.unrealizedPnl = (currentPremium - pos.entryPremium) * pos.numShares;
      pos.returnPct = ((currentPremium - pos.entryPremium) / pos.entryPremium) * 100;
      pos.peakGainPct = Math.max(pos.peakGainPct, pos.returnPct / 100);

      const exit = resolveExit(pos, strat.def, currentPremium, now);
      if (exit) {
        closePosition(eng, strat, exit.exitPremium, currentUnderlying, exit.reason, now);
        continue;
      }
    }

    // ── 3. Count open positions ─────────────────────────────────────────────
    let openCount = 0;
    for (const strat of eng.strategies) {
      if (strat.position) openCount++;
    }

    // ── 4. Evaluate strategies for new entries ─────────────────────────────
    // Build per-stock signal scores for display
    const stockSignalScores: Record<string, number> = {};

    for (const strat of eng.strategies) {
      const now2 = Date.now();

      if (shouldPauseStockStrategy(strat)) {
        strat.status = "COOLING";
        strat.score = 0;
        strat.cooldownUntil = Math.max(strat.cooldownUntil, now2 + STOCK_UNDERPERFORMING_PAUSE_MS);
        continue;
      }

      // Update status
      if (!strat.position) {
        if (strat.status !== "COOLING" || now2 >= strat.cooldownUntil) {
          strat.status = "READY";
        }
      }

      if (strat.status !== "READY") continue;
      if (openCount >= MAX_OPEN_POSITIONS) continue;

      // Scan all stocks for best signal
      let bestScore = 0;
      let bestStock: Nifty50Stock | null = null;

      for (const stock of NIFTY50_STOCKS) {
        const bars = eng.bars[stock.symbol];
        if (!bars || bars.length < strat.def.minBars) continue;

        const inp = buildSignalInputs(bars);
        const score = evalSignal(strat.def.signal, inp);
        const regime = classifyStockRegime(inp);
        const confirmed = score >= SIGNAL_THRESHOLD && passesEntryConfirmation(strat.def, inp, regime);

        // Track per-stock max score
        const displayScore = confirmed ? score : Math.min(score, SIGNAL_THRESHOLD - 1);
        if (displayScore > (stockSignalScores[stock.symbol] ?? 0)) {
          stockSignalScores[stock.symbol] = displayScore;
        }

        if (confirmed && score > bestScore) {
          bestScore = score;
          bestStock = stock;
        }
      }

      strat.score = bestScore;

      if (bestStock && bestScore >= SIGNAL_THRESHOLD) {
        const underlying = eng.bars[bestStock.symbol]?.at(-1) ?? 0;
        if (underlying > 0) {
          if (openPosition(eng, strat, bestStock, underlying, now2)) {
            openCount++;
          }
        }
      }
    }

    // ── 5. Build display state ──────────────────────────────────────────────

    // Which stocks have open positions?
    const stockPositionMap: Record<string, { types: Set<string> }> = {};
    for (const strat of eng.strategies) {
      if (strat.position) {
        const sym = strat.position.stock.symbol;
        if (!stockPositionMap[sym]) stockPositionMap[sym] = { types: new Set() };
        stockPositionMap[sym].types.add(strat.def.optionType);
      }
    }

    const quotes: StockQuoteDisplay[] = NIFTY50_STOCKS.map((s) => {
      const q = eng.quotes[s.symbol];
      const posInfo = stockPositionMap[s.symbol];
      let stratLabel: string | undefined;
      if (posInfo) {
        const types = [...posInfo.types];
        stratLabel = types.join("+");
      }
      return {
        symbol: s.symbol,
        name: s.name,
        sector: s.sector,
        ltp: q?.ltp ?? 0,
        prevClose: q?.close ?? 0,
        changePct: q?.changePct ?? 0,
        signalScore: stockSignalScores[s.symbol] ?? 0,
        hasPosition: !!posInfo,
        strategyLabel: stratLabel,
      };
    });

    const positions: StockOptionPosition[] = [];
    for (const strat of eng.strategies) {
      if (!strat.position) continue;
      const pos = strat.position;
      positions.push({
        id: pos.id,
        strategyId: pos.strategyId,
        strategyName: pos.strategyName,
        symbol: pos.stock.symbol,
        sector: pos.stock.sector,
        optionType: pos.optionType,
        lotSize: pos.stock.lotSize,
        numShares: pos.numShares,
        entryUnderlying: pos.entryUnderlying,
        currentUnderlying: pos.currentUnderlying,
        entryPremium: pos.entryPremium,
        currentPremium: pos.currentPremium,
        tpPremium: pos.tpPremium,
        slPremium: pos.slPremium,
        initialCost: pos.initialCost,
        entryTime: new Date(pos.entryTime).toISOString(),
        unrealizedPnl: pos.unrealizedPnl,
        returnPct: pos.returnPct,
      });
    }

    const strategies: StrategyDisplayStatus[] = eng.strategies.map((s) => ({
      id: s.def.id,
      name: s.def.name,
      category: s.def.category,
      optionType: s.def.optionType,
      status: s.status,
      currentStock: s.currentStock,
      score: s.score,
      allocationINR: ALLOCATION_INR,
      totalTrades: s.totalTrades,
      wins: s.wins,
      losses: s.losses,
      totalPnl: s.totalPnl,
      winRate: s.winRate,
      cooldownUntil: s.cooldownUntil > 0 ? new Date(s.cooldownUntil).toISOString() : undefined,
      lastTradeAt: s.lastTradeAt > 0 ? new Date(s.lastTradeAt).toISOString() : undefined,
    }));

    const trades: StockOptionTrade[] = eng.trades.slice(0, 100).map((t) => ({
      id: t.id,
      strategyId: t.strategyId,
      strategyName: t.strategyName,
      symbol: t.symbol,
      optionType: t.optionType,
      numShares: t.numShares,
      entryUnderlying: t.entryUnderlying,
      exitUnderlying: t.exitUnderlying,
      entryPremium: t.entryPremium,
      exitPremium: t.exitPremium,
      netPnl: t.netPnl,
      returnPct: t.returnPct,
      entryTime: new Date(t.entryTime).toISOString(),
      exitTime: new Date(t.exitTime).toISOString(),
      exitReason: t.exitReason,
      holdSeconds: t.holdSeconds,
    }));

    // Equity = balance + unrealized value of open positions
    const unrealizedPnl = positions.reduce((s, p) => s + p.unrealizedPnl, 0);
    const openPositionCost = positions.reduce((s, p) => s + p.initialCost, 0);
    const equity = eng.balance + openPositionCost + unrealizedPnl;

    const totalStratTrades = eng.strategies.reduce((s, st) => s + st.totalTrades, 0);
    const totalWinRate = (eng.totalWins + eng.totalLosses) > 0
      ? (eng.totalWins / (eng.totalWins + eng.totalLosses)) * 100 : 0;
    const warmingUp = NIFTY50_STOCKS.every(
      (s) => !eng.bars[s.symbol] || eng.bars[s.symbol].length < MIN_BARS_SLOW,
    );

    const stats: StocksEngineStats = {
      equity,
      balance: eng.balance,
      sessionPnl: equity - INITIAL_BALANCE,
      unrealizedPnl,
      realizedPnl: eng.totalRealizedPnl,
      totalTrades: totalStratTrades,
      totalWins: eng.totalWins,
      totalLosses: eng.totalLosses,
      openPositions: openCount,
      winRate: totalWinRate,
      activeStrategies: eng.strategies.filter((s) => s.status !== "WARMING").length,
      warmingUp,
      lastUpdateAt: now,
    };

    setState({ quotes, positions, trades, strategies, stats });

    // Only save after DB has been loaded — avoids overwriting DB with empty state on mount
    if (!dbLoadedRef.current) return;
    const tradeCount = eng.trades.length;
    if (tradeCount !== lastSavedTradeCountRef.current) {
      lastSavedTradeCountRef.current = tradeCount;
      void saveStocksState(eng);
    }
  }, []);

  // ── Load persisted state from DB on mount (only unblock saves after a successful load) ─────
  useEffect(() => {
    void loadStocksState(engRef.current).then((loaded) => {
      dbLoadedRef.current = loaded;
      if (loaded) {
        lastSavedTradeCountRef.current = engRef.current.trades.length;
      }
    });
  }, []);

  // ── Periodic save every 60s ───────────────────────────────────────────────
  useEffect(() => {
    const id = setInterval(() => {
      if (dbLoadedRef.current) void saveStocksState(engRef.current);
    }, 60_000);
    return () => clearInterval(id);
  }, []);

  // ── Save on page close/refresh ────────────────────────────────────────────
  useEffect(() => {
    const onUnload = () => {
      if (!dbLoadedRef.current) return;
      const eng = engRef.current;
      const payload = JSON.stringify({
        balance: eng.balance, totalWins: eng.totalWins, totalLosses: eng.totalLosses,
        totalPnl: eng.totalRealizedPnl, tradeSeq: eng.seq,
        trades: eng.trades.slice(0, 500),
        strategies: eng.strategies.map((s) => ({
          id: s.def.id, totalTrades: s.totalTrades, wins: s.wins,
          losses: s.losses, totalPnl: s.totalPnl, lastTradeAt: s.lastTradeAt,
        })),
      });
      navigator.sendBeacon("/api/nifty/stocks-state", new Blob([payload], { type: "application/json" }));
    };
    window.addEventListener("beforeunload", onUnload);
    return () => window.removeEventListener("beforeunload", onUnload);
  }, []);

  // ── Reset handler ──────────────────────────────────────────────────────────
  const reset = useCallback(() => {
    engRef.current = initEngine();
    lastSavedTradeCountRef.current = 0;
    void saveStocksState(engRef.current);
    setState({
      quotes: NIFTY50_STOCKS.map((s) => ({
        symbol: s.symbol, name: s.name, sector: s.sector,
        ltp: 0, prevClose: 0, changePct: 0, signalScore: 0, hasPosition: false,
      })),
      positions: [],
      trades: [],
      strategies: STRAT_DEFS.map((def) => ({
        id: def.id, name: def.name, category: def.category, optionType: def.optionType,
        status: "WARMING" as const, currentStock: "", score: 0, allocationINR: ALLOCATION_INR,
        totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0,
      })),
      stats: EMPTY_STATS,
    });
  }, []);

  // ── Polling loop ───────────────────────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;

    const tick = async () => {
      if (cancelled) return;
      try {
        const res = await fetch("/api/stocks/ltp", { next: { revalidate: 0 } } as RequestInit);
        if (!res.ok) return;
        const data = await res.json() as { ok: boolean; stocks: StockLTPItem[] };
        if (data.ok && data.stocks?.length) {
          processTick(data.stocks);
        }
      } catch {
        // silent — keep engine running even if a fetch fails
      }
    };

    // First tick immediately, then every POLL_MS
    void tick();
    const interval = setInterval(() => void tick(), POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [processTick]);

  return { ...state, reset };
}
