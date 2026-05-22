/**
 * BTC FT generated strategy pool (IDs 300–399) — removed (research pool deleted).
 * Stubs kept so existing imports don't break.
 */

import type { BtcFtTemplateId, FuturesStratDef } from "@/lib/futuresStratTypes";

export const BTC_FT_GENERATED_ID_START = 300;
export const BTC_FT_GENERATED_ID_END = 399;
export const BTC_FT_GENERATED_COUNT_DEFAULT = 0;

export const BTC_FT_GENERATED_TEMPLATE_CYCLE: readonly BtcFtTemplateId[] = [];

export type GeneratorOptions = {
  startId?: number;
  count?: number;
  templates?: readonly BtcFtTemplateId[];
};

export function buildGeneratedStrategies(_opts: GeneratorOptions = {}): FuturesStratDef[] { return []; }

export const BTC_FT_GENERATED_DEFS: readonly FuturesStratDef[] = [];
export const BTC_FT_GENERATED_STRATEGY_IDS: readonly number[] = [];

export function isGeneratedPoolEnabled(): boolean { return false; }
