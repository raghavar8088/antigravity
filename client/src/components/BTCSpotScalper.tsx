"use client";

import { useEffect, useState } from "react";
import DailyPnlLedger from "@/components/DailyPnlLedger";
import { formatShortDate, formatShortTime } from "@/lib/time";
import type {
  BTCSpotEngineStats,
  BTCSpotPosition,
  BTCSpotQuote,
  BTCSpotStrategyStatus,
  BTCSpotTrade,
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

function QuoteHero({ quote }: { quote: BTCSpotQuote }) {
  const positive = quote.changePct24h >= 0;
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
        <BadgePill label={quote.signalScore >= 62 ? `Signal ${quote.signalScore}` : `Scan ${quote.signalScore}`} tone={quote.signalScore >= 62 ? "info" : "neutral"} />
      </div>
      <div className="mt-3 font-mono text-3xl font-semibold tracking-tight" style={{ color: "var(--text-primary)" }}>
        {quote.ltp > 0 ? quote.ltp.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : "…"}
      </div>
      <div className="mt-1 text-sm font-medium" style={{ color: positive ? "var(--green)" : "var(--red)" }}>
        {quote.ltp > 0 ? `${fmtPct(quote.changePct24h, true)} 24h (Delta ticker)` : "Waiting for Delta candles"}
      </div>
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
}: Props) {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const [isResetting, setIsResetting] = useState(false);

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
                Ten independent 1m strategies share a tiny pool: up to three concurrent clips, each sized to about {fmtUSD(strategies[0]?.targetNotionalUsd ?? 1, { decimals: 2 })}–{fmtUSD(Math.min(initialBalance * 0.4, 3.5), { decimals: 2 })} notional so the book stays tradable near {fmtUSD(initialBalance)}.
                Shorts are synthetic paper only (real spot is long-biased); fees are deducted on exit to stress-test edge.
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 px-1">
              <div className="flex flex-wrap gap-2">
                <BadgePill label={stats.warmingUp ? "Warming 1m bars" : "Engine live"} tone="positive" />
                <BadgePill label={`${openCount}/3 max slots`} tone="warning" />
                <BadgePill label="Tight SL / TP" tone="info" />
              </div>
              <button
                type="button"
                onClick={handleReset}
                disabled={!actionsEnabled || isResetting}
                title={actionsEnabled ? "Reset paper wallet" : "Unlock Actions to reset"}
                className="btn-danger text-sm"
              >
                {isResetting ? "Resetting…" : "Reset $10 paper wallet"}
              </button>
            </div>
          </div>

          <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
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

        <QuoteHero quote={quote} />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <CompactMetric label="Cash (free)" value={fmtUSD(stats.balance)} detail="After margining clips" accent="text-blue-600" />
        <CompactMetric label="Unrealized" value={fmtUSD(stats.unrealizedPnl, { signed: true })} detail="Mark: last 1m close" accent={stats.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <CompactMetric label="Win rate" value={stats.totalTrades ? `${stats.winRate.toFixed(1)}%` : "—"} detail={`${stats.totalWins}W / ${stats.totalLosses}L`} accent="text-zinc-900" />
        <CompactMetric label="Realized" value={fmtUSD(stats.realizedPnl, { signed: true })} detail="After fees" accent={stats.realizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
      </div>

      <div className="glass-panel px-5 py-5">
        <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Open positions</h2>
        {positions.length === 0 ? (
          <p className="mt-3 text-sm" style={{ color: "var(--text-muted)" }}>No active clips — engine scans 1m structure for the next setup.</p>
        ) : (
          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b text-[10px] uppercase tracking-wider" style={{ borderColor: "var(--border-subtle)", color: "var(--text-muted)" }}>
                  <th className="py-2 pr-3">Strategy</th>
                  <th className="py-2 pr-3">Side</th>
                  <th className="py-2 pr-3 text-right">Entry</th>
                  <th className="py-2 pr-3 text-right">Mark</th>
                  <th className="py-2 pr-3 text-right">Notional</th>
                  <th className="py-2 text-right">uPnL</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((p) => (
                  <tr key={p.id} className="border-b" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="py-2 pr-3 font-medium">{p.strategyName}</td>
                    <td className="py-2 pr-3"><SideBadge side={p.side} /></td>
                    <td className="py-2 pr-3 text-right font-mono text-xs">{p.entryPrice.toFixed(2)}</td>
                    <td className="py-2 pr-3 text-right font-mono text-xs">{p.currentPrice.toFixed(2)}</td>
                    <td className="py-2 pr-3 text-right font-mono text-xs">{fmtUSD(p.notional)}</td>
                    <td className={`py-2 text-right font-mono text-xs font-semibold ${p.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                      {fmtUSD(p.unrealizedPnl, { signed: true })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="glass-panel px-5 py-5">
        <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Strategies (10)</h2>
        <p className="mt-1 text-xs" style={{ color: "var(--text-muted)" }}>Regime filter skips mean-reversion in violent tape and trend-chase in dead ranges.</p>
        <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {strategies.map((s) => (
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
        <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Recent trades</h2>
        {trades.length === 0 ? (
          <p className="mt-3 text-sm" style={{ color: "var(--text-muted)" }}>Ledger empty.</p>
        ) : (
          <div className="mt-3 overflow-x-auto max-h-[360px] overflow-y-auto">
            <table className="w-full text-left text-sm">
              <thead className="sticky top-0 bg-[var(--surface-1)]">
                <tr className="border-b text-[10px] uppercase tracking-wider" style={{ borderColor: "var(--border-subtle)", color: "var(--text-muted)" }}>
                  <th className="py-2 pr-2">Time</th>
                  <th className="py-2 pr-2">Strategy</th>
                  <th className="py-2 pr-2">Side</th>
                  <th className="py-2 pr-2">Exit</th>
                  <th className="py-2 pr-2 text-right">Net</th>
                </tr>
              </thead>
              <tbody>
                {trades.slice(0, 80).map((t) => (
                  <tr key={t.id} className="border-b" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="py-2 pr-2 whitespace-nowrap text-[11px]" style={{ color: "var(--text-muted)" }}>
                      {formatShortDate(t.exitTime)} {formatShortTime(t.exitTime)}
                    </td>
                    <td className="py-2 pr-2 text-xs">{t.strategyName}</td>
                    <td className="py-2 pr-2"><SideBadge side={t.side} /></td>
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
