"use client";

import { useMemo, useState } from "react";
import {
  DeskButton,
  DeskChip,
  DeskEmptyState,
} from "@/components/desk/ui";
import {
  computeEdgeCandidates,
  computeNarrowRoster,
  EDGE_STATUS_COLOR,
  EDGE_STATUS_LABEL,
  type EdgeCandidateRow,
  type EdgeCandidateStatus,
} from "@/lib/futuresEdgeCandidates";
import type { StrategyDiagnosticRow } from "@/lib/futuresStrategyDiagnostics";
import type { RotationReport } from "@/lib/futuresStrategyRotation";

type StatusFilter = EdgeCandidateStatus | "ALL";

const STATUS_FILTERS: { key: StatusFilter; label: string }[] = [
  { key: "ALL", label: "All" },
  { key: "PROMOTE_SUGGESTED", label: "Promote" },
  { key: "KEEP", label: "Keep" },
  { key: "WATCH", label: "Watch" },
  { key: "DISABLE_SUGGESTED", label: "Disable?" },
  { key: "INSUFFICIENT_DATA", label: "Pending" },
];

function ConfidenceBar({ value }: { value: number }) {
  const color = value >= 70 ? "#3fb950" : value >= 45 ? "#d29922" : "#8b949e";
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
      <div
        style={{
          width: 48,
          height: 5,
          borderRadius: 3,
          background: "#21262d",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            width: `${value}%`,
            height: "100%",
            background: color,
            borderRadius: 3,
          }}
        />
      </div>
      <span style={{ fontSize: 9, color, fontFamily: "var(--desk-font-mono, monospace)" }}>
        {value}
      </span>
    </div>
  );
}

function StatusChip({ status }: { status: EdgeCandidateStatus }) {
  const color = EDGE_STATUS_COLOR[status];
  return (
    <span
      style={{
        fontSize: 9,
        fontWeight: 700,
        color,
        border: `1px solid ${color}`,
        borderRadius: 4,
        padding: "1px 5px",
        letterSpacing: "0.04em",
        whiteSpace: "nowrap",
        fontFamily: "var(--desk-font-mono, monospace)",
      }}
    >
      {EDGE_STATUS_LABEL[status]}
    </span>
  );
}

function fmtPct(n: number) {
  return `${(n * 100).toFixed(0)}%`;
}

function fmtUsd(n: number) {
  return `${n >= 0 ? "+" : ""}$${n.toFixed(2)}`;
}

type EdgeCandidatesPanelProps = {
  diagnosticRows: StrategyDiagnosticRow[] | null | undefined;
  rotationReport: RotationReport | null | undefined;
  currentEnabledIds?: readonly number[];
};

export function EdgeCandidatesPanel({
  diagnosticRows,
  rotationReport,
  currentEnabledIds = [],
}: EdgeCandidatesPanelProps) {
  const [filter, setFilter] = useState<StatusFilter>("ALL");
  const [copied, setCopied] = useState(false);

  const candidates = useMemo(() => {
    if (!diagnosticRows?.length) return [];
    return computeEdgeCandidates(diagnosticRows, rotationReport ?? null);
  }, [diagnosticRows, rotationReport]);

  const roster = useMemo(
    () => computeNarrowRoster(candidates, currentEnabledIds),
    [candidates, currentEnabledIds],
  );

  const visible = useMemo(() => {
    if (filter === "ALL") return candidates;
    return candidates.filter((c) => c.status === filter);
  }, [candidates, filter]);

  const copyEnv = () => {
    void navigator.clipboard.writeText(roster.envLine).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  if (!diagnosticRows?.length) {
    return (
      <DeskEmptyState
        icon="📊"
        title="No diagnostic data yet"
        subtitle="Close at least 5 production trades to see edge candidates."
      />
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {/* Summary header */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 6, alignItems: "center" }}>
        <span style={{ fontSize: 10, color: "#8b949e" }}>{roster.summary}</span>
        {roster.keepIds.length > 0 || roster.watchIds.length > 0 ? (
          <DeskButton
            variant="outlined"
            style={{ minHeight: 26, fontSize: "0.68rem", padding: "0 8px" }}
            onClick={copyEnv}
          >
            {copied ? "Copied!" : "Copy roster env"}
          </DeskButton>
        ) : null}
      </div>

      {/* Roster env line preview */}
      <pre
        style={{
          fontSize: 9,
          fontFamily: "var(--desk-font-mono, monospace)",
          background: "#0d1117",
          padding: "5px 8px",
          borderRadius: 4,
          color: "#8b949e",
          overflow: "auto",
          maxWidth: "100%",
          margin: 0,
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
        }}
      >
        {roster.envLine}
      </pre>

      {/* Status filter chips */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
        {STATUS_FILTERS.map(({ key, label }) => {
          const count =
            key === "ALL"
              ? candidates.length
              : candidates.filter((c) => c.status === key).length;
          if (count === 0 && key !== "ALL") return null;
          return (
            <button
              key={key}
              type="button"
              onClick={() => setFilter(key)}
              style={{
                fontSize: 9,
                fontWeight: filter === key ? 700 : 400,
                color: filter === key ? "#e6edf3" : "#8b949e",
                border: `1px solid ${filter === key ? "#58a6ff" : "#30363d"}`,
                background: filter === key ? "#1c2333" : "transparent",
                borderRadius: 10,
                padding: "2px 8px",
                cursor: "pointer",
              }}
            >
              {label} ({count})
            </button>
          );
        })}
      </div>

      {/* Candidates table */}
      {visible.length === 0 ? (
        <p style={{ fontSize: 10, color: "#8b949e" }}>No candidates match this filter.</p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table
            style={{
              width: "100%",
              borderCollapse: "collapse",
              fontSize: 10,
              minWidth: 520,
            }}
          >
            <thead>
              <tr style={{ color: "#8b949e", textAlign: "left", borderBottom: "1px solid #21262d" }}>
                <th style={{ padding: "4px 6px", fontWeight: 500 }}>ID</th>
                <th style={{ padding: "4px 6px", fontWeight: 500 }}>Status</th>
                <th style={{ padding: "4px 6px", fontWeight: 500 }}>Conf.</th>
                <th style={{ padding: "4px 6px", fontWeight: 500, textAlign: "right" }}>Trades</th>
                <th style={{ padding: "4px 6px", fontWeight: 500, textAlign: "right" }}>E/trade</th>
                <th style={{ padding: "4px 6px", fontWeight: 500, textAlign: "right" }}>WR</th>
                <th style={{ padding: "4px 6px", fontWeight: 500, textAlign: "right" }}>Fee/Gr</th>
                <th style={{ padding: "4px 6px", fontWeight: 500 }}>Rotation</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((row) => (
                <CandidateRow key={row.strategyId} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Disable candidates callout */}
      {roster.disableIds.length > 0 && (
        <div
          style={{
            border: "1px solid #f85149",
            borderRadius: 6,
            padding: "6px 10px",
            background: "#160d0d",
            fontSize: 10,
          }}
        >
          <span style={{ color: "#f85149", fontWeight: 600 }}>Disable suggested: </span>
          <span style={{ color: "#c9d1d9" }}>
            {roster.disableIds.map((id) => {
              const c = candidates.find((x) => x.strategyId === id);
              return `#${id}${c ? ` (${c.reason.split("·")[0].trim()})` : ""}`;
            }).join(" · ")}
          </span>
        </div>
      )}
    </div>
  );
}

function CandidateRow({ row }: { row: EdgeCandidateRow }) {
  const [expanded, setExpanded] = useState(false);
  const feeColor =
    row.feePctOfAbsGross >= 1.0
      ? "#f85149"
      : row.feePctOfAbsGross >= 0.5
        ? "#d29922"
        : "#3fb950";
  const expColor = row.expectancy >= 2 ? "#3fb950" : row.expectancy >= 0 ? "#58a6ff" : "#f85149";

  return (
    <>
      <tr
        style={{
          borderTop: "1px solid #21262d",
          cursor: "pointer",
          background: expanded ? "#161b22" : "transparent",
        }}
        onClick={() => setExpanded((v) => !v)}
      >
        <td style={{ padding: "5px 6px", color: "#8b949e", whiteSpace: "nowrap" }}>
          #{row.strategyId}
        </td>
        <td style={{ padding: "5px 6px" }}>
          <StatusChip status={row.status} />
        </td>
        <td style={{ padding: "5px 6px" }}>
          <ConfidenceBar value={row.confidence} />
        </td>
        <td
          style={{
            padding: "5px 6px",
            textAlign: "right",
            color: "#c9d1d9",
            fontFamily: "var(--desk-font-mono, monospace)",
          }}
        >
          {row.trades}
        </td>
        <td
          style={{
            padding: "5px 6px",
            textAlign: "right",
            color: expColor,
            fontFamily: "var(--desk-font-mono, monospace)",
          }}
        >
          {fmtUsd(row.expectancy)}
        </td>
        <td
          style={{
            padding: "5px 6px",
            textAlign: "right",
            color: "#c9d1d9",
            fontFamily: "var(--desk-font-mono, monospace)",
          }}
        >
          {fmtPct(row.winRate)}
        </td>
        <td
          style={{
            padding: "5px 6px",
            textAlign: "right",
            color: feeColor,
            fontFamily: "var(--desk-font-mono, monospace)",
          }}
        >
          {fmtPct(row.feePctOfAbsGross)}
        </td>
        <td style={{ padding: "5px 6px" }}>
          {row.rotationStatus ? (
            <DeskChip
              tone={
                row.rotationStatus === "PROMOTED"
                  ? "success"
                  : row.rotationStatus === "ACTIVE"
                    ? "primary"
                    : row.rotationStatus === "SUSPENDED"
                      ? "error"
                      : "warning"
              }
            >
              {row.rotationStatus}{row.rotationScore !== null ? ` ${row.rotationScore.toFixed(0)}` : ""}
            </DeskChip>
          ) : (
            <span style={{ color: "#8b949e", fontSize: 9 }}>—</span>
          )}
        </td>
      </tr>
      {expanded && (
        <tr style={{ background: "#161b22" }}>
          <td
            colSpan={8}
            style={{
              padding: "4px 10px 8px",
              fontSize: 9,
              color: "#8b949e",
              fontStyle: "italic",
              borderBottom: "1px solid #21262d",
            }}
          >
            {row.reason}
            {row.lastSeenAt ? (
              <span style={{ marginLeft: 8 }}>
                · Last trade: {new Date(row.lastSeenAt).toLocaleDateString()}
              </span>
            ) : null}
          </td>
        </tr>
      )}
    </>
  );
}
