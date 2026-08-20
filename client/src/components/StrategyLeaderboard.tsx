"use client";

import type { StrategyScore } from "@/lib/ai/strategyScoringEngine";
import { SortableTh, useTableSort } from "@/components/desk/ui";

interface StrategyLeaderboardProps {
  scores?: StrategyScore[];
  maxRows?: number;
}

function pnlColor(pnl: number): string {
  if (pnl > 0) return "#34d399";
  if (pnl < 0) return "#f87171";
  return "#71717a";
}

function fmtUsd(n: number): string {
  const sign = n >= 0 ? "+" : "";
  return `${sign}$${Math.abs(n).toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

/**
 * StrategyLeaderboard — compact table of strategy scores ranked by overall score.
 * Used in the MockTradingDashboard as a lazy-loaded panel.
 */
export function StrategyLeaderboard({ scores = [], maxRows = 50 }: StrategyLeaderboardProps) {
  // Accessors read the underlying NUMBER, not the rendered string: win rate is
  // stored as a fraction and shown as a percentage, and profit factor can be
  // Infinity, which has no sensible text form.
  const { sort, toggle, sortRows } = useTableSort<StrategyScore>({
    rank: (r) => r.rank,
    strategy: (r) => r.strategyName,
    score: (r) => r.overallScore,
    wr: (r) => r.metrics.winRate,
    pf: (r) => (Number.isFinite(r.metrics.profitFactor) ? r.metrics.profitFactor : null),
    pnl: (r) => r.metrics.netPnl,
  });
  const rows = sortRows(scores.slice(0, maxRows));

  if (rows.length === 0) {
    return (
      <div
        style={{
          padding: 24,
          textAlign: "center",
          color: "#71717a",
          fontSize: 12,
        }}
      >
        No closed trades yet — strategy scores appear after the engine records history.
      </div>
    );
  }

  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", fontSize: 12, borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ borderBottom: "1px solid #27272a", textAlign: "left" }}>
            {([
              ["#", "rank"],
              ["Strategy", "strategy"],
              ["Score", "score"],
              ["Win %", "wr"],
              ["PF", "pf"],
              ["Net PnL", "pnl"],
            ] as const).map(([h, key]) => (
              <SortableTh
                key={key}
                sortKey={key}
                sort={sort}
                onSort={toggle}
                align={h === "#" || h === "Strategy" ? "left" : "right"}
                style={{
                  padding: "6px 8px",
                  fontSize: 9,
                  textTransform: "uppercase",
                  letterSpacing: "0.06em",
                  color: "#71717a",
                  fontWeight: 600,
                }}
              >
                {h}
              </SortableTh>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((s) => (
            <tr
              key={s.strategyId}
              style={{ borderTop: "1px solid #27272a" }}
            >
              <td style={{ padding: "6px 8px", color: "#52525b", fontVariantNumeric: "tabular-nums" }}>
                {s.rank}
              </td>
              <td
                style={{
                  padding: "6px 8px",
                  fontWeight: 600,
                  color: "#e4e4e7",
                  maxWidth: 180,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  textAlign: "left",
                }}
              >
                {s.strategyName}
              </td>
              <td style={{ padding: "6px 8px", textAlign: "right", color: "#a1a1aa" }}>
                {s.overallScore.toFixed(1)}
              </td>
              <td
                style={{
                  padding: "6px 8px",
                  textAlign: "right",
                  color: s.metrics.winRate >= 0.5 ? "#34d399" : "#f87171",
                }}
              >
                {(s.metrics.winRate * 100).toFixed(1)}%
              </td>
              <td style={{ padding: "6px 8px", textAlign: "right", color: "#34d399" }}>
                {Number.isFinite(s.metrics.profitFactor)
                  ? s.metrics.profitFactor.toFixed(2)
                  : "—"}
              </td>
              <td
                style={{
                  padding: "6px 8px",
                  textAlign: "right",
                  fontWeight: 600,
                  color: pnlColor(s.metrics.netPnl),
                }}
              >
                {fmtUsd(s.metrics.netPnl)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
