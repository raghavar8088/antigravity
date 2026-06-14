"use client";

import { useEffect, useState } from "react";
import type { StrategyProfileDoc } from "@/lib/strategyAuthority/types";
import { TerminalCard, Metric } from "./TerminalCard";

type SortKey = "retired_at" | "family" | "demotion_count" | "promotion_count";

function parseReason(reason: string | null | undefined): { primary: string; detail: string } {
  if (!reason) return { primary: "Unknown", detail: "" };
  if (reason.includes("PF<0.90")) return { primary: "Low Profit Factor", detail: reason };
  if (reason.includes("Expectancy")) return { primary: "Negative Expectancy", detail: reason };
  if (reason.includes("drawdown")) return { primary: "Excess Drawdown", detail: reason };
  return { primary: reason.slice(0, 40), detail: reason };
}

const REASON_COLORS: Record<string, string> = {
  "Low Profit Factor": "text-rose-400",
  "Negative Expectancy": "text-orange-400",
  "Excess Drawdown": "text-amber-400",
  "Unknown": "text-zinc-500",
};

function BarChart({ data }: { data: { label: string; value: number; color: string }[] }) {
  const max = Math.max(...data.map((d) => d.value), 1);
  return (
    <div className="space-y-1">
      {data.map((d) => (
        <div key={d.label} className="flex items-center gap-2">
          <span className="w-32 text-[9px] text-zinc-500 truncate flex-shrink-0">{d.label}</span>
          <div className="flex-1 h-3 rounded bg-zinc-800 relative overflow-hidden">
            <div
              className={`h-full rounded transition-all duration-500 ${d.color}`}
              style={{ width: `${(d.value / max) * 100}%` }}
            />
          </div>
          <span className="w-4 text-[9px] tabular-nums text-zinc-400 text-right">{d.value}</span>
        </div>
      ))}
    </div>
  );
}

export function RetirementCenter() {
  const [retired, setRetired] = useState<StrategyProfileDoc[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("retired_at");
  const [familyFilter, setFamilyFilter] = useState("");

  useEffect(() => {
    fetch("/api/strategy-authority/retirements")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setRetired(d.retirements);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  // ── Derived analytics ────────────────────────────────────────────────────────

  const familyCounts = retired.reduce<Record<string, number>>((acc, s) => {
    acc[s.family] = (acc[s.family] ?? 0) + 1;
    return acc;
  }, {});

  const reasonCounts = retired.reduce<Record<string, number>>((acc, s) => {
    const { primary } = parseReason(s.retirement_reason);
    acc[primary] = (acc[primary] ?? 0) + 1;
    return acc;
  }, {});

  const timelineByMonth = retired.reduce<Record<string, number>>((acc, s) => {
    if (!s.retired_at) return acc;
    const key = new Date(s.retired_at).toLocaleDateString("en-US", { month: "short", year: "2-digit" });
    acc[key] = (acc[key] ?? 0) + 1;
    return acc;
  }, {});

  const avgDemotions = retired.length
    ? (retired.reduce((a, s) => a + (s.demotion_count ?? 0), 0) / retired.length).toFixed(1)
    : "0";
  const avgPromotions = retired.length
    ? (retired.reduce((a, s) => a + (s.promotion_count ?? 0), 0) / retired.length).toFixed(1)
    : "0";

  const families = Object.keys(familyCounts).sort((a, b) => familyCounts[b] - familyCounts[a]);

  // ── Filter + sort ────────────────────────────────────────────────────────────

  const filtered = retired
    .filter((s) => {
      const q = search.toLowerCase();
      const matchSearch = !q || s.strategy_name.toLowerCase().includes(q) || s.family.toLowerCase().includes(q) || (s.retirement_reason ?? "").toLowerCase().includes(q);
      const matchFamily = !familyFilter || s.family === familyFilter;
      return matchSearch && matchFamily;
    })
    .sort((a, b) => {
      if (sortKey === "retired_at") {
        return new Date(b.retired_at ?? 0).getTime() - new Date(a.retired_at ?? 0).getTime();
      }
      if (sortKey === "family") return a.family.localeCompare(b.family);
      if (sortKey === "demotion_count") return (b.demotion_count ?? 0) - (a.demotion_count ?? 0);
      if (sortKey === "promotion_count") return (b.promotion_count ?? 0) - (a.promotion_count ?? 0);
      return 0;
    });

  return (
    <div className="m3-page-stack">
      {/* KPI strip */}
      <div className="m3-kpi-strip">
        <Metric label="Total Retired" value={String(retired.length)} tone={retired.length > 0 ? "negative" : "neutral"} />
        <Metric label="Families Affected" value={String(Object.keys(familyCounts).length)} />
        <Metric label="Avg Risk Events" value={avgDemotions} />
        <Metric label="Avg Review Events" value={avgPromotions} />
      </div>

      {/* Search + filter bar */}
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="text"
          placeholder="Search name, family, reason…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="flex-1 min-w-40 rounded border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-600 focus:border-zinc-500 focus:outline-none"
        />
        <select
          value={familyFilter}
          onChange={(e) => setFamilyFilter(e.target.value)}
          className="rounded border border-zinc-700 bg-zinc-900 px-2 py-1.5 text-xs text-zinc-300 focus:outline-none"
        >
          <option value="">All Families</option>
          {families.map((f) => <option key={f} value={f}>{f} ({familyCounts[f]})</option>)}
        </select>
        <select
          value={sortKey}
          onChange={(e) => setSortKey(e.target.value as SortKey)}
          className="rounded border border-zinc-700 bg-zinc-900 px-2 py-1.5 text-xs text-zinc-300 focus:outline-none"
        >
          <option value="retired_at">Sort: Date Retired</option>
          <option value="family">Sort: Family</option>
          <option value="demotion_count">Sort: Risk Events</option>
          <option value="promotion_count">Sort: Review Events</option>
        </select>
        <span className="text-[10px] text-zinc-600">{filtered.length} of {retired.length}</span>
      </div>

      {/* Analytics row */}
      {retired.length > 0 && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <TerminalCard title="Failure Reasons" subtitle="Root cause distribution">
            <BarChart data={Object.entries(reasonCounts).sort((a, b) => b[1] - a[1]).map(([label, value]) => ({
              label,
              value,
              color: label === "Low Profit Factor" ? "bg-rose-700"
                : label === "Negative Expectancy" ? "bg-orange-700"
                : label === "Excess Drawdown" ? "bg-amber-700"
                : "bg-zinc-700",
            }))} />
          </TerminalCard>

          <TerminalCard title="By Family" subtitle="Retirement concentration">
            <BarChart data={Object.entries(familyCounts).sort((a, b) => b[1] - a[1]).slice(0, 8).map(([label, value]) => ({
              label, value, color: "bg-rose-800",
            }))} />
          </TerminalCard>

          <TerminalCard title="Retirement Timeline" subtitle="Monthly retirement count">
            {Object.keys(timelineByMonth).length === 0 ? (
              <div className="py-4 text-center text-xs text-zinc-600">No timeline data</div>
            ) : (
              <BarChart data={Object.entries(timelineByMonth).sort(([a], [b]) => a.localeCompare(b)).map(([label, value]) => ({
                label, value, color: "bg-zinc-600",
              }))} />
            )}
          </TerminalCard>
        </div>
      )}

      {/* Main table */}
      <TerminalCard
        title="Retirement Registry"
        subtitle="Permanently retired — no further execution"
      >
        {loading ? (
          <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading retired strategies…</div>
        ) : filtered.length === 0 ? (
          <div className="py-8 text-center text-xs text-zinc-600">
            {retired.length === 0 ? "No strategies have been retired yet." : "No strategies match your search."}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
                  <th className="py-2 px-2">Strategy</th>
                  <th className="py-2 px-2">Family</th>
                  <th className="py-2 px-2">TF</th>
                  <th className="py-2 px-2">Retired</th>
                  <th className="py-2 px-2">Root Cause</th>
                  <th className="py-2 px-2 text-right">Review</th>
                  <th className="py-2 px-2 text-right">Risk</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((s) => {
                  const { primary, detail } = parseReason(s.retirement_reason);
                  const reasonColor = REASON_COLORS[primary] ?? "text-zinc-500";
                  return (
                    <tr key={s.strategy_id} className="border-t border-zinc-800/50 hover:bg-zinc-900/30" title={detail}>
                      <td className="py-1.5 px-2 text-zinc-300 max-w-[200px] truncate font-medium">{s.strategy_name}</td>
                      <td className="py-1.5 px-2 text-zinc-500 text-[10px]">{s.family}</td>
                      <td className="py-1.5 px-2 text-zinc-600 text-[10px]">{s.timeframe}</td>
                      <td className="py-1.5 px-2 text-zinc-600 text-[10px] whitespace-nowrap">
                        {s.retired_at ? new Date(s.retired_at).toLocaleDateString() : "—"}
                      </td>
                      <td className={`py-1.5 px-2 text-[10px] max-w-[180px] truncate ${reasonColor}`}>
                        {primary}
                      </td>
                      <td className="py-1.5 px-2 text-right tabular-nums text-emerald-600 text-[10px]">{s.promotion_count}</td>
                      <td className="py-1.5 px-2 text-right tabular-nums text-rose-500 text-[10px]">{s.demotion_count}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </TerminalCard>

      <p className="text-[10px] uppercase tracking-wider text-zinc-600">
        Retirement center — forensic review only — no execution permitted on retired strategies
      </p>
    </div>
  );
}
