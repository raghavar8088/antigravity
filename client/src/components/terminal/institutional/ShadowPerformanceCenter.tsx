"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { TerminalNoData } from "@/components/terminal/TerminalAuthorityGuard";
import { Metric, TerminalCard } from "./TerminalCard";
import { pnlClass, usd } from "./format";
import { AutoSortTable } from "@/components/desk/ui";
import { SortableTh, useTableSort } from "@/components/desk/ui";

// Mirrors engine/internal/shadow/ledger.go ShadowPerformance.
type ShadowPerformance = {
  strategy_name: string;
  total_trades: number;
  win_count: number;
  loss_count: number;
  win_rate: number;
  total_net_pnl: number;
  avg_win_pct: number;
  avg_loss_pct: number;
  sharpe_ratio: number;
  max_drawdown: number;
  avg_hold_minutes: number;
  best_trade: number;
  worst_trade: number;
  last_trade_at?: string;
};

// Mirrors engine/internal/shadow/ledger.go ShadowTrade.
type ShadowTrade = {
  id: string;
  strategy_name: string;
  direction: "LONG" | "SHORT";
  entry_price: number;
  size: number;
  stop_loss: number;
  take_profit: number;
  opened_at: string;
  closed_at?: string;
  exit_price?: number;
  exit_reason?: string;
  net_pnl?: number;
  status: "OPEN" | "CLOSED";
};

// Mirrors engine/internal/trading/scalers_eval.go StrategyPerformance.
type LiveStrategyPerformance = {
  name: string;
  status: string;
  total_trades: number;
  win_rate: number;
  sharpe: number;
  avg_rr: number;
  total_pnl: number;
  last_trade_at?: string;
};

type LeaderboardRow = {
  name: string;
  mode: "LIVE" | "SHADOW";
  trades: number;
  winRate: number;
  sharpe: number;
  pnl: number;
  eligible: boolean;
  walkForwardActive: boolean;
};

const PROMOTION_MIN_TRADES = 30;
const PROMOTION_MIN_WIN_RATE = 0.48;
const PROMOTION_MIN_SHARPE = 0.6;

function rate(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}

function int(value: number) {
  return value.toLocaleString("en-US");
}

async function fetchJson<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { cache: "no-store", ...init });
  const data = (await res.json()) as T & { ok?: boolean; error?: string };
  if (!res.ok || data.ok === false) {
    throw new Error((data as { error?: string }).error ?? `Request failed: ${path}`);
  }
  return data;
}

export function ShadowPerformanceCenter() {
  const [shadowPerf, setShadowPerf] = useState<ShadowPerformance[]>([]);
  const [livePerf, setLivePerf] = useState<LiveStrategyPerformance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [expandedTrades, setExpandedTrades] = useState<ShadowTrade[]>([]);
  const [actionPending, setActionPending] = useState<string | null>(null);
  const [confirmModal, setConfirmModal] = useState<LeaderboardRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [shadow, live] = await Promise.all([
        fetchJson<ShadowPerformance[]>("/api/engine/api/shadow/performance"),
        fetchJson<LiveStrategyPerformance[]>("/api/engine/api/strategies/performance"),
      ]);
      setShadowPerf(Array.isArray(shadow) ? shadow : []);
      setLivePerf(Array.isArray(live) ? live : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load shadow performance");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const interval = setInterval(load, 30_000);
    return () => clearInterval(interval);
  }, [load]);

  // A strategy is "live" if it has a live performance record reporting actual
  // trades or an ACTIVE/PROBATIONARY/DEMOTED walk-forward status; everything
  // else with shadow trade history is shadow-only. A name can legitimately
  // appear in both feeds for a short window around promotion — live wins.
  const liveNames = useMemo(() => new Set(livePerf.map((p) => p.name)), [livePerf]);

  // Numbers straight off the row: win rate is stored as a fraction and rendered
  // as a percentage, so sorting the rendered text would compare "48.0%" as text.
  const { sort, toggle, sortRows } = useTableSort<LeaderboardRow>({
    name: (r) => r.name,
    mode: (r) => r.mode,
    trades: (r) => r.trades,
    winRate: (r) => r.winRate,
    sharpe: (r) => r.sharpe,
    pnl: (r) => r.pnl,
  });

  const unsortedRows: LeaderboardRow[] = useMemo(() => {
    const liveRows: LeaderboardRow[] = livePerf.map((p) => ({
      name: p.name,
      mode: "LIVE",
      trades: p.total_trades,
      winRate: p.win_rate,
      sharpe: p.sharpe,
      pnl: p.total_pnl,
      eligible: false,
      walkForwardActive: p.status === "ACTIVE",
    }));
    const shadowRows: LeaderboardRow[] = shadowPerf
      .filter((p) => !liveNames.has(p.strategy_name))
      .map((p) => ({
        name: p.strategy_name,
        mode: "SHADOW",
        trades: p.total_trades,
        winRate: p.win_rate,
        sharpe: p.sharpe_ratio,
        pnl: p.total_net_pnl,
        eligible:
          p.total_trades >= PROMOTION_MIN_TRADES &&
          p.win_rate >= PROMOTION_MIN_WIN_RATE &&
          p.sharpe_ratio >= PROMOTION_MIN_SHARPE,
        walkForwardActive: false,
      }));
    return [...liveRows, ...shadowRows].sort((a, b) => b.winRate - a.winRate || b.pnl - a.pnl);
  }, [livePerf, shadowPerf, liveNames]);

  // The default ordering (win rate, then P&L) still applies until the reader
  // picks a column; sortRows returns the input untouched while nothing is set.
  const rows = sortRows(unsortedRows);

  const liveCount = rows.filter((r) => r.mode === "LIVE").length;
  const shadowCount = rows.filter((r) => r.mode === "SHADOW").length;
  const eligibleRows = rows.filter((r) => r.eligible);

  const toggleExpand = useCallback(async (name: string) => {
    if (expanded === name) {
      setExpanded(null);
      setExpandedTrades([]);
      return;
    }
    setExpanded(name);
    try {
      const detail = await fetchJson<{ closed_trades: ShadowTrade[] }>(
        `/api/engine/api/shadow/performance/${encodeURIComponent(name)}`,
      );
      setExpandedTrades(detail.closed_trades ?? []);
    } catch {
      setExpandedTrades([]);
    }
  }, [expanded]);

  const promote = useCallback(async (name: string) => {
    setActionPending(name);
    try {
      await fetchJson("/api/engine/api/shadow/promote", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ strategyName: name }),
      });
      setConfirmModal(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Promotion failed");
    } finally {
      setActionPending(null);
    }
  }, [load]);

  const demote = useCallback(async (name: string) => {
    setActionPending(name);
    try {
      await fetchJson("/api/engine/api/shadow/demote", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ strategyName: name }),
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Demotion failed");
    } finally {
      setActionPending(null);
    }
  }, [load]);

  return (
    <div className="m3-page-stack">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-[10px] uppercase tracking-widest text-violet-400">Strategy Incubation</div>
          <h1 className="text-xl font-bold text-zinc-100">Shadow Performance Leaderboard</h1>
          <p className="mt-1 max-w-2xl text-xs text-zinc-500">
            All 30 curated strategies ranked together. LIVE strategies trade the real paper account; SHADOW
            strategies fire fully-evaluated signals into a hypothetical ledger — same fees, slippage, and Kelly
            sizing as live — so they build a real track record without risking balance.
          </p>
        </div>
        <button
          type="button"
          onClick={load}
          disabled={loading}
          className="rounded border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-zinc-300 transition-colors hover:border-emerald-700 hover:text-emerald-300 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {loading ? "Refreshing..." : "Refresh"}
        </button>
      </div>

      {error ? (
        <div className="rounded border border-rose-900/60 bg-rose-950/20 px-4 py-3 text-xs text-rose-300" role="alert">
          <span className="font-bold">Shadow performance unavailable.</span> <span className="text-rose-200/80">{error}</span>
        </div>
      ) : null}

      <div className="m3-kpi-strip">
        <Metric label="Total Strategies" value={int(rows.length)} />
        <Metric label="Live" value={int(liveCount)} tone="positive" />
        <Metric label="Shadow" value={int(shadowCount)} tone="neutral" />
        <Metric label="Eligible for Promotion" value={int(eligibleRows.length)} tone={eligibleRows.length > 0 ? "positive" : "neutral"} />
      </div>

      {eligibleRows.map((row) => (
        <TerminalCard
          key={`eligible-${row.name}`}
          title="⭐ Ready for Promotion"
          subtitle={`${row.name} — ${row.trades} shadow trades, ${rate(row.winRate)} win rate, Sharpe ${row.sharpe.toFixed(2)} — exceeds all promotion thresholds`}
          actions={
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => toggleExpand(row.name)}
                className="rounded border border-zinc-700 px-2 py-1 text-[10px] uppercase tracking-wider text-zinc-300 hover:border-zinc-500"
              >
                View Shadow History
              </button>
              <button
                type="button"
                onClick={() => setConfirmModal(row)}
                disabled={actionPending === row.name}
                className="rounded border border-amber-700 bg-amber-950/40 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-amber-300 hover:bg-amber-900/40 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Promote
              </button>
            </div>
          }
        >
          <p className="text-xs text-zinc-400">
            Promoting moves this strategy from shadow to live paper trading. It will start opening real positions
            (subject to live directional caps and the concentration gate) from the next 15m eval cycle.
          </p>
          <p className={`mt-2 text-sm font-semibold ${pnlClass(row.pnl)}`}>
            Shadow PnL (hypothetical): {usd(row.pnl, { signed: true })} across {row.trades} trades
          </p>
        </TerminalCard>
      ))}

      <TerminalCard
        title="Strategy Performance Leaderboard"
        subtitle="All curated strategies ranked by win rate, then hypothetical/realized PnL"
        actions={<span className="text-[9px] uppercase tracking-wider text-zinc-600">{rows.length} strategies · {liveCount} live · {shadowCount} shadow · {eligibleRows.length} eligible</span>}
      >
        {rows.length === 0 ? (
          <TerminalNoData label="No strategy performance data reported by engine yet" />
        ) : (
          <div className="overflow-x-auto">
            {/* Sorted through its DATA rather than by wrapping the table.
                Each entry here is a Fragment holding the strategy row AND its
                conditional expanded detail row, so reordering the rendered rows
                would separate a detail row from the row it belongs to. Ordering
                the array keeps each pair together. */}
            <table className="w-full min-w-[860px] text-xs">
              <thead>
                <tr className="border-b border-zinc-800 text-left text-[9px] uppercase tracking-wider text-zinc-500">
                  <th className="px-2 py-2">Rank</th>
                  {([
                    ["Strategy", "name", "left"],
                    ["Mode", "mode", "left"],
                    ["Trades", "trades", "right"],
                    ["Win Rate", "winRate", "right"],
                    ["Sharpe", "sharpe", "right"],
                    ["PnL", "pnl", "right"],
                  ] as const).map(([label, key, align]) => (
                    <SortableTh
                      key={key}
                      sortKey={key}
                      sort={sort}
                      onSort={toggle}
                      align={align}
                      style={{ padding: "8px" }}
                    >
                      {label}
                    </SortableTh>
                  ))}
                  <th className="px-2 py-2 text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row, index) => (
                  <>
                    <tr
                      key={row.name}
                      className={`border-t transition-colors hover:bg-zinc-900/40 ${
                        row.mode === "SHADOW" ? "border-dotted border-violet-900/60" : "border-zinc-900"
                      }`}
                    >
                      <td className="px-2 py-2 text-[10px] tabular-nums text-zinc-600">{index + 1}</td>
                      <td className="px-2 py-2">
                        <button
                          type="button"
                          onClick={() => toggleExpand(row.name)}
                          className="font-semibold text-zinc-200 hover:text-emerald-300"
                        >
                          {row.eligible ? "⭐ " : ""}
                          {row.name}
                        </button>
                      </td>
                      <td className="px-2 py-2">
                        <span
                          className={`rounded px-2 py-0.5 text-[9px] font-bold uppercase tracking-wider ${
                            row.mode === "LIVE" ? "bg-emerald-950/60 text-emerald-400" : "bg-violet-950/60 text-violet-300"
                          }`}
                        >
                          {row.mode}
                        </span>
                      </td>
                      <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-300">{int(row.trades)}</td>
                      <td className={`px-2 py-2 text-right font-mono tabular-nums ${row.trades > 0 && row.winRate >= 0.5 ? "text-emerald-400" : "text-zinc-400"}`}>
                        {row.trades > 0 ? rate(row.winRate) : "-"}
                      </td>
                      <td className="px-2 py-2 text-right font-mono tabular-nums text-zinc-400">{row.sharpe ? row.sharpe.toFixed(2) : "-"}</td>
                      <td className={`px-2 py-2 text-right font-mono font-semibold tabular-nums ${pnlClass(row.pnl)}`}>
                        {usd(row.pnl, { signed: true, compact: true })}
                        {row.mode === "SHADOW" ? <span className="ml-1 text-[9px] text-zinc-600">*</span> : null}
                      </td>
                      <td className="px-2 py-2 text-right">
                        {row.mode === "SHADOW" && row.eligible ? (
                          <button
                            type="button"
                            onClick={() => setConfirmModal(row)}
                            disabled={actionPending === row.name}
                            className="rounded border border-amber-700 px-2 py-0.5 text-[9px] font-bold uppercase tracking-wider text-amber-300 hover:bg-amber-950/40 disabled:opacity-50"
                          >
                            Promote
                          </button>
                        ) : row.mode === "LIVE" ? (
                          <button
                            type="button"
                            onClick={() => demote(row.name)}
                            disabled={actionPending === row.name}
                            className="rounded border border-zinc-700 px-2 py-0.5 text-[9px] font-bold uppercase tracking-wider text-zinc-400 hover:border-rose-700 hover:text-rose-300 disabled:opacity-50"
                          >
                            Demote
                          </button>
                        ) : null}
                      </td>
                    </tr>
                    {expanded === row.name ? (
                      <tr key={`${row.name}-detail`} className="border-t border-zinc-900 bg-zinc-950/60">
                        <td colSpan={8} className="px-4 py-3">
                          {expandedTrades.length === 0 ? (
                            <div className="text-[10px] text-zinc-600">No closed shadow trades yet.</div>
                          ) : (
                            <AutoSortTable><table className="w-full text-[10px]">
                              <thead>
                                <tr className="text-left uppercase tracking-wider text-zinc-600">
                                  <th className="py-1 pr-3">Direction</th>
                                  <th className="py-1 pr-3">Entry</th>
                                  <th className="py-1 pr-3">Exit</th>
                                  <th className="py-1 pr-3">Reason</th>
                                  <th className="py-1 pr-3 text-right">Net PnL</th>
                                  <th className="py-1 text-right">Closed</th>
                                </tr>
                              </thead>
                              <tbody>
                                {expandedTrades.slice(0, 10).map((t) => (
                                  <tr key={t.id} className="border-t border-zinc-900">
                                    <td className="py-1 pr-3">
                                      <span className={t.direction === "LONG" ? "text-emerald-400" : "text-rose-400"}>{t.direction}</span>
                                    </td>
                                    <td className="py-1 pr-3 font-mono text-zinc-400">{t.entry_price.toFixed(2)}</td>
                                    <td className="py-1 pr-3 font-mono text-zinc-400">{t.exit_price ? t.exit_price.toFixed(2) : "-"}</td>
                                    <td className="py-1 pr-3 text-zinc-500">{t.exit_reason ?? "-"}</td>
                                    <td className={`py-1 pr-3 text-right font-mono ${pnlClass(t.net_pnl ?? 0)}`}>
                                      {t.net_pnl !== undefined ? usd(t.net_pnl, { signed: true }) : "-"}
                                    </td>
                                    <td className="py-1 text-right text-zinc-600">{t.closed_at ? new Date(t.closed_at).toLocaleString() : "-"}</td>
                                  </tr>
                                ))}
                              </tbody>
                            </table></AutoSortTable>
                          )}
                        </td>
                      </tr>
                    ) : null}
                  </>
                ))}
              </tbody>
            </table>
            <div className="mt-2 text-[9px] text-zinc-600">* Shadow PnL is hypothetical — what the strategy would have earned if it were live. It never reflects real account balance.</div>
          </div>
        )}
      </TerminalCard>

      {confirmModal ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4" role="dialog" aria-modal="true">
          <div className="w-full max-w-md rounded border border-zinc-700 bg-zinc-950 p-5">
            <h3 className="text-sm font-bold text-zinc-100">Promote {confirmModal.name} to live paper trading?</h3>
            <p className="mt-2 text-xs text-zinc-400">
              This strategy will start opening real positions (up to the live directional caps and concentration
              gates).
            </p>
            <p className="mt-2 text-xs text-zinc-400">
              Shadow performance: {rate(confirmModal.winRate)} win rate, {usd(confirmModal.pnl, { signed: true })} hypothetical PnL across {confirmModal.trades} trades.
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setConfirmModal(null)}
                className="rounded border border-zinc-700 px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-zinc-300 hover:border-zinc-500"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => promote(confirmModal.name)}
                disabled={actionPending === confirmModal.name}
                className="rounded border border-amber-700 bg-amber-950/40 px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-amber-300 hover:bg-amber-900/40 disabled:opacity-50"
              >
                {actionPending === confirmModal.name ? "Promoting..." : "Confirm Promotion"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
