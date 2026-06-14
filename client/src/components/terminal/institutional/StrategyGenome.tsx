"use client";

import { useMemo } from "react";
import { TerminalCard } from "./TerminalCard";
import type { StrategyWithMetrics } from "@/lib/strategyAuthority/types";

interface StrategyGenomeProps {
  strategies?: StrategyWithMetrics[];
  loading?: boolean;
}

function familyOf(name: string): string {
  const lower = name.toLowerCase();
  if (lower.includes("ema")) return "EMA";
  if (lower.includes("rsi")) return "RSI";
  if (lower.includes("bb") || lower.includes("bollinger")) return "Bollinger";
  if (lower.includes("vwap")) return "VWAP";
  if (lower.includes("cvd") || lower.includes("delta")) return "CVD";
  if (lower.includes("liquidity") || lower.includes("sweep")) return "Liquidity";
  if (lower.includes("funding")) return "Funding";
  if (lower.includes("adx") || lower.includes("momentum")) return "Momentum";
  if (lower.includes("orb") || lower.includes("opening")) return "ORB";
  return "Research";
}

function fmt(n: number | undefined, dec = 2): string {
  if (n == null || !Number.isFinite(n)) return "—";
  return n.toFixed(dec);
}

/**
 * StrategyGenome — visualises the portfolio's strategy mix as a family
 * breakdown table, giving a quick "genome view" of what families dominate.
 */
export function StrategyGenome({ strategies = [], loading = false }: StrategyGenomeProps) {
  const familyRows = useMemo(() => {
    const map = new Map<string, { count: number; totalPnl: number; totalWr: number }>();
    for (const s of strategies) {
      const fam = familyOf(s.strategy_name);
      const prev = map.get(fam) ?? { count: 0, totalPnl: 0, totalWr: 0 };
      map.set(fam, {
        count: prev.count + 1,
        totalPnl: prev.totalPnl + (s.metrics.totalPnl ?? 0),
        totalWr: prev.totalWr + (s.metrics.winRate ?? 0),
      });
    }
    return [...map.entries()]
      .map(([family, { count, totalPnl, totalWr }]) => ({
        family,
        count,
        totalPnl,
        avgWr: count > 0 ? totalWr / count : 0,
      }))
      .sort((a, b) => b.count - a.count);
  }, [strategies]);

  return (
    <TerminalCard title="Strategy Genome" subtitle="Family composition">
      {loading ? (
        <div className="py-8 text-center text-xs text-zinc-600 animate-pulse">Loading…</div>
      ) : familyRows.length === 0 ? (
        <div className="py-8 text-center text-xs text-zinc-600">No strategies to analyse</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
                <th className="py-2 px-2">Family</th>
                <th className="py-2 px-2 text-right">Count</th>
                <th className="py-2 px-2 text-right">Net PnL</th>
                <th className="py-2 px-2 text-right">Avg WR%</th>
              </tr>
            </thead>
            <tbody>
              {familyRows.map((row) => (
                <tr key={row.family} className="border-t border-zinc-800 hover:bg-zinc-900/40">
                  <td className="py-1.5 px-2 font-semibold text-zinc-200">{row.family}</td>
                  <td className="py-1.5 px-2 tabular-nums text-right text-zinc-400">{row.count}</td>
                  <td
                    className={`py-1.5 px-2 tabular-nums text-right font-semibold ${row.totalPnl >= 0 ? "text-emerald-400" : "text-rose-400"}`}
                  >
                    {row.totalPnl >= 0 ? "+" : ""}${row.totalPnl.toFixed(0)}
                  </td>
                  <td className="py-1.5 px-2 tabular-nums text-right text-sky-400">
                    {fmt(row.avgWr * 100, 1)}%
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </TerminalCard>
  );
}
