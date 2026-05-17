/**
 * Shared futures strategy shape (hook, replay, signals, BTC FT templates).
 * Split from `futuresStrategies.ts` so template generators avoid import cycles.
 */

export type RegimeTag = "chop" | "trendLow" | "trendHigh";

/** Keys for dedicated BTC FT extended scoring (`futuresSignals.ts`). */
export type BtcFtTemplateId =
  | "MTF_TREND"
  | "MTF_BREAK"
  | "MEAN_REVERT_BB"
  | "VWAP_REVERT"
  | "MOMENTUM_IMPULSE"
  | "SESSION_OPEN"
  | "WYCKOFF_TRAP"
  | "ORDERFLOW_PROXY";

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
}
