/**
 * BTC Future Trading — extended strategy templates (IDs ≥200).
 * Each template has a dedicated scorer + confirmation in futuresSignals via `btcFtTemplate`.
 * Not marketed as profitable; replay ranking selects Tier B roster additions.
 */

import type { BtcFtTemplateId, FuturesStratDef, RegimeTag } from "@/lib/futuresStratTypes";

export type { BtcFtTemplateId } from "@/lib/futuresStratTypes";

export type BtcFtTemplateMeta = {
  id: BtcFtTemplateId;
  /** Desk category (appears in leaderboard); signal uses `btcFtTemplate`. */
  category: string;
  /** Human label for docs / logs */
  label: string;
  /** Default regimes when explicit regimes omitted on defs */
  defaultRegimes: readonly RegimeTag[];
  baseSlPct: number;
  baseTpPct: number;
  baseHoldMinutes: number;
  baseCooldownMin: number;
  requiresHtf: boolean;
};

/** 10 templates for rotation when scaling toward 100 IDs (cycle × variants × L/S). */
export const BTC_FT_TEMPLATE_CYCLE: readonly BtcFtTemplateId[] = [
  "MTF_TREND",
  "MTF_BREAK",
  "MEAN_REVERT_BB",
  "VWAP_REVERT",
  "MOMENTUM_IMPULSE",
  "SESSION_OPEN",
  "WYCKOFF_TRAP",
  "ORDERFLOW_PROXY",
  "MTF_TREND",
  "MTF_BREAK",
];

export const BTC_FT_TEMPLATE_META: Record<BtcFtTemplateId, BtcFtTemplateMeta> = {
  MTF_TREND: {
    id: "MTF_TREND",
    category: "BTC FT MTF Trend",
    label: "Multi-timeframe trend alignment (5m/15m bars from 1m OHLCV)",
    defaultRegimes: ["trendLow", "trendHigh"],
    baseSlPct: 0.26,
    baseTpPct: 0.82,
    baseHoldMinutes: 30,
    baseCooldownMin: 6,
    requiresHtf: true,
  },
  MTF_BREAK: {
    id: "MTF_BREAK",
    category: "BTC FT MTF Break",
    label: "Range expansion with HTF trend filter",
    defaultRegimes: ["trendLow", "trendHigh"],
    baseSlPct: 0.3,
    baseTpPct: 0.88,
    baseHoldMinutes: 26,
    baseCooldownMin: 5,
    requiresHtf: true,
  },
  MEAN_REVERT_BB: {
    id: "MEAN_REVERT_BB",
    category: "BTC FT MR BB",
    label: "Bollinger stretch + RSI band MR",
    defaultRegimes: ["chop", "trendLow"],
    baseSlPct: 0.32,
    baseTpPct: 0.62,
    baseHoldMinutes: 22,
    baseCooldownMin: 4,
    requiresHtf: false,
  },
  VWAP_REVERT: {
    id: "VWAP_REVERT",
    category: "BTC FT VWAP Rev",
    label: "Distance from session VWAP proxy MR",
    defaultRegimes: ["chop", "trendLow"],
    baseSlPct: 0.3,
    baseTpPct: 0.64,
    baseHoldMinutes: 20,
    baseCooldownMin: 4,
    requiresHtf: false,
  },
  MOMENTUM_IMPULSE: {
    id: "MOMENTUM_IMPULSE",
    category: "BTC FT Momentum",
    label: "ATR-scaled burst + OBV agreement",
    defaultRegimes: ["trendLow", "trendHigh"],
    baseSlPct: 0.28,
    baseTpPct: 0.74,
    baseHoldMinutes: 18,
    baseCooldownMin: 4,
    requiresHtf: false,
  },
  SESSION_OPEN: {
    id: "SESSION_OPEN",
    category: "BTC FT Session",
    label: "UTC hour window + expansion (uses last bar ms when provided)",
    defaultRegimes: ["trendLow", "trendHigh", "chop"],
    baseSlPct: 0.32,
    baseTpPct: 0.72,
    baseHoldMinutes: 16,
    baseCooldownMin: 3,
    requiresHtf: false,
  },
  WYCKOFF_TRAP: {
    id: "WYCKOFF_TRAP",
    category: "BTC FT Wyckoff",
    label: "Mid-structure + CCI thrust proxy",
    defaultRegimes: ["chop", "trendLow"],
    baseSlPct: 0.28,
    baseTpPct: 0.92,
    baseHoldMinutes: 34,
    baseCooldownMin: 7,
    requiresHtf: false,
  },
  ORDERFLOW_PROXY: {
    id: "ORDERFLOW_PROXY",
    category: "BTC FT Flow",
    label: "Volume-weighted thrust vs noise floor",
    defaultRegimes: ["trendLow", "trendHigh"],
    baseSlPct: 0.28,
    baseTpPct: 0.78,
    baseHoldMinutes: 22,
    baseCooldownMin: 5,
    requiresHtf: false,
  },
};

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n));
}

/** SL/TP multipliers from variant grid (0–3). */
function variantRisk(variant: number): { slMul: number; tpMul: number; holdMul: number } {
  const v = variant % 4;
  return {
    slMul: 1 + v * 0.035,
    tpMul: 1 + v * 0.025,
    holdMul: 1 + v * 0.06,
  };
}

function templateShortName(tpl: BtcFtTemplateId): string {
  switch (tpl) {
    case "MTF_TREND":
      return "MTFT";
    case "MTF_BREAK":
      return "MTFB";
    case "MEAN_REVERT_BB":
      return "MRBB";
    case "VWAP_REVERT":
      return "VWAP";
    case "MOMENTUM_IMPULSE":
      return "MOMI";
    case "SESSION_OPEN":
      return "SESS";
    case "WYCKOFF_TRAP":
      return "WYCK";
    case "ORDERFLOW_PROXY":
      return "OFLO";
    default:
      return String(tpl);
  }
}

/**
 * Generate BTC FT extended defs. IDs default to 200–299 block (documented cap 100 slots from startId).
 * Names: `{ShortTpl}_V{variant}_{L|S}_{seq}` for uniqueness.
 */
export function buildBtcFtStrategyDefs(
  startId: number,
  count: number,
  opts?: {
    templates?: readonly BtcFtTemplateId[];
  },
): FuturesStratDef[] {
  const cycle = opts?.templates ?? [...BTC_FT_TEMPLATE_CYCLE];
  if (cycle.length === 0) return [];

  const out: FuturesStratDef[] = [];
  for (let i = 0; i < count; i++) {
    const tpl = cycle[i % cycle.length]!;
    const meta = BTC_FT_TEMPLATE_META[tpl];
    const pairIndex = Math.floor(i / cycle.length);
    const variant = pairIndex % 4;
    const isShort = i % 2 === 1;
    const id = startId + i;
    const { slMul, tpMul, holdMul } = variantRisk(variant);
    const slPct = clamp(meta.baseSlPct * slMul, 0.22, 0.42);
    const tpPct = clamp(meta.baseTpPct * tpMul, 0.55, 1.05);
    const holdMinutes = clamp(Math.round(meta.baseHoldMinutes * holdMul), 8, 45);
    const side = isShort ? "SHORT" : "LONG";
    const name = `BTCFT_${templateShortName(tpl)}_V${variant}_${side}_${id}`;
    const signalKey = `BTCFT_${tpl}_${variant}_${side}`;
    out.push({
      id,
      name,
      category: meta.category,
      signalKey,
      slPct: Math.round(slPct * 10000) / 10000,
      tpPct: Math.round(tpPct * 10000) / 10000,
      cooldownMin: meta.baseCooldownMin,
      holdMinutes,
      confluenceMin: 4,
      requiresHtf: meta.requiresHtf,
      regimes: [...meta.defaultRegimes],
      btcFtTemplate: tpl,
      btcFtVariant: variant,
    });
  }
  return out;
}

/** Proof batch: 20 strategies, IDs 200–219. */
export const BTC_FT_EXTENDED_DEFS_PROOF = buildBtcFtStrategyDefs(200, 20);

/**
 * Scale toward 100 additional defs (IDs 200–299). Call from maintenance scripts only;
 * merge into FUTURES_STRAT_DEFS intentionally capped by roster policy in the UI module.
 */
export function buildBtcFtExtendedDefsFull(): FuturesStratDef[] {
  return buildBtcFtStrategyDefs(200, 100);
}
