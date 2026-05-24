"use client";

/**
 * DeskMonitorPanel
 * Additive component — drop into existing Winners desk layout.
 * Shows: health grade, recommendations, skip reason summary,
 *        top/bottom strategies, runtime blocklist.
 * Polls via useDeskPerformanceMonitor (read-only diagnostics fetch).
 */
import { useEffect, useMemo, useState } from "react";
import {
  useDeskPerformanceMonitor,
  type MonitorState,
  type RotationReport,
} from "@/hooks/useDeskPerformanceMonitor";
import { StrategyRotationPanel } from "@/components/StrategyRotationPanel";
import { generateHealthReport } from "@/lib/futuresHealthReport";
import {
  blockStrategyRuntime,
  getRuntimeBlocklist,
  unblockStrategyRuntime,
} from "@/lib/futuresDeskPolicy";
import { markAgeMs } from "@/lib/canonicalMarkPrice";
import {
  runProductionReadiness,
  type ReadinessReport,
} from "@/lib/futuresProductionReadiness";
import type { TuningRecommendation } from "@/hooks/useDeskPerformanceMonitor";
import { deriveDeskRecommendation, type DeskRecommendation } from "@/lib/futuresDeskRecommendation";
import type { AttributionReport } from "@/lib/futuresAttribution";
import { computeGoLiveGates } from "@/lib/futuresGoLiveGates";
import { generateValidationReport } from "@/lib/futuresValidationReport";
import { GoLiveGatesPanel } from "@/components/GoLiveGatesPanel";
import type { DiagnosticSummary } from "@/lib/futuresStrategyDiagnostics";
import { suggestWinnersFromScorecard } from "@/lib/futuresWinnersRefresh";
import type { SoakDaySnapshot } from "@/lib/futuresSoakTracker";
import type { UnifiedReadiness } from "@/lib/futuresUnifiedReadiness";

type SkipSummaryRow = { reason: string; count: number };

export interface DeskMonitorPanelProps {
  accountKey: string | null | undefined;
  skipReasonSummary?: SkipSummaryRow[] | Record<string, number>;
  runtimeBlocklist?: number[];
  signalThreshold?: number;
  leverage?: number;
  takerFeePct?: number;
  maxSameSide?: number;
  minNotional?: number;
  openPositionCount?: number;
  currentRegime?: string;
  mongoConnected?: boolean;
  currentTpPct?: number;
  currentSlPct?: number;
  onRotationReport?: (report: RotationReport) => void;
  onRestoreRotationStrategy?: (strategyId: number) => void;
  attributionReport?: AttributionReport | null;
  rotationReport?: RotationReport | null;
  qualitySkipCount?: number;
  mtfSkipCount?: number;
  gateEvaluationCount?: number;
  engineDeskRecommendation?: DeskRecommendation | null;
  strategyDiagnostics?: DiagnosticSummary | null;
  unifiedReadiness?: UnifiedReadiness | null;
  unifiedReadinessBlockers?: string[];
  unifiedReadinessNextStep?: string;
  soakHistory?: SoakDaySnapshot[];
  engineReplaySignFlipRate?: number | null;
  onReplaySignFlipRate?: (rate: number | null) => void;
  /** Shared monitor from DeskCommandCenter — avoids duplicate polling. */
  monitorState?: MonitorState;
  /** UI-20: go-live shown on Health tab instead. */
  hideGoLiveGates?: boolean;
  /** UI-20: rotation shown on Health tab instead. */
  hideStrategyRotation?: boolean;
  /** UI-20: winners roster shown in Advanced tab instead. */
  hideWinnersSuggestion?: boolean;
  /** UI-20: collapsed recommendations count on Health tab. */
  compactRecommendations?: boolean;
}

const SEV_COLOR: Record<TuningRecommendation["severity"], string> = {
  INFO: "#58a6ff",
  WARN: "#d29922",
  CRITICAL: "#f85149",
};

const GRADE_COLOR: Record<string, string> = {
  A: "#3fb950",
  B: "#d29922",
  C: "#ff7c2c",
  F: "#f85149",
};

function skipSummaryToEntries(
  summary: SkipSummaryRow[] | Record<string, number> | undefined,
): [string, number][] {
  if (!summary) return [];
  if (Array.isArray(summary)) {
    return summary.map((r) => [r.reason, r.count]);
  }
  return Object.entries(summary);
}

export function DeskMonitorPanel({
  accountKey,
  skipReasonSummary = [],
  runtimeBlocklist = [],
  signalThreshold = 28,
  leverage = 25,
  takerFeePct = 0.001,
  maxSameSide = 2,
  minNotional = 100,
  openPositionCount = 0,
  currentRegime = "unknown",
  mongoConnected = false,
  currentTpPct = 1.5,
  currentSlPct = 0.5,
  onRotationReport,
  onRestoreRotationStrategy,
  attributionReport = null,
  rotationReport: rotationReportProp = null,
  qualitySkipCount = 0,
  mtfSkipCount = 0,
  gateEvaluationCount = 0,
  engineDeskRecommendation = null,
  strategyDiagnostics: engineStrategyDiagnostics = null,
  unifiedReadiness = null,
  unifiedReadinessBlockers = [],
  unifiedReadinessNextStep,
  soakHistory = [],
  engineReplaySignFlipRate = null,
  onReplaySignFlipRate,
  monitorState: monitorStateProp,
  hideGoLiveGates = false,
  hideStrategyRotation = false,
  hideWinnersSuggestion = false,
  compactRecommendations = false,
}: DeskMonitorPanelProps) {
  const internalMonitor = useDeskPerformanceMonitor(
    accountKey,
    signalThreshold,
    currentTpPct,
    currentSlPct,
    maxSameSide,
    { enabled: monitorStateProp === undefined },
  );
  const monitor = monitorStateProp ?? internalMonitor;
  const [expanded, setExpanded] = useState(false);
  const [blocklistRevision, setBlocklistRevision] = useState(0);

  const {
    healthCheck,
    recommendations,
    diagnostics,
    tuneRecommendation,
    timeExitCount,
    gradeHistory,
    rotationReport,
    goLiveGates: monitorGoLiveGates,
    replaySignFlipRate: monitorReplaySignFlipRate,
    tradesAll,
    lastFetchAt,
    fetchError,
    isFetching,
  } = monitor;

  const replaySignFlipRate = monitorReplaySignFlipRate ?? engineReplaySignFlipRate;

  useEffect(() => {
    onReplaySignFlipRate?.(monitorReplaySignFlipRate);
  }, [monitorReplaySignFlipRate, onReplaySignFlipRate]);

  useEffect(() => {
    if (rotationReport && onRotationReport) {
      onRotationReport(rotationReport);
    }
  }, [rotationReport, onRotationReport]);

  const deskRecommendation = useMemo(() => {
    const fromMonitor = deriveDeskRecommendation({
      attribution: attributionReport,
      rotation: rotationReport ?? rotationReportProp,
      tune: tuneRecommendation,
      qualitySkipCount,
      mtfSkipCount,
      totalEvaluations: gateEvaluationCount,
    });
    if (fromMonitor.action !== "NO_CHANGE") return fromMonitor;
    return engineDeskRecommendation ?? fromMonitor;
  }, [
    attributionReport,
    diagnostics,
    rotationReport,
    rotationReportProp,
    tuneRecommendation,
    qualitySkipCount,
    mtfSkipCount,
    gateEvaluationCount,
    engineDeskRecommendation,
  ]);

  useEffect(() => {
    if (deskRecommendation.action !== "NO_CHANGE") {
      console.info(
        `[DeskRec] action=${deskRecommendation.action} confidence=${deskRecommendation.confidence} ` +
          `rationale=${deskRecommendation.rationale}`,
      );
    }
  }, [deskRecommendation]);

  const displayBlocklist = useMemo(() => {
    void blocklistRevision;
    void runtimeBlocklist;
    return getRuntimeBlocklist();
  }, [runtimeBlocklist, blocklistRevision]);

  const skipEntries = useMemo(
    () => skipSummaryToEntries(skipReasonSummary),
    [skipReasonSummary],
  );

  const grade = healthCheck?.grade ?? null;
  const gradeColor = grade ? (GRADE_COLOR[grade] ?? "#8b949e") : "#8b949e";

  const readiness: ReadinessReport | null = useMemo(() => {
    if (healthCheck == null) return null;
    return runProductionReadiness({
      signalThreshold,
      leverage,
      takerFeePct,
      maxSameSide,
      minPositionNotional: minNotional,
      openPositionCount,
      currentRegime,
      markPriceAgeMs: markAgeMs(),
      runtimeBlocklist: runtimeBlocklist ?? [],
      timeExitCount,
      health: healthCheck,
      closedTradeCount: diagnostics?.totalProduction ?? 0,
      nodeEnv: process.env.NODE_ENV ?? "",
      mongoConnected: mongoConnected || Boolean(accountKey?.trim()),
      accountKeySet: Boolean(accountKey?.trim()),
    });
  }, [
    healthCheck,
    signalThreshold,
    leverage,
    takerFeePct,
    maxSameSide,
    minNotional,
    openPositionCount,
    currentRegime,
    runtimeBlocklist,
    timeExitCount,
    diagnostics?.totalProduction,
    mongoConnected,
    accountKey,
  ]);

  const goLiveGates = useMemo(() => {
    if (!healthCheck || tradesAll.length < 10) return monitorGoLiveGates;
    return computeGoLiveGates({
      trades: tradesAll,
      health: healthCheck,
      readiness,
      shadowIntentCount: undefined,
      replaySignFlipRate,
    });
  }, [tradesAll, healthCheck, readiness, monitorGoLiveGates, replaySignFlipRate]);

  const copyValidationReport = () => {
    if (!goLiveGates) return;
    const text = generateValidationReport({
      gates: goLiveGates,
      health: healthCheck,
      readiness,
      attribution: attributionReport,
      accountKey: accountKey ?? "unknown",
      generatedAt: Date.now(),
      unifiedReadiness,
      unifiedBlockers: unifiedReadinessBlockers,
      unifiedNextStep: unifiedReadinessNextStep,
      soakHistory,
      replaySignFlipRate,
    });
    void navigator.clipboard.writeText(text).then(() => {
      console.info("[Monitor] Validation report copied to clipboard");
    });
  };

  const bumpBlocklist = () => setBlocklistRevision((n) => n + 1);

  const winnersSuggestion = useMemo(() => {
    const rows = engineStrategyDiagnostics?.rows ?? diagnostics?.rows;
    if (!rows?.length) return null;
    return suggestWinnersFromScorecard(rows);
  }, [engineStrategyDiagnostics?.rows, diagnostics?.rows]);

  const copyWinnersEnv = () => {
    if (!winnersSuggestion) return;
    void navigator.clipboard.writeText(winnersSuggestion.strategyIdsEnvLine).then(() => {
      console.info("[Monitor] Suggested roster env copied");
    });
  };

  return (
    <div
      style={{
        border: "1px solid #21262d",
        borderRadius: 8,
        padding: "8px 12px",
        fontSize: 11,
        color: "#e6edf3",
        marginTop: 8,
        background: "#0d1117",
      }}
    >
      <div
        style={{
          cursor: "pointer",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
        onClick={() => setExpanded((e) => !e)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setExpanded((v) => !v);
          }
        }}
        role="button"
        tabIndex={0}
      >
        <span style={{ fontWeight: 700, color: gradeColor }}>
          Desk Monitor
          {grade ? ` — Grade ${grade}` : ""}
          {isFetching ? " ⟳" : ""}
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {fetchError ? (
        <div style={{ color: "#f85149", marginTop: 4 }}>⚠ {fetchError}</div>
      ) : null}

      {!expanded && grade ? (
        <div style={{ color: "#8b949e", marginTop: 2 }}>
          {recommendations.filter((r) => r.severity === "CRITICAL").length} critical ·{" "}
          {recommendations.filter((r) => r.severity === "WARN").length} warnings · last updated{" "}
          {lastFetchAt
            ? `${Math.round((Date.now() - lastFetchAt) / 1000)}s ago`
            : "never"}
        </div>
      ) : null}

      {expanded ? (
        <>
          {healthCheck ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>
                Rolling {healthCheck.window}-Trade Health
              </div>
              {(
                [
                  ["Expectancy", `$${healthCheck.expectancy.toFixed(2)}`, healthCheck.expectancyPass],
                  ["Win Rate", `${(healthCheck.winRate * 100).toFixed(1)}%`, healthCheck.winRatePass],
                  [
                    "Fee/Gross",
                    `${(healthCheck.feePctOfAbsGross * 100).toFixed(1)}%`,
                    healthCheck.feePass,
                  ],
                  [
                    "Profit Factor",
                    healthCheck.profitFactor === Infinity
                      ? "∞"
                      : healthCheck.profitFactor.toFixed(2),
                    healthCheck.pfPass,
                  ],
                  ["TP Hits", `${healthCheck.tpHits}/${healthCheck.window}`, healthCheck.tpHitPass],
                ] as const
              ).map(([label, value, pass]) => (
                <div key={label} style={{ display: "flex", gap: 6 }}>
                  <span>{pass ? "✅" : "❌"}</span>
                  <span style={{ color: "#8b949e", minWidth: 90 }}>{label}</span>
                  <span>{value}</span>
                </div>
              ))}
            </div>
          ) : null}

          {recommendations.length > 0 && compactRecommendations ? (
            <div style={{ marginTop: 8, fontSize: 10, color: "#8b949e" }}>
              {recommendations.length} tuning recommendation
              {recommendations.length === 1 ? "" : "s"} — expand Health tab monitor for details
            </div>
          ) : null}

          {recommendations.length > 0 && !compactRecommendations ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>
                Recommendations ({recommendations.length})
              </div>
              {recommendations.map((r, i) => (
                <div
                  key={`${r.type}-${r.strategyId ?? i}`}
                  style={{
                    borderLeft: `2px solid ${SEV_COLOR[r.severity]}`,
                    paddingLeft: 6,
                    marginBottom: 6,
                    color: SEV_COLOR[r.severity],
                  }}
                >
                  <div style={{ fontWeight: 600 }}>
                    [{r.severity}] {r.type}
                    {r.strategyName ? ` — ${r.strategyName}` : ""}
                  </div>
                  <div style={{ color: "#e6edf3", fontSize: 10, marginTop: 2 }}>
                    {r.reason.replace(/\s+/g, " ").trim()}
                  </div>
                  {r.type === "BLOCK_STRATEGY" && r.strategyId != null ? (
                    <button
                      type="button"
                      style={{
                        marginTop: 4,
                        fontSize: 10,
                        padding: "2px 6px",
                        background: "#f85149",
                        border: "none",
                        borderRadius: 4,
                        cursor: "pointer",
                        color: "#fff",
                      }}
                      onClick={() => {
                        blockStrategyRuntime(r.strategyId!);
                        bumpBlocklist();
                      }}
                    >
                      Block Strategy (runtime)
                    </button>
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}

          {tuneRecommendation && tuneRecommendation.target !== "NO_CHANGE" ? (
            <div
              style={{
                marginTop: 8,
                border: "1px solid #d29922",
                borderRadius: 6,
                padding: "6px 10px",
              }}
            >
              <div style={{ fontWeight: 700, color: "#d29922", marginBottom: 4 }}>
                Parameter Tuner — ONE Recommendation
              </div>

              <div style={{ color: "#e6edf3" }}>
                <strong>Change:</strong> {tuneRecommendation.target}{" "}
                {tuneRecommendation.currentValue} → {tuneRecommendation.suggestedValue} (
                {tuneRecommendation.delta > 0 ? "+" : ""}
                {tuneRecommendation.delta})
              </div>

              <div style={{ color: "#8b949e", fontSize: 10, marginTop: 4 }}>
                Confidence: {tuneRecommendation.confidence} · {tuneRecommendation.tradesAnalyzed}{" "}
                trades analyzed
              </div>

              <div style={{ marginTop: 6, fontSize: 10, color: "#e6edf3" }}>
                {tuneRecommendation.rationale}
              </div>

              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr 1fr",
                  gap: 8,
                  marginTop: 8,
                }}
              >
                {[tuneRecommendation.beforeSim, tuneRecommendation.afterSim].map((sim) => (
                  <div
                    key={sim.label}
                    style={{
                      background: "#161b22",
                      borderRadius: 4,
                      padding: "4px 8px",
                      fontSize: 10,
                    }}
                  >
                    <div style={{ fontWeight: 600, marginBottom: 2 }}>{sim.label}</div>
                    <div>WR: {(sim.expectedWinRate * 100).toFixed(1)}%</div>
                    <div>E: ${sim.expectedExpectancy.toFixed(2)}</div>
                    <div>Fee: {(sim.expectedFeePct * 100).toFixed(1)}%</div>
                  </div>
                ))}
              </div>

              <div
                style={{
                  marginTop: 6,
                  fontSize: 10,
                  color: "#f85149",
                  fontStyle: "italic",
                }}
              >
                If ignored: {tuneRecommendation.doNothing}
              </div>

              <div style={{ marginTop: 4, fontSize: 10, color: "#8b949e" }}>
                This is a recommendation only. Apply changes manually in futuresDeskPolicy.ts or
                futuresCategoryStrategies.ts.
              </div>
            </div>
          ) : null}

          {tuneRecommendation?.target === "NO_CHANGE" ? (
            <div
              style={{
                marginTop: 8,
                fontSize: 10,
                color: "#3fb950",
                borderTop: "1px solid #21262d",
                paddingTop: 6,
              }}
            >
              Tuner: No parameter change needed ({tuneRecommendation.tradesAnalyzed} trades
              analyzed)
            </div>
          ) : null}

          {diagnostics ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>Top 5 by Expectancy</div>
              {diagnostics.topByExpectancy.map((r) => (
                <div
                  key={r.strategyId}
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    color: r.avgNetPnl > 0 ? "#3fb950" : "#f85149",
                  }}
                >
                  <span
                    style={{
                      maxWidth: 160,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {r.strategyName}
                  </span>
                  <span>
                    ${r.avgNetPnl.toFixed(2)} ({r.totalTrades}t)
                  </span>
                </div>
              ))}

              <div style={{ fontWeight: 600, marginBottom: 4, marginTop: 6 }}>
                Bottom 5 by Expectancy
              </div>
              {diagnostics.bottomByExpectancy.map((r) => (
                <div
                  key={r.strategyId}
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    color: "#f85149",
                  }}
                >
                  <span
                    style={{
                      maxWidth: 160,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {r.strategyName}
                  </span>
                  <span>
                    ${r.avgNetPnl.toFixed(2)} ({r.totalTrades}t)
                  </span>
                </div>
              ))}
            </div>
          ) : null}

          {winnersSuggestion && !hideWinnersSuggestion ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, marginBottom: 4, color: "#58a6ff" }}>
                Suggested roster (read-only)
              </div>
              <div style={{ fontSize: 10, color: "#8b949e", marginBottom: 6 }}>
                {winnersSuggestion.rationale}
              </div>
              {winnersSuggestion.promote.length > 0 ? (
                <div style={{ fontSize: 10, marginBottom: 4 }}>
                  <span style={{ color: "#3fb950", fontWeight: 600 }}>Promote: </span>
                  {winnersSuggestion.promote.join(", ")}
                </div>
              ) : null}
              {winnersSuggestion.demote.length > 0 ? (
                <div style={{ fontSize: 10, marginBottom: 6 }}>
                  <span style={{ color: "#f85149", fontWeight: 600 }}>Demote: </span>
                  {winnersSuggestion.demote.join(", ")}
                </div>
              ) : null}
              <pre
                style={{
                  fontSize: 9,
                  background: "#0d1117",
                  padding: 6,
                  borderRadius: 4,
                  color: "#8b949e",
                  overflow: "auto",
                }}
              >
                {winnersSuggestion.strategyIdsEnvLine}
              </pre>
              <button
                type="button"
                onClick={copyWinnersEnv}
                style={{
                  marginTop: 4,
                  fontSize: 10,
                  padding: "3px 10px",
                  background: "#161b22",
                  border: "1px solid #58a6ff",
                  borderRadius: 4,
                  cursor: "pointer",
                  color: "#58a6ff",
                }}
              >
                Copy roster env line
              </button>
              <div style={{ fontSize: 9, color: "#6e7681", marginTop: 4 }}>
                Paste into .env.local — does not change roster until you restart
              </div>
            </div>
          ) : null}

          {skipEntries.length > 0 ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>Entry Skip Reasons (session)</div>
              {skipEntries
                .sort(([, a], [, b]) => b - a)
                .map(([reason, count]) => (
                  <div
                    key={reason}
                    style={{
                      display: "flex",
                      justifyContent: "space-between",
                      color: "#8b949e",
                    }}
                  >
                    <span>{reason}</span>
                    <span>{count}</span>
                  </div>
                ))}
            </div>
          ) : null}

          {displayBlocklist.length > 0 ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, color: "#f85149", marginBottom: 4 }}>
                Runtime Blocked Strategies
              </div>
              {displayBlocklist.map((id) => (
                <div
                  key={id}
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    marginBottom: 4,
                  }}
                >
                  <span style={{ color: "#f85149" }}>Strategy {id}</span>
                  <button
                    type="button"
                    style={{
                      fontSize: 10,
                      padding: "1px 4px",
                      background: "transparent",
                      border: "1px solid #3fb950",
                      borderRadius: 3,
                      cursor: "pointer",
                      color: "#3fb950",
                    }}
                    onClick={() => {
                      unblockStrategyRuntime(id);
                      bumpBlocklist();
                    }}
                  >
                    Unblock
                  </button>
                </div>
              ))}
            </div>
          ) : null}

          {readiness ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div
                style={{
                  fontWeight: 700,
                  color: readiness.productionReady ? "#3fb950" : "#f85149",
                  marginBottom: 4,
                }}
              >
                {readiness.productionReady ? "✅" : "❌"} Production Readiness:{" "}
                {(readiness.score * 100).toFixed(0)}%
                {readiness.productionReady
                  ? " — READY"
                  : ` — ${readiness.criticalFails.length} CRITICAL`}
              </div>

              {readiness.checks.map((c) => (
                <div
                  key={c.id}
                  style={{
                    display: "flex",
                    gap: 6,
                    fontSize: 10,
                    color: c.pass
                      ? "#8b949e"
                      : c.severity === "CRITICAL"
                        ? "#f85149"
                        : "#d29922",
                  }}
                >
                  <span>{c.pass ? "✅" : c.severity === "CRITICAL" ? "❌" : "⚠"}</span>
                  <span style={{ minWidth: 180 }}>{c.label}</span>
                  <span>{c.value}</span>
                  {!c.pass ? (
                    <span style={{ color: "#8b949e" }}>(need {c.required})</span>
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}

          {gradeHistory.length > 0 ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>
                Grade History (last {gradeHistory.length} changes)
              </div>
              {gradeHistory.map((entry, i) => (
                <div
                  key={`${entry.timestamp}-${entry.grade}-${i}`}
                  style={{
                    display: "flex",
                    gap: 8,
                    fontSize: 10,
                    color:
                      entry.grade === "A"
                        ? "#3fb950"
                        : entry.grade === "B"
                          ? "#d29922"
                          : entry.grade === "C"
                            ? "#ff7c2c"
                            : "#f85149",
                  }}
                >
                  <span style={{ minWidth: 14, fontWeight: 700 }}>{entry.grade}</span>
                  <span style={{ color: "#8b949e", minWidth: 80 }}>
                    {new Date(entry.timestamp).toLocaleTimeString()}
                  </span>
                  <span>
                    E:${entry.expectancy.toFixed(2)} WR:{(entry.winRate * 100).toFixed(0)}%
                  </span>
                </div>
              ))}
            </div>
          ) : null}

          {deskRecommendation.action !== "NO_CHANGE" ? (
            <div
              style={{
                marginTop: 8,
                borderTop: "1px solid #21262d",
                paddingTop: 6,
                border: "1px solid #388bfd33",
                borderRadius: 6,
                padding: 8,
                background: "#161b22",
              }}
            >
              <div style={{ fontWeight: 700, color: "#58a6ff", marginBottom: 4 }}>
                Closed-loop recommendation ({deskRecommendation.confidence})
              </div>
              <div style={{ fontSize: 11, color: "#e6edf3", marginBottom: 4 }}>
                {deskRecommendation.action.replace(/_/g, " ")}
                {deskRecommendation.suggestedValue != null
                  ? ` → ${deskRecommendation.suggestedValue}`
                  : ""}
              </div>
              <div style={{ fontSize: 10, color: "#8b949e" }}>{deskRecommendation.rationale}</div>
              <div style={{ fontSize: 9, color: "#6e7681", marginTop: 4 }}>
                Recommendation only — not auto-applied
              </div>
            </div>
          ) : null}

          {!hideGoLiveGates ? (
            <GoLiveGatesPanel report={goLiveGates} onCopyReport={copyValidationReport} />
          ) : null}

          <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
            <button
              type="button"
              onClick={() => {
                const report = generateHealthReport({
                  health: healthCheck,
                  diagnostics,
                  rotation: rotationReport,
                  tune: tuneRecommendation,
                  readiness,
                  accountKey: accountKey ?? "unknown",
                  generatedAt: Date.now(),
                });
                void navigator.clipboard.writeText(report).then(() => {
                  console.info("[Monitor] Health report copied to clipboard");
                });
              }}
              style={{
                fontSize: 11,
                padding: "4px 12px",
                background: "#161b22",
                border: "1px solid #58a6ff",
                borderRadius: 4,
                cursor: "pointer",
                color: "#58a6ff",
              }}
            >
              📋 Copy Health Report
            </button>
            <span style={{ color: "#8b949e", fontSize: 10, marginLeft: 8 }}>
              Paste into notes or share with team
            </span>
          </div>

          <div style={{ color: "#8b949e", marginTop: 6, fontSize: 10 }}>
            Updated: {lastFetchAt ? new Date(lastFetchAt).toLocaleTimeString() : "never"} · polls
            every 60s
          </div>
        </>
      ) : null}

      {!hideStrategyRotation ? (
        <StrategyRotationPanel
          report={rotationReport}
          onRestore={onRestoreRotationStrategy}
        />
      ) : null}
    </div>
  );
}
