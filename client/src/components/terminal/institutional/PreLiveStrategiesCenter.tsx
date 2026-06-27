"use client";

import { useEffect, useRef, useState } from "react";
import { PageHeader } from "@/components/ui/PageHeader";

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

type SortKey = "signalsFired" | "trades" | "winRate" | "pnl" | "wins" | "losses";

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
  const [sort, setSort] = useState<SortKey>("signalsFired");
  const [sortDir, setSortDir] = useState<"desc" | "asc">("desc");
  const [search, setSearch] = useState("");
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

  const toggleSort = (key: SortKey) => {
    if (sort === key) setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    else { setSort(key); setSortDir("desc"); }
  };

  const filtered = rows.filter((r) =>
    !search || r.name.toLowerCase().includes(search.toLowerCase()) || r.category.toLowerCase().includes(search.toLowerCase())
  );

  const sorted = [...filtered].sort((a, b) => {
    const va = a[sort] as number;
    const vb = b[sort] as number;
    return sortDir === "desc" ? vb - va : va - vb;
  });

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

  const SortTh = ({ col, label }: { col: SortKey; label: string }) => (
    <th
      className="cursor-pointer select-none py-2 px-3 text-left text-[10px] uppercase tracking-wider text-zinc-500 hover:text-zinc-300"
      onClick={() => toggleSort(col)}
    >
      {label}
      {sort === col && <span className="ml-1">{sortDir === "desc" ? "↓" : "↑"}</span>}
    </th>
  );

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Pre-Live Strategies"
        subtitle="100 backtested-qualified strategies · signal telemetry · live paper PnL"
        actions={
          <span style={{ background: "rgba(251,191,36,0.12)", border: "1px solid rgba(251,191,36,0.35)", color: "#fbbf24", borderRadius: 6, fontSize: 11, padding: "2px 10px", fontWeight: 700, letterSpacing: "0.06em" }}>
            PAPER TRADING · 100 STRATEGIES
          </span>
        }
      />

      {/* KPI summary */}
      <div className="m3-kpi-strip">
        <div className="m3-kpi-card">
          <div className="m3-kpi-label">Strategies</div>
          <div className="m3-kpi-value text-emerald-400">{rows.length} / 100</div>
        </div>
        <div className="m3-kpi-card">
          <div className="m3-kpi-label">Total Signals</div>
          <div className="m3-kpi-value text-sky-400">{totals.signals.toLocaleString()}</div>
        </div>
        <div className="m3-kpi-card">
          <div className="m3-kpi-label">Total Trades</div>
          <div className="m3-kpi-value">{totals.trades}</div>
        </div>
        <div className="m3-kpi-card">
          <div className="m3-kpi-label">Win Rate</div>
          <div className={`m3-kpi-value ${overallWR >= 50 ? "text-emerald-400" : overallWR > 0 ? "text-amber-400" : "text-zinc-400"}`}>
            {pct(overallWR)}
          </div>
        </div>
        <div className="m3-kpi-card">
          <div className="m3-kpi-label">Wins</div>
          <div className="m3-kpi-value text-emerald-400">{totals.wins}</div>
        </div>
        <div className="m3-kpi-card">
          <div className="m3-kpi-label">Losses</div>
          <div className="m3-kpi-value text-rose-400">{totals.losses}</div>
        </div>
        <div className="m3-kpi-card">
          <div className="m3-kpi-label">Total PnL</div>
          <div className={`m3-kpi-value ${pnlCls(totals.pnl)}`}>{usd(totals.pnl)}</div>
        </div>
      </div>

      {/* Search */}
      <div className="flex items-center gap-3">
        <input
          type="text"
          placeholder="Search strategy or category…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="rounded border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-600 focus:border-zinc-500 focus:outline-none w-64"
        />
        <span className="text-[11px] text-zinc-500">{sorted.length} shown</span>
      </div>

      {/* Table */}
      {loading ? (
        <div className="py-16 text-center text-xs text-zinc-500">Loading strategy stats…</div>
      ) : error ? (
        <div className="rounded border border-rose-800 bg-rose-950/30 p-4 text-xs text-rose-400">
          Pre-Live Engine offline — {error}. Start it with: <code className="ml-1 text-rose-300">cd engine && go run ./cmd/pre_live/main.go</code>
        </div>
      ) : (
        <div className="overflow-x-auto rounded border border-zinc-800">
          <table className="w-full text-xs">
            <thead className="border-b border-zinc-800 bg-zinc-900/60">
              <tr>
                <th className="py-2 px-3 text-left text-[10px] uppercase tracking-wider text-zinc-500">#</th>
                <th className="py-2 px-3 text-left text-[10px] uppercase tracking-wider text-zinc-500">Strategy</th>
                <th className="py-2 px-3 text-left text-[10px] uppercase tracking-wider text-zinc-500">Cat / TF</th>
                <SortTh col="signalsFired" label="Signals" />
                <SortTh col="trades" label="Trades" />
                <SortTh col="wins" label="Wins" />
                <SortTh col="losses" label="Losses" />
                <SortTh col="winRate" label="Win %" />
                <SortTh col="pnl" label="PnL" />
              </tr>
            </thead>
            <tbody>
              {sorted.map((row, i) => (
                <tr key={row.name} className="border-t border-zinc-800/60 hover:bg-zinc-800/30">
                  <td className="py-1.5 px-3 text-zinc-600 font-mono">{i + 1}</td>
                  <td className="py-1.5 px-3 font-mono text-zinc-200 max-w-[200px] truncate" title={row.name}>
                    {row.name}
                  </td>
                  <td className="py-1.5 px-3 text-zinc-500">
                    <span className="text-zinc-400">{row.category}</span>
                    <span className="ml-1 text-zinc-600">· {row.timeframe}</span>
                  </td>
                  <td className="py-1.5 px-3 font-mono text-sky-400">{row.signalsFired}</td>
                  <td className="py-1.5 px-3 font-mono text-zinc-300">{row.trades}</td>
                  <td className="py-1.5 px-3 font-mono text-emerald-400">{row.wins}</td>
                  <td className="py-1.5 px-3 font-mono text-rose-400">{row.losses}</td>
                  <td className="py-1.5 px-3 font-mono">
                    <span className={row.winRate >= 50 ? "text-emerald-400" : row.winRate > 0 ? "text-amber-400" : "text-zinc-500"}>
                      {row.trades > 0 ? pct(row.winRate) : "—"}
                    </span>
                  </td>
                  <td className={`py-1.5 px-3 font-mono ${pnlCls(row.pnl)}`}>
                    {row.trades > 0 ? usd(row.pnl) : "—"}
                  </td>
                </tr>
              ))}
              {sorted.length === 0 && (
                <tr>
                  <td colSpan={9} className="py-8 text-center text-zinc-500">No strategies match your search</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <p className="text-[10px] text-zinc-600">
        Refreshes every 5s · Sorted by {sort} {sortDir} · $100 paper balance · All 100 strategies evaluate every market tick
      </p>
    </div>
  );
}
