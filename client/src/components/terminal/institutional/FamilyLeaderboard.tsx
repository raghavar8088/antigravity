"use client";

import { useEffect, useState } from "react";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";
import type { FamilyIntelligenceRow } from "@/lib/strategyAuthority/strategyAuthorityMongo";

const STATUS_ORDER: StrategyStatus[] = ["TRADE_ENGINE", "MAIN_ENGINE", "GRADE_1", "GRADE_2", "GRADE_3", "GRADE_4", "GRADE_5"];
const STATUS_COLOR: Record<StrategyStatus, string> = {
  TRADE_ENGINE: "bg-emerald-700",
  MAIN_ENGINE: "bg-emerald-700",
  GRADE_1: "bg-amber-600",
  GRADE_2: "bg-violet-700",
  GRADE_3: "bg-blue-700",
  GRADE_4: "bg-sky-700",
  GRADE_5: "bg-zinc-700",
  RETIRED: "bg-rose-900",
};
const STATUS_SHORT: Partial<Record<StrategyStatus, string>> = {
  TRADE_ENGINE: "T", MAIN_ENGINE: "M", GRADE_1: "L1", GRADE_2: "L2",
  GRADE_3: "L3", GRADE_4: "L4", GRADE_5: "L5",
};

function fmt(n: number | undefined, dec = 2) {
  if (n == null || isNaN(n)) return "—";
  return n.toFixed(dec);
}

function Pill({ status, count }: { status: StrategyStatus; count: number }) {
  return (
    <span className={`inline-flex items-center rounded px-1 py-0.5 text-[8px] font-bold text-white ${STATUS_COLOR[status]}`}>
      {STATUS_SHORT[status] ?? status.slice(0, 2)}{count > 1 ? `×${count}` : ""}
    </span>
  );
}

function statusDisplayLabel(status: StrategyStatus): string {
  if (status === "TRADE_ENGINE" || status === "MAIN_ENGINE") return "Trade Engine";
  if (status === "RETIRED") return "Retired";
  return "Legacy Active";
}

type SortKey = "avgProfitFactor" | "avgExpectancy" | "avgWinRate" | "avgDrawdown" | "avgSharpe" | "total" | "mainEngineCount";

function SortHeader({ label, sortKey, current, onSort }: {
  label: string; sortKey: SortKey; current: SortKey; onSort: (k: SortKey) => void;
}) {
  const active = current === sortKey;
  return (
    <th
      className={`py-2 px-2 text-right text-[9px] uppercase tracking-wider cursor-pointer select-none ${active ? "text-sky-400" : "text-zinc-500 hover:text-zinc-400"}`}
      onClick={() => onSort(sortKey)}
    >
      {label}{active ? " ↓" : ""}
    </th>
  );
}

function ScoreMiniBar({ value, max, color }: { value: number; max: number; color: string }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div className="flex items-center gap-1.5">
      <div className="flex-1 h-1.5 rounded bg-zinc-800">
        <div className={`h-full rounded ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

export function FamilyLeaderboard() {
  const [families, setFamilies] = useState<FamilyIntelligenceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [sortKey, setSortKey] = useState<SortKey>("avgProfitFactor");
  const [selected, setSelected] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/strategy-authority/families")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setFamilies(d.families);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  const sorted = [...families].sort((a, b) => {
    if (sortKey === "avgDrawdown") return a[sortKey] - b[sortKey]; // lower is better
    return (b[sortKey] as number) - (a[sortKey] as number);
  });

  const maxPF = Math.max(...families.map((f) => f.avgProfitFactor), 1);
  const maxMain = Math.max(...families.map((f) => f.mainEngineCount), 1);

  return (
    <div className="space-y-3">
      {loading ? (
        <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading family intelligence…</div>
      ) : families.length === 0 ? (
        <div className="py-8 text-center text-xs text-zinc-600">
          No family data yet. Run migration and evaluation first.
        </div>
      ) : (
        <>
          {/* Summary KPIs */}
          <div className="grid grid-cols-4 gap-2">
            {[
              { label: "Families", value: String(families.length) },
              { label: "Total Strategies", value: String(families.reduce((a, f) => a + f.total, 0)) },
              { label: "Main Engine", value: String(families.reduce((a, f) => a + f.mainEngineCount, 0)), color: "text-emerald-400" },
              { label: "Retired", value: String(families.reduce((a, f) => a + f.retiredCount, 0)), color: "text-rose-400" },
            ].map((k) => (
              <div key={k.label} className="rounded border border-zinc-800 bg-zinc-900/40 px-2 py-1.5 text-center">
                <div className="text-[9px] uppercase text-zinc-600">{k.label}</div>
                <div className={`text-lg font-bold tabular-nums ${k.color ?? "text-zinc-200"}`}>{k.value}</div>
              </div>
            ))}
          </div>

          {/* Table */}
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left border-b border-zinc-800">
                  <th className="py-2 px-2 text-[9px] uppercase tracking-wider text-zinc-500">#</th>
                  <th className="py-2 px-2 text-[9px] uppercase tracking-wider text-zinc-500">Family</th>
                  <th className="py-2 px-2 text-right text-[9px] uppercase tracking-wider text-zinc-500">Total</th>
                  <SortHeader label="Avg PF" sortKey="avgProfitFactor" current={sortKey} onSort={setSortKey} />
                  <SortHeader label="Avg WR%" sortKey="avgWinRate" current={sortKey} onSort={setSortKey} />
                  <SortHeader label="Avg E" sortKey="avgExpectancy" current={sortKey} onSort={setSortKey} />
                  <SortHeader label="Avg DD%" sortKey="avgDrawdown" current={sortKey} onSort={setSortKey} />
                  <SortHeader label="Sharpe" sortKey="avgSharpe" current={sortKey} onSort={setSortKey} />
                  <SortHeader label="Engine" sortKey="mainEngineCount" current={sortKey} onSort={setSortKey} />
                  <th className="py-2 px-2 text-[9px] uppercase tracking-wider text-zinc-500">Distribution</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((f, i) => {
                  const isSelected = selected === f.family;
                  return (
                    <>
                      <tr
                        key={f.family}
                        className={`border-t border-zinc-800/50 cursor-pointer ${isSelected ? "bg-zinc-800/40" : "hover:bg-zinc-900/30"}`}
                        onClick={() => setSelected(isSelected ? null : f.family)}
                      >
                        <td className="py-2 px-2 text-[10px] tabular-nums text-zinc-600">{i + 1}</td>
                        <td className="py-2 px-2 font-medium text-zinc-200 max-w-[140px] truncate">
                          {f.family}
                        </td>
                        <td className="py-2 px-2 text-right tabular-nums text-zinc-400">{f.total}</td>
                        <td className="py-2 px-2 text-right tabular-nums">
                          <div className="flex flex-col items-end gap-0.5">
                            <span className={f.avgProfitFactor >= 1.2 ? "text-emerald-400" : f.avgProfitFactor >= 1.0 ? "text-zinc-300" : "text-rose-400"}>
                              {fmt(f.avgProfitFactor)}
                            </span>
                            <ScoreMiniBar value={f.avgProfitFactor} max={maxPF} color="bg-sky-700" />
                          </div>
                        </td>
                        <td className="py-2 px-2 text-right tabular-nums text-zinc-400">{fmt(f.avgWinRate, 1)}%</td>
                        <td className="py-2 px-2 text-right tabular-nums">
                          <span className={f.avgExpectancy > 0 ? "text-emerald-400" : "text-rose-400"}>
                            {fmt(f.avgExpectancy)}
                          </span>
                        </td>
                        <td className="py-2 px-2 text-right tabular-nums">
                          <span className={f.avgDrawdown < 15 ? "text-emerald-400" : f.avgDrawdown < 25 ? "text-amber-400" : "text-rose-400"}>
                            {fmt(f.avgDrawdown, 1)}%
                          </span>
                        </td>
                        <td className="py-2 px-2 text-right tabular-nums text-zinc-400">{fmt(f.avgSharpe)}</td>
                        <td className="py-2 px-2 text-right">
                          <div className="flex flex-col items-end gap-0.5">
                            <span className={f.mainEngineCount > 0 ? "text-emerald-400 font-bold" : "text-zinc-600"}>
                              {f.mainEngineCount}
                            </span>
                            <ScoreMiniBar value={f.mainEngineCount} max={maxMain} color="bg-emerald-700" />
                          </div>
                        </td>
                        <td className="py-2 px-2">
                          <div className="flex flex-wrap gap-0.5 justify-end">
                            {STATUS_ORDER.filter((s) => (f.byStatus[s] ?? 0) > 0).map((s) => (
                              <Pill key={s} status={s} count={f.byStatus[s]!} />
                            ))}
                            {(f.byStatus.RETIRED ?? 0) > 0 && (
                              <Pill status="RETIRED" count={f.byStatus.RETIRED!} />
                            )}
                          </div>
                        </td>
                      </tr>
                      {isSelected && (
                        <tr key={`${f.family}-detail`} className="border-t border-zinc-800/50 bg-zinc-900/50">
                          <td colSpan={10} className="px-4 py-3">
                            <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
                              <div>
                                <div className="text-[9px] text-zinc-600 uppercase mb-1">Distribution</div>
                                <div className="space-y-0.5">
                                  {(Object.entries(f.byStatus) as [StrategyStatus, number][])
                                    .sort((a, b) => b[1] - a[1])
                                    .map(([s, n]) => (
                                      <div key={s} className="flex items-center gap-1 text-[9px]">
                                        <div className={`w-1.5 h-1.5 rounded-full ${STATUS_COLOR[s]}`} />
                                        <span className="text-zinc-500">{statusDisplayLabel(s)}</span>
                                        <span className="ml-auto tabular-nums text-zinc-300">{n}</span>
                                      </div>
                                    ))}
                                </div>
                              </div>
                              <div>
                                <div className="text-[9px] text-zinc-600 uppercase mb-1">Performance</div>
                                <div className="space-y-0.5">
                                  {[
                                    { label: "Profit Factor", value: fmt(f.avgProfitFactor) },
                                    { label: "Win Rate", value: `${fmt(f.avgWinRate, 1)}%` },
                                    { label: "Expectancy", value: fmt(f.avgExpectancy) },
                                    { label: "Max Drawdown", value: `${fmt(f.avgDrawdown, 1)}%` },
                                    { label: "Sharpe", value: fmt(f.avgSharpe) },
                                  ].map((r) => (
                                    <div key={r.label} className="flex justify-between text-[9px]">
                                      <span className="text-zinc-600">{r.label}</span>
                                      <span className="text-zinc-300 tabular-nums">{r.value}</span>
                                    </div>
                                  ))}
                                </div>
                              </div>
                              <div>
                                <div className="text-[9px] text-zinc-600 uppercase mb-1">Alerts</div>
                                <div className="space-y-0.5">
                                  <div className="flex justify-between text-[9px]">
                                    <span className="text-emerald-600">Review Candidates</span>
                                    <span className="text-emerald-400 tabular-nums">{f.promotionCandidates}</span>
                                  </div>
                                  <div className="flex justify-between text-[9px]">
                                    <span className="text-rose-600">Risk Review</span>
                                    <span className="text-rose-400 tabular-nums">{f.demotionRisk}</span>
                                  </div>
                                  <div className="flex justify-between text-[9px]">
                                    <span className="text-rose-700">Retired</span>
                                    <span className="text-rose-500 tabular-nums">{f.retiredCount}</span>
                                  </div>
                                </div>
                              </div>
                              <div>
                                <div className="text-[9px] text-zinc-600 uppercase mb-1">Trade Engine</div>
                                <div className="text-2xl font-bold tabular-nums text-emerald-400">{f.mainEngineCount}</div>
                                <div className="text-[9px] text-zinc-600">
                                  {f.total > 0 ? `${((f.mainEngineCount / f.total) * 100).toFixed(0)}% of family` : ""}
                                </div>
                              </div>
                            </div>
                          </td>
                        </tr>
                      )}
                    </>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
