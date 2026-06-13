"use client";

/**
 * useMCXEngine
 *
 * Fully autonomous, client-side MCX commodity option scalping engine.
 * - Polls /api/mcx/ltp every 5 seconds for every configured MCX commodity
 * - Seeds 1-min bars from /api/mcx/candles on mount
 * - Runs signal-driven strategies across the configured commodity roster
 * - Roster system: ACTIVE/WATCHLIST/DISABLED with shadow position tracking
 * - Paper option positions with delta-based mark-to-market, TP/SL auto-exit
 * - MCX session gating: 9:00 AM – 11:30 PM IST (3:30–18:00 UTC)
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { MCX_COMMODITIES, MCX_COMMODITY_MAP, type MCXCommodity } from "@/lib/trading/mcxCommodities";
import type { MCXLTPItem } from "@/app/api/mcx/ltp/route";
import type { MCXCandle } from "@/app/api/mcx/candles/route";

// ─── Constants ─────────────────────────────────────────────────────────────────

const INITIAL_BALANCE = 1_000_000;
const MAX_CONCURRENT = 12;
const MAX_BARS = 200;
const TICK_MS = 5_000;
const MCX_PROFIT_LOCK_PROGRESS = 0.52;
const MCX_PROFIT_LOCK_SHARE = 0.65;
const MCX_LATE_EXIT_PROGRESS = 0.70;
const MCX_LATE_EXIT_MIN_GAIN = 0.14;
const MCX_GRIND_EXIT_PROGRESS = 0.55;
const MCX_GRIND_EXIT_SHARE = 0.50;
const MCX_TRAIL_GIVEBACK_SHARE = 0.22;
const MCX_MIN_SIZE_MULTIPLIER = 0.5;
const MCX_MAX_SIZE_MULTIPLIER = 1.3;
const MCX_ROUND_TRIP_FEE_PCT = 0.06;
const MCX_BID_ASK_SPREAD_PCT = 0.04;
const MCX_LOSS_COOLDOWN_PENALTY = 0.3;
const MCX_UNDERPERFORMING_MIN_TRADES = 8;
const MCX_UNDERPERFORMING_MAX_WINRATE = 35;
const MCX_UNDERPERFORMING_PAUSE_MS = 6 * 60 * 60 * 1000;
const MCX_LOSS_STREAK_DISABLE_THRESHOLD = 5;
const MCX_LOSS_STREAK_COOLDOWN_MS = 50 * 60 * 1000;
const MCX_SHADOW_PROMOTE_TRADES = 3;
const MCX_SHADOW_PROMOTE_WINRATE = 0.45;

// ─── Public types ─────────────────────────────────────────────────────────────

export type MCXRosterState = "ACTIVE" | "WATCHLIST" | "DISABLED";

export type MCXPosition = {
  id: string;
  commodityId: string;
  commodityName: string;
  strategyId: number;
  strategyName: string;
  optionType: "CALL" | "PUT";
  strike: number;
  entryPremium: number;
  currentPremium: number;
  tpPremium: number;
  slPremium: number;
  quantity: number;
  lots: number;
  costBasis: number;
  entryPrice: number;
  currentPrice: number;
  entryTime: string;
  unrealizedPnl: number;
  iv: number;
  delta: number;
};

export type MCXTrade = {
  id: string;
  commodityId: string;
  commodityName: string;
  strategyId: number;
  strategyName: string;
  optionType: "CALL" | "PUT";
  strike: number;
  entryPremium: number;
  exitPremium: number;
  lots: number;
  costBasis: number;
  netPnl: number;
  returnPct: number;
  entryPrice: number;
  exitPrice: number;
  entryTime: string;
  exitTime: string;
  exitReason: string;
};

export type MCXStrategyStatus = {
  strategyId: number;
  name: string;
  commodityId: string;
  commodityName: string;
  optionType: "CALL" | "PUT";
  status: string;
  rosterState: MCXRosterState;
  score: number;
  totalTrades: number;
  wins: number;
  losses: number;
  winRate: number;
  totalPnl: number;
  hasPosition: boolean;
  shadowTrades: number;
  shadowWins: number;
  hasShadowPosition: boolean;
};

export type MCXCommodityQuote = {
  id: string;
  name: string;
  unit: string;
  sector: string;
  bias: "BULLISH" | "BEARISH" | "NEUTRAL" | "WARMING";
  rsi: number;
  momentumPct: number;
  volatilityPct: number;
  ltp: number;
  open: number;
  high: number;
  low: number;
  close: number;
  change: number;
  changePct: number;
  barCount: number;
};

export type MCXStats = {
  balance: number;
  equity: number;
  totalTrades: number;
  openPositions: number;
  totalWins: number;
  totalLosses: number;
  winRate: number;
  totalPnl: number;
  unrealizedPnl: number;
  capitalUtilizationPct: number;
  sessionOpen: boolean;
  maxConcurrentPositions: number;
};

export type MCXCandleIssue = {
  commodityId: string;
  commodityName: string;
  error: string;
};

export type MCXDiagnostics = {
  ltpError: string;
  candleIssues: MCXCandleIssue[];
  lastLtpAt: string | null;
};

// ─── Strategy definitions ──────────────────────────────────────────────────────

interface StratDef {
  id: number;
  name: string;
  commodityId: string;
  optionType: "CALL" | "PUT";
  signal: string;
  tpPct: number;
  slPct: number;
  cooldownSecs: number;
  minBars: number;
  startAsWatchlist?: boolean;
}

type MCXCategory = "Momentum" | "Mean Reversion" | "Breakout";

type MCXCategoryProfile = {
  minTp: number;
  maxTp: number;
  minSl: number;
  maxSl: number;
};

const MCX_CATEGORY_PROFILES: Record<MCXCategory, MCXCategoryProfile> = {
  Momentum: { minTp: 0.46, maxTp: 0.68, minSl: 0.26, maxSl: 0.34 },
  "Mean Reversion": { minTp: 0.36, maxTp: 0.52, minSl: 0.20, maxSl: 0.28 },
  Breakout: { minTp: 0.50, maxTp: 0.70, minSl: 0.26, maxSl: 0.32 },
};

function mcxClamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function categoryForSignal(signal: string): MCXCategory {
  switch (signal) {
    case "RSI_EXTREME_OS":
    case "RSI_EXTREME_OB":
    case "BB_LOWER_TOUCH":
    case "BB_UPPER_TOUCH":
    case "STOCH_OS":
    case "STOCH_OB":
    case "STOCH_BULL_CROSS":
    case "STOCH_BEAR_CROSS":
      return "Mean Reversion";
    case "RESIST_BREAK":
    case "SUPPORT_BREAK":
    case "ATR_BREAKOUT_BULL":
    case "ATR_BREAKOUT_BEAR":
    case "BB_SQUEEZE_BULL":
    case "BB_SQUEEZE_BEAR":
      return "Breakout";
    default:
      // BULL_MOM, BEAR_MOM, STRONG_*, EMA_BULL_CROSS, EMA_BEAR_CROSS,
      // TREND_PULLBACK_BULL, TREND_PULLBACK_BEAR
      return "Momentum";
  }
}

function tuneStrategy(def: StratDef): StratDef {
  const profile = MCX_CATEGORY_PROFILES[categoryForSignal(def.signal)];
  return {
    ...def,
    tpPct: mcxClamp(def.tpPct, profile.minTp, profile.maxTp),
    slPct: mcxClamp(def.slPct, profile.minSl, profile.maxSl),
  };
}

const BASE_STRAT_DEFS: StratDef[] = [
  // ── Crude Oil (volatile, trend-following + BB) — ACTIVE
  { id: 1,  name: "CrudeOil_MomBull_Call",       commodityId: "CRUDEOIL",   optionType: "CALL", signal: "BULL_MOM",           tpPct: 0.70, slPct: 0.25, cooldownSecs: 240, minBars: 15 },
  { id: 2,  name: "CrudeOil_MomBear_Put",        commodityId: "CRUDEOIL",   optionType: "PUT",  signal: "BEAR_MOM",           tpPct: 0.70, slPct: 0.25, cooldownSecs: 240, minBars: 15 },
  { id: 3,  name: "CrudeOil_BB_Lower_Call",      commodityId: "CRUDEOIL",   optionType: "CALL", signal: "BB_LOWER_TOUCH",     tpPct: 0.60, slPct: 0.22, cooldownSecs: 300, minBars: 22 },
  { id: 4,  name: "CrudeOil_BB_Upper_Put",       commodityId: "CRUDEOIL",   optionType: "PUT",  signal: "BB_UPPER_TOUCH",     tpPct: 0.60, slPct: 0.22, cooldownSecs: 300, minBars: 22 },

  // ── Gold Mini (mean-reverting, RSI + EMA) — ACTIVE
  { id: 5,  name: "Gold_RSI_OS_Call",            commodityId: "GOLDM",      optionType: "CALL", signal: "RSI_EXTREME_OS",     tpPct: 0.90, slPct: 0.28, cooldownSecs: 480, minBars: 20 },
  { id: 6,  name: "Gold_RSI_OB_Put",             commodityId: "GOLDM",      optionType: "PUT",  signal: "RSI_EXTREME_OB",     tpPct: 0.90, slPct: 0.28, cooldownSecs: 480, minBars: 20 },
  { id: 7,  name: "Gold_EMA_Bull_Call",          commodityId: "GOLDM",      optionType: "CALL", signal: "EMA_BULL_CROSS",     tpPct: 0.65, slPct: 0.24, cooldownSecs: 480, minBars: 22 },
  { id: 8,  name: "Gold_EMA_Bear_Put",           commodityId: "GOLDM",      optionType: "PUT",  signal: "EMA_BEAR_CROSS",     tpPct: 0.65, slPct: 0.24, cooldownSecs: 480, minBars: 22 },

  // ── Silver Mini (momentum + RSI) — ACTIVE
  { id: 9,  name: "Silver_MomBull_Call",         commodityId: "SILVERM",    optionType: "CALL", signal: "BULL_MOM",           tpPct: 0.75, slPct: 0.26, cooldownSecs: 300, minBars: 15 },
  { id: 10, name: "Silver_MomBear_Put",          commodityId: "SILVERM",    optionType: "PUT",  signal: "BEAR_MOM",           tpPct: 0.75, slPct: 0.26, cooldownSecs: 300, minBars: 15 },
  { id: 11, name: "Silver_RSI_OS_Call",          commodityId: "SILVERM",    optionType: "CALL", signal: "RSI_EXTREME_OS",     tpPct: 0.85, slPct: 0.28, cooldownSecs: 480, minBars: 20 },
  { id: 12, name: "Silver_RSI_OB_Put",           commodityId: "SILVERM",    optionType: "PUT",  signal: "RSI_EXTREME_OB",     tpPct: 0.85, slPct: 0.28, cooldownSecs: 480, minBars: 20 },

  // ── Natural Gas (hyper-volatile, strong momentum + stochastic) — ACTIVE
  { id: 13, name: "NatGas_StrongMom_Call",       commodityId: "NATURALGAS", optionType: "CALL", signal: "STRONG_BULL_MOM",    tpPct: 1.00, slPct: 0.35, cooldownSecs: 360, minBars: 15 },
  { id: 14, name: "NatGas_StrongMom_Put",        commodityId: "NATURALGAS", optionType: "PUT",  signal: "STRONG_BEAR_MOM",    tpPct: 1.00, slPct: 0.35, cooldownSecs: 360, minBars: 15 },
  { id: 15, name: "NatGas_Stoch_OS_Call",        commodityId: "NATURALGAS", optionType: "CALL", signal: "STOCH_OS",           tpPct: 0.80, slPct: 0.30, cooldownSecs: 420, minBars: 20 },
  { id: 16, name: "NatGas_Stoch_OB_Put",         commodityId: "NATURALGAS", optionType: "PUT",  signal: "STOCH_OB",           tpPct: 0.80, slPct: 0.30, cooldownSecs: 420, minBars: 20 },

  // ── Copper (industrial trend-following + breakout) — ACTIVE
  { id: 17, name: "Copper_MomBull_Call",         commodityId: "COPPER",     optionType: "CALL", signal: "BULL_MOM",           tpPct: 0.70, slPct: 0.25, cooldownSecs: 300, minBars: 15 },
  { id: 18, name: "Copper_MomBear_Put",          commodityId: "COPPER",     optionType: "PUT",  signal: "BEAR_MOM",           tpPct: 0.70, slPct: 0.25, cooldownSecs: 300, minBars: 15 },
  { id: 19, name: "Copper_Resist_Break_Call",    commodityId: "COPPER",     optionType: "CALL", signal: "RESIST_BREAK",       tpPct: 0.84, slPct: 0.28, cooldownSecs: 480, minBars: 22 },
  { id: 20, name: "Copper_Support_Break_Put",    commodityId: "COPPER",     optionType: "PUT",  signal: "SUPPORT_BREAK",      tpPct: 0.84, slPct: 0.28, cooldownSecs: 480, minBars: 22 },

  // ──────────────────────────────────────────────────────────────────────────────
  // NEW STRATEGIES — start in WATCHLIST (shadow mode until qualified)
  // ──────────────────────────────────────────────────────────────────────────────

  // ── Crude Oil extended
  { id: 21, name: "CrudeOil_ATR_Break_Call",     commodityId: "CRUDEOIL",   optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.72, slPct: 0.28, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 22, name: "CrudeOil_ATR_Break_Put",      commodityId: "CRUDEOIL",   optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.72, slPct: 0.28, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 23, name: "CrudeOil_Stoch_Cross_Call",   commodityId: "CRUDEOIL",   optionType: "CALL", signal: "STOCH_BULL_CROSS",   tpPct: 0.55, slPct: 0.24, cooldownSecs: 360, minBars: 20, startAsWatchlist: true },
  { id: 24, name: "CrudeOil_Stoch_Cross_Put",    commodityId: "CRUDEOIL",   optionType: "PUT",  signal: "STOCH_BEAR_CROSS",   tpPct: 0.55, slPct: 0.24, cooldownSecs: 360, minBars: 20, startAsWatchlist: true },
  { id: 25, name: "CrudeOil_Pullback_Call",      commodityId: "CRUDEOIL",   optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.68, slPct: 0.26, cooldownSecs: 300, minBars: 60, startAsWatchlist: true },
  { id: 26, name: "CrudeOil_Pullback_Put",       commodityId: "CRUDEOIL",   optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.68, slPct: 0.26, cooldownSecs: 300, minBars: 60, startAsWatchlist: true },

  // ── Gold Mini extended
  { id: 27, name: "Gold_BB_Squeeze_Call",        commodityId: "GOLDM",      optionType: "CALL", signal: "BB_SQUEEZE_BULL",    tpPct: 0.62, slPct: 0.26, cooldownSecs: 360, minBars: 25, startAsWatchlist: true },
  { id: 28, name: "Gold_BB_Squeeze_Put",         commodityId: "GOLDM",      optionType: "PUT",  signal: "BB_SQUEEZE_BEAR",    tpPct: 0.62, slPct: 0.26, cooldownSecs: 360, minBars: 25, startAsWatchlist: true },
  { id: 29, name: "Gold_Stoch_Cross_Call",       commodityId: "GOLDM",      optionType: "CALL", signal: "STOCH_BULL_CROSS",   tpPct: 0.52, slPct: 0.22, cooldownSecs: 480, minBars: 20, startAsWatchlist: true },
  { id: 30, name: "Gold_Stoch_Cross_Put",        commodityId: "GOLDM",      optionType: "PUT",  signal: "STOCH_BEAR_CROSS",   tpPct: 0.52, slPct: 0.22, cooldownSecs: 480, minBars: 20, startAsWatchlist: true },
  { id: 31, name: "Gold_Pullback_Call",          commodityId: "GOLDM",      optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.65, slPct: 0.24, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },
  { id: 32, name: "Gold_Pullback_Put",           commodityId: "GOLDM",      optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.65, slPct: 0.24, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },

  // ── Silver Mini extended
  { id: 33, name: "Silver_ATR_Break_Call",       commodityId: "SILVERM",    optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.68, slPct: 0.27, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 34, name: "Silver_ATR_Break_Put",        commodityId: "SILVERM",    optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.68, slPct: 0.27, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 35, name: "Silver_BB_Squeeze_Call",      commodityId: "SILVERM",    optionType: "CALL", signal: "BB_SQUEEZE_BULL",    tpPct: 0.60, slPct: 0.25, cooldownSecs: 360, minBars: 25, startAsWatchlist: true },
  { id: 36, name: "Silver_BB_Squeeze_Put",       commodityId: "SILVERM",    optionType: "PUT",  signal: "BB_SQUEEZE_BEAR",    tpPct: 0.60, slPct: 0.25, cooldownSecs: 360, minBars: 25, startAsWatchlist: true },
  { id: 37, name: "Silver_Pullback_Call",        commodityId: "SILVERM",    optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.65, slPct: 0.25, cooldownSecs: 300, minBars: 60, startAsWatchlist: true },
  { id: 38, name: "Silver_Pullback_Put",         commodityId: "SILVERM",    optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.65, slPct: 0.25, cooldownSecs: 300, minBars: 60, startAsWatchlist: true },

  // ── Natural Gas extended
  { id: 39, name: "NatGas_ATR_Break_Call",       commodityId: "NATURALGAS", optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.70, slPct: 0.30, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 40, name: "NatGas_ATR_Break_Put",        commodityId: "NATURALGAS", optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.70, slPct: 0.30, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 41, name: "NatGas_BB_Lower_Call",        commodityId: "NATURALGAS", optionType: "CALL", signal: "BB_LOWER_TOUCH",     tpPct: 0.65, slPct: 0.28, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 42, name: "NatGas_BB_Upper_Put",         commodityId: "NATURALGAS", optionType: "PUT",  signal: "BB_UPPER_TOUCH",     tpPct: 0.65, slPct: 0.28, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 43, name: "NatGas_Pullback_Call",        commodityId: "NATURALGAS", optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.68, slPct: 0.28, cooldownSecs: 360, minBars: 60, startAsWatchlist: true },
  { id: 44, name: "NatGas_Pullback_Put",         commodityId: "NATURALGAS", optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.68, slPct: 0.28, cooldownSecs: 360, minBars: 60, startAsWatchlist: true },

  // ── Copper extended
  { id: 45, name: "Copper_ATR_Break_Call",       commodityId: "COPPER",     optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.68, slPct: 0.26, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 46, name: "Copper_ATR_Break_Put",        commodityId: "COPPER",     optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.68, slPct: 0.26, cooldownSecs: 300, minBars: 22, startAsWatchlist: true },
  { id: 47, name: "Copper_BB_Lower_Call",        commodityId: "COPPER",     optionType: "CALL", signal: "BB_LOWER_TOUCH",     tpPct: 0.60, slPct: 0.24, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 48, name: "Copper_BB_Upper_Put",         commodityId: "COPPER",     optionType: "PUT",  signal: "BB_UPPER_TOUCH",     tpPct: 0.60, slPct: 0.24, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 49, name: "Copper_Stoch_Cross_Call",     commodityId: "COPPER",     optionType: "CALL", signal: "STOCH_BULL_CROSS",   tpPct: 0.55, slPct: 0.24, cooldownSecs: 360, minBars: 20, startAsWatchlist: true },
  { id: 50, name: "Copper_Stoch_Cross_Put",      commodityId: "COPPER",     optionType: "PUT",  signal: "STOCH_BEAR_CROSS",   tpPct: 0.55, slPct: 0.24, cooldownSecs: 360, minBars: 20, startAsWatchlist: true },

  // ── Aluminium
  { id: 51, name: "Aluminium_MomBull_Call",      commodityId: "ALUMINIUM",  optionType: "CALL", signal: "BULL_MOM",           tpPct: 0.62, slPct: 0.24, cooldownSecs: 360, minBars: 15 },
  { id: 52, name: "Aluminium_MomBear_Put",       commodityId: "ALUMINIUM",  optionType: "PUT",  signal: "BEAR_MOM",           tpPct: 0.62, slPct: 0.24, cooldownSecs: 360, minBars: 15 },
  { id: 53, name: "Aluminium_Resist_Break_Call", commodityId: "ALUMINIUM",  optionType: "CALL", signal: "RESIST_BREAK",       tpPct: 0.66, slPct: 0.26, cooldownSecs: 480, minBars: 22 },
  { id: 54, name: "Aluminium_Support_Break_Put", commodityId: "ALUMINIUM",  optionType: "PUT",  signal: "SUPPORT_BREAK",      tpPct: 0.66, slPct: 0.26, cooldownSecs: 480, minBars: 22 },
  { id: 55, name: "Aluminium_ATR_Break_Call",    commodityId: "ALUMINIUM",  optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.64, slPct: 0.26, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 56, name: "Aluminium_ATR_Break_Put",     commodityId: "ALUMINIUM",  optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.64, slPct: 0.26, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 57, name: "Aluminium_BB_Lower_Call",     commodityId: "ALUMINIUM",  optionType: "CALL", signal: "BB_LOWER_TOUCH",     tpPct: 0.50, slPct: 0.22, cooldownSecs: 420, minBars: 22, startAsWatchlist: true },
  { id: 58, name: "Aluminium_BB_Upper_Put",      commodityId: "ALUMINIUM",  optionType: "PUT",  signal: "BB_UPPER_TOUCH",     tpPct: 0.50, slPct: 0.22, cooldownSecs: 420, minBars: 22, startAsWatchlist: true },
  { id: 59, name: "Aluminium_Pullback_Call",     commodityId: "ALUMINIUM",  optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.58, slPct: 0.24, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },
  { id: 60, name: "Aluminium_Pullback_Put",      commodityId: "ALUMINIUM",  optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.58, slPct: 0.24, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },

  // ── Zinc
  { id: 61, name: "Zinc_MomBull_Call",           commodityId: "ZINC",       optionType: "CALL", signal: "BULL_MOM",           tpPct: 0.64, slPct: 0.25, cooldownSecs: 360, minBars: 15 },
  { id: 62, name: "Zinc_MomBear_Put",            commodityId: "ZINC",       optionType: "PUT",  signal: "BEAR_MOM",           tpPct: 0.64, slPct: 0.25, cooldownSecs: 360, minBars: 15 },
  { id: 63, name: "Zinc_Resist_Break_Call",      commodityId: "ZINC",       optionType: "CALL", signal: "RESIST_BREAK",       tpPct: 0.68, slPct: 0.27, cooldownSecs: 480, minBars: 22 },
  { id: 64, name: "Zinc_Support_Break_Put",      commodityId: "ZINC",       optionType: "PUT",  signal: "SUPPORT_BREAK",      tpPct: 0.68, slPct: 0.27, cooldownSecs: 480, minBars: 22 },
  { id: 65, name: "Zinc_ATR_Break_Call",         commodityId: "ZINC",       optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.66, slPct: 0.27, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 66, name: "Zinc_ATR_Break_Put",          commodityId: "ZINC",       optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.66, slPct: 0.27, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 67, name: "Zinc_RSI_OS_Call",            commodityId: "ZINC",       optionType: "CALL", signal: "RSI_EXTREME_OS",     tpPct: 0.52, slPct: 0.24, cooldownSecs: 480, minBars: 20, startAsWatchlist: true },
  { id: 68, name: "Zinc_RSI_OB_Put",             commodityId: "ZINC",       optionType: "PUT",  signal: "RSI_EXTREME_OB",     tpPct: 0.52, slPct: 0.24, cooldownSecs: 480, minBars: 20, startAsWatchlist: true },
  { id: 69, name: "Zinc_Pullback_Call",          commodityId: "ZINC",       optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.60, slPct: 0.25, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },
  { id: 70, name: "Zinc_Pullback_Put",           commodityId: "ZINC",       optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.60, slPct: 0.25, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },

  // ── Lead
  { id: 71, name: "Lead_MomBull_Call",           commodityId: "LEAD",       optionType: "CALL", signal: "BULL_MOM",           tpPct: 0.60, slPct: 0.24, cooldownSecs: 360, minBars: 15 },
  { id: 72, name: "Lead_MomBear_Put",            commodityId: "LEAD",       optionType: "PUT",  signal: "BEAR_MOM",           tpPct: 0.60, slPct: 0.24, cooldownSecs: 360, minBars: 15 },
  { id: 73, name: "Lead_Resist_Break_Call",      commodityId: "LEAD",       optionType: "CALL", signal: "RESIST_BREAK",       tpPct: 0.64, slPct: 0.26, cooldownSecs: 480, minBars: 22 },
  { id: 74, name: "Lead_Support_Break_Put",      commodityId: "LEAD",       optionType: "PUT",  signal: "SUPPORT_BREAK",      tpPct: 0.64, slPct: 0.26, cooldownSecs: 480, minBars: 22 },
  { id: 75, name: "Lead_ATR_Break_Call",         commodityId: "LEAD",       optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.62, slPct: 0.26, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 76, name: "Lead_ATR_Break_Put",          commodityId: "LEAD",       optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.62, slPct: 0.26, cooldownSecs: 360, minBars: 22, startAsWatchlist: true },
  { id: 77, name: "Lead_BB_Lower_Call",          commodityId: "LEAD",       optionType: "CALL", signal: "BB_LOWER_TOUCH",     tpPct: 0.48, slPct: 0.22, cooldownSecs: 420, minBars: 22, startAsWatchlist: true },
  { id: 78, name: "Lead_BB_Upper_Put",           commodityId: "LEAD",       optionType: "PUT",  signal: "BB_UPPER_TOUCH",     tpPct: 0.48, slPct: 0.22, cooldownSecs: 420, minBars: 22, startAsWatchlist: true },
  { id: 79, name: "Lead_Pullback_Call",          commodityId: "LEAD",       optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.58, slPct: 0.24, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },
  { id: 80, name: "Lead_Pullback_Put",           commodityId: "LEAD",       optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.58, slPct: 0.24, cooldownSecs: 480, minBars: 60, startAsWatchlist: true },

  // ── Nickel
  { id: 81, name: "Nickel_StrongMom_Call",       commodityId: "NICKEL",     optionType: "CALL", signal: "STRONG_BULL_MOM",    tpPct: 0.72, slPct: 0.30, cooldownSecs: 420, minBars: 15 },
  { id: 82, name: "Nickel_StrongMom_Put",        commodityId: "NICKEL",     optionType: "PUT",  signal: "STRONG_BEAR_MOM",    tpPct: 0.72, slPct: 0.30, cooldownSecs: 420, minBars: 15 },
  { id: 83, name: "Nickel_Resist_Break_Call",    commodityId: "NICKEL",     optionType: "CALL", signal: "RESIST_BREAK",       tpPct: 0.70, slPct: 0.30, cooldownSecs: 480, minBars: 22 },
  { id: 84, name: "Nickel_Support_Break_Put",    commodityId: "NICKEL",     optionType: "PUT",  signal: "SUPPORT_BREAK",      tpPct: 0.70, slPct: 0.30, cooldownSecs: 480, minBars: 22 },
  { id: 85, name: "Nickel_ATR_Break_Call",       commodityId: "NICKEL",     optionType: "CALL", signal: "ATR_BREAKOUT_BULL",  tpPct: 0.70, slPct: 0.30, cooldownSecs: 420, minBars: 22, startAsWatchlist: true },
  { id: 86, name: "Nickel_ATR_Break_Put",        commodityId: "NICKEL",     optionType: "PUT",  signal: "ATR_BREAKOUT_BEAR",  tpPct: 0.70, slPct: 0.30, cooldownSecs: 420, minBars: 22, startAsWatchlist: true },
  { id: 87, name: "Nickel_RSI_OS_Call",          commodityId: "NICKEL",     optionType: "CALL", signal: "RSI_EXTREME_OS",     tpPct: 0.52, slPct: 0.26, cooldownSecs: 540, minBars: 20, startAsWatchlist: true },
  { id: 88, name: "Nickel_RSI_OB_Put",           commodityId: "NICKEL",     optionType: "PUT",  signal: "RSI_EXTREME_OB",     tpPct: 0.52, slPct: 0.26, cooldownSecs: 540, minBars: 20, startAsWatchlist: true },
  { id: 89, name: "Nickel_Pullback_Call",        commodityId: "NICKEL",     optionType: "CALL", signal: "TREND_PULLBACK_BULL", tpPct: 0.66, slPct: 0.28, cooldownSecs: 540, minBars: 60, startAsWatchlist: true },
  { id: 90, name: "Nickel_Pullback_Put",         commodityId: "NICKEL",     optionType: "PUT",  signal: "TREND_PULLBACK_BEAR", tpPct: 0.66, slPct: 0.28, cooldownSecs: 540, minBars: 60, startAsWatchlist: true },
];

// ─── Math helpers ──────────────────────────────────────────────────────────────

const STRAT_DEFS: StratDef[] = BASE_STRAT_DEFS.map(tuneStrategy);

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
  const s = bars.slice(-period);
  return s.reduce((a, v) => a + v, 0) / s.length;
}

function stddev(bars: number[]): number {
  if (bars.length < 2) return 0;
  const m = bars.reduce((s, v) => s + v, 0) / bars.length;
  return Math.sqrt(bars.reduce((s, v) => s + (v - m) ** 2, 0) / bars.length);
}

function bbMid(bars: number[], p: number) { return sma(bars.slice(-p), p); }
function bbUpper(bars: number[], p: number) { return bbMid(bars, p) + 2 * stddev(bars.slice(-p)); }
function bbLower(bars: number[], p: number) { return bbMid(bars, p) - 2 * stddev(bars.slice(-p)); }

function momentum(bars: number[], n: number): number {
  if (bars.length <= n) return 0;
  const prev = bars[bars.length - 1 - n];
  return prev === 0 ? 0 : (bars[bars.length - 1] - prev) / prev;
}

function stochK(bars: number[], period: number): number {
  if (bars.length < period) return 50;
  const s = bars.slice(-period);
  const lo = Math.min(...s), hi = Math.max(...s);
  return hi === lo ? 50 : ((bars[bars.length - 1] - lo) / (hi - lo)) * 100;
}

function atr(bars: number[], period: number): number {
  if (bars.length < period + 1) return 0;
  const slice = bars.slice(-period - 1);
  let total = 0;
  for (let i = 1; i < slice.length; i++) total += Math.abs(slice[i] - slice[i - 1]);
  return total / period;
}

function crossedAbove(bars: number[], fp: number, sp: number): boolean {
  if (bars.length < sp + 2) return false;
  const prev = bars.slice(0, -1);
  return ema(bars, fp) > ema(bars, sp) && ema(prev, fp) <= ema(prev, sp);
}

function crossedBelow(bars: number[], fp: number, sp: number): boolean {
  if (bars.length < sp + 2) return false;
  const prev = bars.slice(0, -1);
  return ema(bars, fp) < ema(bars, sp) && ema(prev, fp) >= ema(prev, sp);
}

// ─── MCX session check ─────────────────────────────────────────────────────────
// MCX hours: 9:00 AM – 11:30 PM IST = 3:30 AM – 6:00 PM UTC

function isMCXSessionOpen(): boolean {
  const now = new Date();
  const utcMinutes = now.getUTCHours() * 60 + now.getUTCMinutes();
  const open = 3 * 60 + 30;   // 03:30 UTC = 09:00 IST
  const close = 18 * 60;       // 18:00 UTC = 23:30 IST
  return utcMinutes >= open && utcMinutes < close;
}

// ─── Signal evaluator ─────────────────────────────────────────────────────────

function evalSignal(signal: string, bars: number[], price: number): boolean {
  const n = bars.length;
  if (n < 2) return false;

  switch (signal) {
    case "STRONG_BULL_MOM": {
      if (n < 15) return false;
      return momentum(bars, 5) > 0.0014 && momentum(bars, 3) > 0.0006 && rsi(bars, 14) < 70;
    }
    case "BULL_MOM": {
      if (n < 15) return false;
      return momentum(bars, 5) > 0.0008 && rsi(bars, 14) < 66 && price > ema(bars, 9);
    }
    case "STRONG_BEAR_MOM": {
      if (n < 15) return false;
      return momentum(bars, 5) < -0.0014 && momentum(bars, 3) < -0.0006 && rsi(bars, 14) > 30;
    }
    case "BEAR_MOM": {
      if (n < 15) return false;
      return momentum(bars, 5) < -0.0008 && rsi(bars, 14) > 34 && price < ema(bars, 9);
    }
    case "RSI_EXTREME_OS":
      // Fixed: removed price >= ema(bars,9) — price IS below EMA9 when RSI<25
      if (n < 20) return false;
      return rsi(bars, 14) < 25 && price > bars[n - 2];
    case "RSI_EXTREME_OB":
      // Fixed: removed price <= ema(bars,9) — price IS above EMA9 when RSI>75
      if (n < 20) return false;
      return rsi(bars, 14) > 75 && price < bars[n - 2];
    case "BB_LOWER_TOUCH": {
      if (n < 22) return false;
      const bl = bbLower(bars, 20);
      return bars[n - 2] <= bl * 1.0015 && price > bars[n - 2] && price < bbMid(bars, 20) && rsi(bars, 14) < 50;
    }
    case "BB_UPPER_TOUCH": {
      if (n < 22) return false;
      const bu = bbUpper(bars, 20);
      return bars[n - 2] >= bu * 0.9985 && price < bars[n - 2] && price > bbMid(bars, 20) && rsi(bars, 14) > 50;
    }
    case "EMA_BULL_CROSS":
      return crossedAbove(bars, 9, 21);
    case "EMA_BEAR_CROSS":
      return crossedBelow(bars, 9, 21);
    case "RESIST_BREAK": {
      if (n < 22) return false;
      const hi = Math.max(...bars.slice(n - 21, n - 1));
      return price > hi * 1.0009 && momentum(bars, 3) > 0.0008;
    }
    case "SUPPORT_BREAK": {
      if (n < 22) return false;
      const lo = Math.min(...bars.slice(n - 21, n - 1));
      return price < lo * 0.9991 && momentum(bars, 3) < -0.0008;
    }
    case "STOCH_OS":
      if (n < 20) return false;
      return stochK(bars, 14) < 20 && price > bars[n - 2] && rsi(bars, 14) < 52;
    case "STOCH_OB":
      if (n < 20) return false;
      return stochK(bars, 14) > 80 && price < bars[n - 2] && rsi(bars, 14) > 48;

    // ── New signals ──────────────────────────────────────────────────────────
    case "ATR_BREAKOUT_BULL": {
      if (n < 22) return false;
      const a = atr(bars, 14);
      const hi20 = Math.max(...bars.slice(n - 21, n - 1));
      return a > 0 && price > hi20 + a * 0.5 && momentum(bars, 3) > 0.0008 && rsi(bars, 14) < 72;
    }
    case "ATR_BREAKOUT_BEAR": {
      if (n < 22) return false;
      const a = atr(bars, 14);
      const lo20 = Math.min(...bars.slice(n - 21, n - 1));
      return a > 0 && price < lo20 - a * 0.5 && momentum(bars, 3) < -0.0008 && rsi(bars, 14) > 28;
    }
    case "STOCH_BULL_CROSS": {
      if (n < 20) return false;
      const prevBars = bars.slice(0, -1);
      const kNow = stochK(bars, 14);
      const kPrev = stochK(prevBars, 14);
      // Was in oversold territory, now crossing upward
      return kPrev < 25 && kNow > kPrev && rsi(bars, 14) < 55;
    }
    case "STOCH_BEAR_CROSS": {
      if (n < 20) return false;
      const prevBars = bars.slice(0, -1);
      const kNow = stochK(bars, 14);
      const kPrev = stochK(prevBars, 14);
      // Was in overbought territory, now crossing downward
      return kPrev > 75 && kNow < kPrev && rsi(bars, 14) > 45;
    }
    case "BB_SQUEEZE_BULL": {
      if (n < 25) return false;
      const bwNow = bbUpper(bars, 20) - bbLower(bars, 20);
      const prev3 = bars.slice(0, -3);
      const bwPrev = prev3.length >= 20 ? bbUpper(prev3, 20) - bbLower(prev3, 20) : bwNow;
      return price > bbMid(bars, 20) && bwNow > bwPrev * 1.1 && momentum(bars, 3) > 0.0005;
    }
    case "BB_SQUEEZE_BEAR": {
      if (n < 25) return false;
      const bwNow = bbUpper(bars, 20) - bbLower(bars, 20);
      const prev3 = bars.slice(0, -3);
      const bwPrev = prev3.length >= 20 ? bbUpper(prev3, 20) - bbLower(prev3, 20) : bwNow;
      return price < bbMid(bars, 20) && bwNow > bwPrev * 1.1 && momentum(bars, 3) < -0.0005;
    }
    case "TREND_PULLBACK_BULL": {
      if (n < 60) return false;
      const e9 = ema(bars, 9);
      const e21 = ema(bars, 21);
      const e55 = ema(bars, 55);
      const r = rsi(bars, 14);
      // Intact uptrend, price pulled back to EMA21 zone
      return e9 > e21 && e21 > e55 && price <= e21 * 1.005 && price >= e21 * 0.993 && r >= 38 && r <= 62;
    }
    case "TREND_PULLBACK_BEAR": {
      if (n < 60) return false;
      const e9 = ema(bars, 9);
      const e21 = ema(bars, 21);
      const e55 = ema(bars, 55);
      const r = rsi(bars, 14);
      // Intact downtrend, price bounced back to EMA21 zone
      return e9 < e21 && e21 < e55 && price >= e21 * 0.995 && price <= e21 * 1.007 && r >= 38 && r <= 62;
    }
  }
  return false;
}

// ─── Option premium model ──────────────────────────────────────────────────────

function estimatePremium(price: number, iv: number, dteDays: number): number {
  const T = dteDays / 252;
  const premium = price * iv * Math.sqrt(T) * 0.4;
  return Math.max(price * 0.002, Math.min(price * 0.05, premium));
}

function markPremium(
  entryPremium: number,
  entryPrice: number,
  currentPrice: number,
  optionType: "CALL" | "PUT",
  barsHeld: number,
): number {
  const direction = optionType === "CALL" ? 1 : -1;
  const premiumDelta = 0.5 * (currentPrice - entryPrice) * direction;
  const thetaDecay = entryPremium * 0.002 * Math.max(0, barsHeld - 1);
  return Math.max(entryPremium * 0.04, entryPremium + premiumDelta - thetaDecay);
}

// ─── Engine internal types ─────────────────────────────────────────────────────

interface InternalPosition {
  id: string;
  strategyId: number;
  strategyName: string;
  commodityId: string;
  optionType: "CALL" | "PUT";
  strike: number;
  entryPremium: number;
  currentPremium: number;
  tpPremium: number;
  slPremium: number;
  lots: number;
  quantity: number;
  costBasis: number;
  entryPrice: number;
  entryTime: number;
  expiryTime: number;
  unrealizedPnl: number;
  peakGainPct: number;
  iv: number;
  barsHeld: number;
}

interface ShadowPosition {
  entryPremium: number;
  entryPrice: number;
  tpPremium: number;
  slPremium: number;
  entryTime: number;
  barsHeld: number;
}

interface InternalStratState {
  def: StratDef;
  position: InternalPosition | null;
  shadowPosition: ShadowPosition | null;
  status: "WARMING" | "READY" | "IN_POSITION" | "COOLING";
  rosterState: MCXRosterState;
  cooldownUntil: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  lastTradeAt: number;
  score: number;
  consecutiveLosses: number;
  shadowTrades: number;
  shadowWins: number;
}

interface InternalTrade {
  id: string;
  strategyId: number;
  strategyName: string;
  commodityId: string;
  optionType: "CALL" | "PUT";
  strike: number;
  entryPremium: number;
  exitPremium: number;
  lots: number;
  costBasis: number;
  netPnl: number;
  returnPct: number;
  entryPrice: number;
  exitPrice: number;
  entryTime: number;
  exitTime: number;
  exitReason: string;
}

interface CommodityState {
  minuteBars: number[];
  lastPrice: number;
  lastMinute: number;
  open: number;
  high: number;
  low: number;
  close: number;
  change: number;
  changePct: number;
}

interface EngineRef {
  commodities: Map<string, CommodityState>;
  strategies: InternalStratState[];
  positions: Map<string, InternalPosition>;
  trades: InternalTrade[];
  balance: number;
  seq: number;
  totalWins: number;
  totalLosses: number;
  totalRealizedPnl: number;
}

type MCXDbCommodityState = {
  id: string;
  minuteBars: number[];
  lastPrice: number;
  lastMinute: number;
  open: number;
  high: number;
  low: number;
  close: number;
  change: number;
  changePct: number;
};

type MCXDbStrategy = {
  id: number;
  status: "WARMING" | "READY" | "IN_POSITION" | "COOLING";
  rosterState?: MCXRosterState;
  cooldownUntil: number;
  totalTrades: number;
  wins: number;
  losses: number;
  totalPnl: number;
  winRate: number;
  lastTradeAt: number;
  score: number;
  consecutiveLosses: number;
  shadowTrades?: number;
  shadowWins?: number;
};

type MCXDbPayload = {
  balance: number;
  totalWins: number;
  totalLosses: number;
  totalPnl: number;
  tradeSeq: number;
  commodities: MCXDbCommodityState[];
  positions: InternalPosition[];
  trades: InternalTrade[];
  strategies: MCXDbStrategy[];
};

// ─── Entry confirmation (fixed for commodity IV scale) ────────────────────────

function passesEntryConfirmation(def: StratDef, bars: number[], price: number): boolean {
  const category = categoryForSignal(def.signal);
  const e9 = ema(bars, 9);
  const r = rsi(bars, 14);
  const n = bars.length;
  const bullish = def.optionType === "CALL";

  switch (category) {
    case "Momentum":
    case "Breakout":
      // Removed EMA stack (e9 >= e21) requirement — too restrictive in ranging markets.
      // Signal already confirms price > ema9 (for BULL_MOM). Just gate on RSI to avoid
      // buying into extreme overbought / selling into extreme oversold.
      return bullish
        ? price >= e9 * 0.998 && r < 72
        : price <= e9 * 1.002 && r > 28;
    case "Mean Reversion":
      // Price IS expected to be below mean when oversold (or above when overbought).
      // Check for a one-bar price reversal — signal already gates RSI extremes.
      return bullish
        ? n >= 2 && price > bars[n - 2] && r <= 62
        : n >= 2 && price < bars[n - 2] && r >= 38;
    default:
      return true;
  }
}

// ─── Roster helpers ────────────────────────────────────────────────────────────

function initialRosterState(def: StratDef): MCXRosterState {
  return def.startAsWatchlist ? "WATCHLIST" : "ACTIVE";
}

function canPromoteToActive(strat: InternalStratState): boolean {
  return (
    strat.shadowTrades >= MCX_SHADOW_PROMOTE_TRADES &&
    strat.shadowTrades > 0 &&
    strat.shadowWins / strat.shadowTrades >= MCX_SHADOW_PROMOTE_WINRATE
  );
}

// ─── Hold time by commodity ───────────────────────────────────────────────────

function holdMinutesFor(commodityId: string): number {
  switch (commodityId) {
    case "NATURALGAS": return 90;
    case "CRUDEOIL":
    case "COPPER":
    case "ALUMINIUM":
    case "ZINC":
    case "LEAD":       return 120;
    case "NICKEL":     return 150;
    default:           return 150;
  }
}

// ─── Exit resolver ────────────────────────────────────────────────────────────

function resolveExit(
  pos: InternalPosition,
  def: StratDef,
  currentPremium: number,
  now: number,
): { reason: string; exitPremium: number } | null {
  const gainPct = pos.entryPremium > 0 ? (currentPremium - pos.entryPremium) / pos.entryPremium : 0;
  const netGainPct = gainPct - (MCX_ROUND_TRIP_FEE_PCT + MCX_BID_ASK_SPREAD_PCT) / 100;
  const maxHoldMs = holdMinutesFor(pos.commodityId) * 60 * 1000;
  const timeProgress = maxHoldMs > 0 ? Math.min(1, (now - pos.entryTime) / maxHoldMs) : 0;
  const profitLockThreshold = Math.max(MCX_LATE_EXIT_MIN_GAIN, def.tpPct * MCX_PROFIT_LOCK_SHARE);

  if (currentPremium >= pos.tpPremium) return { reason: "TP", exitPremium: pos.tpPremium };
  if (currentPremium <= pos.slPremium) return { reason: "SL", exitPremium: pos.slPremium };
  if (netGainPct <= 0) return null;
  if (timeProgress >= MCX_GRIND_EXIT_PROGRESS && gainPct >= Math.max(MCX_LATE_EXIT_MIN_GAIN, def.tpPct * MCX_GRIND_EXIT_SHARE)) {
    return { reason: "PROFIT_LOCK", exitPremium: currentPremium };
  }
  if (timeProgress >= MCX_PROFIT_LOCK_PROGRESS && gainPct >= profitLockThreshold) {
    return { reason: "PROFIT_LOCK", exitPremium: currentPremium };
  }
  if (pos.peakGainPct >= Math.max(MCX_LATE_EXIT_MIN_GAIN, def.tpPct * 0.4) && netGainPct > 0 && gainPct <= pos.peakGainPct * (1 - MCX_TRAIL_GIVEBACK_SHARE)) {
    return { reason: "TRAIL_STOP", exitPremium: currentPremium };
  }
  if (timeProgress >= MCX_LATE_EXIT_PROGRESS && gainPct >= MCX_LATE_EXIT_MIN_GAIN) {
    return { reason: "LATE_EXIT", exitPremium: currentPremium };
  }
  if (now >= pos.expiryTime || (maxHoldMs > 0 && now - pos.entryTime >= maxHoldMs)) {
    return { reason: "TIME_EXIT", exitPremium: currentPremium };
  }
  return null;
}

// Same exit logic reused for shadow positions
function resolveShadowExit(
  shadow: ShadowPosition,
  def: StratDef,
  currentPremium: number,
  now: number,
): "WIN" | "LOSS" | null {
  if (currentPremium >= shadow.tpPremium) return "WIN";
  if (currentPremium <= shadow.slPremium) return "LOSS";
  const maxHoldMs = holdMinutesFor(def.commodityId) * 60 * 1000;
  if (now - shadow.entryTime >= maxHoldMs) {
    return currentPremium > shadow.entryPremium ? "WIN" : "LOSS";
  }
  return null;
}

// ─── Engine initializer ───────────────────────────────────────────────────────

function initEngine(): EngineRef {
  const commodities = new Map<string, CommodityState>();
  for (const c of MCX_COMMODITIES) {
    commodities.set(c.id, { minuteBars: [], lastPrice: 0, lastMinute: 0, open: 0, high: 0, low: 0, close: 0, change: 0, changePct: 0 });
  }
  return {
    commodities,
    strategies: STRAT_DEFS.map((def) => ({
      def,
      position: null,
      shadowPosition: null,
      status: "WARMING",
      rosterState: initialRosterState(def),
      cooldownUntil: 0,
      totalTrades: 0,
      wins: 0,
      losses: 0,
      totalPnl: 0,
      winRate: 0,
      lastTradeAt: 0,
      score: 0,
      consecutiveLosses: 0,
      shadowTrades: 0,
      shadowWins: 0,
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

// ─── Persistence ──────────────────────────────────────────────────────────────

function buildPersistedPayload(eng: EngineRef): MCXDbPayload {
  return {
    balance: eng.balance,
    totalWins: eng.totalWins,
    totalLosses: eng.totalLosses,
    totalPnl: eng.totalRealizedPnl,
    tradeSeq: eng.seq,
    commodities: MCX_COMMODITIES.map((commodity) => {
      const state = eng.commodities.get(commodity.id);
      return {
        id: commodity.id,
        minuteBars: state?.minuteBars ?? [],
        lastPrice: state?.lastPrice ?? 0,
        lastMinute: state?.lastMinute ?? 0,
        open: state?.open ?? 0,
        high: state?.high ?? 0,
        low: state?.low ?? 0,
        close: state?.close ?? 0,
        change: state?.change ?? 0,
        changePct: state?.changePct ?? 0,
      };
    }),
    positions: [...eng.positions.values()],
    trades: eng.trades.slice(0, 500),
    strategies: eng.strategies.map((strategy) => ({
      id: strategy.def.id,
      status: strategy.status,
      rosterState: strategy.rosterState,
      cooldownUntil: strategy.cooldownUntil,
      totalTrades: strategy.totalTrades,
      wins: strategy.wins,
      losses: strategy.losses,
      totalPnl: strategy.totalPnl,
      winRate: strategy.winRate,
      lastTradeAt: strategy.lastTradeAt,
      score: strategy.score,
      consecutiveLosses: strategy.consecutiveLosses,
      shadowTrades: strategy.shadowTrades,
      shadowWins: strategy.shadowWins,
    })),
  };
}

async function saveMCXState(eng: EngineRef): Promise<void> {
  try {
    await fetch("/api/mcx/state", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(buildPersistedPayload(eng)),
    });
  } catch {
    // non-critical
  }
}

async function loadMCXState(eng: EngineRef): Promise<boolean> {
  try {
    const res = await fetch("/api/mcx/state");
    if (!res.ok) return false;
    const data = await res.json() as { ok: boolean; found?: boolean; disabled?: boolean; state?: MCXDbPayload };
    if (!data.ok) return false;
    if (data.disabled) return false;
    if (!data.found || !data.state) return true;

    const saved = data.state;
    eng.balance = saved.balance;
    eng.totalWins = saved.totalWins;
    eng.totalLosses = saved.totalLosses;
    eng.totalRealizedPnl = saved.totalPnl;
    eng.seq = saved.tradeSeq;
    eng.trades = (saved.trades ?? []).slice(0, 500);
    eng.positions.clear();

    for (const commodity of saved.commodities ?? []) {
      eng.commodities.set(commodity.id, {
        minuteBars: commodity.minuteBars ?? [],
        lastPrice: commodity.lastPrice,
        lastMinute: commodity.lastMinute,
        open: commodity.open,
        high: commodity.high,
        low: commodity.low,
        close: commodity.close,
        change: commodity.change,
        changePct: commodity.changePct,
      });
    }

    for (const persisted of saved.positions ?? []) {
      eng.positions.set(persisted.id, persisted);
      const strategy = eng.strategies.find((item) => item.def.id === persisted.strategyId);
      if (strategy) strategy.position = persisted;
    }

    for (const strategy of eng.strategies) {
      const persisted = (saved.strategies ?? []).find((item) => item.id === strategy.def.id);
      if (!persisted) continue;
      strategy.status = persisted.status;
      // Restore roster state — fall back to initial if not saved
      strategy.rosterState = persisted.rosterState ?? initialRosterState(strategy.def);
      strategy.cooldownUntil = persisted.cooldownUntil;
      strategy.totalTrades = persisted.totalTrades;
      strategy.wins = persisted.wins;
      strategy.losses = persisted.losses;
      strategy.totalPnl = persisted.totalPnl;
      strategy.winRate = persisted.winRate;
      strategy.lastTradeAt = persisted.lastTradeAt;
      strategy.score = persisted.score;
      strategy.consecutiveLosses = persisted.consecutiveLosses;
      strategy.shadowTrades = persisted.shadowTrades ?? 0;
      strategy.shadowWins = persisted.shadowWins ?? 0;
    }

    return true;
  } catch {
    return false;
  }
}

// ─── Position sizing ──────────────────────────────────────────────────────────

function mcxSizeMultiplier(strat: InternalStratState): number {
  let multiplier = 1;
  if (strat.totalTrades >= 6 && strat.winRate >= 55) multiplier += 0.15;
  if (strat.totalTrades >= 12 && strat.winRate >= 60) multiplier += 0.2;
  const commodity = MCX_COMMODITY_MAP.get(strat.def.commodityId);
  if (strat.totalTrades > 0 && commodity) {
    const avgPnlRatio = strat.totalPnl / (strat.totalTrades * commodity.positionINR);
    if (avgPnlRatio > 0.08) multiplier += 0.18;
    if (avgPnlRatio < -0.04) multiplier -= 0.15;
  }
  multiplier -= strat.consecutiveLosses * 0.1;
  return mcxClamp(multiplier, MCX_MIN_SIZE_MULTIPLIER, MCX_MAX_SIZE_MULTIPLIER);
}

function mcxCooldownMs(strat: InternalStratState, won: boolean): number {
  const base = strat.def.cooldownSecs * 1000;
  if (won) return base;
  return Math.round(base * (1 + strat.consecutiveLosses * MCX_LOSS_COOLDOWN_PENALTY));
}

function shouldPauseMCXStrategy(strat: InternalStratState): boolean {
  return strat.totalTrades >= MCX_UNDERPERFORMING_MIN_TRADES && strat.totalPnl < 0 && strat.winRate < MCX_UNDERPERFORMING_MAX_WINRATE;
}

// ─── Position management ──────────────────────────────────────────────────────

function closePosition(eng: EngineRef, strat: InternalStratState, exitPremium: number, exitPrice: number, reason: string, now: number) {
  const pos = strat.position;
  if (!pos) return;
  if (pos.entryPremium <= 0) { strat.position = null; eng.positions.delete(pos.id); return; }
  const grossPnl = (exitPremium - pos.entryPremium) * pos.quantity;
  const fees = pos.costBasis * (MCX_ROUND_TRIP_FEE_PCT + MCX_BID_ASK_SPREAD_PCT) / 100;
  const netPnl = grossPnl - fees;
  const returnPct = ((exitPremium - pos.entryPremium) / pos.entryPremium) * 100 - MCX_ROUND_TRIP_FEE_PCT - MCX_BID_ASK_SPREAD_PCT;

  eng.trades.unshift({
    id: pos.id, strategyId: pos.strategyId, strategyName: pos.strategyName,
    commodityId: pos.commodityId, optionType: pos.optionType, strike: pos.strike,
    entryPremium: pos.entryPremium, exitPremium,
    lots: pos.lots, costBasis: pos.costBasis,
    netPnl, returnPct,
    entryPrice: pos.entryPrice, exitPrice,
    entryTime: pos.entryTime, exitTime: now, exitReason: reason,
  });
  if (eng.trades.length > 500) eng.trades.length = 500;

  eng.balance += exitPremium * pos.quantity;
  eng.totalRealizedPnl += netPnl;
  strat.totalTrades++;
  if (netPnl >= 0) {
    strat.wins++; eng.totalWins++; strat.consecutiveLosses = 0;
  } else {
    strat.losses++; eng.totalLosses++; strat.consecutiveLosses += 1;
  }
  strat.totalPnl += netPnl;
  strat.winRate = strat.totalTrades > 0 ? (strat.wins / strat.totalTrades) * 100 : 0;
  strat.lastTradeAt = now;
  strat.cooldownUntil = now + mcxCooldownMs(strat, netPnl >= 0);
  strat.status = "COOLING";
  strat.position = null;
  eng.positions.delete(pos.id);

  // After live losses, consider demoting ACTIVE to WATCHLIST
  if (strat.consecutiveLosses >= MCX_LOSS_STREAK_DISABLE_THRESHOLD && strat.rosterState === "ACTIVE") {
    strat.rosterState = "DISABLED";
    strat.cooldownUntil = Math.max(strat.cooldownUntil, now + MCX_LOSS_STREAK_COOLDOWN_MS);
  }
}

function openPosition(eng: EngineRef, strat: InternalStratState, commodity: MCXCommodity, price: number, iv: number, now: number): boolean {
  const premium = estimatePremium(price, iv, commodity.dteDays);
  if (premium <= 0) return false;
  const costPerLot = premium * commodity.lotSize;
  let lots = costPerLot > 0 ? Math.max(1, Math.round((commodity.positionINR * mcxSizeMultiplier(strat)) / costPerLot)) : 1;
  let cost = premium * commodity.lotSize * lots;
  while (cost > eng.balance && lots > 1) { lots--; cost = premium * commodity.lotSize * lots; }
  if (eng.balance < cost) return false;

  eng.seq++;
  eng.balance -= cost;
  const id = `MCXOPT-${now}-${eng.seq}`;
  const quantity = commodity.lotSize * lots;
  const strike = Math.round(price / commodity.strikeStep) * commodity.strikeStep;

  const pos: InternalPosition = {
    id, strategyId: strat.def.id, strategyName: strat.def.name, commodityId: commodity.id,
    optionType: strat.def.optionType, strike,
    entryPremium: premium, currentPremium: premium,
    tpPremium: premium * (1 + strat.def.tpPct),
    slPremium: premium * (1 - strat.def.slPct),
    lots, quantity, costBasis: cost,
    entryPrice: price, entryTime: now,
    expiryTime: now + commodity.dteDays * 24 * 60 * 60 * 1000,
    unrealizedPnl: 0, peakGainPct: 0, iv, barsHeld: 0,
  };

  strat.position = pos;
  strat.status = "IN_POSITION";
  eng.positions.set(id, pos);
  return true;
}

function openShadowPosition(strat: InternalStratState, commodity: MCXCommodity, price: number, iv: number, now: number) {
  const premium = estimatePremium(price, iv, commodity.dteDays);
  if (premium <= 0) return;
  strat.shadowPosition = {
    entryPremium: premium,
    entryPrice: price,
    tpPremium: premium * (1 + strat.def.tpPct),
    slPremium: premium * (1 - strat.def.slPct),
    entryTime: now,
    barsHeld: 0,
  };
}

// ─── Display builders ─────────────────────────────────────────────────────────

function buildPositions(eng: EngineRef): MCXPosition[] {
  return eng.strategies
    .filter((s) => s.position)
    .map((s) => {
      const pos = s.position!;
      const commodity = MCX_COMMODITY_MAP.get(pos.commodityId);
      return {
        id: pos.id, commodityId: pos.commodityId, commodityName: commodity?.name ?? pos.commodityId,
        strategyId: pos.strategyId, strategyName: pos.strategyName, optionType: pos.optionType,
        strike: pos.strike, entryPremium: pos.entryPremium, currentPremium: pos.currentPremium,
        tpPremium: pos.tpPremium, slPremium: pos.slPremium,
        quantity: pos.quantity, lots: pos.lots, costBasis: pos.costBasis,
        entryPrice: pos.entryPrice,
        currentPrice: eng.commodities.get(pos.commodityId)?.lastPrice ?? pos.entryPrice,
        entryTime: new Date(pos.entryTime).toISOString(), unrealizedPnl: pos.unrealizedPnl,
        iv: pos.iv, delta: 0.5,
      };
    });
}

function buildTrades(eng: EngineRef): MCXTrade[] {
  return eng.trades.slice(0, 200).map((t) => {
    const commodity = MCX_COMMODITY_MAP.get(t.commodityId);
    return {
      id: t.id, commodityId: t.commodityId, commodityName: commodity?.name ?? t.commodityId,
      strategyId: t.strategyId, strategyName: t.strategyName, optionType: t.optionType,
      strike: t.strike, entryPremium: t.entryPremium, exitPremium: t.exitPremium,
      lots: t.lots, costBasis: t.costBasis, netPnl: t.netPnl, returnPct: t.returnPct,
      entryPrice: t.entryPrice, exitPrice: t.exitPrice,
      entryTime: new Date(t.entryTime).toISOString(), exitTime: new Date(t.exitTime).toISOString(),
      exitReason: t.exitReason,
    };
  });
}

function buildStrategies(eng: EngineRef): MCXStrategyStatus[] {
  return eng.strategies.map((s) => {
    const commodity = MCX_COMMODITY_MAP.get(s.def.commodityId);
    return {
      strategyId: s.def.id, name: s.def.name,
      commodityId: s.def.commodityId, commodityName: commodity?.name ?? s.def.commodityId,
      optionType: s.def.optionType,
      status: s.status === "WARMING" ? "WARMING"
            : s.status === "COOLING" ? "COOLING"
            : s.status === "IN_POSITION" ? "IN_POSITION"
            : "READY",
      rosterState: s.rosterState,
      score: s.score, totalTrades: s.totalTrades, wins: s.wins, losses: s.losses,
      winRate: s.winRate, totalPnl: s.totalPnl, hasPosition: !!s.position,
      shadowTrades: s.shadowTrades, shadowWins: s.shadowWins,
      hasShadowPosition: !!s.shadowPosition,
    };
  });
}

function buildStats(eng: EngineRef): MCXStats {
  let openPositions = 0, unrealizedPnl = 0, openCost = 0;
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
    ? (eng.totalWins / (eng.totalWins + eng.totalLosses)) * 100 : 0;
  return {
    balance: eng.balance, equity, totalTrades, openPositions,
    totalWins: eng.totalWins, totalLosses: eng.totalLosses,
    winRate: totalWinRate, totalPnl: eng.totalRealizedPnl, unrealizedPnl,
    capitalUtilizationPct: INITIAL_BALANCE > 0 ? (openCost / INITIAL_BALANCE) * 100 : 0,
    sessionOpen: isMCXSessionOpen(),
    maxConcurrentPositions: MAX_CONCURRENT,
  };
}

function buildQuotes(eng: EngineRef): MCXCommodityQuote[] {
  return MCX_COMMODITIES.map((c) => {
    const state = eng.commodities.get(c.id)!;
    const bars = state.minuteBars;
    const latest = state.lastPrice > 0 ? state.lastPrice : bars[bars.length - 1] ?? 0;
    const barsForSignal = latest > 0 ? [...bars.slice(-MAX_BARS), latest] : bars;
    const rsiValue = barsForSignal.length >= 15 ? rsi(barsForSignal, 14) : 50;
    const momentumValue = barsForSignal.length > 6 ? momentum(barsForSignal, 5) * 100 : 0;
    const volatilityValue = barsForSignal.length >= 20 && latest > 0 ? (stddev(barsForSignal.slice(-20)) / latest) * 100 : 0;
    const fastEma = barsForSignal.length >= 9 ? ema(barsForSignal, 9) : 0;
    const slowEma = barsForSignal.length >= 21 ? ema(barsForSignal, 21) : 0;
    const bias: MCXCommodityQuote["bias"] = barsForSignal.length < 15 || latest <= 0
      ? "WARMING"
      : latest > fastEma && fastEma > slowEma && momentumValue > 0
        ? "BULLISH"
        : latest < fastEma && fastEma < slowEma && momentumValue < 0
          ? "BEARISH"
          : "NEUTRAL";
    return {
      id: c.id, name: c.name, unit: c.unit, sector: c.sector,
      bias,
      rsi: rsiValue,
      momentumPct: momentumValue,
      volatilityPct: volatilityValue,
      ltp: state.lastPrice,
      open: state.open, high: state.high, low: state.low, close: state.close,
      change: state.change, changePct: state.changePct, barCount: state.minuteBars.length,
    };
  });
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export default function useMCXEngine(_refreshKey = 0) {
  void _refreshKey;
  const engRef = useRef<EngineRef>(initEngine());
  const lastSavedSignatureRef = useRef("");
  const dbLoadedRef = useRef(false);
  const [positions, setPositions] = useState<MCXPosition[]>([]);
  const [trades, setTrades] = useState<MCXTrade[]>([]);
  const [strategies, setStrategies] = useState<MCXStrategyStatus[]>(() => buildStrategies(engRef.current));
  const [stats, setStats] = useState<MCXStats>(() => buildStats(engRef.current));
  const [quotes, setQuotes] = useState<MCXCommodityQuote[]>(() => buildQuotes(engRef.current));
  const [diagnostics, setDiagnostics] = useState<MCXDiagnostics>({
    ltpError: "",
    candleIssues: [],
    lastLtpAt: null,
  });

  const pushDisplayState = useCallback(() => {
    const eng = engRef.current;
    setPositions(buildPositions(eng));
    setTrades(buildTrades(eng));
    setStrategies(buildStrategies(eng));
    setStats(buildStats(eng));
    setQuotes(buildQuotes(eng));
    if (!dbLoadedRef.current) return;
    const signature = JSON.stringify({
      balance: eng.balance,
      totalWins: eng.totalWins,
      totalLosses: eng.totalLosses,
      totalPnl: eng.totalRealizedPnl,
      tradeSeq: eng.seq,
      commodityBars: MCX_COMMODITIES.map((commodity) => ({
        id: commodity.id,
        bars: eng.commodities.get(commodity.id)?.minuteBars.length ?? 0,
        lastMinute: eng.commodities.get(commodity.id)?.lastMinute ?? 0,
      })),
      openPositions: [...eng.positions.keys()],
      tradeIds: eng.trades.slice(0, 500).map((trade) => trade.id),
    });
    if (signature !== lastSavedSignatureRef.current) {
      lastSavedSignatureRef.current = signature;
      void saveMCXState(eng);
    }
  }, []);

  useEffect(() => {
    void loadMCXState(engRef.current).then((loaded) => {
      dbLoadedRef.current = loaded;
      if (loaded) {
        lastSavedSignatureRef.current = JSON.stringify({
          balance: engRef.current.balance,
          totalWins: engRef.current.totalWins,
          totalLosses: engRef.current.totalLosses,
          totalPnl: engRef.current.totalRealizedPnl,
          tradeSeq: engRef.current.seq,
          commodityBars: MCX_COMMODITIES.map((commodity) => ({
            id: commodity.id,
            bars: engRef.current.commodities.get(commodity.id)?.minuteBars.length ?? 0,
            lastMinute: engRef.current.commodities.get(commodity.id)?.lastMinute ?? 0,
          })),
          openPositions: [...engRef.current.positions.keys()],
          tradeIds: engRef.current.trades.slice(0, 500).map((trade) => trade.id),
        });
      }
      pushDisplayState();
    });
  }, [pushDisplayState]);

  // ── Engine tick ──────────────────────────────────────────────────────────
  const engineTick = useCallback(() => {
    const eng = engRef.current;
    const now = Date.now();
    const sessionOpen = isMCXSessionOpen();
    let openCount = eng.strategies.filter((s) => s.position).length;

    // ── Mark open positions, check TP/SL/Expiry ──────────────────────────
    for (const strat of eng.strategies) {
      if (!strat.position) continue;
      const pos = strat.position;
      const state = eng.commodities.get(pos.commodityId);
      if (!state || state.lastPrice <= 0) continue;
      const price = state.lastPrice;
      const commodity = MCX_COMMODITY_MAP.get(pos.commodityId)!;

      // eslint-disable-next-line react-hooks/immutability
      pos.barsHeld++;
      pos.currentPremium = markPremium(pos.entryPremium, pos.entryPrice, price, pos.optionType, pos.barsHeld);
      pos.unrealizedPnl = (pos.currentPremium - pos.entryPremium) * pos.quantity;
      if (pos.entryPremium > 0) {
        pos.peakGainPct = Math.max(pos.peakGainPct, (pos.currentPremium - pos.entryPremium) / pos.entryPremium);
      }

      const exit = resolveExit(pos, strat.def, pos.currentPremium, now);
      if (exit) {
        closePosition(eng, strat, exit.exitPremium, price, exit.reason, now);
        openCount--;
      }
      void commodity;
    }

    // ── Mark shadow positions for WATCHLIST strategies ───────────────────
    for (const strat of eng.strategies) {
      if (strat.rosterState !== "WATCHLIST" || !strat.shadowPosition) continue;
      const shadow = strat.shadowPosition;
      const state = eng.commodities.get(strat.def.commodityId);
      if (!state || state.lastPrice <= 0) continue;
      const price = state.lastPrice;
      shadow.barsHeld++;
      const currentPremium = markPremium(shadow.entryPremium, shadow.entryPrice, price, strat.def.optionType, shadow.barsHeld);
      const result = resolveShadowExit(shadow, strat.def, currentPremium, now);
      if (result !== null) {
        strat.shadowTrades++;
        if (result === "WIN") strat.shadowWins++;
        strat.shadowPosition = null;
        // Promote to ACTIVE if qualified
        if (canPromoteToActive(strat)) {
          strat.rosterState = "ACTIVE";
        }
      }
    }

    // ── Roster management for DISABLED strategies ─────────────────────────
    for (const strat of eng.strategies) {
      if (strat.rosterState === "DISABLED" && now >= strat.cooldownUntil) {
        // Return to WATCHLIST after disable cooldown, reset consecutive losses
        strat.rosterState = "WATCHLIST";
        strat.consecutiveLosses = 0;
        strat.shadowTrades = 0;
        strat.shadowWins = 0;
      }
    }

    // Note: sessionOpen is available for display but we do NOT hard-block here.
    // When MCX is closed the LTP API returns stale/no data, so bars don't change
    // and signals won't fire naturally. Hard-blocking prevented testing entirely.
    void sessionOpen;

    // ── Evaluate signals ─────────────────────────────────────────────────
    for (const strat of eng.strategies) {
      if (strat.position) { strat.status = "IN_POSITION"; continue; }

      // Disabled strategies don't trade
      if (strat.rosterState === "DISABLED") { strat.score = 0; continue; }

      if (shouldPauseMCXStrategy(strat)) {
        strat.status = "COOLING";
        strat.score = 0;
        strat.cooldownUntil = Math.max(strat.cooldownUntil, now + MCX_UNDERPERFORMING_PAUSE_MS);
        continue;
      }

      if (strat.status === "COOLING") {
        if (now < strat.cooldownUntil) continue;
        strat.status = "READY";
      }

      const state = eng.commodities.get(strat.def.commodityId);
      if (!state || state.lastPrice <= 0) { strat.status = "WARMING"; continue; }
      const bars = state.minuteBars;

      if (bars.length < strat.def.minBars) { strat.status = "WARMING"; continue; }
      strat.status = "READY";

      const price = state.lastPrice;
      const fires = evalSignal(strat.def.signal, bars, price);
      const confirmed = fires && passesEntryConfirmation(strat.def, bars, price);
      strat.score = confirmed ? 82 : fires ? 56 : 0;

      if (!confirmed) continue;

      const commodity = MCX_COMMODITY_MAP.get(strat.def.commodityId)!;
      const recentStd = bars.length >= 20 ? stddev(bars.slice(-20)) : 0;
      const iv = recentStd > 0
        ? Math.max(commodity.iv * 0.6, Math.min(commodity.iv * 2, (recentStd / price) * Math.sqrt(252 * 375)))
        : commodity.iv;

      if (strat.rosterState === "WATCHLIST") {
        // Open shadow position if none exists
        if (!strat.shadowPosition) {
          openShadowPosition(strat, commodity, price, iv, now);
        }
      } else if (strat.rosterState === "ACTIVE") {
        // Open real position
        if (openCount >= MAX_CONCURRENT) continue;
        if (eng.balance < 5000) continue;
        if (openPosition(eng, strat, commodity, price, iv, now)) {
          openCount++;
        }
      }
    }

    pushDisplayState();
  }, [pushDisplayState]);

  // ── Update commodity state from LTP poll ─────────────────────────────────
  const feedLTP = useCallback((items: MCXLTPItem[]) => {
    const eng = engRef.current;
    const nowMinute = Math.floor(Date.now() / 60000);

    for (const item of items) {
      if (item.ltp <= 0) continue;
      const state = eng.commodities.get(item.id);
      if (!state) continue;
      state.lastPrice = item.ltp;
      state.open = item.open;
      state.high = item.high;
      state.low = item.low;
      state.close = item.close;
      state.change = item.change;
      state.changePct = item.changePct;

      // Build 1-min bars
      if (nowMinute !== state.lastMinute) {
        state.lastMinute = nowMinute;
        state.minuteBars.push(item.ltp);
        if (state.minuteBars.length > MAX_BARS) state.minuteBars.shift();
      }
    }
  }, []);

  // ── Poll LTP ──────────────────────────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;
    const poll = async () => {
      if (cancelled) return;
      try {
        const res = await fetch("/api/mcx/ltp");
        if (cancelled) return;
        if (!res.ok) {
          setDiagnostics((curr) => ({ ...curr, ltpError: `MCX LTP route returned ${res.status}` }));
          return;
        }
        const data = await res.json() as { ok: boolean; data?: MCXLTPItem[]; error?: string };
        if (!data.ok || !data.data) {
          setDiagnostics((curr) => ({ ...curr, ltpError: data.error ?? "MCX LTP unavailable" }));
          return;
        }
        feedLTP(data.data);
        setDiagnostics((curr) => ({ ...curr, ltpError: "", lastLtpAt: new Date().toISOString() }));
      } catch (err) {
        const message = err instanceof Error ? err.message : "MCX LTP request failed";
        setDiagnostics((curr) => ({ ...curr, ltpError: message }));
      }
    };
    void poll();
    const id = setInterval(() => void poll(), TICK_MS);
    return () => { cancelled = true; clearInterval(id); };
  }, [feedLTP]);

  // ── Seed historical candles for each commodity ────────────────────────────
  useEffect(() => {
    const seedAll = async () => {
      const nextIssues: MCXCandleIssue[] = [];
      for (const commodity of MCX_COMMODITIES) {
        try {
          const res = await fetch(`/api/mcx/candles?commodity=${commodity.id}&interval=ONE_MINUTE`);
          if (!res.ok) {
            nextIssues.push({
              commodityId: commodity.id,
              commodityName: commodity.name,
              error: `MCX candles route returned ${res.status}`,
            });
            continue;
          }
          const data = await res.json() as { ok: boolean; candles?: MCXCandle[]; error?: string };
          if (!data.ok) {
            nextIssues.push({
              commodityId: commodity.id,
              commodityName: commodity.name,
              error: data.error ?? "Historical candle warm-up failed",
            });
            continue;
          }
          if (!data.candles?.length) continue;
          const eng = engRef.current;
          const state = eng.commodities.get(commodity.id);
          if (!state || state.minuteBars.length > 0) continue;
          const closes = data.candles.map((c) => c.close).filter((v) => v > 0);
          state.minuteBars = closes.slice(-MAX_BARS);
          state.lastPrice = closes[closes.length - 1] ?? 0;
          const lastCandle = data.candles[data.candles.length - 1];
          state.lastMinute = Math.floor((lastCandle?.time ?? 0) / 60000);
        } catch (err) {
          const message = err instanceof Error ? err.message : "Historical candle warm-up failed";
          nextIssues.push({
            commodityId: commodity.id,
            commodityName: commodity.name,
            error: message,
          });
        }
        await new Promise<void>((r) => setTimeout(r, 300));
      }
      setDiagnostics((curr) => ({ ...curr, candleIssues: nextIssues }));
      pushDisplayState();
    };
    void seedAll();
  }, [pushDisplayState]);

  // ── Engine tick interval ──────────────────────────────────────────────────
  useEffect(() => {
    const id = setInterval(() => void engineTick(), TICK_MS);
    return () => clearInterval(id);
  }, [engineTick]);

  useEffect(() => {
    const id = setInterval(() => {
      if (dbLoadedRef.current) void saveMCXState(engRef.current);
    }, 60_000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    const onUnload = () => {
      if (!dbLoadedRef.current) return;
      const payload = JSON.stringify(buildPersistedPayload(engRef.current));
      navigator.sendBeacon("/api/mcx/state", new Blob([payload], { type: "application/json" }));
    };
    window.addEventListener("beforeunload", onUnload);
    return () => window.removeEventListener("beforeunload", onUnload);
  }, []);

  const clearAll = useCallback(() => {
    engRef.current = initEngine();
    lastSavedSignatureRef.current = "";
    if (dbLoadedRef.current) void saveMCXState(engRef.current);
    pushDisplayState();
  }, [pushDisplayState]);

  return { positions, trades, strategies, stats, quotes, diagnostics, clearAll };
}
