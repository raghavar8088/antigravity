"use client";

import { useMemo, useState } from "react";
import type { MockTrade } from "@/lib/mockTradingEngine";
import { rankStrategies, type StrategyRankRow, type StrategyClassification } from "@/lib/mockStrategyRankingEngine";
import { computePerRegimeMetrics } from "@/lib/mockExtendedMetrics";
import {
  DeskCard,
  DeskSectionHeader,
  DeskChip,
  DeskDataTable,
  type DeskColumn,
  DeskMetricTile,
  DeskSearchField,
} from "@/components/desk/ui";

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtUsd(v: number): string {
  if (!Number.isFinite(v)) return "—";
  const abs = Math.abs(v);
  if (abs >= 1_000_000) return `${v >= 0 ? "+" : ""}$${(v / 1_000_000).toFixed(2)}M`;
  if (abs >= 1_000) return `${v >= 0 ? "+" : ""}$${(v / 1_000).toFixed(1)}K`;
  return `${v >= 0 ? "+" : ""}$${v.toFixed(0)}`;
}

function fmtPct(v: number): string {
  if (!Number.isFinite(v)) return "—";
  return `${(v * 100).toFixed(1)}%`;
}

function fmtNum(v: number | null | undefined, decimals = 2): string {
  if (v == null || !Number.isFinite(v)) return "—";
  return v.toFixed(decimals);
}

// ── Classification chip ───────────────────────────────────────────────────────

function ClassificationChip({ classification }: { classification: StrategyClassification }) {
  const variantMap: Record<StrategyClassification, "success" | "warning" | "danger"> = {
    ACTIVE: "success",
    WATCHLIST: "warning",
    DISABLED: "danger",
  };
  return <DeskChip label={classification} variant={variantMap[classification]} />;
}

// ── Score bar ─────────────────────────────────────────────────────────────────

function ScoreBar({ score }: { score: number }) {
  const color = score >= 60 ? "var(--desk-profit)" : score >= 35 ? "#f59e0b" : "var(--desk-loss)";
  return (
    <div className="flex items-center gap-1.5">
      <div className="flex-1 h-1.5 rounded-full bg-[var(--desk-surface-2)] overflow-hidden">
        <div className="h-full rounded-full" style={{ width: `${score}%`, backgroundColor: color }} />
      </div>
      <span className="text-xs font-medium tabular-nums" style={{ color, minWidth: 24, textAlign: "right" }}>{score}</span>
    </div>
  );
}

// ── Leaderboard columns ───────────────────────────────────────────────────────

const columns: DeskColumn<StrategyRankRow>[] = [
  {
    id: "rank",
    header: "#",
    cell: (r) => <span className="text-xs text-[var(--desk-muted)] tabular-nums">{r.rank}</span>,
    width: "36px",
  },
  {
    id: "classification",
    header: "Status",
    cell: (r) => <ClassificationChip classification={r.classification} />,
    width: "96px",
  },
  {
    id: "strategyName",
    header: "Strategy",
    cell: (r) => (
      <div>
        <p className="text-xs font-medium truncate max-w-[140px]">{r.strategyName}</p>
        {r.strategyFamily && (
          <p className="text-[10px] text-[var(--desk-muted)] truncate">{r.strategyFamily}</p>
        )}
      </div>
    ),
  },
  {
    id: "score",
    header: "Score",
    cell: (r) => <ScoreBar score={r.score} />,
    width: "120px",
  },
  {
    id: "winRate",
    header: "Win Rate",
    cell: (r) => (
      <span
        className={`text-xs tabular-nums font-medium ${
          r.winRate >= 0.5 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"
        }`}
      >
        {fmtPct(r.winRate)}
      </span>
    ),
    width: "72px",
    align: "right",
  },
  {
    id: "netPnl",
    header: "Net PnL",
    cell: (r) => (
      <span
        className={`text-xs tabular-nums font-semibold ${
          r.netPnl >= 0 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"
        }`}
      >
        {fmtUsd(r.netPnl)}
      </span>
    ),
    width: "88px",
    align: "right",
  },
  {
    id: "sharpeRatio",
    header: "Sharpe",
    cell: (r) => <span className="text-xs tabular-nums">{fmtNum(r.sharpeRatio)}</span>,
    width: "60px",
    align: "right",
  },
  {
    id: "maxDrawdownPct",
    header: "Max DD",
    cell: (r) => (
      <span
        className={`text-xs tabular-nums ${
          r.maxDrawdownPct > 0.15 ? "text-[var(--desk-loss)]" : "text-[var(--desk-muted)]"
        }`}
      >
        {fmtPct(r.maxDrawdownPct)}
      </span>
    ),
    width: "68px",
    align: "right",
  },
  {
    id: "totalTrades",
    header: "Trades",
    cell: (r) => (
      <span className="text-xs tabular-nums text-[var(--desk-muted)]">{r.totalTrades}</span>
    ),
    width: "52px",
    align: "right",
  },
  {
    id: "bestRegime",
    header: "Best Regime",
    cell: (r) => (
      <span className="text-[10px] text-[var(--desk-muted)] truncate">{r.bestRegime ?? "—"}</span>
    ),
    width: "96px",
  },
];

// ── Regime performance matrix ─────────────────────────────────────────────────

function RegimePerformanceMatrix({ trades }: { trades: readonly MockTrade[] }) {
  const regimeMetrics = useMemo(() => computePerRegimeMetrics(trades), [trades]);

  if (regimeMetrics.length === 0) {
    return <p className="text-xs text-[var(--desk-muted)] py-2">No regime-tagged trades yet.</p>;
  }

  return (
    <div className="overflow-x-auto mt-2">
      <table className="w-full text-xs border-separate border-spacing-y-0.5">
        <thead>
          <tr className="text-[10px] text-[var(--desk-muted)]">
            <th className="text-left py-1 px-2">Regime</th>
            <th className="text-right px-2">Trades</th>
            <th className="text-right px-2">Win Rate</th>
            <th className="text-right px-2">Expectancy</th>
            <th className="text-right px-2">Net PnL</th>
            <th className="text-right px-2">Sharpe</th>
          </tr>
        </thead>
        <tbody>
          {[...regimeMetrics].sort((a, b) => b.expectancy - a.expectancy).map((r) => (
            <tr key={r.regime} className="hover:bg-[var(--desk-surface-2)] transition-colors">
              <td className="py-1 px-2 rounded-l-md font-medium">{r.regime}</td>
              <td className="text-right px-2 tabular-nums text-[var(--desk-muted)]">{r.totalTrades}</td>
              <td className={`text-right px-2 tabular-nums font-medium ${r.winRate >= 0.5 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}>
                {fmtPct(r.winRate)}
              </td>
              <td className={`text-right px-2 tabular-nums font-medium ${r.expectancy >= 0 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}>
                {fmtUsd(r.expectancy)}
              </td>
              <td className={`text-right px-2 tabular-nums font-semibold ${r.netPnl >= 0 ? "text-[var(--desk-profit)]" : "text-[var(--desk-loss)]"}`}>
                {fmtUsd(r.netPnl)}
              </td>
              <td className="text-right px-2 tabular-nums rounded-r-md">{fmtNum(r.sharpeRatio)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ── Main panel ────────────────────────────────────────────────────────────────

interface MockStrategyLeaderboardPanelProps {
  trades: readonly MockTrade[];
  startingEquityUsd?: number;
}

export function MockStrategyLeaderboardPanel({
  trades,
  startingEquityUsd = 1_000_000,
}: MockStrategyLeaderboardPanelProps) {
  const [search, setSearch] = useState("");
  const [tab, setTab] = useState<"leaderboard" | "regime">("leaderboard");
  const [filter, setFilter] = useState<StrategyClassification | "ALL">("ALL");

  const ranking = useMemo(
    () => rankStrategies({ trades, startingEquityUsd }),
    [trades, startingEquityUsd],
  );

  const visibleRows = useMemo(() => {
    let rows = ranking.rows;
    if (filter !== "ALL") rows = rows.filter((r) => r.classification === filter);
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      rows = rows.filter(
        (r) =>
          r.strategyName.toLowerCase().includes(q) ||
          (r.strategyFamily?.toLowerCase().includes(q) ?? false),
      );
    }
    return rows;
  }, [ranking.rows, filter, search]);

  const tabs = [
    { id: "leaderboard" as const, label: "Leaderboard" },
    { id: "regime" as const, label: "Regime Matrix" },
  ];

  return (
    <div className="flex flex-col gap-4">
      {/* Summary KPIs */}
      <div className="grid grid-cols-3 gap-3">
        <DeskMetricTile
          label="Active Strategies"
          value={String(ranking.activeCount)}
          sub={`${ranking.watchlistCount} on watchlist`}
          subColor="neutral"
        />
        <DeskMetricTile
          label="Disabled"
          value={String(ranking.disabledCount)}
          sub="Failed ranking or WF"
          subColor={ranking.disabledCount > 0 ? "loss" : "neutral"}
        />
        <DeskMetricTile
          label="Total Ranked"
          value={String(ranking.totalStrategies)}
          sub="With ≥ 1 closed trade"
          subColor="neutral"
        />
      </div>

      {/* Tab bar */}
      <div className="flex gap-2 border-b border-[var(--desk-border)] pb-2">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`text-xs px-3 py-1 rounded-full transition-colors ${
              tab === t.id
                ? "bg-[var(--desk-accent)] text-white"
                : "text-[var(--desk-muted)] hover:text-[var(--desk-foreground)]"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "leaderboard" && (
        <DeskCard>
          <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-3">
            <DeskSectionHeader title="Strategy Rankings" />
            <div className="flex items-center gap-2 ml-auto">
              {/* Classification filter */}
              {(["ALL", "ACTIVE", "WATCHLIST", "DISABLED"] as const).map((f) => (
                <button
                  key={f}
                  onClick={() => setFilter(f)}
                  className={`text-[10px] px-2 py-0.5 rounded-full border transition-colors ${
                    filter === f
                      ? "bg-[var(--desk-accent)] text-white border-transparent"
                      : "border-[var(--desk-border)] text-[var(--desk-muted)]"
                  }`}
                >
                  {f}
                </button>
              ))}
            </div>
          </div>
          <DeskSearchField
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search strategy or family…"
          />
          <div className="mt-3">
            <DeskDataTable
              columns={columns}
              rows={visibleRows}
              getRowKey={(r) => String(r.strategyId)}
              empty={
                <p className="text-xs text-[var(--desk-muted)] py-6 text-center">
                  {ranking.totalStrategies === 0
                    ? "No closed trades yet — run the mock engine to generate strategy data."
                    : "No strategies match the current filter."}
                </p>
              }
            />
          </div>
          {visibleRows.length > 0 && (
            <p className="text-[10px] text-[var(--desk-muted)] mt-2">
              Ranked at {new Date(ranking.rankedAt).toLocaleTimeString()} · Composite score: 30% profitability, 20% consistency, 20% drawdown, 15% sample size, 15% risk-adjusted.
            </p>
          )}
        </DeskCard>
      )}

      {tab === "regime" && (
        <DeskCard>
          <DeskSectionHeader title="Regime Performance Matrix" />
          <RegimePerformanceMatrix trades={trades} />
        </DeskCard>
      )}
    </div>
  );
}
