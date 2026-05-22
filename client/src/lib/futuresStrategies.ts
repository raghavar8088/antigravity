/**
 * Futures strategy definitions — CORE 20 WINNERS basket only.
 * All research / extended / generated pools have been removed.
 */

import type { FuturesStratDef } from "@/lib/futuresStratTypes";
import { BTC_FT_PREMIUM_DEFS } from "@/lib/btcFtPremiumStrategies";

export type { BtcFtTemplateId, FuturesStratDef, RegimeTag } from "@/lib/futuresStratTypes";

/** CORE 20 — curated, empirically-validated winners. */
const BASE_FUTURES_STRAT_DEFS: FuturesStratDef[] = [
  // ADX Trend
  { id: 91, name: "Trend_Continuation_Long", category: "Trend", signalKey: "TREND_CONT_LONG", slPct: 0.26, tpPct: 0.80, cooldownMin: 6, holdMinutes: 26, confluenceMin: 5 },
  { id: 92, name: "Trend_Continuation_Short", category: "Trend", signalKey: "TREND_CONT_SHORT", slPct: 0.26, tpPct: 0.80, cooldownMin: 6, holdMinutes: 26, confluenceMin: 5 },

  // Breakout
  { id: 95, name: "Breakout_Long", category: "Breakout", signalKey: "BREAKOUT_LONG", slPct: 0.32, tpPct: 0.85, cooldownMin: 5, holdMinutes: 20, confluenceMin: 5 },
  { id: 96, name: "Breakout_Short", category: "Breakout", signalKey: "BREAKOUT_SHORT", slPct: 0.32, tpPct: 0.85, cooldownMin: 5, holdMinutes: 20, confluenceMin: 5 },

  // MTF Trend
  { id: 111, name: "MTF_Trend_Align_Long", category: "MTF Trend", signalKey: "MTF_TREND_ALIGN_LONG", slPct: 0.26, tpPct: 0.82, cooldownMin: 6, holdMinutes: 32, confluenceMin: 4, requiresHtf: true },
  { id: 112, name: "MTF_Trend_Align_Short", category: "MTF Trend", signalKey: "MTF_TREND_ALIGN_SHORT", slPct: 0.26, tpPct: 0.82, cooldownMin: 6, holdMinutes: 32, confluenceMin: 4, requiresHtf: true },

  // MTF MACD
  { id: 117, name: "MTF_MACD_Align_Long", category: "MTF MACD", signalKey: "MTF_MACD_ALIGN_LONG", slPct: 0.28, tpPct: 0.74, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4, requiresHtf: true },
  { id: 118, name: "MTF_MACD_Align_Short", category: "MTF MACD", signalKey: "MTF_MACD_ALIGN_SHORT", slPct: 0.28, tpPct: 0.74, cooldownMin: 6, holdMinutes: 28, confluenceMin: 4, requiresHtf: true },

  // MTF ADX
  { id: 123, name: "MTF_ADX_Power_Long", category: "MTF ADX", signalKey: "MTF_ADX_POWER_LONG", slPct: 0.28, tpPct: 0.80, cooldownMin: 7, holdMinutes: 34, confluenceMin: 5, requiresHtf: true },
  { id: 124, name: "MTF_ADX_Power_Short", category: "MTF ADX", signalKey: "MTF_ADX_POWER_SHORT", slPct: 0.28, tpPct: 0.80, cooldownMin: 7, holdMinutes: 34, confluenceMin: 5, requiresHtf: true },

  // MTF Breakout
  { id: 125, name: "MTF_Breakout_Long", category: "MTF Break", signalKey: "MTF_BREAKOUT_LONG", slPct: 0.30, tpPct: 0.92, cooldownMin: 6, holdMinutes: 28, confluenceMin: 5, requiresHtf: true },
  { id: 126, name: "MTF_Breakout_Short", category: "MTF Break", signalKey: "MTF_BREAKOUT_SHORT", slPct: 0.30, tpPct: 0.92, cooldownMin: 6, holdMinutes: 28, confluenceMin: 5, requiresHtf: true },

  // Smart Money
  { id: 131, name: "SmartMoney_Accum_Long", category: "Smart Money", signalKey: "SM_ACCUM_LONG", slPct: 0.26, tpPct: 0.85, cooldownMin: 6, holdMinutes: 32, confluenceMin: 5 },
  { id: 132, name: "SmartMoney_Distrib_Short", category: "Smart Money", signalKey: "SM_DISTRIB_SHORT", slPct: 0.26, tpPct: 0.85, cooldownMin: 6, holdMinutes: 32, confluenceMin: 5 },

  // Order Flow
  { id: 133, name: "OrderFlow_Break_Long", category: "Order Flow", signalKey: "OF_BREAK_LONG", slPct: 0.28, tpPct: 0.90, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },
  { id: 134, name: "OrderFlow_Break_Short", category: "Order Flow", signalKey: "OF_BREAK_SHORT", slPct: 0.28, tpPct: 0.90, cooldownMin: 5, holdMinutes: 24, confluenceMin: 5 },

  // Wyckoff
  { id: 139, name: "Wyckoff_Spring_Long", category: "Wyckoff", signalKey: "WYCKOFF_SPRING_LONG", slPct: 0.28, tpPct: 0.95, cooldownMin: 8, holdMinutes: 38, confluenceMin: 5 },
  { id: 140, name: "Wyckoff_Upthrust_Short", category: "Wyckoff", signalKey: "WYCKOFF_UPTHRUST_SHORT", slPct: 0.28, tpPct: 0.95, cooldownMin: 8, holdMinutes: 38, confluenceMin: 5 },

  // Session
  { id: 151, name: "OpeningDrive_Long", category: "Session", signalKey: "OPEN_DRIVE_LONG", slPct: 0.32, tpPct: 0.72, cooldownMin: 3, holdMinutes: 18, confluenceMin: 4 },
  { id: 152, name: "OpeningDrive_Short", category: "Session", signalKey: "OPEN_DRIVE_SHORT", slPct: 0.32, tpPct: 0.72, cooldownMin: 3, holdMinutes: 18, confluenceMin: 4 },
];

export const FUTURES_STRAT_DEFS: readonly FuturesStratDef[] = [
  ...BASE_FUTURES_STRAT_DEFS,
  ...BTC_FT_PREMIUM_DEFS,
];
