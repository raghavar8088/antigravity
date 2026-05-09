"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type MutableRefObject } from "react";

/** Paper desk sized for small accounts — $2 min net PnL is enforced at close time. */
const INITIAL_BALANCE = 100;
const MIN_NOTIONAL_USD = 10;
const MAX_NOTIONAL_USD = 35;
const MAX_OPEN_POSITIONS = 8;
const MAX_BARS = 120;
const MIN_BARS = 18;
const SIGNAL_THRESHOLD = 55;
const POLL_MS = 2_000;
const MAX_TRADES = 2_000;
/** Taker-style round trip (entry + exit) — conservative vs Binance 0.2% VIP0. */
const ROUND_TRIP_FEE_FRAC = 0.0015;
const MAX_DRAWDOWN_LOCK_PCT = 22; // pause new entries if equity drawdown breaches this level

/** Fee breakeven — early exits must beat this to be net positive. */
const FEE_BREAKEVEN_PCT = ROUND_TRIP_FEE_FRAC * 100; // 0.15

const PROFIT_LOCK_PROGRESS = 0.55;
const PROFIT_LOCK_SHARE = 0.55;
const LATE_EXIT_PROGRESS = 0.65;
const LATE_EXIT_MIN_GAIN = 0.18;
const GRIND_EXIT_PROGRESS = 0.55;
const GRIND_EXIT_SHARE = 0.45;
const TRAIL_ACTIVATION_PCT = 0.25;
const TRAIL_GIVEBACK_SHARE = 0.20;
const LOSS_COOLDOWN_PENALTY = 0.4;
const VOL_SPIKE_RATIO = 1.45;
const VOL_BOOST_POINTS = 4;
const VOL_HISTORY = 24;

/** Each closed trade records at least this absolute net PnL (after fees) on the paper ledger. */
const MIN_ABS_NET_PNL_USD = 2;

/** Stable localStorage key — NEVER change this. Old keys are migrated on load. */
const LS_KEY = "btc_spot_scalper_paper_state";
const LS_LEGACY_KEYS = [
  "btc_spot_scalper_paper_v6",
  "btc_spot_scalper_paper_v5",
  "btc_spot_scalper_paper_v4",
  "btc_spot_scalper_paper_v3",
];
const LS_PAUSE_ENTRIES = "btc_spot_pause_entries_v1";

/** Clip notion band for dashboard copy. */
export const BTC_SPOT_CLIP_USD = { min: MIN_NOTIONAL_USD, max: MAX_NOTIONAL_USD } as const;

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
  bbUpper: number;
  bbLower: number;
  bbWidth: number;
  stochK: number;
  stochD: number;
  prevStochK: number;
  prevStochD: number;
  macdLine: number;
  macdSignal: number;
  prevMacdLine: number;
  prevMacdSignal: number;
  atr14: number;
  obvSlope: number;
  momentum10: number;
  rsi7: number;
  williamsR: number;
  cci20: number;
  roc10: number;
  keltnerUpper: number;
  keltnerLower: number;
  donchianHigh: number;
  donchianLow: number;
  donchianMid: number;
  vwapDev: number;
  adxProxy: number;
  ema5: number;
  ema13: number;
  prevEma5: number;
  prevEma13: number;
  rsi21: number;
  macdHist: number;
  prevMacdHist: number;
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
  /** 1m tape regime from latest bar (warmup uses "WARMING"). */
  lastRegime: string;
  lastLocalSaveAt: number;
  lastServerSaveAt: number;
  serverSyncConfigured: boolean;
  /** High-water equity mark for drawdown-from-peak (this browser session + restored book). */
  sessionPeakEquity: number;
  lastVolRatio: number;
  lastRsi14: number;
}

export type BTCSpotQuote = {
  symbol: string;
  ltp: number;
  changePct24h: number;
  signalScore: number;
  hasPosition: boolean;
  sparkline: number[];
  /** Latest 1m volume vs trailing average (≈1 = normal). */
  volRatio: number;
  rsi14: number;
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
  marketRegime: string;
  /** Average net PnL per winning trade (after fees). */
  avgWinUsd: number | null;
  /** Average absolute loss per losing trade (after fees). */
  avgLossUsd: number | null;
  /** Gross wins / gross losses; null when no losses yet (UI may show ∞). */
  profitFactor: number | null;
  /** Realized PnL / closed trade count. */
  expectancyPerTradeUsd: number | null;
  persistence: {
    lastLocalSaveAt: number | null;
    lastServerSaveAt: number | null;
    serverSyncConfigured: boolean;
  };
  /** Drawdown from session peak equity, percent (0 at peak). */
  maxDrawdownFromPeakPct: number;
  sessionPeakEquity: number;
  volRatio: number;
  rsi14: number;
  winStreak: number;
  lossStreak: number;
  exitReasonCounts: Record<string, number>;
  bestTradeUsd: number | null;
  worstTradeUsd: number | null;
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
  { id: 1, name: "Micro Range Breakout", category: "Breakout", side: "LONG", signal: "BREAKOUT", tpPct: 0.70, slPct: 0.35, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 2, name: "Micro Range Breakdown", category: "Breakout", side: "SHORT", signal: "BREAKOUT_SHORT", tpPct: 0.70, slPct: 0.35, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 3, name: "EMA Ribbon Impulse Long", category: "Momentum", side: "LONG", signal: "EMA_CROSS", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 4, name: "EMA Ribbon Impulse Short", category: "Momentum", side: "SHORT", signal: "EMA_CROSS_SHORT", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 5, name: "RSI 1m Oversold Bounce", category: "Mean Reversion", side: "LONG", signal: "RSI_BOUNCE", tpPct: 0.55, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 6, name: "RSI 1m Overbought Fade", category: "Mean Reversion", side: "SHORT", signal: "RSI_BOUNCE_SHORT", tpPct: 0.55, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 7, name: "Session VWAP Reclaim", category: "VWAP", side: "LONG", signal: "VWAP_RECLAIM", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 8, name: "Session VWAP Reject", category: "VWAP", side: "SHORT", signal: "VWAP_RECLAIM_SHORT", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 9, name: "Trend Leg Continuation", category: "Trend", side: "LONG", signal: "TREND_CONT", tpPct: 0.80, slPct: 0.35, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 50 },
  { id: 10, name: "Trend Leg Exhaustion Short", category: "Trend", side: "SHORT", signal: "TREND_CONT_SHORT", tpPct: 0.80, slPct: 0.35, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 50 },
  { id: 11, name: "Micro Breakout Sprint Long", category: "Breakout", side: "LONG", signal: "BREAKOUT", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 12, name: "Micro Breakout Sprint Short", category: "Breakout", side: "SHORT", signal: "BREAKOUT_SHORT", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 13, name: "Breakout Continuation Long", category: "Breakout", side: "LONG", signal: "BREAKOUT", tpPct: 0.75, slPct: 0.35, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 50 },
  { id: 14, name: "Breakout Continuation Short", category: "Breakout", side: "SHORT", signal: "BREAKOUT_SHORT", tpPct: 0.75, slPct: 0.35, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 50 },
  { id: 15, name: "EMA Quick Cross Long", category: "Momentum", side: "LONG", signal: "EMA_CROSS", tpPct: 0.50, slPct: 0.25, cooldownMinutes: 4, minBars: MIN_BARS, holdMinutes: 25 },
  { id: 16, name: "EMA Quick Cross Short", category: "Momentum", side: "SHORT", signal: "EMA_CROSS_SHORT", tpPct: 0.50, slPct: 0.25, cooldownMinutes: 4, minBars: MIN_BARS, holdMinutes: 25 },
  { id: 17, name: "EMA Drive Long", category: "Momentum", side: "LONG", signal: "EMA_CROSS", tpPct: 0.68, slPct: 0.32, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 18, name: "EMA Drive Short", category: "Momentum", side: "SHORT", signal: "EMA_CROSS_SHORT", tpPct: 0.68, slPct: 0.32, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 19, name: "RSI Snapback Long", category: "Mean Reversion", side: "LONG", signal: "RSI_BOUNCE", tpPct: 0.48, slPct: 0.24, cooldownMinutes: 4, minBars: MIN_BARS, holdMinutes: 24 },
  { id: 20, name: "RSI Snapback Short", category: "Mean Reversion", side: "SHORT", signal: "RSI_BOUNCE_SHORT", tpPct: 0.48, slPct: 0.24, cooldownMinutes: 4, minBars: MIN_BARS, holdMinutes: 24 },
  { id: 21, name: "RSI Reversal Hold Long", category: "Mean Reversion", side: "LONG", signal: "RSI_BOUNCE", tpPct: 0.60, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 22, name: "RSI Reversal Hold Short", category: "Mean Reversion", side: "SHORT", signal: "RSI_BOUNCE_SHORT", tpPct: 0.60, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 23, name: "VWAP Pop Long", category: "VWAP", side: "LONG", signal: "VWAP_RECLAIM", tpPct: 0.50, slPct: 0.25, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 26 },
  { id: 24, name: "VWAP Fade Short", category: "VWAP", side: "SHORT", signal: "VWAP_RECLAIM_SHORT", tpPct: 0.50, slPct: 0.25, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 26 },
  { id: 25, name: "VWAP Trend Assist Long", category: "VWAP", side: "LONG", signal: "VWAP_RECLAIM", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 26, name: "VWAP Trend Assist Short", category: "VWAP", side: "SHORT", signal: "VWAP_RECLAIM_SHORT", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 27, name: "Trend Burst Long", category: "Trend", side: "LONG", signal: "TREND_CONT", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 36 },
  { id: 28, name: "Trend Burst Short", category: "Trend", side: "SHORT", signal: "TREND_CONT_SHORT", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 36 },
  { id: 29, name: "Trend Follow Through Long", category: "Trend", side: "LONG", signal: "TREND_CONT", tpPct: 0.85, slPct: 0.38, cooldownMinutes: 9, minBars: MIN_BARS, holdMinutes: 55 },
  { id: 30, name: "Trend Follow Through Short", category: "Trend", side: "SHORT", signal: "TREND_CONT_SHORT", tpPct: 0.85, slPct: 0.38, cooldownMinutes: 9, minBars: MIN_BARS, holdMinutes: 55 },

  // --- Bollinger Band strategies (31-36) ---
  { id: 31, name: "BB Squeeze Breakout Long", category: "Bollinger", side: "LONG", signal: "BB_SQUEEZE_LONG", tpPct: 0.68, slPct: 0.32, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 32, name: "BB Squeeze Breakdown Short", category: "Bollinger", side: "SHORT", signal: "BB_SQUEEZE_SHORT", tpPct: 0.68, slPct: 0.32, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 33, name: "BB Lower Band Bounce", category: "Bollinger MR", side: "LONG", signal: "BB_BOUNCE_LONG", tpPct: 0.55, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 34, name: "BB Upper Band Fade", category: "Bollinger MR", side: "SHORT", signal: "BB_BOUNCE_SHORT", tpPct: 0.55, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 35, name: "BB Band Walk Long", category: "Bollinger", side: "LONG", signal: "BB_WALK_LONG", tpPct: 0.72, slPct: 0.34, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 42 },
  { id: 36, name: "BB Band Walk Short", category: "Bollinger", side: "SHORT", signal: "BB_WALK_SHORT", tpPct: 0.72, slPct: 0.34, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 42 },

  // --- Stochastic strategies (37-42) ---
  { id: 37, name: "Stoch Golden Cross Long", category: "Stochastic", side: "LONG", signal: "STOCH_CROSS_LONG", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 32 },
  { id: 38, name: "Stoch Death Cross Short", category: "Stochastic", side: "SHORT", signal: "STOCH_CROSS_SHORT", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 32 },
  { id: 39, name: "Stoch Bullish Divergence", category: "Stochastic", side: "LONG", signal: "STOCH_DIVERGE_LONG", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 36 },
  { id: 40, name: "Stoch Bearish Divergence", category: "Stochastic", side: "SHORT", signal: "STOCH_DIVERGE_SHORT", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 36 },
  { id: 41, name: "Stoch Oversold Snap Long", category: "Stochastic", side: "LONG", signal: "STOCH_CROSS_LONG", tpPct: 0.50, slPct: 0.25, cooldownMinutes: 4, minBars: MIN_BARS, holdMinutes: 28 },
  { id: 42, name: "Stoch Overbought Snap Short", category: "Stochastic", side: "SHORT", signal: "STOCH_CROSS_SHORT", tpPct: 0.50, slPct: 0.25, cooldownMinutes: 4, minBars: MIN_BARS, holdMinutes: 28 },

  // --- MACD strategies (43-48) ---
  { id: 43, name: "MACD Bullish Cross", category: "MACD", side: "LONG", signal: "MACD_CROSS_LONG", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 44, name: "MACD Bearish Cross", category: "MACD", side: "SHORT", signal: "MACD_CROSS_SHORT", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 45, name: "MACD Hidden Bull Divergence", category: "MACD", side: "LONG", signal: "MACD_DIVERGE_LONG", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 34 },
  { id: 46, name: "MACD Hidden Bear Divergence", category: "MACD", side: "SHORT", signal: "MACD_DIVERGE_SHORT", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 34 },
  { id: 47, name: "MACD Momentum Surge Long", category: "MACD", side: "LONG", signal: "MACD_CROSS_LONG", tpPct: 0.78, slPct: 0.35, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 48 },
  { id: 48, name: "MACD Momentum Surge Short", category: "MACD", side: "SHORT", signal: "MACD_CROSS_SHORT", tpPct: 0.78, slPct: 0.35, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 48 },

  // --- OBV / Volume strategies (49-52) ---
  { id: 49, name: "OBV Accumulation Breakout", category: "Volume", side: "LONG", signal: "OBV_BREAKOUT_LONG", tpPct: 0.70, slPct: 0.32, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 50, name: "OBV Distribution Breakdown", category: "Volume", side: "SHORT", signal: "OBV_BREAKOUT_SHORT", tpPct: 0.70, slPct: 0.32, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 51, name: "OBV Trend Confirm Long", category: "Volume", side: "LONG", signal: "OBV_BREAKOUT_LONG", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 34 },
  { id: 52, name: "OBV Trend Confirm Short", category: "Volume", side: "SHORT", signal: "OBV_BREAKOUT_SHORT", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 34 },

  // --- Multi-indicator confluence (53-56) ---
  { id: 53, name: "Triple Indicator Bull", category: "Confluence", side: "LONG", signal: "TRIPLE_BULL", tpPct: 0.72, slPct: 0.33, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 44 },
  { id: 54, name: "Triple Indicator Bear", category: "Confluence", side: "SHORT", signal: "TRIPLE_BEAR", tpPct: 0.72, slPct: 0.33, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 44 },
  { id: 55, name: "Deep Oversold Reversal", category: "Confluence", side: "LONG", signal: "MEAN_REVERT_DEEP_LONG", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 56, name: "Deep Overbought Reversal", category: "Confluence", side: "SHORT", signal: "MEAN_REVERT_DEEP_SHORT", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 30 },

  // --- ATR / Volatility expansion (57-58) ---
  { id: 57, name: "Vol Expansion Surge Long", category: "Volatility", side: "LONG", signal: "ATR_EXPANSION_LONG", tpPct: 0.78, slPct: 0.36, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 50 },
  { id: 58, name: "Vol Expansion Surge Short", category: "Volatility", side: "SHORT", signal: "ATR_EXPANSION_SHORT", tpPct: 0.78, slPct: 0.36, cooldownMinutes: 8, minBars: MIN_BARS, holdMinutes: 50 },

  // --- Momentum acceleration (59-60) ---
  { id: 59, name: "Momentum Accelerator Long", category: "Momentum Accel", side: "LONG", signal: "MOM_ACCEL_LONG", tpPct: 0.70, slPct: 0.32, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 60, name: "Momentum Accelerator Short", category: "Momentum Accel", side: "SHORT", signal: "MOM_ACCEL_SHORT", tpPct: 0.70, slPct: 0.32, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 40 },

  // ====== Strategies 61-110: Williams %R, CCI, Keltner, Donchian, EMA Ribbon, VWAP Dev, ADX, ROC, MACD Hist, RSI Multi-TF, Squeeze, Dual Stoch, Vol Spike, Cloud, Swing, Exhaustion ======

  // Williams %R — oversold bounce / overbought fade
  { id: 61, name: "Williams %R Oversold Long", category: "Williams MR", side: "LONG", signal: "WILLIAMS_OVERSOLD_LONG", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 62, name: "Williams %R Overbought Short", category: "Williams MR", side: "SHORT", signal: "WILLIAMS_OVERSOLD_SHORT", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 30 },
  // Williams %R — midline momentum continuation
  { id: 63, name: "Williams Midline Long", category: "Williams Trend", side: "LONG", signal: "WILLIAMS_MIDLINE_LONG", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 64, name: "Williams Midline Short", category: "Williams Trend", side: "SHORT", signal: "WILLIAMS_MIDLINE_SHORT", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },

  // CCI — extreme oversold/overbought counter-trend
  { id: 65, name: "CCI Oversold Long", category: "CCI MR", side: "LONG", signal: "CCI_OVERSOLD_LONG", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 32 },
  { id: 66, name: "CCI Overbought Short", category: "CCI MR", side: "SHORT", signal: "CCI_OVERBOUGHT_SHORT", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 32 },
  // CCI — zero-line crossover trend
  { id: 67, name: "CCI Zero Cross Long", category: "CCI Trend", side: "LONG", signal: "CCI_ZERO_CROSS_LONG", tpPct: 0.55, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 28 },
  { id: 68, name: "CCI Zero Cross Short", category: "CCI Trend", side: "SHORT", signal: "CCI_ZERO_CROSS_SHORT", tpPct: 0.55, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 28 },

  // Keltner Channel — breakout through upper/lower band
  { id: 69, name: "Keltner Breakout Long", category: "Keltner", side: "LONG", signal: "KELTNER_BREAKOUT_LONG", tpPct: 0.68, slPct: 0.32, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 38 },
  { id: 70, name: "Keltner Breakout Short", category: "Keltner", side: "SHORT", signal: "KELTNER_BREAKOUT_SHORT", tpPct: 0.68, slPct: 0.32, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 38 },
  // Keltner Channel — mean reversion bounce off band
  { id: 71, name: "Keltner Bounce Long", category: "Keltner MR", side: "LONG", signal: "KELTNER_BOUNCE_LONG", tpPct: 0.55, slPct: 0.26, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 28 },
  { id: 72, name: "Keltner Bounce Short", category: "Keltner MR", side: "SHORT", signal: "KELTNER_BOUNCE_SHORT", tpPct: 0.55, slPct: 0.26, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 28 },

  // Donchian Channel — turtle-style breakout
  { id: 73, name: "Donchian Breakout Long", category: "Donchian Trend", side: "LONG", signal: "DONCHIAN_BREAK_LONG", tpPct: 0.72, slPct: 0.34, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 42 },
  { id: 74, name: "Donchian Breakout Short", category: "Donchian Trend", side: "SHORT", signal: "DONCHIAN_BREAK_SHORT", tpPct: 0.72, slPct: 0.34, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 42 },
  // Donchian Channel — midline cross
  { id: 75, name: "Donchian Mid Long", category: "Donchian Mid", side: "LONG", signal: "DONCHIAN_MID_LONG", tpPct: 0.52, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 28 },
  { id: 76, name: "Donchian Mid Short", category: "Donchian Mid", side: "SHORT", signal: "DONCHIAN_MID_SHORT", tpPct: 0.52, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 28 },

  // EMA Ribbon — fast 5/13 cross inside trend
  { id: 77, name: "EMA Ribbon Cross Long", category: "EMA Ribbon", side: "LONG", signal: "EMA_RIBBON_LONG", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 78, name: "EMA Ribbon Cross Short", category: "EMA Ribbon", side: "SHORT", signal: "EMA_RIBBON_SHORT", tpPct: 0.60, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },

  // VWAP Deviation — deep deviation mean reversion
  { id: 79, name: "VWAP Deviation Long", category: "VWAP MR", side: "LONG", signal: "VWAP_DEV_LONG", tpPct: 0.55, slPct: 0.26, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 28 },
  { id: 80, name: "VWAP Deviation Short", category: "VWAP MR", side: "SHORT", signal: "VWAP_DEV_SHORT", tpPct: 0.55, slPct: 0.26, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 28 },

  // ADX — strong directional trend following
  { id: 81, name: "ADX Strong Trend Long", category: "ADX Trend", side: "LONG", signal: "ADX_TREND_LONG", tpPct: 0.75, slPct: 0.35, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 45 },
  { id: 82, name: "ADX Strong Trend Short", category: "ADX Trend", side: "SHORT", signal: "ADX_TREND_SHORT", tpPct: 0.75, slPct: 0.35, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 45 },

  // Rate of Change — exhaustion reversal
  { id: 83, name: "ROC Reversal Long", category: "ROC MR", side: "LONG", signal: "ROC_REVERSAL_LONG", tpPct: 0.55, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 28 },
  { id: 84, name: "ROC Reversal Short", category: "ROC MR", side: "SHORT", signal: "ROC_REVERSAL_SHORT", tpPct: 0.55, slPct: 0.28, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 28 },
  // Rate of Change — momentum continuation
  { id: 85, name: "ROC Momentum Long", category: "ROC Trend", side: "LONG", signal: "ROC_MOMENTUM_LONG", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 86, name: "ROC Momentum Short", category: "ROC Trend", side: "SHORT", signal: "ROC_MOMENTUM_SHORT", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },

  // MACD Histogram — rising/falling histogram trend
  { id: 87, name: "MACD Histogram Rise Long", category: "MACD Hist", side: "LONG", signal: "MACD_HIST_RISE_LONG", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 32 },
  { id: 88, name: "MACD Histogram Fall Short", category: "MACD Hist", side: "SHORT", signal: "MACD_HIST_FALL_SHORT", tpPct: 0.58, slPct: 0.28, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 32 },

  // RSI Multi-Timeframe — triple-RSI oversold/overbought confluence
  { id: 89, name: "RSI Multi-TF Long", category: "RSI Multi", side: "LONG", signal: "RSI_MULTI_TF_LONG", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 32 },
  { id: 90, name: "RSI Multi-TF Short", category: "RSI Multi", side: "SHORT", signal: "RSI_MULTI_TF_SHORT", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 32 },

  // Squeeze Momentum — BB inside Keltner (low vol → explosive move)
  { id: 91, name: "Squeeze Momentum Long", category: "Squeeze", side: "LONG", signal: "SQUEEZE_MOM_LONG", tpPct: 0.72, slPct: 0.32, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 40 },
  { id: 92, name: "Squeeze Momentum Short", category: "Squeeze", side: "SHORT", signal: "SQUEEZE_MOM_SHORT", tpPct: 0.72, slPct: 0.32, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 40 },

  // Dual Stochastic — K/D cross in neutral RSI zone
  { id: 93, name: "Dual Stochastic Long", category: "Dual Stoch", side: "LONG", signal: "DUAL_STOCH_LONG", tpPct: 0.52, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 28 },
  { id: 94, name: "Dual Stochastic Short", category: "Dual Stoch", side: "SHORT", signal: "DUAL_STOCH_SHORT", tpPct: 0.52, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 28 },

  // Volume Spike — explosive volume + direction confirmation
  { id: 95, name: "Volume Spike Long", category: "Vol Spike", side: "LONG", signal: "VOL_SPIKE_LONG", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 32 },
  { id: 96, name: "Volume Spike Short", category: "Vol Spike", side: "SHORT", signal: "VOL_SPIKE_SHORT", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 32 },

  // Ichimoku-lite Cloud Break — EMA cloud breakout
  { id: 97, name: "Cloud Break Long", category: "Cloud", side: "LONG", signal: "CLOUD_BREAK_LONG", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 98, name: "Cloud Break Short", category: "Cloud", side: "SHORT", signal: "CLOUD_BREAK_SHORT", tpPct: 0.65, slPct: 0.30, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 35 },

  // Swing Failure — failed swing pivot reversal
  { id: 99, name: "Swing Fail Long", category: "Swing", side: "LONG", signal: "SWING_FAIL_LONG", tpPct: 0.50, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 25 },
  { id: 100, name: "Swing Fail Short", category: "Swing", side: "SHORT", signal: "SWING_FAIL_SHORT", tpPct: 0.50, slPct: 0.26, cooldownMinutes: 5, minBars: MIN_BARS, holdMinutes: 25 },

  // Exhaustion / Climax — extreme sell/buy climax reversal
  { id: 101, name: "Exhaustion Reversal Long", category: "Exhaustion", side: "LONG", signal: "EXHAUSTION_LONG", tpPct: 0.70, slPct: 0.34, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 35 },
  { id: 102, name: "Exhaustion Reversal Short", category: "Exhaustion", side: "SHORT", signal: "EXHAUSTION_SHORT", tpPct: 0.70, slPct: 0.34, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 35 },

  // Williams %R + CCI confluence — double oscillator confirmation
  { id: 103, name: "Williams+CCI Oversold Long", category: "Williams MR", side: "LONG", signal: "WILLIAMS_OVERSOLD_LONG", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 32 },
  { id: 104, name: "Williams+CCI Overbought Short", category: "Williams MR", side: "SHORT", signal: "WILLIAMS_OVERSOLD_SHORT", tpPct: 0.62, slPct: 0.30, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 32 },

  // Keltner + ADX — strong trend inside volatility band
  { id: 105, name: "Keltner ADX Trend Long", category: "Keltner", side: "LONG", signal: "KELTNER_BREAKOUT_LONG", tpPct: 0.75, slPct: 0.34, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 42 },
  { id: 106, name: "Keltner ADX Trend Short", category: "Keltner", side: "SHORT", signal: "KELTNER_BREAKOUT_SHORT", tpPct: 0.75, slPct: 0.34, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 42 },

  // ROC + Stochastic — rate-of-change with stochastic filter
  { id: 107, name: "ROC Stoch Reversal Long", category: "ROC MR", side: "LONG", signal: "ROC_REVERSAL_LONG", tpPct: 0.60, slPct: 0.30, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 30 },
  { id: 108, name: "ROC Stoch Reversal Short", category: "ROC MR", side: "SHORT", signal: "ROC_REVERSAL_SHORT", tpPct: 0.60, slPct: 0.30, cooldownMinutes: 7, minBars: MIN_BARS, holdMinutes: 30 },

  // Donchian + MACD — channel break with histogram confirmation
  { id: 109, name: "Donchian MACD Long", category: "Donchian Trend", side: "LONG", signal: "DONCHIAN_BREAK_LONG", tpPct: 0.78, slPct: 0.36, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 44 },
  { id: 110, name: "Donchian MACD Short", category: "Donchian Trend", side: "SHORT", signal: "DONCHIAN_BREAK_SHORT", tpPct: 0.78, slPct: 0.36, cooldownMinutes: 6, minBars: MIN_BARS, holdMinutes: 44 },
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

function stochastic(bars: number[], kPeriod: number, dPeriod: number): { k: number; d: number; prevK: number; prevD: number } {
  if (bars.length < kPeriod + dPeriod) return { k: 50, d: 50, prevK: 50, prevD: 50 };
  const kValues: number[] = [];
  for (let i = kPeriod - 1; i < bars.length; i++) {
    const window = bars.slice(i - kPeriod + 1, i + 1);
    const high = Math.max(...window);
    const low = Math.min(...window);
    kValues.push(high !== low ? ((bars[i] - low) / (high - low)) * 100 : 50);
  }
  const dValues: number[] = [];
  for (let i = dPeriod - 1; i < kValues.length; i++) {
    dValues.push(kValues.slice(i - dPeriod + 1, i + 1).reduce((a, b) => a + b, 0) / dPeriod);
  }
  return {
    k: kValues[kValues.length - 1] ?? 50,
    d: dValues[dValues.length - 1] ?? 50,
    prevK: kValues[kValues.length - 2] ?? 50,
    prevD: dValues[dValues.length - 2] ?? 50,
  };
}

function macd(bars: number[], fastP: number, slowP: number, sigP: number): { line: number; signal: number; prevLine: number; prevSignal: number } {
  if (bars.length < slowP + sigP) return { line: 0, signal: 0, prevLine: 0, prevSignal: 0 };
  const macdValues: number[] = [];
  for (let i = slowP; i <= bars.length; i++) {
    const slice = bars.slice(0, i);
    macdValues.push(ema(slice, fastP) - ema(slice, slowP));
  }
  const sigValues: number[] = [];
  for (let i = sigP; i <= macdValues.length; i++) {
    sigValues.push(ema(macdValues.slice(0, i), sigP));
  }
  return {
    line: macdValues[macdValues.length - 1] ?? 0,
    signal: sigValues[sigValues.length - 1] ?? 0,
    prevLine: macdValues[macdValues.length - 2] ?? 0,
    prevSignal: sigValues[sigValues.length - 2] ?? 0,
  };
}

function atr(bars: number[], period: number): number {
  if (bars.length < period + 1) return 0;
  let sum = 0;
  for (let i = bars.length - period; i < bars.length; i++) {
    sum += Math.abs(bars[i] - bars[i - 1]);
  }
  return sum / period;
}

function obvSlope(bars: number[], volumes: number[], lookback: number): number {
  if (bars.length < lookback + 1 || volumes.length < bars.length) return 0;
  let obv = 0;
  const obvArr: number[] = [0];
  const start = Math.max(1, bars.length - lookback - 5);
  for (let i = start; i < bars.length; i++) {
    const vol = volumes[i] ?? 0;
    if (bars[i] > bars[i - 1]) obv += vol;
    else if (bars[i] < bars[i - 1]) obv -= vol;
    obvArr.push(obv);
  }
  if (obvArr.length < 2) return 0;
  const mid = Math.floor(obvArr.length / 2);
  const recent = obvArr.slice(mid);
  const older = obvArr.slice(0, mid);
  const recentAvg = recent.reduce((a, b) => a + b, 0) / recent.length;
  const olderAvg = older.reduce((a, b) => a + b, 0) / older.length;
  const maxVol = Math.max(...volumes.slice(-lookback).map(Math.abs), 1);
  return (recentAvg - olderAvg) / maxVol;
}

function williamsR(bars: number[], period: number): number {
  if (bars.length < period) return -50;
  const window = bars.slice(-period);
  const high = Math.max(...window);
  const low = Math.min(...window);
  return high === low ? -50 : ((high - bars[bars.length - 1]) / (high - low)) * -100;
}

function cci(bars: number[], period: number): number {
  if (bars.length < period) return 0;
  const slice = bars.slice(-period);
  const tp = slice.reduce((s, v) => s + v, 0) / period;
  const md = slice.reduce((s, v) => s + Math.abs(v - tp), 0) / period;
  return md === 0 ? 0 : (bars[bars.length - 1] - tp) / (0.015 * md);
}

function rateOfChange(bars: number[], period: number): number {
  if (bars.length <= period) return 0;
  const prev = bars[bars.length - 1 - period];
  return prev === 0 ? 0 : ((bars[bars.length - 1] - prev) / prev) * 100;
}

function adxProxy(bars: number[], period: number): number {
  if (bars.length < period + 1) return 0;
  let plusSum = 0;
  let minusSum = 0;
  for (let i = bars.length - period; i < bars.length; i++) {
    const diff = bars[i] - bars[i - 1];
    if (diff > 0) plusSum += diff;
    else minusSum += Math.abs(diff);
  }
  const total = plusSum + minusSum;
  if (total === 0) return 0;
  return Math.abs(plusSum - minusSum) / total * 100;
}

function buildSignalInputs(bars: number[], volRatio: number, volumes?: number[]): SignalInputs {
  const last = bars.length - 1;
  const price = bars[last];
  const previous = bars.slice(0, -1);
  const m20 = sma(bars, 20);
  const s20 = stdDev(bars, 20);
  const stoch = stochastic(bars, 14, 3);
  const mc = macd(bars, 12, 26, 9);
  return {
    price,
    prevPrice: last > 0 ? bars[last - 1] : price,
    fast: ema(bars, 8),
    slow: ema(bars, 21),
    trend: ema(bars, 34),
    prevFast: ema(previous, 8),
    prevSlow: ema(previous, 21),
    mean20: m20,
    std20: s20,
    rsi14: rsi(bars, 14),
    high20: Math.max(...bars.slice(-20)),
    low20: Math.min(...bars.slice(-20)),
    momentum3: last >= 3 ? ((price - bars[last - 3]) / bars[last - 3]) * 100 : 0,
    momentum6: last >= 6 ? ((price - bars[last - 6]) / bars[last - 6]) * 100 : 0,
    volRatio,
    bbUpper: m20 + 2 * s20,
    bbLower: m20 - 2 * s20,
    bbWidth: m20 > 0 ? (4 * s20) / m20 * 100 : 0,
    stochK: stoch.k,
    stochD: stoch.d,
    prevStochK: stoch.prevK,
    prevStochD: stoch.prevD,
    macdLine: mc.line,
    macdSignal: mc.signal,
    prevMacdLine: mc.prevLine,
    prevMacdSignal: mc.prevSignal,
    atr14: atr(bars, 14),
    obvSlope: obvSlope(bars, volumes ?? [], 12),
    momentum10: last >= 10 ? ((price - bars[last - 10]) / bars[last - 10]) * 100 : 0,
    rsi7: rsi(bars, 7),
    williamsR: williamsR(bars, 14),
    cci20: cci(bars, 20),
    roc10: rateOfChange(bars, 10),
    keltnerUpper: ema(bars, 20) + 1.5 * atr(bars, 14),
    keltnerLower: ema(bars, 20) - 1.5 * atr(bars, 14),
    donchianHigh: Math.max(...bars.slice(-20)),
    donchianLow: Math.min(...bars.slice(-20)),
    donchianMid: (Math.max(...bars.slice(-20)) + Math.min(...bars.slice(-20))) / 2,
    vwapDev: m20 > 0 ? ((price - m20) / m20) * 100 : 0,
    adxProxy: adxProxy(bars, 14),
    ema5: ema(bars, 5),
    ema13: ema(bars, 13),
    prevEma5: ema(previous, 5),
    prevEma13: ema(previous, 13),
    rsi21: rsi(bars, 21),
    macdHist: mc.line - mc.signal,
    prevMacdHist: mc.prevLine - mc.prevSignal,
  };
}

/** Signals calibrated on 1m closes (BTC). */
function evalMinuteSignal(signal: string, input: SignalInputs): number {
  const { price, prevPrice, fast, slow, trend, prevFast, prevSlow, mean20, rsi14, high20, low20, momentum3, momentum6 } = input;
  const { bbUpper, bbLower, bbWidth, stochK, stochD, prevStochK, prevStochD, macdLine, macdSignal, prevMacdLine, prevMacdSignal, atr14, obvSlope, momentum10, rsi7 } = input;
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

    // --- Bollinger Band signals ---
    case "BB_SQUEEZE_LONG":
      return bbWidth < 0.35 && price > mean20 && stochK > stochD && momentum3 > 0.02 && rsi14 >= 50
        ? scoreClamp(71 + (0.35 - bbWidth) * 60 + momentum3 * 20) : 0;
    case "BB_SQUEEZE_SHORT":
      return bbWidth < 0.35 && price < mean20 && stochK < stochD && momentum3 < -0.02 && rsi14 <= 50
        ? scoreClamp(71 + (0.35 - bbWidth) * 60 + Math.abs(momentum3) * 20) : 0;
    case "BB_BOUNCE_LONG":
      return price <= bbLower * 1.001 && rsi14 <= 38 && prevPrice < price && momentum3 > -0.06
        ? scoreClamp(70 + (bbLower / price - 0.999) * 8000 + (38 - rsi14) * 0.8) : 0;
    case "BB_BOUNCE_SHORT":
      return price >= bbUpper * 0.999 && rsi14 >= 62 && prevPrice > price && momentum3 < 0.06
        ? scoreClamp(70 + (price / bbUpper - 0.999) * 8000 + (rsi14 - 62) * 0.8) : 0;
    case "BB_WALK_LONG":
      return price > bbUpper * 0.998 && fast > slow && momentum6 > 0.06 && rsi14 >= 55 && rsi14 <= 78
        ? scoreClamp(72 + momentum6 * 35 + (rsi14 - 55) * 0.3) : 0;
    case "BB_WALK_SHORT":
      return price < bbLower * 1.002 && fast < slow && momentum6 < -0.06 && rsi14 >= 22 && rsi14 <= 45
        ? scoreClamp(72 + Math.abs(momentum6) * 35 + (45 - rsi14) * 0.3) : 0;

    // --- Stochastic signals ---
    case "STOCH_CROSS_LONG":
      return prevStochK <= prevStochD && stochK > stochD && stochK < 30 && price > fast && momentum3 > 0
        ? scoreClamp(70 + (30 - stochK) * 0.5 + momentum3 * 15) : 0;
    case "STOCH_CROSS_SHORT":
      return prevStochK >= prevStochD && stochK < stochD && stochK > 70 && price < fast && momentum3 < 0
        ? scoreClamp(70 + (stochK - 70) * 0.5 + Math.abs(momentum3) * 15) : 0;
    case "STOCH_DIVERGE_LONG":
      return stochK < 25 && rsi14 > 40 && price >= prevPrice && momentum3 > -0.02 && fast >= slow
        ? scoreClamp(69 + (25 - stochK) * 0.6 + (rsi14 - 40) * 0.3) : 0;
    case "STOCH_DIVERGE_SHORT":
      return stochK > 75 && rsi14 < 60 && price <= prevPrice && momentum3 < 0.02 && fast <= slow
        ? scoreClamp(69 + (stochK - 75) * 0.6 + (60 - rsi14) * 0.3) : 0;

    // --- MACD signals ---
    case "MACD_CROSS_LONG":
      return prevMacdLine <= prevMacdSignal && macdLine > macdSignal && price > slow && rsi14 >= 48
        ? scoreClamp(71 + Math.min(10, (macdLine - macdSignal) / (atr14 || 1) * 500) + (rsi14 - 48) * 0.25) : 0;
    case "MACD_CROSS_SHORT":
      return prevMacdLine >= prevMacdSignal && macdLine < macdSignal && price < slow && rsi14 <= 52
        ? scoreClamp(71 + Math.min(10, (macdSignal - macdLine) / (atr14 || 1) * 500) + (52 - rsi14) * 0.25) : 0;
    case "MACD_DIVERGE_LONG":
      return macdLine > prevMacdLine && price < prevPrice && rsi14 <= 42 && stochK < 35
        ? scoreClamp(70 + (42 - rsi14) * 0.5 + (35 - stochK) * 0.3) : 0;
    case "MACD_DIVERGE_SHORT":
      return macdLine < prevMacdLine && price > prevPrice && rsi14 >= 58 && stochK > 65
        ? scoreClamp(70 + (rsi14 - 58) * 0.5 + (stochK - 65) * 0.3) : 0;

    // --- OBV / Volume signals ---
    case "OBV_BREAKOUT_LONG":
      return obvSlope > 0.3 && price > mean20 && momentum3 > 0.03 && fast > slow
        ? scoreClamp(70 + obvSlope * 15 + momentum3 * 20) : 0;
    case "OBV_BREAKOUT_SHORT":
      return obvSlope < -0.3 && price < mean20 && momentum3 < -0.03 && fast < slow
        ? scoreClamp(70 + Math.abs(obvSlope) * 15 + Math.abs(momentum3) * 20) : 0;

    // --- Multi-indicator confluence signals ---
    case "TRIPLE_BULL":
      return rsi14 >= 52 && rsi14 <= 70 && stochK > stochD && macdLine > macdSignal && fast > slow && momentum6 > 0.04
        ? scoreClamp(74 + momentum6 * 30 + (rsi14 - 52) * 0.2) : 0;
    case "TRIPLE_BEAR":
      return rsi14 >= 30 && rsi14 <= 48 && stochK < stochD && macdLine < macdSignal && fast < slow && momentum6 < -0.04
        ? scoreClamp(74 + Math.abs(momentum6) * 30 + (48 - rsi14) * 0.2) : 0;
    case "MEAN_REVERT_DEEP_LONG":
      return rsi7 <= 22 && stochK < 15 && price <= bbLower * 1.002 && price >= prevPrice
        ? scoreClamp(72 + (22 - rsi7) * 0.8 + (15 - stochK) * 0.4) : 0;
    case "MEAN_REVERT_DEEP_SHORT":
      return rsi7 >= 78 && stochK > 85 && price >= bbUpper * 0.998 && price <= prevPrice
        ? scoreClamp(72 + (rsi7 - 78) * 0.8 + (stochK - 85) * 0.4) : 0;

    // --- ATR / Volatility signals ---
    case "ATR_EXPANSION_LONG":
      return atr14 > 0 && bbWidth > 0.5 && momentum3 > 0.06 && fast > slow && rsi14 >= 54
        ? scoreClamp(71 + momentum3 * 30 + (bbWidth - 0.5) * 20) : 0;
    case "ATR_EXPANSION_SHORT":
      return atr14 > 0 && bbWidth > 0.5 && momentum3 < -0.06 && fast < slow && rsi14 <= 46
        ? scoreClamp(71 + Math.abs(momentum3) * 30 + (bbWidth - 0.5) * 20) : 0;

    // --- Momentum divergence signals ---
    case "MOM_ACCEL_LONG":
      return momentum3 > 0.05 && momentum10 > 0.08 && momentum3 > momentum6 && rsi14 >= 52 && rsi14 <= 72
        ? scoreClamp(72 + momentum3 * 25 + (momentum3 - momentum6) * 40) : 0;
    case "MOM_ACCEL_SHORT":
      return momentum3 < -0.05 && momentum10 < -0.08 && momentum3 < momentum6 && rsi14 >= 28 && rsi14 <= 48
        ? scoreClamp(72 + Math.abs(momentum3) * 25 + Math.abs(momentum3 - momentum6) * 40) : 0;

    // ====== NEW STRATEGIES (61-110) ======

    // --- Williams %R signals ---
    case "WILLIAMS_OVERSOLD_LONG":
      return input.williamsR < -80 && price >= prevPrice && input.rsi14 <= 38 && input.momentum3 > -0.04
        ? scoreClamp(70 + (-80 - input.williamsR) * 0.8 + (38 - input.rsi14) * 0.4) : 0;
    case "WILLIAMS_OVERSOLD_SHORT":
      return input.williamsR > -20 && price <= prevPrice && input.rsi14 >= 62 && input.momentum3 < 0.04
        ? scoreClamp(70 + (input.williamsR + 20) * 0.8 + (input.rsi14 - 62) * 0.4) : 0;
    case "WILLIAMS_MIDLINE_LONG":
      return input.williamsR > -50 && input.williamsR < -30 && fast > slow && input.momentum3 > 0.03
        ? scoreClamp(69 + input.momentum3 * 30 + (-30 - input.williamsR) * 0.3) : 0;
    case "WILLIAMS_MIDLINE_SHORT":
      return input.williamsR < -50 && input.williamsR > -70 && fast < slow && input.momentum3 < -0.03
        ? scoreClamp(69 + Math.abs(input.momentum3) * 30 + (input.williamsR + 70) * 0.3) : 0;

    // --- CCI (Commodity Channel Index) signals ---
    case "CCI_OVERSOLD_LONG":
      return input.cci20 < -100 && price >= prevPrice && stochK < 30 && fast >= slow * 0.999
        ? scoreClamp(71 + Math.min(15, (-100 - input.cci20) * 0.08) + (30 - stochK) * 0.2) : 0;
    case "CCI_OVERBOUGHT_SHORT":
      return input.cci20 > 100 && price <= prevPrice && stochK > 70 && fast <= slow * 1.001
        ? scoreClamp(71 + Math.min(15, (input.cci20 - 100) * 0.08) + (stochK - 70) * 0.2) : 0;
    case "CCI_ZERO_CROSS_LONG":
      return input.cci20 > 0 && input.cci20 < 50 && input.momentum3 > 0.02 && rsi14 >= 48 && fast > slow
        ? scoreClamp(69 + input.cci20 * 0.15 + input.momentum3 * 20) : 0;
    case "CCI_ZERO_CROSS_SHORT":
      return input.cci20 < 0 && input.cci20 > -50 && input.momentum3 < -0.02 && rsi14 <= 52 && fast < slow
        ? scoreClamp(69 + Math.abs(input.cci20) * 0.15 + Math.abs(input.momentum3) * 20) : 0;

    // --- Keltner Channel signals ---
    case "KELTNER_BREAKOUT_LONG":
      return price > input.keltnerUpper && fast > slow && rsi14 >= 54 && input.momentum3 > 0.04
        ? scoreClamp(72 + (price / input.keltnerUpper - 1) * 6000 + input.momentum3 * 20) : 0;
    case "KELTNER_BREAKOUT_SHORT":
      return price < input.keltnerLower && fast < slow && rsi14 <= 46 && input.momentum3 < -0.04
        ? scoreClamp(72 + (input.keltnerLower / price - 1) * 6000 + Math.abs(input.momentum3) * 20) : 0;
    case "KELTNER_BOUNCE_LONG":
      return price <= input.keltnerLower * 1.002 && price >= prevPrice && rsi14 <= 40 && stochK < 30
        ? scoreClamp(70 + (40 - rsi14) * 0.5 + (30 - stochK) * 0.3) : 0;
    case "KELTNER_BOUNCE_SHORT":
      return price >= input.keltnerUpper * 0.998 && price <= prevPrice && rsi14 >= 60 && stochK > 70
        ? scoreClamp(70 + (rsi14 - 60) * 0.5 + (stochK - 70) * 0.3) : 0;

    // --- Donchian Channel signals ---
    case "DONCHIAN_BREAK_LONG":
      return price >= input.donchianHigh * 0.999 && fast > slow && input.adxProxy > 20 && rsi14 >= 52
        ? scoreClamp(72 + (price / input.donchianHigh - 0.999) * 8000 + input.adxProxy * 0.15) : 0;
    case "DONCHIAN_BREAK_SHORT":
      return price <= input.donchianLow * 1.001 && fast < slow && input.adxProxy > 20 && rsi14 <= 48
        ? scoreClamp(72 + (input.donchianLow / price - 0.999) * 8000 + input.adxProxy * 0.15) : 0;
    case "DONCHIAN_MID_LONG":
      return price > input.donchianMid && prevPrice <= input.donchianMid && fast > slow && rsi14 >= 50
        ? scoreClamp(69 + (price / input.donchianMid - 1) * 6000 + (rsi14 - 50) * 0.3) : 0;
    case "DONCHIAN_MID_SHORT":
      return price < input.donchianMid && prevPrice >= input.donchianMid && fast < slow && rsi14 <= 50
        ? scoreClamp(69 + (input.donchianMid / price - 1) * 6000 + (50 - rsi14) * 0.3) : 0;

    // --- EMA Ribbon signals (5/13 fast cross) ---
    case "EMA_RIBBON_LONG":
      return input.prevEma5 <= input.prevEma13 && input.ema5 > input.ema13 && price > slow && rsi14 >= 50
        ? scoreClamp(71 + (input.ema5 / input.ema13 - 1) * 10000 + (rsi14 - 50) * 0.3) : 0;
    case "EMA_RIBBON_SHORT":
      return input.prevEma5 >= input.prevEma13 && input.ema5 < input.ema13 && price < slow && rsi14 <= 50
        ? scoreClamp(71 + (input.ema13 / input.ema5 - 1) * 10000 + (50 - rsi14) * 0.3) : 0;

    // --- VWAP Deviation signals ---
    case "VWAP_DEV_LONG":
      return input.vwapDev < -0.15 && price >= prevPrice && rsi14 <= 42 && stochK < 35
        ? scoreClamp(70 + Math.abs(input.vwapDev) * 20 + (42 - rsi14) * 0.3) : 0;
    case "VWAP_DEV_SHORT":
      return input.vwapDev > 0.15 && price <= prevPrice && rsi14 >= 58 && stochK > 65
        ? scoreClamp(70 + input.vwapDev * 20 + (rsi14 - 58) * 0.3) : 0;

    // --- ADX Trend Strength signals ---
    case "ADX_TREND_LONG":
      return input.adxProxy > 30 && fast > slow && slow > trend && input.momentum6 > 0.05 && rsi14 >= 52
        ? scoreClamp(73 + input.adxProxy * 0.2 + input.momentum6 * 25) : 0;
    case "ADX_TREND_SHORT":
      return input.adxProxy > 30 && fast < slow && slow < trend && input.momentum6 < -0.05 && rsi14 <= 48
        ? scoreClamp(73 + input.adxProxy * 0.2 + Math.abs(input.momentum6) * 25) : 0;

    // --- ROC (Rate of Change) signals ---
    case "ROC_REVERSAL_LONG":
      return input.roc10 < -0.5 && input.roc10 > -2 && price >= prevPrice && rsi14 <= 38 && stochK > prevStochK
        ? scoreClamp(70 + Math.abs(input.roc10) * 5 + (38 - rsi14) * 0.4) : 0;
    case "ROC_REVERSAL_SHORT":
      return input.roc10 > 0.5 && input.roc10 < 2 && price <= prevPrice && rsi14 >= 62 && stochK < prevStochK
        ? scoreClamp(70 + input.roc10 * 5 + (rsi14 - 62) * 0.4) : 0;
    case "ROC_MOMENTUM_LONG":
      return input.roc10 > 0.3 && momentum3 > 0.04 && fast > slow && rsi14 >= 52 && rsi14 <= 72
        ? scoreClamp(71 + input.roc10 * 8 + momentum3 * 20) : 0;
    case "ROC_MOMENTUM_SHORT":
      return input.roc10 < -0.3 && momentum3 < -0.04 && fast < slow && rsi14 >= 28 && rsi14 <= 48
        ? scoreClamp(71 + Math.abs(input.roc10) * 8 + Math.abs(momentum3) * 20) : 0;

    // --- MACD Histogram Divergence signals ---
    case "MACD_HIST_RISE_LONG":
      return input.macdHist > input.prevMacdHist && input.macdHist > 0 && price > slow && rsi14 >= 50
        ? scoreClamp(70 + (input.macdHist - input.prevMacdHist) / (atr14 || 1) * 300 + (rsi14 - 50) * 0.2) : 0;
    case "MACD_HIST_FALL_SHORT":
      return input.macdHist < input.prevMacdHist && input.macdHist < 0 && price < slow && rsi14 <= 50
        ? scoreClamp(70 + (input.prevMacdHist - input.macdHist) / (atr14 || 1) * 300 + (50 - rsi14) * 0.2) : 0;

    // --- RSI divergence with multiple timeframes ---
    case "RSI_MULTI_TF_LONG":
      return rsi7 <= 32 && rsi14 <= 40 && input.rsi21 <= 45 && price >= prevPrice && stochK < 30
        ? scoreClamp(72 + (32 - rsi7) * 0.5 + (40 - rsi14) * 0.3 + (30 - stochK) * 0.2) : 0;
    case "RSI_MULTI_TF_SHORT":
      return rsi7 >= 68 && rsi14 >= 60 && input.rsi21 >= 55 && price <= prevPrice && stochK > 70
        ? scoreClamp(72 + (rsi7 - 68) * 0.5 + (rsi14 - 60) * 0.3 + (stochK - 70) * 0.2) : 0;

    // --- Squeeze Momentum (BB inside Keltner) ---
    case "SQUEEZE_MOM_LONG":
      return bbUpper < input.keltnerUpper && bbLower > input.keltnerLower && momentum3 > 0.03 && fast > slow
        ? scoreClamp(73 + momentum3 * 30 + (rsi14 - 48) * 0.2) : 0;
    case "SQUEEZE_MOM_SHORT":
      return bbUpper < input.keltnerUpper && bbLower > input.keltnerLower && momentum3 < -0.03 && fast < slow
        ? scoreClamp(73 + Math.abs(momentum3) * 30 + (52 - rsi14) * 0.2) : 0;

    // --- Dual Stochastic (K/D cross with RSI filter) ---
    case "DUAL_STOCH_LONG":
      return prevStochK <= prevStochD && stochK > stochD && stochK < 40 && rsi14 >= 42 && rsi14 <= 58
        ? scoreClamp(70 + (40 - stochK) * 0.4 + (rsi14 - 42) * 0.3) : 0;
    case "DUAL_STOCH_SHORT":
      return prevStochK >= prevStochD && stochK < stochD && stochK > 60 && rsi14 >= 42 && rsi14 <= 58
        ? scoreClamp(70 + (stochK - 60) * 0.4 + (58 - rsi14) * 0.3) : 0;

    // --- Volume-Price Trend signals ---
    case "VOL_SPIKE_LONG":
      return input.volRatio > 1.8 && price > prevPrice && fast > slow && rsi14 >= 50 && rsi14 <= 68
        ? scoreClamp(71 + (input.volRatio - 1.8) * 10 + momentum3 * 15) : 0;
    case "VOL_SPIKE_SHORT":
      return input.volRatio > 1.8 && price < prevPrice && fast < slow && rsi14 >= 32 && rsi14 <= 50
        ? scoreClamp(71 + (input.volRatio - 1.8) * 10 + Math.abs(momentum3) * 15) : 0;

    // --- Ichimoku-lite (EMA cloud) signals ---
    case "CLOUD_BREAK_LONG":
      return price > Math.max(input.ema5, input.ema13) && prevPrice <= Math.max(input.prevEma5, input.prevEma13)
        && fast > slow && rsi14 >= 50
        ? scoreClamp(72 + momentum3 * 25 + (rsi14 - 50) * 0.2) : 0;
    case "CLOUD_BREAK_SHORT":
      return price < Math.min(input.ema5, input.ema13) && prevPrice >= Math.min(input.prevEma5, input.prevEma13)
        && fast < slow && rsi14 <= 50
        ? scoreClamp(72 + Math.abs(momentum3) * 25 + (50 - rsi14) * 0.2) : 0;

    // --- Pivot / Swing signals ---
    case "SWING_FAIL_LONG":
      return price > prevPrice && rsi14 >= 42 && rsi14 <= 55 && momentum3 > 0.02 && input.cci20 > -50 && input.cci20 < 50
        ? scoreClamp(69 + momentum3 * 25 + (rsi14 - 42) * 0.3) : 0;
    case "SWING_FAIL_SHORT":
      return price < prevPrice && rsi14 >= 45 && rsi14 <= 58 && momentum3 < -0.02 && input.cci20 > -50 && input.cci20 < 50
        ? scoreClamp(69 + Math.abs(momentum3) * 25 + (58 - rsi14) * 0.3) : 0;

    // --- Exhaustion / Climax signals ---
    case "EXHAUSTION_LONG":
      return momentum3 < -0.12 && rsi7 <= 25 && input.williamsR < -85 && price >= prevPrice
        ? scoreClamp(73 + (25 - rsi7) * 0.6 + Math.abs(momentum3) * 15) : 0;
    case "EXHAUSTION_SHORT":
      return momentum3 > 0.12 && rsi7 >= 75 && input.williamsR > -15 && price <= prevPrice
        ? scoreClamp(73 + (rsi7 - 75) * 0.6 + momentum3 * 15) : 0;

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
  const mrCats = ["Mean Reversion", "Bollinger MR", "Williams MR", "CCI MR", "Keltner MR", "VWAP MR", "RSI Multi", "Exhaustion"];
  const trendCats = ["Trend", "Volatility", "ADX Trend", "Donchian Trend", "ROC Trend", "Squeeze"];
  if (regime === "HIGH_VOL") return !mrCats.includes(category);
  if (regime === "RANGE") return !trendCats.includes(category);
  return true;
}

function passesEntryConfirmation(def: StratDef, input: SignalInputs, regime: string): boolean {
  if (!isCategoryAligned(def.category, regime)) return false;
  if (def.side === "LONG") {
    if (def.category === "VWAP") return input.price >= input.mean20 * 0.999 && input.rsi14 >= 42;
    if (def.category === "Mean Reversion") return input.rsi14 <= 42 && input.price >= input.prevPrice * 0.999;
    if (def.category === "Breakout") return input.momentum3 > 0.01 && input.price > input.fast;
    if (def.category === "Bollinger") return input.price > input.mean20 && input.momentum3 > -0.02;
    if (def.category === "Bollinger MR") return input.rsi14 <= 45 && input.stochK < 35;
    if (def.category === "Stochastic") return input.stochK > input.stochD && input.price >= input.prevPrice;
    if (def.category === "MACD") return input.macdLine > input.macdSignal && input.momentum3 > -0.01;
    if (def.category === "Volume") return input.obvSlope > 0.1 && input.momentum3 > 0;
    if (def.category === "Confluence") return input.rsi14 >= 45 && input.fast > input.slow;
    if (def.category === "Volatility") return input.bbWidth > 0.3 && input.momentum3 > 0.01;
    if (def.category === "Momentum Accel") return input.momentum3 > 0.02 && input.rsi14 >= 45;
    if (def.category === "Williams MR") return input.williamsR < -70 && input.price >= input.prevPrice;
    if (def.category === "Williams Trend") return input.williamsR > -55 && input.momentum3 > 0.02;
    if (def.category === "CCI MR") return input.cci20 < -80 && input.price >= input.prevPrice;
    if (def.category === "CCI Trend") return input.cci20 > -20 && input.momentum3 > 0.01;
    if (def.category === "Keltner") return input.momentum3 > 0.02 && input.rsi14 >= 48;
    if (def.category === "Keltner MR") return input.rsi14 <= 44 && input.price >= input.prevPrice;
    if (def.category === "Donchian Trend") return input.adxProxy > 15 && input.momentum3 > 0.01;
    if (def.category === "Donchian Mid") return input.price > input.donchianMid && input.rsi14 >= 48;
    if (def.category === "EMA Ribbon") return input.ema5 > input.ema13 && input.rsi14 >= 48;
    if (def.category === "VWAP MR") return input.vwapDev < -0.08 && input.price >= input.prevPrice;
    if (def.category === "ADX Trend") return input.adxProxy > 25 && input.fast > input.slow;
    if (def.category === "ROC MR") return input.roc10 < -0.3 && input.price >= input.prevPrice;
    if (def.category === "ROC Trend") return input.roc10 > 0.1 && input.fast > input.slow;
    if (def.category === "MACD Hist") return input.macdHist > input.prevMacdHist && input.rsi14 >= 46;
    if (def.category === "RSI Multi") return input.rsi7 <= 38 && input.rsi14 <= 44;
    if (def.category === "Squeeze") return input.bbWidth < 0.4 && input.momentum3 > 0.01;
    if (def.category === "Dual Stoch") return input.stochK > input.stochD && input.rsi14 >= 42;
    if (def.category === "Vol Spike") return input.volRatio > 1.5 && input.price > input.prevPrice;
    if (def.category === "Cloud") return input.price > Math.max(input.ema5, input.ema13) && input.rsi14 >= 48;
    if (def.category === "Swing") return input.momentum3 > 0.01 && input.cci20 > -60;
    if (def.category === "Exhaustion") return input.rsi7 <= 30 && input.price >= input.prevPrice;
    return input.price >= input.fast && input.momentum3 > 0;
  }
  if (def.category === "VWAP") return input.price <= input.mean20 * 1.001 && input.rsi14 <= 58;
  if (def.category === "Mean Reversion") return input.rsi14 >= 58 && input.price <= input.prevPrice * 1.001;
  if (def.category === "Breakout") return input.momentum3 < -0.01 && input.price < input.fast;
  if (def.category === "Bollinger") return input.price < input.mean20 && input.momentum3 < 0.02;
  if (def.category === "Bollinger MR") return input.rsi14 >= 55 && input.stochK > 65;
  if (def.category === "Stochastic") return input.stochK < input.stochD && input.price <= input.prevPrice;
  if (def.category === "MACD") return input.macdLine < input.macdSignal && input.momentum3 < 0.01;
  if (def.category === "Volume") return input.obvSlope < -0.1 && input.momentum3 < 0;
  if (def.category === "Confluence") return input.rsi14 <= 55 && input.fast < input.slow;
  if (def.category === "Volatility") return input.bbWidth > 0.3 && input.momentum3 < -0.01;
  if (def.category === "Momentum Accel") return input.momentum3 < -0.02 && input.rsi14 <= 55;
  if (def.category === "Williams MR") return input.williamsR > -30 && input.price <= input.prevPrice;
  if (def.category === "Williams Trend") return input.williamsR < -45 && input.momentum3 < -0.02;
  if (def.category === "CCI MR") return input.cci20 > 80 && input.price <= input.prevPrice;
  if (def.category === "CCI Trend") return input.cci20 < 20 && input.momentum3 < -0.01;
  if (def.category === "Keltner") return input.momentum3 < -0.02 && input.rsi14 <= 52;
  if (def.category === "Keltner MR") return input.rsi14 >= 56 && input.price <= input.prevPrice;
  if (def.category === "Donchian Trend") return input.adxProxy > 15 && input.momentum3 < -0.01;
  if (def.category === "Donchian Mid") return input.price < input.donchianMid && input.rsi14 <= 52;
  if (def.category === "EMA Ribbon") return input.ema5 < input.ema13 && input.rsi14 <= 52;
  if (def.category === "VWAP MR") return input.vwapDev > 0.08 && input.price <= input.prevPrice;
  if (def.category === "ADX Trend") return input.adxProxy > 25 && input.fast < input.slow;
  if (def.category === "ROC MR") return input.roc10 > 0.3 && input.price <= input.prevPrice;
  if (def.category === "ROC Trend") return input.roc10 < -0.1 && input.fast < input.slow;
  if (def.category === "MACD Hist") return input.macdHist < input.prevMacdHist && input.rsi14 <= 54;
  if (def.category === "RSI Multi") return input.rsi7 >= 62 && input.rsi14 >= 56;
  if (def.category === "Squeeze") return input.bbWidth < 0.4 && input.momentum3 < -0.01;
  if (def.category === "Dual Stoch") return input.stochK < input.stochD && input.rsi14 <= 58;
  if (def.category === "Vol Spike") return input.volRatio > 1.5 && input.price < input.prevPrice;
  if (def.category === "Cloud") return input.price < Math.min(input.ema5, input.ema13) && input.rsi14 <= 52;
  if (def.category === "Swing") return input.momentum3 < -0.01 && input.cci20 < 60;
  if (def.category === "Exhaustion") return input.rsi7 >= 70 && input.price <= input.prevPrice;
  return input.price <= input.fast && input.momentum3 < 0;
}

function calcPnl(side: Side, entry: number, exit: number, qty: number): number {
  return (exit - entry) * qty * (side === "LONG" ? 1 : -1);
}

function resolveExit(pos: InternalPosition, def: StratDef, price: number, now: number): { reason: string; exitPrice: number } | null {
  const returnPct = pos.notional > 0 ? (calcPnl(pos.side, pos.entryPrice, price, pos.quantity) / pos.notional) * 100 : 0;
  const netReturnPct = returnPct - FEE_BREAKEVEN_PCT;
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

  // All early exits require net-positive return after fees — prevents "winning" exits
  // that are actually losses once fees are deducted (floored to -$2 by the min PnL rule).
  if (netReturnPct <= 0) return null;

  if (progress >= GRIND_EXIT_PROGRESS && returnPct >= Math.max(LATE_EXIT_MIN_GAIN, def.tpPct * GRIND_EXIT_SHARE)) return { reason: "PROFIT_LOCK", exitPrice: price };
  if (progress >= PROFIT_LOCK_PROGRESS && returnPct >= lockThreshold) return { reason: "PROFIT_LOCK", exitPrice: price };
  if (pos.peakReturnPct >= Math.max(TRAIL_ACTIVATION_PCT, def.tpPct * 0.50) && netReturnPct > 0 && returnPct <= pos.peakReturnPct * (1 - TRAIL_GIVEBACK_SHARE)) return { reason: "TRAIL_STOP", exitPrice: price };
  if (progress >= LATE_EXIT_PROGRESS && returnPct >= LATE_EXIT_MIN_GAIN) return { reason: "LATE_EXIT", exitPrice: price };
  return null;
}

function currentVolRatio(engine: EngineRef): number {
  const vb = engine.volBars.slice(-VOL_HISTORY);
  if (vb.length < 4) return 1;
  const lastVol = vb[vb.length - 1] ?? 0;
  const avgVol = vb.slice(0, -1).reduce((a, b) => a + b, 0) / Math.max(1, vb.length - 1);
  return avgVol > 0 ? lastVol / avgVol : 1;
}

function targetNotionalFor(engine: EngineRef): number {
  const open = engine.positions.size;
  const reserved = open * 2.5;
  const equity =
    engine.balance +
    [...engine.positions.values()].reduce((s, p) => s + p.notional + p.unrealizedPnl, 0);
  const volRatio = currentVolRatio(engine);
  const volSizeMultiplier = volRatio >= 1.8 ? 0.7 : volRatio >= 1.45 ? 0.85 : 1;
  const slice = Math.max(MIN_NOTIONAL_USD, Math.min(MAX_NOTIONAL_USD, (equity - reserved) * 0.18 * volSizeMultiplier));
  return Math.min(slice, Math.max(0, engine.balance - 2));
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
    lastRegime: "WARMING",
    lastLocalSaveAt: 0,
    lastServerSaveAt: 0,
    serverSyncConfigured: false,
    sessionPeakEquity: INITIAL_BALANCE,
    lastVolRatio: 1,
    lastRsi14: 50,
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
  marketRegime: "WARMING",
  avgWinUsd: null,
  avgLossUsd: null,
  profitFactor: null,
  expectancyPerTradeUsd: null,
  persistence: { lastLocalSaveAt: null, lastServerSaveAt: null, serverSyncConfigured: false },
  maxDrawdownFromPeakPct: 0,
  sessionPeakEquity: INITIAL_BALANCE,
  volRatio: 1,
  rsi14: 50,
  winStreak: 0,
  lossStreak: 0,
  exitReasonCounts: {},
  bestTradeUsd: null,
  worstTradeUsd: null,
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
    engine.lastLocalSaveAt = Date.now();
  } catch {
    /* ignore */
  }
}

function loadLs(): DbPayload | null {
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (raw) return normalizeDbPayload(JSON.parse(raw) as Partial<DbPayload>);

    for (const legacyKey of LS_LEGACY_KEYS) {
      const legacy = localStorage.getItem(legacyKey);
      if (legacy) {
        const parsed = normalizeDbPayload(JSON.parse(legacy) as Partial<DbPayload>);
        if (parsed) {
          localStorage.setItem(LS_KEY, legacy);
          localStorage.removeItem(legacyKey);
          return parsed;
        }
      }
    }
    return null;
  } catch {
    return null;
  }
}

function normalizeDbPayload(raw: Partial<DbPayload> | null | undefined): DbPayload | null {
  if (!raw) return null;
  return {
    balance: typeof raw.balance === "number" && Number.isFinite(raw.balance) ? raw.balance : INITIAL_BALANCE,
    totalWins: Math.max(0, Math.trunc(Number(raw.totalWins) || 0)),
    totalLosses: Math.max(0, Math.trunc(Number(raw.totalLosses) || 0)),
    totalPnl: typeof raw.totalPnl === "number" && Number.isFinite(raw.totalPnl) ? raw.totalPnl : 0,
    tradeSeq: Math.max(0, Math.trunc(Number(raw.tradeSeq) || 0)),
    positions: Array.isArray(raw.positions) ? raw.positions : [],
    trades: Array.isArray(raw.trades) ? raw.trades.slice(0, MAX_TRADES) : [],
    strategies: Array.isArray(raw.strategies) ? raw.strategies : [],
  };
}

function compareSavedStates(a: DbPayload, b: DbPayload): number {
  const tradeSeqDiff = (a.tradeSeq ?? 0) - (b.tradeSeq ?? 0);
  if (tradeSeqDiff !== 0) return tradeSeqDiff;
  const tradeCountDiff = (a.trades?.length ?? 0) - (b.trades?.length ?? 0);
  if (tradeCountDiff !== 0) return tradeCountDiff;
  const positionCountDiff = (a.positions?.length ?? 0) - (b.positions?.length ?? 0);
  if (positionCountDiff !== 0) return positionCountDiff;
  return 0;
}

function applySaved(engine: EngineRef, saved: DbPayload): void {
  const persistedBalance = typeof saved.balance === "number" && saved.balance >= 0 ? saved.balance : INITIAL_BALANCE;
  engine.balance = persistedBalance;
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
  const unrealizedSum = [...engine.positions.values()].reduce((a, p) => a + p.unrealizedPnl, 0);
  const openNotional = [...engine.positions.values()].reduce((a, p) => a + p.notional, 0);
  const eq = engine.balance + openNotional + unrealizedSum;
  engine.sessionPeakEquity = Math.max(INITIAL_BALANCE, engine.sessionPeakEquity, eq);
}

async function saveDbState(engine: EngineRef): Promise<void> {
  try {
    const res = await fetch("/api/btc/spot-state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildPayload(engine)),
    });
    const data = (await res.json()) as { ok?: boolean; skipped?: boolean };
    if (res.ok && data.ok && !data.skipped) engine.lastServerSaveAt = Date.now();
  } catch {
    // non-critical persistence path
  }
}

async function loadState(engine: EngineRef): Promise<boolean> {
  const local = loadLs();
  let dbState: DbPayload | null = null;
  try {
    const response = await fetch("/api/btc/spot-state");
    if (response.ok) {
      const data = (await response.json()) as {
        ok?: boolean;
        found?: boolean;
        disabled?: boolean;
        skipped?: boolean;
        state?: Partial<DbPayload>;
      };
      engine.serverSyncConfigured = !data.disabled && !data.skipped;
      if (data.ok && data.found && data.state) dbState = normalizeDbPayload(data.state);
    }
  } catch {
    engine.serverSyncConfigured = false;
  }

  const saved = local && dbState
    ? (compareSavedStates(local, dbState) >= 0 ? local : dbState)
    : (local ?? dbState);
  if (!saved) return false;
  applySaved(engine, saved);
  return true;
}

function createEngineHydratedFromLs(): EngineRef {
  const engine = initEngine();
  const snap = loadLs();
  if (snap) applySaved(engine, snap);
  return engine;
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
  const rawNet = gross - feesUsd;
  let finalNet = rawNet;
  if (Math.abs(rawNet) < MIN_ABS_NET_PNL_USD) {
    const sign =
      rawNet > 0 ? 1
        : rawNet < 0 ? -1
          : gross > 0 ? 1
            : gross < 0 ? -1
              : /SL|TRAIL/i.test(reason) ? -1
                : 1;
    finalNet = sign * MIN_ABS_NET_PNL_USD;
  }
  const trade: InternalTrade = {
    id: pos.id,
    strategyId: pos.strategyId,
    strategyName: pos.strategyName,
    symbol: "BTC",
    side: pos.side,
    quantity: pos.quantity,
    entryPrice: pos.entryPrice,
    exitPrice,
    netPnl: finalNet,
    returnPct: pos.notional > 0 ? (finalNet / pos.notional) * 100 : 0,
    entryTime: pos.entryTime,
    exitTime: now,
    exitReason: reason,
    holdSeconds: Math.round((now - pos.entryTime) / 1000),
    feesUsd,
  };
  engine.trades.unshift(trade);
  if (engine.trades.length > MAX_TRADES) engine.trades.length = MAX_TRADES;
  engine.balance += pos.notional + finalNet;
  engine.totalRealizedPnl += finalNet;
  strategy.totalTrades++;
  if (finalNet >= 0) {
    strategy.wins++;
    engine.totalWins++;
    strategy.consecutiveLosses = 0;
  } else {
    strategy.losses++;
    engine.totalLosses++;
    strategy.consecutiveLosses++;
  }
  strategy.totalPnl += finalNet;
  strategy.winRate = strategy.totalTrades > 0 ? (strategy.wins / strategy.totalTrades) * 100 : 0;
  strategy.cooldownUntil = now + cooldownMsFor(strategy, finalNet >= 0);
  strategy.status = "COOLING";
  strategy.position = null;
  engine.positions.delete(pos.id);
}

export default function useBTCSpotScalperEngine() {
  const engineRef = useRef<EngineRef | null>(null);
  if (engineRef.current === null) {
    engineRef.current = createEngineHydratedFromLs();
  }
  const engineR = engineRef as MutableRefObject<EngineRef>;
  const loadedRef = useRef(false);
  const entriesPausedRef = useRef(false);
  const [entriesPaused, setEntriesPausedState] = useState(false);
  const [quote, setQuote] = useState<BTCSpotQuote>({
    symbol: "BTC",
    ltp: 0,
    changePct24h: 0,
    signalScore: 0,
    hasPosition: false,
    sparkline: [],
    volRatio: 1,
    rsi14: 50,
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
    const engine = engineR.current;
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
      volRatio: engine.lastVolRatio,
      rsi14: engine.lastRsi14,
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
      engine.trades.slice(0, 500).map((t) => ({
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
    const drawdownPct = ((INITIAL_BALANCE - equity) / INITIAL_BALANCE) * 100;
    const lockedByDrawdown = drawdownPct >= MAX_DRAWDOWN_LOCK_PCT;
    let winGross = 0;
    let lossGross = 0;
    for (const t of engine.trades) {
      if (t.netPnl > 0) winGross += t.netPnl;
      else if (t.netPnl < 0) lossGross += Math.abs(t.netPnl);
    }
    const avgWinUsd = engine.totalWins > 0 ? winGross / engine.totalWins : null;
    const avgLossUsd = engine.totalLosses > 0 ? lossGross / engine.totalLosses : null;
    const profitFactor = lossGross > 0 ? winGross / lossGross : null;
    const expectancyPerTradeUsd = tw > 0 ? engine.totalRealizedPnl / tw : null;

    let winStreak = 0;
    let lossStreak = 0;
    for (const t of engine.trades) {
      if (t.netPnl >= 0) {
        if (lossStreak > 0) break;
        winStreak++;
      } else {
        if (winStreak > 0) break;
        lossStreak++;
      }
    }

    const exitReasonCounts: Record<string, number> = {};
    let bestTradeUsd: number | null = null;
    let worstTradeUsd: number | null = null;
    for (const t of engine.trades) {
      const k = t.exitReason || "—";
      exitReasonCounts[k] = (exitReasonCounts[k] ?? 0) + 1;
      if (bestTradeUsd === null || t.netPnl > bestTradeUsd) bestTradeUsd = t.netPnl;
      if (worstTradeUsd === null || t.netPnl < worstTradeUsd) worstTradeUsd = t.netPnl;
    }

    const peakEq = engine.sessionPeakEquity > 0 ? engine.sessionPeakEquity : INITIAL_BALANCE;
    const maxDrawdownFromPeakPct = peakEq > 0 ? Math.max(0, ((peakEq - equity) / peakEq) * 100) : 0;

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
      diagnostics:
        engine.lastError ||
        (lockedByDrawdown
          ? `Risk lock active: drawdown ${drawdownPct.toFixed(1)}% exceeds ${MAX_DRAWDOWN_LOCK_PCT}% (manages open trades, pauses new entries).`
          : engine.lastPrice > 0
            ? "Delta Exchange 1m candles (REST)."
            : "Waiting for candles."),
      feeModelNote: EMPTY_STATS.feeModelNote,
      marketRegime: engine.lastRegime,
      avgWinUsd,
      avgLossUsd,
      profitFactor,
      expectancyPerTradeUsd,
      persistence: {
        lastLocalSaveAt: engine.lastLocalSaveAt > 0 ? engine.lastLocalSaveAt : null,
        lastServerSaveAt: engine.lastServerSaveAt > 0 ? engine.lastServerSaveAt : null,
        serverSyncConfigured: engine.serverSyncConfigured,
      },
      maxDrawdownFromPeakPct,
      sessionPeakEquity: engine.sessionPeakEquity,
      volRatio: engine.lastVolRatio,
      rsi14: engine.lastRsi14,
      winStreak,
      lossStreak,
      exitReasonCounts,
      bestTradeUsd,
      worstTradeUsd,
    });
  }, []);

  const setEntriesPaused = useCallback((next: boolean) => {
    entriesPausedRef.current = next;
    setEntriesPausedState(next);
    try {
      localStorage.setItem(LS_PAUSE_ENTRIES, next ? "1" : "0");
    } catch {
      /* ignore */
    }
  }, []);

  const processKlines = useCallback(
    (closes: number[], volumes: number[], livePrice: number, changePct24h: number, err: string) => {
      if (!loadedRef.current) return;
      const engine = engineR.current;
      const tickPrice = livePrice > 0 ? livePrice : 0;
      if (closes.length >= MIN_BARS) {
        const bars = closes.slice(-MAX_BARS);
        if (tickPrice > 0 && (bars.length === 0 || bars[bars.length - 1] !== tickPrice)) {
          bars.push(tickPrice);
        }
        if (bars.length > MAX_BARS) bars.splice(0, bars.length - MAX_BARS);
        engine.bars = bars;
        engine.lastPrice = tickPrice > 0 ? tickPrice : (bars[bars.length - 1] ?? 0);
      } else if (err) {
        engine.lastError = err;
        pushDisplay();
        return;
      }
      engine.changePct24h = changePct24h;
      if (tickPrice > 0) engine.lastPrice = tickPrice;
      else if (closes.length) engine.lastPrice = closes[closes.length - 1] ?? engine.lastPrice;
      if (err) engine.lastError = err;
      else engine.lastError = "";

      const vb = volumes.slice(-VOL_HISTORY);
      engine.volBars = vb;

      const now = Date.now();
      const bars = engine.bars;
      if (bars.length < MIN_BARS) {
        engine.lastRegime = "WARMING";
        pushDisplay();
        saveLs(engine);
        return;
      }

      const lastVol = vb.length ? vb[vb.length - 1] : 0;
      const avgVol = vb.length >= 3 ? vb.slice(0, -1).reduce((a, b) => a + b, 0) / (vb.length - 1) : 0;
      const volRatio = avgVol > 0 ? lastVol / avgVol : 1;
      const input = buildSignalInputs(bars, volRatio, vb);
      const regime = classifyRegime(input);
      engine.lastRegime = regime;
      engine.lastVolRatio = volRatio;
      engine.lastRsi14 = input.rsi14;
      const equityNow =
        engine.balance +
        [...engine.positions.values()].reduce((sum, pos) => sum + pos.notional + pos.unrealizedPnl, 0);
      engine.sessionPeakEquity = Math.max(engine.sessionPeakEquity, equityNow);
      const drawdownPct = ((INITIAL_BALANCE - equityNow) / INITIAL_BALANCE) * 100;
      const allowNewEntries = drawdownPct < MAX_DRAWDOWN_LOCK_PCT;

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
        if (confirmed && allowNewEntries && !entriesPausedRef.current && engine.positions.size < MAX_OPEN_POSITIONS && engine.lastPrice > 0) {
          openPosition(engine, strategy, engine.lastPrice, now);
        }
      }

      if (!allowNewEntries && !engine.lastError) {
        engine.lastError = `Risk lock: drawdown ${drawdownPct.toFixed(1)}% (>${MAX_DRAWDOWN_LOCK_PCT}%).`;
      }

      pushDisplay();
      saveLs(engine);
      if (loadedRef.current) void saveDbState(engine);
    },
    [pushDisplay],
  );

  const reset = useCallback(() => {
    const sync = engineR.current.serverSyncConfigured;
    engineR.current = initEngine();
    engineR.current.serverSyncConfigured = sync;
    loadedRef.current = true;
    saveLs(engineR.current);
    void saveDbState(engineR.current);
    pushDisplay();
  }, [pushDisplay]);

  useLayoutEffect(() => {
    try {
      const p = localStorage.getItem(LS_PAUSE_ENTRIES) === "1";
      entriesPausedRef.current = p;
      setEntriesPausedState(p);
    } catch {
      /* ignore */
    }
    pushDisplay();
  }, [pushDisplay]);

  useEffect(() => {
    void loadState(engineR.current).then(() => {
      loadedRef.current = true;
      pushDisplay();
    });
  }, [pushDisplay]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (!loadedRef.current) return;
      saveLs(engineR.current);
      void saveDbState(engineR.current);
    }, 30_000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const flushState = () => {
      if (!loadedRef.current) return;
      saveLs(engineR.current);
      const payload = JSON.stringify(buildPayload(engineR.current));
      const blob = new Blob([payload], { type: "application/json" });
      if (typeof navigator.sendBeacon === "function" && navigator.sendBeacon("/api/btc/spot-state", blob)) return;
      void fetch("/api/btc/spot-state", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: payload,
        keepalive: true,
      });
    };
    const onVisChange = () => {
      if (document.visibilityState === "hidden") flushState();
    };
    window.addEventListener("beforeunload", flushState);
    document.addEventListener("visibilitychange", onVisChange);
    return () => {
      window.removeEventListener("beforeunload", flushState);
      document.removeEventListener("visibilitychange", onVisChange);
    };
  }, []);

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
          livePrice?: number;
          changePct24h?: number;
          error?: string;
        };
        if (!res.ok || !data.ok || !data.closes?.length) {
          processKlines([], [], 0, 0, data.error || `HTTP ${res.status}`);
          return;
        }
        processKlines(data.closes, data.volumes ?? [], data.livePrice ?? 0, data.changePct24h ?? 0, "");
      } catch {
        processKlines([], [], 0, 0, "Failed to fetch BTC klines.");
      }
    };
    void tick();
    const id = setInterval(() => void tick(), POLL_MS);
    return () => {
      cancel = true;
      clearInterval(id);
    };
  }, [processKlines]);

  return { quote, positions, trades, strategies, stats, reset, initialBalance: INITIAL_BALANCE, entriesPaused, setEntriesPaused };
}
