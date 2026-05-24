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
import {
  computeStrategyRotation,
  type RotationReport,
} from "@/lib/futuresStrategyRotation";
import { computeGoLiveGates, type GoLiveGateReport } from "@/lib/futuresGoLiveGates";
import { deskReplayGateEnabled } from "@/lib/futuresReplayCompare";
import { utcDateString } from "@/lib/futuresSoakTracker";

export type { TuneRecommendation, RotationReport };

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

export type GradeHistoryEntry = {
  grade: HealthCheckResult["grade"];
  timestamp: number;
  expectancy: number;
  winRate: number;
};

export interface MonitorState {
  diagnostics: DiagnosticSummary | null;
  healthCheck: HealthCheckResult | null;
  recommendations: TuningRecommendation[];
  tuneRecommendation: TuneRecommendation | null;
  timeExitCount: number;
  gradeHistory: GradeHistoryEntry[];
  rotationReport: RotationReport | null;
  goLiveGates: GoLiveGateReport | null;
  replaySignFlipRate: number | null;
  tradesAll: PaperTradeDbRow[];
  lastFetchAt: number | null;
  fetchError: string | null;
  isFetching: boolean;
}

const POLL_INTERVAL_MS = 60_000;
const REPLAY_FETCH_INTERVAL_MS = 3_600_000;

function priorUtcDates(count: number, nowMs = Date.now()): string[] {
  const dates: string[] = [];
  const base = new Date(nowMs);
  for (let i = 0; i < count; i++) {
    const d = new Date(
      Date.UTC(base.getUTCFullYear(), base.getUTCMonth(), base.getUTCDate() - i),
    );
    dates.push(utcDateString(d.getTime()));
  }
  return dates;
}

async function fetchReplaySignFlipRate(accountKey: string): Promise<number | null> {
  if (!deskReplayGateEnabled()) return null;

  const dates = priorUtcDates(3);
  const rates: number[] = [];

  for (const date of dates) {
    try {
      const params = new URLSearchParams({ account_key: accountKey, date });
      const res = await fetch(`/api/paper-replay-compare?${params.toString()}`, {
        cache: "no-store",
      });
      const body = (await res.json()) as {
        ok?: boolean;
        signFlipRate?: number;
      };
      if (body.ok && typeof body.signFlipRate === "number") {
        rates.push(body.signFlipRate);
      }
    } catch {
      /* network / 503 — skip day */
    }
  }

  return rates.length ? Math.max(...rates) : null;
}

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
  options?: { enabled?: boolean },
): MonitorState {
  const pollingEnabled = options?.enabled !== false;
  const [state, setState] = useState<MonitorState>({
    diagnostics: null,
    healthCheck: null,
    recommendations: [],
    tuneRecommendation: null,
    timeExitCount: 0,
    gradeHistory: [],
    rotationReport: null,
    goLiveGates: null,
    replaySignFlipRate: null,
    tradesAll: [],
    lastFetchAt: null,
    fetchError: null,
    isFetching: false,
  });

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const prevGradeRef = useRef<string | null>(null);
  const lastReplayFetchAtRef = useRef(0);
  const replaySignFlipRateRef = useRef<number | null>(null);

  const fetchDiagnostics = useCallback(async () => {
    if (!pollingEnabled) return;
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
        tradesAll?: PaperTradeDbRow[];
      };

      if (!res.ok || !data.ok) {
        throw new Error(data.error ?? `HTTP ${res.status}`);
      }

      const diagnostics = data.diagnostics ?? null;
      const healthCheck = data.healthCheck ?? null;
      const trades = data.trades ?? [];
      const tradesAll = data.tradesAll ?? trades;

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

      const rotationReport = diagnostics
        ? computeStrategyRotation(diagnostics.rows)
        : null;

      if (rotationReport) {
        console.info(
          `[Rotation] Active:${rotationReport.active.length} ` +
            `Promoted:${rotationReport.promoted.length} ` +
            `Probation:${rotationReport.probation.length} ` +
            `Suspended:${rotationReport.suspended.length}`,
        );
      }

      let replaySignFlipRate = replaySignFlipRateRef.current;
      const now = Date.now();
      if (
        accountKey.trim() &&
        deskReplayGateEnabled() &&
        now - lastReplayFetchAtRef.current >= REPLAY_FETCH_INTERVAL_MS
      ) {
        lastReplayFetchAtRef.current = now;
        replaySignFlipRate = await fetchReplaySignFlipRate(accountKey);
        replaySignFlipRateRef.current = replaySignFlipRate;
        if (replaySignFlipRate != null) {
          console.info(
            `[ReplayGate] signFlip=${(replaySignFlipRate * 100).toFixed(1)}% (max of last 3 UTC days)`,
          );
        }
      }

      const goLiveGates =
        tradesAll.length >= 10
          ? computeGoLiveGates({
              trades: tradesAll,
              health: healthCheck,
              readiness: null,
              replaySignFlipRate,
            })
          : null;

      if (goLiveGates) {
        console.info(
          `[GoLive] ${goLiveGates.recommendation} score=${(goLiveGates.score * 100).toFixed(0)}% ` +
            `blockers=${goLiveGates.blockers.filter((g) => !g.pass).length}`,
        );
      }

      setState((prev) => {
        let nextGradeHistory = prev.gradeHistory;
        if (healthCheck && healthCheck.grade !== prevGradeRef.current) {
          const entry: GradeHistoryEntry = {
            grade: healthCheck.grade,
            timestamp: Date.now(),
            expectancy: healthCheck.expectancy,
            winRate: healthCheck.winRate,
          };
          nextGradeHistory = [entry, ...prev.gradeHistory].slice(0, 20);
          console.info(
            `[Monitor] Grade change: ${prevGradeRef.current ?? "N/A"} → ${healthCheck.grade} at ${new Date().toISOString()}`,
          );
          prevGradeRef.current = healthCheck.grade;
        }

        return {
          diagnostics,
          healthCheck,
          recommendations,
          tuneRecommendation,
          timeExitCount,
          gradeHistory: nextGradeHistory,
          rotationReport,
          goLiveGates,
          replaySignFlipRate,
          tradesAll,
          lastFetchAt: Date.now(),
          fetchError: null,
          isFetching: false,
        };
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
  }, [accountKey, currentThreshold, currentTpPct, currentSlPct, currentSameSide, pollingEnabled]);

  useEffect(() => {
    if (!pollingEnabled) return;
    if (!accountKey || accountKey.trim() === "") return;

    void fetchDiagnostics();
    timerRef.current = setInterval(() => {
      void fetchDiagnostics();
    }, POLL_INTERVAL_MS);

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [accountKey, fetchDiagnostics, pollingEnabled]);

  return state;
}
