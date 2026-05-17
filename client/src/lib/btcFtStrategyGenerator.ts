/**
 * BTC Future Trading — Phase 1 generated strategy pool (IDs 300–399).
 *
 * 100 research-only strategies generated from 12 templates × variants × LONG/SHORT.
 * These are NEVER added to CORE_BTC_FT_STRATEGY_IDS or the production default roster.
 * Inclusion is gated by NEXT_PUBLIC_BTC_FT_GENERATED_POOL (default ON in research mode).
 *
 * Rationale: enlarge the testable strategy library so research mode can rotate batches
 * (~30 active / 24h), accumulate per-strategy verdicts, and only promote WINNERS later.
 *
 * Hard rules respected:
 *   1. Every def carries a real `btcFtTemplate` with an evalBtcFtTemplateSignal branch.
 *   2. IDs 300–399 do NOT appear in CORE / production roster.
 *   3. Default active count stays 30 per research batch.
 *
 * See: PAPER_DESK_RUNBOOK.md → "Adding 100 strategies safely".
 */

import type { BtcFtTemplateId, FuturesStratDef } from "@/lib/futuresStratTypes";
import { BTC_FT_TEMPLATE_META, templateShortName } from "@/lib/btcFtStrategyTemplates";

/** ID range reserved for generated strategies (inclusive). */
export const BTC_FT_GENERATED_ID_START = 300;
export const BTC_FT_GENERATED_ID_END = 399;
export const BTC_FT_GENERATED_COUNT_DEFAULT = 100;

/**
 * Template rotation for the 300–399 pool.
 * 12 entries × ceil(100 / 12) passes = 8 full passes + 4 extra.
 * Each pass alternates LONG/SHORT and advances the variant grid.
 */
export const BTC_FT_GENERATED_TEMPLATE_CYCLE: readonly BtcFtTemplateId[] = [
  "MTF_EMA_STACK",
  "MTF_MACD_HIST",
  "MTF_ADX_DI",
  "MTF_DONCHIAN_BREAK",
  "MEANREV_RSI",
  "SESSION_RANGE_BREAK",
  "WYCKOFF_SPRING",
  "SMART_MONEY_FVG",
  // Re-use 4 proven extended templates to broaden parameter coverage
  "VWAP_REVERT",
  "MOMENTUM_IMPULSE",
  "ORDERFLOW_PROXY",
  "MEAN_REVERT_BB",
];

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n));
}

/** Parameter grid (slPct × tpPct × hold) over variants 0..3. */
function variantRisk(variant: number): { slMul: number; tpMul: number; holdMul: number } {
  const v = variant % 4;
  return {
    slMul: 1 + v * 0.04,
    tpMul: 1 + v * 0.03,
    holdMul: 1 + v * 0.07,
  };
}

export type GeneratorOptions = {
  startId?: number;
  count?: number;
  templates?: readonly BtcFtTemplateId[];
};

/**
 * Build N research-pool defs. Defaults: startId=300, count=100, full 12-template cycle.
 *
 * Layout for default cycle (length 12, count 100):
 *   - i % 12 → template index
 *   - floor(i / 12) % 4 → variant (0..3)
 *   - i % 2 → LONG (0) / SHORT (1)
 *
 * Coverage: 96 strategies hit the full 12 × 4 × 2 matrix; the last 4 reuse the first
 * 4 templates at variant 0 (forming additional sample points).
 */
export function buildGeneratedStrategies(opts: GeneratorOptions = {}): FuturesStratDef[] {
  const startId = opts.startId ?? BTC_FT_GENERATED_ID_START;
  const count = opts.count ?? BTC_FT_GENERATED_COUNT_DEFAULT;
  const cycle = opts.templates ?? BTC_FT_GENERATED_TEMPLATE_CYCLE;
  if (cycle.length === 0 || count <= 0) return [];

  const out: FuturesStratDef[] = [];
  for (let i = 0; i < count; i++) {
    const tpl = cycle[i % cycle.length]!;
    const meta = BTC_FT_TEMPLATE_META[tpl];
    const pairIndex = Math.floor(i / cycle.length);
    const variant = pairIndex % 4;
    const isShort = i % 2 === 1;
    const id = startId + i;
    if (id > BTC_FT_GENERATED_ID_END) {
      throw new Error(`buildGeneratedStrategies: id ${id} exceeds reserved range (${BTC_FT_GENERATED_ID_END})`);
    }
    const { slMul, tpMul, holdMul } = variantRisk(variant);
    const slPct = clamp(meta.baseSlPct * slMul, 0.22, 0.45);
    const tpPct = clamp(meta.baseTpPct * tpMul, 0.55, 1.1);
    const holdMinutes = clamp(Math.round(meta.baseHoldMinutes * holdMul), 10, 40);
    const side = isShort ? "SHORT" : "LONG";
    const name = `BTCFT_GEN_${templateShortName(tpl)}_V${variant}_${side}_${id}`;
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

/** Full 100-strategy generated basket (IDs 300–399). */
export const BTC_FT_GENERATED_DEFS: readonly FuturesStratDef[] = buildGeneratedStrategies();

/** Stable ID list for pool resolution. */
export const BTC_FT_GENERATED_STRATEGY_IDS: readonly number[] = BTC_FT_GENERATED_DEFS.map((d) => d.id);

/** Env flag: include 300–399 in the research pool. Default ON when research mode is on. */
export function isGeneratedPoolEnabled(): boolean {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_GENERATED_POOL;
  if (raw === undefined || raw.trim() === "") {
    return process.env.NEXT_PUBLIC_BTC_FT_RESEARCH_MODE === "1";
  }
  return raw === "1";
}
