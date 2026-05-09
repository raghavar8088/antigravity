"use client";

import dynamic from "next/dynamic";
import { useCallback, useEffect, useMemo, useState } from "react";

// Dynamically import the chart (uses DOM APIs — must be client-only, no SSR)
const BTCLiveChart = dynamic(() => import("@/components/BTCLiveChart"), { ssr: false });
import DailyPnlLedger from "@/components/DailyPnlLedger";
import { formatShortDate, formatShortTime } from "@/lib/time";
import {
  BTC_SPOT_CLIP_USD,
  type BTCSpotEngineStats,
  type BTCSpotPosition,
  type BTCSpotQuote,
  type BTCSpotStrategyStatus,
  type BTCSpotTrade,
} from "@/hooks/useBTCSpotScalperEngine";

function fmtUSD(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtPct(value: number, signed = false, decimals = 3) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

function csvEscapeCell(s: string): string {
  if (/[",\n]/.test(s)) return `"${s.replace(/"/g, '""')}"`;
  return s;
}

function formatElapsedSeconds(total: number) {
  const m = Math.floor(total / 60);
  const s = total % 60;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function formatTradeDuration(entryTime: string, exitTime: string) {
  const entry = new Date(entryTime);
  const exit = new Date(exitTime);
  if (Number.isNaN(entry.getTime()) || Number.isNaN(exit.getTime())) return "-";
  return formatElapsedSeconds(Math.max(0, Math.floor((exit.getTime() - entry.getTime()) / 1000)));
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

function StatusBadge({ status }: { status: BTCSpotStrategyStatus["status"] }) {
  const tones: Record<BTCSpotStrategyStatus["status"], { bg: string; color: string }> = {
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

function PositionProgressBar({ returnPct }: { returnPct: number }) {
  const scaleTargetPct = 0.35;
  const width = Math.min(100, Math.max(3, (Math.abs(returnPct) / scaleTargetPct) * 100));
  const positive = returnPct >= 0;
  const intense = Math.abs(returnPct) >= 0.10;
  return (
    <div className="w-32">
      <div className="flex items-center justify-between mb-1.5">
        <span
          className="text-[10px] font-bold uppercase tracking-wider"
          style={{ color: positive ? "var(--green)" : "var(--red)" }}
        >
          {positive ? "Gain" : "Loss"}
        </span>
        <span
          className="font-mono text-[11px] font-bold tabular-nums"
          style={{ color: positive ? "var(--green)" : "var(--red)" }}
        >
          {fmtPct(returnPct, true)}
        </span>
      </div>
      <div
        className="relative h-2.5 w-full overflow-hidden rounded-full"
        style={{ background: positive ? "rgba(24,128,56,0.08)" : "rgba(217,48,37,0.08)" }}
      >
        <div
          className="h-full rounded-full transition-all duration-500 ease-out"
          style={{
            width: `${width}%`,
            background: positive
              ? "linear-gradient(90deg, #22c55e 0%, #10b981 100%)"
              : "linear-gradient(90deg, #f43f5e 0%, #ef4444 100%)",
            boxShadow: intense
              ? positive ? "0 0 8px rgba(34,197,94,0.4)" : "0 0 8px rgba(244,63,94,0.4)"
              : "none",
          }}
        />
        {intense && (
          <div
            className="absolute inset-0 rounded-full animate-pulse"
            style={{
              width: `${width}%`,
              background: positive
                ? "linear-gradient(90deg, transparent, rgba(34,197,94,0.15))"
                : "linear-gradient(90deg, transparent, rgba(244,63,94,0.15))",
            }}
          />
        )}
      </div>
    </div>
  );
}

function regimeLabel(key: string): string {
  const map: Record<string, string> = {
    WARMING: "Warming bars",
    TRENDING_BULL: "Trending · bull",
    TRENDING_BEAR: "Trending · bear",
    HIGH_VOL: "High vol",
    RANGE: "Range",
  };
  return map[key] ?? key.replace(/_/g, " ");
}

function formatSavedAgo(ts: number | null): string {
  if (!ts) return "—";
  const sec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
  if (sec < 60) return `${sec}s ago`;
  const m = Math.floor(sec / 60);
  if (m < 60) return `${m}m ago`;
  return `${Math.floor(m / 60)}h ago`;
}


function QuoteHero({ quote, marketRegime }: { quote: BTCSpotQuote; marketRegime: string }) {
  const positive = quote.changePct24h >= 0;
  const volBoost = quote.volRatio >= 1.45;
  return (
    <div
      className="rounded-[16px] border px-5 py-5 transition-all"
      style={{
        borderColor: quote.hasPosition ? "rgba(245,124,0,0.45)" : "var(--border)",
        background: quote.hasPosition ? "rgba(245,124,0,0.06)" : "var(--surface-2)",
      }}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-xs font-semibold" style={{ color: "var(--text-primary)" }}>BTC (Delta 1m paper book)</div>
          <div className="text-[10px] uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>Delta Exchange REST · tight stops · fee-aware exits</div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <BadgePill label={regimeLabel(marketRegime)} tone={marketRegime === "HIGH_VOL" ? "warning" : marketRegime.startsWith("TRENDING") ? "info" : "neutral"} />
          <BadgePill label={quote.signalScore >= 62 ? `Signal ${quote.signalScore}` : `Scan ${quote.signalScore}`} tone={quote.signalScore >= 62 ? "info" : "neutral"} />
        </div>
      </div>
      <div className="mt-3 font-mono text-3xl font-semibold tracking-tight" style={{ color: "var(--text-primary)" }}>
        {quote.ltp > 0 ? quote.ltp.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : "…"}
      </div>
      <div className="mt-1 text-sm font-medium" style={{ color: positive ? "var(--green)" : "var(--red)" }}>
        {quote.ltp > 0 ? `${fmtPct(quote.changePct24h, true)} 24h (Delta ticker)` : "Waiting for Delta candles"}
      </div>
      {quote.ltp > 0 ? (
        <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px] font-mono" style={{ color: "var(--text-secondary)" }}>
          <span>Vol ratio {quote.volRatio.toFixed(2)}×</span>
          <span>·</span>
          <span>RSI(14) {quote.rsi14.toFixed(1)}</span>
          {volBoost ? <BadgePill label={`Boost ≥${1.45}×`} tone="warning" /> : null}
        </div>
      ) : null}
      {quote.sparkline.length > 4 ? (
        <div className="mt-4 h-10 w-full opacity-80">
          <svg viewBox="0 0 100 24" preserveAspectRatio="none" className="h-full w-full">
            {(() => {
              const ys = quote.sparkline;
              const min = Math.min(...ys);
              const max = Math.max(...ys);
              const r = max - min || 1;
              const d = ys.map((y, i) => {
                const x = (i / (ys.length - 1)) * 100;
                const py = 22 - ((y - min) / r) * 20;
                return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${py.toFixed(1)}`;
              }).join(" ");
              return <path d={d} fill="none" stroke="var(--blue)" strokeWidth="1.2" vectorEffect="non-scaling-stroke" />;
            })()}
          </svg>
        </div>
      ) : null}
    </div>
  );
}

type Props = {
  actionsEnabled?: boolean;
  initialBalance: number;
  quote: BTCSpotQuote;
  positions: BTCSpotPosition[];
  trades: BTCSpotTrade[];
  strategies: BTCSpotStrategyStatus[];
  stats: BTCSpotEngineStats;
  reset: () => void;
  entriesPaused: boolean;
  setEntriesPaused: (next: boolean) => void;
};

export default function BTCSpotScalper({
  actionsEnabled = false,
  initialBalance,
  quote,
  positions,
  trades,
  strategies,
  stats,
  reset,
  entriesPaused,
  setEntriesPaused,
}: Props) {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [isResetting, setIsResetting] = useState(false);
  const [strategyQuery, setStrategyQuery] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [strategySort, setStrategySort] = useState<"id" | "pnl" | "wr" | "trades">("id");
  const [tradeSort, setTradeSort] = useState<"recent" | "pnl_desc" | "pnl_asc" | "hold_long">("recent");
  const [tradeExitFilter, setTradeExitFilter] = useState<string>("all");

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  const totalReturnPct = ((stats.equity - initialBalance) / initialBalance) * 100;
  const latestTrade = trades[0] ?? null;
  const openCount = positions.length;
  const longCount = positions.filter((p) => p.side === "LONG").length;
  const shortCount = positions.filter((p) => p.side === "SHORT").length;

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!confirm(`Reset the BTC spot paper wallet to ${fmtUSD(initialBalance)}? Trades clear from this desk.`)) return;
    setIsResetting(true);
    reset();
    setTimeout(() => setIsResetting(false), 400);
  };

  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));

  const strategyCategories = useMemo(
    () => [...new Set(strategies.map((s) => s.category))].sort(),
    [strategies],
  );

  const filteredStrategies = useMemo(() => {
    const q = strategyQuery.trim().toLowerCase();
    return strategies.filter((s) => {
      if (categoryFilter !== "all" && s.category !== categoryFilter) return false;
      if (statusFilter !== "all" && s.status !== statusFilter) return false;
      if (q && !s.name.toLowerCase().includes(q) && !String(s.id).includes(q)) return false;
      return true;
    });
  }, [strategies, categoryFilter, statusFilter, strategyQuery]);

  const visibleStrategies = useMemo(() => {
    const list = [...filteredStrategies];
    switch (strategySort) {
      case "pnl":
        list.sort((a, b) => b.totalPnl - a.totalPnl);
        break;
      case "wr":
        list.sort((a, b) => b.winRate - a.winRate || b.totalTrades - a.totalTrades);
        break;
      case "trades":
        list.sort((a, b) => b.totalTrades - a.totalTrades);
        break;
      default:
        list.sort((a, b) => a.id - b.id);
    }
    return list;
  }, [filteredStrategies, strategySort]);

  const tradeExitReasons = useMemo(() => [...new Set(trades.map((t) => t.exitReason).filter(Boolean))].sort(), [trades]);

  const displayedTrades = useMemo(() => {
    const list =
      tradeExitFilter === "all"
        ? [...trades]
        : trades.filter((t) => t.exitReason === tradeExitFilter);
    switch (tradeSort) {
      case "pnl_desc":
        list.sort((a, b) => b.netPnl - a.netPnl);
        break;
      case "pnl_asc":
        list.sort((a, b) => a.netPnl - b.netPnl);
        break;
      case "hold_long":
        list.sort((a, b) => b.holdSeconds - a.holdSeconds);
        break;
      default:
        list.sort((a, b) => new Date(b.exitTime).getTime() - new Date(a.exitTime).getTime());
    }
    return list.slice(0, 500);
  }, [trades, tradeSort, tradeExitFilter]);

  const exportLedgerCsv = useCallback(() => {
    const headers = ["exitTime", "strategy", "side", "entryPrice", "exitPrice", "netPnl", "returnPct", "exitReason", "holdSeconds", "feesUsd"];
    const rows = trades.map((t) =>
      [
        t.exitTime,
        t.strategyName,
        t.side,
        String(t.entryPrice),
        String(t.exitPrice),
        String(t.netPnl),
        String(t.returnPct),
        t.exitReason,
        String(t.holdSeconds),
        String(t.feesUsd ?? 0),
      ].map(csvEscapeCell),
    );
    const body = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([body], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `btc-spot-paper-trades-${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }, [trades]);

  const exportLedgerJson = useCallback(() => {
    const payload = {
      exportedAt: new Date().toISOString(),
      module: "btc-spot-scalper-paper",
      initialBalance,
      stats,
      trades,
      positions,
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `btc-spot-paper-ledger-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }, [initialBalance, stats, trades, positions]);

  const pfDisplay =
    stats.profitFactor != null && Number.isFinite(stats.profitFactor)
      ? stats.profitFactor.toFixed(2)
      : stats.totalWins > 0 && stats.totalLosses === 0
        ? "∞"
        : "—";

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 2xl:grid-cols-[minmax(0,1.1fr)_minmax(340px,0.9fr)] items-start gap-5">
        <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
          <div className="absolute -right-16 -top-16 h-44 w-44 rounded-full bg-orange-500/10 blur-3xl pointer-events-none" />

          <div className="flex flex-col gap-5">
            <div className="px-1">
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                BTC SPOT MICRO SCALPER (PAPER)
              </div>
              <div className="mt-4 flex flex-wrap items-end gap-4">
                <div className={`text-[clamp(2.4rem,4.8vw,3.1rem)] font-semibold leading-none tracking-tight ${stats.equity >= initialBalance ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtUSD(stats.equity)}
                </div>
                <div className={`pb-1 text-xl font-semibold leading-none ${stats.sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtPct(totalReturnPct, true, 2)}
                </div>
              </div>
              <div className="mt-2 max-w-xl text-sm leading-relaxed" style={{ color: "var(--text-secondary)" }}>
                130 strategies incl. multi-timeframe (1m/5m/15m alignment, MTF pullback, MTF squeeze fire, MTF momentum cascade) plus Williams %R, CCI, Keltner, Donchian, EMA Ribbon, ADX, ROC, BB, Stochastic, MACD, OBV &amp; more — smart exit system with breakeven move, adaptive trailing &amp; momentum-aware holds. Up to ten concurrent clips, each about {fmtUSD(strategies[0]?.targetNotionalUsd ?? BTC_SPOT_CLIP_USD.min, { decimals: 2 })}–{fmtUSD(BTC_SPOT_CLIP_USD.max, { decimals: 2 })} notional ({fmtUSD(initialBalance)} desk). Each closed trade books at least $2 net win or loss after the fee model.
                Shorts are synthetic paper only (real spot is long-biased); fees are deducted on exit to stress-test edge.
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 px-1">
              <div className="flex flex-wrap gap-2">
                <BadgePill label={stats.warmingUp ? "Warming 1m bars" : "Engine live"} tone="positive" />
                <BadgePill label={`${openCount}/10 max slots`} tone="warning" />
                <BadgePill label="Tight SL / TP" tone="info" />
                {entriesPaused ? <BadgePill label="New entries paused" tone="warning" /> : null}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => setEntriesPaused(!entriesPaused)}
                  className="text-sm font-medium rounded-xl border px-3 py-2 transition-colors"
                  style={{
                    borderColor: entriesPaused ? "rgba(245,124,0,0.5)" : "var(--border)",
                    background: entriesPaused ? "rgba(245,124,0,0.12)" : "var(--surface)",
                    color: entriesPaused ? "var(--amber)" : "var(--text-primary)",
                  }}
                  title="Stops new paper entries; open clips still exit normally."
                >
                  {entriesPaused ? "Resume entries" : "Pause new entries"}
                </button>
                <button type="button" onClick={exportLedgerCsv} className="btn-primary text-sm" title="Export closed trades as CSV">
                  Export CSV
                </button>
                <button type="button" onClick={exportLedgerJson} className="btn-primary text-sm" title="Download full ledger JSON">
                  Export JSON
                </button>
                <button
                  type="button"
                  onClick={handleReset}
                  disabled={!actionsEnabled || isResetting}
                  title={actionsEnabled ? "Reset paper wallet" : "Unlock Actions to reset"}
                  className="btn-danger text-sm"
                >
                  {isResetting ? "Resetting…" : `Reset ${fmtUSD(initialBalance, { decimals: 0 })} paper wallet`}
                </button>
              </div>
            </div>
          </div>

          <div className="mt-6 text-[11px] leading-relaxed px-1" style={{ color: "var(--text-muted)" }}>
            <span className="font-semibold" style={{ color: "var(--text-secondary)" }}>Persistence: </span>
            Local {formatSavedAgo(stats.persistence.lastLocalSaveAt)}
            {stats.persistence.serverSyncConfigured
              ? <> · Server {formatSavedAgo(stats.persistence.lastServerSaveAt)}</>
              : <> · Server sync off (set DATABASE_URL for cross-device)</>}
          </div>
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <CompactMetric label="Session runtime" value={sessionRuntime} detail={stats.feeModelNote} accent="text-zinc-900" />
            <CompactMetric
              label="Last closed"
              value={latestTrade ? fmtUSD(latestTrade.netPnl, { signed: true }) : "—"}
              detail={latestTrade ? `${latestTrade.strategyName} · fees ${fmtUSD(latestTrade.feesUsd ?? 0)}` : "No fills yet"}
              accent={latestTrade ? (latestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600") : "text-zinc-900"}
            />
            <CompactMetric label="Open clips" value={openCount === 0 ? "Flat" : `${longCount}L / ${shortCount}S`} detail={stats.diagnostics} accent="text-zinc-900" />
          </div>
        </div>

        <QuoteHero quote={quote} marketRegime={stats.marketRegime} />
      </div>

      <BTCLiveChart positions={positions} />

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <CompactMetric
          label="Drawdown from peak"
          value={`${stats.maxDrawdownFromPeakPct.toFixed(2)}%`}
          detail={`Session high-water ${fmtUSD(stats.sessionPeakEquity)}`}
          accent={stats.maxDrawdownFromPeakPct > 12 ? "text-rose-600" : "text-zinc-900"}
        />
        <CompactMetric
          label="Current streak"
          value={
            stats.winStreak > 0 ? `${stats.winStreak} wins` : stats.lossStreak > 0 ? `${stats.lossStreak} losses` : "—"
          }
          detail="From most recent closes"
          accent={
            stats.winStreak > 0 ? "text-emerald-600" : stats.lossStreak > 0 ? "text-rose-600" : "text-zinc-900"
          }
        />
        <CompactMetric
          label="Best / worst clip"
          value={
            stats.bestTradeUsd != null && stats.worstTradeUsd != null
              ? `${fmtUSD(stats.bestTradeUsd, { signed: true })} · ${fmtUSD(stats.worstTradeUsd, { signed: true })}`
              : "—"
          }
          detail="Single-trade net after fees"
          accent="text-zinc-900"
        />
        <CompactMetric
          label="Exit mix"
          value={`${Object.keys(stats.exitReasonCounts).length} types`}
          detail={stats.totalTrades ? `${stats.totalTrades} closed in book` : "No breakdown yet"}
          accent="text-zinc-900"
        />
      </div>

      {Object.keys(stats.exitReasonCounts).length > 0 ? (
        <div className="flex flex-wrap gap-2 rounded-xl border px-4 py-3" style={{ borderColor: "var(--border)", background: "var(--surface-1)" }}>
          <span className="w-full text-[10px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>
            Exit reasons (closed trades)
          </span>
          {Object.entries(stats.exitReasonCounts)
            .sort((a, b) => b[1] - a[1])
            .map(([reason, n]) => (
              <span
                key={reason}
                className="rounded-lg border px-2.5 py-1 text-[10px] font-medium"
                style={{ borderColor: "var(--border-subtle)", color: "var(--text-secondary)" }}
              >
                {reason.replace(/_/g, " ")} · {n}
              </span>
            ))}
        </div>
      ) : null}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <CompactMetric label="Cash (free)" value={fmtUSD(stats.balance)} detail="After margining clips" accent="text-blue-600" />
        <CompactMetric label="Unrealized" value={fmtUSD(stats.unrealizedPnl, { signed: true })} detail="Mark: last 1m close" accent={stats.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <CompactMetric label="Win rate" value={stats.totalTrades ? `${stats.winRate.toFixed(1)}%` : "—"} detail={`${stats.totalWins}W / ${stats.totalLosses}L`} accent="text-zinc-900" />
        <CompactMetric label="Realized" value={fmtUSD(stats.realizedPnl, { signed: true })} detail="After fees" accent={stats.realizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <CompactMetric
          label="Avg win"
          value={stats.avgWinUsd != null ? fmtUSD(stats.avgWinUsd) : "—"}
          detail="Per winning trade (net)"
          accent="text-emerald-600"
        />
        <CompactMetric
          label="Avg loss"
          value={stats.avgLossUsd != null ? fmtUSD(-stats.avgLossUsd, { signed: true }) : "—"}
          detail="Per losing trade (net)"
          accent="text-rose-600"
        />
        <CompactMetric label="Profit factor" value={pfDisplay} detail="Gross wins ÷ gross losses" accent="text-zinc-900" />
        <CompactMetric
          label="Expectancy / trade"
          value={stats.expectancyPerTradeUsd != null ? fmtUSD(stats.expectancyPerTradeUsd, { signed: true }) : "—"}
          detail="Realized ÷ closed count"
          accent={
            stats.expectancyPerTradeUsd != null && stats.expectancyPerTradeUsd >= 0 ? "text-emerald-600" : "text-rose-600"
          }
        />
      </div>

      <div className="glass-panel px-5 py-5">
        <h2 className="mb-4 flex flex-wrap items-center gap-3" style={{
          fontFamily: "var(--font-display)",
          fontSize: 11,
          fontWeight: 800,
          letterSpacing: "0.14em",
          color: "var(--text-secondary)",
        }}>
          <span className="pill-green">LIVE</span>
          RUNNING SPOT POSITIONS
          <span style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }} className="font-mono">
            ({positions.length} active)
          </span>
        </h2>
        {positions.length === 0 ? (
          <p className="mt-3 text-sm" style={{ color: "var(--text-muted)" }}>No active clips — engine scans 1m structure for the next setup.</p>
        ) : (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-2 rounded-lg border px-3 py-1.5" style={{ borderColor: "rgba(24,128,56,0.2)", background: "rgba(24,128,56,0.04)" }}>
                  <span className="h-2 w-2 rounded-full" style={{ background: "var(--green)" }} />
                  <span className="text-[11px] font-bold tabular-nums tracking-wide" style={{ color: "var(--green)" }}>
                    {longCount}
                  </span>
                  <span className="text-[10px] font-medium uppercase" style={{ color: "var(--text-muted)" }}>Long</span>
                </div>
                <div className="flex items-center gap-2 rounded-lg border px-3 py-1.5" style={{ borderColor: "rgba(217,48,37,0.2)", background: "rgba(217,48,37,0.04)" }}>
                  <span className="h-2 w-2 rounded-full" style={{ background: "var(--red)" }} />
                  <span className="text-[11px] font-bold tabular-nums tracking-wide" style={{ color: "var(--red)" }}>
                    {shortCount}
                  </span>
                  <span className="text-[10px] font-medium uppercase" style={{ color: "var(--text-muted)" }}>Short</span>
                </div>
              </div>
              <div
                className="flex items-center gap-2 rounded-xl border px-4 py-2"
                style={{
                  background: stats.unrealizedPnl >= 0
                    ? "linear-gradient(135deg, rgba(24,128,56,0.06) 0%, rgba(24,128,56,0.12) 100%)"
                    : "linear-gradient(135deg, rgba(217,48,37,0.06) 0%, rgba(217,48,37,0.12) 100%)",
                  borderColor: stats.unrealizedPnl >= 0 ? "rgba(24,128,56,0.20)" : "rgba(217,48,37,0.20)",
                }}
              >
                <span className="text-[10px] font-semibold uppercase tracking-wider" style={{ color: "var(--text-muted)" }}>
                  Unrealized
                </span>
                <span
                  className="font-mono text-sm font-bold tabular-nums"
                  style={{ color: stats.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}
                >
                  {fmtUSD(stats.unrealizedPnl, { signed: true })}
                </span>
              </div>
            </div>

            <div className="overflow-x-auto rounded-2xl border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
              <table className="w-full text-left text-sm" style={{ minWidth: 1020 }}>
                <thead>
                  <tr
                    className="text-[10px] uppercase tracking-[0.14em]"
                    style={{
                      background: "linear-gradient(180deg, var(--surface-2) 0%, var(--surface-1) 100%)",
                      color: "var(--text-muted)",
                    }}
                  >
                    <th className="px-5 py-3.5 font-semibold">Position</th>
                    <th className="px-4 py-3.5 font-semibold">Side</th>
                    <th className="px-4 py-3.5 font-semibold text-right">Entry</th>
                    <th className="px-4 py-3.5 font-semibold text-right">Mark</th>
                    <th className="px-4 py-3.5 font-semibold text-right">Notional</th>
                    <th className="px-4 py-3.5 font-semibold">Opened</th>
                    <th className="px-4 py-3.5 font-semibold text-right">PnL</th>
                    <th className="px-5 py-3.5 font-semibold text-right">Progress</th>
                  </tr>
                </thead>
                <tbody>
                  {positions.map((p, idx) => {
                    const pnlPositive = p.unrealizedPnl >= 0;
                    return (
                      <tr
                        key={p.id}
                        className="group transition-colors duration-150"
                        style={{
                          borderTop: idx === 0 ? "none" : "1px solid var(--border-subtle)",
                          background: pnlPositive
                            ? "rgba(24,128,56,0.02)"
                            : "rgba(217,48,37,0.02)",
                        }}
                        onMouseEnter={(e) => {
                          (e.currentTarget as HTMLElement).style.background = pnlPositive
                            ? "rgba(24,128,56,0.06)"
                            : "rgba(217,48,37,0.06)";
                        }}
                        onMouseLeave={(e) => {
                          (e.currentTarget as HTMLElement).style.background = pnlPositive
                            ? "rgba(24,128,56,0.02)"
                            : "rgba(217,48,37,0.02)";
                        }}
                      >
                        <td className="px-5 py-4">
                          <div className="flex items-center gap-2.5">
                            <span
                              className="relative flex h-2.5 w-2.5 shrink-0"
                            >
                              <span
                                className="absolute inline-flex h-full w-full animate-ping rounded-full opacity-60"
                                style={{ background: pnlPositive ? "var(--green)" : "var(--red)" }}
                              />
                              <span
                                className="relative inline-flex h-2.5 w-2.5 rounded-full"
                                style={{ background: pnlPositive ? "var(--green)" : "var(--red)" }}
                              />
                            </span>
                            <div>
                              <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
                                {p.strategyName}
                              </div>
                              <div className="text-[10px] font-mono" style={{ color: "var(--text-muted)" }}>
                                {p.quantity.toFixed(6)} BTC
                              </div>
                            </div>
                          </div>
                        </td>
                        <td className="px-4 py-4"><SideBadge side={p.side} /></td>
                        <td className="px-4 py-4 text-right">
                          <span className="font-mono text-xs tabular-nums" style={{ color: "var(--text-secondary)" }}>
                            {p.entryPrice.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                          </span>
                        </td>
                        <td className="px-4 py-4 text-right">
                          <span className="font-mono text-xs font-semibold tabular-nums" style={{ color: "var(--text-primary)" }}>
                            {p.currentPrice.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                          </span>
                        </td>
                        <td className="px-4 py-4 text-right">
                          <span className="font-mono text-xs tabular-nums" style={{ color: "var(--text-secondary)" }}>
                            {fmtUSD(p.notional)}
                          </span>
                        </td>
                        <td className="px-4 py-4">
                          <div className="text-xs font-mono font-medium" style={{ color: "var(--text-primary)" }}>
                            {formatShortTime(p.entryTime)}
                          </div>
                          <div className="text-[10px]" style={{ color: "var(--text-muted)" }}>
                            {formatShortDate(p.entryTime)}
                          </div>
                        </td>
                        <td className="px-4 py-4 text-right">
                          <div
                            className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 font-mono text-xs font-bold tabular-nums"
                            style={{
                              background: pnlPositive ? "rgba(24,128,56,0.10)" : "rgba(217,48,37,0.10)",
                              color: pnlPositive ? "var(--green)" : "var(--red)",
                            }}
                          >
                            <span
                              className="text-[9px]"
                              style={{ lineHeight: 1 }}
                            >
                              {pnlPositive ? "▲" : "▼"}
                            </span>
                            {fmtUSD(p.unrealizedPnl, { signed: true })}
                          </div>
                        </td>
                        <td className="px-5 py-4">
                          <div className="ml-auto">
                            <PositionProgressBar returnPct={p.returnPct} />
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      <div className="glass-panel px-5 py-5">
        <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Strategies (30)</h2>
        <p className="mt-1 text-xs" style={{ color: "var(--text-muted)" }}>Regime filter skips mean-reversion in violent tape and trend-chase in dead ranges.</p>
        <div className="mt-4 flex flex-col gap-3 lg:flex-row lg:flex-wrap lg:items-end">
          <input
            type="search"
            value={strategyQuery}
            onChange={(e) => setStrategyQuery(e.target.value)}
            placeholder="Search name or ID…"
            className="min-w-[200px] flex-1 rounded-xl border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500/30"
            style={{
              borderColor: "var(--border)",
              background: "var(--surface)",
              color: "var(--text-primary)",
            }}
          />
          <div className="flex flex-wrap gap-2">
            <select
              value={strategySort}
              onChange={(e) => setStrategySort(e.target.value as "id" | "pnl" | "wr" | "trades")}
              className="rounded-xl border px-3 py-2 text-xs font-medium outline-none"
              style={{
                borderColor: "var(--border)",
                background: "var(--surface)",
                color: "var(--text-primary)",
              }}
              aria-label="Sort strategies"
            >
              <option value="id">Sort · ID</option>
              <option value="pnl">Sort · Total PnL</option>
              <option value="wr">Sort · Win rate</option>
              <option value="trades">Sort · Trade count</option>
            </select>
            <select
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value)}
              className="rounded-xl border px-3 py-2 text-xs font-medium outline-none"
              style={{
                borderColor: "var(--border)",
                background: "var(--surface)",
                color: "var(--text-primary)",
              }}
              aria-label="Filter by category"
            >
              <option value="all">All categories</option>
              {strategyCategories.map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="rounded-xl border px-3 py-2 text-xs font-medium outline-none"
              style={{
                borderColor: "var(--border)",
                background: "var(--surface)",
                color: "var(--text-primary)",
              }}
              aria-label="Filter by status"
            >
              <option value="all">All statuses</option>
              <option value="WARMING">Warming</option>
              <option value="READY">Ready</option>
              <option value="IN_POSITION">In position</option>
              <option value="COOLING">Cooling</option>
            </select>
          </div>
        </div>
        <p className="mt-2 text-[11px]" style={{ color: "var(--text-muted)" }}>
          Showing {visibleStrategies.length} of {strategies.length} (filters apply; sort is local)
        </p>
        <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {visibleStrategies.map((s) => (
            <div key={s.id} className="rounded-xl border px-3 py-3 text-xs" style={{ borderColor: "var(--border)" }}>
              <div className="flex items-center justify-between gap-2">
                <span className="font-semibold" style={{ color: "var(--text-primary)" }}>{s.id}. {s.name}</span>
                <StatusBadge status={s.status} />
              </div>
              <div className="mt-1 flex flex-wrap gap-2 text-[10px]" style={{ color: "var(--text-secondary)" }}>
                <SideBadge side={s.side} />
                <span>{s.category}</span>
                <span>Score {Math.round(s.score)}</span>
              </div>
              <div className="mt-2 font-mono text-[11px]" style={{ color: "var(--text-muted)" }}>
                {s.totalTrades} trades · {s.winRate.toFixed(0)}% WR · {fmtUSD(s.totalPnl, { signed: true })}
              </div>
            </div>
          ))}
        </div>
      </div>

      <DailyPnlLedger
        trades={trades.map((t) => ({
          id: t.id,
          strategyId: t.strategyId,
          strategyName: t.strategyName,
          symbol: t.symbol,
          side: t.side,
          quantity: t.quantity,
          entryPrice: t.entryPrice,
          exitPrice: t.exitPrice,
          netPnl: t.netPnl,
          returnPct: t.returnPct,
          entryTime: t.entryTime,
          exitTime: t.exitTime,
          exitReason: t.exitReason,
          holdSeconds: t.holdSeconds,
        }))}
        initialEquity={initialBalance}
        title="DAILY PNL LEDGER"
        description="Realized BTC spot paper PnL by exit day (fees included in net PnL)."
        emptyMessage="No closed trades yet."
        formatCurrency={fmtUSD}
      />

      <div className="glass-panel px-5 py-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
          <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Recent trades</h2>
          {trades.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              <select
                value={tradeSort}
                onChange={(e) => setTradeSort(e.target.value as typeof tradeSort)}
                className="rounded-xl border px-2 py-1.5 text-[11px] font-medium outline-none"
                style={{
                  borderColor: "var(--border)",
                  background: "var(--surface)",
                  color: "var(--text-primary)",
                }}
                aria-label="Sort trades"
              >
                <option value="recent">Newest exit first</option>
                <option value="pnl_desc">PnL high → low</option>
                <option value="pnl_asc">PnL low → high</option>
                <option value="hold_long">Longest hold</option>
              </select>
              <select
                value={tradeExitFilter}
                onChange={(e) => setTradeExitFilter(e.target.value)}
                className="rounded-xl border px-2 py-1.5 text-[11px] font-medium outline-none"
                style={{
                  borderColor: "var(--border)",
                  background: "var(--surface)",
                  color: "var(--text-primary)",
                }}
                aria-label="Filter by exit"
              >
                <option value="all">All exits</option>
                {tradeExitReasons.map((r) => (
                  <option key={r} value={r}>{r.replace(/_/g, " ")}</option>
                ))}
              </select>
            </div>
          ) : null}
        </div>
        {trades.length === 0 ? (
          <p className="mt-3 text-sm" style={{ color: "var(--text-muted)" }}>Ledger empty.</p>
        ) : (
          <div className="mt-3 overflow-x-auto max-h-[420px] overflow-y-auto">
            <table className="w-full text-left text-sm">
              <thead className="sticky top-0 z-10 bg-[var(--surface-1)]">
                <tr className="border-b text-[10px] uppercase tracking-wider" style={{ borderColor: "var(--border-subtle)", color: "var(--text-muted)" }}>
                  <th className="py-2 pr-2">Exit</th>
                  <th className="py-2 pr-2">Strategy</th>
                  <th className="py-2 pr-2">Side</th>
                  <th className="py-2 pr-2 text-right">Entry→Exit</th>
                  <th className="py-2 pr-2">Hold</th>
                  <th className="py-2 pr-2">Reason</th>
                  <th className="py-2 pr-2 text-right">Net</th>
                </tr>
              </thead>
              <tbody>
                {displayedTrades.map((t) => (
                  <tr key={t.id} className="border-b" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="py-2 pr-2 whitespace-nowrap text-[11px]" style={{ color: "var(--text-muted)" }}>
                      {formatShortDate(t.exitTime)} {formatShortTime(t.exitTime)}
                    </td>
                    <td className="py-2 pr-2 text-xs max-w-[140px] truncate" title={t.strategyName}>{t.strategyName}</td>
                    <td className="py-2 pr-2"><SideBadge side={t.side} /></td>
                    <td className="py-2 pr-2 text-right font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>
                      {t.entryPrice.toFixed(2)} → {t.exitPrice.toFixed(2)}
                    </td>
                    <td className="py-2 pr-2 text-[11px]" style={{ color: "var(--text-muted)" }}>
                      {formatTradeDuration(t.entryTime, t.exitTime)}
                    </td>
                    <td className="py-2 pr-2"><ExitBadge reason={t.exitReason} /></td>
                    <td className={`py-2 text-right font-mono text-xs font-semibold ${t.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtUSD(t.netPnl, { signed: true })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="text-center text-[11px]" style={{ color: "var(--text-muted)" }}>
        Educational paper only · not investment advice · Delta Exchange 1m OHLC (default symbol BTCUSD)
      </div>
    </div>
  );
}
