"use client";
import { useEffect, useState } from "react";
import Nifty50MarketHero from "@/components/Nifty50MarketHero";
import NiftyOptionChainPanel from "@/components/NiftyOptionChainPanel";
import useNiftyMarket from "@/hooks/useNiftyMarket";
import useNiftyOptionChain from "@/hooks/useNiftyOptionChain";
import type { OptionPosition, OptionTrade, OptionStrategyStatus, OptionStats } from "@/hooks/useNiftyOptions";
import useNiftyVIX from "@/hooks/useNiftyVIX";
import useNiftyCandles, { type Candle } from "@/hooks/useNiftyCandles";
import { formatShortDate, formatShortTime } from "@/lib/time";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const INITIAL_OPTIONS_BALANCE = 1_000_000;

// ── Formatters ──────────────────────────────────────────────────────────────

function fmtUSD(n: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(n).toLocaleString("en-IN", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (signed) return `${n >= 0 ? "+" : "-"}₹${abs}`;
  return `₹${abs}`;
}

function fmtPct(n: number, signed = false, decimals = 1) {
  const s = signed ? (n >= 0 ? "+" : "") : "";
  return `${s}${Math.abs(n).toFixed(decimals)}%`;
}

function fmt(n: number, d = 2) {
  return n.toLocaleString("en-IN", { minimumFractionDigits: d, maximumFractionDigits: d });
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

  if (Number.isNaN(entry.getTime()) || Number.isNaN(exit.getTime())) {
    return "-";
  }

  const totalSeconds = Math.max(0, Math.floor((exit.getTime() - entry.getTime()) / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }

  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }

  return `${seconds}s`;
}

type StrategyNumberMap = Record<string, number>;

function resolveStrategyNumber(name: string, strategyId: number | undefined, strategyNumbers: StrategyNumberMap) {
  if (strategyId && strategyId > 0) return strategyId;
  const number = strategyNumbers[name];
  return number || 0;
}

function formatStrategyLabel(name: string, strategyId: number | undefined, strategyNumbers: StrategyNumberMap) {
  const number = resolveStrategyNumber(name, strategyId, strategyNumbers);
  return number ? `${number}. ${name}` : name;
}

// ── Design-system primitives (mirrors Dashboard) ─────────────────────────────

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

// ── Option-specific badges ───────────────────────────────────────────────────

function TypeBadge({ type }: { type: string }) {
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${
      type === "CALL"
        ? "border-emerald-500/25 bg-emerald-500/10 text-emerald-600"
        : "border-rose-500/25 bg-rose-500/10 text-rose-600"
    }`}>{type}</span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    READY:       "border-emerald-200 bg-emerald-50 text-emerald-700",
    IN_POSITION: "border-blue-200 bg-blue-50 text-blue-700",
    COOLING:     "border-amber-200 bg-amber-50 text-amber-700",
    WATCHLIST:   "border-zinc-200 bg-zinc-50 text-zinc-600",
    SHADOWING:   "border-sky-200 bg-sky-50 text-sky-700",
    DISABLED:    "border-zinc-300 bg-zinc-100 text-zinc-600",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[status] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>
      {status.replace("_", " ")}
    </span>
  );
}

function RosterBadge({ rosterState }: { rosterState: string }) {
  const map: Record<string, string> = {
    ACTIVE: "border-emerald-200 bg-emerald-50 text-emerald-700",
    WATCHLIST: "border-zinc-200 bg-zinc-50 text-zinc-600",
    DISABLED: "border-rose-200 bg-rose-50 text-rose-700",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[rosterState] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>
      {rosterState}
    </span>
  );
}

function formatMaybeDate(value?: string) {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return `${formatShortDate(value)} ${formatShortTime(value)}`;
}

function ExitBadge({ reason }: { reason: string }) {
  const map: Record<string, string> = {
    TP:     "border-emerald-200 bg-emerald-50 text-emerald-700",
    SL:     "border-rose-200 bg-rose-50 text-rose-700",
    EXPIRY: "border-zinc-200 bg-zinc-50 text-zinc-600",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[reason] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>
      {reason}
    </span>
  );
}

// ── Progress bar for premium/PnL ─────────────────────────────────────────────

function PremiumBar({ entry, current }: { entry: number; current: number }) {
  const pct = entry > 0 ? Math.min(200, Math.max(0, (current / entry) * 100)) : 0;
  const positive = current >= entry;
  return (
    <div className="w-full max-w-[80px]">
      <div className="h-1.5 w-full rounded-full overflow-hidden" style={{ background: "var(--border)" }}>
        <div
          className={`h-full rounded-full transition-all ${positive ? "bg-emerald-500" : "bg-rose-500"}`}
          style={{ width: `${Math.min(100, pct)}%` }}
        />
      </div>
    </div>
  );
}

// ── Live Positions table ─────────────────────────────────────────────────────

function LivePositionsPanel({ positions, strategyNumbers }: { positions: OptionPosition[]; strategyNumbers: StrategyNumberMap }) {
  const totalUnrealized = positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const callCount = positions.filter((position) => position.optionType === "CALL").length;
  const putCount = positions.filter((position) => position.optionType === "PUT").length;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2 className="mb-5 flex flex-wrap items-center gap-3" style={{
        fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800,
        letterSpacing: "0.14em", color: "var(--text-secondary)",
      }}>
        <span className="pill-green">LIVE</span>
        RUNNING OPTION POSITIONS
        <span style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }} className="font-mono">
          ({positions.length} active)
        </span>
      </h2>

      {positions.length === 0 ? (
        <div
          className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
          style={{
            color: "var(--text-secondary)",
            borderColor: "var(--border)",
            background: "var(--surface-2)",
          }}
        >
          No open option positions - strategies are scanning for entry signals.
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <span className="h-2 w-2 rounded-full" style={{ background: "var(--green)" }} />
              <span className="text-xs font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
                {callCount} calls | {putCount} puts
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
            <table className="w-full text-left text-sm" style={{ minWidth: 920 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Position</th>
                  <th className="px-4 py-3 font-medium">Strike</th>
                  <th className="px-4 py-3 font-medium">Premium</th>
                  <th className="px-4 py-3 font-medium">Opened</th>
                  <th className="px-4 py-3 font-medium">Greeks</th>
                  <th className="px-4 py-3 font-medium">PnL</th>
                  <th className="px-4 py-3 font-medium text-right">Progress</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((pos) => (
                  <tr key={pos.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3">
                      <div className="flex flex-col gap-1">
                        <div className="flex items-center gap-2">
                          <TypeBadge type={pos.optionType} />
                          <span className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>
                            {formatStrategyLabel(pos.strategyName, pos.strategyId, strategyNumbers)}
                          </span>
                        </div>
                        <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                          Qty {fmt(pos.quantity, 4)} | Cost {fmtUSD(pos.costBasis)}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>₹{fmt(pos.strike, 0)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>NIFTY {fmtUSD(pos.entryBtcPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>In ₹{fmt(pos.entryPremium)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>Now ₹{fmt(pos.currentPremium)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      {pos.entryTime ? (
                        <div>
                          <div className="font-mono" style={{ color: "var(--text-primary)" }}>{formatShortTime(pos.entryTime)}</div>
                          <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(pos.entryTime)}</div>
                        </div>
                      ) : (
                        <span style={{ color: "var(--text-secondary)" }}>-</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>Delta {fmt(pos.delta, 3)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>IV {fmtPct(pos.iv * 100)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="font-mono text-sm font-semibold" style={{ color: pos.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtUSD(pos.unrealizedPnl, { signed: true })}
                      </div>
                      <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                        Expiry {formatShortDate(pos.expiryTime)}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="ml-auto w-28">
                        <PremiumBar entry={pos.entryPremium} current={pos.currentPremium} />
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

// ── Strategies leaderboard ────────────────────────────────────────────────────

function StrategiesPanel({ strategies, strategyNumbers }: { strategies: OptionStrategyStatus[]; strategyNumbers: StrategyNumberMap }) {
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
          ALL {totalStrategies} STRATEGIES - LEADERBOARD
        </h2>
        {totalStrategies > topCount ? (
          <button
            type="button"
            onClick={() => setShowAll((v) => !v)}
            className="btn-gold text-xs px-4 py-1.5 min-h-[32px]"
          >
            {showAll ? `Show Top ${topCount}` : `Show All ${totalStrategies}`}
          </button>
        ) : null}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left" style={{ minWidth: 1280 }}>
          <thead>
            <tr className="border-b" style={{ borderColor: "var(--border)" }}>
              {["ID", "Strategy", "Type", "Roster", "Runtime", "Score", "Live", "Shadow", "Allocation", "Size", "PnL", "Notes"].map((h, i) => (
                <th key={h} className={`py-2 px-3 text-[10px] font-bold uppercase tracking-widest ${i >= 8 && i <= 10 ? "text-right" : ""}`}
                  style={{ color: "var(--text-muted)" }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((s, i) => (
              <tr key={s.name} className="border-b transition-colors hover:bg-black/[0.015]" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="py-2.5 px-3 text-xs font-mono" style={{ color: "var(--text-muted)" }}>{resolveStrategyNumber(s.name, s.strategyId, strategyNumbers) || i + 1}</td>
                <td className="py-2.5 px-3">
                  <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{formatStrategyLabel(s.name, s.strategyId, strategyNumbers)}</div>
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{s.category}</div>
                </td>
                <td className="py-2.5 px-3"><TypeBadge type={s.optionType} /></td>
                <td className="py-2.5 px-3"><RosterBadge rosterState={s.rosterState} /></td>
                <td className="py-2.5 px-3"><StatusBadge status={s.status} /></td>
                <td className="py-2.5 px-3 text-sm font-mono font-semibold" style={{ color: "var(--text-primary)" }}>{s.score.toFixed(1)}</td>
                <td className="py-2.5 px-3 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.totalTrades}T | {s.wins}W / {s.losses}L
                  <div className="text-[11px]">{s.totalTrades > 0 ? fmtPct(s.winRate) : "-"}</div>
                </td>
                <td className="py-2.5 px-3 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.shadowTrades}T | {s.shadowWins}W / {s.shadowLosses}L
                  <div className={`${s.shadowPnl >= 0 ? "text-emerald-600" : "text-rose-600"} text-[11px] font-semibold`}>
                    {s.shadowTrades > 0 ? fmtUSD(s.shadowPnl, { signed: true }) : "-"}
                  </div>
                </td>
                <td className="py-2.5 px-3 text-right text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.rosterState === "ACTIVE" ? fmtUSD(s.allocationUsd) : "-"}
                </td>
                <td className="py-2.5 px-3 text-right text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.rosterState === "ACTIVE" ? `${s.sizeMultiplier.toFixed(2)}x` : "-"}
                </td>
                <td className={`py-2.5 px-3 text-right text-sm font-mono font-bold ${s.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {s.totalTrades > 0 ? fmtUSD(s.totalPnl, { signed: true }) : "-"}
                </td>
                <td className="py-2.5 px-3 text-[11px]" style={{ color: "var(--text-secondary)" }}>
                  <div>{s.regime}</div>
                  {s.disableReason ? <div>{s.disableReason}</div> : null}
                  {s.disabledUntil ? <div>Until {formatMaybeDate(s.disabledUntil)}</div> : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ── Trade history ─────────────────────────────────────────────────────────────

function TradesPanel({ trades, strategyNumbers }: { trades: OptionTrade[]; strategyNumbers: StrategyNumberMap }) {
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
          OPTION TRADE HISTORY
          <span className="ml-3 font-mono font-normal" style={{ color: "var(--text-muted)", fontSize: 10 }}>({totalTrades} total)</span>
        </h2>
        {trades.length > 10 ? (
          <button
            type="button"
            onClick={() => setShowAll((current) => !current)}
            className="btn-gold min-h-[32px] px-4 py-1.5 text-xs"
          >
            {showAll ? "Show Latest 10" : `Show All ${trades.length}`}
          </button>
        ) : null}
      </div>

      {trades.length === 0 ? (
        <div className="rounded-2xl border border-dashed py-12 text-center" style={{ borderColor: "var(--border)", color: "var(--text-muted)", fontSize: 13 }}>
          No completed option trades yet.
        </div>
      ) : (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
            <SummaryCard
              label="Trades"
              value={`${totalTrades}`}
              accent="text-zinc-900"
            />
            <SummaryCard
              label="Win Rate"
              value={`${winRate.toFixed(1)}%`}
              accent={winRate >= 50 ? "text-emerald-600" : "text-rose-600"}
            />
            <SummaryCard
              label="Net PnL"
              value={fmtUSD(totalPnl, { signed: true })}
              accent={totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
            />
            <SummaryCard
              label="Profit Factor"
              value={profitFactor.toFixed(2)}
              accent={profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"}
            />
            <SummaryCard
              label="W / L"
              value={`${wins}/${losses}`}
              accent="text-zinc-900"
            />
          </div>

          <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-left text-sm" style={{ minWidth: 1040 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Time</th>
                  <th className="px-4 py-3 font-medium">Strategy</th>
                  <th className="px-4 py-3 font-medium">Contract</th>
                  <th className="px-4 py-3 font-medium">Premium</th>
                  <th className="px-4 py-3 font-medium">NIFTY Move</th>
                  <th className="px-4 py-3 font-medium">Duration</th>
                  <th className="px-4 py-3 font-medium">Reason</th>
                  <th className="px-4 py-3 font-medium text-right">Return</th>
                  <th className="px-4 py-3 font-medium text-right">Net PnL</th>
                </tr>
              </thead>
              <tbody>
                {visibleTrades.map((t) => (
                  <tr key={t.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3 text-xs">
                      <div>
                        <div className="font-mono" style={{ color: "var(--text-primary)" }}>{formatShortTime(t.exitTime)}</div>
                        <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(t.exitTime)}</div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{formatStrategyLabel(t.strategyName, t.strategyId, strategyNumbers)}</div>
                      <div className="font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>{t.id}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="flex items-center gap-2">
                        <TypeBadge type={t.optionType} />
                        <span className="font-mono" style={{ color: "var(--text-primary)" }}>₹{fmt(t.strike, 0)}</span>
                      </div>
                      <div style={{ color: "var(--text-secondary)", marginTop: 4 }}>
                        {t.expiryMins}m expiry
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>In ₹{fmt(t.entryPremium)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>Out ₹{fmt(t.exitPremium)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {fmtUSD(t.entryBtcPrice)} {"->"} {fmtUSD(t.exitBtcPrice)}
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>
                        Qty {fmt(t.quantity, 2)}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                      {formatTradeDuration(t.entryTime, t.exitTime)}
                    </td>
                    <td className="px-4 py-3"><ExitBadge reason={t.exitReason} /></td>
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

// ── Market Indicators panel ───────────────────────────────────────────────────

function fmtTime(ts: number): string {
  if (!ts) return "--:--";
  return new Date(ts).toLocaleTimeString("en-IN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function fmtVol(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(Math.round(v));
}

function VixBadge({ vix }: { vix: number }) {
  if (vix <= 0) return null;
  const label = vix > 20 ? "High Volatility" : vix > 15 ? "Elevated" : "Low Volatility";
  const cls = vix > 20
    ? "border-rose-200 bg-rose-50 text-rose-700"
    : vix > 15
      ? "border-amber-200 bg-amber-50 text-amber-700"
      : "border-emerald-200 bg-emerald-50 text-emerald-700";
  return (
    <span className={`rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] ${cls}`}>
      {label}
    </span>
  );
}

function MarketIndicatorsPanel({
  vix, vixChange, vixPct, candles,
}: {
  vix: number;
  vixChange: number;
  vixPct: number;
  candles: Candle[];
}) {
  const lastCandle = candles[candles.length - 1] ?? null;
  const recentCandles = candles.slice(-5);
  const vixColor = vix > 20 ? "text-rose-600" : vix > 15 ? "text-amber-600" : "text-emerald-600";

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-5">
        <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
          MARKET INDICATORS
        </h2>
        <span className="rounded-full border border-blue-200 bg-blue-50 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] text-blue-700">
          Source: Angel One · Live
        </span>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {/* India VIX */}
        <div className="metric-card flex flex-col gap-2 min-h-[104px]">
          <div className="metric-label">India VIX</div>
          <div className="flex flex-wrap items-center gap-2">
            <span className={`metric-value ${vixColor}`}>
              {vix > 0 ? vix.toFixed(2) : "--"}
            </span>
            {vix > 0 && <VixBadge vix={vix} />}
          </div>
          {vix > 0 && (
            <div className="text-xs font-mono" style={{ color: "var(--text-secondary)" }}>
              {vixChange >= 0 ? "+" : ""}{vixChange.toFixed(2)} ({vixPct >= 0 ? "+" : ""}{vixPct.toFixed(2)}%)
            </div>
          )}
        </div>

        {/* Today candle count + last candle */}
        <div className="metric-card flex flex-col gap-2 min-h-[104px]">
          <div className="metric-label">Today&apos;s Candles</div>
          <div className="metric-value text-zinc-900">{candles.length > 0 ? candles.length : "--"}</div>
          {lastCandle && (
            <div className="text-xs font-mono space-y-0.5" style={{ color: "var(--text-secondary)" }}>
              <div>O <span className="text-zinc-700 font-semibold">{fmt(lastCandle.open, 2)}</span>{" "}
                H <span className="text-emerald-600 font-semibold">{fmt(lastCandle.high, 2)}</span>{" "}
                L <span className="text-rose-600 font-semibold">{fmt(lastCandle.low, 2)}</span>{" "}
                C <span className="text-zinc-700 font-semibold">{fmt(lastCandle.close, 2)}</span>
              </div>
              <div>V <span className="text-zinc-700 font-semibold">{fmtVol(lastCandle.volume)}</span>{" "}
                @ {fmtTime(lastCandle.time)}
              </div>
            </div>
          )}
        </div>

        {/* Last 5 candles mini table */}
        <div className="metric-card flex flex-col gap-2 min-h-[104px] sm:col-span-2 xl:col-span-1">
          <div className="metric-label">Recent Candles (last 5)</div>
          {recentCandles.length === 0 ? (
            <div className="text-xs" style={{ color: "var(--text-muted)" }}>Waiting for data...</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[11px] font-mono tabular-nums" style={{ minWidth: 320 }}>
                <thead>
                  <tr className="text-[10px] uppercase tracking-wide" style={{ color: "var(--text-muted)" }}>
                    <th className="pb-1 text-left font-semibold pr-2">Time</th>
                    <th className="pb-1 text-right font-semibold pr-2">O</th>
                    <th className="pb-1 text-right font-semibold text-emerald-600 pr-2">H</th>
                    <th className="pb-1 text-right font-semibold text-rose-600 pr-2">L</th>
                    <th className="pb-1 text-right font-semibold pr-2">C</th>
                    <th className="pb-1 text-right font-semibold">Vol</th>
                  </tr>
                </thead>
                <tbody>
                  {[...recentCandles].reverse().map((c) => (
                    <tr key={c.time} style={{ color: "var(--text-secondary)" }}>
                      <td className="pr-2 py-0.5">{fmtTime(c.time)}</td>
                      <td className="pr-2 text-right text-zinc-700">{fmt(c.open, 0)}</td>
                      <td className="pr-2 text-right text-emerald-600">{fmt(c.high, 0)}</td>
                      <td className="pr-2 text-right text-rose-600">{fmt(c.low, 0)}</td>
                      <td className={`pr-2 text-right font-semibold ${c.close >= c.open ? "text-emerald-600" : "text-rose-600"}`}>
                        {fmt(c.close, 0)}
                      </td>
                      <td className="text-right">{fmtVol(c.volume)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Main export ───────────────────────────────────────────────────────────────

type Nifty50OptionScalperProps = {
  actionsEnabled?: boolean;
  // Engine data lifted from TradingDashboard so header cards stay in sync
  positions: OptionPosition[];
  trades: OptionTrade[];
  strategies: OptionStrategyStatus[];
  stats: OptionStats | null;
  clearAll: () => void;
  barCount: number;
  enginePrice: number;
  onRefresh?: () => void;
};

export default function Nifty50OptionScalper({
  actionsEnabled = false,
  positions,
  trades,
  strategies,
  stats,
  clearAll,
  barCount,
  enginePrice,
  onRefresh,
}: Nifty50OptionScalperProps) {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [isResetting, setIsResetting] = useState(false);
  const market = useNiftyMarket();
  const optionChain = useNiftyOptionChain();
  const { vix, change: vixChange, percentChange: vixPct } = useNiftyVIX();
  const { candles } = useNiftyCandles();
  const actionButtonTitle = actionsEnabled
    ? "Action buttons are enabled."
    : "Set Action to Yes to enable reset and clear buttons.";

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  const handleReset = async () => {
    if (!actionsEnabled) {
      return;
    }
    if (!confirm("Reset the NIFTY 50 options paper account to ₹1,000,000? All history will be cleared.")) {
      return;
    }

    setIsResetting(true);
    clearAll();
    try {
      const response = await fetch(`${API_URL}/api/nifty-options/reset`, { method: "POST" });
      if (!response.ok) {
        throw new Error("reset failed");
      }
      onRefresh?.();
    } catch {
      window.alert("NIFTY options account reset failed. Check engine connectivity.");
    } finally {
      setIsResetting(false);
    }
  };

  // ── Derived values ──────────────────────────────────────────────
  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));
  const closedPnl = stats?.totalPnl ?? trades.reduce((sum, trade) => sum + trade.netPnl, 0);
  const unrealized = stats?.unrealizedPnl ?? positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const sessionPnl = closedPnl + unrealized;
  const equity = INITIAL_OPTIONS_BALANCE + sessionPnl;
  const totalReturnPct = (sessionPnl / INITIAL_OPTIONS_BALANCE) * 100;
  const grossProfit = trades.filter((trade) => trade.netPnl > 0).reduce((sum, trade) => sum + trade.netPnl, 0);
  const grossLoss = trades.filter((trade) => trade.netPnl < 0).reduce((sum, trade) => sum + Math.abs(trade.netPnl), 0);
  const totalTrades = Math.max(stats?.totalTrades ?? 0, trades.length);
  const totalWins = stats?.totalWins ?? trades.filter((trade) => trade.netPnl >= 0).length;
  const winRate = totalTrades > 0 ? (totalWins / totalTrades) * 100 : 0;
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;
  const openCount = Math.max(stats?.openPositions ?? 0, positions.length);
  const callCount = positions.filter((p) => p.optionType === "CALL").length;
  const putCount  = positions.filter((p) => p.optionType === "PUT").length;
  const exposureSummary = openCount === 0 ? "No open exposure" : `${callCount} calls / ${putCount} puts`;
  const bestStrategy = [...strategies].sort((a, b) => {
    if (b.totalPnl !== a.totalPnl) return b.totalPnl - a.totalPnl;
    return b.score - a.score;
  })[0] ?? null;
  const latestTrade = trades[0] ?? null;
  const totalStrategies = strategies.length;
  const activeStrategies = strategies.length === 0
    ? 0
    : strategies.filter((s) => s.rosterState === "ACTIVE").length;
  const strategyNumbers = strategies.reduce<StrategyNumberMap>((map, strategy, index) => {
    map[strategy.name] = strategy.strategyId > 0 ? strategy.strategyId : index + 1;
    return map;
  }, {});
  const currentRegime = strategies.length > 0 ? (strategies[0]?.regime ?? "UNKNOWN") : "UNKNOWN";
  const bestTrade = trades.reduce<OptionTrade | null>((best, t) => (!best || t.netPnl > best.netPnl ? t : best), null);
  const avgHoldSecs = trades.length > 0
    ? trades.reduce((sum, t) => {
        const entry = new Date(t.entryTime).getTime();
        const exit = new Date(t.exitTime).getTime();
        return sum + (Number.isNaN(exit - entry) ? 0 : (exit - entry) / 1000);
      }, 0) / trades.length
    : 0;

  // ── Streak ──────────────────────────────────────────────────────
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
  const displayedEnginePrice = enginePrice > 0 ? enginePrice : market.price;

  return (
    <div className="space-y-5">
      <Nifty50MarketHero
        market={market}
        subtitle="Live NIFTY 50 index chart from NSE for the options scalper's underlying spot feed."
        vix={vix}
        currentTime={currentTime}
      />

      <MarketIndicatorsPanel vix={vix} vixChange={vixChange} vixPct={vixPct} candles={candles} />

      {/* ── Hero: Options Equity ── */}
      <div className="grid grid-cols-1 2xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)] items-start gap-5">

        {/* Left: price-hero card */}
        <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
          <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-amber-500/10 blur-3xl pointer-events-none" />

          <div className="flex flex-col gap-5">
            <div className="px-1">
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                NIFTY 50 OPTION EQUITY
              </div>
              <div className="mt-4 flex flex-wrap items-end gap-4">
                <div className={`text-[clamp(2.55rem,5vw,3.35rem)] font-semibold leading-none tracking-tight ${equity >= INITIAL_OPTIONS_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtUSD(equity)}
                </div>
                <div className={`pb-1 text-xl font-semibold leading-none ${sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtPct(totalReturnPct, true, 2)}
                </div>
              </div>
              <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
                Session PnL {fmtUSD(sessionPnl, { signed: true })}
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 px-1">
              <div className="flex flex-wrap gap-2">
                <BadgePill label={displayedEnginePrice > 0 ? `Feed ₹${displayedEnginePrice.toFixed(0)}` : "Feed: Connecting…"} tone={displayedEnginePrice > 0 ? "positive" : "neutral"} />
                <BadgePill label={barCount >= 15 ? `${barCount} bars` : `Warming ${barCount}/15 bars`} tone={barCount >= 15 ? "info" : "warning"} />
                <BadgePill label={`${activeStrategies}/${totalStrategies} Live`} tone="info" />
                <BadgePill label="Separate Account" tone="warning" />
                <BadgePill label="Not Futures" tone="neutral" />
                <BadgePill
                  label={`Regime: ${currentRegime}`}
                  tone={currentRegime === "TREND" ? "positive" : currentRegime === "VOLATILE" ? "negative" : currentRegime === "RANGE" ? "info" : "neutral"}
                />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  disabled={!actionsEnabled || isResetting}
                  title={actionButtonTitle}
                  className="btn-primary text-sm"
                  onClick={async () => {
                    if (!actionsEnabled) return;
                    if (!confirm("Clear completed NIFTY option trades and strategy stats? Open positions and balance will be kept.")) return;
                    clearAll();
                    try {
                      const response = await fetch(`${API_URL}/api/nifty-options/clear-history`, { method: "POST" });
                      if (!response.ok) {
                        throw new Error("clear history failed");
                      }
                      onRefresh?.();
                    } catch {
                      window.alert("Clearing NIFTY option history failed. Check engine connectivity.");
                    }
                  }}
                >
                  Clear NIFTY Trades
                </button>
                <button
                  type="button"
                  onClick={handleReset}
                  disabled={!actionsEnabled || isResetting}
                  title={actionButtonTitle}
                  className="btn-danger text-sm"
                >
                  {isResetting ? "Resetting..." : "Reset NIFTY Account"}
                </button>
              </div>
            </div>
          </div>

          <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <CompactMetric
              label="Session Runtime"
              value={sessionRuntime}
              detail={`${activeStrategies} strategies funded`}
              accent="text-zinc-900"
            />
            <CompactMetric
              label="Last Closed Trade"
              value={latestTrade ? fmtUSD(latestTrade.netPnl, { signed: true }) : "No exits yet"}
              detail={latestTrade ? `${formatStrategyLabel(latestTrade.strategyName, latestTrade.strategyId, strategyNumbers)} | ${latestTrade.exitReason}` : "Waiting for first completed options cycle"}
              accent={latestTrade ? (latestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600") : "text-zinc-900"}
            />
            <CompactMetric
              label="Open Exposure"
              value={exposureSummary}
              detail={`${openCount} of ${activeStrategies} live slots in position`}
              accent="text-zinc-900"
            />
          </div>

          {/* ── Equity sparkline ── */}
          {trades.length >= 2 && (() => {
            const initial = INITIAL_OPTIONS_BALANCE;
            const points: { x: number; y: number }[] = [];
            let running = initial;
            const sorted = [...trades].reverse();
            for (const t of sorted) {
              running += t.netPnl;
              points.push({ x: 0, y: running });
            }
            const ys = points.map((p) => p.y);
            const minY = Math.min(...ys, initial);
            const maxY = Math.max(...ys, initial);
            const range = maxY - minY || 1;
            const W = 400; const H = 56;
            const px = (i: number) => (i / Math.max(points.length - 1, 1)) * W;
            const py = (y: number) => H - ((y - minY) / range) * H;
            const pathD = points.map((p, i) => `${i === 0 ? "M" : "L"} ${px(i).toFixed(1)} ${py(p.y).toFixed(1)}`).join(" ");
            const color = running >= initial ? "#16a34a" : "#dc2626";
            return (
              <div className="mt-4 px-1">
                <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500 mb-2">
                  Equity Curve · {trades.length} trades
                </div>
                <svg viewBox={`0 0 ${W} ${H}`} className="w-full" style={{ height: 56, display: "block" }}>
                  <path d={pathD} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" />
                  <line x1="0" y1={py(initial).toFixed(1)} x2={W} y2={py(initial).toFixed(1)} stroke="#94a3b8" strokeWidth="1" strokeDasharray="4 3" />
                </svg>
              </div>
            );
          })()}
        </div>

        {/* Right: Equity & PnL grid */}
        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="px-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">
            Equity And PnL
          </div>
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <CompactMetric
              label="Options Equity"
              value={fmtUSD(equity)}
              detail={`Base ${fmtUSD(INITIAL_OPTIONS_BALANCE)}`}
              accent="text-zinc-900"
            />
            <CompactMetric
              label="Net PnL"
              value={fmtUSD(sessionPnl, { signed: true })}
              detail={`${totalReturnPct >= 0 ? "+" : ""}${totalReturnPct.toFixed(2)}% vs base`}
              accent={sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
            />
            <CompactMetric
              label="Closed PnL"
              value={fmtUSD(closedPnl, { signed: true })}
              detail={`${totalTrades} completed trades`}
              accent={closedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
            />
            <CompactMetric
              label="Open Positions"
              value={`${openCount}`}
              detail={exposureSummary}
              accent="text-blue-600"
            />
          </div>
        </div>
      </div>

      {/* ── Summary stats row ── */}
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-7 gap-4">
        <SummaryCard
          label="Win Rate"
          value={totalTrades > 0 ? `${winRate.toFixed(1)}%` : "-"}
          accent={winRate >= 50 ? "text-emerald-600" : "text-rose-600"}
        />
        <SummaryCard
          label="Profit Factor"
          value={profitFactor.toFixed(2)}
          accent={profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"}
        />
        <SummaryCard
          label="Trades"
          value={`${totalTrades}`}
          accent="text-zinc-900"
        />
        <SummaryCard
          label="Unrealized"
          value={fmtUSD(unrealized, { signed: true })}
          accent={unrealized >= 0 ? "text-emerald-600" : "text-rose-600"}
        />
        <SummaryCard
          label="Streak"
          value={streak}
          accent="text-amber-500"
        />
        <SummaryCard
          label="Best Trade"
          value={bestTrade ? fmtUSD(bestTrade.netPnl, { signed: true }) : "-"}
          accent={bestTrade && bestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
        />
        <SummaryCard
          label="Avg Hold"
          value={avgHoldSecs > 0 ? formatElapsedSeconds(Math.round(avgHoldSecs)) : "-"}
          accent="text-zinc-900"
        />
      </div>

      {/* ── Live positions ── */}
      <NiftyOptionChainPanel
        data={optionChain.data}
        loading={optionChain.loading}
        error={optionChain.error}
        lastUpdatedAt={optionChain.lastUpdatedAt}
        currentTime={currentTime}
        selectedExpiry={optionChain.selectedExpiry}
        selectExpiry={optionChain.selectExpiry}
      />
      <LivePositionsPanel positions={positions} strategyNumbers={strategyNumbers} />

      {/* ── Strategies leaderboard ── */}
      <StrategiesPanel strategies={strategies} strategyNumbers={strategyNumbers} />

      {/* ── Best strategy callout ── */}
      {bestStrategy && (
        <div className="glass-panel px-6 py-5 flex flex-wrap items-center gap-6 justify-between">
          <div>
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Top Performing Strategy</div>
            <div className="mt-1 text-lg font-bold" style={{ color: "var(--text-primary)" }}>{formatStrategyLabel(bestStrategy.name, bestStrategy.strategyId, strategyNumbers)}</div>
            <div className="mt-0.5 text-xs" style={{ color: "var(--text-secondary)" }}>
              {bestStrategy.wins}W / {bestStrategy.losses}L | {bestStrategy.totalTrades > 0 ? fmtPct(bestStrategy.winRate) : "-"} win rate | score {bestStrategy.score.toFixed(1)}
            </div>
          </div>
          <div className="text-right">
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Strategy PnL</div>
            <div className={`mt-1 text-2xl font-bold ${bestStrategy.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>{fmtUSD(bestStrategy.totalPnl, { signed: true })}</div>
          </div>
          <div className="flex items-center gap-2 ml-auto">
            <TypeBadge type={bestStrategy.optionType} />
            <RosterBadge rosterState={bestStrategy.rosterState} />
            <StatusBadge status={bestStrategy.status} />
          </div>
        </div>
      )}

      {/* ── Trade history ── */}
      <TradesPanel trades={trades} strategyNumbers={strategyNumbers} />

      {/* ── Footer note ── */}
      <div className="text-center text-[11px]" style={{ color: "var(--text-muted)" }}>
        NIFTY 50 options paper account · Black-Scholes pricing · live NSE NIFTY 50 spot feed · ₹1,000,000 starting balance · 1% capital per trade
      </div>

    </div>
  );
}
