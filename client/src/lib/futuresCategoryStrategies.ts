/**
 * 160-strategy research pool across 8 trading categories.
 *
 * ALL strategies here have researchOnly: true — they are NOT auto-active on the
 * production desk. They enter the live roster ONLY when:
 *   1. NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1 (research session), OR
 *   2. The strategy ID appears in winnerIdsFromRankings() (positive expectancy
 *      over ≥10 trades) AND NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1.
 *
 * ID allocation (no collision with CORE 91–152 or premium 500–503):
 *   600–619  Scalping
 *   620–639  Day trading       (stub — PR 3)
 *   640–659  Swing trading     (stub — PR 4)
 *   660–679  Position trading  (stub — PR 5)
 *   680–699  Trend trading     (stub — PR 6)
 *   700–719  Range trading     (stub — PR 7)
 *   720–739  Breakout trading  (stub — PR 8)
 *   740–759  Momentum trading  (stub — PR 9)
 */

import type { FuturesStratDef } from "@/lib/futuresStratTypes";

// ─── Category 1 — Scalping (600–619) ─────────────────────────────────────────
// Primary TF: 1m | Hold: 5–45 min | Leverage: 25× | SL: 0.35–0.65% | TP:SL ≥ 2.5
// Signal: real unique scoring in futuresSignalScoring.ts (scoreScalping)

export const SCALPING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [
  { id: 600, name: "SCP_EMA_Cross_Long",         category: "Scalping", signalKey: "SCP_EMA_CROSS_LONG",      tradingCategory: "scalping", templateFamily: "scp_ema_cross",    primaryBarInterval: "1m", slPct: 0.40, tpPct: 1.20, holdMinutes: 25,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 601, name: "SCP_EMA_Cross_Short",        category: "Scalping", signalKey: "SCP_EMA_CROSS_SHORT",     tradingCategory: "scalping", templateFamily: "scp_ema_cross",    primaryBarInterval: "1m", slPct: 0.40, tpPct: 1.20, holdMinutes: 25,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 602, name: "SCP_VWAP_Bounce_Long",       category: "Scalping", signalKey: "SCP_VWAP_BOUNCE_LONG",    tradingCategory: "scalping", templateFamily: "scp_vwap_bounce",  primaryBarInterval: "1m", slPct: 0.38, tpPct: 1.14, holdMinutes: 20,  cooldownMin: 6,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 603, name: "SCP_VWAP_Reject_Short",      category: "Scalping", signalKey: "SCP_VWAP_REJECT_SHORT",   tradingCategory: "scalping", templateFamily: "scp_vwap_bounce",  primaryBarInterval: "1m", slPct: 0.38, tpPct: 1.14, holdMinutes: 20,  cooldownMin: 6,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 604, name: "SCP_RSI_Snap_Long",          category: "Scalping", signalKey: "SCP_RSI_SNAP_LONG",       tradingCategory: "scalping", templateFamily: "scp_rsi_snap",     primaryBarInterval: "1m", slPct: 0.35, tpPct: 1.05, holdMinutes: 18,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 605, name: "SCP_RSI_Snap_Short",         category: "Scalping", signalKey: "SCP_RSI_SNAP_SHORT",      tradingCategory: "scalping", templateFamily: "scp_rsi_snap",     primaryBarInterval: "1m", slPct: 0.35, tpPct: 1.05, holdMinutes: 18,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 606, name: "SCP_BB_Squeeze_Long",        category: "Scalping", signalKey: "SCP_BB_SQUEEZE_LONG",     tradingCategory: "scalping", templateFamily: "scp_bb_squeeze",   primaryBarInterval: "1m", slPct: 0.42, tpPct: 1.26, holdMinutes: 30,  cooldownMin: 7,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 607, name: "SCP_BB_Squeeze_Short",       category: "Scalping", signalKey: "SCP_BB_SQUEEZE_SHORT",    tradingCategory: "scalping", templateFamily: "scp_bb_squeeze",   primaryBarInterval: "1m", slPct: 0.42, tpPct: 1.26, holdMinutes: 30,  cooldownMin: 7,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 608, name: "SCP_Micro_Break_Long",       category: "Scalping", signalKey: "SCP_MICRO_BREAK_LONG",    tradingCategory: "scalping", templateFamily: "scp_micro_break",  primaryBarInterval: "1m", slPct: 0.45, tpPct: 1.35, holdMinutes: 22,  cooldownMin: 6,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 609, name: "SCP_Micro_Break_Short",      category: "Scalping", signalKey: "SCP_MICRO_BREAK_SHORT",   tradingCategory: "scalping", templateFamily: "scp_micro_break",  primaryBarInterval: "1m", slPct: 0.45, tpPct: 1.35, holdMinutes: 22,  cooldownMin: 6,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 610, name: "SCP_OBI_Imbalance_Long",     category: "Scalping", signalKey: "SCP_OBI_IMBAL_LONG",      tradingCategory: "scalping", templateFamily: "scp_order_imbal",  primaryBarInterval: "1m", slPct: 0.40, tpPct: 1.20, holdMinutes: 15,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 611, name: "SCP_OBI_Imbalance_Short",    category: "Scalping", signalKey: "SCP_OBI_IMBAL_SHORT",     tradingCategory: "scalping", templateFamily: "scp_order_imbal",  primaryBarInterval: "1m", slPct: 0.40, tpPct: 1.20, holdMinutes: 15,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 612, name: "SCP_Momentum_Tick_Long",     category: "Scalping", signalKey: "SCP_MOM_TICK_LONG",       tradingCategory: "scalping", templateFamily: "scp_mom_tick",     primaryBarInterval: "1m", slPct: 0.38, tpPct: 1.14, holdMinutes: 12,  cooldownMin: 4,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 613, name: "SCP_Momentum_Tick_Short",    category: "Scalping", signalKey: "SCP_MOM_TICK_SHORT",      tradingCategory: "scalping", templateFamily: "scp_mom_tick",     primaryBarInterval: "1m", slPct: 0.38, tpPct: 1.14, holdMinutes: 12,  cooldownMin: 4,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 614, name: "SCP_Liq_Sweep_Long",         category: "Scalping", signalKey: "SCP_LIQ_SWEEP_LONG",      tradingCategory: "scalping", templateFamily: "scp_liq_sweep",    primaryBarInterval: "1m", slPct: 0.50, tpPct: 1.50, holdMinutes: 35,  cooldownMin: 8,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 615, name: "SCP_Liq_Sweep_Short",        category: "Scalping", signalKey: "SCP_LIQ_SWEEP_SHORT",     tradingCategory: "scalping", templateFamily: "scp_liq_sweep",    primaryBarInterval: "1m", slPct: 0.50, tpPct: 1.50, holdMinutes: 35,  cooldownMin: 8,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 616, name: "SCP_Session_Open_Long",      category: "Scalping", signalKey: "SCP_SESSION_OPEN_LONG",   tradingCategory: "scalping", templateFamily: "scp_session_open", primaryBarInterval: "1m", slPct: 0.45, tpPct: 1.35, holdMinutes: 28,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 617, name: "SCP_Session_Open_Short",     category: "Scalping", signalKey: "SCP_SESSION_OPEN_SHORT",  tradingCategory: "scalping", templateFamily: "scp_session_open", primaryBarInterval: "1m", slPct: 0.45, tpPct: 1.35, holdMinutes: 28,  cooldownMin: 5,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 618, name: "SCP_ATR_Pullback_Long",      category: "Scalping", signalKey: "SCP_ATR_PULLBACK_LONG",   tradingCategory: "scalping", templateFamily: "scp_atr_pullback", primaryBarInterval: "1m", slPct: 0.42, tpPct: 1.26, holdMinutes: 32,  cooldownMin: 7,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
  { id: 619, name: "SCP_ATR_Pullback_Short",     category: "Scalping", signalKey: "SCP_ATR_PULLBACK_SHORT",  tradingCategory: "scalping", templateFamily: "scp_atr_pullback", primaryBarInterval: "1m", slPct: 0.42, tpPct: 1.26, holdMinutes: 32,  cooldownMin: 7,  confluenceMin: 5, defaultLeverage: 25, researchOnly: true },
];

// ─── Category 2 — Day Trading (620–639) ──────────────────────────────────────
// Primary TF: 5m | Confirm TF: 15m (HTF strats) | Hold: 30–480 min
// Leverage: 15× | SL: 0.50–1.00% | TP:SL ≥ 2.0
// EOD flat enforced via passesCategoryConfirmation when requiresEodFlat: true.

export const DAY_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [
  { id: 620, name: "DAY_VWAP_Trend_Long",     category: "Day Trading", signalKey: "DAY_VWAP_TREND_LONG",   tradingCategory: "day_trading", templateFamily: "vwap_trend",    primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.65, tpPct: 1.95, holdMinutes: 180, cooldownMin: 15, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 621, name: "DAY_VWAP_Trend_Short",    category: "Day Trading", signalKey: "DAY_VWAP_TREND_SHORT",  tradingCategory: "day_trading", templateFamily: "vwap_trend",    primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.65, tpPct: 1.95, holdMinutes: 180, cooldownMin: 15, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 622, name: "DAY_ORB_Break_Long",      category: "Day Trading", signalKey: "DAY_ORB_BREAK_LONG",    tradingCategory: "day_trading", templateFamily: "orb_break",     primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.70, tpPct: 2.10, holdMinutes: 120, cooldownMin: 20, confluenceMin: 6, defaultLeverage: 15, researchOnly: true },
  { id: 623, name: "DAY_ORB_Break_Short",     category: "Day Trading", signalKey: "DAY_ORB_BREAK_SHORT",   tradingCategory: "day_trading", templateFamily: "orb_break",     primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.70, tpPct: 2.10, holdMinutes: 120, cooldownMin: 20, confluenceMin: 6, defaultLeverage: 15, researchOnly: true },
  { id: 624, name: "DAY_MTF_Align_Long",      category: "Day Trading", signalKey: "DAY_MTF_ALIGN_LONG",    tradingCategory: "day_trading", templateFamily: "mtf_align",     primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.60, tpPct: 1.80, holdMinutes: 240, cooldownMin: 18, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 625, name: "DAY_MTF_Align_Short",     category: "Day Trading", signalKey: "DAY_MTF_ALIGN_SHORT",   tradingCategory: "day_trading", templateFamily: "mtf_align",     primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.60, tpPct: 1.80, holdMinutes: 240, cooldownMin: 18, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 626, name: "DAY_MACD_Zero_Long",      category: "Day Trading", signalKey: "DAY_MACD_ZERO_LONG",    tradingCategory: "day_trading", templateFamily: "macd_zero",     primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.55, tpPct: 1.65, holdMinutes: 150, cooldownMin: 15, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 627, name: "DAY_MACD_Zero_Short",     category: "Day Trading", signalKey: "DAY_MACD_ZERO_SHORT",   tradingCategory: "day_trading", templateFamily: "macd_zero",     primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.55, tpPct: 1.65, holdMinutes: 150, cooldownMin: 15, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 628, name: "DAY_Pullback_EMA_Long",   category: "Day Trading", signalKey: "DAY_PB_EMA_LONG",       tradingCategory: "day_trading", templateFamily: "ema_pullback",  primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.58, tpPct: 1.74, holdMinutes: 200, cooldownMin: 16, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 629, name: "DAY_Pullback_EMA_Short",  category: "Day Trading", signalKey: "DAY_PB_EMA_SHORT",      tradingCategory: "day_trading", templateFamily: "ema_pullback",  primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.58, tpPct: 1.74, holdMinutes: 200, cooldownMin: 16, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 630, name: "DAY_Range_Expand_Long",   category: "Day Trading", signalKey: "DAY_RANGE_EXP_LONG",    tradingCategory: "day_trading", templateFamily: "range_expand",  primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.75, tpPct: 2.25, holdMinutes: 90,  cooldownMin: 12, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 631, name: "DAY_Range_Expand_Short",  category: "Day Trading", signalKey: "DAY_RANGE_EXP_SHORT",   tradingCategory: "day_trading", templateFamily: "range_expand",  primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.75, tpPct: 2.25, holdMinutes: 90,  cooldownMin: 12, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 632, name: "DAY_Volume_Climax_Long",  category: "Day Trading", signalKey: "DAY_VOL_CLIMAX_LONG",   tradingCategory: "day_trading", templateFamily: "vol_climax",    primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.80, tpPct: 2.40, holdMinutes: 160, cooldownMin: 18, confluenceMin: 6, defaultLeverage: 15, researchOnly: true },
  { id: 633, name: "DAY_Volume_Climax_Short", category: "Day Trading", signalKey: "DAY_VOL_CLIMAX_SHORT",  tradingCategory: "day_trading", templateFamily: "vol_climax",    primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.80, tpPct: 2.40, holdMinutes: 160, cooldownMin: 18, confluenceMin: 6, defaultLeverage: 15, researchOnly: true },
  { id: 634, name: "DAY_Struct_HH_Long",      category: "Day Trading", signalKey: "DAY_STRUCT_HH_LONG",    tradingCategory: "day_trading", templateFamily: "struct_hh",     primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.62, tpPct: 1.86, holdMinutes: 220, cooldownMin: 20, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 635, name: "DAY_Struct_LL_Short",     category: "Day Trading", signalKey: "DAY_STRUCT_LL_SHORT",   tradingCategory: "day_trading", templateFamily: "struct_hh",     primaryBarInterval: "5m", confirmBarInterval: "15m", requiresHtf: true,  slPct: 0.62, tpPct: 1.86, holdMinutes: 220, cooldownMin: 20, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 636, name: "DAY_Midday_Fade_Long",    category: "Day Trading", signalKey: "DAY_MIDDAY_FADE_LONG",  tradingCategory: "day_trading", templateFamily: "midday_fade",   primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.50, tpPct: 1.50, holdMinutes: 60,  cooldownMin: 10, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 637, name: "DAY_Midday_Fade_Short",   category: "Day Trading", signalKey: "DAY_MIDDAY_FADE_SHORT", tradingCategory: "day_trading", templateFamily: "midday_fade",   primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.50, tpPct: 1.50, holdMinutes: 60,  cooldownMin: 10, confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 638, name: "DAY_Close_Momentum_Long", category: "Day Trading", signalKey: "DAY_CLOSE_MOM_LONG",    tradingCategory: "day_trading", templateFamily: "close_mom",     primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.55, tpPct: 1.65, holdMinutes: 45,  cooldownMin: 8,  confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
  { id: 639, name: "DAY_Close_Momentum_Short",category: "Day Trading", signalKey: "DAY_CLOSE_MOM_SHORT",   tradingCategory: "day_trading", templateFamily: "close_mom",     primaryBarInterval: "5m",                            requiresHtf: false, slPct: 0.55, tpPct: 1.65, holdMinutes: 45,  cooldownMin: 8,  confluenceMin: 5, defaultLeverage: 15, researchOnly: true },
];

// ─── Categories 3–8: stubs (scored in subsequent PRs) ────────────────────────
// Each returns [] until the corresponding PR lands scoring + defs.
export const SWING_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const POSITION_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const TREND_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const RANGE_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const BREAKOUT_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const MOMENTUM_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];

/** Flat array of all 160 research-pool strategy definitions. */
export const CATEGORY_POOL_160: ReadonlyArray<FuturesStratDef> = [
  ...SCALPING_STRATEGIES,
  ...DAY_TRADING_STRATEGIES,
  ...SWING_TRADING_STRATEGIES,
  ...POSITION_TRADING_STRATEGIES,
  ...TREND_TRADING_STRATEGIES,
  ...RANGE_TRADING_STRATEGIES,
  ...BREAKOUT_TRADING_STRATEGIES,
  ...MOMENTUM_TRADING_STRATEGIES,
];

/**
 * Migration map: CORE 20 + premium strategy IDs → tradingCategory.
 * Used for display, filtering, and category roster queries.
 * Does NOT change the active roster or scoring — these IDs use existing logic.
 */
export const LEGACY_CORE_CATEGORY_MAP: ReadonlyMap<number, import("@/lib/futuresStratTypes").TradingCategoryId> =
  new Map([
    [91,  "trend_trading"],
    [92,  "trend_trading"],
    [95,  "breakout_trading"],
    [96,  "breakout_trading"],
    [111, "trend_trading"],
    [112, "trend_trading"],
    [117, "trend_trading"],
    [118, "trend_trading"],
    [123, "trend_trading"],
    [124, "trend_trading"],
    [125, "breakout_trading"],
    [126, "breakout_trading"],
    [131, "momentum_trading"],
    [132, "momentum_trading"],
    [133, "breakout_trading"],
    [134, "breakout_trading"],
    [139, "range_trading"],
    [140, "range_trading"],
    [151, "scalping"],
    [152, "scalping"],
    [500, "range_trading"],
    [501, "range_trading"],
    [502, "momentum_trading"],
    [503, "momentum_trading"],
  ]);
