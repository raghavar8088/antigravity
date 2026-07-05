"use client";

import { useEffect, useState } from "react";
import { Card, Metric } from "@/components/ui/Card";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { SkeletonCard } from "@/components/ui/Skeleton";
import { Sparkline } from "@/components/ui/Sparkline";
import { StatusChip } from "@/components/ui/StatusChip";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import type { JournalTrade, TerminalPosition } from "@/lib/terminal/terminalTypes";
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
import { pct, pnlClass, usd } from "@/components/terminal/institutional/format";

type RegimeRow = { regime: string; trades: number; net_pnl: number; win_rate: number };

function metricTone(value: number, good = 0, warn?: number): "positive" | "negative" | "warning" | "neutral" {
  if (value >= good) return "positive";
  if (warn != null && value >= warn) return "warning";
  if (value < 0) return "negative";
  return "neutral";
}

function dateLabel(value: string | undefined) {
  if (!value) return "";
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) return "";
  return new Date(parsed).toLocaleDateString();
}

function metricUsd(value: number | null | undefined, options?: { compact?: boolean; signed?: boolean }) {
  return usd(typeof value === "number" && Number.isFinite(value) ? value : 0, options);
}

function metricPct(value: number | null | undefined, decimals = 1) {
  return `${(typeof value === "number" && Number.isFinite(value) ? value : 0).toFixed(decimals)}%`;
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
        <PageHeader title="Portfolio" subtitle="Loading Trade Engine portfolio analytics" />
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
        <PageHeader title="Portfolio" subtitle="Trade Engine portfolio analytics" />
        <EmptyState
          title="Portfolio data unavailable"
          subtitle="Waiting for Trade Engine authority or MongoDB snapshot"
        />
      </div>
    );
  }

  const sparkPoints = curve.slice(-50);

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

  const closedTradeColumns: DataTableColumn<JournalTrade>[] = [
    { id: "strategy", header: "Strategy", cell: (t) => t.strategy, sortable: true, sortValue: (t) => t.strategy },
    { id: "side", header: "Side", cell: (t) => t.side, width: "80px" },
    { id: "entry", header: "Entry", align: "right", cell: (t) => usd(t.entryPrice), sortable: true, sortValue: (t) => t.entryPrice },
    { id: "exit", header: "Exit", align: "right", cell: (t) => usd(t.exitPrice), sortable: true, sortValue: (t) => t.exitPrice },
    {
      id: "pnl",
      header: "PnL",
      align: "right",
      cell: (t) => <span className={pnlClass(t.netPnl)}>{usd(t.netPnl, { signed: true })}</span>,
      sortable: true,
      sortValue: (t) => t.netPnl,
    },
    { id: "reason", header: "Reason", cell: (t) => t.exitReason || "Closed" },
  ];

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Portfolio"
        subtitle="Trade Engine portfolio analytics"
        actions={
          <StatusChip
            label={hasAuthority ? "Live" : "Stale"}
            tone={hasAuthority ? "success" : "error"}
          />
        }
      />

      <Card title="Portfolio Overview" subtitle="Equity, PnL, and exposure at a glance">
        <div className="m3-kpi-strip">
          <Metric label="Current Equity" value={metricUsd(equity, { compact: true })} tone={equity > 0 ? "positive" : "neutral"} />
          <Metric label="Realized PnL" value={metricUsd(snapshot.journal.reduce((sum, trade) => sum + trade.netPnl, 0), { signed: true, compact: true })} tone={metricTone(snapshot.journal.reduce((sum, trade) => sum + trade.netPnl, 0))} />
          <Metric label="Unrealized PnL" value={metricUsd(positions.reduce((sum, position) => sum + position.unrealizedPnl, 0), { signed: true, compact: true })} tone={metricTone(positions.reduce((sum, position) => sum + position.unrealizedPnl, 0))} />
          <Metric label="Win Rate" value={analytics.winRatePct != null ? metricPct(analytics.winRatePct, 1) : "0.0%"} tone={metricTone(analytics.winRatePct ?? 0, 50, 40)} />
          <Metric label="Drawdown" value={metricPct(risk.drawdownPct, 2)} tone={risk.drawdownPct < -3 ? "negative" : "neutral"} />
        </div>
      </Card>

      <div className="m3-kpi-strip">
        <Metric label="Equity" value={metricUsd(equity, { compact: true })} tone={equity > 0 ? "positive" : "neutral"} />
        <Metric label="Gross Exposure" value={metricUsd(risk.grossExposureUsd, { compact: true })} />
        <Metric label="Net Exposure" value={metricUsd(risk.netExposureUsd, { compact: true })} tone={metricTone(risk.netExposureUsd)} />
        <Metric label="Drawdown" value={metricPct(risk.drawdownPct, 2)} tone={risk.drawdownPct < -3 ? "negative" : "neutral"} />
        <Metric label="Portfolio PF" value={(analytics.profitFactorTrend ?? 0).toFixed(2)} tone={metricTone(analytics.profitFactorTrend ?? 0, 1.25, 1)} />
        <Metric label="Sharpe (30d)" value={(analytics.rollingSharpe30d ?? 0).toFixed(2)} />
        <Metric label="Win Rate" value={analytics.winRatePct != null ? metricPct(analytics.winRatePct, 1) : "0.0%"} tone={metricTone(analytics.winRatePct ?? 0, 50, 40)} />
        <Metric label="Open Positions" value={String(positions.length)} tone={positions.length > 0 ? "warning" : "neutral"} />
      </div>

      <div className="m3-portfolio-grid">
        <Card title="Equity Chart" subtitle="Last 96 portfolio snapshots">
          {!hasCurve ? (
            <TerminalNoData label="No equity curve data" />
          ) : (
            <div className="m3-chart-theme m3-equity-chart">
              <Sparkline
                data={sparkPoints.map((p) => p.equity)}
                width={Math.max(sparkPoints.length, 2) * 10}
                height={60}
                color="var(--chart-line, var(--m3-primary))"
                stretch
              />
              <div className="m3-equity-chart__axis">
                <span>{dateLabel(sparkPoints[0]?.time) || "Start"}</span>
                <span>{dateLabel(sparkPoints[sparkPoints.length - 1]?.time) || "Latest"}</span>
              </div>
            </div>
          )}
        </Card>

        <Card title="Risk & Exposure" subtitle="VaR · margin · funding">
          <div className="m3-kpi-strip m3-kpi-strip--2col">
            <Metric label="VaR 95" value={metricUsd(-risk.var95Usd)} tone="warning" />
            <Metric label="Heat" value={metricPct(risk.heatPct, 1)} tone={risk.heatPct > 70 ? "negative" : "neutral"} />
            <Metric label="Margin Usage" value={metricPct(risk.marginUsagePct, 1)} />
            <Metric label="Long Exp" value={metricUsd(risk.longExposureUsd, { compact: true })} tone="positive" />
            <Metric label="Short Exp" value={metricUsd(risk.shortExposureUsd, { compact: true })} tone="negative" />
            <Metric label="Fee Drag" value={metricUsd(-(analytics.feeDragUsd ?? 0))} tone="warning" />
          </div>
        </Card>
      </div>

      <Card title="Open Positions" subtitle={`${positions.length} live marks from Trade Engine`}>
        {positions.length === 0 ? (
          <EmptyState title="No open positions" subtitle="Positions appear when Trade Engine strategies are active" />
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

      <Card title="Recent Closed Trades" subtitle="Latest journal entries from Trade Engine">
        {snapshot.journal.length === 0 ? (
          <EmptyState title="No closed trades yet" subtitle="Closed trades will appear here after positions exit." />
        ) : (
          <DataTable
            columns={closedTradeColumns}
            rows={snapshot.journal.slice(0, 25)}
            getRowKey={(trade) => trade.id}
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
