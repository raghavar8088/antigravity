"use client";

export default function StrategyCard({ name, status, exposure, profit, timeframe }: any) {
  const isRunning = status === "RUNNING";
  
  return (
    <div className={`p-4 rounded-xl border transition-all hover:scale-[1.02] ${
      isRunning 
        ? "bg-blue-500/5 border-blue-500/20 shadow-[0_0_15px_rgba(59,130,246,0.03)]" 
        : "bg-gray-800/30 border-gray-700/50 opacity-60"
    }`}>
      <div className="flex justify-between items-center mb-2">
        <h3 className="font-bold font-mono text-xs tracking-wide text-white truncate mr-2">{name}</h3>
        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded-sm uppercase tracking-widest whitespace-nowrap ${
          isRunning 
            ? "bg-green-500/20 text-green-400 border border-green-500/30" 
            : "bg-gray-600/50 text-gray-400 border border-gray-500/30"
        }`}>
          {status}
        </span>
      </div>
      
      <div className="grid grid-cols-3 gap-1 text-xs mt-3 pt-3 border-t border-gray-700/50">
        <div>
          <p className="text-gray-500 text-[10px] font-semibold uppercase tracking-wider mb-0.5">TF</p>
          <p className="font-mono text-gray-300">{timeframe || "1m"}</p>
        </div>
        <div>
          <p className="text-gray-500 text-[10px] font-semibold uppercase tracking-wider mb-0.5">Exp</p>
          <p className="font-mono text-gray-200">{exposure} BTC</p>
        </div>
        <div>
          <p className="text-gray-500 text-[10px] font-semibold uppercase tracking-wider mb-0.5">PnL</p>
          <p className={`font-mono font-bold ${profit >= 0 ? "text-green-400" : "text-red-400"}`}>
            {profit >= 0 ? "+" : ""}${profit.toFixed(0)}
          </p>
        </div>
      </div>
    </div>
  )
}
