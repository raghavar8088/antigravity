import {
  evalMinuteSignal,
  passesEntryConfirmation,
  type FuturesSignalInputs,
} from "@/lib/futuresSignals";
import type { FuturesStratDef } from "@/lib/futuresStratTypes";

export type SignalDirection = "BUY" | "SELL";

export interface MarketSnapshot extends FuturesSignalInputs {
  symbol: string;
  timestamp: number;
}

export interface Signal {
  Symbol: string;
  Direction: SignalDirection;
  Confidence: number;
  Entry: number;
  StopLoss: number;
  TakeProfit: number;
  Timestamp: number;
  StrategyID: number;
  strategyName?: string;
  category?: string;
  rawScore: number;
  reason: string;
}

export interface StrategyEvaluationEngineOptions {
  signalThreshold?: number;
  workerPoolSize?: number;
  batchSize?: number;
  requireConfirmation?: boolean;
}

export interface StrategyEvaluationReport {
  signals: Signal[];
  evaluated: number;
  rejected: number;
  latencyMs: number;
}

const DEFAULT_THRESHOLD = 28;
const DEFAULT_WORKERS = 4;
const DEFAULT_BATCH_SIZE = 64;

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n));
}

function directionFor(strat: FuturesStratDef): SignalDirection {
  return strat.signalKey.includes("SHORT") ? "SELL" : "BUY";
}

function buildSignal(snapshot: MarketSnapshot, strat: FuturesStratDef, rawScore: number, reason: string): Signal {
  const direction = directionFor(strat);
  const entry = snapshot.markPrice > 0 ? snapshot.markPrice : snapshot.price;
  const slPct = strat.slPct / 100;
  const tpPct = strat.tpPct / 100;
  const stopLoss = direction === "BUY" ? entry * (1 - slPct) : entry * (1 + slPct);
  const takeProfit = direction === "BUY" ? entry * (1 + tpPct) : entry * (1 - tpPct);

  return {
    Symbol: snapshot.symbol,
    Direction: direction,
    Confidence: clamp(rawScore, 0, 100),
    Entry: entry,
    StopLoss: stopLoss,
    TakeProfit: takeProfit,
    Timestamp: snapshot.timestamp,
    StrategyID: strat.id,
    strategyName: strat.name,
    category: strat.category,
    rawScore,
    reason,
  };
}

export class StrategyEvaluationEngine {
  private readonly threshold: number;
  private readonly workerPoolSize: number;
  private readonly batchSize: number;
  private readonly requireConfirmation: boolean;

  constructor(opts: StrategyEvaluationEngineOptions = {}) {
    this.threshold = opts.signalThreshold ?? DEFAULT_THRESHOLD;
    this.workerPoolSize = Math.max(1, Math.floor(opts.workerPoolSize ?? DEFAULT_WORKERS));
    this.batchSize = Math.max(1, Math.floor(opts.batchSize ?? DEFAULT_BATCH_SIZE));
    this.requireConfirmation = opts.requireConfirmation ?? true;
  }

  evaluateSync(snapshot: MarketSnapshot, strategies: readonly FuturesStratDef[]): StrategyEvaluationReport {
    const started = performance.now();
    const signals: Signal[] = [];
    let rejected = 0;

    for (const strat of strategies) {
      if (strat.researchOnly) {
        rejected += 1;
        continue;
      }

      const { score, reason } = evalMinuteSignal(snapshot, strat);
      const threshold = strat.dynamicThreshold ?? this.threshold;
      if (!Number.isFinite(score) || score < threshold) {
        rejected += 1;
        continue;
      }
      if (this.requireConfirmation && !passesEntryConfirmation(snapshot, strat)) {
        rejected += 1;
        continue;
      }
      signals.push(buildSignal(snapshot, strat, score, reason));
    }

    return {
      signals,
      evaluated: strategies.length,
      rejected,
      latencyMs: performance.now() - started,
    };
  }

  async evaluate(snapshot: MarketSnapshot, strategies: readonly FuturesStratDef[]): Promise<StrategyEvaluationReport> {
    const started = performance.now();
    const chunks: FuturesStratDef[][] = [];
    const stride = Math.max(this.batchSize, Math.ceil(strategies.length / this.workerPoolSize));

    for (let i = 0; i < strategies.length; i += stride) {
      chunks.push(strategies.slice(i, i + stride));
    }

    const reports = await Promise.all(chunks.map((chunk) => Promise.resolve(this.evaluateSync(snapshot, chunk))));
    const signals = reports.flatMap((r) => r.signals);
    const rejected = reports.reduce((sum, r) => sum + r.rejected, 0);

    return {
      signals,
      evaluated: strategies.length,
      rejected,
      latencyMs: performance.now() - started,
    };
  }
}
