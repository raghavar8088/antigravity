"use client";

/**
 * useNiftyOptionsEngine
 *
 * Fully autonomous, client-side NIFTY 50 option scalping engine.
 * - Subscribes to live NIFTY price via SSE (/api/nifty/stream) — works on Vercel
 * - Pre-seeds 1-minute bars from today's Angel One candles on mount
 * - Runs 20 signal-driven strategies (10 CALL + 10 PUT) identical to the Go engine logic
 * - Manages paper option positions with delta-based mark-to-market, TP/SL auto-exit
 * - Returns the same type shapes as useNiftyOptions so the component needs no UI changes
 */

import { useCallback, useEffect, useRef, useState } from "react";
import type { OptionPosition, OptionTrade, OptionStrategyStatus, OptionStats } from "./useNiftyOptions";
import type { Candle } from "@/app/api/nifty/candles/route";

// ─── Constants ─────────────────────────────────────────────────────────────────

const INITIAL_BALANCE = 1_000_000;   // Rs. 10,00,000 paper account
const LOT_SIZE = 75;                  // NIFTY F&O lot size
const STRIKE_STEP = 50;              // NIFTY strike increment
const NIFTY_IV = 0.18;               // ~18% IV for NIFTY weekly options
const DTE_DAYS = 7;                  // assume 7 trading days to nearest weekly expiry
const MAX_CONCURRENT = 7;
const MAX_BARS = 200;                // keep ~3h 20m of 1-min bars
const TICK_MS = 5_000;               // engine tick interval

// ─── Strategy definitions ──────────────────────────────────────────────────────

interface StratDef {
  id: number;
  name: string;
  category: string;
  optionType: "CALL" | "PUT";
  signal: string;
  tpPct: number;
  slPct: number;
  cooldownSecs: number;
  minBars: number;
  positionINR: number;
}

const STRAT_DEFS: StratDef[] = [
  // ── CALL strategies ───────────────────────────────────────────────────────
  { id: 1,  name: "MomentumBurst_Bull_Call",        category: "Momentum",      optionType: "CALL", signal: "STRONG_BULL_MOM",  tpPct: 0.80, slPct: 0.28, cooldownSecs: 300,  minBars: 15, positionINR: 15000 },
  { id: 2,  name: "ConsecCandle_Bull_Call",          category: "Momentum",      optionType: "CALL", signal: "BULL_MOM",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 180,  minBars: 15, positionINR: 12000 },
  { id: 3,  name: "RSI_Extreme_Oversold_Call",       category: "Mean Reversion",optionType: "CALL", signal: "RSI_EXTREME_OS",   tpPct: 1.00, slPct: 0.30, cooldownSecs: 600,  minBars: 20, positionINR: 18000 },
  { id: 4,  name: "RSI_Oversold_Recovery_Call",      category: "Mean Reversion",optionType: "CALL", signal: "RSI_OVERSOLD",     tpPct: 0.65, slPct: 0.24, cooldownSecs: 480,  minBars: 20, positionINR: 14000 },
  { id: 5,  name: "Overextension_Fade_Call",         category: "Mean Reversion",optionType: "CALL", signal: "BB_LOWER_TOUCH",   tpPct: 0.55, slPct: 0.22, cooldownSecs: 300,  minBars: 22, positionINR: 12000 },
  { id: 6,  name: "EMA_BullCross_Call",              category: "Breakout",      optionType: "CALL", signal: "EMA_BULL_CROSS",   tpPct: 0.65, slPct: 0.25, cooldownSecs: 600,  minBars: 22, positionINR: 14000 },
  { id: 7,  name: "Resistance_Breakout_Call",        category: "Breakout",      optionType: "CALL", signal: "RESIST_BREAK",     tpPct: 0.84, slPct: 0.28, cooldownSecs: 480,  minBars: 22, positionINR: 16000 },
  { id: 8,  name: "Stoch_Oversold_Call",             category: "Mean Reversion",optionType: "CALL", signal: "STOCH_OS",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 360,  minBars: 20, positionINR: 12000 },
  { id: 9,  name: "Capitulation_VReversal_Call",     category: "Capitulation",  optionType: "CALL", signal: "CAPITUL_CALL",     tpPct: 0.90, slPct: 0.35, cooldownSecs: 480,  minBars: 22, positionINR: 18000 },
  { id: 10, name: "BreakoutTrend_Pro_Bull_Call",     category: "Breakout",      optionType: "CALL", signal: "EMA_ABOVE_BOTH",   tpPct: 0.92, slPct: 0.35, cooldownSecs: 720,  minBars: 55, positionINR: 18000 },
  // ── PUT strategies ────────────────────────────────────────────────────────
  { id: 11, name: "MomentumBurst_Bear_Put",          category: "Momentum",      optionType: "PUT",  signal: "STRONG_BEAR_MOM",  tpPct: 0.80, slPct: 0.28, cooldownSecs: 300,  minBars: 15, positionINR: 15000 },
  { id: 12, name: "ConsecCandle_Bear_Put",           category: "Momentum",      optionType: "PUT",  signal: "BEAR_MOM",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 180,  minBars: 15, positionINR: 12000 },
  { id: 13, name: "RSI_Extreme_Overbought_Put",      category: "Mean Reversion",optionType: "PUT",  signal: "RSI_EXTREME_OB",   tpPct: 1.00, slPct: 0.30, cooldownSecs: 600,  minBars: 20, positionINR: 18000 },
  { id: 14, name: "RSI_Overbought_Fade_Put",         category: "Mean Reversion",optionType: "PUT",  signal: "RSI_OVERBOUGHT",   tpPct: 0.65, slPct: 0.24, cooldownSecs: 480,  minBars: 20, positionINR: 14000 },
  { id: 15, name: "Overextension_Fade_Put",          category: "Mean Reversion",optionType: "PUT",  signal: "BB_UPPER_TOUCH",   tpPct: 0.55, slPct: 0.22, cooldownSecs: 300,  minBars: 22, positionINR: 12000 },
  { id: 16, name: "EMA_BearCross_Put",               category: "Breakout",      optionType: "PUT",  signal: "EMA_BEAR_CROSS",   tpPct: 0.65, slPct: 0.25, cooldownSecs: 600,  minBars: 22, positionINR: 14000 },
  { id: 17, name: "Support_Breakdown_Put",           category: "Breakout",      optionType: "PUT",  signal: "SUPPORT_BREAK",    tpPct: 0.84, slPct: 0.28, cooldownSecs: 480,  minBars: 22, positionINR: 16000 },
  { id: 18, name: "Stoch_Overbought_Put",            category: "Mean Reversion",optionType: "PUT",  signal: "STOCH_OB",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 360,  minBars: 20, positionINR: 12000 },
  { id: 19, name: "Capitulation_Reclaim_Elite_Call", category: "Capitulation",  optionType: "CALL", signal: "BB_SQUEEZE_BULL",  tpPct: 1.10, slPct: 0.42, cooldownSecs: 900,  minBars: 40, positionINR: 20000 },
  { id: 20, name: "BreakdownTrend_Pro_Bear_Put",     category: "Breakout",      optionType: "PUT",  signal: "EMA_BELOW_BOTH",   tpPct: 0.92, slPct: 0.35, cooldownSecs: 720,  minBars: 55, positionINR: 18000 },
];

// ─── Math helpers (ported from Go signals.go) ──────────────────────────────────

function ema(bars: number[], period: number): number {
  if (!bars.length) return 0;
  const p = Math.min(period, bars.length);
  const k = 2 / (p + 1);
  let v = bars[0];
  for (let i = 1; i < bars.length; i++) v = bars[i] * k + v * (1 - k);
  return v;
}

function rsi(bars: number[], period: number): number {
  if (bars.length < 2) return 50;
  const slice = bars.slice(-period - 1);
  let gains = 0, losses = 0;
  for (let i = 1; i < slice.length; i++) {
    const d = slice[i] - slice[i - 1];
    if (d > 0) gains += d; else losses -= d;
  }
  if (losses === 0) return 100;
  return 100 - 100 / (1 + gains / losses);
}

function sma(bars: number[], period: number): number {
  const slice = bars.slice(-period);
  return slice.reduce((s, v) => s + v, 0) / slice.length;
}

function stddev(bars: number[]): number {
  if (bars.length < 2) return 0;
  const m = bars.reduce((s, v) => s + v, 0) / bars.length;
  return Math.sqrt(bars.reduce((s, v) => s + (v - m) ** 2, 0) / bars.length);
}

function bbMid(bars: number[], p: number): number { return sma(bars.slice(-p), p); }
function bbUpper(bars: number[], p: number): number {
  const s = bars.slice(-p);
  return bbMid(bars, p) + 2 * stddev(s);
}
function bbLower(bars: number[], p: number): number {
  const s = bars.slice(-p);
  return bbMid(bars, p) - 2 * stddev(s);
}

function momentum(bars: number[], n: number): number {
  if (bars.length <= n) return 0;
  const prev = bars[bars.length - 1 - n];
  return prev === 0 ? 0 : (bars[bars.length - 1] - prev) / prev;
}

function stochK(bars: number[], period: number): number {
  if (bars.length < period) return 50;
  const slice = bars.slice(-period);
  const lo = Math.min(...slice), hi = Math.max(...slice);
  return hi === lo ? 50 : ((bars[bars.length - 1] - lo) / (hi - lo)) * 100;
}

function crossedAbove(bars: number[], fastP: number, slowP: number): boolean {
  if (bars.length < slowP + 2) return false;
  const prev = bars.slice(0, -1);
  return ema(bars, fastP) > ema(bars, slowP) && ema(prev, fastP) <= ema(prev, slowP);
}

function crossedBelow(bars: number[], fastP: number, slowP: number): boolean {
  if (bars.length < slowP + 2) return false;
  const prev = bars.slice(0, -1);
  return ema(bars, fastP) < ema(bars, slowP) && ema(prev, fastP) >= ema(prev, slowP);
}

function avgPrice(bars: number[]): number {
  return bars.reduce((s, v) => s + v, 0) / bars.length;
}

// ─── Signal evaluators (NIFTY-calibrated thresholds) ─────────────────────────

/**
 * NIFTY moves slower than BTC, so momentum thresholds are scaled down:
 *   BTC 5-min strong: 0.0032 → NIFTY: 0.0015
 *   BTC 5-min normal: 0.0018 → NIFTY: 0.0008
 *   BTC resistance break: 0.0018 → NIFTY: 0.0008
 */
function evalSignal(signal: string, bars: number[], price: number): boolean {
  const n = bars.length;
  if (n < 2) return false;

  switch (signal) {
    case "STRONG_BULL_MOM": {
      // ~22 pts over 5 min on NIFTY@22000, short-term burst confirmation
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const m3 = momentum(bars, 3);
      const r = rsi(bars, 14);
      return m5 > 0.0010 && m3 > 0.0004 && r < 72;
    }
    case "BULL_MOM": {
      // ~11 pts over 5 min, price above EMA9
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const r = rsi(bars, 14);
      return m5 > 0.0005 && r < 68 && price > ema(bars, 9);
    }
    case "STRONG_BEAR_MOM": {
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const m3 = momentum(bars, 3);
      const r = rsi(bars, 14);
      return m5 < -0.0010 && m3 < -0.0004 && r > 28;
    }
    case "BEAR_MOM": {
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const r = rsi(bars, 14);
      return m5 < -0.0005 && r > 32 && price < ema(bars, 9);
    }
    case "RSI_EXTREME_OS": {
      // RSI deeply oversold zone + price bouncing (no crossover required)
      if (n < 20) return false;
      const r = rsi(bars, 14);
      return r < 28 && price > bars[n - 2];
    }
    case "RSI_OVERSOLD": {
      // RSI in recovery zone, price reclaiming EMA9
      if (n < 20) return false;
      const r = rsi(bars, 14);
      const e9 = ema(bars, 9);
      return r > 30 && r < 46 && price >= e9 && bars[n - 2] < e9;
    }
    case "RSI_EXTREME_OB": {
      if (n < 20) return false;
      const r = rsi(bars, 14);
      return r > 72 && price < bars[n - 2];
    }
    case "RSI_OVERBOUGHT": {
      if (n < 20) return false;
      const r = rsi(bars, 14);
      const e9 = ema(bars, 9);
      return r > 54 && r < 70 && price <= e9 && bars[n - 2] > e9;
    }
    case "BB_LOWER_TOUCH": {
      // Near or below lower band, bouncing with RSI not overbought
      if (n < 22) return false;
      const bl = bbLower(bars, 20);
      const bm = bbMid(bars, 20);
      const r = rsi(bars, 14);
      return bars[n - 2] <= bl * 1.003 && price > bars[n - 2] && price < bm && r < 52;
    }
    case "BB_UPPER_TOUCH": {
      if (n < 22) return false;
      const bu = bbUpper(bars, 20);
      const bm = bbMid(bars, 20);
      const r = rsi(bars, 14);
      return bars[n - 2] >= bu * 0.997 && price < bars[n - 2] && price > bm && r > 48;
    }
    case "EMA_BULL_CROSS":
      return crossedAbove(bars, 9, 21);
    case "EMA_BEAR_CROSS":
      return crossedBelow(bars, 9, 21);
    case "EMA_ABOVE_BOTH": {
      if (n < 55) return false;
      return price > ema(bars, 20) && price > ema(bars, 50) && crossedAbove(bars, 9, 21);
    }
    case "EMA_BELOW_BOTH": {
      if (n < 55) return false;
      return price < ema(bars, 20) && price < ema(bars, 50) && crossedBelow(bars, 9, 21);
    }
    case "RESIST_BREAK": {
      // Price breaks 20-bar high with momentum confirmation
      if (n < 22) return false;
      const prev = bars.slice(n - 21, n - 1);
      const hi = Math.max(...prev);
      return price > hi * 1.0004 && momentum(bars, 3) > 0.0004;
    }
    case "SUPPORT_BREAK": {
      if (n < 22) return false;
      const prev = bars.slice(n - 21, n - 1);
      const lo = Math.min(...prev);
      return price < lo * 0.9996 && momentum(bars, 3) < -0.0004;
    }
    case "STOCH_OS": {
      // Stoch deeply oversold + price bouncing (no crossover required)
      if (n < 20) return false;
      const k = stochK(bars, 14);
      const r = rsi(bars, 14);
      return k < 25 && price > bars[n - 2] && r < 55;
    }
    case "STOCH_OB": {
      if (n < 20) return false;
      const k = stochK(bars, 14);
      const r = rsi(bars, 14);
      return k > 75 && price < bars[n - 2] && r > 45;
    }
    case "CAPITUL_CALL": {
      if (n < 22) return false;
      const bl = bbLower(bars, 20);
      const isBBLower = bars[n - 2] <= bl * 1.002;
      const isBouncing = price > bars[n - 2];
      const mom5 = momentum(bars, 5);
      return isBBLower && isBouncing && mom5 > 0.0006;
    }
    case "BB_SQUEEZE_BULL": {
      if (n < 40) return false;
      const recentStd = stddev(bars.slice(-10));
      const priorStd = stddev(bars.slice(-30, -10));
      const squeezed = priorStd > 0 && recentStd < priorStd * 0.80;
      return squeezed && momentum(bars, 3) > 0.0005;
    }
  }
  return false;
}

// ─── Market regime ─────────────────────────────────────────────────────────────

function classifyRegime(bars: number[]): string {
  if (bars.length < 22) return "UNKNOWN";
  const price = bars[bars.length - 1];
  const e9 = ema(bars, 9), e21 = ema(bars, 21);
  const r = rsi(bars, 14);
  const s = stddev(bars.slice(-20));
  const volPct = price > 0 ? (s / price) * 100 : 0;
  if (e9 > e21 && r >= 55) return "TRENDING_BULL";
  if (e9 < e21 && r <= 45) return "TRENDING_BEAR";
  if (volPct >= 0.15) return "HIGH_VOL";
  return "RANGE";
}

// ─── Option premium model ──────────────────────────────────────────────────────

function estimatePremium(underlyingPrice: number, iv = NIFTY_IV): number {
  const T = DTE_DAYS / 252;
  const premium = underlyingPrice * iv * Math.sqrt(T) * 0.4;
  // Clamp: 0.2% – 2% of underlying
  return Math.max(underlyingPrice * 0.002, Math.min(underlyingPrice * 0.02, premium));
}

function markPremium(
  entryPremium: number,
  entryUnderlying: number,
  currentUnderlying: number,
  optionType: "CALL" | "PUT",
  barsHeld: number,
): number {
  const delta = 0.5;
  const direction = optionType === "CALL" ? 1 : -1;
  const premiumDelta = delta * (currentUnderlying - entryUnderlying) * direction;
  // Mild theta decay: 0.2% of entry premium per bar
  const thetaDecay = entryPremium * 0.002 * Math.max(0, barsHeld - 1);
  return Math.max(entryPremium * 0.04, entryPremium + premiumDelta - thetaDecay);
}

// ─── Engine internal types ─────────────────────────────────────────────────────

interface InternalPosition {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: "CALL" | "PUT";
  strike: number;
  expiryTime: number; // ms timestamp (7 days from entry)
  entryPremium: number;
  currentPremium: number;
  tpPremium: number;
  slPremium: number;
  quantity: number; // LOT_SIZE
  costBasis: number; // entryPremium × quantity
  entryNiftyPrice: number;
  entryTime: number; // ms
  unrealizedPnl: number;
  iv: number;
  delta: number;
  barsHeld: number;
}

interface InternalStratState {
  def: StratDef;
  position: InternalPosition | null;
  status: "WARMING" | "READY" | "IN_POSITION" | "COOLING" | "DISABLED";
  cooldownUntil: number; // ms
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  lastTradeAt: number; // ms
  score: number;
  regime: string;
}

interface InternalTrade {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: "CALL" | "PUT";
  strike: number;
  expiryMins: number;
  entryPremium: number;
  exitPremium: number;
  quantity: number;
  costBasis: number;
  netPnl: number;
  returnPct: number;
  entryNiftyPrice: number;
  exitNiftyPrice: number;
  entryTime: number;
  exitTime: number;
  exitReason: string;
}

interface EngineRef {
  minuteBars: number[];    // 1-minute sampled NIFTY closes
  lastPrice: number;       // last raw tick price
  lastMinute: number;      // floor(Date.now()/60000) of last bar
  strategies: InternalStratState[];
  positions: Map<string, InternalPosition>;
  trades: InternalTrade[];
  balance: number;
  seq: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
  totalPremiumSpent: number;
}

// ─── Engine helpers ────────────────────────────────────────────────────────────

function atm(price: number): number {
  return Math.round(price / STRIKE_STEP) * STRIKE_STEP;
}

function initEngine(): EngineRef {
  return {
    minuteBars: [],
    lastPrice: 0,
    lastMinute: 0,
    strategies: STRAT_DEFS.map((def) => ({
      def,
      position: null,
      status: "WARMING",
      cooldownUntil: 0,
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
      lastTradeAt: 0,
      score: 0,
      regime: "UNKNOWN",
    })),
    positions: new Map(),
    trades: [],
    balance: INITIAL_BALANCE,
    seq: 0,
    totalWins: 0,
    totalLosses: 0,
    totalRealizedPnl: 0,
    totalPremiumSpent: 0,
  };
}

function closePositionLocked(
  eng: EngineRef,
  strat: InternalStratState,
  exitPremium: number,
  exitPrice: number,
  reason: string,
  now: number,
) {
  const pos = strat.position;
  if (!pos) return;

  const netPnl = (exitPremium - pos.entryPremium) * pos.quantity;
  const returnPct = ((exitPremium - pos.entryPremium) / pos.entryPremium) * 100;

  eng.trades.unshift({
    id: pos.id,
    strategyId: pos.strategyId,
    strategyName: pos.strategyName,
    optionType: pos.optionType,
    strike: pos.strike,
    expiryMins: Math.round((pos.expiryTime - pos.entryTime) / 60000),
    entryPremium: pos.entryPremium,
    exitPremium,
    quantity: pos.quantity,
    costBasis: pos.costBasis,
    netPnl,
    returnPct,
    entryNiftyPrice: pos.entryNiftyPrice,
    exitNiftyPrice: exitPrice,
    entryTime: pos.entryTime,
    exitTime: now,
    exitReason: reason,
  });
  if (eng.trades.length > 500) eng.trades.length = 500;

  eng.balance += exitPremium * pos.quantity;
  eng.totalRealizedPnl += netPnl;

  strat.totalTrades++;
  if (netPnl >= 0) { strat.wins++; eng.totalWins++; }
  else { strat.losses++; eng.totalLosses++; }
  strat.totalPnl += netPnl;
  strat.winRate = strat.totalTrades > 0 ? (strat.wins / strat.totalTrades) * 100 : 0;
  strat.lastTradeAt = now;
  strat.cooldownUntil = now + strat.def.cooldownSecs * 1000;
  strat.status = "COOLING";
  strat.position = null;
  eng.positions.delete(pos.id);
}

function openPositionLocked(
  eng: EngineRef,
  strat: InternalStratState,
  price: number,
  iv: number,
  now: number,
) {
  eng.seq++;
  const premium = estimatePremium(price, iv);
  const cost = premium * LOT_SIZE;

  if (eng.balance < cost) return;

  eng.balance -= cost;
  eng.totalPremiumSpent += cost;

  const id = `NOPT-${now}-${eng.seq}`;
  const expiryTime = now + DTE_DAYS * 24 * 60 * 60 * 1000;

  const pos: InternalPosition = {
    id,
    strategyId: strat.def.id,
    strategyName: strat.def.name,
    optionType: strat.def.optionType,
    strike: atm(price),
    expiryTime,
    entryPremium: premium,
    currentPremium: premium,
    tpPremium: premium * (1 + strat.def.tpPct),
    slPremium: premium * (1 - strat.def.slPct),
    quantity: LOT_SIZE,
    costBasis: cost,
    entryNiftyPrice: price,
    entryTime: now,
    unrealizedPnl: 0,
    iv,
    delta: 0.5,
    barsHeld: 0,
  };

  strat.position = pos;
  strat.status = "IN_POSITION";
  eng.positions.set(id, pos);
}

// ─── Build display state from engine ref ──────────────────────────────────────

function buildDisplayPositions(eng: EngineRef): OptionPosition[] {
  const result: OptionPosition[] = [];
  for (const strat of eng.strategies) {
    if (!strat.position) continue;
    const pos = strat.position;
    result.push({
      id: pos.id,
      strategyId: pos.strategyId,
      strategyName: pos.strategyName,
      optionType: pos.optionType,
      strike: pos.strike,
      expiryTime: new Date(pos.expiryTime).toISOString(),
      entryPremium: pos.entryPremium,
      currentPremium: pos.currentPremium,
      quantity: pos.quantity,
      costBasis: pos.costBasis,
      entryBtcPrice: pos.entryNiftyPrice, // field reused for NIFTY underlying
      entryTime: new Date(pos.entryTime).toISOString(),
      unrealizedPnl: pos.unrealizedPnl,
      iv: pos.iv,
      delta: pos.delta,
    });
  }
  return result;
}

function buildDisplayTrades(eng: EngineRef): OptionTrade[] {
  return eng.trades.slice(0, 100).map((t) => ({
    id: t.id,
    strategyId: t.strategyId,
    strategyName: t.strategyName,
    optionType: t.optionType,
    strike: t.strike,
    expiryMins: t.expiryMins,
    entryPremium: t.entryPremium,
    exitPremium: t.exitPremium,
    quantity: t.quantity,
    costBasis: t.costBasis,
    netPnl: t.netPnl,
    returnPct: t.returnPct,
    entryBtcPrice: t.entryNiftyPrice,
    exitBtcPrice: t.exitNiftyPrice,
    entryTime: new Date(t.entryTime).toISOString(),
    exitTime: new Date(t.exitTime).toISOString(),
    exitReason: t.exitReason,
  }));
}

function buildDisplayStrategies(eng: EngineRef): OptionStrategyStatus[] {
  return eng.strategies.map((s) => ({
    strategyId: s.def.id,
    name: s.def.name,
    category: s.def.category,
    optionType: s.def.optionType,
    rosterState: s.status === "DISABLED" ? "DISABLED" : s.status === "WARMING" ? "WATCHLIST" : "ACTIVE",
    score: s.score,
    regime: s.regime,
    regimeFit: 1,
    allocationUsd: s.def.positionINR,
    totalTrades: s.totalTrades,
    wins: s.wins,
    losses: s.losses,
    totalPnl: s.totalPnl,
    winRate: s.winRate,
    shadowTrades: 0,
    shadowWins: 0,
    shadowLosses: 0,
    shadowPnl: 0,
    shadowWinRate: 0,
    shadowSignals: 0,
    sizeMultiplier: 1,
    disableReason: undefined,
    disabledUntil: undefined,
    lastPromotedAt: s.lastTradeAt > 0 ? new Date(s.lastTradeAt).toISOString() : undefined,
    lastDemotedAt: undefined,
    status: s.status === "WARMING" ? "WATCHLIST"
           : s.status === "COOLING" ? "COOLING"
           : s.status === "IN_POSITION" ? "IN_POSITION"
           : s.status === "DISABLED" ? "DISABLED"
           : "READY",
    hasPosition: !!s.position,
    hasShadowPosition: false,
  }));
}

function buildDisplayStats(eng: EngineRef): OptionStats {
  let openPositions = 0;
  let unrealizedPnl = 0;
  let openCost = 0;

  for (const strat of eng.strategies) {
    if (strat.position) {
      openPositions++;
      unrealizedPnl += strat.position.unrealizedPnl;
      openCost += strat.position.costBasis;
    }
  }

  const equity = eng.balance + openCost + unrealizedPnl;
  const totalTrades = eng.strategies.reduce((s, st) => s + st.totalTrades, 0);
  const totalWinRate = (eng.totalWins + eng.totalLosses) > 0
    ? (eng.totalWins / (eng.totalWins + eng.totalLosses)) * 100
    : 0;

  return {
    balance: eng.balance,
    equity,
    totalTrades,
    openPositions,
    totalWins: eng.totalWins,
    totalLosses: eng.totalLosses,
    winRate: totalWinRate,
    totalPnl: eng.totalRealizedPnl,
    totalPremiumSpent: eng.totalPremiumSpent,
    unrealizedPnl,
  };
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export default function useNiftyOptionsEngine(_refreshKey = 0) {
  const engRef = useRef<EngineRef>(initEngine());

  const [positions, setPositions] = useState<OptionPosition[]>([]);
  const [trades, setTrades] = useState<OptionTrade[]>([]);
  const [strategies, setStrategies] = useState<OptionStrategyStatus[]>(() =>
    buildDisplayStrategies(engRef.current),
  );
  const [stats, setStats] = useState<OptionStats>(() => buildDisplayStats(engRef.current));
  const [barCount, setBarCount] = useState(0);
  const [enginePrice, setEnginePrice] = useState(0);

  const pushDisplayState = useCallback(() => {
    const eng = engRef.current;
    setPositions(buildDisplayPositions(eng));
    setTrades(buildDisplayTrades(eng));
    setStrategies(buildDisplayStrategies(eng));
    setStats(buildDisplayStats(eng));
    setBarCount(eng.minuteBars.length);
    setEnginePrice(eng.lastPrice);
  }, []);

  // ── Engine tick (runs every TICK_MS) ──────────────────────────────────────
  const engineTick = useCallback(() => {
    const eng = engRef.current;
    if (eng.lastPrice <= 0 || eng.minuteBars.length === 0) return;

    const bars = eng.minuteBars;
    const price = eng.lastPrice;
    const now = Date.now();

    // Estimate IV from recent bar volatility
    const recentStd = bars.length >= 20 ? stddev(bars.slice(-20)) : 0;
    const iv = recentStd > 0 ? Math.max(0.12, Math.min(0.35, (recentStd / price) * Math.sqrt(252 * 375))) : NIFTY_IV;

    const regime = classifyRegime(bars);

    // ── Mark open positions, check TP/SL ─────────────────────────────────
    let openCount = 0;
    for (const strat of eng.strategies) {
      if (!strat.position) continue;
      const pos = strat.position;
      pos.barsHeld++;
      pos.currentPremium = markPremium(pos.entryPremium, pos.entryNiftyPrice, price, pos.optionType, pos.barsHeld);
      pos.unrealizedPnl = (pos.currentPremium - pos.entryPremium) * pos.quantity;
      pos.iv = iv;

      // Check TP / SL / Expiry
      if (pos.currentPremium >= pos.tpPremium) {
        closePositionLocked(eng, strat, pos.tpPremium, price, "TP", now);
      } else if (pos.currentPremium <= pos.slPremium) {
        closePositionLocked(eng, strat, pos.slPremium, price, "SL", now);
      } else if (now >= pos.expiryTime) {
        closePositionLocked(eng, strat, pos.currentPremium, price, "EXPIRY", now);
      } else {
        openCount++;
      }
    }

    // ── Evaluate signals for READY strategies ─────────────────────────────
    for (const strat of eng.strategies) {
      strat.regime = regime;

      if (strat.position) { strat.status = "IN_POSITION"; continue; }

      if (strat.status === "COOLING") {
        if (now < strat.cooldownUntil) continue;
        strat.status = "READY";
        strat.cooldownUntil = 0;
      }

      if (bars.length < strat.def.minBars) { strat.status = "WARMING"; continue; }

      strat.status = "READY";

      if (openCount >= MAX_CONCURRENT) continue;
      if (eng.balance < strat.def.positionINR) continue;

      const fires = evalSignal(strat.def.signal, bars, price);
      strat.score = fires ? 75 : 0;

      if (fires) {
        openPositionLocked(eng, strat, price, iv, now);
        openCount++;
      }
    }

    pushDisplayState();
  }, [pushDisplayState]);

  // ── Feed price tick: build 1-minute bars ─────────────────────────────────
  const feedPrice = useCallback((price: number) => {
    if (price <= 0) return;
    const eng = engRef.current;
    eng.lastPrice = price;

    const nowMinute = Math.floor(Date.now() / 60000);
    if (nowMinute !== eng.lastMinute) {
      eng.lastMinute = nowMinute;
      eng.minuteBars.push(price);
      if (eng.minuteBars.length > MAX_BARS) eng.minuteBars.shift();
    }
  }, []);

  // ── Subscribe to NIFTY SSE price stream ──────────────────────────────────
  useEffect(() => {
    let cancelled = false;
    const source = new EventSource("/api/nifty/stream");

    source.onmessage = (e) => {
      if (cancelled) return;
      try {
        const data = JSON.parse(e.data as string) as { price?: number };
        if (data.price && data.price > 0) feedPrice(data.price);
      } catch { /* ignore */ }
    };

    source.onerror = () => { /* EventSource auto-reconnects */ };
    return () => { cancelled = true; source.close(); };
  }, [feedPrice]);

  // ── Pre-seed from today's 1-min candles ──────────────────────────────────
  useEffect(() => {
    const seed = async () => {
      try {
        const res = await fetch("/api/nifty/candles?interval=ONE_MINUTE");
        if (!res.ok) return;
        const data = await res.json() as { ok: boolean; candles?: Candle[] };
        if (!data.ok || !data.candles?.length) return;
        const eng = engRef.current;
        // Only seed if we have no bars yet
        if (eng.minuteBars.length > 0) return;
        const closes = data.candles.map((c) => c.close).filter((v) => v > 0);
        eng.minuteBars = closes.slice(-MAX_BARS);
        eng.lastPrice = closes[closes.length - 1] ?? 0;
        const lastMinute = data.candles[data.candles.length - 1]?.time ?? 0;
        eng.lastMinute = Math.floor(lastMinute / 60000);
        pushDisplayState();
      } catch { /* silent */ }
    };
    void seed();
  }, [pushDisplayState]);

  // ── Engine tick interval ──────────────────────────────────────────────────
  useEffect(() => {
    const id = setInterval(() => void engineTick(), TICK_MS);
    return () => clearInterval(id);
  }, [engineTick]);

  // ── Reset ─────────────────────────────────────────────────────────────────
  const clearAll = useCallback(() => {
    engRef.current = initEngine();
    pushDisplayState();
  }, [pushDisplayState]);

  return { positions, trades, strategies, stats, clearAll, barCount, enginePrice };
}
