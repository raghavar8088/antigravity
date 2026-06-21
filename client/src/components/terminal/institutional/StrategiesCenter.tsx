"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { Metric, TerminalCard } from "./TerminalCard";
import { pnlClass, usd } from "./format";

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
  const [query, setQuery] = useState("");

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

  const filteredStrategies = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return strategies;
    return strategies.filter((strategy) => strategy.name.toLowerCase().includes(q));
  }, [query, strategies]);

  const totalTrades = strategies.reduce((sum, s) => sum + s.total_trades, 0);
  const totalWins = strategies.reduce((sum, s) => sum + s.wins, 0);
  const overallWinRate = totalTrades > 0 ? totalWins / totalTrades : 0;
  const totalPnl = strategies.reduce((sum, s) => sum + s.last_pnl, 0);
  const activeStrategies = strategies.filter((s) => s.active).length;
  const winningStrategies = strategies.filter((s) => s.last_pnl > 0).length;

  return (
    <div className="m3-page-stack">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-[10px] uppercase tracking-widest text-emerald-500">BTC Scalper Engine</div>
          <h1 className="text-xl font-bold text-zinc-100">Strategies</h1>
          <p className="mt-1 max-w-2xl text-xs text-zinc-500">
            Live roster from the curated BTC scalper registry (S1-S29 + SMC expansion) — regime, evaluation cycle
            telemetry, per-strategy win rate, and last realized PnL, sourced directly from the Go trading engine.
          </p>
        </div>
        <div className="flex flex-col items-end gap-1">
          <button
            type="button"
            onClick={loadStats}
            disabled={loading}
            className="rounded border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-zinc-300 transition-colors hover:border-emerald-700 hover:text-emerald-300 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loading ? "Refreshing..." : "Refresh"}
          </button>
          {refreshedAt ? <span className="text-[9px] text-zinc-600">Updated {refreshedAt}</span> : null}
        </div>
      </div>

      {error ? (
        <div className="rounded border border-rose-900/60 bg-rose-950/20 px-4 py-3 text-xs text-rose-300" role="alert">
          <span className="font-bold">Scalper engine stats unavailable.</span>{" "}
          <span className="text-rose-200/80">{error}</span>
        </div>
      ) : null}

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
            <span className="text-[9px] uppercase tracking-wider text-zinc-600">{activeStrategies} active</span>
            {loading ? <span className="text-[9px] uppercase tracking-wider text-amber-500">refreshing</span> : null}
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search strategy"
              className="w-44 rounded border border-zinc-800 bg-zinc-950 px-2 py-1 text-[10px] text-zinc-300 outline-none transition-colors placeholder:text-zinc-700 focus:border-emerald-800"
            />
          </div>
        }
      >
        {filteredStrategies.length === 0 ? (
          <TerminalNoData label={query ? "No matching strategies" : "No scalper strategies reported by engine"} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[820px] text-xs">
              <thead>
                <tr className="border-b border-zinc-800 text-left text-[9px] uppercase tracking-wider text-zinc-500">
                  <th className="px-2 py-2">#</th>
                  <th className="px-2 py-2">Strategy</th>
                  <th className="px-2 py-2 text-right">Status</th>
                  <th className="px-2 py-2 text-right">Trades</th>
                  <th className="px-2 py-2 text-right">Wins</th>
                  <th className="px-2 py-2 text-right">Win Rate</th>
                  <th className="px-2 py-2 text-right">Last PnL</th>
                </tr>
              </thead>
              <tbody>
                {filteredStrategies.map((strategy, index) => (
                  <tr key={strategy.name} className="border-t border-zinc-900 transition-colors hover:bg-zinc-900/40">
                    <td className="px-2 py-2 text-[10px] tabular-nums text-zinc-600">{index + 1}</td>
                    <td className="px-2 py-2">
                      <div className="font-semibold text-zinc-200">{strategy.name}</div>
                      <div className="text-[10px] text-zinc-600">
                        {strategy.total_trades > 0 ? "Trading history" : "Probationary (no trades yet)"}
                      </div>
                    </td>
                    <td className="px-2 py-2 text-right">
                      <span
                        className={`rounded px-2 py-0.5 text-[9px] font-bold uppercase tracking-wider ${
                          strategy.active ? "bg-emerald-950/60 text-emerald-400" : "bg-zinc-800 text-zinc-500"
                        }`}
                      >
                        {strategy.active ? "Active" : "Demoted"}
                      </span>
                    </td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-300">{int(strategy.total_trades)}</td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">{int(strategy.wins)}</td>
                    <td className={`px-2 py-2 text-right font-mono tabular-nums ${strategy.total_trades > 0 && strategy.win_rate >= 0.5 ? "text-emerald-400" : "text-zinc-400"}`}>
                      {strategy.total_trades > 0 ? rate(strategy.win_rate) : "-"}
                    </td>
                    <td className={`px-2 py-2 text-right font-mono font-semibold tabular-nums ${pnlClass(strategy.last_pnl)}`}>
                      {usd(strategy.last_pnl, { signed: true, compact: true })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </TerminalCard>

      {stats?.recent_signals && stats.recent_signals.length > 0 ? (
        <TerminalCard title="Recent Signals" subtitle="Most recent signals evaluated by the scalper engine">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[640px] text-xs">
              <thead>
                <tr className="border-b border-zinc-800 text-left text-[9px] uppercase tracking-wider text-zinc-500">
                  <th className="px-2 py-2">Strategy</th>
                  <th className="px-2 py-2">Side</th>
                  <th className="px-2 py-2 text-right">Confidence</th>
                  <th className="px-2 py-2 text-right">Price</th>
                  <th className="px-2 py-2 text-right">Time</th>
                </tr>
              </thead>
              <tbody>
                {stats.recent_signals.map((signal) => (
                  <tr key={signal.id} className="border-t border-zinc-900">
                    <td className="px-2 py-2 text-zinc-300">{signal.strategy}</td>
                    <td className="px-2 py-2">
                      <span className={signal.side === "LONG" ? "text-emerald-400" : "text-rose-400"}>{signal.side}</span>
                    </td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">{rate(signal.confidence)}</td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">{usd(signal.price, { compact: true })}</td>
                    <td className="px-2 py-2 text-right text-[10px] text-zinc-600">{relativeTime(signal.timestamp)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </TerminalCard>
      ) : null}
    </div>
  );
}
