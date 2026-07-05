"use client";

import { useEffect, useRef, useState } from "react";
import { PageHeader } from "@/components/ui/PageHeader";
import { Metric } from "@/components/ui/Card";
import { Badge } from "@/components/ui/StatusChip";
import { ErrorBanner } from "@/components/ui/ErrorBanner";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";

type StrategyRow = {
  name: string;
  category: string;
  timeframe: string;
  trades: number;
  wins: number;
  losses: number;
  winRate: number;
  pnl: number;
  signalsFired: number;
};

async function fetchStats(): Promise<StrategyRow[]> {
  const r = await fetch("/api/pre-live/api/strategies/stats", { cache: "no-store" });
  if (!r.ok) throw new Error(`${r.status}`);
  return r.json();
}

function usd(v: number) {
  const abs = Math.abs(v);
  const s = abs >= 1000 ? `$${(abs / 1000).toFixed(2)}k` : `$${abs.toFixed(2)}`;
  return v < 0 ? `-${s}` : s;
}

function pct(v: number) {
  return `${v.toFixed(1)}%`;
}

function pnlCls(v: number) {
  return v > 0 ? "text-emerald-400" : v < 0 ? "text-rose-400" : "text-zinc-400";
}

export function PreLiveStrategiesCenter() {
  const [rows, setRows] = useState<StrategyRow[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = async () => {
    try {
      const data = await fetchStats();
      setRows(data);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to reach Pre-Live Engine");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    intervalRef.current = setInterval(load, 5000);
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, []);

  const totals = rows.reduce(
    (acc, r) => ({
      signals: acc.signals + r.signalsFired,
      trades: acc.trades + r.trades,
      wins: acc.wins + r.wins,
      losses: acc.losses + r.losses,
      pnl: acc.pnl + r.pnl,
    }),
    { signals: 0, trades: 0, wins: 0, losses: 0, pnl: 0 }
  );
  const overallWR = totals.trades > 0 ? (totals.wins / totals.trades) * 100 : 0;

  const columns: DataTableColumn<StrategyRow>[] = [
    { id: "name", header: "Strategy", sortable: true, sortValue: (r) => r.name, cell: (r) => <span title={r.name}>{r.name}</span> },
    {
      id: "category",
      header: "Cat / TF",
      cell: (r) => (
        <span>
          <span style={{ color: "var(--text-secondary)" }}>{r.category}</span>
          <span style={{ color: "var(--text-muted)" }}> · {r.timeframe}</span>
        </span>
      ),
    },
    { id: "signals", header: "Signals", align: "right", sortable: true, sortValue: (r) => r.signalsFired, cell: (r) => r.signalsFired },
    { id: "trades", header: "Trades", align: "right", sortable: true, sortValue: (r) => r.trades, cell: (r) => r.trades },
    { id: "wins", header: "Wins", align: "right", sortable: true, sortValue: (r) => r.wins, cell: (r) => <span style={{ color: "var(--green)" }}>{r.wins}</span> },
    { id: "losses", header: "Losses", align: "right", sortable: true, sortValue: (r) => r.losses, cell: (r) => <span style={{ color: "var(--red)" }}>{r.losses}</span> },
    {
      id: "winRate",
      header: "Win %",
      align: "right",
      sortable: true,
      sortValue: (r) => r.winRate,
      cell: (r) => <span className={r.winRate >= 50 ? "font-semibold" : ""} style={{ color: r.winRate >= 50 ? "var(--green)" : r.winRate > 0 ? "var(--amber)" : "var(--text-muted)" }}>{r.trades > 0 ? pct(r.winRate) : "—"}</span>,
    },
    {
      id: "pnl",
      header: "PnL",
      align: "right",
      sortable: true,
      sortValue: (r) => r.pnl,
      cell: (r) => <span className={pnlCls(r.pnl)}>{r.trades > 0 ? usd(r.pnl) : "—"}</span>,
    },
  ];

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Pre-Live Strategies"
        subtitle="100 backtested-qualified strategies · signal telemetry · live paper PnL"
        actions={<Badge variant="warning" size="md">Paper Trading · 100 Strategies</Badge>}
      />

      {error ? (
        <ErrorBanner message={`Pre-Live Engine offline — ${error}. Start it with: cd engine && go run ./cmd/pre_live/main.go`} onRetry={load} />
      ) : null}

      <div className="m3-kpi-strip">
        <Metric label="Strategies" value={`${rows.length} / 100`} tone="positive" />
        <Metric label="Total Signals" value={totals.signals.toLocaleString()} />
        <Metric label="Total Trades" value={String(totals.trades)} />
        <Metric label="Win Rate" value={pct(overallWR)} tone={overallWR >= 50 ? "positive" : overallWR > 0 ? "warning" : "neutral"} />
        <Metric label="Wins" value={String(totals.wins)} tone="positive" />
        <Metric label="Losses" value={String(totals.losses)} tone="negative" />
        <Metric label="Total PnL" value={usd(totals.pnl)} tone={totals.pnl > 0 ? "positive" : totals.pnl < 0 ? "negative" : "neutral"} />
      </div>

      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(r) => r.name}
        loading={loading}
        searchable
        searchPlaceholder="Search strategy or category…"
        searchFilter={(r, q) => r.name.toLowerCase().includes(q) || r.category.toLowerCase().includes(q)}
        emptyTitle="No strategies match your search"
        density="compact"
      />

      <p className="text-[10px]" style={{ color: "var(--text-muted)" }}>
        Refreshes every 5s · $100 paper balance · All 100 strategies evaluate every market tick
      </p>
    </div>
  );
}
