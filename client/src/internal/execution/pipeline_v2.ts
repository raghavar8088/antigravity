import { EventBus } from "@/internal/events";
import { ExecutionEngineV2 } from "@/internal/execution/engine_v2";
import { MarketRegimeEngine } from "@/internal/regime";
import { RegimeStrategyRouter } from "@/internal/regime/router";
import { OMSV2 } from "@/internal/oms";
import { PositionManagerV2 } from "@/internal/positions";
import { StrategyEvaluationEngine, type MarketSnapshot } from "@/internal/strategy/evaluator";
import { SignalAggregatorV2 } from "@/internal/trading/aggregator_v2";
import {
  SignalQualityEngine,
  type SignalQualityFeatures,
  type SignalQualityScore,
} from "@/internal/trading/signal_scoring";
import type { FuturesStratDef } from "@/lib/trading/futuresStratTypes";

export interface PipelineRiskLimits {
  maxOrdersPerTick: number;
  maxNotionalPerOrder: number;
  maxTotalNotional: number;
  minSignalScore: number;
}

export interface PipelineRunInput {
  market: MarketSnapshot;
  strategies: readonly FuturesStratDef[];
  notionalUsd: number;
  riskLimits: PipelineRiskLimits;
  strategyPerformance?: ReadonlyMap<number, number>;
}

export interface PipelineRunResult {
  evaluatedStrategies: number;
  rawSignals: number;
  approvedSignals: number;
  rejectedSignals: number;
  ordersSubmitted: number;
  latencyMs: number;
}

const DEFAULT_FEATURES: Omit<SignalQualityFeatures, "strategyHistoricalPerformance" | "regimeAligned"> = {
  trendStrength: 50,
  volumeConfirmation: 50,
  volatilityContext: 50,
  marketStructure: 50,
};

function featureSet(
  market: MarketSnapshot,
  strategyId: number,
  strategyPerformance: ReadonlyMap<number, number> | undefined,
  regimeAligned: boolean,
): SignalQualityFeatures {
  return {
    ...DEFAULT_FEATURES,
    trendStrength: Math.min(100, Math.max(0, market.adxProxy * 3)),
    volumeConfirmation: Math.min(100, Math.max(0, market.volRatio * 35)),
    volatilityContext: Math.min(100, Math.max(0, market.atr14 > 0 ? 70 : 20)),
    marketStructure: Math.min(100, Math.max(0, Math.abs(market.vwapDev) > 0 ? 60 : 35)),
    strategyHistoricalPerformance: strategyPerformance?.get(strategyId) ?? 50,
    regimeAligned,
  };
}

export class ExecutionPipelineV2 {
  private readonly eventBus: EventBus;
  private readonly regimeEngine = new MarketRegimeEngine();
  private readonly regimeRouter = new RegimeStrategyRouter();
  private readonly evaluator: StrategyEvaluationEngine;
  private readonly scorer = new SignalQualityEngine();
  private readonly aggregator = new SignalAggregatorV2();
  private readonly oms: OMSV2;
  private readonly executionEngine: ExecutionEngineV2;
  private readonly positions: PositionManagerV2;

  constructor(opts: {
    eventBus?: EventBus;
    evaluator?: StrategyEvaluationEngine;
    oms: OMSV2;
    executionEngine: ExecutionEngineV2;
    positions?: PositionManagerV2;
  }) {
    this.eventBus = opts.eventBus ?? new EventBus();
    this.evaluator = opts.evaluator ?? new StrategyEvaluationEngine();
    this.oms = opts.oms;
    this.executionEngine = opts.executionEngine;
    this.positions = opts.positions ?? new PositionManagerV2();
  }

  async run(input: PipelineRunInput): Promise<PipelineRunResult> {
    const started = performance.now();
    const regime = this.regimeEngine.detect(input.market);
    const routedStrategies = this.regimeRouter.route(input.strategies, regime);
    const evaluated = await this.evaluator.evaluate(input.market, routedStrategies);

    const scored: SignalQualityScore[] = evaluated.signals.map((signal) => {
      this.eventBus.publish("SignalCreated", { signal }, { correlationId: `${signal.Symbol}:${signal.StrategyID}:${signal.Timestamp}` });
      return this.scorer.score(
        signal,
        featureSet(input.market, signal.StrategyID, input.strategyPerformance, true),
      );
    });

    const aggregated = this.aggregator.aggregate(scored);
    let totalNotional = 0;
    let ordersSubmitted = 0;
    let rejectedSignals = evaluated.rejected;

    for (const candidate of aggregated.candidates) {
      if (ordersSubmitted >= input.riskLimits.maxOrdersPerTick) break;
      if (candidate.quality.SignalScore < input.riskLimits.minSignalScore) {
        rejectedSignals += 1;
        this.eventBus.publish("SignalRejected", { candidate, reason: "quality score below risk minimum" });
        continue;
      }
      if (input.notionalUsd > input.riskLimits.maxNotionalPerOrder) {
        rejectedSignals += 1;
        this.eventBus.publish("RiskViolation", { candidate, reason: "notional exceeds per-order risk limit" });
        continue;
      }
      if (totalNotional + input.notionalUsd > input.riskLimits.maxTotalNotional) {
        rejectedSignals += 1;
        this.eventBus.publish("RiskViolation", { candidate, reason: "notional exceeds total risk limit" });
        continue;
      }

      const quantity = input.notionalUsd / Math.max(candidate.signal.Entry, 1);
      const order = this.oms.createOrder({
        symbol: candidate.signal.Symbol,
        side: candidate.signal.Direction,
        type: "MARKET",
        mode: "PAPER",
        quantity,
        signal: candidate.signal,
      });
      this.executionEngine.getLatencyTracker().mark(order.orderId, "SignalApproved");
      const result = await this.executionEngine.sendOrder(order);
      ordersSubmitted += 1;
      totalNotional += input.notionalUsd;

      if (result.order.state === "FILLED") {
        const position = this.positions.openFromOrder(result.order);
        this.eventBus.publish("PositionOpened", { position, order: result.order }, { correlationId: order.orderId });
      }
    }

    return {
      evaluatedStrategies: evaluated.evaluated,
      rawSignals: evaluated.signals.length,
      approvedSignals: ordersSubmitted,
      rejectedSignals,
      ordersSubmitted,
      latencyMs: performance.now() - started,
    };
  }
}
