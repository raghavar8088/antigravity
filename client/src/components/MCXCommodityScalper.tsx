"use client";
import { useEffect, useMemo, useState } from "react";
import DailyPnlLedger from "@/components/DailyPnlLedger";
import useMCXEngine from "@/hooks/useMCXEngine";
import type { MCXPosition, MCXTrade, MCXStrategyStatus, MCXCommodityQuote } from "@/hooks/useMCXEngine";
import { MCX_COMMODITIES } from "@/lib/trading/mcxCommodities";
import { formatShortDate, formatShortTime } from "@/lib/utils/time";

const INITIAL_MCX_BALANCE = 1_000_000;

// ── Formatters ───────────────────────────────────────────────────────────────

function fmtINR(n: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(n).toLocaleString("en-IN", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
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
  const h = Math.floor(total / 3600), m = Math.floor((total % 3600) / 60), s = total % 60;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m ${s}s`;
}

function formatDuration(entryTime: string, exitTime: string) {
  const diff = Math.max(0, Math.floor((new Date(exitTime).getTime() - new Date(entryTime).getTime()) / 1000));
  const h = Math.floor(diff / 3600), m = Math.floor((diff % 3600) / 60), s = diff % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function fmtMaybeDate(v?: string) {
  if (!v) return "-";
  const d = new Date(v);
  if (isNaN(d.getTime())) return "-";
  return `${formatShortDate(v)} ${formatShortTime(v)}`;
}

// ── Design primitives ────────────────────────────────────────────────────────

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
  return <span className={`inline-flex items-center rounded-full border px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] ${map[tone]}`}>{label}</span>;
}

function TypeBadge({ type }: { type: string }) {
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${
      type === "CALL" ? "border-emerald-500/25 bg-emerald-500/10 text-emerald-600" : "border-rose-500/25 bg-rose-500/10 text-rose-600"
    }`}>{type}</span>
  );
}

function RosterBadge({ rosterState, shadowTrades, shadowWins }: { rosterState: string; shadowTrades: number; shadowWins: number }) {
  if (rosterState === "ACTIVE") return <span className="rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest border-emerald-500/25 bg-emerald-500/10 text-emerald-600">ACTIVE</span>;
  if (rosterState === "DISABLED") return <span className="rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest border-rose-500/25 bg-rose-500/10 text-rose-600">DISABLED</span>;
  // WATCHLIST
  const pct = shadowTrades > 0 ? Math.round((shadowWins / shadowTrades) * 100) : 0;
  const label = shadowTrades > 0 ? `WATCH ${shadowWins}/${shadowTrades} ${pct}%` : "WATCH";
  return <span className="rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest border-amber-500/25 bg-amber-500/10 text-amber-600">{label}</span>;
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    READY:       "border-emerald-200 bg-emerald-50 text-emerald-700",
    IN_POSITION: "border-blue-200 bg-blue-50 text-blue-700",
    COOLING:     "border-amber-200 bg-amber-50 text-amber-700",
    WARMING:     "border-zinc-200 bg-zinc-50 text-zinc-600",
    BULLISH:     "border-emerald-200 bg-emerald-50 text-emerald-700",
    BEARISH:     "border-rose-200 bg-rose-50 text-rose-700",
    NEUTRAL:     "border-blue-200 bg-blue-50 text-blue-700",
  };
  return <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[status] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>{status.replace("_", " ")}</span>;
}

function ExitBadge({ reason }: { reason: string }) {
  const map: Record<string, string> = {
    TP:     "border-emerald-200 bg-emerald-50 text-emerald-700",
    SL:     "border-rose-200 bg-rose-50 text-rose-700",
    EXPIRY: "border-zinc-200 bg-zinc-50 text-zinc-600",
  };
  return <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[reason] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>{reason}</span>;
}

function PremiumBar({ entry, current }: { entry: number; current: number }) {
  const pct = entry > 0 ? Math.min(200, Math.max(0, (current / entry) * 100)) : 0;
  return (
    <div className="w-full max-w-[80px]">
      <div className="h-1.5 w-full rounded-full overflow-hidden" style={{ background: "var(--border)" }}>
        <div className={`h-full rounded-full transition-all ${current >= entry ? "bg-emerald-500" : "bg-rose-500"}`}
          style={{ width: `${Math.min(100, pct)}%` }} />
      </div>
    </div>
  );
}

// ── Commodity quote card ─────────────────────────────────────────────────────

function CommodityCard({ quote }: { quote: MCXCommodityQuote }) {
  const commodity = MCX_COMMODITIES.find((c) => c.id === quote.id);
  const isUp = quote.change >= 0;
  const hasData = quote.ltp > 0;
  const biasTone: BadgeTone = quote.bias === "BULLISH" ? "positive" : quote.bias === "BEARISH" ? "negative" : quote.bias === "NEUTRAL" ? "info" : "warning";

  return (
    <div className={`rounded-2xl border p-4 ${commodity?.borderClass ?? "border-zinc-700/40"} ${commodity?.bgClass ?? "bg-zinc-900/40"}`}>
      <div className="flex items-start justify-between gap-2 mb-2">
        <div>
          <div className="text-[10px] font-bold uppercase tracking-[0.16em] text-zinc-500">{quote.sector}</div>
          <div className={`text-sm font-bold mt-0.5 ${commodity?.colorClass ?? "text-white"}`}>{quote.name}</div>
        </div>
        <div className={`text-[10px] font-bold rounded-full px-2 py-0.5 border ${
          hasData
            ? isUp ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-400" : "border-rose-500/30 bg-rose-500/10 text-rose-400"
            : "border-zinc-600/40 bg-zinc-800/40 text-zinc-500"
        }`}>
          {hasData ? (isUp ? "+" : "") + fmtPct(quote.changePct, false) : "—"}
        </div>
      </div>
      <div className={`text-xl font-bold font-mono ${hasData ? (isUp ? "text-emerald-300" : "text-rose-300") : "text-zinc-500"}`}>
        {hasData ? fmt(quote.ltp, 2) : "Connecting…"}
      </div>
      <div className="mt-0.5 text-[10px] font-medium text-zinc-500">{quote.unit}</div>
      {hasData && (
        <div className="mt-1 text-[10px] font-mono text-zinc-500 space-y-0.5">
          <div className="flex gap-2">
            <span>O: {fmt(quote.open, 1)}</span>
            <span className="text-emerald-600">H: {fmt(quote.high, 1)}</span>
            <span className="text-rose-600">L: {fmt(quote.low, 1)}</span>
          </div>
        </div>
      )}
      <div className="mt-2 flex items-center gap-1.5">
        <div className={`h-1.5 w-1.5 rounded-full ${quote.barCount >= 15 ? "bg-emerald-400" : "bg-amber-400"} animate-pulse`} />
        <span className="text-[9px] text-zinc-500 font-mono">{quote.barCount >= 15 ? `${quote.barCount} bars` : `Warming ${quote.barCount}/15`}</span>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        <BadgePill label={quote.bias} tone={biasTone} />
        <span className="rounded-full border border-zinc-200 bg-white px-2 py-1 text-[9px] font-mono text-zinc-500">RSI {fmt(quote.rsi, 0)}</span>
        <span className="rounded-full border border-zinc-200 bg-white px-2 py-1 text-[9px] font-mono text-zinc-500">VOL {fmtPct(quote.volatilityPct)}</span>
      </div>
    </div>
  );
}

// ── Positions panel ──────────────────────────────────────────────────────────

type CommodityDeskRow = {
  id: string;
  name: string;
  sector: string;
  colorClass: string;
  ltp: number;
  changePct: number;
  bias: MCXCommodityQuote["bias"];
  rsi: number;
  momentumPct: number;
  volatilityPct: number;
  barCount: number;
  openPositions: number;
  exposure: number;
  realizedPnl: number;
  unrealizedPnl: number;
  totalPnl: number;
  trades: number;
  wins: number;
  winRate: number;
  activeStrategies: number;
  watchlistStrategies: number;
  disabledStrategies: number;
};

type SectorExposureRow = {
  sector: string;
  exposure: number;
  openPositions: number;
  realizedPnl: number;
  unrealizedPnl: number;
  totalPnl: number;
};

function CommodityDeskPanel({ rows }: { rows: CommodityDeskRow[] }) {
  const sorted = [...rows].sort((a, b) => Math.abs(b.totalPnl) - Math.abs(a.totalPnl));

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2 className="mb-5 flex flex-wrap items-center gap-3" style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
        COMMODITY DESK MATRIX
        <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>({rows.length} instruments)</span>
      </h2>

      <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
        <table className="w-full text-left text-sm" style={{ minWidth: 980 }}>
          <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            <tr className="text-[11px] uppercase tracking-[0.12em]">
              {["Commodity", "Bias", "Move", "RSI", "Vol", "Open", "Exposure", "Realized", "Unrealized", "Total P&L", "Trades", "Roster"].map((h) => (
                <th key={h} className="px-3 py-2 font-medium whitespace-nowrap">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map((row) => (
              <tr key={row.id} className="border-t hover:bg-white/[0.02]" style={{ borderColor: "var(--border)" }}>
                <td className="px-3 py-2">
                  <div className={`text-xs font-bold ${row.colorClass}`}>{row.name}</div>
                  <div className="text-[10px]" style={{ color: "var(--text-muted)" }}>{row.sector}</div>
                </td>
                <td className="px-3 py-2"><StatusBadge status={row.bias} /></td>
                <td className="px-3 py-2 font-mono text-xs" style={{ color: row.changePct >= 0 ? "var(--green)" : "var(--red)" }}>{row.ltp > 0 ? fmtPct(row.changePct, true) : "-"}</td>
                <td className="px-3 py-2 font-mono text-xs text-zinc-500">{fmt(row.rsi, 0)}</td>
                <td className="px-3 py-2 font-mono text-xs text-zinc-500">{fmtPct(row.volatilityPct)}</td>
                <td className="px-3 py-2 font-mono text-xs text-zinc-500">{row.openPositions}</td>
                <td className="px-3 py-2 font-mono text-xs text-zinc-500">{fmtINR(row.exposure)}</td>
                <td className="px-3 py-2 font-mono text-xs" style={{ color: row.realizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>{row.realizedPnl !== 0 ? fmtINR(row.realizedPnl, { signed: true }) : "-"}</td>
                <td className="px-3 py-2 font-mono text-xs" style={{ color: row.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>{row.unrealizedPnl !== 0 ? fmtINR(row.unrealizedPnl, { signed: true }) : "-"}</td>
                <td className="px-3 py-2 font-mono text-xs font-bold" style={{ color: row.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>{row.totalPnl !== 0 ? fmtINR(row.totalPnl, { signed: true }) : "-"}</td>
                <td className="px-3 py-2 font-mono text-xs text-zinc-500">{row.trades > 0 ? `${row.trades} / ${fmtPct(row.winRate)}` : "-"}</td>
                <td className="px-3 py-2 font-mono text-xs text-zinc-500">{row.activeStrategies}A / {row.watchlistStrategies}W / {row.disabledStrategies}D</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function SectorExposurePanel({ rows }: { rows: SectorExposureRow[] }) {
  const totalExposure = rows.reduce((sum, row) => sum + row.exposure, 0);
  const sorted = [...rows].sort((a, b) => b.exposure - a.exposure || Math.abs(b.totalPnl) - Math.abs(a.totalPnl));

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2 className="mb-5 flex flex-wrap items-center gap-3" style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
        SECTOR EXPOSURE
        <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>({sorted.length} groups)</span>
      </h2>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
        {sorted.map((row) => {
          const share = totalExposure > 0 ? (row.exposure / totalExposure) * 100 : 0;
          return (
            <div key={row.sector} className="rounded-[18px] border p-4" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-[10px] font-bold uppercase tracking-[0.16em]" style={{ color: "var(--text-muted)" }}>{row.sector}</div>
                  <div className="mt-1 font-mono text-lg font-semibold" style={{ color: row.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>{row.totalPnl !== 0 ? fmtINR(row.totalPnl, { signed: true }) : fmtINR(0)}</div>
                </div>
                <BadgePill label={`${row.openPositions} open`} tone={row.openPositions > 0 ? "info" : "neutral"} />
              </div>
              <div className="mt-4">
                <div className="mb-1 flex items-center justify-between text-[10px] font-mono" style={{ color: "var(--text-secondary)" }}>
                  <span>{fmtINR(row.exposure)} deployed</span>
                  <span>{fmtPct(share)}</span>
                </div>
                <div className="h-1.5 w-full overflow-hidden rounded-full" style={{ background: "var(--border)" }}>
                  <div className="h-full rounded-full bg-blue-500" style={{ width: `${Math.min(100, share)}%` }} />
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function PositionsPanel({ positions }: { positions: MCXPosition[] }) {
  const unrealized = positions.reduce((s, p) => s + p.unrealizedPnl, 0);
  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2 className="mb-5 flex flex-wrap items-center gap-3" style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
        <span className="pill-green">LIVE</span>
        RUNNING COMMODITY OPTION POSITIONS
        <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>({positions.length} active)</span>
      </h2>

      {positions.length === 0 ? (
        <div className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
          style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}>
          No open positions — engine is scanning for entry signals across {MCX_COMMODITIES.length} commodities.
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span className="text-xs font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
              {positions.filter((p) => p.optionType === "CALL").length} calls | {positions.filter((p) => p.optionType === "PUT").length} puts
            </span>
            <span className="rounded-full border px-3 py-1 text-xs font-medium"
              style={{ background: unrealized >= 0 ? "var(--green-dim)" : "var(--red-dim)", color: unrealized >= 0 ? "var(--green)" : "var(--red)", borderColor: unrealized >= 0 ? "rgba(24,128,56,0.14)" : "rgba(217,48,37,0.14)" }}>
              Unrealized {fmtINR(unrealized, { signed: true })}
            </span>
          </div>
          <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-left text-sm" style={{ minWidth: 1000 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  {["Commodity", "Strategy", "Type", "Strike", "Entry ₹", "Mark ₹", "TP ₹", "SL ₹", "Lots", "Cost", "Unrealized P&L", "Progress"].map((h) => (
                    <th key={h} className="px-4 py-3 font-medium whitespace-nowrap">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {positions.map((pos) => {
                  const commodity = MCX_COMMODITIES.find((c) => c.id === pos.commodityId);
                  return (
                    <tr key={pos.id} className="border-t hover:bg-white/[0.02]" style={{ borderColor: "var(--border)" }}>
                      <td className="px-4 py-3">
                        <span className={`text-xs font-bold ${commodity?.colorClass ?? "text-white"}`}>{pos.commodityName}</span>
                      </td>
                      <td className="px-4 py-3 font-mono text-[11px] text-zinc-400">{pos.strategyName}</td>
                      <td className="px-4 py-3"><TypeBadge type={pos.optionType} /></td>
                      <td className="px-4 py-3 font-mono text-xs text-zinc-300">{fmt(pos.strike, 1)}</td>
                      <td className="px-4 py-3 font-mono text-xs text-zinc-300">{fmt(pos.entryPremium, 2)}</td>
                      <td className="px-4 py-3 font-mono text-xs font-bold" style={{ color: pos.currentPremium >= pos.entryPremium ? "var(--green)" : "var(--red)" }}>
                        {fmt(pos.currentPremium, 2)}
                      </td>
                      <td className="px-4 py-3 font-mono text-[11px] text-emerald-600">{fmt(pos.tpPremium, 2)}</td>
                      <td className="px-4 py-3 font-mono text-[11px] text-rose-600">{fmt(pos.slPremium, 2)}</td>
                      <td className="px-4 py-3 font-mono text-xs text-zinc-400">{pos.lots}</td>
                      <td className="px-4 py-3 font-mono text-xs text-zinc-400">{fmtINR(pos.costBasis)}</td>
                      <td className="px-4 py-3 font-mono text-xs font-bold" style={{ color: pos.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtINR(pos.unrealizedPnl, { signed: true })} ({fmtPct(pos.costBasis > 0 ? (pos.unrealizedPnl / pos.costBasis) * 100 : 0, true)})
                      </td>
                      <td className="px-4 py-3"><PremiumBar entry={pos.entryPremium} current={pos.currentPremium} /></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Strategy roster ──────────────────────────────────────────────────────────

function StrategyRoster({ strategies }: { strategies: MCXStrategyStatus[] }) {
  const byCommodity = MCX_COMMODITIES.map((c) => ({
    commodity: c,
    strats: strategies.filter((s) => s.commodityId === c.id),
  }));

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2 className="mb-5 flex flex-wrap items-center gap-3" style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
        STRATEGY ROSTER
        <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>
          ({strategies.filter((s) => s.rosterState === "ACTIVE").length} active / {strategies.filter((s) => s.rosterState === "WATCHLIST").length} watch / {strategies.length} total)
        </span>
      </h2>

      <div className="space-y-6">
        {byCommodity.map(({ commodity, strats }) => (
          <div key={commodity.id}>
            <div className={`text-[10px] font-bold uppercase tracking-[0.18em] mb-2 ${commodity.colorClass}`}>
              {commodity.name} — {commodity.sector}
            </div>
            <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
              <table className="w-full text-left text-sm" style={{ minWidth: 720 }}>
                <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                  <tr className="text-[11px] uppercase tracking-[0.12em]">
                    {["#", "Strategy", "Type", "Roster", "Status", "Trades", "Win Rate", "Total P&L"].map((h) => (
                      <th key={h} className="px-3 py-2 font-medium">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {strats.map((s) => (
                    <tr key={s.strategyId} className="border-t hover:bg-white/[0.02]" style={{ borderColor: "var(--border)" }}>
                      <td className="px-3 py-2 font-mono text-[11px] text-zinc-500">{s.strategyId}</td>
                      <td className="px-3 py-2 font-mono text-xs text-zinc-300">{s.name}</td>
                      <td className="px-3 py-2"><TypeBadge type={s.optionType} /></td>
                      <td className="px-3 py-2"><RosterBadge rosterState={s.rosterState} shadowTrades={s.shadowTrades} shadowWins={s.shadowWins} /></td>
                      <td className="px-3 py-2"><StatusBadge status={s.status} /></td>
                      <td className="px-3 py-2 font-mono text-xs text-zinc-400">{s.totalTrades}</td>
                      <td className="px-3 py-2 font-mono text-xs" style={{ color: s.winRate >= 50 ? "var(--green)" : s.totalTrades > 0 ? "var(--red)" : "var(--text-muted)" }}>
                        {s.totalTrades > 0 ? fmtPct(s.winRate) : "-"}
                      </td>
                      <td className="px-3 py-2 font-mono text-xs font-bold" style={{ color: s.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {s.totalPnl !== 0 ? fmtINR(s.totalPnl, { signed: true }) : "-"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Trade ledger ─────────────────────────────────────────────────────────────

function TradeLedger({ trades }: { trades: MCXTrade[] }) {
  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2 className="mb-5 flex flex-wrap items-center gap-3" style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
        COMPLETED TRADES
        <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>({trades.length} trades)</span>
      </h2>

      {trades.length === 0 ? (
        <div className="flex min-h-[120px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm"
          style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}>
          No completed trades yet.
        </div>
      ) : (
        <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
          <table className="w-full text-left text-sm" style={{ minWidth: 1020 }}>
            <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
              <tr className="text-[11px] uppercase tracking-[0.12em]">
                {["Time", "Commodity", "Strategy", "Type", "Strike", "Entry ₹", "Exit ₹", "Lots", "Net P&L", "Return", "Duration", "Exit"].map((h) => (
                  <th key={h} className="px-3 py-2 font-medium whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {trades.map((t) => {
                const commodity = MCX_COMMODITIES.find((c) => c.id === t.commodityId);
                return (
                  <tr key={t.id} className="border-t hover:bg-white/[0.02]" style={{ borderColor: "var(--border)" }}>
                    <td className="px-3 py-2 font-mono text-[11px] text-zinc-500">{fmtMaybeDate(t.exitTime)}</td>
                    <td className="px-3 py-2"><span className={`text-xs font-bold ${commodity?.colorClass ?? "text-white"}`}>{t.commodityName}</span></td>
                    <td className="px-3 py-2 font-mono text-[11px] text-zinc-400">{t.strategyName}</td>
                    <td className="px-3 py-2"><TypeBadge type={t.optionType} /></td>
                    <td className="px-3 py-2 font-mono text-xs text-zinc-300">{fmt(t.strike, 1)}</td>
                    <td className="px-3 py-2 font-mono text-xs text-zinc-400">{fmt(t.entryPremium, 2)}</td>
                    <td className="px-3 py-2 font-mono text-xs text-zinc-300">{fmt(t.exitPremium, 2)}</td>
                    <td className="px-3 py-2 font-mono text-xs text-zinc-400">{t.lots}</td>
                    <td className="px-3 py-2 font-mono text-xs font-bold" style={{ color: t.netPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                      {fmtINR(t.netPnl, { signed: true })}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs" style={{ color: t.returnPct >= 0 ? "var(--green)" : "var(--red)" }}>
                      {fmtPct(t.returnPct, true)}
                    </td>
                    <td className="px-3 py-2 font-mono text-[11px] text-zinc-500">{formatDuration(t.entryTime, t.exitTime)}</td>
                    <td className="px-3 py-2"><ExitBadge reason={t.exitReason} /></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

type MCXCommodityScalperProps = {
  actionsEnabled?: boolean;
};

export default function MCXCommodityScalper({ actionsEnabled = false }: MCXCommodityScalperProps) {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [refreshKey, setRefreshKey] = useState(0);
  const [isResetting, setIsResetting] = useState(false);

  const { positions, trades, strategies, stats, quotes, diagnostics, clearAll } = useMCXEngine(refreshKey);
  const actionButtonTitle = actionsEnabled
    ? "Action buttons are enabled."
    : "Set Action to Yes to enable reset and clear buttons.";

  useEffect(() => {
    const id = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!confirm("Reset the MCX Commodity paper account to ₹1,000,000? All history will be cleared.")) return;
    setIsResetting(true);
    clearAll();
    setRefreshKey((k) => k + 1);
    setIsResetting(false);
  };

  // ── Derived values ───────────────────────────────────────────────────────
  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));
  const closedPnl = stats.totalPnl;
  const unrealized = stats.unrealizedPnl;
  const sessionPnl = closedPnl + unrealized;
  const equity = INITIAL_MCX_BALANCE + sessionPnl;
  const totalReturnPct = (sessionPnl / INITIAL_MCX_BALANCE) * 100;
  const grossProfit = trades.filter((t) => t.netPnl > 0).reduce((s, t) => s + t.netPnl, 0);
  const grossLoss = trades.filter((t) => t.netPnl < 0).reduce((s, t) => s + Math.abs(t.netPnl), 0);
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;
  const winRate = stats.winRate;
  const openCount = stats.openPositions;
  const activeStrategies = strategies.filter((s) => s.status === "READY" || s.status === "IN_POSITION").length;
  const totalStrategies = strategies.length;
  const activeRoster = strategies.filter((s) => s.rosterState === "ACTIVE").length;
  const watchlistRoster = strategies.filter((s) => s.rosterState === "WATCHLIST").length;
  const disabledRoster = strategies.filter((s) => s.rosterState === "DISABLED").length;
  const feedOk = quotes.some((q) => q.ltp > 0);
  const feedBadgeTone: BadgeTone = feedOk ? "positive" : diagnostics.ltpError ? "negative" : "warning";
  const feedBadgeLabel = feedOk ? "MCX Feed Active" : diagnostics.ltpError ? "Feed Error" : "Feed: Connecting…";
  const sessionBadgeTone: BadgeTone = stats.sessionOpen ? "positive" : "warning";
  const sessionBadgeLabel = stats.sessionOpen ? "MCX Session Open" : "MCX Session Closed";
  const primaryCandleIssue = diagnostics.candleIssues[0];
  const primaryIssue = diagnostics.ltpError || (primaryCandleIssue ? `${primaryCandleIssue.commodityName}: ${primaryCandleIssue.error}` : "");
  const issueHint = diagnostics.ltpError.includes("Not configured")
    ? "Add the Angel One variables to the Next.js app env and restart or redeploy the client."
    : diagnostics.ltpError.includes("Could not resolve any MCX tokens")
      ? "Angel One login is likely working, but the MCX contract lookup did not find matching futures."
      : diagnostics.ltpError
        ? "Once the backend starts returning MCX prices, the engine will leave WARMING automatically."
        : primaryCandleIssue
          ? "Live prices can still warm the engine up, but historical seeding for this commodity is failing right now."
          : "";

  const bestCommodity = [...quotes].sort((a, b) => Math.abs(b.changePct) - Math.abs(a.changePct))[0];
  const commodityNames = MCX_COMMODITIES.map((c) => c.name).join(", ");
  const commodityDeskRows = useMemo<CommodityDeskRow[]>(() => MCX_COMMODITIES.map((commodity) => {
    const quote = quotes.find((q) => q.id === commodity.id);
    const commodityPositions = positions.filter((position) => position.commodityId === commodity.id);
    const commodityTrades = trades.filter((trade) => trade.commodityId === commodity.id);
    const commodityStrategies = strategies.filter((strategy) => strategy.commodityId === commodity.id);
    const wins = commodityTrades.filter((trade) => trade.netPnl >= 0).length;
    const realizedPnl = commodityTrades.reduce((sum, trade) => sum + trade.netPnl, 0);
    const unrealizedPnl = commodityPositions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
    const exposure = commodityPositions.reduce((sum, position) => sum + position.costBasis, 0);
    return {
      id: commodity.id,
      name: commodity.name,
      sector: commodity.sector,
      colorClass: commodity.colorClass,
      ltp: quote?.ltp ?? 0,
      changePct: quote?.changePct ?? 0,
      bias: quote?.bias ?? "WARMING",
      rsi: quote?.rsi ?? 50,
      momentumPct: quote?.momentumPct ?? 0,
      volatilityPct: quote?.volatilityPct ?? 0,
      barCount: quote?.barCount ?? 0,
      openPositions: commodityPositions.length,
      exposure,
      realizedPnl,
      unrealizedPnl,
      totalPnl: realizedPnl + unrealizedPnl,
      trades: commodityTrades.length,
      wins,
      winRate: commodityTrades.length > 0 ? (wins / commodityTrades.length) * 100 : 0,
      activeStrategies: commodityStrategies.filter((strategy) => strategy.rosterState === "ACTIVE").length,
      watchlistStrategies: commodityStrategies.filter((strategy) => strategy.rosterState === "WATCHLIST").length,
      disabledStrategies: commodityStrategies.filter((strategy) => strategy.rosterState === "DISABLED").length,
    };
  }), [positions, quotes, strategies, trades]);
  const sectorExposureRows = useMemo<SectorExposureRow[]>(() => {
    const rows = new Map<string, SectorExposureRow>();
    for (const row of commodityDeskRows) {
      const existing = rows.get(row.sector) ?? {
        sector: row.sector,
        exposure: 0,
        openPositions: 0,
        realizedPnl: 0,
        unrealizedPnl: 0,
        totalPnl: 0,
      };
      existing.exposure += row.exposure;
      existing.openPositions += row.openPositions;
      existing.realizedPnl += row.realizedPnl;
      existing.unrealizedPnl += row.unrealizedPnl;
      existing.totalPnl += row.totalPnl;
      rows.set(row.sector, existing);
    }
    return Array.from(rows.values());
  }, [commodityDeskRows]);
  const bullishCount = quotes.filter((quote) => quote.bias === "BULLISH").length;
  const bearishCount = quotes.filter((quote) => quote.bias === "BEARISH").length;
  const neutralCount = quotes.filter((quote) => quote.bias === "NEUTRAL").length;
  const highVolCount = quotes.filter((quote) => quote.volatilityPct >= 0.35).length;
  const averageRsi = quotes.length > 0 ? quotes.reduce((sum, quote) => sum + quote.rsi, 0) / quotes.length : 50;

  return (
    <main className="space-y-5 pb-10">
      {/* ── Header ── */}
      <div className="glass-panel px-6 py-7 md:px-7 relative overflow-hidden">
        <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-amber-500/10 blur-3xl pointer-events-none" />
        <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div className="px-1">
            <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">MCX Commodity Scalper</div>
            <div className="mt-4 flex flex-wrap items-end gap-4">
              <div className={`text-[clamp(2.2rem,4vw,3rem)] font-semibold leading-none tracking-tight ${sessionPnl >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                {fmtINR(equity)}
              </div>
              <div className={`pb-1 text-xl font-semibold leading-none ${sessionPnl >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                {sessionPnl >= 0 ? "+" : ""}{totalReturnPct.toFixed(2)}%
              </div>
            </div>
            <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
              Session PnL {fmtINR(sessionPnl, { signed: true })} · MCX Commodity Scalper · High Output Engine
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 lg:mt-10">
            <button type="button" disabled={!actionsEnabled || isResetting} title={actionButtonTitle} className="btn-primary text-sm"
              onClick={() => {
                if (!actionsEnabled) return;
                if (!confirm("Clear completed MCX trades and strategy stats? Open positions and balance will be kept.")) return;
                clearAll();
                setRefreshKey((k) => k + 1);
              }}>
              Clear Trades
            </button>
            <button type="button" onClick={handleReset} disabled={!actionsEnabled || isResetting} title={actionButtonTitle} className="btn-danger text-sm">
              {isResetting ? "Resetting…" : "Reset Account"}
            </button>
          </div>
        </div>

        <div className="mt-6 flex flex-wrap gap-2 px-1">
          <BadgePill label={feedBadgeLabel} tone={feedBadgeTone} />
          <BadgePill label={sessionBadgeLabel} tone={sessionBadgeTone} />
          <BadgePill label={`${activeStrategies}/${totalStrategies} Strategies Live`} tone="info" />
          <BadgePill label={`${MCX_COMMODITIES.length} Commodities`} tone="neutral" />
          <BadgePill label={`${bullishCount} Bull / ${bearishCount} Bear`} tone={bullishCount >= bearishCount ? "positive" : "negative"} />
          <BadgePill label="Paper Trading Only" tone="warning" />
        </div>

        {primaryIssue && (
          <div className="mx-1 mt-6 rounded-[20px] border px-4 py-3" style={{
            borderColor: diagnostics.ltpError ? "rgba(217,48,37,0.18)" : "rgba(217,119,6,0.18)",
            background: diagnostics.ltpError ? "rgba(254,242,242,0.92)" : "rgba(255,251,235,0.95)",
          }}>
            <div className="text-[10px] font-semibold uppercase tracking-[0.18em]" style={{ color: diagnostics.ltpError ? "var(--red)" : "#b45309" }}>
              {diagnostics.ltpError ? "MCX Data Error" : "MCX Warm-up Warning"}
            </div>
            <div className="mt-1 text-sm font-medium" style={{ color: "var(--text-primary)" }}>
              {primaryIssue}
            </div>
            <div className="mt-1 text-xs leading-5" style={{ color: "var(--text-secondary)" }}>
              {issueHint}
              {diagnostics.candleIssues.length > 1 ? ` ${diagnostics.candleIssues.length - 1} more commodity warm-up issue(s) are also present.` : ""}
            </div>
          </div>
        )}
      </div>

      {/* ── Commodity price grid ── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-7">
        {quotes.map((q) => <CommodityCard key={q.id} quote={q} />)}
      </div>

      {/* ── Summary metrics ── */}
      <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <CompactMetric
          label="Session Runtime"
          value={sessionRuntime}
          detail={`${activeStrategies} strategies scanning`}
          accent="text-zinc-900"
        />
        <CompactMetric
          label="Realized PnL"
          value={fmtINR(closedPnl, { signed: true })}
          detail={`${stats.totalTrades} completed trades`}
          accent={closedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}
        />
        <CompactMetric
          label="Unrealized PnL"
          value={fmtINR(unrealized, { signed: true })}
          detail={`${openCount} active positions`}
          accent={unrealized >= 0 ? "text-emerald-600" : "text-rose-600"}
        />
        <CompactMetric
          label="Win Rate"
          value={stats.totalTrades > 0 ? fmtPct(winRate) : "—"}
          detail={`${stats.totalWins}W / ${stats.totalLosses}L · PF ${profitFactor > 0 ? fmt(profitFactor) : "—"}`}
          accent={winRate >= 50 ? "text-emerald-600" : stats.totalTrades > 0 ? "text-rose-600" : "text-zinc-500"}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-7 mt-1">
        <SummaryCard label="Account Equity" value={fmtINR(equity)} accent={equity >= INITIAL_MCX_BALANCE ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Cash Balance" value={fmtINR(stats.balance)} accent="text-zinc-700" />
        <SummaryCard label="Total Return" value={fmtPct(totalReturnPct, true)} accent={totalReturnPct >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Open Positions" value={String(openCount)} accent="text-blue-600" />
        <SummaryCard label="Total Trades" value={String(stats.totalTrades)} accent="text-zinc-700" />
        <SummaryCard label="Gross Profit" value={fmtINR(grossProfit)} accent="text-emerald-600" />
        <SummaryCard label="Gross Loss" value={fmtINR(grossLoss)} accent="text-rose-600" />
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
        <CompactMetric
          label="Roster Mix"
          value={`${activeRoster}/${watchlistRoster}/${disabledRoster}`}
          detail="Active / watchlist / disabled strategies"
          accent="text-zinc-900"
        />
        <CompactMetric
          label="Capacity"
          value={`${openCount}/${stats.maxConcurrentPositions}`}
          detail={`${fmtPct(stats.capitalUtilizationPct)} capital currently deployed`}
          accent={openCount >= stats.maxConcurrentPositions ? "text-amber-600" : "text-zinc-900"}
        />
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <CompactMetric
          label="Warm Instruments"
          value={`${quotes.filter((q) => q.barCount >= 15).length}/${MCX_COMMODITIES.length}`}
          detail="Commodities with enough bars for fast signals"
          accent="text-zinc-900"
        />
        <CompactMetric
          label="Market Breadth"
          value={`${bullishCount}/${bearishCount}/${neutralCount}`}
          detail={`Bull / bear / neutral · Avg RSI ${fmt(averageRsi, 0)}`}
          accent={bullishCount >= bearishCount ? "text-emerald-600" : "text-rose-600"}
        />
        <CompactMetric
          label="Top Mover"
          value={bestCommodity && bestCommodity.ltp > 0 ? bestCommodity.name : "—"}
          detail={bestCommodity && bestCommodity.ltp > 0 ? `${fmtPct(bestCommodity.changePct, true)} today` : "Waiting for live MCX prices"}
          accent={bestCommodity && bestCommodity.changePct >= 0 ? "text-emerald-600" : "text-rose-600"}
        />
        <CompactMetric
          label="Volatility Watch"
          value={`${highVolCount}`}
          detail="Instruments above 0.35% short-window volatility"
          accent={highVolCount > 0 ? "text-amber-600" : "text-zinc-900"}
        />
      </div>

      <CommodityDeskPanel rows={commodityDeskRows} />

      <SectorExposurePanel rows={sectorExposureRows} />

      {/* ── Open Positions Snapshot (High visibility) ── */}
      {positions.length > 0 && (
        <div className="glass-panel px-5 py-5 md:px-6">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h2 className="flex items-center gap-3" style={{
              fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800,
              letterSpacing: "0.14em", color: "var(--text-secondary)",
            }}>
              Open Positions Snapshot
              <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>
                Active commodity options being managed for breakout or mean reversion.
              </span>
            </h2>
            <div className="flex items-center gap-3">
              <span className="font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                {positions.length} open
              </span>
              <span className="rounded-full border px-3 py-1 text-xs font-medium"
                style={{
                  background: unrealized >= 0 ? "var(--green-dim)" : "var(--red-dim)",
                  color: unrealized >= 0 ? "var(--green)" : "var(--red)",
                  borderColor: unrealized >= 0 ? "rgba(24, 128, 56, 0.14)" : "rgba(217, 48, 37, 0.14)",
                }}>
                Unrealized {fmtINR(unrealized, { signed: true })}
              </span>
            </div>
          </div>

          <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-left text-sm" style={{ minWidth: 1040 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Position</th>
                  <th className="px-4 py-3 font-medium">Strike / LTP</th>
                  <th className="px-4 py-3 font-medium">Premium (In / Now)</th>
                  <th className="px-4 py-3 font-medium">Lots / Notional</th>
                  <th className="px-4 py-3 font-medium text-right">PnL</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((pos) => {
                   return (
                    <tr key={pos.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <TypeBadge type={pos.optionType} />
                          <div>
                            <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{pos.commodityName}</div>
                            <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{pos.strategyName}</div>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-xs">
                        <div className="font-mono" style={{ color: "var(--text-primary)" }}>₹{fmt(pos.strike, 1)}</div>
                        <div style={{ color: "var(--text-secondary)" }}>LTP ₹{fmt(pos.currentPremium, 2)}</div>
                      </td>
                      <td className="px-4 py-3 text-xs">
                        <div className="font-mono" style={{ color: "var(--text-primary)" }}>₹{fmt(pos.entryPremium, 2)} {"->"} ₹{fmt(pos.currentPremium, 2)}</div>
                      </td>
                      <td className="px-4 py-3 text-xs">
                        <div className="font-mono text-zinc-900 font-medium">{pos.lots} lot(s)</div>
                        <div style={{ color: "var(--text-secondary)" }}>{fmtINR(pos.costBasis)} cost</div>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="font-mono text-sm font-semibold" style={{ color: pos.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                          {fmtINR(pos.unrealizedPnl, { signed: true })}
                        </div>
                        <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{fmtPct(pos.costBasis > 0 ? (pos.unrealizedPnl / pos.costBasis) * 100 : 0, true)}</div>
                      </td>
                    </tr>
                   );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* ── Live positions ── */}
      <PositionsPanel positions={positions} />

      {/* ── Strategy roster ── */}
      <StrategyRoster strategies={strategies} />

      <DailyPnlLedger
        trades={trades}
        initialEquity={INITIAL_MCX_BALANCE}
        title="DAILY PNL LEDGER"
        description="Realized MCX option PnL grouped by exit day, with each day measured from the equity it opened with."
        emptyMessage="No completed MCX trades yet, so there is no daily PnL ledger to display."
        formatCurrency={fmtINR}
      />

      {/* ── Trade ledger ── */}
      <TradeLedger trades={trades} />

      {/* ── Footer ── */}
      <div className="glass-panel px-5 py-4 text-center">
        <p className="text-[11px] leading-5" style={{ color: "var(--text-muted)" }}>
          MCX commodity paper account · Simplified Black-Scholes option premium model · Angel One SmartAPI live MCX feed ·
          ₹1,000,000 starting balance · {MCX_COMMODITIES.length} commodities ({commodityNames}) ·
          {bestCommodity && bestCommodity.ltp > 0 ? ` Most active: ${bestCommodity.name} (${fmtPct(Math.abs(bestCommodity.changePct))} move) ·` : ""}
          {" "}{totalStrategies} strategies · Paper trading only — no real orders placed
        </p>
      </div>
    </main>
  );
}
