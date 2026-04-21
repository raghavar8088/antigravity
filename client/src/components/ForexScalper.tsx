"use client";

import { useState } from "react";
import DailyPnlLedger from "@/components/DailyPnlLedger";
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
  if (!seconds || seconds <= 0) return "0s";
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remainingSeconds = seconds % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m ${remainingSeconds}s`;
  return `${remainingSeconds}s`;
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
    <div className="metric-card flex flex-col justify-between gap-1.5 py-3.5 px-4 min-h-[100px]">
      <div className="metric-label text-[10px] font-bold uppercase tracking-[0.14em] text-zinc-500">{label}</div>
      <div className={`metric-value text-xl font-bold tracking-tight ${accent}`}>{value}</div>
      {detail && <div className="text-[10px] font-medium text-zinc-400 line-clamp-1">{detail}</div>}
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

export default function ForexScalper({ actionsEnabled = false, quotes, positions, trades, strategies, stats, reset }: Props) {
  const [tab, setTab] = useState<"trades" | "strategies">("trades");
  const [isResetting, setIsResetting] = useState(false);
  const totalReturnPct = ((stats.equity - INITIAL_BALANCE) / INITIAL_BALANCE) * 100;
  const bestStrategy = [...strategies].sort((a, b) => b.totalPnl - a.totalPnl)[0];
  const hiddenTradeCount = Math.max(stats.totalTrades - trades.length, 0);

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!confirm("Reset the Forex paper account to $1,000,000? All history will be cleared.")) return;
    setIsResetting(true);
    reset();
    setTimeout(() => setIsResetting(false), 500);
  };

  return (
    <div className="space-y-6">
      {/* ── Hero ── */}
      <section className="glass-panel overflow-hidden px-6 py-7 md:px-8">
        <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div className="px-1">
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <span className="pill-green">LIVE</span>
              <span className="rounded-full px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]" style={{ background: "rgba(26,115,232,0.10)", color: "var(--blue)" }}>
                FOREX · 12 PAIRS
              </span>
              <span className="rounded-full border border-orange-200 bg-orange-50 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] text-orange-700">
                Yahoo Feed
              </span>
            </div>
            <div className="text-xs font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              FOREX TRADING EQUITY
            </div>
            <div className="mt-3 flex flex-wrap items-end gap-4">
              <div className={`text-[clamp(2.65rem,5vw,3.5rem)] font-semibold leading-none tracking-tight ${stats.equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                {fmtUSD(stats.equity)}
              </div>
              <div className={`pb-1 text-xl font-semibold leading-none ${stats.sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                {fmtPct(totalReturnPct, true)}
              </div>
            </div>
            <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
              Session PnL {fmtUSD(stats.sessionPnl, { signed: true })} · Forex Engine · Multi-Currency Setup
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 lg:mt-10">
            <button
              type="button"
              onClick={handleReset}
              disabled={!actionsEnabled || isResetting}
              className="btn-danger text-sm"
            >
              {isResetting ? "Resetting…" : "Reset Forex Account"}
            </button>
          </div>
        </div>

        <div className="mt-8 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <CompactMetric
            label="Active Pairs"
            value={`${stats.liveSymbols}/12`}
            detail={stats.warmingUp ? "Warming up…" : "Stable Yahoo feed"}
            accent="text-zinc-900"
          />
          <CompactMetric
            label="Open Exposure"
            value={`${stats.openPositions} Active`}
            detail={`${strategies.filter(s => s.status === "IN_POSITION").length} strategies in position`}
            accent="text-blue-600"
          />
          <CompactMetric
            label="Win Rate"
            value={`${stats.winRate.toFixed(1)}%`}
            detail={`${stats.totalTrades} completed trades`}
            accent={stats.winRate >= 50 ? "text-emerald-600" : "text-rose-600"}
          />
          <CompactMetric
            label="Best Tracker"
            value={bestStrategy ? (bestStrategy.currentSymbol || "Ready") : "Waiting"}
            detail={bestStrategy ? `${bestStrategy.name} | PnL ${fmtUSD(bestStrategy.totalPnl, { signed: true })}` : "Scanning signals"}
            accent={bestStrategy && bestStrategy.totalPnl >= 0 ? "text-emerald-600" : "text-zinc-900"}
          />
        </div>
      </section>

      {/* ── Summary metrics ── */}
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-6">
        <SummaryCard label="Cash Balance" value={fmtUSD(stats.balance)} />
        <SummaryCard label="Realized PnL" value={fmtUSD(stats.realizedPnl, { signed: true })} accent={stats.realizedPnl >= 0 ? "profit-positive" : "profit-negative"} />
        <SummaryCard label="Unrealized PnL" value={fmtUSD(stats.unrealizedPnl, { signed: true })} accent={stats.unrealizedPnl >= 0 ? "profit-positive" : "profit-negative"} />
        <SummaryCard label="Wins / Losses" value={`${stats.totalWins}/${stats.totalLosses}`} />
        <SummaryCard label="Live Strategies" value={`${strategies.filter(s => s.status === "READY" || s.status === "IN_POSITION").length}`} detail={`${strategies.length} total`} />
        <SummaryCard label="Risk Model" value="Fixed Lot" detail="Directional FX Scalp" />
      </div>

      {/* ── Pairs Scanner ── */}
      <div className="glass-panel px-5 py-6 md:px-6">
        <div className="mb-4">
          <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>Forex Pair Scanner</h2>
          <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>Live rates for 12 major currency pairs tracked for directional breakouts.</div>
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
          {quotes.map(q => <QuoteCard key={q.symbol} quote={q} />)}
        </div>
      </div>

      <DailyPnlLedger
        trades={trades}
        initialEquity={INITIAL_BALANCE}
        title="DAILY PNL LEDGER"
        description="Realized forex PnL grouped by exit day, with returns measured from that day's opening equity."
        emptyMessage="No closed forex trades yet, so there is no daily PnL ledger to display."
        formatCurrency={fmtUSD}
      />

      {/* ── Open Positions Snapshot ── */}
      {positions.length > 0 && (
        <div className="glass-panel px-5 py-5 md:px-6">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h2 className="flex items-center gap-3" style={{
              fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800,
              letterSpacing: "0.14em", color: "var(--text-secondary)",
            }}>
              Open Positions Snapshot
              <span className="font-mono" style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }}>
                Active currency trades being managed for directional moves.
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
                  <th className="px-4 py-3 font-medium">Pair</th>
                  <th className="px-4 py-3 font-medium">Side</th>
                  <th className="px-4 py-3 font-medium text-right">Entry / Current</th>
                  <th className="px-4 py-3 font-medium text-right">TP / SL</th>
                  <th className="px-4 py-3 font-medium text-right">PnL</th>
                </tr>
              </thead>
              <tbody>
                {positions.map(pos => (
                  <tr key={pos.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3">
                       <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{pos.symbol}</div>
                       <div className="text-[10px] text-zinc-400">{pos.strategyName}</div>
                    </td>
                    <td className="px-4 py-3"><SideBadge side={pos.side} /></td>
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

      {/* ── Tabs ── */}
      <div className="glass-panel p-6">
        <div className="flex gap-4 border-b mb-6 overflow-x-auto" style={{ borderColor: "var(--border)" }}>
          {(["trades", "strategies"] as const).map(t => (
             <button key={t} type="button" onClick={() => setTab(t)}
                className={`pb-2 text-sm font-bold uppercase tracking-wider transition-colors border-b-2 ${tab === t ? "border-emerald-500 text-emerald-600" : "border-transparent text-zinc-400 hover:text-zinc-600"}`}>
               {t} {t === "trades" ? `(${trades.length})` : ""}
             </button>
          ))}
        </div>

        {tab === "trades" && (
          trades.length === 0 ? (
            <div className="py-10 text-center text-sm" style={{ color: "var(--text-secondary)" }}>No completed trades recorded for this session.</div>
          ) : (
            <div className="space-y-3">
              {hiddenTradeCount > 0 ? (
                <div className="rounded-[14px] border px-4 py-3 text-xs" style={{ borderColor: "rgba(245,124,0,0.20)", background: "rgba(245,124,0,0.06)", color: "var(--text-secondary)" }}>
                  Showing the latest {trades.length.toLocaleString("en-US")} of {stats.totalTrades.toLocaleString("en-US")} closed forex trades restored for this session.
                </div>
              ) : null}
              <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
               <table className="w-full text-left text-sm" style={{ minWidth: 1180 }}>
                  <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                    <tr className="text-[11px] uppercase tracking-[0.12em]">
                      <th className="px-4 py-3 font-medium">Time</th>
                      <th className="px-4 py-3 font-medium">Strategy</th>
                      <th className="px-4 py-3 font-medium">Pair</th>
                      <th className="px-4 py-3 font-medium">Side</th>
                      <th className="px-4 py-3 text-right font-medium">Entry / Exit</th>
                      <th className="px-4 py-3 text-right font-medium">PnL</th>
                      <th className="px-4 py-3 text-right font-medium">Hold / Exit</th>
                    </tr>
                  </thead>
                  <tbody>
                    {trades.map(t => (
                      <tr key={t.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                        <td className="px-4 py-3 font-mono text-xs text-zinc-500">{fmtTime(t.exitTime)}</td>
                        <td className="px-4 py-3">
                          <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>{t.strategyName}</div>
                          <div className="text-[10px] text-zinc-400">{t.id}</div>
                        </td>
                        <td className="px-4 py-3 font-semibold">{t.symbol}</td>
                        <td className="px-4 py-3"><SideBadge side={t.side} /></td>
                        <td className="px-4 py-3 text-right font-mono text-xs">
                          <div>{fmtRate(t.entryPrice)}</div>
                          <div className="text-zinc-400">{fmtRate(t.exitPrice)}</div>
                        </td>
                        <td className="px-4 py-3 text-right">
                           <div className="font-mono text-sm font-semibold" style={{ color: t.netPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                              {fmtUSD(t.netPnl, { signed: true })}
                           </div>
                           <div className="text-[10px] text-zinc-400">{fmtPct(t.returnPct, true)}</div>
                        </td>
                        <td className="px-4 py-3 text-right text-xs text-zinc-500">{fmtHold(t.holdSeconds)} | {t.exitReason.replace(/_/g, " ")}</td>
                      </tr>
                    ))}
                  </tbody>
               </table>
            </div>
            </div>
          )
        )}

        {tab === "strategies" && (
           <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
              <table className="w-full text-left text-sm" style={{ minWidth: 900 }}>
                 <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                   <tr className="text-[11px] uppercase tracking-[0.12em]">
                     <th className="px-4 py-3 font-medium">Strategy</th>
                     <th className="px-4 py-3 font-medium">Side</th>
                     <th className="px-4 py-3 font-medium">Status</th>
                     <th className="px-4 py-3 text-right font-medium">Score</th>
                     <th className="px-4 py-3 text-right font-medium">PnL</th>
                   </tr>
                 </thead>
                 <tbody>
                   {strategies.map(s => (
                     <tr key={s.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                       <td className="px-4 py-3">
                          <div className="text-sm font-medium">{s.name}</div>
                          <div className="text-[10px] text-zinc-400">{s.category}</div>
                       </td>
                       <td className="px-4 py-3"><SideBadge side={s.side} /></td>
                       <td className="px-4 py-3"><StatusBadge status={s.status} /></td>
                       <td className="px-4 py-3 text-right font-mono text-xs" style={{ color: s.score >= 50 ? "var(--green)" : "var(--text-secondary)" }}>{s.score || "—"}</td>
                       <td className="px-4 py-3 text-right font-mono text-sm font-semibold" style={{ color: s.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                          {s.totalTrades > 0 ? fmtUSD(s.totalPnl, { signed: true }) : "—"}
                       </td>
                     </tr>
                   ))}
                 </tbody>
              </table>
           </div>
        )}
      </div>
    </div>
  );
}
