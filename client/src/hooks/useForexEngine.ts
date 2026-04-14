"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { FOREX_PAIRS, type ForexPair } from "@/lib/forexPairs";
import type { ForexMarketItem } from "@/app/api/forex/markets/route";

// ── Constants ────────────────────────────────────────────────────────────────
const INITIAL_BALANCE   = 1_000_000;
const MAX_OPEN_POSITIONS = 12;
const MAX_BARS           = 120;
const MIN_BARS_FAST      = 18;
const MIN_BARS_SLOW      = 28;
const SIGNAL_THRESHOLD   = 61;
const POLL_MS            = 5_000;   // 5s — Yahoo Finance rate limit friendly
const MAX_TRADES         = 500;
const ALLOCATION_USD     = 15_000;  // larger notional for forex (tight moves)
const PROFIT_LOCK_PROGRESS = 0.28;
const PROFIT_LOCK_SHARE    = 0.40;
const LATE_EXIT_PROGRESS   = 0.55;
const LATE_EXIT_MIN_GAIN   = 0.03;
const GRIND_EXIT_PROGRESS  = 0.42;
const GRIND_EXIT_SHARE     = 0.20;
const TRAIL_ACTIVATION_PCT = 0.25;  // forex moves are small — activate trail early
const TRAIL_GIVEBACK_SHARE = 0.30;
const MIN_SIZE_MULTIPLIER  = 0.5;
const MAX_SIZE_MULTIPLIER  = 2.0;
const LOSS_COOLDOWN_PENALTY = 0.35;
const UNDERPERFORMING_PAUSE_MS = 90 * 60 * 1000;

// ── Types ────────────────────────────────────────────────────────────────────
type Side   = "LONG" | "SHORT";
type Status = "WARMING" | "READY" | "IN_POSITION" | "COOLING";

type SignalInputs = {
  price: number; prevPrice: number;
  fast: number; slow: number; trend: number;
  prevFast: number; prevSlow: number;
  mean20: number; std20: number; rsi14: number;
  high20: number; low20: number;
  momentum3: number; momentum6: number;
};

interface StratDef {
  id: number; name: string; category: string; side: Side;
  signal: string; tpPct: number; slPct: number;
  cooldownMinutes: number; minBars: number; holdMinutes: number;
}

interface InternalPosition {
  id: string; strategyId: number; strategyName: string;
  pair: ForexPair; side: Side;
  entryPrice: number; currentPrice: number;
  tpPrice: number; slPrice: number;
  quantity: number; notional: number; entryTime: number;
  unrealizedPnl: number; returnPct: number; peakReturnPct: number;
}

interface InternalTrade {
  id: string; strategyId: number; strategyName: string;
  symbol: string; side: Side; quantity: number;
  entryPrice: number; exitPrice: number;
  netPnl: number; returnPct: number;
  entryTime: number; exitTime: number;
  exitReason: string; holdSeconds: number;
}

interface InternalStrategyState {
  def: StratDef; position: InternalPosition | null;
  status: Status; cooldownUntil: number; score: number;
  currentSymbol: string; lastSignalSymbol: string;
  totalTrades: number; wins: number; losses: number;
  totalPnl: number; winRate: number; consecutiveLosses: number;
}

interface EngineRef {
  bars: Record<string, number[]>;
  quotes: Record<string, ForexMarketItem>;
  strategies: InternalStrategyState[];
  positions: Map<string, InternalPosition>;
  trades: InternalTrade[];
  balance: number; seq: number;
  totalWins: number; totalLosses: number; totalRealizedPnl: number;
  lastError: string;
}

// ── Public output types ───────────────────────────────────────────────────────
export type ForexQuoteDisplay = {
  symbol: string; category: string;
  ltp: number; changePct: number;
  signalScore: number; hasPosition: boolean;
  strategyLabel?: string; sparkline: number[];
};

export type ForexPosition = {
  id: string; strategyId: number; strategyName: string;
  symbol: string; category: string; side: Side;
  quantity: number; entryPrice: number; currentPrice: number;
  tpPrice: number; slPrice: number; notional: number;
  entryTime: string; unrealizedPnl: number; returnPct: number;
};

export type ForexTrade = {
  id: string; strategyId: number; strategyName: string;
  symbol: string; side: Side; quantity: number;
  entryPrice: number; exitPrice: number;
  netPnl: number; returnPct: number;
  entryTime: string; exitTime: string;
  exitReason: string; holdSeconds: number;
};

export type ForexStrategyStatus = {
  id: number; name: string; category: string; side: Side;
  status: Status; currentSymbol: string; score: number;
  allocationUSD: number; totalTrades: number; wins: number;
  losses: number; totalPnl: number; winRate: number;
  cooldownUntil?: string;
};

export type ForexEngineStats = {
  equity: number; balance: number; sessionPnl: number;
  unrealizedPnl: number; realizedPnl: number;
  totalTrades: number; openPositions: number; winRate: number;
  activeStrategies: number; warmingUp: boolean;
  liveSymbols: number; lastUpdateAt: number; diagnostics: string;
};

type ForexDbPosition = {
  id: string; strategyId: number; symbol: string; side: Side;
  entryPrice: number; currentPrice: number; tpPrice: number; slPrice: number;
  quantity: number; notional: number; entryTime: number;
  unrealizedPnl: number; returnPct: number; peakReturnPct: number;
};

type ForexDbTrade = {
  id: string; strategyId: number; strategyName: string;
  symbol: string; side: Side; quantity: number;
  entryPrice: number; exitPrice: number; netPnl: number; returnPct: number;
  entryTime: number; exitTime: number; exitReason: string; holdSeconds: number;
};

type ForexDbStrategy = {
  id: number; totalTrades: number; wins: number; losses: number;
  totalPnl: number; winRate: number; cooldownUntil: number; consecutiveLosses: number;
};

type ForexDbPayload = {
  balance: number; totalWins: number; totalLosses: number; totalPnl: number; tradeSeq: number;
  positions: ForexDbPosition[]; trades: ForexDbTrade[]; strategies: ForexDbStrategy[];
};

// ── Strategy definitions ──────────────────────────────────────────────────────
type CategoryProfile = { minTp: number; maxTp: number; minSl: number; maxSl: number; holdMins: number };

const CATEGORY_PROFILES: Record<string, CategoryProfile> = {
  Momentum:        { minTp: 0.30, maxTp: 0.65, minSl: 0.14, maxSl: 0.28, holdMins: 90  },
  Breakout:        { minTp: 0.35, maxTp: 0.75, minSl: 0.15, maxSl: 0.30, holdMins: 100 },
  Trend:           { minTp: 0.35, maxTp: 0.70, minSl: 0.14, maxSl: 0.28, holdMins: 110 },
  "Mean Reversion":{ minTp: 0.22, maxTp: 0.45, minSl: 0.12, maxSl: 0.22, holdMins: 70  },
  VWAP:            { minTp: 0.25, maxTp: 0.50, minSl: 0.12, maxSl: 0.22, holdMins: 80  },
};

const BASE_SIGNALS = [
  { signal: "BREAKOUT",     category: "Breakout",        longName: "FX_Range_Breakout",   shortName: "FX_Range_Breakdown",     tpPct: 0.55, slPct: 0.22, cooldownMinutes: 14, minBars: MIN_BARS_SLOW },
  { signal: "EMA_CROSS",    category: "Momentum",        longName: "FX_EMA_Impulse",      shortName: "FX_EMA_Fade",            tpPct: 0.48, slPct: 0.20, cooldownMinutes: 11, minBars: MIN_BARS_SLOW },
  { signal: "RSI_BOUNCE",   category: "Mean Reversion",  longName: "FX_RSI_Reclaim",      shortName: "FX_RSI_Fade",            tpPct: 0.36, slPct: 0.16, cooldownMinutes: 10, minBars: MIN_BARS_FAST },
  { signal: "VWAP_RECLAIM", category: "VWAP",            longName: "FX_VWAP_Reclaim",     shortName: "FX_VWAP_Reject",         tpPct: 0.38, slPct: 0.16, cooldownMinutes: 10, minBars: MIN_BARS_SLOW },
  { signal: "TREND_CONT",   category: "Trend",           longName: "FX_Trend_Continuation",shortName: "FX_Trend_Reversal",     tpPct: 0.52, slPct: 0.20, cooldownMinutes: 16, minBars: MIN_BARS_SLOW },
];

const VARIANTS = [
  { suffix: "Scalp", tpBump: -0.04, slBump: -0.02, cooldown: -2 },
  { suffix: "Pulse", tpBump: -0.01, slBump:  0.00, cooldown: -1 },
  { suffix: "Core",  tpBump:  0.00, slBump:  0.00, cooldown:  0 },
  { suffix: "Flow",  tpBump:  0.02, slBump:  0.01, cooldown:  1 },
  { suffix: "Pro",   tpBump:  0.04, slBump:  0.01, cooldown:  2 },
];

function clamp(v: number, lo: number, hi: number) { return Math.max(lo, Math.min(hi, v)); }
function profileFor(cat: string): CategoryProfile {
  return CATEGORY_PROFILES[cat] ?? { minTp: 0.25, maxTp: 0.55, minSl: 0.12, maxSl: 0.25, holdMins: 85 };
}
function tuneStrategy(def: StratDef): StratDef {
  const p = profileFor(def.category);
  return { ...def, tpPct: clamp(def.tpPct, p.minTp, p.maxTp), slPct: clamp(def.slPct, p.minSl, p.maxSl), holdMinutes: p.holdMins };
}

const STRAT_DEFS: StratDef[] = (() => {
  const defs: StratDef[] = [];
  let id = 1;
  for (const base of BASE_SIGNALS) {
    for (const v of VARIANTS) {
      defs.push(tuneStrategy({ id: id++, name: `${base.longName}_${v.suffix}_LONG`,  category: base.category, side: "LONG",  signal: base.signal,          tpPct: base.tpPct + v.tpBump, slPct: base.slPct + v.slBump, cooldownMinutes: Math.max(6, base.cooldownMinutes + v.cooldown), minBars: base.minBars, holdMinutes: profileFor(base.category).holdMins }));
      defs.push(tuneStrategy({ id: id++, name: `${base.shortName}_${v.suffix}_SHORT`, category: base.category, side: "SHORT", signal: `${base.signal}_SHORT`, tpPct: base.tpPct + v.tpBump, slPct: base.slPct + v.slBump, cooldownMinutes: Math.max(6, base.cooldownMinutes + v.cooldown), minBars: base.minBars, holdMinutes: profileFor(base.category).holdMins }));
    }
  }
  return defs;
})();

const FOREX_PAIR_BY_SYMBOL = new Map(FOREX_PAIRS.map((pair) => [pair.symbol, pair]));

// ── Indicator helpers ─────────────────────────────────────────────────────────
function sma(values: number[], period: number): number {
  const s = values.slice(-period);
  return s.length ? s.reduce((a, b) => a + b, 0) / s.length : 0;
}
function ema(values: number[], period: number): number {
  if (!values.length) return 0;
  const k = 2 / (period + 1);
  let cur = values[0];
  for (let i = 1; i < values.length; i++) cur = values[i] * k + cur * (1 - k);
  return cur;
}
function stdDev(values: number[], period: number): number {
  const s = values.slice(-period);
  if (!s.length) return 0;
  const avg = s.reduce((a, b) => a + b, 0) / s.length;
  return Math.sqrt(s.reduce((a, b) => a + (b - avg) ** 2, 0) / s.length);
}
function rsi(values: number[], period: number): number {
  if (values.length < 2) return 50;
  const start = Math.max(1, values.length - period);
  let gains = 0, losses = 0;
  for (let i = start; i < values.length; i++) {
    const d = values[i] - values[i - 1];
    if (d > 0) gains += d; else losses -= d;
  }
  if (losses === 0) return 100;
  return 100 - 100 / (1 + gains / losses);
}
function scoreClamp(v: number) { return clamp(v, 0, 99); }

function buildSignalInputs(bars: number[]): SignalInputs {
  const last = bars.length - 1;
  const price = bars[last];
  const prev = bars.slice(0, -1);
  return {
    price, prevPrice: last > 0 ? bars[last - 1] : price,
    fast: ema(bars, 6), slow: ema(bars, 15), trend: ema(bars, 28),
    prevFast: ema(prev, 6), prevSlow: ema(prev, 15),
    mean20: sma(bars, 20), std20: stdDev(bars, 20), rsi14: rsi(bars, 14),
    high20: Math.max(...bars.slice(-20)), low20: Math.min(...bars.slice(-20)),
    momentum3: last >= 3 ? ((price - bars[last - 3]) / bars[last - 3]) * 100 : 0,
    momentum6: last >= 6 ? ((price - bars[last - 6]) / bars[last - 6]) * 100 : 0,
  };
}

function evalSignal(signal: string, input: SignalInputs): number {
  const { price, prevPrice, fast, slow, trend, prevFast, prevSlow, mean20, rsi14, high20, low20, momentum3, momentum6 } = input;
  // Forex uses tighter thresholds (smaller % moves)
  switch (signal) {
    case "BREAKOUT":
      return price > high20 * 1.0003 && fast > slow && rsi14 >= 56 && momentum3 > 0.05
        ? scoreClamp(72 + (price / high20 - 1) * 8000 + momentum3 * 15) : 0;
    case "BREAKOUT_SHORT":
      return price < low20 * 0.9997 && fast < slow && rsi14 <= 44 && momentum3 < -0.05
        ? scoreClamp(72 + (low20 / price - 1) * 8000 + Math.abs(momentum3) * 15) : 0;
    case "EMA_CROSS":
      return prevFast <= prevSlow && fast > slow && price > trend && rsi14 >= 53
        ? scoreClamp(70 + (fast / slow - 1) * 12000 + (rsi14 - 50) * 0.4) : 0;
    case "EMA_CROSS_SHORT":
      return prevFast >= prevSlow && fast < slow && price < trend && rsi14 <= 47
        ? scoreClamp(70 + (slow / fast - 1) * 12000 + (50 - rsi14) * 0.4) : 0;
    case "RSI_BOUNCE":
      return rsi14 <= 32 && price >= prevPrice && momentum3 > -0.15
        ? scoreClamp(67 + (34 - rsi14) * 1.4) : 0;
    case "RSI_BOUNCE_SHORT":
      return rsi14 >= 68 && price <= prevPrice && momentum3 < 0.15
        ? scoreClamp(67 + (rsi14 - 66) * 1.4) : 0;
    case "VWAP_RECLAIM":
      return price > mean20 * 1.0002 && prevPrice <= mean20 * 1.0003 && momentum3 > 0.04
        ? scoreClamp(68 + (price / mean20 - 1) * 5500 + momentum3 * 12) : 0;
    case "VWAP_RECLAIM_SHORT":
      return price < mean20 * 0.9998 && prevPrice >= mean20 * 0.9997 && momentum3 < -0.04
        ? scoreClamp(68 + (mean20 / price - 1) * 5500 + Math.abs(momentum3) * 12) : 0;
    case "TREND_CONT":
      return fast > slow && slow > trend && momentum6 > 0.12 && rsi14 >= 55 && rsi14 <= 75
        ? scoreClamp(73 + momentum6 * 25 + (rsi14 - 54) * 0.3) : 0;
    case "TREND_CONT_SHORT":
      return fast < slow && slow < trend && momentum6 < -0.12 && rsi14 >= 25 && rsi14 <= 45
        ? scoreClamp(73 + Math.abs(momentum6) * 25 + (46 - rsi14) * 0.3) : 0;
    default: return 0;
  }
}

function classifyRegime(input: SignalInputs): string {
  const trendGapPct = input.price > 0 ? Math.abs(input.fast - input.slow) / input.price * 100 : 0;
  const volPct = input.price > 0 ? input.std20 / input.price * 100 : 0;
  if (input.fast > input.slow && input.slow > input.trend && input.momentum6 > 0.08) return "TRENDING_BULL";
  if (input.fast < input.slow && input.slow < input.trend && input.momentum6 < -0.08) return "TRENDING_BEAR";
  if (volPct >= 0.15 || trendGapPct >= 0.10) return "HIGH_VOL";
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
    if (def.category === "VWAP") return input.price >= input.mean20 && input.rsi14 >= 49;
    if (def.category === "Mean Reversion") return input.rsi14 <= 45 && input.price >= input.prevPrice;
    return input.price >= input.fast && input.fast >= input.slow && input.momentum3 > 0.02;
  }
  if (def.category === "VWAP") return input.price <= input.mean20 && input.rsi14 <= 51;
  if (def.category === "Mean Reversion") return input.rsi14 >= 55 && input.price <= input.prevPrice;
  return input.price <= input.fast && input.fast <= input.slow && input.momentum3 < -0.02;
}

function calcPnl(side: Side, entry: number, current: number, qty: number): number {
  return (current - entry) * qty * (side === "LONG" ? 1 : -1);
}

function sizeMultiplierFor(strategy: InternalStrategyState): number {
  let m = 1;
  if (strategy.totalTrades >= 6  && strategy.winRate >= 55) m += 0.20;
  if (strategy.totalTrades >= 12 && strategy.winRate >= 60) m += 0.25;
  if (strategy.totalTrades >= 20 && strategy.winRate >= 65) m += 0.30;
  if (strategy.totalTrades > 0) {
    const avg = strategy.totalPnl / (strategy.totalTrades * ALLOCATION_USD);
    if (avg > 0.025) m += 0.25;
    if (avg > 0.05)  m += 0.20;
    if (avg < -0.015) m -= 0.15;
  }
  m -= strategy.consecutiveLosses * LOSS_COOLDOWN_PENALTY * 0.2;
  return clamp(m, MIN_SIZE_MULTIPLIER, MAX_SIZE_MULTIPLIER);
}

function cooldownMsFor(strategy: InternalStrategyState, won: boolean): number {
  const base = strategy.def.cooldownMinutes * 60 * 1000;
  return won ? base : Math.round(base * (1 + strategy.consecutiveLosses * LOSS_COOLDOWN_PENALTY));
}

function resolveExit(pos: InternalPosition, def: StratDef, price: number, now: number): { reason: string; exitPrice: number } | null {
  const returnPct = pos.notional > 0 ? (calcPnl(pos.side, pos.entryPrice, price, pos.quantity) / pos.notional) * 100 : 0;
  const maxHoldMs = def.holdMinutes * 60 * 1000;
  const progress = maxHoldMs > 0 ? Math.min(1, (now - pos.entryTime) / maxHoldMs) : 0;
  const lockThreshold = Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * PROFIT_LOCK_SHARE);
  if (pos.side === "LONG") {
    if (price >= pos.tpPrice) return { reason: "TP", exitPrice: pos.tpPrice };
    if (price <= pos.slPrice) return { reason: "SL", exitPrice: pos.slPrice };
  } else {
    if (price <= pos.tpPrice) return { reason: "TP", exitPrice: pos.tpPrice };
    if (price >= pos.slPrice) return { reason: "SL", exitPrice: pos.slPrice };
  }
  if (progress >= GRIND_EXIT_PROGRESS && returnPct >= Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * GRIND_EXIT_SHARE)) return { reason: "PROFIT_LOCK", exitPrice: price };
  if (progress >= PROFIT_LOCK_PROGRESS && returnPct >= lockThreshold) return { reason: "PROFIT_LOCK", exitPrice: price };
  if (pos.peakReturnPct >= Math.max(TRAIL_ACTIVATION_PCT, def.tpPct * 0.4) && returnPct > 0 && returnPct <= pos.peakReturnPct * (1 - TRAIL_GIVEBACK_SHARE)) return { reason: "TRAIL_STOP", exitPrice: price };
  if (progress >= LATE_EXIT_PROGRESS && returnPct >= LATE_EXIT_MIN_GAIN) return { reason: "LATE_EXIT", exitPrice: price };
  if (maxHoldMs > 0 && now - pos.entryTime >= maxHoldMs) return { reason: "TIME_EXIT", exitPrice: price };
  return null;
}

function initEngine(): EngineRef {
  return {
    bars: {}, quotes: {},
    strategies: STRAT_DEFS.map((def) => ({ def, position: null, status: "WARMING", cooldownUntil: 0, score: 0, currentSymbol: "", lastSignalSymbol: "", totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0, consecutiveLosses: 0 })),
    positions: new Map(), trades: [],
    balance: INITIAL_BALANCE, seq: 0,
    totalWins: 0, totalLosses: 0, totalRealizedPnl: 0, lastError: "",
  };
}

function openPosition(engine: EngineRef, strategy: InternalStrategyState, pair: ForexPair, entryPrice: number, now: number): boolean {
  const quantity = entryPrice > 0 ? Math.max(1, ALLOCATION_USD / entryPrice) * sizeMultiplierFor(strategy) : 0;
  const notional = quantity * entryPrice;
  if (notional <= 0 || engine.balance < notional) return false;
  engine.seq++;
  engine.balance -= notional;
  const tpMul = 1 + strategy.def.tpPct / 100;
  const slMul = 1 - strategy.def.slPct / 100;
  const pos: InternalPosition = {
    id: `FX-${Date.now()}-${engine.seq}`,
    strategyId: strategy.def.id, strategyName: strategy.def.name,
    pair, side: strategy.def.side,
    entryPrice, currentPrice: entryPrice,
    tpPrice: strategy.def.side === "LONG" ? entryPrice * tpMul : entryPrice * (2 - tpMul),
    slPrice: strategy.def.side === "LONG" ? entryPrice * slMul : entryPrice * (2 - slMul),
    quantity, notional, entryTime: now,
    unrealizedPnl: 0, returnPct: 0, peakReturnPct: 0,
  };
  strategy.position = pos;
  strategy.status = "IN_POSITION";
  strategy.currentSymbol = pair.symbol;
  engine.positions.set(pos.id, pos);
  return true;
}

function closePosition(engine: EngineRef, strategy: InternalStrategyState, exitPrice: number, reason: string, now: number) {
  const pos = strategy.position;
  if (!pos) return;
  const netPnl = calcPnl(pos.side, pos.entryPrice, exitPrice, pos.quantity);
  const trade: InternalTrade = {
    id: pos.id, strategyId: pos.strategyId, strategyName: pos.strategyName,
    symbol: pos.pair.symbol, side: pos.side, quantity: pos.quantity,
    entryPrice: pos.entryPrice, exitPrice, netPnl,
    returnPct: pos.notional > 0 ? (netPnl / pos.notional) * 100 : 0,
    entryTime: pos.entryTime, exitTime: now,
    exitReason: reason, holdSeconds: Math.round((now - pos.entryTime) / 1000),
  };
  engine.trades.unshift(trade);
  if (engine.trades.length > MAX_TRADES) engine.trades.length = MAX_TRADES;
  engine.balance += pos.notional + netPnl;
  engine.totalRealizedPnl += netPnl;
  strategy.totalTrades++;
  if (netPnl >= 0) { strategy.wins++; engine.totalWins++; strategy.consecutiveLosses = 0; }
  else { strategy.losses++; engine.totalLosses++; strategy.consecutiveLosses++; }
  strategy.totalPnl += netPnl;
  strategy.winRate = strategy.totalTrades > 0 ? (strategy.wins / strategy.totalTrades) * 100 : 0;
  strategy.cooldownUntil = now + cooldownMsFor(strategy, netPnl >= 0);
  strategy.status = "COOLING";
  strategy.currentSymbol = "";
  strategy.position = null;
  engine.positions.delete(pos.id);
}

const EMPTY_STATS: ForexEngineStats = {
  equity: INITIAL_BALANCE, balance: INITIAL_BALANCE, sessionPnl: 0,
  unrealizedPnl: 0, realizedPnl: 0, totalTrades: 0, openPositions: 0,
  winRate: 0, activeStrategies: 0, warmingUp: true, liveSymbols: 0,
  lastUpdateAt: 0, diagnostics: "Bootstrapping forex feed.",
};

function buildPersistedPayload(engine: EngineRef): ForexDbPayload {
  return {
    balance: engine.balance,
    totalWins: engine.totalWins,
    totalLosses: engine.totalLosses,
    totalPnl: engine.totalRealizedPnl,
    tradeSeq: engine.seq,
    positions: [...engine.positions.values()].map((position) => ({
      id: position.id,
      strategyId: position.strategyId,
      symbol: position.pair.symbol,
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
    })),
  };
}

async function saveForexState(engine: EngineRef): Promise<void> {
  try {
    await fetch("/api/forex/state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildPersistedPayload(engine)),
    });
  } catch {
    // non-critical
  }
}

async function loadForexState(engine: EngineRef): Promise<boolean> {
  try {
    const response = await fetch("/api/forex/state");
    if (!response.ok) return false;
    const data = await response.json() as { ok: boolean; found: boolean; state?: ForexDbPayload };
    if (!data.ok || !data.found || !data.state) return false;
    const saved = data.state;
    engine.balance = saved.balance;
    engine.totalWins = saved.totalWins;
    engine.totalLosses = saved.totalLosses;
    engine.totalRealizedPnl = saved.totalPnl;
    engine.seq = saved.tradeSeq;
    engine.trades = (saved.trades ?? []).slice(0, MAX_TRADES);
    engine.positions.clear();

    for (const persisted of saved.positions ?? []) {
      const pair = FOREX_PAIR_BY_SYMBOL.get(persisted.symbol);
      if (!pair) continue;
      const position: InternalPosition = {
        id: persisted.id,
        strategyId: persisted.strategyId,
        strategyName: STRAT_DEFS.find((def) => def.id === persisted.strategyId)?.name ?? persisted.id,
        pair,
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
        strategy.currentSymbol = pair.symbol;
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
      if (!strategy.position) strategy.status = strategy.cooldownUntil > Date.now() ? "COOLING" : "WARMING";
    }
    return true;
  } catch {
    return false;
  }
}

// ── Hook ──────────────────────────────────────────────────────────────────────
export default function useForexEngine() {
  const engineRef = useRef<EngineRef>(initEngine());
  const lastSavedSignatureRef = useRef("");
  const dbLoadedRef = useRef(false);
  const [quotes, setQuotes] = useState<ForexQuoteDisplay[]>(
    FOREX_PAIRS.map((p) => ({ symbol: p.symbol, category: p.category, ltp: 0, changePct: 0, signalScore: 0, hasPosition: false, sparkline: [] }))
  );
  const [positions, setPositions] = useState<ForexPosition[]>([]);
  const [trades, setTrades]       = useState<ForexTrade[]>([]);
  const [strategies, setStrategies] = useState<ForexStrategyStatus[]>(
    STRAT_DEFS.map((def) => ({ id: def.id, name: def.name, category: def.category, side: def.side, status: "WARMING", currentSymbol: "", score: 0, allocationUSD: ALLOCATION_USD, totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0 }))
  );
  const [stats, setStats] = useState<ForexEngineStats>(EMPTY_STATS);

  const processTick = useCallback((items: ForexMarketItem[], errorMessage = "") => {
    const engine = engineRef.current;
    const now = Date.now();
    if (errorMessage) engine.lastError = errorMessage;
    else if (items.length) engine.lastError = "";

    for (const item of items) {
      if (item.price <= 0) continue;
      engine.quotes[item.symbol] = item;
      let bars = engine.bars[item.symbol] ?? [];

      // Pre-seed with historical 1-min candles on first tick (eliminates warmup delay)
      if (item.candles && item.candles.length > bars.length) {
        bars = [...item.candles];
      }

      // Append current price as the latest bar (avoid duplicate if unchanged)
      if (!bars.length || bars[bars.length - 1] !== item.price) {
        bars.push(item.price);
      }
      if (bars.length > MAX_BARS) bars.splice(0, bars.length - MAX_BARS);
      engine.bars[item.symbol] = bars;
    }

    for (const strategy of engine.strategies) {
      if (strategy.position) {
        const latest = engine.quotes[strategy.position.pair.symbol];
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
      if (strategy.totalTrades >= 8 && strategy.totalPnl < 0 && strategy.winRate < 35) {
        strategy.cooldownUntil = Math.max(strategy.cooldownUntil, now + UNDERPERFORMING_PAUSE_MS);
        strategy.status = "COOLING"; strategy.score = 0; continue;
      }
      if (strategy.cooldownUntil > now) { strategy.status = "COOLING"; strategy.score = 0; continue; }
      strategy.status = "READY"; strategy.score = 0; strategy.lastSignalSymbol = "";
      if (engine.positions.size >= MAX_OPEN_POSITIONS) continue;

      let bestPair: ForexPair | null = null;
      let bestScore = 0;
      for (const pair of FOREX_PAIRS) {
        const bars = engine.bars[pair.symbol];
        if (!bars || bars.length < strategy.def.minBars) continue;
        const input = buildSignalInputs(bars);
        const score = evalSignal(strategy.def.signal, input);
        const confirmed = score >= SIGNAL_THRESHOLD && passesEntryConfirmation(strategy.def, input, classifyRegime(input));
        const displayScore = confirmed ? score : Math.min(score, SIGNAL_THRESHOLD - 1);
        if (displayScore > strategy.score) { strategy.score = displayScore; strategy.lastSignalSymbol = pair.symbol; }
        if (confirmed && score > bestScore) { bestScore = score; bestPair = pair; }
      }
      if (bestPair) {
        const price = engine.quotes[bestPair.symbol]?.price ?? 0;
        if (price > 0) openPosition(engine, strategy, bestPair, price, now);
      } else if (FOREX_PAIRS.every((p) => (engine.bars[p.symbol]?.length ?? 0) < strategy.def.minBars)) {
        strategy.status = "WARMING";
      }
    }

    const positionSides: Record<string, Set<Side>> = {};
    const symbolScores: Record<string, number> = {};
    for (const strategy of engine.strategies) {
      const sym = strategy.position?.pair.symbol ?? strategy.lastSignalSymbol;
      if (sym && strategy.score > 0) symbolScores[sym] = Math.max(symbolScores[sym] ?? 0, strategy.score);
      if (strategy.position) {
        if (!positionSides[strategy.position.pair.symbol]) positionSides[strategy.position.pair.symbol] = new Set();
        positionSides[strategy.position.pair.symbol].add(strategy.def.side);
      }
    }

    const pairByCat = Object.fromEntries(FOREX_PAIRS.map((p) => [p.symbol, p.category]));
    setQuotes(FOREX_PAIRS.map((pair) => {
      const q = engine.quotes[pair.symbol];
      const sides = positionSides[pair.symbol];
      return { symbol: pair.symbol, category: pair.category, ltp: q?.price ?? 0, changePct: q?.changePct ?? 0, signalScore: symbolScores[pair.symbol] ?? 0, hasPosition: Boolean(sides?.size), strategyLabel: sides ? [...sides].join("+") : undefined, sparkline: (engine.bars[pair.symbol] ?? []).slice(-24) };
    }));
    setPositions([...engine.positions.values()].map((p) => ({ id: p.id, strategyId: p.strategyId, strategyName: p.strategyName, symbol: p.pair.symbol, category: pairByCat[p.pair.symbol] ?? "Major", side: p.side, quantity: p.quantity, entryPrice: p.entryPrice, currentPrice: p.currentPrice, tpPrice: p.tpPrice, slPrice: p.slPrice, notional: p.notional, entryTime: new Date(p.entryTime).toISOString(), unrealizedPnl: p.unrealizedPnl, returnPct: p.returnPct })));
    setTrades(engine.trades.slice(0, 120).map((t) => ({ id: t.id, strategyId: t.strategyId, strategyName: t.strategyName, symbol: t.symbol, side: t.side, quantity: t.quantity, entryPrice: t.entryPrice, exitPrice: t.exitPrice, netPnl: t.netPnl, returnPct: t.returnPct, entryTime: new Date(t.entryTime).toISOString(), exitTime: new Date(t.exitTime).toISOString(), exitReason: t.exitReason, holdSeconds: t.holdSeconds })));
    setStrategies(engine.strategies.map((s) => ({ id: s.def.id, name: s.def.name, category: s.def.category, side: s.def.side, status: s.status, currentSymbol: s.currentSymbol || s.lastSignalSymbol, score: s.score, allocationUSD: Math.round(ALLOCATION_USD * sizeMultiplierFor(s)), totalTrades: s.totalTrades, wins: s.wins, losses: s.losses, totalPnl: s.totalPnl, winRate: s.winRate, cooldownUntil: s.cooldownUntil > 0 ? new Date(s.cooldownUntil).toISOString() : undefined })));

    const unrealizedPnl = [...engine.positions.values()].reduce((sum, p) => sum + p.unrealizedPnl, 0);
    const openNotional  = [...engine.positions.values()].reduce((sum, p) => sum + p.notional, 0);
    const equity = engine.balance + openNotional + unrealizedPnl;
    const liveSymbols = FOREX_PAIRS.filter((p) => (engine.quotes[p.symbol]?.price ?? 0) > 0).length;
    const totalTrades = engine.strategies.reduce((sum, s) => sum + s.totalTrades, 0);
    const winRate = engine.totalWins + engine.totalLosses > 0 ? (engine.totalWins / (engine.totalWins + engine.totalLosses)) * 100 : 0;
    setStats({ equity, balance: engine.balance, sessionPnl: equity - INITIAL_BALANCE, unrealizedPnl, realizedPnl: engine.totalRealizedPnl, totalTrades, openPositions: engine.positions.size, winRate, activeStrategies: engine.strategies.filter((s) => s.status !== "WARMING").length, warmingUp: FOREX_PAIRS.every((p) => (engine.bars[p.symbol]?.length ?? 0) < MIN_BARS_SLOW), liveSymbols, lastUpdateAt: now, diagnostics: engine.lastError || (liveSymbols > 0 ? `Tracking ${liveSymbols}/12 forex pairs live.` : "Waiting for forex market quotes.") });

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
      void saveForexState(engine);
    }
  }, []);

  const reset = useCallback(() => {
    engineRef.current = initEngine();
    lastSavedSignatureRef.current = "";
    if (dbLoadedRef.current) void saveForexState(engineRef.current);
    setQuotes(FOREX_PAIRS.map((p) => ({ symbol: p.symbol, category: p.category, ltp: 0, changePct: 0, signalScore: 0, hasPosition: false, sparkline: [] })));
    setPositions([]); setTrades([]);
    setStrategies(STRAT_DEFS.map((def) => ({ id: def.id, name: def.name, category: def.category, side: def.side, status: "WARMING", currentSymbol: "", score: 0, allocationUSD: ALLOCATION_USD, totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0 })));
    setStats(EMPTY_STATS);
  }, []);

  useEffect(() => {
    void loadForexState(engineRef.current).then(() => {
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
  }, []);

  useEffect(() => {
    const interval = setInterval(() => {
      if (dbLoadedRef.current) void saveForexState(engineRef.current);
    }, 60_000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const onUnload = () => {
      if (!dbLoadedRef.current) return;
      const payload = JSON.stringify(buildPersistedPayload(engineRef.current));
      navigator.sendBeacon("/api/forex/state", new Blob([payload], { type: "application/json" }));
    };
    window.addEventListener("beforeunload", onUnload);
    return () => window.removeEventListener("beforeunload", onUnload);
  }, []);

  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      if (cancelled) return;
      try {
        const res = await fetch("/api/forex/markets", { next: { revalidate: 0 } } as RequestInit);
        const payload = await res.json() as { ok?: boolean; data?: ForexMarketItem[]; error?: string };
        if (!res.ok || !payload.ok || !payload.data?.length) {
          processTick([], payload.error || `Forex API returned ${res.status}`);
          return;
        }
        processTick(payload.data, "");
      } catch {
        processTick([], "Unable to fetch forex market data.");
      }
    };
    void tick();
    const interval = setInterval(() => void tick(), POLL_MS);
    return () => { cancelled = true; clearInterval(interval); };
  }, [processTick]);

  return { quotes, positions, trades, strategies, stats, reset };
}
