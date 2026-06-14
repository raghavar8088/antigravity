/**
 * Research-backed BTC strategy registry.
 *
 * Strategy definitions have been removed from the application. Types and family
 * labels remain for UI filters and historical data that may still reference
 * strategy metadata.
 */

import type { MarketRegime } from "@/lib/ai/marketRegimeClassifier";
import type { OHLCVCandle, ResearchSignal } from "@/lib/ai/mockResearchIndicators";

export type BtcResearchFamily =
  | "VwapMeanReversion"
  | "VwapPullback"
  | "BBMeanReversion"
  | "BBSqueezeBreakout"
  | "AtrChannelBreakout"
  | "EmaCrossoverFiltered"
  | "TripleEma"
  | "MacdVwapMomentum"
  | "RsiFastExtreme"
  | "StochasticReversion"
  | "KeltnerRsiPullback"
  | "VolumeSpikeReversal"
  | "StopHuntSfp"
  | "RsiVwapRubberBand"
  | "EmaDeviationRevert"
  | "MacdDeceleration"
  | "BBStdDevRejection"
  | "EmaRibbonAlignment"
  | "ZScoreReversion"
  | "OpeningRangeBreakout"
  | "SessionOpenMomentum"
  | "InsideBarBreakout"
  | "CvdDivergenceStub"
  | "OiOvercrowdingStub"
  | "LiquidationCascadeStub";

export type BtcRequiredData =
  | "OHLCV"
  | "VOLUME"
  | "OI"
  | "FUNDING"
  | "LIQUIDATIONS"
  | "ORDER_BOOK";

export interface BtcResearchStrategy {
  id: number;
  name: string;
  family: BtcResearchFamily;
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
  dataFeedRequired: boolean;
  side: "LONG" | "SHORT" | "BOTH";
}

export const ALL_BTC_RESEARCH_FAMILIES: BtcResearchFamily[] = [
  "VwapMeanReversion",
  "VwapPullback",
  "BBMeanReversion",
  "BBSqueezeBreakout",
  "AtrChannelBreakout",
  "EmaCrossoverFiltered",
  "TripleEma",
  "MacdVwapMomentum",
  "RsiFastExtreme",
  "StochasticReversion",
  "KeltnerRsiPullback",
  "VolumeSpikeReversal",
  "StopHuntSfp",
  "RsiVwapRubberBand",
  "EmaDeviationRevert",
  "MacdDeceleration",
  "BBStdDevRejection",
  "EmaRibbonAlignment",
  "ZScoreReversion",
  "OpeningRangeBreakout",
  "SessionOpenMomentum",
  "InsideBarBreakout",
  "CvdDivergenceStub",
  "OiOvercrowdingStub",
  "LiquidationCascadeStub",
];

export const BTC_RESEARCH_FAMILY_LABELS: Record<BtcResearchFamily, string> = {
  VwapMeanReversion: "VWAP Mean Reversion",
  VwapPullback: "VWAP Pullback",
  BBMeanReversion: "Bollinger Mean Reversion",
  BBSqueezeBreakout: "Bollinger Squeeze Breakout",
  AtrChannelBreakout: "ATR Channel Breakout",
  EmaCrossoverFiltered: "EMA Crossover Filtered",
  TripleEma: "Triple EMA",
  MacdVwapMomentum: "MACD VWAP Momentum",
  RsiFastExtreme: "RSI Fast Extreme",
  StochasticReversion: "Stochastic Reversion",
  KeltnerRsiPullback: "Keltner RSI Pullback",
  VolumeSpikeReversal: "Volume Spike Reversal",
  StopHuntSfp: "Stop Hunt SFP",
  RsiVwapRubberBand: "RSI VWAP Rubber Band",
  EmaDeviationRevert: "EMA Deviation Reversion",
  MacdDeceleration: "MACD Deceleration",
  BBStdDevRejection: "BB Std Dev Rejection",
  EmaRibbonAlignment: "EMA Ribbon Alignment",
  ZScoreReversion: "Z-Score Reversion",
  OpeningRangeBreakout: "Opening Range Breakout",
  SessionOpenMomentum: "Session Open Momentum",
  InsideBarBreakout: "Inside Bar Breakout",
  CvdDivergenceStub: "CVD Divergence (Data Pending)",
  OiOvercrowdingStub: "OI Overcrowding (Data Pending)",
  LiquidationCascadeStub: "Liquidation Cascade (Data Pending)",
};

export const BTC_RESEARCH_STRATEGIES: BtcResearchStrategy[] = [];
