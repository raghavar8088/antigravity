/**
 * Institutional BTC futures strategy registry.
 *
 * Strategy definitions have been removed from the application. Public types and
 * family labels remain for historical records and UI filters.
 */

import type { MarketRegime } from "@/lib/ai/marketRegimeClassifier";
import type { OHLCVCandle, ResearchSignal } from "@/lib/ai/mockResearchIndicators";
import type { BtcRequiredData } from "@/lib/trading/btcResearchStrategyRegistry";

export type InstitutionalFamily =
  | "VwapTrendPullback"
  | "LiquiditySweepReversal"
  | "BBKeltnerSqueeze"
  | "EmaPullbackRider"
  | "VolumeSpikeM"
  | "AtrCompressionBreakout"
  | "RsiVwapRubberBand"
  | "BollingerRejectionFade"
  | "MacdMomentumDecel"
  | "LiquidationCascadeSnap";

export const INSTITUTIONAL_FAMILY_LABELS: Record<InstitutionalFamily, string> = {
  VwapTrendPullback: "VWAP Trend Pullback Pro",
  LiquiditySweepReversal: "Liquidity Sweep Reversal",
  BBKeltnerSqueeze: "BB-Keltner Squeeze Expansion",
  EmaPullbackRider: "EMA Pullback Rider",
  VolumeSpikeM: "Volume Spike Momentum",
  AtrCompressionBreakout: "ATR Compression Breakout",
  RsiVwapRubberBand: "RSI-VWAP Rubber Band",
  BollingerRejectionFade: "Bollinger Rejection Fade",
  MacdMomentumDecel: "MACD Momentum Deceleration",
  LiquidationCascadeSnap: "Liquidation Cascade Snap-Back",
};

export const ALL_INSTITUTIONAL_FAMILIES = Object.keys(
  INSTITUTIONAL_FAMILY_LABELS,
) as InstitutionalFamily[];

export interface InstitutionalStrategyMeta {
  tier: 1 | 2 | 3;
  timeframes: string[];
  tpPct: number;
  slPct: number;
  trailingStopPct: number;
  trailingStopLogic: string;
  confidenceScore: number;
  signalScore: number;
  riskScore: number;
  estimatedWinRate: number;
  expectedProfitAfterFees: number;
  expectedLossAfterFees: number;
  netExpectancyEstimate: number;
  bestRegimes: MarketRegime[];
  worstRegimes: MarketRegime[];
  indicatorSnapshot: (candles: OHLCVCandle[]) => Record<string, number>;
}

export interface InstitutionalStrategy {
  id: number;
  name: string;
  family: InstitutionalFamily;
  enabled: true;
  description: string;
  params: Record<string, number | string>;
  timeframe: "1m" | "5m" | "15m";
  minCandles: number;
  signal: (candles: OHLCVCandle[]) => ResearchSignal;
  entryRules: string[];
  exitRules: string[];
  stopLossRules: string[];
  takeProfitRules: string[];
  requiredIndicators: string[];
  requiredData: BtcRequiredData[];
  bestRegime: MarketRegime[];
  worstRegime: MarketRegime[];
  researchConfidenceScore: number;
  sourceDocument: string;
  dataFeedRequired: false;
  side: "LONG" | "SHORT" | "BOTH";
  meta: InstitutionalStrategyMeta;
}

export const INSTITUTIONAL_STRATEGIES: InstitutionalStrategy[] = [];

export const INSTITUTIONAL_STRATEGY_BY_ID: ReadonlyMap<number, InstitutionalStrategy> = new Map();

export const INSTITUTIONAL_STRATEGY_IDS: readonly number[] = [];
