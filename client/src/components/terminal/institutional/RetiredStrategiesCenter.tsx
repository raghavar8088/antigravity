"use client";

import { useEffect, useState } from "react";
import type { StrategyProfileDoc } from "@/lib/strategyAuthority/types";
import { TerminalCard, Metric } from "./TerminalCard";

export function RetiredStrategiesCenter() {
  const [retired, setRetired] = useState<StrategyProfileDoc[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");

  useEffect(() => {
    fetch("/api/strategy-authority/retirements")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setRetired(d.retirements);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  const filtered = retired.filter(
    (s) =>
      !search ||
      s.strategy_name.toLowerCase().includes(search.toLowerCase()) ||
      s.family.toLowerCase().includes(search.toLowerCase())
  );

  const familyCounts = retired.reduce<Record<string, number>>((acc, s) => {
    acc[s.family] = (acc[s.family] ?? 0) + 1;
    return acc;
  }, {});

  const topFamily = Object.entries(familyCounts).sort((a, b) => b[1] - a[1])[0];

  return (
    <div className="m3-page-stack">
      <div className="m3-kpi-strip">
        <Metric label="Total Retired" value={String(retired.length)} tone={retired.length > 0 ? "negative" : "neutral"} />
        <Metric label="Families Affected" value={String(Object.keys(familyCounts).length)} />
        <Metric label="Most Retired Family" value={topFamily ? topFamily[0] : "—"} />
        <Metric label="Capital Protected" value="Active — no live capital" tone="positive" />
      </div>

      <div className="flex items-center gap-3">
        <div className="flex-1">
          <input
            type="text"
            placeholder="Search by name or family…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full rounded border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-600 focus:border-zinc-500 focus:outline-none"
          />
        </div>
        <div className="text-[10px] text-zinc-600">{filtered.length} of {retired.length}</div>
      </div>

      <TerminalCard
        title="Retirement Center"
        subtitle="Permanently retired strategies — no further execution allowed"
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
                  <th className="py-2 px-2">Timeframe</th>
                  <th className="py-2 px-2">Retired</th>
                  <th className="py-2 px-2">Reason</th>
                  <th className="py-2 px-2 text-right">Promotions</th>
                  <th className="py-2 px-2 text-right">Demotions</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((s) => (
                  <tr key={s.strategy_id} className="border-t border-zinc-800/50 hover:bg-zinc-900/30">
                    <td className="py-1.5 px-2 text-zinc-300 max-w-[200px] truncate font-medium">{s.strategy_name}</td>
                    <td className="py-1.5 px-2 text-zinc-500 text-[10px]">{s.family}</td>
                    <td className="py-1.5 px-2 text-zinc-600 text-[10px]">{s.timeframe}</td>
                    <td className="py-1.5 px-2 text-zinc-600 text-[10px]">
                      {s.retired_at ? new Date(s.retired_at).toLocaleDateString() : "—"}
                    </td>
                    <td className="py-1.5 px-2 text-rose-400/70 text-[10px] max-w-[240px] truncate">
                      {s.retirement_reason ?? "—"}
                    </td>
                    <td className="py-1.5 px-2 text-right tabular-nums text-zinc-500 text-[10px]">{s.promotion_count}</td>
                    <td className="py-1.5 px-2 text-right tabular-nums text-zinc-500 text-[10px]">{s.demotion_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </TerminalCard>

      {retired.length > 0 && (
        <TerminalCard title="Retirement by Family" subtitle="Distribution of retired strategies">
          <div className="grid grid-cols-3 gap-1.5">
            {Object.entries(familyCounts)
              .sort((a, b) => b[1] - a[1])
              .slice(0, 9)
              .map(([family, count]) => (
                <div key={family} className="rounded border border-zinc-800 px-2 py-1.5">
                  <div className="text-[10px] font-semibold text-zinc-300 truncate">{family}</div>
                  <div className="text-lg font-bold tabular-nums text-rose-400">{count}</div>
                </div>
              ))}
          </div>
        </TerminalCard>
      )}

      <p className="text-[10px] uppercase tracking-wider text-zinc-600">
        Retirement center — forensic review only — no execution permitted on retired strategies
      </p>
    </div>
  );
}
