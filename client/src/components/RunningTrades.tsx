"use client";

import { formatUSD } from "@/lib/money";

interface RunningTrade {
  id: string;
  strategy: string;
  side: string;
  size: number;
  entry: number;
  slPct: number;
  tpPct: number;
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

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse shadow-[0_0_8px_#10b981]"></span>
        <span className="text-xs text-gray-400 font-bold uppercase tracking-widest">{trades.length} Active Scalps Running</span>
      </div>

      <div className="w-full overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="text-xs text-gray-400 uppercase tracking-widest border-b border-gray-700/50">
            <tr>
              <th className="py-3 px-2">ID</th>
              <th className="py-3 px-2">Strategy</th>
              <th className="py-3 px-2">Side</th>
              <th className="py-3 px-2">Size</th>
              <th className="py-3 px-2">Entry</th>
              <th className="py-3 px-2">Mark</th>
              <th className="py-3 px-2"><span className="text-red-400">Stop Loss</span></th>
              <th className="py-3 px-2"><span className="text-green-400">Take Profit</span></th>
              <th className="py-3 px-2">Elapsed</th>
              <th className="py-3 px-2">PnL</th>
              <th className="py-3 px-2 text-right">Progress</th>
            </tr>
          </thead>
          <tbody>
            {trades.map((t) => {
              const markPrice = currentPrice > 0 ? currentPrice : t.entry;
              const sl = t.side === "LONG" ? t.entry * (1 - t.slPct / 100) : t.entry * (1 + t.slPct / 100);
              const tp = t.side === "LONG" ? t.entry * (1 + t.tpPct / 100) : t.entry * (1 - t.tpPct / 100);
              const pnl = t.side === "LONG" ? (markPrice - t.entry) * t.size : (t.entry - markPrice) * t.size;
              const totalRange = tp - sl;
              const pricePos = totalRange !== 0 ? ((markPrice - sl) / totalRange) * 100 : 50;
              const clamped = Math.max(0, Math.min(100, pricePos));
              const sideClasses = t.side === "LONG" ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400";

              return (
                <tr key={t.id} className="border-b border-gray-800/50 hover:bg-white/5 transition-colors group">
                  <td className="py-3 px-2 font-mono text-xs text-gray-500">{t.id}</td>
                  <td className="py-3 px-2 font-mono text-xs text-blue-400">{t.strategy}</td>
                  <td className="py-3 px-2">
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wider ${sideClasses}`}>
                      {t.side}
                    </span>
                  </td>
                  <td className="py-3 px-2 font-mono text-xs">{t.size}</td>
                  <td className="py-3 px-2 font-mono text-xs">${t.entry.toFixed(2)}</td>
                  <td className={`py-3 px-2 font-mono text-xs transition-colors duration-150 ${pnl >= 0 ? "text-green-300" : "text-red-300"}`}>
                    ${markPrice.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </td>
                  <td className="py-3 px-2">
                    <span className="font-mono text-xs text-red-400">${sl.toFixed(2)}</span>
                    <span className="text-[9px] text-red-500/50 ml-1">-{t.slPct}%</span>
                  </td>
                  <td className="py-3 px-2">
                    <span className="font-mono text-xs text-green-400">${tp.toFixed(2)}</span>
                    <span className="text-[9px] text-green-500/50 ml-1">+{t.tpPct}%</span>
                  </td>
                  <td className="py-3 px-2 font-mono text-xs text-gray-400">
                    <span className="animate-pulse">{t.elapsed}</span>
                  </td>
                  <td className={`py-3 px-2 font-mono text-xs font-bold ${pnl >= 0 ? "text-green-400" : "text-red-400"}`}>
                    {formatUSD(pnl, { signed: true })}
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
