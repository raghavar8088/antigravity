"use client";

import { useState } from "react";
import Nifty50MarketHero from "@/components/Nifty50MarketHero";
import useNiftyMarket from "@/hooks/useNiftyMarket";
import type { OptionPosition, OptionStats, OptionStrategyStatus, OptionTrade } from "@/hooks/useNiftyOptions";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const INITIAL_BALANCE = 1_000_000;

function fmtINR(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-IN", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  return signed ? `${value >= 0 ? "+" : "-"}₹${abs}` : `₹${abs}`;
}

function fmtPct(value: number, signed = false) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(2)}%`;
}

function fmtTime(value: string) {
  try {
    return new Date(value).toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
  } catch {
    return "--";
  }
}

function SummaryCard({ label, value, detail, accent = "" }: { label: string; value: string; detail?: string; accent?: string }) {
  return (
    <div className="summary-card flex min-h-[110px] flex-col justify-between gap-2">
      <div className="summary-label">{label}</div>
      <div className={`summary-value ${accent}`}>{value}</div>
      {detail ? <div className="text-xs" style={{ color: "var(--text-secondary)" }}>{detail}</div> : null}
    </div>
  );
}

function TypeBadge({ type }: { type: string }) {
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${type === "CALL" ? "border-rose-500/25 bg-rose-500/10 text-rose-600" : "border-emerald-500/25 bg-emerald-500/10 text-emerald-600"}`}>
      SHORT {type}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const cls = status === "IN_POSITION" ? "border-blue-200 bg-blue-50 text-blue-700" : status === "READY" ? "border-emerald-200 bg-emerald-50 text-emerald-700" : status === "COOLING" ? "border-amber-200 bg-amber-50 text-amber-700" : "border-zinc-200 bg-zinc-50 text-zinc-600";
  return <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${cls}`}>{status.replace("_", " ")}</span>;
}

type Props = {
  actionsEnabled?: boolean;
  positions: OptionPosition[];
  trades: OptionTrade[];
  strategies: OptionStrategyStatus[];
  stats: OptionStats | null;
  clearAll: () => void;
  barCount: number;
  enginePrice: number;
  onRefresh?: () => void;
};

export default function Nifty50OptionSellingScalper({
  actionsEnabled = false,
  positions,
  trades,
  strategies,
  stats,
  clearAll,
  barCount,
  enginePrice,
  onRefresh,
}: Props) {
  const market = useNiftyMarket();
  const [isResetting, setIsResetting] = useState(false);
  const actionButtonTitle = actionsEnabled ? "Action buttons are enabled." : "Set Action to Yes to enable reset and clear buttons.";
  const resolvedStats: OptionStats = stats ?? {
    balance: INITIAL_BALANCE,
    equity: INITIAL_BALANCE,
    totalTrades: 0,
    openPositions: positions.length,
    totalWins: 0,
    totalLosses: 0,
    winRate: 0,
    totalPnl: 0,
    totalPremiumSpent: 0,
    unrealizedPnl: 0,
  };
  const equity = resolvedStats.equity;
  const sessionPnl = equity - INITIAL_BALANCE;
  const bestStrategy = [...strategies].sort((a, b) => b.totalPnl - a.totalPnl)[0] ?? null;

  const handleReset = async () => {
    if (!actionsEnabled) return;
    if (!confirm("Reset the NIFTY option selling paper account to ₹10,00,000? All history will be cleared.")) return;
    setIsResetting(true);
    clearAll();
    try {
      const response = await fetch(`${API_URL}/api/nifty-options-selling/reset`, { method: "POST" });
      if (!response.ok) {
        throw new Error("reset failed");
      }
      onRefresh?.();
    } catch {
      window.alert("NIFTY option selling reset failed. Check engine connectivity.");
    } finally {
      setIsResetting(false);
    }
  };
  const displayedEnginePrice = enginePrice > 0 ? enginePrice : market.price;

  return (
    <div className="space-y-6">
      <Nifty50MarketHero
        market={market}
        subtitle="Live NIFTY 50 index feed powering the separate NIFTY option-selling workspace."
      />

      <section className="glass-panel overflow-hidden">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.1fr)_420px]">
          <div className="px-6 py-7 md:px-8">
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <span className="pill-green">LIVE</span>
              <span className="rounded-full px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ background: "rgba(26,115,232,0.10)", color: "var(--blue)" }}>
                NIFTY OPTION SELLING
              </span>
              <span className="rounded-full border border-orange-200 bg-orange-50 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] text-orange-700">
                Separate Account
              </span>
            </div>
            <div className="text-xs font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              NIFTY 50 OPTION SELLING EQUITY
            </div>
            <div className={`mt-3 text-[clamp(2.65rem,5vw,3.5rem)] font-semibold leading-none tracking-tight ${equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
              {fmtINR(equity)}
            </div>
            <div className="mt-3 text-sm leading-6" style={{ color: "var(--text-secondary)" }}>
              Autonomous premium-selling workspace for NIFTY 50. The engine shorts calls in bearish/range conditions and shorts puts in bullish/range conditions, then manages decay-based exits with TP, SL, and timed profit locking.
            </div>
            <div className="mt-4 flex flex-wrap gap-2">
              <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 text-[11px] font-medium text-zinc-600">
                Feed {displayedEnginePrice > 0 ? `₹${displayedEnginePrice.toFixed(0)}` : "Connecting..."}
              </span>
              <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 text-[11px] font-medium text-zinc-600">
                {barCount >= 12 ? `${barCount} bars` : `Warming ${barCount}/12`}
              </span>
              <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 text-[11px] font-medium text-zinc-600">
                {positions.length} open shorts
              </span>
            </div>
          </div>
          <div className="flex flex-col justify-between gap-5 border-t px-6 py-7 md:px-8 lg:border-l lg:border-t-0" style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}>
            <div className="grid grid-cols-2 gap-3">
              <SummaryCard label="Session PnL" value={fmtINR(sessionPnl, { signed: true })} accent={sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
              <SummaryCard label="Win Rate" value={resolvedStats.totalTrades > 0 ? fmtPct(resolvedStats.winRate) : "--"} detail={`${resolvedStats.totalTrades} trades`} />
              <SummaryCard label="Open Positions" value={`${resolvedStats.openPositions}`} detail="Short premium positions" />
              <SummaryCard label="Best Strategy" value={bestStrategy ? bestStrategy.name.replace(/_/g, " ") : "Waiting"} detail={bestStrategy ? fmtINR(bestStrategy.totalPnl, { signed: true }) : "No closed trades yet"} />
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <button type="button" disabled={!actionsEnabled || isResetting} title={actionButtonTitle} className="btn-primary text-sm" onClick={async () => {
                if (!actionsEnabled) return;
                if (!confirm("Clear completed NIFTY option selling trades and strategy stats? Open positions and balance will be kept.")) return;
                clearAll();
                try {
                  const response = await fetch(`${API_URL}/api/nifty-options-selling/clear-history`, { method: "POST" });
                  if (!response.ok) {
                    throw new Error("clear history failed");
                  }
                  onRefresh?.();
                } catch {
                  window.alert("Clearing NIFTY option selling history failed. Check engine connectivity.");
                }
              }}>
                Clear NIFTY Selling Trades
              </button>
              <button type="button" disabled={!actionsEnabled || isResetting} title={actionButtonTitle} className="btn-danger text-sm" onClick={handleReset}>
                {isResetting ? "Resetting..." : "Reset Selling Account"}
              </button>
            </div>
          </div>
        </div>
      </section>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-6">
        <SummaryCard label="Cash Balance" value={fmtINR(resolvedStats.balance)} />
        <SummaryCard label="Realized PnL" value={fmtINR(resolvedStats.totalPnl, { signed: true })} accent={resolvedStats.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Unrealized" value={fmtINR(resolvedStats.unrealizedPnl, { signed: true })} accent={resolvedStats.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Wins / Losses" value={`${resolvedStats.totalWins}/${resolvedStats.totalLosses}`} />
        <SummaryCard label="Strategies" value={`${strategies.length}`} detail={`${strategies.filter((item) => item.status === "READY" || item.status === "IN_POSITION").length} live`} />
        <SummaryCard label="Premium Engine" value="Decay First" detail="Short calls and short puts" />
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <div className="space-y-6">
          <div className="glass-panel p-6">
            <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Open Short Positions</div>
            {positions.length === 0 ? (
              <div className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm" style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}>
                No short option positions are open yet. The engine is waiting for NIFTY premium-selling setups.
              </div>
            ) : (
              <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
                <table className="w-full min-w-[900px] text-left text-sm">
                  <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                    <tr className="text-[11px] uppercase tracking-[0.12em]">
                      <th className="px-4 py-3 font-medium">Position</th>
                      <th className="px-4 py-3 font-medium">Strike</th>
                      <th className="px-4 py-3 font-medium">Premium</th>
                      <th className="px-4 py-3 font-medium">Opened</th>
                      <th className="px-4 py-3 font-medium text-right">PnL</th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((position) => (
                      <tr key={position.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <TypeBadge type={position.optionType} />
                            <div className="text-xs" style={{ color: "var(--text-secondary)" }}>{position.strategyName.replace(/_/g, " ")}</div>
                          </div>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>{position.strike}</td>
                        <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                          ₹{position.entryPremium.toFixed(2)} to ₹{position.currentPremium.toFixed(2)}
                        </td>
                        <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>{fmtTime(position.entryTime)}</td>
                        <td className="px-4 py-3 text-right">
                          <div className="font-mono text-sm font-semibold" style={{ color: position.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                            {fmtINR(position.unrealizedPnl, { signed: true })}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="glass-panel p-6">
            <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Trade Ledger</div>
            {trades.length === 0 ? (
              <div className="flex min-h-[160px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm" style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}>
                No closed trades yet. Completed short-option trades will appear here.
              </div>
            ) : (
              <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
                <table className="w-full min-w-[980px] text-left text-sm">
                  <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                    <tr className="text-[11px] uppercase tracking-[0.12em]">
                      <th className="px-4 py-3 font-medium">Time</th>
                      <th className="px-4 py-3 font-medium">Strategy</th>
                      <th className="px-4 py-3 font-medium">Type</th>
                      <th className="px-4 py-3 font-medium">Premium</th>
                      <th className="px-4 py-3 font-medium">Exit</th>
                      <th className="px-4 py-3 font-medium text-right">PnL</th>
                    </tr>
                  </thead>
                  <tbody>
                    {trades.slice(0, 20).map((trade) => (
                      <tr key={trade.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                        <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>{fmtTime(trade.exitTime)}</td>
                        <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{trade.strategyName.replace(/_/g, " ")}</td>
                        <td className="px-4 py-3"><TypeBadge type={trade.optionType} /></td>
                        <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>₹{trade.entryPremium.toFixed(2)} to ₹{trade.exitPremium.toFixed(2)}</td>
                        <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{trade.exitReason}</td>
                        <td className="px-4 py-3 text-right">
                          <div className="font-mono text-sm font-semibold" style={{ color: trade.netPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                            {fmtINR(trade.netPnl, { signed: true })}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>

        <div className="glass-panel p-6">
          <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Strategy Roster</div>
          <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Strategy</th>
                  <th className="px-4 py-3 font-medium">Type</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Score</th>
                  <th className="px-4 py-3 font-medium text-right">PnL</th>
                </tr>
              </thead>
              <tbody>
                {strategies.map((strategy) => (
                  <tr key={strategy.strategyId} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3">
                      <div className="font-medium text-xs" style={{ color: "var(--text-primary)" }}>{strategy.name.replace(/_/g, " ")}</div>
                      <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{strategy.category}</div>
                    </td>
                    <td className="px-4 py-3"><TypeBadge type={strategy.optionType} /></td>
                    <td className="px-4 py-3"><StatusBadge status={strategy.status} /></td>
                    <td className="px-4 py-3 font-mono text-xs" style={{ color: strategy.score >= 70 ? "var(--green)" : "var(--text-secondary)" }}>{strategy.score.toFixed(0)}</td>
                    <td className="px-4 py-3 text-right font-mono text-xs" style={{ color: strategy.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                      {strategy.totalTrades > 0 ? fmtINR(strategy.totalPnl, { signed: true }) : "--"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}
