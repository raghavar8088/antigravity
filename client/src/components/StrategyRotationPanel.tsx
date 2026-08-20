"use client";

import type { RotationReport, RotationScore } from "@/lib/trading/futuresStrategyRotation";
import { AutoSortTable } from "@/components/desk/ui";

interface StrategyRotationPanelProps {
  report: RotationReport | null | undefined;
  onRestore?: (strategyId: number) => void;
}

const STATUS_COLOR: Record<string, string> = {
  PROMOTED: "#34d399",
  ACTIVE: "#60a5fa",
  PROBATION: "#fbbf24",
  SUSPENDED: "#f87171",
  INSUFFICIENT: "#71717a",
};

function fmt(n: number, dec = 2): string {
  if (!Number.isFinite(n)) return "—";
  return n.toFixed(dec);
}

function ScoreRow({
  row,
  onRestore,
  isSuspended,
}: {
  row: RotationScore;
  onRestore?: (id: number) => void;
  isSuspended: boolean;
}) {
  return (
    <tr style={{ borderTop: "1px solid #27272a" }}>
      <td style={{ padding: "5px 8px", fontVariantNumeric: "tabular-nums", color: "#52525b", fontSize: 11 }}>
        {row.rank}
      </td>
      <td
        style={{
          padding: "5px 8px",
          fontWeight: 600,
          color: "#e4e4e7",
          maxWidth: 200,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          fontSize: 11,
        }}
      >
        {row.strategyName}
      </td>
      <td style={{ padding: "5px 8px", fontVariantNumeric: "tabular-nums", color: "#a1a1aa", fontSize: 11 }}>
        {fmt(row.score, 1)}
      </td>
      <td style={{ padding: "5px 8px", fontSize: 11 }}>
        <span
          style={{
            fontWeight: 700,
            fontSize: 9,
            textTransform: "uppercase" as const,
            color: STATUS_COLOR[row.status] ?? "#71717a",
          }}
        >
          {row.status}
        </span>
      </td>
      <td
        style={{
          padding: "5px 8px",
          color: "#71717a",
          fontSize: 10,
          maxWidth: 220,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {row.reasoning}
      </td>
      {onRestore && isSuspended ? (
        <td style={{ padding: "5px 8px" }}>
          <button
            type="button"
            onClick={() => onRestore(row.strategyId)}
            style={{
              padding: "2px 8px",
              fontSize: 10,
              borderRadius: 4,
              border: "1px solid #3f3f46",
              background: "#18181b",
              color: "#a1a1aa",
              cursor: "pointer",
            }}
          >
            Restore
          </button>
        </td>
      ) : (
        <td />
      )}
    </tr>
  );
}

/**
 * StrategyRotationPanel — displays the strategy rotation report with promoted,
 * active, probation, suspended, and insufficient-sample-size rows.
 */
export function StrategyRotationPanel({ report, onRestore }: StrategyRotationPanelProps) {
  if (!report || report.scores.length === 0) {
    return (
      <div
        style={{
          padding: "16px 12px",
          borderRadius: 8,
          border: "1px solid #27272a",
          background: "#18181b",
          color: "#71717a",
          fontSize: 12,
          textAlign: "center",
        }}
      >
        No rotation data yet — scores appear after the desk has enough trade history.
      </div>
    );
  }

  const suspendedIds = new Set([...report.suspended, ...report.insufficient].map((r) => r.strategyId));

  const summary = [
    { label: "Promoted", count: report.promoted.length, color: STATUS_COLOR.PROMOTED },
    { label: "Active", count: report.active.length, color: STATUS_COLOR.ACTIVE },
    { label: "Probation", count: report.probation.length, color: STATUS_COLOR.PROBATION },
    { label: "Suspended", count: report.suspended.length, color: STATUS_COLOR.SUSPENDED },
    { label: "Insufficient", count: report.insufficient.length, color: STATUS_COLOR.INSUFFICIENT },
  ];

  return (
    <div
      style={{
        borderRadius: 8,
        border: "1px solid #27272a",
        background: "#18181b",
        overflow: "hidden",
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: "8px 12px",
          borderBottom: "1px solid #27272a",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <span style={{ fontSize: 11, fontWeight: 700, color: "#e4e4e7", textTransform: "uppercase", letterSpacing: "0.06em" }}>
          Strategy Rotation
        </span>
        <div style={{ display: "flex", gap: 12 }}>
          {summary.map((s) => (
            <span key={s.label} style={{ fontSize: 10, color: "#71717a" }}>
              <span style={{ color: s.color, fontWeight: 700 }}>{s.count}</span>{" "}
              {s.label}
            </span>
          ))}
        </div>
      </div>

      {/* Table */}
      <div style={{ overflowX: "auto" }}>
        <AutoSortTable><table style={{ width: "100%", fontSize: 12, borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ borderBottom: "1px solid #27272a" }}>
              {["#", "Strategy", "Score", "Status", "Reasoning", ""].map((h) => (
                <th
                  key={h}
                  style={{
                    padding: "6px 8px",
                    fontSize: 9,
                    textTransform: "uppercase",
                    letterSpacing: "0.06em",
                    color: "#71717a",
                    fontWeight: 600,
                    textAlign: "left",
                  }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {report.scores.map((row) => (
              <ScoreRow
                key={row.strategyId}
                row={row}
                onRestore={onRestore}
                isSuspended={suspendedIds.has(row.strategyId)}
              />
            ))}
          </tbody>
        </table></AutoSortTable>
      </div>
    </div>
  );
}
