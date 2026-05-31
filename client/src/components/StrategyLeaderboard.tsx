"use client";

import { useMemo } from "react";
import { TerminalPanel } from "./terminal/TerminalPanel";
import { DeskDataTable, type DeskColumn } from "./desk/ui";
import { fmtUsd, fmtPct, pnlClass } from "@/lib/utils";
import type { MockTrade, MockStrategyAggregate } from "@/lib/mockTradingEngine";
import { computeAnalytics } from "@/lib/mockTradingEngine";

interface StrategyLeaderboardProps {
  trades: readonly MockTrade[];
}

const COLUMNS: DeskColumn<MockStrategyAggregate>[] = [
  {
    id: "strategy",
    header: "Strategy",
    cell: (row) => (
      <div style={{ display: "flex", flexDirection: "column" }}>
        <span style={{ fontWeight: 700, fontSize: 12 }}>{row.strategyName}</span>
        <span style={{ fontSize: 10, color: "var(--text-muted)" }}>ID: {row.strategyId}</span>
      </div>
    ),
  },
  {
    id: "trades",
    header: "Trades",
    align: "right",
    cell: (row) => (
      <div style={{ fontSize: 11 }}>
        {row.closed} <span style={{ color: "var(--text-muted)" }}>({row.open} open)</span>
      </div>
    ),
  },
  {
    id: "winRate",
    header: "Win Rate",
    align: "right",
    cell: (row) => (
      <div style={{ fontSize: 11, fontWeight: 600 }}>
        {fmtPct(row.winRate * 100)}
      </div>
    ),
  },
  {
    id: "realizedPnl",
    header: "Realized PnL",
    align: "right",
    cell: (row) => (
      <div className={pnlClass(row.realizedPnl)} style={{ fontSize: 11, fontWeight: 700 }}>
        {fmtUsd(row.realizedPnl)}
      </div>
    ),
  },
  {
    id: "unrealizedPnl",
    header: "Unrealized",
    align: "right",
    cell: (row) => (
      <div className={pnlClass(row.unrealizedPnl)} style={{ fontSize: 11 }}>
        {fmtUsd(row.unrealizedPnl)}
      </div>
    ),
  },
  {
    id: "totalPnl",
    header: "Total PnL",
    align: "right",
    cell: (row) => (
      <div className={pnlClass(row.totalPnl)} style={{ fontSize: 12, fontWeight: 800 }}>
        {fmtUsd(row.totalPnl)}
      </div>
    ),
  },
];

export function StrategyLeaderboard({ trades }: StrategyLeaderboardProps) {
  const analytics = useMemo(() => computeAnalytics(trades), [trades]);

  return (
    <TerminalPanel
      title="Strategy Leaderboard"
      subtitle="Institutional ranking by absolute performance"
      padding="none"
    >
      <DeskDataTable
        rows={analytics.perStrategy}
        columns={COLUMNS}
        getRowKey={(row) => String(row.strategyId)}
        empty={
          <div style={{ padding: 20, textAlign: "center", color: "var(--text-muted)", fontSize: 12 }}>
            No strategy data available yet.
          </div>
        }
      />
    </TerminalPanel>
  );
}
