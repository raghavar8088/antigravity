/**
 * BTC FT extended strategy templates — removed (research pool deleted).
 * Stubs kept so existing imports don't break.
 */

import type { BtcFtTemplateId, FuturesStratDef, RegimeTag } from "@/lib/futuresStratTypes";

export type { BtcFtTemplateId } from "@/lib/futuresStratTypes";

export type BtcFtTemplateMeta = {
  id: BtcFtTemplateId;
  category: string;
  label: string;
  defaultRegimes: readonly RegimeTag[];
  baseSlPct: number;
  baseTpPct: number;
  baseHoldMinutes: number;
  baseCooldownMin: number;
  requiresHtf: boolean;
};

export const BTC_FT_TEMPLATE_CYCLE: readonly BtcFtTemplateId[] = [];
export const BTC_FT_TEMPLATE_META: Record<BtcFtTemplateId, BtcFtTemplateMeta> = {} as Record<BtcFtTemplateId, BtcFtTemplateMeta>;

export function templateShortName(_tpl: BtcFtTemplateId): string { return ""; }

export const BTC_FT_EXTENDED_DEFS_PROOF: readonly FuturesStratDef[] = [];
export const BTC_FT_EXTENDED_DEFS_FULL: readonly FuturesStratDef[] = [];
export function buildBtcFtExtendedDefsFull(): FuturesStratDef[] { return []; }
