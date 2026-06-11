"use client";

import { useEffect, useState } from "react";
import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";

type IntelSummary = {
  healthy: number;
  warning: number;
  critical: number;
  insufficient_data: number;
  total: number;
};

export function ResearchCenter({ snapshot }: { snapshot: TerminalSnapshot }) {
  const sorted = [...snapshot.strategies].sort((a, b) => b.promotionScore - a.promotionScore);
  const [summary, setSummary] = useState<IntelSummary | null>(null);

  useEffect(() => {
    let active = true;
    fetch("/api/strategy-intelligence?view=all&limit=600")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (active && data?.ok && data.summary) setSummary(data.summary);
      })
      .catch(() => {});
    return () => { active = false; };
  }, []);

  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
      <TerminalCard title="Strategy Leaderboard" subtitle="Backend authority · strategy_scores + strategy_health">
        {sorted.length === 0 ? (
          <TerminalNoData label="NO STRATEGY DATA — backend authority empty" />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[860px] text-xs">
              <thead className="text-left text-[10px] uppercase tracking-[0.12em] text-zinc-500">
                <tr>
                  <th className="py-2">Rank</th>
                  <th>Strategy</th>
                  <th>Health</th>
                  <th>Family</th>
                  <th className="text-right">Sharpe (N/A)</th>
                  <th className="text-right">Expectancy</th>
                  <th className="text-right">Max DD</th>
                  <th className="text-right">OOS PF</th>
                  <th className="text-right">Evidence</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((strategy, index) => (
                  <tr key={strategy.id} className="border-t border-zinc-800">
                    <td className="py-2 font-mono text-zinc-500">#{index + 1}</td>
                    <td className="font-medium text-zinc-200">{strategy.name}</td>
                    <td><HealthPill health={strategy.health} /></td>
                    <td className="text-zinc-400">{strategy.family}</td>
                    <td className="text-right font-mono text-zinc-200">
                      {strategy.sharpe != null ? strategy.sharpe.toFixed(2) : "—"}
                    </td>
                    <td className="text-right font-mono text-emerald-300">{usd(strategy.expectancy)}</td>
                    <td className="text-right font-mono text-amber-300">{strategy.maxDrawdownPct.toFixed(1)}%</td>
                    <td className="text-right font-mono text-zinc-200">
                      {strategy.oosProfitFactor != null ? strategy.oosProfitFactor.toFixed(2) : "—"}
                    </td>
                    <td className="text-right font-mono text-sky-300">{strategy.promotionScore}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </TerminalCard>
      <div className="space-y-3">
        <TerminalCard title="Strategy Pool Summary" subtitle="Live from /api/strategy-intelligence">
          {summary ? (
            <div className="grid grid-cols-2 gap-2">
              <Metric label="Total" value={String(summary.total)} />
              <Metric label="Healthy" value={String(summary.healthy)} tone="positive" />
              <Metric label="Warning" value={String(summary.warning)} tone="warning" />
              <Metric label="Critical" value={String(summary.critical)} tone="negative" />
            </div>
          ) : (
            <TerminalNoData />
          )}
        </TerminalCard>
        <TerminalCard title="Retirement Candidates" subtitle="Critical + tier F strategies">
          <RetirementPreview />
        </TerminalCard>
      </div>
    </div>
  );
}

function RetirementPreview() {
  const [rows, setRows] = useState<{ strategy_id: string; expectancy: number; status: string }[]>([]);

  useEffect(() => {
    fetch("/api/strategy-intelligence?view=retirement&limit=20")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data?.ok && Array.isArray(data.strategies)) {
          setRows(data.strategies.slice(0, 5));
        }
      })
      .catch(() => {});
  }, []);

  if (rows.length === 0) return <TerminalNoData label="NO RETIREMENT CANDIDATES" />;

  return (
    <div className="space-y-2 text-xs">
      {rows.map((r) => (
        <div key={r.strategy_id} className="rounded-lg border border-zinc-800 bg-zinc-950/40 p-2">
          <span className="font-semibold text-rose-300">{r.strategy_id}</span>
          <p className="mt-1 text-zinc-400">{r.status} · E={r.expectancy.toFixed(2)}</p>
        </div>
      ))}
    </div>
  );
}

function HealthPill({ health }: { health: "ACTIVE" | "WATCHLIST" | "DISABLED" }) {
  const cls = health === "ACTIVE" ? "bg-emerald-500/10 text-emerald-300" : health === "WATCHLIST" ? "bg-amber-500/10 text-amber-300" : "bg-rose-500/10 text-rose-300";
  return <span className={`rounded-full px-2 py-1 text-[10px] font-semibold ${cls}`}>{health}</span>;
}
