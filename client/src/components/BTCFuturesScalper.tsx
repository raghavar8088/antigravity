"use client";

import { useState } from "react";
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
  return n.toLocaleString("en-US");
}

function formatShortTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: false });
}

function formatShortDate(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
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
    <span className={`inline-flex items-center rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] ${map[tone]}`}>
      {label}
    </span>
  );
}

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${
      side === "LONG"
        ? "border-emerald-500/25 bg-emerald-500/10 text-emerald-600"
        : "border-rose-500/25 bg-rose-500/10 text-rose-600"
    }`}>{side}</span>
  );
}

function CompactMetric({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-3 flex min-h-[88px] flex-col justify-between gap-2">
      <div>
        <div className="text-[10px] font-semibold uppercase tracking-widest text-zinc-500">{label}</div>
        <div className="text-lg font-bold tabular-nums text-zinc-900">{value}</div>
      </div>
      <div className="text-[10px] text-zinc-500 min-h-[16px]">{detail ?? ""}</div>
    </div>
  );
}

function SummaryCard({ label, value, accent }: { label: string; value: string; accent?: string }) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-4 flex min-h-[100px] flex-col justify-between gap-2">
      <div className="text-[10px] font-semibold uppercase tracking-widest text-zinc-500">{label}</div>
      <div className={`text-2xl font-bold tabular-nums ${accent || "text-zinc-900"}`}>{value}</div>
    </div>
  );
}

// ========== POSITION PROGRESS BAR ==========
function PositionProgressBar({
  returnPct,
  unrealizedPnl,
  entryPrice,
  tpPrice,
  slPrice,
  liquidationPrice,
  side,
}: {
  returnPct: number;
  unrealizedPnl: number;
  entryPrice: number;
  tpPrice: number;
  slPrice: number;
  liquidationPrice: number;
  side: "LONG" | "SHORT";
}) {
  const pnlPositive = unrealizedPnl >= 0;

  return (
    <div className="w-full max-w-[200px]">
      <div className="mb-1 flex items-center justify-between">
        <span className={`text-[10px] font-bold ${pnlPositive ? "text-emerald-600" : "text-rose-600"}`}>
          {pnlPositive ? "▲" : "▼"} {fmtPct(returnPct, true)}
        </span>
      </div>
      <div className="relative h-2 w-full rounded-full bg-zinc-100 overflow-hidden">
        <div className="absolute left-0 top-0 h-full w-[20%] rounded-l-full bg-rose-100" />
        <div className="absolute right-0 top-0 h-full w-[20%] rounded-r-full bg-emerald-100" />
        <div className={`absolute top-1/2 h-2 w-2 -translate-y-1/2 rounded-full ${pnlPositive ? "bg-emerald-500" : "bg-rose-500"}`}
          style={{ left: `${Math.max(10, Math.min(90, 50 + returnPct * 2))}%` }} />
      </div>
      <div className="mt-1 flex justify-between text-[9px] text-zinc-400">
        <span>Liq ${Math.round(liquidationPrice).toLocaleString()}</span>
        <span>Entry ${Math.round(entryPrice).toLocaleString()}</span>
        <span>TP ${Math.round(tpPrice).toLocaleString()}</span>
      </div>
    </div>
  );
}

// ========== MARKET HERO ==========
function MarketHero({ quote, isReady }: { quote: any; isReady: boolean }) {
  if (!quote) return null;

  const changePositive = quote.changePct24h >= 0;
  const fundingPositive = quote.fundingRate >= 0;

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-[10px] font-bold uppercase tracking-widest text-emerald-700">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
            {isReady ? "LIVE" : "WARMING"}
          </span>
          <span className="text-[10px] font-semibold uppercase tracking-widest text-zinc-500">
            BTCUSD PERPETUAL FUTURES · 25x LEVERAGE
          </span>
        </div>
        <div className="flex items-center gap-4 text-xs text-zinc-500">
          <span>Mark: <span className="font-mono font-medium text-zinc-900">${quote.markPrice.toLocaleString()}</span></span>
          <span>Last: <span className="font-mono">${quote.lastPrice.toLocaleString()}</span></span>
          <span>Funding: <span className={`font-mono ${fundingPositive ? "text-emerald-600" : "text-rose-600"}`}>{fmtPct(quote.fundingRate * 100, true, 4)}</span></span>
        </div>
      </div>

      <div className="mt-4 grid grid-cols-2 gap-4 md:grid-cols-4">
        <CompactMetric label="Mark Price" value={`$${quote.markPrice.toLocaleString()}`} detail="Fair value for PnL" />
        <CompactMetric label="24h Change" value={fmtPct(quote.changePct24h, true)} detail={changePositive ? "Bullish momentum" : "Bearish pressure"} />
        <CompactMetric label="Funding Rate" value={fmtPct(quote.fundingRate * 100, true, 4)} detail={fundingPositive ? "Longs pay shorts" : "Shorts pay longs"} />
        <CompactMetric label="Next Funding" value={new Date(quote.nextFunding * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} detail="Every 8 hours" />
      </div>
    </div>
  );
}

// ========== POSITIONS PANEL ==========
function PositionsPanel({ positions, quote }: { positions: BTCFuturesPosition[]; quote: any }) {
  const totalUnrealized = positions.reduce((s, p) => s + p.unrealizedPnl, 0);
  const longCount = positions.filter(p => p.side === "LONG").length;
  const shortCount = positions.filter(p => p.side === "SHORT").length;

  if (positions.length === 0) {
    return (
      <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-6">
        <h2 className="text-[11px] font-extrabold uppercase tracking-[0.14em] text-zinc-500">
          <span className="inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-[10px] font-bold uppercase tracking-widest text-emerald-700 mr-2">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
            LIVE
          </span>
          OPEN FUTURES POSITIONS
        </h2>
        <div className="mt-4 flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed border-zinc-300 px-6 py-12 text-center text-sm text-zinc-500 bg-zinc-50">
          No open futures positions yet — the engine scans 180 strategies on each tick for high-probability setups.
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-6">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-[11px] font-extrabold uppercase tracking-[0.14em] text-zinc-500">
          <span className="inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-[10px] font-bold uppercase tracking-widest text-emerald-700 mr-2">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
            LIVE
          </span>
          OPEN FUTURES POSITIONS · {longCount} LONG / {shortCount} SHORT
          <span className="font-mono ml-2 text-[10px] font-medium text-zinc-400">
            ({positions.length} active)
          </span>
        </h2>
        <span className="rounded-full border px-3 py-1 text-xs font-medium"
          style={{
            background: totalUnrealized >= 0 ? "rgba(16, 185, 129, 0.1)" : "rgba(244, 63, 94, 0.1)",
            color: totalUnrealized >= 0 ? "#10b981" : "#f43f5e",
            borderColor: totalUnrealized >= 0 ? "rgba(16, 185, 129, 0.2)" : "rgba(244, 63, 94, 0.2)",
          }}>
          Unrealized {fmtUSD(totalUnrealized, { signed: true })}
        </span>
      </div>

      <div className="overflow-x-auto rounded-[20px] border border-zinc-200 bg-white">
        <table className="w-full text-left text-sm" style={{ minWidth: 1100 }}>
          <thead className="bg-zinc-50 text-zinc-500">
            <tr className="text-[11px] uppercase tracking-[0.12em]">
              <th className="px-4 py-3 font-medium">Position</th>
              <th className="px-4 py-3 font-medium">Side</th>
              <th className="px-4 py-3 font-medium">Contracts</th>
              <th className="px-4 py-3 font-medium">Entry/Mark</th>
              <th className="px-4 py-3 font-medium">Margin Used</th>
              <th className="px-4 py-3 font-medium">Opened</th>
              <th className="px-4 py-3 font-medium">PnL</th>
              <th className="px-4 py-3 font-medium">Liq Price</th>
              <th className="px-4 py-3 font-medium text-right">Progress</th>
            </tr>
          </thead>
          <tbody>
            {positions.map((pos) => (
              <tr key={pos.id} className="border-t border-zinc-100">
                <td className="px-4 py-3">
                  <div className="flex flex-col gap-1">
                    <div className="flex items-center gap-2">
                      <span className={`h-2 w-2 rounded-full ${pos.unrealizedPnl >= 0 ? "bg-emerald-500" : "bg-rose-500"}`} />
                      <span className="text-sm font-semibold text-zinc-900">{pos.strategyName}</span>
                    </div>
                    <div className="text-[11px] text-zinc-500">{pos.leverage}x · {pos.marginMode}</div>
                  </div>
                </td>
                <td className="px-4 py-3"><SideBadge side={pos.side} /></td>
                <td className="px-4 py-3 text-xs font-mono text-zinc-900">{fmtContracts(pos.contracts)}</td>
                <td className="px-4 py-3 text-xs">
                  <div className="font-mono text-zinc-900">${pos.entryPrice.toLocaleString()}</div>
                  <div className="text-zinc-500">→ ${pos.markPrice.toLocaleString()}</div>
                </td>
                <td className="px-4 py-3 text-xs font-mono text-zinc-900">{fmtUSD(pos.marginUsed)}</td>
                <td className="px-4 py-3 text-xs">
                  <div className="font-mono text-zinc-900">{formatShortTime(pos.openedAt)}</div>
                  <div className="text-zinc-500">{formatShortDate(pos.openedAt)}</div>
                </td>
                <td className="px-4 py-3">
                  <div className={`font-mono text-sm font-semibold ${pos.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                    {fmtUSD(pos.unrealizedPnl, { signed: true })}
                  </div>
                  <div className="text-[11px] text-zinc-500">{fmtPct(pos.returnPct, true)}</div>
                </td>
                <td className="px-4 py-3">
                  <div className={`font-mono text-xs ${pos.side === "LONG" && pos.liquidationPrice >= pos.markPrice * 0.95 ? "text-red-600 font-bold" : pos.side === "SHORT" && pos.liquidationPrice <= pos.markPrice * 1.05 ? "text-red-600 font-bold" : "text-zinc-500"}`}>
                    ${pos.liquidationPrice.toLocaleString()}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <div className="ml-auto">
                    <PositionProgressBar
                      returnPct={pos.returnPct}
                      unrealizedPnl={pos.unrealizedPnl}
                      entryPrice={pos.entryPrice}
                      tpPrice={pos.tpPrice}
                      slPrice={pos.slPrice}
                      liquidationPrice={pos.liquidationPrice}
                      side={pos.side}
                    />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ========== STATS PANEL ==========
function StatsPanel({ stats, balance, equity }: { stats: BTCFuturesEngineStats; balance: number; equity: number }) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-6">
      <h2 className="text-[11px] font-extrabold uppercase tracking-[0.14em] text-zinc-500 mb-4">
        PERFORMANCE METRICS
      </h2>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4 lg:grid-cols-6">
        <SummaryCard label="Balance" value={fmtUSD(balance)} />
        <SummaryCard label="Equity" value={fmtUSD(equity, { signed: true })} accent={equity >= balance ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Total PnL" value={fmtUSD(stats.netPnl, { signed: true })} accent={stats.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Win Rate" value={fmtPct(stats.winRate)} accent={stats.winRate > 50 ? "text-emerald-600" : ""} />
        <SummaryCard label="Trades" value={`${stats.totalTrades}`} />
        <SummaryCard label="Open Pos" value={`${stats.openPositions}/${stats.maxPositions}`} accent={stats.openPositions > 8 ? "text-amber-600" : ""} />
        <CompactMetric label="Available Margin" value={fmtUSD(stats.availableMargin)} detail={stats.marginUtilization > 80 ? "High utilization" : "Healthy buffer"} />
        <CompactMetric label="Used Margin" value={fmtUSD(stats.usedMargin)} detail={`${fmtPct(stats.marginUtilization)} of balance`} />
        <CompactMetric label="Long/Short" value={`${stats.longCount}/${stats.shortCount}`} detail="Position balance" />
        <CompactMetric label="Liq Risk" value={`${stats.liquidationRisk}`} detail={stats.liquidationRisk > 0 ? "⚠️ Near liquidation" : "Safe distance"} />
        <CompactMetric label="Avg Leverage" value={`${stats.avgLeverage.toFixed(1)}x`} detail="Target: 25x" />
        <CompactMetric label="Profit Factor" value={stats.profitFactor.toFixed(2)} detail={stats.profitFactor > 1.5 ? "Strong edge" : "Building edge"} />
      </div>
    </div>
  );
}

// ========== STRATEGIES PANEL ==========
function StrategiesPanel({ strategies, disabledStrategies, setDisabledStrategies }: {
  strategies: BTCFuturesStrategyStatus[];
  disabledStrategies: number[];
  setDisabledStrategies: (ids: number[]) => void;
}) {
  const [showAll, setShowAll] = useState(false);
  const visible = showAll ? strategies : strategies.slice(0, 24);

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-6">
      <div className="mb-5 flex items-center justify-between">
        <h2 className="text-[11px] font-extrabold uppercase tracking-[0.14em] text-zinc-500">
          STRATEGIES — {strategies.length} TOTAL
        </h2>
        <button
          onClick={() => setShowAll(!showAll)}
          className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-1.5 text-xs font-medium text-zinc-600"
        >
          {showAll ? "Show Top 24" : `Show All ${strategies.length}`}
        </button>
      </div>

      <div className="grid grid-cols-2 gap-2 md:grid-cols-4 lg:grid-cols-6">
        {visible.map((s) => (
          <button
            key={s.id}
            onClick={() => {
              const newDisabled = disabledStrategies.includes(s.id)
                ? disabledStrategies.filter(id => id !== s.id)
                : [...disabledStrategies, s.id];
              setDisabledStrategies(newDisabled);
            }}
            className={`rounded-xl border p-3 text-left transition-all ${
              s.disabled
                ? "opacity-50 border-zinc-200 bg-zinc-100"
                : s.status === "OPEN"
                ? "border-emerald-200 bg-emerald-50"
                : s.status === "COOLING"
                ? "border-amber-200 bg-amber-50"
                : "border-zinc-200 bg-white hover:border-zinc-300"
            }`}
          >
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-mono text-zinc-400">#{s.id}</span>
              {s.openCount > 0 && <BadgePill label={`${s.openCount} open`} tone="positive" />}
            </div>
            <div className="mt-1 text-xs font-semibold text-zinc-900 truncate">{s.name}</div>
            <div className="text-[10px] text-zinc-500">{s.category}</div>
          </button>
        ))}
      </div>
    </div>
  );
}

// ========== TRADES PANEL ==========
function TradesPanel({ trades, clearTradeHistory }: { trades: BTCFuturesTrade[]; clearTradeHistory: () => void }) {
  const [showAll, setShowAll] = useState(false);
  const visible = showAll ? trades.slice().reverse() : trades.slice().reverse().slice(0, 20);

  if (trades.length === 0) {
    return (
      <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-6">
        <h2 className="text-[11px] font-extrabold uppercase tracking-[0.14em] text-zinc-500">
          TRADE HISTORY
        </h2>
        <div className="mt-4 rounded-[20px] border border-dashed border-zinc-300 px-6 py-12 text-center text-sm text-zinc-500 bg-zinc-50">
          No closed trades yet — the engine will record each completed futures trade here.
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-5 py-6">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-[11px] font-extrabold uppercase tracking-[0.14em] text-zinc-500">
          TRADE HISTORY ({trades.length})
        </h2>
        <div className="flex gap-2">
          <button
            onClick={() => setShowAll(!showAll)}
            className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-1.5 text-xs font-medium text-zinc-600"
          >
            {showAll ? "Show Recent" : "Show All"}
          </button>
          <button
            onClick={clearTradeHistory}
            className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs font-medium text-rose-700"
          >
            Clear
          </button>
        </div>
      </div>

      <div className="overflow-x-auto rounded-[20px] border border-zinc-200 bg-white">
        <table className="w-full text-left text-xs" style={{ minWidth: 1000 }}>
          <thead className="bg-zinc-50 text-zinc-500">
            <tr className="text-[10px] uppercase tracking-widest">
              <th className="px-3 py-2 font-medium">Time</th>
              <th className="px-3 py-2 font-medium">Strategy</th>
              <th className="px-3 py-2 font-medium">Side</th>
              <th className="px-3 py-2 font-medium">Contracts</th>
              <th className="px-3 py-2 font-medium">Entry/Exit</th>
              <th className="px-3 py-2 font-medium">Gross PnL</th>
              <th className="px-3 py-2 font-medium">Fees</th>
              <th className="px-3 py-2 font-medium">Net PnL</th>
              <th className="px-3 py-2 font-medium">Return %</th>
              <th className="px-3 py-2 font-medium">Reason</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((t) => (
              <tr key={t.id} className="border-t border-zinc-100">
                <td className="px-3 py-2 text-[10px] text-zinc-500">
                  {formatShortTime(t.closedAt)} {formatShortDate(t.closedAt)}
                </td>
                <td className="px-3 py-2 text-xs font-medium text-zinc-900">{t.strategyName}</td>
                <td className="px-3 py-2"><SideBadge side={t.side} /></td>
                <td className="px-3 py-2 text-xs font-mono text-zinc-500">{fmtContracts(t.contracts)}</td>
                <td className="px-3 py-2 text-xs font-mono text-zinc-500">
                  ${t.entryPrice.toLocaleString()} → ${t.exitPrice.toLocaleString()}
                </td>
                <td className="px-3 py-2 text-xs font-mono text-zinc-900">{fmtUSD(t.realizedPnl, { signed: true })}</td>
                <td className="px-3 py-2 text-xs font-mono text-rose-600">-{fmtUSD(t.fees)}</td>
                <td className={`px-3 py-2 text-xs font-mono font-bold ${t.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtUSD(t.netPnl, { signed: true })}
                </td>
                <td className={`px-3 py-2 text-xs font-mono ${t.netPnlPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtPct(t.netPnlPct, true)}
                </td>
                <td className="px-3 py-2">
                  <BadgePill label={t.exitReason} tone={t.exitReason === "TP" ? "positive" : t.exitReason === "SL" || t.exitReason === "LIQUIDATION_RISK" ? "negative" : "neutral"} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

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

  const [activeTab, setActiveTab] = useState<"positions" | "strategies" | "trades">("positions");

  return (
    <div className="min-h-screen bg-zinc-50 p-4">
      {/* Header */}
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-zinc-900">BTC Futures Scalper</h1>
          <p className="text-[11px] text-zinc-500">
            180 strategies · 25x leverage · $1,000 paper · Delta Exchange compatible
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={togglePause}
            className={`rounded-lg border px-4 py-2 text-xs font-semibold ${
              pauseEntries
                ? "border-amber-200 bg-amber-50 text-amber-700"
                : "border-emerald-200 bg-emerald-50 text-emerald-700"
            }`}
          >
            {pauseEntries ? "⏸ PAUSED" : "▶ LIVE"}
          </button>
          <button
            onClick={resetPaperAccount}
            className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-2 text-xs font-semibold text-rose-700"
          >
            Reset Account
          </button>
        </div>
      </div>

      {/* Market Hero */}
      <div className="mb-4">
        <MarketHero quote={quote} isReady={isReady} />
      </div>

      {/* Stats */}
      <div className="mb-4">
        <StatsPanel stats={stats} balance={balance} equity={equity} />
      </div>

      {/* Tab Navigation */}
      <div className="mb-4 flex gap-2">
        {["positions", "strategies", "trades"].map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab as any)}
            className={`rounded-lg border px-4 py-2 text-xs font-medium capitalize ${
              activeTab === tab
                ? "border-zinc-300 bg-zinc-100 text-zinc-900"
                : "border-transparent text-zinc-500 hover:text-zinc-700"
            }`}
          >
            {tab} ({tab === "positions" ? positions.length : tab === "strategies" ? 180 : trades.length})
          </button>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === "positions" && <PositionsPanel positions={positions} quote={quote} />}
      {activeTab === "strategies" && <StrategiesPanel strategies={strategyStatuses} disabledStrategies={disabledStrategies} setDisabledStrategies={setDisabledStrategies} />}
      {activeTab === "trades" && <TradesPanel trades={trades} clearTradeHistory={clearTradeHistory} />}
    </div>
  );
}
