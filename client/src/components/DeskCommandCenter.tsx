"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { BTCFuturesEngineStats } from "@/hooks/useBTCFuturesScalperEngine";
import { useDeskPerformanceMonitor, type RotationReport } from "@/hooks/useDeskPerformanceMonitor";
import { DeskHealthBadge } from "@/components/DeskHealthBadge";
import { DeskMonitorPanel } from "@/components/DeskMonitorPanel";
import { AttributionPanel } from "@/components/AttributionPanel";
import { SignalQualityPanel } from "@/components/SignalQualityPanel";
import { MTFConfluencePanel } from "@/components/MTFConfluencePanel";
import { DeskPnLScorecardPanel } from "@/components/DeskPnLScorecardPanel";
import { ScorecardActionPanel } from "@/components/ScorecardActionPanel";
import { SoakTrendPanel } from "@/components/SoakTrendPanel";
import { GoLiveGatesPanel } from "@/components/GoLiveGatesPanel";
import { StrategyRotationPanel } from "@/components/StrategyRotationPanel";
import { ShadowIntentLogPanel } from "@/components/ShadowIntentLogPanel";
import { TestnetOpsPanel } from "@/components/TestnetOpsPanel";
import {
  DeskButton,
  DeskCard,
  DeskChip,
  DeskMetricTile,
  DeskTabs,
} from "@/components/desk/ui";
import {
  profitModeAllocationByEdgeEnabled,
  profitModeSessionGateEnabled,
  type ProfitModeConfig,
} from "@/lib/futuresProfitMode";
import { unifiedReadinessLabel } from "@/lib/futuresUnifiedReadiness";
import type { UnifiedReadiness } from "@/lib/futuresUnifiedReadiness";
import { formatScorecardActionEnv } from "@/lib/futuresScorecardActions";
import { scorecardShowsEnvFixCopy } from "@/components/deskCommandCenterHelpers";
import { computeGoLiveGates } from "@/lib/futuresGoLiveGates";
import { generateValidationReport } from "@/lib/futuresValidationReport";
import { runProductionReadiness } from "@/lib/futuresProductionReadiness";
import { markAgeMs } from "@/lib/canonicalMarkPrice";
import { suggestWinnersFromScorecard } from "@/lib/futuresWinnersRefresh";
import {
  commandCenterTabLabel,
  DEFAULT_COMMAND_CENTER_TAB,
  type DeskCommandCenterTab,
} from "@/components/deskCommandCenterTabs";
import { EntryDebugPanel } from "@/components/btcFutures/EntryDebugPanel";
import { EdgeCandidatesPanel } from "@/components/btcFutures/EdgeCandidatesPanel";
import { AiAppTrackerPanel } from "@/components/AiAppTrackerPanel";
import { SignalTracePanel } from "@/components/SignalTracePanel";
import { VerificationTrackPanel } from "@/components/VerificationTrackPanel";
import { EdgeLabPanel } from "@/components/EdgeLabPanel";
import { PaperOmsPanel } from "@/components/PaperOmsPanel";
import type { DeskEntryPollDebug } from "@/lib/futuresEntryDebug";
import {
  computeDeskOperatorHealth,
  OPERATOR_HEALTH_SEVERITY_COLOR,
} from "@/lib/deskOperatorHealth";
import { computeStrategySafety, formatDisableListAsEnv } from "@/lib/deskStrategySafety";
import { BTC_FT_DESK_BUILD, isDeploymentStale } from "@/lib/btcFtDeskBuild";
import type { WorkerEventDoc } from "@/lib/mongoTradesClient";

const READINESS_COLOR: Record<UnifiedReadiness, string> = {
  NOT_READY: "#f85149",
  COLLECT_DATA: "#8b949e",
  PAPER_EDGE_OK: "#d29922",
  PAPER_READY: "#58a6ff",
  TESTNET_SOAK_READY: "#3fb950",
};

export type DeskCommandCenterProps = {
  stats: BTCFuturesEngineStats;
  cloudAccountKey: string | null;
  profitModeCfg: ProfitModeConfig;
  setReplaySignFlipRate?: (rate: number | null) => void;
  onRotationReport?: (report: RotationReport) => void;
  onRestoreRotationStrategy?: (strategyId: number) => void;
  deskShadowIntentsEnabled: boolean;
  deskTestnetOpsEnabled: boolean;
  /** Compact: Advanced tab hidden until user opts in. */
  advancedTabGated?: boolean;
  /** Entry debug data forwarded from engine (shown in Advanced tab developer fold). */
  entryDebug?: DeskEntryPollDebug | null;
  pauseEntries?: boolean;
  /** Probe-dominant flag from BTCFuturesScalper (last 5 trades all probe/bootstrap). */
  probeDominant?: boolean;
  /** Worker enabled flag from env. */
  workerEnabled?: boolean;
  /** Worker is stale (heartbeat expired). */
  workerStale?: boolean;
  /** Base balance for initial balance reference. */
  baseBalance?: number;
  /** Server build SHA (from /api/health/desk-worker) for stale-deploy detection. */
  serverBuildSha?: string | null;
  /** Currently enabled strategy IDs (for safety suggestions env line). */
  enabledStrategyIds?: readonly number[];
};

function StrategyRotationSummary({
  report,
}: {
  report: RotationReport | null;
}) {
  if (!report) {
    return <p className="desk-label-md">Rotation: awaiting diagnostics</p>;
  }
  const top = [...report.scores]
    .sort((a, b) => b.score - a.score)
    .slice(0, 5);
  return (
    <div style={{ fontSize: "0.6875rem", color: "var(--desk-on-surface)" }}>
      <p className="desk-label-md" style={{ marginBottom: 8 }}>
        Suspended: {report.suspended.length} · Promoted: {report.promoted.length} · Active:{" "}
        {report.active.length}
      </p>
      {top.length > 0 ? (
        <table style={{ width: "100%", fontSize: "0.6875rem", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ color: "var(--desk-on-surface-variant)", textAlign: "left" }}>
              <th style={{ paddingBottom: 4 }}>Strategy</th>
              <th style={{ paddingBottom: 4 }}>Status</th>
              <th style={{ paddingBottom: 4 }}>Score</th>
            </tr>
          </thead>
          <tbody>
            {top.map((row) => (
              <tr key={row.strategyId} style={{ borderTop: "1px solid var(--desk-outline-variant)" }}>
                <td style={{ padding: "3px 0", color: "var(--desk-on-surface)" }}>
                  #{row.strategyId} {row.strategyName}
                </td>
                <td style={{ padding: "3px 0", color: "var(--desk-on-surface-variant)" }}>{row.status}</td>
                <td style={{ padding: "3px 0", color: "var(--desk-on-surface)" }}>{row.score.toFixed(0)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
    </div>
  );
}

export function DeskCommandCenter({
  stats,
  cloudAccountKey,
  profitModeCfg,
  setReplaySignFlipRate,
  onRotationReport,
  onRestoreRotationStrategy,
  deskShadowIntentsEnabled,
  deskTestnetOpsEnabled,
  advancedTabGated = false,
  entryDebug,
  pauseEntries = false,
  probeDominant = false,
  workerEnabled = false,
  workerStale = false,
  baseBalance = 1000,
  serverBuildSha = null,
  enabledStrategyIds = [],
}: DeskCommandCenterProps) {
  const [cardExpanded, setCardExpanded] = useState(true);
  const [activeTab, setActiveTab] = useState<DeskCommandCenterTab>(DEFAULT_COMMAND_CENTER_TAB);
  const [showAdvancedTab, setShowAdvancedTab] = useState(!advancedTabGated);
  const [blockersExpanded, setBlockersExpanded] = useState(false);
  const [workerEvents, setWorkerEvents] = useState<WorkerEventDoc[]>([]);
  const [eventsLoaded, setEventsLoaded] = useState(false);
  const deploymentStale = isDeploymentStale(serverBuildSha);

  // Lazy-load worker events when Advanced tab is opened
  useEffect(() => {
    if (activeTab !== "advanced" || eventsLoaded) return;
    setEventsLoaded(true);
    fetch("/api/desk-worker-events?limit=50", { cache: "no-store" })
      .then((r) => r.json())
      .then((data: { ok?: boolean; events?: WorkerEventDoc[] }) => {
        if (data.ok && Array.isArray(data.events)) setWorkerEvents(data.events);
      })
      .catch(() => {});
  }, [activeTab, eventsLoaded]);

  const monitor = useDeskPerformanceMonitor(
    cloudAccountKey,
    stats.effectiveSignalThreshold,
    1.5,
    0.5,
    2,
  );

  const {
    healthCheck,
    recommendations,
    diagnostics,
    rotationReport,
    tradesAll,
    replaySignFlipRate: monitorReplaySignFlipRate,
  } = monitor;

  const replaySignFlipRate = monitorReplaySignFlipRate ?? stats.replaySignFlipRate;

  useEffect(() => {
    setReplaySignFlipRate?.(monitorReplaySignFlipRate);
  }, [monitorReplaySignFlipRate, setReplaySignFlipRate]);

  useEffect(() => {
    if (rotationReport && onRotationReport) {
      onRotationReport(rotationReport);
    }
  }, [rotationReport, onRotationReport]);

  const readiness = useMemo(() => {
    if (!healthCheck) return null;
    return runProductionReadiness({
      signalThreshold: stats.effectiveSignalThreshold,
      leverage: 25,
      takerFeePct: 0.001,
      maxSameSide: 2,
      minPositionNotional: 100,
      openPositionCount: stats.openPositions,
      currentRegime: stats.deskLastRegimeTag,
      markPriceAgeMs: markAgeMs(),
      runtimeBlocklist: stats.runtimeBlocklistIds ?? [],
      timeExitCount: monitor.timeExitCount,
      health: healthCheck,
      closedTradeCount: diagnostics?.totalProduction ?? stats.totalTrades,
      nodeEnv: process.env.NODE_ENV ?? "",
      mongoConnected: Boolean(cloudAccountKey?.trim()),
      accountKeySet: Boolean(cloudAccountKey?.trim()),
    });
  }, [healthCheck, stats, cloudAccountKey, diagnostics?.totalProduction, monitor.timeExitCount]);

  const goLiveGates = useMemo(() => {
    if (stats.goLiveGates && tradesAll.length < 10) return stats.goLiveGates;
    if (!healthCheck || tradesAll.length < 10) return stats.goLiveGates;
    return computeGoLiveGates({
      trades: tradesAll,
      health: healthCheck,
      readiness,
      replaySignFlipRate,
    });
  }, [stats.goLiveGates, tradesAll, healthCheck, readiness, replaySignFlipRate]);

  const copyValidationReport = useCallback(() => {
    if (!goLiveGates) return;
    const text = generateValidationReport({
      gates: goLiveGates,
      health: healthCheck,
      readiness,
      attribution: stats.attributionReport,
      accountKey: cloudAccountKey ?? "unknown",
      generatedAt: Date.now(),
      unifiedReadiness: stats.unifiedReadiness,
      unifiedBlockers: stats.unifiedReadinessBlockers,
      unifiedNextStep: stats.unifiedReadinessNextStep,
      soakHistory: stats.soakHistory,
      replaySignFlipRate,
    });
    void navigator.clipboard.writeText(text).then(() => {
      console.info("[UI] Copied to clipboard");
    });
  }, [goLiveGates, healthCheck, readiness, stats, cloudAccountKey, replaySignFlipRate]);

  const winnersSuggestion = useMemo(() => {
    const rows = stats.strategyDiagnostics?.rows ?? diagnostics?.rows;
    if (!rows?.length) return null;
    return suggestWinnersFromScorecard(rows);
  }, [stats.strategyDiagnostics?.rows, diagnostics?.rows]);

  const copyWinnersEnv = () => {
    if (!winnersSuggestion) return;
    void navigator.clipboard.writeText(winnersSuggestion.strategyIdsEnvLine).then(() => {
      console.info("[UI] Copied to clipboard");
    });
  };

  // ── Operator health score ──────────────────────────────────────────────────
  const activeRotationReport = rotationReport ?? stats.rotationReport;
  const operatorHealth = useMemo(() => {
    const sc = stats.deskPnLScorecard;
    return computeDeskOperatorHealth({
      workerEnabled,
      workerLastPollAt: stats.workerLastPollAt,
      workerStale,
      balanceDriftUsd: stats.balanceDriftUsd ?? 0,
      totalTrades: stats.totalTrades,
      openPositions: stats.openPositions,
      closedTrades48h: sc?.closes48h ?? 0,
      regime: stats.deskLastRegimeTag ?? "unknown",
      skipRegime: stats.deskSkippedByRegime ?? 0,
      skipAtrFees: stats.deskSkippedMinExpectedMove ?? 0,
      rotationSuspended: activeRotationReport?.suspended.length ?? 0,
      rotationPending: activeRotationReport?.insufficient.length ?? 0,
      rotationActive: activeRotationReport?.active.length ?? 0,
      rotationPromoted: activeRotationReport?.promoted.length ?? 0,
      pauseEntries: pauseEntries ?? false,
      drawdownLocked: stats.isDrawdownLocked,
      probeDominant,
      scorecardState: sc?.paperReadyHint,
    });
  }, [
    workerEnabled, stats.workerLastPollAt, workerStale, stats.balanceDriftUsd,
    stats.totalTrades, stats.openPositions, stats.deskPnLScorecard,
    stats.deskLastRegimeTag, stats.deskSkippedByRegime, stats.deskSkippedMinExpectedMove,
    activeRotationReport, pauseEntries, stats.isDrawdownLocked, probeDominant,
  ]);

  // ── Strategy safety suggestions ───────────────────────────────────────────
  const strategySafety = useMemo(() => {
    if (!activeRotationReport && !stats.strategyDiagnostics) return null;
    return computeStrategySafety({
      rotationReport: activeRotationReport ?? null,
      diagnostics: stats.strategyDiagnostics ?? null,
      enabledStrategyIds,
    });
  }, [activeRotationReport, stats.strategyDiagnostics, enabledStrategyIds]);

  const disableListEnv = useMemo(() => {
    if (!strategySafety || strategySafety.autoDisableCandidates.length === 0) return null;
    return formatDisableListAsEnv(enabledStrategyIds, strategySafety.autoDisableCandidates);
  }, [strategySafety, enabledStrategyIds]);

  const scorecard = stats.deskPnLScorecard;
  const action = stats.scorecardAction;
  const last50 = scorecard?.last50;
  const readinessColor = READINESS_COLOR[stats.unifiedReadiness];
  const blockers = stats.unifiedReadinessBlockers;
  const showEnvFix = scorecardShowsEnvFixCopy(action);

  const topSkips = useMemo(() => {
    const rows = stats.skipReasonSummary ?? [];
    return [...rows].sort((a, b) => b.count - a.count).slice(0, 8);
  }, [stats.skipReasonSummary]);

  const tabItems = useMemo(() => {
    const base: { key: DeskCommandCenterTab; label: string }[] = [
      { key: "today", label: commandCenterTabLabel("today") },
      {
        key: "health",
        label:
          recommendations.length > 0
            ? `${commandCenterTabLabel("health")} (${recommendations.length})`
            : commandCenterTabLabel("health"),
      },
      { key: "gates", label: commandCenterTabLabel("gates") },
      { key: "signals", label: commandCenterTabLabel("signals") },
    ];
    if (showAdvancedTab) {
      base.push({ key: "advanced", label: commandCenterTabLabel("advanced") });
    }
    return base;
  }, [recommendations.length, showAdvancedTab]);

  const blockerPreview = blockers.slice(0, 2);
  const blockerRest = blockers.length > 2 ? blockers.slice(2) : [];

  const healthColor = OPERATOR_HEALTH_SEVERITY_COLOR[operatorHealth.severity];

  return (
    <DeskCard className="desk-command-center" padding="md">
      {/* ── Run Status card ── */}
      <div
        style={{
          borderLeft: `3px solid ${healthColor}`,
          paddingLeft: 10,
          marginBottom: 12,
          background: "#0d1117",
          borderRadius: "0 6px 6px 0",
          padding: "8px 10px",
        }}
      >
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center", marginBottom: 4 }}>
          <span style={{ fontWeight: 700, fontSize: 12, color: healthColor }}>
            {operatorHealth.status}
          </span>
          <DeskChip tone={workerEnabled && !workerStale ? "success" : workerEnabled ? "warning" : "default"}>
            Worker: {workerEnabled ? (workerStale ? "STALE" : "ACTIVE") : "BROWSER"}
          </DeskChip>
          <DeskChip tone={(stats.balanceDriftUsd ?? 0) > 1 ? "warning" : probeDominant ? "warning" : "default"}>
            State: {(stats.balanceDriftUsd ?? 0) > 1 ? "DRIFT" : probeDominant ? "PROBE-DOM" : "CLEAN"}
          </DeskChip>
          <DeskChip tone={(stats.deskLastRegimeTag ?? "").includes("chop") ? "warning" : "success"}>
            Market: {(stats.deskLastRegimeTag ?? "unknown").toUpperCase()}
          </DeskChip>
          {activeRotationReport && (
            <DeskChip>
              Rot: {activeRotationReport.active.length}A / {activeRotationReport.promoted.length}P / {activeRotationReport.suspended.length}S / {activeRotationReport.insufficient.length}⏳
            </DeskChip>
          )}
        </div>
        <p style={{ fontSize: 11, color: "#c9d1d9", margin: 0, lineHeight: 1.4 }}>
          {operatorHealth.nextAction}
        </p>
        {operatorHealth.blockers.length > 0 && (
          <ul style={{ margin: "4px 0 0", paddingLeft: 16, fontSize: 10, color: "#f85149" }}>
            {operatorHealth.blockers.map((b) => <li key={b}>{b}</li>)}
          </ul>
        )}
      </div>

      <div style={{ borderLeft: `3px solid ${readinessColor}`, paddingLeft: 12 }}>
      <div
        onClick={() => setCardExpanded((v) => !v)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setCardExpanded((v) => !v);
          }
        }}
        role="button"
        tabIndex={0}
        style={{ cursor: "pointer" }}
      >
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
            gap: 8,
            alignItems: "center",
            marginBottom: 8,
          }}
        >
          <span style={{ fontSize: 18, fontWeight: 800, color: readinessColor }}>
            {unifiedReadinessLabel(stats.unifiedReadiness)}
          </span>
          <DeskChip tone={profitModeCfg.enabled ? "success" : "default"}>
            Profit mode {profitModeCfg.enabled ? "ON" : "OFF"}
          </DeskChip>
          <DeskChip tone="primary">Regime {stats.deskLastRegimeTag}</DeskChip>
          <DeskChip>Signal {stats.effectiveSignalThreshold}</DeskChip>
          {last50 ? (
            <DeskChip>
              50: fee/gross {last50.feePctOfAbsGross.toFixed(1)}% · E ${last50.expectancy.toFixed(2)}
            </DeskChip>
          ) : null}
          <DeskChip tone={stats.soakSummary.greenDays >= 7 ? "success" : "warning"}>
            Soak {stats.soakSummary.greenDays}/7 green
          </DeskChip>
        </div>

        {(blockers.length > 0 || stats.unifiedReadinessNextStep) && (
          <div style={{ fontSize: 11, color: "#c9d1d9", marginBottom: 8, maxWidth: "100%" }}>
            {blockerPreview.length > 0 ? (
              <ul
                style={{
                  margin: 0,
                  paddingLeft: 18,
                  color: "#f85149",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                }}
              >
                {blockerPreview.map((b) => (
                  <li
                    key={b}
                    style={{
                      whiteSpace: "nowrap",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      maxWidth: "min(100%, 720px)",
                    }}
                  >
                    {b}
                  </li>
                ))}
              </ul>
            ) : null}
            {blockers.length > 2 ? (
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setBlockersExpanded((v) => !v);
                }}
                style={{
                  background: "none",
                  border: "none",
                  color: "#58a6ff",
                  cursor: "pointer",
                  fontSize: 10,
                  padding: 0,
                  marginTop: 4,
                }}
              >
                {blockersExpanded ? "Show less" : `Show all ${blockers.length} blockers`}
              </button>
            ) : null}
            {blockersExpanded && blockerRest.length > 0 ? (
              <ul style={{ margin: "4px 0 0", paddingLeft: 18, color: "#f85149" }}>
                {blockerRest.map((b) => (
                  <li key={b}>{b}</li>
                ))}
              </ul>
            ) : null}
            <p
              style={{
                margin: "6px 0 0",
                lineHeight: 1.4,
                overflow: "hidden",
                display: "-webkit-box",
                WebkitLineClamp: 2,
                WebkitBoxOrient: "vertical",
              }}
            >
              {stats.unifiedReadinessNextStep}
            </p>
          </div>
        )}

        <div
          style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: cardExpanded ? 12 : 0 }}
          onClick={(e) => e.stopPropagation()}
        >
          {showEnvFix ? (
            <DeskButton
              variant="outlined"
              style={{ minHeight: 32, fontSize: "0.75rem" }}
              onClick={() => {
                void navigator.clipboard.writeText(formatScorecardActionEnv(action!)).then(() => {
                  console.info("[UI] Copied to clipboard");
                });
              }}
            >
              Copy env fix
            </DeskButton>
          ) : null}
          <DeskButton
            variant="outlined"
            style={{ minHeight: 32, fontSize: "0.75rem" }}
            disabled={!goLiveGates}
            onClick={copyValidationReport}
          >
            Copy validation report
          </DeskButton>
          {winnersSuggestion ? (
            <DeskButton
              variant="outlined"
              style={{ minHeight: 32, fontSize: "0.75rem" }}
              onClick={copyWinnersEnv}
            >
              Copy roster env
            </DeskButton>
          ) : null}
        </div>
      </div>

      {cardExpanded ? (
        <>
          {advancedTabGated && !showAdvancedTab ? (
            <DeskButton
              variant="text"
              style={{ minHeight: 32, marginBottom: 8, fontSize: "0.75rem" }}
              onClick={() => {
                setShowAdvancedTab(true);
                setActiveTab("advanced");
              }}
            >
              Show Advanced tab →
            </DeskButton>
          ) : null}

          <DeskTabs
            items={tabItems}
            active={activeTab}
            onChange={setActiveTab}
            variant="secondary"
          />

          <div style={{ marginTop: 12 }} role="tabpanel">
            {activeTab === "today" ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
                <SoakTrendPanel
                  variant="tableOnly"
                  unifiedState={stats.unifiedReadiness}
                  blockers={[]}
                  nextStep=""
                  soakHistory={stats.soakHistory}
                  soakSummary={stats.soakSummary}
                  replaySignFlipRate={replaySignFlipRate}
                  accountKey={cloudAccountKey}
                />
                <ScorecardActionPanel
                  embedded
                  scorecard={scorecard}
                  action={action}
                />
                <DeskPnLScorecardPanel
                  embedded
                  scorecard={scorecard}
                  profitMode={profitModeCfg}
                  profitModeSkipCount={stats.profitModeSkipCount ?? 0}
                  sessionGateOn={profitModeSessionGateEnabled(profitModeCfg)}
                  allocationByEdgeOn={profitModeAllocationByEdgeEnabled(profitModeCfg)}
                />
              </div>
            ) : null}

            {activeTab === "health" ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                <DeskHealthBadge health={stats.rollingHealthCheck ?? healthCheck ?? null} />
                <GoLiveGatesPanel report={goLiveGates} onCopyReport={copyValidationReport} />
                <StrategyRotationSummary report={rotationReport ?? stats.rotationReport} />
              </div>
            ) : null}

            {activeTab === "gates" ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                <SignalQualityPanel
                  quality={stats.lastSignalQuality ?? null}
                  skipCount={stats.qualitySkipCount ?? 0}
                />
                <MTFConfluencePanel
                  result={stats.lastMtfConfluence ?? null}
                  skipCount={stats.mtfSkipCount ?? 0}
                />
                {topSkips.length > 0 ? (
                  <div>
                    <p className="desk-label-md" style={{ marginBottom: 6 }}>
                      Top skip reasons (session)
                    </p>
                    <table style={{ width: "100%", fontSize: 10, borderCollapse: "collapse" }}>
                      <tbody>
                        {topSkips.map((row) => (
                          <tr key={row.reason} style={{ borderTop: "1px solid #21262d" }}>
                            <td style={{ color: "#c9d1d9" }}>{row.reason}</td>
                            <td style={{ textAlign: "right", color: "#8b949e" }}>{row.count}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : null}
                <DeskMetricTile
                  compact
                  label="Profit-mode skips"
                  value={String(stats.profitModeSkipCount ?? 0)}
                  detail={
                    stats.gateEvaluationCount
                      ? `of ${stats.gateEvaluationCount} evals`
                      : undefined
                  }
                />
                {profitModeCfg.enabled && (
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginTop: 4 }}>
                    <DeskChip
                      tone={
                        (stats.entryRotationBlockedCount ?? 0) > 0 &&
                        (stats.entryRotationAllowedCount ?? 0) === 0
                          ? "warning"
                          : "default"
                      }
                      title="Strategies allowed through rotation gate this session"
                    >
                      Rotation allowed: {stats.entryRotationAllowedCount ?? 0}
                    </DeskChip>
                    <DeskChip
                      tone={(stats.entryRotationBlockedCount ?? 0) > 0 ? "warning" : "default"}
                      title="Strategies blocked by rotation gate (PROBATION/SUSPENDED, not INSUFFICIENT)"
                    >
                      Rotation blocked: {stats.entryRotationBlockedCount ?? 0}
                    </DeskChip>
                  </div>
                )}
              </div>
            ) : null}

            {activeTab === "signals" ? (
              <>
                <SignalTracePanel accountKey={cloudAccountKey} />
                <div style={{ marginTop: 16 }}>
                  <VerificationTrackPanel accountKey={cloudAccountKey} />
                </div>
              </>
            ) : null}

            {activeTab === "edge" ? (
              <EdgeLabPanel />
            ) : null}

            {activeTab === "oms" ? (
              <PaperOmsPanel />
            ) : null}

            {activeTab === "advanced" && showAdvancedTab ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                <DeskMonitorPanel
                  monitorState={monitor}
                  accountKey={cloudAccountKey}
                  skipReasonSummary={stats.skipReasonSummary}
                  runtimeBlocklist={stats.runtimeBlocklistIds}
                  signalThreshold={stats.effectiveSignalThreshold}
                  leverage={25}
                  takerFeePct={0.001}
                  maxSameSide={2}
                  minNotional={100}
                  openPositionCount={stats.openPositions}
                  currentRegime={stats.deskLastRegimeTag}
                  mongoConnected={Boolean(cloudAccountKey)}
                  currentTpPct={1.5}
                  currentSlPct={0.5}
                  onRestoreRotationStrategy={onRestoreRotationStrategy}
                  attributionReport={stats.attributionReport}
                  rotationReport={stats.rotationReport}
                  qualitySkipCount={stats.qualitySkipCount}
                  mtfSkipCount={stats.mtfSkipCount}
                  gateEvaluationCount={stats.gateEvaluationCount}
                  engineDeskRecommendation={stats.deskRecommendation}
                  strategyDiagnostics={stats.strategyDiagnostics}
                  unifiedReadiness={stats.unifiedReadiness}
                  unifiedReadinessBlockers={stats.unifiedReadinessBlockers}
                  unifiedReadinessNextStep={stats.unifiedReadinessNextStep}
                  soakHistory={stats.soakHistory}
                  engineReplaySignFlipRate={stats.replaySignFlipRate}
                  hideGoLiveGates
                  hideStrategyRotation
                  hideWinnersSuggestion
                  compactRecommendations
                />
                <StrategyRotationPanel
                  report={rotationReport ?? stats.rotationReport}
                  onRestore={onRestoreRotationStrategy}
                />
                <AttributionPanel report={stats.attributionReport ?? null} />
                {winnersSuggestion ? (
                  <div
                    style={{
                      border: "1px solid #21262d",
                      borderRadius: 8,
                      padding: "8px 12px",
                      fontSize: 11,
                      background: "#161b22",
                    }}
                  >
                    <div style={{ fontWeight: 600, marginBottom: 4, color: "#58a6ff" }}>
                      Suggested roster (read-only)
                    </div>
                    <p style={{ fontSize: 10, color: "#8b949e", marginBottom: 6 }}>
                      {winnersSuggestion.rationale}
                    </p>
                    <pre
                      style={{
                        fontSize: 9,
                        fontFamily: "var(--desk-font-mono, monospace)",
                        background: "#0d1117",
                        padding: 6,
                        borderRadius: 4,
                        color: "#8b949e",
                        overflow: "auto",
                        maxWidth: "100%",
                      }}
                    >
                      {winnersSuggestion.strategyIdsEnvLine}
                    </pre>
                    <DeskButton
                      variant="outlined"
                      style={{ minHeight: 32, marginTop: 6, fontSize: "0.75rem" }}
                      onClick={copyWinnersEnv}
                    >
                      Copy roster env
                    </DeskButton>
                  </div>
                ) : null}
                {/* ── Strategy Safety Suggestions ── */}
                {strategySafety && (
                  <div
                    style={{
                      border: "1px solid #21262d",
                      borderRadius: 8,
                      padding: "8px 12px",
                      fontSize: 11,
                      background: "#161b22",
                    }}
                  >
                    <div style={{ fontWeight: 600, marginBottom: 4, color: "#d29922" }}>
                      Suggested safety actions (read-only)
                    </div>
                    <p style={{ fontSize: 10, color: "#8b949e", marginBottom: 6 }}>
                      Disable candidates have ≥5 production closes with negative expectancy AND fee/gross &gt;100%, or rotation SUSPENDED. Never auto-applied.
                    </p>
                    {strategySafety.autoDisableCandidates.length === 0 ? (
                      <p style={{ fontSize: 10, color: "#3fb950" }}>No disable candidates — all strategies look safe.</p>
                    ) : (
                      <>
                        <p style={{ fontSize: 10, color: "#f85149", marginBottom: 4 }}>
                          Consider disabling: {strategySafety.autoDisableCandidates.join(", ")}
                        </p>
                        <ul style={{ paddingLeft: 16, margin: "0 0 6px", fontSize: 10, color: "#c9d1d9" }}>
                          {strategySafety.autoDisableCandidates.map((id) => (
                            <li key={id}>#{id}: {strategySafety.reasonByStrategyId[id] ?? "see rotation report"}</li>
                          ))}
                        </ul>
                        {disableListEnv && (
                          <>
                            <pre
                              style={{
                                fontSize: 9,
                                fontFamily: "var(--desk-font-mono, monospace)",
                                background: "#0d1117",
                                padding: 6,
                                borderRadius: 4,
                                color: "#8b949e",
                                overflow: "auto",
                                maxWidth: "100%",
                              }}
                            >
                              {disableListEnv}
                            </pre>
                            <DeskButton
                              variant="outlined"
                              style={{ minHeight: 28, marginTop: 4, fontSize: "0.7rem" }}
                              onClick={() => {
                                void navigator.clipboard.writeText(disableListEnv);
                              }}
                            >
                              Copy disable env
                            </DeskButton>
                          </>
                        )}
                      </>
                    )}
                  </div>
                )}

                {/* ── AI App Tracker ── */}
                <AiAppTrackerPanel
                  workerStatus={
                    workerEnabled
                      ? workerStale
                        ? `STALE (${stats.workerLastPollAt ? Math.round((Date.now() - stats.workerLastPollAt) / 1000) : "?"}s ago)`
                        : "LIVE"
                      : "BROWSER"
                  }
                  dominantBlocker={
                    (entryDebug?.dominantBlocker ?? "unknown").toLowerCase()
                  }
                />

                {/* ── Deployment Sanity ── */}
                <div
                  style={{
                    border: `1px solid ${deploymentStale ? "#d29922" : "#21262d"}`,
                    borderRadius: 8,
                    padding: "8px 12px",
                    fontSize: 11,
                    background: "#161b22",
                  }}
                >
                  <div style={{ fontWeight: 600, marginBottom: 6, color: deploymentStale ? "#d29922" : "#58a6ff" }}>
                    Deployment Sanity
                  </div>
                  <table style={{ width: "100%", fontSize: 10, borderCollapse: "collapse" }}>
                    <tbody>
                      {[
                        ["UI build SHA", BTC_FT_DESK_BUILD],
                        ["API build SHA", serverBuildSha?.slice(0, 7) ?? "unknown"],
                        ["SHA match", deploymentStale ? "MISMATCH ⚠️" : "OK"],
                        ["Worker owner", stats.workerOwner ?? "none"],
                        ["Worker last tick", stats.workerLastPollAt ? `${Math.round((Date.now() - stats.workerLastPollAt) / 1000)}s ago` : "never"],
                        ["MongoDB", cloudAccountKey ? "configured" : "not configured"],
                        ["Profit mode", profitModeCfg.enabled ? "ON" : "OFF"],
                        ["Workers enabled", workerEnabled ? "YES" : "NO"],
                        ["Account key", cloudAccountKey ? `…${cloudAccountKey.slice(-4)}` : "not set"],
                      ].map(([label, value]) => (
                        <tr key={label} style={{ borderTop: "1px solid #21262d" }}>
                          <td style={{ color: "#8b949e", paddingRight: 8, paddingTop: 3, paddingBottom: 3 }}>{label}</td>
                          <td style={{ color: "#c9d1d9", fontFamily: "var(--desk-font-mono, monospace)" }}>{value}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {deploymentStale && (
                    <p style={{ fontSize: 10, color: "#d29922", marginTop: 6 }}>
                      UI/API SHA mismatch: redeploy Vercel and run <code>pm2 restart btc-ft-worker</code> before debugging strategy behavior.
                    </p>
                  )}
                </div>

                {/* ── Worker Events ── */}
                <details style={{ marginTop: 4 }}>
                  <summary
                    style={{
                      cursor: "pointer",
                      fontSize: 11,
                      color: "#58a6ff",
                      padding: "6px 0",
                      listStyle: "none",
                    }}
                  >
                    Worker Events (last 50)
                  </summary>
                  <div style={{ marginTop: 6 }}>
                    {workerEvents.length === 0 ? (
                      <p style={{ fontSize: 10, color: "#8b949e" }}>No events loaded yet (open this panel to fetch).</p>
                    ) : (
                      <table style={{ width: "100%", fontSize: 9, borderCollapse: "collapse", fontFamily: "var(--desk-font-mono, monospace)" }}>
                        <thead>
                          <tr style={{ color: "#8b949e" }}>
                            <th style={{ textAlign: "left", paddingBottom: 4 }}>Time</th>
                            <th style={{ textAlign: "left", paddingBottom: 4 }}>Type</th>
                            <th style={{ textAlign: "left", paddingBottom: 4 }}>Message</th>
                          </tr>
                        </thead>
                        <tbody>
                          {workerEvents.map((ev, i) => (
                            <tr
                              key={i}
                              style={{
                                borderTop: "1px solid #21262d",
                                color: ev.severity === "error" ? "#f85149" : ev.severity === "warning" ? "#d29922" : "#c9d1d9",
                              }}
                            >
                              <td style={{ paddingRight: 8, whiteSpace: "nowrap", paddingTop: 2, paddingBottom: 2 }}>
                                {new Date(ev.created_at).toLocaleTimeString()}
                              </td>
                              <td style={{ paddingRight: 8, whiteSpace: "nowrap" }}>{ev.type}</td>
                              <td style={{ wordBreak: "break-word", maxWidth: 240 }}>{ev.message}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>
                </details>

                <ShadowIntentLogPanel
                  enabled={deskShadowIntentsEnabled}
                  signedIn={Boolean(cloudAccountKey)}
                />
                {deskTestnetOpsEnabled ? <TestnetOpsPanel /> : null}
                <details style={{ marginTop: 8 }}>
                  <summary
                    style={{
                      cursor: "pointer",
                      fontSize: 11,
                      color: "var(--desk-readiness-collect-data, #8b949e)",
                      padding: "6px 0",
                      listStyle: "none",
                    }}
                  >
                    Developer — entry poll debug
                  </summary>
                  <div style={{ marginTop: 8 }}>
                    <EntryDebugPanel
                      forceVisible
                      profitModeEnabled={profitModeCfg.enabled}
                      entryDebug={entryDebug ?? null}
                      pauseEntries={pauseEntries}
                      drawdownLocked={stats.isDrawdownLocked}
                      sessionSkips={{
                        minMove: stats.deskSkippedMinExpectedMove,
                        regime: stats.deskSkippedByRegime,
                        spread: stats.deskSkippedSpread,
                        session: stats.deskSkippedOutsideSession,
                        category: stats.deskSkippedCategoryCap,
                        lowPriority: stats.deskSkippedLowPriorityEntry,
                        regimeBreakdown: stats.deskSkippedByRegimeBreakdown,
                      }}
                    />
                  </div>
                </details>
              </div>
            ) : null}
          </div>
        </>
      ) : null}
      </div>
    </DeskCard>
  );
}
