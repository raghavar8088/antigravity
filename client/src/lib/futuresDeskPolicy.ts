/**
 * BTC futures **paper** desk policy (Phase 2–3).
 * Phase 3: IDs 79–110 are “fake diversity” placeholders without dedicated `evalMinuteSignal` wiring
 * for their branded categories — excluded by default. Set `NEXT_PUBLIC_DESK_ENABLE_FAKE_DIV_STRATS=1` to include.
 */

import type { FuturesStratDef } from "./futuresStrategies";
import type { FuturesStrategyProfile } from "./futuresSessionMetrics";
import { paperWidenTpToMinSlRatio } from "./futuresPaperMath";

/** When `scalp_aggro_v1` + desk-widened TP, multiply strat base `holdMinutes` before profile `holdTimeMul` in `paperResolveHardExit`. */
export const HOLD_MUL_AFTER_TP_WIDEN = 1.18;

export function deskEffectiveHoldMinutesAtOpen(
  baseHoldMinutes: number,
  profile: FuturesStrategyProfile,
  deskTpWidened: boolean | undefined,
): { holdMinutes: number; profileAdjusted: boolean } {
  if (
    profile === "scalp_aggro_v1" &&
    deskTpWidened === true &&
    Number.isFinite(baseHoldMinutes) &&
    baseHoldMinutes > 0
  ) {
    return { holdMinutes: baseHoldMinutes * HOLD_MUL_AFTER_TP_WIDEN, profileAdjusted: true };
  }
  return { holdMinutes: baseHoldMinutes, profileAdjusted: false };
}

/** Inclusive range 79…110 — see Phase 3 note in module header. */
export const FAKE_DIVERSITY_STRAT_IDS: readonly number[] = Array.from({ length: 32 }, (_, i) => 79 + i);

/** Hard cap on widened TP% — beyond this, strategy is excluded rather than distorting intent. */
export const DESK_WIDEN_TP_MAX_PCT = 4.8;

export function deskFakeDiversityEnabledViaEnv(): boolean {
  return process.env.NEXT_PUBLIC_DESK_ENABLE_FAKE_DIV_STRATS === "1";
}

export function deskMinTpSlRatioFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MIN_TP_SL_RATIO;
  if (raw === undefined || raw === "") return 2;
  const n = Number(raw);
  return Number.isFinite(n) && n >= 1 ? n : 2;
}

export type DeskStrategyBuildResult = {
  strategies: FuturesStratDef[];
  tpWidenedStratIds: readonly number[];
  lowRrSkippedStratIds: readonly number[];
  fakeDiversityFilteredCount: number;
};

/**
 * Apply allow-list, fake-diversity filter, and TP widen vs `minTpSlRatio` (or skip if widen exceeds cap).
 */
export function buildPaperDeskStrategies(
  raw: readonly FuturesStratDef[],
  opts: {
    strategyIdAllowlist: Set<number> | null;
    minTpSlRatio: number;
    allowFakeDiversity: boolean;
  },
): DeskStrategyBuildResult {
  const tpWidenedStratIds: number[] = [];
  const lowRrSkippedStratIds: number[] = [];
  let fakeDiversityFilteredCount = 0;

  const afterAllow = raw.filter((d) => {
    if (opts.strategyIdAllowlist && !opts.strategyIdAllowlist.has(d.id)) return false;
    if (!opts.allowFakeDiversity && FAKE_DIVERSITY_STRAT_IDS.includes(d.id)) {
      fakeDiversityFilteredCount += 1;
      return false;
    }
    return true;
  });

  const strategies: FuturesStratDef[] = [];
  for (const d of afterAllow) {
    const w = paperWidenTpToMinSlRatio(d.slPct, d.tpPct, opts.minTpSlRatio, DESK_WIDEN_TP_MAX_PCT);
    if (!w.included) {
      lowRrSkippedStratIds.push(d.id);
      continue;
    }
    const widened = Math.abs(w.tpPct - d.tpPct) > 1e-9;
    if (widened) tpWidenedStratIds.push(d.id);
    strategies.push({
      ...d,
      tpPct: w.tpPct,
      deskTpWidened: widened,
    });
  }

  return {
    strategies,
    tpWidenedStratIds,
    lowRrSkippedStratIds,
    fakeDiversityFilteredCount,
  };
}
