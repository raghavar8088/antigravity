"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Metric, TerminalCard } from "./TerminalCard";
import { pnlClass, usd } from "./format";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorBanner } from "@/components/ui/ErrorBanner";
import { Badge } from "@/components/ui/StatusChip";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";

// Mirrors engine/internal/trading/scalers_eval.go: ScalersStrategyStats /
// ScalersSnapshot — the REAL curated scalper registry (BuildCuratedScalpers,
// 30 strategies as of the S10-S29 expansion), not a mock/static roster.
type ScalerStrategyStat = {
  name: string;
  total_trades: number;
  wins: number;
  win_rate: number;
  active: boolean;
  last_pnl: number;
};

type ScalerSignalSnapshot = {
  id: string;
  strategy: string;
  side: string;
  confidence: number;
  price: number;
  timestamp: string;
};

type ScalersStatsResponse = {
  ok?: boolean;
  error?: string;
  strategies: ScalerStrategyStat[];
  regime: string;
  cvd: number;
  eval_count: number;
  last_eval_at?: string;
  raw_signals_last_cycle: number;
  approved_signals_last_cycle: number;
  rejected_signals_last_cycle: number;
  recent_signals: ScalerSignalSnapshot[];
};

function rate(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}

function int(value: number) {
  return value.toLocaleString("en-US");
}

function relativeTime(iso?: string) {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  const diffSec = Math.round((Date.now() - d.getTime()) / 1000);
  if (diffSec < 5) return "just now";
  if (diffSec < 60) return `${diffSec}s ago`;
  if (diffSec < 3600) return `${Math.round(diffSec / 60)}m ago`;
  return `${Math.round(diffSec / 3600)}h ago`;
}

// Active strategies first, then by last realized PnL, then by trade count,
// then alphabetically — exported for unit testing.
export function sortScalerStrategies(rows: readonly ScalerStrategyStat[]): ScalerStrategyStat[] {
  return [...rows].sort((a, b) => {
    if (a.active !== b.active) return a.active ? -1 : 1;
    if (b.last_pnl !== a.last_pnl) return b.last_pnl - a.last_pnl;
    if (b.total_trades !== a.total_trades) return b.total_trades - a.total_trades;
    return a.name.localeCompare(b.name);
  });
}

export function StrategiesCenter() {
  const [stats, setStats] = useState<ScalersStatsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshedAt, setRefreshedAt] = useState<string | null>(null);

  const loadStats = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/scalers/stats", { cache: "no-store" });
      const data = (await res.json()) as ScalersStatsResponse;
      if (!res.ok || data.ok === false) {
        throw new Error(data.error ?? "Unable to load scalper engine stats");
      }
      setStats(data);
      setRefreshedAt(new Date().toLocaleString());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load scalper engine stats");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadStats();
    const interval = setInterval(loadStats, 30_000);
    return () => clearInterval(interval);
  }, [loadStats]);

  const strategies = useMemo(() => sortScalerStrategies(stats?.strategies ?? []), [stats]);

  const totalTrades = strategies.reduce((sum, s) => sum + s.total_trades, 0);
  const totalWins = strategies.reduce((sum, s) => sum + s.wins, 0);
  const overallWinRate = totalTrades > 0 ? totalWins / totalTrades : 0;
  const totalPnl = strategies.reduce((sum, s) => sum + s.last_pnl, 0);
  const activeStrategies = strategies.filter((s) => s.active).length;
  const winningStrategies = strategies.filter((s) => s.last_pnl > 0).length;

  const strategyColumns: DataTableColumn<ScalerStrategyStat>[] = [
    {
      id: "name",
      header: "Strategy",
      sortable: true,
      sortValue: (s) => s.name,
      cell: (s) => (
        <div>
          <div className="text-[13px] font-bold tracking-tight text-[var(--text-primary)]">{s.name.replace(/_/g, " ")}</div>
          <div className="text-[11px] text-[var(--text-muted)]">{s.total_trades > 0 ? "Trading history" : "Probationary (no trades yet)"}</div>
        </div>
      ),
    },
    {
      id: "status",
      header: "Status",
      align: "right",
      cell: (s) => <Badge variant={s.active ? "profit" : "neutral"} size="sm">{s.active ? "Active" : "Demoted"}</Badge>,
    },
    { id: "trades", header: "Trades", align: "right", sortable: true, sortValue: (s) => s.total_trades, cell: (s) => int(s.total_trades) },
    { id: "wins", header: "Wins", align: "right", sortable: true, sortValue: (s) => s.wins, cell: (s) => int(s.wins) },
    {
      id: "winRate",
      header: "Win Rate",
      align: "right",
      sortable: true,
      sortValue: (s) => s.win_rate,
      cell: (s) => <span className={s.total_trades > 0 && s.win_rate >= 0.5 ? "font-semibold text-emerald-600" : ""}>{s.total_trades > 0 ? rate(s.win_rate) : "-"}</span>,
    },
    {
      id: "lastPnl",
      header: "Last PnL",
      align: "right",
      sortable: true,
      sortValue: (s) => s.last_pnl,
      cell: (s) => <span className={`font-semibold ${pnlClass(s.last_pnl)}`}>{usd(s.last_pnl, { signed: true, compact: true })}</span>,
    },
  ];

  const signalColumns: DataTableColumn<ScalerSignalSnapshot>[] = [
    { id: "strategy", header: "Strategy", sortable: true, sortValue: (s) => s.strategy, cell: (s) => <span className="font-semibold text-[var(--text-primary)]">{s.strategy.replace(/_/g, " ")}</span> },
    { id: "side", header: "Side", cell: (s) => <Badge variant={s.side === "LONG" ? "profit" : "loss"} size="sm">{s.side}</Badge> },
    { id: "confidence", header: "Confidence", align: "right", sortable: true, sortValue: (s) => s.confidence, cell: (s) => rate(s.confidence) },
    { id: "price", header: "Price", align: "right", sortable: true, sortValue: (s) => s.price, cell: (s) => usd(s.price, { compact: true }) },
    { id: "time", header: "Time", align: "right", cell: (s) => relativeTime(s.timestamp) },
  ];

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Strategies"
        breadcrumb="BTC Scalper Engine"
        subtitle="Live roster from the curated BTC scalper registry (S1-S29 + SMC expansion) — regime, evaluation cycle telemetry, per-strategy win rate, and last realized PnL, sourced directly from the Go trading engine."
        actions={
          <div className="flex flex-col items-end gap-1">
            <div className="flex items-center gap-2">
              <Link
                href="/terminal/strategies/shadow-performance"
                className="rounded border border-violet-800 bg-violet-950/30 px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-violet-300 transition-colors hover:border-violet-600"
              >
                Shadow Leaderboard
              </Link>
              <button
                type="button"
                onClick={loadStats}
                disabled={loading}
                className="rounded border border-[var(--card-border)] bg-[var(--card-bg)] px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)] transition-colors hover:border-emerald-600 hover:text-emerald-600 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {loading ? "Refreshing..." : "Refresh"}
              </button>
            </div>
            {refreshedAt ? <span className="text-[9px] text-[var(--text-muted)]">Updated {refreshedAt}</span> : null}
          </div>
        }
      />

      {error ? <ErrorBanner message={`Scalper engine stats unavailable: ${error}`} onRetry={loadStats} /> : null}

      <div className="m3-kpi-strip">
        <Metric label="Strategies" value={int(strategies.length)} tone={strategies.length > 0 ? "positive" : "neutral"} />
        <Metric label="Active" value={int(activeStrategies)} tone={activeStrategies > 0 ? "positive" : "neutral"} />
        <Metric label="Regime" value={stats?.regime ?? "-"} />
        <Metric label="Eval Count" value={stats ? int(stats.eval_count) : "-"} />
        <Metric label="Last Eval" value={relativeTime(stats?.last_eval_at)} />
        <Metric label="Win Rate" value={totalTrades > 0 ? rate(overallWinRate) : "-"} tone={totalTrades > 0 && overallWinRate >= 0.5 ? "positive" : "warning"} />
        <Metric label="Total PnL" value={usd(totalPnl, { signed: true, compact: true })} tone={pnlClass(totalPnl) === "text-emerald-400" ? "positive" : pnlClass(totalPnl) === "text-rose-400" ? "negative" : "neutral"} />
        <Metric label="Winning Strategies" value={int(winningStrategies)} tone={winningStrategies > 0 ? "positive" : "neutral"} />
      </div>

      <div className="m3-kpi-strip">
        <Metric label="Raw Signals (last cycle)" value={stats ? int(stats.raw_signals_last_cycle) : "-"} />
        <Metric label="Approved (last cycle)" value={stats ? int(stats.approved_signals_last_cycle) : "-"} tone="positive" />
        <Metric label="Rejected (last cycle)" value={stats ? int(stats.rejected_signals_last_cycle) : "-"} tone="warning" />
        <Metric label="CVD" value={stats ? stats.cvd.toLocaleString("en-US") : "-"} />
      </div>

      <TerminalCard
        title="Strategy Roll-up"
        subtitle="Active strategies first, ranked by last realized PnL then trade count"
        actions={
          <div className="flex items-center gap-2">
            <span className="text-[9px] uppercase tracking-wider text-[var(--text-muted)]">{activeStrategies} active</span>
            {loading ? <span className="text-[9px] uppercase tracking-wider text-amber-500">refreshing</span> : null}
          </div>
        }
      >
        <DataTable
          columns={strategyColumns}
          rows={strategies}
          getRowKey={(s) => s.name}
          searchable
          searchPlaceholder="Search strategy"
          searchFilter={(s, q) => s.name.toLowerCase().includes(q)}
          emptyTitle="No scalper strategies reported by engine"
          density="compact"
        />
      </TerminalCard>

      {stats?.recent_signals && stats.recent_signals.length > 0 ? (
        <TerminalCard title="Recent Signals" subtitle="Most recent signals evaluated by the scalper engine">
          <DataTable
            columns={signalColumns}
            rows={stats.recent_signals}
            getRowKey={(s) => s.id}
            density="compact"
          />
        </TerminalCard>
      ) : null}
    </div>
  );
}
