import { describe, expect, it } from "vitest";
import { EventBus } from "@/internal/events";
import { ExecutionEngineV2 } from "@/internal/execution/engine_v2";
import { ExecutionPipelineV2 } from "@/internal/execution/pipeline_v2";
import { PaperExchangeAdapter } from "@/internal/exchange";
import { OMSV2 } from "@/internal/oms";
import { StrategyEvaluationEngine, type MarketSnapshot, type Signal } from "@/internal/strategy/evaluator";
import type { FuturesStratDef } from "@/lib/futuresStratTypes";

const market: MarketSnapshot = {
  symbol: "BTCUSD",
  timestamp: 1,
  price: 100,
  markPrice: 100,
  prevPrice: 99,
  fast: 101,
  slow: 99,
  trend: 1,
  prevFast: 100,
  prevSlow: 99,
  mean20: 99,
  std20: 1,
  rsi14: 55,
  high20: 101,
  low20: 97,
  momentum3: 1,
  momentum6: 2,
  volRatio: 1.5,
  bbUpper: 102,
  bbLower: 96,
  bbWidth: 0.02,
  stochK: 60,
  stochD: 55,
  prevStochK: 50,
  prevStochD: 50,
  macdLine: 1,
  macdSignal: 0.5,
  prevMacdLine: 0.5,
  prevMacdSignal: 0.4,
  atr14: 1,
  atr14Avg30: 1,
  volZ30: 1,
  obvSlope: 1,
  momentum10: 2,
  rsi7: 58,
  williamsR: -40,
  prevWilliamsR: -45,
  cci20: 80,
  roc10: 0.5,
  keltnerUpper: 102,
  keltnerLower: 96,
  donchianHigh: 101,
  donchianLow: 97,
  donchianMid: 99,
  vwapDev: 0.2,
  adxProxy: 28,
  ema5: 101,
  ema13: 99,
  prevEma5: 100,
  prevEma13: 99,
  rsi21: 54,
  macdHist: 0.5,
  prevMacdHist: 0.4,
  htf5_fast: 101,
  htf5_slow: 99,
  htf5_rsi: 55,
  htf5_momentum: 1,
  htf5_trend: 1,
  htf15_fast: 101,
  htf15_slow: 99,
  htf15_rsi: 55,
  htf15_momentum: 1,
  htf15_trend: 1,
  htf5_macdHist: 1,
  htf15_macdHist: 1,
  htf5_bbWidth: 0.02,
  htf15_bbWidth: 0.02,
  htf5_adx: 28,
  htf15_adx: 28,
};

const signal: Signal = {
  Symbol: "BTCUSD",
  Direction: "BUY",
  Confidence: 92,
  Entry: 100,
  StopLoss: 98,
  TakeProfit: 104,
  Timestamp: 1,
  StrategyID: 91,
  strategyName: "Trend_Continuation_Long",
  category: "Trend",
  rawScore: 92,
  reason: "test",
};

class StaticEvaluator extends StrategyEvaluationEngine {
  override async evaluate(): Promise<{ signals: Signal[]; evaluated: number; rejected: number; latencyMs: number }> {
    return { signals: [signal], evaluated: 1, rejected: 0, latencyMs: 0 };
  }
}

describe("ExecutionPipelineV2", () => {
  it("routes deterministic signals through OMS and paper execution without AI", async () => {
    const eventBus = new EventBus();
    const oms = new OMSV2();
    const executionEngine = new ExecutionEngineV2({
      exchange: new PaperExchangeAdapter(() => 100),
      oms,
      eventBus,
    });
    const pipeline = new ExecutionPipelineV2({
      eventBus,
      evaluator: new StaticEvaluator(),
      oms,
      executionEngine,
    });
    const strategies: FuturesStratDef[] = [{
      id: 91,
      name: "Trend_Continuation_Long",
      category: "Trend",
      signalKey: "TREND_CONT_LONG",
      slPct: 0.5,
      tpPct: 1.5,
      cooldownMin: 1,
      holdMinutes: 10,
      confluenceMin: 1,
    }];

    const result = await pipeline.run({
      market,
      strategies,
      notionalUsd: 100,
      riskLimits: {
        maxOrdersPerTick: 1,
        maxNotionalPerOrder: 200,
        maxTotalNotional: 200,
        minSignalScore: 50,
      },
    });

    expect(result.ordersSubmitted).toBe(1);
    expect(oms.listOrders()[0]!.state).toBe("FILLED");
  });
});
