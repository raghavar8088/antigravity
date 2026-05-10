"use client";

import { useMemo, useState } from "react";
import {
  useBTCFuturesScalperEngine,
  type BTCFuturesPosition,
  type BTCFuturesTrade,
  type BTCFuturesEngineStats,
  type BTCFuturesStrategyStatus,
} from "@/hooks/useBTCFuturesScalperEngine";

// ========== FORMATTERS ==========
function fmtUSD(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtPct(value: number, signed = false, decimals = 2) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

function fmtContracts(n: number) {
  return n.toLocaleString("en-US", { maximumFractionDigits: 0 });
}

function formatShortTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: false });
}

function formatShortDate(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatDate(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

// ========== DESIGN PRIMITIVES ==========
type BadgeTone = "neutral" | "positive" | "negative" | "info" | "warning";

function BadgePill({ label, tone = "neutral" }: { label: string; tone?: BadgeTone }) {
  const map: Record<BadgeTone, string> = {
    neutral:  "border-zinc-200 bg-white text-zinc-600",
    positive: "border-emerald-200 bg-emerald-50 text-emerald-700",
    negative: "border-rose-200 bg-rose-50 text-rose-700",
    info:     "border-blue-200 bg-blue-50 text-blue-700",
    warning:  "border-amber-200 bg-amber-50 text-amber-700",
  };
  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[10px] font-medium uppercase tracking-[0.1em] ${map[tone]}`}>
      {label}
    </span>
  );
}

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  return (
    <span className={`rounded px-1.5 py-0.5 text-[10px] font-bold tracking-wider ${
      side === "LONG"
        ? "bg-emerald-100 text-emerald-700"
        : "bg-rose-100 text-rose-700"
    }`}>{side}</span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    READY:       "bg-emerald-100 text-emerald-700",
    IN_POSITION: "bg-blue-100 text-blue-700",
    COOLING:     "bg-amber-100 text-amber-700",
    AVAILABLE:   "bg-zinc-100 text-zinc-600",
  };
  return (
    <span className={`rounded px-1.5 py-0.5 text-[10px] font-bold tracking-wider ${map[status] ?? "bg-zinc-100 text-zinc-500"}`}>
      {status.replace("_", " ")}
    </span>
  );
}

function CompactMetric({ label, value, detail, accent = "" }: {
  label: string; value: string; detail?: string; accent?: string;
}) {
  return (
    <div className="metric-card flex min-h-[104px] flex-col justify-between gap-3">
      <div>
        <div className="metric-label">{label}</div>
        <div className={`metric-value ${accent}`}>{value}</div>
      </div>
      <div className="text-xs" style={{ color: "var(--text-secondary)", minHeight: 18 }}>{detail ?? ""}</div>
    </div>
  );
}

function SummaryCard({ label, value, accent }: { label: string; value: string; accent: string }) {
  return (
    <div className="summary-card flex min-h-[112px] flex-col justify-between gap-3">
      <div className="summary-label">{label}</div>
      <div className={`summary-value ${accent}`}>{value}</div>
    </div>
  );
}

// ========== PREMIUM BAR (Green/Red bars like options selling) ==========
function PremiumBar({ progress, isPositive }: { progress: number; isPositive: boolean }) {
  const width = Math.min(100, Math.max(5, Math.abs(progress) * 2));
  return (
    <div className="h-1.5 w-20 rounded-full bg-zinc-200 overflow-hidden">
      <div
        className={`h-full rounded-full ${isPositive ? "bg-emerald-500" : "bg-rose-500"}`}
        style={{ width: `${width}%` }}
      />
    </div>
  );
}

type FuturesWatchItem = {
  symbol: string;
  name: string;
  leverage: string;
  lastPrice: string;
  change24h: string;
  volume24h: string;
};

const FUTURES_WATCHLIST: FuturesWatchItem[] = [
  { symbol: "BTCUSD", name: "Bitcoin Perpetual", leverage: "200x", lastPrice: "$80845", change24h: "0.68%", volume24h: "$374.18M" },
  { symbol: "ETHUSD", name: "Ethereum Perpetual", leverage: "200x", lastPrice: "$2329.5", change24h: "0.94%", volume24h: "$267.65M" },
  { symbol: "SOLUSD", name: "Solana Perpetual", leverage: "100x", lastPrice: "$93.926", change24h: "0.99%", volume24h: "$92.70M" },
  { symbol: "BNBUSD", name: "BNB Perpetual", leverage: "125x", lastPrice: "$592.40", change24h: "1.12%", volume24h: "$128.44M" },
  { symbol: "ADAUSD", name: "Cardano Perpetual", leverage: "100x", lastPrice: "$0.4621", change24h: "2.06%", volume24h: "$64.81M" },
  { symbol: "DOGEUSD", name: "Dogecoin Perpetual", leverage: "100x", lastPrice: "$0.11102", change24h: "1.34%", volume24h: "$3.17M" },
  { symbol: "UNIUSD", name: "Uniswap Perpetual", leverage: "100x", lastPrice: "$4.04", change24h: "8.98%", volume24h: "$9.28M" },
  { symbol: "RAVEUSD", name: "RaveDAO Perpetual", leverage: "20x", lastPrice: "$0.7511", change24h: "-15.03%", volume24h: "$3.15M" },
  { symbol: "PAXGUSD", name: "PAX Gold Perpetual", leverage: "5x", lastPrice: "$4717.8", change24h: "0.19%", volume24h: "$6.64M" },
  { symbol: "XAUTUSD", name: "Tether Gold Perpetual", leverage: "100x", lastPrice: "$4713.91", change24h: "0.18%", volume24h: "$3.72M" },
  { symbol: "BIOUSD", name: "Bio Protocol Perpetual", leverage: "20x", lastPrice: "$0.05304", change24h: "-5.84%", volume24h: "$2.45M" },
  { symbol: "HUSD", name: "Humanity Protocol Perpetual", leverage: "20x", lastPrice: "$0.20211", change24h: "-1.80%", volume24h: "$1.07M" },
  { symbol: "TSTUSD", name: "Test Token Perpetual", leverage: "20x", lastPrice: "$0.02177", change24h: "-10.26%", volume24h: "$1.22M" },
  { symbol: "RIVERUSD", name: "River Perpetual", leverage: "20x", lastPrice: "$6.474", change24h: "-1.05%", volume24h: "$1.62M" },
  { symbol: "JASMYUSD", name: "JasmyCoin Perpetual", leverage: "20x", lastPrice: "$0.007239", change24h: "2.03%", volume24h: "$1.81M" },
  { symbol: "AIOTUSD", name: "OKZOO Perpetual", leverage: "20x", lastPrice: "$0.08904", change24h: "3.29%", volume24h: "$995.53K" },
  { symbol: "MUSD", name: "MemeCore Perpetual", leverage: "20x", lastPrice: "$3.3034", change24h: "-4.05%", volume24h: "$1.55M" },
  { symbol: "LABUSD", name: "LAB Perpetual", leverage: "20x", lastPrice: "$4.789", change24h: "16.01%", volume24h: "$24.26M" },
  { symbol: "SIRENUSD", name: "Siren Perpetual", leverage: "20x", lastPrice: "$1.17798", change24h: "-7.76%", volume24h: "$13.22M" },
  { symbol: "LAYERUSD", name: "Solayer Perpetual", leverage: "20x", lastPrice: "$0.1297", change24h: "37.39%", volume24h: "$12.67M" },
  { symbol: "SAHARAUSD", name: "Sahara AI Perpetual", leverage: "20x", lastPrice: "$0.03567", change24h: "-8.35%", volume24h: "$9.10M" },
  { symbol: "ZECUSD", name: "Zcash Perpetual", leverage: "20x", lastPrice: "$599.94", change24h: "2.16%", volume24h: "$8.41M" },
  { symbol: "AINUSD", name: "Ainfinity Ground Perpetual", leverage: "20x", lastPrice: "$0.0956", change24h: "-4.12%", volume24h: "$8.04M" },
  { symbol: "VVVUSD", name: "Venice Token Perpetual", leverage: "20x", lastPrice: "$15.149", change24h: "-1.41%", volume24h: "$7.31M" },
  { symbol: "SKYAIUSD", name: "SkyAI Perpetual", leverage: "20x", lastPrice: "$0.54383", change24h: "-5.67%", volume24h: "$7.30M" },
  { symbol: "XRPUSD", name: "Ripple Perpetual", leverage: "100x", lastPrice: "$1.4345", change24h: "1.29%", volume24h: "$6.10M" },
];

// ========== MAIN COMPONENT ==========
export function BTCFuturesScalper() {
  const {
    positions,
    trades,
    balance,
    equity,
    stats,
    quote,
    isReady,
    pauseEntries,
    disabledStrategies,
    togglePause,
    resetPaperAccount,
    clearTradeHistory,
    setDisabledStrategies,
    strategyStatuses,
  } = useBTCFuturesScalperEngine();

  const [showAllStrategies, setShowAllStrategies] = useState(false);
  const [showAllTrades, setShowAllTrades] = useState(false);
  const [watchSearch, setWatchSearch] = useState("");

  const sessionPnL = equity - 1000; // vs $1,000 base
  const pnlPositive = sessionPnL >= 0;
  const totalReturn = ((equity - 1000) / 1000) * 100;

  const longCount = positions.filter(p => p.side === "LONG").length;
  const shortCount = positions.filter(p => p.side === "SHORT").length;
  const totalUnrealized = positions.reduce((s, p) => s + p.unrealizedPnl, 0);

  const sortedStrategies = [...strategyStatuses].sort((a, b) => b.score - a.score);
  const visibleStrategies = showAllStrategies ? sortedStrategies : sortedStrategies.slice(0, 12);

  const sortedTrades = [...trades].reverse();
  const visibleTrades = showAllTrades ? sortedTrades : sortedTrades.slice(0, 10);
  const visibleWatchlist = useMemo(() => {
    const q = watchSearch.trim().toLowerCase();
    if (!q) return FUTURES_WATCHLIST;
    return FUTURES_WATCHLIST.filter(
      (item) =>
        item.symbol.toLowerCase().includes(q) ||
        item.name.toLowerCase().includes(q),
    );
  }, [watchSearch]);

  // Daily ledger
  const tradesByDay = trades.reduce((acc, t) => {
    const day = t.closedAt.split("T")[0];
    if (!acc[day]) acc[day] = { trades: 0, wins: 0, losses: 0, pnl: 0 };
    acc[day].trades++;
    if (t.netPnl > 0) acc[day].wins++;
    else acc[day].losses++;
    acc[day].pnl += t.netPnl;
    return acc;
  }, {} as Record<string, { trades: number; wins: number; losses: number; pnl: number }>);

  return (
    <div className="space-y-5 p-4 md:p-6">
      {/* Header */}
      <div className="glass-panel px-6 py-6">
        <div className="flex items-center gap-2 mb-1">
          <span className="inline-flex items-center gap-1.5 rounded border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-emerald-700">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
            {pauseEntries ? "PAUSED" : "LIVE"}
          </span>
          <span className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
            BTCUSD PERPETUAL FUTURES · 25x LEVERAGE · 180 STRATEGIES
          </span>
        </div>
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-zinc-900">Future Trading</h1>
          <div className="flex gap-2">
            <button
              onClick={togglePause}
              className="rounded border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-600 hover:bg-zinc-50"
            >
              {pauseEntries ? "Resume" : "Pause"}
            </button>
            <button
              onClick={resetPaperAccount}
              className="rounded border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs font-medium text-rose-600 hover:bg-rose-100"
            >
              Reset Account
            </button>
            <button
              onClick={clearTradeHistory}
              className="rounded border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-600 hover:bg-zinc-50"
            >
              Clear Trades
            </button>
          </div>
        </div>
      </div>

      <div className="glass-panel px-5 py-6 md:px-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-[11px] font-bold uppercase tracking-[0.14em] text-zinc-500">
            Futures Watchlist
          </h2>
          <input
            type="search"
            value={watchSearch}
            onChange={(e) => setWatchSearch(e.target.value)}
            placeholder="Search symbol"
            className="w-full max-w-56 rounded-lg border border-zinc-300 bg-white/90 px-3 py-2 text-xs text-zinc-800 outline-none focus:border-blue-400"
          />
        </div>
        <div className="overflow-x-auto rounded-[16px] border border-zinc-200 bg-white/85">
          <table className="w-full text-left text-xs" style={{ minWidth: 760 }}>
            <thead className="bg-zinc-50 text-[10px] font-bold uppercase tracking-[0.12em] text-zinc-500">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Last Price</th>
                <th className="px-4 py-3">24h Chg.</th>
                <th className="px-4 py-3">24h Vol.</th>
              </tr>
            </thead>
            <tbody>
              {visibleWatchlist.map((item) => {
                const positive = !item.change24h.startsWith("-");
                return (
                  <tr key={item.symbol} className="border-t border-zinc-100">
                    <td className="px-4 py-3">
                      <div className="font-semibold text-zinc-900">
                        {item.symbol}
                        <span className="ml-2 rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-bold text-amber-700">
                          {item.leverage}
                        </span>
                      </div>
                      <div className="text-[11px] text-zinc-500">{item.name}</div>
                    </td>
                    <td className="px-4 py-3 font-mono text-zinc-900">{item.lastPrice}</td>
                    <td className={`px-4 py-3 font-mono ${positive ? "text-emerald-600" : "text-rose-600"}`}>{item.change24h}</td>
                    <td className="px-4 py-3 font-mono text-zinc-800">{item.volume24h}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* Main Equity Display */}
      <div className="mb-6 grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)]">
        <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
          <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-blue-500/10 blur-3xl pointer-events-none" />
          <div className="mb-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
            FUTURE TRADING EQUITY
          </div>
          <div className="flex items-baseline gap-3">
            <span className={`text-4xl font-bold tabular-nums ${pnlPositive ? "text-emerald-600" : "text-rose-600"}`}>
              {fmtUSD(equity)}
            </span>
            <span className={`text-lg font-semibold ${pnlPositive ? "text-emerald-600" : "text-rose-600"}`}>
              {fmtPct(totalReturn, true)}
            </span>
          </div>
          <div className="mt-2 text-xs text-zinc-500">
            Session PnL {fmtUSD(sessionPnL, { signed: true })} · {stats.totalTrades} completed trades
          </div>

          {quote && (
            <div className="mt-4 flex flex-wrap items-center gap-4 border-t border-zinc-100 pt-3 text-xs">
              <span className="text-zinc-400">
                BTC/USD <span className="font-mono text-zinc-900">${quote.markPrice.toLocaleString()}</span>
              </span>
              <span className="text-zinc-400">
                24h <span className={`font-mono ${quote.changePct24h >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {quote.changePct24h >= 0 ? "+" : ""}{quote.changePct24h.toFixed(2)}%
                </span>
              </span>
              <span className="text-zinc-400">
                Funding <span className={`font-mono ${quote.fundingRate >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtPct(quote.fundingRate * 100, true, 4)}
                </span>
              </span>
            </div>
          )}
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <CompactMetric
              label="Open Exposure"
              value={`${stats.openPositions} positions`}
              detail={`${longCount} long / ${shortCount} short`}
              accent="text-zinc-900"
            />
            <CompactMetric
              label="Session Runtime"
              value={isReady ? "Live" : "Syncing"}
              detail={`${strategyStatuses.length} strategies active`}
              accent={isReady ? "text-emerald-600" : "text-amber-600"}
            />
            <CompactMetric
              label="Last Closed Trade"
              value={trades.length > 0 ? fmtUSD(sortedTrades[0].netPnl, { signed: true }) : "No exits yet"}
              detail={trades.length > 0 ? sortedTrades[0].strategyName : "Waiting for first close"}
              accent={trades.length > 0 && sortedTrades[0].netPnl >= 0 ? "text-emerald-600" : "text-zinc-900"}
            />
          </div>
        </div>

        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="mb-3 text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
            Equity And PnL
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <CompactMetric label="Account Equity" value={fmtUSD(equity)} detail="Base $1,000.00" accent="text-zinc-900" />
            <CompactMetric label="Net PnL" value={fmtUSD(sessionPnL, { signed: true })} detail={`${fmtPct(totalReturn, true)} vs base`} accent={pnlPositive ? "text-emerald-600" : "text-rose-600"} />
            <CompactMetric label="Closed PnL" value={fmtUSD(stats.netPnl, { signed: true })} detail={`${stats.totalTrades} completed trades`} accent={stats.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
            <CompactMetric label="Unrealized" value={fmtUSD(totalUnrealized, { signed: true })} detail="Live open position PnL" accent={totalUnrealized >= 0 ? "text-emerald-600" : "text-rose-600"} />
          </div>
        </div>
      </div>

      {/* Compact Metric Cards */}
      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-8">
        <SummaryCard label="Win Rate" value={fmtPct(stats.winRate, false, 1)} accent="text-zinc-900" />
        <SummaryCard label="Profit Factor" value={stats.profitFactor > 100 ? "∞" : stats.profitFactor.toFixed(2)} accent={stats.profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Trades" value={`${stats.totalTrades}`} accent="text-zinc-900" />
        <SummaryCard label="Unrealized" value={fmtUSD(totalUnrealized, { signed: true })} accent={totalUnrealized >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Streak" value={`${stats.winCount}W`} accent="text-amber-500" />
        <SummaryCard label="Best Trade" value={trades.length > 0 ? fmtUSD(Math.max(...trades.map((t) => t.netPnl)), { signed: true }) : "$0.00"} accent="text-emerald-600" />
        <SummaryCard label="Open Pos" value={`${stats.openPositions}`} accent="text-zinc-900" />
        <SummaryCard label="Liq Risk" value={`${stats.liquidationRisk}`} accent={stats.liquidationRisk > 0 ? "text-rose-600" : "text-zinc-900"} />
      </div>

      {/* Open Positions */}
      <div className="mb-6 glass-panel px-5 py-6 md:px-6">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-emerald-700">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
              LIVE
            </span>
            <span className="text-[11px] font-bold uppercase tracking-wider text-zinc-500">
              OPEN FUTURES POSITIONS ({positions.length} active)
            </span>
          </div>
          <span className={`rounded-full px-3 py-1 text-xs font-medium ${totalUnrealized >= 0 ? "bg-emerald-50 text-emerald-700" : "bg-rose-50 text-rose-700"}`}>
            Unrealized {fmtUSD(totalUnrealized, { signed: true })}
          </span>
        </div>

        {positions.length === 0 ? (
          <div className="flex min-h-[120px] items-center justify-center rounded-xl border border-dashed border-zinc-300 bg-zinc-50 text-sm text-zinc-500">
            No open positions — the engine scans 180 strategies on each tick for high-probability setups.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs" style={{ minWidth: 1100 }}>
              <thead className="text-[10px] font-bold uppercase tracking-wider text-zinc-400">
                <tr className="border-b border-zinc-200">
                  <th className="py-2 pr-3">Position</th>
                  <th className="py-2 pr-3">Side</th>
                  <th className="py-2 pr-3">Contracts</th>
                  <th className="py-2 pr-3">Entry</th>
                  <th className="py-2 pr-3">Mark</th>
                  <th className="py-2 pr-3">Margin</th>
                  <th className="py-2 pr-3">Opened</th>
                  <th className="py-2 pr-3">PnL</th>
                  <th className="py-2 pr-3">Liq Price</th>
                  <th className="py-2 text-right">Progress</th>
                </tr>
              </thead>
              <tbody className="text-zinc-700">
                {positions.map((pos) => (
                  <tr key={pos.id} className="border-b border-zinc-100 last:border-0">
                    <td className="py-2 pr-3">
                      <div className="flex items-center gap-2">
                        <span className={`h-2 w-2 rounded-full ${pos.unrealizedPnl >= 0 ? "bg-emerald-500" : "bg-rose-500"}`} />
                        <div>
                          <div className="font-medium text-zinc-900">{pos.strategyName}</div>
                          <div className="text-[10px] text-zinc-400">{pos.leverage}x · {pos.marginMode}</div>
                        </div>
                      </div>
                    </td>
                    <td className="py-2 pr-3"><SideBadge side={pos.side} /></td>
                    <td className="py-2 pr-3 font-mono">{fmtContracts(pos.contracts)}</td>
                    <td className="py-2 pr-3 font-mono">${pos.entryPrice.toLocaleString()}</td>
                    <td className="py-2 pr-3 font-mono">${pos.markPrice.toLocaleString()}</td>
                    <td className="py-2 pr-3 font-mono">{fmtUSD(pos.marginUsed)}</td>
                    <td className="py-2 pr-3">
                      <div className="font-mono">{formatShortTime(pos.openedAt)}</div>
                      <div className="text-[10px] text-zinc-400">{formatShortDate(pos.openedAt)}</div>
                    </td>
                    <td className="py-2 pr-3">
                      <div className={`font-mono font-bold ${pos.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                        {fmtUSD(pos.unrealizedPnl, { signed: true })}
                      </div>
                      <div className="text-[10px] text-zinc-400">{fmtPct(pos.returnPct, true)}</div>
                    </td>
                    <td className="py-2 pr-3 font-mono text-zinc-500">
                      ${Math.round(pos.liquidationPrice).toLocaleString()}
                    </td>
                    <td className="py-2 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <span className={`text-[10px] font-mono ${pos.returnPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                          {pos.returnPct >= 0 ? "+" : ""}{pos.returnPct.toFixed(1)}%
                        </span>
                        <PremiumBar progress={pos.returnPct} isPositive={pos.returnPct >= 0} />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Daily PnL Ledger */}
      {trades.length > 0 && (
        <div className="mb-6 glass-panel px-5 py-6 md:px-6">
          <div className="mb-4 text-[11px] font-bold uppercase tracking-wider text-zinc-500">
            Daily PnL Ledger ({Object.keys(tradesByDay).length} trading days)
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs" style={{ minWidth: 700 }}>
              <thead className="text-[10px] font-bold uppercase tracking-wider text-zinc-400">
                <tr className="border-b border-zinc-200">
                  <th className="py-2 pr-3">Date</th>
                  <th className="py-2 pr-3">Trades</th>
                  <th className="py-2 pr-3">W / L</th>
                  <th className="py-2 pr-3">Daily PnL</th>
                  <th className="py-2 text-right">Daily %</th>
                </tr>
              </thead>
              <tbody className="text-zinc-700">
                {Object.entries(tradesByDay).sort((a, b) => b[0].localeCompare(a[0])).map(([day, data]) => (
                  <tr key={day} className="border-b border-zinc-100 last:border-0">
                    <td className="py-2 pr-3">
                      <div className="font-medium">{formatDate(day)}</div>
                      <div className="text-[10px] text-zinc-400">{day}</div>
                    </td>
                    <td className="py-2 pr-3 font-mono">{data.trades}</td>
                    <td className="py-2 pr-3 font-mono">{data.wins}W / {data.losses}L</td>
                    <td className={`py-2 pr-3 font-mono font-bold ${data.pnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtUSD(data.pnl, { signed: true })}
                    </td>
                    <td className={`py-2 text-right font-mono ${data.pnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtPct((data.pnl / 1000) * 100, true)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Strategy Leaderboard */}
      <div className="mb-6 glass-panel px-5 py-6 md:px-6">
        <div className="mb-4 flex items-center justify-between">
          <span className="text-[11px] font-bold uppercase tracking-wider text-zinc-500">
            Strategy Leaderboard ({strategyStatuses.length} strategies)
          </span>
          <button
            onClick={() => setShowAllStrategies(!showAllStrategies)}
            className="rounded border border-zinc-200 bg-white px-3 py-1 text-xs font-medium text-zinc-600 hover:bg-zinc-50"
          >
            {showAllStrategies ? "Top 12" : `All ${strategyStatuses.length}`}
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs" style={{ minWidth: 900 }}>
            <thead className="text-[10px] font-bold uppercase tracking-wider text-zinc-400">
              <tr className="border-b border-zinc-200">
                <th className="py-2 pr-2">#</th>
                <th className="py-2 pr-2">Strategy</th>
                <th className="py-2 pr-2">Category</th>
                <th className="py-2 pr-2">Status</th>
                <th className="py-2 pr-2">Score</th>
                <th className="py-2 pr-2">Live</th>
                <th className="py-2 text-right">PnL</th>
              </tr>
            </thead>
            <tbody className="text-zinc-700">
              {visibleStrategies.map((s, i) => (
                <tr key={s.id} className="border-b border-zinc-100 last:border-0">
                  <td className="py-2 pr-2 font-mono text-zinc-400">{s.id}</td>
                  <td className="py-2 pr-2">
                    <div className="font-medium text-zinc-900">{s.name}</div>
                    <div className="text-[10px] text-zinc-400">{s.category}</div>
                  </td>
                  <td className="py-2 pr-2 text-zinc-500">{s.category}</td>
                  <td className="py-2 pr-2"><StatusBadge status={s.status} /></td>
                  <td className="py-2 pr-2 font-mono font-bold">{s.score.toFixed(1)}</td>
                  <td className="py-2 pr-2 font-mono">
                    {s.openCount > 0 ? `${s.openCount} pos` : "-"}
                  </td>
                  <td className="py-2 text-right">
                    <button
                      onClick={() => {
                        const newDisabled = disabledStrategies.includes(s.id)
                          ? disabledStrategies.filter(id => id !== s.id)
                          : [...disabledStrategies, s.id];
                        setDisabledStrategies(newDisabled);
                      }}
                      className={`text-[10px] px-2 py-1 rounded ${
                        s.disabled
                          ? "bg-zinc-100 text-zinc-400"
                          : "bg-emerald-50 text-emerald-700"
                      }`}
                    >
                      {s.disabled ? "Disabled" : "Active"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Trade History */}
      {trades.length > 0 && (
        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-4">
              <div className="text-center">
                <div className="text-xs text-zinc-400">Trades</div>
                <div className="text-xl font-bold text-zinc-900">{stats.totalTrades}</div>
              </div>
              <div className="text-center">
                <div className="text-xs text-zinc-400">Win Rate</div>
                <div className="text-xl font-bold text-emerald-600">{fmtPct(stats.winRate, false, 1)}</div>
              </div>
              <div className="text-center">
                <div className="text-xs text-zinc-400">Net PnL</div>
                <div className={`text-xl font-bold ${stats.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtUSD(stats.netPnl, { signed: true })}
                </div>
              </div>
              <div className="text-center">
                <div className="text-xs text-zinc-400">W/L</div>
                <div className="text-xl font-bold text-zinc-900">{stats.winCount}/{stats.lossCount}</div>
              </div>
            </div>
            <button
              onClick={() => setShowAllTrades(!showAllTrades)}
              className="rounded border border-zinc-200 bg-white px-3 py-1 text-xs font-medium text-zinc-600 hover:bg-zinc-50"
            >
              {showAllTrades ? "Recent 10" : `All ${trades.length}`}
            </button>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs" style={{ minWidth: 1000 }}>
              <thead className="text-[10px] font-bold uppercase tracking-wider text-zinc-400">
                <tr className="border-b border-zinc-200">
                  <th className="py-2 pr-3">Time</th>
                  <th className="py-2 pr-3">Strategy</th>
                  <th className="py-2 pr-3">Side</th>
                  <th className="py-2 pr-3">Contracts</th>
                  <th className="py-2 pr-3">Entry/Exit</th>
                  <th className="py-2 pr-3">Duration</th>
                  <th className="py-2 pr-3">Exit</th>
                  <th className="py-2 pr-3">Return</th>
                  <th className="py-2 text-right">Net PnL</th>
                </tr>
              </thead>
              <tbody className="text-zinc-700">
                {visibleTrades.map((t) => (
                  <tr key={t.id} className="border-b border-zinc-100 last:border-0">
                    <td className="py-2 pr-3">
                      <div className="font-mono">{formatShortTime(t.closedAt)}</div>
                      <div className="text-[10px] text-zinc-400">{formatShortDate(t.closedAt)}</div>
                    </td>
                    <td className="py-2 pr-3">
                      <div className="font-medium text-zinc-900">{t.strategyName}</div>
                    </td>
                    <td className="py-2 pr-3"><SideBadge side={t.side} /></td>
                    <td className="py-2 pr-3 font-mono">{fmtContracts(t.contracts)}</td>
                    <td className="py-2 pr-3 font-mono text-zinc-500">
                      ${t.entryPrice.toLocaleString()} → ${t.exitPrice.toLocaleString()}
                    </td>
                    <td className="py-2 pr-3 font-mono text-zinc-500">
                      {(() => {
                        const mins = Math.floor((new Date(t.closedAt).getTime() - new Date(t.openedAt).getTime()) / 60000);
                        return mins > 0 ? `${mins}m` : "<1m";
                      })()}
                    </td>
                    <td className="py-2 pr-3">
                      <BadgePill
                        label={t.exitReason}
                        tone={t.exitReason === "TP" ? "positive" : t.exitReason === "SL" ? "negative" : "neutral"}
                      />
                    </td>
                    <td className={`py-2 pr-3 font-mono ${t.netPnlPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtPct(t.netPnlPct, true)}
                    </td>
                    <td className={`py-2 text-right font-mono font-bold ${t.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtUSD(t.netPnl, { signed: true })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
