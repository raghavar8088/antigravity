"use client";

import { useEffect, useState } from "react";
import type { StrategyCorrelationPair, StrategyCorrelationSummary } from "@/lib/strategyAuthority/portfolioTypes";

function corrColor(v: number): string {
  const abs = Math.abs(v);
  if (abs >= 0.80) return v > 0 ? "bg-rose-700 text-white" : "bg-blue-700 text-white";
  if (abs >= 0.60) return v > 0 ? "bg-rose-500/60 text-rose-100" : "bg-blue-500/60 text-blue-100";
  if (abs >= 0.40) return v > 0 ? "bg-orange-600/40 text-orange-200" : "bg-sky-600/40 text-sky-200";
  if (abs >= 0.20) return "bg-zinc-700/60 text-zinc-300";
  return "bg-zinc-900 text-zinc-500";
}

function fmt2(n: number) { return n.toFixed(2); }

interface MatrixData {
  pairs: StrategyCorrelationPair[];
  strategies: string[];
}

export function CorrelationMatrix() {
  const [matrix, setMatrix] = useState<MatrixData | null>(null);
  const [summaries, setSummaries] = useState<StrategyCorrelationSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [view, setView] = useState<"matrix" | "list">("list");

  useEffect(() => {
    Promise.all([
      fetch("/api/strategy-authority/correlation?limit=30").then((r) => r.json()),
      fetch("/api/strategy-authority/diversification?limit=100").then((r) => r.json()),
    ]).then(([mData, dData]) => {
      if (mData.ok) setMatrix({ pairs: mData.matrix, strategies: mData.strategies });
      if (dData.ok) setSummaries(dData.summaries);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  // Build correlation lookup for matrix view
  const corrLookup = new Map<string, number>();
  if (matrix) {
    for (const p of matrix.pairs) {
      corrLookup.set(`${p.strategy_name_a}|||${p.strategy_name_b}`, p.pearson_correlation);
      corrLookup.set(`${p.strategy_name_b}|||${p.strategy_name_a}`, p.pearson_correlation);
    }
  }

  if (loading) {
    return <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading correlation data…</div>;
  }

  if (!matrix || matrix.strategies.length === 0) {
    return (
      <div className="py-8 text-center text-xs text-zinc-600">
        No correlation data computed yet.
        <br />
        <span className="text-[10px] text-zinc-700">Run Portfolio Intelligence Compute to generate correlation matrix.</span>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* View toggle */}
      <div className="flex items-center gap-2">
        {(["matrix", "list"] as const).map((v) => (
          <button
            key={v}
            onClick={() => setView(v)}
            className={`px-3 py-0.5 rounded text-[9px] font-bold uppercase border transition-colors ${
              view === v ? "border-sky-700 bg-sky-900/40 text-sky-400" : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
            }`}
          >
            {v === "matrix" ? "Heatmap" : "List"}
          </button>
        ))}
        <span className="ml-auto text-[9px] text-zinc-600">{matrix.pairs.length} pairs · {matrix.strategies.length} strategies</span>
      </div>

      {view === "matrix" && (
        <div className="overflow-x-auto">
          <table className="text-[8px] border-collapse">
            <thead>
              <tr>
                <th className="w-4" />
                {matrix.strategies.slice(0, 20).map((s) => (
                  <th key={s} className="px-0.5 py-1 text-zinc-600 font-normal max-w-12 truncate" title={s}>
                    <div className="writing-mode-vertical h-16 flex items-end justify-center overflow-hidden">
                      <span className="truncate text-[7px]">{s.slice(0, 12)}</span>
                    </div>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {matrix.strategies.slice(0, 20).map((rowS) => (
                <tr key={rowS}>
                  <td className="pr-1 text-[7px] text-zinc-500 whitespace-nowrap max-w-20 truncate text-right" title={rowS}>
                    {rowS.slice(0, 14)}
                  </td>
                  {matrix.strategies.slice(0, 20).map((colS) => {
                    const val = rowS === colS ? 1 : (corrLookup.get(`${rowS}|||${colS}`) ?? null);
                    return (
                      <td
                        key={colS}
                        className={`w-6 h-5 text-center font-mono text-[7px] border border-zinc-900/50 ${val !== null ? corrColor(val) : "bg-zinc-950 text-zinc-800"}`}
                        title={val !== null ? `${rowS} ↔ ${colS}: ${fmt2(val)}` : "No overlap data"}
                      >
                        {val !== null ? (rowS === colS ? "1" : fmt2(val)) : "—"}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {view === "list" && (
        <div className="space-y-2">
          {/* Diversification leaderboard */}
          <div className="text-[9px] uppercase text-zinc-600 mb-1">Diversification Scores</div>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-[9px] uppercase text-zinc-500 border-b border-zinc-800">
                  <th className="py-1 px-2">Strategy</th>
                  <th className="py-1 px-2">Family</th>
                  <th className="py-1 px-2 text-right">Div Score</th>
                  <th className="py-1 px-2 text-right">Avg |Corr|</th>
                  <th className="py-1 px-2 text-right">Max Corr</th>
                  <th className="py-1 px-2">Top Peer</th>
                  <th className="py-1 px-2 text-center">Cluster</th>
                </tr>
              </thead>
              <tbody>
                {summaries.slice(0, 50).map((s) => (
                  <tr key={s.strategy_id} className="border-t border-zinc-800/50 hover:bg-zinc-900/30">
                    <td className="py-1 px-2 text-zinc-200 max-w-44 truncate text-xs">{s.strategy_name}</td>
                    <td className="py-1 px-2 text-[10px] text-zinc-500">{s.family}</td>
                    <td className="py-1 px-2 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <div className="w-12 h-1.5 rounded bg-zinc-800">
                          <div
                            className={`h-full rounded ${s.diversification_score >= 70 ? "bg-emerald-600" : s.diversification_score >= 40 ? "bg-amber-600" : "bg-rose-700"}`}
                            style={{ width: `${s.diversification_score}%` }}
                          />
                        </div>
                        <span className={`tabular-nums text-[10px] ${s.diversification_score >= 70 ? "text-emerald-400" : s.diversification_score >= 40 ? "text-amber-400" : "text-rose-400"}`}>
                          {s.diversification_score}
                        </span>
                      </div>
                    </td>
                    <td className="py-1 px-2 text-right tabular-nums text-[10px] text-zinc-400">{s.avg_abs_correlation.toFixed(3)}</td>
                    <td className="py-1 px-2 text-right tabular-nums text-[10px] text-zinc-400">{s.max_corr_value.toFixed(3)}</td>
                    <td className="py-1 px-2 text-[9px] text-zinc-600 max-w-32 truncate">{s.max_corr_peer_name || "—"}</td>
                    <td className="py-1 px-2 text-center text-[9px] text-zinc-600">{s.correlation_cluster}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Top correlated pairs */}
          <div className="mt-2">
            <div className="text-[9px] uppercase text-zinc-600 mb-1">Highest Correlation Pairs (redundancy risk)</div>
            <div className="space-y-0.5">
              {matrix.pairs
                .filter((p) => Math.abs(p.pearson_correlation) >= 0.60)
                .sort((a, b) => Math.abs(b.pearson_correlation) - Math.abs(a.pearson_correlation))
                .slice(0, 10)
                .map((p, i) => (
                  <div key={i} className="flex items-center gap-2 rounded border border-zinc-800/50 px-2 py-1 text-[10px]">
                    <span className={`w-8 font-bold tabular-nums text-right ${Math.abs(p.pearson_correlation) >= 0.80 ? "text-rose-400" : "text-amber-400"}`}>
                      {fmt2(p.pearson_correlation)}
                    </span>
                    <span className="text-zinc-500 truncate max-w-36">{p.strategy_name_a}</span>
                    <span className="text-zinc-700">↔</span>
                    <span className="text-zinc-500 truncate max-w-36">{p.strategy_name_b}</span>
                    <span className="ml-auto text-zinc-700 text-[9px]">{p.overlap_days}d overlap</span>
                  </div>
                ))}
              {matrix.pairs.filter((p) => Math.abs(p.pearson_correlation) >= 0.60).length === 0 && (
                <div className="text-[10px] text-zinc-700 py-2">No high-correlation pairs detected — portfolio is well diversified.</div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
