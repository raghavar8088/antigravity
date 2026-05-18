/**
 * Shared futures strategy shape (hook, replay, signals, BTC FT templates).
 * Split from `futuresStrategies.ts` so template generators avoid import cycles.
 */

export type RegimeTag = "chop" | "trendLow" | "trendHigh";

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
  // Premium hypothesis-driven strategies (IDs 500–503). Each has a documented
  // microstructure thesis, not pattern-matching. Tier "premium" treatment in hook.
  | "PRM_VWAP_REJECT"
  | "PRM_VOL_DIVERGENCE";

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
}
