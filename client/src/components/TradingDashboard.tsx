"use client";

import { startTransition, useDeferredValue, useEffect, useRef, useState } from "react";
import ActivityFeed from "@/components/ActivityFeed";
import DashboardHeader from "@/components/DashboardHeader";
import OptionsAccountHeader from "@/components/OptionsAccountHeader";
import MarketChart from "@/components/MarketChart";
import PerformanceCharts from "@/components/PerformanceCharts";
import RunningTrades from "@/components/RunningTrades";
import SignalInsightCard from "@/components/SignalInsightCard";
import StrategyCard from "@/components/StrategyCard";
import TradeHistory from "@/components/TradeHistory";
import AIInsightPanel from "@/components/AIInsightPanel";
import CommandCenter from "@/components/CommandCenter";
import FearGreedWidget from "@/components/FearGreedWidget";
import OptionsScalper from "@/components/OptionsScalper";
import BTCOptionChain from "@/components/BTCOptionChain";
import useAIInsights from "@/hooks/useAIInsights";
import useEngineLogs from "@/hooks/useEngineLogs";
import useEngineState from "@/hooks/useEngineState";
import useLiveBTCMarket from "@/hooks/useLiveBTCMarket";
import useOptions from "@/hooks/useOptions";
import usePositions from "@/hooks/usePositions";
import useStrategies from "@/hooks/useStrategies";
import useTrades from "@/hooks/useTrades";
import { formatUSD } from "@/lib/money";
import { formatElapsed, safeFormatDate } from "@/lib/time";
import { calcMarketSentiment, detectMarketSignal } from "@/lib/marketSignal";

type StrategyCardView = {
  name: string;
  category: string;
  timeframe: string;
  status: string;
  exposure: number;
  profit: number;
  wins: number;
  losses: number;
  totalTrades: number;
};

type FeedTone = "info" | "buy" | "sell" | "win" | "loss" | "admin";

type FeedEntry = {
  id: string;
  message: string;
  tone: FeedTone;
  time: number;
};

type RunningTradeView = {
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
};

type TradeReason = "TP_HIT" | "SL_HIT" | "TRAILING_STOP" | "BREAK_EVEN" | "MANUAL";

type ChartPricePoint = { time: number; price: number };
type ChartEquityPoint = { time: number; equity: number };

const SOUND_STORAGE_KEY = "raig.sound.enabled";
const INITIAL_BALANCE = 1000000;
const INITIAL_OPTIONS_BALANCE = 1000000;

const DEFAULT_STRATEGIES: StrategyCardView[] = [
  { name: "EMA_Cross_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "ADX_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "VolumeWeighted_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Pullback_Continuation_Pro_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "VWAP_Bounce_Pro_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "VWAP_RSI2_Reversion_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Bollinger_RSI_Fade_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "MACD_VWAP_Flip_Scalp", category: "Momentum Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Stochastic_Range_Scalp", category: "Mean Reversion", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Donchian_Breakout_Scalp", category: "Breakout", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "ATR_Breakout_Scalp", category: "Breakout Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "ATR_Volume_Impulse_Scalp", category: "Breakout Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "VolSqueeze_Explosion_Scalp", category: "Volatility", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "RangeCompress_Breakout_Scalp", category: "Volatility", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "OpeningRange_Breakout_Scalp", category: "Time-of-Day", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "VolumeBreakout_Impulse_Scalp", category: "Breakout Elite", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "OrderFlow_Pressure_Pro_Scalp", category: "Microstructure", timeframe: "tick", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "LinReg_Statistical_Scalp", category: "Statistical", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "ZScoreBand_MeanRev_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "RSI_BB_Confluence_Scalp", category: "Mean Rev Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "TripleFilter_Alpha_Scalp", category: "Multi-Signal", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Sentiment_Confluence_Pro_Scalp", category: "Multi-Signal", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "TrendMomentum_Score_Scalp", category: "Multi-Signal", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Exhaustion_Reversal_Scalp", category: "Price Action Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Chart_DoubleTap_Reversal_Scalp", category: "Price Action Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "Chart_Wedge_Breakout_Scalp", category: "Price Action Elite", timeframe: "5m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "RSI_MACD_Divergence_Scalp", category: "Price Action Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "AdaptiveRSI_Dynamic_Scalp", category: "Adaptive Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "KAMA_Adaptive_Scalp", category: "Adaptive", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "SessionOpen_Momentum_Scalp", category: "Time-of-Day", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "TripleTrend_Confluence_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "VolumeDelta_Spike_Scalp", category: "Microstructure", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "MACD_ZeroCross_Confluence_Scalp", category: "Momentum Elite", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
  { name: "BollingerWalk_Trend_Scalp", category: "Trend", timeframe: "1m", status: "RUNNING", exposure: 0, profit: 0, wins: 0, losses: 0 },
].map((strategy) => ({ ...strategy, totalTrades: 0 }));

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

function readStoredSound(): boolean {
  if (typeof window === "undefined") {
    return true;
  }
  const value = window.localStorage.getItem(SOUND_STORAGE_KEY);
  return value === null ? true : value === "true";
}

function formatDuration(durationNs: number): string {
  if (durationNs <= 0) {
    return "-";
  }

  const totalSeconds = Math.floor(durationNs / 1e9);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return minutes > 0 ? `${minutes}m ${seconds}s` : `${seconds}s`;
}

function formatElapsedSeconds(totalSeconds: number): string {
  if (totalSeconds <= 0) {
    return "0m 0s";
  }

  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m ${seconds}s`;
}

function beep(kind: "buy" | "sell" | "win" | "loss") {
  try {
    const webkitAudioContext = (window as Window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    const AudioContextConstructor = window.AudioContext || webkitAudioContext;
    if (!AudioContextConstructor) {
      return;
    }

    const context = new AudioContextConstructor();
    const oscillator = context.createOscillator();
    const gain = context.createGain();

    oscillator.connect(gain);
    gain.connect(context.destination);

    const frequency = kind === "buy"
      ? 880
      : kind === "sell"
        ? 520
        : kind === "win"
          ? 960
          : 420;

    oscillator.frequency.setValueAtTime(frequency, context.currentTime);
    gain.gain.setValueAtTime(0.08, context.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, context.currentTime + 0.22);
    oscillator.start();
    oscillator.stop(context.currentTime + 0.22);
  } catch {
    // Ignore audio failures.
  }
}

function makeFeedEntry(message: string, tone: FeedTone): FeedEntry {
  return {
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    message,
    tone,
    time: Date.now(),
  };
}

function SummaryCard({
  label,
  value,
  accent,
}: {
  label: string;
  value: string;
  accent: string;
}) {
  return (
    <div className="summary-card flex min-h-[112px] flex-col justify-between gap-3">
      <div className="summary-label">{label}</div>
      <div className={`summary-value ${accent}`}>{value}</div>
    </div>
  );
}

type BadgeTone = "neutral" | "positive" | "negative" | "info" | "warning";

function BadgePill({
  label,
  tone = "neutral",
}: {
  label: string;
  tone?: BadgeTone;
}) {
  const toneClasses: Record<BadgeTone, string> = {
    neutral: "border-zinc-200 bg-white text-zinc-600",
    positive: "border-emerald-200 bg-emerald-50 text-emerald-700",
    negative: "border-rose-200 bg-rose-50 text-rose-700",
    info: "border-blue-200 bg-blue-50 text-blue-700",
    warning: "border-amber-200 bg-amber-50 text-amber-700",
  };

  return (
    <span className={`inline-flex items-center rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] ${toneClasses[tone]}`}>
      {label}
    </span>
  );
}

function CompactMetric({
  label,
  value,
  detail,
  accent = "",
}: {
  label: string;
  value: string;
  detail?: string;
  accent?: string;
}) {
  return (
    <div className="metric-card flex min-h-[104px] flex-col justify-between gap-3">
      <div>
        <div className="metric-label">
          {label}
        </div>
        <div className={`metric-value ${accent}`}>
          {value}
        </div>
      </div>
      <div className="text-xs" style={{ color: "var(--text-secondary)", minHeight: 18 }}>
        {detail ?? ""}
      </div>
    </div>
  );
}

export default function TradingDashboard() {
  const [resetRefreshKey, setResetRefreshKey] = useState(0);
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [activeModule, setActiveModule] = useState<"dashboard" | "engine" | "history" | "options" | "chain">("dashboard");
  const [activeTab, setActiveTab] = useState<"trade" | "stats" | "strategies" | "history" | "feed">("trade");
  const [isSoundOn, setIsSoundOn] = useState(() => readStoredSound());
  const [isClearingLedger, setIsClearingLedger] = useState(false);
  const [feed, setFeed] = useState<FeedEntry[]>([]);
  const [combatMode, setCombatMode] = useState(false);
  const [milestoneToast, setMilestoneToast] = useState<string | null>(null);
  const milestoneRef = useRef<Set<number>>(new Set());
  const milestoneTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const { engineOnline, balance: engineBalance } = useEngineState();
  const { positions: optionPositions, stats: optionStats } = useOptions();
  const market = useLiveBTCMarket();
  const deferredCandles = useDeferredValue(market.candles);
  const { strategies: liveStrategies } = useStrategies(resetRefreshKey);
  const { positions: livePositions } = usePositions(resetRefreshKey);
  const { trades: liveTrades, stats: liveStats } = useTrades(resetRefreshKey);
  const { logs: engineLogs } = useEngineLogs(resetRefreshKey);
  const aiInsights = useAIInsights(resetRefreshKey);

  const previousConnectionState = useRef<string>("");
  const previousExchange = useRef<string>("");
  const previousPositionIds = useRef<string[]>([]);
  const previousTradeIds = useRef<string[]>([]);
  const positionsHydrated = useRef(false);
  const tradesHydrated = useRef(false);

  useEffect(() => {
    const interval = setInterval(() => {
      setCurrentTime(Date.now());
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(SOUND_STORAGE_KEY, String(isSoundOn));
    }
  }, [isSoundOn]);

  useEffect(() => {
    return () => {
      if (milestoneTimeoutRef.current !== null) {
        clearTimeout(milestoneTimeoutRef.current);
      }
    };
  }, []);

  const pushFeed = (message: string, tone: FeedTone = "info") => {
    startTransition(() => {
      setFeed((previous) => [makeFeedEntry(message, tone), ...previous].slice(0, 80));
    });
  };

  useEffect(() => {
    if (previousExchange.current === market.exchange) {
      return;
    }
    previousExchange.current = market.exchange;
    pushFeed(
      `Streaming market data from ${market.exchange === "binance" ? "Binance" : "Bybit"}.`,
      "info",
    );
  }, [market.exchange]);

  useEffect(() => {
    if (previousConnectionState.current === market.connectionState) {
      return;
    }

    previousConnectionState.current = market.connectionState;
    if (market.connectionState === "live") {
      pushFeed(`Market feed connected on ${market.exchange}.`, "info");
      return;
    }
    if (market.connectionState === "reconnecting") {
      pushFeed(`Market feed reconnecting on ${market.exchange}.`, "info");
      return;
    }
    if (market.connectionState === "error" && market.connectionError) {
      pushFeed(`Market feed error: ${market.connectionError}`, "loss");
    }
  }, [market.connectionError, market.connectionState, market.exchange]);

  useEffect(() => {
    if (!positionsHydrated.current) {
      positionsHydrated.current = true;
      previousPositionIds.current = livePositions.map((position) => position.id);
      return;
    }

    const previousIds = new Set(previousPositionIds.current);
    for (const position of livePositions) {
      if (previousIds.has(position.id)) {
        continue;
      }

      const side = position.side === "BUY" ? "LONG" : "SHORT";
      pushFeed(
        `${side} ${position.strategyName} opened @ ${formatUSD(position.entryPrice)}`,
        position.side === "BUY" ? "buy" : "sell",
      );
      if (isSoundOn) {
        beep(position.side === "BUY" ? "buy" : "sell");
      }
    }
    previousPositionIds.current = livePositions.map((position) => position.id);
  }, [isSoundOn, livePositions]);

  useEffect(() => {
    if (!tradesHydrated.current) {
      tradesHydrated.current = true;
      previousTradeIds.current = liveTrades.map((trade) => trade.id);
      return;
    }

    const previousIds = new Set(previousTradeIds.current);
    for (const trade of liveTrades) {
      if (previousIds.has(trade.id)) {
        continue;
      }

      const positive = trade.netPnl >= 0;
      pushFeed(
        `${trade.strategyName} closed ${positive ? "green" : "red"} ${formatUSD(trade.netPnl, { signed: true })}`,
        positive ? "win" : "loss",
      );
      if (isSoundOn) {
        beep(positive ? "win" : "loss");
      }
    }
    previousTradeIds.current = liveTrades.map((trade) => trade.id);
  }, [isSoundOn, liveTrades]);

  const displayStrategies: StrategyCardView[] = liveStrategies.length > 0
    ? liveStrategies.map((strategy) => ({
        name: strategy.name,
        category: strategy.category,
        timeframe: strategy.timeframe,
        status: strategy.disabled ? "DISABLED" : "RUNNING",
        exposure: 0,
        profit: strategy.totalPnl,
        wins: strategy.wins,
        losses: strategy.losses,
        totalTrades: strategy.totalTrades || (strategy.wins + strategy.losses),
      }))
    : DEFAULT_STRATEGIES;

  const displayCategories = [...new Set(displayStrategies.map((strategy) => strategy.category))].sort((left, right) => {
    const leftIndex = CATEGORY_ORDER.indexOf(left);
    const rightIndex = CATEGORY_ORDER.indexOf(right);

    if (leftIndex === -1 && rightIndex === -1) {
      return left.localeCompare(right);
    }
    if (leftIndex === -1) {
      return 1;
    }
    if (rightIndex === -1) {
      return -1;
    }
    return leftIndex - rightIndex;
  });

  const runningTrades: RunningTradeView[] = livePositions.map((position) => {
    const openedAt = new Date(position.openedAt);
    const validDate = !isNaN(openedAt.getTime());
    const elapsedSeconds = validDate ? Math.max(0, Math.floor((currentTime - openedAt.getTime()) / 1000)) : 0;

    return {
      id: position.id,
      strategy: position.strategyName,
      side: position.side === "BUY" ? "LONG" : "SHORT",
      size: position.size,
      entry: position.entryPrice,
      stopLoss: position.stopLoss,
      takeProfit: position.takeProfit,
      originalSize: position.originalSize,
      trailingActive: position.trailingActive,
      partialClosed: position.partialClosed,
      openTime: safeFormatDate(position.openedAt),
      elapsed: formatElapsed(elapsedSeconds),
    };
  });

  const closedPnl = liveStats?.aggregate?.totalPnl ?? liveTrades.reduce((sum, trade) => sum + trade.netPnl, 0);
  const tradeCount = Math.max(liveStats?.aggregate.totalTrades ?? 0, liveTrades.length);
  const price = market.price > 0 ? market.price : liveStats?.lastPrice ?? 0;
  const unrealized = livePositions.reduce((sum, position) => {
    const markPrice = price > 0 ? price : position.entryPrice;
    const pnl = position.side === "BUY"
      ? (markPrice - position.entryPrice) * position.size
      : (position.entryPrice - markPrice) * position.size;
    return sum + pnl;
  }, 0);
  const derivedEquity = INITIAL_BALANCE + closedPnl + unrealized;
  const balance = liveStats?.equity ?? derivedEquity;
  const sessionPnl = balance - INITIAL_BALANCE;
  const priceSeries: ChartPricePoint[] = market.recentPrices.length > 0
    ? market.recentPrices
    : price > 0
      ? [{ time: currentTime, price }]
      : [];
  const secondsSinceLastMarketEvent = market.lastMarketEventAt
    ? Math.max(0, Math.floor((currentTime - market.lastMarketEventAt) / 1000))
    : null;
  const marketSentiment = calcMarketSentiment(priceSeries.map((point) => point.price));
  const latestSignal = detectMarketSignal(deferredCandles);
  const strategyBars = displayStrategies.map((strategy) => ({
    name: strategy.name,
    pnl: strategy.profit,
  }));

  const baselineBalance = INITIAL_BALANCE;

  const equitySeries: ChartEquityPoint[] = [];
  let cumulativeEquity = baselineBalance;
  const orderedTrades = [...liveTrades].sort(
    (left, right) => new Date(left.exitTime).getTime() - new Date(right.exitTime).getTime(),
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

  const streak = (() => {
    if (liveTrades.length === 0) {
      return "0";
    }

    const lastWasWin = liveTrades[0].netPnl >= 0;
    let count = 0;
    for (const trade of liveTrades) {
      const currentWasWin = trade.netPnl >= 0;
      if (currentWasWin !== lastWasWin) {
        break;
      }
      count += 1;
    }
    return `${count}${lastWasWin ? "W" : "L"}`;
  })();

  const strategyBreakdown = (() => {
    const byStrategy = new Map<string, { wins: number; losses: number; pnl: number }>();

    for (const trade of liveTrades) {
      const entry = byStrategy.get(trade.strategyName) ?? { wins: 0, losses: 0, pnl: 0 };
      if (trade.netPnl >= 0) {
        entry.wins += 1;
      } else {
        entry.losses += 1;
      }
      entry.pnl += trade.netPnl;
      byStrategy.set(trade.strategyName, entry);
    }

    return [...byStrategy.entries()].sort((left, right) => right[1].pnl - left[1].pnl);
  })();

  const topProfitable = [...displayStrategies]
    .filter((s) => s.profit > 0)
    .sort((a, b) => b.profit - a.profit)
    .slice(0, 5);

  const topLosing = [...displayStrategies]
    .filter((s) => s.profit < 0)
    .sort((a, b) => a.profit - b.profit)
    .slice(0, 5);

  const strategyRows = [...displayStrategies]
    .map((strategy) => {
      const totalTrades = strategy.totalTrades || (strategy.wins + strategy.losses);
      const winRate = totalTrades > 0 ? (strategy.wins / totalTrades) * 100 : 0;
      return {
        ...strategy,
        totalTrades,
        winRate,
      };
    })
    .sort((left, right) => {
      if (right.profit !== left.profit) {
        return right.profit - left.profit;
      }
      if (right.totalTrades !== left.totalTrades) {
        return right.totalTrades - left.totalTrades;
      }
      return left.name.localeCompare(right.name);
    });

  const profitableStrategyCount = strategyRows.filter((strategy) => strategy.profit > 0).length;
  const losingStrategyCount = strategyRows.filter((strategy) => strategy.profit < 0).length;
  const totalStrategyTrades = strategyRows.reduce((sum, strategy) => sum + strategy.totalTrades, 0);

  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));
  const bestStrategy = topProfitable[0] ?? null;
  const weakestStrategy = topLosing[0] ?? null;
  const longOpenCount = livePositions.filter((position) => position.side === "BUY").length;
  const shortOpenCount = livePositions.filter((position) => position.side === "SELL").length;
  const longShortSummary = livePositions.length === 0
    ? "No open exposure"
    : `${longOpenCount} long / ${shortOpenCount} short`;
  const connectionLabel = market.connectionState === "live"
    ? "Feed Live"
    : market.connectionState === "reconnecting"
      ? "Feed Reconnecting"
      : market.connectionState === "error"
        ? "Feed Error"
        : "Feed Pending";
  const connectionTone: BadgeTone = market.connectionState === "live"
    ? "positive"
    : market.connectionState === "error"
      ? "negative"
      : "warning";
  const soundTone: BadgeTone = isSoundOn ? "info" : "neutral";
  const signalAccent = latestSignal?.side === "BUY"
    ? "text-emerald-300"
    : latestSignal?.side === "SELL"
      ? "text-rose-300"
      : "text-zinc-200";
  const sessionHigh = market.high24h > 0 ? market.high24h : Math.max(...deferredCandles.slice(-60).map((candle) => candle.high), price || 0);
  const sessionLow = market.low24h > 0 ? market.low24h : Math.min(...deferredCandles.slice(-60).map((candle) => candle.low), price || 0);
  const activeStrategyCount = displayStrategies.filter((strategy) => strategy.status === "RUNNING").length;
  const winRateValue = liveStats?.aggregate.winRate ?? 0;
  const tradeBiasLabel = longOpenCount === shortOpenCount
    ? "Balanced"
    : longOpenCount > shortOpenCount
      ? "Long Bias"
      : "Short Bias";
  const totalReturnPct = (sessionPnl / INITIAL_BALANCE) * 100;
  const historyItems = liveTrades.map((trade) => ({
    id: trade.id,
    strategy: trade.strategyName,
    side: trade.side === "BUY" ? "LONG" : "SHORT",
    size: trade.size,
    entry: trade.entryPrice,
    exit: trade.exitPrice,
    pnl: trade.netPnl,
    reason: mapTradeReason(trade.reason),
    duration: formatDuration(trade.duration),
    time: safeFormatDate(trade.exitTime),
  }));

  const handleReset = () => {
    setResetRefreshKey((current) => current + 1);
  };

  const handleAdminEvent = (message: string, tone: "admin" | "info") => {
    pushFeed(message, tone);
    setResetRefreshKey((current) => current + 1);
  };

  const handleClearLedger = async () => {
    if (!confirm("Clear completed trade history and strategy stats? Open positions and balance will be kept.")) {
      return;
    }

    setIsClearingLedger(true);
    try {
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const response = await fetch(`${apiUrl}/api/admin/clear-history`, { method: "POST" });
      if (!response.ok) {
        throw new Error("action failed");
      }
      handleAdminEvent("Trade history cleared.", "admin");
    } catch {
      pushFeed("Admin action failed. Check engine connectivity.", "admin");
    } finally {
      setIsClearingLedger(false);
    }
  };

  // ── Dynamic Color Intelligence ──────────────────────────────────
  const dailyPnlValue = sessionPnl;
  useEffect(() => {
    const pct = (dailyPnlValue / INITIAL_BALANCE) * 100;
    const root = document.documentElement;
    if (pct >= 5) {
      root.style.setProperty("--dynamic-glow", "rgba(0,255,136,0.22)");
    } else if (pct >= 2) {
      root.style.setProperty("--dynamic-glow", "rgba(0,255,136,0.12)");
    } else if (pct >= 0.5) {
      root.style.setProperty("--dynamic-glow", "rgba(0,255,136,0.06)");
    } else if (pct <= -5) {
      root.style.setProperty("--dynamic-glow", "rgba(255,59,48,0.18)");
    } else if (pct <= -2) {
      root.style.setProperty("--dynamic-glow", "rgba(255,59,48,0.10)");
    } else if (pct <= -0.5) {
      root.style.setProperty("--dynamic-glow", "rgba(255,59,48,0.05)");
    } else {
      root.style.setProperty("--dynamic-glow", "transparent");
    }
  }, [dailyPnlValue]);

  // ── Combat Mode body class ──────────────────────────────────────
  useEffect(() => {
    if (combatMode) {
      document.body.classList.add("combat-mode");
    } else {
      document.body.classList.remove("combat-mode");
    }
    return () => document.body.classList.remove("combat-mode");
  }, [combatMode]);

  // ── Keyboard shortcuts ─────────────────────────────────────────
  // Space = Combat mode | M = Mute | 1/2 = Modules
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      switch (e.key) {
        case " ":
          e.preventDefault();
          setCombatMode((prev) => !prev);
          break;
        case "m":
        case "M":
          setIsSoundOn((prev) => !prev);
          break;
        case "1":
          setActiveModule("dashboard");
          break;
        case "2":
          setActiveModule("engine");
          break;
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  // ── Profit milestone animations ────────────────────────────────
  useEffect(() => {
    const milestones: [number, string][] = [
      [101000,  "🟢 $1K PROFIT UNLOCKED!"],
      [105000,  "⚡ $5K MILESTONE REACHED!"],
      [110000,  "🔥 $10K — RAIG IS PRINTING!"],
      [125000,  "💰 $25K — INSTITUTIONAL LEVEL!"],
      [150000,  "🏆 $50K — ELITE TRADER STATUS!"],
      [200000,  "👑 $100K — RAIG LEGEND MODE!"],
    ];
    let latestMilestoneLabel: string | null = null;

    for (const [threshold, label] of milestones) {
      if (balance >= threshold && !milestoneRef.current.has(threshold)) {
        milestoneRef.current.add(threshold);
        latestMilestoneLabel = label;
      }
    }

    if (!latestMilestoneLabel) {
      return;
    }

    const milestoneLabel = latestMilestoneLabel;
    const showTimer = setTimeout(() => {
      setMilestoneToast(milestoneLabel);
      if (milestoneTimeoutRef.current !== null) {
        clearTimeout(milestoneTimeoutRef.current);
      }
      milestoneTimeoutRef.current = setTimeout(() => {
        setMilestoneToast(null);
        milestoneTimeoutRef.current = null;
      }, 4000);
    }, 0);

    return () => clearTimeout(showTimer);
  }, [balance]);

  // ── Risk meter computation ─────────────────────────────────────
  const riskPct = Math.min(100, Math.max(0, (livePositions.length / 5) * 100));
  const riskLevel = riskPct >= 80 ? "danger" : riskPct >= 50 ? "warning" : "safe";
  const riskLabel = riskLevel === "danger" ? "HIGH RISK" : riskLevel === "warning" ? "MODERATE" : "SAFE";
  const optionsModuleActive = activeModule === "options" || activeModule === "chain";
  const optionEquity = optionStats?.equity ?? INITIAL_OPTIONS_BALANCE;
  const optionSessionPnl = optionEquity - INITIAL_OPTIONS_BALANCE;
  const optionOpenPositions = Math.max(optionStats?.openPositions ?? 0, optionPositions.length);
  const optionsOnline = optionStats !== null || optionPositions.length > 0;

  return (
    <main className="gmail-shell space-y-5">
      {milestoneToast && (
        <div className="milestone-toast">{milestoneToast}</div>
      )}

      {optionsModuleActive ? (
        <OptionsAccountHeader
          online={optionsOnline}
          equity={optionEquity}
          dailyPnL={optionSessionPnl}
          openPositions={optionOpenPositions}
        />
      ) : (
        <DashboardHeader
          online={engineOnline}
          balance={balance}
          dailyPnL={sessionPnl}
          openPositions={livePositions.length}
          onResetSuccess={handleReset}
          onAdminEvent={handleAdminEvent}
          combatMode={combatMode}
          onToggleCombat={() => setCombatMode((prev) => !prev)}
        />
      )}

      <div className="glass-panel px-5 py-3 flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          {[
            { key: "dashboard", label: "Dashboard" },
            { key: "engine", label: "Trade Engine" },
            { key: "history", label: "Trade History" },
            { key: "options", label: "BTC Option Scalper" },
            { key: "chain", label: "BTC Option Chain" },
          ].map((module) => (
            <button
              key={module.key}
              onClick={() => setActiveModule(module.key as "dashboard" | "engine" | "history" | "options" | "chain")}
              className={`groww-tab${activeModule === module.key ? " active" : ""}`}
            >
              {module.label}
            </button>
          ))}
        </div>
        <div
          className="max-w-[760px] text-xs leading-5 md:text-sm lg:ml-auto lg:text-right"
          style={{ color: "var(--text-secondary)" }}
        >
          {activeModule === "dashboard"
            ? "FUTURES ACCOUNT — BTC price, live futures positions, equity, PnL, and key stats. Options data is fully separate."
            : activeModule === "engine"
            ? "FUTURES ACCOUNT — Advanced charts, AI panels, controls, strategy analytics, and logs. No options data here."
            : activeModule === "history"
            ? "FUTURES ACCOUNT — Completed futures trade ledger and strategy breakdown. Options trades are logged separately."
            : activeModule === "options"
            ? "OPTIONS ACCOUNT — 50 autonomous BTC option scalping strategies. Completely separate $1,000,000 paper account. Zero overlap with futures."
            : "OPTIONS VIEW — Live BTC option chain with full Greeks and IV smile. Delta Exchange layout. Read-only, no trading account."}
        </div>
      </div>

      {activeModule === "dashboard" && (
        <div className="space-y-5">
          <div className="grid grid-cols-1 2xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)] items-start gap-5">
            <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
              <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-sky-500/10 blur-3xl pointer-events-none" />
              <div className="flex flex-col gap-5">
                <div className="px-1">
                  <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                    BTC Price
                  </div>
                  <div className="mt-4 flex flex-wrap items-end gap-4">
                    <div className={`text-[clamp(2.55rem,5vw,3.35rem)] font-semibold leading-none tracking-tight ${market.change24h >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                      {formatUSD(price)}
                    </div>
                    <div className={`pb-1 text-xl font-semibold leading-none ${market.change24h >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                      {market.change24h >= 0 ? "+" : ""}
                      {market.change24h.toFixed(2)}%
                    </div>
                  </div>
                </div>

                <div className="flex flex-wrap gap-2 px-1">
                  <BadgePill label={engineOnline ? "Engine Online" : "Engine Offline"} tone={engineOnline ? "positive" : "negative"} />
                  <BadgePill label={connectionLabel} tone={connectionTone} />
                  <BadgePill label={market.exchange === "binance" ? "Binance Feed" : "Bybit Feed"} tone="info" />
                </div>
              </div>

              <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                <CompactMetric
                  label="Runtime"
                  value={sessionRuntime}
                  detail={`${activeStrategyCount} live strategies`}
                  accent="text-white"
                />
                <CompactMetric
                  label="Last Market Event"
                  value={secondsSinceLastMarketEvent === null ? "-" : `${secondsSinceLastMarketEvent}s`}
                  detail={`${market.ticksPerSecond} ticks/sec`}
                  accent={secondsSinceLastMarketEvent !== null && secondsSinceLastMarketEvent <= 3 ? "text-emerald-300" : "text-zinc-100"}
                />
                <CompactMetric
                  label="Open Exposure"
                  value={longShortSummary}
                  detail={`${(liveStats?.exposure ?? 0).toFixed(4)} BTC net`}
                  accent="text-white"
                />
              </div>
            </div>

            <div className="glass-panel px-5 py-6 md:px-6">
              <div className="px-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
                Equity And PnL
              </div>
              <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
                <CompactMetric
                  label="Futures Equity"
                  value={formatUSD(balance)}
                  detail={`Base ${formatUSD(INITIAL_BALANCE)}`}
                  accent="text-white"
                />
                <CompactMetric
                  label="Futures PnL Today"
                  value={formatUSD(sessionPnl, { signed: true })}
                  detail={`${totalReturnPct.toFixed(2)}% vs base`}
                  accent={sessionPnl >= 0 ? "text-emerald-300" : "text-rose-300"}
                />
                <CompactMetric
                  label="Futures Closed PnL"
                  value={formatUSD(closedPnl, { signed: true })}
                  detail={`${tradeCount} completed trades`}
                  accent={closedPnl >= 0 ? "text-emerald-300" : "text-rose-300"}
                />
                <CompactMetric
                  label="Live Futures Positions"
                  value={`${livePositions.length}`}
                  detail={tradeBiasLabel}
                  accent="text-sky-300"
                />
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-4">
            <SummaryCard
              label="Win Rate"
              value={`${winRateValue.toFixed(1)}%`}
              accent={winRateValue >= 50 ? "text-green-400" : "text-red-400"}
            />
            <SummaryCard
              label="Profit Factor"
              value={(liveStats?.aggregate.profitFactor ?? 0).toFixed(2)}
              accent={(liveStats?.aggregate.profitFactor ?? 0) >= 1 ? "text-green-400" : "text-red-400"}
            />
            <SummaryCard label="Trades" value={`${tradeCount}`} accent="text-white" />
            <SummaryCard label="Unrealized" value={formatUSD(unrealized, { signed: true })} accent={unrealized >= 0 ? "text-green-400" : "text-red-400"} />
            <SummaryCard label="Streak" value={streak} accent="text-amber-300" />
          </div>

          <div className="glass-panel px-5 py-6 md:px-6">
            <h2 className="mb-5 flex flex-wrap items-center gap-3" style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
              <span className="pill-green">LIVE</span>
              RUNNING POSITIONS
              <span style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }} className="font-mono">
                ({livePositions.length} active)
              </span>
            </h2>
            <RunningTrades currentPrice={price} trades={runningTrades} />
          </div>

        </div>
      )}

      {activeModule === "engine" && (
        <>

      {/* AI Command Center */}
      <CommandCenter />

      {/* ── Risk Visualization + Keyboard Hints ── */}
      <div className="glass-panel px-5 py-3 flex flex-col md:flex-row items-center gap-4 justify-between">
        {/* Risk Meter */}
        <div className="flex items-center gap-4 flex-1">
          <div style={{ fontFamily: "var(--font-display)", fontSize: 8, fontWeight: 800, letterSpacing: "0.18em", color: "var(--text-muted)" }}>
            RISK METER
          </div>
          <div style={{ flex: 1, maxWidth: 200 }}>
            <div className="risk-meter-track">
              <div
                className={`risk-meter-fill ${riskLevel}`}
                style={{ width: `${riskPct}%` }}
              />
            </div>
          </div>
          <div style={{ fontSize: 9, fontWeight: 800, fontFamily: "var(--font-display)", letterSpacing: "0.12em" }} className={`risk-label-${riskLevel}`}>
            {riskLabel} · {livePositions.length} pos
          </div>
          {/* Drawdown indicator */}
          {(() => {
            const drawdown = ((INITIAL_BALANCE - balance) / INITIAL_BALANCE) * 100;
            if (drawdown <= 0) return null;
            return (
              <div style={{ fontSize: 9, color: "var(--red)", fontFamily: "var(--font-display)", letterSpacing: "0.1em", fontWeight: 700 }}>
                DD: {drawdown.toFixed(1)}%
              </div>
            );
          })()}
        </div>

        {/* AI Waveform indicator */}
        <div className="flex items-center gap-3">
          <div style={{ fontSize: 8, fontFamily: "var(--font-display)", color: "var(--text-muted)", letterSpacing: "0.15em" }}>RAIG AI</div>
          <div className="ai-waveform">
            {[1,2,3,4,5].map((i) => (
              <div key={i} className="ai-waveform-bar" style={{ animationDelay: `${(i-1)*0.15}s` }} />
            ))}
          </div>
          <div style={{ fontSize: 8, fontFamily: "var(--font-display)", color: "var(--green)", letterSpacing: "0.12em", fontWeight: 700 }}>ACTIVE</div>
        </div>

        {/* Keyboard hints */}
        <div className="flex items-center gap-2 shrink-0">
          <span style={{ fontSize: 8, color: "var(--text-muted)", letterSpacing: "0.1em" }}>SHORTCUTS:</span>
          {[["SPACE","COMBAT"],["M","MUTE"],["1-5","TABS"]].map(([key, label]) => (
            <div key={key} className="flex items-center gap-1">
              <span className="kbd">{key}</span>
              <span style={{ fontSize: 7, color: "var(--text-muted)" }}>{label}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1.55fr,1fr,1fr] gap-5">
        <div className="glass-panel relative overflow-hidden p-6">
          <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-sky-500/10 blur-3xl pointer-events-none" />
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                BTC Trader Cockpit
              </div>
              <div className="mt-3 flex flex-wrap items-end gap-4">
                <div className={`text-4xl font-semibold tracking-tight ${market.change24h >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                  {formatUSD(price)}
                </div>
                <div className={`pb-1 text-lg font-semibold ${market.change24h >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                  {market.change24h >= 0 ? "+" : ""}{market.change24h.toFixed(2)}%
                </div>
              </div>
            </div>

            <div className="flex flex-wrap justify-end gap-2">
              <BadgePill label={engineOnline ? "Engine Online" : "Engine Offline"} tone={engineOnline ? "positive" : "negative"} />
              <BadgePill label={connectionLabel} tone={connectionTone} />
              <BadgePill label={market.exchange === "binance" ? "Binance Feed" : "Bybit Feed"} tone="info" />
              <BadgePill label={isSoundOn ? "Sound On" : "Muted"} tone={soundTone} />
              <BadgePill label={tradeBiasLabel} tone="warning" />
            </div>
          </div>

          <div className="mt-5 grid grid-cols-2 md:grid-cols-4 gap-3">
            <CompactMetric
              label="Runtime"
              value={sessionRuntime}
              detail={`${activeStrategyCount} live strategies`}
              accent="text-white"
            />
            <CompactMetric
              label="Last Market Event"
              value={secondsSinceLastMarketEvent === null ? "-" : `${secondsSinceLastMarketEvent}s`}
              detail={`${market.ticksPerSecond} ticks/sec`}
              accent={secondsSinceLastMarketEvent !== null && secondsSinceLastMarketEvent <= 3 ? "text-emerald-300" : "text-zinc-100"}
            />
            <CompactMetric
              label="Session High / Low"
              value={`${sessionHigh.toFixed(0)} / ${sessionLow.toFixed(0)}`}
              detail={`Range ${(sessionHigh - sessionLow).toFixed(0)}`}
              accent="text-sky-300"
            />
            <CompactMetric
              label="Open Exposure"
              value={longShortSummary}
              detail={`${(liveStats?.exposure ?? 0).toFixed(4)} BTC net`}
              accent="text-white"
            />
          </div>
        </div>

        <div className="glass-panel p-5">
          <div className="text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
            Trade History
          </div>
          <div className="mt-4 grid grid-cols-2 gap-3">
            <CompactMetric
              label="Balance"
              value={formatUSD(balance)}
              detail={`Base ${formatUSD(INITIAL_BALANCE)}`}
              accent="text-white"
            />
            <CompactMetric
              label="Closed PnL"
              value={formatUSD(closedPnl, { signed: true })}
              detail={`Return ${(((balance - INITIAL_BALANCE) / INITIAL_BALANCE) * 100).toFixed(2)}%`}
              accent={closedPnl >= 0 ? "text-emerald-300" : "text-rose-300"}
            />
            <CompactMetric
              label="Unrealized"
              value={formatUSD(unrealized, { signed: true })}
              detail={`${livePositions.length} live positions`}
              accent={unrealized >= 0 ? "text-emerald-300" : "text-rose-300"}
            />
            <CompactMetric
              label="Win Rate"
              value={`${winRateValue.toFixed(1)}%`}
              detail={`PF ${(liveStats?.aggregate.profitFactor ?? 0).toFixed(2)} | ${streak}`}
              accent={winRateValue >= 50 ? "text-emerald-300" : "text-amber-300"}
            />
          </div>
        </div>

        <div className="glass-panel p-5">
          <div className="text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
            Market Bias
          </div>
          <div className="mt-4 rounded-2xl border p-4" style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-500">
              Live Read
            </div>
            <div className={`mt-2 text-2xl font-semibold ${marketSentiment.colorClass}`}>
              {marketSentiment.label}
            </div>
            <div className={`mt-2 text-sm font-semibold ${signalAccent}`}>
              {latestSignal ? `${latestSignal.side} ${latestSignal.tag}` : "Waiting for signal alignment"}
            </div>
            <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
              {latestSignal ? `${latestSignal.confidence}% confidence | B:${latestSignal.scoreBuy} S:${latestSignal.scoreSell}` : "No high-conviction signal yet"}
            </div>
          </div>

          <div className="mt-4 space-y-3">
            <div className="rounded-xl border px-4 py-3" style={{ borderColor: "rgba(24, 128, 56, 0.14)", background: "var(--green-dim)" }}>
              <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-500">
                Best Strategy
              </div>
              <div className="mt-2 text-sm font-semibold text-emerald-300">
                {bestStrategy ? bestStrategy.name : "No profitable strategy yet"}
              </div>
              <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
                {bestStrategy ? `${formatUSD(bestStrategy.profit, { signed: true })} | ${bestStrategy.wins}W ${bestStrategy.losses}L` : "Waiting for trade history"}
              </div>
            </div>

            <div className="rounded-xl border px-4 py-3" style={{ borderColor: "rgba(217, 48, 37, 0.14)", background: "var(--red-dim)" }}>
              <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-500">
                Weakest Strategy
              </div>
              <div className="mt-2 text-sm font-semibold text-rose-300">
                {weakestStrategy ? weakestStrategy.name : "No losing strategy yet"}
              </div>
              <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
                {weakestStrategy ? `${formatUSD(weakestStrategy.profit, { signed: true })} | ${weakestStrategy.wins}W ${weakestStrategy.losses}L` : "No drawdown leader"}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="glass-panel px-5 py-3 flex flex-col xl:flex-row xl:items-center justify-between gap-3">
        {/* Controls */}
        <div className="flex flex-wrap items-center gap-2">
          {(["binance", "bybit"] as const).map((exchange) => (
            <button
              key={exchange}
              onClick={() => market.setExchange(exchange)}
              style={market.exchange === exchange
                ? { background: "var(--green-dim)", color: "var(--green)", border: "1px solid rgba(0,208,156,0.25)", borderRadius: 8, padding: "5px 14px", fontSize: 11, fontWeight: 700, letterSpacing: "0.08em", cursor: "pointer" }
                : { background: "var(--surface-2)", color: "var(--text-secondary)", border: "1px solid var(--border)", borderRadius: 8, padding: "5px 14px", fontSize: 11, fontWeight: 600, letterSpacing: "0.08em", cursor: "pointer" }
              }
            >
              {exchange === "binance" ? "Binance" : "Bybit"}
            </button>
          ))}
          <button
            onClick={() => setIsSoundOn((current) => !current)}
            style={isSoundOn
              ? { background: "rgba(83,103,255,0.12)", color: "#818CF8", border: "1px solid rgba(83,103,255,0.25)", borderRadius: 8, padding: "5px 14px", fontSize: 11, fontWeight: 700, cursor: "pointer" }
              : { background: "var(--surface-2)", color: "var(--text-muted)", border: "1px solid var(--border)", borderRadius: 8, padding: "5px 14px", fontSize: 11, fontWeight: 600, cursor: "pointer" }
            }
          >
            {isSoundOn ? "🔊 Sound" : "🔇 Muted"}
          </button>
          <div style={{ color: "var(--text-muted)", fontSize: 11, padding: "5px 12px", background: "var(--surface-2)", border: "1px solid var(--border)", borderRadius: 8 }}>
            {market.connectionState === "live" ? "● Live" : `⚠ ${market.connectionState}`}
            {market.connectionError ? ` · ${market.connectionError}` : ""}
          </div>
        </div>

        {/* Groww-style tab bar */}
        <div className="flex items-center gap-1" style={{ background: "var(--surface-2)", borderRadius: 999, padding: 4, border: "1px solid var(--border)" }}>
          {[
            { key: "trade", label: "Trade" },
            { key: "stats", label: "Stats" },
            { key: "strategies", label: `Strategies (${strategyRows.length})` },
            { key: "history", label: `Trade History (${liveTrades.length})` },
            { key: "feed", label: `Feed (${feed.length})` },
          ].map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key as "trade" | "stats" | "strategies" | "history" | "feed")}
              className={`groww-tab${activeTab === tab.key ? " active" : ""}`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        <SummaryCard
          label="Win Rate"
          value={`${(liveStats?.aggregate.winRate ?? 0).toFixed(1)}%`}
          accent={(liveStats?.aggregate.winRate ?? 0) >= 50 ? "text-green-400" : "text-red-400"}
        />
        <SummaryCard
          label="Profit Factor"
          value={(liveStats?.aggregate.profitFactor ?? 0).toFixed(2)}
          accent={(liveStats?.aggregate.profitFactor ?? 0) >= 1 ? "text-green-400" : "text-red-400"}
        />
        <SummaryCard label="Trades" value={`${tradeCount}`} accent="text-white" />
        <SummaryCard label="Unrealized" value={formatUSD(unrealized, { signed: true })} accent={unrealized >= 0 ? "text-green-400" : "text-red-400"} />
        <SummaryCard label="Streak" value={streak} accent="text-amber-300" />
      </div>

      {activeTab === "trade" && (
        <div className="space-y-5">
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4">
            <CompactMetric
              label="Live Signal"
              value={latestSignal ? `${latestSignal.side} ${latestSignal.tag}` : "Standby"}
              detail={latestSignal ? `${latestSignal.confidence}% confidence` : "Waiting for aligned candle structure"}
              accent={signalAccent}
            />
            <CompactMetric
              label="Trade Bias"
              value={tradeBiasLabel}
              detail={longShortSummary}
              accent={longOpenCount >= shortOpenCount ? "text-sky-300" : "text-fuchsia-300"}
            />
            <CompactMetric
              label="Session Range"
              value={formatUSD(sessionHigh - sessionLow)}
              detail={`${sessionLow.toFixed(0)} low | ${sessionHigh.toFixed(0)} high`}
              accent="text-amber-300"
            />
            <CompactMetric
              label="Lead Strategy"
              value={bestStrategy ? bestStrategy.name : "No leader yet"}
              detail={bestStrategy ? `${formatUSD(bestStrategy.profit, { signed: true })}` : "Need more closed trades"}
              accent={bestStrategy ? "text-emerald-300" : "text-zinc-200"}
            />
          </div>

          {/* ── MAIN ZONE: Chart 70% | AI Panel 30% ── */}
          <div className="grid grid-cols-1 xl:grid-cols-[1fr,420px] gap-5">

            {/* Hero Chart */}
            <div className="glass-panel overflow-hidden" style={{ minHeight: 480 }}>
              <div style={{
                display: "flex", alignItems: "center", gap: 10,
                padding: "12px 16px 0",
                fontFamily: "var(--font-display)", fontSize: 9, fontWeight: 800,
                letterSpacing: "0.18em", color: "var(--text-muted)",
              }}>
                <span style={{ color: "var(--gold)" }}>▣</span>
                BTC / USDT · LIVE CHART
                {livePositions.length > 0 && (
                  <span style={{ marginLeft: 8, color: "var(--green)", fontSize: 8 }}>
                    ● {livePositions.length} OPEN
                  </span>
                )}
              </div>
              <MarketChart
                candles={deferredCandles}
                currentPrice={price}
                positions={livePositions.map((position) => ({
                  id: position.id,
                  strategy: position.strategyName,
                  side: position.side === "BUY" ? "LONG" : "SHORT",
                  entry: position.entryPrice,
                  stopLoss: position.stopLoss,
                  takeProfit: position.takeProfit,
                }))}
              />
            </div>

            {/* AI Panel 30% */}
            <div className="space-y-4">
              <FearGreedWidget />
              <AIInsightPanel
                enabled={aiInsights.enabled}
                geminiEnabled={aiInsights.geminiEnabled}
                message={aiInsights.message}
                latest={aiInsights.latest}
                recent={aiInsights.recent}
                auditLogs={aiInsights.auditLogs}
              />
              <SignalInsightCard signal={latestSignal} />
              <ActivityFeed entries={feed.slice(0, 8)} />
            </div>
          </div>

          {/* ── SECONDARY ZONE: Running Positions ── */}
          <div className="glass-panel p-5">
            <h2 className="mb-4 flex items-center gap-3" style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
              <span className="pill-green">LIVE</span>
              RUNNING POSITIONS
              <span style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }} className="font-mono">
                ({livePositions.length} active)
              </span>
            </h2>
            <RunningTrades currentPrice={price} trades={runningTrades} />
          </div>

          {/* ── STRATEGY ZONE: Top 5 Profitable + Top 5 Losing ── */}
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-5">

            {/* Top 5 Profitable */}
            <div className="glass-panel p-5">
              <div style={{ fontFamily: "var(--font-display)", fontSize: 9, fontWeight: 800, letterSpacing: "0.18em", color: "var(--green)", marginBottom: 14 }}>
                🟢 PROFITABLE STRATEGIES
              </div>
              {topProfitable.length === 0 ? (
                <div style={{ fontSize: 11, color: "var(--text-muted)", padding: "12px 0" }}>No profitable strategies yet.</div>
              ) : (
                <div className="space-y-2">
                  {topProfitable.map((s) => {
                    const total = s.wins + s.losses;
                    const wr = total > 0 ? (s.wins / total) * 100 : 0;
                    return (
                      <div key={s.name} style={{ display: "flex", alignItems: "center", gap: 10, padding: "8px 10px", background: "rgba(0,255,136,0.04)", borderRadius: 8, border: "1px solid rgba(0,255,136,0.10)" }}>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div style={{ fontSize: 11, fontWeight: 700, color: "var(--text-secondary)", fontFamily: "var(--font-display)", letterSpacing: "0.06em", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                            {s.name}
                          </div>
                          <div style={{ fontSize: 9, color: "var(--text-muted)", marginTop: 2 }}>
                            {s.wins}W / {s.losses}L · {wr.toFixed(0)}% WR · {s.timeframe}
                          </div>
                        </div>
                        <div style={{ fontSize: 13, fontWeight: 800, color: "var(--green)", fontFamily: "var(--font-display)", whiteSpace: "nowrap" }}>
                          +{formatUSD(s.profit)}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Top 5 Losing */}
            <div className="glass-panel p-5">
              <div style={{ fontFamily: "var(--font-display)", fontSize: 9, fontWeight: 800, letterSpacing: "0.18em", color: "var(--red)", marginBottom: 14 }}>
                🔴 LOSING STRATEGIES
              </div>
              {topLosing.length === 0 ? (
                <div style={{ fontSize: 11, color: "var(--text-muted)", padding: "12px 0" }}>No losing strategies yet.</div>
              ) : (
                <div className="space-y-2">
                  {topLosing.map((s) => {
                    const total = s.wins + s.losses;
                    const wr = total > 0 ? (s.wins / total) * 100 : 0;
                    return (
                      <div key={s.name} style={{ display: "flex", alignItems: "center", gap: 10, padding: "8px 10px", background: "rgba(255,59,48,0.04)", borderRadius: 8, border: "1px solid rgba(255,59,48,0.10)" }}>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div style={{ fontSize: 11, fontWeight: 700, color: "var(--text-secondary)", fontFamily: "var(--font-display)", letterSpacing: "0.06em", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                            {s.name}
                          </div>
                          <div style={{ fontSize: 9, color: "var(--text-muted)", marginTop: 2 }}>
                            {s.wins}W / {s.losses}L · {wr.toFixed(0)}% WR · {s.timeframe}
                          </div>
                        </div>
                        <div style={{ fontSize: 13, fontWeight: 800, color: "var(--red)", fontFamily: "var(--font-display)", whiteSpace: "nowrap" }}>
                          {formatUSD(s.profit)}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

        </div>
      )}

      {activeTab === "stats" && (
        <div className="space-y-6">
          <PerformanceCharts priceSeries={priceSeries} equitySeries={equitySeries} strategyBars={strategyBars} />

          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <SummaryCard label="Best Trade" value={formatUSD(liveStats?.aggregate.bestTrade ?? 0, { signed: true })} accent="text-green-400" />
            <SummaryCard label="Worst Trade" value={formatUSD(liveStats?.aggregate.worstTrade ?? 0, { signed: true })} accent="text-red-400" />
            <SummaryCard label="Total Return" value={`${(((balance - INITIAL_BALANCE) / INITIAL_BALANCE) * 100).toFixed(2)}%`} accent={balance >= INITIAL_BALANCE ? "text-green-400" : "text-red-400"} />
            <SummaryCard label="Total Strategy PnL" value={formatUSD(closedPnl, { signed: true })} accent={closedPnl >= 0 ? "text-green-400" : "text-red-400"} />
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <SummaryCard label="Avg Win" value={formatUSD(liveStats?.aggregate.avgWin ?? 0, { signed: true })} accent="text-green-400" />
            <SummaryCard label="Avg Loss" value={formatUSD(-(liveStats?.aggregate.avgLoss ?? 0), { signed: true })} accent="text-rose-400" />
            <SummaryCard label="Max Drawdown" value={formatUSD(-(liveStats?.aggregate.maxDrawdown ?? 0), { signed: true })} accent={(liveStats?.aggregate.maxDrawdown ?? 0) > 0 ? "text-orange-400" : "text-zinc-400"} />
            <SummaryCard
              label="Avg Duration"
              value={(() => {
                const ms = liveStats?.aggregate.avgDurationMs ?? 0;
                if (ms <= 0) return "\u2013";
                const totalSecs = Math.floor(ms / 1000);
                const mins = Math.floor(totalSecs / 60);
                const secs = totalSecs % 60;
                return mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
              })()}
              accent="text-sky-300"
            />
          </div>

          <div className="glass-panel p-6">
            <h2 className="mb-4 text-sm font-bold uppercase tracking-widest" style={{ color: "var(--text-secondary)" }}>Category Performance</h2>
            <div className="w-full overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="border-b text-[10px] uppercase tracking-widest" style={{ color: "var(--text-secondary)", borderColor: "var(--border-subtle)" }}>
                  <tr>
                    <th className="py-2 px-3">Category</th>
                    <th className="py-2 px-3 text-center">Strategies</th>
                    <th className="py-2 px-3 text-center">W</th>
                    <th className="py-2 px-3 text-center">L</th>
                    <th className="py-2 px-3 text-center">Win%</th>
                    <th className="py-2 px-3 text-right">PnL</th>
                  </tr>
                </thead>
                <tbody>
                  {displayCategories.map((cat) => {
                    const cats = displayStrategies.filter((s) => s.category === cat);
                    const w = cats.reduce((sum, s) => sum + (s.wins ?? 0), 0);
                    const l = cats.reduce((sum, s) => sum + (s.losses ?? 0), 0);
                    const pnl = cats.reduce((sum, s) => sum + (s.profit ?? 0), 0);
                    const wr = w + l > 0 ? (w / (w + l)) * 100 : null;
                    return (
                      <tr key={cat} className="border-b transition-colors" style={{ borderColor: "var(--border-subtle)" }}>
                        <td className="py-2 px-3 font-mono" style={{ color: "var(--text-primary)" }}>
                          <div className="flex items-center gap-2">
                            <span className={`h-1.5 w-1.5 rounded-full flex-shrink-0 ${CAT_COLORS[cat] || "bg-gray-500"}`}></span>
                            {cat}
                          </div>
                        </td>
                        <td className="py-2 px-3 text-center" style={{ color: "var(--text-secondary)" }}>{cats.length}</td>
                        <td className="py-2 px-3 text-center font-mono" style={{ color: "var(--green)" }}>{w}</td>
                        <td className="py-2 px-3 text-center font-mono" style={{ color: "var(--red)" }}>{l}</td>
                        <td className="py-2 px-3 text-center font-mono font-bold">
                          {wr !== null ? (
                            <span className={wr >= 50 ? "text-green-400" : "text-red-400"}>{wr.toFixed(1)}%</span>
                          ) : (
                            <span style={{ color: "var(--text-secondary)" }}>-</span>
                          )}
                        </td>
                        <td className={`py-2 px-3 text-right font-mono font-bold ${pnl >= 0 ? "text-green-400" : "text-red-400"}`}>
                          {pnl >= 0 ? "+" : ""}{formatUSD(pnl)}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          <div className="space-y-6">
            {displayCategories.map((category) => {
              const categoryStrategies = displayStrategies.filter((strategy) => strategy.category === category);
              if (categoryStrategies.length === 0) {
                return null;
              }

              return (
                <div key={category} className="glass-panel p-6">
                  <h2 className="mb-4 flex items-center gap-2 text-lg font-bold">
                    <span className={`h-2 w-2 rounded-full ${CAT_COLORS[category] || "bg-gray-500"} animate-pulse`}></span>
                    {category}
                    <span className="ml-2 text-xs font-mono text-gray-500">{categoryStrategies.length} strategies</span>
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
                        wins={strategy.wins}
                        losses={strategy.losses}
                      />
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {activeTab === "strategies" && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <SummaryCard label="All Strategies" value={`${strategyRows.length}`} accent="text-white" />
            <SummaryCard label="Profitable" value={`${profitableStrategyCount}`} accent="text-green-400" />
            <SummaryCard label="Losing" value={`${losingStrategyCount}`} accent="text-red-400" />
            <SummaryCard label="Executed Trades" value={`${totalStrategyTrades}`} accent="text-sky-300" />
          </div>

          <div className="glass-panel p-6">
            <div className="mb-5 flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
              <div>
                <h2 className="text-sm font-bold uppercase tracking-[0.18em] text-zinc-400">
                  Strategy Performance Ledger
                </h2>
                <div className="mt-2 text-sm text-zinc-500">
                  Full strategy list with profit/loss, win percentage, and executed trades.
                </div>
              </div>
              <div className="text-xs font-mono text-zinc-500">
                Sorted by PnL, then trades
              </div>
            </div>

            <div className="w-full overflow-x-auto">
              <table className="w-full min-w-[980px] text-left text-xs">
                <thead className="border-b border-zinc-800/80 text-[10px] uppercase tracking-[0.18em] text-zinc-500">
                  <tr>
                    <th className="px-3 py-3">#</th>
                    <th className="px-3 py-3">Strategy</th>
                    <th className="px-3 py-3">Category</th>
                    <th className="px-3 py-3">TF</th>
                    <th className="px-3 py-3">Status</th>
                    <th className="px-3 py-3 text-center">Trades</th>
                    <th className="px-3 py-3 text-center">Wins</th>
                    <th className="px-3 py-3 text-center">Losses</th>
                    <th className="px-3 py-3 text-center">Win %</th>
                    <th className="px-3 py-3 text-right">Profit / Loss</th>
                  </tr>
                </thead>
                <tbody>
                  {strategyRows.map((strategy, index) => (
                    <tr
                      key={strategy.name}
                      className="border-b border-zinc-900/80 transition-colors hover:bg-white/5"
                    >
                      <td className="px-3 py-3 font-mono text-zinc-500">
                        {index + 1}
                      </td>
                      <td className="px-3 py-3">
                        <div className="font-mono text-zinc-200">{strategy.name}</div>
                      </td>
                      <td className="px-3 py-3">
                        <div className="inline-flex rounded-full border border-zinc-800 bg-zinc-900/80 px-2 py-1 text-[10px] text-zinc-400">
                          {strategy.category}
                        </div>
                      </td>
                      <td className="px-3 py-3 font-mono text-zinc-400">
                        {strategy.timeframe}
                      </td>
                      <td className="px-3 py-3">
                        <span className={`inline-flex rounded-full px-2 py-1 text-[10px] font-bold tracking-wider ${
                          strategy.status === "RUNNING"
                            ? "bg-green-500/10 text-green-400"
                            : "bg-zinc-700/40 text-zinc-400"
                        }`}>
                          {strategy.status}
                        </span>
                      </td>
                      <td className="px-3 py-3 text-center font-mono text-zinc-200">
                        {strategy.totalTrades}
                      </td>
                      <td className="px-3 py-3 text-center font-mono text-green-400">
                        {strategy.wins}
                      </td>
                      <td className="px-3 py-3 text-center font-mono text-red-400">
                        {strategy.losses}
                      </td>
                      <td className="px-3 py-3 text-center font-mono font-bold">
                        {strategy.totalTrades > 0 ? (
                          <span className={strategy.winRate >= 50 ? "text-green-400" : "text-red-400"}>
                            {strategy.winRate.toFixed(1)}%
                          </span>
                        ) : (
                          <span className="text-zinc-600">-</span>
                        )}
                      </td>
                      <td className={`px-3 py-3 text-right font-mono font-bold ${
                        strategy.profit >= 0 ? "text-green-400" : "text-red-400"
                      }`}>
                        {formatUSD(strategy.profit, { signed: true })}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {activeTab === "history" && (
        <div className="space-y-6">
          {strategyBreakdown.length > 0 && (
            <div className="glass-panel p-6">
              <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em] text-zinc-400">
                Strategy Breakdown
              </div>
              <div className="flex flex-wrap gap-2">
                {strategyBreakdown.map(([strategyName, stats]) => (
                  <div
                    key={strategyName}
                    className={`rounded-xl border px-3 py-2 text-sm ${
                      stats.pnl >= 0
                        ? "border-green-500/20 bg-green-500/10 text-green-200"
                        : "border-red-500/20 bg-red-500/10 text-red-200"
                    }`}
                  >
                    <span className="font-semibold text-white">{strategyName}</span>
                    <span className="ml-2 text-zinc-400">{stats.wins}W/{stats.losses}L</span>
                    <span className="ml-2 font-mono">{formatUSD(stats.pnl, { signed: true })}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="glass-panel p-6">
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <h2 className="flex items-center gap-3 text-xl font-bold">
                <span className="rounded-lg border border-blue-500/20 bg-blue-500/10 px-3 py-1 text-xs font-bold tracking-widest text-blue-400">LOG</span>
                Trade History
                <span className="text-sm font-mono text-gray-500">({liveTrades.length} completed)</span>
              </h2>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={handleClearLedger}
                  disabled={isClearingLedger}
                  className="btn-primary"
                >
                  {isClearingLedger ? "Clearing…" : "Clear Trade History"}
                </button>
                <button
                  type="button"
                  disabled={isClearingLedger}
                  className="btn-danger"
                  onClick={() => {
                    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
                    if (!confirm("Reset futures paper account to $1,000,000? All history and open positions will be cleared.")) return;
                    fetch(`${apiUrl}/api/admin/reset`, { method: "POST" })
                      .then((r) => { if (r.ok) handleAdminEvent("Futures account reset to $1,000,000.", "admin"); })
                      .catch(() => handleAdminEvent("Reset failed. Check engine connectivity.", "admin"));
                  }}
                >
                  Reset Account
                </button>
              </div>
            </div>
            <TradeHistory history={historyItems} />
          </div>
        </div>
      )}

      {activeTab === "feed" && (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
          <ActivityFeed entries={feed} />

          <div className="rounded-2xl border border-zinc-800/80 bg-zinc-950/70">
            <div className="border-b border-zinc-800/80 px-5 py-4">
              <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-zinc-400">
                Engine Log Buffer
              </h3>
            </div>
            <div className="max-h-[420px] overflow-y-auto p-5">
              {engineLogs.length === 0 ? (
                <div className="text-sm text-zinc-500">No engine logs available yet.</div>
              ) : (
                <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-6 text-zinc-300">
                  {engineLogs.join("")}
                </pre>
              )}
            </div>
          </div>
        </div>
      )}
        </>
      )}

      {activeModule === "history" && (
        <div className="space-y-6">
          {strategyBreakdown.length > 0 && (
            <div className="glass-panel p-6">
              <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em] text-zinc-400">
                Strategy Breakdown
              </div>
              <div className="flex flex-wrap gap-2">
                {strategyBreakdown.map(([strategyName, stats]) => (
                  <div
                    key={strategyName}
                    className={`rounded-xl border px-3 py-2 text-sm ${
                      stats.pnl >= 0
                        ? "border-green-500/20 bg-green-500/10 text-green-200"
                        : "border-red-500/20 bg-red-500/10 text-red-200"
                    }`}
                  >
                    <span className="font-semibold text-white">{strategyName}</span>
                    <span className="ml-2 text-zinc-400">{stats.wins}W/{stats.losses}L</span>
                    <span className="ml-2 font-mono">{formatUSD(stats.pnl, { signed: true })}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="glass-panel p-6">
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <h2 className="flex items-center gap-3 text-xl font-bold">
                <span className="rounded-lg border border-blue-500/20 bg-blue-500/10 px-3 py-1 text-xs font-bold tracking-widest text-blue-400">LOG</span>
                Trade History
                <span className="text-sm font-mono text-gray-500">({liveTrades.length} completed)</span>
              </h2>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={handleClearLedger}
                  disabled={isClearingLedger}
                  className="btn-primary"
                >
                  {isClearingLedger ? "Clearing…" : "Clear Trade History"}
                </button>
                <button
                  type="button"
                  disabled={isClearingLedger}
                  className="btn-danger"
                  onClick={() => {
                    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
                    if (!confirm("Reset futures paper account to $1,000,000? All history and open positions will be cleared.")) return;
                    fetch(`${apiUrl}/api/admin/reset`, { method: "POST" })
                      .then((r) => { if (r.ok) handleAdminEvent("Futures account reset to $1,000,000.", "admin"); })
                      .catch(() => handleAdminEvent("Reset failed. Check engine connectivity.", "admin"));
                  }}
                >
                  Reset Account
                </button>
              </div>
            </div>
            <TradeHistory history={historyItems} />
          </div>
        </div>
      )}

      {activeModule === "options" && <OptionsScalper />}

      {activeModule === "chain" && <BTCOptionChain />}
    </main>
  );
}
