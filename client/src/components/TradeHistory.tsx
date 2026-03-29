"use client";

import { useState } from "react";

interface HistoricalTrade {
  id: string;
  strategy: string;
  side: string;
  size: number;
  entry: number;
  exit: number;
  pnl: number;
  reason: "TP_HIT" | "SL_HIT" | "TIMEOUT" | "MANUAL";
  duration: string;
  time: string;
}

const DEFAULT_TRADE_HISTORY: HistoricalTrade[] = [];

export default function TradeHistory({ history = DEFAULT_TRADE_HISTORY }: { history?: HistoricalTrade[] }) {
  const [showAll, setShowAll] = useState(false);
  const tradeHistory = history;
  const visibleTrades = showAll ? tradeHistory : tradeHistory.slice(0, 8);

  const totalTrades = tradeHistory.length;
  const wins = tradeHistory.filter(t => t.reason === "TP_HIT").length;
  const losses = tradeHistory.filter(t => t.reason === "SL_HIT").length;
  const winRate = totalTrades > 0 ? ((wins / totalTrades) * 100).toFixed(1) : "0.0";
  const totalPnl = tradeHistory.reduce((sum, t) => sum + t.pnl, 0);
  const avgWin = wins > 0 ? tradeHistory.filter(t => t.pnl > 0).reduce((s, t) => s + t.pnl, 0) / wins : 0;
  const avgLoss = losses > 0 ? Math.abs(tradeHistory.filter(t => t.pnl < 0).reduce((s, t) => s + t.pnl, 0) / losses) : 0;
  const profitFactor = losses > 0 && avgLoss > 0 ? (avgWin * wins) / (avgLoss * losses) : 0;

  return (
    <div>
      {/* Summary Stats Bar */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-5">
        <div className="bg-gray-800/40 rounded-lg p-3 border border-gray-700/30">
          <p className="text-[10px] text-gray-500 uppercase tracking-wider font-bold">Total Trades</p>
          <p className="text-xl font-mono font-bold text-white">{totalTrades}</p>
        </div>
        <div className="bg-gray-800/40 rounded-lg p-3 border border-gray-700/30">
          <p className="text-[10px] text-gray-500 uppercase tracking-wider font-bold">Win Rate</p>
          <p className={`text-xl font-mono font-bold ${parseFloat(winRate) >= 50 ? "text-green-400" : "text-red-400"}`}>{winRate}%</p>
        </div>
        <div className="bg-gray-800/40 rounded-lg p-3 border border-gray-700/30">
          <p className="text-[10px] text-gray-500 uppercase tracking-wider font-bold">Net PnL</p>
          <p className={`text-xl font-mono font-bold ${totalPnl >= 0 ? "text-green-400" : "text-red-400"}`}>
            {totalPnl >= 0 ? "+" : ""}${totalPnl.toFixed(2)}
          </p>
        </div>
        <div className="bg-gray-800/40 rounded-lg p-3 border border-gray-700/30">
          <p className="text-[10px] text-gray-500 uppercase tracking-wider font-bold">Profit Factor</p>
          <p className={`text-xl font-mono font-bold ${profitFactor >= 1 ? "text-green-400" : "text-red-400"}`}>{profitFactor.toFixed(2)}</p>
        </div>
        <div className="bg-gray-800/40 rounded-lg p-3 border border-gray-700/30">
          <p className="text-[10px] text-gray-500 uppercase tracking-wider font-bold">W / L</p>
          <p className="text-xl font-mono font-bold">
            <span className="text-green-400">{wins}</span>
            <span className="text-gray-600 mx-1">/</span>
            <span className="text-red-400">{losses}</span>
          </p>
        </div>
      </div>

      {/* History Table */}
      <div className="w-full overflow-x-auto">
        {tradeHistory.length === 0 ? (
          <div className="py-14 text-center text-sm text-gray-400">No trade history yet. Session starts from zero.</div>
        ) : (
          <table className="w-full text-left text-sm">
            <thead className="text-xs text-gray-400 uppercase tracking-widest border-b border-gray-700/50">
            <tr>
              <th className="py-3 px-2">Time</th>
              <th className="py-3 px-2">ID</th>
              <th className="py-3 px-2">Strategy</th>
              <th className="py-3 px-2">Side</th>
              <th className="py-3 px-2">Size</th>
              <th className="py-3 px-2">Entry</th>
              <th className="py-3 px-2">Exit</th>
              <th className="py-3 px-2">Duration</th>
              <th className="py-3 px-2">Exit Reason</th>
              <th className="py-3 px-2 text-right">Realized PnL</th>
            </tr>
          </thead>
          <tbody>
            {visibleTrades.map((t, i) => (
               <tr key={i} className="border-b border-gray-800/50 hover:bg-white/5 transition-colors">
                 <td className="py-3 px-2 font-mono text-xs text-gray-500">{t.time}</td>
                 <td className="py-3 px-2 font-mono text-xs text-gray-500">{t.id}</td>
                 <td className="py-3 px-2 font-mono text-xs text-blue-400">{t.strategy}</td>
                 <td className="py-3 px-2">
                   <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wider ${t.side === "LONG" ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400"}`}>
                     {t.side}
                   </span>
                 </td>
                 <td className="py-3 px-2 font-mono text-xs">{t.size} BTC</td>
                 <td className="py-3 px-2 font-mono text-xs">${t.entry.toFixed(2)}</td>
                 <td className="py-3 px-2 font-mono text-xs">${t.exit.toFixed(2)}</td>
                 <td className="py-3 px-2 font-mono text-xs text-gray-400">{t.duration}</td>
                 <td className="py-3 px-2">
                    <span className={`px-2 py-0.5 rounded text-[10px] font-bold tracking-wider ${
                      t.reason === "TP_HIT" 
                        ? "bg-green-500/10 text-green-400 border border-green-500/20" 
                        : t.reason === "SL_HIT"
                        ? "bg-red-500/10 text-red-400 border border-red-500/20"
                        : t.reason === "TIMEOUT"
                        ? "bg-yellow-500/10 text-yellow-400 border border-yellow-500/20"
                        : "bg-gray-500/10 text-gray-400 border border-gray-500/20"
                    }`}>
                      {t.reason === "TP_HIT" ? "🎯 TP HIT" : t.reason === "SL_HIT" ? "🛑 SL HIT" : t.reason === "TIMEOUT" ? "⏰ TIMEOUT" : "✋ MANUAL"}
                    </span>
                 </td>
                 <td className={`py-3 px-2 text-right font-mono text-xs font-bold ${t.pnl >= 0 ? "text-green-400" : "text-red-400"}`}>
                   {t.pnl >= 0 ? "+" : ""}${t.pnl.toFixed(2)}
                 </td>
               </tr>
            ))}
          </tbody>
          </table>
        )}
      </div>

      {/* Show More / Less Toggle */}
      {tradeHistory.length > 8 && (
        <button
          onClick={() => setShowAll(!showAll)}
          className="mt-4 w-full py-2 text-xs font-bold uppercase tracking-widest text-gray-400 hover:text-white border border-gray-700/50 rounded-lg hover:bg-white/5 transition-all"
        >
          {showAll ? `Show Less ↑` : `Show All ${tradeHistory.length} Trades ↓`}
        </button>
      )}
    </div>
  );
}
