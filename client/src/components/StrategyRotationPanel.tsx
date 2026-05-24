"use client";

/**
 * StrategyRotationPanel
 * Collapsible panel showing strategy rotation scores and status.
 * Additive only — drop into existing Winners desk layout.
 * No new state, no new fetches — consumes monitor output.
 */
import { useState } from "react";
import type { RotationReport, RotationStatus } from "@/lib/futuresStrategyRotation";

interface Props {
  report: RotationReport | null;
  onRestore?: (strategyId: number) => void;
}

const STATUS_COLOR: Record<RotationStatus, string> = {
  PROMOTED: "#3fb950",
  ACTIVE: "#58a6ff",
  PROBATION: "#d29922",
  SUSPENDED: "#f85149",
  INSUFFICIENT: "#8b949e",
};

const STATUS_ICON: Record<RotationStatus, string> = {
  PROMOTED: "🚀",
  ACTIVE: "✅",
  PROBATION: "⚠️",
  SUSPENDED: "🚫",
  INSUFFICIENT: "⏳",
};

const FILTER_OPTIONS: { status: RotationStatus; label: string; color: string }[] = [
  { status: "PROMOTED", label: "🚀 Promoted", color: "#3fb950" },
  { status: "ACTIVE", label: "✅ Active", color: "#58a6ff" },
  { status: "PROBATION", label: "⚠ Probation", color: "#d29922" },
  { status: "SUSPENDED", label: "🚫 Suspended", color: "#f85149" },
  { status: "INSUFFICIENT", label: "⏳ Pending", color: "#8b949e" },
];

export function StrategyRotationPanel({ report, onRestore }: Props) {
  const [expanded, setExpanded] = useState(false);
  const [showAll, setShowAll] = useState(false);
  const [filterStatus, setFilterStatus] = useState<RotationStatus | null>(null);

  if (!report) {
    return (
      <div
        style={{
          fontSize: 11,
          color: "#8b949e",
          padding: "4px 0",
          marginTop: 6,
        }}
      >
        Strategy Rotation: awaiting diagnostics data
      </div>
    );
  }

  const summary = FILTER_OPTIONS.map((opt) => ({
    ...opt,
    count:
      opt.status === "PROMOTED"
        ? report.promoted.length
        : opt.status === "ACTIVE"
          ? report.active.length
          : opt.status === "PROBATION"
            ? report.probation.length
            : opt.status === "SUSPENDED"
              ? report.suspended.length
              : report.insufficient.length,
  }));

  const displayScores = filterStatus
    ? report.scores.filter((s) => s.status === filterStatus)
    : showAll
      ? report.scores
      : report.scores.slice(0, 10);

  return (
    <div
      style={{
        border: "1px solid #21262d",
        borderRadius: 8,
        padding: "8px 12px",
        fontSize: 11,
        color: "#e6edf3",
        marginTop: 8,
        background: "#0d1117",
      }}
    >
      <div
        style={{
          cursor: "pointer",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
        onClick={() => setExpanded((e) => !e)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setExpanded((v) => !v);
          }
        }}
        role="button"
        tabIndex={0}
      >
        <span style={{ fontWeight: 700, color: "#58a6ff" }}>
          🔄 Strategy Rotation
          {report.topStrategyId != null ? ` — Top #${report.topStrategyId}` : ""}
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {!expanded ? (
        <div
          style={{
            display: "flex",
            gap: 10,
            marginTop: 4,
            flexWrap: "wrap",
          }}
        >
          {summary.map((s) => (
            <span key={s.status} style={{ color: s.color }}>
              {s.label}: {s.count}
            </span>
          ))}
        </div>
      ) : null}

      {expanded ? (
        <>
          <div
            style={{
              display: "flex",
              gap: 6,
              marginTop: 8,
              flexWrap: "wrap",
            }}
          >
            {summary.map((s) => (
              <button
                key={s.status}
                type="button"
                onClick={() =>
                  setFilterStatus(filterStatus === s.status ? null : s.status)
                }
                style={{
                  fontSize: 10,
                  padding: "2px 6px",
                  background: "transparent",
                  border: `1px solid ${s.color}`,
                  borderRadius: 4,
                  cursor: "pointer",
                  color: s.color,
                  fontWeight: filterStatus === s.status ? 700 : 400,
                }}
              >
                {s.label}: {s.count}
              </button>
            ))}
            <button
              type="button"
              onClick={() => setFilterStatus(null)}
              style={{
                fontSize: 10,
                padding: "2px 6px",
                background: "transparent",
                border: "1px solid #8b949e",
                borderRadius: 4,
                cursor: "pointer",
                color: "#8b949e",
              }}
            >
              All
            </button>
          </div>

          <div style={{ marginTop: 8 }}>
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "20px 1fr 45px 45px 45px 45px 50px",
                gap: 4,
                color: "#8b949e",
                borderBottom: "1px solid #21262d",
                paddingBottom: 4,
                marginBottom: 4,
              }}
            >
              <span>#</span>
              <span>Strategy</span>
              <span>Score</span>
              <span>WR%</span>
              <span>E($)</span>
              <span>Fee%</span>
              <span>Status</span>
            </div>

            {displayScores.map((s) => (
              <div
                key={s.strategyId}
                style={{
                  display: "grid",
                  gridTemplateColumns: "20px 1fr 45px 45px 45px 45px 50px",
                  gap: 4,
                  padding: "2px 0",
                  color: STATUS_COLOR[s.status] ?? "#e6edf3",
                  borderBottom: "1px solid #0d1117",
                }}
                title={s.reasoning}
              >
                <span style={{ color: "#8b949e" }}>{s.rank}</span>
                <span
                  style={{
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {STATUS_ICON[s.status]} {s.strategyName}
                </span>
                <span>{s.score}</span>
                <span>
                  {s.tradesAnalyzed < 5
                    ? "--"
                    : `${((s.winRateScore / 25) * 100).toFixed(0)}`}
                </span>
                <span>
                  {s.tradesAnalyzed < 5
                    ? "--"
                    : `${s.expectancyScore >= 15 ? "+" : ""}${(s.expectancyScore - 12).toFixed(0)}`}
                </span>
                <span>{s.tradesAnalyzed < 5 ? "--" : `${s.feeScore}`}</span>
                <span>{s.status}</span>
              </div>
            ))}
          </div>

          {report.scores.length > 10 ? (
            <button
              type="button"
              onClick={() => setShowAll((a) => !a)}
              style={{
                marginTop: 6,
                fontSize: 10,
                padding: "2px 8px",
                background: "transparent",
                border: "1px solid #58a6ff",
                borderRadius: 4,
                cursor: "pointer",
                color: "#58a6ff",
              }}
            >
              {showAll
                ? "Show less"
                : `Show all ${report.scores.length} strategies`}
            </button>
          ) : null}

          {report.suspended.length > 0 ? (
            <div
              style={{
                marginTop: 8,
                borderTop: "1px solid #21262d",
                paddingTop: 6,
              }}
            >
              <div style={{ color: "#f85149", fontWeight: 600, marginBottom: 4 }}>
                🚫 Suspended Strategies ({report.suspended.length})
              </div>
              {report.suspended.map((s) => (
                <div
                  key={s.strategyId}
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    marginBottom: 3,
                  }}
                >
                  <span style={{ color: "#f85149", fontSize: 10 }}>
                    {s.strategyName} (score: {s.score})
                  </span>
                  {onRestore ? (
                    <button
                      type="button"
                      onClick={() => onRestore(s.strategyId)}
                      style={{
                        fontSize: 10,
                        padding: "1px 6px",
                        background: "transparent",
                        border: "1px solid #3fb950",
                        borderRadius: 3,
                        cursor: "pointer",
                        color: "#3fb950",
                      }}
                    >
                      Override
                    </button>
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}

          <div style={{ color: "#8b949e", marginTop: 6, fontSize: 10 }}>
            Computed: {new Date(report.computedAt).toLocaleTimeString()}
            · Hover row for full reasoning
          </div>
        </>
      ) : null}
    </div>
  );
}
