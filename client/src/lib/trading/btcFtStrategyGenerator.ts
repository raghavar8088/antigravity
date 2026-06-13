/**
 * BTC FT generated strategy pool (300–399) — removed.
 * Stubs kept so existing imports don't break.
 */

import type { FuturesStratDef } from "@/lib/trading/futuresStratTypes";

export const BTC_FT_GENERATED_ID_START = 300;
export const BTC_FT_GENERATED_ID_END = 399;
export const BTC_FT_GENERATED_COUNT_DEFAULT = 0;
export const BTC_FT_GENERATED_TEMPLATE_CYCLE: readonly string[] = [];
export const BTC_FT_GENERATED_DEFS: readonly FuturesStratDef[] = [];
export const BTC_FT_GENERATED_STRATEGY_IDS: readonly number[] = [];

export function isGeneratedPoolEnabled(): boolean { return false; }

export function buildGeneratedStrategies(): FuturesStratDef[] { return []; }
