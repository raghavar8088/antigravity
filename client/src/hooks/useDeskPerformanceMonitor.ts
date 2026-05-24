"use client";

/**
 * useDeskPerformanceMonitor
 * Polls /api/paper-diagnostics every 60s.
 * Emits tuning recommendations based on real closed-trade data.
 * Read-only recommendations — runtime blocks require explicit user action
 * in DeskMonitorPanel (no auto-block on poll).
 */
import { useCallback, useEffect, useRef, useState } from "react";
import type {
  DiagnosticSummary,
  HealthCheckResult,
} from "@/lib/futuresStrategyDiagnostics";
import {
  recommendOneTune,
  type TuneRecommendation,
} from "@/lib/futuresParameterTuner";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";

export type { TuneRecommendation };

export interface TuningRecommendation {
  type:
    | "RAISE_THRESHOLD"
    | "LOWER_THRESHOLD"
    | "BLOCK_STRATEGY"
    | "REDUCE_SAME_SIDE_CAP"
    | "INCREASE_TP"
    | "WIDEN_SL"
    | "REGIME_MISMATCH"
    | "FEE_TOO_HIGH";
  strategyId?: number;
  strategyName?: string;
  reason: string;
  severity: "INFO" | "WARN" | "CRITICAL";
  suggestedValue?: number;
  currentValue?: number;
}

export interface MonitorState {
  diagnostics: DiagnosticSummary | null;
  healthCheck: HealthCheckResult | null;
  recommendations: TuningRecommendation[];
  tuneRecommendation: TuneRecommendation | null;
  timeExitCount: number;
  lastFetchAt: number | null;
  fetchError: string | null;
  isFetching: boolean;
}

const POLL_INTERVAL_MS = 60_000;

function deriveRecommendations(
  diagnostics: DiagnosticSummary,
  health: HealthCheckResult,
): TuningRecommendation[] {
  const recs: TuningRecommendation[] = [];

  if (!health.expectancyPass) {
    recs.push({
      type: "RAISE_THRESHOLD",
      reason: `Expectancy ${health.expectancy.toFixed(2)} is negative. Raise signal threshold by 4 pts to filter low-quality entries.`,
      severity: "CRITICAL",
      suggestedValue: 4,
    });
  }

  if (!health.feePass) {
    recs.push({
      type: "FEE_TOO_HIGH",
      reason: `fee/|gross| = ${(health.feePctOfAbsGross * 100).toFixed(1)}%. Target < 50%. Entries closing too soon or notional too small.`,
      severity: "CRITICAL",
    });
  }

  if (!health.tpHitPass) {
    recs.push({
      type: "INCREASE_TP",
      reason: `Only ${health.tpHits} TP hits in ${health.window} trades. Consider widening SL or tightening TP to improve hit rate.`,
      severity: "WARN",
    });
  }

  for (const row of diagnostics.slDominatedStrats) {
    if (row.totalTrades < 3) continue;

    const slPct = row.slCount / row.totalTrades;

    if (slPct > 0.8) {
      recs.push({
        type: "BLOCK_STRATEGY",
        strategyId: row.strategyId,
        strategyName: row.strategyName,
        reason: `${row.strategyName} has ${(slPct * 100).toFixed(0)}% SL rate over ${row.totalTrades} trades. Expected edge is gone.`,
        severity: "CRITICAL",
      });
    } else {
      recs.push({
        type: "WIDEN_SL",
        strategyId: row.strategyId,
        strategyName: row.strategyName,
        reason: `${row.strategyName} SL rate ${(slPct * 100).toFixed(0)}%. SL may be too tight for current volatility.`,
        severity: "WARN",
      });
    }
  }

  for (const row of diagnostics.highFeeStrategies) {
    recs.push({
      type: "FEE_TOO_HIGH",
      strategyId: row.strategyId,
      strategyName: row.strategyName,
      reason: `${row.strategyName} fee/|gross|=${(row.feePctOfAbsGross * 100).toFixed(1)}%. Avg hold ${row.avgHoldMinutes.toFixed(1)}m is too short.`,
      severity: "WARN",
    });
  }

  for (const row of diagnostics.bottomByExpectancy) {
    if (row.totalTrades >= 5 && row.avgNetPnl < -10) {
      recs.push({
        type: "BLOCK_STRATEGY",
        strategyId: row.strategyId,
        strategyName: row.strategyName,
        reason: `${row.strategyName} avg PnL $${row.avgNetPnl.toFixed(2)} over ${row.totalTrades} trades. Negative edge confirmed.`,
        severity: row.avgNetPnl < -20 ? "CRITICAL" : "WARN",
      });
    }
  }

  const seen = new Set<string>();
  return recs.filter((r) => {
    const key = `${r.type}-${r.strategyId ?? "global"}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function useDeskPerformanceMonitor(
  accountKey: string | null | undefined,
  currentThreshold = 28,
  currentTpPct = 1.5,
  currentSlPct = 0.5,
  currentSameSide = 2,
): MonitorState {
  const [state, setState] = useState<MonitorState>({
    diagnostics: null,
    healthCheck: null,
    recommendations: [],
    tuneRecommendation: null,
    timeExitCount: 0,
    lastFetchAt: null,
    fetchError: null,
    isFetching: false,
  });

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchDiagnostics = useCallback(async () => {
    if (!accountKey || accountKey.trim() === "") return;

    setState((prev) => ({ ...prev, isFetching: true, fetchError: null }));

    try {
      const params = new URLSearchParams({
        account_key: accountKey.trim(),
        window: "20",
      });
      const res = await fetch(`/api/paper-diagnostics?${params}`, {
        credentials: "include",
        cache: "no-store",
      });

      const data = (await res.json()) as {
        ok?: boolean;
        error?: string;
        diagnostics?: DiagnosticSummary;
        healthCheck?: HealthCheckResult;
        trades?: PaperTradeDbRow[];
      };

      if (!res.ok || !data.ok) {
        throw new Error(data.error ?? `HTTP ${res.status}`);
      }

      const diagnostics = data.diagnostics ?? null;
      const healthCheck = data.healthCheck ?? null;
      const trades = data.trades ?? [];

      const timeExitCount = trades.filter((t) => t.exit_reason === "TIME").length;

      const tuneRecommendation =
        trades.length >= 10
          ? recommendOneTune(
              trades,
              currentThreshold,
              currentTpPct,
              currentSlPct,
              currentSameSide,
              50,
            )
          : null;

      if (tuneRecommendation && tuneRecommendation.target !== "NO_CHANGE") {
        console.info(
          `[Tuner] Recommend: ${tuneRecommendation.target} ` +
            `${tuneRecommendation.currentValue} → ${tuneRecommendation.suggestedValue} ` +
            `(${tuneRecommendation.confidence} confidence, ${tuneRecommendation.tradesAnalyzed} trades)`,
        );
      }

      const recommendations =
        diagnostics && healthCheck
          ? deriveRecommendations(diagnostics, healthCheck)
          : [];

      recommendations
        .filter((r) => r.severity === "CRITICAL")
        .forEach((r) => console.warn("[Monitor] CRITICAL:", r.reason));

      setState({
        diagnostics,
        healthCheck,
        recommendations,
        tuneRecommendation,
        timeExitCount,
        lastFetchAt: Date.now(),
        fetchError: null,
        isFetching: false,
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error("[Monitor] fetch failed:", msg);
      setState((prev) => ({
        ...prev,
        isFetching: false,
        fetchError: msg,
      }));
    }
  }, [accountKey, currentThreshold, currentTpPct, currentSlPct, currentSameSide]);

  useEffect(() => {
    if (!accountKey || accountKey.trim() === "") return;

    void fetchDiagnostics();
    timerRef.current = setInterval(() => {
      void fetchDiagnostics();
    }, POLL_INTERVAL_MS);

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [accountKey, fetchDiagnostics]);

  return state;
}
