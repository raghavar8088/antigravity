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
    <div className="min-h-screen bg-zinc-50 p-4 md:p-6">
      {/* Header */}
      <div className="mb-6">
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
          <h1 className="text-xl font-bold text-zinc-900">BTC Futures Scalper</h1>
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

      {/* Main Equity Display */}
      <div className="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-3">
        {/* Left: Big Equity */}
        <div className="rounded-2xl border border-zinc-200 bg-white p-5 lg:col-span-2">
          <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
            BTC FUTURES SCALPER EQUITY
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
            <div className="mt-4 flex items-center gap-4 border-t border-zinc-100 pt-3 text-xs">
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
        </div>

        {/* Right: Session Stats */}
        <div className="rounded-2xl border border-zinc-200 bg-white p-5">
          <div className="mb-3 text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
            Equity And PnL
          </div>
          <div className="space-y-3">
            <div>
              <div className="text-[10px] text-zinc-400">Account Equity</div>
              <div className="text-lg font-bold text-zinc-900">{fmtUSD(equity)}</div>
              <div className="text-[10px] text-zinc-400">Base $1,000.00</div>
            </div>
            <div className="border-t border-zinc-100 pt-3">
              <div className="text-[10px] text-zinc-400">Net PnL</div>
              <div className={`text-lg font-bold ${pnlPositive ? "text-emerald-600" : "text-rose-600"}`}>
                {fmtUSD(sessionPnL, { signed: true })}
              </div>
              <div className="text-[10px] text-zinc-400">{fmtPct(totalReturn, true)} vs base</div>
            </div>
          </div>
        </div>
      </div>

      {/* Compact Metric Cards */}
      <div className="mb-6 grid grid-cols-2 gap-3 md:grid-cols-4 lg:grid-cols-8">
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Win Rate</div>
          <div className="text-xl font-bold text-zinc-900">{fmtPct(stats.winRate, false, 1)}</div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Profit Factor</div>
          <div className="text-xl font-bold text-zinc-900">{stats.profitFactor > 100 ? "∞" : stats.profitFactor.toFixed(2)}</div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Trades</div>
          <div className="text-xl font-bold text-zinc-900">{stats.totalTrades}</div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Unrealized</div>
          <div className={`text-xl font-bold ${totalUnrealized >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
            {fmtUSD(totalUnrealized, { signed: true })}
          </div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Streak</div>
          <div className="text-xl font-bold text-zinc-900">{stats.winCount}W</div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Best Trade</div>
          <div className="text-xl font-bold text-emerald-600">
            {trades.length > 0 ? fmtUSD(Math.max(...trades.map(t => t.netPnl)), { signed: true }) : "$0.00"}
          </div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Open Pos</div>
          <div className="text-xl font-bold text-zinc-900">{stats.openPositions}</div>
          <div className="text-[10px] text-zinc-400">{longCount} long / {shortCount} short</div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-3">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">Liq Risk</div>
          <div className={`text-xl font-bold ${stats.liquidationRisk > 0 ? "text-rose-600" : "text-zinc-900"}`}>
            {stats.liquidationRisk}
          </div>
        </div>
      </div>

      {/* Open Positions */}
      <div className="mb-6 rounded-2xl border border-zinc-200 bg-white p-5">
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
        <div className="mb-6 rounded-2xl border border-zinc-200 bg-white p-5">
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
      <div className="mb-6 rounded-2xl border border-zinc-200 bg-white p-5">
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
        <div className="rounded-2xl border border-zinc-200 bg-white p-5">
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
