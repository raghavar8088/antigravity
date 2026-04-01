"use client";

import { formatUSD } from "@/lib/money";

interface RunningTrade {
  id: string;
  strategy: string;
  side: string;
  size: number;
  entry: number;
  stopLoss: number;
  takeProfit: number;
  originalSize: number;
  trailingActive: boolean;
  partialClosed: boolean;
  openTime: string;
  elapsed: string;
}

export default function RunningTrades({ currentPrice, trades }: { currentPrice: number; trades: RunningTrade[] }) {
  if (trades.length === 0) {
    return (
      <div className="py-14 text-center text-sm text-gray-400">
        No active scalping trades yet. The trading session is starting from zero.
      </div>
    );
  }

  const totalUnrealized = trades.reduce((sum, t) => {
    const mark = currentPrice > 0 ? currentPrice : t.entry;
    return sum + (t.side === "LONG" ? (mark - t.entry) * t.size : (t.entry - mark) * t.size);
  }, 0);

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse shadow-[0_0_8px_#10b981]"></span>
          <span className="text-xs text-gray-400 font-bold uppercase tracking-widest">{trades.length} Active Scalps Running</span>
        </div>
        <span className={`text-xs font-mono font-bold px-2 py-1 rounded ${totalUnrealized >= 0 ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400"}`}>
          Unrealized: {totalUnrealized >= 0 ? "+" : ""}{formatUSD(totalUnrealized)}
        </span>
      </div>

      <div className="w-full overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="text-xs text-gray-400 uppercase tracking-widest border-b border-gray-700/50">
            <tr>
              <th className="py-3 px-2">ID</th>
              <th className="py-3 px-2">Strategy</th>
              <th className="py-3 px-2">Side</th>
              <th className="py-3 px-2">Entry</th>
              <th className="py-3 px-2">Mark</th>
              <th className="py-3 px-2"><span className="text-red-400">Stop</span></th>
              <th className="py-3 px-2"><span className="text-green-400">Target</span></th>
              <th className="py-3 px-2">Size</th>
              <th className="py-3 px-2">Flags</th>
              <th className="py-3 px-2">Elapsed</th>
              <th className="py-3 px-2">PnL</th>
              <th className="py-3 px-2">%</th>
              <th className="py-3 px-2 text-right">Progress</th>
            </tr>
          </thead>
          <tbody>
            {trades.map((t) => {
              const markPrice = currentPrice > 0 ? currentPrice : t.entry;
              const pnl = t.side === "LONG" ? (markPrice - t.entry) * t.size : (t.entry - markPrice) * t.size;
              const pnlPct = t.entry > 0 ? (t.side === "LONG" ? (markPrice - t.entry) / t.entry : (t.entry - markPrice) / t.entry) * 100 : 0;
              const totalRange = t.takeProfit - t.stopLoss;
              const pricePos = totalRange !== 0 ? ((markPrice - t.stopLoss) / totalRange) * 100 : 50;
              const clamped = Math.max(0, Math.min(100, pricePos));
              const sideClasses = t.side === "LONG" ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400";
              const risk = Math.abs(t.entry - t.stopLoss);
              const reward = Math.abs(t.takeProfit - t.entry);
              const rewardToRisk = risk > 0 ? reward / risk : 0;

              return (
                <tr key={t.id} className="border-b border-gray-800/50 hover:bg-white/5 transition-colors group">
                  <td className="py-3 px-2 font-mono text-xs text-gray-500">{t.id}</td>
                  <td className="py-3 px-2 font-mono text-xs text-blue-400">{t.strategy}</td>
                  <td className="py-3 px-2">
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wider ${sideClasses}`}>
                      {t.side}
                    </span>
                  </td>
                  <td className="py-3 px-2 font-mono text-xs">${t.entry.toFixed(2)}</td>
                  <td className={`py-3 px-2 font-mono text-xs transition-colors duration-150 ${pnl >= 0 ? "text-green-300" : "text-red-300"}`}>
                    ${markPrice.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </td>
                  <td className="py-3 px-2">
                    <span className="font-mono text-xs text-red-400">${t.stopLoss.toFixed(2)}</span>
                  </td>
                  <td className="py-3 px-2">
                    <span className="font-mono text-xs text-green-400">${t.takeProfit.toFixed(2)}</span>
                    <span className="ml-2 text-[10px] font-mono text-violet-300">{rewardToRisk.toFixed(2)}R</span>
                  </td>
                  <td className="py-3 px-2 font-mono text-xs text-zinc-300">
                    {t.size.toFixed(4)} BTC
                    {t.originalSize > t.size && (
                      <span className="ml-2 text-[10px] text-zinc-500">from {t.originalSize.toFixed(4)}</span>
                    )}
                  </td>
                  <td className="py-3 px-2">
                    <div className="flex flex-wrap gap-1">
                      {t.trailingActive && (
                        <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wider text-amber-300">
                          Trail
                        </span>
                      )}
                      {t.partialClosed && (
                        <span className="rounded bg-fuchsia-500/10 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wider text-fuchsia-300">
                          TP1
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="py-3 px-2 font-mono text-xs text-gray-400">
                    <span className="animate-pulse">{t.elapsed}</span>
                  </td>
                  <td className={`py-3 px-2 font-mono text-xs font-bold ${pnl >= 0 ? "text-green-400" : "text-red-400"}`}>
                    {formatUSD(pnl, { signed: true })}
                  </td>
                  <td className={`py-3 px-2 font-mono text-xs ${pnlPct >= 0 ? "text-green-400" : "text-red-400"}`}>
                    {pnlPct >= 0 ? "+" : ""}{pnlPct.toFixed(2)}%
                  </td>
                  <td className="py-3 px-2">
                    <div className="w-20 h-2 bg-gray-800 rounded-full overflow-hidden relative">
                      <div className="absolute inset-0 flex">
                        <div className="w-1/2 bg-gradient-to-r from-red-500/20 to-transparent"></div>
                        <div className="w-1/2 bg-gradient-to-l from-green-500/20 to-transparent"></div>
                      </div>
                      <div
                        className={`absolute h-full w-1.5 rounded-full transition-all duration-500 ${pnl >= 0 ? "bg-green-400 shadow-[0_0_6px_#10b981]" : "bg-red-400 shadow-[0_0_6px_#ef4444]"}`}
                        style={{ left: `${clamped}%`, transform: "translateX(-50%)" }}
                      ></div>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
