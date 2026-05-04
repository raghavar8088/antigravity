"use client";

/**
 * Nifty BEES paper scalper — Angel One live NSE LTP for NIFTYBEES ETF.
 * ₹10,000 paper, 10 strategies, client-side auto entries/exits.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import type { NiftyBeesCandle } from "@/app/api/nifty-bees/candles/route";
import type { NiftyBeesLtpPayload } from "@/app/api/nifty-bees/ltp/route";

const SYMBOL = "NIFTYBEES";
const INITIAL_BALANCE = 10_000;
const MAX_OPEN_POSITIONS = 4;
const MAX_BARS = 120;
const MIN_BARS_FAST = 18;
const MIN_BARS_SLOW = 28;
const SIGNAL_THRESHOLD = 61;
const POLL_MS = 4_000;
const MAX_TRADES = 2_000;
const TRADE_CAP_PCT = 0.22;
const TRADE_CAP_MAX = 2_500;
const LOCAL_STORAGE_KEY = "nifty_bees_paper_v1";

const PROFIT_LOCK_PROGRESS = 0.28;
const PROFIT_LOCK_SHARE = 0.4;
const LATE_EXIT_PROGRESS = 0.55;
const LATE_EXIT_MIN_GAIN = 0.04;
const GRIND_EXIT_PROGRESS = 0.42;
const GRIND_EXIT_SHARE = 0.22;
const TRAIL_ACTIVATION_PCT = 0.2;
const TRAIL_GIVEBACK_SHARE = 0.32;
const LOSS_COOLDOWN_PENALTY = 0.35;

type Side = "LONG" | "SHORT";
type Status = "WARMING" | "READY" | "IN_POSITION" | "COOLING";
type Regime = "UNKNOWN" | "TRENDING_BULL" | "TRENDING_BEAR" | "HIGH_VOL" | "RANGE";

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
  breakoutHigh20: number;
  breakoutLow20: number;
  momentum3: number;
  momentum6: number;
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
  ltp: number;
  changePct: number;
  lastFeedAt: number;
  lastError: string;
  strategies: InternalStrategyState[];
  positions: Map<string, InternalPosition>;
  trades: InternalTrade[];
  balance: number;
  seq: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
  tradingSymbol: string;
}

// ── NSE cash session (Mon–Fri 09:15–15:30 IST) — block new entries outside window

function isNSECashSessionOpen(): boolean {
  const parts = new Intl.DateTimeFormat("en-IN", {
    timeZone: "Asia/Kolkata",
    weekday: "long",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(new Date());
  let weekday = "";
  let hour = 0;
  let minute = 0;
  for (const p of parts) {
    if (p.type === "weekday") weekday = p.value;
    if (p.type === "hour") hour = parseInt(p.value, 10) || 0;
    if (p.type === "minute") minute = parseInt(p.value, 10) || 0;
  }
  if (weekday === "Sunday" || weekday === "Saturday") return false;
  const mins = hour * 60 + minute;
  return mins >= 9 * 60 + 15 && mins <= 15 * 60 + 30;
}

const STRAT_DEFS: StratDef[] = [
  { id: 1, name: "BEES_Range_Breakout_Long", category: "Breakout", side: "LONG", signal: "BREAKOUT", tpPct: 0.42, slPct: 0.18, cooldownMinutes: 12, minBars: MIN_BARS_SLOW, holdMinutes: 95 },
  { id: 2, name: "BEES_Range_Breakdown_Short", category: "Breakout", side: "SHORT", signal: "BREAKOUT_SHORT", tpPct: 0.42, slPct: 0.18, cooldownMinutes: 12, minBars: MIN_BARS_SLOW, holdMinutes: 95 },
  { id: 3, name: "BEES_EMA_Impulse_Long", category: "Momentum", side: "LONG", signal: "EMA_CROSS", tpPct: 0.38, slPct: 0.16, cooldownMinutes: 10, minBars: MIN_BARS_SLOW, holdMinutes: 88 },
  { id: 4, name: "BEES_EMA_Fade_Short", category: "Momentum", side: "SHORT", signal: "EMA_CROSS_SHORT", tpPct: 0.38, slPct: 0.16, cooldownMinutes: 10, minBars: MIN_BARS_SLOW, holdMinutes: 88 },
  { id: 5, name: "BEES_RSI_Reclaim_Long", category: "Mean Reversion", side: "LONG", signal: "RSI_BOUNCE", tpPct: 0.28, slPct: 0.14, cooldownMinutes: 9, minBars: MIN_BARS_FAST, holdMinutes: 65 },
  { id: 6, name: "BEES_RSI_Fade_Short", category: "Mean Reversion", side: "SHORT", signal: "RSI_BOUNCE_SHORT", tpPct: 0.28, slPct: 0.14, cooldownMinutes: 9, minBars: MIN_BARS_FAST, holdMinutes: 65 },
  { id: 7, name: "BEES_VWAP_Reclaim_Long", category: "VWAP", side: "LONG", signal: "VWAP_RECLAIM", tpPct: 0.3, slPct: 0.14, cooldownMinutes: 9, minBars: MIN_BARS_SLOW, holdMinutes: 78 },
  { id: 8, name: "BEES_VWAP_Reject_Short", category: "VWAP", side: "SHORT", signal: "VWAP_RECLAIM_SHORT", tpPct: 0.3, slPct: 0.14, cooldownMinutes: 9, minBars: MIN_BARS_SLOW, holdMinutes: 78 },
  { id: 9, name: "BEES_Trend_Cont_Long", category: "Trend", side: "LONG", signal: "TREND_CONT", tpPct: 0.4, slPct: 0.16, cooldownMinutes: 14, minBars: MIN_BARS_SLOW, holdMinutes: 100 },
  { id: 10, name: "BEES_Trend_Cont_Short", category: "Trend", side: "SHORT", signal: "TREND_CONT_SHORT", tpPct: 0.4, slPct: 0.16, cooldownMinutes: 14, minBars: MIN_BARS_SLOW, holdMinutes: 100 },
];

function clamp(v: number, lo: number, hi: number) {
  return Math.max(lo, Math.min(hi, v));
}

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
  let gains = 0;
  let losses = 0;
  for (let i = start; i < values.length; i++) {
    const d = values[i] - values[i - 1];
    if (d > 0) gains += d;
    else losses -= d;
  }
  if (losses === 0) return 100;
  return 100 - 100 / (1 + gains / losses);
}

function scoreClamp(v: number) {
  return clamp(v, 0, 99);
}

function buildSignalInputs(bars: number[]): SignalInputs {
  const last = bars.length - 1;
  const price = bars[last];
  const prev = bars.slice(0, -1);
  const recent20 = bars.slice(-20);
  const prior20 = bars.slice(-21, -1);
  const breakoutWindow = prior20.length > 0 ? prior20 : recent20;
  return {
    price,
    prevPrice: last > 0 ? bars[last - 1] : price,
    fast: ema(bars, 6),
    slow: ema(bars, 15),
    trend: ema(bars, 28),
    prevFast: ema(prev, 6),
    prevSlow: ema(prev, 15),
    mean20: sma(bars, 20),
    std20: stdDev(bars, 20),
    rsi14: rsi(bars, 14),
    high20: Math.max(...recent20),
    low20: Math.min(...recent20),
    breakoutHigh20: Math.max(...breakoutWindow),
    breakoutLow20: Math.min(...breakoutWindow),
    momentum3: last >= 3 ? ((price - bars[last - 3]) / bars[last - 3]) * 100 : 0,
    momentum6: last >= 6 ? ((price - bars[last - 6]) / bars[last - 6]) * 100 : 0,
  };
}

function evalSignal(signal: string, input: SignalInputs): number {
  const { price, prevPrice, fast, slow, trend, prevFast, prevSlow, mean20, rsi14, breakoutHigh20, breakoutLow20, momentum3, momentum6 } = input;
  switch (signal) {
    case "BREAKOUT":
      return price > breakoutHigh20 * 1.00025 && fast > slow && rsi14 >= 54 && momentum3 > 0.03
        ? scoreClamp(72 + (price / breakoutHigh20 - 1) * 8000 + momentum3 * 15)
        : 0;
    case "BREAKOUT_SHORT":
      return price < breakoutLow20 * 0.99975 && fast < slow && rsi14 <= 46 && momentum3 < -0.03
        ? scoreClamp(72 + (breakoutLow20 / price - 1) * 8000 + Math.abs(momentum3) * 15)
        : 0;
    case "EMA_CROSS":
      return prevFast <= prevSlow && fast > slow && price > trend && rsi14 >= 52
        ? scoreClamp(70 + (fast / slow - 1) * 12000 + (rsi14 - 50) * 0.4)
        : 0;
    case "EMA_CROSS_SHORT":
      return prevFast >= prevSlow && fast < slow && price < trend && rsi14 <= 48
        ? scoreClamp(70 + (slow / fast - 1) * 12000 + (50 - rsi14) * 0.4)
        : 0;
    case "RSI_BOUNCE":
      return rsi14 <= 32 && price >= prevPrice && momentum3 > -0.15 ? scoreClamp(67 + (34 - rsi14) * 1.4) : 0;
    case "RSI_BOUNCE_SHORT":
      return rsi14 >= 68 && price <= prevPrice && momentum3 < 0.15 ? scoreClamp(67 + (rsi14 - 66) * 1.4) : 0;
    case "VWAP_RECLAIM":
      return price > mean20 * 1.00015 && prevPrice <= mean20 * 1.0003 && momentum3 > 0.02
        ? scoreClamp(68 + (price / mean20 - 1) * 5500 + momentum3 * 12)
        : 0;
    case "VWAP_RECLAIM_SHORT":
      return price < mean20 * 0.99985 && prevPrice >= mean20 * 0.9997 && momentum3 < -0.02
        ? scoreClamp(68 + (mean20 / price - 1) * 5500 + Math.abs(momentum3) * 12)
        : 0;
    case "TREND_CONT":
      return fast > slow && slow > trend && momentum6 > 0.08 && rsi14 >= 53 && rsi14 <= 76
        ? scoreClamp(73 + momentum6 * 25 + (rsi14 - 54) * 0.3)
        : 0;
    case "TREND_CONT_SHORT":
      return fast < slow && slow < trend && momentum6 < -0.08 && rsi14 >= 24 && rsi14 <= 47
        ? scoreClamp(73 + Math.abs(momentum6) * 25 + (46 - rsi14) * 0.3)
        : 0;
    default:
      return 0;
  }
}

function classifyRegime(input: SignalInputs): Regime {
  const trendGapPct = input.price > 0 ? (Math.abs(input.fast - input.slow) / input.price) * 100 : 0;
  const volPct = input.price > 0 ? (input.std20 / input.price) * 100 : 0;
  if (input.price <= 0) return "UNKNOWN";
  if (input.momentum6 === 0 && input.momentum3 === 0) return "UNKNOWN";
  if (input.fast > input.slow && input.slow > input.trend && input.momentum6 > 0.08) return "TRENDING_BULL";
  if (input.fast < input.slow && input.slow < input.trend && input.momentum6 < -0.08) return "TRENDING_BEAR";
  if (volPct >= 0.15 || trendGapPct >= 0.1) return "HIGH_VOL";
  return "RANGE";
}

function passesEntryConfirmation(def: StratDef, input: SignalInputs, regime: Regime): boolean {
  if (def.side === "LONG") {
    if (def.category === "VWAP") return input.price >= input.mean20 * 0.9998 && input.rsi14 >= 44;
    if (def.category === "Mean Reversion") return input.rsi14 <= 52 && input.price >= input.prevPrice * 0.9998;
    if (regime === "TRENDING_BULL") return input.price >= input.fast * 0.9997 && input.momentum3 > -0.02;
    return input.price >= input.prevPrice * 0.9997 && input.momentum3 > -0.05;
  }
  if (def.category === "VWAP") return input.price <= input.mean20 * 1.0002 && input.rsi14 <= 56;
  if (def.category === "Mean Reversion") return input.rsi14 >= 48 && input.price <= input.prevPrice * 1.0002;
  if (regime === "TRENDING_BEAR") return input.price <= input.fast * 1.0003 && input.momentum3 < 0.02;
  return input.price <= input.prevPrice * 1.0003 && input.momentum3 < 0.05;
}

function calcPnl(side: Side, entry: number, current: number, qty: number): number {
  return (current - entry) * qty * (side === "LONG" ? 1 : -1);
}

function cooldownMsFor(strategy: InternalStrategyState, won: boolean): number {
  const base = strategy.def.cooldownMinutes * 60 * 1000;
  return won ? base : Math.round(base * (1 + strategy.consecutiveLosses * LOSS_COOLDOWN_PENALTY));
}

function resolveExit(pos: InternalPosition, def: StratDef, price: number, now: number): { reason: string; exitPrice: number } | null {
  const netPnl = calcPnl(pos.side, pos.entryPrice, price, pos.quantity);
  const returnPct = pos.notional > 0 ? (netPnl / pos.notional) * 100 : 0;
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
  if (progress >= GRIND_EXIT_PROGRESS && returnPct >= Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * GRIND_EXIT_SHARE))
    return { reason: "PROFIT_LOCK", exitPrice: price };
  if (progress >= PROFIT_LOCK_PROGRESS && returnPct >= lockThreshold) return { reason: "PROFIT_LOCK", exitPrice: price };
  if (pos.peakReturnPct >= Math.max(TRAIL_ACTIVATION_PCT, def.tpPct * 0.4) && returnPct > 0 && returnPct <= pos.peakReturnPct * (1 - TRAIL_GIVEBACK_SHARE))
    return { reason: "TRAIL_STOP", exitPrice: price };
  if (progress >= LATE_EXIT_PROGRESS && returnPct >= LATE_EXIT_MIN_GAIN) return { reason: "LATE_EXIT", exitPrice: price };
  if (maxHoldMs > 0 && now - pos.entryTime >= maxHoldMs) return { reason: "TIME_EXIT", exitPrice: price };
  return null;
}

function initEngine(): EngineRef {
  return {
    bars: [],
    ltp: 0,
    changePct: 0,
    lastFeedAt: 0,
    lastError: "",
    tradingSymbol: SYMBOL,
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
  };
}

function allocationINR(balance: number): number {
  return Math.min(TRADE_CAP_MAX, Math.max(500, balance * TRADE_CAP_PCT));
}

function openPosition(engine: EngineRef, strategy: InternalStrategyState, entryPrice: number, now: number): boolean {
  const cap = allocationINR(engine.balance);
  const qty = entryPrice > 0 ? Math.max(1, Math.floor(cap / entryPrice)) : 0;
  const notional = qty * entryPrice;
  if (qty <= 0 || notional <= 0 || engine.balance < notional) return false;

  engine.seq++;
  engine.balance -= notional;
  const tpMul = 1 + strategy.def.tpPct / 100;
  const slMul = 1 - strategy.def.slPct / 100;
  const pos: InternalPosition = {
    id: `BEES-${Date.now()}-${engine.seq}`,
    strategyId: strategy.def.id,
    strategyName: strategy.def.name,
    side: strategy.def.side,
    entryPrice,
    currentPrice: entryPrice,
    tpPrice: strategy.def.side === "LONG" ? entryPrice * tpMul : entryPrice * (2 - tpMul),
    slPrice: strategy.def.side === "LONG" ? entryPrice * slMul : entryPrice * (2 - slMul),
    quantity: qty,
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

function closePosition(engine: EngineRef, strategy: InternalStrategyState, exitPrice: number, reason: string, now: number) {
  const pos = strategy.position;
  if (!pos) return;
  const netPnl = calcPnl(pos.side, pos.entryPrice, exitPrice, pos.quantity);
  const trade: InternalTrade = {
    id: pos.id,
    strategyId: pos.strategyId,
    strategyName: pos.strategyName,
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
  strategy.regime = "UNKNOWN";
  strategy.position = null;
  engine.positions.delete(pos.id);
}

function normalizeAccount(engine: EngineRef) {
  const openNotional = [...engine.positions.values()].reduce((s, p) => s + p.notional, 0);
  const unrealized = [...engine.positions.values()].reduce((s, p) => s + p.unrealizedPnl, 0);
  const normalizedBalance = INITIAL_BALANCE + engine.totalRealizedPnl - openNotional;
  if (Number.isFinite(normalizedBalance) && Math.abs(engine.balance - normalizedBalance) > 0.01) {
    engine.balance = normalizedBalance;
  }
  return {
    balance: engine.balance,
    equity: engine.balance + openNotional + unrealized,
    sessionPnl: engine.totalRealizedPnl + unrealized,
    unrealized,
  };
}

function persist(engine: EngineRef) {
  try {
    localStorage.setItem(
      LOCAL_STORAGE_KEY,
      JSON.stringify({
        balance: engine.balance,
        totalWins: engine.totalWins,
        totalLosses: engine.totalLosses,
        totalPnl: engine.totalRealizedPnl,
        tradeSeq: engine.seq,
        barsTail: engine.bars.slice(-40),
        positions: [...engine.positions.values()],
        trades: engine.trades.slice(0, 400),
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
      }),
    );
  } catch {
    /* ignore */
  }
}

function loadPersisted(engine: EngineRef): boolean {
  try {
    const raw = localStorage.getItem(LOCAL_STORAGE_KEY);
    if (!raw) return false;
    const data = JSON.parse(raw) as {
      balance?: number;
      totalWins?: number;
      totalLosses?: number;
      totalPnl?: number;
      tradeSeq?: number;
      positions?: InternalPosition[];
      trades?: InternalTrade[];
      strategies?: Array<{
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
    engine.balance = Number(data.balance) || INITIAL_BALANCE;
    engine.totalWins = Math.max(0, Math.trunc(Number(data.totalWins) || 0));
    engine.totalLosses = Math.max(0, Math.trunc(Number(data.totalLosses) || 0));
    engine.totalRealizedPnl = Number(data.totalPnl) || 0;
    engine.seq = Math.max(0, Math.trunc(Number(data.tradeSeq) || 0));
    engine.trades = (data.trades ?? []).slice(0, MAX_TRADES);
    engine.positions.clear();
    for (const p of data.positions ?? []) {
      if (!p?.id || !p.strategyId) continue;
      engine.positions.set(p.id, { ...p, peakReturnPct: p.peakReturnPct ?? 0 });
      const st = engine.strategies.find((x) => x.def.id === p.strategyId);
      if (st) {
        st.position = engine.positions.get(p.id) ?? null;
        st.status = st.position ? "IN_POSITION" : "WARMING";
      }
    }
    for (const st of engine.strategies) {
      const sv = data.strategies?.find((x) => x.id === st.def.id);
      if (!sv) continue;
      st.totalTrades = sv.totalTrades;
      st.wins = sv.wins;
      st.losses = sv.losses;
      st.totalPnl = sv.totalPnl;
      st.winRate = sv.winRate;
      st.cooldownUntil = sv.cooldownUntil;
      st.consecutiveLosses = sv.consecutiveLosses;
      if (!st.position) st.status = st.cooldownUntil > Date.now() ? "COOLING" : "WARMING";
    }
    normalizeAccount(engine);
    return true;
  } catch {
    return false;
  }
}

// ── Public types ─────────────────────────────────────────────────────────────

export type NiftyBeesQuote = {
  symbol: string;
  tradingSymbol: string;
  ltp: number;
  changePct: number;
  barCount: number;
  signalScore: number;
};

export type NiftyBeesPosition = {
  id: string;
  strategyId: number;
  strategyName: string;
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

export type NiftyBeesTrade = {
  id: string;
  strategyId: number;
  strategyName: string;
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

export type NiftyBeesStrategyStatus = {
  id: number;
  name: string;
  category: string;
  side: Side;
  status: Status;
  score: number;
  regime: Regime;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  cooldownUntil?: string;
};

export type NiftyBeesStats = {
  balance: number;
  equity: number;
  sessionPnl: number;
  unrealizedPnl: number;
  realizedPnl: number;
  totalTrades: number;
  totalWins: number;
  totalLosses: number;
  openPositions: number;
  winRate: number;
  sessionOpen: boolean;
  lastUpdateAt: number;
  diagnostics: string;
};

const EMPTY_STATS: NiftyBeesStats = {
  balance: INITIAL_BALANCE,
  equity: INITIAL_BALANCE,
  sessionPnl: 0,
  unrealizedPnl: 0,
  realizedPnl: 0,
  totalTrades: 0,
  totalWins: 0,
  totalLosses: 0,
  openPositions: 0,
  winRate: 0,
  sessionOpen: false,
  lastUpdateAt: 0,
  diagnostics: "Starting Nifty BEES engine.",
};

export default function useNiftyBeesEngine() {
  const engineRef = useRef<EngineRef>(initEngine());
  const [quote, setQuote] = useState<NiftyBeesQuote>({
    symbol: SYMBOL,
    tradingSymbol: SYMBOL,
    ltp: 0,
    changePct: 0,
    barCount: 0,
    signalScore: 0,
  });
  const [positions, setPositions] = useState<NiftyBeesPosition[]>([]);
  const [trades, setTrades] = useState<NiftyBeesTrade[]>([]);
  const [strategies, setStrategies] = useState<NiftyBeesStrategyStatus[]>(
    STRAT_DEFS.map((d) => ({
      id: d.id,
      name: d.name,
      category: d.category,
      side: d.side,
      status: "WARMING",
      score: 0,
      regime: "UNKNOWN",
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
    })),
  );
  const [stats, setStats] = useState<NiftyBeesStats>(EMPTY_STATS);
  const seededRef = useRef(false);

  const pushDisplay = useCallback(() => {
    const e = engineRef.current;
    let maxScore = 0;
    for (const s of e.strategies) {
      if (s.score > maxScore) maxScore = s.score;
    }
    const { balance, equity, sessionPnl, unrealized } = normalizeAccount(e);
    setQuote({
      symbol: SYMBOL,
      tradingSymbol: e.tradingSymbol,
      ltp: e.ltp,
      changePct: e.changePct,
      barCount: e.bars.length,
      signalScore: maxScore,
    });
    setPositions(
      [...e.positions.values()].map((p) => ({
        id: p.id,
        strategyId: p.strategyId,
        strategyName: p.strategyName,
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
      e.trades.map((t) => ({
        id: t.id,
        strategyId: t.strategyId,
        strategyName: t.strategyName,
        side: t.side,
        quantity: t.quantity,
        entryPrice: t.entryPrice,
        exitPrice: t.exitPrice,
        netPnl: t.netPnl,
        returnPct: t.returnPct,
        entryTime: new Date(t.entryTime).toISOString(),
        exitTime: new Date(t.exitTime).toISOString(),
        exitReason: t.exitReason,
        holdSeconds: t.holdSeconds,
      })),
    );
    setStrategies(
      e.strategies.map((s) => ({
        id: s.def.id,
        name: s.def.name,
        category: s.def.category,
        side: s.def.side,
        status: s.status,
        score: s.score,
        regime: s.regime,
        totalTrades: s.totalTrades,
        wins: s.wins,
        losses: s.losses,
        totalPnl: s.totalPnl,
        winRate: s.winRate,
        cooldownUntil: s.cooldownUntil > 0 ? new Date(s.cooldownUntil).toISOString() : undefined,
      })),
    );
    const totalTrades = e.strategies.reduce((sum, s) => sum + s.totalTrades, 0);
    const winRate = e.totalWins + e.totalLosses > 0 ? (e.totalWins / (e.totalWins + e.totalLosses)) * 100 : 0;
    const sessionOpen = isNSECashSessionOpen();
    const diag =
      e.lastError ||
      (e.ltp > 0
        ? `Live Angel One NSE · ${e.tradingSymbol} · session ${sessionOpen ? "open" : "closed (entries paused)"}`
        : "Waiting for NIFTYBEES LTP — configure LIGHTSAIL_ENGINE_URL for Angel proxy.");
    setStats({
      balance,
      equity,
      sessionPnl,
      unrealizedPnl: unrealized,
      realizedPnl: e.totalRealizedPnl,
      totalTrades,
      totalWins: e.totalWins,
      totalLosses: e.totalLosses,
      openPositions: e.positions.size,
      winRate,
      sessionOpen,
      lastUpdateAt: e.lastFeedAt,
      diagnostics: diag,
    });
  }, []);

  const processTick = useCallback((payload: NiftyBeesLtpPayload, pauseNewEntries: boolean) => {
    const e = engineRef.current;
    const now = Date.now();
    if (payload.tradingSymbol) e.tradingSymbol = payload.tradingSymbol;
    if (payload.error) e.lastError = payload.error;
    else if (payload.ok && payload.ltp > 0) e.lastError = "";

    const live = payload.ltp > 0 ? payload.ltp : 0;
    if (live > 0) {
      e.ltp = live;
      e.changePct = payload.changePct ?? 0;
      e.lastFeedAt = now;
      const bars = [...e.bars];
      if (!bars.length || bars[bars.length - 1] !== live) bars.push(live);
      if (bars.length > MAX_BARS) bars.splice(0, bars.length - MAX_BARS);
      e.bars = bars;
    }

    const sessionOpen = isNSECashSessionOpen();
    const allowEntries = sessionOpen && !pauseNewEntries && payload.ok && live > 0;

    for (const strategy of e.strategies) {
      if (strategy.position) {
        if (live > 0) {
          strategy.position.currentPrice = live;
          strategy.position.unrealizedPnl = calcPnl(strategy.position.side, strategy.position.entryPrice, live, strategy.position.quantity);
          strategy.position.returnPct =
            strategy.position.notional > 0 ? (strategy.position.unrealizedPnl / strategy.position.notional) * 100 : 0;
          strategy.position.peakReturnPct = Math.max(strategy.position.peakReturnPct, strategy.position.returnPct);
          const b = e.bars;
          if (b.length >= strategy.def.minBars) {
            strategy.regime = classifyRegime(buildSignalInputs(b));
          }
          const exit = resolveExit(strategy.position, strategy.def, live, now);
          if (exit) closePosition(e, strategy, exit.exitPrice, exit.reason, now);
        }
        continue;
      }
      if (strategy.cooldownUntil > now) {
        strategy.status = "COOLING";
        strategy.score = 0;
        continue;
      }
      strategy.status = "READY";
      strategy.score = 0;
      strategy.regime = "UNKNOWN";
      if (e.positions.size >= MAX_OPEN_POSITIONS) continue;
      if (!allowEntries) continue;

      const bars = e.bars;
      if (bars.length < strategy.def.minBars) {
        strategy.status = "WARMING";
        continue;
      }
      const input = buildSignalInputs(bars);
      const regime = classifyRegime(input);
      const score = evalSignal(strategy.def.signal, input);
      const confirmed = score >= SIGNAL_THRESHOLD && passesEntryConfirmation(strategy.def, input, regime);
      strategy.score = confirmed ? score : Math.min(score, SIGNAL_THRESHOLD - 1);
      strategy.regime = regime;
      if (confirmed && live > 0) {
        openPosition(e, strategy, live, now);
      }
    }

    pushDisplay();
    persist(e);
  }, [pushDisplay]);

  const reset = useCallback(() => {
    engineRef.current = initEngine();
    seededRef.current = false;
    try {
      localStorage.removeItem(LOCAL_STORAGE_KEY);
    } catch {
      /* ignore */
    }
    pushDisplay();
  }, [pushDisplay]);

  const clearTrades = useCallback(() => {
    const e = engineRef.current;
    e.trades = [];
    persist(e);
    pushDisplay();
  }, [pushDisplay]);

  useEffect(() => {
    loadPersisted(engineRef.current);
    pushDisplay();
  }, [pushDisplay]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      if (seededRef.current) return;
      try {
        const res = await fetch("/api/nifty-bees/candles?interval=ONE_MINUTE", { cache: "no-store" });
        const data = await res.json() as { ok?: boolean; candles?: NiftyBeesCandle[] };
        if (cancelled || !data.ok || !data.candles?.length) return;
        const closes = data.candles.map((c) => c.close).filter((c) => c > 0);
        const e = engineRef.current;
        if (e.bars.length < 20) {
          e.bars = closes.slice(-MAX_BARS);
          if (data.candles.length && e.tradingSymbol === SYMBOL) {
            /* keep SYMBOL */
          }
        }
        seededRef.current = true;
        pushDisplay();
      } catch {
        /* ignore */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [pushDisplay]);

  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      if (cancelled) return;
      try {
        const res = await fetch("/api/nifty-bees/ltp", { cache: "no-store" });
        const payload = (await res.json()) as NiftyBeesLtpPayload;
        processTick(payload, !res.ok || !payload.ok);
      } catch {
        processTick(
          {
            ok: false,
            ltp: 0,
            open: 0,
            high: 0,
            low: 0,
            close: 0,
            change: 0,
            changePct: 0,
            token: "",
            tradingSymbol: "",
            error: "LTP fetch failed",
          },
          true,
        );
      }
    };
    void tick();
    const id = setInterval(() => void tick(), POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [processTick]);

  return { quote, positions, trades, strategies, stats, reset, clearTrades };
}
