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
const MAX_CONCURRENT = 15;
const MAX_BARS = 200;                // keep ~3h 20m of 1-min bars
const TICK_MS = 1_000;               // engine tick every second

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
  { id: 1,  name: "MomentumBurst_Bull_Call",        category: "Momentum",      optionType: "CALL", signal: "STRONG_BULL_MOM",  tpPct: 0.80, slPct: 0.28, cooldownSecs: 120,  minBars: 3, positionINR: 15000 },
  { id: 2,  name: "ConsecCandle_Bull_Call",          category: "Momentum",      optionType: "CALL", signal: "BULL_MOM",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 60,   minBars: 3, positionINR: 12000 },
  { id: 3,  name: "RSI_Extreme_Oversold_Call",       category: "Mean Reversion",optionType: "CALL", signal: "RSI_EXTREME_OS",   tpPct: 1.00, slPct: 0.30, cooldownSecs: 240,  minBars: 5, positionINR: 18000 },
  { id: 4,  name: "RSI_Oversold_Recovery_Call",      category: "Mean Reversion",optionType: "CALL", signal: "RSI_OVERSOLD",     tpPct: 0.65, slPct: 0.24, cooldownSecs: 180,  minBars: 5, positionINR: 14000 },
  { id: 5,  name: "Overextension_Fade_Call",         category: "Mean Reversion",optionType: "CALL", signal: "BB_LOWER_TOUCH",   tpPct: 0.55, slPct: 0.22, cooldownSecs: 120,  minBars: 8, positionINR: 12000 },
  { id: 6,  name: "EMA_BullCross_Call",              category: "Breakout",      optionType: "CALL", signal: "EMA_BULL_CROSS",   tpPct: 0.65, slPct: 0.25, cooldownSecs: 180,  minBars: 8, positionINR: 14000 },
  { id: 7,  name: "Resistance_Breakout_Call",        category: "Breakout",      optionType: "CALL", signal: "RESIST_BREAK",     tpPct: 0.84, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 8,  name: "Stoch_Oversold_Call",             category: "Mean Reversion",optionType: "CALL", signal: "STOCH_OS",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 120,  minBars: 5, positionINR: 12000 },
  { id: 9,  name: "Capitulation_VReversal_Call",     category: "Capitulation",  optionType: "CALL", signal: "CAPITUL_CALL",     tpPct: 0.90, slPct: 0.35, cooldownSecs: 180,  minBars: 8, positionINR: 18000 },
  { id: 10, name: "BreakoutTrend_Pro_Bull_Call",     category: "Breakout",      optionType: "CALL", signal: "EMA_ABOVE_BOTH",   tpPct: 0.92, slPct: 0.35, cooldownSecs: 240,  minBars: 15, positionINR: 18000 },
  // ── PUT strategies ────────────────────────────────────────────────────────
  { id: 11, name: "MomentumBurst_Bear_Put",          category: "Momentum",      optionType: "PUT",  signal: "STRONG_BEAR_MOM",  tpPct: 0.80, slPct: 0.28, cooldownSecs: 120,  minBars: 3, positionINR: 15000 },
  { id: 12, name: "ConsecCandle_Bear_Put",           category: "Momentum",      optionType: "PUT",  signal: "BEAR_MOM",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 60,   minBars: 3, positionINR: 12000 },
  { id: 13, name: "RSI_Extreme_Overbought_Put",      category: "Mean Reversion",optionType: "PUT",  signal: "RSI_EXTREME_OB",   tpPct: 1.00, slPct: 0.30, cooldownSecs: 240,  minBars: 5, positionINR: 18000 },
  { id: 14, name: "RSI_Overbought_Fade_Put",         category: "Mean Reversion",optionType: "PUT",  signal: "RSI_OVERBOUGHT",   tpPct: 0.65, slPct: 0.24, cooldownSecs: 180,  minBars: 5, positionINR: 14000 },
  { id: 15, name: "Overextension_Fade_Put",          category: "Mean Reversion",optionType: "PUT",  signal: "BB_UPPER_TOUCH",   tpPct: 0.55, slPct: 0.22, cooldownSecs: 120,  minBars: 8, positionINR: 12000 },
  { id: 16, name: "EMA_BearCross_Put",               category: "Breakout",      optionType: "PUT",  signal: "EMA_BEAR_CROSS",   tpPct: 0.65, slPct: 0.25, cooldownSecs: 180,  minBars: 8, positionINR: 14000 },
  { id: 17, name: "Support_Breakdown_Put",           category: "Breakout",      optionType: "PUT",  signal: "SUPPORT_BREAK",    tpPct: 0.84, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 18, name: "Stoch_Overbought_Put",            category: "Mean Reversion",optionType: "PUT",  signal: "STOCH_OB",         tpPct: 0.55, slPct: 0.22, cooldownSecs: 120,  minBars: 5, positionINR: 12000 },
  { id: 19, name: "Capitulation_Reclaim_Elite_Call", category: "Capitulation",  optionType: "CALL", signal: "BB_SQUEEZE_BULL",  tpPct: 1.10, slPct: 0.42, cooldownSecs: 300,  minBars: 12, positionINR: 20000 },
  { id: 20, name: "BreakdownTrend_Pro_Bear_Put",     category: "Breakout",      optionType: "PUT",  signal: "EMA_BELOW_BOTH",   tpPct: 0.92, slPct: 0.35, cooldownSecs: 240,  minBars: 15, positionINR: 18000 },

  // ── Extended CALL strategies ──────────────────────────────────────────────
  { id: 21, name: "VWAP_Reclaim_Bull_Call",          category: "VWAP",          optionType: "CALL", signal: "VWAP_BULL_RECLAIM", tpPct: 0.70, slPct: 0.24, cooldownSecs: 150,  minBars: 8, positionINR: 14000 },
  { id: 22, name: "EMA5_FastPull_Bull_Call",         category: "Momentum",      optionType: "CALL", signal: "EMA5_BULL_PULL",    tpPct: 0.55, slPct: 0.20, cooldownSecs: 90,   minBars: 5, positionINR: 12000 },
  { id: 23, name: "OpenRange_Break_Bull_Call",       category: "Breakout",      optionType: "CALL", signal: "ORB_BULL",          tpPct: 0.90, slPct: 0.30, cooldownSecs: 240,  minBars: 10, positionINR: 16000 },
  { id: 24, name: "Trend_Pullback_Bull_Call",        category: "Trend",         optionType: "CALL", signal: "TREND_PULLBACK_BULL",tpPct: 0.75, slPct: 0.26, cooldownSecs: 150, minBars: 8, positionINR: 15000 },
  { id: 25, name: "HH_HL_Continuation_Call",        category: "Price Action",   optionType: "CALL", signal: "HH_HL_BULL",        tpPct: 0.65, slPct: 0.22, cooldownSecs: 120,  minBars: 5, positionINR: 13000 },
  { id: 26, name: "BB_MidBand_Bounce_Call",         category: "Mean Reversion", optionType: "CALL", signal: "BB_MID_BOUNCE_BULL",tpPct: 0.55, slPct: 0.22, cooldownSecs: 120,  minBars: 8, positionINR: 12000 },
  { id: 27, name: "MultiEMA_Stack_Bull_Call",       category: "Trend",          optionType: "CALL", signal: "EMA_STACK_BULL",    tpPct: 0.80, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 28, name: "RSI_Midline_Bull_Cross_Call",    category: "Mean Reversion", optionType: "CALL", signal: "RSI_MID_BULL",      tpPct: 0.60, slPct: 0.22, cooldownSecs: 120,  minBars: 5, positionINR: 13000 },
  { id: 29, name: "Squeeze_Momentum_Bull_Call",     category: "Volatility",     optionType: "CALL", signal: "SQUEEZE_FIRE_BULL", tpPct: 1.00, slPct: 0.35, cooldownSecs: 240,  minBars: 10, positionINR: 18000 },
  { id: 30, name: "Higher_Low_Reversal_Call",       category: "Price Action",   optionType: "CALL", signal: "HIGHER_LOW_BULL",   tpPct: 0.70, slPct: 0.25, cooldownSecs: 150,  minBars: 5, positionINR: 14000 },

  // ── Extended PUT strategies ───────────────────────────────────────────────
  { id: 31, name: "VWAP_Rejection_Bear_Put",         category: "VWAP",          optionType: "PUT",  signal: "VWAP_BEAR_REJECT",  tpPct: 0.70, slPct: 0.24, cooldownSecs: 150,  minBars: 8, positionINR: 14000 },
  { id: 32, name: "EMA5_FastPull_Bear_Put",          category: "Momentum",      optionType: "PUT",  signal: "EMA5_BEAR_PULL",    tpPct: 0.55, slPct: 0.20, cooldownSecs: 90,   minBars: 5, positionINR: 12000 },
  { id: 33, name: "OpenRange_Break_Bear_Put",        category: "Breakout",      optionType: "PUT",  signal: "ORB_BEAR",          tpPct: 0.90, slPct: 0.30, cooldownSecs: 240,  minBars: 10, positionINR: 16000 },
  { id: 34, name: "Trend_Pullback_Bear_Put",         category: "Trend",         optionType: "PUT",  signal: "TREND_PULLBACK_BEAR",tpPct: 0.75, slPct: 0.26, cooldownSecs: 150, minBars: 8, positionINR: 15000 },
  { id: 35, name: "LH_LL_Continuation_Put",         category: "Price Action",   optionType: "PUT",  signal: "LH_LL_BEAR",        tpPct: 0.65, slPct: 0.22, cooldownSecs: 120,  minBars: 5, positionINR: 13000 },
  { id: 36, name: "BB_MidBand_Reject_Put",          category: "Mean Reversion", optionType: "PUT",  signal: "BB_MID_REJECT_BEAR",tpPct: 0.55, slPct: 0.22, cooldownSecs: 120,  minBars: 8, positionINR: 12000 },
  { id: 37, name: "MultiEMA_Stack_Bear_Put",        category: "Trend",          optionType: "PUT",  signal: "EMA_STACK_BEAR",    tpPct: 0.80, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 38, name: "RSI_Midline_Bear_Cross_Put",     category: "Mean Reversion", optionType: "PUT",  signal: "RSI_MID_BEAR",      tpPct: 0.60, slPct: 0.22, cooldownSecs: 120,  minBars: 5, positionINR: 13000 },
  { id: 39, name: "Squeeze_Momentum_Bear_Put",      category: "Volatility",     optionType: "PUT",  signal: "SQUEEZE_FIRE_BEAR", tpPct: 1.00, slPct: 0.35, cooldownSecs: 240,  minBars: 10, positionINR: 18000 },
  { id: 40, name: "Lower_High_Reversal_Put",        category: "Price Action",   optionType: "PUT",  signal: "LOWER_HIGH_BEAR",   tpPct: 0.70, slPct: 0.25, cooldownSecs: 150,  minBars: 5, positionINR: 14000 },

  // ── Advanced CALL strategies (ids 41-50) ─────────────────────────────────
  { id: 41, name: "Fib_618_Bounce_Call",            category: "Fibonacci",      optionType: "CALL", signal: "FIB_618_BULL",       tpPct: 0.85, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 42, name: "Hammer_Candle_Call",             category: "Price Action",   optionType: "CALL", signal: "HAMMER_BULL",        tpPct: 0.70, slPct: 0.24, cooldownSecs: 120,  minBars: 5, positionINR: 14000 },
  { id: 43, name: "Three_Bar_Thrust_Call",          category: "Momentum",       optionType: "CALL", signal: "THREE_BAR_BULL",     tpPct: 0.75, slPct: 0.25, cooldownSecs: 120,  minBars: 5, positionINR: 15000 },
  { id: 44, name: "RSI_Divergence_Bull_Call",       category: "Divergence",     optionType: "CALL", signal: "RSI_DIVERGE_BULL",   tpPct: 0.90, slPct: 0.30, cooldownSecs: 240,  minBars: 8, positionINR: 17000 },
  { id: 45, name: "Vol_Contraction_Bull_Call",      category: "Volatility",     optionType: "CALL", signal: "VOL_CONTRACT_BULL",  tpPct: 0.80, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 15000 },
  { id: 46, name: "Inside_Bar_Breakout_Call",       category: "Price Action",   optionType: "CALL", signal: "INSIDE_BAR_BULL",    tpPct: 0.75, slPct: 0.26, cooldownSecs: 150,  minBars: 5, positionINR: 14000 },
  { id: 47, name: "Momentum_Diverge_Recovery_Call", category: "Divergence",     optionType: "CALL", signal: "MOM_DIVERGE_BULL",   tpPct: 0.80, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 48, name: "Range_Expansion_Bull_Call",      category: "Volatility",     optionType: "CALL", signal: "RANGE_EXPAND_BULL",  tpPct: 0.85, slPct: 0.28, cooldownSecs: 150,  minBars: 5, positionINR: 16000 },
  { id: 49, name: "ADX_Trend_Strength_Call",        category: "Trend",          optionType: "CALL", signal: "ADX_BULL_TREND",     tpPct: 0.90, slPct: 0.30, cooldownSecs: 240,  minBars: 10, positionINR: 18000 },
  { id: 50, name: "Parabolic_SAR_Bull_Call",        category: "Trend",          optionType: "CALL", signal: "PSAR_BULL_FLIP",     tpPct: 0.75, slPct: 0.25, cooldownSecs: 150,  minBars: 5, positionINR: 15000 },

  // ── Advanced PUT strategies (ids 51-60) ──────────────────────────────────
  { id: 51, name: "Fib_618_Rejection_Put",          category: "Fibonacci",      optionType: "PUT",  signal: "FIB_618_BEAR",       tpPct: 0.85, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 52, name: "ShootingStar_Candle_Put",        category: "Price Action",   optionType: "PUT",  signal: "SHOOTING_STAR_BEAR", tpPct: 0.70, slPct: 0.24, cooldownSecs: 120,  minBars: 5, positionINR: 14000 },
  { id: 53, name: "Three_Bar_Thrust_Put",           category: "Momentum",       optionType: "PUT",  signal: "THREE_BAR_BEAR",     tpPct: 0.75, slPct: 0.25, cooldownSecs: 120,  minBars: 5, positionINR: 15000 },
  { id: 54, name: "RSI_Divergence_Bear_Put",        category: "Divergence",     optionType: "PUT",  signal: "RSI_DIVERGE_BEAR",   tpPct: 0.90, slPct: 0.30, cooldownSecs: 240,  minBars: 8, positionINR: 17000 },
  { id: 55, name: "Vol_Contraction_Bear_Put",       category: "Volatility",     optionType: "PUT",  signal: "VOL_CONTRACT_BEAR",  tpPct: 0.80, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 15000 },
  { id: 56, name: "Inside_Bar_Breakdown_Put",       category: "Price Action",   optionType: "PUT",  signal: "INSIDE_BAR_BEAR",    tpPct: 0.75, slPct: 0.26, cooldownSecs: 150,  minBars: 5, positionINR: 14000 },
  { id: 57, name: "Momentum_Diverge_Fade_Put",      category: "Divergence",     optionType: "PUT",  signal: "MOM_DIVERGE_BEAR",   tpPct: 0.80, slPct: 0.28, cooldownSecs: 180,  minBars: 8, positionINR: 16000 },
  { id: 58, name: "Range_Expansion_Bear_Put",       category: "Volatility",     optionType: "PUT",  signal: "RANGE_EXPAND_BEAR",  tpPct: 0.85, slPct: 0.28, cooldownSecs: 150,  minBars: 5, positionINR: 16000 },
  { id: 59, name: "ADX_Trend_Strength_Put",         category: "Trend",          optionType: "PUT",  signal: "ADX_BEAR_TREND",     tpPct: 0.90, slPct: 0.30, cooldownSecs: 240,  minBars: 10, positionINR: 18000 },
  { id: 60, name: "Parabolic_SAR_Bear_Put",         category: "Trend",          optionType: "PUT",  signal: "PSAR_BEAR_FLIP",     tpPct: 0.75, slPct: 0.25, cooldownSecs: 150,  minBars: 5, positionINR: 15000 },
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
      // Strong upward momentum: ~7 pts over 5 min on NIFTY@22000
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const m3 = momentum(bars, 3);
      const r = rsi(bars, 14);
      return m5 > 0.0006 && m3 > 0.0002 && r < 74;
    }
    case "BULL_MOM": {
      // Mild upward momentum: price above EMA9 + positive 5-bar drift
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const r = rsi(bars, 14);
      return m5 > 0.0003 && r < 70 && price > ema(bars, 9);
    }
    case "STRONG_BEAR_MOM": {
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const m3 = momentum(bars, 3);
      const r = rsi(bars, 14);
      return m5 < -0.0006 && m3 < -0.0002 && r > 26;
    }
    case "BEAR_MOM": {
      if (n < 15) return false;
      const m5 = momentum(bars, 5);
      const r = rsi(bars, 14);
      return m5 < -0.0003 && r > 30 && price < ema(bars, 9);
    }
    case "RSI_EXTREME_OS": {
      if (n < 14) return false;
      const r = rsi(bars, 14);
      return r < 32 && price >= bars[n - 2];
    }
    case "RSI_OVERSOLD": {
      if (n < 14) return false;
      const r = rsi(bars, 14);
      const e9 = ema(bars, 9);
      return r > 32 && r < 48 && price >= e9;
    }
    case "RSI_EXTREME_OB": {
      if (n < 14) return false;
      const r = rsi(bars, 14);
      return r > 68 && price <= bars[n - 2];
    }
    case "RSI_OVERBOUGHT": {
      if (n < 14) return false;
      const r = rsi(bars, 14);
      const e9 = ema(bars, 9);
      return r > 52 && r < 68 && price <= e9;
    }
    case "BB_LOWER_TOUCH": {
      if (n < 20) return false;
      const bl = bbLower(bars, 20);
      const bm = bbMid(bars, 20);
      const r = rsi(bars, 14);
      return price <= bl * 1.005 && price < bm && r < 52;
    }
    case "BB_UPPER_TOUCH": {
      if (n < 20) return false;
      const bu = bbUpper(bars, 20);
      const bm = bbMid(bars, 20);
      const r = rsi(bars, 14);
      return price >= bu * 0.995 && price > bm && r > 48;
    }
    case "EMA_BULL_CROSS": {
      // State-based: EMA9 currently above EMA21 with upward momentum
      if (n < 22) return false;
      const e9 = ema(bars, 9), e21 = ema(bars, 21);
      return e9 > e21 && momentum(bars, 3) > 0.0002;
    }
    case "EMA_BEAR_CROSS": {
      if (n < 22) return false;
      const e9 = ema(bars, 9), e21 = ema(bars, 21);
      return e9 < e21 && momentum(bars, 3) < -0.0002;
    }
    case "EMA_ABOVE_BOTH": {
      if (n < 50) return false;
      const e9 = ema(bars, 9), e21 = ema(bars, 21);
      return price > ema(bars, 20) && price > ema(bars, 50) && e9 > e21 && momentum(bars, 5) > 0.0003;
    }
    case "EMA_BELOW_BOTH": {
      if (n < 50) return false;
      const e9 = ema(bars, 9), e21 = ema(bars, 21);
      return price < ema(bars, 20) && price < ema(bars, 50) && e9 < e21 && momentum(bars, 5) < -0.0003;
    }
    case "RESIST_BREAK": {
      if (n < 22) return false;
      const hi = Math.max(...bars.slice(n - 21, n - 1));
      return price > hi * 1.0002 && momentum(bars, 3) > 0.0002;
    }
    case "SUPPORT_BREAK": {
      if (n < 22) return false;
      const lo = Math.min(...bars.slice(n - 21, n - 1));
      return price < lo * 0.9998 && momentum(bars, 3) < -0.0002;
    }
    case "STOCH_OS": {
      if (n < 14) return false;
      return stochK(bars, 14) < 28 && price >= bars[n - 2] && rsi(bars, 14) < 55;
    }
    case "STOCH_OB": {
      if (n < 14) return false;
      return stochK(bars, 14) > 72 && price <= bars[n - 2] && rsi(bars, 14) > 45;
    }
    case "CAPITUL_CALL": {
      if (n < 20) return false;
      const bl = bbLower(bars, 20);
      return price <= bl * 1.004 && price >= bars[n - 2] && momentum(bars, 5) > 0.0003;
    }
    case "BB_SQUEEZE_BULL": {
      if (n < 30) return false;
      const recentStd = stddev(bars.slice(-10));
      const priorStd = stddev(bars.slice(-25, -10));
      return priorStd > 0 && recentStd < priorStd * 0.85 && momentum(bars, 3) > 0.0002;
    }

    // ── New signals ─────────────────────────────────────────────────────────

    case "VWAP_BULL_RECLAIM": {
      // Price reclaims VWAP (approximated as SMA20) from below with RSI rising
      if (n < 20) return false;
      const vwap = sma(bars, 20);
      const prev = bars[n - 2];
      const r = rsi(bars, 14);
      return prev < vwap && price >= vwap && r > 40 && r < 65;
    }
    case "VWAP_BEAR_REJECT": {
      if (n < 20) return false;
      const vwap = sma(bars, 20);
      const prev = bars[n - 2];
      const r = rsi(bars, 14);
      return prev > vwap && price <= vwap && r > 35 && r < 60;
    }

    case "EMA5_BULL_PULL": {
      // Price pulls back to EMA5 in uptrend and bounces
      if (n < 10) return false;
      const e5 = ema(bars, 5), e9 = ema(bars, 9);
      return e9 > e5 * 0.998 && price >= e5 * 0.999 && price <= e5 * 1.002
        && bars[n - 2] < e5 && momentum(bars, 3) > 0.0001;
    }
    case "EMA5_BEAR_PULL": {
      if (n < 10) return false;
      const e5 = ema(bars, 5), e9 = ema(bars, 9);
      return e9 < e5 * 1.002 && price <= e5 * 1.001 && price >= e5 * 0.998
        && bars[n - 2] > e5 && momentum(bars, 3) < -0.0001;
    }

    case "ORB_BULL": {
      // Opening Range Breakout bull: price > first 30-bar high with RSI momentum
      if (n < 30) return false;
      const orbHigh = Math.max(...bars.slice(0, Math.min(30, n)));
      return price > orbHigh * 1.0002 && momentum(bars, 5) > 0.0003 && rsi(bars, 14) > 50;
    }
    case "ORB_BEAR": {
      if (n < 30) return false;
      const orbLow = Math.min(...bars.slice(0, Math.min(30, n)));
      return price < orbLow * 0.9998 && momentum(bars, 5) < -0.0003 && rsi(bars, 14) < 50;
    }

    case "TREND_PULLBACK_BULL": {
      // Bullish trend pullback: EMA9 > EMA21, price dipped below EMA9 and recovered
      if (n < 25) return false;
      const e9 = ema(bars, 9), e21 = ema(bars, 21);
      return e9 > e21 && bars[n - 3] < e9 && price >= e9 && momentum(bars, 3) > 0.0002;
    }
    case "TREND_PULLBACK_BEAR": {
      if (n < 25) return false;
      const e9 = ema(bars, 9), e21 = ema(bars, 21);
      return e9 < e21 && bars[n - 3] > e9 && price <= e9 && momentum(bars, 3) < -0.0002;
    }

    case "HH_HL_BULL": {
      // Higher Highs + Higher Lows: last 3 bars show ascending structure
      if (n < 15) return false;
      const b1 = bars[n - 4], b2 = bars[n - 3], b3 = bars[n - 2];
      return b2 > b1 && b3 > b2 * 0.9998 && price >= b3 && rsi(bars, 14) < 70;
    }
    case "LH_LL_BEAR": {
      if (n < 15) return false;
      const b1 = bars[n - 4], b2 = bars[n - 3], b3 = bars[n - 2];
      return b2 < b1 && b3 < b2 * 1.0002 && price <= b3 && rsi(bars, 14) > 30;
    }

    case "BB_MID_BOUNCE_BULL": {
      // Price bounces off BB midline from below in uptrend
      if (n < 22) return false;
      const bm = bbMid(bars, 20);
      const e9 = ema(bars, 9);
      return bars[n - 2] < bm && price >= bm && e9 > bm * 0.999 && rsi(bars, 14) > 45;
    }
    case "BB_MID_REJECT_BEAR": {
      if (n < 22) return false;
      const bm = bbMid(bars, 20);
      const e9 = ema(bars, 9);
      return bars[n - 2] > bm && price <= bm && e9 < bm * 1.001 && rsi(bars, 14) < 55;
    }

    case "EMA_STACK_BULL": {
      // Full EMA stack: EMA5 > EMA9 > EMA21, momentum positive
      if (n < 25) return false;
      const e5 = ema(bars, 5), e9 = ema(bars, 9), e21 = ema(bars, 21);
      return e5 > e9 && e9 > e21 && price > e5 && momentum(bars, 5) > 0.0002;
    }
    case "EMA_STACK_BEAR": {
      if (n < 25) return false;
      const e5 = ema(bars, 5), e9 = ema(bars, 9), e21 = ema(bars, 21);
      return e5 < e9 && e9 < e21 && price < e5 && momentum(bars, 5) < -0.0002;
    }

    case "RSI_MID_BULL": {
      // RSI crosses above 50 midline with price above EMA9
      if (n < 14) return false;
      const r = rsi(bars, 14);
      const prev = rsi(bars.slice(0, -1), 14);
      return prev < 50 && r >= 50 && price > ema(bars, 9);
    }
    case "RSI_MID_BEAR": {
      if (n < 14) return false;
      const r = rsi(bars, 14);
      const prev = rsi(bars.slice(0, -1), 14);
      return prev > 50 && r <= 50 && price < ema(bars, 9);
    }

    case "SQUEEZE_FIRE_BULL": {
      // Bollinger squeeze release: narrow bands expanding + bull momentum
      if (n < 25) return false;
      const recent = stddev(bars.slice(-5));
      const prior = stddev(bars.slice(-20, -5));
      const expanding = prior > 0 && recent > prior * 1.15;
      return expanding && momentum(bars, 5) > 0.0004 && rsi(bars, 14) > 50;
    }
    case "SQUEEZE_FIRE_BEAR": {
      if (n < 25) return false;
      const recent = stddev(bars.slice(-5));
      const prior = stddev(bars.slice(-20, -5));
      const expanding = prior > 0 && recent > prior * 1.15;
      return expanding && momentum(bars, 5) < -0.0004 && rsi(bars, 14) < 50;
    }

    case "HIGHER_LOW_BULL": {
      // Higher Low pattern: recent trough higher than previous trough
      if (n < 15) return false;
      const recentLow = Math.min(...bars.slice(n - 5, n - 1));
      const priorLow = Math.min(...bars.slice(n - 12, n - 6));
      return recentLow > priorLow && price > bars[n - 2] && rsi(bars, 14) < 65;
    }
    case "LOWER_HIGH_BEAR": {
      if (n < 15) return false;
      const recentHigh = Math.max(...bars.slice(n - 5, n - 1));
      const priorHigh = Math.max(...bars.slice(n - 12, n - 6));
      return recentHigh < priorHigh && price < bars[n - 2] && rsi(bars, 14) > 35;
    }

    // ── Fibonacci 61.8% retracement ──────────────────────────────────────────
    case "FIB_618_BULL": {
      // Price retraces 61.8% of recent swing low→high and holds
      if (n < 20) return false;
      const swingHigh = Math.max(...bars.slice(n - 20, n - 5));
      const swingLow  = Math.min(...bars.slice(n - 20, n - 5));
      const fib618 = swingHigh - (swingHigh - swingLow) * 0.618;
      return price >= fib618 * 0.998 && price <= fib618 * 1.004 &&
             momentum(bars, 3) > 0.0001 && rsi(bars, 14) > 38;
    }
    case "FIB_618_BEAR": {
      if (n < 20) return false;
      const swingHigh = Math.max(...bars.slice(n - 20, n - 5));
      const swingLow  = Math.min(...bars.slice(n - 20, n - 5));
      const fib618 = swingLow + (swingHigh - swingLow) * 0.618;
      return price <= fib618 * 1.002 && price >= fib618 * 0.996 &&
             momentum(bars, 3) < -0.0001 && rsi(bars, 14) < 62;
    }

    // ── Hammer / Shooting Star candlestick (price-action proxy) ─────────────
    case "HAMMER_BULL": {
      // Hammer: price dipped significantly below open and recovered near close
      if (n < 10) return false;
      const open = bars[n - 2];
      const low  = Math.min(...bars.slice(n - 4, n - 1));
      const wick = open - low;
      const body = Math.abs(price - open);
      // Long lower wick (≥2× body), closing near top
      return wick > body * 2 && price > open && wick > open * 0.0015 &&
             rsi(bars, 14) < 55;
    }
    case "SHOOTING_STAR_BEAR": {
      if (n < 10) return false;
      const open = bars[n - 2];
      const high = Math.max(...bars.slice(n - 4, n - 1));
      const wick = high - open;
      const body = Math.abs(price - open);
      return wick > body * 2 && price < open && wick > open * 0.0015 &&
             rsi(bars, 14) > 45;
    }

    // ── Three consecutive bars in same direction ──────────────────────────────
    case "THREE_BAR_BULL": {
      if (n < 10) return false;
      const b1 = bars[n - 4], b2 = bars[n - 3], b3 = bars[n - 2];
      return b2 > b1 && b3 > b2 && price > b3 &&
             momentum(bars, 4) > 0.0002 && rsi(bars, 14) < 72;
    }
    case "THREE_BAR_BEAR": {
      if (n < 10) return false;
      const b1 = bars[n - 4], b2 = bars[n - 3], b3 = bars[n - 2];
      return b2 < b1 && b3 < b2 && price < b3 &&
             momentum(bars, 4) < -0.0002 && rsi(bars, 14) > 28;
    }

    // ── RSI divergence: price makes lower low but RSI makes higher low ────────
    case "RSI_DIVERGE_BULL": {
      if (n < 20) return false;
      const priceRecLow  = Math.min(...bars.slice(n - 5,  n - 1));
      const pricePriorLow = Math.min(...bars.slice(n - 15, n - 6));
      const rsiRecLow    = rsi(bars.slice(n - 5),  14);
      const rsiPriorLow  = rsi(bars.slice(n - 15, n - 5), 14);
      return priceRecLow < pricePriorLow && rsiRecLow > rsiPriorLow &&
             price > priceRecLow && rsi(bars, 14) < 50;
    }
    case "RSI_DIVERGE_BEAR": {
      if (n < 20) return false;
      const priceRecHigh  = Math.max(...bars.slice(n - 5,  n - 1));
      const pricePriorHigh = Math.max(...bars.slice(n - 15, n - 6));
      const rsiRecHigh    = rsi(bars.slice(n - 5),  14);
      const rsiPriorHigh  = rsi(bars.slice(n - 15, n - 5), 14);
      return priceRecHigh > pricePriorHigh && rsiRecHigh < rsiPriorHigh &&
             price < priceRecHigh && rsi(bars, 14) > 50;
    }

    // ── Volatility contraction → directional move ─────────────────────────────
    case "VOL_CONTRACT_BULL": {
      if (n < 20) return false;
      const recentRange = Math.max(...bars.slice(n - 5, n)) - Math.min(...bars.slice(n - 5, n));
      const priorRange  = Math.max(...bars.slice(n - 20, n - 5)) - Math.min(...bars.slice(n - 20, n - 5));
      return recentRange < priorRange * 0.40 && momentum(bars, 3) > 0.0002 &&
             price > ema(bars, 9);
    }
    case "VOL_CONTRACT_BEAR": {
      if (n < 20) return false;
      const recentRange = Math.max(...bars.slice(n - 5, n)) - Math.min(...bars.slice(n - 5, n));
      const priorRange  = Math.max(...bars.slice(n - 20, n - 5)) - Math.min(...bars.slice(n - 20, n - 5));
      return recentRange < priorRange * 0.40 && momentum(bars, 3) < -0.0002 &&
             price < ema(bars, 9);
    }

    // ── Inside bar: narrow bar followed by breakout ───────────────────────────
    case "INSIDE_BAR_BULL": {
      if (n < 10) return false;
      const motherHigh = bars[n - 3], motherLow = bars[n - 4];
      const insideHigh = bars[n - 2], insideLow = bars[n - 3];
      const isInside = insideHigh <= motherHigh && insideLow >= motherLow;
      return isInside && price > motherHigh * 1.0001 && momentum(bars, 3) > 0.0001;
    }
    case "INSIDE_BAR_BEAR": {
      if (n < 10) return false;
      const motherHigh = bars[n - 3], motherLow = bars[n - 4];
      const insideHigh = bars[n - 2], insideLow = bars[n - 3];
      const isInside = insideHigh <= motherHigh && insideLow >= motherLow;
      return isInside && price < motherLow * 0.9999 && momentum(bars, 3) < -0.0001;
    }

    // ── Momentum divergence: RSI rising while momentum slowing (bull recovery) ─
    case "MOM_DIVERGE_BULL": {
      if (n < 20) return false;
      const m5now  = momentum(bars, 5);
      const m5prev = momentum(bars.slice(0, -3), 5);
      const r = rsi(bars, 14);
      const rprev = rsi(bars.slice(0, -3), 14);
      // Price momentum slowing but RSI recovering — exhaustion of sellers
      return m5now < m5prev && m5prev < -0.0002 && r > rprev && r < 52 && price > bars[n - 2];
    }
    case "MOM_DIVERGE_BEAR": {
      if (n < 20) return false;
      const m5now  = momentum(bars, 5);
      const m5prev = momentum(bars.slice(0, -3), 5);
      const r = rsi(bars, 14);
      const rprev = rsi(bars.slice(0, -3), 14);
      return m5now > m5prev && m5prev > 0.0002 && r < rprev && r > 48 && price < bars[n - 2];
    }

    // ── Range expansion: bar range well above average → trend continuation ────
    case "RANGE_EXPAND_BULL": {
      if (n < 15) return false;
      const curRange = Math.abs(price - bars[n - 2]);
      const avgRange = bars.slice(n - 10, n - 1).reduce((s, v, i, a) =>
        i === 0 ? 0 : s + Math.abs(v - a[i - 1]), 0) / 9;
      return curRange > avgRange * 1.8 && price > bars[n - 2] &&
             momentum(bars, 3) > 0.0002 && rsi(bars, 14) < 72;
    }
    case "RANGE_EXPAND_BEAR": {
      if (n < 15) return false;
      const curRange = Math.abs(price - bars[n - 2]);
      const avgRange = bars.slice(n - 10, n - 1).reduce((s, v, i, a) =>
        i === 0 ? 0 : s + Math.abs(v - a[i - 1]), 0) / 9;
      return curRange > avgRange * 1.8 && price < bars[n - 2] &&
             momentum(bars, 3) < -0.0002 && rsi(bars, 14) > 28;
    }

    // ── ADX proxy: sustained directional trend (high EMA separation) ─────────
    case "ADX_BULL_TREND": {
      // Proxy ADX via EMA spread: strong when fast EMA much higher than slow
      if (n < 30) return false;
      const e9  = ema(bars, 9), e21 = ema(bars, 21);
      const spread = (e9 - e21) / e21;
      return spread > 0.0012 && momentum(bars, 10) > 0.0005 && rsi(bars, 14) > 52 && rsi(bars, 14) < 74;
    }
    case "ADX_BEAR_TREND": {
      if (n < 30) return false;
      const e9  = ema(bars, 9), e21 = ema(bars, 21);
      const spread = (e21 - e9) / e21;
      return spread > 0.0012 && momentum(bars, 10) < -0.0005 && rsi(bars, 14) < 48 && rsi(bars, 14) > 26;
    }

    // ── Parabolic SAR flip proxy: price crosses above/below trailing extremum ─
    case "PSAR_BULL_FLIP": {
      if (n < 15) return false;
      // Trailing stop proxy: lowest low of last 5 bars
      const trailStop = Math.min(...bars.slice(n - 6, n - 1));
      const prevBelow = bars[n - 3] < trailStop || bars[n - 4] < trailStop;
      return prevBelow && price > trailStop * 1.0002 &&
             momentum(bars, 3) > 0.0001 && rsi(bars, 14) > 45;
    }
    case "PSAR_BEAR_FLIP": {
      if (n < 15) return false;
      const trailStop = Math.max(...bars.slice(n - 6, n - 1));
      const prevAbove = bars[n - 3] > trailStop || bars[n - 4] > trailStop;
      return prevAbove && price < trailStop * 0.9998 &&
             momentum(bars, 3) < -0.0001 && rsi(bars, 14) < 55;
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
    if (eng.lastPrice <= 0) return;

    // Always include live price as latest bar so signals fire immediately
    // even before a full 1-min bar boundary is crossed
    const bars = eng.minuteBars.length > 0
      ? eng.minuteBars
      : [eng.lastPrice];
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

      // Append live SSE price as the "current bar" so momentum/RSI/EMA
      // reflect the actual market right now, not the last minute's close.
      const barsForEval = [...bars, price];
      const fires = evalSignal(strat.def.signal, barsForEval, price);
      strat.score = fires ? 75 : 0;

      if (fires) {
        openPositionLocked(eng, strat, price, iv, now);
        openCount++;
      }
    }

    pushDisplayState();
  }, [pushDisplayState]);

  // ── Feed price tick: build 1-minute bars + trigger engine immediately ────
  const feedPrice = useCallback((price: number, triggerTick: () => void) => {
    if (price <= 0) return;
    const eng = engRef.current;
    eng.lastPrice = price;

    const nowMinute = Math.floor(Date.now() / 60000);
    if (nowMinute !== eng.lastMinute) {
      eng.lastMinute = nowMinute;
      eng.minuteBars.push(price);
      if (eng.minuteBars.length > MAX_BARS) eng.minuteBars.shift();
    }
    // Run engine on every price tick — no need to wait for interval
    triggerTick();
  }, []);

  // ── Subscribe to NIFTY SSE price stream ──────────────────────────────────
  useEffect(() => {
    let cancelled = false;
    const source = new EventSource("/api/nifty/stream");

    source.onmessage = (e) => {
      if (cancelled) return;
      try {
        const data = JSON.parse(e.data as string) as { price?: number };
        if (data.price && data.price > 0) feedPrice(data.price, engineTick);
      } catch { /* ignore */ }
    };

    source.onerror = () => { /* EventSource auto-reconnects */ };
    return () => { cancelled = true; source.close(); };
  }, [feedPrice, engineTick]);

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
