"use client";

import { useCallback, useEffect, useRef, useState, useMemo, type MutableRefObject } from "react";
import { PRIMARY_QUOTE_SYMBOL, TRADING_SYMBOLS } from "@/lib/futuresMarketData";

/**
 * Multi-asset futures paper engine (Delta India REST via /api/btc/futures-klines?symbol=)
 * Each listed symbol runs the same strategy library on its own 1m candle stream.
 * Key features:
 * - 25x leverage support (4% margin requirement)
 * - Contract-based position sizing (notional/price)
 * - Liquidation price tracking
 * - Mark price-based PnL (not last price)
 * - Funding rate cost tracking
 * - Real commission structure (0.1% taker)
 */

// ========== FUTURES-SPECIFIC CONSTANTS ==========
const LEVERAGE = 25; // Default leverage
const MARGIN_PCT = 1 / LEVERAGE; // 4% margin requirement
const MAKER_FEE_PCT = 0.0005; // 0.05% maker
const TAKER_FEE_PCT = 0.001; // 0.10% taker (market orders)
const ROUND_TRIP_FEE_FRAC = TAKER_FEE_PCT * 2; // 0.2% round trip
const FEE_BREAKEVEN_PCT = ROUND_TRIP_FEE_FRAC * 100; // 0.2%

// Account settings
const INITIAL_BALANCE = 1000; // $1000 paper balance for futures (higher than spot)
const MIN_CONTRACTS = 1;
const MAX_CONTRACTS = 50;
const MAX_OPEN_POSITIONS = 48;
const CONTRACT_SIZE = 1; // 1 USD per contract on Delta

// Risk management
const MAX_DRAWDOWN_LOCK_PCT = 25; // Pause entries if drawdown > 25%
const MAX_LOSS_PER_TRADE_PCT = 2; // Max 2% loss per trade
/**
 * Stats only: mark is within this % of modeled liquidation (see calculateDistanceToLiquidation).
 */
const LIQUIDATION_RISK_DISPLAY_PCT = 1.0;

// Strategy parameters
// Paper desk default: strict HTF rules used to reject almost all MTF entries when 5m/15m was NEUTRAL.
const SIGNAL_THRESHOLD = 28;
const MAX_BARS = 120;
const MIN_BARS = 18;
/** Slightly slower poll: many symbols × REST calls per tick */
const POLL_MS = 4_000;
const SYMBOL_FETCH_CHUNK = 5;
const MAX_TRADES = 2_000;

// Exit management
const MIN_ABS_NET_PNL_USD = 2;
const PROFIT_LOCK_PROGRESS = 0.60;
const PROFIT_LOCK_SHARE = 0.60;
const LATE_EXIT_PROGRESS = 0.70;
const LATE_EXIT_MIN_GAIN = 0.22;
const BREAKEVEN_TRIGGER_FRAC = 0.40;
const TRAIL_ACTIVATION_PCT = 0.30;
const TRAIL_GIVEBACK_SHARE = 0.18;
const MTF_HOLD_BONUS = 1.3;
const MOMENTUM_HOLD_EXTEND = 1.25;

// Storage namespace defaults are built per-hook instance.

// ========== TYPES ==========
export type Side = "LONG" | "SHORT";
export type Status = "WARMING" | "READY" | "IN_POSITION" | "COOLING";
export type MarginMode = "isolated" | "cross";

/** Futures Position */
export interface BTCFuturesPosition {
  id: string;
  /** Perpetual contract symbol e.g. BTCUSD, ETHUSD */
  symbol: string;
  strategyId: number;
  strategyName: string;
  side: Side;
  entryPrice: number; // Entry price (mark price at open)
  markPrice: number; // Current mark price
  lastPrice: number; // Current last price
  contracts: number; // Number of contracts (USD notional)
  notional: number; // Position notional value in USD
  marginUsed: number; // Margin allocated to this position
  leverage: number; // Leverage used (default 25x)
  liquidationPrice: number; // Price at which position gets liquidated
  unrealizedPnl: number; // Unrealized PnL in USD
  unrealizedPnlPct: number; // Unrealized PnL as % of notional
  returnPct: number; // Return % relative to margin used
  tpPrice: number; // Take profit price
  slPrice: number; // Stop loss price
  fundingCosts: number; // Accumulated funding rate costs
  openedAt: string; // ISO timestamp
  holdMinutes: number;
  exitReason?: "TP" | "SL" | "TIME" | "TRAIL" | "BREAKEVEN" | "LIQUIDATION_RISK";
  marginMode: MarginMode;
  // Internal tracking
  adaptiveSl: number;
  breakevenMoved: boolean;
  initialMargin: number;
}

/** Futures Trade (closed position) */
export interface BTCFuturesTrade {
  id: string;
  symbol: string;
  strategyId: number;
  strategyName: string;
  side: Side;
  entryPrice: number;
  exitPrice: number;
  contracts: number;
  notional: number;
  marginUsed: number;
  realizedPnl: number; // Gross PnL
  fees: number; // Entry + exit fees
  netPnl: number; // Net after fees
  netPnlPct: number; // Net % return on margin
  fundingCosts: number; // Total funding paid/received
  openedAt: string;
  closedAt: string;
  exitReason: "TP" | "SL" | "TIME" | "TRAIL" | "BREAKEVEN" | "LIQUIDATION_RISK";
  liquidationPrice: number; // For reference
  liquidationDistancePct: number; // How close to liquidation at close
}

/** Strategy Definition */
interface StratDef {
  id: number;
  name: string;
  category: string;
  signalKey: string;
  slPct: number;
  tpPct: number;
  cooldownMin: number;
  holdMinutes: number;
  confluenceMin: number;
  requiresHtf?: boolean;
}

/** Funding Rate Info */
interface FundingInfo {
  rate: number; // Funding rate (e.g., 0.0001 = 0.01%)
  nextFundingTime: number; // Unix timestamp
  timestamp: number;
}

/** Engine State */
interface EngineState {
  balance: number;
  positions: BTCFuturesPosition[];
  trades: BTCFuturesTrade[];
  quote: {
    lastPrice: number;
    markPrice: number;
    indexPrice: number;
    changePct24h: number;
    fundingRate: number;
    nextFunding: number;
    timestamp: number;
  } | null;
  status: Status;
  warmingPct: number;
  disabledStrategies: number[];
  pauseEntries: boolean;
  lastTradeAt: number;
  dayStartBalance: number;
  dayStartDate: number;
}

/** Engine Reference */
export interface EngineRef {
  positions: BTCFuturesPosition[];
  trades: BTCFuturesTrade[];
  balance: number;
  equity: number;
  availableMargin: number;
  usedMargin: number;
  stats: BTCFuturesEngineStats;
  quote: EngineState["quote"];
  isReady: boolean;
  pauseEntries: boolean;
  disabledStrategies: number[];
  togglePause: () => void;
  resetPaperAccount: () => void;
  clearTradeHistory: () => void;
  setDisabledStrategies: (ids: number[]) => void;
  exportCSV: () => string;
  exportJSON: () => string;
}

/** Engine Stats */
export interface BTCFuturesEngineStats {
  totalTrades: number;
  winCount: number;
  lossCount: number;
  winRate: number;
  avgWin: number;
  avgLoss: number;
  profitFactor: number;
  realizedPnl: number;
  unrealizedPnl: number;
  totalFees: number;
  totalFundingCosts: number;
  netPnl: number;
  maxDrawdownPct: number;
  currentDrawdownPct: number;
  openPositions: number;
  maxPositions: number;
  longCount: number;
  shortCount: number;
  balance: number;
  equity: number;
  availableMargin: number;
  usedMargin: number;
  marginUtilization: number;
  liquidationRisk: number; // Positions near liquidation
  avgLeverage: number;
  dayStartBalance: number;
}

/** Strategy Status */
export interface BTCFuturesStrategyStatus {
  id: number;
  name: string;
  category: string;
  status: "OPEN" | "COOLING" | "AVAILABLE";
  disabled: boolean;
  openCount: number;
  lastTradeAt: number | null;
  score: number; // Performance score (0-100)
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
}

export type BTCFuturesEngineOptions = {
  /** Separate persistence namespace so multiple futures modules don't share state. */
  storageNamespace?: string;
  /** Optional strategy allow-list. If empty, full strategy library runs. */
  strategyIds?: number[];
  /** Optional symbol allow-list. If empty, full trading symbol list runs. */
  symbols?: readonly string[];
  /** Optional module-specific signal threshold override. */
  signalThreshold?: number;
};

// ========== SIGNAL INPUTS ==========
type SignalInputs = {
  price: number;
  markPrice: number;
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
  prevWilliamsR: number;
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
  htf5_fast: number;
  htf5_slow: number;
  htf5_rsi: number;
  htf5_momentum: number;
  htf5_trend: number;
  htf15_fast: number;
  htf15_slow: number;
  htf15_rsi: number;
  htf15_momentum: number;
  htf15_trend: number;
  htf5_macdHist: number;
  htf15_macdHist: number;
  htf5_bbWidth: number;
  htf15_bbWidth: number;
  htf5_adx: number;
  htf15_adx: number;
};

// ========== STRATEGY DEFINITIONS (130 strategies) ==========
const STRAT_DEFS: StratDef[] = [
  // 1-30: Original strategies (EMA, BB, RSI, Stoch, MACD, OBV, Confluence)
  { id: 1, name: "EMA_Cross_Long", category: "Trend", signalKey: "EMA_CROSS_LONG", slPct: 0.28, tpPct: 0.62, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 2, name: "EMA_Cross_Short", category: "Trend", signalKey: "EMA_CROSS_SHORT", slPct: 0.28, tpPct: 0.62, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 3, name: "BB_MeanRev_Long", category: "MeanRev", signalKey: "BB_MEANREV_LONG", slPct: 0.32, tpPct: 0.58, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 4, name: "BB_MeanRev_Short", category: "MeanRev", signalKey: "BB_MEANREV_SHORT", slPct: 0.32, tpPct: 0.58, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 5, name: "Momentum_Surge_Long", category: "Momentum", signalKey: "MOM_SURGE_LONG", slPct: 0.30, tpPct: 0.70, cooldownMin: 5, holdMinutes: 15, confluenceMin: 4 },
  { id: 6, name: "Momentum_Surge_Short", category: "Momentum", signalKey: "MOM_SURGE_SHORT", slPct: 0.30, tpPct: 0.70, cooldownMin: 5, holdMinutes: 15, confluenceMin: 4 },
  { id: 7, name: "RSI_Dip_Long", category: "RSI", signalKey: "RSI_DIP_LONG", slPct: 0.30, tpPct: 0.62, cooldownMin: 4, holdMinutes: 22, confluenceMin: 3 },
  { id: 8, name: "RSI_Spike_Short", category: "RSI", signalKey: "RSI_SPIKE_SHORT", slPct: 0.30, tpPct: 0.62, cooldownMin: 4, holdMinutes: 22, confluenceMin: 3 },
  { id: 9, name: "Stoch_Oversold_Long", category: "Stoch", signalKey: "STOCH_OVERSOLD_LONG", slPct: 0.28, tpPct: 0.60, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 10, name: "Stoch_Overbought_Short", category: "Stoch", signalKey: "STOCH_OVERBOUGHT_SHORT", slPct: 0.28, tpPct: 0.60, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 11, name: "MACD_Hist_Rise_Long", category: "MACD", signalKey: "MACD_HIST_RISE_LONG", slPct: 0.30, tpPct: 0.65, cooldownMin: 5, holdMinutes: 20, confluenceMin: 3 },
  { id: 12, name: "MACD_Hist_Fall_Short", category: "MACD", signalKey: "MACD_HIST_FALL_SHORT", slPct: 0.30, tpPct: 0.65, cooldownMin: 5, holdMinutes: 20, confluenceMin: 3 },
  { id: 13, name: "OBV_Trend_Long", category: "OBV", signalKey: "OBV_TREND_LONG", slPct: 0.32, tpPct: 0.60, cooldownMin: 4, holdMinutes: 24, confluenceMin: 4 },
  { id: 14, name: "OBV_Trend_Short", category: "OBV", signalKey: "OBV_TREND_SHORT", slPct: 0.32, tpPct: 0.60, cooldownMin: 4, holdMinutes: 24, confluenceMin: 4 },
  { id: 15, name: "Confluence_Break_Long", category: "Confluence", signalKey: "CONF_BREAK_LONG", slPct: 0.26, tpPct: 0.75, cooldownMin: 6, holdMinutes: 20, confluenceMin: 5 },
  { id: 16, name: "Confluence_Break_Short", category: "Confluence", signalKey: "CONF_BREAK_SHORT", slPct: 0.26, tpPct: 0.75, cooldownMin: 6, holdMinutes: 20, confluenceMin: 5 },
  { id: 17, name: "Vol_Spike_Long", category: "Vol", signalKey: "VOL_SPIKE_LONG", slPct: 0.34, tpPct: 0.68, cooldownMin: 4, holdMinutes: 16, confluenceMin: 4 },
  { id: 18, name: "Vol_Spike_Short", category: "Vol", signalKey: "VOL_SPIKE_SHORT", slPct: 0.34, tpPct: 0.68, cooldownMin: 4, holdMinutes: 16, confluenceMin: 4 },
  { id: 19, name: "BB_Squeeze_Long", category: "BB", signalKey: "BB_SQUEEZE_LONG", slPct: 0.30, tpPct: 0.72, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 20, name: "BB_Squeeze_Short", category: "BB", signalKey: "BB_SQUEEZE_SHORT", slPct: 0.30, tpPct: 0.72, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 21, name: "Stoch_Cross_Long", category: "Stoch", signalKey: "STOCH_CROSS_LONG", slPct: 0.28, tpPct: 0.60, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 22, name: "Stoch_Cross_Short", category: "Stoch", signalKey: "STOCH_CROSS_SHORT", slPct: 0.28, tpPct: 0.60, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 23, name: "MACD_ZeroCross_Long", category: "MACD", signalKey: "MACD_ZERO_LONG", slPct: 0.32, tpPct: 0.68, cooldownMin: 5, holdMinutes: 20, confluenceMin: 3 },
  { id: 24, name: "MACD_ZeroCross_Short", category: "MACD", signalKey: "MACD_ZERO_SHORT", slPct: 0.32, tpPct: 0.68, cooldownMin: 5, holdMinutes: 20, confluenceMin: 3 },
  { id: 25, name: "ATR_Break_Long", category: "Vol", signalKey: "ATR_BREAK_LONG", slPct: 0.35, tpPct: 0.70, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 26, name: "ATR_Break_Short", category: "Vol", signalKey: "ATR_BREAK_SHORT", slPct: 0.35, tpPct: 0.70, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 27, name: "VWAP_Dev_Long", category: "VWAP", signalKey: "VWAP_DEV_LONG", slPct: 0.30, tpPct: 0.65, cooldownMin: 4, holdMinutes: 22, confluenceMin: 4 },
  { id: 28, name: "VWAP_Dev_Short", category: "VWAP", signalKey: "VWAP_DEV_SHORT", slPct: 0.30, tpPct: 0.65, cooldownMin: 4, holdMinutes: 22, confluenceMin: 4 },
  { id: 29, name: "Multi_Conf_Long", category: "Confluence", signalKey: "MULTI_CONF_LONG", slPct: 0.24, tpPct: 0.85, cooldownMin: 8, holdMinutes: 24, confluenceMin: 6 },
  { id: 30, name: "Multi_Conf_Short", category: "Confluence", signalKey: "MULTI_CONF_SHORT", slPct: 0.24, tpPct: 0.85, cooldownMin: 8, holdMinutes: 24, confluenceMin: 6 },

  // 31-60: Advanced indicators (Williams %R, CCI, Keltner, Donchian, EMA Ribbon, Squeeze, ADX, ROC)
  { id: 31, name: "Williams_Oversold_Long", category: "Williams MR", signalKey: "WILLIAMS_OVERSOLD_LONG", slPct: 0.32, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 32, name: "Williams_Overbought_Short", category: "Williams MR", signalKey: "WILLIAMS_OVERBOUGHT_SHORT", slPct: 0.32, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 33, name: "CCI_Oversold_Long", category: "CCI MR", signalKey: "CCI_OVERSOLD_LONG", slPct: 0.34, tpPct: 0.62, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 34, name: "CCI_Overbought_Short", category: "CCI MR", signalKey: "CCI_OVERBOUGHT_SHORT", slPct: 0.34, tpPct: 0.62, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 35, name: "Keltner_Breakout_Long", category: "Keltner MR", signalKey: "KELTNER_BREAKOUT_LONG", slPct: 0.30, tpPct: 0.68, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 36, name: "Keltner_Breakout_Short", category: "Keltner MR", signalKey: "KELTNER_BREAKOUT_SHORT", slPct: 0.30, tpPct: 0.68, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 37, name: "Donchian_Break_Long", category: "Donchian Trend", signalKey: "DONCHIAN_BREAK_LONG", slPct: 0.28, tpPct: 0.72, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 38, name: "Donchian_Break_Short", category: "Donchian Trend", signalKey: "DONCHIAN_BREAK_SHORT", slPct: 0.28, tpPct: 0.72, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 39, name: "EMA_Ribbon_Long", category: "Ribbon", signalKey: "EMA_RIBBON_LONG", slPct: 0.26, tpPct: 0.70, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 40, name: "EMA_Ribbon_Short", category: "Ribbon", signalKey: "EMA_RIBBON_SHORT", slPct: 0.26, tpPct: 0.70, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 41, name: "Squeeze_Fire_Long", category: "Squeeze", signalKey: "SQUEEZE_FIRE_LONG", slPct: 0.32, tpPct: 0.78, cooldownMin: 5, holdMinutes: 20, confluenceMin: 4 },
  { id: 42, name: "Squeeze_Fire_Short", category: "Squeeze", signalKey: "SQUEEZE_FIRE_SHORT", slPct: 0.32, tpPct: 0.78, cooldownMin: 5, holdMinutes: 20, confluenceMin: 4 },
  { id: 43, name: "ADX_Trend_Long", category: "ADX Trend", signalKey: "ADX_TREND_LONG", slPct: 0.30, tpPct: 0.66, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4 },
  { id: 44, name: "ADX_Trend_Short", category: "ADX Trend", signalKey: "ADX_TREND_SHORT", slPct: 0.30, tpPct: 0.66, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4 },
  { id: 45, name: "ROC_Reversal_Long", category: "ROC Trend", signalKey: "ROC_REVERSAL_LONG", slPct: 0.34, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 46, name: "ROC_Reversal_Short", category: "ROC Trend", signalKey: "ROC_REVERSAL_SHORT", slPct: 0.34, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 47, name: "VWAP_MeanRev_Long", category: "VWAP MR", signalKey: "VWAP_MEANREV_LONG", slPct: 0.30, tpPct: 0.60, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 48, name: "VWAP_MeanRev_Short", category: "VWAP MR", signalKey: "VWAP_MEANREV_SHORT", slPct: 0.30, tpPct: 0.60, cooldownMin: 3, holdMinutes: 18, confluenceMin: 3 },
  { id: 49, name: "Williams_Strong_Long", category: "Williams MR", signalKey: "WILLIAMS_STRONG_LONG", slPct: 0.34, tpPct: 0.66, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 50, name: "Williams_Strong_Short", category: "Williams MR", signalKey: "WILLIAMS_STRONG_SHORT", slPct: 0.34, tpPct: 0.66, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 51, name: "CCI_Extreme_Long", category: "CCI MR", signalKey: "CCI_EXTREME_LONG", slPct: 0.36, tpPct: 0.60, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 52, name: "CCI_Extreme_Short", category: "CCI MR", signalKey: "CCI_EXTREME_SHORT", slPct: 0.36, tpPct: 0.60, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 53, name: "Keltner_Squeeze_Long", category: "Keltner MR", signalKey: "KELTNER_SQUEEZE_LONG", slPct: 0.32, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 54, name: "Keltner_Squeeze_Short", category: "Keltner MR", signalKey: "KELTNER_SQUEEZE_SHORT", slPct: 0.32, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 55, name: "Donchian_MeanRev_Long", category: "Donchian MR", signalKey: "DONCHIAN_MR_LONG", slPct: 0.30, tpPct: 0.58, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 56, name: "Donchian_MeanRev_Short", category: "Donchian MR", signalKey: "DONCHIAN_MR_SHORT", slPct: 0.30, tpPct: 0.58, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 57, name: "Ribbon_Accel_Long", category: "Ribbon", signalKey: "RIBBON_ACCEL_LONG", slPct: 0.28, tpPct: 0.74, cooldownMin: 5, holdMinutes: 20, confluenceMin: 4 },
  { id: 58, name: "Ribbon_Accel_Short", category: "Ribbon", signalKey: "RIBBON_ACCEL_SHORT", slPct: 0.28, tpPct: 0.74, cooldownMin: 5, holdMinutes: 20, confluenceMin: 4 },
  { id: 59, name: "Squeeze_Continuation_Long", category: "Squeeze", signalKey: "SQUEEZE_CONT_LONG", slPct: 0.32, tpPct: 0.72, cooldownMin: 6, holdMinutes: 24, confluenceMin: 4 },
  { id: 60, name: "Squeeze_Continuation_Short", category: "Squeeze", signalKey: "SQUEEZE_CONT_SHORT", slPct: 0.32, tpPct: 0.72, cooldownMin: 6, holdMinutes: 24, confluenceMin: 4 },

  // 61-110: Extended strategies
  { id: 61, name: "Williams_Momentum_Long", category: "Williams Trend", signalKey: "WILLIAMS_MOM_LONG", slPct: 0.32, tpPct: 0.68, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 62, name: "Williams_Momentum_Short", category: "Williams Trend", signalKey: "WILLIAMS_MOM_SHORT", slPct: 0.32, tpPct: 0.68, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 63, name: "CCI_Momentum_Long", category: "CCI Trend", signalKey: "CCI_MOM_LONG", slPct: 0.34, tpPct: 0.66, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 64, name: "CCI_Momentum_Short", category: "CCI Trend", signalKey: "CCI_MOM_SHORT", slPct: 0.34, tpPct: 0.66, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 65, name: "Keltner_Momentum_Long", category: "Keltner Trend", signalKey: "KELTNER_MOM_LONG", slPct: 0.30, tpPct: 0.70, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 66, name: "Keltner_Momentum_Short", category: "Keltner Trend", signalKey: "KELTNER_MOM_SHORT", slPct: 0.30, tpPct: 0.70, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 67, name: "Donchian_Momentum_Long", category: "Donchian Trend", signalKey: "DONCHIAN_MOM_LONG", slPct: 0.28, tpPct: 0.74, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 68, name: "Donchian_Momentum_Short", category: "Donchian Trend", signalKey: "DONCHIAN_MOM_SHORT", slPct: 0.28, tpPct: 0.74, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 69, name: "Ribbon_Momentum_Long", category: "Ribbon", signalKey: "RIBBON_MOM_LONG", slPct: 0.26, tpPct: 0.76, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 70, name: "Ribbon_Momentum_Short", category: "Ribbon", signalKey: "RIBBON_MOM_SHORT", slPct: 0.26, tpPct: 0.76, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 71, name: "Squeeze_Momentum_Long", category: "Squeeze", signalKey: "SQUEEZE_MOM_LONG", slPct: 0.32, tpPct: 0.80, cooldownMin: 6, holdMinutes: 22, confluenceMin: 4 },
  { id: 72, name: "Squeeze_Momentum_Short", category: "Squeeze", signalKey: "SQUEEZE_MOM_SHORT", slPct: 0.32, tpPct: 0.80, cooldownMin: 6, holdMinutes: 22, confluenceMin: 4 },
  { id: 73, name: "ADX_Momentum_Long", category: "ADX Trend", signalKey: "ADX_MOM_LONG", slPct: 0.30, tpPct: 0.72, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4 },
  { id: 74, name: "ADX_Momentum_Short", category: "ADX Trend", signalKey: "ADX_MOM_SHORT", slPct: 0.30, tpPct: 0.72, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4 },
  { id: 75, name: "ROC_Momentum_Long", category: "ROC Trend", signalKey: "ROC_MOM_LONG", slPct: 0.34, tpPct: 0.68, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 76, name: "ROC_Momentum_Short", category: "ROC Trend", signalKey: "ROC_MOM_SHORT", slPct: 0.34, tpPct: 0.68, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 77, name: "Volatility_Break_Long", category: "Vol", signalKey: "VOL_BREAK_LONG", slPct: 0.36, tpPct: 0.74, cooldownMin: 4, holdMinutes: 18, confluenceMin: 4 },
  { id: 78, name: "Volatility_Break_Short", category: "Vol", signalKey: "VOL_BREAK_SHORT", slPct: 0.36, tpPct: 0.74, cooldownMin: 4, holdMinutes: 18, confluenceMin: 4 },
  { id: 79, name: "RSI_Divergence_Long", category: "RSI Div", signalKey: "RSI_DIV_LONG", slPct: 0.32, tpPct: 0.70, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 80, name: "RSI_Divergence_Short", category: "RSI Div", signalKey: "RSI_DIV_SHORT", slPct: 0.32, tpPct: 0.70, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 81, name: "MACD_Divergence_Long", category: "MACD Div", signalKey: "MACD_DIV_LONG", slPct: 0.32, tpPct: 0.72, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 82, name: "MACD_Divergence_Short", category: "MACD Div", signalKey: "MACD_DIV_SHORT", slPct: 0.32, tpPct: 0.72, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 83, name: "Stoch_Divergence_Long", category: "Stoch Div", signalKey: "STOCH_DIV_LONG", slPct: 0.30, tpPct: 0.68, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 84, name: "Stoch_Divergence_Short", category: "Stoch Div", signalKey: "STOCH_DIV_SHORT", slPct: 0.30, tpPct: 0.68, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 85, name: "BB_Divergence_Long", category: "BB Div", signalKey: "BB_DIV_LONG", slPct: 0.32, tpPct: 0.66, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 86, name: "BB_Divergence_Short", category: "BB Div", signalKey: "BB_DIV_SHORT", slPct: 0.32, tpPct: 0.66, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 87, name: "VWAP_Divergence_Long", category: "VWAP Div", signalKey: "VWAP_DIV_LONG", slPct: 0.30, tpPct: 0.64, cooldownMin: 5, holdMinutes: 22, confluenceMin: 3 },
  { id: 88, name: "VWAP_Divergence_Short", category: "VWAP Div", signalKey: "VWAP_DIV_SHORT", slPct: 0.30, tpPct: 0.64, cooldownMin: 5, holdMinutes: 22, confluenceMin: 3 },
  { id: 89, name: "Support_Bounce_Long", category: "SR", signalKey: "SUPPORT_BOUNCE_LONG", slPct: 0.28, tpPct: 0.70, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 90, name: "Resistance_Bounce_Short", category: "SR", signalKey: "RESISTANCE_BOUNCE_SHORT", slPct: 0.28, tpPct: 0.70, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 91, name: "Trend_Continuation_Long", category: "Trend", signalKey: "TREND_CONT_LONG", slPct: 0.26, tpPct: 0.80, cooldownMin: 6, holdMinutes: 26, confluenceMin: 5 },
  { id: 92, name: "Trend_Continuation_Short", category: "Trend", signalKey: "TREND_CONT_SHORT", slPct: 0.26, tpPct: 0.80, cooldownMin: 6, holdMinutes: 26, confluenceMin: 5 },
  { id: 93, name: "Mean_Reversion_Long", category: "MR", signalKey: "MEAN_REV_LONG", slPct: 0.34, tpPct: 0.62, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 94, name: "Mean_Reversion_Short", category: "MR", signalKey: "MEAN_REV_SHORT", slPct: 0.34, tpPct: 0.62, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 95, name: "Breakout_Long", category: "Breakout", signalKey: "BREAKOUT_LONG", slPct: 0.32, tpPct: 0.85, cooldownMin: 5, holdMinutes: 20, confluenceMin: 5 },
  { id: 96, name: "Breakout_Short", category: "Breakout", signalKey: "BREAKOUT_SHORT", slPct: 0.32, tpPct: 0.85, cooldownMin: 5, holdMinutes: 20, confluenceMin: 5 },
  { id: 97, name: "False_Breakout_Long", category: "False Break", signalKey: "FALSE_BREAK_LONG", slPct: 0.30, tpPct: 0.60, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 98, name: "False_Breakout_Short", category: "False Break", signalKey: "FALSE_BREAK_SHORT", slPct: 0.30, tpPct: 0.60, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 99, name: "Range_Bound_Long", category: "Range", signalKey: "RANGE_BOUND_LONG", slPct: 0.32, tpPct: 0.58, cooldownMin: 3, holdMinutes: 16, confluenceMin: 3 },
  { id: 100, name: "Range_Bound_Short", category: "Range", signalKey: "RANGE_BOUND_SHORT", slPct: 0.32, tpPct: 0.58, cooldownMin: 3, holdMinutes: 16, confluenceMin: 3 },
  { id: 101, name: "Gap_Fill_Long", category: "Gap", signalKey: "GAP_FILL_LONG", slPct: 0.30, tpPct: 0.64, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 102, name: "Gap_Fill_Short", category: "Gap", signalKey: "GAP_FILL_SHORT", slPct: 0.30, tpPct: 0.64, cooldownMin: 4, holdMinutes: 18, confluenceMin: 3 },
  { id: 103, name: "News_Driven_Long", category: "News", signalKey: "NEWS_LONG", slPct: 0.38, tpPct: 0.90, cooldownMin: 3, holdMinutes: 14, confluenceMin: 4 },
  { id: 104, name: "News_Driven_Short", category: "News", signalKey: "NEWS_SHORT", slPct: 0.38, tpPct: 0.90, cooldownMin: 3, holdMinutes: 14, confluenceMin: 4 },
  { id: 105, name: "Flash_Crash_Long", category: "Crash", signalKey: "FLASH_CRASH_LONG", slPct: 0.42, tpPct: 1.00, cooldownMin: 2, holdMinutes: 12, confluenceMin: 3 },
  { id: 106, name: "Flash_Pump_Short", category: "Pump", signalKey: "FLASH_PUMP_SHORT", slPct: 0.42, tpPct: 1.00, cooldownMin: 2, holdMinutes: 12, confluenceMin: 3 },
  { id: 107, name: "Algorithmic_Long", category: "Algo", signalKey: "ALGO_LONG", slPct: 0.28, tpPct: 0.68, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 108, name: "Algorithmic_Short", category: "Algo", signalKey: "ALGO_SHORT", slPct: 0.28, tpPct: 0.68, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 109, name: "Quantitative_Long", category: "Quant", signalKey: "QUANT_LONG", slPct: 0.26, tpPct: 0.72, cooldownMin: 6, holdMinutes: 24, confluenceMin: 5 },
  { id: 110, name: "Quantitative_Short", category: "Quant", signalKey: "QUANT_SHORT", slPct: 0.26, tpPct: 0.72, cooldownMin: 6, holdMinutes: 24, confluenceMin: 5 },

  // 111-130: Multi-Timeframe strategies
  { id: 111, name: "MTF_Trend_Align_Long", category: "MTF Trend", signalKey: "MTF_TREND_ALIGN_LONG", slPct: 0.26, tpPct: 0.82, cooldownMin: 6, holdMinutes: 32, confluenceMin: 4, requiresHtf: true },
  { id: 112, name: "MTF_Trend_Align_Short", category: "MTF Trend", signalKey: "MTF_TREND_ALIGN_SHORT", slPct: 0.26, tpPct: 0.82, cooldownMin: 6, holdMinutes: 32, confluenceMin: 4, requiresHtf: true },
  { id: 113, name: "MTF_RSI_Converge_Long", category: "MTF RSI", signalKey: "MTF_RSI_CONVERGE_LONG", slPct: 0.28, tpPct: 0.76, cooldownMin: 5, holdMinutes: 28, confluenceMin: 4, requiresHtf: true },
  { id: 114, name: "MTF_RSI_Converge_Short", category: "MTF RSI", signalKey: "MTF_RSI_CONVERGE_SHORT", slPct: 0.28, tpPct: 0.76, cooldownMin: 5, holdMinutes: 28, confluenceMin: 4, requiresHtf: true },
  { id: 115, name: "MTF_Mom_Cascade_Long", category: "MTF Mom", signalKey: "MTF_MOM_CASCADE_LONG", slPct: 0.30, tpPct: 0.78, cooldownMin: 6, holdMinutes: 30, confluenceMin: 5, requiresHtf: true },
  { id: 116, name: "MTF_Mom_Cascade_Short", category: "MTF Mom", signalKey: "MTF_MOM_CASCADE_SHORT", slPct: 0.30, tpPct: 0.78, cooldownMin: 6, holdMinutes: 30, confluenceMin: 5, requiresHtf: true },
  { id: 117, name: "MTF_MACD_Align_Long", category: "MTF MACD", signalKey: "MTF_MACD_ALIGN_LONG", slPct: 0.28, tpPct: 0.74, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4, requiresHtf: true },
  { id: 118, name: "MTF_MACD_Align_Short", category: "MTF MACD", signalKey: "MTF_MACD_ALIGN_SHORT", slPct: 0.28, tpPct: 0.74, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4, requiresHtf: true },
  { id: 119, name: "MTF_Squeeze_Fire_Long", category: "MTF Squeeze", signalKey: "MTF_SQUEEZE_FIRE_LONG", slPct: 0.30, tpPct: 0.86, cooldownMin: 7, holdMinutes: 32, confluenceMin: 5, requiresHtf: true },
  { id: 120, name: "MTF_Squeeze_Fire_Short", category: "MTF Squeeze", signalKey: "MTF_SQUEEZE_FIRE_SHORT", slPct: 0.30, tpPct: 0.86, cooldownMin: 7, holdMinutes: 32, confluenceMin: 5, requiresHtf: true },
  { id: 121, name: "MTF_Pullback_Long", category: "MTF Pullback", signalKey: "MTF_PULLBACK_LONG", slPct: 0.32, tpPct: 0.70, cooldownMin: 5, holdMinutes: 26, confluenceMin: 4, requiresHtf: true },
  { id: 122, name: "MTF_Pullback_Short", category: "MTF Pullback", signalKey: "MTF_PULLBACK_SHORT", slPct: 0.32, tpPct: 0.70, cooldownMin: 5, holdMinutes: 26, confluenceMin: 4, requiresHtf: true },
  { id: 123, name: "MTF_ADX_Power_Long", category: "MTF ADX", signalKey: "MTF_ADX_POWER_LONG", slPct: 0.28, tpPct: 0.80, cooldownMin: 7, holdMinutes: 34, confluenceMin: 5, requiresHtf: true },
  { id: 124, name: "MTF_ADX_Power_Short", category: "MTF ADX", signalKey: "MTF_ADX_POWER_SHORT", slPct: 0.28, tpPct: 0.80, cooldownMin: 7, holdMinutes: 34, confluenceMin: 5, requiresHtf: true },
  { id: 125, name: "MTF_Breakout_Long", category: "MTF Break", signalKey: "MTF_BREAKOUT_LONG", slPct: 0.30, tpPct: 0.92, cooldownMin: 6, holdMinutes: 28, confluenceMin: 5, requiresHtf: true },
  { id: 126, name: "MTF_Breakout_Short", category: "MTF Break", signalKey: "MTF_BREAKOUT_SHORT", slPct: 0.30, tpPct: 0.92, cooldownMin: 6, holdMinutes: 28, confluenceMin: 5, requiresHtf: true },
  { id: 127, name: "MTF_MeanRev_Long", category: "MTF MR", signalKey: "MTF_MEAN_REVERT_LONG", slPct: 0.34, tpPct: 0.64, cooldownMin: 5, holdMinutes: 24, confluenceMin: 3, requiresHtf: true },
  { id: 128, name: "MTF_MeanRev_Short", category: "MTF MR", signalKey: "MTF_MEAN_REVERT_SHORT", slPct: 0.34, tpPct: 0.64, cooldownMin: 5, holdMinutes: 24, confluenceMin: 3, requiresHtf: true },
  { id: 129, name: "MTF_Confluence_Long", category: "MTF Conf", signalKey: "MTF_CONFLUENCE_LONG", slPct: 0.24, tpPct: 0.88, cooldownMin: 8, holdMinutes: 36, confluenceMin: 6, requiresHtf: true },
  { id: 130, name: "MTF_Confluence_Short", category: "MTF Conf", signalKey: "MTF_CONFLUENCE_SHORT", slPct: 0.24, tpPct: 0.88, cooldownMin: 8, holdMinutes: 36, confluenceMin: 6, requiresHtf: true },

  // 131-180: Advanced Futures Strategies (Pro Grade)
  // Smart Money & Order Flow Concepts
  { id: 131, name: "SmartMoney_Accum_Long", category: "Smart Money", signalKey: "SM_ACCUM_LONG", slPct: 0.26, tpPct: 0.85, cooldownMin: 6, holdMinutes: 32, confluenceMin: 5 },
  { id: 132, name: "SmartMoney_Distrib_Short", category: "Smart Money", signalKey: "SM_DISTRIB_SHORT", slPct: 0.26, tpPct: 0.85, cooldownMin: 6, holdMinutes: 32, confluenceMin: 5 },
  { id: 133, name: "OrderFlow_Break_Long", category: "Order Flow", signalKey: "OF_BREAK_LONG", slPct: 0.28, tpPct: 0.90, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 134, name: "OrderFlow_Break_Short", category: "Order Flow", signalKey: "OF_BREAK_SHORT", slPct: 0.28, tpPct: 0.90, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 135, name: "LiquidityGrab_Long", category: "Liquidity", signalKey: "LIQ_GRAB_LONG", slPct: 0.30, tpPct: 0.82, cooldownMin: 4, holdMinutes: 20, confluenceMin: 4 },
  { id: 136, name: "LiquidityGrab_Short", category: "Liquidity", signalKey: "LIQ_GRAB_SHORT", slPct: 0.30, tpPct: 0.82, cooldownMin: 4, holdMinutes: 20, confluenceMin: 4 },
  { id: 137, name: "StopHunt_Long", category: "Stop Hunt", signalKey: "STOP_HUNT_LONG", slPct: 0.32, tpPct: 0.78, cooldownMin: 3, holdMinutes: 16, confluenceMin: 4 },
  { id: 138, name: "StopHunt_Short", category: "Stop Hunt", signalKey: "STOP_HUNT_SHORT", slPct: 0.32, tpPct: 0.78, cooldownMin: 3, holdMinutes: 16, confluenceMin: 4 },

  // Wyckoff Method Strategies
  { id: 139, name: "Wyckoff_Spring_Long", category: "Wyckoff", signalKey: "WYCKOFF_SPRING_LONG", slPct: 0.28, tpPct: 0.95, cooldownMin: 8, holdMinutes: 38, confluenceMin: 5 },
  { id: 140, name: "Wyckoff_Upthrust_Short", category: "Wyckoff", signalKey: "WYCKOFF_UPTHRUST_SHORT", slPct: 0.28, tpPct: 0.95, cooldownMin: 8, holdMinutes: 38, confluenceMin: 5 },
  { id: 141, name: "Wyckoff_MarkUp_Long", category: "Wyckoff", signalKey: "WYCKOFF_MARKUP_LONG", slPct: 0.24, tpPct: 0.88, cooldownMin: 6, holdMinutes: 30, confluenceMin: 5 },
  { id: 142, name: "Wyckoff_MarkDown_Short", category: "Wyckoff", signalKey: "WYCKOFF_MARKDOWN_SHORT", slPct: 0.24, tpPct: 0.88, cooldownMin: 6, holdMinutes: 30, confluenceMin: 5 },

  // Volume Profile & Market Structure
  { id: 143, name: "VolProfile_HVN_Long", category: "Vol Profile", signalKey: "VP_HVN_LONG", slPct: 0.30, tpPct: 0.76, cooldownMin: 5, holdMinutes: 26, confluenceMin: 4 },
  { id: 144, name: "VolProfile_HVN_Short", category: "Vol Profile", signalKey: "VP_HVN_SHORT", slPct: 0.30, tpPct: 0.76, cooldownMin: 5, holdMinutes: 26, confluenceMin: 4 },
  { id: 145, name: "MarketStructure_BOS_Long", category: "Market Structure", signalKey: "MS_BOS_LONG", slPct: 0.26, tpPct: 0.92, cooldownMin: 6, holdMinutes: 28, confluenceMin: 5 },
  { id: 146, name: "MarketStructure_BOS_Short", category: "Market Structure", signalKey: "MS_BOS_SHORT", slPct: 0.26, tpPct: 0.92, cooldownMin: 6, holdMinutes: 28, confluenceMin: 5 },
  { id: 147, name: "CHoCH_Long", category: "Market Structure", signalKey: "CHOCH_LONG", slPct: 0.28, tpPct: 0.84, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 148, name: "CHoCH_Short", category: "Market Structure", signalKey: "CHOCH_SHORT", slPct: 0.28, tpPct: 0.84, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },

  // Institutional Style Strategies
  { id: 149, name: "Institutional_Pivot_Long", category: "Institutional", signalKey: "INST_PIVOT_LONG", slPct: 0.25, tpPct: 0.80, cooldownMin: 7, holdMinutes: 34, confluenceMin: 6 },
  { id: 150, name: "Institutional_Pivot_Short", category: "Institutional", signalKey: "INST_PIVOT_SHORT", slPct: 0.25, tpPct: 0.80, cooldownMin: 7, holdMinutes: 34, confluenceMin: 6 },
  { id: 151, name: "OpeningDrive_Long", category: "Session", signalKey: "OPEN_DRIVE_LONG", slPct: 0.32, tpPct: 0.72, cooldownMin: 3, holdMinutes: 18, confluenceMin: 4 },
  { id: 152, name: "OpeningDrive_Short", category: "Session", signalKey: "OPEN_DRIVE_SHORT", slPct: 0.32, tpPct: 0.72, cooldownMin: 3, holdMinutes: 18, confluenceMin: 4 },
  { id: 153, name: "ClosingRange_Long", category: "Session", signalKey: "CLOSE_RANGE_LONG", slPct: 0.30, tpPct: 0.68, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 154, name: "ClosingRange_Short", category: "Session", signalKey: "CLOSE_RANGE_SHORT", slPct: 0.30, tpPct: 0.68, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },

  // Statistical & Quant Strategies
  { id: 155, name: "StatArb_ZScore_Long", category: "Statistical", signalKey: "STAT_ZSCORE_LONG", slPct: 0.34, tpPct: 0.66, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 156, name: "StatArb_ZScore_Short", category: "Statistical", signalKey: "STAT_ZSCORE_SHORT", slPct: 0.34, tpPct: 0.66, cooldownMin: 5, holdMinutes: 22, confluenceMin: 4 },
  { id: 157, name: "Regression_Mean_Long", category: "Statistical", signalKey: "REG_MEAN_LONG", slPct: 0.32, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 158, name: "Regression_Mean_Short", category: "Statistical", signalKey: "REG_MEAN_SHORT", slPct: 0.32, tpPct: 0.64, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 },
  { id: 159, name: "Momentum_Divergence_Long", category: "Divergence", signalKey: "MOM_DIV_LONG", slPct: 0.30, tpPct: 0.74, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },
  { id: 160, name: "Momentum_Divergence_Short", category: "Divergence", signalKey: "MOM_DIV_SHORT", slPct: 0.30, tpPct: 0.74, cooldownMin: 6, holdMinutes: 26, confluenceMin: 4 },

  // Harmonic & Pattern Strategies
  { id: 161, name: "Harmonic_Bat_Long", category: "Harmonic", signalKey: "HARM_BAT_LONG", slPct: 0.28, tpPct: 0.86, cooldownMin: 8, holdMinutes: 36, confluenceMin: 5 },
  { id: 162, name: "Harmonic_Bat_Short", category: "Harmonic", signalKey: "HARM_BAT_SHORT", slPct: 0.28, tpPct: 0.86, cooldownMin: 8, holdMinutes: 36, confluenceMin: 5 },
  { id: 163, name: "Pattern_Flag_Long", category: "Chart Pattern", signalKey: "PATTERN_FLAG_LONG", slPct: 0.26, tpPct: 0.78, cooldownMin: 5, holdMinutes: 28, confluenceMin: 4 },
  { id: 164, name: "Pattern_Flag_Short", category: "Chart Pattern", signalKey: "PATTERN_FLAG_SHORT", slPct: 0.26, tpPct: 0.78, cooldownMin: 5, holdMinutes: 28, confluenceMin: 4 },
  { id: 165, name: "Pattern_Pennant_Long", category: "Chart Pattern", signalKey: "PATTERN_PENNANT_LONG", slPct: 0.27, tpPct: 0.80, cooldownMin: 6, holdMinutes: 30, confluenceMin: 4 },
  { id: 166, name: "Pattern_Pennant_Short", category: "Chart Pattern", signalKey: "PATTERN_PENNANT_SHORT", slPct: 0.27, tpPct: 0.80, cooldownMin: 6, holdMinutes: 30, confluenceMin: 4 },

  // Options Greeks Inspired (adapted for futures)
  { id: 167, name: "Delta_Squeeze_Long", category: "Greek-Inspired", signalKey: "DELTA_SQUEEZE_LONG", slPct: 0.30, tpPct: 0.82, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 168, name: "Delta_Squeeze_Short", category: "Greek-Inspired", signalKey: "DELTA_SQUEEZE_SHORT", slPct: 0.30, tpPct: 0.82, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 169, name: "Gamma_Spike_Long", category: "Greek-Inspired", signalKey: "GAMMA_SPIKE_LONG", slPct: 0.34, tpPct: 0.76, cooldownMin: 4, holdMinutes: 18, confluenceMin: 4 },
  { id: 170, name: "Gamma_Spike_Short", category: "Greek-Inspired", signalKey: "GAMMA_SPIKE_SHORT", slPct: 0.34, tpPct: 0.76, cooldownMin: 4, holdMinutes: 18, confluenceMin: 4 },

  // Event & News Driven
  { id: 171, name: "Event_Driven_Long", category: "Event", signalKey: "EVENT_LONG", slPct: 0.40, tpPct: 1.10, cooldownMin: 3, holdMinutes: 15, confluenceMin: 4 },
  { id: 172, name: "Event_Driven_Short", category: "Event", signalKey: "EVENT_SHORT", slPct: 0.40, tpPct: 1.10, cooldownMin: 3, holdMinutes: 15, confluenceMin: 4 },
  { id: 173, name: "PostEvent_Retrace_Long", category: "Event", signalKey: "POST_EVENT_LONG", slPct: 0.32, tpPct: 0.70, cooldownMin: 4, holdMinutes: 22, confluenceMin: 3 },
  { id: 174, name: "PostEvent_Retrace_Short", category: "Event", signalKey: "POST_EVENT_SHORT", slPct: 0.32, tpPct: 0.70, cooldownMin: 4, holdMinutes: 22, confluenceMin: 3 },

  // Machine Learning Style (rule-based approximations)
  { id: 175, name: "ML_Ensemble_Long", category: "ML-Style", signalKey: "ML_ENSEMBLE_LONG", slPct: 0.26, tpPct: 0.84, cooldownMin: 6, holdMinutes: 30, confluenceMin: 6 },
  { id: 176, name: "ML_Ensemble_Short", category: "ML-Style", signalKey: "ML_ENSEMBLE_SHORT", slPct: 0.26, tpPct: 0.84, cooldownMin: 6, holdMinutes: 30, confluenceMin: 6 },
  { id: 177, name: "ML_Classifier_Long", category: "ML-Style", signalKey: "ML_CLASS_LONG", slPct: 0.28, tpPct: 0.80, cooldownMin: 5, holdMinutes: 26, confluenceMin: 5 },
  { id: 178, name: "ML_Classifier_Short", category: "ML-Style", signalKey: "ML_CLASS_SHORT", slPct: 0.28, tpPct: 0.80, cooldownMin: 5, holdMinutes: 26, confluenceMin: 5 },

  // Correlation & Intermarket
  { id: 179, name: "RiskOn_Rally_Long", category: "Macro", signalKey: "RISK_ON_LONG", slPct: 0.30, tpPct: 0.74, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
  { id: 180, name: "RiskOff_Dump_Short", category: "Macro", signalKey: "RISK_OFF_SHORT", slPct: 0.30, tpPct: 0.74, cooldownMin: 5, holdMinutes: 24, confluenceMin: 4 },
];

// ========== HELPER FUNCTIONS ==========
function sma(arr: number[], period: number): number[] {
  const out: number[] = [];
  for (let i = 0; i < arr.length; i++) {
    if (i + 1 < period) {
      out.push(NaN);
      continue;
    }
    let sum = 0;
    for (let j = 0; j < period; j++) sum += arr[i - j];
    out.push(sum / period);
  }
  return out;
}

function ema(arr: number[], period: number): number[] {
  const k = 2 / (period + 1);
  const out: number[] = [];
  let prev: number | null = null;
  for (let i = 0; i < arr.length; i++) {
    if (i + 1 < period) {
      out.push(NaN);
      continue;
    }
    if (prev === null) {
      let sum = 0;
      for (let j = 0; j < period; j++) sum += arr[i - j];
      prev = sum / period;
    } else {
      prev = arr[i] * k + prev * (1 - k);
    }
    out.push(prev);
  }
  return out;
}

function rsi(closes: number[], period = 14): number[] {
  const out: number[] = [];
  let gainSum = 0;
  let lossSum = 0;
  for (let i = 1; i < closes.length; i++) {
    const change = closes[i] - closes[i - 1];
    const gain = Math.max(change, 0);
    const loss = Math.max(-change, 0);
    if (i < period) {
      gainSum += gain;
      lossSum += loss;
      out.push(NaN);
    } else if (i === period) {
      gainSum += gain;
      lossSum += loss;
      const avgGain = gainSum / period;
      const avgLoss = lossSum / period;
      const rs = avgLoss === 0 ? 0 : avgGain / avgLoss;
      out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs));
    } else {
      const prevAvgGain = (gainSum / period) * (period - 1) + gain;
      const prevAvgLoss = (lossSum / period) * (period - 1) + loss;
      gainSum = prevAvgGain;
      lossSum = prevAvgLoss;
      const avgGain = gainSum / period;
      const avgLoss = lossSum / period;
      const rs = avgLoss === 0 ? 0 : avgGain / avgLoss;
      out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs));
    }
  }
  return out;
}

function stdDev(arr: number[], period: number): number[] {
  const out: number[] = [];
  for (let i = 0; i < arr.length; i++) {
    if (i + 1 < period) {
      out.push(NaN);
      continue;
    }
    let sum = 0;
    for (let j = 0; j < period; j++) sum += arr[i - j];
    const mean = sum / period;
    let sqSum = 0;
    for (let j = 0; j < period; j++) sqSum += Math.pow(arr[i - j] - mean, 2);
    out.push(Math.sqrt(sqSum / period));
  }
  return out;
}

function stochastic(high: number[], low: number[], close: number[], kPeriod = 14, dPeriod = 3): { k: number[]; d: number[] } {
  const k: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i + 1 < kPeriod) {
      k.push(NaN);
      continue;
    }
    let highest = -Infinity;
    let lowest = Infinity;
    for (let j = 0; j < kPeriod; j++) {
      highest = Math.max(highest, high[i - j]);
      lowest = Math.min(lowest, low[i - j]);
    }
    const range = highest - lowest;
    k.push(range === 0 ? 50 : ((close[i] - lowest) / range) * 100);
  }
  const d = sma(k, dPeriod);
  return { k, d };
}

function macd(close: number[], fast = 12, slow = 26, signal = 9): { line: number[]; signal: number[]; hist: number[] } {
  const fastEma = ema(close, fast);
  const slowEma = ema(close, slow);
  const line: number[] = [];
  for (let i = 0; i < close.length; i++) line.push(fastEma[i] - slowEma[i]);
  const sig = ema(line.filter(n => !isNaN(n)), signal);
  const paddedSig: number[] = [];
  for (let i = 0; i < line.length; i++) paddedSig.push(isNaN(line[i]) ? NaN : (sig[i - (line.length - sig.length)] ?? NaN));
  const hist: number[] = [];
  for (let i = 0; i < line.length; i++) hist.push(line[i] - paddedSig[i]);
  return { line, signal: paddedSig, hist };
}

function atr(high: number[], low: number[], close: number[], period = 14): number[] {
  const tr: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i === 0) { tr.push(high[i] - low[i]); continue; }
    const v1 = high[i] - low[i];
    const v2 = Math.abs(high[i] - close[i - 1]);
    const v3 = Math.abs(low[i] - close[i - 1]);
    tr.push(Math.max(v1, v2, v3));
  }
  return sma(tr, period);
}

function obv(close: number[], volume: number[]): number[] {
  const out: number[] = [volume[0] ?? 0];
  for (let i = 1; i < close.length; i++) {
    if (close[i] > close[i - 1]) out.push(out[i - 1] + volume[i]);
    else if (close[i] < close[i - 1]) out.push(out[i - 1] - volume[i]);
    else out.push(out[i - 1]);
  }
  return out;
}

function slope(arr: number[], period = 5): number[] {
  const out: number[] = [];
  for (let i = 0; i < arr.length; i++) {
    if (i < period) { out.push(0); continue; }
    out.push(arr[i] - arr[i - period]);
  }
  return out;
}

function williamsR(high: number[], low: number[], close: number[], period = 14): number[] {
  const out: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i + 1 < period) { out.push(NaN); continue; }
    let highest = -Infinity;
    let lowest = Infinity;
    for (let j = 0; j < period; j++) {
      highest = Math.max(highest, high[i - j]);
      lowest = Math.min(lowest, low[i - j]);
    }
    const range = highest - lowest;
    out.push(range === 0 ? -50 : ((highest - close[i]) / range) * -100);
  }
  return out;
}

function cci(high: number[], low: number[], close: number[], period = 20): number[] {
  const out: number[] = [];
  const tp = high.map((h, i) => (h + low[i] + close[i]) / 3);
  const ma = sma(tp, period);
  const md: number[] = [];
  for (let i = 0; i < tp.length; i++) {
    if (i + 1 < period) { md.push(NaN); continue; }
    let sum = 0;
    for (let j = 0; j < period; j++) sum += Math.abs(tp[i - j] - ma[i]);
    md.push(sum / period);
  }
  for (let i = 0; i < tp.length; i++) out.push(md[i] === 0 ? 0 : (tp[i] - ma[i]) / (0.015 * md[i]));
  return out;
}

function rateOfChange(close: number[], period = 10): number[] {
  const out: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i < period) { out.push(NaN); continue; }
    out.push(((close[i] - close[i - period]) / close[i - period]) * 100);
  }
  return out;
}

function adxProxy(high: number[], low: number[], close: number[], period = 14): number[] {
  const tr: number[] = [];
  const plusDM: number[] = [];
  const minusDM: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i === 0) { tr.push(high[i] - low[i]); plusDM.push(0); minusDM.push(0); continue; }
    tr.push(Math.max(high[i] - low[i], Math.abs(high[i] - close[i - 1]), Math.abs(low[i] - close[i - 1])));
    plusDM.push(high[i] - high[i - 1] > low[i - 1] - low[i] ? Math.max(high[i] - high[i - 1], 0) : 0);
    minusDM.push(low[i - 1] - low[i] > high[i] - high[i - 1] ? Math.max(low[i - 1] - low[i], 0) : 0);
  }
  const atrVals = sma(tr, period);
  const plusDI: number[] = [];
  const minusDI: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (atrVals[i] === 0 || isNaN(atrVals[i])) { plusDI.push(NaN); minusDI.push(NaN); continue; }
    plusDI.push((sma(plusDM, period)[i] / atrVals[i]) * 100);
    minusDI.push((sma(minusDM, period)[i] / atrVals[i]) * 100);
  }
  const dx: number[] = [];
  for (let i = 0; i < close.length; i++) {
    const sum = plusDI[i] + minusDI[i];
    dx.push(sum === 0 || isNaN(sum) ? NaN : (Math.abs(plusDI[i] - minusDI[i]) / sum) * 100);
  }
  return sma(dx, period);
}

function keltner(high: number[], low: number[], close: number[], emaPeriod = 20, atrPeriod = 14, multiplier = 2): { upper: number[]; lower: number[] } {
  const mid = ema(close, emaPeriod);
  const atrVals = atr(high, low, close, atrPeriod);
  const upper: number[] = [];
  const lower: number[] = [];
  for (let i = 0; i < close.length; i++) {
    upper.push(mid[i] + multiplier * atrVals[i]);
    lower.push(mid[i] - multiplier * atrVals[i]);
  }
  return { upper, lower };
}

function donchian(high: number[], low: number[], period = 20): { upper: number[]; lower: number[]; mid: number[] } {
  const upper: number[] = [];
  const lower: number[] = [];
  for (let i = 0; i < high.length; i++) {
    if (i + 1 < period) { upper.push(NaN); lower.push(NaN); continue; }
    let highest = -Infinity;
    let lowest = Infinity;
    for (let j = 0; j < period; j++) {
      highest = Math.max(highest, high[i - j]);
      lowest = Math.min(lowest, low[i - j]);
    }
    upper.push(highest);
    lower.push(lowest);
  }
  const mid = upper.map((u, i) => (u + lower[i]) / 2);
  return { upper, lower, mid };
}

function aggregateBars(bars1m: number[], periodMinutes: number): number[] {
  const aggregated: number[] = [];
  let sum = 0;
  let count = 0;
  for (let i = 0; i < bars1m.length; i++) {
    sum += bars1m[i];
    count++;
    if (count === periodMinutes) {
      aggregated.push(sum / periodMinutes);
      sum = 0;
      count = 0;
    }
  }
  return aggregated;
}

function htfTrend(fast: number, slow: number, momentum: number): "UP" | "DOWN" | "NEUTRAL" {
  if (fast > slow && momentum > 0) return "UP";
  if (fast < slow && momentum < 0) return "DOWN";
  return "NEUTRAL";
}

// ========== LIQUIDATION CALCULATION ==========
function calculateLiquidationPrice(entryPrice: number, side: Side, leverage: number, mmPct = 0.005): number {
  // mmPct = maintenance margin (0.5% default)
  // Liquidation when: margin - loss = maintenance margin
  // For LONG: entry * (1 - 1/leverage + mm) 
  // For SHORT: entry * (1 + 1/leverage - mm)
  if (side === "LONG") {
    return entryPrice * (1 - 1 / leverage + mmPct);
  } else {
    return entryPrice * (1 + 1 / leverage - mmPct);
  }
}

function calculateDistanceToLiquidation(price: number, liquidationPrice: number, side: Side): number {
  if (side === "LONG") {
    return ((price - liquidationPrice) / price) * 100;
  } else {
    return ((liquidationPrice - price) / price) * 100;
  }
}

// ========== MARGIN CALCULATIONS ==========
function calculateMarginRequired(notional: number, leverage: number): number {
  return notional / leverage;
}

function calculateContracts(notional: number, price: number): number {
  return Math.floor(notional / CONTRACT_SIZE);
}

function calculateNotional(contracts: number): number {
  return contracts * CONTRACT_SIZE;
}

// ========== PnL CALCULATIONS ==========
/** USD PnL for isolated linear-style paper: notional moves ~1:1 with underlying % change. */
function calculateUnrealizedPnL(entryPrice: number, markPrice: number, notional: number, side: Side): number {
  if (!entryPrice || !Number.isFinite(notional) || notional <= 0) return 0;
  const pct = side === "LONG" ? (markPrice - entryPrice) / entryPrice : (entryPrice - markPrice) / entryPrice;
  return pct * notional;
}

function calculateReturnOnMargin(unrealizedPnL: number, marginUsed: number): number {
  return marginUsed > 0 ? (unrealizedPnL / marginUsed) * 100 : 0;
}

function applyMarkToPosition(
  p: BTCFuturesPosition,
  markPrice: number,
  lastPrice: number,
  fundingRate: number,
): BTCFuturesPosition {
  const unrealizedPnL = calculateUnrealizedPnL(p.entryPrice, markPrice, p.notional, p.side);
  const returnPct = calculateReturnOnMargin(unrealizedPnL, p.marginUsed);
  const unrealizedPnLPct = p.notional > 0 ? (unrealizedPnL / p.notional) * 100 : 0;
  const fundingCost = p.notional * fundingRate;
  return {
    ...p,
    markPrice,
    lastPrice,
    unrealizedPnl: unrealizedPnL,
    unrealizedPnlPct: unrealizedPnLPct,
    returnPct,
    fundingCosts: p.fundingCosts + fundingCost,
  };
}

function chunkArray<T>(arr: readonly T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < arr.length; i += size) {
    out.push([...arr.slice(i, i + size)]);
  }
  return out;
}

// ========== SIGNAL EVALUATION ==========
function buildSignalInputs(
  closes: number[],
  highs: number[],
  lows: number[],
  volumes: number[],
  markPrice: number,
): SignalInputs {
  const fast = ema(closes, 9);
  const slow = ema(closes, 21);
  const mean20 = sma(closes, 20);
  const std20 = stdDev(closes, 20);
  const rsi14 = rsi(closes, 14);
  const rsi7 = rsi(closes, 7);
  const rsi21 = rsi(closes, 21);
  const stochVals = stochastic(highs, lows, closes, 14, 3);
  const macdVals = macd(closes);
  const atr14 = atr(highs, lows, closes, 14);
  const obvSlopeVals = slope(obv(closes, volumes), 5);
  const bbUpper = mean20.map((m, i) => m + 2 * std20[i]);
  const bbLower = mean20.map((m, i) => m - 2 * std20[i]);
  const bbWidth = mean20.map((m, i) => m > 0 ? (4 * std20[i]) / m : 0);
  const williamsR_vals = williamsR(highs, lows, closes);
  const cci20 = cci(highs, lows, closes, 20);
  const roc10 = rateOfChange(closes, 10);
  const keltnerVals = keltner(highs, lows, closes);
  const donchianVals = donchian(highs, lows);
  const vwap = sma(closes.map((c, i) => c * volumes[i]), 20).map((s, i) => volumes[i] > 0 ? s / volumes[i] : 0);
  const vwapDev = closes.map((c, i) => c - vwap[i]);
  const adxVals = adxProxy(highs, lows, closes);
  const ema5 = ema(closes, 5);
  const ema13 = ema(closes, 13);

  const idx = closes.length - 1;
  const htf5 = buildHtfFields(closes, highs, lows, volumes, 5);
  const htf15 = buildHtfFields(closes, highs, lows, volumes, 15);

  return {
    price: closes[idx],
    markPrice,
    prevPrice: closes[idx - 1] ?? closes[idx],
    fast: fast[idx],
    slow: slow[idx],
    trend: fast[idx] - slow[idx],
    prevFast: fast[idx - 1] ?? fast[idx],
    prevSlow: slow[idx - 1] ?? slow[idx],
    mean20: mean20[idx],
    std20: std20[idx],
    rsi14: rsi14[idx],
    high20: Math.max(...closes.slice(-20)),
    low20: Math.min(...closes.slice(-20)),
    momentum3: closes[idx] - (closes[idx - 3] ?? closes[idx]),
    momentum6: closes[idx] - (closes[idx - 6] ?? closes[idx]),
    momentum10: closes[idx] - (closes[idx - 10] ?? closes[idx]),
    volRatio: volumes[idx] / (sma(volumes, 20)[idx] || volumes[idx] || 1),
    bbUpper: bbUpper[idx],
    bbLower: bbLower[idx],
    bbWidth: bbWidth[idx],
    stochK: stochVals.k[idx],
    stochD: stochVals.d[idx],
    prevStochK: stochVals.k[idx - 1] ?? stochVals.k[idx],
    prevStochD: stochVals.d[idx - 1] ?? stochVals.d[idx],
    macdLine: macdVals.line[idx],
    macdSignal: macdVals.signal[idx],
    prevMacdLine: macdVals.line[idx - 1] ?? macdVals.line[idx],
    prevMacdSignal: macdVals.signal[idx - 1] ?? macdVals.signal[idx],
    macdHist: macdVals.hist[idx],
    prevMacdHist: macdVals.hist[idx - 1] ?? macdVals.hist[idx],
    atr14: atr14[idx],
    obvSlope: obvSlopeVals[idx],
    williamsR: williamsR_vals[idx],
    prevWilliamsR: williamsR_vals[idx - 1] ?? williamsR_vals[idx],
    cci20: cci20[idx],
    roc10: roc10[idx],
    keltnerUpper: keltnerVals.upper[idx],
    keltnerLower: keltnerVals.lower[idx],
    donchianHigh: donchianVals.upper[idx],
    donchianLow: donchianVals.lower[idx],
    donchianMid: donchianVals.mid[idx],
    vwapDev: vwapDev[idx],
    adxProxy: adxVals[idx],
    ema5: ema5[idx],
    ema13: ema13[idx],
    prevEma5: ema5[idx - 1] ?? ema5[idx],
    prevEma13: ema13[idx - 1] ?? ema13[idx],
    rsi7: rsi7[idx],
    rsi21: rsi21[idx],
    ...htf5,
    ...htf15,
  };
}

function buildHtfFields(
  closes: number[],
  highs: number[],
  lows: number[],
  volumes: number[],
  period: number,
): {
  htf5_fast: number;
  htf5_slow: number;
  htf5_rsi: number;
  htf5_momentum: number;
  htf5_trend: number;
  htf5_macdHist: number;
  htf5_bbWidth: number;
  htf5_adx: number;
  htf15_fast: number;
  htf15_slow: number;
  htf15_rsi: number;
  htf15_momentum: number;
  htf15_trend: number;
  htf15_macdHist: number;
  htf15_bbWidth: number;
  htf15_adx: number;
} {
  const aggClose = aggregateBars(closes, period);
  const aggHigh = aggregateBars(highs, period);
  const aggLow = aggregateBars(lows, period);
  const aggVol = aggregateBars(volumes, period);

  const fast = ema(aggClose, 9);
  const slow = ema(aggClose, 21);
  const rsi = (() => {
    const out: number[] = [];
    let gainSum = 0;
    let lossSum = 0;
    for (let i = 1; i < aggClose.length; i++) {
      const change = aggClose[i] - aggClose[i - 1];
      const gain = Math.max(change, 0);
      const loss = Math.max(-change, 0);
      if (i < 14) { gainSum += gain; lossSum += loss; out.push(NaN); }
      else if (i === 14) { gainSum += gain; lossSum += loss; const avgGain = gainSum / 14; const avgLoss = lossSum / 14; const rs = avgLoss === 0 ? 0 : avgGain / avgLoss; out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs)); }
      else { const prevAvgGain = (gainSum / 14) * 13 + gain; const prevAvgLoss = (lossSum / 14) * 13 + loss; gainSum = prevAvgGain; lossSum = prevAvgLoss; const avgGain = gainSum / 14; const avgLoss = lossSum / 14; const rs = avgLoss === 0 ? 0 : avgGain / avgLoss; out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs)); }
    }
    return out;
  })();
  const mom = aggClose.map((c, i) => i >= 3 ? c - aggClose[i - 3] : 0);
  const macdVals = macd(aggClose);
  const mean20 = sma(aggClose, 20);
  const std20 = stdDev(aggClose, 20);
  const bbWidth = mean20.map((m, i) => m > 0 ? (4 * std20[i]) / m : 0);
  const adxVals = adxProxy(aggHigh, aggLow, aggClose);

  const idx = aggClose.length - 1;
  const prefix = period === 5 ? "htf5" : "htf15";

  const result = {
    htf5_fast: 0, htf5_slow: 0, htf5_rsi: 0, htf5_momentum: 0, htf5_trend: 0,
    htf5_macdHist: 0, htf5_bbWidth: 0, htf5_adx: 0,
    htf15_fast: 0, htf15_slow: 0, htf15_rsi: 0, htf15_momentum: 0, htf15_trend: 0,
    htf15_macdHist: 0, htf15_bbWidth: 0, htf15_adx: 0,
  };

  if (period === 5) {
    result.htf5_fast = fast[idx];
    result.htf5_slow = slow[idx];
    result.htf5_rsi = rsi[idx];
    result.htf5_momentum = mom[idx];
    result.htf5_trend = htfTrend(fast[idx], slow[idx], mom[idx]) === "UP" ? 1 : htfTrend(fast[idx], slow[idx], mom[idx]) === "DOWN" ? -1 : 0;
    result.htf5_macdHist = macdVals.hist[idx];
    result.htf5_bbWidth = bbWidth[idx];
    result.htf5_adx = adxVals[idx];
  } else {
    result.htf15_fast = fast[idx];
    result.htf15_slow = slow[idx];
    result.htf15_rsi = rsi[idx];
    result.htf15_momentum = mom[idx];
    result.htf15_trend = htfTrend(fast[idx], slow[idx], mom[idx]) === "UP" ? 1 : htfTrend(fast[idx], slow[idx], mom[idx]) === "DOWN" ? -1 : 0;
    result.htf15_macdHist = macdVals.hist[idx];
    result.htf15_bbWidth = bbWidth[idx];
    result.htf15_adx = adxVals[idx];
  }

  return result;
}

function evalMinuteSignal(s: SignalInputs, strat: StratDef): { score: number; reason: string } {
  let score = 0;
  const reasons: string[] = [];

  const add = (pts: number, desc: string) => { score += pts; if (pts > 0) reasons.push(desc); };
  /** Short strategies must earn bearish points; legacy scoring was almost entirely long-biased. */
  const short = strat.signalKey.includes("SHORT");

  // Category signals
  if (strat.category === "Trend" || strat.category === "MTF Trend") {
    if (short) {
      if (s.fast < s.slow && s.momentum3 < 0) add(10, "EMA bearish");
      if (s.fast < s.slow && s.momentum6 < 0) add(8, "6m down");
      if (s.rsi14 > 25 && s.rsi14 < 45) add(6, "RSI bear zone");
      if (s.adxProxy > 25) add(8, "Trending(ADX)");
      if (s.roc10 < -0.5) add(6, "ROC negative");
    } else {
      if (s.fast > s.slow && s.momentum3 > 0) add(10, "EMA bullish");
      if (s.fast > s.slow && s.momentum6 > 0) add(8, "6m momentum");
      if (s.rsi14 > 55 && s.rsi14 < 75) add(6, "RSI strong");
      if (s.adxProxy > 25) add(8, "Trending(ADX)");
      if (s.roc10 > 0.5) add(6, "ROC positive");
    }
  }

  if (strat.category === "MeanRev" || strat.category === "MR") {
    if (short) {
      if (s.price > s.bbUpper) add(12, "BB upper breach");
      if (s.rsi14 > 65) add(10, "RSI overbought");
      if (s.stochK > 80) add(8, "Stoch overbought");
      if (s.vwapDev > 0.015 * s.price) add(6, "VWAP+ deviation");
    } else {
      if (s.price < s.bbLower) add(12, "BB lower breach");
      if (s.rsi14 < 35) add(10, "RSI oversold");
      if (s.stochK < 20) add(8, "Stoch oversold");
      if (s.vwapDev < -0.015 * s.price) add(6, "VWAP deviation");
    }
  }

  if (strat.category === "Momentum") {
    if (Math.abs(s.momentum3) > s.atr14 * 0.8) add(10, "Strong 3m momentum");
    if (short) {
      if (s.obvSlope < 0) add(8, "OBV falling");
      if (s.macdHist < 0 && s.prevMacdHist < 0 && s.macdHist < s.prevMacdHist) add(8, "MACD accel down");
    } else {
      if (s.obvSlope > 0) add(8, "OBV rising");
      if (s.macdHist > 0 && s.prevMacdHist > 0 && s.macdHist > s.prevMacdHist) add(8, "MACD accel");
    }
  }

  if (strat.category === "RSI") {
    if (short) {
      if (s.rsi14 > 68) add(12, "RSI extreme high");
      if (s.rsi7 > s.rsi14 + 5) add(8, "RSI7 bear div");
    } else {
      if (s.rsi14 < 32) add(12, "RSI extreme low");
      if (s.rsi7 < s.rsi14 - 5) add(8, "RSI7 divergence");
    }
  }

  if (strat.category === "Stoch") {
    if (short) {
      if (s.stochK > 80 && s.stochD > 80) add(12, "Stoch overbought");
      if (s.stochK < s.stochD && s.prevStochK >= s.prevStochD) add(10, "Stoch cross down");
    } else {
      if (s.stochK < 20 && s.stochD < 20) add(12, "Stoch oversold");
      if (s.stochK > s.stochD && s.prevStochK <= s.prevStochD) add(10, "Stoch cross up");
    }
  }

  if (strat.category === "MACD") {
    if (short) {
      if (s.macdLine < s.macdSignal && s.prevMacdLine >= s.prevMacdSignal) add(12, "MACD cross down");
      if (s.macdHist < 0 && s.macdHist < s.prevMacdHist) add(8, "MACD falling");
    } else {
      if (s.macdLine > s.macdSignal && s.prevMacdLine <= s.prevMacdSignal) add(12, "MACD cross");
      if (s.macdHist > 0 && s.macdHist > s.prevMacdHist) add(8, "MACD rising");
    }
  }

  if (strat.category === "OBV") {
    if (short) {
      if (s.obvSlope < 0 && s.momentum3 < 0) add(10, "OBV+price down");
      if (s.obvSlope < -Math.abs(s.atr14) * 10) add(8, "OBV strong down");
    } else {
      if (s.obvSlope > 0 && s.momentum3 > 0) add(10, "OBV+price up");
      if (s.obvSlope > s.atr14 * 10) add(8, "OBV strong");
    }
  }

  if (strat.category === "Confluence") {
    const checks = short
      ? [
          s.fast < s.slow,
          s.rsi14 > 30 && s.rsi14 < 50,
          s.macdLine < s.macdSignal,
          s.stochK < s.stochD,
          s.momentum3 < 0,
        ]
      : [
          s.fast > s.slow,
          s.rsi14 > 50 && s.rsi14 < 70,
          s.macdLine > s.macdSignal,
          s.stochK > s.stochD,
          s.momentum3 > 0,
        ];
    const passed = checks.filter(Boolean).length;
    if (passed >= 3) add(10 + passed * 2, `Confluence(${passed})`);
  }

  if (strat.category === "Vol") {
    if (s.volRatio > 1.5) add(10, "Volume spike");
    if (short) {
      if (s.bbWidth < 0.015 && s.momentum3 < -s.atr14) add(12, "Squeeze breakdown");
    } else {
      if (s.bbWidth < 0.015 && s.momentum3 > s.atr14) add(12, "Squeeze breakout");
    }
  }

  if (strat.category === "Breakout" || strat.category === "MTF Break") {
    if (short) {
      if (s.price < s.low20) add(14, "Range breakdown");
      if (s.volRatio > 1.4) add(10, "Breakout volume");
      if (s.adxProxy > 22) add(8, "Trend strength");
      if (s.momentum3 < 0) add(8, "Breakout momentum");
    } else {
      if (s.price > s.high20) add(14, "Range breakout");
      if (s.volRatio > 1.4) add(10, "Breakout volume");
      if (s.adxProxy > 22) add(8, "Trend strength");
      if (s.momentum3 > 0) add(8, "Breakout momentum");
    }
  }

  if (strat.category === "BB") {
    if (s.price < s.bbLower) add(12, "BB lower");
    if (s.price > s.bbUpper) add(10, "BB upper");
    if (s.bbWidth < 0.01) add(8, "BB squeeze");
  }

  if (strat.category === "Williams MR") {
    if (s.williamsR < -80) add(12, "Williams oversold");
    if (s.williamsR > -20) add(10, "Williams overbought");
    if (s.williamsR > -50 && s.prevWilliamsR <= -50) add(8, "Williams cross mid");
  }

  if (strat.category === "CCI MR") {
    if (s.cci20 < -100) add(12, "CCI oversold");
    if (s.cci20 > 100) add(10, "CCI overbought");
    if (Math.abs(s.cci20) < 50) add(6, "CCI neutral");
  }

  if (strat.category === "Williams Trend") {
    if (short) {
      if (s.williamsR > -35 && s.momentum3 < 0) add(12, "Williams bear trend");
      if (s.williamsR > -22) add(10, "Williams elevated");
      if (s.williamsR < s.prevWilliamsR && s.williamsR > -45) add(8, "Williams roll from OB");
    } else {
      if (s.williamsR < -65 && s.momentum3 > 0) add(12, "Williams bull trend");
      if (s.williamsR < -80) add(10, "Williams deep OS");
      if (s.williamsR > s.prevWilliamsR && s.williamsR < -50) add(8, "Williams bounce");
    }
  }

  if (strat.category === "CCI Trend") {
    if (short) {
      if (s.cci20 > 40 && s.momentum3 < 0) add(12, "CCI bear trend");
      if (s.cci20 > 120) add(10, "CCI extended high");
    } else {
      if (s.cci20 < -40 && s.momentum3 > 0) add(12, "CCI bull trend");
      if (s.cci20 < -120) add(10, "CCI extended low");
    }
  }

  if (strat.category === "Keltner MR" || strat.category === "Keltner Trend") {
    if (s.price > s.keltnerUpper) add(10, "Keltner upper breach");
    if (s.price < s.keltnerLower) add(10, "Keltner lower breach");
  }

  if (strat.category === "Donchian Trend" || strat.category === "Donchian MR") {
    if (s.price > s.donchianHigh) add(12, "Donchian high break");
    if (s.price < s.donchianLow) add(12, "Donchian low break");
  }

  if (strat.category === "Ribbon") {
    if (short) {
      if (s.ema5 < s.ema13 && s.prevEma5 >= s.prevEma13) add(12, "Ribbon cross down");
      if (s.ema5 < s.ema13 && s.fast < s.slow) add(8, "Ribbon bear aligned");
    } else {
      if (s.ema5 > s.ema13 && s.prevEma5 <= s.prevEma13) add(12, "Ribbon cross");
      if (s.ema5 > s.ema13 && s.fast > s.slow) add(8, "Ribbon aligned");
    }
  }

  if (strat.category === "Squeeze") {
    if (s.bbWidth < 0.01 && s.adxProxy > 20) add(12, "Squeeze + ADX");
    if (s.volRatio > 2) add(10, "Vol spike after squeeze");
  }

  if (strat.category === "ADX Trend") {
    if (s.adxProxy > 30) add(12, "Strong trend");
    if (short) {
      if (s.adxProxy > 25 && s.fast < s.slow) add(10, "ADX + EMA bear");
    } else {
      if (s.adxProxy > 25 && s.fast > s.slow) add(10, "ADX + EMA");
    }
  }

  if (strat.category === "ROC Trend") {
    if (s.roc10 > 1) add(10, "ROC strong up");
    if (s.roc10 < -1) add(10, "ROC strong down");
  }

  if (strat.category === "MTF Trend") {
    if (short) {
      if (s.fast < s.slow) add(10, "LTF trend down");
      if (s.momentum6 < 0) add(8, "LTF momentum down");
      if (s.adxProxy > 20) add(6, "LTF ADX");
    } else {
      if (s.fast > s.slow) add(10, "LTF trend up");
      if (s.momentum6 > 0) add(8, "LTF momentum up");
      if (s.adxProxy > 20) add(6, "LTF ADX");
    }
  }

  if (strat.category === "MTF MACD") {
    if (short) {
      if (s.macdLine < s.macdSignal) add(10, "LTF MACD bear");
      if (s.macdHist < 0) add(8, "LTF hist below zero");
      if (s.htf5_macdHist < 0) add(8, "HTF5 MACD bear");
      if (s.htf15_macdHist < 0) add(6, "HTF15 MACD bear");
    } else {
      if (s.macdLine > s.macdSignal) add(10, "LTF MACD bull");
      if (s.macdHist > 0) add(8, "LTF hist above zero");
      if (s.htf5_macdHist > 0) add(8, "HTF5 MACD bull");
      if (s.htf15_macdHist > 0) add(6, "HTF15 MACD bull");
    }
  }

  if (strat.category === "MTF ADX") {
    if (short) {
      if (s.adxProxy > 22 && s.fast < s.slow) add(10, "LTF ADX bear");
      if (s.htf5_adx > 24) add(8, "HTF5 ADX");
      if (s.htf15_adx > 24) add(8, "HTF15 ADX");
      if (s.htf5_trend < 0 && s.htf15_trend < 0) add(10, "HTF trend down");
    } else {
      if (s.adxProxy > 22 && s.fast > s.slow) add(10, "LTF ADX bull");
      if (s.htf5_adx > 24) add(8, "HTF5 ADX");
      if (s.htf15_adx > 24) add(8, "HTF15 ADX");
      if (s.htf5_trend > 0 && s.htf15_trend > 0) add(10, "HTF trend up");
    }
  }

  // MTF signals
  if (strat.requiresHtf) {
    const htf5Trend = htfTrend(s.htf5_fast, s.htf5_slow, s.htf5_momentum);
    const htf15Trend = htfTrend(s.htf15_fast, s.htf15_slow, s.htf15_momentum);

    if (htf5Trend === "UP" && htf15Trend === "UP") add(14, "HTF aligned up");
    if (htf5Trend === "DOWN" && htf15Trend === "DOWN") add(14, "HTF aligned down");
    if (short) {
      if (s.htf5_rsi > 30 && s.htf5_rsi < 50) add(8, "HTF RSI bear zone");
      if (s.htf15_rsi > 30 && s.htf15_rsi < 50) add(6, "HTF15 RSI bear zone");
      if (s.htf5_macdHist < 0) add(8, "HTF MACD bear");
      if (s.htf15_macdHist < 0) add(6, "HTF15 MACD bear");
    } else {
      if (s.htf5_rsi > 50 && s.htf5_rsi < 70) add(8, "HTF RSI healthy");
      if (s.htf15_rsi > 50 && s.htf15_rsi < 70) add(6, "HTF15 RSI healthy");
      if (s.htf5_macdHist > 0) add(8, "HTF MACD bullish");
      if (s.htf15_macdHist > 0) add(6, "HTF15 MACD bullish");
    }
    if (s.htf5_adx > 25) add(8, "HTF trending");
  }

  // Smart Money & Order Flow (Pro Grade)
  if (strat.category === "Smart Money") {
    if (short) {
      if (s.volRatio > 2 && s.price > s.mean20) add(14, "Distribution vol");
      if (s.obvSlope < 0 && s.momentum3 < 0) add(12, "Smart money out");
    } else {
      if (s.volRatio > 2 && s.price < s.mean20) add(14, "Accumulation vol");
      if (s.obvSlope > 0 && s.momentum3 > 0) add(12, "Smart money in");
    }
  }

  if (strat.category === "Order Flow") {
    if (short) {
      if (s.momentum3 < -s.atr14 * 1.5) add(14, "Strong flow down");
      if (s.volRatio > 1.8 && s.momentum3 < 0) add(12, "Sell flow");
    } else {
      if (s.momentum3 > s.atr14 * 1.5) add(14, "Strong flow");
      if (s.volRatio > 1.8 && s.price > s.vwapDev + s.mean20) add(12, "Buy flow");
    }
  }

  if (strat.category === "Liquidity") {
    if (short) {
      if (s.price > s.high20 * 0.998) add(14, "Liquidity at highs");
      if (s.williamsR > -15) add(12, "Overbought liquidity");
    } else {
      if (s.price < s.low20 * 1.002) add(14, "Liquidity sweep");
      if (s.williamsR < -85) add(12, "Oversold liquidity");
    }
  }

  if (strat.category === "Stop Hunt") {
    if (short) {
      if (Math.abs(s.price - s.high20) < s.atr14 * 0.3) add(14, "Stop hunt highs");
    } else {
      if (Math.abs(s.price - s.low20) < s.atr14 * 0.3) add(14, "Stop hunt zone");
    }
  }

  // Wyckoff & Market Structure
  if (strat.category === "Wyckoff") {
    if (short) {
      if (s.price < s.donchianMid && s.volRatio > 1.5) add(14, "Wyckoff markdown");
      if (s.cci20 < -100 && s.momentum6 < 0) add(12, "Distribution leg");
    } else {
      if (s.price > s.donchianMid && s.volRatio > 1.5) add(14, "Wyckoff markup");
      if (s.cci20 > 100 && s.momentum6 > 0) add(12, "Spring complete");
    }
  }

  if (strat.category === "Market Structure") {
    if (s.price > s.high20 && s.htf5_trend > 0) add(14, "BOS bullish");
    if (s.price < s.low20 && s.htf5_trend < 0) add(14, "BOS bearish");
  }

  // Statistical & Institutional
  if (strat.category === "Statistical") {
    const zscore = (s.price - s.mean20) / (s.std20 || 1);
    if (Math.abs(zscore) > 2) add(14, "Statistical extreme");
    if (short) {
      if (zscore > 1.5 && s.rsi14 > 60) add(12, "Mean reversion short");
    } else {
      if (zscore < -1.5 && s.rsi14 < 40) add(12, "Mean reversion long");
    }
  }

  if (strat.category === "Institutional") {
    if (s.adxProxy > 30 && s.volRatio > 2) add(14, "Inst activity");
    if (s.htf5_adx > 25 && s.htf15_adx > 25) add(12, "Inst trend");
  }

  if (strat.category === "Session") {
    if (short) {
      if (s.momentum3 < 0 && s.volRatio > 1.5) add(12, "Session sell");
    } else {
      if (s.momentum3 > 0 && s.volRatio > 1.5) add(12, "Session momentum");
    }
  }

  // Harmonic & Patterns
  if (strat.category === "Harmonic") {
    if (Math.abs(s.price - s.bbLower) / s.price < 0.005) add(12, "Harmonic support");
    if (s.rsi14 > 30 && s.rsi14 < 50 && s.stochK > s.stochD) add(10, "Harmonic bounce");
  }

  if (strat.category === "Chart Pattern") {
    if (s.bbWidth < 0.015 && s.volRatio > 1.5) add(12, "Pattern breakout");
    if (s.atr14 < s.mean20 * 0.008) add(10, "Consolidation pattern");
  }

  // Greek-Inspired (adapted for futures)
  if (strat.category === "Greek-Inspired") {
    if (s.bbWidth < 0.012 && s.adxProxy > 20) add(14, "Gamma squeeze setup");
    if (s.volRatio > 3 && Math.abs(s.momentum3) > s.atr14) add(12, "Delta spike");
  }

  // Event & Macro
  if (strat.category === "Event") {
    if (s.volRatio > 4 && Math.abs(s.momentum3) > s.atr14 * 2) add(16, "Event volatility");
  }

  if (strat.category === "Macro") {
    if (s.htf5_trend > 0 && s.htf15_trend > 0 && s.rsi14 > 50) add(14, "Risk on");
    if (s.htf5_trend < 0 && s.htf15_trend < 0 && s.rsi14 < 50) add(14, "Risk off");
  }

  // ML-Style (multi-factor ensemble)
  if (strat.category === "ML-Style") {
    const factors = (
      short
        ? [
            s.fast < s.slow,
            s.rsi14 > 25 && s.rsi14 < 55,
            s.macdHist < 0,
            s.adxProxy > 20,
            s.volRatio > 1.2,
          ]
        : [
            s.fast > s.slow,
            s.rsi14 > 45 && s.rsi14 < 75,
            s.macdHist > 0,
            s.adxProxy > 20,
            s.volRatio > 1.2,
          ]
    ).filter(Boolean).length;
    if (factors >= 4) add(16, "ML ensemble strong");
    if (factors === 3) add(10, "ML ensemble medium");
  }

  return { score, reason: reasons.slice(0, 3).join(", ") };
}

function isCategoryAligned(signalKey: string, category: string): boolean {
  const map: Record<string, string[]> = {
    Trend: ["EMA_CROSS", "TREND_CONT", "ADX_MOM", "TREND"],
    MeanRev: ["BB_MEANREV", "MEAN_REV", "MR"],
    Momentum: ["MOM_SURGE", "MOM"],
    RSI: ["RSI_DIP", "RSI_SPIKE", "RSI"],
    Stoch: ["STOCH"],
    MACD: ["MACD"],
    OBV: ["OBV"],
    Confluence: ["CONF", "MULTI_CONF"],
    Vol: ["VOL", "ATR"],
    BB: ["BB"],
    "Williams MR": ["WILLIAMS"],
    "CCI MR": ["CCI"],
    "Keltner MR": ["KELTNER"],
    "VWAP MR": ["VWAP"],
    "RSI Multi": ["RSI"],
    Exhaustion: ["EXHAUSTION"],
    "ADX Trend": ["ADX"],
    "Donchian Trend": ["DONCHIAN"],
    "ROC Trend": ["ROC"],
    Squeeze: ["SQUEEZE"],
    Ribbon: ["RIBBON", "EMA_RIBBON"],
    "Smart Money": ["SM_", "SMART"],
    "Order Flow": ["OF_", "ORDER"],
    Liquidity: ["LIQ_", "LIQUIDITY"],
    "Stop Hunt": ["STOP_HUNT", "HUNT"],
    Wyckoff: ["WYCKOFF"],
    "Vol Profile": ["VP_", "VOL_PROFILE"],
    "Market Structure": ["MS_", "CHOCH", "MARKET_STRUCTURE"],
    Institutional: ["INST_", "INSTITUTIONAL"],
    Session: ["OPEN", "CLOSE", "SESSION"],
    Statistical: ["STAT_", "REG_", "STATISTICAL"],
    Divergence: ["DIV", "DIVERGENCE"],
    Harmonic: ["HARM_", "HARMONIC"],
    "Chart Pattern": ["PATTERN", "FLAG", "PENNANT"],
    "Greek-Inspired": ["DELTA_", "GAMMA_", "GREEK"],
    Event: ["EVENT", "POST_EVENT"],
    "ML-Style": ["ML_", "ENSEMBLE", "CLASSIFIER"],
    Macro: ["RISK_ON", "RISK_OFF", "MACRO"],
  };
  const prefixes = map[category] || [category];
  return prefixes.some(p => signalKey.includes(p));
}

function passesEntryConfirmation(s: SignalInputs, strat: StratDef): boolean {
  const isShort = strat.signalKey.includes("SHORT");

  const confluence = (
    isShort
      ? [
          s.fast < s.slow,
          s.rsi14 > 25 && s.rsi14 < 55,
          s.macdLine < s.macdSignal,
          s.stochK < s.stochD,
          s.momentum3 < 0,
          s.obvSlope < 0,
          s.atr14 > 0,
          s.bbWidth > 0,
        ]
      : [
          s.fast > s.slow,
          s.rsi14 > 45 && s.rsi14 < 75,
          s.macdLine > s.macdSignal,
          s.stochK > s.stochD,
          s.momentum3 > 0,
          s.obvSlope > 0,
          s.atr14 > 0,
          s.bbWidth > 0,
        ]
  ).filter(Boolean).length;

  if (strat.category === "Trend" || strat.category === "MTF Trend") {
    if (isShort) {
      if (s.fast >= s.slow) return false;
      if (s.rsi14 < 20 || s.rsi14 > 65) return false;
    } else {
      if (s.fast <= s.slow) return false;
      if (s.rsi14 < 40 || s.rsi14 > 80) return false;
    }
  }

  if (strat.category === "MeanRev" || strat.category === "MR") {
    if (isShort) {
      if (s.rsi14 < 55) return false;
      if (s.price < s.bbUpper && s.price < s.mean20) return false;
    } else {
      if (s.rsi14 > 45) return false;
      if (s.price > s.bbLower && s.price > s.mean20) return false;
    }
  }

  if (strat.category === "Williams MR") {
    if (isShort) {
      if (s.williamsR < -25) return false;
    } else {
      if (s.williamsR > -70) return false;
    }
  }

  if (strat.category === "CCI MR") {
    if (isShort) {
      if (s.cci20 < 80) return false;
    } else {
      if (s.cci20 > -80) return false;
    }
  }

  if (strat.category === "Keltner MR") {
    if (isShort) {
      if (s.price < s.keltnerUpper) return false;
    } else {
      if (s.price > s.keltnerLower) return false;
    }
  }

  if (strat.category === "VWAP MR") {
    if (isShort) {
      if (s.vwapDev < 0.01 * s.price) return false;
    } else {
      if (s.vwapDev > -0.01 * s.price) return false;
    }
  }

  if (strat.category === "ADX Trend") {
    if (s.adxProxy < 20) return false;
  }

  if (strat.category === "Donchian Trend") {
    if (isShort) {
      if (s.price > s.donchianLow * 1.01) return false;
    } else {
      if (s.price < s.donchianHigh * 0.99) return false;
    }
  }

  if (strat.category === "ROC Trend") {
    if (isShort) {
      if (s.roc10 > -0.3) return false;
    } else {
      if (s.roc10 < 0.3) return false;
    }
  }

  if (strat.category === "Squeeze") {
    if (s.bbWidth > 0.02) return false;
  }

  if (strat.requiresHtf) {
    const htf5Trend = htfTrend(s.htf5_fast, s.htf5_slow, s.htf5_momentum);
    const htf15Trend = htfTrend(s.htf15_fast, s.htf15_slow, s.htf15_momentum);
    const ltfBull = s.fast > s.slow && s.momentum3 > 0;
    const ltfBear = s.fast < s.slow && s.momentum3 < 0;
    if (isShort) {
      if (htf5Trend === "UP" || htf15Trend === "UP") return false;
      if (htf5Trend === "DOWN" && htf15Trend === "DOWN") {
        /* strict bearish HTF */
      } else if (!ltfBear) {
        return false;
      }
    } else {
      if (htf5Trend === "DOWN" || htf15Trend === "DOWN") return false;
      if (htf5Trend === "UP" && htf15Trend === "UP") {
        /* strict bullish HTF */
      } else if (!ltfBull) {
        return false;
      }
    }
  }

  // Pro Grade category validations
  if (strat.category === "Smart Money") {
    if (s.volRatio < 1.5) return false;
    if (isShort) {
      if (s.obvSlope >= 0) return false;
    } else {
      if (s.obvSlope <= 0) return false;
    }
  }

  if (strat.category === "Order Flow") {
    if (isShort) {
      if (s.momentum3 >= -s.atr14) return false;
    } else {
      if (s.momentum3 <= s.atr14) return false;
    }
    if (s.volRatio < 1.5) return false;
  }

  if (strat.category === "Liquidity") {
    if (isShort) {
      if (s.price < s.high20 * 0.995) return false;
    } else {
      if (s.price > s.low20 * 1.005) return false;
    }
  }

  if (strat.category === "Stop Hunt") {
    if (isShort) {
      if (Math.abs(s.price - s.high20) > s.atr14 * 0.5) return false;
    } else {
      if (Math.abs(s.price - s.low20) > s.atr14 * 0.5) return false;
    }
  }

  if (strat.category === "Wyckoff") {
    if (isShort) {
      if (s.cci20 > -80) return false;
      if (s.volRatio < 1.3) return false;
    } else {
      if (s.cci20 < 80) return false;
      if (s.volRatio < 1.3) return false;
    }
  }

  if (strat.category === "Market Structure") {
    if (isShort) {
      if (s.price > s.low20 * 1.002 && s.price < s.high20 * 0.998) return false;
    } else {
      if (s.price < s.high20 * 0.998 && s.price > s.low20 * 1.002) return false;
    }
  }

  if (strat.category === "Statistical") {
    const zscore = Math.abs((s.price - s.mean20) / (s.std20 || 1));
    if (zscore < 1.5) return false;
  }

  if (strat.category === "Institutional") {
    if (s.adxProxy < 25) return false;
    if (s.volRatio < 1.5) return false;
  }

  if (strat.category === "Session") {
    if (s.volRatio < 1.3) return false;
  }

  if (strat.category === "Harmonic") {
    if (s.rsi14 > 60 || s.rsi14 < 20) return false;
  }

  if (strat.category === "Chart Pattern") {
    if (s.bbWidth > 0.02) return false;
  }

  if (strat.category === "Greek-Inspired") {
    if (s.bbWidth > 0.015) return false;
    if (s.adxProxy < 18) return false;
  }

  if (strat.category === "Event") {
    if (s.volRatio < 2.5) return false;
  }

  if (strat.category === "Macro") {
    if (s.htf5_rsi < 45 || s.htf5_rsi > 75) return false;
  }

  if (strat.category === "ML-Style") {
    const factors = (
      isShort
        ? [
            s.fast < s.slow,
            s.rsi14 > 25 && s.rsi14 < 55,
            s.macdHist < 0,
            s.adxProxy > 20,
          ]
        : [
            s.fast > s.slow,
            s.rsi14 > 45 && s.rsi14 < 75,
            s.macdHist > 0,
            s.adxProxy > 20,
          ]
    ).filter(Boolean).length;
    if (factors < 3) return false;
  }

  const strictConfluenceCategory =
    strat.category === "Confluence" || strat.category === "MTF Conf";
  const requiredHits = strictConfluenceCategory
    ? strat.confluenceMin
    : Math.max(2, strat.confluenceMin - 1);
  return confluence >= requiredHits;
}

// ========== HOOK ==========
export function useBTCFuturesScalperEngine(options: BTCFuturesEngineOptions = {}): {
  positions: BTCFuturesPosition[];
  trades: BTCFuturesTrade[];
  balance: number;
  equity: number;
  availableMargin: number;
  usedMargin: number;
  stats: BTCFuturesEngineStats;
  quote: EngineState["quote"];
  isReady: boolean;
  pauseEntries: boolean;
  disabledStrategies: number[];
  engineRef: React.MutableRefObject<EngineRef | null>;
  togglePause: () => void;
  resetPaperAccount: () => void;
  clearTradeHistory: () => void;
  setDisabledStrategies: (ids: number[]) => void;
  exportCSV: () => string;
  exportJSON: () => string;
  strategyStatuses: BTCFuturesStrategyStatus[];
} {
  const storageNamespace = options.storageNamespace?.trim() || "btc_futures_scalper";
  const strategyIds = options.strategyIds ?? null;
  const symbols = options.symbols ?? null;
  const activeSignalThreshold = Number.isFinite(options.signalThreshold)
    ? Math.max(1, Math.min(100, Number(options.signalThreshold)))
    : SIGNAL_THRESHOLD;
  const stateStorageKey = `${storageNamespace}_paper_state`;

  const activeStratDefs = useMemo(() => {
    if (!strategyIds || strategyIds.length === 0) return STRAT_DEFS;
    const allow = new Set(strategyIds);
    const filtered = STRAT_DEFS.filter((s) => allow.has(s.id));
    return filtered.length > 0 ? filtered : STRAT_DEFS;
  }, [strategyIds]);
  const activeStrategyIdSet = useMemo(
    () => new Set(activeStratDefs.map((s) => s.id)),
    [activeStratDefs],
  );
  const activeSymbols = useMemo(() => {
    if (!symbols || symbols.length === 0) return [...TRADING_SYMBOLS];
    const cleaned = symbols.map((s) => s.trim().toUpperCase()).filter(Boolean);
    return cleaned.length > 0 ? cleaned : [...TRADING_SYMBOLS];
  }, [symbols]);

  // State
  const [balance, setBalance] = useState(INITIAL_BALANCE);
  const [positions, setPositions] = useState<BTCFuturesPosition[]>([]);
  const [trades, setTrades] = useState<BTCFuturesTrade[]>([]);
  const [quote, setQuote] = useState<EngineState["quote"]>(null);
  const [status, setStatus] = useState<Status>("WARMING");
  const statusRef = useRef(status);
  useEffect(() => {
    statusRef.current = status;
  }, [status]);
  const [warmingPct, setWarmingPct] = useState(0);
  const [pauseEntries, setPauseEntries] = useState(false);
  const [disabledStrategies, setDisabledStrategies] = useState<number[]>([]);
  const [lastTradeAt, setLastTradeAt] = useState(0);
  const [dayStartBalance, setDayStartBalance] = useState(INITIAL_BALANCE);
  const [dayStartDate, setDayStartDate] = useState(() => new Date().getDate());

  // Refs
  const engineRef = useRef<EngineRef | null>(null);
  const positionsRef = useRef(positions);
  const tradesRef = useRef(trades);
  const balanceRef = useRef(balance);
  const disabledRef = useRef(disabledStrategies);
  const pauseRef = useRef(pauseEntries);
  const lastTradeAtRef = useRef(lastTradeAt);
  const stratCooldownsRef = useRef<Record<string, number>>({});
  const dayStartBalanceRef = useRef(dayStartBalance);
  const dayStartDateRef = useRef(dayStartDate);

  // Sync refs
  useEffect(() => { positionsRef.current = positions; }, [positions]);
  useEffect(() => { tradesRef.current = trades; }, [trades]);
  useEffect(() => { balanceRef.current = balance; }, [balance]);
  useEffect(() => { disabledRef.current = disabledStrategies; }, [disabledStrategies]);
  useEffect(() => { pauseRef.current = pauseEntries; }, [pauseEntries]);
  useEffect(() => { lastTradeAtRef.current = lastTradeAt; }, [lastTradeAt]);
  useEffect(() => { dayStartBalanceRef.current = dayStartBalance; }, [dayStartBalance]);
  useEffect(() => { dayStartDateRef.current = dayStartDate; }, [dayStartDate]);

  // ========== LOCAL STORAGE ==========
  const loadLs = useCallback((): Partial<EngineState> | null => {
    try {
      const raw = localStorage.getItem(stateStorageKey);
      if (!raw) return null;
      const parsed = JSON.parse(raw) as EngineState;
      return parsed;
    } catch {
      return null;
    }
  }, [stateStorageKey]);

  const saveLs = useCallback((state: EngineState) => {
    try {
      localStorage.setItem(stateStorageKey, JSON.stringify(state));
    } catch {
      // ignore
    }
  }, [stateStorageKey]);

  // Initial load
  useEffect(() => {
    const saved = loadLs();
    if (saved) {
      if (typeof saved.balance === "number") setBalance(saved.balance);
      if (Array.isArray(saved.positions)) {
        setPositions(
          saved.positions.map((p: BTCFuturesPosition) => ({
            ...p,
            symbol: p.symbol || PRIMARY_QUOTE_SYMBOL,
          })),
        );
      }
      if (Array.isArray(saved.trades)) {
        setTrades(
          saved.trades.slice(-MAX_TRADES).map((t: BTCFuturesTrade) => ({
            ...t,
            symbol: t.symbol || PRIMARY_QUOTE_SYMBOL,
          })),
        );
      }
      if (typeof saved.pauseEntries === "boolean") setPauseEntries(saved.pauseEntries);
      if (Array.isArray(saved.disabledStrategies)) {
        setDisabledStrategies(saved.disabledStrategies.filter((id) => activeStrategyIdSet.has(id)));
      }
      if (typeof saved.lastTradeAt === "number") setLastTradeAt(saved.lastTradeAt);
      if (typeof saved.dayStartBalance === "number") setDayStartBalance(saved.dayStartBalance);
      if (typeof saved.dayStartDate === "number") setDayStartDate(saved.dayStartDate);
    }
  }, [activeStrategyIdSet, loadLs]);

  // Periodic save
  useEffect(() => {
    const id = setInterval(() => {
      saveLs({
        balance,
        positions,
        trades: trades.slice(-MAX_TRADES),
        quote,
        status,
        warmingPct,
        disabledStrategies,
        pauseEntries,
        lastTradeAt,
        dayStartBalance,
        dayStartDate,
      });
    }, 30000);
    return () => clearInterval(id);
  }, [balance, positions, trades, quote, status, warmingPct, disabledStrategies, pauseEntries, lastTradeAt, dayStartBalance, dayStartDate, saveLs]);

  // ========== ACTIONS ==========
  const togglePause = useCallback(() => {
    setPauseEntries(p => !p);
  }, []);

  const resetPaperAccount = useCallback(() => {
    setBalance(INITIAL_BALANCE);
    setPositions([]);
    setTrades([]);
    setLastTradeAt(0);
    setDayStartBalance(INITIAL_BALANCE);
    setDayStartDate(new Date().getDate());
    stratCooldownsRef.current = {};
    localStorage.removeItem(stateStorageKey);
  }, [stateStorageKey]);

  const clearTradeHistory = useCallback(() => {
    setTrades([]);
    setLastTradeAt(0);
    stratCooldownsRef.current = {};
  }, []);

  const setDisabledStrategiesHandler = useCallback((ids: number[]) => {
    setDisabledStrategies(ids.filter((id) => activeStrategyIdSet.has(id)));
  }, [activeStrategyIdSet]);

  const exportCSV = useCallback(() => {
    const headers = ["ID", "Symbol", "Strategy", "Side", "Entry", "Exit", "Contracts", "Realized PnL", "Fees", "Net PnL", "Net PnL %", "Funding", "Opened", "Closed", "Exit Reason"];
    const rows = trades.map(t => [
      t.id,
      t.symbol || PRIMARY_QUOTE_SYMBOL,
      t.strategyName,
      t.side,
      t.entryPrice.toFixed(2),
      t.exitPrice.toFixed(2),
      t.contracts,
      t.realizedPnl.toFixed(2),
      t.fees.toFixed(4),
      t.netPnl.toFixed(2),
      t.netPnlPct.toFixed(2),
      t.fundingCosts.toFixed(4),
      t.openedAt,
      t.closedAt,
      t.exitReason,
    ]);
    return [headers.join(","), ...rows.map(r => r.join(","))].join("\n");
  }, [trades]);

  const exportJSON = useCallback(() => {
    return JSON.stringify({ balance, positions, trades, stats: calculateStats() }, null, 2);
  }, [balance, positions, trades]);

  // ========== STATS ==========
  const calculateStats = useCallback((): BTCFuturesEngineStats => {
    const wins = trades.filter(t => t.netPnl > 0);
    const losses = trades.filter(t => t.netPnl <= 0);
    const winCount = wins.length;
    const lossCount = losses.length;
    const totalTrades = trades.length;
    const winRate = totalTrades > 0 ? (winCount / totalTrades) * 100 : 0;
    const avgWin = winCount > 0 ? wins.reduce((s, t) => s + t.netPnl, 0) / winCount : 0;
    const avgLoss = lossCount > 0 ? losses.reduce((s, t) => s + t.netPnl, 0) / lossCount : 0;
    const profitFactor = avgLoss !== 0 ? Math.abs(avgWin / avgLoss) : 0;
    const realizedPnl = trades.reduce((s, t) => s + t.netPnl, 0);
    const unrealizedPnl = positions.reduce((s, p) => s + p.unrealizedPnl, 0);
    const totalFees = trades.reduce((s, t) => s + t.fees, 0);
    const totalFunding = trades.reduce((s, t) => s + t.fundingCosts, 0) + positions.reduce((s, p) => s + p.fundingCosts, 0);
    const netPnl = realizedPnl + unrealizedPnl;

    const peak = Math.max(INITIAL_BALANCE, ...trades.map((_, i) => INITIAL_BALANCE + trades.slice(0, i + 1).reduce((s, t) => s + t.netPnl, 0)));
    const currentEquity = balance + unrealizedPnl;
    const drawdown = peak > 0 ? ((peak - currentEquity) / peak) * 100 : 0;

    const usedMargin = positions.reduce((s, p) => s + p.marginUsed, 0);
    const availableMargin = balance - usedMargin;
    const marginUtilization = balance > 0 ? (usedMargin / balance) * 100 : 0;

    const longCount = positions.filter(p => p.side === "LONG").length;
    const shortCount = positions.filter(p => p.side === "SHORT").length;

    const liquidationRisk = positions.filter(p => {
      const dist = calculateDistanceToLiquidation(p.markPrice, p.liquidationPrice, p.side);
      return dist >= 0 && dist < LIQUIDATION_RISK_DISPLAY_PCT;
    }).length;

    const avgLeverage = positions.length > 0 ? positions.reduce((s, p) => s + p.leverage, 0) / positions.length : 0;

    return {
      totalTrades,
      winCount,
      lossCount,
      winRate,
      avgWin,
      avgLoss,
      profitFactor,
      realizedPnl,
      unrealizedPnl,
      totalFees,
      totalFundingCosts: totalFunding,
      netPnl,
      maxDrawdownPct: drawdown,
      currentDrawdownPct: drawdown,
      openPositions: positions.length,
      maxPositions: MAX_OPEN_POSITIONS,
      longCount,
      shortCount,
      balance,
      equity: currentEquity,
      availableMargin,
      usedMargin,
      marginUtilization,
      liquidationRisk,
      avgLeverage,
      dayStartBalance: dayStartBalanceRef.current,
    };
  }, [trades, positions, balance]);

  // ========== STRATEGY STATUSES ==========
  const strategyStatuses = useMemo((): BTCFuturesStrategyStatus[] => {
    const now = Date.now();
    return activeStratDefs.map(strat => {
      const openCount = positions.filter(p => p.strategyId === strat.id).length;
      const stratTrades = trades.filter(t => t.strategyId === strat.id);
      const lastTrade = stratTrades[stratTrades.length - 1];
      const inCooldown = Object.entries(stratCooldownsRef.current).some(
        ([key, until]) => key.endsWith(`:${strat.id}`) && (until ?? 0) > now,
      );
      const wins = stratTrades.filter(t => t.netPnl > 0).length;
      const losses = stratTrades.filter(t => t.netPnl <= 0).length;
      const totalPnl = stratTrades.reduce((s, t) => s + t.netPnl, 0);
      const winRate = stratTrades.length > 0 ? (wins / stratTrades.length) * 100 : 0;
      // Score: weighted combination of win rate and total PnL
      const score = stratTrades.length > 0
        ? Math.min(100, winRate * 0.7 + Math.min(30, Math.abs(totalPnl) / 100) * 0.3)
        : 0;

      return {
        id: strat.id,
        name: strat.name,
        category: strat.category,
        status: openCount > 0 ? "OPEN" : inCooldown ? "COOLING" : "AVAILABLE",
        disabled: disabledStrategies.includes(strat.id),
        openCount,
        lastTradeAt: lastTrade ? new Date(lastTrade.closedAt).getTime() : null,
        score,
        totalTrades: stratTrades.length,
        wins,
        losses,
        totalPnl,
        winRate,
      };
    });
  }, [positions, trades, disabledStrategies, activeStratDefs]);

  // ========== POSITION MANAGEMENT ==========
  const openPosition = useCallback((strat: StratDef, side: Side, price: number, markPrice: number, symbol: string) => {
    const id = `${symbol}-${strat.id}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const bal = balanceRef.current;
    const notional = Math.min(Math.max(bal * 0.1, 100), 500); // 10% of balance, min $100, max $500
    const contracts = Math.max(MIN_CONTRACTS, Math.min(MAX_CONTRACTS, calculateContracts(notional, price)));
    const actualNotional = calculateNotional(contracts);
    const marginUsed = calculateMarginRequired(actualNotional, LEVERAGE);
    if (bal < marginUsed) return;
    /** Keep ref in sync so multiple opens in one poll tick do not all read stale balance. */
    balanceRef.current = bal - marginUsed;
    const liquidationPrice = calculateLiquidationPrice(price, side, LEVERAGE);
    const slPrice = side === "LONG" ? price * (1 - strat.slPct / 100) : price * (1 + strat.slPct / 100);
    const tpPrice = side === "LONG" ? price * (1 + strat.tpPct / 100) : price * (1 - strat.tpPct / 100);

    const position: BTCFuturesPosition = {
      id,
      symbol,
      strategyId: strat.id,
      strategyName: strat.name,
      side,
      entryPrice: price,
      markPrice,
      lastPrice: price,
      contracts,
      notional: actualNotional,
      marginUsed,
      leverage: LEVERAGE,
      liquidationPrice,
      unrealizedPnl: 0,
      unrealizedPnlPct: 0,
      returnPct: 0,
      tpPrice,
      slPrice,
      fundingCosts: 0,
      openedAt: new Date().toISOString(),
      holdMinutes: strat.holdMinutes,
      marginMode: "isolated",
      adaptiveSl: slPrice,
      breakevenMoved: false,
      initialMargin: marginUsed,
    };

    setPositions(prev => [...prev, position]);
    setBalance(balanceRef.current);
    stratCooldownsRef.current[`${symbol}:${strat.id}`] = Date.now() + strat.cooldownMin * 60000;
    setLastTradeAt(Date.now());
  }, []);

  const closePosition = useCallback((position: BTCFuturesPosition, exitPrice: number, exitReason: BTCFuturesPosition["exitReason"]) => {
    const grossPnL = calculateUnrealizedPnL(position.entryPrice, exitPrice, position.notional, position.side);
    const fees = position.notional * TAKER_FEE_PCT * 2; // Entry + exit
    let netPnl = grossPnL - fees - position.fundingCosts;

    // Floor tiny wins only — do not inflate small losses to -MIN (was forcing -$2 on ~$2 margin → -100% return).
    if (netPnl > 0 && netPnl < MIN_ABS_NET_PNL_USD) {
      netPnl = MIN_ABS_NET_PNL_USD;
    }

    const netPnlPct = position.marginUsed > 0 ? (netPnl / position.marginUsed) * 100 : 0;
    const liqDist = calculateDistanceToLiquidation(exitPrice, position.liquidationPrice, position.side);

    const trade: BTCFuturesTrade = {
      id: position.id,
      symbol: position.symbol,
      strategyId: position.strategyId,
      strategyName: position.strategyName,
      side: position.side,
      entryPrice: position.entryPrice,
      exitPrice,
      contracts: position.contracts,
      notional: position.notional,
      marginUsed: position.marginUsed,
      realizedPnl: grossPnL,
      fees,
      netPnl,
      netPnlPct,
      fundingCosts: position.fundingCosts,
      openedAt: position.openedAt,
      closedAt: new Date().toISOString(),
      exitReason: exitReason!,
      liquidationPrice: position.liquidationPrice,
      liquidationDistancePct: liqDist,
    };

    setTrades(prev => [...prev.slice(-MAX_TRADES + 1), trade]);
    setPositions(prev => prev.filter(p => p.id !== position.id));
    setBalance(prev => prev + position.marginUsed + netPnl);
  }, []);

  const resolveExit = useCallback((p: BTCFuturesPosition, input: SignalInputs): { shouldClose: boolean; reason?: BTCFuturesPosition["exitReason"]; exitPrice: number } => {
    const markPrice = p.markPrice;
    const returnPct = p.returnPct;
    const progress = Math.abs(returnPct) / (Math.abs((p.tpPrice - p.entryPrice) / p.entryPrice) * 100);
    const ageMin = (Date.now() - new Date(p.openedAt).getTime()) / 60000;

    // Liquidation check
    if (p.side === "LONG" && markPrice <= p.liquidationPrice) {
      return { shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: p.liquidationPrice };
    }
    if (p.side === "SHORT" && markPrice >= p.liquidationPrice) {
      return { shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: p.liquidationPrice };
    }

    // No "near liquidation" auto-close — any %-of-price heuristic collided with normal 25x cushion (~3–4%)
    // and closed winners as LIQUIDATION_RISK. Paper desk exits on true liq cross above, then SL/TP/TIME.

    // SL hit
    if (p.side === "LONG" && markPrice <= p.adaptiveSl) {
      return { shouldClose: true, reason: "SL", exitPrice: p.adaptiveSl };
    }
    if (p.side === "SHORT" && markPrice >= p.adaptiveSl) {
      return { shouldClose: true, reason: "SL", exitPrice: p.adaptiveSl };
    }

    // TP hit
    if (p.side === "LONG" && markPrice >= p.tpPrice) {
      return { shouldClose: true, reason: "TP", exitPrice: p.tpPrice };
    }
    if (p.side === "SHORT" && markPrice <= p.tpPrice) {
      return { shouldClose: true, reason: "TP", exitPrice: p.tpPrice };
    }

    // Time-based exit
    const holdExtend = p.holdMinutes * MTF_HOLD_BONUS;
    if (ageMin >= holdExtend) {
      return { shouldClose: true, reason: "TIME", exitPrice: markPrice };
    }

    // Trailing stop
    if (progress >= TRAIL_ACTIVATION_PCT && !p.breakevenMoved) {
      const newSl = p.side === "LONG"
        ? p.entryPrice + (markPrice - p.entryPrice) * (1 - TRAIL_GIVEBACK_SHARE)
        : p.entryPrice - (p.entryPrice - markPrice) * (1 - TRAIL_GIVEBACK_SHARE);
      return { shouldClose: false, exitPrice: markPrice }; // Update adaptive SL
    }

    // Breakeven move
    if (progress >= BREAKEVEN_TRIGGER_FRAC && !p.breakevenMoved) {
      return { shouldClose: false, exitPrice: markPrice }; // Will update breakeven
    }

    return { shouldClose: false, exitPrice: markPrice };
  }, []);

  // ========== DATA POLLING (multi-symbol) ==========
  useEffect(() => {
    let mounted = true;
    let interval: NodeJS.Timeout | null = null;

    type KlinePayload = {
      ok: boolean;
      candles: { time: number; open: number; high: number; low: number; close: number; volume: number }[];
      lastPrice: number;
      markPrice: number;
      indexPrice: number;
      changePct24h: number;
      fundingRate: number;
      nextFunding: number;
      fetchedAt: string;
    };

    const poll = async () => {
      try {
        const payloads = new Map<string, KlinePayload>();

        for (const batch of chunkArray(activeSymbols, SYMBOL_FETCH_CHUNK)) {
          const results = await Promise.all(
            batch.map(async (sym) => {
              try {
                const res = await fetch(
                  `/api/btc/futures-klines?symbol=${encodeURIComponent(sym)}`,
                  { cache: "no-store" },
                );
                if (!res.ok) return null;
                return (await res.json()) as KlinePayload;
              } catch {
                return null;
              }
            }),
          );
          for (let i = 0; i < batch.length; i++) {
            const sym = batch[i];
            const j = results[i];
            if (j?.ok && Array.isArray(j.candles) && j.candles.length >= MIN_BARS) {
              payloads.set(sym, j);
            }
          }
        }

        if (!mounted) return;

        /** At least one symbol returned enough bars — drives quotes + entries (see statusRef note below). */
        const hasMarketData = payloads.size > 0;

        const primary =
          payloads.get(PRIMARY_QUOTE_SYMBOL) ?? payloads.values().next().value ?? null;
        if (primary) {
          setQuote({
            lastPrice: primary.lastPrice,
            markPrice: primary.markPrice,
            indexPrice: primary.indexPrice,
            changePct24h: primary.changePct24h,
            fundingRate: primary.fundingRate,
            nextFunding: primary.nextFunding,
            timestamp: new Date(primary.fetchedAt).getTime(),
          });
          setWarmingPct(Math.min(100, Math.round((primary.candles.length / MIN_BARS) * 100)));
        }

        if (hasMarketData && statusRef.current === "WARMING") {
          setStatus("READY");
        }

        setPositions((prev) =>
          prev.map((p) => {
            const d = payloads.get(p.symbol);
            if (!d) return p;
            return applyMarkToPosition(p, d.markPrice, d.lastPrice, d.fundingRate);
          }),
        );

        const mergedForLogic = positionsRef.current.map((p) => {
          const d = payloads.get(p.symbol);
          if (!d) return p;
          return applyMarkToPosition(p, d.markPrice, d.lastPrice, d.fundingRate);
        });

        const exitJobs: {
          pos: BTCFuturesPosition;
          exitPrice: number;
          reason: NonNullable<BTCFuturesPosition["exitReason"]>;
        }[] = [];

        for (const pos of mergedForLogic) {
          const d = payloads.get(pos.symbol);
          if (!d || d.candles.length < MIN_BARS) continue;
          const closes = d.candles.map((c) => c.close);
          const highs = d.candles.map((c) => c.high);
          const lows = d.candles.map((c) => c.low);
          const volumes = d.candles.map((c) => c.volume);
          const input = buildSignalInputs(closes, highs, lows, volumes, d.markPrice);
          const exit = resolveExit(pos, input);
          if (exit.shouldClose && exit.reason) {
            exitJobs.push({ pos, exitPrice: exit.exitPrice, reason: exit.reason });
          }
        }

        const exitingIds = new Set(exitJobs.map((j) => j.pos.id));
        const remainingAfterExits = mergedForLogic.filter((p) => !exitingIds.has(p.id));

        for (const job of exitJobs) {
          closePosition(job.pos, job.exitPrice, job.reason);
        }

        let openCount = remainingAfterExits.length;
        const occupied = new Set(remainingAfterExits.map((p) => `${p.symbol}:${p.strategyId}`));

        // Do not gate entries on statusRef === READY: setStatus is async and statusRef updates next render,
        // so same poll tick would never open trades. Use live payload presence instead.
        if (hasMarketData && !pauseRef.current) {
          for (const symbol of activeSymbols) {
            if (openCount >= MAX_OPEN_POSITIONS) break;
            const d = payloads.get(symbol);
            if (!d || d.candles.length < MIN_BARS) continue;

            const closes = d.candles.map((c) => c.close);
            const highs = d.candles.map((c) => c.high);
            const lows = d.candles.map((c) => c.low);
            const volumes = d.candles.map((c) => c.volume);
            const input = buildSignalInputs(closes, highs, lows, volumes, d.markPrice);

            for (const strat of activeStratDefs) {
              if (openCount >= MAX_OPEN_POSITIONS) break;
              if (disabledRef.current.includes(strat.id)) continue;
              if (occupied.has(`${symbol}:${strat.id}`)) continue;
              const ck = `${symbol}:${strat.id}`;
              if ((stratCooldownsRef.current[ck] ?? 0) > Date.now()) continue;

              const signal = evalMinuteSignal(input, strat);
              if (signal.score >= activeSignalThreshold && passesEntryConfirmation(input, strat)) {
                const side = strat.signalKey.includes("SHORT") ? "SHORT" : "LONG";
                openPosition(strat, side, d.lastPrice, d.markPrice, symbol);
                occupied.add(`${symbol}:${strat.id}`);
                openCount++;
              }
            }
          }
        }
      } catch (e) {
        console.error("Futures polling error:", e);
      }
    };

    poll();
    interval = setInterval(poll, POLL_MS);
    return () => {
      mounted = false;
      if (interval) clearInterval(interval);
    };
  }, [openPosition, closePosition, resolveExit, activeStratDefs, activeSymbols, activeSignalThreshold]);

  // ========== ENGINE REF ==========
  useEffect(() => {
    engineRef.current = {
      positions,
      trades,
      balance,
      equity: balance + positions.reduce((s, p) => s + p.unrealizedPnl, 0),
      availableMargin: balance - positions.reduce((s, p) => s + p.marginUsed, 0),
      usedMargin: positions.reduce((s, p) => s + p.marginUsed, 0),
      stats: calculateStats(),
      quote,
      isReady: status === "READY",
      pauseEntries,
      disabledStrategies,
      togglePause,
      resetPaperAccount,
      clearTradeHistory,
      setDisabledStrategies: setDisabledStrategiesHandler,
      exportCSV,
      exportJSON,
    };
  }, [positions, trades, balance, quote, status, pauseEntries, disabledStrategies, calculateStats, togglePause, resetPaperAccount, clearTradeHistory, setDisabledStrategiesHandler, exportCSV, exportJSON]);

  // ========== DAILY RESET ==========
  useEffect(() => {
    const checkDay = () => {
      const now = new Date();
      const currentDate = now.getDate();
      if (currentDate !== dayStartDateRef.current) {
        setDayStartDate(currentDate);
        setDayStartBalance(balanceRef.current);
      }
    };
    const id = setInterval(checkDay, 60000);
    return () => clearInterval(id);
  }, []);

  const stats = calculateStats();
  const usedMargin = positions.reduce((s, p) => s + p.marginUsed, 0);
  const availableMargin = balance - usedMargin;
  const equity = balance + positions.reduce((s, p) => s + p.unrealizedPnl, 0);

  return {
    positions,
    trades,
    balance,
    equity,
    availableMargin,
    usedMargin,
    stats,
    quote,
    isReady: status === "READY",
    pauseEntries,
    disabledStrategies,
    engineRef,
    togglePause,
    resetPaperAccount,
    clearTradeHistory,
    setDisabledStrategies: setDisabledStrategiesHandler,
    exportCSV,
    exportJSON,
    strategyStatuses,
  };
}

