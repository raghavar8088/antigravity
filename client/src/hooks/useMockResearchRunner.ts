"use client";

/**
 * useMockResearchRunner — evaluates all 500 mock research strategies on every
 * new 1-minute candle and feeds BUY/SELL signals into the mock trading engine
 * via ingestResearchSignals().
 *
 * ISOLATION: Never calls real broker, OMS, Delta, or Angel One APIs. All
 * signals are mock-only and route exclusively to the mock trading engine.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import {
  ALL_BTC_RESEARCH_FAMILIES,
  BTC_RESEARCH_FAMILY_LABELS,
  BTC_RESEARCH_STRATEGIES,
  type BtcResearchFamily,
  type BtcResearchStrategy,
} from "@/lib/btcResearchStrategyRegistry";
import {
  ALL_RESEARCH_FAMILIES,
  RESEARCH_FAMILY_LABELS,
  RESEARCH_STRATEGIES,
  type ResearchFamily,
  type ResearchStrategy,
} from "@/lib/mockResearchStrategies";
import type { OHLCVCandle } from "@/lib/mockResearchIndicators";
import type { MockResearchSignalInput } from "@/lib/mockTradingEngine";
import type { MarketRegime } from "@/lib/marketRegimeClassifier";

const MAX_SIGNALS_PER_MINUTE_DEFAULT = 10;
const MIN_CONFIDENCE_DEFAULT = 50;

export type ResearchSelectionMode = "RESEARCH_MODE" | "PROFIT_MODE" | "REGIME_MODE";
type AnyResearchFamily = ResearchFamily | BtcResearchFamily;
type AnyResearchStrategy = ResearchStrategy | BtcResearchStrategy;

export interface ResearchRunnerConfig {
  /** Master enable/disable for all 500 research strategies. */
  enabled: boolean;
  /** Set of enabled family names. Empty means no families are enabled. */
  enabledFamilies: Set<ResearchFamily>;
  /** Hard cap on signals forwarded per evaluation cycle. */
  maxSignalsPerMinute: number;
  /** Minimum confidence score (0–100) required to emit a signal. */
  minConfidence: number;
  /** Research accepts all; Profit/Regime modes are restricted to research-backed strategy IDs. */
  selectionMode: ResearchSelectionMode;
  /** Number of ranked strategies allowed in Profit/Regime mode. */
  topN: number;
  /** Ranked strategy IDs approved by the scoring hook for Profit/Regime mode. */
  approvedStrategyIds: ReadonlySet<number>;
  /** Enabled BTC research-backed families. Empty means no BTC families are enabled. */
  enabledBtcFamilies: Set<BtcResearchFamily>;
}

export const DEFAULT_RESEARCH_RUNNER_CONFIG: ResearchRunnerConfig = {
  enabled: true,
  enabledFamilies: new Set(ALL_RESEARCH_FAMILIES),
  maxSignalsPerMinute: MAX_SIGNALS_PER_MINUTE_DEFAULT,
  minConfidence: MIN_CONFIDENCE_DEFAULT,
  selectionMode: "RESEARCH_MODE",
  topN: 10,
  approvedStrategyIds: new Set<number>(),
  enabledBtcFamilies: new Set(ALL_BTC_RESEARCH_FAMILIES),
};

export interface ResearchSignalSummary extends MockResearchSignalInput {
  strategyId: number;
  strategyName: string;
  family: AnyResearchFamily;
  strategyFamily: AnyResearchFamily;
  side: "BUY" | "SELL";
  confidence: number;
  confidenceScore: number;
  params: Record<string, number | string>;
  evaluatedAt: number;
}

export interface UseResearchRunnerResult {
  config: ResearchRunnerConfig;
  setConfig: (next: ResearchRunnerConfig) => void;
  /** Strategies evaluated in the last cycle. */
  lastEvalCount: number;
  /** Signals emitted in the last cycle. */
  lastSignalCount: number;
  /** Whether any evaluation has run. */
  hasRun: boolean;
  /** Epoch ms of the last evaluation cycle. */
  lastEvalAt: number | null;
  /** Most recent signals (ring buffer, newest first, max 200). */
  recentSignals: ResearchSignalSummary[];
  /** List of all 500 strategy definitions (for UI display). */
  strategies: readonly AnyResearchStrategy[];
  /** Family display labels. */
  familyLabels: Record<string, string>;
}

const SIGNAL_RING_CAP = 200;

export interface ResearchEvaluationResult {
  evaluatedCount: number;
  signals: ResearchSignalSummary[];
}

export function evaluateMockResearchStrategies(
  candles: readonly OHLCVCandle[],
  config: ResearchRunnerConfig,
  currentRegimeOrNow: MarketRegime | null | number = null,
  nowArg?: number,
): ResearchEvaluationResult {
  if (!config.enabled || candles.length < 5) return { evaluatedCount: 0, signals: [] };
  const currentRegime = typeof currentRegimeOrNow === "number" ? null : currentRegimeOrNow;
  const now = typeof currentRegimeOrNow === "number" ? currentRegimeOrNow : (nowArg ?? Date.now());

  const isFamilyEnabled = (fam: ResearchFamily): boolean => {
    return config.enabledFamilies.has(fam);
  };
  const isBtcFamilyEnabled = (fam: BtcResearchFamily): boolean => {
    return config.enabledBtcFamilies.has(fam);
  };
  const passesSelectionMode = (strategyId: number): boolean => {
    if (config.selectionMode === "RESEARCH_MODE") return true;
    if (strategyId < 2000 || strategyId > 2059) return false;
    if (config.approvedStrategyIds.size === 0) return false;
    return config.approvedStrategyIds.has(strategyId);
  };

  const snap = candles.slice();
  const signals: ResearchSignalSummary[] = [];
  let evaluatedCount = 0;

  for (const strat of RESEARCH_STRATEGIES) {
    if (!strat.enabled) continue;
    if (!isFamilyEnabled(strat.family)) continue;
    if (snap.length < strat.minCandles) continue;
    evaluatedCount++;
    try {
      const result = strat.signal(snap);
      if (result.side === "NO_SIGNAL") continue;
      if (result.confidence < config.minConfidence) continue;
      if (!passesSelectionMode(strat.id)) continue;
      signals.push({
        strategyId: strat.id,
        strategyName: strat.name,
        family: strat.family,
        strategyFamily: strat.family,
        side: result.side,
        confidence: result.confidence,
        confidenceScore: result.confidence,
        params: strat.params,
        evaluatedAt: now,
        regimeAtEntry: currentRegime ?? undefined,
      });
    } catch {
      // Suppress individual strategy errors so one bad signal doesn't block others.
    }
  }

  for (const strat of BTC_RESEARCH_STRATEGIES) {
    if (!strat.enabled) continue;
    if (strat.dataFeedRequired) continue;
    if (!isBtcFamilyEnabled(strat.family)) continue;
    if (snap.length < strat.minCandles) continue;
    evaluatedCount++;
    try {
      const result = strat.signal(snap);
      if (result.side === "NO_SIGNAL") continue;
      if (result.confidence < config.minConfidence) continue;
      if (!passesSelectionMode(strat.id)) continue;
      signals.push({
        strategyId: strat.id,
        strategyName: strat.name,
        family: strat.family,
        strategyFamily: strat.family,
        side: result.side,
        confidence: result.confidence,
        confidenceScore: result.confidence,
        params: strat.params,
        evaluatedAt: now,
        regimeAtEntry: currentRegime ?? undefined,
      });
    } catch {
      // Suppress individual strategy errors so one bad signal doesn't block others.
    }
  }

  const capped =
    signals.length <= config.maxSignalsPerMinute
      ? signals
      : [...signals].sort((a, b) => b.confidence - a.confidence).slice(0, config.maxSignalsPerMinute);

  return { evaluatedCount, signals: capped };
}

export interface ResearchRunnerDeps {
  /** Closed candle snapshot from useMockCandleBuilder. */
  candles: OHLCVCandle[];
  /** True when a new 1-min candle just closed (pulse). */
  newCandleReady: boolean;
  /** Inject signals into the mock trading engine. */
  ingestResearchSignals: (signals: ResearchSignalSummary[], price: number) => void;
  /** Live BTC price — used as entry price for mock trades. */
  price: number;
  /** Current live regime snapshot label, recorded onto mock trades. */
  currentRegime?: MarketRegime | null;
}

export function useMockResearchRunner(deps: ResearchRunnerDeps): UseResearchRunnerResult {
  const { candles, newCandleReady, ingestResearchSignals, price, currentRegime = null } = deps;

  const [config, setConfigState] = useState<ResearchRunnerConfig>(DEFAULT_RESEARCH_RUNNER_CONFIG);
  const [lastEvalCount, setLastEvalCount] = useState(0);
  const [lastSignalCount, setLastSignalCount] = useState(0);
  const [hasRun, setHasRun] = useState(false);
  const [lastEvalAt, setLastEvalAt] = useState<number | null>(null);
  const [recentSignals, setRecentSignals] = useState<ResearchSignalSummary[]>([]);

  const configRef = useRef(config);
  const candlesRef = useRef(candles);
  const priceRef = useRef(price);
  const regimeRef = useRef<MarketRegime | null>(currentRegime);

  useEffect(() => { configRef.current = config; }, [config]);
  useEffect(() => { candlesRef.current = candles; }, [candles]);
  useEffect(() => { priceRef.current = price; }, [price]);
  useEffect(() => { regimeRef.current = currentRegime; }, [currentRegime]);

  const runEvaluation = useCallback(() => {
    const cfg = configRef.current;
    if (!cfg.enabled) return;
    const snap = candlesRef.current;
    if (snap.length < 5) return; // need minimum history
    const livePrice = priceRef.current;
    if (!Number.isFinite(livePrice) || livePrice <= 0) return;

    const now = Date.now();
    const { evaluatedCount, signals: capped } = evaluateMockResearchStrategies(snap, cfg, regimeRef.current, now);

    setLastEvalCount(evaluatedCount);
    setLastSignalCount(capped.length);
    setHasRun(true);
    setLastEvalAt(now);

    if (capped.length > 0) {
      setRecentSignals((prev) => {
        const combined = [...capped, ...prev];
        return combined.length > SIGNAL_RING_CAP ? combined.slice(0, SIGNAL_RING_CAP) : combined;
      });
      ingestResearchSignals(capped, livePrice);
      if (typeof fetch === "function") {
        void fetch("/api/mock-trading/signals", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            signals: capped.map((signal) => ({
              strategy_id: signal.strategyId,
              strategy_name: signal.strategyName,
              strategy_family: signal.strategyFamily,
              symbol: "BTCUSD",
              side: signal.side,
              timeframe: "1m",
              entry_price: livePrice,
              confidence_score: signal.confidenceScore,
              market_regime: signal.regimeAtEntry,
              indicator_snapshot: {},
              entry_reason: `${signal.strategyName} emitted ${signal.side}`,
              timestamp: signal.evaluatedAt,
            })),
          }),
        }).catch(() => {
          // Signal persistence is best-effort; mock trade simulation continues locally.
        });
      }
    }
  }, [ingestResearchSignals]);

  // Trigger evaluation when a new candle is ready.
  useEffect(() => {
    if (!newCandleReady) return;
    // Use setTimeout(0) to defer evaluation off the render cycle, preventing
    // UI jank when evaluating all 500 strategies.
    const id = setTimeout(() => runEvaluation(), 0);
    return () => clearTimeout(id);
  }, [newCandleReady, runEvaluation]);

  const setConfig = useCallback((next: ResearchRunnerConfig) => setConfigState(next), []);

  return {
    config,
    setConfig,
    lastEvalCount,
    lastSignalCount,
    hasRun,
    lastEvalAt,
    recentSignals,
    strategies: [...RESEARCH_STRATEGIES, ...BTC_RESEARCH_STRATEGIES],
    familyLabels: { ...RESEARCH_FAMILY_LABELS, ...BTC_RESEARCH_FAMILY_LABELS },
  };
}
