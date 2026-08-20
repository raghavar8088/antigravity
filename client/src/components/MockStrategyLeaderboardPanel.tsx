"use client";

import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import { rankStrategies } from "@/lib/ai/mockStrategyRankingEngine";
import { useMemo } from "react";
import { AutoSortTable } from "@/components/desk/ui";

interface MockStrategyLeaderboardPanelProps {
  trades?: MockTrade[];
  maxRows?: number;
}

function fmtUsd(n: number): string {
  const sign = n >= 0 ? "+" : "";
  return `${sign}$${Math.abs(n).toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

function fmtPct(n: number): string {
  return `${(n * 100).toFixed(1)}%`;
}

const classColor: Record<string, string> = {
  ACTIVE: "#34d399",
  WATCHLIST: "#fbbf24",
  DISABLED: "#f87171",
};

/**
 * MockStrategyLeaderboardPanel — ranks all strategies that have appeared in
 * mock trade history and renders a sortable leaderboard table.
 */
export function MockStrategyLeaderboardPanel({
  trades = [],
  maxRows = 100,
}: MockStrategyLeaderboardPanelProps) {
  const rows = useMemo(
    () => rankStrategies({ trades }).rows.slice(0, maxRows),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [trades.filter((t) => t.status === "CLOSED").length],
  );

  if (rows.length === 0) {
    return (
      <div style={{ padding: 24, textAlign: "center", color: "#71717a", fontSize: 12 }}>
        No closed trades yet — leaderboard appears after the engine records history.
      </div>
    );
  }

  return (
    <div style={{ overflowX: "auto" }}>
      <AutoSortTable><table style={{ width: "100%", fontSize: 12, borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ borderBottom: "1px solid #27272a", textAlign: "left" }}>
            {["#", "Strategy", "Family", "Score", "Win %", "Trades", "PF", "Net PnL", "Status"].map(
              (h) => (
                <th
                  key={h}
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
                </th>
              ),
            )}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.strategyId} style={{ borderTop: "1px solid #27272a" }}>
              <td style={{ padding: "6px 8px", color: "#52525b" }}>{r.rank}</td>
              <td
                style={{
                  padding: "6px 8px",
                  fontWeight: 600,
                  color: "#e4e4e7",
                  maxWidth: 160,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {r.strategyName}
              </td>
              <td style={{ padding: "6px 8px", color: "#71717a", fontSize: 10 }}>
                {r.strategyFamily ?? "—"}
              </td>
              <td style={{ padding: "6px 8px", color: "#a1a1aa" }}>{r.score.toFixed(1)}</td>
              <td
                style={{
                  padding: "6px 8px",
                  color: r.winRate >= 0.5 ? "#34d399" : "#f87171",
                }}
              >
                {fmtPct(r.winRate)}
              </td>
              <td style={{ padding: "6px 8px", color: "#a1a1aa" }}>{r.totalTrades}</td>
              <td style={{ padding: "6px 8px", color: "#34d399" }}>
                {r.profitFactor != null ? r.profitFactor.toFixed(2) : "—"}
              </td>
              <td
                style={{
                  padding: "6px 8px",
                  fontWeight: 600,
                  color: r.netPnl >= 0 ? "#34d399" : "#f87171",
                }}
              >
                {fmtUsd(r.netPnl)}
              </td>
              <td style={{ padding: "6px 8px" }}>
                <span
                  style={{
                    fontSize: 9,
                    fontWeight: 700,
                    textTransform: "uppercase",
                    color: classColor[r.classification] ?? "#71717a",
                  }}
                >
                  {r.classification}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table></AutoSortTable>
    </div>
  );
}
