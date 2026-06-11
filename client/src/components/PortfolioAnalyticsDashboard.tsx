"use client";

import { useEffect, useState } from "react";
import { Card, Metric } from "@/components/ui/Card";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { SkeletonCard } from "@/components/ui/Skeleton";
import { StatusChip } from "@/components/ui/StatusChip";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import type { TerminalPosition } from "@/lib/terminal/terminalTypes";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { pct, pnlClass, usd } from "@/components/terminal/institutional/format";

type RegimeRow = { regime: string; trades: number; net_pnl: number; win_rate: number };

function metricTone(value: number, good = 0, warn?: number): "positive" | "negative" | "warning" | "neutral" {
  if (value >= good) return "positive";
  if (warn != null && value >= warn) return "warning";
  if (value < 0) return "negative";
  return "neutral";
}

export default function PortfolioAnalyticsDashboard() {
  const snapshot = useTerminalSnapshot();
  const [regimes, setRegimes] = useState<RegimeRow[]>([]);

  useEffect(() => {
    fetch("/api/paper-trades/regime-analysis")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data?.ok && Array.isArray(data.regimes)) setRegimes(data.regimes);
      })
      .catch(() => {});
  }, []);

  const { analytics, risk, positions, hasAuthority } = snapshot;
  const equity =
    analytics.equityCurve.length > 0
      ? analytics.equityCurve[analytics.equityCurve.length - 1]?.equity ?? 0
      : 0;
  const curve = analytics.equityCurve;
  const hasCurve = curve.length > 1;

  if (snapshot.loading && !hasAuthority) {
    return (
      <div className="m3-page-stack">
        <PageHeader title="Portfolio Analytics" subtitle="Loading authority data…" />
        <div className="m3-kpi-strip">
          {Array.from({ length: 4 }).map((_, i) => (
            <SkeletonCard key={i} rows={2} />
          ))}
        </div>
      </div>
    );
  }

  if (!hasAuthority && equity === 0 && positions.length === 0) {
    return (
      <div className="m3-page-stack">
        <PageHeader title="Portfolio Analytics" subtitle="Equity · exposure · drawdown · allocation" />
        <EmptyState
          title="Portfolio data unavailable"
          subtitle="Waiting for mock-trading authority or MongoDB snapshot"
        />
      </div>
    );
  }

  const sparkPoints = curve.slice(-50);
  const sparkMin = sparkPoints.length ? Math.min(...sparkPoints.map((p) => p.equity)) : 0;
  const sparkMax = sparkPoints.length ? Math.max(...sparkPoints.map((p) => p.equity)) : 1;
  const sparkRange = sparkMax - sparkMin || 1;

  const positionColumns: DataTableColumn<TerminalPosition>[] = [
    { id: "strategy", header: "Strategy", cell: (p) => p.strategy, sortable: true, sortValue: (p) => p.strategy },
    { id: "side", header: "Side", cell: (p) => p.side, width: "80px" },
    {
      id: "size",
      header: "Size",
      align: "right",
      cell: (p) => `${p.sizeBtc.toFixed(4)} BTC`,
      sortable: true,
      sortValue: (p) => p.sizeBtc,
    },
    {
      id: "pnl",
      header: "Unrealized PnL",
      align: "right",
      cell: (p) => <span className={pnlClass(p.unrealizedPnl)}>{usd(p.unrealizedPnl, { signed: true })}</span>,
      sortable: true,
      sortValue: (p) => p.unrealizedPnl,
    },
    {
      id: "return",
      header: "Return",
      align: "right",
      cell: (p) => <span className={pnlClass(p.returnPct)}>{pct(p.returnPct)}</span>,
      sortable: true,
      sortValue: (p) => p.returnPct,
    },
  ];

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Portfolio Analytics"
        subtitle="Google Finance + Analytics style · mock-trading authority"
        actions={
          <StatusChip
            label={hasAuthority ? "Live" : "Stale"}
            tone={hasAuthority ? "success" : "error"}
          />
        }
      />

      <div className="m3-kpi-strip">
        <Metric label="Equity" value={equity > 0 ? usd(equity, { compact: true }) : "—"} tone={equity > 0 ? "positive" : "neutral"} />
        <Metric label="Gross Exposure" value={risk.grossExposureUsd > 0 ? usd(risk.grossExposureUsd, { compact: true }) : "—"} />
        <Metric label="Net Exposure" value={usd(risk.netExposureUsd, { compact: true })} tone={metricTone(risk.netExposureUsd)} />
        <Metric label="Drawdown" value={risk.drawdownPct !== 0 ? `${risk.drawdownPct.toFixed(2)}%` : "—"} tone={risk.drawdownPct < -3 ? "negative" : "warning"} />
        <Metric label="Portfolio PF" value={analytics.profitFactorTrend != null ? analytics.profitFactorTrend.toFixed(2) : "—"} tone={metricTone(analytics.profitFactorTrend ?? 0, 1.25, 1)} />
        <Metric label="Sharpe (30d)" value={analytics.rollingSharpe30d != null ? analytics.rollingSharpe30d.toFixed(2) : "—"} />
        <Metric label="Win Rate" value={analytics.winRatePct != null ? `${analytics.winRatePct.toFixed(1)}%` : "—"} tone={metricTone(analytics.winRatePct ?? 0, 50, 40)} />
        <Metric label="Open Positions" value={String(positions.length)} tone={positions.length > 0 ? "warning" : "neutral"} />
      </div>

      <div className="m3-portfolio-grid">
        <Card title="Equity Curve" subtitle="MongoDB authority · last 96 snapshots">
          {!hasCurve ? (
            <TerminalNoData label="No equity curve data" />
          ) : (
            <div className="m3-chart-theme m3-equity-chart">
              <svg viewBox={`0 0 ${Math.max(sparkPoints.length, 2)} 60`} preserveAspectRatio="none" role="img" aria-label="Equity curve">
                <polyline
                  fill="none"
                  stroke="var(--chart-line, var(--m3-primary))"
                  strokeWidth="1.5"
                  points={sparkPoints.map((p, i) => `${i},${60 - ((p.equity - sparkMin) / sparkRange) * 56}`).join(" ")}
                />
              </svg>
              <div className="m3-equity-chart__axis">
                <span>{new Date(sparkPoints[0]?.time ?? "").toLocaleDateString()}</span>
                <span>{new Date(sparkPoints[sparkPoints.length - 1]?.time ?? "").toLocaleDateString()}</span>
              </div>
            </div>
          )}
        </Card>

        <Card title="Risk & Exposure" subtitle="VaR · margin · funding">
          <div className="m3-kpi-strip m3-kpi-strip--2col">
            <Metric label="VaR 95" value={risk.var95Usd !== 0 ? usd(-risk.var95Usd) : "—"} tone="warning" />
            <Metric label="Heat" value={`${risk.heatPct.toFixed(1)}%`} tone={risk.heatPct > 70 ? "negative" : "warning"} />
            <Metric label="Margin Usage" value={`${risk.marginUsagePct.toFixed(1)}%`} />
            <Metric label="Long Exp" value={usd(risk.longExposureUsd, { compact: true })} tone="positive" />
            <Metric label="Short Exp" value={usd(risk.shortExposureUsd, { compact: true })} tone="negative" />
            <Metric label="Fee Drag" value={analytics.feeDragUsd != null ? usd(-analytics.feeDragUsd) : "—"} tone="warning" />
          </div>
        </Card>
      </div>

      <Card title="Open Positions" subtitle={`${positions.length} live marks from engine`}>
        {positions.length === 0 ? (
          <EmptyState title="No open positions" subtitle="Positions appear when mock-trading strategies are active" />
        ) : (
          <DataTable
            columns={positionColumns}
            rows={positions}
            getRowKey={(p) => p.id}
            density="compact"
            pageSize={25}
          />
        )}
      </Card>

      {regimes.length > 0 ? (
        <Card title="Regime Breakdown" subtitle="/api/paper-trades/regime-analysis">
          <div className="m3-regime-list">
            {regimes.map((r) => (
              <div key={r.regime} className="m3-regime-row">
                <span className="m3-regime-row__name">{r.regime}</span>
                <span className={`m3-regime-row__stats ${pnlClass(r.net_pnl)}`}>
                  {usd(r.net_pnl, { signed: true })} · {(r.win_rate * 100).toFixed(0)}% · {r.trades}t
                </span>
              </div>
            ))}
          </div>
        </Card>
      ) : null}
    </div>
  );
}
