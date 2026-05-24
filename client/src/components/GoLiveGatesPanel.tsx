"use client";

import { useState } from "react";
import type { GoLiveGateReport } from "@/lib/futuresGoLiveGates";

const REC_COLOR: Record<string, string> = {
  NOT_READY: "#f85149",
  COLLECT_MORE_DATA: "#d29922",
  REVIEW_WARNINGS: "#d29922",
  PAPER_READY: "#3fb950",
};

interface Props {
  report: GoLiveGateReport | null;
  onCopyReport?: () => void;
}

export function GoLiveGatesPanel({ report, onCopyReport }: Props) {
  const [expanded, setExpanded] = useState(false);

  if (!report) {
    return (
      <div style={{ fontSize: 11, color: "#8b949e", padding: "4px 0", marginTop: 6 }}>
        Go-live gates: awaiting 10+ production trades
      </div>
    );
  }

  const recColor = REC_COLOR[report.recommendation] ?? "#8b949e";
  const scorePct = Math.round(report.score * 100);
  const failedBlockers = report.blockers.filter((g) => !g.pass);
  const failedWarnings = report.warnings.filter((g) => !g.pass);

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
        <span style={{ fontWeight: 700, color: recColor }}>
          🚦 Go-live validation — {report.recommendation.replace(/_/g, " ")}
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {!expanded ? (
        <div style={{ marginTop: 4, fontSize: 10, color: "#8b949e" }}>
          Score {scorePct}% · {report.totalProduction} trades · {report.daysOfData.toFixed(1)}d ·{" "}
          {failedBlockers.length} blockers · {failedWarnings.length} warnings
        </div>
      ) : null}

      {expanded ? (
        <div style={{ marginTop: 8 }}>
          <div style={{ marginBottom: 8 }}>
            <div
              style={{
                height: 8,
                background: "#21262d",
                borderRadius: 4,
                overflow: "hidden",
              }}
            >
              <div
                style={{
                  width: `${scorePct}%`,
                  height: "100%",
                  background: recColor,
                }}
              />
            </div>
            <div style={{ fontSize: 10, color: "#8b949e", marginTop: 4 }}>
              Gate score {scorePct}% · {report.totalProduction} production trades ·{" "}
              {report.daysOfData.toFixed(1)} days
            </div>
          </div>

          {failedBlockers.length > 0 ? (
            <div style={{ marginBottom: 8 }}>
              <div style={{ fontWeight: 600, color: "#f85149", marginBottom: 4 }}>Blockers</div>
              {failedBlockers.map((g) => (
                <div key={g.id} style={{ fontSize: 10, marginBottom: 3, color: "#f85149" }}>
                  ✗ {g.label}: {g.value} (need {g.required})
                </div>
              ))}
            </div>
          ) : null}

          {failedWarnings.length > 0 ? (
            <div style={{ marginBottom: 8 }}>
              <div style={{ fontWeight: 600, color: "#d29922", marginBottom: 4 }}>Warnings</div>
              {failedWarnings.map((g) => (
                <div key={g.id} style={{ fontSize: 10, marginBottom: 3, color: "#d29922" }}>
                  ⚠ {g.label}: {g.value} (need {g.required})
                </div>
              ))}
            </div>
          ) : null}

          <div style={{ fontWeight: 600, color: "#8b949e", marginBottom: 4 }}>All gates</div>
          {report.gates.map((g) => (
            <div
              key={g.id}
              style={{
                fontSize: 10,
                color: g.pass ? "#6e7681" : g.severity === "BLOCKER" ? "#f85149" : "#d29922",
                marginBottom: 2,
              }}
            >
              {g.pass ? "✓" : "✗"} [{g.category}] {g.label}: {g.value}
            </div>
          ))}

          {onCopyReport ? (
            <button
              type="button"
              onClick={onCopyReport}
              style={{
                marginTop: 8,
                fontSize: 11,
                padding: "4px 12px",
                background: "#161b22",
                border: "1px solid #3fb950",
                borderRadius: 4,
                cursor: "pointer",
                color: "#3fb950",
              }}
            >
              📋 Copy Validation Report
            </button>
          ) : null}

          <div style={{ fontSize: 9, color: "#6e7681", marginTop: 6 }}>
            Evaluates LIVE_TRADING_PHASE.md §3 — does not enable live trading
          </div>
        </div>
      ) : null}
    </div>
  );
}
