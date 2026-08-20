"use client";

import { useEffect, useState } from "react";
import type { StrategyWithMetrics } from "@/lib/strategyAuthority/types";
import { TerminalCard, Metric } from "./TerminalCard";
import { AutoSortTable } from "@/components/desk/ui";

function fmt(n: number, dec = 2) {
  return isNaN(n) ? "—" : n.toFixed(dec);
}

export function MainEngineCenter() {
  const [strategies, setStrategies] = useState<StrategyWithMetrics[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("/api/strategy-authority/main-engine")
      .then((r) => r.json())
      .then((d) => {
        if (d.ok) setStrategies(d.strategies);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  const totalPnl = strategies.reduce((a, s) => a + s.metrics.totalPnl, 0);
  const avgPF = strategies.length
    ? strategies.reduce((a, s) => a + s.metrics.profitFactor, 0) / strategies.length
    : 0;
  const avgWR = strategies.length
    ? strategies.reduce((a, s) => a + s.metrics.winRate, 0) / strategies.length
    : 0;
  const avgDD = strategies.length
    ? strategies.reduce((a, s) => a + s.metrics.maxDrawdown, 0) / strategies.length
    : 0;

  return (
    <div className="m3-page-stack">
      <div className="m3-kpi-strip">
        <Metric
          label="Trade Engine Strategies"
          value={String(strategies.length)}
          tone={strategies.length > 0 ? "positive" : "neutral"}
        />
        <Metric
          label="Combined PnL"
          value={strategies.length ? `$${totalPnl.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}` : "—"}
          tone={totalPnl > 0 ? "positive" : totalPnl < 0 ? "negative" : "neutral"}
        />
        <Metric
          label="Avg Profit Factor"
          value={strategies.length ? fmt(avgPF) : "—"}
          tone={avgPF >= 1.3 ? "positive" : "warning"}
        />
        <Metric
          label="Avg Win Rate"
          value={strategies.length ? `${fmt(avgWR, 1)}%` : "—"}
          tone={avgWR >= 55 ? "positive" : "warning"}
        />
        <Metric
          label="Avg Drawdown"
          value={strategies.length ? `${fmt(avgDD, 1)}%` : "—"}
          tone={avgDD < 10 ? "positive" : "warning"}
        />
        <Metric
          label="Min Required Trades"
          value="575"
        />
      </div>

      <TerminalCard
        title="Trade Engine"
        subtitle="Elite survivors — 575+ trade requirement fulfilled across all admission gates"
      >
        {loading ? (
          <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading main engine strategies…</div>
        ) : strategies.length === 0 ? (
          <div className="py-12 text-center space-y-2">
            <div className="text-sm font-semibold text-zinc-500">No Strategies in Trade Engine</div>
            <div className="text-xs text-zinc-600 max-w-md mx-auto">
              Strategies must complete 575+ total trades across all admission gates with positive expectancy and PF {">"} 1 before earning Trade Engine admission.
            </div>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <AutoSortTable><table className="w-full text-xs">
              <thead>
                <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
                  <th className="py-2 px-2">#</th>
                  <th className="py-2 px-2">Strategy</th>
                  <th className="py-2 px-2">Family</th>
                  <th className="py-2 px-2 text-right">Total PnL</th>
                  <th className="py-2 px-2 text-right">PF</th>
                  <th className="py-2 px-2 text-right">Win Rate</th>
                  <th className="py-2 px-2 text-right">Expectancy</th>
                  <th className="py-2 px-2 text-right">Sharpe</th>
                  <th className="py-2 px-2 text-right">Drawdown</th>
                  <th className="py-2 px-2 text-right">Trades</th>
                  <th className="py-2 px-2 text-center">Status</th>
                </tr>
              </thead>
              <tbody>
                {strategies.map((s, i) => (
                  <tr key={s.strategy_id} className="border-t border-zinc-800 hover:bg-zinc-900/40 transition-colors">
                    <td className="py-1.5 px-2 tabular-nums text-zinc-600 text-[10px]">{i + 1}</td>
                    <td className="py-1.5 px-2 text-xs font-semibold text-emerald-300 max-w-[200px] truncate">{s.strategy_name}</td>
                    <td className="py-1.5 px-2 text-[10px] text-zinc-500">{s.family}</td>
                    <td className={`py-1.5 px-2 tabular-nums text-xs text-right font-mono ${s.metrics.totalPnl >= 0 ? "text-emerald-400" : "text-rose-400"}`}>
                      ${fmt(s.metrics.totalPnl, 0)}
                    </td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-emerald-400 font-semibold">
                      {fmt(s.metrics.profitFactor, 2)}
                    </td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-emerald-400">
                      {fmt(s.metrics.winRate, 1)}%
                    </td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-emerald-400">
                      {fmt(s.metrics.expectancy, 2)}
                    </td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-sky-400">
                      {fmt(s.metrics.sharpeRatio, 2)}
                    </td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-zinc-400">
                      {fmt(s.metrics.maxDrawdown, 1)}%
                    </td>
                    <td className="py-1.5 px-2 tabular-nums text-xs text-right text-zinc-400">
                      {s.metrics.closedTrades}
                    </td>
                    <td className="py-1.5 px-2 text-center">
                      {s.demotionRisk ? (
                        <span className="text-[9px] text-rose-400">↓ RISK</span>
                      ) : (
                        <span className="text-[9px] text-emerald-400">ACTIVE</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table></AutoSortTable>
          </div>
        )}
      </TerminalCard>

      <TerminalCard
        title="Admission Requirements"
        subtitle="All gates must pass before engine admission"
      >
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {[
            { label: "Admission Gate 1", trades: 50, wr: 55, pf: 1.10, dd: "—", sharpe: "—" },
            { label: "Admission Gate 2", trades: 75, wr: 55, pf: 1.15, dd: "25%", sharpe: "—" },
            { label: "Admission Gate 3", trades: 100, wr: 55, pf: 1.20, dd: "20%", sharpe: "—" },
            { label: "Admission Gate 4", trades: 150, wr: 55, pf: 1.25, dd: "15%", sharpe: "0.50" },
            { label: "Engine Admission", trades: 200, wr: 55, pf: 1.30, dd: "10%", sharpe: "0.75" },
            { label: "Total Required", trades: 575, wr: 55, pf: 1.30, dd: "10%", sharpe: "0.75" },
          ].map((r) => (
            <div key={r.label} className={`rounded border p-2.5 ${r.label === "Total Required" ? "border-emerald-800 bg-emerald-950/30" : "border-zinc-800"}`}>
              <div className={`text-[10px] font-bold uppercase tracking-wide mb-1.5 ${r.label === "Total Required" ? "text-emerald-400" : "text-zinc-400"}`}>
                {r.label}
              </div>
              <div className="space-y-0.5 text-[10px] tabular-nums text-zinc-500">
                <div>Trades: <span className="text-zinc-300">{r.trades}</span></div>
                <div>Win Rate: <span className="text-zinc-300">≥{r.wr}%</span></div>
                <div>PF: <span className="text-zinc-300">≥{r.pf}</span></div>
                <div>Drawdown: <span className="text-zinc-300">{"<"}{r.dd}</span></div>
                <div>Sharpe: <span className="text-zinc-300">≥{r.sharpe}</span></div>
              </div>
            </div>
          ))}
        </div>
      </TerminalCard>

      <p className="text-[10px] uppercase tracking-wider text-zinc-600">
        Trade Engine — elite survivors only — no live capital at risk
      </p>
    </div>
  );
}
