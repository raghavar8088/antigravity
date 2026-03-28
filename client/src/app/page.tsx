"use client";

import { useState } from "react";
import DashboardHeader from "@/components/DashboardHeader";
import StrategyCard from "@/components/StrategyCard";
import MarketTicker from "@/components/MarketTicker";
import RunningTrades from "@/components/RunningTrades";
import TradeHistory from "@/components/TradeHistory";
import useEngineState from "@/hooks/useEngineState";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";

const DEFAULT_STRATEGIES = [
  // ═══ ORIGINAL 20 ═══
  // Trend (6)
  { name: "EMA_Cross_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "TripleEMA_Ribbon_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "HullMA_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ADX_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Ichimoku_TK_Cross_Scalp", category: "Trend", timeframe: "5m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ParabolicSAR_Reversal_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Mean Reversion (7)
  { name: "RSI_Reversal_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Bollinger_Squeeze_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "VWAP_MeanRev_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "MeanReversion_ZScore_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "StochRSI_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "WilliamsR_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "CCI_Divergence_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Breakout (4)
  { name: "Momentum_Breakout_Scalp", category: "Breakout", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Donchian_Breakout_Scalp", category: "Breakout", timeframe: "5m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Keltner_Breakout_Scalp", category: "Breakout", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Pivot_Bounce_Scalp", category: "Breakout", timeframe: "1h", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Momentum (2)
  { name: "MACD_Histogram_Scalp", category: "Momentum", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ROC_Reversal_Scalp", category: "Momentum", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Microstructure (1)
  { name: "OrderFlow_Imbalance_Scalp", category: "Microstructure", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },

  // ═══ ADVANCED 20 (HIGH ALPHA) ═══
  // Velocity (3)
  { name: "TickVelocity_Momentum", category: "Velocity", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "VolumeSpike_Reversal_Scalp", category: "Velocity", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "GapFill_MeanRev_Scalp", category: "Velocity", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Statistical (4)
  { name: "Fibonacci_GoldenRatio_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "LinReg_Statistical_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "EMASpread_MeanRev_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "TripleConsensus_Alpha_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Volatility (2)
  { name: "VolSqueeze_Explosion_Scalp", category: "Volatility", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "RangeCompress_Breakout_Scalp", category: "Volatility", timeframe: "5m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Smart Money (4)
  { name: "OBV_SmartMoney_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ChaikinMF_Flow_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "AccumDistrib_Stealth_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Aroon_EarlyTrend_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Price Action (4)
  { name: "Engulfing_PriceAction_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "HeikinAshi_Momentum_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ZigZag_SwingReversal_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "MicroPullback_Continuation_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // Adaptive (3)
  { name: "Supertrend_Flip_Scalp", category: "Adaptive", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "KAMA_Adaptive_Scalp", category: "Adaptive", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "MTF_RSI_Confluence_Scalp", category: "Adaptive", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
];

const CATEGORIES = [
  "Trend", "Mean Reversion", "Breakout", "Momentum", "Microstructure",
  "Velocity", "Statistical", "Volatility", "Smart Money", "Price Action", "Adaptive"
];

const CAT_COLORS: Record<string, string> = {
  "Trend": "bg-blue-500",
  "Mean Reversion": "bg-purple-500",
  "Breakout": "bg-orange-500",
  "Momentum": "bg-cyan-500",
  "Microstructure": "bg-green-500",
  "Velocity": "bg-yellow-500",
  "Statistical": "bg-indigo-500",
  "Volatility": "bg-rose-500",
  "Smart Money": "bg-emerald-500",
  "Price Action": "bg-amber-500",
  "Adaptive": "bg-teal-500",
};

type RunningTrade = {
  id: string;
  strategy: string;
  side: string;
  size: number;
  entry: number;
  slPct: number;
  tpPct: number;
  openTime: string;
  elapsed: string;
};

const DEFAULT_RUNNING_TRADES: RunningTrade[] = [
  { id: "POS-000001", strategy: "EMA_Cross_Scalp", side: "LONG", size: 0.01, entry: 66350.00, slPct: 0.5, tpPct: 1.0, openTime: "09:00:12", elapsed: "2m 14s" },
  { id: "POS-000002", strategy: "Bollinger_Squeeze_Scalp", side: "LONG", size: 0.01, entry: 66380.00, slPct: 0.4, tpPct: 0.8, openTime: "09:01:08", elapsed: "1m 18s" },
  { id: "POS-000003", strategy: "MACD_Histogram_Scalp", side: "LONG", size: 0.01, entry: 66410.25, slPct: 0.5, tpPct: 1.0, openTime: "09:02:55", elapsed: "0m 32s" },
  { id: "POS-000004", strategy: "RSI_Reversal_Scalp", side: "LONG", size: 0.01, entry: 66425.50, slPct: 0.4, tpPct: 0.9, openTime: "09:03:40", elapsed: "0m 09s" },
];

export default function Home() {
  const { engineOnline, balance } = useEngineState();
  const [strategies, setStrategies] = useState(DEFAULT_STRATEGIES);
  const [runningTrades, setRunningTrades] = useState(DEFAULT_RUNNING_TRADES);
  const btc = useLiveBTCPrice();

  const tradeDailyPnl = runningTrades.reduce((sum, t) => {
    const markPrice = btc.price > 0 ? btc.price : t.entry;
    const pnl = t.side === "LONG"
      ? (markPrice - t.entry) * t.size
      : (t.entry - markPrice) * t.size;
    return sum + pnl;
  }, 0);

  const totalStrategyPnl = tradeDailyPnl;
  const activeCount = strategies.filter(s => s.status === "RUNNING").length;

  const handleReset = () => {
    setStrategies(DEFAULT_STRATEGIES);
    setRunningTrades(DEFAULT_RUNNING_TRADES);
  };

  return (
    <main className="min-h-screen p-6 max-w-[1600px] mx-auto space-y-6">
      <DashboardHeader online={engineOnline} balance={balance} dailyPnL={tradeDailyPnl} onResetSuccess={handleReset} />

      {/* Top Stats */}
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <MarketTicker price={btc.price} prevPrice={btc.prevPrice} change={btc.change24h} connected={btc.connected} ticksPerSecond={btc.ticksPerSecond} />
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Active Strategies</p>
          <p className="text-4xl font-mono font-bold text-white">{activeCount}<span className="text-lg text-gray-500">/{strategies.length}</span></p>
        </div>
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Total Strategy PnL</p>
          <p className={`text-3xl font-mono font-bold ${totalStrategyPnl >= 0 ? "text-green-400" : "text-red-400"}`}>
            {totalStrategyPnl >= 0 ? "+" : ""}${totalStrategyPnl.toFixed(2)}
          </p>
        </div>
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Total Exposure</p>
          <p className="text-3xl font-mono font-bold text-blue-400">
            {strategies.reduce((sum, s) => sum + s.exposure, 0).toFixed(2)} BTC
          </p>
        </div>
      </div>

      {/* Running Trades */}
      <div className="glass-panel p-6">
        <h2 className="text-xl font-bold mb-4 flex items-center gap-3">
          <span className="bg-green-500/10 text-green-400 border border-green-500/20 px-3 py-1 rounded-lg text-xs font-bold tracking-widest">LIVE</span>
          Running Scalp Trades
        </h2>
        <RunningTrades currentPrice={btc.price} trades={runningTrades} />
      </div>

      {/* Trade History */}
      <div className="glass-panel p-6">
        <h2 className="text-xl font-bold mb-4 flex items-center gap-3">
          <span className="bg-blue-500/10 text-blue-400 border border-blue-500/20 px-3 py-1 rounded-lg text-xs font-bold tracking-widest">LOG</span>
          Scalping Trade History
        </h2>
        <TradeHistory />
      </div>

      {/* Strategy Grid — All 40 */}
      <div className="space-y-6">
        {CATEGORIES.map(cat => {
          const catStrategies = strategies.filter(s => s.category === cat);
          if (catStrategies.length === 0) return null;
          return (
            <div key={cat} className="glass-panel p-6">
              <h2 className="text-lg font-bold mb-4 flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full ${CAT_COLORS[cat] || "bg-gray-500"} animate-pulse`}></span>
                {cat}
                <span className="text-xs text-gray-500 font-mono ml-2">{catStrategies.length} strategies</span>
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                {catStrategies.map((s, i) => (
                  <StrategyCard key={i} name={s.name} status={s.status} exposure={s.exposure} profit={s.profit} timeframe={s.timeframe} />
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </main>
  );
}
