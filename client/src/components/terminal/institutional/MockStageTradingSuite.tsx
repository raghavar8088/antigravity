"use client";

import { useMemo } from "react";
import useLiveBTCPrice from "@/hooks/useLiveBTCPrice";
import { useMockTradingEngine } from "@/hooks/useMockTradingEngine";
import {
  computeAnalytics,
  type MockTrade,
} from "@/lib/mockTradingEngine";
import { rankStrategies, type StrategyRankRow } from "@/lib/mockStrategyRankingEngine";
import {
  getMockConfigForPipelineStage,
  mockAccountKeyForStage,
} from "@/lib/strategyAuthority/gradeStageMockConfig";
import type { StrategyStatus } from "@/lib/strategyAuthority/types";
import { Metric, TerminalCard } from "./TerminalCard";

const STAGE_LABEL: Record<StrategyStatus, string> = {
  GRADE_5: "Grade 5",
  GRADE_4: "Grade 4",
  GRADE_3: "Grade 3",
  GRADE_2: "Grade 2",
  GRADE_1: "Grade 1",
  MAIN_ENGINE: "Main Mock Trading Engine",
  RETIRED: "Retired",
};

const TABLE_CAP = 100;

function fmtUsd(value: number) {
  if (!Number.isFinite(value)) return "—";
  const sign = value >= 0 ? "+" : "";
  return `${sign}$${Math.abs(value).toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

function fmtPct(value: number) {
  if (!Number.isFinite(value)) return "—";
  return `${value.toFixed(1)}%`;
}

function fmtPrice(value: number) {
  if (!Number.isFinite(value)) return "—";
  return `$${value.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

function fmtTime(ts: number | null) {
  if (ts == null || !Number.isFinite(ts)) return "—";
  return new Date(ts).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function pnlClass(value: number) {
  if (value > 0) return "text-emerald-400";
  if (value < 0) return "text-rose-400";
  return "text-zinc-400";
}

function sideClass(side: MockTrade["side"]) {
  return side === "BUY" ? "text-emerald-400" : "text-rose-400";
}

function ScoreBar({ score }: { score: number }) {
  const pct = Math.max(0, Math.min(100, score));
  const color = pct >= 60 ? "bg-emerald-500" : pct >= 35 ? "bg-amber-500" : "bg-rose-500";
  return (
    <div className="flex items-center gap-1.5 min-w-[72px]">
      <div className="flex-1 h-1 rounded bg-zinc-800">
        <div className={`h-full rounded ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-[10px] tabular-nums text-zinc-300 w-6 text-right">{Math.round(pct)}</span>
    </div>
  );
}

function TradeTable({
  trades,
  mode,
  emptyLabel,
}: {
  trades: MockTrade[];
  mode: "open" | "closed";
  emptyLabel: string;
}) {
  if (trades.length === 0) {
    return <div className="py-8 text-center text-xs text-zinc-600">{emptyLabel}</div>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
            <th className="py-2 px-2">Strategy</th>
            <th className="py-2 px-2">Side</th>
            <th className="py-2 px-2 text-right">Score</th>
            <th className="py-2 px-2 text-right">Entry</th>
            {mode === "open" ? (
              <>
                <th className="py-2 px-2 text-right">Mark</th>
                <th className="py-2 px-2 text-right">uPnL</th>
              </>
            ) : (
              <>
                <th className="py-2 px-2 text-right">Exit</th>
                <th className="py-2 px-2 text-right">PnL</th>
                <th className="py-2 px-2">Reason</th>
              </>
            )}
            <th className="py-2 px-2 text-right">{mode === "open" ? "Opened" : "Closed"}</th>
          </tr>
        </thead>
        <tbody>
          {trades.map((t) => {
            const pnl = mode === "open" ? t.unrealizedPnl : t.realizedPnl;
            return (
              <tr key={t.id} className="border-t border-zinc-800 hover:bg-zinc-900/40">
                <td className="py-1.5 px-2 max-w-[180px] truncate font-semibold text-zinc-200">
                  {t.strategyName}
                </td>
                <td className={`py-1.5 px-2 font-bold ${sideClass(t.side)}`}>{t.side}</td>
                <td className="py-1.5 px-2 tabular-nums text-right text-sky-400">
                  {fmtPct(t.signalScore)}
                </td>
                <td className="py-1.5 px-2 tabular-nums text-right">{fmtPrice(t.entryPrice)}</td>
                {mode === "open" ? (
                  <>
                    <td className="py-1.5 px-2 tabular-nums text-right">{fmtPrice(t.currentPrice)}</td>
                    <td className={`py-1.5 px-2 tabular-nums text-right font-semibold ${pnlClass(pnl)}`}>
                      {fmtUsd(pnl)}
                    </td>
                  </>
                ) : (
                  <>
                    <td className="py-1.5 px-2 tabular-nums text-right">
                      {t.exitPrice != null ? fmtPrice(t.exitPrice) : "—"}
                    </td>
                    <td className={`py-1.5 px-2 tabular-nums text-right font-semibold ${pnlClass(pnl)}`}>
                      {fmtUsd(pnl)}
                    </td>
                    <td className="py-1.5 px-2 text-[10px] text-zinc-500">{t.exitReason ?? "—"}</td>
                  </>
                )}
                <td className="py-1.5 px-2 tabular-nums text-right text-[10px] text-zinc-500">
                  {fmtTime(mode === "open" ? t.openedAt : t.closedAt)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function LeaderboardTable({ rows }: { rows: StrategyRankRow[] }) {
  if (rows.length === 0) {
    return (
      <div className="py-8 text-center text-xs text-zinc-600">
        No closed trades yet — strategy scores appear after the engine records history.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-left text-[9px] uppercase tracking-wider text-zinc-500 border-b border-zinc-800">
            <th className="py-2 px-2 w-8">#</th>
            <th className="py-2 px-2">Strategy</th>
            <th className="py-2 px-2">Family</th>
            <th className="py-2 px-2 min-w-[100px]">Score</th>
            <th className="py-2 px-2 text-right">Win %</th>
            <th className="py-2 px-2 text-right">Trades</th>
            <th className="py-2 px-2 text-right">PF</th>
            <th className="py-2 px-2 text-right">Net PnL</th>
            <th className="py-2 px-2 text-center">Status</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.strategyId} className="border-t border-zinc-800 hover:bg-zinc-900/40">
              <td className="py-1.5 px-2 tabular-nums text-zinc-600">{r.rank}</td>
              <td className="py-1.5 px-2 font-semibold text-zinc-200 max-w-[160px] truncate">
                {r.strategyName}
              </td>
              <td className="py-1.5 px-2 text-[10px] text-zinc-500 truncate max-w-[100px]">
                {r.strategyFamily ?? "—"}
              </td>
              <td className="py-1.5 px-2"><ScoreBar score={r.score} /></td>
              <td className={`py-1.5 px-2 tabular-nums text-right ${r.winRate >= 0.5 ? "text-emerald-400" : "text-rose-400"}`}>
                {fmtPct(r.winRate * 100)}
              </td>
              <td className="py-1.5 px-2 tabular-nums text-right">{r.totalTrades}</td>
              <td className="py-1.5 px-2 tabular-nums text-right text-emerald-400">
                {r.profitFactor != null ? r.profitFactor.toFixed(2) : "—"}
              </td>
              <td className={`py-1.5 px-2 tabular-nums text-right font-semibold ${pnlClass(r.netPnl)}`}>
                {fmtUsd(r.netPnl)}
              </td>
              <td className="py-1.5 px-2 text-center">
                <span
                  className={`text-[9px] font-bold uppercase ${
                    r.classification === "ACTIVE"
                      ? "text-emerald-400"
                      : r.classification === "WATCHLIST"
                        ? "text-amber-400"
                        : "text-rose-400"
                  }`}
                >
                  {r.classification}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function MockStageTradingSuite({ status }: { status: StrategyStatus }) {
  const label = STAGE_LABEL[status];
  const live = useLiveBTCPrice();
  const engine = useMockTradingEngine({
    price: live.price,
    accountKey: mockAccountKeyForStage(status),
    pipelineStage: status === "RETIRED" ? null : status,
    initialConfig: getMockConfigForPipelineStage(status),
  });

  const sourceTrades = engine.portfolioTrades;

  const openTrades = useMemo(
    () =>
      [...sourceTrades]
        .filter((t) => t.status === "OPEN")
        .sort((a, b) => b.openedAt - a.openedAt)
        .slice(0, TABLE_CAP),
    [sourceTrades],
  );

  const closedTrades = useMemo(
    () =>
      [...sourceTrades]
        .filter((t) => t.status === "CLOSED")
        .sort((a, b) => (b.closedAt ?? 0) - (a.closedAt ?? 0))
        .slice(0, TABLE_CAP),
    [sourceTrades],
  );

  const leaderboard = useMemo(
    () => rankStrategies({ trades: sourceTrades }).rows.slice(0, TABLE_CAP),
    [sourceTrades],
  );

  const analytics = useMemo(() => computeAnalytics(sourceTrades), [sourceTrades]);

  const dash = "—";
  const priceReady = Number.isFinite(live.price) && live.price > 0;
  const maxOpen = engine.config.maxOpenMockTrades;

  return (
    <div className="grid gap-3">
      <TerminalCard
        title={`${label} — Live Engine`}
        subtitle={`Account ${mockAccountKeyForStage(status)} · mock_trades authority`}
      >
        <div className="m3-kpi-strip">
          <Metric
            label="BTC Mark"
            value={priceReady ? fmtPrice(live.price) : dash}
            tone={priceReady ? "positive" : "warning"}
          />
          <Metric
            label="Open Positions"
            value={String(engine.account.openCount)}
            tone={engine.account.openCount > 0 ? "positive" : "neutral"}
          />
          <Metric label="Closed Trades" value={String(analytics.closedTrades)} />
          <Metric label="Win Rate" value={fmtPct(analytics.winRate * 100)} />
          <Metric label="Max Open Cap" value={String(maxOpen)} />
          <Metric label="Equity" value={fmtUsd(engine.account.equity)} />
          <Metric
            label="Realized PnL"
            value={fmtUsd(analytics.realizedPnl)}
            tone={analytics.realizedPnl >= 0 ? "positive" : "negative"}
          />
          <Metric
            label="Persistence"
            value={engine.persistence.status === "mongo" ? "MongoDB" : engine.persistence.status}
            tone={engine.persistence.status === "mongo" ? "positive" : "warning"}
          />
        </div>

        <div className="mt-3 grid gap-1 text-[10px] text-zinc-500">
          {!priceReady ? (
            <p className="text-amber-400">Waiting for live BTC price before trade creation can start.</p>
          ) : null}
          {engine.error ? <p className="text-rose-400">Signal tick: {engine.error}</p> : null}
          {engine.diagnostics.recentRejections[0] ? (
            <p className="text-zinc-400">
              Latest rejection: {engine.diagnostics.recentRejections[0].message}
            </p>
          ) : null}
        </div>
      </TerminalCard>

      <TerminalCard
        title="Open Positions"
        subtitle={`${openTrades.length} open · signal score per trade`}
      >
        <TradeTable
          trades={openTrades}
          mode="open"
          emptyLabel="No open positions — engine is waiting for executable candidates."
        />
      </TerminalCard>

      <TerminalCard
        title="Closed Trades"
        subtitle={`${closedTrades.length} recent closes · newest first`}
      >
        <TradeTable
          trades={closedTrades}
          mode="closed"
          emptyLabel="No closed trades yet for this stage account."
        />
      </TerminalCard>

      <TerminalCard
        title="Strategy Leadership Board"
        subtitle="Composite score · win rate · profit factor · net PnL"
      >
        <LeaderboardTable rows={leaderboard} />
      </TerminalCard>
    </div>
  );
}
