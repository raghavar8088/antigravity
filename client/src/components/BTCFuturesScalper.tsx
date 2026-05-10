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
  return n.toLocaleString("en-US");
}

function formatElapsed(total: number) {
  const m = Math.floor(total / 60);
  const s = total % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

// ========== COMPONENTS ==========
function BadgePill({ label, tone = "neutral" }: { label: string; tone?: "neutral" | "positive" | "negative" | "info" | "warning" }) {
  const map: Record<string, string> = {
    neutral: "border-zinc-200 bg-white text-zinc-600",
    positive: "border-emerald-200 bg-emerald-50 text-emerald-700",
    negative: "border-rose-200 bg-rose-50 text-rose-700",
    info: "border-blue-200 bg-blue-50 text-blue-700",
    warning: "border-amber-200 bg-amber-50 text-amber-700",
  };
  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-[11px] font-semibold ${map[tone]}`}>
      {label}
    </span>
  );
}

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  const isLong = side === "LONG";
  return (
    <span className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-bold ${isLong ? "bg-emerald-100 text-emerald-700" : "bg-rose-100 text-rose-700"}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${isLong ? "bg-emerald-500" : "bg-rose-500"}`} />
      {side}
    </span>
  );
}

function CompactMetric({ label, value, tone = "neutral", suffix = "" }: { label: string; value: string; tone?: "neutral" | "positive" | "negative"; suffix?: string }) {
  const map: Record<string, string> = {
    neutral: "text-zinc-900",
    positive: "text-emerald-700",
    negative: "text-rose-700",
  };
  return (
    <div className="flex flex-col">
      <span className="text-[10px] font-medium text-zinc-500">{label}</span>
      <span className={`text-sm font-bold tabular-nums ${map[tone]}`}>
        {value}{suffix}
      </span>
    </div>
  );
}

function PositionProgressBar({
  returnPct,
  unrealizedPnl,
  entryPrice,
  tpPrice,
  slPrice,
  liquidationPrice,
  currentPrice,
  contracts,
  side,
}: {
  returnPct: number;
  unrealizedPnl: number;
  entryPrice: number;
  tpPrice: number;
  slPrice: number;
  liquidationPrice: number;
  currentPrice: number;
  contracts: number;
  side: "LONG" | "SHORT";
}) {
  const isLong = side === "LONG";
  const range = Math.max(tpPrice, slPrice, liquidationPrice) - Math.min(tpPrice, slPrice, liquidationPrice);
  const minPrice = Math.min(entryPrice, tpPrice, slPrice, liquidationPrice) - range * 0.05;
  const maxPrice = Math.max(entryPrice, tpPrice, slPrice, liquidationPrice) + range * 0.05;

  const getPct = (price: number) => ((price - minPrice) / (maxPrice - minPrice)) * 100;

  const liqDist = isLong
    ? ((currentPrice - liquidationPrice) / currentPrice) * 100
    : ((liquidationPrice - currentPrice) / currentPrice) * 100;

  const projectedPnlAt = (price: number) => {
    const diff = isLong ? price - entryPrice : entryPrice - price;
    return (diff * contracts) / price;
  };

  return (
    <div className="w-full max-w-md">
      {/* Header */}
      <div className="mb-1 flex items-center justify-between">
        <span className={`text-[10px] font-bold ${returnPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
          {returnPct >= 0 ? "▲ GAIN" : "▼ LOSS"} {fmtPct(returnPct, true)}
        </span>
        <span className={`text-[10px] ${returnPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
          {fmtUSD(unrealizedPnl, { signed: true })}
        </span>
      </div>

      {/* Progress Track */}
      <div className="relative h-3 w-full rounded-full bg-zinc-100">
        {/* Loss zone */}
        <div className="absolute left-0 top-0 h-full w-1/2 rounded-l-full bg-rose-100/50" />
        {/* Profit zone */}
        <div className="absolute right-0 top-0 h-full w-1/2 rounded-r-full bg-emerald-100/50" />

        {/* Entry marker */}
        <div className="absolute top-1/2 z-10 h-4 w-0.5 -translate-y-1/2 rounded bg-blue-500" style={{ left: `${getPct(entryPrice)}%` }} />

        {/* SL marker */}
        <div className="absolute top-1/2 z-10 h-3 w-0.5 -translate-y-1/2 rounded bg-rose-500" style={{ left: `${getPct(slPrice)}%` }} />

        {/* TP marker */}
        <div className="absolute top-1/2 z-10 h-3 w-0.5 -translate-y-1/2 rounded bg-emerald-500" style={{ left: `${getPct(tpPrice)}%` }} />

        {/* Liquidation marker */}
        <div className="absolute top-1/2 z-10 h-5 w-1 -translate-y-1/2 rounded bg-red-700" style={{ left: `${getPct(liquidationPrice)}%` }} />

        {/* Current price dot */}
        <div
          className={`absolute top-1/2 z-20 h-2.5 w-2.5 -translate-y-1/2 rounded-full ${returnPct >= 0 ? "bg-emerald-500 shadow-emerald-200" : "bg-rose-500 shadow-rose-200"} shadow-md`}
          style={{ left: `${getPct(currentPrice)}%` }}
        />

        {/* Progress fill */}
        <div
          className={`absolute top-0 h-full rounded-full ${returnPct >= 0 ? "bg-emerald-500/30" : "bg-rose-500/30"}`}
          style={{
            left: `${getPct(entryPrice)}%`,
            width: `${Math.abs(returnPct) / 2}%`,
          }}
        />
      </div>

      {/* Labels */}
      <div className="mt-1 flex justify-between text-[9px] text-zinc-400">
        <div className="text-center">
          <div className="text-rose-600">SL</div>
          <div>{fmtUSD(projectedPnlAt(slPrice), { signed: true })}</div>
        </div>
        <div className="text-center">
          <div className="text-blue-600">Entry</div>
          <div>${entryPrice.toLocaleString()}</div>
        </div>
        <div className="text-center">
          <div className="text-emerald-600">TP</div>
          <div>{fmtUSD(projectedPnlAt(tpPrice), { signed: true })}</div>
        </div>
      </div>

      {/* Liquidation warning */}
      {liqDist < 15 && (
        <div className="mt-1 flex items-center gap-1 text-[9px] font-bold text-red-700">
          <span>⚠️</span> Liquidation {liqDist.toFixed(1)}% away
        </div>
      )}
    </div>
  );
}

function DailyPnlLedger({ trades, balance, dayStartBalance }: { trades: BTCFuturesTrade[]; balance: number; dayStartBalance: number }) {
  const today = new Date().toISOString().split("T")[0];
  const todayTrades = trades.filter(t => t.closedAt.startsWith(today));
  const dayPnl = todayTrades.reduce((s, t) => s + t.netPnl, 0);
  const dayFees = todayTrades.reduce((s, t) => s + t.fees, 0);
  const dayFunding = todayTrades.reduce((s, t) => s + t.fundingCosts, 0);

  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <span className="text-[10px] font-bold uppercase tracking-wider text-zinc-500">Daily Ledger (Today)</span>
        <span className="text-[10px] text-zinc-400">{todayTrades.length} trades</span>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <CompactMetric label="Day PnL" value={fmtUSD(dayPnl, { signed: true })} tone={dayPnl >= 0 ? "positive" : "negative"} />
        <CompactMetric label="Day Fees" value={fmtUSD(-dayFees)} tone="negative" />
        <CompactMetric label="Day Funding" value={fmtUSD(-dayFunding)} tone="negative" />
        <CompactMetric label="Day Net" value={fmtUSD(dayPnl - dayFees - dayFunding, { signed: true })} tone={dayPnl - dayFees - dayFunding >= 0 ? "positive" : "negative"} />
      </div>
      <div className="mt-3 border-t border-zinc-100 pt-2">
        <CompactMetric label="Day Change" value={fmtPct(((balance - dayStartBalance) / dayStartBalance) * 100, true)} tone={balance >= dayStartBalance ? "positive" : "negative"} />
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
    availableMargin,
    usedMargin,
    stats,
    quote,
    isReady,
    pauseEntries,
    disabledStrategies,
    togglePause,
    resetPaperAccount,
    clearTradeHistory,
    setDisabledStrategies,
    exportCSV,
    exportJSON,
    strategyStatuses,
  } = useBTCFuturesScalperEngine();

  const [showTrades, setShowTrades] = useState(false);
  const [showStrategies, setShowStrategies] = useState(false);

  const openLongs = positions.filter(p => p.side === "LONG").length;
  const openShorts = positions.filter(p => p.side === "SHORT").length;

  const liqRiskPositions = positions.filter(p => {
    const dist = p.side === "LONG"
      ? ((p.markPrice - p.liquidationPrice) / p.markPrice) * 100
      : ((p.liquidationPrice - p.markPrice) / p.markPrice) * 100;
    return dist < 10;
  });

  return (
    <div className="min-h-screen bg-zinc-50 p-4">
      {/* Header */}
      <div className="mb-4 rounded-2xl bg-white p-4 shadow-sm">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-lg font-bold text-zinc-900">BTC Futures Scalper</h1>
            <p className="text-[11px] text-zinc-500">
              {STRAT_DEFS_COUNT} strategies | {LEVERAGE}x leverage | ${INITIAL_BALANCE} paper | ${MIN_ABS_NET_PNL_USD} min PnL
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={togglePause}
              className={`rounded-lg px-3 py-1.5 text-[11px] font-semibold ${pauseEntries ? "bg-amber-100 text-amber-700" : "bg-emerald-100 text-emerald-700"}`}
            >
              {pauseEntries ? "⏸ PAUSED" : "▶ LIVE"}
            </button>
            <button
              onClick={() => setShowStrategies(!showStrategies)}
              className="rounded-lg bg-zinc-100 px-3 py-1.5 text-[11px] font-semibold text-zinc-700"
            >
              Strategies
            </button>
            <button
              onClick={() => setShowTrades(!showTrades)}
              className="rounded-lg bg-zinc-100 px-3 py-1.5 text-[11px] font-semibold text-zinc-700"
            >
              Trades ({trades.length})
            </button>
          </div>
        </div>

        {/* Quote Bar */}
        {quote && (
          <div className="mt-3 flex flex-wrap items-center gap-4 rounded-lg bg-zinc-100 p-2">
            <CompactMetric label="Mark" value={`$${quote.markPrice.toLocaleString()}`} tone="neutral" />
            <CompactMetric label="Last" value={`$${quote.lastPrice.toLocaleString()}`} tone="neutral" />
            <CompactMetric label="Index" value={`$${quote.indexPrice.toLocaleString()}`} tone="neutral" />
            <CompactMetric
              label="24h"
              value={fmtPct(quote.changePct24h, true)}
              tone={quote.changePct24h >= 0 ? "positive" : "negative"}
            />
            <CompactMetric
              label="Funding"
              value={fmtPct(quote.fundingRate * 100, true, 4)}
              tone={quote.fundingRate >= 0 ? "positive" : "negative"}
            />
          </div>
        )}
      </div>

      {/* Stats Grid */}
      <div className="mb-4 grid grid-cols-2 gap-3 md:grid-cols-4 lg:grid-cols-6">
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Balance" value={fmtUSD(balance)} tone="neutral" />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Equity" value={fmtUSD(equity, { signed: true })} tone={equity >= balance ? "positive" : "negative"} />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Available" value={fmtUSD(availableMargin)} tone={availableMargin > 100 ? "positive" : "neutral"} />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Used Margin" value={fmtUSD(usedMargin)} tone="neutral" />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Margin %" value={fmtPct(stats.marginUtilization)} tone={stats.marginUtilization > 80 ? "negative" : stats.marginUtilization > 50 ? "negative" : "neutral"} />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Leverage" value={stats.avgLeverage.toFixed(1)} suffix="x" tone="neutral" />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Win Rate" value={fmtPct(stats.winRate)} tone={stats.winRate > 50 ? "positive" : "neutral"} />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="PnL" value={fmtUSD(stats.netPnl, { signed: true })} tone={stats.netPnl >= 0 ? "positive" : "negative"} />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Trades" value={`${stats.totalTrades}`} tone="neutral" />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Open" value={`${stats.openPositions}/${stats.maxPositions}`} tone={stats.openPositions >= stats.maxPositions ? "negative" : "neutral"} />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Longs/Shorts" value={`${stats.longCount}/${stats.shortCount}`} tone="neutral" />
        </div>
        <div className="rounded-xl bg-white p-3 shadow-sm">
          <CompactMetric label="Liq Risk" value={`${liqRiskPositions.length}`} tone={liqRiskPositions.length > 0 ? "negative" : "neutral"} />
        </div>
      </div>

      {/* Liquidation Warning */}
      {liqRiskPositions.length > 0 && (
        <div className="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">
          <strong>⚠️ Liquidation Risk:</strong> {liqRiskPositions.length} position(s) near liquidation. Consider closing or adjusting.
        </div>
      )}

      {/* Positions Table */}
      {positions.length > 0 && (
        <div className="mb-4 overflow-x-auto rounded-2xl bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="bg-zinc-100 text-[10px] uppercase tracking-wider text-zinc-500">
                <th className="px-4 py-2">Position</th>
                <th className="px-4 py-2">Side</th>
                <th className="px-4 py-2 text-right">Contracts</th>
                <th className="px-4 py-2 text-right">Entry</th>
                <th className="px-4 py-2 text-right">Mark</th>
                <th className="px-4 py-2 text-right">Margin</th>
                <th className="px-4 py-2 text-right">PnL</th>
                <th className="px-4 py-2 text-right">Liq Price</th>
                <th className="px-4 py-2">Progress</th>
              </tr>
            </thead>
            <tbody>
              {positions.map((p) => {
                const pnlPositive = p.unrealizedPnl >= 0;
                const liqDist = p.side === "LONG"
                  ? ((p.markPrice - p.liquidationPrice) / p.markPrice) * 100
                  : ((p.liquidationPrice - p.markPrice) / p.markPrice) * 100;

                return (
                  <tr key={p.id} className="border-t border-zinc-100 hover:bg-zinc-50">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={`h-2 w-2 rounded-full ${pnlPositive ? "bg-emerald-500" : "bg-rose-500"}`} />
                        <div>
                          <div className="font-semibold text-zinc-900">{p.strategyName}</div>
                          <div className="text-[10px] text-zinc-500">{p.leverage}x {p.marginMode}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3"><SideBadge side={p.side} /></td>
                    <td className="px-4 py-3 text-right font-mono text-xs">{fmtContracts(p.contracts)}</td>
                    <td className="px-4 py-3 text-right font-mono text-xs">${p.entryPrice.toLocaleString()}</td>
                    <td className="px-4 py-3 text-right font-mono text-xs">${p.markPrice.toLocaleString()}</td>
                    <td className="px-4 py-3 text-right font-mono text-xs">{fmtUSD(p.marginUsed)}</td>
                    <td className="px-4 py-3 text-right">
                      <div className={`font-mono font-bold ${pnlPositive ? "text-emerald-600" : "text-rose-600"}`}>
                        {fmtUSD(p.unrealizedPnl, { signed: true })}
                      </div>
                      <div className="text-[10px] text-zinc-500">{fmtPct(p.returnPct, true)}</div>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className={`font-mono text-xs ${liqDist < 10 ? "font-bold text-red-600" : "text-zinc-600"}`}>
                        ${p.liquidationPrice.toLocaleString()}
                      </div>
                      <div className="text-[10px] text-zinc-500">{liqDist.toFixed(1)}% away</div>
                    </td>
                    <td className="px-4 py-3">
                      <PositionProgressBar
                        returnPct={p.returnPct}
                        unrealizedPnl={p.unrealizedPnl}
                        entryPrice={p.entryPrice}
                        tpPrice={p.tpPrice}
                        slPrice={p.slPrice}
                        liquidationPrice={p.liquidationPrice}
                        currentPrice={p.markPrice}
                        contracts={p.contracts}
                        side={p.side}
                      />
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Trades History */}
      {showTrades && trades.length > 0 && (
        <div className="mb-4 overflow-x-auto rounded-2xl bg-white shadow-sm">
          <div className="flex items-center justify-between border-b border-zinc-100 p-4">
            <h3 className="font-bold text-zinc-900">Trade History ({trades.length})</h3>
            <div className="flex gap-2">
              <button
                onClick={() => navigator.clipboard.writeText(exportCSV())}
                className="rounded-lg bg-zinc-100 px-3 py-1.5 text-[11px] font-semibold text-zinc-700 hover:bg-zinc-200"
              >
                Copy CSV
              </button>
              <button
                onClick={() => navigator.clipboard.writeText(exportJSON())}
                className="rounded-lg bg-zinc-100 px-3 py-1.5 text-[11px] font-semibold text-zinc-700 hover:bg-zinc-200"
              >
                Copy JSON
              </button>
              <button
                onClick={clearTradeHistory}
                className="rounded-lg bg-rose-100 px-3 py-1.5 text-[11px] font-semibold text-rose-700 hover:bg-rose-200"
              >
                Clear
              </button>
            </div>
          </div>
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="bg-zinc-100 text-[10px] uppercase tracking-wider text-zinc-500">
                <th className="px-4 py-2">Time</th>
                <th className="px-4 py-2">Strategy</th>
                <th className="px-4 py-2">Side</th>
                <th className="px-4 py-2 text-right">Contracts</th>
                <th className="px-4 py-2 text-right">Entry → Exit</th>
                <th className="px-4 py-2 text-right">Gross</th>
                <th className="px-4 py-2 text-right">Fees</th>
                <th className="px-4 py-2 text-right">Net PnL</th>
                <th className="px-4 py-2 text-right">Return %</th>
                <th className="px-4 py-2">Reason</th>
              </tr>
            </thead>
            <tbody>
              {trades.slice().reverse().slice(0, 50).map((t) => (
                <tr key={t.id} className="border-t border-zinc-100 hover:bg-zinc-50">
                  <td className="px-4 py-2 text-zinc-500">{new Date(t.closedAt).toLocaleTimeString()}</td>
                  <td className="px-4 py-2 font-medium text-zinc-900">{t.strategyName}</td>
                  <td className="px-4 py-2"><SideBadge side={t.side} /></td>
                  <td className="px-4 py-2 text-right font-mono">{fmtContracts(t.contracts)}</td>
                  <td className="px-4 py-2 text-right font-mono">${t.entryPrice.toLocaleString()} → ${t.exitPrice.toLocaleString()}</td>
                  <td className="px-4 py-2 text-right font-mono">{fmtUSD(t.realizedPnl, { signed: true })}</td>
                  <td className="px-4 py-2 text-right font-mono text-rose-600">-{fmtUSD(t.fees)}</td>
                  <td className={`px-4 py-2 text-right font-mono font-bold ${t.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                    {fmtUSD(t.netPnl, { signed: true })}
                  </td>
                  <td className={`px-4 py-2 text-right font-mono ${t.netPnlPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                    {fmtPct(t.netPnlPct, true)}
                  </td>
                  <td className="px-4 py-2">
                    <BadgePill
                      label={t.exitReason}
                      tone={t.exitReason === "TP" ? "positive" : t.exitReason === "SL" || t.exitReason === "LIQUIDATION_RISK" ? "negative" : "neutral"}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Strategy Status */}
      {showStrategies && (
        <div className="mb-4 rounded-2xl bg-white p-4 shadow-sm">
          <h3 className="mb-3 font-bold text-zinc-900">Strategy Status ({STRAT_DEFS_COUNT} total)</h3>
          <div className="grid grid-cols-2 gap-2 md:grid-cols-4 lg:grid-cols-6">
            {strategyStatuses.map((s) => (
              <button
                key={s.id}
                onClick={() => {
                  const newDisabled = disabledStrategies.includes(s.id)
                    ? disabledStrategies.filter(id => id !== s.id)
                    : [...disabledStrategies, s.id];
                  setDisabledStrategies(newDisabled);
                }}
                className={`rounded-lg border p-2 text-left text-[11px] transition-all ${
                  s.disabled
                    ? "border-zinc-200 bg-zinc-100 opacity-50"
                    : s.status === "OPEN"
                    ? "border-emerald-200 bg-emerald-50"
                    : s.status === "COOLING"
                    ? "border-amber-200 bg-amber-50"
                    : "border-zinc-200 bg-white hover:border-zinc-300"
                }`}
              >
                <div className="flex items-center justify-between">
                  <span className="font-semibold">#{s.id}</span>
                  {s.openCount > 0 && <BadgePill label={`${s.openCount} open`} tone="positive" />}
                </div>
                <div className="mt-1 truncate text-zinc-600">{s.name}</div>
                <div className="text-zinc-400">{s.category}</div>
                {s.status === "COOLING" && <div className="text-amber-600">In cooldown</div>}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Daily Ledger */}
      <div className="mb-4">
        <DailyPnlLedger trades={trades} balance={balance} dayStartBalance={INITIAL_BALANCE} />
      </div>

      {/* Reset */}
      <div className="flex justify-end">
        <button
          onClick={resetPaperAccount}
          className="rounded-lg bg-rose-100 px-4 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-200"
        >
          Reset Paper Account
        </button>
      </div>
    </div>
  );
}

// Constants for display
const INITIAL_BALANCE = 1000;
const MIN_ABS_NET_PNL_USD = 2;
const LEVERAGE = 25;

// Strategy defs reference (matches hook)
const STRAT_DEFS_COUNT = 130;
