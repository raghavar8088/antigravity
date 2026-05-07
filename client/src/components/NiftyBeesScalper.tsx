"use client";

import { useEffect, useState } from "react";
import DailyPnlLedger from "@/components/DailyPnlLedger";
import type {
  NiftyBeesPosition,
  NiftyBeesQuote,
  NiftyBeesStats,
  NiftyBeesStrategyStatus,
  NiftyBeesTrade,
} from "@/hooks/useNiftyBeesEngine";
import { formatShortDate, formatShortTime } from "@/lib/time";

const INITIAL_BEES_BALANCE = 10_000;

function fmtINR(n: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(n).toLocaleString("en-IN", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  if (signed) return `${n >= 0 ? "+" : "-"}₹${abs}`;
  return `₹${abs}`;
}

function fmtPct(n: number, signed = false, decimals = 2) {
  const s = signed ? (n >= 0 ? "+" : "") : "";
  return `${s}${Math.abs(n).toFixed(decimals)}%`;
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

function SideBadge({ side }: { side: "LONG" | "SHORT" }) {
  const long = side === "LONG";
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
      style={{
        background: long ? "rgba(24,128,56,0.12)" : "rgba(217,48,37,0.12)",
        color: long ? "var(--green)" : "var(--red)",
      }}
    >
      {side}
    </span>
  );
}

function StatusBadge({ status }: { status: NiftyBeesStrategyStatus["status"] }) {
  const map: Record<string, { bg: string; color: string }> = {
    WARMING: { bg: "rgba(100,100,100,0.1)", color: "var(--text-secondary)" },
    READY: { bg: "rgba(26,115,232,0.1)", color: "var(--blue)" },
    IN_POSITION: { bg: "rgba(24,128,56,0.1)", color: "var(--green)" },
    COOLING: { bg: "rgba(245,124,0,0.12)", color: "var(--amber)" },
  };
  const t = map[status] ?? map.READY;
  return (
    <span className="rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ background: t.bg, color: t.color }}>
      {status}
    </span>
  );
}

function CompactMetric({ label, value, detail, accent = "" }: { label: string; value: string; detail?: string; accent?: string }) {
  return (
    <div className="metric-card flex min-h-[96px] flex-col justify-between gap-2">
      <div>
        <div className="metric-label">{label}</div>
        <div className={`metric-value ${accent}`}>{value}</div>
      </div>
      {detail ? <div className="text-xs" style={{ color: "var(--text-secondary)" }}>{detail}</div> : null}
    </div>
  );
}

type NiftyBeesScalperProps = {
  actionsEnabled?: boolean;
  quote: NiftyBeesQuote;
  positions: NiftyBeesPosition[];
  trades: NiftyBeesTrade[];
  strategies: NiftyBeesStrategyStatus[];
  stats: NiftyBeesStats;
  reset: () => void;
  clearTrades: () => void;
};

export default function NiftyBeesScalper({
  actionsEnabled = false,
  quote,
  positions,
  trades,
  strategies,
  stats,
  reset,
  clearTrades,
}: NiftyBeesScalperProps) {
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const actionTitle = actionsEnabled ? "Actions enabled." : "Set Actions to enable reset and clear.";

  useEffect(() => {
    const id = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const sessionPnl = stats.sessionPnl;
  const equity = stats.equity;
  const totalReturnPct = (sessionPnl / INITIAL_BEES_BALANCE) * 100;
  const feedOk = quote.ltp > 0;
  const feedTone: BadgeTone = feedOk ? "positive" : stats.diagnostics.includes("Not configured") || stats.diagnostics.includes("Could not") ? "negative" : "warning";
  const sessionTone: BadgeTone = stats.sessionOpen ? "positive" : "warning";

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!confirm("Reset the Nifty BEES paper account to ₹10,000? Positions and history will be cleared.")) return;
    reset();
  };

  return (
    <main className="space-y-5 pb-10">
      <div className="glass-panel px-6 py-7 md:px-7 relative overflow-hidden">
        <div className="absolute -right-10 -top-10 h-36 w-36 rounded-full bg-sky-500/10 blur-3xl pointer-events-none" />
        <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div className="px-1">
            <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">Nifty BEES Scalper</div>
            <div className="mt-4 flex flex-wrap items-end gap-4">
              <div className={`text-[clamp(2rem,3.5vw,2.6rem)] font-semibold leading-none tracking-tight ${sessionPnl >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                {fmtINR(equity)}
              </div>
              <div className={`pb-1 text-lg font-semibold leading-none ${sessionPnl >= 0 ? "text-emerald-300" : "text-rose-300"}`}>
                {sessionPnl >= 0 ? "+" : ""}{totalReturnPct.toFixed(2)}%
              </div>
            </div>
            <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
              Session Bal: {fmtINR(stats.balance)} · {fmtINR(stats.unrealizedPnl, { signed: true })} unrealized · Angel/Yahoo live NSE · 30 strategies · ₹10k paper
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2 lg:mt-8">
            <button
              type="button"
              disabled={!actionsEnabled}
              title={actionTitle}
              className="btn-primary text-sm"
              onClick={() => {
                if (!actionsEnabled) return;
                if (!confirm("Clear completed Nifty BEES trades from the ledger? Balance and open positions stay as they are.")) return;
                clearTrades();
              }}
            >
              Clear Trades
            </button>
            <button type="button" disabled={!actionsEnabled} title={actionTitle} className="btn-danger text-sm" onClick={handleReset}>
              Reset Account
            </button>
          </div>
        </div>

        <div className="mt-5 flex flex-wrap gap-2 px-1">
          <BadgePill label={feedOk ? "Angel NSE LTP" : "Feed: waiting…"} tone={feedTone} />
          <BadgePill label={stats.sessionOpen ? "NSE session open" : "NSE session closed"} tone={sessionTone} />
          <BadgePill label={`${strategies.length} strategies`} tone="info" />
          <BadgePill label={quote.tradingSymbol || "NIFTYBEES"} tone="neutral" />
        </div>

        {!feedOk && (
          <div
            className="mx-1 mt-5 rounded-[20px] border px-4 py-3"
            style={{
              borderColor: "rgba(217,119,6,0.2)",
              background: "rgba(255,251,235,0.95)",
            }}
          >
            <div className="text-[10px] font-semibold uppercase tracking-[0.18em]" style={{ color: "#b45309" }}>
              Feed
            </div>
            <div className="mt-1 text-sm" style={{ color: "var(--text-primary)" }}>
              {stats.diagnostics}
            </div>
          </div>
        )}
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <CompactMetric label="LTP (NSE)" value={quote.ltp > 0 ? fmtINR(quote.ltp, { decimals: 2 }) : "—"} detail={`${quote.tradingSymbol} · ${fmtPct(quote.changePct, true)} day`} accent="" />
        <CompactMetric label="1m bars" value={String(quote.barCount)} detail="Seeded from Angel candles + live ticks" accent="" />
        <CompactMetric label="Open" value={String(stats.openPositions)} detail={`Win rate ${stats.winRate.toFixed(1)}%`} accent="" />
        <CompactMetric label="Trades (session)" value={String(stats.totalTrades)} detail={formatShortTime(new Date(currentTime).toISOString())} accent="" />
      </div>

      <div className="glass-panel px-5 py-5">
        <h3 className="text-[11px] font-bold uppercase tracking-[0.14em] text-zinc-500 mb-3">Open positions</h3>
        {positions.length === 0 ? (
          <div className="text-sm" style={{ color: "var(--text-secondary)" }}>No open BEES positions.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead style={{ color: "var(--text-secondary)" }}>
                <tr className="text-[10px] uppercase tracking-wider">
                  <th className="pb-2 pr-3">Strategy</th>
                  <th className="pb-2 pr-3">Side</th>
                  <th className="pb-2 pr-3">Qty</th>
                  <th className="pb-2 pr-3">Entry</th>
                  <th className="pb-2 pr-3">Mark</th>
                  <th className="pb-2 pr-3">uPnL</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((p: NiftyBeesPosition) => (
                  <tr key={p.id} className="border-t border-zinc-100">
                    <td className="py-2 pr-3 font-medium">{p.strategyName}</td>
                    <td className="py-2 pr-3"><SideBadge side={p.side} /></td>
                    <td className="py-2 pr-3">{p.quantity}</td>
                    <td className="py-2 pr-3">{fmtINR(p.entryPrice)}</td>
                    <td className="py-2 pr-3">{fmtINR(p.currentPrice)}</td>
                    <td className="py-2 pr-3">{fmtINR(p.unrealizedPnl, { signed: true })}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="glass-panel px-5 py-5">
        <h3 className="text-[11px] font-bold uppercase tracking-[0.14em] text-zinc-500 mb-3">Strategy roster</h3>
        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {strategies.map((s: NiftyBeesStrategyStatus) => (
            <div key={s.id} className="rounded-xl border border-zinc-100 bg-white/60 px-3 py-2.5">
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-semibold truncate">{s.name}</span>
                <StatusBadge status={s.status} />
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2 text-[10px]" style={{ color: "var(--text-secondary)" }}>
                <SideBadge side={s.side} />
                <span>Score {Math.round(s.score)}</span>
                <span>{s.totalTrades} trades · {s.winRate.toFixed(0)}% WR</span>
              </div>
            </div>
          ))}
        </div>
      </div>

      <DailyPnlLedger
        trades={trades}
        initialEquity={INITIAL_BEES_BALANCE}
        title="Daily PnL ledger"
        description="Realized ETF scalp PnL by exit day (IST calendar)."
        emptyMessage="No completed Nifty BEES trades yet."
        formatCurrency={fmtINR}
      />

      <div className="glass-panel px-5 py-5">
        <h3 className="text-[11px] font-bold uppercase tracking-[0.14em] text-zinc-500 mb-3">Recent trades</h3>
        {trades.length === 0 ? (
          <div className="text-sm" style={{ color: "var(--text-secondary)" }}>No closed trades yet.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead style={{ color: "var(--text-secondary)" }}>
                <tr className="text-[10px] uppercase tracking-wider">
                  <th className="pb-2 pr-3">Exit</th>
                  <th className="pb-2 pr-3">Strategy</th>
                  <th className="pb-2 pr-3">Side</th>
                  <th className="pb-2 pr-3">PnL</th>
                  <th className="pb-2">Reason</th>
                </tr>
              </thead>
              <tbody>
                {trades.slice(0, 40).map((t: NiftyBeesTrade) => (
                  <tr key={t.id} className="border-t border-zinc-100">
                    <td className="py-2 pr-3 whitespace-nowrap">{formatShortDate(t.exitTime)} {formatShortTime(t.exitTime)}</td>
                    <td className="py-2 pr-3 text-xs">{t.strategyName}</td>
                    <td className="py-2 pr-3"><SideBadge side={t.side} /></td>
                    <td className={`py-2 pr-3 font-medium ${t.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtINR(t.netPnl, { signed: true })}
                    </td>
                    <td className="py-2 text-xs">{t.exitReason}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="glass-panel px-5 py-4 text-center">
        <p className="text-[11px] leading-5" style={{ color: "var(--text-muted)" }}>
          Nippon India ETF Nifty 50 BeES (NSE) · Paper only · Auto entries when NSE cash session is open (09:15–15:30 IST) ·
          Live last traded price via Angel One SmartAPI proxy ({quote.tradingSymbol || "NIFTYBEES"}) · Not investment advice.
        </p>
      </div>
    </main>
  );
}
