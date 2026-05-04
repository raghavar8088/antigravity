"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/** Paper desk sized for micro accounts — live sizing still uses the same math. */
const INITIAL_BALANCE = 10;
const MIN_NOTIONAL_USD = 1;
const MAX_NOTIONAL_USD = 3.5;
const MAX_OPEN_POSITIONS = 3;
const MAX_BARS = 120;
const MIN_BARS = 26;
const SIGNAL_THRESHOLD = 62;
const POLL_MS = 8_000;
const MAX_TRADES = 2_000;
/** Taker-style round trip (entry + exit) — conservative vs Binance 0.2% VIP0. */
const ROUND_TRIP_FEE_FRAC = 0.0015;

const PROFIT_LOCK_PROGRESS = 0.32;
const PROFIT_LOCK_SHARE = 0.38;
const LATE_EXIT_PROGRESS = 0.58;
const LATE_EXIT_MIN_GAIN = 0.04;
const GRIND_EXIT_PROGRESS = 0.45;
const GRIND_EXIT_SHARE = 0.22;
const TRAIL_ACTIVATION_PCT = 0.2;
const TRAIL_GIVEBACK_SHARE = 0.35;
const LOSS_COOLDOWN_PENALTY = 0.4;
const VOL_SPIKE_RATIO = 1.45;
const VOL_BOOST_POINTS = 4;
const VOL_HISTORY = 24;

const LS_KEY = "btc_spot_scalper_paper_v1";

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
  volRatio: number;
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
  feesUsd: number;
}

interface InternalStrategyState {
  def: StratDef;
  position: InternalPosition | null;
  status: Status;
  cooldownUntil: number;
  score: number;
  lastSignalSymbol: string;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  consecutiveLosses: number;
}

interface EngineRef {
  bars: number[];
  volBars: number[];
  strategies: InternalStrategyState[];
  positions: Map<string, InternalPosition>;
  trades: InternalTrade[];
  balance: number;
  seq: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
  lastError: string;
  lastPrice: number;
  changePct24h: number;
}

export type BTCSpotQuote = {
  symbol: string;
  ltp: number;
  changePct24h: number;
  signalScore: number;
  hasPosition: boolean;
  sparkline: number[];
};

export type BTCSpotPosition = {
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

export type BTCSpotTrade = {
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
  feesUsd: number;
};

export type BTCSpotStrategyStatus = {
  id: number;
  name: string;
  category: string;
  side: Side;
  status: Status;
  score: number;
  targetNotionalUsd: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  cooldownUntil?: string;
};

export type BTCSpotEngineStats = {
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
  warmingUp: boolean;
  lastUpdateAt: number;
  diagnostics: string;
  feeModelNote: string;
};

type DbPayload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  tradeSeq: number;
  positions: Array<{
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
  }>;
  trades: InternalTrade[];
  strategies: Array<{
    id: number;
    totalTrades: number;
    wins: number;
    losses: number;
    totalPnl: number;
    winRate: number;
    cooldownUntil: number;
    consecutiveLosses: number;
  }>;
};

const STRAT_DEFS: StratDef[] = [
  { id: 1, name: "Micro Range Breakout", category: "Breakout", side: "LONG", signal: "BREAKOUT", tpPct: 0.32, slPct: 0.16, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 45 },
  { id: 2, name: "Micro Range Breakdown", category: "Breakout", side: "SHORT", signal: "BREAKOUT_SHORT", tpPct: 0.32, slPct: 0.16, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 45 },
  { id: 3, name: "EMA Ribbon Impulse Long", category: "Momentum", side: "LONG", signal: "EMA_CROSS", tpPct: 0.28, slPct: 0.14, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 4, name: "EMA Ribbon Impulse Short", category: "Momentum", side: "SHORT", signal: "EMA_CROSS_SHORT", tpPct: 0.28, slPct: 0.14, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 5, name: "RSI 1m Oversold Bounce", category: "Mean Reversion", side: "LONG", signal: "RSI_BOUNCE", tpPct: 0.24, slPct: 0.13, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 6, name: "RSI 1m Overbought Fade", category: "Mean Reversion", side: "SHORT", signal: "RSI_BOUNCE_SHORT", tpPct: 0.24, slPct: 0.13, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 7, name: "Session VWAP Reclaim", category: "VWAP", side: "LONG", signal: "VWAP_RECLAIM", tpPct: 0.26, slPct: 0.13, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 8, name: "Session VWAP Reject", category: "VWAP", side: "SHORT", signal: "VWAP_RECLAIM_SHORT", tpPct: 0.26, slPct: 0.13, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 9, name: "Trend Leg Continuation", category: "Trend", side: "LONG", signal: "TREND_CONT", tpPct: 0.34, slPct: 0.15, cooldownMinutes: 9, minBars: MIN_BARS, holdMinutes: 50 },
  { id: 10, name: "Trend Leg Exhaustion Short", category: "Trend", side: "SHORT", signal: "TREND_CONT_SHORT", tpPct: 0.34, slPct: 0.15, cooldownMinutes: 9, minBars: MIN_BARS, holdMinutes: 50 },
];

function sma(values: number[], period: number): number {
  const slice = values.slice(-period);
  return slice.length ? slice.reduce((a, b) => a + b, 0) / slice.length : 0;
}

function ema(values: number[], period: number): number {
  if (!values.length) return 0;
  const k = 2 / (period + 1);
  let current = values[0];
  for (let i = 1; i < values.length; i++) current = values[i] * k + current * (1 - k);
  return current;
}

function stdDev(values: number[], period: number): number {
  const slice = values.slice(-period);
  if (!slice.length) return 0;
  const avg = slice.reduce((a, b) => a + b, 0) / slice.length;
  return Math.sqrt(slice.reduce((a, b) => a + (b - avg) ** 2, 0) / slice.length);
}

function rsi(values: number[], period: number): number {
  if (values.length < 2) return 50;
  const start = Math.max(1, values.length - period);
  let gains = 0;
  let losses = 0;
  for (let i = start; i < values.length; i++) {
    const diff = values[i] - values[i - 1];
    if (diff > 0) gains += diff;
    else losses -= diff;
  }
  if (losses === 0) return 100;
  return 100 - 100 / (1 + gains / losses);
}

function scoreClamp(value: number): number {
  return Math.max(0, Math.min(99, value));
}

function buildSignalInputs(bars: number[], volRatio: number): SignalInputs {
  const last = bars.length - 1;
  const price = bars[last];
  const previous = bars.slice(0, -1);
  return {
    price,
    prevPrice: last > 0 ? bars[last - 1] : price,
    fast: ema(bars, 8),
    slow: ema(bars, 21),
    trend: ema(bars, 34),
    prevFast: ema(previous, 8),
    prevSlow: ema(previous, 21),
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

/** Signals calibrated on 1m closes (BTC). */
function evalMinuteSignal(signal: string, input: SignalInputs): number {
  const { price, prevPrice, fast, slow, trend, prevFast, prevSlow, mean20, rsi14, high20, low20, momentum3, momentum6 } = input;
  switch (signal) {
    case "BREAKOUT":
      return price > high20 * 1.0009 && fast > slow && rsi14 >= 52 && momentum3 > 0.06
        ? scoreClamp(72 + (price / high20 - 1) * 9000 + momentum3 * 25) : 0;
    case "BREAKOUT_SHORT":
      return price < low20 * 0.9991 && fast < slow && rsi14 <= 48 && momentum3 < -0.06
        ? scoreClamp(72 + (low20 / price - 1) * 9000 + Math.abs(momentum3) * 25) : 0;
    case "EMA_CROSS":
      return prevFast <= prevSlow && fast > slow && price > trend && rsi14 >= 50
        ? scoreClamp(70 + (fast / slow - 1) * 12000 + (rsi14 - 50) * 0.35) : 0;
    case "EMA_CROSS_SHORT":
      return prevFast >= prevSlow && fast < slow && price < trend && rsi14 <= 50
        ? scoreClamp(70 + (slow / fast - 1) * 12000 + (50 - rsi14) * 0.35) : 0;
    case "RSI_BOUNCE":
      return rsi14 <= 34 && price >= prevPrice && momentum3 > -0.08
        ? scoreClamp(67 + (36 - rsi14) * 1.2) : 0;
    case "RSI_BOUNCE_SHORT":
      return rsi14 >= 66 && price <= prevPrice && momentum3 < 0.08
        ? scoreClamp(67 + (rsi14 - 64) * 1.2) : 0;
    case "VWAP_RECLAIM":
      return price > mean20 && prevPrice <= mean20 * 1.0005 && momentum3 > 0.03
        ? scoreClamp(68 + (price / mean20 - 1) * 8000 + momentum3 * 20) : 0;
    case "VWAP_RECLAIM_SHORT":
      return price < mean20 && prevPrice >= mean20 * 0.9995 && momentum3 < -0.03
        ? scoreClamp(68 + (mean20 / price - 1) * 8000 + Math.abs(momentum3) * 20) : 0;
    case "TREND_CONT":
      return fast > slow && slow > trend && momentum6 > 0.07 && rsi14 >= 52 && rsi14 <= 76
        ? scoreClamp(73 + momentum6 * 45 + (rsi14 - 52) * 0.25) : 0;
    case "TREND_CONT_SHORT":
      return fast < slow && slow < trend && momentum6 < -0.07 && rsi14 >= 24 && rsi14 <= 48
        ? scoreClamp(73 + Math.abs(momentum6) * 45 + (48 - rsi14) * 0.25) : 0;
    default:
      return 0;
  }
}

function classifyRegime(input: SignalInputs): string {
  const trendGapPct = input.price > 0 ? Math.abs(input.fast - input.slow) / input.price * 100 : 0;
  const volPct = input.price > 0 ? input.std20 / input.price * 100 : 0;
  if (input.fast > input.slow && input.slow > input.trend && input.momentum6 > 0.05) return "TRENDING_BULL";
  if (input.fast < input.slow && input.slow < input.trend && input.momentum6 < -0.05) return "TRENDING_BEAR";
  if (volPct >= 0.22 || trendGapPct >= 0.08) return "HIGH_VOL";
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
    if (def.category === "VWAP") return input.price >= input.mean20 * 0.9995 && input.rsi14 >= 46;
    if (def.category === "Mean Reversion") return input.rsi14 <= 40 && input.price >= input.prevPrice * 0.9998;
    if (def.category === "Breakout") return input.momentum3 > 0.02;
    return input.price >= input.fast && input.fast >= input.slow && input.momentum3 > 0.02;
  }
  if (def.category === "VWAP") return input.price <= input.mean20 * 1.0005 && input.rsi14 <= 54;
  if (def.category === "Mean Reversion") return input.rsi14 >= 60 && input.price <= input.prevPrice * 1.0002;
  if (def.category === "Breakout") return input.momentum3 < -0.02;
  return input.price <= input.fast && input.fast <= input.slow && input.momentum3 < -0.02;
}

function calcPnl(side: Side, entry: number, exit: number, qty: number): number {
  return (exit - entry) * qty * (side === "LONG" ? 1 : -1);
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
  if (pos.peakReturnPct >= Math.max(TRAIL_ACTIVATION_PCT, def.tpPct * 0.45) && returnPct > 0 && returnPct <= pos.peakReturnPct * (1 - TRAIL_GIVEBACK_SHARE)) return { reason: "TRAIL_STOP", exitPrice: price };
  if (progress >= LATE_EXIT_PROGRESS && returnPct >= LATE_EXIT_MIN_GAIN) return { reason: "LATE_EXIT", exitPrice: price };
  if (maxHoldMs > 0 && now - pos.entryTime >= maxHoldMs) return { reason: "TIME_EXIT", exitPrice: price };
  return null;
}

function targetNotionalFor(engine: EngineRef): number {
  const open = engine.positions.size;
  const reserved = open * 0.15;
  const equity =
    engine.balance +
    [...engine.positions.values()].reduce((s, p) => s + p.notional + p.unrealizedPnl, 0);
  const slice = Math.max(MIN_NOTIONAL_USD, Math.min(MAX_NOTIONAL_USD, (equity - reserved) * 0.38));
  return Math.min(slice, Math.max(0, engine.balance - 0.25));
}

function cooldownMsFor(strategy: InternalStrategyState, won: boolean): number {
  const base = strategy.def.cooldownMinutes * 60 * 1000;
  if (won) return base;
  return Math.round(base * (1 + strategy.consecutiveLosses * LOSS_COOLDOWN_PENALTY));
}

function initEngine(): EngineRef {
  return {
    bars: [],
    volBars: [],
    strategies: STRAT_DEFS.map((def) => ({
      def,
      position: null,
      status: "WARMING",
      cooldownUntil: 0,
      score: 0,
      lastSignalSymbol: "",
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
    lastPrice: 0,
    changePct24h: 0,
  };
}

const EMPTY_STATS: BTCSpotEngineStats = {
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
  warmingUp: true,
  lastUpdateAt: 0,
  diagnostics: "Loading BTC 1m candles…",
  feeModelNote: `Round-trip fee model ≈ ${(ROUND_TRIP_FEE_FRAC * 100).toFixed(2)}% of notional (conservative spot taker assumption).`,
};

function buildPayload(engine: EngineRef): DbPayload {
  return {
    balance: engine.balance,
    totalWins: engine.totalWins,
    totalLosses: engine.totalLosses,
    totalPnl: engine.totalRealizedPnl,
    tradeSeq: engine.seq,
    positions: [...engine.positions.values()].map((p) => ({
      id: p.id,
      strategyId: p.strategyId,
      side: p.side,
      entryPrice: p.entryPrice,
      currentPrice: p.currentPrice,
      tpPrice: p.tpPrice,
      slPrice: p.slPrice,
      quantity: p.quantity,
      notional: p.notional,
      entryTime: p.entryTime,
      unrealizedPnl: p.unrealizedPnl,
      returnPct: p.returnPct,
      peakReturnPct: p.peakReturnPct,
    })),
    trades: engine.trades.slice(0, MAX_TRADES),
    strategies: engine.strategies.map((s) => ({
      id: s.def.id,
      totalTrades: s.totalTrades,
      wins: s.wins,
      losses: s.losses,
      totalPnl: s.totalPnl,
      winRate: s.winRate,
      cooldownUntil: s.cooldownUntil,
      consecutiveLosses: s.consecutiveLosses,
    })),
  };
}

function saveLs(engine: EngineRef): void {
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(buildPayload(engine)));
  } catch {
    /* ignore */
  }
}

function loadLs(): DbPayload | null {
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as DbPayload;
  } catch {
    return null;
  }
}

function applySaved(engine: EngineRef, saved: DbPayload): void {
  engine.balance = typeof saved.balance === "number" && saved.balance >= 0 ? saved.balance : INITIAL_BALANCE;
  engine.totalWins = saved.totalWins ?? 0;
  engine.totalLosses = saved.totalLosses ?? 0;
  engine.totalRealizedPnl = saved.totalPnl ?? 0;
  engine.seq = saved.tradeSeq ?? 0;
  engine.trades = (saved.trades ?? []).slice(0, MAX_TRADES);
  engine.positions.clear();
  for (const row of saved.positions ?? []) {
    const def = STRAT_DEFS.find((d) => d.id === row.strategyId);
    if (!def) continue;
    const pos: InternalPosition = {
      id: row.id,
      strategyId: row.strategyId,
      strategyName: def.name,
      side: row.side,
      entryPrice: row.entryPrice,
      currentPrice: row.currentPrice,
      tpPrice: row.tpPrice,
      slPrice: row.slPrice,
      quantity: row.quantity,
      notional: row.notional,
      entryTime: row.entryTime,
      unrealizedPnl: row.unrealizedPnl,
      returnPct: row.returnPct,
      peakReturnPct: row.peakReturnPct ?? row.returnPct,
    };
    engine.positions.set(pos.id, pos);
    const st = engine.strategies.find((s) => s.def.id === pos.strategyId);
    if (st) {
      st.position = pos;
      st.status = "IN_POSITION";
    }
  }
  for (const st of engine.strategies) {
    const row = saved.strategies?.find((r) => r.id === st.def.id);
    if (!row) continue;
    st.totalTrades = row.totalTrades;
    st.wins = row.wins;
    st.losses = row.losses;
    st.totalPnl = row.totalPnl;
    st.winRate = row.winRate;
    st.cooldownUntil = row.cooldownUntil;
    st.consecutiveLosses = row.consecutiveLosses ?? 0;
    if (!st.position) st.status = st.cooldownUntil > Date.now() ? "COOLING" : "WARMING";
  }
}

function openPosition(engine: EngineRef, strategy: InternalStrategyState, price: number, now: number): boolean {
  if (engine.positions.size >= MAX_OPEN_POSITIONS) return false;
  if (strategy.position) return false;
  const notional = targetNotionalFor(engine);
  if (notional < MIN_NOTIONAL_USD || engine.balance < notional) return false;
  const quantity = price > 0 ? notional / price : 0;
  if (quantity <= 0) return false;
  engine.seq++;
  engine.balance -= notional;
  const tpM = 1 + strategy.def.tpPct / 100;
  const slM = 1 - strategy.def.slPct / 100;
  const pos: InternalPosition = {
    id: `BTC-SPOT-${Date.now()}-${engine.seq}`,
    strategyId: strategy.def.id,
    strategyName: strategy.def.name,
    side: strategy.def.side,
    entryPrice: price,
    currentPrice: price,
    tpPrice: strategy.def.side === "LONG" ? price * tpM : price * (2 - tpM),
    slPrice: strategy.def.side === "LONG" ? price * slM : price * (2 - slM),
    quantity,
    notional,
    entryTime: now,
    unrealizedPnl: 0,
    returnPct: 0,
    peakReturnPct: 0,
  };
  strategy.position = pos;
  strategy.status = "IN_POSITION";
  engine.positions.set(pos.id, pos);
  return true;
}

function closePosition(engine: EngineRef, strategy: InternalStrategyState, exitPrice: number, reason: string, now: number): void {
  const pos = strategy.position;
  if (!pos) return;
  const gross = calcPnl(pos.side, pos.entryPrice, exitPrice, pos.quantity);
  const feesUsd = pos.notional * ROUND_TRIP_FEE_FRAC;
  const netPnl = gross - feesUsd;
  const trade: InternalTrade = {
    id: pos.id,
    strategyId: pos.strategyId,
    strategyName: pos.strategyName,
    symbol: "BTC",
    side: pos.side,
    quantity: pos.quantity,
    entryPrice: pos.entryPrice,
    exitPrice,
    netPnl,
    returnPct: pos.notional > 0 ? (netPnl / pos.notional) * 100 : 0,
    entryTime: pos.entryTime,
    exitTime: now,
    exitReason: reason,
    holdSeconds: Math.round((now - pos.entryTime) / 1000),
    feesUsd,
  };
  engine.trades.unshift(trade);
  if (engine.trades.length > MAX_TRADES) engine.trades.length = MAX_TRADES;
  engine.balance += pos.notional + netPnl;
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
  strategy.position = null;
  engine.positions.delete(pos.id);
}

export default function useBTCSpotScalperEngine() {
  const engineRef = useRef<EngineRef>(initEngine());
  const loadedRef = useRef(false);
  const [quote, setQuote] = useState<BTCSpotQuote>({
    symbol: "BTC",
    ltp: 0,
    changePct24h: 0,
    signalScore: 0,
    hasPosition: false,
    sparkline: [],
  });
  const [positions, setPositions] = useState<BTCSpotPosition[]>([]);
  const [trades, setTrades] = useState<BTCSpotTrade[]>([]);
  const [strategies, setStrategies] = useState<BTCSpotStrategyStatus[]>(
    STRAT_DEFS.map((d) => ({
      id: d.id,
      name: d.name,
      category: d.category,
      side: d.side,
      status: "WARMING",
      score: 0,
      targetNotionalUsd: MIN_NOTIONAL_USD,
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
    })),
  );
  const [stats, setStats] = useState<BTCSpotEngineStats>(EMPTY_STATS);

  const pushDisplay = useCallback(() => {
    const engine = engineRef.current;
    const now = Date.now();
    let maxScore = 0;
    for (const s of engine.strategies) {
      if (s.score > maxScore) maxScore = s.score;
    }
    setQuote({
      symbol: "BTC",
      ltp: engine.lastPrice,
      changePct24h: engine.changePct24h,
      signalScore: maxScore,
      hasPosition: engine.positions.size > 0,
      sparkline: engine.bars.slice(-32),
    });
    setPositions(
      [...engine.positions.values()].map((p) => ({
        id: p.id,
        strategyId: p.strategyId,
        strategyName: p.strategyName,
        symbol: "BTC",
        side: p.side,
        quantity: p.quantity,
        entryPrice: p.entryPrice,
        currentPrice: p.currentPrice,
        tpPrice: p.tpPrice,
        slPrice: p.slPrice,
        notional: p.notional,
        entryTime: new Date(p.entryTime).toISOString(),
        unrealizedPnl: p.unrealizedPnl,
        returnPct: p.returnPct,
      })),
    );
    setTrades(
      engine.trades.slice(0, 120).map((t) => ({
        ...t,
        entryTime: new Date(t.entryTime).toISOString(),
        exitTime: new Date(t.exitTime).toISOString(),
      })),
    );
    setStrategies(
      engine.strategies.map((s) => ({
        id: s.def.id,
        name: s.def.name,
        category: s.def.category,
        side: s.def.side,
        status: s.status,
        score: s.score,
        targetNotionalUsd: Math.round(targetNotionalFor(engine) * 100) / 100,
        totalTrades: s.totalTrades,
        wins: s.wins,
        losses: s.losses,
        totalPnl: s.totalPnl,
        winRate: s.winRate,
        cooldownUntil: s.cooldownUntil > 0 ? new Date(s.cooldownUntil).toISOString() : undefined,
      })),
    );
    const unrealized = [...engine.positions.values()].reduce((a, p) => a + p.unrealizedPnl, 0);
    const openN = [...engine.positions.values()].reduce((a, p) => a + p.notional, 0);
    const equity = engine.balance + openN + unrealized;
    const tw = engine.totalWins + engine.totalLosses;
    setStats({
      equity,
      balance: engine.balance,
      sessionPnl: equity - INITIAL_BALANCE,
      unrealizedPnl: unrealized,
      realizedPnl: engine.totalRealizedPnl,
      totalTrades: tw,
      totalWins: engine.totalWins,
      totalLosses: engine.totalLosses,
      openPositions: engine.positions.size,
      winRate: tw > 0 ? (engine.totalWins / tw) * 100 : 0,
      warmingUp: engine.bars.length < MIN_BARS,
      lastUpdateAt: now,
      diagnostics: engine.lastError || (engine.lastPrice > 0 ? "Delta Exchange 1m candles (REST)." : "Waiting for candles."),
      feeModelNote: EMPTY_STATS.feeModelNote,
    });
  }, []);

  const processKlines = useCallback(
    (closes: number[], volumes: number[], changePct24h: number, err: string) => {
      if (!loadedRef.current) return;
      const engine = engineRef.current;
      if (closes.length >= MIN_BARS) {
        engine.bars = closes.slice(-MAX_BARS);
        engine.lastPrice = closes[closes.length - 1] ?? 0;
      } else if (err) {
        engine.lastError = err;
        pushDisplay();
        return;
      }
      engine.changePct24h = changePct24h;
      if (closes.length) engine.lastPrice = closes[closes.length - 1] ?? engine.lastPrice;
      if (err) engine.lastError = err;
      else engine.lastError = "";

      const vb = volumes.slice(-VOL_HISTORY);
      engine.volBars = vb;

      const now = Date.now();
      const bars = engine.bars;
      if (bars.length < MIN_BARS) {
        pushDisplay();
        saveLs(engine);
        return;
      }

      const lastVol = vb.length ? vb[vb.length - 1] : 0;
      const avgVol = vb.length >= 3 ? vb.slice(0, -1).reduce((a, b) => a + b, 0) / (vb.length - 1) : 0;
      const volRatio = avgVol > 0 ? lastVol / avgVol : 1;
      const input = buildSignalInputs(bars, volRatio);
      const regime = classifyRegime(input);

      for (const strategy of engine.strategies) {
        if (strategy.position) {
          const price = engine.lastPrice;
          if (price > 0) {
            const u = calcPnl(strategy.position.side, strategy.position.entryPrice, price, strategy.position.quantity);
            const rp = strategy.position.notional > 0 ? (u / strategy.position.notional) * 100 : 0;
            strategy.position = {
              ...strategy.position,
              currentPrice: price,
              unrealizedPnl: u,
              returnPct: rp,
              peakReturnPct: Math.max(strategy.position.peakReturnPct, rp),
            };
            engine.positions.set(strategy.position.id, strategy.position);
            const ex = resolveExit(strategy.position, strategy.def, price, now);
            if (ex) closePosition(engine, strategy, ex.exitPrice, ex.reason, now);
          }
          continue;
        }
        if (strategy.cooldownUntil > now) {
          strategy.status = "COOLING";
          strategy.score = 0;
          continue;
        }
        strategy.status = "READY";
        const raw = evalMinuteSignal(strategy.def.signal, input);
        const score = raw > 0 && volRatio >= VOL_SPIKE_RATIO ? Math.min(99, raw + VOL_BOOST_POINTS) : raw;
        const confirmed = score >= SIGNAL_THRESHOLD && passesEntryConfirmation(strategy.def, input, regime);
        strategy.score = confirmed ? score : Math.min(score, SIGNAL_THRESHOLD - 1);
        strategy.lastSignalSymbol = "BTC";
        if (confirmed && engine.positions.size < MAX_OPEN_POSITIONS && engine.lastPrice > 0) {
          openPosition(engine, strategy, engine.lastPrice, now);
        }
      }

      pushDisplay();
      saveLs(engine);
    },
    [pushDisplay],
  );

  const reset = useCallback(() => {
    engineRef.current = initEngine();
    loadedRef.current = true;
    saveLs(engineRef.current);
    pushDisplay();
  }, [pushDisplay]);

  useEffect(() => {
    const saved = loadLs();
    if (saved) applySaved(engineRef.current, saved);
    loadedRef.current = true;
    pushDisplay();
  }, [pushDisplay]);

  useEffect(() => {
    let cancel = false;
    const tick = async () => {
      if (cancel) return;
      try {
        const res = await fetch("/api/btc/spot-klines", { cache: "no-store" });
        const data = (await res.json()) as {
          ok?: boolean;
          closes?: number[];
          volumes?: number[];
          changePct24h?: number;
          error?: string;
        };
        if (!res.ok || !data.ok || !data.closes?.length) {
          processKlines([], [], 0, data.error || `HTTP ${res.status}`);
          return;
        }
        processKlines(data.closes, data.volumes ?? [], data.changePct24h ?? 0, "");
      } catch {
        processKlines([], [], 0, "Failed to fetch BTC klines.");
      }
    };
    void tick();
    const id = setInterval(() => void tick(), POLL_MS);
    return () => {
      cancel = true;
      clearInterval(id);
    };
  }, [processKlines]);

  return { quote, positions, trades, strategies, stats, reset, initialBalance: INITIAL_BALANCE };
}
