"use client";

import { useEffect, useState } from "react";
import DashboardHeader from "@/components/DashboardHeader";
import StrategyCard from "@/components/StrategyCard";
import MarketTicker from "@/components/MarketTicker";
import RunningTrades from "@/components/RunningTrades";
import TradeHistory from "@/components/TradeHistory";
import useEngineState from "@/hooks/useEngineState";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import useStrategies from "@/hooks/useStrategies";
import usePositions from "@/hooks/usePositions";
import useTrades from "@/hooks/useTrades";

// Fallback strategies (shown while API connects)
const DEFAULT_STRATEGIES = [
  // ═══ ORIGINAL 20 ═══
  { name: "EMA_Cross_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "TripleEMA_Ribbon_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "HullMA_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ADX_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Ichimoku_TK_Cross_Scalp", category: "Trend", timeframe: "5m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ParabolicSAR_Reversal_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "RSI_Reversal_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Bollinger_Squeeze_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "VWAP_MeanRev_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "MeanReversion_ZScore_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "StochRSI_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "WilliamsR_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "CCI_Divergence_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Momentum_Breakout_Scalp", category: "Breakout", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Donchian_Breakout_Scalp", category: "Breakout", timeframe: "5m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Keltner_Breakout_Scalp", category: "Breakout", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Pivot_Bounce_Scalp", category: "Breakout", timeframe: "1h", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "MACD_Histogram_Scalp", category: "Momentum", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ROC_Reversal_Scalp", category: "Momentum", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "OrderFlow_Imbalance_Scalp", category: "Microstructure", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  // ═══ ADVANCED 20 ═══
  { name: "TickVelocity_Momentum", category: "Velocity", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "VolumeSpike_Reversal_Scalp", category: "Velocity", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "GapFill_MeanRev_Scalp", category: "Velocity", timeframe: "tick", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Fibonacci_GoldenRatio_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "LinReg_Statistical_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "EMASpread_MeanRev_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "TripleConsensus_Alpha_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "VolSqueeze_Explosion_Scalp", category: "Volatility", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "RangeCompress_Breakout_Scalp", category: "Volatility", timeframe: "5m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "OBV_SmartMoney_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ChaikinMF_Flow_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "AccumDistrib_Stealth_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Aroon_EarlyTrend_Scalp", category: "Smart Money", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "Engulfing_PriceAction_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "HeikinAshi_Momentum_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "ZigZag_SwingReversal_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
  { name: "MicroPullback_Continuation_Scalp", category: "Price Action", timeframe: "1m", status: "RUNNING", exposure: 0.0, profit: 0.0 },
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

export default function Home() {
  const [resetRefreshKey, setResetRefreshKey] = useState(0);
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const { engineOnline, balance: engineBalance } = useEngineState();
  const btc = useLiveBTCPrice();
  const { strategies: liveStrategies } = useStrategies(resetRefreshKey);
  const { positions: livePositions } = usePositions(resetRefreshKey);
  const { trades: liveTrades, stats: liveStats } = useTrades(resetRefreshKey);
  const [fallbackStrategies] = useState(DEFAULT_STRATEGIES);

  useEffect(() => {
    const interval = setInterval(() => {
      setCurrentTime(Date.now());
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  // Use live data if available, otherwise fallback
  const hasLiveStrategies = liveStrategies.length > 0;
  const displayStrategies = hasLiveStrategies
    ? liveStrategies.map(s => ({
        name: s.name,
        category: s.category,
        timeframe: s.timeframe,
        status: s.disabled ? "DISABLED" : "RUNNING",
        exposure: 0,
        profit: s.totalPnl,
      }))
    : fallbackStrategies;

  // Build running trades from live positions
  const runningTrades: RunningTrade[] = livePositions.length > 0
    ? livePositions.map(p => {
        const openTime = new Date(p.openedAt);
        const elapsed = Math.floor((currentTime - openTime.getTime()) / 1000);
        const mins = Math.floor(elapsed / 60);
        const secs = elapsed % 60;
        return {
          id: p.id,
          strategy: p.strategyName,
          side: p.side === "BUY" ? "LONG" : "SHORT",
          size: p.size,
          entry: p.entryPrice,
          slPct: p.stopLossPct,
          tpPct: p.takeProfitPct,
          openTime: openTime.toLocaleTimeString(),
          elapsed: `${mins}m ${secs}s`,
        };
      })
    : [];

  // Calculate PnL from live data
  const balance = liveStats?.balance ?? engineBalance;
  const tradeDailyPnl = liveStats?.dailyPnl ?? runningTrades.reduce((sum, t) => {
    const markPrice = btc.price > 0 ? btc.price : t.entry;
    const pnl = t.side === "LONG"
      ? (markPrice - t.entry) * t.size
      : (t.entry - markPrice) * t.size;
    return sum + pnl;
  }, 0);

  const totalStrategyPnl = liveStats?.aggregate?.totalPnl ?? tradeDailyPnl;
  const activeCount = displayStrategies.filter(s => s.status === "RUNNING").length;

  const handleReset = () => {
    setResetRefreshKey((current) => current + 1);
  };

  return (
    <main className="min-h-screen p-6 max-w-[1600px] mx-auto space-y-6">
      <DashboardHeader online={engineOnline} balance={balance} dailyPnL={tradeDailyPnl} onResetSuccess={handleReset} />

      {/* Top Stats */}
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <MarketTicker price={btc.price} prevPrice={btc.prevPrice} change={btc.change24h} connected={btc.connected} ticksPerSecond={btc.ticksPerSecond} />
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Active Strategies</p>
          <p className="text-4xl font-mono font-bold text-white">{activeCount}<span className="text-lg text-gray-500">/{displayStrategies.length}</span></p>
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
            {(liveStats?.exposure ?? 0).toFixed(4)} BTC
          </p>
        </div>
      </div>

      {/* Live Stats Bar */}
      {liveStats && (
        <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
          <div className="glass-panel p-4 text-center">
            <p className="text-xs text-gray-400 uppercase tracking-wider mb-1">Win Rate</p>
            <p className={`text-2xl font-mono font-bold ${liveStats.aggregate.winRate >= 50 ? "text-green-400" : "text-red-400"}`}>
              {liveStats.aggregate.winRate.toFixed(1)}%
            </p>
          </div>
          <div className="glass-panel p-4 text-center">
            <p className="text-xs text-gray-400 uppercase tracking-wider mb-1">Profit Factor</p>
            <p className={`text-2xl font-mono font-bold ${liveStats.aggregate.profitFactor >= 1 ? "text-green-400" : "text-red-400"}`}>
              {liveStats.aggregate.profitFactor.toFixed(2)}
            </p>
          </div>
          <div className="glass-panel p-4 text-center">
            <p className="text-xs text-gray-400 uppercase tracking-wider mb-1">Total Trades</p>
            <p className="text-2xl font-mono font-bold text-white">{liveStats.aggregate.totalTrades}</p>
          </div>
          <div className="glass-panel p-4 text-center">
            <p className="text-xs text-gray-400 uppercase tracking-wider mb-1">Best Trade</p>
            <p className="text-2xl font-mono font-bold text-green-400">+${liveStats.aggregate.bestTrade.toFixed(2)}</p>
          </div>
          <div className="glass-panel p-4 text-center">
            <p className="text-xs text-gray-400 uppercase tracking-wider mb-1">Fees Paid</p>
            <p className="text-2xl font-mono font-bold text-orange-400">${liveStats.totalFees.toFixed(2)}</p>
          </div>
        </div>
      )}

      {/* Running Trades */}
      <div className="glass-panel p-6">
        <h2 className="text-xl font-bold mb-4 flex items-center gap-3">
          <span className="bg-green-500/10 text-green-400 border border-green-500/20 px-3 py-1 rounded-lg text-xs font-bold tracking-widest">LIVE</span>
          Running Scalp Trades
          {livePositions.length > 0 && (
            <span className="text-sm text-gray-400 font-mono">({livePositions.length} active)</span>
          )}
        </h2>
        <RunningTrades currentPrice={btc.price} trades={runningTrades} />
      </div>

      {/* Trade History */}
      <div className="glass-panel p-6">
        <h2 className="text-xl font-bold mb-4 flex items-center gap-3">
          <span className="bg-blue-500/10 text-blue-400 border border-blue-500/20 px-3 py-1 rounded-lg text-xs font-bold tracking-widest">LOG</span>
          Scalping Trade History
          {liveTrades.length > 0 && (
            <span className="text-sm text-gray-400 font-mono">({liveTrades.length} completed)</span>
          )}
        </h2>
        <TradeHistory history={liveTrades.map(t => ({
          id: t.id,
          strategy: t.strategyName,
          side: t.side === "BUY" ? "LONG" : "SHORT",
          size: t.size,
          entry: t.entryPrice,
          exit: t.exitPrice,
          pnl: t.netPnl,
          reason: t.reason === "TAKE_PROFIT" ? "TP_HIT" : t.reason === "STOP_LOSS" || t.reason === "TRAILING_STOP" || t.reason === "BREAK_EVEN" ? "SL_HIT" : "MANUAL",
          duration: t.duration > 0 ? `${Math.floor(t.duration / 1e9 / 60)}m ${Math.floor((t.duration / 1e9) % 60)}s` : "—",
          time: new Date(t.exitTime).toLocaleTimeString(),
        }))} />
      </div>

      {/* Strategy Grid — All 40 */}
      <div className="space-y-6">
        {CATEGORIES.map(cat => {
          const catStrategies = displayStrategies.filter(s => s.category === cat);
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
