"use client";

import { useEffect, useState } from "react";
import DashboardHeader from "@/components/DashboardHeader";
import StrategyCard from "@/components/StrategyCard";
import MarketTicker from "@/components/MarketTicker";
import RunningTrades from "@/components/RunningTrades";
import TradeHistory from "@/components/TradeHistory";
import PerformanceCharts from "@/components/PerformanceCharts";
import useEngineState from "@/hooks/useEngineState";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import useStrategies from "@/hooks/useStrategies";
import usePositions from "@/hooks/usePositions";
import useTrades from "@/hooks/useTrades";
import { formatUSD } from "@/lib/money";

type StrategyCardView = {
  name: string;
  category: string;
  timeframe: string;
  status: string;
  exposure: number;
  profit: number;
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

type TradeReason = "TP_HIT" | "SL_HIT" | "TRAILING_STOP" | "BREAK_EVEN" | "MANUAL";
type ChartPricePoint = { time: number; price: number };
type ChartEquityPoint = { time: number; equity: number };
type MarketSentiment = {
  label: "STRONG BUY" | "BULLISH" | "NEUTRAL" | "BEARISH" | "STRONG SELL";
  colorClass: string;
};

const DEFAULT_STRATEGIES: StrategyCardView[] = [
  { name: "EMA_Cross_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "ADX_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "VolumeWeighted_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Pullback_Continuation_Pro_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "VWAP_RSI2_Reversion_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Bollinger_RSI_Fade_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "MACD_VWAP_Flip_Scalp", category: "Momentum Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Stochastic_Range_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Donchian_Breakout_Scalp", category: "Breakout", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "ATR_Breakout_Scalp", category: "Breakout Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "ATR_Volume_Impulse_Scalp", category: "Breakout Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "VolSqueeze_Explosion_Scalp", category: "Volatility", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "RangeCompress_Breakout_Scalp", category: "Volatility", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "PriceChannel_Breakout_Scalp", category: "Breakout Elite", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "OpeningRange_Breakout_Scalp", category: "Time-of-Day", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "VolumeBreakout_Impulse_Scalp", category: "Breakout Elite", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "OrderFlow_Pressure_Pro_Scalp", category: "Microstructure", timeframe: "tick", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "LinReg_Statistical_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "ZScoreBand_MeanRev_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "RSI_BB_Confluence_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "TripleFilter_Alpha_Scalp", category: "Multi-Signal", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Sentiment_Confluence_Pro_Scalp", category: "Multi-Signal", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Exhaustion_Reversal_Scalp", category: "Price Action Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Chart_DoubleTap_Reversal_Scalp", category: "Price Action Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "Chart_Wedge_Breakout_Scalp", category: "Price Action Elite", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "AdaptiveRSI_Dynamic_Scalp", category: "Adaptive Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
  { name: "KAMA_Adaptive_Scalp", category: "Adaptive", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0 },
];

const CATEGORY_ORDER = [
  "Trend",
  "Mean Rev Elite",
  "Mean Reversion",
  "Momentum Elite",
  "Breakout",
  "Breakout Elite",
  "Volatility",
  "Time-of-Day",
  "Microstructure",
  "Statistical",
  "Multi-Signal",
  "Price Action Elite",
  "Adaptive Elite",
  "Adaptive",
];

const CAT_COLORS: Record<string, string> = {
  Trend: "bg-blue-500",
  "Mean Rev Elite": "bg-fuchsia-500",
  "Mean Reversion": "bg-violet-500",
  "Momentum Elite": "bg-cyan-500",
  Breakout: "bg-orange-500",
  "Breakout Elite": "bg-amber-500",
  Volatility: "bg-rose-500",
  "Time-of-Day": "bg-sky-500",
  Microstructure: "bg-emerald-500",
  Statistical: "bg-indigo-500",
  "Multi-Signal": "bg-yellow-500",
  "Price Action Elite": "bg-red-500",
  "Adaptive Elite": "bg-lime-500",
  Adaptive: "bg-teal-500",
};

function mapTradeReason(reason: string): TradeReason {
  switch (reason) {
    case "TAKE_PROFIT":
      return "TP_HIT";
    case "STOP_LOSS":
      return "SL_HIT";
    case "TRAILING_STOP":
      return "TRAILING_STOP";
    case "BREAK_EVEN":
      return "BREAK_EVEN";
    default:
      return "MANUAL";
  }
}

function formatDuration(durationNs: number): string {
  if (durationNs <= 0) {
    return "-";
  }

  const totalSeconds = Math.floor(durationNs / 1e9);
  const mins = Math.floor(totalSeconds / 60);
  const secs = totalSeconds % 60;
  return `${mins}m ${secs}s`;
}

function calcEMA(series: number[], period: number): number[] {
  if (series.length === 0) return [];
  const k = 2 / (period + 1);
  const out: number[] = [series[0]];
  for (let i = 1; i < series.length; i++) {
    out.push(series[i] * k + out[i - 1] * (1 - k));
  }
  return out;
}

function calcRSILast(series: number[], period = 14): number {
  if (series.length < period + 1) return 50;
  let gains = 0;
  let losses = 0;
  for (let i = series.length - period; i < series.length; i++) {
    const change = series[i] - series[i - 1];
    if (change > 0) gains += change;
    if (change < 0) losses -= change;
  }
  if (losses === 0) return 100;
  const rs = (gains / period) / (losses / period);
  return 100 - 100 / (1 + rs);
}

function calcMACDHistLast(series: number[]): number {
  if (series.length < 35) return 0;
  const fast = calcEMA(series, 12);
  const slow = calcEMA(series, 26);
  const macd = fast.map((value, index) => value - slow[index]);
  const signal = calcEMA(macd, 9);
  return macd[macd.length - 1] - signal[signal.length - 1];
}

function calcMarketSentiment(series: number[]): MarketSentiment {
  if (series.length < 40) {
    return { label: "NEUTRAL", colorClass: "text-gray-300" };
  }

  const last = series[series.length - 1];
  const rsi = calcRSILast(series, 14);
  const macdHist = calcMACDHistLast(series);
  const ema9Series = calcEMA(series, 9);
  const ema21Series = calcEMA(series, 21);
  const fairValueSeries = calcEMA(series, 55);
  const ema9 = ema9Series[ema9Series.length - 1] ?? last;
  const ema21 = ema21Series[ema21Series.length - 1] ?? last;
  const fairValue = fairValueSeries[fairValueSeries.length - 1] ?? last;

  let bullishCloses = 0;
  for (let i = Math.max(1, series.length - 5); i < series.length; i++) {
    if (series[i] > series[i - 1]) bullishCloses++;
  }

  let score = 0;
  if (rsi > 60) score += 2;
  else if (rsi < 40) score -= 2;
  if (macdHist > 0) score += 2;
  else if (macdHist < 0) score -= 2;
  if (ema9 > ema21) score += 1;
  else score -= 1;
  if (bullishCloses >= 4) score += 2;
  else if (bullishCloses <= 1) score -= 2;
  if (last > fairValue) score += 1;
  else score -= 1;

  if (score >= 4) return { label: "STRONG BUY", colorClass: "text-green-300" };
  if (score >= 2) return { label: "BULLISH", colorClass: "text-emerald-300" };
  if (score <= -4) return { label: "STRONG SELL", colorClass: "text-red-300" };
  if (score <= -2) return { label: "BEARISH", colorClass: "text-rose-300" };
  return { label: "NEUTRAL", colorClass: "text-gray-300" };
}

export default function Home() {
  const [resetRefreshKey, setResetRefreshKey] = useState(0);
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const { engineOnline, balance: engineBalance } = useEngineState();
  const btc = useLiveBTCPrice();
  const { strategies: liveStrategies } = useStrategies(resetRefreshKey);
  const { positions: livePositions } = usePositions(resetRefreshKey);
  const { trades: liveTrades, stats: liveStats } = useTrades(resetRefreshKey);

  useEffect(() => {
    const interval = setInterval(() => {
      setCurrentTime(Date.now());
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  const displayStrategies: StrategyCardView[] = liveStrategies.length > 0
    ? liveStrategies.map((strategy) => ({
        name: strategy.name,
        category: strategy.category,
        timeframe: strategy.timeframe,
        status: strategy.disabled ? "DISABLED" : "RUNNING",
        exposure: 0,
        profit: strategy.totalPnl,
      }))
    : DEFAULT_STRATEGIES;

  const displayCategories = [...new Set(displayStrategies.map((strategy) => strategy.category))].sort((left, right) => {
    const leftIndex = CATEGORY_ORDER.indexOf(left);
    const rightIndex = CATEGORY_ORDER.indexOf(right);

    if (leftIndex === -1 && rightIndex === -1) return left.localeCompare(right);
    if (leftIndex === -1) return 1;
    if (rightIndex === -1) return -1;
    return leftIndex - rightIndex;
  });

  const runningTrades: RunningTrade[] = livePositions.length > 0
    ? livePositions.map((position) => {
        const openTime = new Date(position.openedAt);
        const elapsedSeconds = Math.floor((currentTime - openTime.getTime()) / 1000);
        const mins = Math.floor(elapsedSeconds / 60);
        const secs = elapsedSeconds % 60;

        return {
          id: position.id,
          strategy: position.strategyName,
          side: position.side === "BUY" ? "LONG" : "SHORT",
          size: position.size,
          entry: position.entryPrice,
          slPct: position.stopLossPct,
          tpPct: position.takeProfitPct,
          openTime: openTime.toLocaleTimeString(),
          elapsed: `${mins}m ${secs}s`,
        };
      })
    : [];

  const balance = liveStats?.balance ?? engineBalance;
  const tradeDailyPnl = liveStats?.dailyPnl ?? runningTrades.reduce((sum, trade) => {
    const markPrice = btc.price > 0 ? btc.price : trade.entry;
    const pnl = trade.side === "LONG"
      ? (markPrice - trade.entry) * trade.size
      : (trade.entry - markPrice) * trade.size;
    return sum + pnl;
  }, 0);

  const totalStrategyPnl = liveStats?.aggregate?.totalPnl ?? tradeDailyPnl;
  const activeCount = displayStrategies.filter((strategy) => strategy.status === "RUNNING").length;
  const priceSeries: ChartPricePoint[] = btc.recentPrices.length > 0
    ? btc.recentPrices
    : btc.price > 0
      ? [{ time: currentTime, price: btc.price }]
      : [];
  const marketSentiment = calcMarketSentiment(priceSeries.map((point) => point.price));
  const strategyBars = displayStrategies.map((strategy) => ({
    name: strategy.name,
    pnl: strategy.profit,
  }));

  const baselineBalance = liveStats
    ? liveStats.balance - liveStats.aggregate.totalPnl
    : 100000;

  const equitySeries: ChartEquityPoint[] = [];
  let cumulativeEquity = baselineBalance;
  const orderedTrades = [...liveTrades].sort(
    (left, right) => new Date(left.exitTime).getTime() - new Date(right.exitTime).getTime()
  );

  for (const trade of orderedTrades) {
    cumulativeEquity += trade.netPnl;
    equitySeries.push({
      time: new Date(trade.exitTime).getTime(),
      equity: cumulativeEquity,
    });
  }

  if (equitySeries.length === 0 && balance > 0) {
    equitySeries.push({ time: currentTime, equity: balance });
  } else if (equitySeries.length > 0 && balance > 0) {
    equitySeries.push({ time: currentTime, equity: balance });
  }

  const handleReset = () => {
    setResetRefreshKey((current) => current + 1);
  };

  return (
    <main className="min-h-screen p-6 max-w-[1600px] mx-auto space-y-6">
      <DashboardHeader online={engineOnline} balance={balance} dailyPnL={tradeDailyPnl} onResetSuccess={handleReset} />

      <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
        <MarketTicker price={btc.price} prevPrice={btc.prevPrice} change={btc.change24h} connected={btc.connected} ticksPerSecond={btc.ticksPerSecond} />
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Active Strategies</p>
          <p className="text-4xl font-mono font-bold text-white">{activeCount}<span className="text-lg text-gray-500">/{displayStrategies.length}</span></p>
        </div>
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Market Sentiment</p>
          <p className={`text-3xl font-mono font-bold ${marketSentiment.colorClass}`}>
            {marketSentiment.label}
          </p>
        </div>
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Total Strategy PnL</p>
          <p className={`text-3xl font-mono font-bold ${totalStrategyPnl >= 0 ? "text-green-400" : "text-red-400"}`}>
            {formatUSD(totalStrategyPnl, { signed: true })}
          </p>
        </div>
        <div className="glass-panel p-6 flex flex-col justify-center">
          <p className="text-xs text-gray-400 font-bold uppercase tracking-[0.15em] mb-1">Total Exposure</p>
          <p className="text-3xl font-mono font-bold text-blue-400">
            {(liveStats?.exposure ?? 0).toFixed(4)} BTC
          </p>
        </div>
      </div>

      <PerformanceCharts priceSeries={priceSeries} equitySeries={equitySeries} strategyBars={strategyBars} />

      {liveStats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
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
        </div>
      )}

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

      <div className="glass-panel p-6">
        <h2 className="text-xl font-bold mb-4 flex items-center gap-3">
          <span className="bg-blue-500/10 text-blue-400 border border-blue-500/20 px-3 py-1 rounded-lg text-xs font-bold tracking-widest">LOG</span>
          Scalping Trade History
          {liveTrades.length > 0 && (
            <span className="text-sm text-gray-400 font-mono">({liveTrades.length} completed)</span>
          )}
        </h2>
        <TradeHistory history={liveTrades.map((trade) => ({
          id: trade.id,
          strategy: trade.strategyName,
          side: trade.side === "BUY" ? "LONG" : "SHORT",
          size: trade.size,
          entry: trade.entryPrice,
          exit: trade.exitPrice,
          pnl: trade.netPnl,
          reason: mapTradeReason(trade.reason),
          duration: formatDuration(trade.duration),
          time: new Date(trade.exitTime).toLocaleTimeString(),
        }))} />
      </div>

      <div className="space-y-6">
        {displayCategories.map((category) => {
          const categoryStrategies = displayStrategies.filter((strategy) => strategy.category === category);
          if (categoryStrategies.length === 0) return null;

          return (
            <div key={category} className="glass-panel p-6">
              <h2 className="text-lg font-bold mb-4 flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full ${CAT_COLORS[category] || "bg-gray-500"} animate-pulse`}></span>
                {category}
                <span className="text-xs text-gray-500 font-mono ml-2">{categoryStrategies.length} strategies</span>
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                {categoryStrategies.map((strategy) => (
                  <StrategyCard
                    key={strategy.name}
                    name={strategy.name}
                    status={strategy.status}
                    exposure={strategy.exposure}
                    profit={strategy.profit}
                    timeframe={strategy.timeframe}
                  />
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </main>
  );
}
