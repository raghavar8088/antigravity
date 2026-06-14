/**
 * Premium BTC futures strategies.
 *
 * Strategy definitions have been removed from the application. Constants remain
 * so existing imports can distinguish the reserved premium ID range from the
 * currently empty active inventory.
 */

import type { FuturesStratDef } from "@/lib/trading/futuresStratTypes";

export const BTC_FT_PREMIUM_ID_START = 500;
export const BTC_FT_PREMIUM_ID_END = 527;
export const PREMIUM_NOTIONAL_MULTIPLIER = 2.0;

export const BTC_FT_PREMIUM_DEFS: ReadonlyArray<FuturesStratDef> = [];
export const BTC_FT_PREMIUM_STRATEGY_IDS: readonly number[] = [];
