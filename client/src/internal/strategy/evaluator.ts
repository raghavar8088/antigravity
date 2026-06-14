import type { FuturesStratDef } from "@/lib/trading/futuresStratTypes";

export type SignalDirection = "BUY" | "SELL";

export interface Signal {
  Symbol: string;
  Direction: SignalDirection;
  Confidence: number;
  Entry: number;
  StopLoss: number;
  TakeProfit: number;
  Timestamp: number;
  StrategyID: number;
  strategyName: string;
  category?: string;
  rawScore: number;
  reason: string;
}

export interface MarketSnapshot {
  symbol: string;
  timestamp: number;
  price: number;
  markPrice: number;
  prevPrice: number;
  fast: number;
  slow: number;
  trend: number;
  prevFast: number;
  prevSlow: number;
  mean20: number;
  std20: number;
  rsi14: number;
  high20: number;
  low20: number;
  momentum3: number;
  momentum6: number;
  volRatio: number;
  bbUpper: number;
  bbLower: number;
  bbWidth: number;
  stochK: number;
  stochD: number;
  prevStochK: number;
  prevStochD: number;
  macdLine: number;
  macdSignal: number;
  prevMacdLine: number;
  prevMacdSignal: number;
  atr14: number;
  atr14Avg30: number;
  volZ30: number;
  obvSlope: number;
  momentum10: number;
  rsi7: number;
  williamsR: number;
  prevWilliamsR: number;
  cci20: number;
  roc10: number;
  keltnerUpper: number;
  keltnerLower: number;
  donchianHigh: number;
  donchianLow: number;
  donchianMid: number;
  vwapDev: number;
  adxProxy: number;
  ema5: number;
  ema13: number;
  prevEma5: number;
  prevEma13: number;
  rsi21: number;
  macdHist: number;
  prevMacdHist: number;
  htf5_fast: number;
  htf5_slow: number;
  htf5_rsi: number;
  htf5_momentum: number;
  htf5_trend: number;
  htf15_fast: number;
  htf15_slow: number;
  htf15_rsi: number;
  htf15_momentum: number;
  htf15_trend: number;
  htf5_macdHist: number;
  htf15_macdHist: number;
  htf5_bbWidth: number;
  htf15_bbWidth: number;
  htf5_adx: number;
  htf15_adx: number;
}

export interface StrategyEvaluationResult {
  signals: Signal[];
  evaluated: number;
  rejected: number;
  latencyMs: number;
}

function clamp(n: number, min = 0, max = 100): number {
  return Math.min(max, Math.max(min, Number.isFinite(n) ? n : 0));
}

function signalDirection(strategy: FuturesStratDef): SignalDirection {
  const key = `${strategy.signalKey} ${strategy.name}`.toUpperCase();
  return key.includes("SHORT") || key.includes("SELL") || key.includes("BEAR") ? "SELL" : "BUY";
}

function rawSignalScore(market: MarketSnapshot, direction: SignalDirection): number {
  const trendScore = direction === "BUY"
    ? market.fast - market.slow + market.momentum3
    : market.slow - market.fast - market.momentum3;
  return clamp(50 + trendScore * 8 + market.adxProxy + Math.max(0, market.volRatio - 1) * 10);
}

export class StrategyEvaluationEngine {
  async evaluate(
    market: MarketSnapshot,
    strategies: readonly FuturesStratDef[],
  ): Promise<StrategyEvaluationResult> {
    const started = performance.now();
    const entry = market.markPrice > 0 ? market.markPrice : market.price;
    const signals: Signal[] = [];
    let rejected = 0;

    for (const strategy of strategies) {
      const direction = signalDirection(strategy);
      const rawScore = rawSignalScore(market, direction);
      if (rawScore < 50) {
        rejected += 1;
        continue;
      }

      const slPct = Math.max(0, strategy.slPct ?? 0.5) / 100;
      const tpPct = Math.max(0, strategy.tpPct ?? 1) / 100;
      signals.push({
        Symbol: market.symbol,
        Direction: direction,
        Confidence: Math.round(rawScore),
        Entry: entry,
        StopLoss: direction === "BUY" ? entry * (1 - slPct) : entry * (1 + slPct),
        TakeProfit: direction === "BUY" ? entry * (1 + tpPct) : entry * (1 - tpPct),
        Timestamp: market.timestamp,
        StrategyID: strategy.id,
        strategyName: strategy.name,
        category: strategy.category,
        rawScore,
        reason: `deterministic score ${rawScore.toFixed(1)}`,
      });
    }

    return {
      signals,
      evaluated: strategies.length,
      rejected,
      latencyMs: performance.now() - started,
    };
  }
}
