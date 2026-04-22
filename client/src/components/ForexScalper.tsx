"use client";

import { useEffect, useState } from "react";
import DailyPnlLedger from "@/components/DailyPnlLedger";
import { formatShortDate, formatShortTime } from "@/lib/time";
import type {
  ForexEngineStats,
  ForexPosition,
  ForexQuoteDisplay,
  ForexStrategyStatus,
  ForexTrade,
} from "@/hooks/useForexEngine";

const INITIAL_BALANCE = 1_000_000;

function fmtUSD(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtRate(value: number) {
  if (value === 0) return "—";
  if (value >= 100) return value.toFixed(3);
  return value.toFixed(5);
}

function fmtPct(value: number, signed = false, decimals = 3) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

function formatElapsedSeconds(total: number) {
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m ${s}s`;
}

function formatTradeDuration(entryTime: string, exitTime: string) {
  const entry = new Date(entryTime);
  const exit = new Date(exitTime);
  if (Number.isNaN(entry.getTime()) || Number.isNaN(exit.getTime())) return "-";
  const totalSeconds = Math.max(0, Math.floor((exit.getTime() - entry.getTime()) / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}

type StrategyNumberMap = Record<string, number>;

function resolveNum(name: string, id: number | undefined, map: StrategyNumberMap) {
  if (id && id > 0) return id;
  return map[name] || 0;
}

function fmtLabel(name: string, id: number | undefined, map: StrategyNumberMap) {
  const n = resolveNum(name, id, map);
  return n ? `${n}. ${name}` : name;
}

type BadgeTone = "neutral" | "positive" | "negative" | "info" | "warning";

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

function SummaryCard({ label, value, detail, accent = "" }: { label: string; value: string; detail?: string; accent?: string }) {
  return (
    <div className="summary-card flex min-h-[110px] flex-col justify-between gap-2">
      <div className="summary-label">{label}</div>
      <div className={`summary-value ${accent}`}>{value}</div>
      {detail ? <div className="text-xs" style={{ color: "var(--text-secondary)" }}>{detail}</div> : null}
    </div>
  );
}

function CompactMetric({ label, value, detail, accent = "" }: { label: string; value: string; detail?: string; accent?: string }) {
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

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  const pos = side === "LONG";
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
      style={{ background: pos ? "rgba(24,128,56,0.12)" : "rgba(217,48,37,0.12)", color: pos ? "var(--green)" : "var(--red)" }}
    >
      {side}
    </span>
  );
}

function StatusBadge({ status }: { status: ForexStrategyStatus["status"] }) {
  const tones: Record<ForexStrategyStatus["status"], { bg: string; color: string }> = {
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

function RosterBadge({ rosterState }: { rosterState: ForexStrategyStatus["rosterState"] }) {
  const map: Record<string, string> = {
    ACTIVE: "border-emerald-200 bg-emerald-50 text-emerald-700",
    WATCHLIST: "border-zinc-200 bg-zinc-50 text-zinc-600",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[rosterState] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>
      {rosterState}
    </span>
  );
}

function ExitBadge({ reason }: { reason: string }) {
  const r = reason.toUpperCase();
  const cls =
    r.includes("TP") || r.includes("PROFIT")
      ? "border-emerald-200 bg-emerald-50 text-emerald-700"
      : r.includes("SL") || r.includes("LOSS")
        ? "border-rose-200 bg-rose-50 text-rose-700"
        : "border-zinc-200 bg-zinc-50 text-zinc-600";
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${cls}`}>
      {reason.replace(/_/g, " ")}
    </span>
  );
}

function QuoteCard({ quote }: { quote: ForexQuoteDisplay }) {
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
          <div className="text-[10px] uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>{quote.category}</div>
        </div>
        {quote.strategyLabel ? (
          <span className="rounded-full px-1.5 py-0.5 text-[9px] font-bold uppercase" style={{ background: "rgba(245,124,0,0.16)", color: "var(--amber)" }}>
            {quote.strategyLabel}
          </span>
        ) : null}
      </div>
      <div className="mt-2 font-mono text-lg font-semibold" style={{ color: "var(--text-primary)" }}>
        {quote.ltp > 0 ? fmtRate(quote.ltp) : "Waiting..."}
      </div>
      <div className="mt-1 text-[11px] font-medium" style={{ color: positive ? "var(--green)" : "var(--red)" }}>
        {quote.ltp > 0 ? fmtPct(quote.changePct, true) : "Feed pending"}
      </div>
    </div>
  );
}

function LivePositionsPanel({ positions }: { positions: ForexPosition[] }) {
  const totalUnrealized = positions.reduce((sum, p) => sum + p.unrealizedPnl, 0);
  const longCount = positions.filter((p) => p.side === "LONG").length;
  const shortCount = positions.filter((p) => p.side === "SHORT").length;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2
        className="mb-5 flex flex-wrap items-center gap-3"
        style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}
      >
        <span className="pill-green">LIVE</span>
        OPEN FX POSITIONS
        <span style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }} className="font-mono">
          ({positions.length} active)
        </span>
      </h2>

      {positions.length === 0 ? (
        <div
          className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
          style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}
        >
          No open forex positions yet — the engine assigns live roster slots as Yahoo quotes and signals stabilize.
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
            <table className="w-full text-left text-sm" style={{ minWidth: 1040 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Pair</th>
                  <th className="px-4 py-3 font-medium">Side</th>
                  <th className="px-4 py-3 font-medium text-right">Entry / Current</th>
                  <th className="px-4 py-3 font-medium text-right">TP / SL</th>
                  <th className="px-4 py-3 font-medium text-right">PnL</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((pos) => (
                  <tr key={pos.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3">
                      <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{pos.symbol}</div>
                      <div className="text-[10px] text-zinc-400">{pos.strategyName}</div>
                    </td>
                    <td className="px-4 py-3">
                      <SideBadge side={pos.side} />
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <div style={{ color: "var(--text-primary)" }}>{fmtRate(pos.entryPrice)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>{fmtRate(pos.currentPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <div style={{ color: "var(--green)" }}>{fmtRate(pos.tpPrice)}</div>
                      <div style={{ color: "var(--red)" }}>{fmtRate(pos.slPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="font-mono text-sm font-semibold" style={{ color: pos.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtUSD(pos.unrealizedPnl, { signed: true })}
                      </div>
                      <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{fmtPct(pos.returnPct, true)}</div>
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

function StrategiesPanel({ strategies, strategyNumbers }: { strategies: ForexStrategyStatus[]; strategyNumbers: StrategyNumberMap }) {
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
          FOREX STRATEGIES — ROSTER ({totalStrategies})
        </h2>
        {totalStrategies > topCount ? (
          <button type="button" onClick={() => setShowAll((v) => !v)} className="btn-gold text-xs px-4 py-1.5 min-h-[32px]">
            {showAll ? `Top ${topCount}` : `All ${totalStrategies}`}
          </button>
        ) : null}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left" style={{ minWidth: 1100 }}>
          <thead>
            <tr className="border-b" style={{ borderColor: "var(--border)" }}>
              {["ID", "Strategy", "Side", "Roster", "Status", "Score", "Trades", "Allocation", "Size", "PnL", "Regime"].map((h, i) => (
                <th
                  key={h}
                  className={`py-2 px-3 text-[10px] font-bold uppercase tracking-widest ${i >= 7 && i <= 9 ? "text-right" : ""}`}
                  style={{ color: "var(--text-muted)" }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((s, i) => (
              <tr key={s.id} className="border-b transition-colors hover:bg-black/[0.015]" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="py-2.5 px-3 text-xs font-mono" style={{ color: "var(--text-muted)" }}>{resolveNum(s.name, s.id, strategyNumbers) || i + 1}</td>
                <td className="py-2.5 px-3">
                  <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{fmtLabel(s.name, s.id, strategyNumbers)}</div>
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{s.category}</div>
                </td>
                <td className="py-2.5 px-3">
                  <SideBadge side={s.side} />
                </td>
                <td className="py-2.5 px-3">
                  <RosterBadge rosterState={s.rosterState} />
                </td>
                <td className="py-2.5 px-3">
                  <StatusBadge status={s.status} />
                </td>
                <td className="py-2.5 px-3 text-sm font-mono font-semibold" style={{ color: "var(--text-primary)" }}>{s.score || "—"}</td>
                <td className="py-2.5 px-3 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.totalTrades}T | {s.wins}W / {s.losses}L
                  <div className="text-[11px]">{s.totalTrades > 0 ? `${s.winRate.toFixed(1)}%` : "—"}</div>
                </td>
                <td className="py-2.5 px-3 text-right text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.rosterState === "ACTIVE" ? fmtUSD(s.allocationUSD) : "—"}
                </td>
                <td className="py-2.5 px-3 text-right text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.rosterState === "ACTIVE" ? `${s.sizeMultiplier.toFixed(2)}x` : "—"}
                </td>
                <td className={`py-2.5 px-3 text-right text-sm font-mono font-bold ${s.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {s.totalTrades > 0 ? fmtUSD(s.totalPnl, { signed: true }) : "—"}
                </td>
                <td className="py-2.5 px-3 text-[11px]" style={{ color: "var(--text-secondary)" }}>
                  {s.regime}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function TradesPanel({
  trades,
  strategyNumbers,
  statsTotalTrades,
}: {
  trades: ForexTrade[];
  strategyNumbers: StrategyNumberMap;
  statsTotalTrades: number;
}) {
  const [showAll, setShowAll] = useState(false);
  const visibleTrades = showAll ? trades : trades.slice(0, 10);
  const totalTrades = Math.max(statsTotalTrades, trades.length);
  const wins = trades.filter((t) => t.netPnl > 0).length;
  const losses = trades.filter((t) => t.netPnl < 0).length;
  const winRate = trades.length > 0 ? (wins / trades.length) * 100 : 0;
  const totalPnl = trades.reduce((sum, t) => sum + t.netPnl, 0);
  const grossProfit = trades.filter((t) => t.netPnl > 0).reduce((sum, t) => sum + t.netPnl, 0);
  const grossLoss = trades.filter((t) => t.netPnl < 0).reduce((sum, t) => sum + Math.abs(t.netPnl), 0);
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;
  const hiddenTradeCount = Math.max(statsTotalTrades - trades.length, 0);

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
          FOREX — TRADE HISTORY
          <span className="ml-3 font-mono font-normal" style={{ color: "var(--text-muted)", fontSize: 10 }}>({totalTrades.toLocaleString("en-US")} total)</span>
        </h2>
        {trades.length > 10 ? (
          <button type="button" onClick={() => setShowAll((c) => !c)} className="btn-gold min-h-[32px] px-4 py-1.5 text-xs">
            {showAll ? "Latest 10" : `All ${trades.length}`}
          </button>
        ) : null}
      </div>

      {trades.length === 0 ? (
        <div className="rounded-2xl border border-dashed py-12 text-center" style={{ borderColor: "var(--border)", color: "var(--text-muted)", fontSize: 13 }}>
          No completed forex trades yet.
        </div>
      ) : (
        <div className="space-y-4">
          {hiddenTradeCount > 0 ? (
            <div
              className="rounded-[14px] border px-4 py-3 text-xs"
              style={{ borderColor: "rgba(245,124,0,0.20)", background: "rgba(245,124,0,0.06)", color: "var(--text-secondary)" }}
            >
              Showing the latest {trades.length.toLocaleString("en-US")} of {totalTrades.toLocaleString("en-US")} closed forex trades in this browser session.
            </div>
          ) : null}
          <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
            <SummaryCard label="Trades (loaded)" value={`${trades.length}`} accent="text-zinc-900" />
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
                  <th className="px-4 py-3 font-medium">Pair</th>
                  <th className="px-4 py-3 font-medium">Side</th>
                  <th className="px-4 py-3 font-medium text-right">Entry / Exit</th>
                  <th className="px-4 py-3 font-medium">Duration</th>
                  <th className="px-4 py-3 font-medium">Exit</th>
                  <th className="px-4 py-3 font-medium text-right">Return</th>
                  <th className="px-4 py-3 font-medium text-right">Net PnL</th>
                </tr>
              </thead>
              <tbody>
                {visibleTrades.map((t) => (
                  <tr key={t.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>{formatShortTime(t.exitTime)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(t.exitTime)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{fmtLabel(t.strategyName, t.strategyId, strategyNumbers)}</div>
                      <div className="font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>{t.id}</div>
                    </td>
                    <td className="px-4 py-3 font-semibold">{t.symbol}</td>
                    <td className="px-4 py-3">
                      <SideBadge side={t.side} />
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <div>{fmtRate(t.entryPrice)}</div>
                      <div className="text-zinc-400">{fmtRate(t.exitPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                      {formatTradeDuration(t.entryTime, t.exitTime)}
                    </td>
                    <td className="px-4 py-3">
                      <ExitBadge reason={t.exitReason} />
                    </td>
                    <td className={`px-4 py-3 text-right font-mono text-sm font-semibold ${t.returnPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtPct(t.returnPct, true)}
                    </td>
                    <td className={`px-4 py-3 text-right font-mono text-sm font-bold ${t.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
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

type Props = {
  actionsEnabled?: boolean;
  quotes: ForexQuoteDisplay[];
  positions: ForexPosition[];
  trades: ForexTrade[];
  strategies: ForexStrategyStatus[];
  stats: ForexEngineStats;
  reset: () => void;
};

function regimeTone(regime: ForexStrategyStatus["regime"]): BadgeTone {
  if (regime === "TRENDING_BULL") return "positive";
  if (regime === "TRENDING_BEAR") return "negative";
  if (regime === "HIGH_VOL") return "negative";
  if (regime === "RANGE") return "info";
  return "neutral";
}

export default function ForexScalper({ actionsEnabled = false, quotes, positions, trades, strategies, stats, reset }: Props) {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [isResetting, setIsResetting] = useState(false);

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  const actionButtonTitle = actionsEnabled
    ? "Reset is enabled for this paper forex desk."
    : "Locked: reset hidden. The forex engine still runs in your session.";

  const totalReturnPct = ((stats.equity - INITIAL_BALANCE) / INITIAL_BALANCE) * 100;
  const bestStrategy = [...strategies].sort((a, b) => b.totalPnl - a.totalPnl)[0] ?? null;
  const strategyNumbers = strategies.reduce<StrategyNumberMap>((map, s) => {
    map[s.name] = s.id > 0 ? s.id : 0;
    return map;
  }, {});

  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));
  const latestTrade = trades[0] ?? null;
  const totalStrategies = strategies.length;
  const activeStrategies = strategies.filter((s) => s.rosterState === "ACTIVE").length;
  const liveSlotReady = strategies.filter((s) => s.status === "READY" || s.status === "IN_POSITION").length;
  const openCount = Math.max(stats.openPositions, positions.length);
  const longCount = positions.filter((p) => p.side === "LONG").length;
  const shortCount = positions.filter((p) => p.side === "SHORT").length;
  const exposureSummary = openCount === 0 ? "No open exposure" : `${longCount} long / ${shortCount} short`;

  const currentRegime =
    strategies.find((s) => s.status === "IN_POSITION")?.regime
    ?? strategies[0]?.regime
    ?? "UNKNOWN";

  const grossProfit = trades.filter((t) => t.netPnl > 0).reduce((sum, t) => sum + t.netPnl, 0);
  const grossLoss = trades.filter((t) => t.netPnl < 0).reduce((sum, t) => sum + Math.abs(t.netPnl), 0);
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;
  const totalTrades = Math.max(stats.totalTrades, trades.length);
  const winRate = totalTrades > 0 ? (stats.totalWins / totalTrades) * 100 : 0;
  const bestTrade = trades.reduce<ForexTrade | null>((best, t) => (!best || t.netPnl > best.netPnl ? t : best), null);
  const avgHoldSecs = trades.length > 0 ? trades.reduce((sum, t) => sum + (t.holdSeconds > 0 ? t.holdSeconds : 0), 0) / trades.length : 0;

  const streak = (() => {
    if (trades.length === 0) return "0";
    const lastWasWin = trades[0].netPnl >= 0;
    let count = 0;
    for (const t of trades) {
      if ((t.netPnl >= 0) !== lastWasWin) break;
      count++;
    }
    return `${count}${lastWasWin ? "W" : "L"}`;
  })();

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!confirm("Reset the Forex paper account to $1,000,000? All history will be cleared.")) return;
    setIsResetting(true);
    reset();
    setTimeout(() => setIsResetting(false), 500);
  };

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 2xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)] items-start gap-5">
        <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
          <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-sky-500/10 blur-3xl pointer-events-none" />

          <div className="flex flex-col gap-5">
            <div className="px-1">
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                FOREX TRADING EQUITY
              </div>
              <div className="mt-4 flex flex-wrap items-end gap-4">
                <div className={`text-[clamp(2.55rem,5vw,3.35rem)] font-semibold leading-none tracking-tight ${stats.equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtUSD(stats.equity)}
                </div>
                <div className={`pb-1 text-xl font-semibold leading-none ${stats.sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtPct(totalReturnPct, true, 3)}
                </div>
              </div>
              <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
                Session PnL {fmtUSD(stats.sessionPnl, { signed: true })} · Yahoo multi-pair feed · {stats.warmingUp ? "Warming up" : "Engine live"}
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 px-1">
              <div className="flex flex-wrap gap-2">
                <BadgePill label={stats.lastUpdateAt > 0 || positions.length > 0 ? "Forex Engine Online" : "Forex Engine Starting"} tone="positive" />
                <BadgePill label={`${activeStrategies}/${totalStrategies} Roster`} tone="info" />
                <BadgePill label={`${stats.liveSymbols}/12 Pairs`} tone="info" />
                <BadgePill label="Directional Scalp" tone="warning" />
                <BadgePill label={`Regime: ${currentRegime}`} tone={regimeTone(currentRegime)} />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={handleReset}
                  disabled={!actionsEnabled || isResetting}
                  title={actionButtonTitle}
                  className="btn-danger text-sm"
                >
                  {isResetting ? "Resetting…" : "Reset Forex Account"}
                </button>
              </div>
            </div>
          </div>

          <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <CompactMetric label="Session Runtime" value={sessionRuntime} detail={`${liveSlotReady} strategies ready or in position`} accent="text-zinc-900" />
            <CompactMetric
              label="Last Closed Trade"
              value={latestTrade ? fmtUSD(latestTrade.netPnl, { signed: true }) : "No exits yet"}
              detail={latestTrade ? `${fmtLabel(latestTrade.strategyName, latestTrade.strategyId, strategyNumbers)} | ${latestTrade.exitReason.replace(/_/g, " ")}` : "Waiting for first closed scalp"}
              accent={latestTrade ? (latestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600") : "text-zinc-900"}
            />
            <CompactMetric label="Open Exposure" value={exposureSummary} detail={`${openCount} active slots · ${stats.diagnostics || "—"}`} accent="text-zinc-900" />
          </div>

          {trades.length >= 2 && (() => {
            const points: { y: number }[] = [];
            let running = INITIAL_BALANCE;
            for (const t of [...trades].reverse()) {
              running += t.netPnl;
              points.push({ y: running });
            }
            const ys = points.map((p) => p.y);
            const minY = Math.min(...ys, INITIAL_BALANCE);
            const maxY = Math.max(...ys, INITIAL_BALANCE);
            const range = maxY - minY || 1;
            const W = 400;
            const H = 56;
            const px = (i: number) => (i / Math.max(points.length - 1, 1)) * W;
            const py = (y: number) => H - ((y - minY) / range) * H;
            const pathD = points.map((p, i) => `${i === 0 ? "M" : "L"} ${px(i).toFixed(1)} ${py(p.y).toFixed(1)}`).join(" ");
            const color = running >= INITIAL_BALANCE ? "#16a34a" : "#dc2626";
            return (
              <div className="mt-4 px-1">
                <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500 mb-2">
                  Session Equity Curve · {trades.length} loaded trades
                </div>
                <svg viewBox={`0 0 ${W} ${H}`} className="w-full" style={{ height: 56, display: "block" }}>
                  <path d={pathD} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" />
                  <line x1="0" y1={py(INITIAL_BALANCE).toFixed(1)} x2={W} y2={py(INITIAL_BALANCE).toFixed(1)} stroke="#94a3b8" strokeWidth="1" strokeDasharray="4 3" />
                </svg>
              </div>
            );
          })()}
        </div>

        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="px-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">Equity And PnL</div>
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <CompactMetric label="Forex Equity" value={fmtUSD(stats.equity)} detail={`Base ${fmtUSD(INITIAL_BALANCE)}`} accent="text-zinc-900" />
            <CompactMetric label="Session PnL" value={fmtUSD(stats.sessionPnl, { signed: true })} detail={`${totalReturnPct >= 0 ? "+" : ""}${totalReturnPct.toFixed(3)}% vs base`} accent={stats.sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
            <CompactMetric label="Realized PnL" value={fmtUSD(stats.realizedPnl, { signed: true })} detail={`${stats.totalTrades} completed trades`} accent={stats.realizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
            <CompactMetric label="Cash Balance" value={fmtUSD(stats.balance)} detail={`Unrealized ${fmtUSD(stats.unrealizedPnl, { signed: true })}`} accent="text-blue-600" />
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-7 gap-4">
        <SummaryCard label="Win Rate" value={totalTrades > 0 ? `${winRate.toFixed(1)}%` : "-"} accent={winRate >= 50 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Profit Factor" value={profitFactor.toFixed(2)} accent={profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Trades" value={`${totalTrades.toLocaleString("en-US")}`} accent="text-zinc-900" />
        <SummaryCard label="Unrealized" value={fmtUSD(stats.unrealizedPnl, { signed: true })} accent={stats.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Streak" value={streak} accent="text-amber-500" />
        <SummaryCard label="Best Trade" value={bestTrade ? fmtUSD(bestTrade.netPnl, { signed: true }) : "-"} accent={bestTrade && bestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Avg Hold" value={avgHoldSecs > 0 ? formatElapsedSeconds(Math.round(avgHoldSecs)) : "-"} accent="text-zinc-900" />
      </div>

      <LivePositionsPanel positions={positions} />
      <StrategiesPanel strategies={strategies} strategyNumbers={strategyNumbers} />

      {bestStrategy ? (
        <div className="glass-panel px-6 py-5 flex flex-wrap items-center gap-6 justify-between">
          <div>
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Top Forex Strategy</div>
            <div className="mt-1 text-lg font-bold" style={{ color: "var(--text-primary)" }}>{fmtLabel(bestStrategy.name, bestStrategy.id, strategyNumbers)}</div>
            <div className="mt-0.5 text-xs" style={{ color: "var(--text-secondary)" }}>
              {bestStrategy.wins}W / {bestStrategy.losses}L | {bestStrategy.totalTrades > 0 ? `${bestStrategy.winRate.toFixed(1)}%` : "-"} win rate | score {bestStrategy.score || "—"}
            </div>
          </div>
          <div className="text-right">
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Strategy PnL</div>
            <div className={`mt-1 text-2xl font-bold ${bestStrategy.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>{fmtUSD(bestStrategy.totalPnl, { signed: true })}</div>
          </div>
          <div className="flex items-center gap-2 ml-auto">
            <SideBadge side={bestStrategy.side} />
            <RosterBadge rosterState={bestStrategy.rosterState} />
            <StatusBadge status={bestStrategy.status} />
          </div>
        </div>
      ) : null}

      <DailyPnlLedger
        trades={trades}
        initialEquity={INITIAL_BALANCE}
        title="DAILY PNL LEDGER"
        description="Realized forex PnL grouped by exit day, with returns measured from that day's opening equity."
        emptyMessage="No closed forex trades yet, so there is no daily PnL ledger to display."
        formatCurrency={fmtUSD}
      />

      <TradesPanel trades={trades} strategyNumbers={strategyNumbers} statsTotalTrades={stats.totalTrades} />

      <div className="glass-panel px-5 py-6 md:px-6">
        <div className="mb-4">
          <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Forex Pair Scanner</h2>
          <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>Live rates for 12 major currency pairs tracked for directional breakouts.</div>
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
          {quotes.map((q) => (
            <QuoteCard key={q.symbol} quote={q} />
          ))}
        </div>
      </div>

      <div className="text-center text-[11px]" style={{ color: "var(--text-muted)" }}>
        Forex paper desk · Yahoo Finance spot quotes · $1,000,000 starting capital · 1% of initial capital per trade entry (fixed notional)
      </div>
    </div>
  );
}
