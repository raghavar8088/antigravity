"use client";

import { useState } from "react";
import Nifty50MarketHero from "@/components/Nifty50MarketHero";
import useNiftyMarket from "@/hooks/useNiftyMarket";
import useNiftyStocksEngine, {
  type StockQuoteDisplay,
  type StockOptionPosition,
  type StockOptionTrade,
  type StrategyDisplayStatus,
  type StocksEngineStats,
} from "@/hooks/useNiftyStocksEngine";

const INITIAL_BALANCE = 1_000_000;

// ─── Formatters ────────────────────────────────────────────────────────────────

function fmtINR(v: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(v).toLocaleString("en-IN", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (signed) return `${v >= 0 ? "+" : "-"}₹${abs}`;
  return `₹${abs}`;
}

function fmtPct(v: number, signed = false) {
  const prefix = signed ? (v >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(v).toFixed(2)}%`;
}

function fmtTime(iso: string): string {
  if (!iso) return "--";
  try {
    return new Date(iso).toLocaleTimeString("en-IN", {
      hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false,
    });
  } catch { return "--"; }
}

function fmtHold(seconds: number): string {
  if (!seconds || seconds <= 0) return "0s";
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

// ─── Signal score colour helper ───────────────────────────────────────────────

function scoreColor(score: number): string {
  if (score >= 80) return "var(--green)";
  if (score >= 68) return "var(--amber)";
  return "var(--text-secondary)";
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function SummaryCard({ label, value, detail, accent = "" }: {
  label: string; value: string; detail?: string; accent?: string;
}) {
  return (
    <div className="summary-card flex min-h-[110px] flex-col justify-between gap-2">
      <div className="summary-label">{label}</div>
      <div className={`summary-value ${accent}`}>{value}</div>
      {detail && <div className="text-xs" style={{ color: "var(--text-secondary)" }}>{detail}</div>}
    </div>
  );
}

function MetricCard({ label, value, detail, accent = "" }: {
  label: string; value: string; detail?: string; accent?: string;
}) {
  return (
    <div className="metric-card flex min-h-[90px] flex-col justify-between gap-2">
      <div className="metric-label">{label}</div>
      <div className={`metric-value ${accent}`}>{value}</div>
      {detail && <div className="text-xs" style={{ color: "var(--text-secondary)" }}>{detail}</div>}
    </div>
  );
}

function OptionTypeBadge({ type }: { type: "CE" | "PE" }) {
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
      style={{
        background: type === "CE" ? "rgba(24,128,56,0.12)" : "rgba(217,48,37,0.12)",
        color: type === "CE" ? "var(--green)" : "var(--red)",
      }}
    >
      {type}
    </span>
  );
}

function StatusBadge({ status }: { status: StrategyDisplayStatus["status"] }) {
  const map: Record<string, { bg: string; color: string; label: string }> = {
    WARMING:     { bg: "rgba(100,100,100,0.10)", color: "var(--text-secondary)", label: "WARMING" },
    READY:       { bg: "rgba(26,115,232,0.10)",  color: "var(--blue)",           label: "READY" },
    IN_POSITION: { bg: "rgba(24,128,56,0.10)",   color: "var(--green)",          label: "IN POSITION" },
    COOLING:     { bg: "rgba(245,124,0,0.10)",   color: "var(--amber)",          label: "COOLING" },
  };
  const t = map[status] ?? map.READY;
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em]"
      style={{ background: t.bg, color: t.color }}
    >
      {t.label}
    </span>
  );
}

function ExitBadge({ reason }: { reason: string }) {
  const map: Record<string, { bg: string; color: string }> = {
    TP: { bg: "rgba(24,128,56,0.12)", color: "var(--green)" },
    SL: { bg: "rgba(217,48,37,0.12)", color: "var(--red)" },
  };
  const t = map[reason] ?? { bg: "var(--surface-3)", color: "var(--text-secondary)" };
  return (
    <span className="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={{ background: t.bg, color: t.color }}>
      {reason}
    </span>
  );
}

// ─── Stock Screener Grid ───────────────────────────────────────────────────────

function StockCard({ q }: { q: StockQuoteDisplay }) {
  const hasData = q.ltp > 0;
  const positive = q.changePct >= 0;
  const hasSignal = q.signalScore >= 62;
  const hasPosition = q.hasPosition;

  return (
    <div
      className="relative rounded-[16px] border px-3 py-3 transition-all"
      style={{
        borderColor: hasPosition
          ? "rgba(245,124,0,0.5)"
          : hasSignal
          ? "rgba(26,115,232,0.35)"
          : "var(--border)",
        background: hasPosition
          ? "rgba(245,124,0,0.06)"
          : hasSignal
          ? "rgba(26,115,232,0.04)"
          : "var(--surface-2)",
      }}
    >
      {/* Header row */}
      <div className="flex items-center justify-between gap-1">
        <span className="text-xs font-semibold" style={{ color: "var(--text-primary)" }}>
          {q.symbol}
        </span>
        {hasPosition && q.strategyLabel && (
          <span
            className="rounded-full px-1.5 py-0.5 text-[9px] font-bold uppercase"
            style={{ background: "rgba(245,124,0,0.18)", color: "var(--amber)" }}
          >
            {q.strategyLabel}
          </span>
        )}
        {!hasPosition && hasSignal && (
          <span
            className="h-1.5 w-1.5 rounded-full"
            style={{ background: scoreColor(q.signalScore) }}
          />
        )}
      </div>

      {/* Price */}
      <div className="mt-1">
        {hasData ? (
          <>
            <div className="font-mono text-sm font-semibold leading-none" style={{ color: "var(--text-primary)" }}>
              {q.ltp.toLocaleString("en-IN", { maximumFractionDigits: 1 })}
            </div>
            <div
              className="mt-0.5 text-[11px] font-medium"
              style={{ color: positive ? "var(--green)" : "var(--red)" }}
            >
              {fmtPct(q.changePct, true)}
            </div>
          </>
        ) : (
          <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
            Waiting...
          </div>
        )}
      </div>

      {/* Sector */}
      <div className="mt-1.5 text-[9px] uppercase tracking-wider" style={{ color: "var(--text-secondary)" }}>
        {q.sector}
      </div>

      {/* Signal score bar */}
      {q.signalScore > 0 && (
        <div className="mt-2 h-1 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
          <div
            className="h-full rounded-full transition-all"
            style={{
              width: `${Math.min(100, q.signalScore)}%`,
              background: scoreColor(q.signalScore),
            }}
          />
        </div>
      )}
    </div>
  );
}

function StockScreenerGrid({ quotes }: { quotes: StockQuoteDisplay[] }) {
  const live = quotes.filter((q) => q.ltp > 0).length;
  const withSignal = quotes.filter((q) => q.signalScore >= 62).length;
  const inPosition = quotes.filter((q) => q.hasPosition).length;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
            NIFTY 50 Stock Scanner
          </h2>
          <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
            Live LTP for all 50 constituent stocks via Angel One API
          </div>
        </div>
        <div className="flex flex-wrap gap-2 text-[11px]">
          <span className="rounded-full border border-zinc-200 bg-white px-3 py-1 font-medium text-zinc-600">
            {live}/50 live
          </span>
          {withSignal > 0 && (
            <span className="rounded-full px-3 py-1 font-medium" style={{ background: "rgba(26,115,232,0.10)", color: "var(--blue)" }}>
              {withSignal} signal{withSignal !== 1 ? "s" : ""}
            </span>
          )}
          {inPosition > 0 && (
            <span className="rounded-full px-3 py-1 font-medium" style={{ background: "rgba(245,124,0,0.10)", color: "var(--amber)" }}>
              {inPosition} in position
            </span>
          )}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6">
        {quotes.map((q) => (
          <StockCard key={q.symbol} q={q} />
        ))}
      </div>
      <div className="mt-3 text-[10px]" style={{ color: "var(--text-secondary)" }}>
        Signal bar: blue = moderate (62–79) · green = strong (80+) · orange border = open position
      </div>
    </div>
  );
}

// ─── Active Positions Panel ────────────────────────────────────────────────────

function PositionsPanel({ positions }: { positions: StockOptionPosition[] }) {
  if (positions.length === 0) {
    return (
      <div
        className="flex min-h-[200px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm"
        style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}
      >
        No option positions open yet. Engine is scanning all 50 NIFTY stocks for setups.
      </div>
    );
  }

  const totalUnrealized = positions.reduce((s, p) => s + p.unrealizedPnl, 0);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-medium uppercase tracking-[0.12em]" style={{ color: "var(--text-secondary)" }}>
          {positions.length} open option position{positions.length !== 1 ? "s" : ""}
        </span>
        <span
          className="rounded-full border px-3 py-1 text-xs font-medium"
          style={{
            background: totalUnrealized >= 0 ? "rgba(24,128,56,0.10)" : "rgba(217,48,37,0.10)",
            color: totalUnrealized >= 0 ? "var(--green)" : "var(--red)",
            borderColor: totalUnrealized >= 0 ? "rgba(24,128,56,0.14)" : "rgba(217,48,37,0.14)",
          }}
        >
          Unrealized {fmtINR(totalUnrealized, { signed: true })}
        </span>
      </div>

      <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
        <table className="w-full min-w-[900px] text-left text-sm">
          <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            <tr className="text-[11px] uppercase tracking-[0.12em]">
              <th className="px-4 py-3 font-medium">Stock · Option</th>
              <th className="px-4 py-3 font-medium">Strategy</th>
              <th className="px-4 py-3 font-medium">Entry</th>
              <th className="px-4 py-3 font-medium">Premium</th>
              <th className="px-4 py-3 font-medium">TP / SL</th>
              <th className="px-4 py-3 font-medium">Lots</th>
              <th className="px-4 py-3 font-medium">Opened</th>
              <th className="px-4 py-3 font-medium text-right">PnL</th>
            </tr>
          </thead>
          <tbody>
            {positions.map((pos) => {
              const progress = ((pos.currentPremium - pos.slPremium) / (pos.tpPremium - pos.slPremium)) * 100;
              const clamped = Math.max(0, Math.min(100, progress));
              return (
                <tr key={pos.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <OptionTypeBadge type={pos.optionType} />
                      <div>
                        <div className="font-semibold" style={{ color: "var(--text-primary)" }}>{pos.symbol}</div>
                        <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{pos.sector}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                    {pos.strategyName.replace(/_/g, " ")}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">
                    <div style={{ color: "var(--text-primary)" }}>
                      ₹{pos.entryUnderlying.toLocaleString("en-IN", { maximumFractionDigits: 1 })}
                    </div>
                    <div style={{ color: "var(--text-secondary)" }}>
                      ₹{pos.currentUnderlying.toLocaleString("en-IN", { maximumFractionDigits: 1 })} now
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">
                    <div style={{ color: "var(--text-primary)" }}>
                      ₹{pos.entryPremium.toFixed(2)} → ₹{pos.currentPremium.toFixed(2)}
                    </div>
                    <div className="mt-1 h-1.5 w-24 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
                      <div
                        className="h-full rounded-full"
                        style={{
                          width: `${clamped}%`,
                          background: pos.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)",
                        }}
                      />
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">
                    <div style={{ color: "var(--green)" }}>TP ₹{pos.tpPremium.toFixed(2)}</div>
                    <div style={{ color: "var(--red)" }}>SL ₹{pos.slPremium.toFixed(2)}</div>
                  </td>
                  <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                    <div>{pos.numShares} shares</div>
                    <div>Lot {pos.lotSize}</div>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                    {fmtTime(pos.entryTime)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div
                      className="font-mono text-sm font-semibold"
                      style={{ color: pos.unrealizedPnl >= 0 ? "var(--green)" : "var(--red)" }}
                    >
                      {fmtINR(pos.unrealizedPnl, { signed: true })}
                    </div>
                    <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                      {fmtPct(pos.returnPct, true)}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ─── Strategy Roster ───────────────────────────────────────────────────────────

function StrategyRoster({ strategies }: { strategies: StrategyDisplayStatus[] }) {
  return (
    <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
      <table className="w-full min-w-[820px] text-left text-sm">
        <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
          <tr className="text-[11px] uppercase tracking-[0.12em]">
            <th className="px-4 py-3 font-medium">Strategy</th>
            <th className="px-4 py-3 font-medium">Type</th>
            <th className="px-4 py-3 font-medium">Category</th>
            <th className="px-4 py-3 font-medium">Status</th>
            <th className="px-4 py-3 font-medium">Score</th>
            <th className="px-4 py-3 font-medium">Trades</th>
            <th className="px-4 py-3 font-medium">Win %</th>
            <th className="px-4 py-3 font-medium text-right">PnL</th>
          </tr>
        </thead>
        <tbody>
          {strategies.map((s) => (
            <tr key={s.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
              <td className="px-4 py-3">
                <div className="font-medium text-xs" style={{ color: "var(--text-primary)" }}>
                  {s.name.replace(/_/g, " ")}
                </div>
                {s.currentStock && (
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                    {s.status === "IN_POSITION" ? `In: ${s.currentStock}` : `Last: ${s.currentStock}`}
                  </div>
                )}
              </td>
              <td className="px-4 py-3">
                <OptionTypeBadge type={s.optionType} />
              </td>
              <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                {s.category}
              </td>
              <td className="px-4 py-3">
                <StatusBadge status={s.status} />
              </td>
              <td className="px-4 py-3">
                {s.score > 0 ? (
                  <div className="flex items-center gap-1.5">
                    <div className="h-1.5 w-12 overflow-hidden rounded-full" style={{ background: "var(--surface-3)" }}>
                      <div
                        className="h-full rounded-full"
                        style={{ width: `${Math.min(100, s.score)}%`, background: scoreColor(s.score) }}
                      />
                    </div>
                    <span className="font-mono text-xs" style={{ color: scoreColor(s.score) }}>
                      {s.score.toFixed(0)}
                    </span>
                  </div>
                ) : (
                  <span className="text-xs" style={{ color: "var(--text-secondary)" }}>—</span>
                )}
              </td>
              <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-primary)" }}>
                {s.totalTrades}
              </td>
              <td className="px-4 py-3 font-mono text-xs" style={{ color: s.winRate >= 50 ? "var(--green)" : "var(--text-primary)" }}>
                {s.totalTrades > 0 ? fmtPct(s.winRate) : "—"}
              </td>
              <td className="px-4 py-3 text-right font-mono text-xs" style={{ color: s.totalPnl >= 0 ? "var(--green)" : "var(--red)" }}>
                {s.totalTrades > 0 ? fmtINR(s.totalPnl, { signed: true }) : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Trade Ledger ──────────────────────────────────────────────────────────────

function TradeLedger({ trades }: { trades: StockOptionTrade[] }) {
  const [showAll, setShowAll] = useState(false);
  const visible = showAll ? trades : trades.slice(0, 12);

  if (trades.length === 0) {
    return (
      <div
        className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-10 text-center text-sm"
        style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}
      >
        No completed trades yet. Engine will log every closed option position here.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="overflow-x-auto rounded-[18px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
        <table className="w-full min-w-[980px] text-left text-sm">
          <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
            <tr className="text-[11px] uppercase tracking-[0.12em]">
              <th className="px-4 py-3 font-medium">Time</th>
              <th className="px-4 py-3 font-medium">Stock · Option</th>
              <th className="px-4 py-3 font-medium">Strategy</th>
              <th className="px-4 py-3 font-medium">Underlying</th>
              <th className="px-4 py-3 font-medium">Premium</th>
              <th className="px-4 py-3 font-medium">Shares</th>
              <th className="px-4 py-3 font-medium">Hold</th>
              <th className="px-4 py-3 font-medium">Exit</th>
              <th className="px-4 py-3 font-medium text-right">PnL</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((t) => (
              <tr key={t.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                  {fmtTime(t.exitTime)}
                </td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <OptionTypeBadge type={t.optionType} />
                    <span className="font-semibold text-xs" style={{ color: "var(--text-primary)" }}>{t.symbol}</span>
                  </div>
                </td>
                <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                  {t.strategyName.replace(/_/g, " ")}
                </td>
                <td className="px-4 py-3 font-mono text-xs">
                  <div style={{ color: "var(--text-secondary)" }}>
                    ₹{t.entryUnderlying.toLocaleString("en-IN", { maximumFractionDigits: 1 })}
                    {" → "}₹{t.exitUnderlying.toLocaleString("en-IN", { maximumFractionDigits: 1 })}
                  </div>
                </td>
                <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                  ₹{t.entryPremium.toFixed(2)} → ₹{t.exitPremium.toFixed(2)}
                </td>
                <td className="px-4 py-3 font-mono text-xs" style={{ color: "var(--text-secondary)" }}>
                  {t.numShares}
                </td>
                <td className="px-4 py-3 text-xs" style={{ color: "var(--text-secondary)" }}>
                  {fmtHold(t.holdSeconds)}
                </td>
                <td className="px-4 py-3">
                  <ExitBadge reason={t.exitReason} />
                </td>
                <td className="px-4 py-3 text-right">
                  <div
                    className="font-mono text-sm font-semibold"
                    style={{ color: t.netPnl >= 0 ? "var(--green)" : "var(--red)" }}
                  >
                    {fmtINR(t.netPnl, { signed: true })}
                  </div>
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>
                    {fmtPct(t.returnPct, true)}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {trades.length > 12 && (
        <button type="button" onClick={() => setShowAll((p) => !p)} className="btn-primary w-full">
          {showAll ? "Show fewer trades" : `Show all ${trades.length} trades`}
        </button>
      )}
    </div>
  );
}

// ─── Equity sparkline ──────────────────────────────────────────────────────────

function EquitySparkline({ stats }: { stats: StocksEngineStats }) {
  const positive = stats.equity >= INITIAL_BALANCE;
  return (
    <div className="flex items-center gap-3 px-1">
      <div
        className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full"
        style={{ background: positive ? "rgba(24,128,56,0.12)" : "rgba(217,48,37,0.12)" }}
      >
        <span className="text-base">{positive ? "▲" : "▼"}</span>
      </div>
      <div>
        <div className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--text-secondary)" }}>
          Session
        </div>
        <div className="font-mono text-sm font-semibold" style={{ color: positive ? "var(--green)" : "var(--red)" }}>
          {fmtINR(stats.sessionPnl, { signed: true })}
        </div>
      </div>
    </div>
  );
}

// ─── Main component ────────────────────────────────────────────────────────────

export default function Nifty50StocksScalper() {
  const market = useNiftyMarket();
  const { quotes, positions, trades, strategies, stats, reset } = useNiftyStocksEngine();

  const [isResetting, setIsResetting] = useState(false);

  const topStrategy = [...strategies]
    .filter((s) => s.totalTrades > 0)
    .sort((a, b) => b.totalPnl - a.totalPnl)[0] ?? null;

  const bestTrade = [...trades].sort((a, b) => b.netPnl - a.netPnl)[0] ?? null;

  const handleReset = () => {
    if (!confirm("Reset the NIFTY 50 Stocks Option Scalper? All positions, trades, and capital history will be cleared.")) {
      return;
    }
    setIsResetting(true);
    reset();
    setTimeout(() => setIsResetting(false), 400);
  };

  return (
    <div className="space-y-6">
      {/* Market Hero */}
      <Nifty50MarketHero
        market={market}
        subtitle="Live NIFTY 50 index — autonomous option scalper for all 50 constituent stocks."
      />

      {/* Hero Panel */}
      <section className="glass-panel overflow-hidden">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.1fr)_420px]">
          <div className="px-6 py-7 md:px-8">
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <span className="pill-green">LIVE</span>
              <span
                className="rounded-full px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]"
                style={{ background: "rgba(26,115,232,0.10)", color: "var(--blue)" }}
              >
                NIFTY 50 STOCKS · OPTIONS
              </span>
              <span className="rounded-full border border-orange-200 bg-orange-50 px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em] text-orange-700">
                Angel One
              </span>
              {stats.warmingUp && (
                <span
                  className="rounded-full px-3 py-1 text-[10px] font-medium uppercase tracking-[0.12em]"
                  style={{ background: "rgba(100,100,100,0.10)", color: "var(--text-secondary)" }}
                >
                  Warming up (~60s)
                </span>
              )}
            </div>
            <div className="max-w-3xl">
              <div className="text-xs font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
                NIFTY 50 STOCKS OPTION EQUITY
              </div>
              <div className={`mt-3 text-[clamp(2.65rem,5vw,3.5rem)] font-semibold leading-none tracking-tight ${stats.equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                {fmtINR(stats.equity)}
              </div>
              <div className="mt-3 max-w-2xl text-sm leading-6" style={{ color: "var(--text-secondary)" }}>
                Autonomous option scalper — fetches live LTP for all 50 NIFTY stocks every 3 seconds via Angel One API,
                runs 12 signal strategies (6 CE + 6 PE), and autonomously enters ATM option positions when a high-conviction
                setup is detected. TP / SL tracked in real time.
              </div>
            </div>
          </div>

          <div
            className="flex flex-col justify-between gap-5 border-t px-6 py-7 md:px-8 lg:border-l lg:border-t-0"
            style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}
          >
            <div className="space-y-3">
              <div className="text-xs font-semibold uppercase tracking-[0.16em]" style={{ color: "var(--text-secondary)" }}>
                Engine pulse
              </div>
              <div className="grid grid-cols-2 gap-3">
                <MetricCard
                  label="Open Positions"
                  value={`${stats.openPositions}`}
                  detail={`Max ${8} concurrent`}
                />
                <MetricCard
                  label="Active Strategies"
                  value={`${stats.activeStrategies}/12`}
                  detail={stats.warmingUp ? "Warming up…" : "All 50 stocks scanned"}
                />
              </div>
              <EquitySparkline stats={stats} />
            </div>

            <button
              type="button"
              onClick={handleReset}
              disabled={isResetting}
              className="btn-gold w-full"
            >
              {isResetting ? "Resetting…" : "Reset Stock Options Account"}
            </button>
          </div>
        </div>
      </section>

      {/* Summary Stats */}
      <section className="grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-7">
        <SummaryCard
          label="Cash Balance"
          value={fmtINR(stats.balance)}
          detail="Available capital"
        />
        <SummaryCard
          label="Session PnL"
          value={fmtINR(stats.sessionPnl, { signed: true })}
          detail="vs ₹10,00,000 start"
          accent={stats.sessionPnl >= 0 ? "profit-positive" : "profit-negative"}
        />
        <SummaryCard
          label="Realized PnL"
          value={fmtINR(stats.realizedPnl, { signed: true })}
          detail={`${stats.totalTrades} completed trades`}
          accent={stats.realizedPnl >= 0 ? "profit-positive" : "profit-negative"}
        />
        <SummaryCard
          label="Unrealized PnL"
          value={fmtINR(stats.unrealizedPnl, { signed: true })}
          detail={`${stats.openPositions} open positions`}
          accent={stats.unrealizedPnl >= 0 ? "profit-positive" : "profit-negative"}
        />
        <SummaryCard
          label="Win Rate"
          value={stats.totalTrades > 0 ? fmtPct(stats.winRate) : "—"}
          detail={topStrategy ? `Leader: ${topStrategy.name.replace(/_/g, " ")}` : "Awaiting trades"}
          accent={stats.winRate >= 50 ? "profit-positive" : ""}
        />
        <SummaryCard
          label="Best Trade"
          value={bestTrade ? fmtINR(bestTrade.netPnl, { signed: true }) : "—"}
          detail={bestTrade ? `${bestTrade.symbol} ${bestTrade.optionType}` : "None yet"}
          accent={bestTrade && bestTrade.netPnl >= 0 ? "profit-positive" : ""}
        />
        <SummaryCard
          label="Allocation / Trade"
          value="₹10,000"
          detail="ATM option premium outlay"
        />
      </section>

      {/* Stock Screener */}
      <StockScreenerGrid quotes={quotes} />

      {/* Active Positions + Strategy Roster */}
      <section className="grid gap-6 xl:grid-cols-[minmax(0,1.3fr)_minmax(380px,0.7fr)]">
        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="mb-4">
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Open Option Positions
            </h2>
            <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
              Live ATM CE/PE positions across NIFTY 50 stocks — delta-marked to market, auto-exit at TP/SL.
            </div>
          </div>
          <PositionsPanel positions={positions} />
        </div>

        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="mb-4">
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              12-Strategy Roster
            </h2>
            <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
              6 CE + 6 PE strategies scan all 50 stocks every 3 seconds for high-conviction setups.
            </div>
          </div>
          <StrategyRoster strategies={strategies} />
        </div>
      </section>

      {/* Trade Ledger */}
      <section className="glass-panel px-5 py-6 md:px-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em]" style={{ color: "var(--text-secondary)" }}>
              Option Trade Ledger
            </h2>
            <div className="mt-1 text-xs" style={{ color: "var(--text-secondary)" }}>
              All closed NIFTY 50 stock option positions — entry/exit premium, underlying move, P&L.
            </div>
          </div>
          <div className="text-xs font-medium" style={{ color: "var(--text-secondary)" }}>
            {stats.totalTrades} trade{stats.totalTrades !== 1 ? "s" : ""} recorded
          </div>
        </div>
        <TradeLedger trades={trades} />
      </section>

      {/* Footer */}
      <section className="glass-panel px-5 py-4 text-xs leading-6" style={{ color: "var(--text-secondary)" }}>
        NIFTY 50 Stock Option Scalper · Angel One SmartAPI · 50 stocks · 12 strategies (6 CE + 6 PE) ·
        ATM weekly options · Delta-based mark-to-market · ₹10,000 per position · Paper trading · Max 8 concurrent positions
      </section>
    </div>
  );
}
