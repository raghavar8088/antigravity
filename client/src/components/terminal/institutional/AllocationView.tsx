"use client";

import { useEffect, useState } from "react";
import type { PortfolioAllocationSummary, StrategyAllocationWeight } from "@/lib/strategyAuthority/portfolioTypes";

const TIER_COLOR: Record<string, string> = {
  CORE: "text-emerald-400",
  SATELLITE: "text-sky-400",
  WATCH: "text-amber-400",
  EXCLUDED: "text-zinc-600",
};
const TIER_BG: Record<string, string> = {
  CORE: "bg-emerald-900/30 border-emerald-800/50",
  SATELLITE: "bg-sky-900/20 border-sky-800/40",
  WATCH: "bg-amber-900/20 border-amber-800/40",
  EXCLUDED: "bg-zinc-900/20 border-zinc-800",
};

function WeightBar({ weight, max }: { weight: number; max: number }) {
  const pct = max > 0 ? Math.min(100, (weight / max) * 100) : 0;
  const color = weight >= 10 ? "bg-emerald-600" : weight >= 5 ? "bg-sky-600" : weight >= 1 ? "bg-amber-700" : "bg-zinc-700";
  return (
    <div className="flex items-center gap-1.5">
      <div className="flex-1 h-1.5 rounded bg-zinc-800">
        <div className={`h-full rounded ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-[9px] tabular-nums text-zinc-400 w-8 text-right">{weight.toFixed(1)}%</span>
    </div>
  );
}

export function AllocationView() {
  const [summary, setSummary] = useState<PortfolioAllocationSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<"ALL" | "CORE" | "SATELLITE" | "WATCH" | "EXCLUDED">("ALL");
  const [showExcluded, setShowExcluded] = useState(false);

  useEffect(() => {
    fetch("/api/strategy-authority/allocation")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setSummary(d.allocation);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) return <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading allocation data…</div>;

  if (!summary || summary.strategies.length === 0) {
    return (
      <div className="py-8 text-center text-xs text-zinc-600">
        No allocation data. Run Portfolio Intelligence Compute first.
      </div>
    );
  }

  const included = summary.strategies.filter((s) => s.allocation_weight > 0);
  const maxWeight = Math.max(...included.map((s) => s.allocation_weight), 0.1);

  const displayed = summary.strategies.filter((s) =>
    filter === "ALL" ? (showExcluded || s.allocation_weight > 0) : s.allocation_tier === filter
  );

  return (
    <div className="space-y-3">
      {/* Portfolio KPIs */}
      <div className="grid grid-cols-4 gap-2 md:grid-cols-7">
        {[
          { label: "Allocated", value: String(summary.total_allocated_strategies), color: "text-emerald-400" },
          { label: "Excluded", value: String(summary.excluded_strategies), color: "text-zinc-500" },
          { label: "Exp PF", value: summary.expected_portfolio_pf.toFixed(3), color: "text-sky-400" },
          { label: "Exp Sharpe", value: summary.expected_portfolio_sharpe.toFixed(3), color: "text-blue-400" },
          { label: "Max Weight", value: `${summary.max_single_weight.toFixed(1)}%`, color: "text-amber-400" },
          { label: "HHI", value: String(Math.round(summary.hhi)), color: summary.hhi < 1000 ? "text-emerald-400" : "text-rose-400" },
          { label: "Div Ratio", value: summary.diversification_ratio.toFixed(3), color: "text-emerald-400" },
        ].map((k) => (
          <div key={k.label} className="rounded border border-zinc-800 bg-zinc-900/40 px-2 py-1.5 text-center">
            <div className="text-[9px] uppercase text-zinc-600">{k.label}</div>
            <div className={`text-sm font-bold tabular-nums ${k.color}`}>{k.value}</div>
          </div>
        ))}
      </div>

      {/* Family exposure bar chart */}
      {Object.keys(summary.family_exposure).length > 0 && (
        <div className="rounded border border-zinc-800 bg-zinc-900/20 px-3 py-2">
          <div className="text-[9px] uppercase text-zinc-600 mb-2">Family Exposure</div>
          <div className="grid grid-cols-2 gap-x-4 gap-y-1 md:grid-cols-3">
            {Object.entries(summary.family_exposure)
              .sort(([, a], [, b]) => b - a)
              .map(([family, pct]) => (
                <div key={family} className="flex items-center gap-1.5 text-[9px]">
                  <span className="w-28 text-zinc-500 truncate">{family}</span>
                  <div className="flex-1 h-1.5 rounded bg-zinc-800">
                    <div
                      className={`h-full rounded ${pct >= 30 ? "bg-rose-700" : pct >= 20 ? "bg-amber-700" : "bg-sky-700"}`}
                      style={{ width: `${Math.min(100, pct)}%` }}
                    />
                  </div>
                  <span className="tabular-nums text-zinc-400 w-8 text-right">{pct.toFixed(1)}%</span>
                </div>
              ))}
          </div>
        </div>
      )}

      {/* Tier filter */}
      <div className="flex items-center gap-1 flex-wrap">
        {(["ALL", "CORE", "SATELLITE", "WATCH"] as const).map((t) => {
          const count = t === "ALL"
            ? included.length
            : summary.strategies.filter((s) => s.allocation_tier === t).length;
          return (
            <button
              key={t}
              onClick={() => setFilter(t)}
              className={`px-2 py-0.5 rounded text-[9px] font-bold uppercase border transition-colors ${
                filter === t
                  ? `${TIER_BG[t] ?? "border-zinc-600 bg-zinc-800"} ${TIER_COLOR[t] ?? "text-zinc-300"}`
                  : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
              }`}
            >
              {t} <span className="ml-0.5 opacity-60">{count}</span>
            </button>
          );
        })}
        <label className="ml-auto flex items-center gap-1 text-[9px] text-zinc-600 cursor-pointer">
          <input type="checkbox" checked={showExcluded} onChange={(e) => setShowExcluded(e.target.checked)} className="accent-zinc-500" />
          Show excluded
        </label>
      </div>

      {/* Allocation table */}
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
              <th className="py-2 px-2">Strategy</th>
              <th className="py-2 px-2">Family</th>
              <th className="py-2 px-2 text-center">Tier</th>
              <th className="py-2 px-2">Allocation Weight</th>
              <th className="py-2 px-2 text-right">Kelly</th>
              <th className="py-2 px-2 text-right">Authority</th>
              <th className="py-2 px-2 text-right">Div Score</th>
            </tr>
          </thead>
          <tbody>
            {displayed.map((s: StrategyAllocationWeight) => (
              <tr key={s.strategy_id} className="border-t border-zinc-800/50 hover:bg-zinc-900/30">
                <td className="py-1.5 px-2 text-zinc-200 max-w-44 truncate font-medium">{s.strategy_name}</td>
                <td className="py-1.5 px-2 text-[10px] text-zinc-500">{s.family}</td>
                <td className="py-1.5 px-2 text-center">
                  <span className={`text-[9px] font-bold uppercase ${TIER_COLOR[s.allocation_tier]}`}>
                    {s.allocation_tier}
                  </span>
                </td>
                <td className="py-1.5 px-2 w-36">
                  <WeightBar weight={s.allocation_weight} max={maxWeight} />
                </td>
                <td className="py-1.5 px-2 text-right tabular-nums text-[10px] text-zinc-400">
                  {(s.kelly_fraction * 100).toFixed(1)}%
                </td>
                <td className="py-1.5 px-2 text-right tabular-nums text-[10px] text-zinc-400">{s.authority_score}</td>
                <td className="py-1.5 px-2 text-right tabular-nums text-[10px] text-zinc-400">{s.diversification_score}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
