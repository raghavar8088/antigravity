"use client";

import { useEffect, useState } from "react";
import BtcSpotStrip from "@/components/BtcSpotStrip";
import DailyPnlLedger from "@/components/DailyPnlLedger";
import { formatShortDate, formatShortTime } from "@/lib/utils/time";
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

function fmtPct(value: number, signed = false, decimals = 1) {
  const prefix = signed ? (value >= 0 ? "+" : "") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

function fmtPrice(value: number) {
  if (value >= 1_000) return fmtUSD(value);
  if (value >= 1) return fmtUSD(value, { decimals: 4 });
  if (value >= 0.01) return fmtUSD(value, { decimals: 4 });
  if (value >= 0.0001) return fmtUSD(value, { decimals: 6 });
  return fmtUSD(value, { decimals: 8 });
}

function fmtNumber(value: number, decimals = 2) {
  return value.toLocaleString("en-US", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

function formatElapsedSeconds(total: number) {
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}

function humanizeStrategy(name: string) {
  return name.replace(/_/g, " ");
}

function fmtStrategyLabel(name: string, id?: number) {
  const label = humanizeStrategy(name);
  return id && id > 0 ? `${id}. ${label}` : label;
}

function formatMaybeDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return `${formatShortDate(value)} ${formatShortTime(value)}`;
}

function scoreColor(score: number) {
  if (score >= 82) return "var(--green)";
  if (score >= 70) return "var(--blue)";
  if (score >= 61) return "var(--amber)";
  return "var(--text-secondary)";
}

type BadgeTone = "neutral" | "positive" | "negative" | "info" | "warning";

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
        <div className="metric-label">{label}</div>
        <div className={`metric-value ${accent}`}>{value}</div>
      </div>
      <div className="text-xs" style={{ color: "var(--text-secondary)", minHeight: 18 }}>
        {detail ?? ""}
      </div>
    </div>
  );
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

function BadgePill({ label, tone = "neutral" }: { label: string; tone?: BadgeTone }) {
  const map: Record<BadgeTone, string> = {
    neutral: "border-zinc-200 bg-white text-zinc-600",
    positive: "border-emerald-200 bg-emerald-50 text-emerald-700",
    negative: "border-rose-200 bg-rose-50 text-rose-700",
    info: "border-blue-200 bg-blue-50 text-blue-700",
    warning: "border-amber-200 bg-amber-50 text-amber-700",
  };
  return (
    <span className={`inline-flex items-center rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] ${map[tone]}`}>
      {label}
    </span>
  );
}

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  return (
    <span
      className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${
        side === "LONG"
          ? "border-emerald-200 bg-emerald-50 text-emerald-700"
          : "border-rose-200 bg-rose-50 text-rose-700"
      }`}
    >
      {side}
    </span>
  );
}

function StatusBadge({ status }: { status: CryptoStrategyStatus["status"] }) {
  const map: Record<CryptoStrategyStatus["status"], string> = {
    READY: "border-emerald-200 bg-emerald-50 text-emerald-700",
    IN_POSITION: "border-blue-200 bg-blue-50 text-blue-700",
    COOLING: "border-amber-200 bg-amber-50 text-amber-700",
    WARMING: "border-zinc-200 bg-zinc-50 text-zinc-600",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[status]}`}>
      {status.replace("_", " ")}
    </span>
  );
}

function ExitBadge({ reason }: { reason: string }) {
  const map: Record<string, string> = {
    TP: "border-emerald-200 bg-emerald-50 text-emerald-700",
    SL: "border-rose-200 bg-rose-50 text-rose-700",
    PROFIT_LOCK: "border-emerald-200 bg-emerald-50 text-emerald-700",
    TRAIL_EXIT: "border-blue-200 bg-blue-50 text-blue-700",
    TIME_EXIT: "border-zinc-200 bg-zinc-50 text-zinc-600",
    LATE_EXIT: "border-amber-200 bg-amber-50 text-amber-700",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[reason] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>
      {reason.replace(/_/g, " ")}
    </span>
  );
}

function SignalBar({ score }: { score: number }) {
  return (
    <div className="w-full max-w-[84px]">
      <div className="h-1.5 w-full overflow-hidden rounded-full" style={{ background: "var(--border)" }}>
        <div
          className="h-full rounded-full transition-all"
          style={{ width: `${Math.min(100, Math.max(0, score))}%`, background: scoreColor(score) }}
        />
      </div>
    </div>
  );
}

function LivePositionsPanel({ positions }: { positions: CryptoPosition[] }) {
  const totalUnrealized = positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const longCount = positions.filter((position) => position.side === "LONG").length;
  const shortCount = positions.length - longCount;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2
        className="mb-5 flex flex-wrap items-center gap-3"
        style={{
          fontFamily: "var(--font-display)",
          fontSize: 11,
          fontWeight: 800,
          letterSpacing: "0.14em",
          color: "var(--text-secondary)",
        }}
      >
        <span className="pill-green">LIVE</span>
        OPEN CRYPTO EQUITY POSITIONS
        <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>
          ({positions.length} active)
        </span>
      </h2>

      {positions.length === 0 ? (
        <div
          className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
          style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}
        >
          No open crypto positions - engine is scanning the 40-coin spot roster for entries.
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <span className="h-2 w-2 rounded-full" style={{ background: "var(--green)" }} />
              <span className="text-xs font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
                {longCount} long | {shortCount} short
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
              Unrealized {fmtUSD(totalUnrealized, { signed: true })}
            </span>
          </div>

          <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-left text-sm" style={{ minWidth: 980 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Position</th>
                  <th className="px-4 py-3 font-medium">Entry</th>
                  <th className="px-4 py-3 font-medium">Targets</th>
                  <th className="px-4 py-3 font-medium">Opened</th>
                  <th className="px-4 py-3 font-medium">Size</th>
                  <th className="px-4 py-3 font-medium">Signal</th>
                  <th className="px-4 py-3 font-medium text-right">PnL</th>
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
                            {position.symbol}
                          </span>
                        </div>
                        <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                          {fmtStrategyLabel(position.strategyName, position.strategyId)}
                        </div>
                        <div className="text-[11px]" style={{ color: "var(--text-muted)" }}>
                          {position.sector}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {fmtPrice(position.entryPrice)}
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>Now {fmtPrice(position.currentPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--green)" }}>
                        TP {fmtPrice(position.tpPrice)}
                      </div>
                      <div style={{ color: "var(--red)" }}>SL {fmtPrice(position.slPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {formatShortTime(position.entryTime)}
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(position.entryTime)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {fmtNumber(position.quantity, 4)} units
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>{fmtUSD(position.notional)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="ml-auto w-28">
                        <SignalBar score={Math.max(0, Math.min(100, Math.abs(position.returnPct) * 12))} />
                      </div>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="font-mono text-sm font-semibold" style={{ color: position.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtUSD(position.unrealizedPnl, { signed: true })}
                      </div>
                      <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                        {fmtPct(position.returnPct, true, 2)}
                      </div>
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

function StrategiesPanel({ strategies }: { strategies: CryptoStrategyStatus[] }) {
  const [showAll, setShowAll] = useState(false);
  const sorted = [...strategies].sort((a, b) => {
    if (b.score !== a.score) return b.score - a.score;
    return b.totalPnl - a.totalPnl;
  });
  const visible = showAll ? sorted : sorted.slice(0, 20);
  const totalStrategies = strategies.length;
  const topCount = Math.min(20, totalStrategies);

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="mb-5 flex items-center justify-between gap-3">
        <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
          CRYPTO STRATEGIES - LEADERBOARD ({totalStrategies})
        </h2>
        {totalStrategies > topCount ? (
          <button type="button" onClick={() => setShowAll((value) => !value)} className="btn-gold min-h-[32px] px-4 py-1.5 text-xs">
            {showAll ? `Top ${topCount}` : `All ${totalStrategies}`}
          </button>
        ) : null}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left" style={{ minWidth: 1160 }}>
          <thead>
            <tr className="border-b" style={{ borderColor: "var(--border)" }}>
              {["ID", "Strategy", "Side", "Category", "Runtime", "Score", "Live", "Allocation", "PnL", "Notes"].map((heading, index) => (
                <th
                  key={heading}
                  className={`px-3 py-2 text-[10px] font-bold uppercase tracking-widest ${index >= 7 && index <= 8 ? "text-right" : ""}`}
                  style={{ color: "var(--text-muted)" }}
                >
                  {heading}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((strategy) => (
              <tr key={strategy.id} className="border-b transition-colors hover:bg-black/[0.015]" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="px-3 py-2.5 text-xs font-mono" style={{ color: "var(--text-muted)" }}>
                  {strategy.id}
                </td>
                <td className="px-3 py-2.5">
                  <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
                    {fmtStrategyLabel(strategy.name, strategy.id)}
                  </div>
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                    {strategy.currentSymbol || "Scanner wide"}
                  </div>
                </td>
                <td className="px-3 py-2.5">
                  <SideBadge side={strategy.side} />
                </td>
                <td className="px-3 py-2.5 text-sm" style={{ color: "var(--text-secondary)" }}>
                  {strategy.category}
                </td>
                <td className="px-3 py-2.5">
                  <StatusBadge status={strategy.status} />
                </td>
                <td className="px-3 py-2.5 text-sm font-mono font-semibold" style={{ color: scoreColor(strategy.score) }}>
                  {strategy.score > 0 ? strategy.score.toFixed(1) : "--"}
                </td>
                <td className="px-3 py-2.5 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {strategy.totalTrades}T | {strategy.wins}W / {strategy.losses}L
                  <div className="text-[11px]">{strategy.totalTrades > 0 ? fmtPct(strategy.winRate) : "-"}</div>
                </td>
                <td className="px-3 py-2.5 text-right text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {fmtUSD(strategy.allocationUSD)}
                </td>
                <td className={`px-3 py-2.5 text-right text-sm font-mono font-bold ${strategy.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {strategy.totalTrades > 0 ? fmtUSD(strategy.totalPnl, { signed: true }) : "-"}
                </td>
                <td className="px-3 py-2.5 text-[11px]" style={{ color: "var(--text-secondary)" }}>
                  <div>{strategy.currentSymbol ? `Locked to ${strategy.currentSymbol}` : "Scanning watchlist"}</div>
                  {strategy.cooldownUntil ? <div>Until {formatMaybeDate(strategy.cooldownUntil)}</div> : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function DailyPnlPanel({ trades }: { trades: CryptoTrade[] }) {
  return (
    <DailyPnlLedger
      trades={trades}
      initialEquity={INITIAL_BALANCE}
      title="DAILY PNL LEDGER"
      description="Realized crypto-equity PnL grouped by trade exit day, with daily return based on that day's starting equity."
      emptyMessage="No closed crypto-equity trades yet, so there is no daily PnL ledger to display."
      formatCurrency={fmtUSD}
    />
  );
}

function TradesPanel({ trades }: { trades: CryptoTrade[] }) {
  const [showAll, setShowAll] = useState(false);
  const visibleTrades = showAll ? trades : trades.slice(0, 10);
  const totalTrades = trades.length;
  const wins = trades.filter((trade) => trade.netPnl > 0).length;
  const losses = trades.filter((trade) => trade.netPnl < 0).length;
  const winRate = totalTrades > 0 ? (wins / totalTrades) * 100 : 0;
  const totalPnl = trades.reduce((sum, trade) => sum + trade.netPnl, 0);
  const grossProfit = trades.filter((trade) => trade.netPnl > 0).reduce((sum, trade) => sum + trade.netPnl, 0);
  const grossLoss = trades.filter((trade) => trade.netPnl < 0).reduce((sum, trade) => sum + Math.abs(trade.netPnl), 0);
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
          CRYPTO EQUITY - TRADE HISTORY
          <span className="ml-3 font-mono font-normal" style={{ color: "var(--text-muted)", fontSize: 10 }}>
            ({totalTrades} total)
          </span>
        </h2>
        {trades.length > 10 ? (
          <button type="button" onClick={() => setShowAll((value) => !value)} className="btn-gold min-h-[32px] px-4 py-1.5 text-xs">
            {showAll ? "Latest 10" : `All ${trades.length}`}
          </button>
        ) : null}
      </div>

      {trades.length === 0 ? (
        <div className="rounded-2xl border border-dashed py-12 text-center" style={{ borderColor: "var(--border)", color: "var(--text-muted)", fontSize: 13 }}>
          No completed crypto-equity trades yet.
        </div>
      ) : (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
            <SummaryCard label="Trades" value={`${totalTrades}`} accent="text-zinc-900" />
            <SummaryCard label="Win Rate" value={`${winRate.toFixed(1)}%`} accent={winRate >= 50 ? "text-emerald-600" : "text-rose-600"} />
            <SummaryCard label="Net PnL" value={fmtUSD(totalPnl, { signed: true })} accent={totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
            <SummaryCard label="Profit Factor" value={profitFactor.toFixed(2)} accent={profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"} />
            <SummaryCard label="W / L" value={`${wins}/${losses}`} accent="text-zinc-900" />
          </div>

          <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-left text-sm" style={{ minWidth: 1040 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Time</th>
                  <th className="px-4 py-3 font-medium">Strategy</th>
                  <th className="px-4 py-3 font-medium">Position</th>
                  <th className="px-4 py-3 font-medium">Entry / Exit</th>
                  <th className="px-4 py-3 font-medium">Size</th>
                  <th className="px-4 py-3 font-medium">Duration</th>
                  <th className="px-4 py-3 font-medium">Exit</th>
                  <th className="px-4 py-3 font-medium text-right">Return</th>
                  <th className="px-4 py-3 font-medium text-right">Net PnL</th>
                </tr>
              </thead>
              <tbody>
                {visibleTrades.map((trade) => (
                  <tr key={trade.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {formatShortTime(trade.exitTime)}
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(trade.exitTime)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
                        {fmtStrategyLabel(trade.strategyName, trade.strategyId)}
                      </div>
                      <div className="font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>
                        {trade.id}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="flex items-center gap-2">
                        <SideBadge side={trade.side} />
                        <span className="font-mono" style={{ color: "var(--text-primary)" }}>
                          {trade.symbol}
                        </span>
                      </div>
                      <div style={{ color: "var(--text-secondary)", marginTop: 4 }}>{trade.side.toLowerCase()} spot execution</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {fmtPrice(trade.entryPrice)} to {fmtPrice(trade.exitPrice)}
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>Qty {fmtNumber(trade.quantity, 4)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {fmtNumber(trade.quantity, 4)}
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>Paper units</div>
                    </td>
                    <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                      {formatElapsedSeconds(trade.holdSeconds)}
                    </td>
                    <td className="px-4 py-3">
                      <ExitBadge reason={trade.exitReason} />
                    </td>
                    <td className={`px-4 py-3 text-right font-mono text-sm font-semibold ${trade.returnPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtPct(trade.returnPct, true, 2)}
                    </td>
                    <td className={`px-4 py-3 text-right font-mono text-sm font-bold ${trade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtUSD(trade.netPnl, { signed: true })}
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

function ScannerQuoteCard({ quote }: { quote: CryptoQuoteDisplay }) {
  const changePositive = quote.changePct >= 0;
  const signalTone = quote.signalScore >= 70 ? "var(--blue)" : quote.signalScore >= 61 ? "var(--amber)" : "var(--text-secondary)";

  return (
    <div className="rounded-[18px] border px-4 py-4 transition-all" style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}>
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
            {quote.symbol}
          </div>
          <div className="text-[11px] uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
            {quote.sector}
          </div>
        </div>
        {quote.strategyLabel ? (
          <span className="rounded-md border border-amber-200 bg-amber-50 px-2 py-0.5 text-[10px] font-bold tracking-widest text-amber-700">
            {quote.strategyLabel}
          </span>
        ) : null}
      </div>
      <div className="mt-3 font-mono text-lg font-semibold" style={{ color: "var(--text-primary)" }}>
        {quote.ltp > 0 ? fmtPrice(quote.ltp) : "Waiting..."}
      </div>
      <div className="mt-1 text-[11px] font-medium" style={{ color: changePositive ? "var(--green)" : "var(--red)" }}>
        {quote.ltp > 0 ? fmtPct(quote.changePct, true, 2) : "Feed pending"}
      </div>
      <div className="mt-4 flex items-center justify-between gap-3">
        <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
          {quote.hasPosition ? `Live: ${quote.strategyLabel}` : "Scanner ready"}
        </div>
        <div className="text-[11px] font-mono" style={{ color: signalTone }}>
          {quote.signalScore.toFixed(0)}
        </div>
      </div>
      <div className="mt-2">
        <SignalBar score={quote.signalScore} />
      </div>
    </div>
  );
}

function ScannerPanel({ quotes, positions }: { quotes: CryptoQuoteDisplay[]; positions: CryptoPosition[] }) {
  const activeSignals = quotes.filter((quote) => quote.signalScore >= 66).length;
  const liveQuotes = quotes.filter((quote) => quote.ltp > 0).length;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
            CRYPTO SCANNER
          </h2>
          <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
            40 liquid crypto symbols scanned continuously for momentum, breakout, VWAP, trend, and mean-reversion setups.
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <BadgePill label={`${liveQuotes}/${quotes.length} Live`} tone="info" />
          <BadgePill label={`${activeSignals} Active Signals`} tone="warning" />
          <BadgePill label={`${positions.length} Open Trades`} tone="neutral" />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
        {quotes.map((quote) => (
          <ScannerQuoteCard key={quote.symbol} quote={quote} />
        ))}
      </div>
    </div>
  );
}

type Props = {
  actionsEnabled?: boolean;
  btcSpotUsd?: number;
  btcChange24hPct?: number;
  quotes: CryptoQuoteDisplay[];
  positions: CryptoPosition[];
  trades: CryptoTrade[];
  strategies: CryptoStrategyStatus[];
  stats: CryptoEngineStats;
  reset: () => void;
};

export default function CryptoEquityScalper({
  actionsEnabled = false,
  btcSpotUsd,
  btcChange24hPct,
  quotes,
  positions,
  trades,
  strategies,
  stats,
  reset,
}: Props) {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [isResetting, setIsResetting] = useState(false);
  const actionButtonTitle = actionsEnabled
    ? "Action buttons are enabled."
    : "Set Action to Yes to enable reset and clear buttons.";

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!confirm("Reset the Crypto Equity module? All positions, trades, and capital history will be cleared.")) return;
    setIsResetting(true);
    reset();
    setTimeout(() => setIsResetting(false), 400);
  };

  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));
  const totalReturnPct = (stats.sessionPnl / INITIAL_BALANCE) * 100;
  const longCount = positions.filter((position) => position.side === "LONG").length;
  const shortCount = positions.length - longCount;
  const exposureSummary = positions.length === 0 ? "No open exposure" : `${longCount} long / ${shortCount} short`;
  const activeSignals = quotes.filter((quote) => quote.signalScore >= 66).length;
  const bestStrategy =
    [...strategies]
      .filter((strategy) => strategy.totalTrades > 0)
      .sort((a, b) => {
        if (b.totalPnl !== a.totalPnl) return b.totalPnl - a.totalPnl;
        return b.score - a.score;
      })[0] ?? null;
  const latestTrade = trades[0] ?? null;
  const grossProfit = trades.filter((trade) => trade.netPnl > 0).reduce((sum, trade) => sum + trade.netPnl, 0);
  const grossLoss = trades.filter((trade) => trade.netPnl < 0).reduce((sum, trade) => sum + Math.abs(trade.netPnl), 0);
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;
  const bestTrade = trades.reduce<CryptoTrade | null>((best, trade) => (!best || trade.netPnl > best.netPnl ? trade : best), null);
  const avgHoldSeconds = trades.length > 0 ? trades.reduce((sum, trade) => sum + trade.holdSeconds, 0) / trades.length : 0;
  const streak = (() => {
    if (trades.length === 0) return "0";
    const lastWasWin = trades[0].netPnl >= 0;
    let count = 0;
    for (const trade of trades) {
      if ((trade.netPnl >= 0) !== lastWasWin) break;
      count++;
    }
    return `${count}${lastWasWin ? "W" : "L"}`;
  })();

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 items-start gap-5 2xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)]">
        <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
          <div className="pointer-events-none absolute -right-12 -top-12 h-40 w-40 rounded-full bg-violet-500/10 blur-3xl" />

          <div className="flex flex-col gap-5">
            <div className="px-1">
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                CRYPTO EQUITY
              </div>
              <div className="mt-4 flex flex-wrap items-end gap-4">
                <div className={`text-[clamp(2.55rem,5vw,3.35rem)] font-semibold leading-none tracking-tight ${stats.equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtUSD(stats.equity)}
                </div>
                <div className={`pb-1 text-xl font-semibold leading-none ${stats.sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtPct(totalReturnPct, true, 2)}
                </div>
              </div>
              <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
                Session PnL {fmtUSD(stats.sessionPnl, { signed: true })} · 40 coins · 50 autonomous spot strategies
              </div>
              <BtcSpotStrip btcSpotUsd={btcSpotUsd} btcChange24hPct={btcChange24hPct} />
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 px-1">
              <div className="flex flex-wrap gap-2">
                <BadgePill label="Crypto Engine Online" tone="positive" />
                <BadgePill label={`${stats.liveSymbols}/${quotes.length || 40} Live`} tone="info" />
                <BadgePill label="40-Coin Workspace" tone="warning" />
                <BadgePill label="50 Spot Strategies" tone="neutral" />
                <BadgePill label={stats.warmingUp ? "Warming Up" : "Binance Feed Ready"} tone={stats.warmingUp ? "warning" : "positive"} />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={handleReset}
                  disabled={!actionsEnabled || isResetting}
                  title={actionButtonTitle}
                  className="btn-danger text-sm"
                >
                  {isResetting ? "Resetting..." : "Reset Module"}
                </button>
              </div>
            </div>
          </div>

          <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <CompactMetric
              label="Session Runtime"
              value={sessionRuntime}
              detail={`${stats.activeStrategies}/50 strategies live`}
              accent="text-zinc-900"
            />
            <CompactMetric
              label="Last Closed Trade"
              value={latestTrade ? fmtUSD(latestTrade.netPnl, { signed: true }) : "No exits yet"}
              detail={latestTrade ? `${fmtStrategyLabel(latestTrade.strategyName, latestTrade.strategyId)} | ${latestTrade.exitReason}` : "Waiting for first crypto exit cycle"}
              accent={latestTrade ? (latestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600") : "text-zinc-900"}
            />
            <CompactMetric
              label="Open Exposure"
              value={exposureSummary}
              detail={`${positions.length} active positions | ${activeSignals} active signals`}
              accent="text-zinc-900"
            />
          </div>

          {trades.length >= 2 &&
            (() => {
              const points: { x: number; y: number }[] = [];
              let running = INITIAL_BALANCE;
              for (const trade of [...trades].reverse()) {
                running += trade.netPnl;
                points.push({ x: 0, y: running });
              }
              const ys = points.map((point) => point.y);
              const minY = Math.min(...ys, INITIAL_BALANCE);
              const maxY = Math.max(...ys, INITIAL_BALANCE);
              const range = maxY - minY || 1;
              const width = 400;
              const height = 56;
              const px = (index: number) => (index / Math.max(points.length - 1, 1)) * width;
              const py = (value: number) => height - ((value - minY) / range) * height;
              const path = points
                .map((point, index) => `${index === 0 ? "M" : "L"} ${px(index).toFixed(1)} ${py(point.y).toFixed(1)}`)
                .join(" ");
              const color = running >= INITIAL_BALANCE ? "#16a34a" : "#dc2626";

              return (
                <div className="mt-4 px-1">
                  <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                    Crypto Equity Curve - {trades.length} trades
                  </div>
                  <svg viewBox={`0 0 ${width} ${height}`} className="w-full" style={{ height: 56, display: "block" }}>
                    <path d={path} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" />
                    <line
                      x1="0"
                      y1={py(INITIAL_BALANCE).toFixed(1)}
                      x2={width}
                      y2={py(INITIAL_BALANCE).toFixed(1)}
                      stroke="#94a3b8"
                      strokeWidth="1"
                      strokeDasharray="4 3"
                    />
                  </svg>
                </div>
              );
            })()}
        </div>

        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="px-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">Equity And PnL</div>
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <CompactMetric label="Crypto Equity" value={fmtUSD(stats.equity)} detail={`Base ${fmtUSD(INITIAL_BALANCE)}`} accent="text-zinc-900" />
            <CompactMetric
              label="Net PnL"
              value={fmtUSD(stats.sessionPnl, { signed: true })}
              detail={`${totalReturnPct >= 0 ? "+" : ""}${totalReturnPct.toFixed(2)}% vs base`}
              accent={stats.sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
            />
            <CompactMetric
              label="Realized PnL"
              value={fmtUSD(stats.realizedPnl, { signed: true })}
              detail={`${stats.totalTrades} completed trades`}
              accent={stats.realizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
            />
            <CompactMetric
              label="Unrealized PnL"
              value={fmtUSD(stats.unrealizedPnl, { signed: true })}
              detail={`${stats.openPositions} open positions`}
              accent={stats.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
            />
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-7">
        <SummaryCard label="Win Rate" value={stats.totalTrades > 0 ? `${stats.winRate.toFixed(1)}%` : "-"} accent={stats.winRate >= 50 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Profit Factor" value={profitFactor.toFixed(2)} accent={profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Trades" value={`${stats.totalTrades}`} accent="text-zinc-900" />
        <SummaryCard label="Unrealized" value={fmtUSD(stats.unrealizedPnl, { signed: true })} accent={stats.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Streak" value={streak} accent="text-amber-500" />
        <SummaryCard label="Best Trade" value={bestTrade ? fmtUSD(bestTrade.netPnl, { signed: true }) : "-"} accent={bestTrade && bestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Avg Hold" value={avgHoldSeconds > 0 ? formatElapsedSeconds(Math.round(avgHoldSeconds)) : "-"} accent="text-zinc-900" />
      </div>

      {/* ── Open Positions Snapshot (promoted above strategies, matching selling layout) ── */}
      {positions.length > 0 && (
        <div className="glass-panel px-5 py-5 md:px-6">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h2 className="flex items-center gap-3" style={{
              fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800,
              letterSpacing: "0.14em", color: "var(--text-secondary)",
            }}>
              Open Positions Snapshot
              <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>
                Active Crypto Equity positions, surfaced near the top for quicker monitoring.
              </span>
            </h2>
            <div className="flex items-center gap-3">
              <span className="font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                {positions.length} open
              </span>
              <span className="rounded-full border px-3 py-1 text-xs font-medium"
                style={{
                  background: stats.unrealizedPnl >= 0 ? "var(--green-dim)" : "var(--red-dim)",
                  color: stats.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)",
                  borderColor: stats.unrealizedPnl >= 0 ? "rgba(24, 128, 56, 0.14)" : "rgba(217, 48, 37, 0.14)",
                }}>
                Unrealized {fmtUSD(stats.unrealizedPnl, { signed: true })}
              </span>
            </div>
          </div>

          <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-left text-sm" style={{ minWidth: 1040 }}>
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
                          <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{position.symbol}</div>
                          <div className="text-[11px]" style={{ color: "var(--text-muted)" }}>{position.sector}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>{fmtStrategyLabel(position.strategyName, position.strategyId)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>{fmtPrice(position.entryPrice)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>{fmtPrice(position.currentPrice)} now</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--green)" }}>TP {fmtPrice(position.tpPrice)}</div>
                      <div style={{ color: "var(--red)" }}>SL {fmtPrice(position.slPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>{fmtNumber(position.quantity, 4)} units</div>
                      <div style={{ color: "var(--text-secondary)" }}>{fmtUSD(position.notional)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>{formatShortTime(position.entryTime)}</div>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="font-mono text-sm font-semibold" style={{ color: position.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtUSD(position.unrealizedPnl, { signed: true })}
                      </div>
                      <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{fmtPct(position.returnPct, true, 2)}</div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <ScannerPanel quotes={quotes} positions={positions} />

      <DailyPnlPanel trades={trades} />

      <LivePositionsPanel positions={positions} />
      <StrategiesPanel strategies={strategies} />

      {bestStrategy ? (
        <div className="glass-panel flex flex-wrap items-center justify-between gap-6 px-6 py-5">
          <div>
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>
              Top Crypto Strategy
            </div>
            <div className="mt-1 text-lg font-bold" style={{ color: "var(--text-primary)" }}>
              {fmtStrategyLabel(bestStrategy.name, bestStrategy.id)}
            </div>
            <div className="mt-0.5 text-xs" style={{ color: "var(--text-secondary)" }}>
              {bestStrategy.wins}W / {bestStrategy.losses}L | {bestStrategy.totalTrades > 0 ? fmtPct(bestStrategy.winRate) : "-"} win rate | score {bestStrategy.score.toFixed(1)}
            </div>
          </div>
          <div className="text-right">
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>
              Strategy PnL
            </div>
            <div className={`mt-1 text-2xl font-bold ${bestStrategy.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
              {fmtUSD(bestStrategy.totalPnl, { signed: true })}
            </div>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <SideBadge side={bestStrategy.side} />
            <StatusBadge status={bestStrategy.status} />
          </div>
        </div>
      ) : null}

      <TradesPanel trades={trades} />

      <div className="text-center text-[11px]" style={{ color: "var(--text-muted)" }}>
        Crypto equity paper account · Binance spot pricing · $1,000,000 starting capital · 5% capital per trade · TP, SL, profit-lock, and time-based exits
      </div>
    </div>
  );
}
