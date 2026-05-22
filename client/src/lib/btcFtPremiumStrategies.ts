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
export const BTC_FT_PREMIUM_ID_END = 503;

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
  },
];

export const BTC_FT_PREMIUM_STRATEGY_IDS: readonly number[] = BTC_FT_PREMIUM_DEFS.map((d) => d.id);

/** Stable check used by the live hook to apply the premium notional multiplier. */
export function isPremiumStrategy(strat: { tier?: string }): boolean {
  return strat.tier === "premium";
}
