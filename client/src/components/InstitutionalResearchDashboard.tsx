"use client";

/**
 * Institutional Research Dashboard
 *
 * Renders research metrics for the 10 institutional BTC futures strategy
 * modules: live rankings, regime leaderboard, walk-forward validation,
 * strategy health grid, and static design metadata.
 *
 * MOCK / RESEARCH ONLY — this component never displays or triggers real orders.
 */

import { useMemo, useState } from "react";
import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import { REGIME_LABELS, type MarketRegime } from "@/lib/ai/marketRegimeClassifier";
import {
  buildInstitutionalReport,
  buildWalkForwardSummary,
  getStaticStrategyTable,
  type InstitutionalResearchReport,
  type InstitutionalStrategyMetrics,
} from "@/lib/ai/institutionalResearchEngine";
import { INSTITUTIONAL_STRATEGIES } from "@/lib/trading/btcInstitutionalStrategies";
import { AutoSortTable } from "@/components/desk/ui";
import {
  DeskCard,
  DeskChip,
  DeskDataTable,
  DeskEmptyState,
  DeskMetricTile,
  DeskSectionHeader,
  DeskTabs,
  type DeskColumn,
  type DeskTabItem,
} from "@/components/desk/ui";

// ── Formatting helpers ─────────────────────────────────────────────────────────

function fmtUsd(v: number): string {
  if (!Number.isFinite(v)) return "—";
  const sign = v < 0 ? "-" : v > 0 ? "+" : "";
  return `${sign}$${Math.abs(v).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function fmtPct(v: number, decimals = 1): string {
  if (!Number.isFinite(v)) return "—";
  return `${(v * 100).toFixed(decimals)}%`;
}

function fmtNum(v: number, decimals = 2): string {
  if (!Number.isFinite(v)) return "—";
  return v.toFixed(decimals);
}

// ── Sub-component: Summary tiles ──────────────────────────────────────────────

function SummaryTiles({ report }: { report: InstitutionalResearchReport }) {
  const bestByPnl = report.rankByPnl[0];
  const activeCount = report.activeStrategies.length;
  const watchlistCount = report.watchlistStrategies.length;
  const disabledCount = report.disabledStrategies.length;
  const totalPnl = report.strategies.reduce((s, m) => s + m.netPnl, 0);

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
      <DeskMetricTile
        label="Total Net PnL"
        value={fmtUsd(totalPnl)}
        valueClassName={totalPnl >= 0 ? "desk-pnl-positive" : "desk-pnl-negative"}
      />
      <DeskMetricTile label="Closed Trades" value={String(report.totalClosedTrades)} />
      <DeskMetricTile label="Active" value={String(activeCount)} />
      <DeskMetricTile label="Watchlist" value={String(watchlistCount)} />
      <DeskMetricTile
        label="Disabled"
        value={String(disabledCount)}
        valueClassName={disabledCount > 0 ? "desk-pnl-negative" : undefined}
      />
      {bestByPnl && (
        <DeskMetricTile
          label="Best PnL"
          value={bestByPnl.strategyName.replace(/_/g, " ").slice(0, 20)}
          detail={fmtUsd(bestByPnl.netPnl)}
        />
      )}
    </div>
  );
}

// ── Inline badge helpers (text-based, no custom component needed) ──────────────

function HealthBadge({ state }: { state: "ACTIVE" | "WATCHLIST" | "DISABLED" }) {
  const tone =
    state === "ACTIVE" ? "success" : state === "WATCHLIST" ? "warning" : "error";
  return <DeskChip tone={tone}>{state}</DeskChip>;
}

function WfBadge({ status }: { status: "PASS" | "FAIL" | "COLLECT_DATA" }) {
  const tone = status === "PASS" ? "success" : status === "FAIL" ? "error" : "default";
  const label = status === "COLLECT_DATA" ? "DATA" : status;
  return <DeskChip tone={tone}>{label}</DeskChip>;
}

function TierBadge({ tier }: { tier: 1 | 2 | 3 }) {
  const tone = tier === 1 ? "success" : tier === 2 ? "warning" : "default";
  return <DeskChip tone={tone}>T{tier}</DeskChip>;
}

// ── Sub-component: Live Rankings tab ─────────────────────────────────────────

type RankMetric =
  | "score"
  | "pnl"
  | "profitFactor"
  | "sharpe"
  | "expectancy"
  | "winRate"
  | "drawdown";

function LiveRankingsTab({ report }: { report: InstitutionalResearchReport }) {
  const [metric, setMetric] = useState<RankMetric>("score");

  const ranked: InstitutionalStrategyMetrics[] = {
    score: report.rankByScore,
    pnl: report.rankByPnl,
    profitFactor: report.rankByProfitFactor,
    sharpe: report.rankBySharpe,
    expectancy: report.rankByExpectancy,
    winRate: report.rankByWinRate,
    drawdown: report.rankByLowestDrawdown,
  }[metric];

  const metricButtons: Array<{ key: RankMetric; label: string }> = [
    { key: "score",        label: "Overall Score" },
    { key: "pnl",         label: "Net PnL" },
    { key: "profitFactor",label: "Profit Factor" },
    { key: "sharpe",      label: "Sharpe" },
    { key: "expectancy",  label: "Expectancy" },
    { key: "winRate",     label: "Win Rate" },
    { key: "drawdown",    label: "Low Drawdown" },
  ];

  const columns: DeskColumn<InstitutionalStrategyMetrics>[] = [
    {
      id: "rank",
      header: "#",
      cell: (_row, i) => String(i + 1),
    },
    {
      id: "strategyName",
      header: "Strategy",
      cell: (row) => (
        <span style={{ fontFamily: "var(--desk-font-mono)", fontSize: "0.72rem" }}>
          {row.strategyName.replace(/_/g, " ")}
        </span>
      ),
    },
    {
      id: "tier",
      header: "Tier",
      cell: (row) => <TierBadge tier={row.strategyTier} />,
    },
    {
      id: "healthState",
      header: "Health",
      cell: (row) => <HealthBadge state={row.healthState} />,
    },
    {
      id: "closedTrades",
      header: "Trades",
      cell: (row) => String(row.closedTrades),
      align: "right",
    },
    {
      id: "netPnl",
      header: "Net PnL",
      cell: (row) => (
        <span style={{ color: row.netPnl >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
          {fmtUsd(row.netPnl)}
        </span>
      ),
      align: "right",
    },
    {
      id: "winRate",
      header: "Win%",
      cell: (row) => fmtPct(row.winRate),
      align: "right",
    },
    {
      id: "profitFactor",
      header: "PF",
      cell: (row) => (
        <span style={{ color: row.profitFactor >= 1.5 ? "var(--desk-success)" : row.profitFactor >= 1.0 ? "var(--desk-warning)" : "var(--desk-error)" }}>
          {fmtNum(row.profitFactor)}
        </span>
      ),
      align: "right",
    },
    {
      id: "sharpeRatio",
      header: "Sharpe",
      cell: (row) => (
        <span style={{ color: row.sharpeRatio >= 1 ? "var(--desk-success)" : "var(--desk-on-surface-variant)" }}>
          {fmtNum(row.sharpeRatio)}
        </span>
      ),
      align: "right",
    },
    {
      id: "liveNetExpectancy",
      header: "Exp $",
      cell: (row) => (
        <span style={{ color: row.liveNetExpectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
          {fmtUsd(row.liveNetExpectancy)}
        </span>
      ),
      align: "right",
    },
    {
      id: "maxDrawdownPct",
      header: "MaxDD%",
      cell: (row) => (
        <span style={{ color: "var(--desk-error)" }}>{fmtNum(row.maxDrawdownPct, 1)}%</span>
      ),
      align: "right",
    },
    {
      id: "walkForwardScore",
      header: "WF",
      cell: (row) => fmtNum(row.walkForwardScore, 0),
      align: "right",
    },
  ];

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-4)" }}>
      {/* Metric selector */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--desk-space-2)" }}>
        {metricButtons.map(({ key, label }) => (
          <button
            key={key}
            type="button"
            onClick={() => setMetric(key)}
            style={{
              padding: "4px 12px",
              borderRadius: "var(--desk-radius-chip)",
              border: "1px solid var(--desk-outline)",
              background: metric === key ? "var(--desk-primary-container)" : "transparent",
              color: metric === key ? "var(--desk-primary)" : "var(--desk-on-surface-variant)",
              fontSize: "0.75rem",
              fontWeight: metric === key ? 600 : 400,
              cursor: "pointer",
            }}
          >
            {label}
          </button>
        ))}
      </div>

      {ranked.length === 0 ? (
        <DeskEmptyState
          title="No data yet"
          subtitle="Run the Trade Engine to accumulate trade history."
        />
      ) : (
        <DeskDataTable
          columns={columns}
          rows={ranked}
          getRowKey={(row) => String(row.strategyId)}
          zebra
        />
      )}
    </div>
  );
}

// ── Sub-component: Regime Leaderboard tab ─────────────────────────────────────

const REGIME_ICONS: Partial<Record<MarketRegime, string>> = {
  STRONG_TREND: "📈",
  WEAK_TREND: "📉",
  RANGE: "↔️",
  HIGH_VOLATILITY: "⚡",
  LOW_VOLATILITY: "😴",
  BREAKOUT: "🚀",
  REVERSAL: "🔄",
  TRENDING: "📈",
  RANGING: "↔️",
  HIGH_VOLATILITY_BREAKOUT: "⚡",
  LOW_VOLATILITY_CHOP: "😴",
};

function RegimeLeaderboardTab({ report }: { report: InstitutionalResearchReport }) {
  const staticTable = useMemo(() => getStaticStrategyTable(), []);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-5)" }}>
      {/* Best per regime */}
      <div>
        <DeskSectionHeader title="Best Strategy Per Regime (Live)" />
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {report.regimeLeaderboard.map((row) => (
            <DeskCard key={row.regime} style={{ padding: "var(--desk-space-4)" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
                <span style={{ fontSize: "1.25rem" }}>{REGIME_ICONS[row.regime] ?? "📊"}</span>
                <span style={{ fontSize: "0.875rem", fontWeight: 600 }}>
                  {REGIME_LABELS[row.regime] ?? row.regime}
                </span>
              </div>
              {row.regimeTrades === 0 ? (
                <p style={{ fontSize: "0.75rem", color: "var(--desk-on-surface-variant)" }}>
                  No trades yet
                </p>
              ) : (
                <div style={{ display: "flex", flexDirection: "column", gap: 4, fontSize: "0.75rem" }}>
                  <p style={{ fontFamily: "var(--desk-font-mono)", overflow: "hidden", textOverflow: "ellipsis" }}>
                    {row.strategyName.replace(/_/g, " ")}
                  </p>
                  <p>
                    Win:{" "}
                    <span style={{ color: "var(--desk-success)" }}>{fmtPct(row.regimeWinRate)}</span>
                  </p>
                  <p>
                    Exp:{" "}
                    <span style={{ color: row.regimeExpectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
                      {fmtUsd(row.regimeExpectancy)}
                    </span>
                  </p>
                  <p>Trades: {row.regimeTrades}</p>
                </div>
              )}
            </DeskCard>
          ))}
        </div>
      </div>

      {/* Static design regime affinity */}
      <div>
        <DeskSectionHeader title="Designed Regime Affinity (Research)" />
        <div style={{ overflowX: "auto" }}>
          <AutoSortTable><table style={{ width: "100%", fontSize: "0.75rem", borderCollapse: "collapse" }}>
            <thead>
              <tr style={{ borderBottom: "1px solid var(--desk-outline)", color: "var(--desk-on-surface-variant)" }}>
                {["Strategy", "Tier", "Best Regimes", "Avoid", "TP%", "SL%", "Est WR"].map((h) => (
                  <th key={h} style={{ padding: "8px 12px 8px 0", textAlign: "left", fontWeight: 500 }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {staticTable.map((row) => (
                <tr
                  key={row.id}
                  style={{ borderBottom: "1px solid var(--desk-outline)", color: "var(--desk-on-surface)" }}
                >
                  <td style={{ padding: "6px 12px 6px 0", fontFamily: "var(--desk-font-mono)", maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {row.name.replace(/_/g, " ")}
                  </td>
                  <td style={{ padding: "6px 12px 6px 0" }}><TierBadge tier={row.tier} /></td>
                  <td style={{ padding: "6px 12px 6px 0", color: "var(--desk-success)" }}>{row.bestRegimes.replace(/,/g, " ·")}</td>
                  <td style={{ padding: "6px 12px 6px 0", color: "var(--desk-error)" }}>{row.worstRegimes.replace(/,/g, " ·")}</td>
                  <td style={{ padding: "6px 12px 6px 0" }}>{row.tpPct}%</td>
                  <td style={{ padding: "6px 12px 6px 0" }}>{row.slPct}%</td>
                  <td style={{ padding: "6px 0" }}>{fmtPct(row.estimatedWinRate)}</td>
                </tr>
              ))}
            </tbody>
          </table></AutoSortTable>
        </div>
      </div>
    </div>
  );
}

// ── Sub-component: Walk-Forward tab ───────────────────────────────────────────

function WalkForwardTab({ report }: { report: InstitutionalResearchReport }) {
  const summary = useMemo(() => buildWalkForwardSummary(report), [report]);

  type WfRow = (typeof summary)[0];

  const columns: DeskColumn<WfRow>[] = [
    {
      id: "strategyName",
      header: "Strategy",
      cell: (row) => (
        <span style={{ fontFamily: "var(--desk-font-mono)", fontSize: "0.72rem" }}>
          {row.strategyName.replace(/_/g, " ")}
        </span>
      ),
    },
    { id: "tier",   header: "Tier",      cell: (row) => <TierBadge tier={row.tier} /> },
    { id: "status", header: "WF Status", cell: (row) => <WfBadge status={row.status} /> },
    { id: "health", header: "Health",    cell: (row) => <HealthBadge state={row.healthState} /> },
    {
      id: "walkForwardScore",
      header: "WF Score",
      align: "right",
      cell: (row) => (
        <span style={{ color: row.walkForwardScore >= 50 ? "var(--desk-success)" : "var(--desk-on-surface-variant)" }}>
          {row.walkForwardScore.toFixed(0)}
        </span>
      ),
    },
    { id: "windows",  header: "Windows",    align: "right", cell: (row) => String(row.windows) },
    {
      id: "isSampleExpectancy",
      header: "IS Exp",
      align: "right",
      cell: (row) => (
        <span style={{ color: row.inSampleExpectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
          {fmtUsd(row.inSampleExpectancy)}
        </span>
      ),
    },
    {
      id: "outOfSampleExpectancy",
      header: "OOS Exp",
      align: "right",
      cell: (row) => (
        <span style={{ color: row.outOfSampleExpectancy >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
          {fmtUsd(row.outOfSampleExpectancy)}
        </span>
      ),
    },
    { id: "outOfSampleTrades", header: "OOS Trades", align: "right", cell: (row) => String(row.outOfSampleTrades) },
  ];

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-3)" }}>
      <p style={{ fontSize: "0.75rem", color: "var(--desk-on-surface-variant)" }}>
        Walk-forward: 14-day training → 7-day out-of-sample. Strategies need ≥ 10 closed trades to generate windows.
      </p>
      {summary.length === 0 ? (
        <DeskEmptyState title="No walk-forward data" subtitle="Accumulate trade history to run walk-forward validation." />
      ) : (
        <DeskDataTable
          columns={columns}
          rows={summary}
          getRowKey={(row) => String(row.strategyId)}
          zebra
        />
      )}
    </div>
  );
}

// ── Sub-component: Strategy Specs tab ─────────────────────────────────────────

function StrategySpecsTab() {
  const [expanded, setExpanded] = useState<number | null>(null);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-2)" }}>
      {INSTITUTIONAL_STRATEGIES.map((strat) => (
        <DeskCard
          key={strat.id}
          style={{
            padding: "var(--desk-space-3)",
            cursor: "pointer",
            outline: expanded === strat.id ? "1px solid var(--desk-primary)" : undefined,
          }}
          onClick={() => setExpanded(expanded === strat.id ? null : strat.id)}
        >
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}>
              <TierBadge tier={strat.meta.tier} />
              <span style={{ fontFamily: "var(--desk-font-mono)", fontSize: "0.75rem", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {strat.name.replace(/_/g, " ")}
              </span>
            </div>
            <div style={{ display: "flex", gap: 12, fontSize: "0.72rem", color: "var(--desk-on-surface-variant)", flexShrink: 0 }}>
              <span>TP {strat.meta.tpPct}%</span>
              <span>SL {strat.meta.slPct}%</span>
              <span>RR {(strat.meta.tpPct / strat.meta.slPct).toFixed(1)}:1</span>
              <span style={{ color: strat.meta.netExpectancyEstimate >= 0 ? "var(--desk-success)" : "var(--desk-error)" }}>
                Exp {(strat.meta.netExpectancyEstimate * 100).toFixed(3)}%
              </span>
            </div>
          </div>

          {expanded === strat.id && (
            <div style={{ marginTop: "var(--desk-space-3)", display: "flex", flexDirection: "column", gap: "var(--desk-space-3)", fontSize: "0.75rem" }}>
              <p style={{ color: "var(--desk-on-surface-variant)" }}>{strat.description}</p>

              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                {[
                  { label: "Conf Score",          value: String(strat.meta.confidenceScore) },
                  { label: "Signal Score",         value: String(strat.meta.signalScore) },
                  { label: "Risk Score",           value: String(strat.meta.riskScore) },
                  { label: "Est Win Rate",         value: `${(strat.meta.estimatedWinRate * 100).toFixed(0)}%` },
                  { label: "Profit/trade (fees)", value: `${(strat.meta.expectedProfitAfterFees * 100).toFixed(3)}%` },
                  { label: "Loss/trade (fees)",   value: `${(strat.meta.expectedLossAfterFees * 100).toFixed(3)}%` },
                  { label: "Net Expectancy",       value: `${(strat.meta.netExpectancyEstimate * 100).toFixed(4)}%` },
                  { label: "Trail Stop",           value: `${strat.meta.trailingStopPct}%` },
                ].map(({ label, value }) => (
                  <div key={label}>
                    <p style={{ color: "var(--desk-on-surface-variant)", marginBottom: 2 }}>{label}</p>
                    <p style={{ fontWeight: 600 }}>{value}</p>
                  </div>
                ))}
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                {[
                  { title: "Entry Rules", rules: strat.entryRules, color: "var(--desk-primary)" },
                  { title: "Exit Rules",  rules: strat.exitRules,  color: "var(--desk-warning)" },
                  { title: "Stop Loss",   rules: strat.stopLossRules,   color: "var(--desk-error)" },
                  { title: "Take Profit", rules: strat.takeProfitRules, color: "var(--desk-success)" },
                ].map(({ title, rules, color }) => (
                  <div key={title}>
                    <p style={{ fontWeight: 600, marginBottom: 4 }}>{title}</p>
                    <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
                      {rules.map((rule, i) => (
                        <li key={i} style={{ display: "flex", gap: 6, marginBottom: 2 }}>
                          <span style={{ color, flexShrink: 0 }}>›</span>
                          <span style={{ color: "var(--desk-on-surface-variant)" }}>{rule}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>

              <div>
                <p style={{ fontWeight: 600, marginBottom: 4 }}>Trailing Stop Logic</p>
                <p style={{ color: "var(--desk-on-surface-variant)" }}>{strat.meta.trailingStopLogic}</p>
              </div>
            </div>
          )}
        </DeskCard>
      ))}
    </div>
  );
}

// ── Main dashboard component ───────────────────────────────────────────────────

export interface InstitutionalResearchDashboardProps {
  trades: readonly MockTrade[];
  currentRegime: MarketRegime;
}

export function InstitutionalResearchDashboard({
  trades,
  currentRegime,
}: InstitutionalResearchDashboardProps) {
  const report = useMemo(
    () => buildInstitutionalReport(trades, currentRegime),
    [trades, currentRegime],
  );

  type TabKey = "rankings" | "regime" | "walkforward" | "specs";

  const tabs: DeskTabItem<TabKey>[] = [
    { key: "rankings",    label: "Live Rankings" },
    { key: "regime",      label: "Regime Board" },
    { key: "walkforward", label: "Walk-Forward" },
    { key: "specs",       label: "Strategy Specs" },
  ];

  const [activeTab, setActiveTab] = useState<TabKey>("rankings");

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--desk-space-4)" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16 }}>
        <div>
          <h2 style={{ fontFamily: "var(--desk-font-display)", fontSize: "1rem", fontWeight: 600, margin: 0 }}>
            Institutional Research Engine
          </h2>
          <p style={{ fontSize: "0.75rem", color: "var(--desk-on-surface-variant)", marginTop: 2 }}>
            10 BTC futures strategy modules · IDs 2100–2119 · Mock only · No real orders
          </p>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: "0.75rem", color: "var(--desk-on-surface-variant)" }}>
          <span>Regime:</span>
          <DeskChip tone="primary">
            {REGIME_LABELS[currentRegime] ?? currentRegime}
          </DeskChip>
        </div>
      </div>

      {/* Summary tiles */}
      <SummaryTiles report={report} />

      {/* Tabs */}
      <DeskTabs items={tabs} active={activeTab} onChange={setActiveTab} />

      {/* Tab content */}
      <div style={{ minHeight: 200 }}>
        {activeTab === "rankings"    && <LiveRankingsTab report={report} />}
        {activeTab === "regime"      && <RegimeLeaderboardTab report={report} />}
        {activeTab === "walkforward" && <WalkForwardTab report={report} />}
        {activeTab === "specs"       && <StrategySpecsTab />}
      </div>
    </div>
  );
}

export default InstitutionalResearchDashboard;
