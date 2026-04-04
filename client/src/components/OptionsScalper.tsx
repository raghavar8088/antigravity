"use client";
import { useEffect, useState } from "react";
import useOptions, { OptionPosition, OptionTrade, OptionStrategyStatus } from "@/hooks/useOptions";
import { formatShortDate, formatShortTime } from "@/lib/time";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const INITIAL_OPTIONS_BALANCE = 1_000_000;

// ── Formatters ──────────────────────────────────────────────────────────────

function fmtUSD(n: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(n).toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (signed) return `${n >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtPct(n: number, signed = false, decimals = 1) {
  const s = signed ? (n >= 0 ? "+" : "") : "";
  return `${s}${Math.abs(n).toFixed(decimals)}%`;
}

function fmt(n: number, d = 2) {
  return n.toLocaleString(undefined, { minimumFractionDigits: d, maximumFractionDigits: d });
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
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[status] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>
      {status.replace("_", " ")}
    </span>
  );
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

function LivePositionsPanel({ positions }: { positions: OptionPosition[] }) {
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
                          <span className="text-sm font-medium" style={{ color: "var(--text-primary)" }}>{pos.strategyName}</span>
                        </div>
                        <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                          Qty {fmt(pos.quantity, 4)} | Cost {fmtUSD(pos.costBasis)}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>${fmt(pos.strike, 0)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>BTC {fmtUSD(pos.entryBtcPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>In ${fmt(pos.entryPremium)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>Now ${fmt(pos.currentPremium)}</div>
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

function StrategiesPanel({ strategies }: { strategies: OptionStrategyStatus[] }) {
  const [showAll, setShowAll] = useState(false);
  const sorted = [...strategies].sort((a, b) => b.totalPnl - a.totalPnl);
  const visible = showAll ? sorted : sorted.slice(0, 20);

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="mb-5 flex items-center justify-between gap-3">
        <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
          ALL 50 STRATEGIES - LEADERBOARD
        </h2>
        <button
          type="button"
          onClick={() => setShowAll((v) => !v)}
          className="btn-gold text-xs px-4 py-1.5 min-h-[32px]"
        >
          {showAll ? "Show Top 20" : "Show All 50"}
        </button>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left" style={{ minWidth: 720 }}>
          <thead>
            <tr className="border-b" style={{ borderColor: "var(--border)" }}>
              {["#", "Strategy", "Type", "Status", "Trades", "W / L", "Win Rate", "Total PnL"].map((h, i) => (
                <th key={h} className={`py-2 px-3 text-[10px] font-bold uppercase tracking-widest ${i === 7 ? "text-right" : ""}`}
                  style={{ color: "var(--text-muted)" }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((s, i) => (
              <tr key={s.name} className="border-b transition-colors hover:bg-black/[0.015]" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="py-2.5 px-3 text-xs font-mono" style={{ color: "var(--text-muted)" }}>{i + 1}</td>
                <td className="py-2.5 px-3 text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{s.name}</td>
                <td className="py-2.5 px-3"><TypeBadge type={s.optionType} /></td>
                <td className="py-2.5 px-3"><StatusBadge status={s.status} /></td>
                <td className="py-2.5 px-3 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>{s.totalTrades}</td>
                <td className="py-2.5 px-3 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>{s.wins}W / {s.losses}L</td>
                <td className="py-2.5 px-3 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {s.totalTrades > 0 ? fmtPct(s.winRate) : "-"}
                </td>
                <td className={`py-2.5 px-3 text-right text-sm font-mono font-bold ${s.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {s.totalTrades > 0 ? fmtUSD(s.totalPnl, { signed: true }) : "-"}
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

function TradesPanel({ trades }: { trades: OptionTrade[] }) {
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
                  <th className="px-4 py-3 font-medium">BTC Move</th>
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
                      <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{t.strategyName}</div>
                      <div className="font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>{t.id}</div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="flex items-center gap-2">
                        <TypeBadge type={t.optionType} />
                        <span className="font-mono" style={{ color: "var(--text-primary)" }}>${fmt(t.strike, 0)}</span>
                      </div>
                      <div style={{ color: "var(--text-secondary)", marginTop: 4 }}>
                        {t.expiryMins}m expiry
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>In ${fmt(t.entryPremium)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>Out ${fmt(t.exitPremium)}</div>
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

// ── Main export ───────────────────────────────────────────────────────────────

export default function OptionsScalper() {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [refreshKey, setRefreshKey] = useState(0);
  const [isResetting, setIsResetting] = useState(false);
  const { positions, trades, strategies, stats } = useOptions(refreshKey);

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  const handleReset = async () => {
    if (!confirm("Reset the options paper account to $1,000,000? All history will be cleared.")) {
      return;
    }

    setIsResetting(true);
    try {
      const response = await fetch(`${API_URL}/api/options/reset`, { method: "POST" });
      if (!response.ok) {
        throw new Error("reset failed");
      }
      setRefreshKey((k) => k + 1);
    } catch {
      window.alert("Options account reset failed. Check engine connectivity.");
    } finally {
      setIsResetting(false);
    }
  };

  // ── Derived values ──────────────────────────────────────────────
  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));
  const closedPnl = stats?.totalPnl ?? trades.reduce((sum, trade) => sum + trade.netPnl, 0);
  const unrealized = stats?.unrealizedPnl ?? positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const equity = stats?.equity ?? INITIAL_OPTIONS_BALANCE + closedPnl + unrealized;
  const sessionPnl = equity - INITIAL_OPTIONS_BALANCE;
  const totalReturnPct = (sessionPnl / INITIAL_OPTIONS_BALANCE) * 100;
  const grossProfit = trades.filter((trade) => trade.netPnl > 0).reduce((sum, trade) => sum + trade.netPnl, 0);
  const grossLoss = trades.filter((trade) => trade.netPnl < 0).reduce((sum, trade) => sum + Math.abs(trade.netPnl), 0);
  const totalTrades = Math.max(stats?.totalTrades ?? 0, trades.length);
  const totalWins = stats?.totalWins ?? trades.filter((trade) => trade.netPnl >= 0).length;
  const totalLosses = stats?.totalLosses ?? trades.filter((trade) => trade.netPnl < 0).length;
  const winRate = totalTrades > 0 ? (totalWins / totalTrades) * 100 : 0;
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;
  const openCount = Math.max(stats?.openPositions ?? 0, positions.length);
  const callCount = positions.filter((p) => p.optionType === "CALL").length;
  const putCount  = positions.filter((p) => p.optionType === "PUT").length;
  const exposureSummary = openCount === 0 ? "No open exposure" : `${callCount} calls / ${putCount} puts`;
  const bestStrategy = [...strategies].sort((a, b) => b.totalPnl - a.totalPnl)[0] ?? null;
  const latestTrade = trades[0] ?? null;
  const activeStrategies = strategies.filter((s) => s.status !== "COOLING").length || strategies.length;

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

  return (
    <div className="space-y-5">

      {/* ── Hero: Options Equity ── */}
      <div className="grid grid-cols-1 2xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)] items-start gap-5">

        {/* Left: price-hero card */}
        <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
          <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-amber-500/10 blur-3xl pointer-events-none" />

          <div className="flex flex-col gap-5">
            <div className="px-1">
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                BTC OPTION EQUITY
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
                <BadgePill label="Options Engine Online" tone="positive" />
                <BadgePill label="50 Strategies Active" tone="info" />
                <BadgePill label="Separate Account" tone="warning" />
                <BadgePill label="Not Futures" tone="neutral" />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  disabled={isResetting}
                  className="btn-primary text-sm"
                  onClick={async () => {
                    if (!confirm("Clear completed option trades and strategy stats? Open positions and balance will be kept.")) return;
                    await fetch(`${API_URL}/api/options/clear-history`, { method: "POST" });
                    setRefreshKey((k) => k + 1);
                  }}
                >
                  Clear Option Trades
                </button>
                <button
                  type="button"
                  onClick={handleReset}
                  disabled={isResetting}
                  className="btn-danger text-sm"
                >
                  {isResetting ? "Resetting…" : "Reset Options Account"}
                </button>
              </div>
            </div>
          </div>

          <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <CompactMetric
              label="Session Runtime"
              value={sessionRuntime}
              detail={`${activeStrategies} strategies scanning`}
              accent="text-zinc-900"
            />
            <CompactMetric
              label="Last Closed Trade"
              value={latestTrade ? fmtUSD(latestTrade.netPnl, { signed: true }) : "No exits yet"}
              detail={latestTrade ? `${latestTrade.strategyName} | ${latestTrade.exitReason}` : "Waiting for first completed options cycle"}
              accent={latestTrade ? (latestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600") : "text-zinc-900"}
            />
            <CompactMetric
              label="Open Exposure"
              value={exposureSummary}
              detail={`${openCount} of 50 strategies in position`}
              accent="text-zinc-900"
            />
          </div>
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
      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-4">
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
      </div>

      {/* ── Live positions ── */}
      <LivePositionsPanel positions={positions} />

      {/* ── Strategies leaderboard ── */}
      <StrategiesPanel strategies={strategies} />

      {/* ── Best strategy callout ── */}
      {bestStrategy && (
        <div className="glass-panel px-6 py-5 flex flex-wrap items-center gap-6 justify-between">
          <div>
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Top Performing Strategy</div>
            <div className="mt-1 text-lg font-bold" style={{ color: "var(--text-primary)" }}>{bestStrategy.name}</div>
            <div className="mt-0.5 text-xs" style={{ color: "var(--text-secondary)" }}>
              {bestStrategy.wins}W / {bestStrategy.losses}L | {bestStrategy.totalTrades > 0 ? fmtPct(bestStrategy.winRate) : "-"} win rate
            </div>
          </div>
          <div className="text-right">
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Total PnL</div>
            <div className={`mt-1 text-2xl font-bold ${bestStrategy.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>{fmtUSD(bestStrategy.totalPnl, { signed: true })}</div>
          </div>
          <div className="flex items-center gap-2 ml-auto">
            <TypeBadge type={bestStrategy.optionType} />
            <StatusBadge status={bestStrategy.status} />
          </div>
        </div>
      )}

      {/* ── Trade history ── */}
      <TradesPanel trades={trades} />

      {/* ── Footer note ── */}
      <div className="text-center text-[11px]" style={{ color: "var(--text-muted)" }}>
        Options paper account · Black-Scholes pricing · $1,000,000 starting balance · Fully separate from futures engine
      </div>

    </div>
  );
}
