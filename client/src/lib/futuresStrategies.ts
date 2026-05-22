/**
 * Futures strategy definitions — CORE 20 winners basket only.
 */

import type { FuturesStratDef } from "@/lib/futuresStratTypes";
import { BTC_FT_PREMIUM_DEFS } from "@/lib/btcFtPremiumStrategies";

export type { BtcFtTemplateId, FuturesStratDef, RegimeTag } from "@/lib/futuresStratTypes";

const BASE_FUTURES_STRAT_DEFS: FuturesStratDef[] = [
  { id: 91, name: "Trend_Continuation_Long",  category: "Trend",     signalKey: "TREND_CONT_LONG",        slPct: 0.50, tpPct: 1.50, cooldownMin: 8,  holdMinutes: 40, confluenceMin: 5 },
  { id: 92, name: "Trend_Continuation_Short", category: "Trend",     signalKey: "TREND_CONT_SHORT",       slPct: 0.50, tpPct: 1.50, cooldownMin: 8,  holdMinutes: 40, confluenceMin: 5 },
  { id: 95, name: "Breakout_Long",            category: "Breakout",  signalKey: "BREAKOUT_LONG",          slPct: 0.55, tpPct: 1.65, cooldownMin: 7,  holdMinutes: 32, confluenceMin: 5 },
  { id: 96, name: "Breakout_Short",           category: "Breakout",  signalKey: "BREAKOUT_SHORT",         slPct: 0.55, tpPct: 1.65, cooldownMin: 7,  holdMinutes: 32, confluenceMin: 5 },
  { id: 111, name: "MTF_Trend_Align_Long",    category: "MTF Trend", signalKey: "MTF_TREND_ALIGN_LONG",   slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 5, requiresHtf: true },
  { id: 112, name: "MTF_Trend_Align_Short",   category: "MTF Trend", signalKey: "MTF_TREND_ALIGN_SHORT",  slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 5, requiresHtf: true },
  { id: 117, name: "MTF_MACD_Align_Long",     category: "MTF MACD",  signalKey: "MTF_MACD_ALIGN_LONG",    slPct: 0.52, tpPct: 1.56, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true },
  { id: 118, name: "MTF_MACD_Align_Short",    category: "MTF MACD",  signalKey: "MTF_MACD_ALIGN_SHORT",   slPct: 0.52, tpPct: 1.56, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true },
  { id: 123, name: "MTF_ADX_Power_Long",      category: "MTF ADX",   signalKey: "MTF_ADX_POWER_LONG",     slPct: 0.52, tpPct: 1.56, cooldownMin: 9,  holdMinutes: 52, confluenceMin: 5, requiresHtf: true },
  { id: 124, name: "MTF_ADX_Power_Short",     category: "MTF ADX",   signalKey: "MTF_ADX_POWER_SHORT",    slPct: 0.52, tpPct: 1.56, cooldownMin: 9,  holdMinutes: 52, confluenceMin: 5, requiresHtf: true },
  { id: 125, name: "MTF_Breakout_Long",       category: "MTF Break", signalKey: "MTF_BREAKOUT_LONG",      slPct: 0.55, tpPct: 1.65, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true },
  { id: 126, name: "MTF_Breakout_Short",      category: "MTF Break", signalKey: "MTF_BREAKOUT_SHORT",     slPct: 0.55, tpPct: 1.65, cooldownMin: 8,  holdMinutes: 45, confluenceMin: 5, requiresHtf: true },
  { id: 131, name: "SmartMoney_Accum_Long",   category: "Smart Money", signalKey: "SM_ACCUM_LONG",        slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 6 },
  { id: 132, name: "SmartMoney_Distrib_Short",category: "Smart Money", signalKey: "SM_DISTRIB_SHORT",     slPct: 0.50, tpPct: 1.55, cooldownMin: 8,  holdMinutes: 50, confluenceMin: 6 },
  { id: 133, name: "OrderFlow_Break_Long",    category: "Order Flow",  signalKey: "OF_BREAK_LONG",        slPct: 0.52, tpPct: 1.56, cooldownMin: 7,  holdMinutes: 38, confluenceMin: 5 },
  { id: 134, name: "OrderFlow_Break_Short",   category: "Order Flow",  signalKey: "OF_BREAK_SHORT",       slPct: 0.52, tpPct: 1.56, cooldownMin: 7,  holdMinutes: 38, confluenceMin: 5 },
  { id: 139, name: "Wyckoff_Spring_Long",     category: "Wyckoff",   signalKey: "WYCKOFF_SPRING_LONG",    slPct: 0.55, tpPct: 1.65, cooldownMin: 10, holdMinutes: 60, confluenceMin: 6 },
  { id: 140, name: "Wyckoff_Upthrust_Short",  category: "Wyckoff",   signalKey: "WYCKOFF_UPTHRUST_SHORT", slPct: 0.55, tpPct: 1.65, cooldownMin: 10, holdMinutes: 60, confluenceMin: 6 },
  { id: 151, name: "OpeningDrive_Long",       category: "Session",   signalKey: "OPEN_DRIVE_LONG",        slPct: 0.50, tpPct: 1.50, cooldownMin: 5,  holdMinutes: 30, confluenceMin: 5 },
  { id: 152, name: "OpeningDrive_Short",      category: "Session",   signalKey: "OPEN_DRIVE_SHORT",       slPct: 0.50, tpPct: 1.50, cooldownMin: 5,  holdMinutes: 30, confluenceMin: 5 },
];

export const FUTURES_STRAT_DEFS: readonly FuturesStratDef[] = [
  ...BASE_FUTURES_STRAT_DEFS,
  ...BTC_FT_PREMIUM_DEFS,
];
