"use client";

export default function MarketTicker({ price, prevPrice, change, connected, ticksPerSecond }: {
  price: number;
  prevPrice: number;
  change: number;
  connected: boolean;
  ticksPerSecond: number;
}) {
  const isUp = change >= 0;
  const priceDirection = price > prevPrice ? "up" : price < prevPrice ? "down" : "neutral";
  
  return (
    <div className="glass-panel p-6 relative overflow-hidden border border-gray-700/50">
      {/* Decorative gradient orb */}
      <div className="absolute top-0 right-0 w-32 h-32 bg-blue-500/10 rounded-full blur-3xl -mr-10 -mt-10 pointer-events-none"></div>
      
      <p className="text-gray-400 text-xs font-bold uppercase tracking-[0.2em] mb-2 flex items-center gap-2">
         <span className={`w-1.5 h-1.5 rounded-full ${connected ? "bg-green-400 animate-pulse" : "bg-red-400"}`}></span>
         {connected ? "Live BTC Feed" : "Connecting..."}
         {connected && (
           <span className="ml-auto text-[10px] font-mono text-gray-500">{ticksPerSecond} ticks/s</span>
         )}
      </p>
      
      <div className="flex items-end gap-3 mt-1">
        <h2 className={`text-4xl font-mono font-bold drop-shadow-md transition-colors duration-150 ${
          priceDirection === "up" ? "text-green-400" : 
          priceDirection === "down" ? "text-red-400" : 
          "text-white"
        }`}>
           {price > 0 
             ? `$${price.toLocaleString(undefined, {minimumFractionDigits: 2, maximumFractionDigits: 2})}` 
             : "Loading..."}
        </h2>
        {price > 0 && (
          <p className={`font-mono text-lg font-bold mb-1 tracking-wider ${isUp ? "text-green-400" : "text-red-400"}`}>
            {isUp ? "▲" : "▼"} {Math.abs(change).toFixed(2)}%
          </p>
        )}
      </div>
      
      {/* Live throughput bar */}
      <div className="mt-5 h-1 w-full bg-gray-800 rounded-full overflow-hidden">
         <div 
           className={`h-full rounded-full transition-all duration-1000 ${connected ? "bg-gradient-to-r from-blue-600 via-blue-400 to-green-400 opacity-80" : "bg-red-500/50 opacity-40"}`}
           style={{ width: connected ? `${Math.min(100, ticksPerSecond * 3)}%` : "5%" }}
         ></div>
      </div>
    </div>
  )
}
