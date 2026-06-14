"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { FUTURES_STRAT_DEFS, type FuturesStratDef } from "@/lib/trading/futuresStrategies";
import { Metric, TerminalCard } from "./TerminalCard";
import { pnlClass, usd } from "./format";

type StrategyAggregate = {
  strategyId: number;
  strategyName: string;
  total: number;
  open: number;
  closed: number;
  wins: number;
  losses: number;
  winRate: number;
  totalPnl: number;
  realizedPnl: number;
  unrealizedPnl: number;
  exposure: number;
};

type StrategyAnalytics = {
  totalTrades: number;
  openTrades: number;
  closedTrades: number;
  winRate: number;
  totalPnl: number;
  realizedPnl: number;
  unrealizedPnl: number;
  profitFactor: number | null;
  perStrategy: StrategyAggregate[];
};

type StrategyRow = StrategyAggregate & {
  category: string;
  signalKey: string;
  templateFamily: string;
  tpPct: number;
  slPct: number;
  holdMinutes: number;
  hasAnalytics: boolean;
};

type AnalyticsResponse =
  | { ok: true; analytics: StrategyAnalytics; source?: string; storage?: string }
  | { ok: false; code?: string; error?: string; detail?: string };

const TRADE_ENGINE_STRATEGY_DEFS = [...FUTURES_STRAT_DEFS].sort((a, b) => a.id - b.id);

function emptyStrategyAggregate(strategy: FuturesStratDef): StrategyAggregate {
  return {
    strategyId: strategy.id,
    strategyName: strategy.name,
    total: 0,
    open: 0,
    closed: 0,
    wins: 0,
    losses: 0,
    winRate: 0,
    totalPnl: 0,
    realizedPnl: 0,
    unrealizedPnl: 0,
    exposure: 0,
  };
}

function strategyRowFromDef(strategy: FuturesStratDef, analytics?: StrategyAggregate): StrategyRow {
  return {
    ...emptyStrategyAggregate(strategy),
    ...analytics,
    category: strategy.category,
    signalKey: strategy.signalKey,
    templateFamily: strategy.templateFamily ?? strategy.signalKey,
    tpPct: strategy.tpPct,
    slPct: strategy.slPct,
    holdMinutes: strategy.holdMinutes,
    hasAnalytics: analytics != null,
  };
}

export function buildTradeEngineStrategyRows(analyticsRows: readonly StrategyAggregate[] = []): StrategyRow[] {
  const analyticsById = new Map(analyticsRows.map((strategy) => [strategy.strategyId, strategy]));
  const rosterRows = TRADE_ENGINE_STRATEGY_DEFS.map((strategy) => {
    const analytics = analyticsById.get(strategy.id);
    analyticsById.delete(strategy.id);
    return strategyRowFromDef(strategy, analytics);
  });

  const externalRows: StrategyRow[] = [...analyticsById.values()].map((strategy) => ({
    ...strategy,
    category: "Trade Engine",
    signalKey: `STRATEGY_${strategy.strategyId}`,
    templateFamily: "Persisted Analytics",
    tpPct: 0,
    slPct: 0,
    holdMinutes: 0,
    hasAnalytics: true,
  }));

  return [...rosterRows, ...externalRows].sort((a, b) => {
    if (b.totalPnl !== a.totalPnl) return b.totalPnl - a.totalPnl;
    if (b.total !== a.total) return b.total - a.total;
    return a.strategyId - b.strategyId;
  });
}

function rate(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}

function int(value: number) {
  return value.toLocaleString("en-US");
}

function pnlTone(value: number): "positive" | "negative" | "neutral" {
  if (value > 0) return "positive";
  if (value < 0) return "negative";
  return "neutral";
}

function pf(value: number | null) {
  return value == null ? "-" : value.toFixed(2);
}

export function StrategiesCenter() {
  const [analytics, setAnalytics] = useState<StrategyAnalytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshedAt, setRefreshedAt] = useState<string | null>(null);
  const [query, setQuery] = useState("");

  const loadAnalytics = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/mock-trading/analytics", { cache: "no-store" });
      const data = (await res.json()) as AnalyticsResponse;
      if (!res.ok || !data.ok) {
        throw new Error(!data.ok ? data.error ?? data.code ?? "Unable to load strategy analytics" : "Unable to load strategy analytics");
      }
      setAnalytics(data.analytics);
      setRefreshedAt(new Date().toLocaleString());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load strategy analytics");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadAnalytics();
  }, [loadAnalytics]);

  const strategies = useMemo(() => buildTradeEngineStrategyRows(analytics?.perStrategy), [analytics]);
  const filteredStrategies = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return strategies;
    return strategies.filter((strategy) =>
      `${strategy.strategyId} ${strategy.strategyName} ${strategy.category} ${strategy.signalKey} ${strategy.templateFamily}`
        .toLowerCase()
        .includes(q),
    );
  }, [query, strategies]);

  const winningStrategies = strategies.filter((strategy) => strategy.totalPnl > 0).length;
  const activeStrategies = strategies.filter((strategy) => strategy.open > 0).length;

  return (
    <div className="m3-page-stack">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-[10px] uppercase tracking-widest text-emerald-500">Trade Engine</div>
          <h1 className="text-xl font-bold text-zinc-100">Strategies</h1>
          <p className="mt-1 max-w-2xl text-xs text-zinc-500">
            Active Trade Engine roster with ledger metrics overlaid when trades exist: win rate, trade count, open
            exposure, realized PnL, unrealized PnL, and total PnL.
          </p>
        </div>
        <div className="flex flex-col items-end gap-1">
          <button
            type="button"
            onClick={loadAnalytics}
            disabled={loading}
            className="rounded border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-zinc-300 transition-colors hover:border-emerald-700 hover:text-emerald-300 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loading ? "Refreshing..." : "Refresh"}
          </button>
          {refreshedAt ? <span className="text-[9px] text-zinc-600">Updated {refreshedAt}</span> : null}
        </div>
      </div>

      {error ? (
        <div className="rounded border border-rose-900/60 bg-rose-950/20 px-4 py-3 text-xs text-rose-300" role="alert">
          <span className="font-bold">Strategy analytics unavailable.</span>{" "}
          <span className="text-rose-200/80">{error}</span>
        </div>
      ) : null}

      <div className="m3-kpi-strip">
        <Metric label="Strategies" value={int(strategies.length)} tone={strategies.length > 0 ? "positive" : "neutral"} />
        <Metric label="Total Trades" value={analytics ? int(analytics.totalTrades) : "-"} />
        <Metric label="Closed Trades" value={analytics ? int(analytics.closedTrades) : "-"} />
        <Metric label="Open Positions" value={analytics ? int(analytics.openTrades) : "-"} tone={analytics?.openTrades ? "warning" : "neutral"} />
        <Metric label="Win Rate" value={analytics ? rate(analytics.winRate) : "-"} tone={analytics && analytics.winRate >= 0.5 ? "positive" : "warning"} />
        <Metric label="Total PnL" value={analytics ? usd(analytics.totalPnl, { signed: true, compact: true }) : "-"} tone={pnlTone(analytics?.totalPnl ?? 0)} />
        <Metric label="Profit Factor" value={analytics ? pf(analytics.profitFactor) : "-"} tone={analytics?.profitFactor != null && analytics.profitFactor >= 1.2 ? "positive" : "neutral"} />
        <Metric label="Winning Strategies" value={analytics ? int(winningStrategies) : "-"} tone={winningStrategies > 0 ? "positive" : "neutral"} />
      </div>

      <TerminalCard
        title="Strategy Roll-up"
        subtitle="Active roster first, ranked by total PnL when ledger analytics are available"
        actions={
          <div className="flex items-center gap-2">
            <span className="text-[9px] uppercase tracking-wider text-zinc-600">{activeStrategies} active</span>
            {loading ? <span className="text-[9px] uppercase tracking-wider text-amber-500">loading metrics</span> : null}
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search strategy"
              className="w-44 rounded border border-zinc-800 bg-zinc-950 px-2 py-1 text-[10px] text-zinc-300 outline-none transition-colors placeholder:text-zinc-700 focus:border-emerald-800"
            />
          </div>
        }
      >
        {filteredStrategies.length === 0 ? (
          <TerminalNoData label={query ? "No matching strategies" : "No Trade Engine strategies available"} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[1200px] text-xs">
              <thead>
                <tr className="border-b border-zinc-800 text-left text-[9px] uppercase tracking-wider text-zinc-500">
                  <th className="px-2 py-2">#</th>
                  <th className="px-2 py-2">Strategy</th>
                  <th className="px-2 py-2">Category</th>
                  <th className="px-2 py-2">Template</th>
                  <th className="px-2 py-2 text-right">TP / SL</th>
                  <th className="px-2 py-2 text-right">Hold</th>
                  <th className="px-2 py-2 text-right">Trades</th>
                  <th className="px-2 py-2 text-right">Open</th>
                  <th className="px-2 py-2 text-right">Closed</th>
                  <th className="px-2 py-2 text-right">W/L</th>
                  <th className="px-2 py-2 text-right">Win Rate</th>
                  <th className="px-2 py-2 text-right">Exposure</th>
                  <th className="px-2 py-2 text-right">Realized PnL</th>
                  <th className="px-2 py-2 text-right">Unrealized PnL</th>
                  <th className="px-2 py-2 text-right">Total PnL</th>
                </tr>
              </thead>
              <tbody>
                {filteredStrategies.map((strategy, index) => (
                  <tr key={strategy.strategyId} className="border-t border-zinc-900 transition-colors hover:bg-zinc-900/40">
                    <td className="px-2 py-2 text-[10px] tabular-nums text-zinc-600">{index + 1}</td>
                    <td className="px-2 py-2">
                      <div className="font-semibold text-zinc-200">#{strategy.strategyId} {strategy.strategyName}</div>
                      <div className="text-[10px] text-zinc-600">
                        {strategy.open > 0
                          ? "In position"
                          : strategy.closed > 0
                            ? "Closed history"
                            : strategy.hasAnalytics
                              ? "No closed trades yet"
                              : "Roster strategy"}
                      </div>
                    </td>
                    <td className="px-2 py-2 text-[10px] text-zinc-500">{strategy.category}</td>
                    <td className="px-2 py-2 text-[10px] text-zinc-500">{strategy.templateFamily}</td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">
                      {strategy.tpPct > 0 || strategy.slPct > 0 ? `${strategy.tpPct.toFixed(2)}% / ${strategy.slPct.toFixed(2)}%` : "-"}
                    </td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">
                      {strategy.holdMinutes > 0 ? `${strategy.holdMinutes}m` : "-"}
                    </td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-300">{int(strategy.total)}</td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-amber-300">{int(strategy.open)}</td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">{int(strategy.closed)}</td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">{int(strategy.wins)}/{int(strategy.losses)}</td>
                    <td className={`px-2 py-2 text-right font-mono tabular-nums ${strategy.closed > 0 && strategy.winRate >= 0.5 ? "text-emerald-400" : "text-zinc-400"}`}>
                      {strategy.closed > 0 ? rate(strategy.winRate) : "-"}
                    </td>
                    <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">{usd(strategy.exposure, { compact: true })}</td>
                    <td className={`px-2 py-2 text-right font-mono tabular-nums ${pnlClass(strategy.realizedPnl)}`}>
                      {usd(strategy.realizedPnl, { signed: true, compact: true })}
                    </td>
                    <td className={`px-2 py-2 text-right font-mono tabular-nums ${pnlClass(strategy.unrealizedPnl)}`}>
                      {usd(strategy.unrealizedPnl, { signed: true, compact: true })}
                    </td>
                    <td className={`px-2 py-2 text-right font-mono font-semibold tabular-nums ${pnlClass(strategy.totalPnl)}`}>
                      {usd(strategy.totalPnl, { signed: true, compact: true })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </TerminalCard>
    </div>
  );
}
