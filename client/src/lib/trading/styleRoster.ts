/**
 * Builds the active strategy roster for a given time-horizon desk and optional
 * playbook overlay. This is the single entry point for "which strategies run on
 * this desk right now?"
 *
 * Call order:
 *   1. Filter FUTURES_STRAT_DEFS by styleId (and playbookId when set)
 *   2. Apply winners-only gate (when winnerIds provided and research mode off)
 *   3. Apply buildPaperDeskStrategies (RR widen, fake-diversity off)
 *
 * Returns the processed defs plus metadata for logging/UI.
 */

import { FUTURES_STRAT_DEFS } from "@/lib/trading/futuresStrategies";
import type { FuturesStratDef, TradingStyleId, PlaybookId } from "@/lib/trading/futuresStratTypes";
import {
  buildPaperDeskStrategies,
  applyWinnersOnlyGate,
  deskResearchModeEnabled,
  type DeskStrategyBuildResult,
} from "@/lib/trading/futuresDeskPolicy";
import { getTradingStyleProfile } from "@/lib/trading/tradingStyleRegistry";
import { getPlaybookProfile, categoryPassesPlaybook } from "@/lib/trading/playbookRegistry";

export type StyleRosterMeta = {
  styleId: TradingStyleId;
  playbookId: PlaybookId | null;
  /** Total defs in FUTURES_STRAT_DEFS eligible for this style (before winners gate). */
  totalEligible: number;
  /** Defs remaining after winners-only gate. */
  afterWinnersGate: number;
  /** Final active strategy count (after RR filter). */
  active: number;
  /** Strategy IDs whose TP was widened to meet min R:R. */
  tpWidenedIds: readonly number[];
  /** Strategy IDs skipped for sub-minimum R:R that couldn't be widened. */
  lowRrSkippedIds: readonly number[];
  researchMode: boolean;
};

export type StyleRosterResult = {
  defs: FuturesStratDef[];
  meta: StyleRosterMeta;
};

export type BuildStyleRosterOpts = {
  /**
   * Set of strategy IDs with verified positive expectancy.
   * When provided and research mode is off, the winners-only gate is applied.
   * Pass an empty set to get the full roster (e.g. on first boot with no data).
   */
  winnerIds?: ReadonlySet<number> | readonly number[];
};

/**
 * Returns the active strategy roster for `styleId`, optionally narrowed to
 * `playbookId`. Uses `FUTURES_STRAT_DEFS` as the source of truth.
 */
export function buildStyleRoster(
  styleId: TradingStyleId,
  playbookId?: PlaybookId | null,
  opts: BuildStyleRosterOpts = {},
): StyleRosterResult {
  const styleProfile = getTradingStyleProfile(styleId);
  const playbookProfile = playbookId ? getPlaybookProfile(playbookId) : null;

  // Step 1: filter by style tag (untagged defs pass — backward compatible)
  const eligible = (FUTURES_STRAT_DEFS as readonly FuturesStratDef[]).filter((d) => {
    if (d.styles && d.styles.length > 0 && !d.styles.includes(styleId)) return false;
    if (playbookProfile && !categoryPassesPlaybook(d.category, playbookProfile)) return false;
    return true;
  });

  // Step 2: winners-only gate
  const winnerSet = resolveWinnerSet(opts.winnerIds);
  const afterGate = applyWinnersOnlyGate(eligible, winnerSet);

  // Step 3: RR widen + fake-diversity filter
  const buildResult: DeskStrategyBuildResult = buildPaperDeskStrategies(afterGate, {
    strategyIdAllowlist: null,
    minTpSlRatio: styleProfile.tpSlRatioMin,
    allowFakeDiversity: false,
  });

  return {
    defs: buildResult.strategies,
    meta: {
      styleId,
      playbookId: playbookId ?? null,
      totalEligible: eligible.length,
      afterWinnersGate: afterGate.length,
      active: buildResult.strategies.length,
      tpWidenedIds: buildResult.tpWidenedStratIds,
      lowRrSkippedIds: buildResult.lowRrSkippedStratIds,
      researchMode: deskResearchModeEnabled(),
    },
  };
}

function resolveWinnerSet(
  winnerIds: ReadonlySet<number> | readonly number[] | undefined,
): ReadonlySet<number> {
  if (!winnerIds) return new Set<number>();
  if (winnerIds instanceof Set) return winnerIds;
  return new Set(winnerIds as readonly number[]);
}
