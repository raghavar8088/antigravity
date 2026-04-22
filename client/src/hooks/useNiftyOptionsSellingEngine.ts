"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Candle } from "@/app/api/nifty/candles/route";
import type { OptionPosition, OptionStats, OptionStrategyStatus, OptionTrade } from "./useNiftyOptions";

const INITIAL_BALANCE = 1_000_000;
const LOT_SIZE = 75;
const STRIKE_STEP = 50;
const BASE_IV = 0.17;
const DTE_DAYS = 7;
const MAX_CONCURRENT = 14;
const MAX_BARS = 200;
const TICK_MS = 1_000;
const MIN_BARS = 12;
const PROFIT_LOCK_PROGRESS = 0.26;
const PROFIT_LOCK_SHARE = 0.38;
const LATE_EXIT_PROGRESS = 0.54;
const LATE_EXIT_MIN_GAIN = 0.04;
const GRIND_EXIT_PROGRESS = 0.46;
const GRIND_EXIT_SHARE = 0.24;
const NIFTY_SELL_TRAIL_GIVEBACK_SHARE = 0.38;
const NIFTY_SELL_MIN_SIZE_MULTIPLIER = 0.5;
const NIFTY_SELL_MAX_SIZE_MULTIPLIER = 1.8;
const NIFTY_SELL_LOSS_COOLDOWN_PENALTY = 0.3;
const NIFTY_SELL_UNDERPERFORMING_MIN_TRADES = 8;
const NIFTY_SELL_UNDERPERFORMING_MAX_WINRATE = 38;
const NIFTY_SELL_UNDERPERFORMING_PAUSE_MS = 6 * 60 * 60 * 1000;

type Regime = "BULL" | "BEAR" | "RANGE" | "VOLATILE";

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
  marginINR: number;
}

interface InternalPosition {
  id: string;
  strategyId: number;
  strategyName: string;
  optionType: "CALL" | "PUT";
  strike: number;
  expiryTime: string;
  entryPremium: number;
  currentPremium: number;
  quantity: number;
  marginBlocked: number;
  entryNiftyPrice: number;
  entryTime: string;
  unrealizedPnl: number;
  peakGainPct: number;
  iv: number;
  delta: number;
  barsHeld: number;
}

interface InternalStrategy {
  def: StratDef;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  score: number;
  status: OptionStrategyStatus["status"];
  rosterState: OptionStrategyStatus["rosterState"];
  hasPosition: boolean;
  cooldownUntil: number;
  regime: string;
  consecutiveLosses: number;
}

interface EngineRef {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
  totalPremiumSpent: number;
  seq: number;
  lastPrice: number;
  lastMinute: number;
  minuteBars: number[];
  positions: InternalPosition[];
  trades: OptionTrade[];
  strategies: InternalStrategy[];
}

type NiftySellingDbStrategy = {
  id: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  score: number;
  status: OptionStrategyStatus["status"];
  rosterState: OptionStrategyStatus["rosterState"];
  hasPosition: boolean;
  cooldownUntil: number;
  regime: string;
  consecutiveLosses: number;
};

type NiftySellingDbPayload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  totalPremiumSpent: number;
  tradeSeq: number;
  lastPrice: number;
  lastMinute: number;
  minuteBars: number[];
  positions: InternalPosition[];
  trades: OptionTrade[];
  strategies: NiftySellingDbStrategy[];
};

const STRATEGIES: StratDef[] = [
  { id: 1, name: "Bull_Put_VWAP", category: "VWAP", optionType: "PUT", signal: "BULL_VWAP", tpPct: 0.28, slPct: 0.22, cooldownSecs: 120, minBars: MIN_BARS, marginINR: 12000 },
  { id: 2, name: "Bull_Put_Trend", category: "Trend", optionType: "PUT", signal: "BULL_TREND", tpPct: 0.32, slPct: 0.24, cooldownSecs: 180, minBars: MIN_BARS, marginINR: 14000 },
  { id: 3, name: "Bull_Put_Momentum", category: "Momentum", optionType: "PUT", signal: "BULL_MOM", tpPct: 0.26, slPct: 0.2, cooldownSecs: 120, minBars: 8, marginINR: 11000 },
  { id: 4, name: "Bull_Put_Range", category: "Mean Reversion", optionType: "PUT", signal: "RANGE_UP", tpPct: 0.22, slPct: 0.18, cooldownSecs: 90, minBars: 8, marginINR: 10000 },
  { id: 5, name: "Bull_Put_Compression", category: "Volatility", optionType: "PUT", signal: "SQUEEZE_UP", tpPct: 0.3, slPct: 0.22, cooldownSecs: 180, minBars: MIN_BARS, marginINR: 13000 },
  { id: 6, name: "Bull_Put_Opening", category: "Momentum", optionType: "PUT", signal: "OPENING_BULL", tpPct: 0.24, slPct: 0.2, cooldownSecs: 120, minBars: 6, marginINR: 10000 },
  { id: 7, name: "Bull_Put_Reclaim", category: "Price Action", optionType: "PUT", signal: "RECLAIM_BULL", tpPct: 0.25, slPct: 0.2, cooldownSecs: 120, minBars: 8, marginINR: 10500 },
  { id: 8, name: "Bull_Put_HigherLow", category: "Trend", optionType: "PUT", signal: "HIGHER_LOW", tpPct: 0.29, slPct: 0.22, cooldownSecs: 180, minBars: MIN_BARS, marginINR: 12500 },
  { id: 9, name: "Bear_Call_VWAP", category: "VWAP", optionType: "CALL", signal: "BEAR_VWAP", tpPct: 0.28, slPct: 0.22, cooldownSecs: 120, minBars: MIN_BARS, marginINR: 12000 },
  { id: 10, name: "Bear_Call_Trend", category: "Trend", optionType: "CALL", signal: "BEAR_TREND", tpPct: 0.32, slPct: 0.24, cooldownSecs: 180, minBars: MIN_BARS, marginINR: 14000 },
  { id: 11, name: "Bear_Call_Momentum", category: "Momentum", optionType: "CALL", signal: "BEAR_MOM", tpPct: 0.26, slPct: 0.2, cooldownSecs: 120, minBars: 8, marginINR: 11000 },
  { id: 12, name: "Bear_Call_Range", category: "Mean Reversion", optionType: "CALL", signal: "RANGE_DOWN", tpPct: 0.22, slPct: 0.18, cooldownSecs: 90, minBars: 8, marginINR: 10000 },
  { id: 13, name: "Bear_Call_Compression", category: "Volatility", optionType: "CALL", signal: "SQUEEZE_DOWN", tpPct: 0.3, slPct: 0.22, cooldownSecs: 180, minBars: MIN_BARS, marginINR: 13000 },
  { id: 14, name: "Bear_Call_Opening", category: "Momentum", optionType: "CALL", signal: "OPENING_BEAR", tpPct: 0.24, slPct: 0.2, cooldownSecs: 120, minBars: 6, marginINR: 10000 },
  { id: 15, name: "Bear_Call_Rejection", category: "Price Action", optionType: "CALL", signal: "REJECT_BEAR", tpPct: 0.25, slPct: 0.2, cooldownSecs: 120, minBars: 8, marginINR: 10500 },
  { id: 16, name: "Bear_Call_LowerHigh", category: "Trend", optionType: "CALL", signal: "LOWER_HIGH", tpPct: 0.29, slPct: 0.22, cooldownSecs: 180, minBars: MIN_BARS, marginINR: 12500 },
];

function ema(values: number[], period: number): number {
  if (!values.length) return 0;
  const k = 2 / (Math.min(period, values.length) + 1);
  let out = values[0];
  for (let i = 1; i < values.length; i++) out = values[i] * k + out * (1 - k);
  return out;
}

function sma(values: number[], period: number): number {
  const slice = values.slice(-period);
  return slice.length ? slice.reduce((sum, value) => sum + value, 0) / slice.length : 0;
}

function stddev(values: number[]): number {
  if (values.length < 2) return 0;
  const avg = values.reduce((sum, value) => sum + value, 0) / values.length;
  return Math.sqrt(values.reduce((sum, value) => sum + (value - avg) ** 2, 0) / values.length);
}

function rsi(values: number[], period: number): number {
  if (values.length < 2) return 50;
  const slice = values.slice(-period - 1);
  let gains = 0;
  let losses = 0;
  for (let i = 1; i < slice.length; i++) {
    const diff = slice[i] - slice[i - 1];
    if (diff > 0) gains += diff;
    else losses -= diff;
  }
  if (losses === 0) return 100;
  return 100 - 100 / (1 + gains / losses);
}

function momentum(values: number[], lookback: number): number {
  if (values.length <= lookback) return 0;
  const prev = values[values.length - 1 - lookback];
  return prev === 0 ? 0 : (values[values.length - 1] - prev) / prev;
}

function roundStrike(price: number, offsetSteps = 0): number {
  return Math.round(price / STRIKE_STEP) * STRIKE_STEP + offsetSteps * STRIKE_STEP;
}

function estimatePremium(price: number, strike: number, optionType: "CALL" | "PUT", iv: number): number {
  const atmDistance = Math.abs(price - strike) / Math.max(price, 1);
  const intrinsic = optionType === "CALL" ? Math.max(0, price - strike) : Math.max(0, strike - price);
  const timeValue = price * iv * Math.sqrt(DTE_DAYS / 365) * (0.22 - atmDistance * 0.18);
  return Math.max(8, intrinsic + Math.max(price * 0.0025, timeValue));
}

function markPremium(entryPremium: number, entryPrice: number, currentPrice: number, optionType: "CALL" | "PUT", barsHeld: number): number {
  const delta = optionType === "CALL" ? 0.24 : -0.24;
  const move = currentPrice - entryPrice;
  const theta = entryPremium * 0.0025 * barsHeld;
  return Math.max(entryPremium * 0.1, entryPremium + delta * move - theta);
}

function classifyRegime(values: number[]): Regime {
  if (values.length < 10) return "RANGE";
  const fast = ema(values, 5);
  const slow = ema(values, 13);
  const trend = ema(values, 21);
  const mom = momentum(values, 5);
  const vol = stddev(values.slice(-20)) / Math.max(values[values.length - 1], 1);
  if (vol > 0.0045) return "VOLATILE";
  if (fast > slow && slow > trend && mom > 0.0012) return "BULL";
  if (fast < slow && slow < trend && mom < -0.0012) return "BEAR";
  return "RANGE";
}

function evalSignal(signal: string, values: number[]): boolean {
  const price = values[values.length - 1];
  const fast = ema(values, 5);
  const slow = ema(values, 13);
  const trend = ema(values, 21);
  const mean = sma(values, 20);
  const band = stddev(values.slice(-20));
  const r = rsi(values, 14);
  const mom3 = momentum(values, 3);
  const mom6 = momentum(values, 6);

  switch (signal) {
    case "BULL_VWAP": return price > mean && fast > slow && r > 52;
    case "BULL_TREND": return fast > slow && slow > trend && mom6 > 0.0015;
    case "BULL_MOM": return mom3 > 0.001 && r > 54;
    case "RANGE_UP": return r < 60 && r > 46 && price >= mean;
    case "SQUEEZE_UP": return band / Math.max(price, 1) < 0.002 && mom3 > 0.0008;
    case "OPENING_BULL": return mom3 > 0.0012 && price > fast;
    case "RECLAIM_BULL": return price > mean && values.length > 2 && values[values.length - 2] <= mean;
    case "HIGHER_LOW": return price > slow && values.slice(-5)[0] < values.slice(-3)[0];
    case "BEAR_VWAP": return price < mean && fast < slow && r < 48;
    case "BEAR_TREND": return fast < slow && slow < trend && mom6 < -0.0015;
    case "BEAR_MOM": return mom3 < -0.001 && r < 46;
    case "RANGE_DOWN": return r > 40 && r < 54 && price <= mean;
    case "SQUEEZE_DOWN": return band / Math.max(price, 1) < 0.002 && mom3 < -0.0008;
    case "OPENING_BEAR": return mom3 < -0.0012 && price < fast;
    case "REJECT_BEAR": return price < mean && values.length > 2 && values[values.length - 2] >= mean;
    case "LOWER_HIGH": return price < slow && values.slice(-5)[0] > values.slice(-3)[0];
    default: return false;
  }
}

function passesEntryConfirmation(def: StratDef, values: number[]): boolean {
  const r = rsi(values, 14);
  const mom3 = momentum(values, 3);

  if (def.optionType === "PUT") {
    switch (def.category) {
      case "Trend":
      case "Momentum":
        // signal already validated direction; just gate on RSI not oversold and not strongly bearish
        return mom3 > -0.002 && r >= 44;
      case "VWAP":
        return r >= 44;
      case "Mean Reversion":
        return r >= 38 && r <= 68;
      case "Volatility":
        return Math.abs(mom3) <= 0.005;
      default:
        return mom3 >= -0.003;
    }
  }

  switch (def.category) {
    case "Trend":
    case "Momentum":
      // signal already validated direction; just gate on RSI not overbought and not strongly bullish
      return mom3 < 0.002 && r <= 56;
    case "VWAP":
      return r <= 56;
    case "Mean Reversion":
      return r >= 32 && r <= 62;
    case "Volatility":
      return Math.abs(mom3) <= 0.005;
    default:
      return mom3 <= 0.003;
  }
}

function initEngine(): EngineRef {
  return {
    balance: INITIAL_BALANCE,
    totalWins: 0,
    totalLosses: 0,
    totalRealizedPnl: 0,
    totalPremiumSpent: 0,
    seq: 0,
    lastPrice: 0,
    lastMinute: 0,
    minuteBars: [],
    positions: [],
    trades: [],
    strategies: STRATEGIES.map((def) => ({
      def,
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      score: 0,
      status: "WATCHLIST",
      rosterState: "ACTIVE",
      hasPosition: false,
      cooldownUntil: 0,
      regime: "UNKNOWN",
      consecutiveLosses: 0,
    })),
  };
}

function buildPersistedPayload(eng: EngineRef): NiftySellingDbPayload {
  return {
    balance: eng.balance,
    totalWins: eng.totalWins,
    totalLosses: eng.totalLosses,
    totalPnl: eng.totalRealizedPnl,
    totalPremiumSpent: eng.totalPremiumSpent,
    tradeSeq: eng.seq,
    lastPrice: eng.lastPrice,
    lastMinute: eng.lastMinute,
    minuteBars: eng.minuteBars,
    positions: eng.positions,
    trades: eng.trades.slice(0, 500),
    strategies: eng.strategies.map((strategy) => ({
      id: strategy.def.id,
      totalTrades: strategy.totalTrades,
      wins: strategy.wins,
      losses: strategy.losses,
      totalPnl: strategy.totalPnl,
      score: strategy.score,
      status: strategy.status,
      rosterState: strategy.rosterState,
      hasPosition: strategy.hasPosition,
      cooldownUntil: strategy.cooldownUntil,
      regime: strategy.regime,
      consecutiveLosses: strategy.consecutiveLosses,
    })),
  };
}

async function saveSellingState(eng: EngineRef): Promise<void> {
  try {
    await fetch("/api/nifty/selling-state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildPersistedPayload(eng)),
    });
  } catch {
    // non-critical
  }
}

async function loadSellingState(eng: EngineRef): Promise<boolean> {
  try {
    const res = await fetch("/api/nifty/selling-state");
    if (!res.ok) return false;
    const data = await res.json() as { ok: boolean; found: boolean; state?: NiftySellingDbPayload };
    if (!data.ok || !data.found || !data.state) return false;
    const saved = data.state;
    eng.balance = saved.balance;
    eng.totalWins = saved.totalWins;
    eng.totalLosses = saved.totalLosses;
    eng.totalRealizedPnl = saved.totalPnl;
    eng.totalPremiumSpent = saved.totalPremiumSpent;
    eng.seq = saved.tradeSeq;
    eng.lastPrice = saved.lastPrice;
    eng.lastMinute = saved.lastMinute;
    eng.minuteBars = saved.minuteBars ?? [];
    eng.positions = saved.positions ?? [];
    eng.trades = (saved.trades ?? []).slice(0, 500);
    for (const strategy of eng.strategies) {
      const persisted = (saved.strategies ?? []).find((item) => item.id === strategy.def.id);
      if (!persisted) continue;
      strategy.totalTrades = persisted.totalTrades;
      strategy.wins = persisted.wins;
      strategy.losses = persisted.losses;
      strategy.totalPnl = persisted.totalPnl;
      strategy.score = persisted.score;
      strategy.status = persisted.status;
      strategy.rosterState = persisted.rosterState;
      strategy.hasPosition = persisted.hasPosition;
      strategy.cooldownUntil = persisted.cooldownUntil;
      strategy.regime = persisted.regime;
      strategy.consecutiveLosses = persisted.consecutiveLosses;
    }
    // Recover from hasPosition desync: if strategy says it has a position but none exists, reset it
    for (const strategy of eng.strategies) {
      if (strategy.hasPosition && !eng.positions.some((p) => p.strategyId === strategy.def.id)) {
        strategy.hasPosition = false;
        strategy.status = "WATCHLIST";
      }
    }
    return true;
  } catch {
    return false;
  }
}

function niftySellingSizeMultiplier(strategy: InternalStrategy): number {
  let multiplier = 1;
  const winRate = strategy.totalTrades > 0 ? (strategy.wins / strategy.totalTrades) * 100 : 0;
  if (strategy.totalTrades >= 6 && winRate >= 55) multiplier += 0.15;
  if (strategy.totalTrades >= 12 && winRate >= 60) multiplier += 0.2;
  if (strategy.totalTrades > 0) {
    const avgPnlRatio = strategy.totalPnl / (strategy.totalTrades * strategy.def.marginINR);
    if (avgPnlRatio > 0.07) multiplier += 0.18;
    if (avgPnlRatio < -0.04) multiplier -= 0.15;
  }
  multiplier -= strategy.consecutiveLosses * 0.1;
  return Math.max(NIFTY_SELL_MIN_SIZE_MULTIPLIER, Math.min(NIFTY_SELL_MAX_SIZE_MULTIPLIER, multiplier));
}

function niftySellingCooldownMs(strategy: InternalStrategy, won: boolean): number {
  const base = strategy.def.cooldownSecs * 1000;
  if (won) return base;
  return Math.round(base * (1 + strategy.consecutiveLosses * NIFTY_SELL_LOSS_COOLDOWN_PENALTY));
}

function shouldPauseNiftySeller(strategy: InternalStrategy): boolean {
  const winRate = strategy.totalTrades > 0 ? (strategy.wins / strategy.totalTrades) * 100 : 0;
  return strategy.totalTrades >= NIFTY_SELL_UNDERPERFORMING_MIN_TRADES && strategy.totalPnl < 0 && winRate < NIFTY_SELL_UNDERPERFORMING_MAX_WINRATE;
}

function buildDisplayPositions(eng: EngineRef): OptionPosition[] {
  return eng.positions.map((position) => ({
    id: position.id,
    strategyId: position.strategyId,
    strategyName: position.strategyName,
    optionType: position.optionType,
    strike: position.strike,
    expiryTime: position.expiryTime,
    entryPremium: position.entryPremium,
    currentPremium: position.currentPremium,
    quantity: position.quantity,
    costBasis: position.marginBlocked,
    entryBtcPrice: position.entryNiftyPrice,
    entryTime: position.entryTime,
    unrealizedPnl: position.unrealizedPnl,
    iv: position.iv,
    delta: position.delta,
  }));
}

function buildDisplayStrategies(eng: EngineRef): OptionStrategyStatus[] {
  return eng.strategies.map((strategy) => ({
    strategyId: strategy.def.id,
    name: strategy.def.name,
    category: strategy.def.category,
    optionType: strategy.def.optionType,
    rosterState: strategy.rosterState,
    score: strategy.score,
    regime: strategy.regime,
    regimeFit: strategy.regime === "RANGE" ? 0.7 : 0.82,
    allocationUsd: strategy.def.marginINR,
    totalTrades: strategy.totalTrades,
    wins: strategy.wins,
    losses: strategy.losses,
    totalPnl: strategy.totalPnl,
    winRate: strategy.totalTrades > 0 ? (strategy.wins / strategy.totalTrades) * 100 : 0,
    shadowTrades: 0,
    shadowWins: 0,
    shadowLosses: 0,
    shadowPnl: 0,
    shadowWinRate: 0,
    shadowSignals: 0,
    sizeMultiplier: 1,
    status: strategy.status,
    hasPosition: strategy.hasPosition,
    hasShadowPosition: false,
  }));
}

function buildDisplayStats(eng: EngineRef): OptionStats {
  const unrealizedPnl = eng.positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const lockedMargin = eng.positions.reduce((sum, position) => sum + position.marginBlocked, 0);
  return {
    balance: eng.balance,
    equity: eng.balance + lockedMargin + unrealizedPnl,
    totalTrades: eng.trades.length,
    openPositions: eng.positions.length,
    totalWins: eng.totalWins,
    totalLosses: eng.totalLosses,
    winRate: eng.totalWins + eng.totalLosses > 0 ? (eng.totalWins / (eng.totalWins + eng.totalLosses)) * 100 : 0,
    totalPnl: eng.totalRealizedPnl,
    totalPremiumSpent: eng.totalPremiumSpent,
    unrealizedPnl,
  };
}

function closePosition(eng: EngineRef, strategy: InternalStrategy, position: InternalPosition, currentPrice: number, reason: string) {
  const trade: OptionTrade = {
    id: position.id,
    strategyId: position.strategyId,
    strategyName: position.strategyName,
    optionType: position.optionType,
    strike: position.strike,
    expiryMins: DTE_DAYS * 24 * 60,
    entryPremium: position.entryPremium,
    exitPremium: position.currentPremium,
    quantity: position.quantity,
    costBasis: position.marginBlocked,
    netPnl: position.unrealizedPnl,
    returnPct: position.marginBlocked > 0 ? (position.unrealizedPnl / position.marginBlocked) * 100 : 0,
    entryBtcPrice: position.entryNiftyPrice,
    exitBtcPrice: currentPrice,
    entryTime: position.entryTime,
    exitTime: new Date().toISOString(),
    exitReason: reason,
  };

  eng.balance += position.marginBlocked + position.unrealizedPnl;
  eng.totalRealizedPnl += position.unrealizedPnl;
  strategy.totalTrades++;
  if (position.unrealizedPnl >= 0) {
    eng.totalWins++;
    strategy.wins++;
    strategy.consecutiveLosses = 0;
  } else {
    eng.totalLosses++;
    strategy.losses++;
    strategy.consecutiveLosses += 1;
  }
  strategy.totalPnl += position.unrealizedPnl;
  strategy.status = "COOLING";
  strategy.hasPosition = false;
  strategy.cooldownUntil = Date.now() + niftySellingCooldownMs(strategy, position.unrealizedPnl >= 0);
  eng.positions = eng.positions.filter((item) => item.id !== position.id);
  eng.trades.unshift(trade);
}

export default function useNiftyOptionsSellingEngine() {
  const engRef = useRef<EngineRef>(initEngine());
  const lastSavedSignatureRef = useRef("");
  const dbLoadedRef = useRef(false);
  const [positions, setPositions] = useState<OptionPosition[]>([]);
  const [trades, setTrades] = useState<OptionTrade[]>([]);
  const [strategies, setStrategies] = useState<OptionStrategyStatus[]>(buildDisplayStrategies(engRef.current));
  const [stats, setStats] = useState<OptionStats>(buildDisplayStats(engRef.current));
  const [barCount, setBarCount] = useState(0);
  const [enginePrice, setEnginePrice] = useState(0);

  const pushDisplayState = useCallback(() => {
    const eng = engRef.current;
    setPositions(buildDisplayPositions(eng));
    setTrades(eng.trades);
    setStrategies(buildDisplayStrategies(eng));
    setStats(buildDisplayStats(eng));
    setBarCount(eng.minuteBars.length);
    setEnginePrice(eng.lastPrice);
    if (!dbLoadedRef.current) return;
    const signature = JSON.stringify({
      balance: eng.balance,
      totalWins: eng.totalWins,
      totalLosses: eng.totalLosses,
      totalPnl: eng.totalRealizedPnl,
      tradeSeq: eng.seq,
      lastPrice: eng.lastPrice,
      bars: eng.minuteBars.length,
      openPositions: eng.positions.map((position) => position.id),
      tradeIds: eng.trades.slice(0, 500).map((trade) => trade.id),
    });
    if (signature !== lastSavedSignatureRef.current) {
      lastSavedSignatureRef.current = signature;
      void saveSellingState(eng);
    }
  }, []);

  const tickEngine = useCallback(() => {
    const eng = engRef.current;
    if (eng.lastPrice <= 0 || eng.minuteBars.length < 2) return;
    const bars = [...eng.minuteBars, eng.lastPrice];
    const regime = classifyRegime(bars);
    const currentPrice = eng.lastPrice;
    const iv = Math.max(0.12, Math.min(0.28, BASE_IV + stddev(bars.slice(-20)) / Math.max(currentPrice, 1)));

    for (const position of [...eng.positions]) {
      // barsHeld = minutes elapsed since entry (time-based, not tick-based)
      position.barsHeld = Math.floor((Date.now() - new Date(position.entryTime).getTime()) / 60_000);
      position.currentPremium = markPremium(position.entryPremium, position.entryNiftyPrice, currentPrice, position.optionType, position.barsHeld);
      position.unrealizedPnl = (position.entryPremium - position.currentPremium) * position.quantity;
      if (position.entryPremium > 0) {
        position.peakGainPct = Math.max(position.peakGainPct, (position.entryPremium-position.currentPremium)/position.entryPremium);
      }
      position.iv = iv;
      const strategy = eng.strategies.find((item) => item.def.id === position.strategyId);
      if (!strategy) continue;
      const progress = position.marginBlocked > 0 ? position.unrealizedPnl / position.marginBlocked : 0;
      const timeProgress = position.barsHeld / 120;
      const tpPremium = position.entryPremium * (1 - strategy.def.tpPct);
      const slPremium = position.entryPremium * (1 + strategy.def.slPct);

      if (position.currentPremium <= tpPremium) {
        closePosition(eng, strategy, position, currentPrice, "TP");
      } else if (position.currentPremium >= slPremium) {
        closePosition(eng, strategy, position, currentPrice, "SL");
      } else if (timeProgress >= GRIND_EXIT_PROGRESS && progress >= Math.max(LATE_EXIT_MIN_GAIN, strategy.def.tpPct * GRIND_EXIT_SHARE)) {
        closePosition(eng, strategy, position, currentPrice, "PROFIT_LOCK");
      } else if (timeProgress >= PROFIT_LOCK_PROGRESS && progress >= strategy.def.tpPct * PROFIT_LOCK_SHARE) {
        closePosition(eng, strategy, position, currentPrice, "PROFIT_LOCK");
      } else if (position.peakGainPct >= Math.max(LATE_EXIT_MIN_GAIN, strategy.def.tpPct * 0.4) && progress > 0 && progress <= position.peakGainPct * (1 - NIFTY_SELL_TRAIL_GIVEBACK_SHARE)) {
        closePosition(eng, strategy, position, currentPrice, "TRAIL_STOP");
      } else if (timeProgress >= LATE_EXIT_PROGRESS && progress >= LATE_EXIT_MIN_GAIN) {
        closePosition(eng, strategy, position, currentPrice, "LATE_EXIT");
      }
    }

    for (const strategy of eng.strategies) {
      // eslint-disable-next-line react-hooks/immutability
      strategy.regime = regime;
      if (strategy.rosterState === "DISABLED") {
        strategy.status = "DISABLED";
        continue;
      }
      if (strategy.hasPosition) {
        strategy.status = "IN_POSITION";
        continue;
      }
      if (shouldPauseNiftySeller(strategy)) {
        strategy.status = "COOLING";
        strategy.score = 0;
        strategy.cooldownUntil = Math.max(strategy.cooldownUntil, Date.now() + NIFTY_SELL_UNDERPERFORMING_PAUSE_MS);
        continue;
      }
      if (strategy.cooldownUntil > Date.now()) {
        strategy.status = "COOLING";
        continue;
      }
      if (bars.length < strategy.def.minBars) {
        strategy.status = "WATCHLIST";
        continue;
      }

      const fired = evalSignal(strategy.def.signal, bars);
      strategy.score = fired ? 78 : 0;
      strategy.status = fired ? "READY" : "WATCHLIST";

      if (!fired || !passesEntryConfirmation(strategy.def, bars)) continue;
      if (eng.positions.length >= MAX_CONCURRENT || eng.balance < strategy.def.marginINR) continue;

      const strikeOffset = strategy.def.optionType === "CALL" ? 1 : -1;
      const strike = roundStrike(currentPrice, strikeOffset);
      const premium = estimatePremium(currentPrice, strike, strategy.def.optionType, iv);
      const sizedMargin = strategy.def.marginINR * niftySellingSizeMultiplier(strategy);
      const lots = Math.max(1, Math.floor(sizedMargin / Math.max(premium * LOT_SIZE, 1)));
      const quantity = lots * LOT_SIZE;
      const marginBlocked = Math.max(strategy.def.marginINR * NIFTY_SELL_MIN_SIZE_MULTIPLIER, sizedMargin);

      eng.balance -= marginBlocked;
      eng.totalPremiumSpent += premium * quantity;
      eng.positions.push({
        id: `NIFTY-SHORT-${Date.now()}-${eng.seq++}`,
        strategyId: strategy.def.id,
        strategyName: strategy.def.name,
        optionType: strategy.def.optionType,
        strike,
        expiryTime: new Date(Date.now() + DTE_DAYS * 24 * 60 * 60 * 1000).toISOString(),
        entryPremium: premium,
        currentPremium: premium,
        quantity,
        marginBlocked,
        entryNiftyPrice: currentPrice,
        entryTime: new Date().toISOString(),
        unrealizedPnl: 0,
        peakGainPct: 0,
        iv,
        delta: strategy.def.optionType === "CALL" ? -0.24 : 0.24,
        barsHeld: 0,
      });
      strategy.hasPosition = true;
      strategy.status = "IN_POSITION";
    }

    pushDisplayState();
  }, [pushDisplayState]);

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
    tickEngine();
  }, [tickEngine]);

  useEffect(() => {
    void loadSellingState(engRef.current).then(() => {
      dbLoadedRef.current = true;
      lastSavedSignatureRef.current = JSON.stringify({
        balance: engRef.current.balance,
        totalWins: engRef.current.totalWins,
        totalLosses: engRef.current.totalLosses,
        totalPnl: engRef.current.totalRealizedPnl,
        tradeSeq: engRef.current.seq,
        lastPrice: engRef.current.lastPrice,
        bars: engRef.current.minuteBars.length,
        openPositions: engRef.current.positions.map((position) => position.id),
        tradeIds: engRef.current.trades.slice(0, 500).map((trade) => trade.id),
      });
      pushDisplayState();
    });
  }, [pushDisplayState]);

  useEffect(() => {
    let cancelled = false;
    const source = new EventSource("/api/nifty/stream");
    source.onmessage = (event) => {
      if (cancelled) return;
      try {
        const data = JSON.parse(event.data as string) as { price?: number };
        if (data.price && data.price > 0) feedPrice(data.price);
      } catch {
        // ignore malformed events
      }
    };
    return () => {
      cancelled = true;
      source.close();
    };
  }, [feedPrice]);

  useEffect(() => {
    const seed = async () => {
      try {
        const res = await fetch("/api/nifty/candles?interval=ONE_MINUTE");
        if (!res.ok) return;
        const data = await res.json() as { ok: boolean; candles?: Candle[] };
        if (!data.ok || !data.candles?.length) return;
        const closes = data.candles.map((candle) => candle.close).filter((close) => close > 0);
        if (!closes.length) return;
        engRef.current.minuteBars = closes.slice(-MAX_BARS);
        engRef.current.lastPrice = closes[closes.length - 1] ?? 0;
        engRef.current.lastMinute = Math.floor((data.candles[data.candles.length - 1]?.time ?? 0) / 60000);
        pushDisplayState();
      } catch {
        // ignore seed failures
      }
    };
    void seed();
  }, [pushDisplayState]);

  useEffect(() => {
    const id = setInterval(() => void tickEngine(), TICK_MS);
    return () => clearInterval(id);
  }, [tickEngine]);

  useEffect(() => {
    const id = setInterval(() => {
      if (dbLoadedRef.current) void saveSellingState(engRef.current);
    }, 60_000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    const onUnload = () => {
      if (!dbLoadedRef.current) return;
      const payload = JSON.stringify(buildPersistedPayload(engRef.current));
      navigator.sendBeacon("/api/nifty/selling-state", new Blob([payload], { type: "application/json" }));
    };
    window.addEventListener("beforeunload", onUnload);
    return () => window.removeEventListener("beforeunload", onUnload);
  }, []);

  const clearAll = useCallback(() => {
    engRef.current = initEngine();
    lastSavedSignatureRef.current = "";
    if (dbLoadedRef.current) void saveSellingState(engRef.current);
    pushDisplayState();
  }, [pushDisplayState]);

  return { positions, trades, strategies, stats, clearAll, barCount, enginePrice };
}
