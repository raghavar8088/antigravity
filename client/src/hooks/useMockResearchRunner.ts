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
} from "@/lib/trading/btcResearchStrategyRegistry";
import {
  INSTITUTIONAL_STRATEGIES,
  INSTITUTIONAL_FAMILY_LABELS,
  type InstitutionalStrategy,
} from "@/lib/trading/btcInstitutionalStrategies";
import {
  ALL_RESEARCH_FAMILIES,
  RESEARCH_FAMILY_LABELS,
  RESEARCH_STRATEGIES,
  type ResearchFamily,
  type ResearchStrategy,
} from "@/lib/ai/mockResearchStrategies";
import type { OHLCVCandle } from "@/lib/ai/mockResearchIndicators";
import {
  emptyMockDiagnosticFunnel,
  emptyMockRejectionCounts,
  type MockDiagnosticFunnel,
  type MockRejectionCounts,
  type MockRejectionEvent,
  type MockResearchSignalInput,
} from "@/lib/trading/mockTradingEngine";
import type { MarketRegime } from "@/lib/ai/marketRegimeClassifier";

const MAX_SIGNALS_PER_MINUTE_DEFAULT = 10;
const MIN_CONFIDENCE_DEFAULT = 50;

export type ResearchSelectionMode = "RESEARCH_MODE" | "PROFIT_MODE" | "REGIME_MODE";
type AnyResearchFamily = ResearchFamily | BtcResearchFamily | string;
type AnyResearchStrategy = ResearchStrategy | BtcResearchStrategy | InstitutionalStrategy;

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
  diagnostics: ResearchRunnerDiagnostics;
  readiness: ResearchReadinessSummary;
  /** List of all 500 strategy definitions (for UI display). */
  strategies: readonly AnyResearchStrategy[];
  /** Family display labels. */
  familyLabels: Record<string, string>;
}

const SIGNAL_RING_CAP = 200;

export interface ResearchEvaluationResult {
  evaluatedCount: number;
  signals: ResearchSignalSummary[];
  diagnostics: ResearchEvaluationDiagnostics;
  readiness: ResearchReadinessSummary;
}

export interface StrategyReadinessRow {
  strategyId: number;
  strategyName: string;
  family: string;
  requiredCandles: number;
  availableCandles: number;
  ready: boolean;
  enabled: boolean;
  dataFeedRequired: boolean;
}

export interface ResearchReadinessSummary {
  totalStrategies: number;
  strategiesReady: number;
  strategiesWaitingForHistory: number;
  minCandlesAvailable: number;
  maxCandlesAvailable: number;
  rows: StrategyReadinessRow[];
}

export interface ResearchEvaluationDiagnostics {
  funnel: MockDiagnosticFunnel;
  rejectionCounts: MockRejectionCounts;
  recentRejections: MockRejectionEvent[];
}

export interface ResearchRunnerDiagnostics {
  latest: ResearchEvaluationDiagnostics;
  totals: ResearchEvaluationDiagnostics;
}

function emptyResearchEvaluationDiagnostics(): ResearchEvaluationDiagnostics {
  return {
    funnel: emptyMockDiagnosticFunnel(),
    rejectionCounts: emptyMockRejectionCounts(),
    recentRejections: [],
  };
}

function buildReadinessSummary(
  strategies: readonly AnyResearchStrategy[],
  availableCandles: number,
): ResearchReadinessSummary {
  const rows = strategies.map((strategy) => {
    const dataFeedRequired = "dataFeedRequired" in strategy ? strategy.dataFeedRequired : false;
    const enabled = strategy.enabled === true;
    return {
      strategyId: strategy.id,
      strategyName: strategy.name,
      family: strategy.family,
      requiredCandles: strategy.minCandles,
      availableCandles,
      ready: enabled && !dataFeedRequired && availableCandles >= strategy.minCandles,
      enabled,
      dataFeedRequired,
    };
  });
  return {
    totalStrategies: rows.length,
    strategiesReady: rows.filter((row) => row.ready).length,
    strategiesWaitingForHistory: rows.filter((row) => row.enabled && !row.dataFeedRequired && row.availableCandles < row.requiredCandles).length,
    minCandlesAvailable: rows.length > 0 ? availableCandles : 0,
    maxCandlesAvailable: rows.length > 0 ? availableCandles : 0,
    rows,
  };
}

function emptyReadinessSummary(): ResearchReadinessSummary {
  return buildReadinessSummary([], 0);
}

function emptyResearchRunnerDiagnostics(): ResearchRunnerDiagnostics {
  return {
    latest: emptyResearchEvaluationDiagnostics(),
    totals: emptyResearchEvaluationDiagnostics(),
  };
}

function addResearchRejection(
  diagnostics: ResearchEvaluationDiagnostics,
  event: MockRejectionEvent,
): void {
  diagnostics.rejectionCounts[event.category] += 1;
  diagnostics.recentRejections.unshift(event);
  if (diagnostics.recentRejections.length > 25) diagnostics.recentRejections.length = 25;
}

function mergeResearchDiagnostics(
  previous: ResearchRunnerDiagnostics,
  latest: ResearchEvaluationDiagnostics,
): ResearchRunnerDiagnostics {
  const totals: ResearchEvaluationDiagnostics = {
    funnel: { ...previous.totals.funnel },
    rejectionCounts: { ...previous.totals.rejectionCounts },
    recentRejections: [...latest.recentRejections, ...previous.totals.recentRejections].slice(0, 50),
  };
  for (const key of Object.keys(latest.funnel) as Array<keyof MockDiagnosticFunnel>) {
    totals.funnel[key] += latest.funnel[key];
  }
  for (const key of Object.keys(latest.rejectionCounts) as Array<keyof MockRejectionCounts>) {
    totals.rejectionCounts[key] += latest.rejectionCounts[key];
  }
  return { latest, totals };
}

export function evaluateMockResearchStrategies(
  candles: readonly OHLCVCandle[],
  config: ResearchRunnerConfig,
  currentRegimeOrNow: MarketRegime | null | number = null,
  nowArg?: number,
): ResearchEvaluationResult {
  if (!config.enabled || candles.length < 5) {
    return {
      evaluatedCount: 0,
      signals: [],
      diagnostics: emptyResearchEvaluationDiagnostics(),
      readiness: buildReadinessSummary([...RESEARCH_STRATEGIES, ...BTC_RESEARCH_STRATEGIES], candles.length),
    };
  }
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
    // Institutional strategies (2100–2119) always pass in any mode
    if (strategyId >= 2100 && strategyId <= 2119) return true;
    if (strategyId < 2000 || strategyId > 2059) return false;
    if (config.approvedStrategyIds.size === 0) return false;
    return config.approvedStrategyIds.has(strategyId);
  };

  const snap = candles.slice();
  const readiness = buildReadinessSummary(
    [...RESEARCH_STRATEGIES, ...BTC_RESEARCH_STRATEGIES, ...INSTITUTIONAL_STRATEGIES],
    snap.length,
  );
  const signals: ResearchSignalSummary[] = [];
  const diagnostics = emptyResearchEvaluationDiagnostics();
  let evaluatedCount = 0;

  for (const strat of RESEARCH_STRATEGIES) {
    if (!strat.enabled) continue;
    if (!isFamilyEnabled(strat.family)) continue;
    if (snap.length < strat.minCandles) continue;
    evaluatedCount++;
    try {
      const result = strat.signal(snap);
      if (result.side === "NO_SIGNAL") continue;
      diagnostics.funnel.signalsGenerated++;
      if (result.confidence < config.minConfidence) {
        addResearchRejection(diagnostics, {
          ts: now,
          category: "LOW_CONFIDENCE",
          strategyId: strat.id,
          strategyName: strat.name,
          side: result.side,
          confidenceScore: result.confidence,
          message: `confidence ${result.confidence.toFixed(1)} below ${config.minConfidence.toFixed(1)}`,
        });
        continue;
      }
      diagnostics.funnel.confidencePassed++;
      if (!passesSelectionMode(strat.id)) {
        addResearchRejection(diagnostics, {
          ts: now,
          category: "NO_APPROVED_STRATEGY",
          strategyId: strat.id,
          strategyName: strat.name,
          side: result.side,
          confidenceScore: result.confidence,
          message: "strategy is not in the approved Profit/Regime mode set",
        });
        continue;
      }
      diagnostics.funnel.scorePassed++;
      diagnostics.funnel.riskRewardPassed++;
      diagnostics.funnel.riskPassed++;
      diagnostics.funnel.cooldownPassed++;
      diagnostics.funnel.familyConflictPassed++;
      diagnostics.funnel.exposurePassed++;
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
      diagnostics.funnel.signalsGenerated++;
      if (result.confidence < config.minConfidence) {
        addResearchRejection(diagnostics, {
          ts: now,
          category: "LOW_CONFIDENCE",
          strategyId: strat.id,
          strategyName: strat.name,
          side: result.side,
          confidenceScore: result.confidence,
          message: `confidence ${result.confidence.toFixed(1)} below ${config.minConfidence.toFixed(1)}`,
        });
        continue;
      }
      diagnostics.funnel.confidencePassed++;
      if (!passesSelectionMode(strat.id)) {
        addResearchRejection(diagnostics, {
          ts: now,
          category: "NO_APPROVED_STRATEGY",
          strategyId: strat.id,
          strategyName: strat.name,
          side: result.side,
          confidenceScore: result.confidence,
          message: config.selectionMode === "RESEARCH_MODE"
            ? "strategy not selected"
            : "strategy is not in the approved Profit/Regime mode set",
        });
        continue;
      }
      diagnostics.funnel.scorePassed++;
      diagnostics.funnel.riskRewardPassed++;
      diagnostics.funnel.riskPassed++;
      diagnostics.funnel.cooldownPassed++;
      diagnostics.funnel.familyConflictPassed++;
      diagnostics.funnel.exposurePassed++;
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

  // ── Institutional strategies (IDs 2100–2119) — always evaluated in RESEARCH_MODE ──
  for (const strat of INSTITUTIONAL_STRATEGIES) {
    if (!strat.enabled) continue;
    if (snap.length < strat.minCandles) continue;
    evaluatedCount++;
    try {
      const result = strat.signal(snap);
      if (result.side === "NO_SIGNAL") continue;
      diagnostics.funnel.signalsGenerated++;
      if (result.confidence < config.minConfidence) {
        addResearchRejection(diagnostics, {
          ts: now,
          category: "LOW_CONFIDENCE",
          strategyId: strat.id,
          strategyName: strat.name,
          side: result.side,
          confidenceScore: result.confidence,
          message: `confidence ${result.confidence.toFixed(1)} below ${config.minConfidence.toFixed(1)}`,
        });
        continue;
      }
      diagnostics.funnel.confidencePassed++;
      diagnostics.funnel.scorePassed++;
      diagnostics.funnel.riskRewardPassed++;
      diagnostics.funnel.riskPassed++;
      diagnostics.funnel.cooldownPassed++;
      diagnostics.funnel.familyConflictPassed++;
      diagnostics.funnel.exposurePassed++;
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
      // Suppress individual strategy errors — one bad signal must not block others.
    }
  }

  const capped =
    signals.length <= config.maxSignalsPerMinute
      ? signals
      : [...signals].sort((a, b) => b.confidence - a.confidence).slice(0, config.maxSignalsPerMinute);

  if (signals.length > capped.length) {
    const dropped = signals.length - capped.length;
    diagnostics.rejectionCounts.OTHER += dropped;
  }
  return { evaluatedCount, signals: capped, diagnostics, readiness };
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
  const [diagnostics, setDiagnostics] = useState<ResearchRunnerDiagnostics>(emptyResearchRunnerDiagnostics);
  const [readiness, setReadiness] = useState<ResearchReadinessSummary>(() => emptyReadinessSummary());

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
    const {
      evaluatedCount,
      signals: capped,
      diagnostics: latestDiagnostics,
      readiness: latestReadiness,
    } = evaluateMockResearchStrategies(snap, cfg, regimeRef.current, now);

    setLastEvalCount(evaluatedCount);
    setLastSignalCount(capped.length);
    setHasRun(true);
    setLastEvalAt(now);
    setDiagnostics((prev) => mergeResearchDiagnostics(prev, latestDiagnostics));
    setReadiness(latestReadiness);

    for (const row of latestReadiness.rows) {
      if (row.enabled && !row.dataFeedRequired && row.availableCandles < row.requiredCandles) {
        console.info("[CANDLE_HISTORY_INSUFFICIENT]", {
          ts: now,
          strategyId: row.strategyId,
          strategyName: row.strategyName,
          requiredCandles: row.requiredCandles,
          availableCandles: row.availableCandles,
        });
      }
    }

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
    diagnostics,
    readiness,
    strategies: [...RESEARCH_STRATEGIES, ...BTC_RESEARCH_STRATEGIES, ...INSTITUTIONAL_STRATEGIES],
    familyLabels: { ...RESEARCH_FAMILY_LABELS, ...BTC_RESEARCH_FAMILY_LABELS, ...INSTITUTIONAL_FAMILY_LABELS },
  };
}
