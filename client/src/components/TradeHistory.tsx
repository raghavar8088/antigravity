"use client";

import { useState } from "react";
import { formatUSD } from "@/lib/money";
import { formatShortDate, formatShortTime } from "@/lib/time";

type ExitReason = "TP_HIT" | "SL_HIT" | "TRAILING_STOP" | "BREAK_EVEN" | "MANUAL";

interface HistoricalTrade {
  id: string;
  strategy: string;
  side: string;
  size: number;
  entry: number;
  exit: number;
  pnl: number;
  reason: ExitReason;
  duration: string;
  time: string;
}

const reasonStyles: Record<ExitReason, { bg: string; color: string }> = {
  TP_HIT: { bg: "var(--green-dim)", color: "var(--green)" },
  SL_HIT: { bg: "var(--red-dim)", color: "var(--red)" },
  TRAILING_STOP: { bg: "var(--amber-dim)", color: "var(--amber)" },
  BREAK_EVEN: { bg: "var(--accent-dim)", color: "var(--accent)" },
  MANUAL: { bg: "var(--surface-3)", color: "var(--text-secondary)" },
};

const reasonLabels: Record<ExitReason, string> = {
  TP_HIT: "TP",
  SL_HIT: "SL",
  TRAILING_STOP: "TRAIL",
  BREAK_EVEN: "EVEN",
  MANUAL: "MANUAL",
};

export default function TradeHistory({
  history = [],
  showSummary = true,
}: {
  history?: HistoricalTrade[];
  showSummary?: boolean;
}) {
  const [showAll, setShowAll] = useState(false);
  const visibleTrades = showAll ? history : history.slice(0, 8);
  const totalTrades = history.length;
  const wins = history.filter((t) => t.pnl > 0).length;
  const losses = history.filter((t) => t.pnl < 0).length;
  const winRate = totalTrades > 0 ? (wins / totalTrades) * 100 : 0;
  const totalPnl = history.reduce((sum, t) => sum + t.pnl, 0);
  const grossProfit = history.filter((t) => t.pnl > 0).reduce((sum, t) => sum + t.pnl, 0);
  const grossLoss = history.filter((t) => t.pnl < 0).reduce((sum, t) => sum + Math.abs(t.pnl), 0);
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : 0;

  return (
    <div className="space-y-4">
      {showSummary ? (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
          {[
            { label: "Trades", value: `${totalTrades}`, tone: "var(--text-primary)" },
            { label: "Win Rate", value: `${winRate.toFixed(1)}%`, tone: winRate >= 50 ? "var(--green)" : "var(--red)" },
            { label: "Net PnL", value: formatUSD(totalPnl, { signed: true }), tone: totalPnl >= 0 ? "var(--green)" : "var(--red)" },
            { label: "Profit Factor", value: profitFactor.toFixed(2), tone: profitFactor >= 1 ? "var(--green)" : "var(--red)" },
            { label: "W / L", value: `${wins}/${losses}`, tone: "var(--text-primary)" },
          ].map((item) => (
            <div key={item.label} className="summary-card">
              <div className="summary-label">{item.label}</div>
              <div className="summary-value" style={{ color: item.tone }}>{item.value}</div>
            </div>
          ))}
        </div>
      ) : null}

      {history.length === 0 ? (
        <div className="py-12 text-center text-sm" style={{ color: "var(--text-secondary)" }}>No trade history yet.</div>
      ) : (
        <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
          <table className="w-full text-left text-sm">
            <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
              <tr className="text-[11px] uppercase tracking-[0.12em]">
                <th className="px-4 py-3 font-medium">Time</th>
                <th className="px-4 py-3 font-medium">Strategy</th>
                <th className="px-4 py-3 font-medium">Side</th>
                <th className="px-4 py-3 font-medium">Entry</th>
                <th className="px-4 py-3 font-medium">Exit</th>
                <th className="px-4 py-3 font-medium">Duration</th>
                <th className="px-4 py-3 font-medium">Reason</th>
                <th className="px-4 py-3 font-medium text-right">PnL</th>
              </tr>
            </thead>
            <tbody>
              {visibleTrades.map((t) => {
                const style = reasonStyles[t.reason];
                return (
                  <tr key={t.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3 text-xs">
                      {t.time && t.time !== "-" ? (
                        <div>
                          <div className="font-mono" style={{ color: "var(--text-primary)" }}>{formatShortTime(t.time)}</div>
                          <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(t.time)}</div>
                        </div>
                      ) : (
                        <span style={{ color: "var(--text-secondary)" }}>-</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>{t.strategy}</div>
                      <div className="text-[11px] font-mono" style={{ color: "var(--text-secondary)" }}>{t.id}</div>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className="rounded-full px-2 py-1 text-[10px] font-medium uppercase"
                        style={{ background: t.side === "LONG" ? "var(--green-dim)" : "var(--red-dim)", color: t.side === "LONG" ? "var(--green)" : "var(--red)" }}
                      >
                        {t.side}
                      </span>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>${t.entry.toFixed(2)}</td>
                    <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>${t.exit.toFixed(2)}</td>
                    <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{t.duration}</td>
                    <td className="px-4 py-3">
                      <span className="rounded-full px-2 py-1 text-[10px] font-medium" style={{ background: style.bg, color: style.color }}>{reasonLabels[t.reason]}</span>
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-sm font-semibold" style={{ color: t.pnl >= 0 ? "var(--green)" : "var(--red)" }}>
                      {formatUSD(t.pnl, { signed: true })}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {history.length > 8 && (
        <button onClick={() => setShowAll(!showAll)} className="btn-gold w-full">
          {showAll ? "Show less" : `Show all ${history.length} trades`}
        </button>
      )}
    </div>
  );
}
