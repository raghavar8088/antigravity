"use client";

import { useState } from "react";
import type {
  CryptoEngineStats,
  CryptoPosition,
  CryptoQuoteDisplay,
  CryptoStrategyStatus,
  CryptoTrade,
} from "@/hooks/useCryptoEquityEngine";

const INITIAL_BALANCE = 1_000_000;

function fmtUSD(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-US", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtPct(value: number, signed = false) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(2)}%`;
}

function fmtTime(iso: string) {
  if (!iso) return "--";
  try {
    return new Date(iso).toLocaleTimeString("en-US", {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  } catch {
    return "--";
  }
}

function fmtHold(seconds: number) {
  if (seconds <= 0) return "0s";
  const minutes = Math.floor(seconds / 60);
  const rem = seconds % 60;
  return minutes > 0 ? `${minutes}m ${rem}s` : `${rem}s`;
}

function scoreColor(score: number) {
  if (score >= 82) return "var(--green)";
  if (score >= 70) return "var(--blue)";
  return "var(--text-secondary)";
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

function MetricCard({ label, value, detail, accent = "" }: { label: string; value: string; detail?: string; accent?: string }) {
  return (
    <div className="metric-card flex min-h-[96px] flex-col justify-between gap-2">
      <div className="metric-label">{label}</div>
      <div className={`metric-value ${accent}`}>{value}</div>
      {detail ? <div className="text-xs" style={{ color: "var(--text-secondary)" }}>{detail}</div> : null}
    </div>
  );
}

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  const positive = side === "LONG";
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
      style={{
        background: positive ? "rgba(24,128,56,0.12)" : "rgba(217,48,37,0.12)",
        color: positive ? "var(--green)" : "var(--red)",
      }}
    >
      {side}
    </span>
  );
}

function StatusBadge({ status }: { status: CryptoStrategyStatus["status"] }) {
  const tones: Record<CryptoStrategyStatus["status"], { bg: string; color: string }> = {
    WARMING: { bg: "rgba(100,100,100,0.1)", color: "var(--text-secondary)" },
    READY: { bg: "rgba(26,115,232,0.1)", color: "var(--blue)" },
    IN_POSITION: { bg: "rgba(24,128,56,0.1)", color: "var(--green)" },
    COOLING: { bg: "rgba(245,124,0,0.12)", color: "var(--amber)" },
  };
  const tone = tones[status];
  return (
    <span className="rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ background: tone.bg, color: tone.color }}>
      {status}
    </span>
  );
}

function QuoteCard({ quote }: { quote: CryptoQuoteDisplay }) {
  const positive = quote.changePct >= 0;
  const activeSignal = quote.signalScore >= 66;
  return (
    <div
      className="rounded-[16px] border px-3 py-3 transition-all"
      style={{
        borderColor: quote.hasPosition ? "rgba(245,124,0,0.45)" : activeSignal ? "rgba(26,115,232,0.35)" : "var(--border)",
        background: quote.hasPosition ? "rgba(245,124,0,0.06)" : activeSignal ? "rgba(26,115,232,0.04)" : "var(--surface-2)",
      }}
    >
      <div className="flex items-center justify-between gap-2">
        <div>
          <div className="text-xs font-semibold" style={{ color: "var(--text-primary)" }}>{quote.symbol}</div>
          <div className="text-[10px] uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>{quote.sector}</div>
        </div>
        {quote.strategyLabel ? (
          <span className="rounded-full px-1.5 py-0.5 text-[9px] font-bold uppercase" style={{ background: "rgba(245,124,0,0.16)", color: "var(--amber)" }}>
            {quote.strategyLabel}
          </span>
        ) : null}
      </div>
      <div className="mt-2 font-mono text-lg font-semibold" style={{ color: "var(--text-primary)" }}>
        {quote.ltp > 0 ? fmtUSD(quote.ltp) : "Waiting..."}
      </div>
      <div className="mt-1 text-[11px] font-medium" style={{ color: positive ? "var(--green)" : "var(--red)" }}>
        {quote.ltp > 0 ? fmtPct(quote.changePct, true) : "Feed pending"}
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
        <div className="h-full rounded-full" style={{ width: `${Math.min(100, quote.signalScore)}%`, background: scoreColor(quote.signalScore) }} />
      </div>
    </div>
  );
}

function PositionSnapshotCard({ position }: { position: CryptoPosition }) {
  return (
    <div
      className="rounded-[16px] border px-4 py-3"
      style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <SideBadge side={position.side} />
            <span className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{position.symbol}</span>
          </div>
          <div className="mt-1 text-[11px]" style={{ color: "var(--text-secondary)" }}>
            {position.strategyName.replace(/_/g, " ")}
          </div>
        </div>
        <div className="text-right">
          <div className="font-mono text-sm font-semibold" style={{ color: position.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
            {fmtUSD(position.unrealizedPnl, { signed: true })}
          </div>
          <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
            {fmtPct(position.returnPct, true)}
          </div>
        </div>
      </div>
      <div className="mt-3 grid grid-cols-2 gap-3 text-[11px]">
        <div>
          <div style={{ color: "var(--text-secondary)" }}>Entry</div>
          <div className="font-mono" style={{ color: "var(--text-primary)" }}>{fmtUSD(position.entryPrice, { decimals: 4 })}</div>
        </div>
        <div>
          <div style={{ color: "var(--text-secondary)" }}>Current</div>
          <div className="font-mono" style={{ color: "var(--text-primary)" }}>{fmtUSD(position.currentPrice, { decimals: 4 })}</div>
        </div>
        <div>
          <div style={{ color: "var(--text-secondary)" }}>Size</div>
          <div className="font-mono" style={{ color: "var(--text-primary)" }}>{position.quantity.toFixed(4)}</div>
        </div>
        <div>
          <div style={{ color: "var(--text-secondary)" }}>Opened</div>
          <div className="font-mono" style={{ color: "var(--text-primary)" }}>{fmtTime(position.entryTime)}</div>
        </div>
      </div>
    </div>
  );
}

type Props = {
  actionsEnabled?: boolean;
  quotes: CryptoQuoteDisplay[];
  positions: CryptoPosition[];
  trades: CryptoTrade[];
  strategies: CryptoStrategyStatus[];
  stats: CryptoEngineStats;
  reset: () => void;
};

export default function CryptoEquityScalper({
  actionsEnabled = false,
  quotes,
  positions,
  trades,
  strategies,
  stats,
  reset,
}: Props) {
  const [isResetting, setIsResetting] = useState(false);
  const [showAllTrades, setShowAllTrades] = useState(false);
  const actionButtonTitle = actionsEnabled ? "Action buttons are enabled." : "Set Action to Yes to enable reset and clear buttons.";
  const topStrategy = [...strategies].filter((item) => item.totalTrades > 0).sort((a, b) => b.totalPnl - a.totalPnl)[0] ?? null;
  const shownTrades = showAllTrades ? trades : trades.slice(0, 12);

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!confirm("Reset the Crypto Equity module? All positions, trades, and capital history will be cleared.")) return;
    setIsResetting(true);
    reset();
    setTimeout(() => setIsResetting(false), 400);
  };

  return (
    <div className="space-y-6">
      <section className="glass-panel overflow-hidden">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.15fr)_420px]">
          <div className="px-6 py-7 md:px-8">
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <span className="pill-green">LIVE</span>
              <span className="rounded-full px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ background: "rgba(26,115,232,0.10)", color: "var(--blue)" }}>
                TOP 20 CRYPTO
              </span>
              <span className="rounded-full border border-orange-200 bg-orange-50 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] text-orange-700">
                Binance Feed
              </span>
              {stats.warmingUp ? (
                <span className="rounded-full px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ background: "rgba(100,100,100,0.10)", color: "var(--text-secondary)" }}>
                  Warming up
                </span>
              ) : null}
            </div>
            <div className="max-w-3xl">
              <div className="text-xs font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
                CRYPTO EQUITY
              </div>
              <div className={`mt-3 text-[clamp(2.65rem,5vw,3.5rem)] font-semibold leading-none tracking-tight ${stats.equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                {fmtUSD(stats.equity)}
              </div>
              <div className="mt-3 max-w-2xl text-sm leading-6" style={{ color: "var(--text-secondary)" }}>
                Autonomous top-20 crypto workspace. The engine tracks 20 spot markets, runs 50 directional strategies,
                and deploys 5% paper capital per trade with TP, SL, profit-lock, and time-based exits.
              </div>
              <div className="mt-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                {stats.diagnostics}
              </div>
            </div>
          </div>

          <div className="flex flex-col justify-between gap-5 border-t px-6 py-7 md:px-8 lg:border-l lg:border-t-0" style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}>
            <div className="space-y-3">
              <div className="text-xs font-semibold uppercase tracking-[0.16em]" style={{ color: "var(--text-secondary)" }}>
                Engine pulse
              </div>
              <div className="grid grid-cols-2 gap-3">
                <MetricCard label="Open Positions" value={`${stats.openPositions}`} detail="Max 10 concurrent" />
                <MetricCard label="Live Symbols" value={`${stats.liveSymbols}/20`} detail="Tracked every 3 seconds" />
                <MetricCard label="Strategies" value={`${stats.activeStrategies}/50`} detail={stats.warmingUp ? "Loading bars..." : "Scanner online"} />
                <MetricCard label="Win Rate" value={stats.totalTrades > 0 ? fmtPct(stats.winRate) : "--"} detail={`${stats.totalTrades} closed trades`} accent={stats.winRate >= 50 ? "text-emerald-600" : ""} />
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <button type="button" disabled={!actionsEnabled || isResetting} title={actionButtonTitle} className="btn-danger" onClick={handleReset}>
                {isResetting ? "Resetting..." : "Reset Module"}
              </button>
              <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
                Reset is guarded by the Action toggle.
              </div>
            </div>
          </div>
        </div>
      </section>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-6">
        <SummaryCard label="Account Equity" value={fmtUSD(stats.equity)} accent={stats.equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Cash Balance" value={fmtUSD(stats.balance)} />
        <SummaryCard label="Session PnL" value={fmtUSD(stats.sessionPnl, { signed: true })} accent={stats.sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Unrealized" value={fmtUSD(stats.unrealizedPnl, { signed: true })} accent={stats.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Realized" value={fmtUSD(stats.realizedPnl, { signed: true })} accent={stats.realizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Best Strategy" value={topStrategy ? topStrategy.name.replace(/_/g, " ") : "Waiting"} detail={topStrategy ? fmtUSD(topStrategy.totalPnl, { signed: true }) : "No closed trades yet"} />
      </div>

      <div className="glass-panel px-5 py-6 md:px-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Open Positions Snapshot
            </h2>
            <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
              Active Crypto Equity positions, surfaced near the top for quicker monitoring.
            </div>
          </div>
          <div className="flex flex-wrap gap-2 text-[11px]">
            <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 font-medium text-zinc-600">
              {positions.length} open
            </span>
            <span className="rounded-full px-3 py-1 font-medium" style={{ background: "rgba(245,124,0,0.10)", color: "var(--amber)" }}>
              Unrealized {fmtUSD(stats.unrealizedPnl, { signed: true })}
            </span>
          </div>
        </div>
        {positions.length === 0 ? (
          <div className="flex min-h-[140px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm" style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}>
            No crypto positions are open right now.
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {positions.map((position) => (
              <PositionSnapshotCard key={position.id} position={position} />
            ))}
          </div>
        )}
      </div>

      <div className="glass-panel px-5 py-6 md:px-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Crypto Scanner
            </h2>
            <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
              Top 20 liquid crypto symbols scanned continuously for momentum, breakout, VWAP, trend, and mean-reversion setups.
            </div>
          </div>
          <div className="flex flex-wrap gap-2 text-[11px]">
            <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 font-medium text-zinc-600">{stats.liveSymbols}/20 live</span>
            <span className="rounded-full px-3 py-1 font-medium" style={{ background: "rgba(26,115,232,0.10)", color: "var(--blue)" }}>{quotes.filter((quote) => quote.signalScore >= 66).length} active signals</span>
            <span className="rounded-full px-3 py-1 font-medium" style={{ background: "rgba(245,124,0,0.10)", color: "var(--amber)" }}>{positions.length} open trades</span>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5">
          {quotes.map((quote) => (
            <QuoteCard key={quote.symbol} quote={quote} />
          ))}
        </div>
      </div>

      <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <div className="space-y-6">
          <div className="glass-panel p-6">
            <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Open Positions
            </div>
            {positions.length === 0 ? (
              <div className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm" style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}>
                No crypto positions are open yet. The engine is scanning all 20 markets for qualified setups.
              </div>
            ) : (
              <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
                <table className="w-full min-w-[920px] text-left text-sm">
                  <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                    <tr className="text-[11px] uppercase tracking-[0.12em]">
                      <th className="px-4 py-3 font-medium">Symbol</th>
                      <th className="px-4 py-3 font-medium">Strategy</th>
                      <th className="px-4 py-3 font-medium">Entry</th>
                      <th className="px-4 py-3 font-medium">TP / SL</th>
                      <th className="px-4 py-3 font-medium">Size</th>
                      <th className="px-4 py-3 font-medium">Opened</th>
                      <th className="px-4 py-3 font-medium text-right">PnL</th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((position) => (
                      <tr key={position.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <SideBadge side={position.side} />
                            <div>
                              <div className="font-semibold" style={{ color: "var(--text-primary)" }}>{position.symbol}</div>
                              <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{position.sector}</div>
                            </div>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{position.strategyName.replace(/_/g, " ")}</td>
                        <td className="px-4 py-3 font-mono text-xs">
                          <div style={{ color: "var(--text-primary)" }}>{fmtUSD(position.entryPrice, { decimals: 4 })}</div>
                          <div style={{ color: "var(--text-secondary)" }}>{fmtUSD(position.currentPrice, { decimals: 4 })} now</div>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs">
                          <div style={{ color: "var(--green)" }}>TP {fmtUSD(position.tpPrice, { decimals: 4 })}</div>
                          <div style={{ color: "var(--red)" }}>SL {fmtUSD(position.slPrice, { decimals: 4 })}</div>
                        </td>
                        <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                          <div>{position.quantity.toFixed(4)} units</div>
                          <div>{fmtUSD(position.notional)}</div>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>{fmtTime(position.entryTime)}</td>
                        <td className="px-4 py-3 text-right">
                          <div className="font-mono text-sm font-semibold" style={{ color: position.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                            {fmtUSD(position.unrealizedPnl, { signed: true })}
                          </div>
                          <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{fmtPct(position.returnPct, true)}</div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="glass-panel p-6">
            <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Trade Ledger
            </div>
            {shownTrades.length === 0 ? (
              <div className="flex min-h-[160px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm" style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}>
                No completed trades yet. Closed crypto positions will appear here.
              </div>
            ) : (
              <div className="space-y-3">
                <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
                  <table className="w-full min-w-[980px] text-left text-sm">
                    <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                      <tr className="text-[11px] uppercase tracking-[0.12em]">
                        <th className="px-4 py-3 font-medium">Time</th>
                        <th className="px-4 py-3 font-medium">Symbol</th>
                        <th className="px-4 py-3 font-medium">Strategy</th>
                        <th className="px-4 py-3 font-medium">Entry / Exit</th>
                        <th className="px-4 py-3 font-medium">Size</th>
                        <th className="px-4 py-3 font-medium">Hold</th>
                        <th className="px-4 py-3 font-medium">Exit</th>
                        <th className="px-4 py-3 font-medium text-right">PnL</th>
                      </tr>
                    </thead>
                    <tbody>
                      {shownTrades.map((trade) => (
                        <tr key={trade.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                          <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>{fmtTime(trade.exitTime)}</td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <SideBadge side={trade.side} />
                              <span className="font-semibold text-xs" style={{ color: "var(--text-primary)" }}>{trade.symbol}</span>
                            </div>
                          </td>
                          <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{trade.strategyName.replace(/_/g, " ")}</td>
                          <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                            {fmtUSD(trade.entryPrice, { decimals: 4 })} to {fmtUSD(trade.exitPrice, { decimals: 4 })}
                          </td>
                          <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>{trade.quantity.toFixed(4)}</td>
                          <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{fmtHold(trade.holdSeconds)}</td>
                          <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{trade.exitReason}</td>
                          <td className="px-4 py-3 text-right">
                            <div className="font-mono text-sm font-semibold" style={{ color: trade.netPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                              {fmtUSD(trade.netPnl, { signed: true })}
                            </div>
                            <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{fmtPct(trade.returnPct, true)}</div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                {trades.length > 12 ? (
                  <button type="button" onClick={() => setShowAllTrades((previous) => !previous)} className="btn-primary w-full">
                    {showAllTrades ? "Show fewer trades" : `Show all ${trades.length} trades`}
                  </button>
                ) : null}
              </div>
            )}
          </div>
        </div>

        <div className="glass-panel p-6">
          <div className="mb-4 text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
            Strategy Roster
          </div>
          <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full min-w-[780px] text-left text-sm">
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Strategy</th>
                  <th className="px-4 py-3 font-medium">Side</th>
                  <th className="px-4 py-3 font-medium">Category</th>
                  <th className="px-4 py-3 font-medium">Status</th>
                  <th className="px-4 py-3 font-medium">Score</th>
                  <th className="px-4 py-3 font-medium">Trades</th>
                  <th className="px-4 py-3 font-medium text-right">PnL</th>
                </tr>
              </thead>
              <tbody>
                {strategies.map((strategy) => (
                  <tr key={strategy.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3">
                      <div className="font-medium text-xs" style={{ color: "var(--text-primary)" }}>{strategy.name.replace(/_/g, " ")}</div>
                      {strategy.currentSymbol ? <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{strategy.currentSymbol}</div> : null}
                    </td>
                    <td className="px-4 py-3"><SideBadge side={strategy.side} /></td>
                    <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>{strategy.category}</td>
                    <td className="px-4 py-3"><StatusBadge status={strategy.status} /></td>
                    <td className="px-4 py-3">
                      {strategy.score > 0 ? (
                        <div className="flex items-center gap-1.5">
                          <div className="h-1.5 w-12 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
                            <div className="h-full rounded-full" style={{ width: `${Math.min(100, strategy.score)}%`, background: scoreColor(strategy.score) }} />
                          </div>
                          <span className="font-mono text-xs" style={{ color: scoreColor(strategy.score) }}>{strategy.score.toFixed(0)}</span>
                        </div>
                      ) : (
                        <span className="text-xs" style={{ color: "var(--text-secondary)" }}>--</span>
                      )}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>{strategy.totalTrades}</td>
                    <td className="px-4 py-3 text-right font-mono text-xs" style={{ color: strategy.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                      {strategy.totalTrades > 0 ? fmtUSD(strategy.totalPnl, { signed: true }) : "--"}
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
