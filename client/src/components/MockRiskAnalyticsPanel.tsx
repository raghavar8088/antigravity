"use client";

import { useMemo } from "react";
import type { MockTrade, MockAccountState, MockTradingConfig } from "@/lib/mockTradingEngine";
import { computeExtendedMetrics, computeDailyPnl } from "@/lib/mockExtendedMetrics";
import { runStressTest, buildScenarioComparisonTable } from "@/lib/mockStressTest";
import { computeQualityAggregate } from "@/lib/mockTradeQualityScorer";
import {
  DeskCard,
  DeskSectionHeader,
  DeskMetricTile,
  DeskChip,
} from "@/components/desk/ui";

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtUsd(v: number): string {
  if (!Number.isFinite(v)) return "—";
  const abs = Math.abs(v);
  if (abs >= 1_000) return `${v >= 0 ? "+" : ""}$${(v / 1_000).toFixed(1)}K`;
  return `${v >= 0 ? "+" : ""}$${v.toFixed(0)}`;
}

function fmtPct(v: number, places = 2): string {
  if (!Number.isFinite(v)) return "—";
  return `${v >= 0 ? "+" : ""}${(v * 100).toFixed(places)}%`;
}

// ── Drawdown gauge ────────────────────────────────────────────────────────────

function DrawdownGauge({ value, max, label }: { value: number; max: number; label: string }) {
  const frac = Math.min(1, value / Math.max(0.01, max));
  const color = frac < 0.5 ? "var(--desk-profit)" : frac < 0.8 ? "#f59e0b" : "var(--desk-loss)";
  return (
    <div className="flex flex-col gap-1">
      <div className="flex justify-between text-xs">
        <span className="text-[var(--desk-muted)]">{label}</span>
        <span className="font-medium" style={{ color }}>{fmtPct(value, 1)}</span>
      </div>
      <div className="h-1.5 rounded-full bg-[var(--desk-surface-2)] overflow-hidden">
        <div className="h-full rounded-full transition-all" style={{ width: `${frac * 100}%`, backgroundColor: color }} />
      </div>
      <div className="flex justify-between text-[10px] text-[var(--desk-muted)]">
        <span>0%</span><span>Max: {fmtPct(max, 0)}</span>
      </div>
    </div>
  );
}

// ── Kill-switch status ────────────────────────────────────────────────────────

interface KillSwitchStatus {
  active: boolean;
  reason: string;
  triggered: string[];
  safe: string[];
}

function computeKillSwitchStatus(
  account: MockAccountState | null,
  config: MockTradingConfig,
  dailyPnl: ReturnType<typeof computeDailyPnl>,
): KillSwitchStatus {
  const triggered: string[] = [];
  const safe: string[] = [];

  if (!account) {
    return { active: false, reason: "No account data", triggered, safe };
  }

  const todayPnl = dailyPnl.at(-1)?.pnl ?? 0;
  const dailyLimit = -account.startingBalance * (config.dailyLossLimitPct / 100);
  if (todayPnl <= dailyLimit) triggered.push(`Daily loss limit ${fmtPct(config.dailyLossLimitPct / 100, 0)} breached (${fmtUsd(todayPnl)})`);
  else safe.push(`Daily loss ${fmtUsd(todayPnl)} / limit ${fmtUsd(dailyLimit)}`);

  const maxDdLimit = config.maxDrawdownPct / 100;
  if (account.maxDrawdownPct >= maxDdLimit) triggered.push(`Max drawdown ${fmtPct(account.maxDrawdownPct, 1)} breached`);
  else safe.push(`Drawdown ${fmtPct(account.maxDrawdownPct, 1)} / limit ${fmtPct(maxDdLimit, 0)}`);

  const marginRatio = account.marginUsed / Math.max(1, account.equity);
  if (marginRatio > 0.8) triggered.push(`Margin usage critical (${fmtPct(marginRatio, 0)} of equity)`);
  else safe.push(`Margin usage ${fmtPct(marginRatio, 0)}`);

  const active = triggered.length > 0;
  const reason = active ? `${triggered.length} kill-switch condition(s) active` : "All systems nominal";
  return { active, reason, triggered, safe };
}

// ── Stress test summary strip ─────────────────────────────────────────────────

function StressTestStrip({ trades }: { trades: readonly MockTrade[] }) {
  const report = useMemo(() => runStressTest({ trades }), [trades]);
  if (!report) {
    return (
      <p className="text-xs text-[var(--desk-muted)] py-2">
        Need ≥ 10 closed trades for stress testing.
      </p>
    );
  }

  const table = buildScenarioComparisonTable(report);
  const severityColor: Record<string, string> = {
    LOW: "var(--desk-profit)",
    MODERATE: "#f59e0b",
    SEVERE: "var(--desk-loss)",
    CRITICAL: "#dc2626",
  };

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs border-separate border-spacing-y-0.5">
        <thead>
          <tr className="text-[10px] text-[var(--desk-muted)]">
            <th className="text-left py-1 px-2">Scenario</th>
            <th className="text-right px-2">Net PnL</th>
            <th className="text-right px-2">Win Rate</th>
            <th className="text-right px-2">Max DD</th>
            <th className="text-right px-2">Severity</th>
          </tr>
        </thead>
        <tbody>
          {table.map((row) => (
            <tr key={row.scenario} className="hover:bg-[var(--desk-surface-2)] transition-colors">
              <td className="py-1 px-2 rounded-l-md">{row.label}</td>
              <td className={`text-right px-2 tabular-nums font-medium ${row.netPnl >= 0 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}>
                {fmtUsd(row.netPnl)}
              </td>
              <td className="text-right px-2 tabular-nums">{(row.winRate * 100).toFixed(1)}%</td>
              <td className="text-right px-2 tabular-nums">{(row.maxDrawdownPct * 100).toFixed(1)}%</td>
              <td className="text-right px-2 rounded-r-md">
                <span className="text-[10px] font-bold" style={{ color: severityColor[row.severity] }}>
                  {row.severity}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-[10px] text-[var(--desk-muted)] mt-1">
        Most vulnerable: <strong>{report.summary.mostVulnerableScenario.replace(/_/g, " ")}</strong> |
        Most resilient: <strong>{report.summary.mostResilientScenario.replace(/_/g, " ")}</strong>
      </p>
    </div>
  );
}

// ── Trade quality summary ─────────────────────────────────────────────────────

function TradeQualitySummary({ trades }: { trades: readonly MockTrade[] }) {
  const quality = useMemo(() => computeQualityAggregate(trades), [trades]);
  if (trades.length === 0) return <p className="text-xs text-[var(--desk-muted)]">No trades yet.</p>;

  const gradeColor: Record<string, string> = {
    A: "var(--desk-profit)", B: "#22c55e", C: "#f59e0b", D: "#f97316", F: "var(--desk-loss)",
  };

  return (
    <div className="flex flex-col gap-3">
      {/* Grade distribution */}
      <div className="flex gap-2 flex-wrap">
        {(["A", "B", "C", "D", "F"] as const).map((g) => (
          <div key={g} className="flex flex-col items-center px-2 py-1 rounded-md bg-[var(--desk-surface-2)]">
            <span className="text-sm font-bold" style={{ color: gradeColor[g] }}>{g}</span>
            <span className="text-[10px] text-[var(--desk-muted)]">{quality.gradeDistribution[g]}</span>
          </div>
        ))}
        <div className="flex flex-col items-center px-2 py-1 rounded-md bg-[var(--desk-surface-2)]">
          <span className="text-sm font-bold">{quality.averageOverall.toFixed(1)}</span>
          <span className="text-[10px] text-[var(--desk-muted)]">Avg Score</span>
        </div>
      </div>

      {/* Component breakdown */}
      <div className="grid grid-cols-5 gap-1 text-center">
        {[
          { label: "Entry", value: quality.averageEntry, max: 25 },
          { label: "Exit", value: quality.averageExit, max: 25 },
          { label: "Risk", value: quality.averageRisk, max: 25 },
          { label: "Timing", value: quality.averageTiming, max: 15 },
          { label: "Exec", value: quality.averageExecution, max: 10 },
        ].map((c) => (
          <div key={c.label} className="flex flex-col gap-0.5">
            <span className="text-[10px] text-[var(--desk-muted)]">{c.label}</span>
            <span className="text-xs font-medium">{c.value.toFixed(1)}</span>
            <div className="h-1 rounded-full bg-[var(--desk-surface-2)] overflow-hidden">
              <div
                className="h-full rounded-full bg-[var(--desk-accent)]"
                style={{ width: `${(c.value / c.max) * 100}%` }}
              />
            </div>
            <span className="text-[10px] text-[var(--desk-muted)]">/{c.max}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Health monitor ────────────────────────────────────────────────────────────

function HealthMonitor({ metrics }: { metrics: ReturnType<typeof computeExtendedMetrics> }) {
  const checks = [
    {
      label: "Win Rate",
      ok: metrics.winRate >= 0.45,
      value: `${(metrics.winRate * 100).toFixed(1)}%`,
      threshold: "≥ 45%",
    },
    {
      label: "Profit Factor",
      ok: (metrics.profitFactor ?? 0) >= 1.2,
      value: metrics.profitFactor?.toFixed(2) ?? "—",
      threshold: "≥ 1.2",
    },
    {
      label: "Sharpe Ratio",
      ok: (metrics.sharpeRatio ?? -99) >= 0.5,
      value: metrics.sharpeRatio?.toFixed(2) ?? "—",
      threshold: "≥ 0.5",
    },
    {
      label: "Max Drawdown",
      ok: metrics.maxDrawdownPct < 0.2,
      value: `${(metrics.maxDrawdownPct * 100).toFixed(1)}%`,
      threshold: "< 20%",
    },
    {
      label: "Expectancy",
      ok: metrics.expectancy > 0,
      value: fmtUsd(metrics.expectancy),
      threshold: "> $0",
    },
    {
      label: "Recovery Factor",
      ok: (metrics.recoveryFactor ?? 0) >= 1.0,
      value: metrics.recoveryFactor?.toFixed(2) ?? "—",
      threshold: "≥ 1.0",
    },
  ];

  const passCount = checks.filter((c) => c.ok).length;
  const allPass = passCount === checks.length;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <div
          className="w-2 h-2 rounded-full"
          style={{ backgroundColor: allPass ? "var(--desk-profit)" : passCount >= 4 ? "#f59e0b" : "var(--desk-loss)" }}
        />
        <span className="text-xs font-medium">
          Health Score: {passCount}/{checks.length} checks passing
        </span>
        <DeskChip
          label={allPass ? "HEALTHY" : passCount >= 4 ? "WARNING" : "DEGRADED"}
          variant={allPass ? "success" : passCount >= 4 ? "warning" : "danger"}
        />
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
        {checks.map((c) => (
          <div
            key={c.label}
            className="flex items-center justify-between px-2 py-1.5 rounded-md bg-[var(--desk-surface-2)] gap-2"
          >
            <div>
              <p className="text-[10px] text-[var(--desk-muted)]">{c.label}</p>
              <p className={`text-xs font-semibold ${c.ok ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}>
                {c.value}
              </p>
            </div>
            <span className="text-[10px] text-[var(--desk-muted)] shrink-0">{c.threshold}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Main panel ────────────────────────────────────────────────────────────────

interface MockRiskAnalyticsPanelProps {
  trades: readonly MockTrade[];
  account: MockAccountState | null;
  config: MockTradingConfig;
  startingEquityUsd?: number;
}

export function MockRiskAnalyticsPanel({
  trades,
  account,
  config,
  startingEquityUsd = 1_000_000,
}: MockRiskAnalyticsPanelProps) {
  const metrics = useMemo(() => computeExtendedMetrics(trades, startingEquityUsd), [trades, startingEquityUsd]);
  const dailyPnl = useMemo(() => computeDailyPnl(trades), [trades]);
  const ks = useMemo(() => computeKillSwitchStatus(account, config, dailyPnl), [account, config, dailyPnl]);

  const currentEquity = account?.equity ?? startingEquityUsd;
  const currentDdPct = account != null
    ? (account.peakEquity - currentEquity) / Math.max(1, account.peakEquity)
    : 0;

  return (
    <div className="flex flex-col gap-4">
      {/* Kill-switch status */}
      <DeskCard className={ks.active ? "border border-[var(--desk-loss)]" : ""}>
        <div className="flex items-center justify-between">
          <DeskSectionHeader title="Kill-Switch Monitor" />
          <DeskChip
            label={ks.active ? "TRIGGERED" : "NOMINAL"}
            variant={ks.active ? "danger" : "success"}
          />
        </div>
        <p className="text-xs text-[var(--desk-muted)] mt-1">{ks.reason}</p>
        <div className="mt-2 flex flex-col gap-1">
          {ks.triggered.map((t, i) => (
            <div key={i} className="flex items-start gap-1.5 text-xs text-[var(--desk-loss)]">
              <span>✕</span><span>{t}</span>
            </div>
          ))}
          {ks.safe.map((s, i) => (
            <div key={i} className="flex items-start gap-1.5 text-xs text-[var(--desk-muted)]">
              <span>✓</span><span>{s}</span>
            </div>
          ))}
        </div>
      </DeskCard>

      {/* Drawdown gauges */}
      <DeskCard>
        <DeskSectionHeader title="Drawdown Limits" />
        <div className="flex flex-col gap-3 mt-2">
          <DrawdownGauge
            value={currentDdPct}
            max={config.maxDrawdownPct / 100}
            label="Current Drawdown"
          />
          <DrawdownGauge
            value={metrics.maxDrawdownPct}
            max={config.maxDrawdownPct / 100}
            label="Historical Max Drawdown"
          />
        </div>
      </DeskCard>

      {/* Health monitor */}
      <DeskCard>
        <DeskSectionHeader title="Strategy Health" />
        <div className="mt-2">
          {metrics.totalTrades >= 5
            ? <HealthMonitor metrics={metrics} />
            : <p className="text-xs text-[var(--desk-muted)]">Need ≥ 5 closed trades to assess health.</p>
          }
        </div>
      </DeskCard>

      {/* Trade quality */}
      <DeskCard>
        <DeskSectionHeader title="Trade Quality Analytics" />
        <div className="mt-2">
          <TradeQualitySummary trades={trades} />
        </div>
      </DeskCard>

      {/* Open position analytics */}
      <DeskCard>
        <DeskSectionHeader title="Open Position Analytics" />
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mt-2">
          <DeskMetricTile
            label="Open Positions"
            value={String(account?.openCount ?? 0)}
            sub={`Max: ${config.maxOpenMockTrades}`}
            subColor="neutral"
          />
          <DeskMetricTile
            label="Open Exposure"
            value={`$${((account?.exposure ?? 0) / 1_000).toFixed(0)}K`}
            sub="Total notional"
            subColor="neutral"
          />
          <DeskMetricTile
            label="Margin Used"
            value={`$${((account?.marginUsed ?? 0) / 1_000).toFixed(0)}K`}
            sub={`${((account?.marginUsed ?? 0) / Math.max(1, account?.equity ?? 1) * 100).toFixed(1)}% of equity`}
            subColor="neutral"
          />
          <DeskMetricTile
            label="Unrealized PnL"
            value={fmtUsd(account?.unrealizedPnl ?? 0)}
            sub="Net of fees"
            subColor={((account?.unrealizedPnl ?? 0)) >= 0 ? "profit" : "loss"}
          />
        </div>
      </DeskCard>

      {/* Stress test */}
      <DeskCard>
        <DeskSectionHeader title="Stress Test Scenarios" />
        <div className="mt-2">
          <StressTestStrip trades={trades} />
        </div>
      </DeskCard>
    </div>
  );
}
