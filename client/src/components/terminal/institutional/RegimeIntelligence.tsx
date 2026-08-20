"use client";

import { useEffect, useState } from "react";
import type { MarketRegime, StrategyRegimeMetrics } from "@/lib/strategyAuthority/portfolioTypes";
import { AutoSortTable } from "@/components/desk/ui";

const REGIME_CONFIG: Record<MarketRegime, { label: string; color: string; bg: string }> = {
  TRENDING: { label: "Trending", color: "text-emerald-400", bg: "bg-emerald-900/30 border-emerald-800/40" },
  RANGING: { label: "Ranging", color: "text-sky-400", bg: "bg-sky-900/20 border-sky-800/40" },
  HIGH_VOLATILITY_BREAKOUT: { label: "High Vol", color: "text-amber-400", bg: "bg-amber-900/20 border-amber-800/40" },
  LOW_VOLATILITY_CHOP: { label: "Low Vol", color: "text-zinc-400", bg: "bg-zinc-900/20 border-zinc-800" },
};

const ALL_REGIMES: MarketRegime[] = [
  "TRENDING", "RANGING", "HIGH_VOLATILITY_BREAKOUT", "LOW_VOLATILITY_CHOP",
];

function fmt(n: number, d = 2) { return isNaN(n) ? "—" : n.toFixed(d); }

function RegimePill({ regime }: { regime: MarketRegime }) {
  const cfg = REGIME_CONFIG[regime];
  return (
    <span className={`rounded border px-1.5 py-0.5 text-[8px] font-bold uppercase ${cfg.bg} ${cfg.color}`}>
      {cfg.label}
    </span>
  );
}

function StrengthBar({ score }: { score: number }) {
  const color = score >= 70 ? "bg-emerald-600" : score >= 50 ? "bg-sky-600" : score >= 30 ? "bg-amber-700" : "bg-rose-800";
  return (
    <div className="flex items-center gap-1.5">
      <div className="flex-1 h-1.5 rounded bg-zinc-800">
        <div className={`h-full rounded ${color}`} style={{ width: `${score}%` }} />
      </div>
      <span className={`text-[9px] tabular-nums w-5 ${score >= 70 ? "text-emerald-400" : score >= 50 ? "text-sky-400" : "text-zinc-500"}`}>{score}</span>
    </div>
  );
}

export function RegimeIntelligence() {
  const [metrics, setMetrics] = useState<StrategyRegimeMetrics[]>([]);
  const [currentRegime, setCurrentRegime] = useState<MarketRegime | null>(null);
  const [loading, setLoading] = useState(true);
  const [sortBy, setSortBy] = useState<"strength" | "current">("strength");
  const [filterRegime, setFilterRegime] = useState<MarketRegime | "ALL">("ALL");

  useEffect(() => {
    fetch("/api/strategy-authority/regimes?limit=200")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) {
          setMetrics(d.metrics);
          setCurrentRegime(d.currentRegime ?? null);
        }
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) return <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading regime data…</div>;

  if (metrics.length === 0) {
    return (
      <div className="py-8 text-center text-xs text-zinc-600">
        No regime data computed. Run Portfolio Intelligence Compute first.
        <br /><span className="text-[10px] text-zinc-700">Requires regime_snapshots data in MongoDB.</span>
      </div>
    );
  }

  // Summary stats per regime
  const regimeSummary = ALL_REGIMES.map((regime) => {
    const withData = metrics.filter((m) => m.regimes[regime]?.tradeCount ?? 0 > 0);
    const avgPf = withData.length
      ? withData.reduce((a, m) => a + (m.regimes[regime]?.profitFactor ?? 0), 0) / withData.length
      : 0;
    const profitable = withData.filter((m) => (m.regimes[regime]?.profitFactor ?? 0) > 1).length;
    return { regime, strategies: withData.length, avgPf, profitable };
  });

  const sorted = [...metrics].sort((a, b) => {
    if (sortBy === "strength") return b.regime_strength_score - a.regime_strength_score;
    if (sortBy === "current" && currentRegime) {
      return (b.regimes[currentRegime]?.profitFactor ?? 0) - (a.regimes[currentRegime]?.profitFactor ?? 0);
    }
    return b.regime_strength_score - a.regime_strength_score;
  }).filter((m) => {
    if (filterRegime === "ALL") return true;
    return (m.regimes[filterRegime]?.tradeCount ?? 0) > 0;
  });

  return (
    <div className="space-y-3">
      {/* Current regime + summary */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="rounded border border-zinc-800 bg-zinc-900/40 px-3 py-2">
          <div className="text-[9px] uppercase text-zinc-600">Current Market Regime</div>
          <div className="mt-0.5">
            {currentRegime
              ? <RegimePill regime={currentRegime} />
              : <span className="text-[10px] text-zinc-600">Unknown — no regime snapshot</span>
            }
          </div>
        </div>
        {regimeSummary.map((rs) => (
          <div key={rs.regime} className="rounded border border-zinc-800 bg-zinc-900/30 px-2 py-1.5 text-center min-w-20">
            <RegimePill regime={rs.regime} />
            <div className="mt-1 text-xs font-bold tabular-nums text-zinc-300">{rs.strategies}</div>
            <div className="text-[8px] text-zinc-600">strategies with data</div>
            <div className="text-[9px] tabular-nums text-zinc-400">avg PF {fmt(rs.avgPf)}</div>
          </div>
        ))}
      </div>

      {/* Controls */}
      <div className="flex flex-wrap items-center gap-2">
        <select
          value={sortBy}
          onChange={(e) => setSortBy(e.target.value as "strength" | "current")}
          className="rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-[10px] text-zinc-300 focus:outline-none"
        >
          <option value="strength">Sort: Regime Strength</option>
          <option value="current">Sort: Current Regime PF</option>
        </select>
        <select
          value={filterRegime}
          onChange={(e) => setFilterRegime(e.target.value as MarketRegime | "ALL")}
          className="rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-[10px] text-zinc-300 focus:outline-none"
        >
          <option value="ALL">All Regimes</option>
          {ALL_REGIMES.map((r) => <option key={r} value={r}>{REGIME_CONFIG[r].label}</option>)}
        </select>
        <span className="ml-auto text-[9px] text-zinc-600">{sorted.length} strategies</span>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <AutoSortTable><table className="w-full text-xs">
          <thead>
            <tr className="text-left text-[9px] uppercase text-zinc-500 border-b border-zinc-800">
              <th className="py-2 px-2">Strategy</th>
              <th className="py-2 px-2">Family</th>
              <th className="py-2 px-2">Strength</th>
              {ALL_REGIMES.map((r) => (
                <th key={r} className={`py-2 px-2 text-right ${currentRegime === r ? "text-emerald-500" : ""}`}>
                  {REGIME_CONFIG[r].label}
                </th>
              ))}
              <th className="py-2 px-2">Best</th>
              <th className="py-2 px-2">Worst</th>
            </tr>
          </thead>
          <tbody>
            {sorted.slice(0, 60).map((m) => (
              <tr key={m.strategy_id} className="border-t border-zinc-800/50 hover:bg-zinc-900/30">
                <td className="py-1.5 px-2 text-zinc-200 max-w-44 truncate font-medium">{m.strategy_name}</td>
                <td className="py-1.5 px-2 text-[10px] text-zinc-500">{m.family}</td>
                <td className="py-1.5 px-2 w-24"><StrengthBar score={m.regime_strength_score} /></td>
                {ALL_REGIMES.map((regime) => {
                  const perf = m.regimes[regime];
                  const pf = perf?.profitFactor;
                  const isCurrent = regime === currentRegime;
                  return (
                    <td
                      key={regime}
                      className={`py-1.5 px-2 text-right tabular-nums text-[10px] ${
                        pf === undefined ? "text-zinc-700" :
                        pf >= 1.3 ? "text-emerald-400" :
                        pf >= 1.0 ? "text-zinc-300" : "text-rose-400"
                      } ${isCurrent ? "font-bold" : ""}`}
                      title={perf ? `${perf.tradeCount} trades, WR ${fmt(perf.winRate, 1)}%` : "No data"}
                    >
                      {pf !== undefined ? fmt(pf) : "—"}
                    </td>
                  );
                })}
                <td className="py-1.5 px-2">
                  {m.best_regime ? <RegimePill regime={m.best_regime} /> : <span className="text-zinc-700">—</span>}
                </td>
                <td className="py-1.5 px-2">
                  {m.worst_regime ? <RegimePill regime={m.worst_regime} /> : <span className="text-zinc-700">—</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table></AutoSortTable>
      </div>
    </div>
  );
}
