/**
 * Shared futures strategy shape (hook, replay, signals, BTC FT templates).
 * Split from `futuresStrategies.ts` so template generators avoid import cycles.
 */

export type RegimeTag = "chop" | "trendLow" | "trendHigh";

/**
 * Semantic trading category — describes the *strategy type*, not the time horizon.
 * Each category has its own ID block (600–759), signal scorer, and entry gates.
 * CORE 20 legacy strategies are retroactively tagged via the migration map in
 * futuresCategoryStrategies.ts; they keep their original IDs and scoring.
 */
export type TradingCategoryId =
  | "scalping"
  | "day_trading"
  | "swing_trading"
  | "position_trading"
  | "trend_trading"
  | "range_trading"
  | "breakout_trading"
  | "momentum_trading";

/**
 * Time-horizon module identifiers. Each represents a distinct desk with its own
 * bar interval, poll cadence, leverage, and hold-time window.
 */
export type TradingStyleId = "scalp" | "day" | "swing" | "position";

/**
 * Signal-and-confirmation overlays that filter strategies within a desk.
 * They are NOT separate modules — they reduce the active roster and adjust
 * entry confirmation, hold multiplier, and cooldown.
 */
export type PlaybookId = "trend" | "range" | "breakout" | "momentum";

/** Keys for dedicated BTC FT extended scoring (`futuresSignals.ts`). */
export type BtcFtTemplateId =
  // Original extended templates (IDs 200–299)
  | "MTF_TREND"
  | "MTF_BREAK"
  | "MEAN_REVERT_BB"
  | "VWAP_REVERT"
  | "MOMENTUM_IMPULSE"
  | "SESSION_OPEN"
  | "WYCKOFF_TRAP"
  | "ORDERFLOW_PROXY"
  // Phase 1 generator templates (research pool, IDs 300–399, never auto-active in production)
  | "MTF_EMA_STACK"
  | "MTF_MACD_HIST"
  | "MTF_ADX_DI"
  | "MTF_DONCHIAN_BREAK"
  | "MEANREV_RSI"
  | "SESSION_RANGE_BREAK"
  | "WYCKOFF_SPRING"
  | "SMART_MONEY_FVG"
  // Premium hypothesis-driven strategies (IDs 500–513). Each has a documented
  // microstructure thesis, not pattern-matching. Tier "premium" treatment in hook.
  | "PRM_VWAP_REJECT"
  | "PRM_VOL_DIVERGENCE"
  | "PRM_EMA_TREND_FOLLOW"
  | "PRM_VWAP_SCALP"
  | "PRM_FUNDING_FADE"
  | "PRM_BREAKOUT_RETEST"
  | "PRM_BB_MEAN_REVERT";

export interface FuturesStratDef {
  id: number;
  name: string;
  category: string;
  signalKey: string;
  slPct: number;
  tpPct: number;
  cooldownMin: number;
  holdMinutes: number;
  confluenceMin: number;
  requiresHtf?: boolean;
  deskTpWidened?: boolean;
  regimes?: RegimeTag[];
  /** Extended BTC FT templates (IDs ≥200): dedicated signal + confirm branches. */
  btcFtTemplate?: BtcFtTemplateId;
  /** Parameter grid index 0–3 (SL/TP/hold scaling in generator). */
  btcFtVariant?: number;
  /**
   * Tier classification. "premium" strategies are hypothesis-driven
   * (e.g. VWAP session reject, volume divergence) and receive special
   * treatment in the hook: 2× notional, separate UI badge, always active
   * in production roster regardless of pool-mode env flags.
   */
  tier?: "premium";
  /**
   * Per-strategy signal threshold override (P1.1.2 adaptive threshold).
   * When set, replaces the global `signalThreshold` for this strategy only.
   * Computed from rolling win-rate by `computeAdaptiveThreshold`; injected
   * at runtime by the hook or ranking pipeline, never hard-coded in strat defs.
   */
  dynamicThreshold?: number;
  /**
   * Which time-horizon desks may use this strategy. When absent, all styles
   * are eligible (backward-compatible with pre-tagging strats).
   */
  styles?: readonly TradingStyleId[];
  /**
   * Signal/confirmation playbooks this strategy is suited for.
   * When absent, the strategy is eligible regardless of active playbook filter.
   */
  playbooks?: readonly PlaybookId[];
  /**
   * Explicit family dedup key for burst guard and template-family cap.
   * Falls back to `btcFtTemplateFamilyKey(name)` when absent.
   */
  templateFamily?: string;
  /**
   * Minimum warm-up bars required by this strategy's indicators.
   * Overrides the style-level MIN_BARS default when present.
   */
  minBars?: number;
  /**
   * Semantic trading category. Required for all IDs 600–759 (research pool).
   * Optional on CORE 20 (91–152) and premium (500–503) — set via migration map.
   */
  tradingCategory?: TradingCategoryId;
  /**
   * Primary bar interval this strategy was designed for. Defaults to the desk
   * bar interval when absent; used by multi-TF engine to fetch the right cache.
   */
  primaryBarInterval?: "1m" | "5m" | "15m" | "4h" | "1d";
  /**
   * Optional higher-timeframe confirmation bar interval.
   * When set, the engine fetches this TF and passes htf fields to scoring.
   */
  confirmBarInterval?: "5m" | "15m" | "1h" | "4h";
  /**
   * Per-strategy leverage override. When set, overrides the category default
   * from CATEGORY_REGISTRY. Must not exceed category maxLeverage.
   */
  defaultLeverage?: number;
  /**
   * When true: strategy is in the research pool only.
   * It will NOT appear in the live production roster unless explicitly promoted
   * via rankings (winnerIdsFromRankings) and WINNERS_ONLY mode.
   * All IDs 600–759 are researchOnly: true.
   */
  researchOnly?: boolean;
}
