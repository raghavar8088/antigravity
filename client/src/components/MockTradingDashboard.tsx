"use client";

import { useEffect, useMemo, useState } from "react";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockCandleBuilder } from "@/hooks/useMockCandleBuilder";
import { useMockResearchRunner } from "@/hooks/useMockResearchRunner";
import { useMockTradingEngine } from "@/hooks/useMockTradingEngine";
import { useMarketRegime } from "@/hooks/useMarketRegime";
import { useStrategyScoring } from "@/hooks/useStrategyScoring";
import { useTerminalLayout } from "@/hooks/useTerminalLayout";
import dynamic from "next/dynamic";

const MockResearchChartsPanel = dynamic(() => import("@/components/MockResearchChartsPanel").then(mod => mod.MockResearchChartsPanel), {
  loading: () => <div style={{ height: 400, background: "rgba(48, 54, 61, 0.1)", borderRadius: 8 }} />,
  ssr: false,
});

const PineEditorPanel = dynamic(() => import("@/components/PineEditorPanel").then(mod => mod.PineEditorPanel), {
  loading: () => <div style={{ height: 300, background: "rgba(48, 54, 61, 0.1)", borderRadius: 8 }} />,
  ssr: false,
});

const StrategyLeaderboard = dynamic(() => import("@/components/StrategyLeaderboard").then(mod => mod.StrategyLeaderboard), {
  loading: () => <div style={{ height: 300, background: "rgba(48, 54, 61, 0.1)", borderRadius: 8 }} />,
  ssr: false,
});

const MockEquityCurvePanel = dynamic(() => import("@/components/MockEquityCurvePanel").then(mod => mod.MockEquityCurvePanel), {
  loading: () => <div style={{ height: 400, background: "rgba(48, 54, 61, 0.1)", borderRadius: 8 }} />,
  ssr: false,
});

const MockMonteCarloPanel = dynamic(() => import("@/components/MockMonteCarloPanel").then(mod => mod.MockMonteCarloPanel), {
  loading: () => <div style={{ height: 400, background: "rgba(48, 54, 61, 0.1)", borderRadius: 8 }} />,
  ssr: false,
});

const MockStrategyLeaderboardPanel = dynamic(() => import("@/components/MockStrategyLeaderboardPanel").then(mod => mod.MockStrategyLeaderboardPanel), {
  loading: () => <div style={{ height: 400, background: "rgba(48, 54, 61, 0.1)", borderRadius: 8 }} />,
  ssr: false,
});

const MockRiskAnalyticsPanel = dynamic(() => import("@/components/MockRiskAnalyticsPanel").then(mod => mod.MockRiskAnalyticsPanel), {
  loading: () => <div style={{ height: 400, background: "rgba(48, 54, 61, 0.1)", borderRadius: 8 }} />,
  ssr: false,
});

import {
  TerminalPanel,
  TerminalWorkspace,
  InstitutionalChart,
} from "@/components/terminal";
import { M3AppShell } from "@/components/ui/M3AppShell";
import { StatusChip } from "@/components/ui/StatusChip";
import {
  DeskBanner,
  DeskButton,
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSectionHeader,
  type DeskColumn,
} from "@/components/desk/ui";
import {
  computeAnalytics,
  emptyMockDiagnosticFunnel,
  emptyMockRejectionCounts,
  filterMockTrades,
  mockTradeAgeMinutes,
  MOCK_REJECTION_CATEGORIES,
  MOCK_TRADE_SORT_OPTIONS,
  sortMockTrades,
  type MockAccountState,
  type MockTradeAgeFilterMode,
  type MockSide,
  type MockSizingMode,
  type MockTrade,
  type MockTradeFilter,
  type MockTradeSortKey,
  type MockTradeStatus,
  type MockTradingConfig,
  type MockDiagnosticFunnel,
  type MockRejectionCounts,
  type MockTradingDiagnostics,
} from "@/lib/trading/mockTradingEngine";
import { ALL_RESEARCH_FAMILIES, type ResearchFamily } from "@/lib/ai/mockResearchStrategies";
import {
  fmtUsd,
  fmtUsdK,
  fmtPrice,
  pnlClass,
  fmtPct,
  exportToCsv,
  exportToJson,
} from "@/lib/utils/utils";
import {
  computeAdvancedResearchAnalytics,
  computeDailyPnlPoints,
  computeMonthlyPnl,
  computeWeeklyPnl,
  createEquitySnapshot,
  type AdvancedResearchAnalytics,
} from "@/lib/ai/mockResearchAnalytics";
import { computeMockWalkForwardRows, type MockWalkForwardRow } from "@/lib/ai/mockResearchWalkForward";
import { computePortfolioAllocation, type PortfolioAllocationResult } from "@/lib/ai/mockResearchPortfolioAllocation";
import { computeStrategyHealth, type StrategyHealthRow } from "@/lib/ai/strategyHealthEngine";
import type { StrategyScore } from "@/lib/ai/strategyScoringEngine";

const selectStyle: React.CSSProperties = {
  padding: "4px 8px",
  borderRadius: 4,
  border: "1px solid var(--border)",
  background: "var(--surface-2)",
  color: "var(--text-primary)",
  fontSize: "0.75rem",
};

function addCounts(a: MockRejectionCounts, b: MockRejectionCounts): MockRejectionCounts {
  const next = emptyMockRejectionCounts();
  for (const key of MOCK_REJECTION_CATEGORIES) {
    next[key] = a[key] + b[key];
  }
  return next;
}

function dominantRejection(counts: MockRejectionCounts): { reason: string; count: number } | null {
  const [reason, count] = Object.entries(counts).sort((a, b) => b[1] - a[1])[0] ?? [];
  return reason && count > 0 ? { reason, count } : null;
}

function combineDiagnostics(
  researchFunnel: MockDiagnosticFunnel,
  engineFunnel: MockDiagnosticFunnel,
): MockDiagnosticFunnel {
  return {
    signalsGenerated: researchFunnel.signalsGenerated > 0 ? researchFunnel.signalsGenerated : engineFunnel.signalsGenerated,
    confidencePassed: researchFunnel.confidencePassed > 0 ? researchFunnel.confidencePassed : engineFunnel.confidencePassed,
    scorePassed: engineFunnel.scorePassed,
    riskRewardPassed: engineFunnel.riskRewardPassed,
    riskPassed: engineFunnel.riskPassed,
    cooldownPassed: engineFunnel.cooldownPassed,
    familyConflictPassed: engineFunnel.familyConflictPassed,
    exposurePassed: engineFunnel.exposurePassed,
    tradesCreated: engineFunnel.tradesCreated,
  };
}

function fmtAge(openedAt: number, ref: number = Date.now()): string {
  const secs = Math.max(0, Math.floor((ref - openedAt) / 1000));
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  return `${hrs}h ${mins % 60}m`;
}

function fmtTradeAge(trade: MockTrade, ref: number = Date.now()): string {
  const ageMs = mockTradeAgeMinutes(trade, ref) * 60_000;
  return fmtAge(0, ageMs);
}

function parseMinuteInput(value: string): number | null {
  if (value.trim() === "") return null;
  const next = Number(value);
  return Number.isFinite(next) && next >= 0 ? next : null;
}

function safeRatio(value: number, max: number): number {
  if (!Number.isFinite(value) || !Number.isFinite(max) || max <= 0) return 0;
  return value / max;
}

function logColor(event: string): string {
  if (event === "MOCK_TRADE_CREATED") return "var(--desk-success)";
  if (event === "MOCK_TRADE_TP_HIT") return "var(--desk-success)";
  if (event === "MOCK_TRADE_SL_HIT") return "var(--desk-error)";
  if (event === "MOCK_TRADE_REJECTED") return "var(--desk-warning)";
  if (event === "MOCK_TRADE_LIMIT_REACHED") return "var(--desk-warning)";
  return "var(--desk-primary)";
}

export const MOCK_TRADE_TABLE_REQUIRED_HEADERS = [
  "Current PnL",
  "TP Profit $",
  "SL Loss $",
  "Exit Reason",
] as const;

function exitReasonBadge(reason: MockTrade["exitReason"]) {
  if (!reason) return <span style={{ color: "var(--desk-on-surface-variant)" }}>—</span>;
  const tone =
    reason === "TAKE_PROFIT"
      ? "success"
      : reason === "STOP_LOSS"
        ? "error"
        : reason === "MAX_HOLD"
          ? "warning"
          : "default";
  return <DeskChip tone={tone}>{reason}</DeskChip>;
}

function MarketRegimePanel({
  snapshot,
  onMaximize,
  isMaximized,
  onClose,
}: {
  snapshot: ReturnType<typeof useMarketRegime>["snapshot"];
  onMaximize?: () => void;
  isMaximized?: boolean;
  onClose?: () => void;
}) {
  return (
    <TerminalPanel
      title="Market Regime"
      subtitle="BTC OHLCV classification"
      onMaximize={onMaximize}
      isMaximized={isMaximized}
      onClose={onClose}
    >
      {snapshot == null ? (
        <DeskEmptyState title="Waiting for history" subtitle="Classification starts after enough candles." />
      ) : (
        <>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 12 }}>
            <DeskChip tone={regimeTone(snapshot.regime)}>{snapshot.regime}</DeskChip>
            <div style={{ flex: 1, height: 6, borderRadius: 999, background: "var(--surface-3)", overflow: "hidden" }}>
              <div
                style={{
                  width: `${Math.max(0, Math.min(100, snapshot.confidence))}%`,
                  height: "100%",
                  background: "var(--accent)",
                }}
              />
            </div>
            <span style={{ fontSize: 10, color: "var(--text-secondary)" }}>
              {snapshot.confidence.toFixed(0)}%
            </span>
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 8 }}>
            <DeskMetricTile label="ADX" value={snapshot.adx.toFixed(1)} compact />
            <DeskMetricTile label="ATR %" value={`${snapshot.atrPct.toFixed(3)}%`} compact />
            <DeskMetricTile label="BB width %" value={`${snapshot.bbWidthPct.toFixed(3)}%`} compact />
            <DeskMetricTile label="EMA slope" value={`${(snapshot.emaSlope * 100).toFixed(3)}%`} compact />
          </div>
        </>
      )}
    </TerminalPanel>
  );
}

function ResearchLabOverviewContent({
  scoring,
  research,
}: {
  scoring: ReturnType<typeof useStrategyScoring>;
  research: ReturnType<typeof useMockResearchRunner>;
}) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 10 }}>
        <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
          <span className="desk-label-md" style={{ fontSize: 10 }}>Selection mode</span>
          <select
            value={research.config.selectionMode}
            onChange={(event) =>
              research.setConfig({
                ...research.config,
                selectionMode: event.target.value as typeof research.config.selectionMode,
              })
            }
            style={{ ...selectStyle, width: "100%", minWidth: 0, fontSize: 11 }}
          >
            <option value="RESEARCH_MODE">Research</option>
            <option value="PROFIT_MODE">Profit</option>
            <option value="REGIME_MODE">Regime</option>
          </select>
        </label>
        <ConfigNumber
          label="Top N"
          value={research.config.topN}
          step={1}
          onChange={(value) =>
            research.setConfig({
              ...research.config,
              topN: Math.max(1, Math.min(50, Math.floor(value))),
            })
          }
        />
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 10 }}>
        <DeskMetricTile label="Ranked" value={scoring.scores.length} compact />
        <DeskMetricTile
          label="Approved"
          value={
            research.config.selectionMode === "REGIME_MODE"
              ? scoring.approvedRegimeIds.size
              : scoring.approvedProfitIds.size
          }
          compact
        />
      </div>
    </div>
  );
}

function TradeHistoryFilters({
  filter,
  setFilter,
  strategyOptions,
  blockerOptions,
  familyOptions,
  research,
  sortKey,
  setSortKey,
  displayPageSize,
  setDisplayPageSize,
  strategySearch,
  setStrategySearch,
  showOpenOnly,
  setShowOpenOnly,
  engine,
}: {
  filter: MockTradeFilter;
  setFilter: React.Dispatch<React.SetStateAction<MockTradeFilter>>;
  strategyOptions: [number, string][];
  blockerOptions: string[];
  familyOptions: string[];
  research: ReturnType<typeof useMockResearchRunner>;
  sortKey: MockTradeSortKey;
  setSortKey: (k: MockTradeSortKey) => void;
  displayPageSize: number;
  setDisplayPageSize: (s: number) => void;
  strategySearch: string;
  setStrategySearch: (s: string) => void;
  showOpenOnly: boolean;
  setShowOpenOnly: (s: boolean) => void;
  engine: ReturnType<typeof useMockTradingEngine>;
}) {
  return (
    <div
      style={{
        display: "flex",
        flexWrap: "wrap",
        gap: 8,
        alignItems: "center",
        marginBottom: 12,
      }}
    >
      <select
        value={filter.strategyId ?? ""}
        onChange={(e) =>
          setFilter((f) => ({
            ...f,
            strategyId: e.target.value ? Number(e.target.value) : null,
          }))
        }
        style={{ ...selectStyle, minWidth: 110 }}
      >
        <option value="">All strats</option>
        {strategyOptions.map(([id, name]) => (
          <option key={id} value={id}>
            #{id} {name}
          </option>
        ))}
      </select>

      <select
        value={filter.side ?? ""}
        onChange={(e) => setFilter((f) => ({ ...f, side: (e.target.value || null) as MockSide | null }))}
        style={{ ...selectStyle, minWidth: 90 }}
      >
        <option value="">All sides</option>
        <option value="BUY">BUY</option>
        <option value="SELL">SELL</option>
      </select>

      <select
        value={filter.status ?? ""}
        onChange={(e) =>
          setFilter((f) => ({ ...f, status: (e.target.value || null) as MockTradeStatus | null }))
        }
        style={{ ...selectStyle, minWidth: 90 }}
      >
        <option value="">All status</option>
        <option value="OPEN">OPEN</option>
        <option value="CLOSED">CLOSED</option>
      </select>

      <select
        value={sortKey}
        onChange={(e) => setSortKey(e.target.value as MockTradeSortKey)}
        style={{ ...selectStyle, minWidth: 130 }}
        aria-label="Sort trades"
      >
        {MOCK_TRADE_SORT_OPTIONS.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>

      <input
        type="search"
        value={strategySearch}
        onChange={(e) => setStrategySearch(e.target.value)}
        placeholder="Search..."
        style={{ ...selectStyle, minWidth: 120 }}
      />

      <label style={{ display: "flex", gap: 4, alignItems: "center", fontSize: 11 }}>
        <input
          type="checkbox"
          checked={showOpenOnly}
          onChange={(e) => setShowOpenOnly(e.target.checked)}
        />
        Open
      </label>

      <div style={{ marginLeft: "auto", display: "flex", gap: 6 }}>
        <DeskButton
          variant="outlined"
          onClick={() => {
            setFilter({});
            setStrategySearch("");
          }}
          style={{ minHeight: 28, fontSize: "0.75rem" }}
        >
          Clear
        </DeskButton>
        <DeskButton
          variant="danger-tonal"
          onClick={engine.reset}
          style={{ minHeight: 28, fontSize: "0.75rem" }}
        >
          Reset
        </DeskButton>
      </div>
    </div>
  );
}

function AccountSummaryCard({
  account,
  maxOpenMockTrades,
  openUsagePct,
  onMaximize,
  isMaximized,
  onClose,
}: {
  account: MockAccountState;
  maxOpenMockTrades: number;
  openUsagePct: number;
  onMaximize?: () => void;
  isMaximized?: boolean;
  onClose?: () => void;
}) {
  const equityClass = pnlClass(account.equity - account.startingBalance);
  const realizedClass = pnlClass(account.realizedPnl);
  const unrealClass = pnlClass(account.unrealizedPnl);
  const returnClass = pnlClass(account.returnPct);
  return (
    <TerminalPanel
      title="Account Summary"
      subtitle="Equity, margin and exposure"
      onMaximize={onMaximize}
      isMaximized={isMaximized}
      onClose={onClose}
    >
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(2, 1fr)",
          gap: 10,
        }}
      >
        <DeskMetricTile label="Starting" value={fmtUsd(account.startingBalance, 0)} compact />
        <DeskMetricTile label="Cash" value={fmtUsd(account.cashBalance, 0)} compact />
        <DeskMetricTile label="Equity" value={fmtUsd(account.equity, 0)} valueClassName={equityClass} compact />
        <DeskMetricTile label="Realized" value={fmtUsd(account.realizedPnl, 2)} valueClassName={realizedClass} compact />
        <DeskMetricTile label="Unrealized" value={fmtUsd(account.unrealizedPnl, 2)} valueClassName={unrealClass} compact />
        <DeskMetricTile label="Exposure" value={fmtUsd(account.exposure, 0)} compact />
        <DeskMetricTile label="Margin" value={fmtUsd(account.marginUsed, 0)} compact />
        <DeskMetricTile label="Available" value={fmtUsd(account.availableBalance, 0)} compact />
        <DeskMetricTile label="Return %" value={fmtPct(account.returnPct, 2)} valueClassName={returnClass} compact />
        <DeskMetricTile label="Max DD" value={fmtPct(account.maxDrawdownPct, 2)} compact />
      </div>
      <div style={{ marginTop: 12 }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            fontSize: 9,
            color: "var(--text-muted)",
            marginBottom: 4,
            textTransform: "uppercase",
            fontWeight: 600,
          }}
        >
          <span>Trade Limit Usage</span>
          <span>{openUsagePct.toFixed(1)}%</span>
        </div>
        <div
          style={{
            height: 4,
            borderRadius: 999,
            overflow: "hidden",
            background: "var(--surface-3)",
          }}
        >
          <div
            style={{
              width: `${Math.min(100, openUsagePct)}%`,
              height: "100%",
              background: openUsagePct >= 80 ? "var(--red)" : "var(--accent)",
            }}
          />
        </div>
      </div>
    </TerminalPanel>
  );
}

function MockTradingReadinessAuditCard({
  research,
  scoring,
  diagnostics,
  onMaximize,
  isMaximized,
  onClose,
}: {
  research: ReturnType<typeof useMockResearchRunner>;
  scoring: ReturnType<typeof useStrategyScoring>;
  diagnostics: MockTradingDiagnostics;
  onMaximize?: () => void;
  isMaximized?: boolean;
  onClose?: () => void;
}) {
  const readiness = research.readiness;
  const approvedIds =
    research.config.selectionMode === "REGIME_MODE"
      ? scoring.approvedRegimeIds
      : scoring.approvedProfitIds;
  const modeLabel =
    research.config.selectionMode === "RESEARCH_MODE"
      ? "Research"
      : research.config.selectionMode === "PROFIT_MODE"
        ? "Profit"
        : "Regime";
  const waitingRows = readiness.rows
    .filter((row) => !row.ready)
    .sort((a, b) => b.requiredCandles - a.requiredCandles || a.strategyId - b.strategyId);
  const displayedRows = waitingRows.length > 0 ? waitingRows : readiness.rows;
  const signalsGenerated = diagnostics.funnel.signalsGenerated;
  const signalsPassed = diagnostics.funnel.exposurePassed;
  const signalsFiltered = Math.max(0, signalsGenerated - signalsPassed);
  const topRejection = dominantRejection(diagnostics.rejectionCounts);
  const rootCause = firstZeroPipelineStage({
    readiness,
    researchHasRun: research.hasRun,
    selectionMode: research.config.selectionMode,
    approvedCount: approvedIds.size,
    funnel: diagnostics.funnel,
  });

  return (
    <TerminalPanel
      title="Live Data Readiness Audit"
      subtitle="Strategy candle history and approved state"
      onMaximize={onMaximize}
      isMaximized={isMaximized}
      onClose={onClose}
    >
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(120px, 1fr))", gap: 10 }}>
        <DeskMetricTile label="Total" value={readiness.totalStrategies.toLocaleString("en-US")} compact />
        <DeskMetricTile label="Ready" value={readiness.strategiesReady.toLocaleString("en-US")} compact />
        <DeskMetricTile label="Waiting" value={readiness.strategiesWaitingForHistory.toLocaleString("en-US")} compact />
        <DeskMetricTile label="Min Candles" value={readiness.minCandlesAvailable.toLocaleString("en-US")} compact />
        <DeskMetricTile label="Mode" value={modeLabel} compact />
        <DeskMetricTile label="Approved" value={approvedIds.size.toLocaleString("en-US")} compact />
        <DeskMetricTile label="Signals" value={signalsGenerated.toLocaleString("en-US")} compact />
        <DeskMetricTile label="Trades" value={diagnostics.funnel.tradesCreated.toLocaleString("en-US")} compact />
      </div>

      <div
        style={{
          marginTop: 10,
          padding: 8,
          borderRadius: 6,
          border: "1px solid var(--border)",
          background: "var(--surface-2)",
          fontSize: 10,
        }}
      >
        <div className="desk-label-md" style={{ marginBottom: 2, fontSize: 9 }}>Root-Cause Snapshot</div>
        <div style={{ color: "var(--text-secondary)" }}>{rootCause}</div>
      </div>

      <div style={{ marginTop: 10, maxHeight: 180, overflowY: "auto" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 10 }}>
          <thead>
            <tr style={{ color: "var(--text-muted)", textAlign: "left", borderBottom: "1px solid var(--border)" }}>
              <th style={{ padding: "4px 6px" }}>Strategy</th>
              <th style={{ padding: "4px 6px", textAlign: "right" }}>Req</th>
              <th style={{ padding: "4px 6px", textAlign: "right" }}>Avail</th>
              <th style={{ padding: "4px 6px" }}>Ready</th>
            </tr>
          </thead>
          <tbody>
            {displayedRows.slice(0, 20).map((row) => (
              <tr key={row.strategyId} style={{ borderBottom: "1px solid var(--border-subtle)" }}>
                <td style={{ padding: "4px 6px" }}>#{row.strategyId} {row.strategyName}</td>
                <td style={{ padding: "4px 6px", textAlign: "right" }}>{row.requiredCandles}</td>
                <td style={{ padding: "4px 6px", textAlign: "right" }}>{row.availableCandles}</td>
                <td style={{ padding: "4px 6px" }}>
                  <DeskChip tone={row.ready ? "success" : "default"} style={{ fontSize: 9, padding: "0 4px" }}>
                    {row.ready ? "YES" : "NO"}
                  </DeskChip>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TerminalPanel>
  );
}

function MockTradingDiagnosticsCard({
  diagnostics,
  latestResearch,
  onMaximize,
  isMaximized,
  onClose,
}: {
  diagnostics: MockTradingDiagnostics;
  latestResearch: { funnel: MockDiagnosticFunnel; rejectionCounts: MockRejectionCounts };
  onMaximize?: () => void;
  isMaximized?: boolean;
  onClose?: () => void;
}) {
  const funnel = diagnostics.funnel;
  const totalRejections = Object.values(diagnostics.rejectionCounts).reduce((sum, count) => sum + count, 0);
  const dominant = dominantRejection(diagnostics.rejectionCounts);
  const stageRows: Array<{ label: string; value: number }> = [
    { label: "Generated", value: funnel.signalsGenerated },
    { label: "Confidence", value: funnel.confidencePassed },
    { label: "Score", value: funnel.scorePassed },
    { label: "Risk", value: funnel.riskPassed },
    { label: "Cooldown", value: funnel.cooldownPassed },
    { label: "Exposure", value: funnel.exposurePassed },
    { label: "Created", value: funnel.tradesCreated },
  ];
  const rejectionRows = MOCK_REJECTION_CATEGORIES
    .map((reason) => ({
      reason,
      count: diagnostics.rejectionCounts[reason],
      pct: totalRejections > 0 ? diagnostics.rejectionCounts[reason] / totalRejections : 0,
    }))
    .filter((row) => row.count > 0)
    .sort((a, b) => b.count - a.count);

  return (
    <TerminalPanel
      title="Mock Trading Diagnostics"
      subtitle="Root-cause funnel for mock signals"
      onMaximize={onMaximize}
      isMaximized={isMaximized}
      onClose={onClose}
    >
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        <div>
          <div className="desk-label-md" style={{ marginBottom: 6, fontSize: 9 }}>Funnel View</div>
          <div style={{ display: "grid", gap: 4 }}>
            {stageRows.map((stage, idx) => {
              const previous = idx === 0 ? stage.value : stageRows[idx - 1]?.value ?? 0;
              const passPct = idx === 0 || previous <= 0 ? 1 : stage.value / previous;
              return (
                <div key={stage.label} style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: 8, fontSize: 10 }}>
                  <span style={{ color: "var(--text-muted)" }}>
                    {idx > 0 ? "→ " : ""}{stage.label}
                  </span>
                  <span>
                    {stage.value.toLocaleString("en-US")} · {fmtPct(passPct, 0)}
                  </span>
                </div>
              );
            })}
          </div>
        </div>

        <div>
          <div className="desk-label-md" style={{ marginBottom: 6, fontSize: 9 }}>Top Rejections</div>
          {rejectionRows.length === 0 ? (
            <div style={{ color: "var(--text-muted)", fontSize: 10 }}>No events yet.</div>
          ) : (
            <div style={{ display: "grid", gap: 4 }}>
              {rejectionRows.slice(0, 7).map((row) => (
                <div key={row.reason} style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: 8, fontSize: 10 }}>
                  <span style={{ color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{row.reason}</span>
                  <span>{row.count}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </TerminalPanel>
  );
}

function MockConfigCard({
  config,
  onChange,
  onMaximize,
  isMaximized,
  onClose,
}: {
  config: MockTradingConfig;
  onChange: (next: MockTradingConfig) => void;
  onMaximize?: () => void;
  isMaximized?: boolean;
  onClose?: () => void;
}) {
  return (
    <TerminalPanel
      title="Mock Account & Sizing"
      subtitle="Risk-checked execution costs"
      onMaximize={onMaximize}
      isMaximized={isMaximized}
      onClose={onClose}
    >
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))", gap: 10 }}>
        <ConfigNumber
          label="Balance ($)"
          value={config.startingBalanceUsd}
          step={10_000}
          onChange={(v) => onChange({ ...config, startingBalanceUsd: v })}
        />
        <ConfigNumber
          label="Max trades"
          value={config.maxOpenMockTrades}
          step={1}
          onChange={(v) => onChange({ ...config, maxOpenMockTrades: Math.max(1, Math.floor(v)) })}
        />
        <ConfigNumber
          label="Cooldown (min)"
          value={config.tradeCooldownMinutes}
          step={1}
          onChange={(v) => onChange({ ...config, tradeCooldownMinutes: Math.max(15, v) })}
        />
        <ConfigNumber
          label="Min score"
          value={config.minSignalScore}
          step={1}
          onChange={(v) => onChange({ ...config, minSignalScore: Math.max(0, Math.min(100, v)) })}
        />
        <ConfigSelect
          label="Sizing mode"
          value={config.sizingMode}
          options={[
            { value: "fixed_pct_equity", label: "Fixed %" },
            { value: "fixed_notional", label: "Fixed $" },
            { value: "risk_pct_equity", label: "Risk %" },
          ]}
          onChange={(v) => onChange({ ...config, sizingMode: v as MockSizingMode })}
        />
        <ConfigNumber
          label="Leverage (×)"
          value={config.leverage}
          step={1}
          onChange={(v) => onChange({ ...config, leverage: Math.max(1, Math.min(125, Math.floor(v))) })}
        />
        <ConfigNumber
          label="TP %"
          value={config.takeProfitPct}
          step={0.1}
          onChange={(v) => onChange({ ...config, takeProfitPct: Math.max(0.01, v) })}
        />
        <ConfigNumber
          label="SL %"
          value={config.stopLossPct}
          step={0.1}
          onChange={(v) => onChange({ ...config, stopLossPct: Math.max(0.01, v) })}
        />
      </div>
    </TerminalPanel>
  );
}

function ResearchControlsCard({
  research,
  closedCandles,
  onMaximize,
  isMaximized,
  onClose,
}: {
  research: ReturnType<typeof useMockResearchRunner>;
  closedCandles: number;
  onMaximize?: () => void;
  isMaximized?: boolean;
  onClose?: () => void;
}) {
  const updateFamily = (family: ResearchFamily, enabled: boolean) => {
    const next = new Set(research.config.enabledFamilies);
    if (enabled) next.add(family);
    else next.delete(family);
    research.setConfig({ ...research.config, enabledFamilies: next });
  };

  return (
    <TerminalPanel
      title="Research Runner"
      subtitle="500-strategy variant evaluator"
      onMaximize={onMaximize}
      isMaximized={isMaximized}
      onClose={onClose}
    >
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(130px, 1fr))", gap: 10 }}>
        <label style={{ display: "flex", gap: 6, alignItems: "center", fontSize: 11 }}>
          <input
            type="checkbox"
            checked={research.config.enabled}
            onChange={(e) => research.setConfig({ ...research.config, enabled: e.target.checked })}
          />
          Enabled
        </label>
        <ConfigNumber
          label="Max signals/min"
          value={research.config.maxSignalsPerMinute}
          step={1}
          onChange={(v) =>
            research.setConfig({
              ...research.config,
              maxSignalsPerMinute: Math.max(1, Math.min(500, Math.floor(v))),
            })
          }
        />
        <ConfigNumber
          label="Min confidence"
          value={research.config.minConfidence}
          step={1}
          onChange={(v) =>
            research.setConfig({
              ...research.config,
              minConfidence: Math.max(0, Math.min(100, v)),
            })
          }
        />
        <DeskMetricTile label="Registry" value={`${research.strategies.length}`} compact />
        <DeskMetricTile label="Candles" value={closedCandles} compact />
        <DeskMetricTile label="Last signals" value={research.lastSignalCount} compact />
      </div>

      <div style={{ marginTop: 10, display: "flex", gap: 6, flexWrap: "wrap" }}>
        <DeskButton
          variant="outlined"
          onClick={() => research.setConfig({ ...research.config, enabledFamilies: new Set(ALL_RESEARCH_FAMILIES) })}
          style={{ minHeight: 24, fontSize: "0.7rem", padding: "0 8px" }}
        >
          All
        </DeskButton>
        <DeskButton
          variant="outlined"
          onClick={() => research.setConfig({ ...research.config, enabledFamilies: new Set<ResearchFamily>() })}
          style={{ minHeight: 24, fontSize: "0.7rem", padding: "0 8px" }}
        >
          None
        </DeskButton>
      </div>

      <div
        style={{
          marginTop: 10,
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
          gap: 6,
          maxHeight: 120,
          overflowY: "auto",
          padding: 6,
          background: "var(--surface-2)",
          borderRadius: 4,
        }}
      >
        {ALL_RESEARCH_FAMILIES.map((family) => (
          <label key={family} style={{ display: "flex", gap: 4, alignItems: "center", fontSize: 10 }}>
            <input
              type="checkbox"
              checked={research.config.enabledFamilies.has(family)}
              onChange={(e) => updateFamily(family, e.target.checked)}
            />
            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {research.familyLabels[family]}
            </span>
          </label>
        ))}
      </div>
    </TerminalPanel>
  );
}

function ConfigNumber({
  label,
  value,
  step,
  onChange,
}: {
  label: string;
  value: number;
  step: number;
  onChange: (v: number) => void;
}) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <span className="desk-label-md" style={{ fontSize: 10 }}>{label}</span>
      <input
        type="number"
        value={value}
        step={step}
        min={0}
        onChange={(e) => {
          const v = Number(e.target.value);
          if (Number.isFinite(v) && v >= 0) onChange(v);
        }}
        style={{
          padding: "4px 8px",
          borderRadius: 4,
          border: "1px solid var(--border)",
          background: "var(--surface-2)",
          color: "var(--text-primary)",
          fontSize: "0.75rem",
          fontFamily: "var(--font-mono, monospace)",
        }}
      />
    </label>
  );
}

function ConfigSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <span className="desk-label-md" style={{ fontSize: 10 }}>{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        style={{
          padding: "4px 8px",
          borderRadius: 4,
          border: "1px solid var(--border)",
          background: "var(--surface-2)",
          color: "var(--text-primary)",
          fontSize: "0.75rem",
        }}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function regimeTone(regime: string | null | undefined): "success" | "warning" | "error" | "primary" | "default" {
  if (regime === "STRONG_TREND" || regime === "TRENDING") return "success";
  if (regime === "WEAK_TREND") return "primary";
  if (regime === "BREAKOUT" || regime === "HIGH_VOLATILITY_BREAKOUT") return "warning";
  if (regime === "HIGH_VOLATILITY") return "error";
  if (regime === "REVERSAL") return "warning";
  if (regime === "LOW_VOLATILITY" || regime === "LOW_VOLATILITY_CHOP") return "default";
  if (regime === "RANGE" || regime === "RANGING") return "primary";
  return "default";
}

function PerStrategyCard({
  analytics,
  startingBalance,
  onMaximize,
  isMaximized,
  onClose,
}: {
  analytics: ReturnType<typeof computeAnalytics>;
  startingBalance: number;
  onMaximize?: () => void;
  isMaximized?: boolean;
  onClose?: () => void;
}) {
  return (
    <TerminalPanel
      title="Per-Strategy Roll-up"
      subtitle="Profitability by strategy ID"
      onMaximize={onMaximize}
      isMaximized={isMaximized}
      onClose={onClose}
      padding="none"
    >
      {analytics.perStrategy.length === 0 ? (
        <DeskEmptyState title="No data yet" subtitle="Populates as trades are created." />
      ) : (
        <DeskDataTable
          columns={[
            { id: "id", header: "Strategy", cell: (r) => `#${r.strategyId} ${r.strategyName}` },
            { id: "total", header: "Total", align: "right", cell: (r) => r.total },
            { id: "wins", header: "W/L", align: "right", cell: (r) => `${r.wins}/${r.losses}` },
            { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
            { id: "exp", header: "Exposure", align: "right", cell: (r) => fmtUsdK(r.exposure) },
            {
              id: "pnl",
              header: "Total PnL",
              align: "right",
              cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
            },
            {
              id: "realized",
              header: "Realized",
              align: "right",
              cell: (r) => <span className={pnlClass(r.realizedPnl)}>{fmtUsd(r.realizedPnl)}</span>,
            },
          ]}
          rows={analytics.perStrategy.slice(0, 50)}
          getRowKey={(r) => String(r.strategyId)}
          minWidth={760}
        />
      )}
    </TerminalPanel>
  );
}

function firstZeroPipelineStage({
  readiness,
  researchHasRun,
  selectionMode,
  approvedCount,
  funnel,
}: {
  readiness: ReturnType<typeof useMockResearchRunner>["readiness"];
  researchHasRun: boolean;
  selectionMode: ReturnType<typeof useMockResearchRunner>["config"]["selectionMode"];
  approvedCount: number;
  funnel: MockDiagnosticFunnel;
}): string {
  if (readiness.totalStrategies > 0 && readiness.strategiesReady === 0) {
    return "First zero stage: candle readiness. No enabled strategy has enough closed candles yet.";
  }
  if (!researchHasRun) {
    return "First zero stage: research cycle. The runner has not completed an evaluation cycle yet.";
  }
  if (selectionMode !== "RESEARCH_MODE" && approvedCount === 0) {
    return "First zero stage: approved strategy filter. Profit/Regime mode has no approved strategy IDs yet.";
  }
  if (funnel.signalsGenerated === 0) {
    return "First zero stage: signal generation. Strategies evaluated, but no BUY/SELL research signals were emitted.";
  }
  if (funnel.confidencePassed === 0) {
    return "First zero stage: confidence filter. Signals exist, but none pass the configured confidence threshold.";
  }
  if (funnel.scorePassed === 0) {
    return "First zero stage: signal score filter. Signals pass confidence, but none pass the configured score threshold.";
  }
  if (funnel.riskRewardPassed === 0) {
    return "First zero stage: risk/reward. Candidate signals fail the configured minimum R/R gate.";
  }
  if (funnel.cooldownPassed === 0) {
    return "First zero stage: cooldown. Candidate trades are blocked by per-strategy cooldown.";
  }
  if (funnel.familyConflictPassed === 0) {
    return "First zero stage: family conflict. Candidate trades conflict with an open opposite-family position.";
  }
  if (funnel.exposurePassed === 0) {
    return "First zero stage: exposure/risk controls. Candidate trades pass earlier gates but fail portfolio risk limits.";
  }
  if (funnel.tradesCreated === 0) {
    return "First zero stage: final creation. Candidates passed funnel counters but did not create trades; inspect latest rejection log.";
  }
  return "No zero stage detected. Signals are reaching trade creation under the current mock-only controls.";
}

function PerBlockerCard({ analytics }: { analytics: ReturnType<typeof computeAnalytics> }) {
  return (
    <TerminalPanel title="Blockers" subtitle="Frequency and hypothetical PnL">
      {analytics.perBlocker.length === 0 ? (
        <DeskEmptyState
          title="No blockers"
          subtitle="Blockers appear when signals are rejected by production gates."
        />
      ) : (
        <DeskDataTable
          columns={[
            { id: "gate", header: "Gate", cell: (r) => <DeskChip tone="warning" style={{ fontSize: 9, padding: "0 4px" }}>{r.gate}</DeskChip> },
            { id: "total", header: "Count", align: "right", cell: (r) => r.total },
            { id: "wr", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 0) },
            {
              id: "pnl",
              header: "PnL",
              align: "right",
              cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl, 0)}</span>,
            },
          ]}
          rows={analytics.perBlocker}
          getRowKey={(r) => r.gate}
          minWidth={280}
        />
      )}
    </TerminalPanel>
  );
}

function ResearchAnalyticsCards({
  analytics,
  startingBalance,
}: {
  analytics: ReturnType<typeof computeAnalytics>;
  startingBalance: number;
}) {
  const top = analytics.perStrategy.filter((r) => r.total > 0).slice(0, 10);
  const families = analytics.perFamily.slice(0, 10);

  return (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
      <TerminalPanel title="Top Strategies" subtitle="Best by total net PnL" padding="none">
        {top.length === 0 ? (
          <DeskEmptyState title="No results" subtitle="Waiting for trades." />
        ) : (
          <StrategyRankTable rows={top} startingBalance={startingBalance} />
        )}
      </TerminalPanel>

      <TerminalPanel title="PnL by Family" subtitle="Family-level performance" padding="none">
        {families.length === 0 ? (
          <DeskEmptyState title="No data" subtitle="Waiting for trades." />
        ) : (
          <DeskDataTable
            columns={[
              { id: "family", header: "Family", cell: (r) => r.family },
              { id: "count", header: "Trades", align: "right", cell: (r) => r.tradeCount },
              { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 0) },
              {
                id: "pnl",
                header: "PnL",
                align: "right",
                cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl, 0)}</span>,
              },
            ]}
            rows={families}
            getRowKey={(r) => r.family}
            minWidth={300}
          />
        )}
      </TerminalPanel>
    </div>
  );
}

function StrategyRankTable({
  rows,
  startingBalance,
}: {
  rows: ReturnType<typeof computeAnalytics>["perStrategy"];
  startingBalance: number;
}) {
  return (
    <DeskDataTable
      columns={[
        { id: "id", header: "Strategy", cell: (r) => `#${r.strategyId} ${r.strategyName}` },
        { id: "count", header: "Trades", align: "right", cell: (r) => r.total },
        { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 0) },
        {
          id: "pnl",
          header: "PnL",
          align: "right",
          cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl, 0)}</span>,
        },
      ]}
      rows={rows}
      getRowKey={(r) => String(r.strategyId)}
      minWidth={320}
    />
  );
}

export default function MockTradingDashboard() {
  const live = useLiveBTCPrice();
  const engine = useMockTradingEngine({ price: live.price });
  const candles = useMockCandleBuilder(live.price);
  const regime = useMarketRegime({ candles: candles.snapshot, newCandleReady: candles.newCandleReady });
  const research = useMockResearchRunner({
    candles: candles.snapshot,
    newCandleReady: candles.newCandleReady,
    ingestResearchSignals: engine.ingestResearchSignals,
    price: live.price,
    currentRegime: regime.regime,
  });
  const scoring = useStrategyScoring({
    trades: engine.portfolioTrades,
    currentRegime: regime.regime ?? "unknown",
    newCandleReady: candles.newCandleReady,
    topNCount: research.config.topN,
  });

  const { layout, saveLayout, resetLayout } = useTerminalLayout("mock-trading-layout");
  const [maximizedPanel, setMaximizedPanel] = useState<string | null>(null);

  const togglePanel = (id: string) => {
    const nextPanels = layout.panels.map(p => p.id === id ? { ...p, visible: !p.visible } : p);
    saveLayout({ ...layout, panels: nextPanels });
  };

  const isVisible = (id: string) => layout.panels.find(p => p.id === id)?.visible ?? true;
  const getGridColumn = (id: string) => layout.panels.find(p => p.id === id)?.gridColumn ?? "span 12";

  const [filter, setFilter] = useState<MockTradeFilter>({});
  const [showOpenOnly, setShowOpenOnly] = useState(false);
  const [sortKey, setSortKey] = useState<MockTradeSortKey>("most_profitable");
  const [strategySearch, setStrategySearch] = useState("");
  const [displayPage, setDisplayPage] = useState(1);
  const [displayPageSize, setDisplayPageSize] = useState(100);
  const [ageFilterNow, setAgeFilterNow] = useState(() => Date.now());

  const tableSourceTrades =
    engine.persistence.status === "mongo" || engine.historyTrades.length > 0
      ? engine.historyTrades
      : engine.portfolioTrades;

  const trades = useMemo(() => {
    const combined: MockTradeFilter = { ...filter };
    if (showOpenOnly) combined.status = "OPEN";
    const filtered = filterMockTrades(tableSourceTrades, combined, ageFilterNow);
    const search = strategySearch.trim().toLowerCase();
    const searched = search
      ? filtered.filter((t) =>
          String(t.strategyId).includes(search) ||
          t.strategyName.toLowerCase().includes(search) ||
          (t.strategyFamily ?? "").toLowerCase().includes(search)
        )
      : filtered;
    return sortMockTrades(searched, sortKey);
  }, [tableSourceTrades, filter, showOpenOnly, sortKey, strategySearch, ageFilterNow]);

  const maxOpenMockTrades = Math.max(1, Math.floor(engine.config.maxOpenMockTrades));
  const openUsage = Math.min(1, safeRatio(engine.account.openCount, maxOpenMockTrades));
  const openUsagePct = openUsage * 100;
  const openUsageWarning = engine.account.openCount >= maxOpenMockTrades * 0.8;
  const displayTotalPages = Math.max(1, Math.ceil(trades.length / displayPageSize));
  const displayStart = trades.length === 0 ? 0 : (displayPage - 1) * displayPageSize + 1;
  const displayEnd = Math.min(trades.length, displayPage * displayPageSize);
  const displayedTrades = useMemo(
    () => trades.slice((displayPage - 1) * displayPageSize, displayPage * displayPageSize),
    [displayPage, displayPageSize, trades],
  );

  useEffect(() => {
    setDisplayPage(1);
  }, [filter, showOpenOnly, sortKey, strategySearch, displayPageSize]);

  useEffect(() => {
    const nextApproved =
      research.config.selectionMode === "REGIME_MODE"
        ? scoring.approvedRegimeIds
        : scoring.approvedProfitIds;
    if (research.config.approvedStrategyIds === nextApproved) return;
    research.setConfig({ ...research.config, approvedStrategyIds: nextApproved });
  }, [research.config, research.setConfig, scoring.approvedProfitIds, scoring.approvedRegimeIds]);

  useEffect(() => {
    if (!regime.snapshot || typeof fetch !== "function") return;
    void fetch("/api/mock-trading/regime", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ snapshot: regime.snapshot }),
    }).catch(() => {
      // Regime persistence is best-effort; dashboard state remains local.
    });
  }, [regime.snapshot]);

  useEffect(() => {
    if (!candles.newCandleReady || typeof fetch !== "function") return;
    const snapshot = createEquitySnapshot({ account: engine.account, trades: engine.portfolioTrades, regime: regime.regime });
    void fetch("/api/mock-trading/equity", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        point: {
          timestamp: snapshot.timestamp,
          equity: snapshot.equity,
          realizedPnl: snapshot.realizedPnl,
          unrealizedPnl: snapshot.unrealizedPnl,
          drawdownPct: snapshot.drawdownPct,
          dailyPnl: snapshot.dailyPnl,
          regime: snapshot.regime ?? undefined,
        },
      }),
    }).catch(() => {
      // Equity persistence is best-effort; local chart state continues updating.
    });

    const latestDaily = computeDailyPnlPoints(engine.portfolioTrades).at(-1);
    if (latestDaily) {
      const dayStart = Math.floor(latestDaily.timestamp / 86_400_000) * 86_400_000;
      const tradeCount = engine.portfolioTrades.filter(
        (trade) => trade.status === "CLOSED" && trade.closedAt != null && trade.closedAt >= dayStart,
      ).length;
      void fetch("/api/mock-trading/daily-pnl", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          point: {
            day: dayStart,
            pnl: latestDaily.value,
            tradeCount,
            regime: regime.regime ?? undefined,
          },
        }),
      }).catch(() => {
        // Daily PnL history is best-effort.
      });
    }
  }, [candles.newCandleReady, engine.account, engine.portfolioTrades, regime.regime]);

  useEffect(() => {
    if (scoring.scores.length === 0 || typeof fetch !== "function") return;
    void fetch("/api/mock-trading/scores", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ scores: scoring.scores }),
    }).catch(() => {
      // Latest ranking and ranking-history persistence is best-effort.
    });
  }, [scoring.lastScoredAt, scoring.scores]);

  useEffect(() => {
    setDisplayPage((page) => Math.min(page, displayTotalPages));
  }, [displayTotalPages]);

  useEffect(() => {
    const id = setInterval(() => setAgeFilterNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  const strategyOptions = useMemo(() => {
    const seen = new Map<number, string>();
    for (const t of engine.trades) {
      if (!seen.has(t.strategyId)) seen.set(t.strategyId, t.strategyName);
    }
    return [...seen.entries()].sort(([a], [b]) => a - b);
  }, [engine.trades]);

  const blockerOptions = useMemo(() => {
    const set = new Set<string>();
    for (const t of engine.trades) for (const b of t.blockers) set.add(b.gate);
    return [...set].sort();
  }, [engine.trades]);

  const familyOptions = useMemo(() => {
    const set = new Set<string>(ALL_RESEARCH_FAMILIES);
    for (const t of engine.trades) if (t.strategyFamily) set.add(t.strategyFamily);
    return [...set].sort();
  }, [engine.trades]);

  const columns = useMemo<DeskColumn<MockTrade>[]>(
    () => [
      { id: "ts", header: "Opened", cell: (t) => new Date(t.openedAt).toLocaleTimeString() },
      { id: "strategy", header: "Strategy", cell: (t) => `#${t.strategyId} ${t.strategyName}` },
      {
        id: "side",
        header: "Side",
        cell: (t) => (
          <span style={{ color: t.side === "BUY" ? "var(--desk-success)" : "var(--desk-error)" }}>
            {t.side}
          </span>
        ),
      },
      { id: "entry", header: "Entry", align: "right", cell: (t) => fmtPrice(t.entryPrice) },
      {
        id: "tpUsd",
        header: "TP Profit $",
        align: "right",
        cell: (t) => (
          <span style={{ color: "var(--desk-success)" }}>{fmtUsd(t.takeProfitUsd)}</span>
        ),
      },
      {
        id: "slUsd",
        header: "SL Loss $",
        align: "right",
        cell: (t) => (
          <span style={{ color: "var(--desk-error)" }}>{fmtUsd(-t.stopLossUsd)}</span>
        ),
      },
      {
        id: "pnl",
        header: "Current PnL",
        align: "right",
        cell: (t) => {
          const pnl = t.status === "OPEN" ? t.unrealizedPnl : t.realizedPnl;
          return <span className={pnlClass(pnl)}>{fmtUsd(pnl)}</span>;
        },
      },
      {
        id: "status",
        header: "Status",
        cell: (t) => (
          <DeskChip tone={t.status === "OPEN" ? "primary" : "default"}>{t.status}</DeskChip>
        ),
      },
      {
        id: "exitReason",
        header: "Exit Reason",
        cell: (t) => exitReasonBadge(t.exitReason),
      },
      {
        id: "blockers",
        header: "Blockers",
        cell: (t) =>
          t.blockers.length === 0 ? (
            <span style={{ color: "var(--desk-on-surface-variant)" }}>—</span>
          ) : (
            <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
              {t.blockers.map((b, i) => (
                <DeskChip key={`${t.id}-b-${i}`} tone="warning">
                  {b.gate}
                </DeskChip>
              ))}
            </div>
          ),
      },
      { id: "age", header: "Age", align: "right", cell: (t) => fmtTradeAge(t, ageFilterNow) },
      {
        id: "actions",
        header: "",
        cell: (t) =>
          t.status === "OPEN" ? (
            <DeskButton
              variant="outlined"
              onClick={() => engine.closeTrade(t.id)}
              style={{ minHeight: 28, fontSize: "0.75rem", padding: "0 10px" }}
            >
              Close
            </DeskButton>
          ) : (
            <span style={{ color: "var(--desk-on-surface-variant)", fontSize: 11 }}>—</span>
          ),
      },
    ],
    [engine, ageFilterNow],
  );

  const acct = engine.account;
  const totalPnlClass = pnlClass(engine.analytics.totalPnl);
  const realizedClass = pnlClass(engine.analytics.realizedPnl);
  const unrealizedClass = pnlClass(engine.analytics.unrealizedPnl);
  const advancedResearch = useMemo(
    () => computeAdvancedResearchAnalytics({ trades: engine.portfolioTrades, scores: scoring.scores, account: acct }),
    [acct, engine.portfolioTrades, scoring.scores],
  );
  const strategyHealth = useMemo(
    () => computeStrategyHealth(scoring.scores, engine.portfolioTrades),
    [engine.portfolioTrades, scoring.scores],
  );
  const walkForwardRows = useMemo(
    () => computeMockWalkForwardRows(engine.portfolioTrades),
    [engine.portfolioTrades],
  );
  const strategyFamilyById = useMemo(() => {
    const map = new Map<number, string>();
    for (const trade of engine.portfolioTrades) {
      if (trade.strategyFamily) map.set(trade.strategyId, trade.strategyFamily);
    }
    return map;
  }, [engine.portfolioTrades]);

  const portfolioAllocation = useMemo(
    () =>
      computePortfolioAllocation({
        scores: scoring.scores,
        healthRows: strategyHealth,
        equity: acct.equity,
        strategyFamilyById,
      }),
    [acct.equity, scoring.scores, strategyFamilyById, strategyHealth],
  );

  const dailyPnl = useMemo(
    () => createEquitySnapshot({ account: acct, trades: engine.portfolioTrades, regime: regime.regime }).dailyPnl,
    [acct, engine.portfolioTrades, regime.regime],
  );
  const weeklyPnl = useMemo(
    () => computeWeeklyPnl(engine.portfolioTrades),
    [engine.portfolioTrades],
  );
  const monthlyPnl = useMemo(
    () => computeMonthlyPnl(engine.portfolioTrades),
    [engine.portfolioTrades],
  );
  const activeStrategies = useMemo(
    () => new Set(engine.portfolioTrades.map((trade) => trade.strategyId)).size,
    [engine.portfolioTrades],
  );

  const combinedDiagnostics = useMemo<MockTradingDiagnostics>(() => ({
    funnel: combineDiagnostics(research.diagnostics.totals.funnel, engine.diagnostics.funnel),
    rejectionCounts: addCounts(research.diagnostics.totals.rejectionCounts, engine.diagnostics.rejectionCounts),
    recentRejections: [
      ...engine.diagnostics.recentRejections,
      ...research.diagnostics.totals.recentRejections,
    ].sort((a, b) => b.ts - a.ts).slice(0, 75),
    lastUpdatedAt: Math.max(research.lastEvalAt ?? 0, engine.diagnostics.lastUpdatedAt ?? 0) || null,
  }), [engine.diagnostics, research.diagnostics.totals, research.lastEvalAt]);

  const connectionStatus = live.connected ? "live" as const : "offline" as const;
  const persistenceStatus = engine.persistence.status === "mongo"
    ? "mongo" as const
    : engine.persistence.status === "hydrating"
      ? "hydrating" as const
      : "local" as const;

  return (
    <M3AppShell
      pageTitle="Mock Trading"
      price={live.price}
      priceChange24hPct={live.change24h}
      statusChips={
        <>
          <StatusChip label={connectionStatus === "live" ? "Live" : "Offline"} tone={connectionStatus === "live" ? "success" : "error"} />
          <StatusChip label={persistenceStatus === "mongo" ? "Mongo" : persistenceStatus} tone={persistenceStatus === "mongo" ? "info" : "warning"} />
          {engine.error ? <StatusChip label="Degraded" tone="warning" /> : null}
        </>
      }
    >
      <TerminalWorkspace columns="repeat(12, 1fr)">

        {/* Row 1: Account, Ticker, Regime */}
        {isVisible("account") && (
          <div style={{ gridColumn: getGridColumn("account") }}>
            <AccountSummaryCard
              account={acct}
              maxOpenMockTrades={maxOpenMockTrades}
              openUsagePct={openUsagePct}
              onMaximize={() => setMaximizedPanel(maximizedPanel === "account" ? null : "account")}
              isMaximized={maximizedPanel === "account"}
              onClose={() => togglePanel("account")}
            />
          </div>
        )}

        {isVisible("ticker") && (
          <div style={{ gridColumn: getGridColumn("ticker") }}>
            <TerminalPanel
              title="Live Ticker"
              subtitle={`BTCUSD · ${live.connected ? "Binance stream" : "reconnecting"}`}
              onMaximize={() => setMaximizedPanel(maximizedPanel === "ticker" ? null : "ticker")}
              isMaximized={maximizedPanel === "ticker"}
              onClose={() => togglePanel("ticker")}
            >
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "repeat(2, 1fr)",
                  gap: 10,
                }}
              >
                <DeskMetricTile label="BTC Price" value={fmtPrice(live.price)} compact />
                <DeskMetricTile label="24h Change" value={`${live.change24h.toFixed(2)}%`} valueClassName={pnlClass(live.change24h)} compact />
                <DeskMetricTile label="24h High" value={fmtPrice(live.high24h)} compact />
                <DeskMetricTile label="24h Low" value={fmtPrice(live.low24h)} compact />
                <DeskMetricTile label="Ticks / s" value={live.ticksPerSecond} compact />
                <DeskMetricTile
                  label="Trace age"
                  value={engine.traceAgeSeconds == null ? "—" : `${engine.traceAgeSeconds}s`}
                  compact
                />
              </div>
            </TerminalPanel>
          </div>
        )}

        {isVisible("regime") && (
          <div style={{ gridColumn: getGridColumn("regime") }}>
            <MarketRegimePanel
              snapshot={regime.snapshot}
              onMaximize={() => setMaximizedPanel(maximizedPanel === "regime" ? null : "regime")}
              isMaximized={maximizedPanel === "regime"}
              onClose={() => togglePanel("regime")}
            />
          </div>
        )}

        {/* Row 2: Research Controls, Readiness */}
        {isVisible("research") && (
          <div style={{ gridColumn: getGridColumn("research") }}>
            <ResearchControlsCard
              research={research}
              closedCandles={candles.closedCount}
              onMaximize={() => setMaximizedPanel(maximizedPanel === "research" ? null : "research")}
              isMaximized={maximizedPanel === "research"}
              onClose={() => togglePanel("research")}
            />
          </div>
        )}

        {isVisible("readiness") && (
          <div style={{ gridColumn: getGridColumn("readiness") }}>
            <MockTradingReadinessAuditCard
              research={research}
              scoring={scoring}
              diagnostics={combinedDiagnostics}
              onMaximize={() => setMaximizedPanel(maximizedPanel === "readiness" ? null : "readiness")}
              isMaximized={maximizedPanel === "readiness"}
              onClose={() => togglePanel("readiness")}
            />
          </div>
        )}

        {isVisible("pine-editor") && (
          <div style={{ gridColumn: getGridColumn("pine-editor") }}>
            <PineEditorPanel
              candles={candles.snapshot}
              onExecute={(res) => {
                console.log("Pine Execution Result:", res);
              }}
            />
          </div>
        )}

        {/* Row 3: Diagnostics, Research Lab */}
        {isVisible("diagnostics") && (
          <div style={{ gridColumn: getGridColumn("diagnostics") }}>
            <MockTradingDiagnosticsCard
              diagnostics={combinedDiagnostics}
              latestResearch={research.diagnostics.latest}
              onMaximize={() => setMaximizedPanel(maximizedPanel === "diagnostics" ? null : "diagnostics")}
              isMaximized={maximizedPanel === "diagnostics"}
              onClose={() => togglePanel("diagnostics")}
            />
          </div>
        )}

        {isVisible("lab") && (
          <div style={{ gridColumn: getGridColumn("lab") }}>
            <TerminalPanel
              title="Research Lab Overview"
              subtitle="Strategy selection and ranking"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "lab" ? null : "lab")}
              isMaximized={maximizedPanel === "lab"}
              onClose={() => togglePanel("lab")}
            >
              <ResearchLabOverviewContent
                scoring={scoring}
                research={research}
              />
            </TerminalPanel>
          </div>
        )}

        {/* Row 4: Main Chart */}
        {isVisible("charts") && (
          <div style={{ gridColumn: getGridColumn("charts") }}>
            <TerminalPanel title="BTCUSD Professional Chart" subtitle="Real-time 1m candles with trade overlays" padding="none">
              <InstitutionalChart
                candles={candles.snapshot}
                trades={engine.trades}
                height={420}
              />
            </TerminalPanel>
          </div>
        )}

        {/* Row 5: Analytics & Research Charts */}
        <div style={{ gridColumn: "span 12" }}>
          <MockResearchChartsPanel
            trades={engine.trades}
            account={acct}
            regime={regime.regime}
          />
        </div>

        {/* Row 6: Analytics */}
        {isVisible("analytics") && (
          <div style={{ gridColumn: getGridColumn("analytics") }}>
            <TerminalPanel
              title="Trade Analytics"
              subtitle="Performance metrics and profit factor"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "analytics" ? null : "analytics")}
              isMaximized={maximizedPanel === "analytics"}
              onClose={() => togglePanel("analytics")}
            >
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
                  gap: 12,
                }}
              >
                <DeskMetricTile label="Total trades" value={engine.analytics.totalTrades} compact />
                <DeskMetricTile
                  label="Open / Max"
                  value={`${engine.analytics.openTrades.toLocaleString("en-US")} / ${maxOpenMockTrades.toLocaleString("en-US")}`}
                  compact
                />
                <DeskMetricTile label="Closed" value={engine.analytics.closedTrades} compact />
                <DeskMetricTile label="Risk rejects" value={engine.mockLimitRejectedSignals.toLocaleString("en-US")} compact />
                <DeskMetricTile label="Win rate" value={fmtPct(engine.analytics.winRate, 1)} compact />
                <DeskMetricTile label="Total PnL" value={fmtUsd(engine.analytics.totalPnl)} valueClassName={totalPnlClass} compact />
                <DeskMetricTile label="Realized" value={fmtUsd(engine.analytics.realizedPnl)} valueClassName={realizedClass} compact />
                <DeskMetricTile label="Unrealized" value={fmtUsd(engine.analytics.unrealizedPnl)} valueClassName={unrealizedClass} compact />
                <DeskMetricTile label="Avg realized PnL" value={fmtUsd(engine.analytics.averageRealizedPnl)} compact />
                <DeskMetricTile label="Average trade" value={fmtUsd(engine.analytics.averageTrade)} compact />
                <DeskMetricTile label="Avg win" value={fmtUsd(engine.analytics.averageWin)} valueClassName={pnlClass(engine.analytics.averageWin)} compact />
                <DeskMetricTile label="Avg loss" value={fmtUsd(engine.analytics.averageLoss)} valueClassName={pnlClass(engine.analytics.averageLoss)} compact />
                <DeskMetricTile label="TP wins" value={engine.analytics.takeProfitWins} compact />
                <DeskMetricTile label="SL losses" value={engine.analytics.stopLossLosses} compact />
                <DeskMetricTile label="TP hit rate" value={fmtPct(engine.analytics.takeProfitHitRate, 1)} compact />
                <DeskMetricTile label="SL hit rate" value={fmtPct(engine.analytics.stopLossHitRate, 1)} compact />
                <DeskMetricTile
                  label="Profit factor"
                  value={engine.analytics.profitFactor == null ? "—" : engine.analytics.profitFactor.toFixed(2)}
                  compact
                />
                <DeskMetricTile
                  label="Sharpe ratio"
                  value={engine.analytics.sharpeRatio == null ? "—" : engine.analytics.sharpeRatio.toFixed(2)}
                  compact
                />
              </div>
            </TerminalPanel>
          </div>
        )}

        {isVisible("leaderboard") && (
          <div style={{ gridColumn: getGridColumn("leaderboard") }}>
            <StrategyLeaderboard scores={scoring.scores} />
          </div>
        )}

        {/* Row 6: Configuration */}
        {isVisible("config") && (
          <div style={{ gridColumn: getGridColumn("config") }}>
            <MockConfigCard
              config={engine.config}
              onChange={engine.setConfig}
              onMaximize={() => setMaximizedPanel(maximizedPanel === "config" ? null : "config")}
              isMaximized={maximizedPanel === "config"}
              onClose={() => togglePanel("config")}
            />
          </div>
        )}

        {/* Row 7: Equity Curve */}
        {isVisible("equity-curve") && (
          <div style={{ gridColumn: getGridColumn("equity-curve") }}>
            <TerminalPanel
              title="Equity Curve & PnL"
              subtitle="Sparkline, daily bars, monthly grid, extended metrics"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "equity-curve" ? null : "equity-curve")}
              isMaximized={maximizedPanel === "equity-curve"}
              onClose={() => togglePanel("equity-curve")}
            >
              <MockEquityCurvePanel
                trades={engine.trades}
                startingEquityUsd={engine.config.startingBalanceUsd}
              />
            </TerminalPanel>
          </div>
        )}

        {/* Row 8: Monte Carlo + Strategy Leaderboard (side by side) */}
        {isVisible("monte-carlo") && (
          <div style={{ gridColumn: getGridColumn("monte-carlo") }}>
            <TerminalPanel
              title="Monte Carlo Analysis"
              subtitle="Bootstrap equity simulation · 1 000 paths"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "monte-carlo" ? null : "monte-carlo")}
              isMaximized={maximizedPanel === "monte-carlo"}
              onClose={() => togglePanel("monte-carlo")}
            >
              <MockMonteCarloPanel
                trades={engine.trades}
                startingEquityUsd={engine.config.startingBalanceUsd}
              />
            </TerminalPanel>
          </div>
        )}

        {isVisible("strategy-leaderboard") && (
          <div style={{ gridColumn: getGridColumn("strategy-leaderboard") }}>
            <TerminalPanel
              title="Strategy Leaderboard"
              subtitle="Ranked by composite score · regime matrix"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "strategy-leaderboard" ? null : "strategy-leaderboard")}
              isMaximized={maximizedPanel === "strategy-leaderboard"}
              onClose={() => togglePanel("strategy-leaderboard")}
            >
              <MockStrategyLeaderboardPanel
                trades={engine.trades}
              />
            </TerminalPanel>
          </div>
        )}

        {/* Row 9: Risk Analytics */}
        {isVisible("risk-analytics") && (
          <div style={{ gridColumn: getGridColumn("risk-analytics") }}>
            <TerminalPanel
              title="Risk Analytics"
              subtitle="Kill-switch, health monitor, trade quality, stress tests"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "risk-analytics" ? null : "risk-analytics")}
              isMaximized={maximizedPanel === "risk-analytics"}
              onClose={() => togglePanel("risk-analytics")}
            >
              <MockRiskAnalyticsPanel
                trades={engine.trades}
                startingEquityUsd={engine.config.startingBalanceUsd}
                config={engine.config}
                account={engine.account}
              />
            </TerminalPanel>
          </div>
        )}

        {/* Row 10: Trade History */}
        {isVisible("history") && (
          <div style={{ gridColumn: getGridColumn("history") }}>
            <TerminalPanel
              title="Open Positions & Trade History"
              subtitle={`${displayStart}-${displayEnd} of ${trades.length} matching displayed · ${engine.history.total || engine.trades.length} persisted total`}
              padding="none"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "history" ? null : "history")}
              isMaximized={maximizedPanel === "history"}
              onClose={() => togglePanel("history")}
            >
              <div style={{ padding: 12 }}>
                <TradeHistoryFilters
                  filter={filter}
                  setFilter={setFilter}
                  strategyOptions={strategyOptions}
                  blockerOptions={blockerOptions}
                  familyOptions={familyOptions}
                  research={research}
                  sortKey={sortKey}
                  setSortKey={setSortKey}
                  displayPageSize={displayPageSize}
                  setDisplayPageSize={setDisplayPageSize}
                  strategySearch={strategySearch}
                  setStrategySearch={setStrategySearch}
                  showOpenOnly={showOpenOnly}
                  setShowOpenOnly={setShowOpenOnly}
                  engine={engine}
                />

                <div
                  style={{
                    display: "flex",
                    flexWrap: "wrap",
                    gap: 8,
                    alignItems: "center",
                    marginBottom: 12,
                    color: "var(--text-secondary)",
                    fontSize: 11,
                  }}
                >
                  <span>
                    Mongo history page {engine.history.page} / {engine.history.totalPages}
                    {engine.history.loading ? " · loading" : ""}
                  </span>
                  <span>
                    Display page {displayPage} / {displayTotalPages}
                  </span>
                  <DeskButton
                    variant="outlined"
                    onClick={() => engine.history.setPage(Math.max(1, engine.history.page - 1))}
                    disabled={engine.history.page <= 1 || engine.history.loading}
                    style={{ minHeight: 24, fontSize: "0.7rem", padding: "0 8px" }}
                  >
                    Prev
                  </DeskButton>
                  <DeskButton
                    variant="outlined"
                    onClick={() => engine.history.setPage(Math.min(engine.history.totalPages, engine.history.page + 1))}
                    disabled={engine.history.page >= engine.history.totalPages || engine.history.loading}
                    style={{ minHeight: 24, fontSize: "0.7rem", padding: "0 8px" }}
                  >
                    Next
                  </DeskButton>
                  <DeskButton
                    variant="outlined"
                    onClick={() => setDisplayPage(Math.max(1, displayPage - 1))}
                    disabled={displayPage <= 1}
                    style={{ minHeight: 24, fontSize: "0.7rem", padding: "0 8px" }}
                  >
                    Display Prev
                  </DeskButton>
                  <DeskButton
                    variant="outlined"
                    onClick={() => setDisplayPage(Math.min(displayTotalPages, displayPage + 1))}
                    disabled={displayPage >= displayTotalPages}
                    style={{ minHeight: 24, fontSize: "0.7rem", padding: "0 8px" }}
                  >
                    Display Next
                  </DeskButton>
                  {engine.history.error && (
                    <span style={{ color: "var(--desk-error)", fontSize: 11 }}>
                      {engine.history.error}
                    </span>
                  )}
                </div>

                <DeskDataTable
                  columns={columns}
                  rows={displayedTrades}
                  getRowKey={(t) => t.id}
                  empty={
                    <DeskEmptyState
                      title="No mock trades yet"
                      subtitle={
                        engine.error
                          ? `Trace fetch error: ${engine.error}`
                          : engine.persistence.loading
                            ? "Hydrating persisted Mock Trading state from MongoDB."
                            : "Waiting for the next strategy signal trace tick."
                      }
                    />
                  }
                  minWidth={1540}
                />

                <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
                  <DeskButton
                    variant="outlined"
                    onClick={() =>
                      exportToCsv(
                        `mock-trades-${Date.now()}.csv`,
                        ["ID", "Strategy", "Side", "Entry", "Exit", "PnL", "Status", "Exit Reason"],
                        displayedTrades.map((t) => [
                          t.id,
                          `${t.strategyId} ${t.strategyName}`,
                          t.side,
                          t.entryPrice,
                          t.exitPrice ?? "",
                          t.realizedPnl,
                          t.status,
                          t.exitReason ?? "",
                        ]),
                      )
                    }
                    style={{ fontSize: "0.75rem", minHeight: 28, padding: "0 12px" }}
                  >
                    Export CSV
                  </DeskButton>
                  <DeskButton
                    variant="outlined"
                    onClick={() => exportToJson(`mock-trades-${Date.now()}.json`, displayedTrades)}
                    style={{ fontSize: "0.75rem", minHeight: 28, padding: "0 12px" }}
                  >
                    Export JSON
                  </DeskButton>
                </div>
              </div>
            </TerminalPanel>
          </div>
        )}

        {/* Per-strategy roll-up */}
        {isVisible("per-strategy") && (
          <div style={{ gridColumn: getGridColumn("per-strategy") }}>
            <TerminalPanel
              title="Per-Strategy Roll-up"
              subtitle="Sorted by total PnL · return % relative to starting balance"
              padding="none"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "per-strategy" ? null : "per-strategy")}
              isMaximized={maximizedPanel === "per-strategy"}
              onClose={() => togglePanel("per-strategy")}
            >
              {engine.analytics.perStrategy.length === 0 ? (
                <DeskEmptyState title="No data yet" subtitle="Per-strategy roll-up populates as mock trades are created." />
              ) : (
                <DeskDataTable
                  columns={[
                    { id: "id", header: "Strategy", cell: (r) => `#${r.strategyId} ${r.strategyName}` },
                    { id: "total", header: "Total", align: "right", cell: (r) => r.total },
                    { id: "open", header: "Open", align: "right", cell: (r) => r.open },
                    { id: "closed", header: "Closed", align: "right", cell: (r) => r.closed },
                    { id: "wins", header: "W/L", align: "right", cell: (r) => `${r.wins}/${r.losses}` },
                    { id: "win", header: "Win %", align: "right", cell: (r) => fmtPct(r.winRate, 1) },
                    { id: "exp", header: "Exposure", align: "right", cell: (r) => fmtUsdK(r.exposure) },
                    {
                      id: "pnl",
                      header: "Total PnL",
                      align: "right",
                      cell: (r) => <span className={pnlClass(r.totalPnl)}>{fmtUsd(r.totalPnl)}</span>,
                    },
                    {
                      id: "realized",
                      header: "Realized",
                      align: "right",
                      cell: (r) => <span className={pnlClass(r.realizedPnl)}>{fmtUsd(r.realizedPnl)}</span>,
                    },
                  ]}
                  rows={engine.analytics.perStrategy}
                  getRowKey={(r) => String(r.strategyId)}
                  minWidth={760}
                />
              )}
            </TerminalPanel>
          </div>
        )}

        {/* Trade Log */}
        {isVisible("log") && (
          <div style={{ gridColumn: getGridColumn("log") }}>
            <TerminalPanel
              title="Mock Trade Log"
              subtitle="Most recent events — newest first"
              onMaximize={() => setMaximizedPanel(maximizedPanel === "log" ? null : "log")}
              isMaximized={maximizedPanel === "log"}
              onClose={() => togglePanel("log")}
            >
              {engine.logs.length === 0 ? (
                <DeskEmptyState
                  title="No log events"
                  subtitle="MOCK_TRADE_CREATED, MOCK_TRADE_TP_HIT, MOCK_TRADE_SL_HIT events will appear here as signals arrive."
                />
              ) : (
                <div
                  style={{
                    fontFamily: "var(--font-mono, monospace)",
                    fontSize: 11,
                    lineHeight: 1.5,
                    maxHeight: 300,
                    overflowY: "auto",
                    background: "rgba(13, 17, 23, 0.6)",
                    padding: 12,
                    borderRadius: 6,
                  }}
                >
                  {engine.logs.map((log, i) => (
                    <div key={i}>
                      <span style={{ color: "var(--text-muted)" }}>
                        {new Date(log.ts).toLocaleTimeString()}
                      </span>{" "}
                      <span style={{ color: logColor(log.event) }}>
                        {log.event}
                      </span>{" "}
                      {log.tradeId ? `id=${log.tradeId} ` : ""}
                      strategy=#{log.strategyId} {log.strategyName} side={log.side} price=
                      {fmtPrice(log.price)}
                      {log.message ? ` ${log.message}` : ""}
                      {log.entryPrice != null ? ` entry=${fmtPrice(log.entryPrice)}` : ""}
                      {log.exitPrice != null ? ` exit=${fmtPrice(log.exitPrice)}` : ""}
                      {log.exitReason ? ` reason=${log.exitReason}` : ""}
                      {log.notional != null ? ` notional=${fmtUsdK(log.notional)}` : ""}
                      {log.pnl != null ? ` pnl=${fmtUsd(log.pnl)}` : ""}
                    </div>
                  ))}
                </div>
              )}
              <div style={{ marginTop: 8, fontSize: 11, color: "var(--text-muted)" }}>
                Rejected signals are logged for diagnostics; blockers no longer create mock trades.
              </div>
            </TerminalPanel>
          </div>
        )}

      </TerminalWorkspace>
    </M3AppShell>
  );
}
