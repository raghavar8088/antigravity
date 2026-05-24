/**
 * Premium hypothesis-driven BTC FT strategies (IDs 500–503).
 *
 * Unlike the 100 generated research-pool strategies (IDs 300–399) which are
 * parameter variations of generic templates, these strategies each embody
 * a SPECIFIC, MECHANISTIC market hypothesis with empirical/microstructure
 * grounding. They:
 *
 *   - Are always active in the production roster (no env gating)
 *   - Receive 2× notional when fired (the `tier: "premium"` tag → hook reads it)
 *   - Use higher confluenceMin (6) so they don't fire on weak signals
 *   - Use wider R:R (2.5–2.6:1) to capture the full reversion targets
 *   - Render with a "PREMIUM" badge in the leaderboard UI
 *
 * If these strategies prove edgeless after 100+ trades, they get retired
 * — same bar as any other. "Premium" means *higher conviction in the design*,
 * NOT *guaranteed wins*. The market answers; the tier just buys them more
 * notional and faster verdict resolution.
 *
 * --------------------------------------------------------------------------
 *
 * Strategy 1 — VWAP Session-Open Rejection (IDs 500 LONG, 501 SHORT)
 *
 *   Hypothesis: During the first 60 minutes of London (UTC 08–09) or NY
 *   (UTC 13–14) open, when BTC extends ≥0.5% from session VWAP, the move
 *   reverts to VWAP within 60 minutes.
 *
 *   Mechanism: Session opens see overnight inventory clearing (forced flows,
 *   not directional conviction). Once that initial thrust completes,
 *   institutional desks fade extremes back toward VWAP. Liquidity is highest
 *   in these windows, so reversion happens fast and cleanly.
 *
 *   Why it should work: Documented in equity markets (VWAP rejection at open
 *   is a staple of institutional trading desks); analogous mechanics in
 *   crypto futures with similar liquidity profiles around major-region opens.
 *
 *   Why it might NOT work for us: BTC is 24/7 — "session opens" are less
 *   sharp than equity opens. If the 0.5% threshold is reached too often
 *   (false positives) we'll fee-bleed. The confluenceMin=6 + extra confirm
 *   gate guards against this; if it still fires too much in practice, the
 *   threshold can be raised to 0.7%+.
 *
 * --------------------------------------------------------------------------
 *
 * Strategy 2 — Volume-Price Divergence Reversal (IDs 502 LONG, 503 SHORT)
 *
 *   Hypothesis: When BTC prints a new 20-bar high/low but OBV does NOT
 *   confirm AND volume on the new extreme is thin, the move reverses
 *   within 75 minutes.
 *
 *   Mechanism: New extreme without volume = position-trapping, not new
 *   conviction. OBV diverging from price means cumulative buying/selling
 *   pressure is fading. Thin volume on the extreme means no large players
 *   defending the level. Classical "exhaustion" pattern.
 *
 *   Why it should work: Volume-price divergence is one of the few retail
 *   TA patterns with statistical backing across markets. The mechanism is
 *   real: someone has to keep buying at the highs for prices to hold; if
 *   they're not, gravity takes over.
 *
 *   Why it might NOT work for us: 20-bar window on 1m candles = 20 minutes
 *   of context — short horizon, lots of noise. Strong trends will print
 *   many "false" divergences before the actual top/bottom. The volRatio
 *   < 1.2 gate filters the loudest false positives.
 *
 */

import type { FuturesStratDef } from "@/lib/futuresStratTypes";

/** Stable ID range for premium strategies. */
export const BTC_FT_PREMIUM_ID_START = 500;
export const BTC_FT_PREMIUM_ID_END = 523;

/** Notional multiplier applied when a premium strategy fires. */
export const PREMIUM_NOTIONAL_MULTIPLIER = 2.0;

export const BTC_FT_PREMIUM_DEFS: ReadonlyArray<FuturesStratDef> = [
  // -------------------- Strategy 1: VWAP Session Rejection --------------------
  {
    id: 500,
    name: "PRM_VWAP_SessionReject_Long",
    category: "PREMIUM VWAP Reject",
    signalKey: "BTCFT_PRM_VWAP_REJECT_0_LONG",
    slPct: 0.50,
    tpPct: 1.50,
    cooldownMin: 30,
    holdMinutes: 80,
    confluenceMin: 6,
    requiresHtf: true,
    regimes: ["chop", "trendLow"],
    btcFtTemplate: "PRM_VWAP_REJECT",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["range"],
    templateFamily: "PRM_VWAP_REJECT",
  },
  {
    id: 501,
    name: "PRM_VWAP_SessionReject_Short",
    category: "PREMIUM VWAP Reject",
    signalKey: "BTCFT_PRM_VWAP_REJECT_0_SHORT",
    slPct: 0.50,
    tpPct: 1.50,
    cooldownMin: 30,
    holdMinutes: 80,
    confluenceMin: 6,
    requiresHtf: true,
    regimes: ["chop", "trendLow"],
    btcFtTemplate: "PRM_VWAP_REJECT",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["range"],
    templateFamily: "PRM_VWAP_REJECT",
  },
  // -------------------- Strategy 2: Volume-Price Divergence --------------------
  {
    id: 502,
    name: "PRM_VolDivergence_Long",
    category: "PREMIUM Vol Divergence",
    signalKey: "BTCFT_PRM_VOL_DIVERGENCE_0_LONG",
    slPct: 0.55,
    tpPct: 1.65,
    cooldownMin: 30,
    holdMinutes: 95,
    confluenceMin: 6,
    requiresHtf: false,
    regimes: ["chop", "trendLow", "trendHigh"],
    btcFtTemplate: "PRM_VOL_DIVERGENCE",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["momentum"],
    templateFamily: "PRM_VOL_DIVERGENCE",
  },
  {
    id: 503,
    name: "PRM_VolDivergence_Short",
    category: "PREMIUM Vol Divergence",
    signalKey: "BTCFT_PRM_VOL_DIVERGENCE_0_SHORT",
    slPct: 0.55,
    tpPct: 1.65,
    cooldownMin: 30,
    holdMinutes: 95,
    confluenceMin: 6,
    requiresHtf: false,
    regimes: ["chop", "trendLow", "trendHigh"],
    btcFtTemplate: "PRM_VOL_DIVERGENCE",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["momentum"],
    templateFamily: "PRM_VOL_DIVERGENCE",
  },
  // -------------------- Strategy 3: EMA Trend Follow --------------------
  {
    id: 504,
    name: "PRM_EMA_TrendFollow_Long",
    category: "PREMIUM EMA Trend",
    signalKey: "BTCFT_PRM_EMA_TREND_FOLLOW_0_LONG",
    slPct: 0.55,
    tpPct: 1.80,
    cooldownMin: 20,
    holdMinutes: 75,
    confluenceMin: 6,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_EMA_TREND_FOLLOW",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["trend"],
    templateFamily: "PRM_EMA_TREND_FOLLOW",
  },
  {
    id: 505,
    name: "PRM_EMA_TrendFollow_Short",
    category: "PREMIUM EMA Trend",
    signalKey: "BTCFT_PRM_EMA_TREND_FOLLOW_0_SHORT",
    slPct: 0.55,
    tpPct: 1.80,
    cooldownMin: 20,
    holdMinutes: 75,
    confluenceMin: 6,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_EMA_TREND_FOLLOW",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["trend"],
    templateFamily: "PRM_EMA_TREND_FOLLOW",
  },
  // -------------------- Strategy 4: VWAP Scalp --------------------
  {
    id: 506,
    name: "PRM_VWAP_Scalp_Long",
    category: "PREMIUM VWAP Scalp",
    signalKey: "BTCFT_PRM_VWAP_SCALP_0_LONG",
    slPct: 0.45,
    tpPct: 1.35,
    cooldownMin: 15,
    holdMinutes: 25,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop", "trendLow"],
    btcFtTemplate: "PRM_VWAP_SCALP",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp"],
    playbooks: ["range"],
    templateFamily: "PRM_VWAP_SCALP",
  },
  {
    id: 507,
    name: "PRM_VWAP_Scalp_Short",
    category: "PREMIUM VWAP Scalp",
    signalKey: "BTCFT_PRM_VWAP_SCALP_0_SHORT",
    slPct: 0.45,
    tpPct: 1.35,
    cooldownMin: 15,
    holdMinutes: 25,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop", "trendLow"],
    btcFtTemplate: "PRM_VWAP_SCALP",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp"],
    playbooks: ["range"],
    templateFamily: "PRM_VWAP_SCALP",
  },
  // -------------------- Strategy 5: Funding Rate Fade --------------------
  {
    id: 508,
    name: "PRM_FundingFade_Long",
    category: "PREMIUM Funding Fade",
    signalKey: "BTCFT_PRM_FUNDING_FADE_0_LONG",
    slPct: 0.55,
    tpPct: 1.80,
    cooldownMin: 30,
    holdMinutes: 90,
    confluenceMin: 6,
    requiresHtf: false,
    regimes: ["trendHigh"],
    btcFtTemplate: "PRM_FUNDING_FADE",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["momentum"],
    templateFamily: "PRM_FUNDING_FADE",
  },
  {
    id: 509,
    name: "PRM_FundingFade_Short",
    category: "PREMIUM Funding Fade",
    signalKey: "BTCFT_PRM_FUNDING_FADE_0_SHORT",
    slPct: 0.55,
    tpPct: 1.80,
    cooldownMin: 30,
    holdMinutes: 90,
    confluenceMin: 6,
    requiresHtf: false,
    regimes: ["trendHigh"],
    btcFtTemplate: "PRM_FUNDING_FADE",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["momentum"],
    templateFamily: "PRM_FUNDING_FADE",
  },
  // -------------------- Strategy 6: Breakout + Retest --------------------
  {
    id: 510,
    name: "PRM_BreakoutRetest_Long",
    category: "PREMIUM Breakout Retest",
    signalKey: "BTCFT_PRM_BREAKOUT_RETEST_0_LONG",
    slPct: 0.52,
    tpPct: 1.56,
    cooldownMin: 15,
    holdMinutes: 50,
    confluenceMin: 5,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_BREAKOUT_RETEST",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["breakout"],
    templateFamily: "PRM_BREAKOUT_RETEST",
  },
  {
    id: 511,
    name: "PRM_BreakoutRetest_Short",
    category: "PREMIUM Breakout Retest",
    signalKey: "BTCFT_PRM_BREAKOUT_RETEST_0_SHORT",
    slPct: 0.52,
    tpPct: 1.56,
    cooldownMin: 15,
    holdMinutes: 50,
    confluenceMin: 5,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_BREAKOUT_RETEST",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["breakout"],
    templateFamily: "PRM_BREAKOUT_RETEST",
  },
  // -------------------- Strategy 7: BB Mean Reversion --------------------
  {
    id: 512,
    name: "PRM_BB_MeanRevert_Long",
    category: "PREMIUM BB Revert",
    signalKey: "BTCFT_PRM_BB_MEAN_REVERT_0_LONG",
    slPct: 0.50,
    tpPct: 1.50,
    cooldownMin: 15,
    holdMinutes: 45,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop"],
    btcFtTemplate: "PRM_BB_MEAN_REVERT",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp"],
    playbooks: ["range"],
    templateFamily: "PRM_BB_MEAN_REVERT",
  },
  {
    id: 513,
    name: "PRM_BB_MeanRevert_Short",
    category: "PREMIUM BB Revert",
    signalKey: "BTCFT_PRM_BB_MEAN_REVERT_0_SHORT",
    slPct: 0.50,
    tpPct: 1.50,
    cooldownMin: 15,
    holdMinutes: 45,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop"],
    btcFtTemplate: "PRM_BB_MEAN_REVERT",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp"],
    playbooks: ["range"],
    templateFamily: "PRM_BB_MEAN_REVERT",
  },
  // -------------------- Strategy 8: Ascending / Descending Triangle --------------------
  {
    id: 514,
    name: "PRM_AscTriangle_Long",
    category: "PREMIUM Asc Triangle",
    signalKey: "BTCFT_PRM_ASC_TRIANGLE_0_LONG",
    slPct: 0.52,
    tpPct: 1.56,
    cooldownMin: 15,
    holdMinutes: 45,
    confluenceMin: 5,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_ASC_TRIANGLE",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["breakout"],
    templateFamily: "PRM_ASC_TRIANGLE",
  },
  {
    id: 515,
    name: "PRM_DescTriangle_Short",
    category: "PREMIUM Desc Triangle",
    signalKey: "BTCFT_PRM_ASC_TRIANGLE_0_SHORT",
    slPct: 0.52,
    tpPct: 1.56,
    cooldownMin: 15,
    holdMinutes: 45,
    confluenceMin: 5,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_ASC_TRIANGLE",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["breakout"],
    templateFamily: "PRM_ASC_TRIANGLE",
  },
  // -------------------- Strategy 9: Bull / Bear Flag --------------------
  {
    id: 516,
    name: "PRM_BullFlag_Long",
    category: "PREMIUM Bull Flag",
    signalKey: "BTCFT_PRM_BULL_FLAG_0_LONG",
    slPct: 0.50,
    tpPct: 1.65,
    cooldownMin: 15,
    holdMinutes: 60,
    confluenceMin: 5,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_BULL_FLAG",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["trend", "breakout"],
    templateFamily: "PRM_BULL_FLAG",
  },
  {
    id: 517,
    name: "PRM_BearFlag_Short",
    category: "PREMIUM Bear Flag",
    signalKey: "BTCFT_PRM_BULL_FLAG_0_SHORT",
    slPct: 0.50,
    tpPct: 1.65,
    cooldownMin: 15,
    holdMinutes: 60,
    confluenceMin: 5,
    requiresHtf: true,
    regimes: ["trendLow", "trendHigh"],
    btcFtTemplate: "PRM_BULL_FLAG",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["trend", "breakout"],
    templateFamily: "PRM_BULL_FLAG",
  },
  // -------------------- Strategy 10: S/R Retest (mean20 proxy) --------------------
  {
    id: 518,
    name: "PRM_SR_Retest_Long",
    category: "PREMIUM S/R Retest",
    signalKey: "BTCFT_PRM_SR_RETEST_CLASSIC_0_LONG",
    slPct: 0.48,
    tpPct: 1.44,
    cooldownMin: 12,
    holdMinutes: 45,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop", "trendLow", "trendHigh"],
    btcFtTemplate: "PRM_SR_RETEST_CLASSIC",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp"],
    playbooks: ["range", "breakout"],
    templateFamily: "PRM_SR_RETEST_CLASSIC",
  },
  {
    id: 519,
    name: "PRM_SR_Retest_Short",
    category: "PREMIUM S/R Retest",
    signalKey: "BTCFT_PRM_SR_RETEST_CLASSIC_0_SHORT",
    slPct: 0.48,
    tpPct: 1.44,
    cooldownMin: 12,
    holdMinutes: 45,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop", "trendLow", "trendHigh"],
    btcFtTemplate: "PRM_SR_RETEST_CLASSIC",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp"],
    playbooks: ["range", "breakout"],
    templateFamily: "PRM_SR_RETEST_CLASSIC",
  },
  // -------------------- Strategy 11: Double Bottom / Double Top --------------------
  {
    id: 520,
    name: "PRM_DoubleBottom_Long",
    category: "PREMIUM Double Pattern",
    signalKey: "BTCFT_PRM_DOUBLE_PATTERN_0_LONG",
    slPct: 0.55,
    tpPct: 1.80,
    cooldownMin: 20,
    holdMinutes: 70,
    confluenceMin: 6,
    requiresHtf: false,
    regimes: ["chop", "trendLow"],
    btcFtTemplate: "PRM_DOUBLE_PATTERN",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["range", "momentum"],
    templateFamily: "PRM_DOUBLE_PATTERN",
  },
  {
    id: 521,
    name: "PRM_DoubleTop_Short",
    category: "PREMIUM Double Pattern",
    signalKey: "BTCFT_PRM_DOUBLE_PATTERN_0_SHORT",
    slPct: 0.55,
    tpPct: 1.80,
    cooldownMin: 20,
    holdMinutes: 70,
    confluenceMin: 6,
    requiresHtf: false,
    regimes: ["chop", "trendLow"],
    btcFtTemplate: "PRM_DOUBLE_PATTERN",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["range", "momentum"],
    templateFamily: "PRM_DOUBLE_PATTERN",
  },
  // -------------------- Strategy 12: Range Breakout (BB compression) --------------------
  {
    id: 522,
    name: "PRM_RangeBreak_Long",
    category: "PREMIUM Range Break",
    signalKey: "BTCFT_PRM_RANGE_BREAK_CLASSIC_0_LONG",
    slPct: 0.52,
    tpPct: 1.82,
    cooldownMin: 15,
    holdMinutes: 55,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop", "trendLow", "trendHigh"],
    btcFtTemplate: "PRM_RANGE_BREAK_CLASSIC",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["breakout"],
    templateFamily: "PRM_RANGE_BREAK_CLASSIC",
  },
  {
    id: 523,
    name: "PRM_RangeBreak_Short",
    category: "PREMIUM Range Break",
    signalKey: "BTCFT_PRM_RANGE_BREAK_CLASSIC_0_SHORT",
    slPct: 0.52,
    tpPct: 1.82,
    cooldownMin: 15,
    holdMinutes: 55,
    confluenceMin: 5,
    requiresHtf: false,
    regimes: ["chop", "trendLow", "trendHigh"],
    btcFtTemplate: "PRM_RANGE_BREAK_CLASSIC",
    btcFtVariant: 0,
    tier: "premium",
    styles: ["scalp", "day"],
    playbooks: ["breakout"],
    templateFamily: "PRM_RANGE_BREAK_CLASSIC",
  },
];

export const BTC_FT_PREMIUM_STRATEGY_IDS: readonly number[] = BTC_FT_PREMIUM_DEFS.map((d) => d.id);

/** Stable check used by the live hook to apply the premium notional multiplier. */
export function isPremiumStrategy(strat: { tier?: string }): boolean {
  return strat.tier === "premium";
}
