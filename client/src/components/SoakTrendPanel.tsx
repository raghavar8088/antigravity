"use client";

import { useState } from "react";
import type { SoakDaySnapshot } from "@/lib/futuresSoakTracker";
import type { UnifiedReadiness } from "@/lib/futuresUnifiedReadiness";
import { unifiedReadinessLabel } from "@/lib/futuresUnifiedReadiness";
import { deskReplayGateEnabled } from "@/lib/futuresReplayCompare";

const STATE_COLOR: Record<UnifiedReadiness, string> = {
  NOT_READY: "#f85149",
  COLLECT_DATA: "#8b949e",
  PAPER_EDGE_OK: "#d29922",
  PAPER_READY: "#58a6ff",
  TESTNET_SOAK_READY: "#3fb950",
};

interface Props {
  unifiedState: UnifiedReadiness;
  blockers: string[];
  nextStep: string;
  soakHistory: SoakDaySnapshot[];
  soakSummary: {
    daysTracked: number;
    greenDays: number;
    avgExpectancy7d: number;
    improving: boolean;
  };
  replaySignFlipRate: number | null;
  accountKey?: string | null;
}

export function SoakTrendPanel({
  unifiedState,
  blockers,
  nextStep,
  soakHistory,
  soakSummary,
  replaySignFlipRate,
  accountKey,
}: Props) {
  const [expanded, setExpanded] = useState(true);
  const color = STATE_COLOR[unifiedState];
  const last7 = soakHistory.slice(-7);
  const replayGateOn = deskReplayGateEnabled();

  return (
    <div
      style={{
        border: `2px solid ${color}55`,
        borderRadius: 8,
        padding: "10px 12px",
        marginBottom: 8,
        background: "#0d1117",
        fontSize: 11,
        color: "#e6edf3",
      }}
    >
      <div
        style={{ display: "flex", justifyContent: "space-between", cursor: "pointer" }}
        onClick={() => setExpanded((e) => !e)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setExpanded((v) => !v);
          }
        }}
      >
        <span style={{ fontSize: 15, fontWeight: 800, color }}>
          {unifiedReadinessLabel(unifiedState)}
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {!expanded ? (
        <div style={{ marginTop: 6, fontSize: 10, color: "#8b949e" }}>
          {soakSummary.greenDays}/7 green · replay{" "}
          {replaySignFlipRate != null
            ? `${(replaySignFlipRate * 100).toFixed(0)}% flip`
            : replayGateOn
              ? "pending"
              : "off"}
        </div>
      ) : (
        <div style={{ marginTop: 10 }}>
          <div style={{ fontSize: 10, color: "#8b949e", marginBottom: 8 }}>
            Soak: {soakSummary.greenDays}/7 green days · avg E ${soakSummary.avgExpectancy7d.toFixed(2)}
            {soakSummary.improving ? " · ↑ improving" : " · → flat/down"}
            {replaySignFlipRate != null
              ? ` · replay flip ${(replaySignFlipRate * 100).toFixed(1)}%`
              : ""}
          </div>

          {last7.length > 0 ? (
            <table style={{ width: "100%", fontSize: 10, borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ color: "#8b949e", textAlign: "left" }}>
                  <th>UTC date</th>
                  <th>closes</th>
                  <th>E</th>
                  <th>fee/gross</th>
                  <th>grade</th>
                </tr>
              </thead>
              <tbody>
                {last7.map((d) => (
                  <tr key={d.dateUtc} style={{ borderTop: "1px solid #21262d" }}>
                    <td>{d.dateUtc}</td>
                    <td>{d.closes}</td>
                    <td style={{ color: d.expectancy >= 0 ? "#3fb950" : "#f85149" }}>
                      ${d.expectancy.toFixed(2)}
                    </td>
                    <td>{d.feePctOfAbsGross.toFixed(1)}%</td>
                    <td
                      style={{
                        color:
                          d.grade === "GREEN"
                            ? "#3fb950"
                            : d.grade === "YELLOW"
                              ? "#d29922"
                              : "#f85149",
                      }}
                    >
                      {d.grade}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div style={{ color: "#8b949e" }}>No soak days recorded yet — snapshots append daily.</div>
          )}

          {blockers.length > 0 ? (
            <ul style={{ margin: "10px 0 0", paddingLeft: 18, color: "#f85149" }}>
              {blockers.map((b) => (
                <li key={b}>{b}</li>
              ))}
            </ul>
          ) : null}

          <p style={{ marginTop: 10, color: "#c9d1d9", lineHeight: 1.45 }}>{nextStep}</p>

          {replaySignFlipRate != null && replaySignFlipRate > 0.15 ? (
            <p style={{ marginTop: 8, fontSize: 10, color: "#d29922" }}>
              Run replay compare:{" "}
              <code style={{ fontSize: 9 }}>
                npm run replay:compare -- --account_key={accountKey ?? "<key>"} --date=
                {new Date().toISOString().slice(0, 10)}
              </code>
            </p>
          ) : null}
        </div>
      )}
    </div>
  );
}
