"use client";

export default function MarketTickerCard({
  price,
  prevPrice,
  change,
  connected,
  ticksPerSecond,
  exchange,
  connectionState,
  high24h,
  low24h,
  volume24h,
  secondsSinceLastEvent,
}: {
  price: number;
  prevPrice: number;
  change: number;
  connected: boolean;
  ticksPerSecond: number;
  exchange: "binance" | "bybit";
  connectionState: string;
  high24h: number;
  low24h: number;
  volume24h: number;
  secondsSinceLastEvent: number | null;
}) {
  const isUp = change >= 0;
  const priceDirection = price > prevPrice ? "up" : price < prevPrice ? "down" : "neutral";

  return (
    <div className="glass-panel p-6 relative overflow-hidden border border-gray-700/50">
      <div className="absolute top-0 right-0 h-32 w-32 rounded-full bg-blue-500/10 blur-3xl -mr-10 -mt-10 pointer-events-none"></div>

      <p className="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-[0.2em] text-gray-400">
        <span className={`h-1.5 w-1.5 rounded-full ${connected ? "bg-green-400 animate-pulse" : "bg-red-400"}`}></span>
        {exchange === "binance" ? "Binance BTC Feed" : "Bybit BTC Feed"}
        <span className="ml-auto text-[10px] font-mono text-gray-500">{ticksPerSecond} ticks/s</span>
      </p>

      <div className="mt-1 flex items-end gap-3">
        <h2 className={`text-4xl font-mono font-bold drop-shadow-md transition-colors duration-150 ${
          priceDirection === "up" ? "text-green-400" :
          priceDirection === "down" ? "text-red-400" :
          "text-white"
        }`}>
          {price > 0
            ? `$${price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
            : "Loading..."}
        </h2>
        {price > 0 && (
          <p className={`mb-1 text-lg font-mono font-bold tracking-wider ${isUp ? "text-green-400" : "text-red-400"}`}>
            {isUp ? "+" : "-"} {Math.abs(change).toFixed(2)}%
          </p>
        )}
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3 text-xs font-mono">
        <div className="rounded-lg border border-gray-800/60 bg-black/10 px-3 py-2">
          <div className="text-gray-500">State</div>
          <div className={`mt-1 ${connected ? "text-emerald-300" : "text-amber-300"}`}>
            {connectionState.toUpperCase()}
          </div>
        </div>
        <div className="rounded-lg border border-gray-800/60 bg-black/10 px-3 py-2">
          <div className="text-gray-500">Last Event</div>
          <div className="mt-1 text-gray-200">{secondsSinceLastEvent === null ? "-" : `${secondsSinceLastEvent}s ago`}</div>
        </div>
        <div className="rounded-lg border border-gray-800/60 bg-black/10 px-3 py-2">
          <div className="text-gray-500">24h High / Low</div>
          <div className="mt-1 text-gray-200">${high24h.toFixed(0)} / ${low24h.toFixed(0)}</div>
        </div>
        <div className="rounded-lg border border-gray-800/60 bg-black/10 px-3 py-2">
          <div className="text-gray-500">24h Volume</div>
          <div className="mt-1 text-gray-200">{volume24h.toFixed(2)} BTC</div>
        </div>
      </div>

      <div className="mt-5 h-1 w-full overflow-hidden rounded-full bg-gray-800">
        <div
          className={`h-full rounded-full transition-all duration-1000 ${connected ? "bg-gradient-to-r from-blue-600 via-blue-400 to-green-400 opacity-80" : "bg-red-500/50 opacity-40"}`}
          style={{ width: connected ? `${Math.min(100, ticksPerSecond * 3)}%` : "5%" }}
        ></div>
      </div>
    </div>
  );
}
