"use client";

import { useEffect, useState } from "react";
import type { PortfolioConstructionResult } from "@/lib/strategyAuthority/portfolioTypes";

function HHIGauge({ hhi }: { hhi: number }) {
  // HHI: 0 = perfect competition, 10000 = monopoly
  // Good portfolio: < 1000 (moderate diversity)
  const pct = Math.min(100, (hhi / 10000) * 100);
  const color = hhi < 1000 ? "text-emerald-400" : hhi < 2500 ? "text-amber-400" : "text-rose-400";
  const label = hhi < 1000 ? "Well Diversified" : hhi < 2500 ? "Moderate Concentration" : "Highly Concentrated";
  return (
    <div className="text-center">
      <div className={`text-xl font-bold tabular-nums ${color}`}>{Math.round(hhi)}</div>
      <div className="mt-0.5 h-2 rounded-full bg-zinc-800 mx-2">
        <div
          className={`h-full rounded-full ${hhi < 1000 ? "bg-emerald-600" : hhi < 2500 ? "bg-amber-600" : "bg-rose-700"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className="text-[9px] text-zinc-600 mt-0.5">{label}</div>
    </div>
  );
}

export function PortfolioConstruction() {
  const [construction, setConstruction] = useState<PortfolioConstructionResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [sortBy, setSortBy] = useState<"weight" | "family">("weight");

  useEffect(() => {
    fetch("/api/strategy-authority/portfolio-construction")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setConstruction(d.construction);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  if (loading) return <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading portfolio construction…</div>;

  if (!construction || construction.recommended_strategy_ids.length === 0) {
    return (
      <div className="py-8 text-center text-xs text-zinc-600">
        No portfolio construction data. Run Portfolio Intelligence Compute first.
        <br /><span className="text-[10px] text-zinc-700">Requires strategies in Main Engine or passing all 5 candidate gates.</span>
      </div>
    );
  }

  const weightEntries = Object.entries(construction.weights)
    .map(([id, w]) => ({
      id,
      name: construction.recommended_strategy_names[construction.recommended_strategy_ids.indexOf(id)] ?? id,
      weight: w,
    }))
    .sort((a, b) => sortBy === "weight" ? b.weight - a.weight : a.name.localeCompare(b.name));

  const maxWeight = Math.max(...weightEntries.map((e) => e.weight), 0.1);

  return (
    <div className="space-y-3">
      {/* Portfolio KPIs */}
      <div className="grid grid-cols-3 gap-2 md:grid-cols-6">
        {[
          { label: "Strategies", value: String(construction.recommended_strategy_ids.length) },
          { label: "Expected PF", value: construction.expected_portfolio_pf.toFixed(3), color: "text-sky-400" },
          { label: "Expected Sharpe", value: construction.expected_portfolio_sharpe.toFixed(3), color: "text-blue-400" },
          { label: "Expected DD", value: `${construction.expected_max_drawdown.toFixed(1)}%`, color: "text-amber-400" },
          { label: "Max Weight", value: `${construction.max_single_weight.toFixed(1)}%` },
          { label: "Div Ratio", value: construction.diversification_ratio.toFixed(3), color: "text-emerald-400" },
        ].map((k) => (
          <div key={k.label} className="rounded border border-zinc-800 bg-zinc-900/40 px-2 py-1.5 text-center">
            <div className="text-[9px] uppercase text-zinc-600">{k.label}</div>
            <div className={`text-sm font-bold tabular-nums ${k.color ?? "text-zinc-200"}`}>{k.value}</div>
          </div>
        ))}
      </div>

      {/* HHI */}
      <div className="rounded border border-zinc-800 bg-zinc-900/20 p-3">
        <div className="text-[9px] uppercase text-zinc-600 mb-2">Concentration Index (HHI)</div>
        <HHIGauge hhi={construction.hhi} />
        <div className="mt-2 flex gap-3 text-[9px] text-zinc-600 justify-center">
          <span>{"< 1000"} = Diversified</span>
          <span>1000–2500 = Moderate</span>
          <span>{"> 2500"} = Concentrated</span>
        </div>
      </div>

      {/* Family exposure */}
      <div className="rounded border border-zinc-800 bg-zinc-900/20 px-3 py-2">
        <div className="text-[9px] uppercase text-zinc-600 mb-2">Family Exposure</div>
        <div className="space-y-1">
          {Object.entries(construction.family_exposure)
            .sort(([, a], [, b]) => b - a)
            .map(([family, pct]) => (
              <div key={family} className="flex items-center gap-2 text-[9px]">
                <span className="w-32 text-zinc-500 truncate">{family}</span>
                <div className="flex-1 h-1.5 rounded bg-zinc-800">
                  <div
                    className={`h-full rounded ${pct >= 30 ? "bg-rose-700" : pct >= 20 ? "bg-amber-700" : "bg-sky-700"}`}
                    style={{ width: `${Math.min(100, pct)}%` }}
                  />
                </div>
                <span className="tabular-nums text-zinc-400 w-8 text-right">{pct.toFixed(1)}%</span>
                {pct >= 40 && <span className="text-rose-500">⚠ OVER</span>}
              </div>
            ))}
        </div>
      </div>

      {/* Weight table */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <div className="text-[9px] uppercase text-zinc-600">Recommended Allocation ({construction.recommended_strategy_ids.length} strategies)</div>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as "weight" | "family")}
            className="ml-auto rounded border border-zinc-700 bg-zinc-900 px-2 py-0.5 text-[10px] text-zinc-300 focus:outline-none"
          >
            <option value="weight">Sort: Weight</option>
            <option value="family">Sort: Name</option>
          </select>
        </div>
        <div className="space-y-0.5">
          {weightEntries.map((entry) => (
            <div key={entry.id} className="flex items-center gap-2 text-[10px] hover:bg-zinc-900/30 rounded px-1 py-0.5">
              <span className="w-44 text-zinc-300 truncate">{entry.name}</span>
              <div className="flex-1 h-2 rounded bg-zinc-800">
                <div
                  className={`h-full rounded ${entry.weight >= 15 ? "bg-emerald-700" : entry.weight >= 8 ? "bg-sky-700" : "bg-zinc-600"}`}
                  style={{ width: `${(entry.weight / maxWeight) * 100}%` }}
                />
              </div>
              <span className="w-10 text-right tabular-nums font-bold text-zinc-300">{entry.weight.toFixed(1)}%</span>
            </div>
          ))}
        </div>
      </div>

      {/* Excluded */}
      {Object.keys(construction.excluded_reasons).length > 0 && (
        <div className="rounded border border-zinc-800 bg-zinc-900/10 px-3 py-2">
          <div className="text-[9px] uppercase text-zinc-600 mb-1">Excluded ({Object.keys(construction.excluded_reasons).length})</div>
          <div className="space-y-0.5">
            {Object.entries(construction.excluded_reasons).slice(0, 5).map(([id, reason]) => (
              <div key={id} className="flex gap-2 text-[9px]">
                <span className="text-zinc-600 truncate max-w-40">{id}</span>
                <span className="text-zinc-700">{reason}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="text-[9px] text-zinc-700">
        Portfolio construction computed: {new Date(construction.computed_at).toLocaleString()}
      </div>
    </div>
  );
}
