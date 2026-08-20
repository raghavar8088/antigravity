"use client";

import { useMemo, useState } from "react";
import type { TerminalSnapshot } from "@/lib/terminal/terminalTypes";
import { pnlClass, px, usd } from "./format";
import { Metric, TerminalCard } from "./TerminalCard";
import { AutoSortTable } from "@/components/desk/ui";

export function TradeJournalPro({ snapshot }: { snapshot: TerminalSnapshot }) {
  const [filter, setFilter] = useState("ALL");
  const trades = useMemo(() => {
    if (filter === "WIN") return snapshot.journal.filter((t) => t.netPnl > 0);
    if (filter === "LOSS") return snapshot.journal.filter((t) => t.netPnl < 0);
    return snapshot.journal;
  }, [filter, snapshot.journal]);
  const selected = trades[0] ?? snapshot.journal[0];
  const exportCsv = () => {
    const header = ["id", "strategy", "family", "side", "entryTime", "exitTime", "entryPrice", "exitPrice", "netPnl", "rMultiple", "setupTag", "exitReason", "holdingMinutes"];
    const rows = trades.map((trade) => header.map((key) => JSON.stringify(trade[key as keyof typeof trade] ?? "")).join(","));
    const blob = new Blob([[header.join(","), ...rows].join("\n")], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `terminal-trades-${filter.toLowerCase()}.csv`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_420px]">
      <TerminalCard
        title="Professional Trade Journal"
        subtitle="Replay-ready trades with setup, R-multiple, holding time, and exit reason"
        actions={
          <div className="flex gap-1">
            {["ALL", "WIN", "LOSS"].map((item) => (
              <button key={item} onClick={() => setFilter(item)} className={`rounded px-2 py-1 text-[10px] font-semibold ${filter === item ? "bg-sky-500/20 text-sky-200" : "bg-zinc-900 text-zinc-500"}`}>
                {item}
              </button>
            ))}
          </div>
        }
      >
        <div className="overflow-x-auto">
          <AutoSortTable><table className="w-full min-w-[820px] text-xs">
            <thead className="text-left text-[10px] uppercase tracking-[0.12em] text-zinc-500">
              <tr>
                <th className="py-2">Trade</th>
                <th>Strategy</th>
                <th>Setup</th>
                <th>Side</th>
                <th className="text-right">Entry / Exit</th>
                <th className="text-right">R</th>
                <th className="text-right">Hold</th>
                <th className="text-right">Net PnL</th>
              </tr>
            </thead>
            <tbody>
              {trades.map((trade) => (
                <tr key={trade.id} className="border-t border-zinc-800">
                  <td className="py-2 font-mono text-zinc-500">{trade.id}</td>
                  <td className="text-zinc-200">{trade.strategy}</td>
                  <td className="text-zinc-400">{trade.setupTag}</td>
                  <td className={trade.side === "BUY" ? "text-emerald-300" : "text-rose-300"}>{trade.side}</td>
                  <td className="text-right font-mono text-zinc-300">{px(trade.entryPrice)} / {px(trade.exitPrice)}</td>
                  <td className="text-right font-mono text-sky-300">{trade.rMultiple.toFixed(2)}R</td>
                  <td className="text-right font-mono text-zinc-400">{trade.holdingMinutes}m</td>
                  <td className={`text-right font-mono font-semibold ${pnlClass(trade.netPnl)}`}>{usd(trade.netPnl, { signed: true })}</td>
                </tr>
              ))}
            </tbody>
          </table></AutoSortTable>
        </div>
      </TerminalCard>

      <div className="space-y-3">
        <TerminalCard title="Replay Mode" subtitle="Selected trade context">
          {selected ? (
            <div className="space-y-3">
              <div className="rounded-lg border border-zinc-800 bg-zinc-950/40 p-3">
                <div className="text-xs font-semibold text-zinc-200">{selected.strategy}</div>
                <div className="mt-1 text-[11px] text-zinc-500">{selected.setupTag} · {selected.exitReason}</div>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <Metric label="R-Multiple" value={`${selected.rMultiple.toFixed(2)}R`} />
                <Metric label="Holding" value={`${selected.holdingMinutes}m`} />
                <Metric label="Entry" value={px(selected.entryPrice)} />
                <Metric label="Exit" value={px(selected.exitPrice)} />
              </div>
              <div className="rounded-lg border border-zinc-800 bg-zinc-950/40 p-3 text-xs text-zinc-400">
                Replay context includes entry chart anchor, signal stack, market regime, risk score, MFE/MAE, and exit decision trail fields.
              </div>
            </div>
          ) : null}
        </TerminalCard>
        <TerminalCard title="Export">
          <button onClick={exportCsv} className="w-full rounded-lg border border-zinc-700 px-3 py-3 text-xs font-semibold uppercase tracking-[0.12em] text-zinc-200 hover:bg-zinc-900">
            Export Filtered Trades CSV
          </button>
        </TerminalCard>
      </div>
    </div>
  );
}
