"use client";

import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";

export function ResearchCenter({ snapshot }: { snapshot: TerminalSnapshot }) {
  const sorted = [...snapshot.strategies].sort((a, b) => b.promotionScore - a.promotionScore);
  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
      <TerminalCard title="Strategy Leaderboard" subtitle="Ranked by Sharpe, expectancy, drawdown, OOS PF">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[860px] text-xs">
            <thead className="text-left text-[10px] uppercase tracking-[0.12em] text-zinc-500">
              <tr>
                <th className="py-2">Rank</th>
                <th>Strategy</th>
                <th>Health</th>
                <th>Family</th>
                <th className="text-right">Sharpe</th>
                <th className="text-right">Expectancy</th>
                <th className="text-right">Max DD</th>
                <th className="text-right">OOS PF</th>
                <th className="text-right">Promotion</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((strategy, index) => (
                <tr key={strategy.id} className="border-t border-zinc-800">
                  <td className="py-2 font-mono text-zinc-500">#{index + 1}</td>
                  <td className="font-medium text-zinc-200">{strategy.name}</td>
                  <td><HealthPill health={strategy.health} /></td>
                  <td className="text-zinc-400">{strategy.family}</td>
                  <td className="text-right font-mono text-zinc-200">{strategy.sharpe.toFixed(2)}</td>
                  <td className="text-right font-mono text-emerald-300">{usd(strategy.expectancy)}</td>
                  <td className="text-right font-mono text-amber-300">{strategy.maxDrawdownPct.toFixed(1)}%</td>
                  <td className="text-right font-mono text-zinc-200">{strategy.oosProfitFactor.toFixed(2)}</td>
                  <td className="text-right font-mono text-sky-300">{strategy.promotionScore}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </TerminalCard>
      <div className="space-y-3">
        <TerminalCard title="Research Tournament">
          <div className="grid grid-cols-2 gap-2">
            <Metric label="Active Pool" value="64" />
            <Metric label="Eligible" value="11" tone="positive" />
            <Metric label="Retired" value="7" tone="warning" />
            <Metric label="OOS Pass" value="73%" />
          </div>
        </TerminalCard>
        <TerminalCard title="Walk Forward Validation">
          <div className="space-y-2">
            {["2023H2", "2024H1", "2024H2", "2025H1"].map((fold, i) => (
              <div key={fold} className="grid grid-cols-[80px_1fr_64px] items-center gap-2 text-xs">
                <span className="text-zinc-500">{fold}</span>
                <div className="h-2 rounded-full bg-zinc-950">
                  <div className="h-2 rounded-full bg-emerald-400" style={{ width: `${74 - i * 7}%` }} />
                </div>
                <span className="text-right font-mono text-zinc-300">{(1.42 - i * 0.08).toFixed(2)} PF</span>
              </div>
            ))}
          </div>
        </TerminalCard>
        <TerminalCard title="Promotion / Demotion">
          <div className="space-y-2 text-xs">
            <Event label="Promote" text="Funding Mean Reversion passed benchmark alpha and robustness gate." tone="positive" />
            <Event label="Watch" text="Liquidity Sweep requires another OOS fold before live sizing." tone="warning" />
            <Event label="Disable" text="MSS Continuation failed drawdown stability threshold." tone="negative" />
          </div>
        </TerminalCard>
      </div>
    </div>
  );
}

function HealthPill({ health }: { health: "ACTIVE" | "WATCHLIST" | "DISABLED" }) {
  const cls = health === "ACTIVE" ? "bg-emerald-500/10 text-emerald-300" : health === "WATCHLIST" ? "bg-amber-500/10 text-amber-300" : "bg-rose-500/10 text-rose-300";
  return <span className={`rounded-full px-2 py-1 text-[10px] font-semibold ${cls}`}>{health}</span>;
}

function Event({ label, text, tone }: { label: string; text: string; tone: "positive" | "warning" | "negative" }) {
  const cls = tone === "positive" ? "text-emerald-300" : tone === "warning" ? "text-amber-300" : "text-rose-300";
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-950/40 p-2">
      <span className={`font-semibold ${cls}`}>{label}</span>
      <p className="mt-1 text-zinc-400">{text}</p>
    </div>
  );
}
