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
import type { DeskEntryPollDebug } from "@/lib/futuresEntryDebug";

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
    <div style={{ fontSize: 11, color: "#e6edf3" }}>
      <p className="desk-label-md" style={{ marginBottom: 8 }}>
        Suspended: {report.suspended.length} · Promoted: {report.promoted.length} · Active:{" "}
        {report.active.length}
      </p>
      {top.length > 0 ? (
        <table style={{ width: "100%", fontSize: 10, borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ color: "#8b949e", textAlign: "left" }}>
              <th>Strategy</th>
              <th>Status</th>
              <th>Score</th>
            </tr>
          </thead>
          <tbody>
            {top.map((row) => (
              <tr key={row.strategyId} style={{ borderTop: "1px solid #21262d" }}>
                <td>
                  #{row.strategyId} {row.strategyName}
                </td>
                <td>{row.status}</td>
                <td>{row.score.toFixed(0)}</td>
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
}: DeskCommandCenterProps) {
  const [cardExpanded, setCardExpanded] = useState(true);
  const [activeTab, setActiveTab] = useState<DeskCommandCenterTab>(DEFAULT_COMMAND_CENTER_TAB);
  const [showAdvancedTab, setShowAdvancedTab] = useState(!advancedTabGated);
  const [blockersExpanded, setBlockersExpanded] = useState(false);

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
    ];
    if (showAdvancedTab) {
      base.push({ key: "advanced", label: commandCenterTabLabel("advanced") });
    }
    return base;
  }, [recommendations.length, showAdvancedTab]);

  const blockerPreview = blockers.slice(0, 2);
  const blockerRest = blockers.length > 2 ? blockers.slice(2) : [];

  return (
    <DeskCard className="desk-command-center" padding="md">
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
