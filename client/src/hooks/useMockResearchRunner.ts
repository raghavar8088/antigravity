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
  ALL_RESEARCH_FAMILIES,
  RESEARCH_FAMILY_LABELS,
  RESEARCH_STRATEGIES,
  type ResearchFamily,
  type ResearchStrategy,
} from "@/lib/mockResearchStrategies";
import type { OHLCVCandle } from "@/lib/mockResearchIndicators";
import type { MockResearchSignalInput } from "@/lib/mockTradingEngine";

const MAX_SIGNALS_PER_MINUTE_DEFAULT = 50;
const MIN_CONFIDENCE_DEFAULT = 30; // permissive by default per spec

export interface ResearchRunnerConfig {
  /** Master enable/disable for all 500 research strategies. */
  enabled: boolean;
  /** Set of enabled family names. Empty means no families are enabled. */
  enabledFamilies: Set<ResearchFamily>;
  /** Hard cap on signals forwarded per evaluation cycle. */
  maxSignalsPerMinute: number;
  /** Minimum confidence score (0–100) required to emit a signal. */
  minConfidence: number;
}

export const DEFAULT_RESEARCH_RUNNER_CONFIG: ResearchRunnerConfig = {
  enabled: true,
  enabledFamilies: new Set(ALL_RESEARCH_FAMILIES),
  maxSignalsPerMinute: MAX_SIGNALS_PER_MINUTE_DEFAULT,
  minConfidence: MIN_CONFIDENCE_DEFAULT,
};

export interface ResearchSignalSummary extends MockResearchSignalInput {
  strategyId: number;
  strategyName: string;
  family: ResearchFamily;
  strategyFamily: ResearchFamily;
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
  strategies: readonly ResearchStrategy[];
  /** Family display labels. */
  familyLabels: typeof RESEARCH_FAMILY_LABELS;
}

const SIGNAL_RING_CAP = 200;

export interface ResearchEvaluationResult {
  evaluatedCount: number;
  signals: ResearchSignalSummary[];
}

export function evaluateMockResearchStrategies(
  candles: readonly OHLCVCandle[],
  config: ResearchRunnerConfig,
  now = Date.now(),
): ResearchEvaluationResult {
  if (!config.enabled || candles.length < 5) return { evaluatedCount: 0, signals: [] };

  const isFamilyEnabled = (fam: ResearchFamily): boolean => {
    return config.enabledFamilies.has(fam);
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
}

export function useMockResearchRunner(deps: ResearchRunnerDeps): UseResearchRunnerResult {
  const { candles, newCandleReady, ingestResearchSignals, price } = deps;

  const [config, setConfigState] = useState<ResearchRunnerConfig>(DEFAULT_RESEARCH_RUNNER_CONFIG);
  const [lastEvalCount, setLastEvalCount] = useState(0);
  const [lastSignalCount, setLastSignalCount] = useState(0);
  const [hasRun, setHasRun] = useState(false);
  const [lastEvalAt, setLastEvalAt] = useState<number | null>(null);
  const [recentSignals, setRecentSignals] = useState<ResearchSignalSummary[]>([]);

  const configRef = useRef(config);
  const candlesRef = useRef(candles);
  const priceRef = useRef(price);

  useEffect(() => { configRef.current = config; }, [config]);
  useEffect(() => { candlesRef.current = candles; }, [candles]);
  useEffect(() => { priceRef.current = price; }, [price]);

  const runEvaluation = useCallback(() => {
    const cfg = configRef.current;
    if (!cfg.enabled) return;
    const snap = candlesRef.current;
    if (snap.length < 5) return; // need minimum history
    const livePrice = priceRef.current;
    if (!Number.isFinite(livePrice) || livePrice <= 0) return;

    const now = Date.now();
    const { evaluatedCount, signals: capped } = evaluateMockResearchStrategies(snap, cfg, now);

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
    strategies: RESEARCH_STRATEGIES,
    familyLabels: RESEARCH_FAMILY_LABELS,
  };
}
