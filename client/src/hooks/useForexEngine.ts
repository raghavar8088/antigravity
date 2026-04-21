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
const MAX_TRADES         = 5_000;
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
const LOCAL_STORAGE_KEY = "forex_state_v2";

// ── Types ────────────────────────────────────────────────────────────────────
type Side   = "LONG" | "SHORT";
type Status = "WARMING" | "READY" | "IN_POSITION" | "COOLING";
type Regime = "UNKNOWN" | "TRENDING_BULL" | "TRENDING_BEAR" | "HIGH_VOL" | "RANGE";
type RosterState = "ACTIVE" | "WATCHLIST";

type SignalInputs = {
  price: number; prevPrice: number;
  fast: number; slow: number; trend: number;
  prevFast: number; prevSlow: number;
  mean20: number; std20: number; rsi14: number;
  high20: number; low20: number;
  breakoutHigh20: number; breakoutLow20: number;
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
  regime: Regime;
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
  lastFeedAt: number;
  lastFeedMode: string;
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
  regime: Regime; rosterState: RosterState;
  allocationUSD: number; sizeMultiplier: number; totalTrades: number; wins: number;
  losses: number; totalPnl: number; winRate: number;
  cooldownUntil?: string;
};

export type ForexEngineStats = {
  equity: number; balance: number; sessionPnl: number;
  unrealizedPnl: number; realizedPnl: number;
  totalTrades: number; totalWins: number; totalLosses: number;
  openPositions: number; winRate: number;
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
  totalPnl: number; winRate: number; cooldownUntil: number; consecutiveLosses: number; regime?: Regime;
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
  const recent20 = bars.slice(-20);
  const prior20 = bars.slice(-21, -1);
  const breakoutWindow = prior20.length > 0 ? prior20 : recent20;
  return {
    price, prevPrice: last > 0 ? bars[last - 1] : price,
    fast: ema(bars, 6), slow: ema(bars, 15), trend: ema(bars, 28),
    prevFast: ema(prev, 6), prevSlow: ema(prev, 15),
    mean20: sma(bars, 20), std20: stdDev(bars, 20), rsi14: rsi(bars, 14),
    high20: Math.max(...recent20), low20: Math.min(...recent20),
    breakoutHigh20: Math.max(...breakoutWindow), breakoutLow20: Math.min(...breakoutWindow),
    momentum3: last >= 3 ? ((price - bars[last - 3]) / bars[last - 3]) * 100 : 0,
    momentum6: last >= 6 ? ((price - bars[last - 6]) / bars[last - 6]) * 100 : 0,
  };
}

function evalSignal(signal: string, input: SignalInputs): number {
  const { price, prevPrice, fast, slow, trend, prevFast, prevSlow, mean20, rsi14, breakoutHigh20, breakoutLow20, momentum3, momentum6 } = input;
  // Forex uses tighter thresholds (smaller % moves)
  switch (signal) {
    case "BREAKOUT":
      return price > breakoutHigh20 * 1.00025 && fast > slow && rsi14 >= 54 && momentum3 > 0.03
        ? scoreClamp(72 + (price / breakoutHigh20 - 1) * 8000 + momentum3 * 15) : 0;
    case "BREAKOUT_SHORT":
      return price < breakoutLow20 * 0.99975 && fast < slow && rsi14 <= 46 && momentum3 < -0.03
        ? scoreClamp(72 + (breakoutLow20 / price - 1) * 8000 + Math.abs(momentum3) * 15) : 0;
    case "EMA_CROSS":
      return prevFast <= prevSlow && fast > slow && price > trend && rsi14 >= 52
        ? scoreClamp(70 + (fast / slow - 1) * 12000 + (rsi14 - 50) * 0.4) : 0;
    case "EMA_CROSS_SHORT":
      return prevFast >= prevSlow && fast < slow && price < trend && rsi14 <= 48
        ? scoreClamp(70 + (slow / fast - 1) * 12000 + (50 - rsi14) * 0.4) : 0;
    case "RSI_BOUNCE":
      return rsi14 <= 32 && price >= prevPrice && momentum3 > -0.15
        ? scoreClamp(67 + (34 - rsi14) * 1.4) : 0;
    case "RSI_BOUNCE_SHORT":
      return rsi14 >= 68 && price <= prevPrice && momentum3 < 0.15
        ? scoreClamp(67 + (rsi14 - 66) * 1.4) : 0;
    case "VWAP_RECLAIM":
      return price > mean20 * 1.00015 && prevPrice <= mean20 * 1.0003 && momentum3 > 0.02
        ? scoreClamp(68 + (price / mean20 - 1) * 5500 + momentum3 * 12) : 0;
    case "VWAP_RECLAIM_SHORT":
      return price < mean20 * 0.99985 && prevPrice >= mean20 * 0.9997 && momentum3 < -0.02
        ? scoreClamp(68 + (mean20 / price - 1) * 5500 + Math.abs(momentum3) * 12) : 0;
    case "TREND_CONT":
      return fast > slow && slow > trend && momentum6 > 0.08 && rsi14 >= 53 && rsi14 <= 76
        ? scoreClamp(73 + momentum6 * 25 + (rsi14 - 54) * 0.3) : 0;
    case "TREND_CONT_SHORT":
      return fast < slow && slow < trend && momentum6 < -0.08 && rsi14 >= 24 && rsi14 <= 47
        ? scoreClamp(73 + Math.abs(momentum6) * 25 + (46 - rsi14) * 0.3) : 0;
    default: return 0;
  }
}

function classifyRegime(input: SignalInputs): Regime {
  const trendGapPct = input.price > 0 ? Math.abs(input.fast - input.slow) / input.price * 100 : 0;
  const volPct = input.price > 0 ? input.std20 / input.price * 100 : 0;
  if (input.price <= 0) return "UNKNOWN";
  if (input.momentum6 === 0 && input.momentum3 === 0) return "UNKNOWN";
  if (input.fast > input.slow && input.slow > input.trend && input.momentum6 > 0.08) return "TRENDING_BULL";
  if (input.fast < input.slow && input.slow < input.trend && input.momentum6 < -0.08) return "TRENDING_BEAR";
  if (volPct >= 0.15 || trendGapPct >= 0.10) return "HIGH_VOL";
  return "RANGE";
}

function passesEntryConfirmation(def: StratDef, input: SignalInputs, regime: Regime): boolean {
  // Regime-based category blocking removed — signals already encode RSI/price extremes
  // so they won't fire when conditions are wrong. Blocking by regime prevented
  // RSI_BOUNCE (Mean Reversion) from ever firing on trending pairs (which is most forex pairs).
  if (def.side === "LONG") {
    if (def.category === "VWAP")
      return input.price >= input.mean20 * 0.9998 && input.rsi14 >= 44;
    if (def.category === "Mean Reversion")
      // RSI_BOUNCE already requires price >= prevPrice — don't double-gate on momentum
      return input.rsi14 <= 52 && input.price >= input.prevPrice * 0.9998;
    if (regime === "TRENDING_BULL")
      // Relax fast >= slow EMA stack requirement — allow near-alignment
      return input.price >= input.fast * 0.9997 && input.momentum3 > -0.02;
    // General confirmation: direction check only, no strict momentum threshold
    return input.price >= input.prevPrice * 0.9997 && input.momentum3 > -0.05;
  }
  if (def.category === "VWAP")
    return input.price <= input.mean20 * 1.0002 && input.rsi14 <= 56;
  if (def.category === "Mean Reversion")
    return input.rsi14 >= 48 && input.price <= input.prevPrice * 1.0002;
  if (regime === "TRENDING_BEAR")
    return input.price <= input.fast * 1.0003 && input.momentum3 < 0.02;
  return input.price <= input.prevPrice * 1.0003 && input.momentum3 < 0.05;
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
    strategies: STRAT_DEFS.map((def) => ({ def, position: null, status: "WARMING", cooldownUntil: 0, score: 0, currentSymbol: "", lastSignalSymbol: "", regime: "UNKNOWN", totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0, consecutiveLosses: 0 })),
    positions: new Map(), trades: [],
    balance: INITIAL_BALANCE, seq: 0,
    totalWins: 0, totalLosses: 0, totalRealizedPnl: 0, lastError: "",
    lastFeedAt: 0, lastFeedMode: "",
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
  strategy.regime = "UNKNOWN";
  strategy.position = null;
  engine.positions.delete(pos.id);
}

function rosterStateFor(status: Status): RosterState {
  return status === "WARMING" ? "WATCHLIST" : "ACTIVE";
}

const EMPTY_STATS: ForexEngineStats = {
  equity: INITIAL_BALANCE, balance: INITIAL_BALANCE, sessionPnl: 0,
  unrealizedPnl: 0, realizedPnl: 0, totalTrades: 0, totalWins: 0, totalLosses: 0,
  openPositions: 0, winRate: 0, activeStrategies: 0, warmingUp: true, liveSymbols: 0,
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

function loadFromLocalStorage(): ForexDbPayload | null {
  try {
    const raw = localStorage.getItem(LOCAL_STORAGE_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as ForexDbPayload;
  } catch {
    return null;
  }
}

function compareSavedStates(a: ForexDbPayload, b: ForexDbPayload): number {
  const tradeSeqDiff = (a.tradeSeq ?? 0) - (b.tradeSeq ?? 0);
  if (tradeSeqDiff !== 0) return tradeSeqDiff;

  const tradeCountDiff = (a.trades?.length ?? 0) - (b.trades?.length ?? 0);
  if (tradeCountDiff !== 0) return tradeCountDiff;

  const positionCountDiff = (a.positions?.length ?? 0) - (b.positions?.length ?? 0);
  if (positionCountDiff !== 0) return positionCountDiff;

  return 0;
}

async function saveForexState(engine: EngineRef): Promise<void> {
  saveToLocalStorage(engine);
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

function applySavedState(engine: EngineRef, saved: ForexDbPayload): boolean {
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
  return true;
}

async function loadForexState(engine: EngineRef): Promise<boolean> {
  const local = loadFromLocalStorage();
  let dbState: ForexDbPayload | null = null;
  try {
    const response = await fetch("/api/forex/state");
    if (response.ok) {
      const data = await response.json() as { ok: boolean; found: boolean; state?: ForexDbPayload };
      if (data.ok && data.found && data.state) dbState = data.state;
    }
  } catch {
    // Fall back to the newest local snapshot when the DB is unavailable.
  }

  const saved = local && dbState
    ? (compareSavedStates(local, dbState) >= 0 ? local : dbState)
    : (local ?? dbState);

  return saved ? applySavedState(engine, saved) : false;
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
    STRAT_DEFS.map((def) => ({ id: def.id, name: def.name, category: def.category, side: def.side, status: "WARMING", currentSymbol: "", score: 0, regime: "UNKNOWN", rosterState: "WATCHLIST", allocationUSD: ALLOCATION_USD, sizeMultiplier: 1, totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0 }))
  );
  const [stats, setStats] = useState<ForexEngineStats>(EMPTY_STATS);

  const pushDisplayState = useCallback(() => {
    const engine = engineRef.current;
    const pairByCat = Object.fromEntries(FOREX_PAIRS.map((pair) => [pair.symbol, pair.category]));
    const positionSides: Record<string, Set<Side>> = {};
    const symbolScores: Record<string, number> = {};

    for (const strategy of engine.strategies) {
      const symbol = strategy.position?.pair.symbol ?? strategy.lastSignalSymbol;
      if (symbol && strategy.score > 0) symbolScores[symbol] = Math.max(symbolScores[symbol] ?? 0, strategy.score);
      if (strategy.position) {
        if (!positionSides[strategy.position.pair.symbol]) positionSides[strategy.position.pair.symbol] = new Set();
        positionSides[strategy.position.pair.symbol].add(strategy.def.side);
      }
    }

    setQuotes(FOREX_PAIRS.map((pair) => {
      const quote = engine.quotes[pair.symbol];
      const sides = positionSides[pair.symbol];
      return {
        symbol: pair.symbol,
        category: pair.category,
        ltp: quote?.price ?? 0,
        changePct: quote?.changePct ?? 0,
        signalScore: symbolScores[pair.symbol] ?? 0,
        hasPosition: Boolean(sides?.size),
        strategyLabel: sides ? [...sides].join("+") : undefined,
        sparkline: (engine.bars[pair.symbol] ?? []).slice(-24),
      };
    }));

    setPositions([...engine.positions.values()].map((position) => ({
      id: position.id,
      strategyId: position.strategyId,
      strategyName: position.strategyName,
      symbol: position.pair.symbol,
      category: pairByCat[position.pair.symbol] ?? "Major",
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

    setStrategies(engine.strategies.map((strategy) => {
      const sizeMultiplier = sizeMultiplierFor(strategy);
      return {
        id: strategy.def.id,
        name: strategy.def.name,
        category: strategy.def.category,
        side: strategy.def.side,
        status: strategy.status,
        currentSymbol: strategy.currentSymbol || strategy.lastSignalSymbol,
        score: strategy.score,
        regime: strategy.regime,
        rosterState: rosterStateFor(strategy.status),
        allocationUSD: Math.round(ALLOCATION_USD * sizeMultiplier),
        sizeMultiplier,
        totalTrades: strategy.totalTrades,
        wins: strategy.wins,
        losses: strategy.losses,
        totalPnl: strategy.totalPnl,
        winRate: strategy.winRate,
        cooldownUntil: strategy.cooldownUntil > 0 ? new Date(strategy.cooldownUntil).toISOString() : undefined,
      };
    }));

    const unrealizedPnl = [...engine.positions.values()].reduce((sum, position) => sum + position.unrealizedPnl, 0);
    const openNotional  = [...engine.positions.values()].reduce((sum, position) => sum + position.notional, 0);
    const equity = engine.balance + openNotional + unrealizedPnl;
    const liveSymbols = FOREX_PAIRS.filter((pair) => (engine.quotes[pair.symbol]?.price ?? 0) > 0).length;
    const totalTrades = engine.strategies.reduce((sum, strategy) => sum + strategy.totalTrades, 0);
    const winRate = engine.totalWins + engine.totalLosses > 0 ? (engine.totalWins / (engine.totalWins + engine.totalLosses)) * 100 : 0;
    const diagnostics = engine.lastError || (liveSymbols > 0
      ? `Tracking ${liveSymbols}/12 forex pairs live${engine.lastFeedMode ? ` via ${engine.lastFeedMode}` : ""}.`
      : "Waiting for forex market quotes.");

    setStats({
      equity,
      balance: engine.balance,
      sessionPnl: equity - INITIAL_BALANCE,
      unrealizedPnl,
      realizedPnl: engine.totalRealizedPnl,
      totalTrades,
      totalWins: engine.totalWins,
      totalLosses: engine.totalLosses,
      openPositions: engine.positions.size,
      winRate,
      activeStrategies: engine.strategies.filter((strategy) => strategy.status !== "WARMING").length,
      warmingUp: FOREX_PAIRS.every((pair) => (engine.bars[pair.symbol]?.length ?? 0) < MIN_BARS_SLOW),
      liveSymbols,
      lastUpdateAt: engine.lastFeedAt,
      diagnostics,
    });
  }, []);

  const processTick = useCallback((items: ForexMarketItem[], errorMessage = "", pauseEntries = false) => {
    const engine = engineRef.current;
    const now = Date.now();
    if (items.length > 0) engine.lastFeedAt = now;
    if (errorMessage) engine.lastError = errorMessage;
    else if (items.length && !pauseEntries) engine.lastError = "";

    for (const item of items) {
      const historyBars = (item.candles ?? []).filter((bar) => bar > 0).slice(-MAX_BARS);
      const livePrice = item.price > 0 ? item.price : (historyBars.length > 0 ? historyBars[historyBars.length - 1] : 0);
      if (livePrice <= 0) continue;

      if (item.interval) engine.lastFeedMode = item.interval;
      engine.quotes[item.symbol] = { ...item, price: livePrice };
      const bars = historyBars.length > 0 ? [...historyBars] : [...(engine.bars[item.symbol] ?? [])];

      // Refresh the 1-minute candle window every tick, then append an in-flight quote if it moved.
      if (!bars.length || bars[bars.length - 1] !== livePrice) {
        bars.push(livePrice);
      }
      if (bars.length > MAX_BARS) bars.splice(0, bars.length - MAX_BARS);
      engine.bars[item.symbol] = bars;
    }

    for (const strategy of engine.strategies) {
      if (strategy.position) {
        const latest = engine.quotes[strategy.position.pair.symbol];
        if (latest?.price > 0) {
          // eslint-disable-next-line react-hooks/immutability
          strategy.position.currentPrice = latest.price;
          strategy.position.unrealizedPnl = calcPnl(strategy.position.side, strategy.position.entryPrice, latest.price, strategy.position.quantity);
          strategy.position.returnPct = strategy.position.notional > 0 ? (strategy.position.unrealizedPnl / strategy.position.notional) * 100 : 0;
          strategy.position.peakReturnPct = Math.max(strategy.position.peakReturnPct, strategy.position.returnPct);
          const currentBars = engine.bars[strategy.position.pair.symbol];
          if (currentBars && currentBars.length >= strategy.def.minBars) {
            strategy.regime = classifyRegime(buildSignalInputs(currentBars));
          }
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
      strategy.status = "READY"; strategy.score = 0; strategy.lastSignalSymbol = ""; strategy.regime = "UNKNOWN";
      if (engine.positions.size >= MAX_OPEN_POSITIONS) continue;

      let bestPair: ForexPair | null = null;
      let bestScore = 0;
      let bestRegime: Regime = "UNKNOWN";
      for (const pair of FOREX_PAIRS) {
        const bars = engine.bars[pair.symbol];
        if (!bars || bars.length < strategy.def.minBars) continue;
        const input = buildSignalInputs(bars);
        const regime = classifyRegime(input);
        const score = evalSignal(strategy.def.signal, input);
        const confirmed = !pauseEntries && score >= SIGNAL_THRESHOLD && passesEntryConfirmation(strategy.def, input, regime);
        const displayScore = confirmed ? score : Math.min(score, SIGNAL_THRESHOLD - 1);
        if (displayScore > strategy.score) {
          strategy.score = displayScore;
          strategy.lastSignalSymbol = pair.symbol;
          strategy.regime = regime;
        }
        if (confirmed && score > bestScore) {
          bestScore = score;
          bestPair = pair;
          bestRegime = regime;
        }
      }
      if (bestPair) {
        const price = engine.quotes[bestPair.symbol]?.price ?? 0;
        if (price > 0 && openPosition(engine, strategy, bestPair, price, now)) strategy.regime = bestRegime;
      } else if (FOREX_PAIRS.every((p) => (engine.bars[p.symbol]?.length ?? 0) < strategy.def.minBars)) {
        strategy.status = "WARMING";
        strategy.regime = "UNKNOWN";
      }
    }

    pushDisplayState();

    if (!dbLoadedRef.current) return;
    const signature = buildSaveSignature(engine);
    if (signature !== lastSavedSignatureRef.current) {
      lastSavedSignatureRef.current = signature;
      void saveForexState(engine);
    }
  }, [pushDisplayState]);

  const reset = useCallback(() => {
    engineRef.current = initEngine();
    lastSavedSignatureRef.current = "";
    if (dbLoadedRef.current) void saveForexState(engineRef.current);
    setQuotes(FOREX_PAIRS.map((p) => ({ symbol: p.symbol, category: p.category, ltp: 0, changePct: 0, signalScore: 0, hasPosition: false, sparkline: [] })));
    setPositions([]); setTrades([]);
    setStrategies(STRAT_DEFS.map((def) => ({ id: def.id, name: def.name, category: def.category, side: def.side, status: "WARMING", currentSymbol: "", score: 0, regime: "UNKNOWN", rosterState: "WATCHLIST", allocationUSD: ALLOCATION_USD, sizeMultiplier: 1, totalTrades: 0, wins: 0, losses: 0, totalPnl: 0, winRate: 0 })));
    setStats(EMPTY_STATS);
  }, []);

  useEffect(() => {
    void loadForexState(engineRef.current).then(() => {
      dbLoadedRef.current = true;
      lastSavedSignatureRef.current = buildSaveSignature(engineRef.current);
      pushDisplayState();
    });
  }, [pushDisplayState]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (dbLoadedRef.current) void saveForexState(engineRef.current);
    }, 60_000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const onUnload = () => {
      if (!dbLoadedRef.current) return;
      saveToLocalStorage(engineRef.current);
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
        const payload = await res.json() as { ok?: boolean; data?: ForexMarketItem[]; error?: string; stale?: boolean; cached?: boolean };
        if (!res.ok || !payload.ok || !payload.data?.length) {
          processTick([], payload.error || `Forex API returned ${res.status}`);
          return;
        }
        // pauseEntries removed: Yahoo returns stale:true on rate-limiting/failures, which
        // was blocking ALL new position entries. Signals are self-gating via RSI/price
        // checks. Stale data just means candles don't advance — signals won't fire on
        // unchanged bars anyway. Keep the diagnostic message but don't pause entries.
        processTick(payload.data, payload.stale ? payload.error || "Using cached forex quotes." : "", false);
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
