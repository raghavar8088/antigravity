/**
 * BTC futures category strategy pools.
 *
 * All category strategy definitions have been removed. Empty exports are kept
 * for roster resolution, category filters, and UI code paths.
 */

import type { FuturesStratDef, TradingCategoryId } from "@/lib/trading/futuresStratTypes";

export const SCALPING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const DAY_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const SWING_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const POSITION_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const TREND_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const RANGE_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const BREAKOUT_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];
export const MOMENTUM_TRADING_STRATEGIES: ReadonlyArray<FuturesStratDef> = [];

export const CATEGORY_POOL_160: ReadonlyArray<FuturesStratDef> = [];

export const LEGACY_CORE_CATEGORY_MAP: ReadonlyMap<number, TradingCategoryId> = new Map();
