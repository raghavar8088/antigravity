"use client";

export default function PositionsTable({ currentPrice }: { currentPrice: number }) {
  // Each position now carries SL/TP as absolute price levels.
  const positions = [
    { id: "POS-183012-1", symbol: "BTCUSDT", side: "LONG", entry: 63850.1, size: 0.15, slPct: 0.5, tpPct: 1.0, strategy: "Bollinger_Squeeze_Scalp" },
    { id: "POS-183024-2", symbol: "BTCUSDT", side: "LONG", entry: 64100.0, size: 0.05, slPct: 0.5, tpPct: 1.0, strategy: "EMA_Cross_Scalp" },
    { id: "POS-183105-3", symbol: "BTCUSDT", side: "LONG", entry: 65200.5, size: 0.01, slPct: 0.3, tpPct: 0.8, strategy: "OrderFlow_Imbalance_Scalp" },
  ];

  return (
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
            <th className="py-3 px-2">PnL</th>
            <th className="py-3 px-2 text-right">Status</th>
          </tr>
        </thead>
        <tbody>
          {positions.map((p) => {
            const markPrice = currentPrice > 0 ? currentPrice : p.entry;
            const stopLoss = p.side === "LONG" ? p.entry * (1 - p.slPct / 100) : p.entry * (1 + p.slPct / 100);
            const takeProfit = p.side === "LONG" ? p.entry * (1 + p.tpPct / 100) : p.entry * (1 - p.tpPct / 100);
            const unrealizedPnL = p.side === "LONG" ? (markPrice - p.entry) * p.size : (p.entry - markPrice) * p.size;
            const totalRange = takeProfit - stopLoss;
            const pricePosition = ((markPrice - stopLoss) / totalRange) * 100;
            const clampedPosition = Math.max(0, Math.min(100, pricePosition));

            return (
              <tr key={p.id} className="border-b border-gray-800/50 hover:bg-white/5 transition-colors group">
                <td className="py-4 px-2 font-mono text-xs text-gray-500">{p.id}</td>
                <td className="py-4 px-2 font-mono text-xs text-blue-400">{p.strategy}</td>
                <td className="py-4 px-2">
                  <span className={`px-2 py-1 rounded text-xs font-bold tracking-wider ${p.side === "LONG" ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400"}`}>
                    {p.side}
                  </span>
                </td>
                <td className="py-4 px-2 font-mono text-xs">{p.size} BTC</td>
                <td className="py-4 px-2 font-mono text-xs">${p.entry.toFixed(2)}</td>
                <td className="py-4 px-2 font-mono text-xs text-gray-300">
                  ${markPrice.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                </td>
                <td className="py-4 px-2">
                  <div className="flex flex-col">
                    <span className="font-mono text-xs text-red-400 font-bold">${stopLoss.toFixed(2)}</span>
                    <span className="text-[10px] text-red-500/60">-{p.slPct}%</span>
                  </div>
                </td>
                <td className="py-4 px-2">
                  <div className="flex flex-col">
                    <span className="font-mono text-xs text-green-400 font-bold">${takeProfit.toFixed(2)}</span>
                    <span className="text-[10px] text-green-500/60">+{p.tpPct}%</span>
                  </div>
                </td>
                <td className="py-4 px-2">
                  <div className="flex flex-col gap-1">
                    <span className={`font-mono text-xs font-bold transition-colors duration-150 ${unrealizedPnL >= 0 ? "text-green-400" : "text-red-400"}`}>
                      {unrealizedPnL >= 0 ? "+" : ""}${unrealizedPnL.toFixed(2)}
                    </span>
                    <div className="w-24 h-1.5 bg-gray-800 rounded-full overflow-hidden relative">
                      <div className="absolute inset-0 flex">
                        <div className="w-1/2 bg-gradient-to-r from-red-500/30 to-transparent"></div>
                        <div className="w-1/2 bg-gradient-to-l from-green-500/30 to-transparent"></div>
                      </div>
                      <div
                        className={`absolute h-full w-1 rounded-full ${unrealizedPnL >= 0 ? "bg-green-400" : "bg-red-400"} shadow-[0_0_6px_rgba(255,255,255,0.3)]`}
                        style={{ left: `${clampedPosition}%`, transform: "translateX(-50%)" }}
                      ></div>
                    </div>
                  </div>
                </td>
                <td className="py-4 px-2 text-right">
                  <span className="text-[10px] font-bold px-2 py-1 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20 tracking-wider">
                    ACTIVE
                  </span>
                </td>
              </tr>
            );
          })}
          {positions.length === 0 && (
            <tr>
              <td colSpan={10} className="text-center py-8 text-gray-500 italic">No active positions tracked by the Position Manager.</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
