"use client";

import { useEffect, useState } from "react";
import DailyPnlLedger from "@/components/DailyPnlLedger";
import { formatShortDate, formatShortTime } from "@/lib/time";
import type {
  GoldEngineStats,
  GoldPosition,
  GoldQuoteDisplay,
  GoldStrategyStatus,
  GoldTrade,
} from "@/hooks/useForexGoldEngine";

const INITIAL_BALANCE = 1_000_000;

function fmtUSD(value: number, opts: { signed?: boolean; decimals?: number } = {}) {
  const { signed = false, decimals = 2 } = opts;
  const abs = Math.abs(value).toLocaleString("en-US", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (signed) return `${value >= 0 ? "+" : "-"}$${abs}`;
  return `$${abs}`;
}

function fmtPrice(value: number) {
  if (value <= 0) return "Waiting...";
  return value.toLocaleString("en-US", {
    minimumFractionDigits: value >= 100 ? 2 : 4,
    maximumFractionDigits: value >= 100 ? 2 : 4,
  });
}

function fmtPct(value: number, signed = false, decimals = 2) {
  const prefix = signed ? (value >= 0 ? "+" : "-") : "";
  return `${prefix}${Math.abs(value).toFixed(decimals)}%`;
}

function formatElapsedSeconds(total: number) {
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m ${seconds}s`;
}

function formatTradeDuration(entryTime: string, exitTime: string) {
  const entry = new Date(entryTime);
  const exit = new Date(exitTime);
  if (Number.isNaN(entry.getTime()) || Number.isNaN(exit.getTime())) return "-";
  const totalSeconds = Math.max(0, Math.floor((exit.getTime() - entry.getTime()) / 1000));
  return formatElapsedSeconds(totalSeconds);
}

type StrategyNumberMap = Record<string, number>;

function resolveNum(name: string, id: number | undefined, map: StrategyNumberMap) {
  if (id && id > 0) return id;
  return map[name] || 0;
}

function fmtLabel(name: string, id: number | undefined, map: StrategyNumberMap) {
  const num = resolveNum(name, id, map);
  return num ? `${num}. ${name}` : name;
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

function SummaryCard({ label, value, accent, detail }: { label: string; value: string; accent: string; detail?: string }) {
  return (
    <div className="summary-card flex min-h-[112px] flex-col justify-between gap-3">
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
  const positive = side === "LONG";
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider"
      style={{
        background: positive ? "rgba(24,128,56,0.12)" : "rgba(217,48,37,0.12)",
        color: positive ? "var(--green)" : "var(--red)",
      }}
    >
      {side}
    </span>
  );
}

function StatusBadge({ status }: { status: GoldStrategyStatus["status"] }) {
  const map: Record<GoldStrategyStatus["status"], string> = {
    WARMING: "border-zinc-200 bg-zinc-50 text-zinc-600",
    READY: "border-emerald-200 bg-emerald-50 text-emerald-700",
    IN_POSITION: "border-blue-200 bg-blue-50 text-blue-700",
    COOLING: "border-amber-200 bg-amber-50 text-amber-700",
  };
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${map[status] ?? "border-zinc-200 bg-zinc-50 text-zinc-500"}`}>
      {status.replace("_", " ")}
    </span>
  );
}

function RosterBadge({ rosterState }: { rosterState: GoldStrategyStatus["rosterState"] }) {
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
  const upper = reason.toUpperCase();
  const cls = upper.includes("TP") || upper.includes("PROFIT")
    ? "border-emerald-200 bg-emerald-50 text-emerald-700"
    : upper.includes("SL") || upper.includes("LOSS")
      ? "border-rose-200 bg-rose-50 text-rose-700"
      : "border-zinc-200 bg-zinc-50 text-zinc-600";
  return (
    <span className={`rounded-md border px-2 py-0.5 text-[10px] font-bold tracking-widest ${cls}`}>
      {reason.replace(/_/g, " ")}
    </span>
  );
}

function LivePositionsPanel({ positions, strategyNumbers }: { positions: GoldPosition[]; strategyNumbers: StrategyNumberMap }) {
  const totalUnrealized = positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const longCount = positions.filter((position) => position.side === "LONG").length;
  const shortCount = positions.filter((position) => position.side === "SHORT").length;

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <h2
        className="mb-5 flex flex-wrap items-center gap-3"
        style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}
      >
        <span className="pill-green">LIVE</span>
        OPEN GOLD POSITIONS
        <span style={{ color: "var(--text-muted)", fontSize: 10, fontWeight: 500 }} className="font-mono">
          ({positions.length} active)
        </span>
      </h2>

      {positions.length === 0 ? (
        <div
          className="flex min-h-[180px] items-center justify-center rounded-[20px] border border-dashed px-6 py-12 text-center text-sm"
          style={{ color: "var(--text-secondary)", borderColor: "var(--border)", background: "var(--surface-2)" }}
        >
          No open gold positions yet. The desk is warming the gold tape and will auto-trigger entries once the 100-strategy roster confirms.
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
            <table className="w-full text-left text-sm" style={{ minWidth: 1120 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Strategy</th>
                  <th className="px-4 py-3 font-medium">Side</th>
                  <th className="px-4 py-3 font-medium text-right">Entry / Current</th>
                  <th className="px-4 py-3 font-medium text-right">TP / SL</th>
                  <th className="px-4 py-3 font-medium text-right">Exposure</th>
                  <th className="px-4 py-3 font-medium text-right">PnL</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((position) => (
                  <tr key={position.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3">
                      <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
                        {fmtLabel(position.strategyName, position.strategyId, strategyNumbers)}
                      </div>
                      <div className="font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>
                        {position.symbol} | Qty {position.quantity.toFixed(4)}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <SideBadge side={position.side} />
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <div style={{ color: "var(--text-primary)" }}>{fmtPrice(position.entryPrice)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>{fmtPrice(position.currentPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <div className="text-emerald-600">{fmtPrice(position.tpPrice)}</div>
                      <div className="text-rose-600">{fmtPrice(position.slPrice)}</div>
                    </td>
                    <td className="px-4 py-3 text-right text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>{fmtUSD(position.notional)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>{formatShortTime(position.entryTime)}</div>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className={`font-mono text-sm font-bold ${position.unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                        {fmtUSD(position.unrealizedPnl, { signed: true })}
                      </div>
                      <div className={`text-[11px] ${position.returnPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                        {fmtPct(position.returnPct, true)}
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

function StrategiesPanel({ strategies, strategyNumbers }: { strategies: GoldStrategyStatus[]; strategyNumbers: StrategyNumberMap }) {
  const [showAll, setShowAll] = useState(false);
  const sorted = [...strategies].sort((left, right) => {
    if (right.totalPnl !== left.totalPnl) return right.totalPnl - left.totalPnl;
    if (right.score !== left.score) return right.score - left.score;
    return right.winRate - left.winRate;
  });
  const visible = showAll ? sorted : sorted.slice(0, 25);

  return (
    <div className="glass-panel px-5 py-6 md:px-6">
      <div className="mb-5 flex items-center justify-between gap-3">
        <h2 style={{ fontFamily: "var(--font-display)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em", color: "var(--text-secondary)" }}>
          GOLD STRATEGIES - LEADERBOARD ({strategies.length})
        </h2>
        {strategies.length > 25 ? (
          <button type="button" onClick={() => setShowAll((current) => !current)} className="btn-gold px-4 py-1.5 text-xs min-h-[32px]">
            {showAll ? "Top 25" : `All ${strategies.length}`}
          </button>
        ) : null}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left" style={{ minWidth: 1220 }}>
          <thead>
            <tr className="border-b" style={{ borderColor: "var(--border)" }}>
              {["ID", "Strategy", "Side", "Roster", "Runtime", "Score", "Trades", "Allocation", "PnL", "Regime"].map((heading, index) => (
                <th
                  key={heading}
                  className={`py-2 px-3 text-[10px] font-bold uppercase tracking-widest ${index >= 7 && index <= 8 ? "text-right" : ""}`}
                  style={{ color: "var(--text-muted)" }}
                >
                  {heading}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((strategy, index) => (
              <tr key={strategy.id} className="border-b transition-colors hover:bg-black/[0.015]" style={{ borderColor: "var(--border-subtle)" }}>
                <td className="py-2.5 px-3 text-xs font-mono" style={{ color: "var(--text-muted)" }}>
                  {resolveNum(strategy.name, strategy.id, strategyNumbers) || index + 1}
                </td>
                <td className="py-2.5 px-3">
                  <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
                    {fmtLabel(strategy.name, strategy.id, strategyNumbers)}
                  </div>
                  <div className="text-[11px]" style={{ color: "var(--text-secondary)" }}>{strategy.category}</div>
                </td>
                <td className="py-2.5 px-3">
                  <SideBadge side={strategy.side} />
                </td>
                <td className="py-2.5 px-3">
                  <RosterBadge rosterState={strategy.rosterState} />
                </td>
                <td className="py-2.5 px-3">
                  <StatusBadge status={strategy.status} />
                </td>
                <td className="py-2.5 px-3 text-sm font-mono font-semibold" style={{ color: "var(--text-primary)" }}>
                  {strategy.score.toFixed(1)}
                </td>
                <td className="py-2.5 px-3 text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {strategy.totalTrades}T | {strategy.wins}W / {strategy.losses}L
                  <div className="text-[11px]">{strategy.totalTrades > 0 ? fmtPct(strategy.winRate) : "-"}</div>
                </td>
                <td className="py-2.5 px-3 text-right text-sm font-mono" style={{ color: "var(--text-secondary)" }}>
                  {fmtUSD(strategy.allocationUSD)}
                </td>
                <td className={`py-2.5 px-3 text-right text-sm font-mono font-bold ${strategy.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {strategy.totalTrades > 0 ? fmtUSD(strategy.totalPnl, { signed: true }) : "-"}
                </td>
                <td className="py-2.5 px-3 text-[11px]" style={{ color: "var(--text-secondary)" }}>
                  <div>{strategy.regime}</div>
                  {strategy.cooldownUntil ? <div>Until {formatShortTime(strategy.cooldownUntil)}</div> : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function DailyPnlPanel({ trades }: { trades: GoldTrade[] }) {
  return (
    <DailyPnlLedger
      trades={trades}
      initialEquity={INITIAL_BALANCE}
      title="DAILY PNL LEDGER"
      description="Realized gold PnL grouped by exit day, with returns measured from that day's opening equity."
      emptyMessage="No closed gold trades yet, so there is no daily PnL ledger to display."
      formatCurrency={fmtUSD}
    />
  );
}

function TradesPanel({ trades, strategyNumbers }: { trades: GoldTrade[]; strategyNumbers: StrategyNumberMap }) {
  const [showAll, setShowAll] = useState(false);
  const visibleTrades = showAll ? trades : trades.slice(0, 20);
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
          GOLD TRADING - TRADE HISTORY
          <span className="ml-3 font-mono font-normal" style={{ color: "var(--text-muted)", fontSize: 10 }}>
            ({totalTrades} total)
          </span>
        </h2>
        {trades.length > 20 ? (
          <button type="button" onClick={() => setShowAll((current) => !current)} className="btn-gold min-h-[32px] px-4 py-1.5 text-xs">
            {showAll ? "Latest 20" : `All ${trades.length}`}
          </button>
        ) : null}
      </div>

      {trades.length === 0 ? (
        <div className="rounded-2xl border border-dashed py-12 text-center" style={{ borderColor: "var(--border)", color: "var(--text-muted)", fontSize: 13 }}>
          No completed gold trades yet.
        </div>
      ) : (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
            <SummaryCard label="Trades" value={`${totalTrades}`} accent="text-zinc-900" />
            <SummaryCard label="Win Rate" value={`${winRate.toFixed(1)}%`} accent={winRate >= 50 ? "text-emerald-600" : "text-rose-600"} />
            <SummaryCard label="Net PnL" value={fmtUSD(totalPnl, { signed: true })} accent={totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
            <SummaryCard label="Profit Factor" value={profitFactor.toFixed(2)} accent={profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"} />
            <SummaryCard label="W / L" value={`${wins}/${losses}`} accent="text-zinc-900" />
          </div>

          <div className="overflow-x-auto rounded-[20px] border" style={{ borderColor: "var(--border)", background: "var(--surface)" }}>
            <table className="w-full text-left text-sm" style={{ minWidth: 1080 }}>
              <thead style={{ background: "var(--surface-2)", color: "var(--text-secondary)" }}>
                <tr className="text-[11px] uppercase tracking-[0.12em]">
                  <th className="px-4 py-3 font-medium">Time</th>
                  <th className="px-4 py-3 font-medium">Strategy</th>
                  <th className="px-4 py-3 font-medium">Side</th>
                  <th className="px-4 py-3 font-medium">Entry / Exit</th>
                  <th className="px-4 py-3 font-medium">PnL</th>
                  <th className="px-4 py-3 font-medium">Hold / Exit</th>
                </tr>
              </thead>
              <tbody>
                {visibleTrades.map((trade) => (
                  <tr key={trade.id} className="border-t" style={{ borderColor: "var(--border-subtle)" }}>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>{formatShortTime(trade.exitTime)}</div>
                      <div style={{ color: "var(--text-secondary)" }}>{formatShortDate(trade.exitTime)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
                        {fmtLabel(trade.strategyName, trade.strategyId, strategyNumbers)}
                      </div>
                      <div className="font-mono text-[11px]" style={{ color: "var(--text-secondary)" }}>{trade.id}</div>
                    </td>
                    <td className="px-4 py-3">
                      <SideBadge side={trade.side} />
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div className="font-mono" style={{ color: "var(--text-primary)" }}>
                        {fmtPrice(trade.entryPrice)} {"->"} {fmtPrice(trade.exitPrice)}
                      </div>
                      <div style={{ color: "var(--text-secondary)" }}>Qty {trade.quantity.toFixed(4)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <div className={`font-mono text-sm font-bold ${trade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                        {fmtUSD(trade.netPnl, { signed: true })}
                      </div>
                      <div className={`text-[11px] ${trade.returnPct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                        {fmtPct(trade.returnPct, true)}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs">
                      <div style={{ color: "var(--text-secondary)" }}>
                        {formatTradeDuration(trade.entryTime, trade.exitTime)}
                      </div>
                      <div className="mt-1">
                        <ExitBadge reason={trade.exitReason} />
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

type Props = {
  actionsEnabled?: boolean;
  quote: GoldQuoteDisplay;
  positions: GoldPosition[];
  trades: GoldTrade[];
  strategies: GoldStrategyStatus[];
  stats: GoldEngineStats;
  reset: () => void;
};

export default function ForexGoldScalper({
  actionsEnabled = false,
  quote,
  positions,
  trades,
  strategies,
  stats,
  reset,
}: Props) {
  const [sessionStartedAt] = useState(() => Date.now());
  const [currentTime, setCurrentTime] = useState(() => Date.now());
  const actionButtonTitle = actionsEnabled
    ? "Reset is enabled."
    : "Locked: reset hidden. The gold engine still runs in your session.";

  useEffect(() => {
    const interval = setInterval(() => setCurrentTime(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  const handleReset = () => {
    if (!actionsEnabled) return;
    if (!window.confirm("Reset the gold trading paper account to $1,000,000? All gold trade history will be cleared.")) return;
    reset();
  };

  const sessionRuntime = formatElapsedSeconds(Math.max(0, Math.floor((currentTime - sessionStartedAt) / 1000)));
  const realizedPnl = stats.realizedPnl ?? trades.reduce((sum, trade) => sum + trade.netPnl, 0);
  const unrealizedPnl = stats.unrealizedPnl ?? positions.reduce((sum, position) => sum + position.unrealizedPnl, 0);
  const sessionPnl = stats.sessionPnl ?? (realizedPnl + unrealizedPnl);
  const equity = stats.equity ?? (INITIAL_BALANCE + sessionPnl);
  const totalReturnPct = (sessionPnl / INITIAL_BALANCE) * 100;
  const grossProfit = trades.filter((trade) => trade.netPnl > 0).reduce((sum, trade) => sum + trade.netPnl, 0);
  const grossLoss = trades.filter((trade) => trade.netPnl < 0).reduce((sum, trade) => sum + Math.abs(trade.netPnl), 0);
  const totalTrades = Math.max(stats.totalTrades ?? 0, trades.length);
  const totalWins = stats.totalWins ?? trades.filter((trade) => trade.netPnl >= 0).length;
  const winRate = totalTrades > 0 ? (totalWins / totalTrades) * 100 : 0;
  const profitFactor = grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? grossProfit : 0;
  const openCount = Math.max(stats.openPositions ?? 0, positions.length);
  const bestStrategy = [...strategies].sort((left, right) => {
    if (right.totalPnl !== left.totalPnl) return right.totalPnl - left.totalPnl;
    return right.score - left.score;
  })[0] ?? null;
  const latestTrade = trades[0] ?? null;
  const bestTrade = trades.reduce<GoldTrade | null>((best, trade) => (!best || trade.netPnl > best.netPnl ? trade : best), null);
  const avgHoldSecs = trades.length > 0
    ? trades.reduce((sum, trade) => sum + trade.holdSeconds, 0) / trades.length
    : 0;
  const streak = (() => {
    if (trades.length === 0) return "0";
    const lastWasWin = trades[0].netPnl >= 0;
    let count = 0;
    for (const trade of trades) {
      if ((trade.netPnl >= 0) !== lastWasWin) break;
      count++;
    }
    return `${count}${lastWasWin ? "W" : "L"}`;
  })();
  const strategyNumbers = strategies.reduce<StrategyNumberMap>((map, strategy, index) => {
    map[strategy.name] = strategy.id > 0 ? strategy.id : index + 1;
    return map;
  }, {});
  const currentRegime = stats.regime ?? quote.regime ?? "UNKNOWN";

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 2xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)] items-start gap-5">
        <div className="glass-panel relative overflow-hidden px-6 py-7 md:px-7">
          <div className="absolute -right-12 -top-12 h-40 w-40 rounded-full bg-amber-500/10 blur-3xl pointer-events-none" />

          <div className="flex flex-col gap-5">
            <div className="px-1">
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                FOREX GOLD TRADING EQUITY
              </div>
              <div className="mt-4 flex flex-wrap items-end gap-4">
                <div className={`text-[clamp(2.55rem,5vw,3.35rem)] font-semibold leading-none tracking-tight ${equity >= INITIAL_BALANCE ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtUSD(equity)}
                </div>
                <div className={`pb-1 text-xl font-semibold leading-none ${sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                  {fmtPct(totalReturnPct, true)}
                </div>
              </div>
              <div className="mt-2 px-0.5 text-sm" style={{ color: "var(--text-secondary)" }}>
                Session PnL {fmtUSD(sessionPnl, { signed: true })} · Auto-triggered gold desk · {quote.proxySymbol} live proxy
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 px-1">
              <div className="flex flex-wrap gap-2">
                <BadgePill label={stats.live ? "Gold engine online" : "Feed pending"} tone={stats.live ? "positive" : "warning"} />
                <BadgePill label={`${strategies.length} strategies`} tone="info" />
                <BadgePill label="1% risk / trade" tone="neutral" />
                <BadgePill label={`Regime: ${currentRegime}`} tone={currentRegime === "TRENDING_BULL" ? "positive" : currentRegime === "TRENDING_BEAR" ? "negative" : currentRegime === "VOLATILE" ? "warning" : "info"} />
                <BadgePill label={quote.proxySymbol} tone="neutral" />
              </div>
              <button
                type="button"
                onClick={handleReset}
                disabled={!actionsEnabled}
                title={actionButtonTitle}
                className="btn-danger text-sm"
              >
                Reset Gold Account
              </button>
            </div>
          </div>

          <div className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <CompactMetric label="Session Runtime" value={sessionRuntime} detail={`${stats.activeStrategies} strategies warmed`} accent="text-zinc-900" />
            <CompactMetric
              label="Live Gold Quote"
              value={fmtPrice(quote.ltp)}
              detail={quote.ltp > 0 ? `${fmtPct(quote.changePct, true)} on the day` : "Waiting for price feed"}
              accent={quote.changePct >= 0 ? "text-emerald-600" : "text-rose-600"}
            />
            <CompactMetric
              label="Open Exposure"
              value={`${openCount} active`}
              detail={`${positions.filter((position) => position.side === "LONG").length} long / ${positions.filter((position) => position.side === "SHORT").length} short`}
              accent="text-zinc-900"
            />
          </div>

          {trades.length >= 2 && (() => {
            const points: { x: number; y: number }[] = [];
            let running = INITIAL_BALANCE;
            for (const trade of [...trades].reverse()) {
              running += trade.netPnl;
              points.push({ x: 0, y: running });
            }
            const values = points.map((point) => point.y);
            const minY = Math.min(...values, INITIAL_BALANCE);
            const maxY = Math.max(...values, INITIAL_BALANCE);
            const range = maxY - minY || 1;
            const width = 400;
            const height = 56;
            const px = (index: number) => (index / Math.max(points.length - 1, 1)) * width;
            const py = (value: number) => height - ((value - minY) / range) * height;
            const pathD = points.map((point, index) => `${index === 0 ? "M" : "L"} ${px(index).toFixed(1)} ${py(point.y).toFixed(1)}`).join(" ");
            const color = running >= INITIAL_BALANCE ? "#16a34a" : "#dc2626";
            return (
              <div className="mt-4 px-1">
                <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  Gold equity curve · {trades.length} trades
                </div>
                <svg viewBox={`0 0 ${width} ${height}`} className="w-full" style={{ height: 56, display: "block" }}>
                  <path d={pathD} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" />
                  <line x1="0" y1={py(INITIAL_BALANCE).toFixed(1)} x2={width} y2={py(INITIAL_BALANCE).toFixed(1)} stroke="#94a3b8" strokeWidth="1" strokeDasharray="4 3" />
                </svg>
              </div>
            );
          })()}
        </div>

        <div className="glass-panel px-5 py-6 md:px-6">
          <div className="px-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-zinc-500">Equity And PnL</div>
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <CompactMetric label="Gold Equity" value={fmtUSD(equity)} detail={`Base ${fmtUSD(INITIAL_BALANCE)}`} accent="text-zinc-900" />
            <CompactMetric label="Session PnL" value={fmtUSD(sessionPnl, { signed: true })} detail={`${fmtPct(totalReturnPct, true)} vs base`} accent={sessionPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
            <CompactMetric label="Realized PnL" value={fmtUSD(realizedPnl, { signed: true })} detail={`${totalTrades} completed gold trades`} accent={realizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
            <CompactMetric label="Cash Balance" value={fmtUSD(stats.balance)} detail={`Unrealized ${fmtUSD(unrealizedPnl, { signed: true })}`} accent="text-blue-600" />
          </div>

          <div className="mt-5 rounded-[20px] border p-4" style={{ borderColor: "var(--border)", background: "var(--surface-2)" }}>
            <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500">Gold Tape</div>
            <div className="mt-3 grid grid-cols-3 gap-3 text-sm">
              <div>
                <div style={{ color: "var(--text-secondary)" }}>Price</div>
                <div className="font-mono font-semibold" style={{ color: "var(--text-primary)" }}>{fmtPrice(quote.ltp)}</div>
              </div>
              <div>
                <div style={{ color: "var(--text-secondary)" }}>Day High</div>
                <div className="font-mono font-semibold text-emerald-600">{fmtPrice(quote.dayHigh)}</div>
              </div>
              <div>
                <div style={{ color: "var(--text-secondary)" }}>Day Low</div>
                <div className="font-mono font-semibold text-rose-600">{fmtPrice(quote.dayLow)}</div>
              </div>
            </div>
            <div className="mt-3 text-xs" style={{ color: "var(--text-secondary)" }}>
              {stats.diagnostics}
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-7 gap-4">
        <SummaryCard label="Win Rate" value={totalTrades > 0 ? `${winRate.toFixed(1)}%` : "-"} accent={winRate >= 50 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Profit Factor" value={profitFactor.toFixed(2)} accent={profitFactor >= 1 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Trades" value={`${totalTrades}`} accent="text-zinc-900" />
        <SummaryCard label="Unrealized" value={fmtUSD(unrealizedPnl, { signed: true })} accent={unrealizedPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Streak" value={streak} accent="text-amber-500" />
        <SummaryCard label="Best Trade" value={bestTrade ? fmtUSD(bestTrade.netPnl, { signed: true }) : "-"} accent={bestTrade && bestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"} />
        <SummaryCard label="Avg Hold" value={avgHoldSecs > 0 ? formatElapsedSeconds(Math.round(avgHoldSecs)) : "-"} accent="text-zinc-900" />
      </div>

      <LivePositionsPanel positions={positions} strategyNumbers={strategyNumbers} />
      <StrategiesPanel strategies={strategies} strategyNumbers={strategyNumbers} />

      {bestStrategy && (
        <div className="glass-panel px-6 py-5 flex flex-wrap items-center gap-6 justify-between">
          <div>
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Top Gold Strategy</div>
            <div className="mt-1 text-lg font-bold" style={{ color: "var(--text-primary)" }}>
              {fmtLabel(bestStrategy.name, bestStrategy.id, strategyNumbers)}
            </div>
            <div className="mt-0.5 text-xs" style={{ color: "var(--text-secondary)" }}>
              {bestStrategy.wins}W / {bestStrategy.losses}L | {bestStrategy.totalTrades > 0 ? fmtPct(bestStrategy.winRate) : "-"} win rate | score {bestStrategy.score.toFixed(1)}
            </div>
          </div>
          <div className="text-right">
            <div className="text-[10px] font-bold uppercase tracking-[0.18em]" style={{ color: "var(--text-muted)" }}>Strategy PnL</div>
            <div className={`mt-1 text-2xl font-bold ${bestStrategy.totalPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
              {fmtUSD(bestStrategy.totalPnl, { signed: true })}
            </div>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <SideBadge side={bestStrategy.side} />
            <RosterBadge rosterState={bestStrategy.rosterState} />
            <StatusBadge status={bestStrategy.status} />
          </div>
        </div>
      )}

      <DailyPnlPanel trades={trades} />
      <TradesPanel trades={trades} strategyNumbers={strategyNumbers} />

      <div className="text-center text-[11px]" style={{ color: "var(--text-muted)" }}>
        Gold trading paper account · {quote.proxySymbol} Yahoo proxy · $1,000,000 starting capital · 1% capital per trade · 100 autonomous strategies
      </div>

      {latestTrade ? (
        <div className="glass-panel px-5 py-4">
          <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500">Last Closed Trade</div>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--text-primary)" }}>
                {fmtLabel(latestTrade.strategyName, latestTrade.strategyId, strategyNumbers)}
              </div>
              <div className="text-xs" style={{ color: "var(--text-secondary)" }}>
                {formatShortDate(latestTrade.exitTime)} {formatShortTime(latestTrade.exitTime)} | {latestTrade.exitReason}
              </div>
            </div>
            <div className={`text-lg font-bold ${latestTrade.netPnl >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
              {fmtUSD(latestTrade.netPnl, { signed: true })}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
