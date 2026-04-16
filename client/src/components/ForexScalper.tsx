"use client";

import { useState } from "react";
import type {
  ForexEngineStats,
  ForexPosition,
  ForexQuoteDisplay,
  ForexStrategyStatus,
  ForexTrade,
} from "@/hooks/useForexEngine";

function fmtUSD(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-US", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtRate(value: number) {
  // forex prices: show enough decimals
  if (value === 0) return "—";
  if (value >= 100) return value.toFixed(3);  // JPY pairs
  return value.toFixed(5);
}

function fmtPct(value: number, signed = false) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(3)}%`;
}

function fmtTime(iso: string) {
  if (!iso) return "--";
  try { return new Date(iso).toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }); }
  catch { return "--"; }
}

function fmtHold(seconds: number) {
  if (seconds <= 0) return "0s";
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
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
  const pos = side === "LONG";
  return (
    <span className="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
      style={{ background: pos ? "rgba(24,128,56,0.12)" : "rgba(217,48,37,0.12)", color: pos ? "var(--green)" : "var(--red)" }}>
      {side}
    </span>
  );
}

function StatusBadge({ status }: { status: ForexStrategyStatus["status"] }) {
  const tones: Record<ForexStrategyStatus["status"], { bg: string; color: string }> = {
    WARMING:     { bg: "rgba(100,100,100,0.1)", color: "var(--text-secondary)" },
    READY:       { bg: "rgba(26,115,232,0.1)",  color: "var(--blue)"           },
    IN_POSITION: { bg: "rgba(24,128,56,0.1)",   color: "var(--green)"          },
    COOLING:     { bg: "rgba(245,124,0,0.12)",  color: "var(--amber)"          },
  };
  const tone = tones[status];
  return (
    <span className="rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em]"
      style={{ background: tone.bg, color: tone.color }}>
      {status}
    </span>
  );
}

function RegimeBadge({ regime }: { regime: ForexStrategyStatus["regime"] }) {
  const tones: Record<ForexStrategyStatus["regime"], { bg: string; color: string }> = {
    UNKNOWN:        { bg: "rgba(100,100,100,0.1)", color: "var(--text-secondary)" },
    RANGE:          { bg: "rgba(26,115,232,0.1)", color: "var(--blue)" },
    HIGH_VOL:       { bg: "rgba(245,124,0,0.12)", color: "var(--amber)" },
    TRENDING_BULL:  { bg: "rgba(24,128,56,0.1)", color: "var(--green)" },
    TRENDING_BEAR:  { bg: "rgba(217,48,37,0.12)", color: "var(--red)" },
  };
  const tone = tones[regime];
  return (
    <span className="rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em]"
      style={{ background: tone.bg, color: tone.color }}>
      {regime.replace("_", " ")}
    </span>
  );
}

function QuoteCard({ quote }: { quote: ForexQuoteDisplay }) {
  const positive = quote.changePct >= 0;
  const activeSignal = quote.signalScore >= 66;
  return (
    <div className="rounded-[16px] border px-3 py-3 transition-all"
      style={{
        borderColor: quote.hasPosition ? "rgba(245,124,0,0.45)" : activeSignal ? "rgba(26,115,232,0.35)" : "var(--border)",
        background:  quote.hasPosition ? "rgba(245,124,0,0.06)" : activeSignal ? "rgba(26,115,232,0.04)" : "var(--surface-2)",
      }}>
      <div className="flex items-center justify-between gap-2">
        <div>
          <div className="text-xs font-semibold" style={{ color: "var(--text-primary)" }}>{quote.symbol}</div>
          <div className="text-[10px] uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>{quote.category}</div>
        </div>
        {quote.strategyLabel ? (
          <span className="rounded-full px-1.5 py-0.5 text-[9px] font-bold uppercase"
            style={{ background: "rgba(245,124,0,0.16)", color: "var(--amber)" }}>
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
      <div className="mt-2 h-1.5 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
        <div className="h-full rounded-full" style={{ width: `${Math.min(100, quote.signalScore)}%`, background: scoreColor(quote.signalScore) }} />
      </div>
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

type Tab = "positions" | "trades" | "strategies" | "quotes";

export default function ForexScalper({ actionsEnabled = false, quotes, positions, trades, strategies, stats, reset }: Props) {
  const [tab, setTab] = useState<Tab>("positions");
  const [isResetting, setIsResetting] = useState(false);
  const [stratFilter, setStratFilter] = useState<"ALL" | "LONG" | "SHORT">("ALL");
  const [stratSort, setStratSort] = useState<"score" | "pnl" | "winRate">("score");

  const actionTitle = actionsEnabled ? "Action buttons enabled." : "Set Action to Yes to enable reset.";

  const handleReset = async () => {
    if (!actionsEnabled) return;
    setIsResetting(true);
    await new Promise((r) => setTimeout(r, 300));
    reset();
    setIsResetting(false);
  };

  const sessionPnlPositive = stats.sessionPnl >= 0;
  const winRateColor = stats.winRate >= 55 ? "text-green" : stats.winRate >= 45 ? "" : "text-red";

  const filteredStrategies = strategies
    .filter((s) => stratFilter === "ALL" || s.side === stratFilter)
    .sort((a, b) => {
      if (stratSort === "score")   return b.score - a.score;
      if (stratSort === "pnl")     return b.totalPnl - a.totalPnl;
      if (stratSort === "winRate") return b.winRate - a.winRate;
      return 0;
    });
  const activeRoster = strategies.filter((s) => s.rosterState === "ACTIVE").length;
  const watchlistCount = strategies.length - activeRoster;
  const bestStrategy = [...strategies].sort((a, b) => b.score - a.score)[0];
  const hasBestSetup = Boolean(bestStrategy && bestStrategy.score > 0);

  return (
    <div className="flex flex-col gap-5">
      {/* ── Header ─────────────────────────────────────────────────── */}
      <div className="glass-panel px-5 py-4 flex items-center justify-between gap-4 flex-wrap">
        <div>
          <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
            💱 Forex Scalper
          </div>
          <div className="text-xs mt-0.5" style={{ color: "var(--text-secondary)" }}>
            {stats.liveSymbols}/12 pairs live · {stats.openPositions} open · {stats.totalTrades} trades
            {stats.warmingUp ? " · Warming up…" : ""}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <div className="text-xs rounded-full px-3 py-1 font-medium"
            style={{ background: stats.lastUpdateAt > 0 ? "rgba(24,128,56,0.12)" : "rgba(100,100,100,0.1)", color: stats.lastUpdateAt > 0 ? "var(--green)" : "var(--text-secondary)" }}>
            {stats.lastUpdateAt > 0 ? "LIVE" : "CONNECTING"}
          </div>
          <button type="button" disabled={!actionsEnabled || isResetting} title={actionTitle}
            className="btn-danger text-xs px-3 py-1.5" onClick={handleReset}>
            {isResetting ? "Resetting…" : "Reset"}
          </button>
        </div>
      </div>

      {/* ── Summary cards ──────────────────────────────────────────── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SummaryCard label="Equity" value={fmtUSD(stats.equity)} detail={`Balance ${fmtUSD(stats.balance)}`} />
        <SummaryCard label="Session P&L" value={fmtUSD(stats.sessionPnl, { signed: true })} accent={sessionPnlPositive ? "text-green" : "text-red"} detail={`Realized ${fmtUSD(stats.realizedPnl, { signed: true })}`} />
        <SummaryCard label="Open Positions" value={String(stats.openPositions)} detail={`Unrealized ${fmtUSD(stats.unrealizedPnl, { signed: true })}`} accent={stats.unrealizedPnl >= 0 ? "text-green" : "text-red"} />
        <SummaryCard label="Win Rate" value={`${stats.winRate.toFixed(1)}%`} accent={winRateColor} detail={`${stats.totalTrades} total trades`} />
      </div>

      {/* ── Metric strip ───────────────────────────────────────────── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
        <MetricCard label="Active Strategies" value={String(activeRoster)} detail={`${watchlistCount} warming/watchlist`} />
        <MetricCard label="Best Setup" value={hasBestSetup ? (bestStrategy?.currentSymbol || "Ready") : "Scanning"} detail={hasBestSetup && bestStrategy ? `${bestStrategy.regime} · score ${bestStrategy.score}` : "Waiting for confirmed FX signals"} />
        <MetricCard label="Realized P&L" value={fmtUSD(stats.realizedPnl, { signed: true })} accent={stats.realizedPnl >= 0 ? "text-green" : "text-red"} />
        <MetricCard label="Diagnostics" value={stats.warmingUp ? "Warming" : "Active"} detail={stats.diagnostics} />
        <MetricCard label="Live Pairs" value={`${stats.liveSymbols} / 12`} detail="Yahoo Finance feed" />
      </div>

      {/* ── Tabs ───────────────────────────────────────────────────── */}
      <div className="glass-panel px-5 py-4">
        <div className="flex gap-2 border-b mb-4 overflow-x-auto" style={{ borderColor: "var(--border)" }}>
          {(["positions", "trades", "strategies", "quotes"] as Tab[]).map((t) => (
            <button key={t} type="button" onClick={() => setTab(t)}
              className={`groww-tab pb-2 capitalize${tab === t ? " active" : ""}`}>
              {t}{t === "positions" ? ` (${positions.length})` : t === "trades" ? ` (${trades.length})` : ""}
            </button>
          ))}
        </div>

        {/* POSITIONS */}
        {tab === "positions" && (
          positions.length === 0 ? (
            <div className="py-10 text-center text-sm" style={{ color: "var(--text-secondary)" }}>
              No open positions — engine scanning 12 forex pairs
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-left" style={{ color: "var(--text-secondary)" }}>
                    <th className="pb-2 pr-3">Pair</th>
                    <th className="pb-2 pr-3">Strategy</th>
                    <th className="pb-2 pr-3">Side</th>
                    <th className="pb-2 pr-3 text-right">Entry</th>
                    <th className="pb-2 pr-3 text-right">Current</th>
                    <th className="pb-2 pr-3 text-right">TP</th>
                    <th className="pb-2 pr-3 text-right">SL</th>
                    <th className="pb-2 pr-3 text-right">P&L</th>
                    <th className="pb-2 text-right">Return</th>
                  </tr>
                </thead>
                <tbody>
                  {positions.map((pos) => (
                    <tr key={pos.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                      <td className="py-2 pr-3 font-semibold">{pos.symbol}</td>
                      <td className="py-2 pr-3 max-w-[140px] truncate" style={{ color: "var(--text-secondary)" }}>{pos.strategyName}</td>
                      <td className="py-2 pr-3"><SideBadge side={pos.side} /></td>
                      <td className="py-2 pr-3 text-right font-mono">{fmtRate(pos.entryPrice)}</td>
                      <td className="py-2 pr-3 text-right font-mono">{fmtRate(pos.currentPrice)}</td>
                      <td className="py-2 pr-3 text-right font-mono" style={{ color: "var(--green)" }}>{fmtRate(pos.tpPrice)}</td>
                      <td className="py-2 pr-3 text-right font-mono" style={{ color: "var(--red)" }}>{fmtRate(pos.slPrice)}</td>
                      <td className="py-2 pr-3 text-right font-mono" style={{ color: pos.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtUSD(pos.unrealizedPnl, { signed: true })}
                      </td>
                      <td className="py-2 text-right" style={{ color: pos.returnPct >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtPct(pos.returnPct, true)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        )}

        {/* TRADES */}
        {tab === "trades" && (
          trades.length === 0 ? (
            <div className="py-10 text-center text-sm" style={{ color: "var(--text-secondary)" }}>
              No completed trades yet
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-left" style={{ color: "var(--text-secondary)" }}>
                    <th className="pb-2 pr-3">Time</th>
                    <th className="pb-2 pr-3">Pair</th>
                    <th className="pb-2 pr-3">Side</th>
                    <th className="pb-2 pr-3 text-right">Entry</th>
                    <th className="pb-2 pr-3 text-right">Exit</th>
                    <th className="pb-2 pr-3 text-right">P&L</th>
                    <th className="pb-2 pr-3 text-right">Return</th>
                    <th className="pb-2 pr-3 text-right">Hold</th>
                    <th className="pb-2 text-right">Reason</th>
                  </tr>
                </thead>
                <tbody>
                  {trades.map((t) => (
                    <tr key={t.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                      <td className="py-2 pr-3 font-mono" style={{ color: "var(--text-secondary)" }}>{fmtTime(t.exitTime)}</td>
                      <td className="py-2 pr-3 font-semibold">{t.symbol}</td>
                      <td className="py-2 pr-3"><SideBadge side={t.side} /></td>
                      <td className="py-2 pr-3 text-right font-mono">{fmtRate(t.entryPrice)}</td>
                      <td className="py-2 pr-3 text-right font-mono">{fmtRate(t.exitPrice)}</td>
                      <td className="py-2 pr-3 text-right font-mono" style={{ color: t.netPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtUSD(t.netPnl, { signed: true })}
                      </td>
                      <td className="py-2 pr-3 text-right" style={{ color: t.returnPct >= 0 ? "var(--green)" : "var(--red)" }}>
                        {fmtPct(t.returnPct, true)}
                      </td>
                      <td className="py-2 pr-3 text-right font-mono">{fmtHold(t.holdSeconds)}</td>
                      <td className="py-2 text-right">
                        <span className="rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase"
                          style={{ background: t.exitReason === "TP" ? "rgba(24,128,56,0.12)" : t.exitReason === "SL" ? "rgba(217,48,37,0.12)" : "rgba(26,115,232,0.1)", color: t.exitReason === "TP" ? "var(--green)" : t.exitReason === "SL" ? "var(--red)" : "var(--blue)" }}>
                          {t.exitReason}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        )}

        {/* STRATEGIES */}
        {tab === "strategies" && (
          <div className="flex flex-col gap-3">
            <div className="flex flex-wrap items-center gap-2">
              {(["ALL", "LONG", "SHORT"] as const).map((f) => (
                <button key={f} type="button" onClick={() => setStratFilter(f)}
                  className={`groww-tab text-xs${stratFilter === f ? " active" : ""}`}>{f}</button>
              ))}
              <div className="ml-auto flex gap-1">
                {(["score", "pnl", "winRate"] as const).map((s) => (
                  <button key={s} type="button" onClick={() => setStratSort(s)}
                    className={`groww-tab text-xs capitalize${stratSort === s ? " active" : ""}`}>{s}</button>
                ))}
              </div>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-left" style={{ color: "var(--text-secondary)" }}>
                    <th className="pb-2 pr-3">Strategy</th>
                    <th className="pb-2 pr-3">Category</th>
                    <th className="pb-2 pr-3">Side</th>
                    <th className="pb-2 pr-3">Status</th>
                    <th className="pb-2 pr-3">Regime</th>
                    <th className="pb-2 pr-3">Pair</th>
                    <th className="pb-2 pr-3 text-right">Score</th>
                    <th className="pb-2 pr-3 text-right">Size</th>
                    <th className="pb-2 pr-3 text-right">Alloc</th>
                    <th className="pb-2 pr-3 text-right">W/L</th>
                    <th className="pb-2 pr-3 text-right">Win%</th>
                    <th className="pb-2 text-right">P&L</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredStrategies.map((s) => (
                    <tr key={s.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                      <td className="py-2 pr-3 max-w-[160px] truncate font-medium">{s.name}</td>
                      <td className="py-2 pr-3" style={{ color: "var(--text-secondary)" }}>{s.category}</td>
                      <td className="py-2 pr-3"><SideBadge side={s.side} /></td>
                      <td className="py-2 pr-3"><StatusBadge status={s.status} /></td>
                      <td className="py-2 pr-3"><RegimeBadge regime={s.regime} /></td>
                      <td className="py-2 pr-3 font-mono text-[11px]">{s.currentSymbol || "—"}</td>
                      <td className="py-2 pr-3 text-right font-mono" style={{ color: scoreColor(s.score) }}>{s.score > 0 ? s.score : "—"}</td>
                      <td className="py-2 pr-3 text-right font-mono">{s.sizeMultiplier.toFixed(2)}x</td>
                      <td className="py-2 pr-3 text-right font-mono">{fmtUSD(s.allocationUSD)}</td>
                      <td className="py-2 pr-3 text-right font-mono">{s.wins}/{s.losses}</td>
                      <td className="py-2 pr-3 text-right" style={{ color: s.winRate >= 55 ? "var(--green)" : s.winRate > 0 ? "var(--amber)" : "var(--text-secondary)" }}>
                        {s.totalTrades > 0 ? `${s.winRate.toFixed(0)}%` : "—"}
                      </td>
                      <td className="py-2 text-right font-mono" style={{ color: s.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                        {s.totalTrades > 0 ? fmtUSD(s.totalPnl, { signed: true }) : "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* QUOTES */}
        {tab === "quotes" && (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6">
            {quotes.map((q) => <QuoteCard key={q.symbol} quote={q} />)}
          </div>
        )}
      </div>
    </div>
  );
}
