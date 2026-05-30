/**
 * Research-backed BTC strategy registry for the mock research lab.
 *
 * ISOLATION: These strategies emit metadata-only mock signals. They never call
 * broker, exchange, OMS, paper desk worker, or live execution APIs.
 */

import {
  NO_SIGNAL,
  calcAdx,
  calcAtr,
  calcBollinger,
  calcDonchian,
  calcEma,
  calcEmaSlope,
  calcKeltner,
  calcMacd,
  calcRsi,
  calcStochastic,
  calcVolumeRatio,
  calcVwap,
  calcZScore,
  clampConf,
  isBearishPinBar,
  isBullishPinBar,
  type OHLCVCandle,
  type ResearchSignal,
} from "@/lib/mockResearchIndicators";
import type { MarketRegime } from "@/lib/marketRegimeClassifier";

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

type StrategySide = "BUY" | "SELL";

interface StrategyDefinition {
  family: BtcResearchFamily;
  baseName: string;
  description: string;
  sourceDocument: string;
  entryRules: string[];
  exitRules: string[];
  stopLossRules: string[];
  takeProfitRules: string[];
  requiredIndicators: string[];
  requiredData: BtcRequiredData[];
  bestRegime: MarketRegime[];
  worstRegime: MarketRegime[];
  researchConfidenceScore: number;
  minCandles: number;
  timeframe?: "1m" | "5m" | "15m";
  params: Record<string, number | string>;
  signalFactory: (side: StrategySide, params: Record<string, number | string>) => (candles: OHLCVCandle[]) => ResearchSignal;
}

const ALGO_SOURCE = "BTC Algorithmic Trading Strategy Research.pdf";
const INTRADAY_SOURCE = "BTC Intraday Trading Strategy Families (Prioritized).pdf";
const CLAUDE_SOURCE = "BTC CLAUDE RESEARCH.pdf";

function n(params: Record<string, number | string>, key: string, fallback: number): number {
  const value = params[key];
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function latest(candles: readonly OHLCVCandle[]): OHLCVCandle | null {
  return candles[candles.length - 1] ?? null;
}

function candleBody(candle: OHLCVCandle): number {
  return Math.abs(candle.close - candle.open);
}

function std(values: readonly number[]): number {
  if (values.length === 0) return 0;
  const mean = values.reduce((sum, value) => sum + value, 0) / values.length;
  return Math.sqrt(values.reduce((sum, value) => sum + (value - mean) ** 2, 0) / values.length);
}

function sideSignal(side: StrategySide, confidence: number): ResearchSignal {
  return { side, confidence: clampConf(confidence) };
}

function crossedAbove(prevFast: number, prevSlow: number, fast: number, slow: number): boolean {
  return prevFast <= prevSlow && fast > slow;
}

function crossedBelow(prevFast: number, prevSlow: number, fast: number, slow: number): boolean {
  return prevFast >= prevSlow && fast < slow;
}

function swingHigh(candles: readonly OHLCVCandle[], period: number): number {
  const slice = candles.slice(-period - 1, -1);
  return slice.length > 0 ? Math.max(...slice.map((candle) => candle.high)) : 0;
}

function swingLow(candles: readonly OHLCVCandle[], period: number): number {
  const slice = candles.slice(-period - 1, -1);
  return slice.length > 0 ? Math.min(...slice.map((candle) => candle.low)) : 0;
}

function vwapMeanReversion(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const rsi = calcRsi(closes, n(params, "rsi", 14));
    const vwap = calcVwap(candles);
    const deviation = n(params, "deviation", 2) * std(closes.slice(-20));
    const adx = calcAdx(candles, 14);
    const long = rsi < n(params, "longRsi", 25) && candle.close < vwap - deviation && adx < 20;
    const short = rsi > n(params, "shortRsi", 75) && candle.close > vwap + deviation && adx < 20;
    if ((side === "BUY" && long) || (side === "SELL" && short)) {
      return sideSignal(side, 62 + Math.abs(candle.close - vwap) / Math.max(1, deviation) * 8);
    }
    return NO_SIGNAL;
  };
}

function vwapPullback(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const vwap = calcVwap(candles);
    const adx = calcAdx(candles, 14);
    const emaSlope = calcEmaSlope(closes, n(params, "ema", 50), 10);
    const tolerance = (n(params, "toleranceBps", 18) / 10_000) * candle.close;
    const touched = candle.low <= vwap + tolerance && candle.high >= vwap - tolerance;
    const long = adx > 25 && emaSlope > 0 && touched && candle.close > vwap && candle.close > candle.open;
    const short = adx > 25 && emaSlope < 0 && touched && candle.close < vwap && candle.close < candle.open;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 58 + adx);
    return NO_SIGNAL;
  };
}

function rsiVwapRubberBand(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const rsi = calcRsi(closes, n(params, "rsi", 14));
    const vwap = calcVwap(candles);
    const adx = calcAdx(candles, 14);
    const stretch = n(params, "vwapStretchPct", 1.5) / 100;
    const long = rsi < 20 && candle.close < vwap * (1 - stretch) && adx < 20;
    const short = rsi > 80 && candle.close > vwap * (1 + stretch) && adx < 20;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 72);
    return NO_SIGNAL;
  };
}

function bbMeanReversion(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const bb = calcBollinger(closes, n(params, "period", 20), n(params, "mult", 2));
    const rsi = calcRsi(closes, n(params, "rsi", 14));
    const adx = calcAdx(candles, 14);
    const long = candle.low <= bb.lower && candle.close > bb.lower && rsi < 25 && adx < 20;
    const short = candle.high >= bb.upper && candle.close < bb.upper && rsi > 75 && adx < 20;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 70);
    return NO_SIGNAL;
  };
}

function bbSqueezeBreakout(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle || candles.length < 25) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    let insideCount = 0;
    for (let offset = 0; offset < 5; offset++) {
      const slice = candles.slice(0, candles.length - offset);
      const sliceCloses = slice.map((c) => c.close);
      const bb = calcBollinger(sliceCloses, 20, 2);
      const kc = calcKeltner(slice, 20, 1.5);
      if (bb.upper < kc.upper && bb.lower > kc.lower) insideCount++;
    }
    const bb = calcBollinger(closes, 20, 2);
    const volRatio = calcVolumeRatio(candles, 20);
    const long = insideCount >= 3 && candle.close > bb.upper && volRatio > n(params, "volumeRatio", 1.5);
    const short = insideCount >= 3 && candle.close < bb.lower && volRatio > n(params, "volumeRatio", 1.5);
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 74 + volRatio * 4);
    return NO_SIGNAL;
  };
}

function zScoreReversion(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const z = calcZScore(candles.map((c) => c.close), n(params, "period", 20));
    if (side === "BUY" && z < -n(params, "z", 2)) return sideSignal(side, 65 + Math.abs(z) * 5);
    if (side === "SELL" && z > n(params, "z", 2)) return sideSignal(side, 65 + Math.abs(z) * 5);
    return NO_SIGNAL;
  };
}

function bbStdDevRejection(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const bb = calcBollinger(candles.map((c) => c.close), 20, n(params, "mult", 2));
    const long = candle.low < bb.lower && candle.close > bb.lower && isBullishPinBar(candle);
    const short = candle.high > bb.upper && candle.close < bb.upper && isBearishPinBar(candle);
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 76);
    return NO_SIGNAL;
  };
}

function emaCrossoverFiltered(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const closes = candles.map((c) => c.close);
    const fast = n(params, "fast", 9);
    const slow = n(params, "slow", 21);
    const trend = n(params, "trend", 50);
    const currFast = calcEma(closes, fast);
    const currSlow = calcEma(closes, slow);
    const prevFast = calcEma(closes.slice(0, -1), fast);
    const prevSlow = calcEma(closes.slice(0, -1), slow);
    const emaTrend = calcEma(closes, trend);
    const rsi = calcRsi(closes, 14);
    const adx = calcAdx(candles, 14);
    const price = closes[closes.length - 1] ?? 0;
    const long = crossedAbove(prevFast, prevSlow, currFast, currSlow) && price > emaTrend && rsi > 50 && adx > 25;
    const short = crossedBelow(prevFast, prevSlow, currFast, currSlow) && price < emaTrend && rsi < 50 && adx > 25;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 60 + adx);
    return NO_SIGNAL;
  };
}

function tripleEmaPullback(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const fast = calcEma(closes, n(params, "fast", 9));
    const mid = calcEma(closes, n(params, "mid", 21));
    const slow = calcEma(closes, n(params, "slow", 55));
    const atr = calcAtr(candles, 14);
    const long = fast > mid && mid > slow && candle.low <= mid + atr * 0.25 && candle.close > mid;
    const short = fast < mid && mid < slow && candle.high >= mid - atr * 0.25 && candle.close < mid;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 68);
    return NO_SIGNAL;
  };
}

function emaRibbonAlignment(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const closes = candles.map((c) => c.close);
    const e5 = calcEma(closes, 5);
    const e8 = calcEma(closes, 8);
    const e13 = calcEma(closes, 13);
    const e21 = calcEma(closes, 21);
    const slope = calcEmaSlope(closes, n(params, "slopeEma", 21), 5);
    const long = e5 > e8 && e8 > e13 && e13 > e21 && slope > 0.0005;
    const short = e5 < e8 && e8 < e13 && e13 < e21 && slope < -0.0005;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 62 + Math.abs(slope) * 20_000);
    return NO_SIGNAL;
  };
}

function emaDeviationRevert(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const ema = calcEma(closes, n(params, "ema", 200));
    const deviation = n(params, "deviationPct", 1) / 100;
    const long = candle.close < ema * (1 - deviation);
    const short = candle.close > ema * (1 + deviation);
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 66);
    return NO_SIGNAL;
  };
}

function rsiFastExtreme(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const closes = candles.map((c) => c.close);
    const period = n(params, "rsi", 7);
    const prev = calcRsi(closes.slice(0, -1), period);
    const curr = calcRsi(closes, period);
    const long = prev < 10 && curr > 15;
    const short = prev > 90 && curr < 85;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 78);
    return NO_SIGNAL;
  };
}

function stochasticReversion(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const curr = calcStochastic(candles, n(params, "k", 14), n(params, "d", 3));
    const prev = calcStochastic(candles.slice(0, -1), n(params, "k", 14), n(params, "d", 3));
    const adx = calcAdx(candles, 14);
    const long = prev.k <= prev.d && curr.k > curr.d && curr.k < 25 && adx < 20;
    const short = prev.k >= prev.d && curr.k < curr.d && curr.k > 75 && adx < 20;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 70);
    return NO_SIGNAL;
  };
}

function keltnerRsiPullback(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const kc = calcKeltner(candles, 20, n(params, "atrMult", 2));
    const rsi = calcRsi(closes, 14);
    const slope = calcEmaSlope(closes, 50, 10);
    const long = slope > 0 && candle.close > kc.middle && candle.low <= kc.middle && rsi > 35 && rsi < 55;
    const short = slope < 0 && candle.close < kc.middle && candle.high >= kc.middle && rsi < 65 && rsi > 45;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 66);
    return NO_SIGNAL;
  };
}

function macdDeceleration(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const closes = candles.map((c) => c.close);
    const macd1 = calcMacd(closes.slice(0, -2));
    const macd2 = calcMacd(closes.slice(0, -1));
    const macd3 = calcMacd(closes);
    const long = macd1.histogram < 0 && macd2.histogram < 0 && macd3.histogram < 0 && macd1.histogram < macd2.histogram && macd2.histogram < macd3.histogram;
    const short = macd1.histogram > 0 && macd2.histogram > 0 && macd3.histogram > 0 && macd1.histogram > macd2.histogram && macd2.histogram > macd3.histogram;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 64);
    return NO_SIGNAL;
  };
}

function macdVwapMomentum(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const closes = candles.map((c) => c.close);
    const vwap = calcVwap(candles);
    const macd = calcMacd(closes);
    const prevMacd = calcMacd(closes.slice(0, -1));
    const long = candle.close > vwap && crossedAbove(prevMacd.line, prevMacd.signal, macd.line, macd.signal);
    const short = candle.close < vwap && crossedBelow(prevMacd.line, prevMacd.signal, macd.line, macd.signal);
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 72);
    return NO_SIGNAL;
  };
}

function stopHuntSfp(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const lookback = n(params, "lookback", 20);
    const low = swingLow(candles, lookback);
    const high = swingHigh(candles, lookback);
    const long = low > 0 && candle.low < low && candle.close > low;
    const short = high > 0 && candle.high > high && candle.close < high;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 80);
    return NO_SIGNAL;
  };
}

function volumeSpikeReversal(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const volRatio = calcVolumeRatio(candles, 20);
    const range = candle.high - candle.low;
    if (range <= 0 || volRatio < n(params, "volumeRatio", 2)) return NO_SIGNAL;
    const long = isBullishPinBar(candle) && candle.close > candle.low + range * 0.6;
    const short = isBearishPinBar(candle) && candle.close < candle.high - range * 0.6;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 62 + volRatio * 8);
    return NO_SIGNAL;
  };
}

function atrChannelBreakout(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const candle = latest(candles);
    if (!candle) return NO_SIGNAL;
    const lookback = n(params, "lookback", 20);
    const channel = calcDonchian(candles.slice(0, -1), lookback);
    const atr = calcAtr(candles, 14);
    const atrPct = candle.close > 0 ? atr / candle.close : 0;
    const minAtrPct = n(params, "minAtrPct", 0.001);
    const volumeRatio = calcVolumeRatio(candles, 20);
    const long = atrPct > minAtrPct && candle.close > channel.upper && volumeRatio > 1.1;
    const short = atrPct > minAtrPct && candle.close < channel.lower && volumeRatio > 1.1;
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 68 + volumeRatio * 5);
    return NO_SIGNAL;
  };
}

function insideBarBreakout(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 3) return NO_SIGNAL;
    const mother = candles[candles.length - 2];
    const child = candles[candles.length - 1];
    const prior = candles[candles.length - 3];
    const wasInside = mother.high < prior.high && mother.low > prior.low;
    const long = wasInside && child.close > mother.high && calcVolumeRatio(candles, 20) > n(params, "volumeRatio", 1.2);
    const short = wasInside && child.close < mother.low && calcVolumeRatio(candles, 20) > n(params, "volumeRatio", 1.2);
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 64);
    return NO_SIGNAL;
  };
}

function openingRangeBreakout(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    const rangeBars = n(params, "rangeBars", 15);
    const candle = latest(candles);
    if (!candle || candles.length < rangeBars + 1) return NO_SIGNAL;
    const range = candles.slice(-rangeBars - 1, -1);
    const high = Math.max(...range.map((c) => c.high));
    const low = Math.min(...range.map((c) => c.low));
    const volumeRatio = calcVolumeRatio(candles, 20);
    const long = candle.close > high && volumeRatio > n(params, "volumeRatio", 1.5);
    const short = candle.close < low && volumeRatio > n(params, "volumeRatio", 1.5);
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 70 + volumeRatio * 5);
    return NO_SIGNAL;
  };
}

function sessionOpenMomentum(side: StrategySide, params: Record<string, number | string>) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 4) return NO_SIGNAL;
    const recent = candles.slice(-3);
    const volumeRatio = calcVolumeRatio(candles, 20);
    const bodies = recent.map(candleBody);
    const avgBody = bodies.reduce((sum, value) => sum + value, 0) / bodies.length;
    const atr = calcAtr(candles, 14);
    const long = recent.every((c) => c.close > c.open) && avgBody > atr * 0.35 && volumeRatio > n(params, "volumeRatio", 1.3);
    const short = recent.every((c) => c.close < c.open) && avgBody > atr * 0.35 && volumeRatio > n(params, "volumeRatio", 1.3);
    if ((side === "BUY" && long) || (side === "SELL" && short)) return sideSignal(side, 66 + volumeRatio * 5);
    return NO_SIGNAL;
  };
}

const DEFINITIONS: StrategyDefinition[] = [
  {
    family: "VwapMeanReversion",
    baseName: "VWAP Mean Reversion",
    description: "Fade stretched BTC candles back toward session VWAP when RSI is extreme and ADX confirms range conditions.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["RSI is extreme", "Price closes beyond VWAP by a volatility-adjusted deviation", "ADX remains below 20"],
    exitRules: ["Exit on VWAP touch or opposing oscillator extreme"],
    stopLossRules: ["Stop beyond recent swing or 0.8-1.2 ATR from entry"],
    takeProfitRules: ["Primary target is VWAP; secondary target is opposite deviation band"],
    requiredIndicators: ["VWAP", "RSI", "ADX", "Standard deviation"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["RANGING"],
    worstRegime: ["TRENDING", "HIGH_VOLATILITY_BREAKOUT"],
    researchConfidenceScore: 88,
    minCandles: 60,
    params: { rsi: 14, deviation: 2, longRsi: 25, shortRsi: 75 },
    signalFactory: vwapMeanReversion,
  },
  {
    family: "VwapPullback",
    baseName: "VWAP Pullback Trend",
    description: "Join an established intraday trend when price rejects a VWAP pullback with ADX confirmation.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["ADX above 25", "EMA slope agrees with trade direction", "Candle touches and reclaims/rejects VWAP"],
    exitRules: ["Exit on VWAP failure or momentum deceleration"],
    stopLossRules: ["Stop beyond VWAP rejection wick"],
    takeProfitRules: ["Target 1.5-2.5R or prior session high/low"],
    requiredIndicators: ["VWAP", "ADX", "EMA slope"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 84,
    minCandles: 70,
    params: { ema: 50, toleranceBps: 18 },
    signalFactory: vwapPullback,
  },
  {
    family: "RsiVwapRubberBand",
    baseName: "RSI VWAP Rubber Band",
    description: "Mean-revert BTC when RSI and VWAP displacement both show an exhausted intraday move.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["RSI < 20 for longs or > 80 for shorts", "Price stretches at least 1.5% from VWAP", "ADX below 20"],
    exitRules: ["Exit on VWAP mean reversion or RSI normalization"],
    stopLossRules: ["Stop if price extends another 0.5-0.8% beyond entry"],
    takeProfitRules: ["Target VWAP or half-distance back to VWAP in fast markets"],
    requiredIndicators: ["RSI", "VWAP", "ADX"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["RANGING"],
    worstRegime: ["TRENDING"],
    researchConfidenceScore: 82,
    minCandles: 60,
    params: { rsi: 14, vwapStretchPct: 1.5 },
    signalFactory: rsiVwapRubberBand,
  },
  {
    family: "BBMeanReversion",
    baseName: "Bollinger Mean Reversion",
    description: "Fade Bollinger band rejection in low-ADX regimes after candle reclaim/rejection of the outer band.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["Candle pierces outer Bollinger band", "Close returns inside band", "RSI confirms exhaustion", "ADX below 20"],
    exitRules: ["Exit at middle band or opposite band"],
    stopLossRules: ["Stop beyond rejected wick"],
    takeProfitRules: ["Target middle Bollinger band first, then opposite band"],
    requiredIndicators: ["Bollinger Bands", "RSI", "ADX"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING"],
    worstRegime: ["HIGH_VOLATILITY_BREAKOUT", "TRENDING"],
    researchConfidenceScore: 86,
    minCandles: 40,
    params: { period: 20, mult: 2, rsi: 14 },
    signalFactory: bbMeanReversion,
  },
  {
    family: "BBSqueezeBreakout",
    baseName: "Bollinger Squeeze Breakout",
    description: "Trade expansion after Bollinger compression inside Keltner channels with volume confirmation.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Bollinger bands compress inside Keltner channels", "Close breaks outside Bollinger band", "Volume > threshold"],
    exitRules: ["Exit on failed breakout back inside band or ATR target"],
    stopLossRules: ["Stop inside compression range"],
    takeProfitRules: ["Target 1.5-3 ATR depending on expansion strength"],
    requiredIndicators: ["Bollinger Bands", "Keltner Channels", "Volume ratio"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 90,
    minCandles: 50,
    params: { volumeRatio: 1.5 },
    signalFactory: bbSqueezeBreakout,
  },
  {
    family: "ZScoreReversion",
    baseName: "Z-Score Reversion",
    description: "Fade statistically stretched closes when price deviates more than two standard deviations from its rolling mean.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["Rolling close z-score beyond threshold"],
    exitRules: ["Exit when z-score reverts toward zero"],
    stopLossRules: ["Stop if z-score expands beyond 3"],
    takeProfitRules: ["Target rolling mean"],
    requiredIndicators: ["Z-score"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING"],
    worstRegime: ["TRENDING"],
    researchConfidenceScore: 72,
    minCandles: 40,
    params: { period: 20, z: 2 },
    signalFactory: zScoreReversion,
  },
  {
    family: "BBStdDevRejection",
    baseName: "Bollinger Std Dev Rejection",
    description: "Trade pin-bar rejection after BTC wicks outside a Bollinger envelope and closes back inside.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["Wick extends outside Bollinger band", "Body closes back inside", "Pin-bar rejection candle"],
    exitRules: ["Exit at middle or opposite band"],
    stopLossRules: ["Stop beyond rejection wick"],
    takeProfitRules: ["Target middle Bollinger band"],
    requiredIndicators: ["Bollinger Bands", "Candle wick structure"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING"],
    worstRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    researchConfidenceScore: 76,
    minCandles: 40,
    params: { mult: 2 },
    signalFactory: bbStdDevRejection,
  },
  {
    family: "EmaCrossoverFiltered",
    baseName: "EMA 9/21 + 50 Filter",
    description: "Use 9/21 EMA crossover only when price and momentum align with the 50 EMA trend filter.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["EMA fast crosses EMA slow", "Price is on correct side of EMA50", "RSI agrees", "ADX above 25"],
    exitRules: ["Exit on opposite EMA cross or loss of EMA50 trend filter"],
    stopLossRules: ["Stop below EMA50 or recent pullback low"],
    takeProfitRules: ["Target 1.5-2R or trail under EMA21"],
    requiredIndicators: ["EMA", "RSI", "ADX"],
    requiredData: ["OHLCV"],
    bestRegime: ["TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP", "RANGING"],
    researchConfidenceScore: 84,
    minCandles: 70,
    params: { fast: 9, slow: 21, trend: 50 },
    signalFactory: emaCrossoverFiltered,
  },
  {
    family: "TripleEma",
    baseName: "Triple EMA Pullback",
    description: "Trade pullbacks to the middle EMA when fast/mid/slow EMAs are stacked in trend order.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Fast EMA > mid EMA > slow EMA for longs, inverse for shorts", "Pullback touches mid EMA", "Close rejects in trend direction"],
    exitRules: ["Trail using slow EMA or exit on stack break"],
    stopLossRules: ["Stop beyond slow EMA or pullback wick"],
    takeProfitRules: ["Target recent swing extension or 2R"],
    requiredIndicators: ["EMA", "ATR"],
    requiredData: ["OHLCV"],
    bestRegime: ["TRENDING"],
    worstRegime: ["RANGING"],
    researchConfidenceScore: 80,
    minCandles: 80,
    params: { fast: 9, mid: 21, slow: 55 },
    signalFactory: tripleEmaPullback,
  },
  {
    family: "EmaRibbonAlignment",
    baseName: "EMA Ribbon Alignment",
    description: "Follow strong BTC trend continuation when short EMA ribbon is fully stacked and sloping.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["EMA 5/8/13/21 stacked in direction", "EMA slope confirms trend pressure"],
    exitRules: ["Exit when ribbon compresses or flips"],
    stopLossRules: ["Stop beyond EMA21"],
    takeProfitRules: ["Trail while ribbon remains stacked"],
    requiredIndicators: ["EMA ribbon", "EMA slope"],
    requiredData: ["OHLCV"],
    bestRegime: ["TRENDING", "HIGH_VOLATILITY_BREAKOUT"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 78,
    minCandles: 50,
    params: { slopeEma: 21 },
    signalFactory: emaRibbonAlignment,
  },
  {
    family: "EmaDeviationRevert",
    baseName: "200 EMA Rapid Deviation",
    description: "Mean-revert sharp deviations from the 200 EMA toward shorter-term equilibrium.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["Price deviates more than threshold from EMA200"],
    exitRules: ["Exit at EMA50/EMA200 midpoint or mean reversion completion"],
    stopLossRules: ["Stop if deviation expands materially"],
    takeProfitRules: ["Target EMA50 or half-distance to EMA200"],
    requiredIndicators: ["EMA200"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING", "LOW_VOLATILITY_CHOP"],
    worstRegime: ["TRENDING"],
    researchConfidenceScore: 70,
    minCandles: 210,
    params: { ema: 200, deviationPct: 1 },
    signalFactory: emaDeviationRevert,
  },
  {
    family: "RsiFastExtreme",
    baseName: "RSI(7) Extreme Reversal",
    description: "Trade a fast RSI snapback after BTC reaches extreme oscillator exhaustion.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["RSI(7) pierces extreme zone", "RSI crosses back through trigger level"],
    exitRules: ["Exit at RSI neutral zone"],
    stopLossRules: ["Stop beyond trigger candle"],
    takeProfitRules: ["Target prior micro swing or VWAP"],
    requiredIndicators: ["RSI"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING"],
    worstRegime: ["TRENDING"],
    researchConfidenceScore: 74,
    minCandles: 20,
    params: { rsi: 7 },
    signalFactory: rsiFastExtreme,
  },
  {
    family: "StochasticReversion",
    baseName: "Stochastic Reversion",
    description: "Fade short-term exhaustion when stochastic %K crosses %D in oversold/overbought territory.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["%K crosses %D below 20 for longs or above 80 for shorts", "ADX below 20"],
    exitRules: ["Exit at stochastic midline or opposite cross"],
    stopLossRules: ["Stop beyond recent range extreme"],
    takeProfitRules: ["Target VWAP or middle Bollinger band"],
    requiredIndicators: ["Stochastic", "ADX"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING"],
    worstRegime: ["TRENDING"],
    researchConfidenceScore: 76,
    minCandles: 35,
    params: { k: 14, d: 3 },
    signalFactory: stochasticReversion,
  },
  {
    family: "KeltnerRsiPullback",
    baseName: "Keltner RSI Pullback",
    description: "Use Keltner channel midpoint as trend support/resistance and RSI as pullback reset confirmation.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["Trend slope is positive/negative", "Price touches Keltner middle", "RSI resets without reaching exhaustion"],
    exitRules: ["Exit at outer Keltner channel or slope failure"],
    stopLossRules: ["Stop beyond Keltner middle and pullback wick"],
    takeProfitRules: ["Target outer Keltner channel"],
    requiredIndicators: ["Keltner Channels", "RSI", "EMA slope"],
    requiredData: ["OHLCV"],
    bestRegime: ["TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 75,
    minCandles: 70,
    params: { atrMult: 2 },
    signalFactory: keltnerRsiPullback,
  },
  {
    family: "MacdDeceleration",
    baseName: "MACD Deceleration",
    description: "Detect waning BTC downside/upside momentum through three improving MACD histogram bars.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["MACD histogram remains below/above zero", "Last three histogram bars decelerate toward zero"],
    exitRules: ["Exit on MACD signal cross or histogram re-acceleration"],
    stopLossRules: ["Stop beyond recent swing"],
    takeProfitRules: ["Target VWAP or EMA21"],
    requiredIndicators: ["MACD"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING", "TRENDING"],
    worstRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    researchConfidenceScore: 72,
    minCandles: 45,
    params: {},
    signalFactory: macdDeceleration,
  },
  {
    family: "MacdVwapMomentum",
    baseName: "MACD + VWAP Momentum",
    description: "Trade MACD signal cross when BTC holds the correct side of VWAP support/resistance.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["Price is above VWAP for longs/below VWAP for shorts", "MACD crosses signal in trade direction"],
    exitRules: ["Exit on VWAP loss or opposite MACD cross"],
    stopLossRules: ["Stop beyond VWAP"],
    takeProfitRules: ["Target 1.5-2R or prior swing"],
    requiredIndicators: ["MACD", "VWAP"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 82,
    minCandles: 60,
    params: {},
    signalFactory: macdVwapMomentum,
  },
  {
    family: "StopHuntSfp",
    baseName: "Stop Hunt Swing Failure",
    description: "Trade liquidity sweep reversals when BTC pierces a swing level and closes back inside.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Price sweeps prior 20-bar high/low", "Candle closes back inside swept level"],
    exitRules: ["Exit at opposite range side or failed reclaim"],
    stopLossRules: ["Stop beyond sweep wick"],
    takeProfitRules: ["Target range midpoint then opposite liquidity"],
    requiredIndicators: ["Swing high/low structure"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING", "LOW_VOLATILITY_CHOP"],
    worstRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    researchConfidenceScore: 88,
    minCandles: 30,
    params: { lookback: 20 },
    signalFactory: stopHuntSfp,
  },
  {
    family: "VolumeSpikeReversal",
    baseName: "Volume Spike Reversal",
    description: "Fade failed BTC pushes when abnormal volume forms a rejection wick at an extreme.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Volume spike above threshold", "Rejection wick/pin bar forms", "Close rejects extreme"],
    exitRules: ["Exit at VWAP/range midpoint"],
    stopLossRules: ["Stop beyond rejection wick"],
    takeProfitRules: ["Target 1.5R or range midpoint"],
    requiredIndicators: ["Volume ratio", "Candle wick structure"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["RANGING", "HIGH_VOLATILITY_BREAKOUT"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 78,
    minCandles: 30,
    params: { volumeRatio: 2 },
    signalFactory: volumeSpikeReversal,
  },
  {
    family: "AtrChannelBreakout",
    baseName: "ATR Donchian Breakout",
    description: "Trade Donchian channel breaks only when ATR confirms enough range to overcome costs.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["Close breaks prior N-bar channel", "ATR percent exceeds threshold", "Volume confirms participation"],
    exitRules: ["Exit on channel failure or ATR target hit"],
    stopLossRules: ["Stop back inside channel by 0.5 ATR"],
    takeProfitRules: ["Target 1.5-3 ATR"],
    requiredIndicators: ["ATR", "Donchian Channel", "Volume ratio"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT", "TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 86,
    minCandles: 40,
    params: { lookback: 20, minAtrPct: 0.001 },
    signalFactory: atrChannelBreakout,
  },
  {
    family: "InsideBarBreakout",
    baseName: "Inside Bar OCO Breakout",
    description: "Trade expansion from an inside-bar compression pattern with volume confirmation.",
    sourceDocument: CLAUDE_SOURCE,
    entryRules: ["Inside bar forms inside prior candle", "Next candle closes beyond inside-bar high/low", "Volume confirms"],
    exitRules: ["Exit on failed break back inside pattern"],
    stopLossRules: ["Stop at opposite side of inside bar"],
    takeProfitRules: ["Target pattern height expansion or 2R"],
    requiredIndicators: ["Inside bar pattern", "Volume ratio"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 70,
    minCandles: 30,
    params: { volumeRatio: 1.2 },
    signalFactory: insideBarBreakout,
  },
  {
    family: "OpeningRangeBreakout",
    baseName: "Opening Range Breakout",
    description: "Break the recent session opening range with volume confirmation.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Define 15-30 minute opening range", "Close breaks range high/low", "Volume expands"],
    exitRules: ["Exit if price re-enters opening range"],
    stopLossRules: ["Stop at midpoint/opposite side of opening range"],
    takeProfitRules: ["Target range height projection or 2R"],
    requiredIndicators: ["Opening range", "Volume ratio"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT", "TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 82,
    minCandles: 40,
    params: { rangeBars: 15, volumeRatio: 1.5 },
    signalFactory: openingRangeBreakout,
  },
  {
    family: "SessionOpenMomentum",
    baseName: "Session Open Momentum",
    description: "Capture early session directional participation after multiple strong same-color candles.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Recent candles align directionally", "Average body exceeds ATR fraction", "Volume expands"],
    exitRules: ["Exit on momentum stall or opposite candle sequence"],
    stopLossRules: ["Stop below/above momentum sequence"],
    takeProfitRules: ["Target 1.5R or VWAP extension"],
    requiredIndicators: ["ATR", "Volume ratio", "Candle body sequence"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["TRENDING", "HIGH_VOLATILITY_BREAKOUT"],
    worstRegime: ["RANGING"],
    researchConfidenceScore: 74,
    minCandles: 30,
    params: { volumeRatio: 1.3 },
    signalFactory: sessionOpenMomentum,
  },
  {
    family: "VwapPullback",
    baseName: "VWAP Reclaim",
    description: "Enter when BTC loses VWAP intrabar but closes back through it in the direction of the higher-slope trend.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Price trades through VWAP", "Close reclaims/rejects VWAP in trend direction", "ADX confirms participation"],
    exitRules: ["Exit on VWAP failure or opposite MACD/VWAP signal"],
    stopLossRules: ["Stop beyond reclaim wick"],
    takeProfitRules: ["Target prior liquidity or 2R"],
    requiredIndicators: ["VWAP", "ADX", "EMA slope"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 82,
    minCandles: 70,
    params: { ema: 34, toleranceBps: 25 },
    signalFactory: vwapPullback,
  },
  {
    family: "VwapMeanReversion",
    baseName: "VWAP Deviation Reversion",
    description: "Fade statistically large VWAP deviations when RSI and low ADX confirm a mean-reversion regime.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["Price stretches beyond VWAP deviation band", "RSI confirms exhaustion", "ADX below trend threshold"],
    exitRules: ["Exit at VWAP or when RSI normalizes"],
    stopLossRules: ["Stop if deviation expands another half band"],
    takeProfitRules: ["Primary target is VWAP"],
    requiredIndicators: ["VWAP", "RSI", "ADX", "Standard deviation"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["RANGING"],
    worstRegime: ["TRENDING"],
    researchConfidenceScore: 84,
    minCandles: 60,
    params: { rsi: 14, deviation: 1.75, longRsi: 25, shortRsi: 75 },
    signalFactory: vwapMeanReversion,
  },
  {
    family: "AtrChannelBreakout",
    baseName: "Donchian Breakout",
    description: "Trade BTC continuation when price closes beyond a prior Donchian channel with volume and ATR participation.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Close breaks prior channel high/low", "ATR percent is sufficient", "Volume ratio confirms participation"],
    exitRules: ["Exit on channel re-entry or ATR target"],
    stopLossRules: ["Stop back inside channel"],
    takeProfitRules: ["Target channel height projection or 2 ATR"],
    requiredIndicators: ["Donchian Channel", "ATR", "Volume ratio"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT", "TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 82,
    minCandles: 45,
    params: { lookback: 30, minAtrPct: 0.0015 },
    signalFactory: atrChannelBreakout,
  },
  {
    family: "AtrChannelBreakout",
    baseName: "Range Expansion Breakout",
    description: "Trade expansion after BTC breaks a short range with ATR and volume confirming the move can overcome costs.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["Close breaks short lookback range", "ATR percent expands", "Volume ratio is above baseline"],
    exitRules: ["Exit on failed expansion or ATR objective"],
    stopLossRules: ["Stop inside the broken range"],
    takeProfitRules: ["Target 1.5-2.5 ATR"],
    requiredIndicators: ["ATR", "Donchian Channel", "Volume ratio"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 78,
    minCandles: 35,
    params: { lookback: 12, minAtrPct: 0.001 },
    signalFactory: atrChannelBreakout,
  },
  {
    family: "StopHuntSfp",
    baseName: "Equal High Sweep",
    description: "Fade a stop run above equal highs when BTC closes back below the swept liquidity level.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Price sweeps prior equal/swing highs", "Close returns inside range"],
    exitRules: ["Exit at range midpoint or opposing liquidity"],
    stopLossRules: ["Stop above sweep wick"],
    takeProfitRules: ["Target midpoint then opposite sweep zone"],
    requiredIndicators: ["Swing high/low structure"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING", "LOW_VOLATILITY_CHOP"],
    worstRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    researchConfidenceScore: 86,
    minCandles: 30,
    params: { lookback: 20 },
    signalFactory: stopHuntSfp,
  },
  {
    family: "StopHuntSfp",
    baseName: "Equal Low Sweep",
    description: "Fade a stop run below equal lows when BTC reclaims the swept liquidity level.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Price sweeps prior equal/swing lows", "Close reclaims level"],
    exitRules: ["Exit at range midpoint or opposing liquidity"],
    stopLossRules: ["Stop below sweep wick"],
    takeProfitRules: ["Target midpoint then opposite sweep zone"],
    requiredIndicators: ["Swing high/low structure"],
    requiredData: ["OHLCV"],
    bestRegime: ["RANGING", "LOW_VOLATILITY_CHOP"],
    worstRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    researchConfidenceScore: 86,
    minCandles: 30,
    params: { lookback: 20 },
    signalFactory: stopHuntSfp,
  },
  {
    family: "VolumeSpikeReversal",
    baseName: "Volume Spike Breakout",
    description: "Trade volume-confirmed breakout participation when BTC closes beyond a prior range on abnormal volume.",
    sourceDocument: INTRADAY_SOURCE,
    entryRules: ["Close breaks range", "Volume spike confirms participation", "Candle body closes near extreme"],
    exitRules: ["Exit on failed breakout or VWAP loss"],
    stopLossRules: ["Stop back inside breakout candle"],
    takeProfitRules: ["Target 2R or prior session liquidity"],
    requiredIndicators: ["Volume ratio", "Donchian Channel", "ATR"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT", "TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 78,
    minCandles: 35,
    params: { lookback: 20, minAtrPct: 0.001 },
    signalFactory: atrChannelBreakout,
  },
  {
    family: "MacdVwapMomentum",
    baseName: "VWAP Trend Continuation",
    description: "Join BTC continuation when price holds VWAP and MACD confirms renewed momentum.",
    sourceDocument: ALGO_SOURCE,
    entryRules: ["Price holds correct side of VWAP", "MACD crosses signal in trend direction"],
    exitRules: ["Exit on VWAP loss or opposite MACD cross"],
    stopLossRules: ["Stop beyond VWAP support/resistance"],
    takeProfitRules: ["Target recent swing extension or 2R"],
    requiredIndicators: ["VWAP", "MACD"],
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: ["TRENDING"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 80,
    minCandles: 60,
    params: {},
    signalFactory: macdVwapMomentum,
  },
];

const PARAM_VARIANTS: Partial<Record<BtcResearchFamily, Record<string, number | string>[]>> = {};

function expandDefinition(definition: StrategyDefinition, firstId: number): BtcResearchStrategy[] {
  const variants = PARAM_VARIANTS[definition.family] ?? [definition.params];
  const strategies: BtcResearchStrategy[] = [];

  variants.forEach((params, variantIndex) => {
    (["BUY", "SELL"] as const).forEach((side, sideIndex) => {
      const sideText = side === "BUY" ? "Long" : "Short";
      const variantSuffix = variants.length > 1 ? ` v${variantIndex + 1}` : "";
      strategies.push({
        id: firstId + variantIndex * 2 + sideIndex,
        name: `${definition.baseName}${variantSuffix} ${sideText}`,
        family: definition.family,
        enabled: true,
        description: definition.description,
        params,
        timeframe: definition.timeframe ?? "1m",
        minCandles: definition.minCandles,
        signal: definition.signalFactory(side, params),
        entryRules: definition.entryRules,
        exitRules: definition.exitRules,
        stopLossRules: definition.stopLossRules,
        takeProfitRules: definition.takeProfitRules,
        requiredIndicators: definition.requiredIndicators,
        requiredData: definition.requiredData,
        bestRegime: definition.bestRegime,
        worstRegime: definition.worstRegime,
        researchConfidenceScore: definition.researchConfidenceScore,
        sourceDocument: definition.sourceDocument,
        dataFeedRequired: false,
        side: side === "BUY" ? "LONG" : "SHORT",
      });
    });
  });

  return strategies;
}

function buildActiveStrategies(): BtcResearchStrategy[] {
  const strategies: BtcResearchStrategy[] = [];
  let nextId = 2000;

  for (const definition of DEFINITIONS) {
    const expanded = expandDefinition(definition, nextId);
    for (const strategy of expanded) {
      if (nextId > 2059) return strategies;
      strategies.push({ ...strategy, id: nextId });
      nextId++;
    }
  }

  return strategies;
}

const ACTIVE_BTC_RESEARCH_STRATEGIES = buildActiveStrategies();

const STUB_FAMILIES: BtcResearchFamily[] = [
  "CvdDivergenceStub",
  "OiOvercrowdingStub",
  "LiquidationCascadeStub",
];

function stubStrategy(id: number): BtcResearchStrategy {
  const family = STUB_FAMILIES[(id - 2060) % STUB_FAMILIES.length];
  const sourceDocument =
    family === "CvdDivergenceStub" ? CLAUDE_SOURCE : family === "OiOvercrowdingStub" ? ALGO_SOURCE : INTRADAY_SOURCE;
  const data: BtcRequiredData[] =
    family === "CvdDivergenceStub"
      ? ["OHLCV", "ORDER_BOOK"]
      : family === "OiOvercrowdingStub"
        ? ["OHLCV", "OI", "FUNDING"]
        : ["OHLCV", "LIQUIDATIONS"];

  return {
    id,
    name: `${BTC_RESEARCH_FAMILY_LABELS[family]} #${id}`,
    family,
    enabled: true,
    description: "Research-documented advanced strategy registered for provenance, but inactive until required external market microstructure data is available.",
    params: { dataStatus: "pending" },
    timeframe: "1m",
    minCandles: 1,
    signal: () => NO_SIGNAL,
    entryRules: ["Requires external data feed not present in the mock candle builder"],
    exitRules: ["No live signal generation until required data exists"],
    stopLossRules: ["Pending external data integration"],
    takeProfitRules: ["Pending external data integration"],
    requiredIndicators: [BTC_RESEARCH_FAMILY_LABELS[family]],
    requiredData: data,
    bestRegime: ["HIGH_VOLATILITY_BREAKOUT"],
    worstRegime: ["LOW_VOLATILITY_CHOP"],
    researchConfidenceScore: 50,
    sourceDocument,
    dataFeedRequired: true,
    side: "BOTH",
  };
}

export const BTC_RESEARCH_STRATEGIES: BtcResearchStrategy[] = [
  ...ACTIVE_BTC_RESEARCH_STRATEGIES,
  ...Array.from({ length: 40 }, (_, index) => stubStrategy(2060 + index)),
];

const _ids = BTC_RESEARCH_STRATEGIES.map((strategy) => strategy.id);
if (new Set(_ids).size !== _ids.length) throw new Error("BTC strategy ID collision");
if (_ids.some((id) => id < 2000 || id > 2099)) throw new Error("BTC strategy ID out of range");
