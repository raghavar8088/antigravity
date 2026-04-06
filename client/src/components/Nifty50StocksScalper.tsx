"use client";

import { useState } from "react";
import useNiftyStocks, {
  NiftyStockPosition,
  NiftyStockStrategyStatus,
  NiftyStockTrade,
} from "@/hooks/useNiftyStocks";
import { formatShortDate, formatShortTime } from "@/lib/time";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const INITIAL_STOCKS_BALANCE = 1_000_000;

function fmtINR(value: number, options: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = options;
  const abs = Math.abs(value).toLocaleString("en-IN", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });

  if (signed) {
    return `${value >= 0 ? "+" : "-"}Rs.${abs}`;
  }
  return `Rs.${abs}`;
}

function fmtPct(value: number, signed = false, decimals = 2) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

function formatHold(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "0m 0s";
  }

  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);

  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m ${secs}s`;
}

function SummaryCard({ label, value, detail, accent = "" }: {
  label: string;
  value: string;
  detail?: string;
  accent?: string;
}) {
  return (
    <div className="summary-card flex min-h-[116px] flex-col justify-between gap-3">
      <div className="summary-label">{label}</div>
      <div className={`summary-value ${accent}`}>{value}</div>
      <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
        {detail ?? ""}
      </div>
    </div>
  );
}

function MetricCard({ label, value, detail, accent = "" }: {
  label: string;
  value: string;
  detail?: string;
  accent?: string;
}) {
  return (
    <div className="metric-card flex min-h-[108px] flex-col justify-between gap-3">
      <div className="metric-label">{label}</div>
      <div className={`metric-value ${accent}`}>{value}</div>
      <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
        {detail ?? ""}
      </div>
    </div>
  );
}

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  const positive = side === "LONG";
  return (
    <span
      className="rounded-full px-2 py-1 text-[10px] font-medium uppercase tracking-[0.12em]"
      style={{
        background: positive ? "var(--green-dim)" : "var(--red-dim)",
        color: positive ? "var(--green)" : "var(--red)",
      }}
    >
      {side}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const tones: Record<string, { bg: string; color: string }> = {
    READY: { bg: "rgba(26, 115, 232, 0.10)", color: "var(--blue)" },
    IN_POSITION: { bg: "rgba(24, 128, 56, 0.10)", color: "var(--green)" },
    COOLING: { bg: "rgba(245, 124, 0, 0.10)", color: "var(--amber)" },
  };
  const tone = tones[status] ?? { bg: "var(--surface-3)", color: "var(--text-secondary)" };
  return (
    <span
      className="rounded-full px-2 py-1 text-[10px] font-medium uppercase tracking-[0.12em]"
      style={{ background: tone.bg, color: tone.color }}
    >
      {status.replace("_", " ")}
    </span>
  );
}

function ReasonBadge({ reason }: { reason: string }) {
  const tones: Record<string, { bg: string; color: string }> = {
    TP: { bg: "var(--green-dim)", color: "var(--green)" },
    SL: { bg: "var(--red-dim)", color: "var(--red)" },
    MANUAL: { bg: "var(--surface-3)", color: "var(--text-secondary)" },
  };
  const tone = tones[reason] ?? tones.MANUAL;
  return (
    <span
      className="rounded-full px-2 py-1 text-[10px] font-medium uppercase tracking-[0.12em]"
      style={{ background: tone.bg, color: tone.color }}
    >
      {reason}
    </span>
  );
}

function PositionProgress({ position }: { position: NiftyStockPosition }) {
  const totalRange = position.side === "LONG"
    ? position.takeProfit - position.stopLoss
    : position.stopLoss - position.takeProfit;
  const progress = totalRange > 0
    ? position.side === "LONG"
      ? ((position.currentPrice - position.stopLoss) / totalRange) * 100
      : ((position.stopLoss - position.currentPrice) / totalRange) * 100
    : 50;
  const clamped = Math.max(0, Math.min(100, progress));

  return (
    <div className="w-28">
      <div className="h-2 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
        <div
          className="h-full rounded-full"
          style={{
            width: `${clamped}%`,
            background: position.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)",
            transition: "width 0.3s ease",
          }}
        />
      </div>
    </div>
  );
}

function PositionsPanel({ positions }: { positions: NiftyStockPosition[] }) {
  if (positions.length === 0) {
    return (
      <div
        className="flex min-h-[220px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
        style={{
          color: "var(--text-secondary)",
          borderColor: "var(--border)",
          background: "var(--surface-2)",
        }}
      >
        No NIFTY stock positions are open yet. The synthetic stocks engine is waiting for fresh setups.
      </div>
    );
  }

  const totalUnrealized = positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="h-2 w-2 rounded-full" style={{ background: "var(--green)" }} />
          <span className="text-xs font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
            {positions.length} live stock positions
          </span>
        </div>
        <span
          className="rounded-full border px-3 py-1 text-xs font-medium"
          style={{
            background: totalUnrealized >= 0 ? "var(--green-dim)" : "var(--red-dim)",
            color: totalUnrealized >= 0 ? "var(--green)" : "var(--red)",
            borderColor: totalUnrealized >= 0 ? "rgba(24, 128, 56, 0.14)" : "rgba(217, 48, 37, 0.14)",
          }}
        >
          Unrealized {fmtINR(totalUnrealized, { signed: true })}
        </span>
      </div>

      <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
        <table className="w-full min-w-[980px] text-left text-sm">
          <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            <tr className="text-[11px] uppercase tracking-[0.12em]">
              <th className="px-4 py-3 font-medium">Position</th>
              <th className="px-4 py-3 font-medium">Entry</th>
              <th className="px-4 py-3 font-medium">Mark</th>
              <th className="px-4 py-3 font-medium">Quantity</th>
              <th className="px-4 py-3 font-medium">Risk</th>
              <th className="px-4 py-3 font-medium">Opened</th>
              <th className="px-4 py-3 font-medium">PnL</th>
              <th className="px-4 py-3 font-medium text-right">Progress</th>
            </tr>
          </thead>
          <tbody>
            {positions.map((position) => (
              <tr key={position.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="px-4 py-3">
                  <div className="flex flex-col gap-1">
                    <div className="flex items-center gap-2">
                      <SideBadge side={position.side} />
                      <span className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
                        {position.strategyName}
                      </span>
                    </div>
                    <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                      {position.symbol} | {position.id}
                    </div>
                  </div>
                </td>
                <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                  {fmtINR(position.entryPrice)}
                </td>
                <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                  {fmtINR(position.currentPrice)}
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                    {fmtPct(position.returnPct, true)}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <div className="font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                    {position.quantity.toFixed(4)} units
                  </div>
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                    Notional {fmtINR(position.notional)}
                  </div>
                </td>
                <td className="px-4 py-3 text-xs">
                  <div className="font-mono" style={{ color: "var(--red)" }}>
                    SL {fmtINR(position.stopLoss)}
                  </div>
                  <div className="font-mono" style={{ color: "var(--green)" }}>
                    TP {fmtINR(position.takeProfit)}
                  </div>
                </td>
                <td className="px-4 py-3 text-xs">
                  <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                    {formatShortTime(position.entryTime)}
                  </div>
                  <div style={{ color: "var(--text-secondary)" }}>
                    {formatShortDate(position.entryTime)}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <div className="font-mono text-sm font-semibold" style={{ color: position.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                    {fmtINR(position.unrealizedPnl, { signed: true })}
                  </div>
                </td>
                <td className="px-4 py-3 text-right">
                  <div className="ml-auto">
                    <PositionProgress position={position} />
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

function StrategyPanel({ strategies }: { strategies: NiftyStockStrategyStatus[] }) {
  const orderedStrategies = [...strategies].sort((left, right) => {
    if (right.totalPnl !== left.totalPnl) {
      return right.totalPnl - left.totalPnl;
    }
    return right.score - left.score;
  });

  return (
    <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
      <table className="w-full min-w-[940px] text-left text-sm">
        <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
          <tr className="text-[11px] uppercase tracking-[0.12em]">
            <th className="px-4 py-3 font-medium">Strategy</th>
            <th className="px-4 py-3 font-medium">Category</th>
            <th className="px-4 py-3 font-medium">Bias</th>
            <th className="px-4 py-3 font-medium">Status</th>
            <th className="px-4 py-3 font-medium">Score</th>
            <th className="px-4 py-3 font-medium">Trades</th>
            <th className="px-4 py-3 font-medium">Win %</th>
            <th className="px-4 py-3 font-medium">PnL</th>
            <th className="px-4 py-3 font-medium">Regime</th>
          </tr>
        </thead>
        <tbody>
          {orderedStrategies.map((strategy) => (
            <tr key={strategy.strategyId} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
              <td className="px-4 py-3">
                <div className="font-medium" style={{ color: "var(--text-primary)" }}>{strategy.name}</div>
                <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                  Allocation {fmtINR(strategy.allocationInr)}
                </div>
              </td>
              <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                {strategy.category}
              </td>
              <td className="px-4 py-3 text-xs font-medium" style={{ color: "var(--text-primary)" }}>
                {strategy.bias}
              </td>
              <td className="px-4 py-3">
                <StatusBadge status={strategy.status} />
              </td>
              <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                {strategy.score.toFixed(1)}
              </td>
              <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                {strategy.totalTrades}
              </td>
              <td className="px-4 py-3 font-mono text-xs" style={{ color: strategy.winRate >= 50 ? "var(--green)" : "var(--text-primary)" }}>
                {strategy.totalTrades > 0 ? `${strategy.winRate.toFixed(1)}%` : "-"}
              </td>
              <td className="px-4 py-3 font-mono text-xs" style={{ color: strategy.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                {fmtINR(strategy.totalPnl, { signed: true })}
              </td>
              <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                {strategy.regime || "-"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TradesPanel({ trades }: { trades: NiftyStockTrade[] }) {
  const [showAll, setShowAll] = useState(false);
  const visibleTrades = showAll ? trades : trades.slice(0, 10);

  if (trades.length === 0) {
    return (
      <div
        className="flex min-h-[220px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
        style={{
          color: "var(--text-secondary)",
          borderColor: "var(--border)",
          background: "var(--surface-2)",
        }}
      >
        No completed NIFTY stock trades yet.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
        <table className="w-full min-w-[980px] text-left text-sm">
          <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            <tr className="text-[11px] uppercase tracking-[0.12em]">
              <th className="px-4 py-3 font-medium">Time</th>
              <th className="px-4 py-3 font-medium">Strategy</th>
              <th className="px-4 py-3 font-medium">Side</th>
              <th className="px-4 py-3 font-medium">Entry</th>
              <th className="px-4 py-3 font-medium">Exit</th>
              <th className="px-4 py-3 font-medium">Hold</th>
              <th className="px-4 py-3 font-medium">Reason</th>
              <th className="px-4 py-3 font-medium text-right">PnL</th>
            </tr>
          </thead>
          <tbody>
            {visibleTrades.map((trade) => (
              <tr key={trade.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="px-4 py-3 text-xs">
                  <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                    {formatShortTime(trade.exitTime)}
                  </div>
                  <div style={{ color: "var(--text-secondary)" }}>
                    {formatShortDate(trade.exitTime)}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <div className="font-medium" style={{ color: "var(--text-primary)" }}>{trade.strategyName}</div>
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{trade.id}</div>
                </td>
                <td className="px-4 py-3">
                  <SideBadge side={trade.side} />
                </td>
                <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                  {fmtINR(trade.entryPrice)}
                </td>
                <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                  {fmtINR(trade.exitPrice)}
                </td>
                <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                  {formatHold(trade.holdSeconds)}
                </td>
                <td className="px-4 py-3">
                  <ReasonBadge reason={trade.exitReason} />
                </td>
                <td className="px-4 py-3 text-right font-mono text-sm font-semibold" style={{ color: trade.netPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                  {fmtINR(trade.netPnl, { signed: true })}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {trades.length > 10 ? (
        <button type="button" onClick={() => setShowAll((prev) => !prev)} className="btn-primary w-full">
          {showAll ? "Show fewer stock trades" : `Show all ${trades.length} stock trades`}
        </button>
      ) : null}
    </div>
  );
}

export default function Nifty50StocksScalper() {
  const [refreshKey, setRefreshKey] = useState(0);
  const [isResetting, setIsResetting] = useState(false);
  const [isClearing, setIsClearing] = useState(false);
  const { positions, trades, strategies, stats, clearAll } = useNiftyStocks(refreshKey);

  const equity = stats?.equity ?? INITIAL_STOCKS_BALANCE;
  const cashBalance = stats?.balance ?? INITIAL_STOCKS_BALANCE;
  const sessionPnl = equity - INITIAL_STOCKS_BALANCE;
  const unrealizedPnl = stats?.unrealizedPnl ?? positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const realizedPnl = stats?.totalPnl ?? 0;
  const lastPrice = stats?.lastPrice ?? 0;
  const openPositions = stats?.openPositions ?? positions.length;
  const totalTrades = stats?.totalTrades ?? trades.length;
  const winRate = stats?.winRate ?? 0;
  const topStrategy = [...strategies].sort((left, right) => right.totalPnl - left.totalPnl)[0] ?? null;

  const handleReset = async () => {
    if (!confirm("Reset the NIFTY 50 stocks paper account to Rs.1,000,000? All history and open positions will be cleared.")) {
      return;
    }
    setIsResetting(true);
    try {
      const response = await fetch(`${API_URL}/api/nifty-stocks/reset`, { method: "POST" });
      if (!response.ok) {
        throw new Error("reset failed");
      }
      clearAll();
      setRefreshKey((prev) => prev + 1);
    } catch {
      window.alert("NIFTY 50 stocks account reset failed. Check engine connectivity.");
    } finally {
      setIsResetting(false);
    }
  };

  const handleClearHistory = async () => {
    if (!confirm("Clear completed NIFTY stock trades and strategy stats? Open positions and balance will be kept.")) {
      return;
    }
    setIsClearing(true);
    try {
      const response = await fetch(`${API_URL}/api/nifty-stocks/clear-history`, { method: "POST" });
      if (!response.ok) {
        throw new Error("clear failed");
      }
      clearAll();
      setRefreshKey((prev) => prev + 1);
    } catch {
      window.alert("NIFTY 50 stock trade history could not be cleared.");
    } finally {
      setIsClearing(false);
    }
  };

  return (
    <div className="space-y-6">
      <section className="glass-panel overflow-hidden">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.1fr)_420px]">
          <div className="px-6 py-7 md:px-8">
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <span className="pill-green">LIVE</span>
              <span
                className="rounded-full px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]"
                style={{ background: "rgba(26, 115, 232, 0.10)", color: "var(--blue)" }}
              >
                NIFTY 50 STOCKS
              </span>
            </div>
            <div className="max-w-3xl">
              <div className="text-xs font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
                NIFTY 50 STOCK EQUITY
              </div>
              <div className={`mt-3 text-[clamp(2.65rem,5vw,3.5rem)] font-semibold leading-none tracking-tight ${equity >= INITIAL_STOCKS_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                {fmtINR(equity)}
              </div>
              <div className="mt-4 max-w-2xl text-sm leading-6" style={{ color: "var(--text-secondary)" }}>
                Separate paper scalper for synthetic NIFTY 50 stock movement, using the BTC equity desk as the reference design and a dedicated Rs.1,000,000 account.
              </div>
            </div>
          </div>

          <div
            className="flex flex-col justify-between gap-5 border-t px-6 py-7 md:px-8 lg:border-l lg:border-t-0"
            style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}
          >
            <div className="space-y-3">
              <div className="text-xs font-semibold uppercase tracking-[0.16em]" style={{ color: "var(--text-secondary)" }}>
                Live pulse
              </div>
              <div className="grid grid-cols-2 gap-3">
                <MetricCard
                  label="Synthetic NIFTY"
                  value={lastPrice > 0 ? fmtINR(lastPrice) : "Waiting"}
                  detail={lastPrice > 0 ? "BTC-mapped reference feed" : "Engine warming up"}
                />
                <MetricCard
                  label="Open Positions"
                  value={`${openPositions}`}
                  detail={`${stats?.activeStrategies ?? strategies.length} active strategies`}
                />
              </div>
            </div>

            <div className="flex flex-wrap gap-3">
              <button type="button" onClick={handleClearHistory} disabled={isClearing || isResetting} className="btn-primary flex-1">
                {isClearing ? "Clearing..." : "Clear Stock Trades"}
              </button>
              <button type="button" onClick={handleReset} disabled={isResetting || isClearing} className="btn-gold flex-1">
                {isResetting ? "Resetting..." : "Reset Stock Account"}
              </button>
            </div>
          </div>
        </div>
      </section>

      <section className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-5">
        <SummaryCard
          label="Cash Balance"
          value={fmtINR(cashBalance)}
          detail="Unallocated stock capital"
        />
        <SummaryCard
          label="Session PnL"
          value={fmtINR(sessionPnl, { signed: true })}
          detail="Vs starting stock capital"
          accent={sessionPnl >= 0 ? "profit-positive" : "profit-negative"}
        />
        <SummaryCard
          label="Realized PnL"
          value={fmtINR(realizedPnl, { signed: true })}
          detail={`${totalTrades} completed stock trades`}
          accent={realizedPnl >= 0 ? "profit-positive" : "profit-negative"}
        />
        <SummaryCard
          label="Unrealized PnL"
          value={fmtINR(unrealizedPnl, { signed: true })}
          detail={`${openPositions} open stock positions`}
          accent={unrealizedPnl >= 0 ? "profit-positive" : "profit-negative"}
        />
        <SummaryCard
          label="Win Rate"
          value={totalTrades > 0 ? `${winRate.toFixed(1)}%` : "-"}
          detail={topStrategy ? `Leader: ${topStrategy.name}` : "Awaiting completed trades"}
          accent={winRate >= 50 ? "profit-positive" : ""}
        />
      </section>

      <section className="grid gap-6 xl:grid-cols-[minmax(0,1.25fr)_minmax(380px,0.75fr)]">
        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
                Running Stock Positions
              </h2>
              <div className="mt-2 text-sm" style={{ color: "var(--text-secondary)" }}>
                Live NIFTY 50 stock positions with stop-loss, take-profit, and mark-to-market tracking.
              </div>
            </div>
          </div>
          <PositionsPanel positions={positions} />
        </div>

        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="mb-5">
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Strategy Roster
            </h2>
            <div className="mt-2 text-sm" style={{ color: "var(--text-secondary)" }}>
              Eight NIFTY stock scalpers modeled after the BTC equity desk categories.
            </div>
          </div>
          <StrategyPanel strategies={strategies} />
        </div>
      </section>

      <section className="glass-panel px-5 py-6 md:px-6">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Stock Trade Ledger
            </h2>
            <div className="mt-2 text-sm" style={{ color: "var(--text-secondary)" }}>
              Completed NIFTY 50 stock trades from the separate paper account.
            </div>
          </div>
          <div className="text-xs font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
            {totalTrades} trades recorded
          </div>
        </div>
        <TradesPanel trades={trades} />
      </section>

      <section className="glass-panel px-5 py-5 text-sm leading-6" style={{ color: "var(--text-secondary)" }}>
        NIFTY 50 stocks paper account | Synthetic NIFTY feed mapped from live BTC movement | Dedicated stock strategy roster | Separate Rs.1,000,000 capital pool
      </section>
    </div>
  );
}
