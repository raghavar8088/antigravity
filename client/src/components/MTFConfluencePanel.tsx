"use client";

import { useState } from "react";
import type { MTFConfluenceResult } from "@/lib/trading/futuresMTFConfluence";

interface Props {
  result: MTFConfluenceResult | null;
  skipCount: number;
}

const BIAS_COLOR: Record<string, string> = {
  BULLISH: "#3fb950",
  BEARISH: "#f85149",
  NEUTRAL: "#8b949e",
};

export function MTFConfluencePanel({ result, skipCount }: Props) {
  const [expanded, setExpanded] = useState(false);

  if (!result) {
    return (
      <div style={{ fontSize: 11, color: "#8b949e", padding: "4px 0", marginTop: 6 }}>
        MTF Confluence: awaiting entry evaluation (skips this session: {skipCount})
      </div>
    );
  }

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
        <span style={{ fontWeight: 700, color: BIAS_COLOR[result.overallBias] ?? "#58a6ff" }}>
          📐 MTF — {result.overallBias} {result.confluenceScore}%
          {result.isConfluent ? " ✓" : ""}
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {!expanded ? (
        <div style={{ marginTop: 4, fontSize: 10, color: "#8b949e" }}>
          agrees={result.agrees ? "yes" : "no"} · {result.alignedCount}/{result.totalAvailable} TFs
          aligned · skips: {skipCount}
        </div>
      ) : null}

      {expanded ? (
        <div style={{ marginTop: 8 }}>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: 6,
              fontSize: 10,
              marginBottom: 8,
            }}
          >
            <span>
              Score: <strong>{result.confluenceScore}</strong>
            </span>
            <span>
              Agrees:{" "}
              <strong style={{ color: result.agrees ? "#3fb950" : "#f85149" }}>
                {result.agrees ? "yes" : "no"}
              </strong>
            </span>
            <span>
              High TF: <strong style={{ color: BIAS_COLOR[result.highTFBias] }}>{result.highTFBias}</strong>
            </span>
            <span>
              Low TF: <strong style={{ color: BIAS_COLOR[result.lowTFBias] }}>{result.lowTFBias}</strong>
            </span>
            <span>Confluent: {result.isConfluent ? "yes" : "no"}</span>
            <span>
              Aligned: {result.alignedCount}/{result.totalAvailable}
            </span>
          </div>

          <div style={{ fontWeight: 600, color: "#8b949e", marginBottom: 4 }}>Per timeframe</div>
          {result.tfResults.map((row) => (
            <div
              key={row.tf}
              style={{
                display: "grid",
                gridTemplateColumns: "36px 56px 24px 1fr",
                gap: 4,
                fontSize: 10,
                marginBottom: 3,
                color: BIAS_COLOR[row.bias] ?? "#e6edf3",
              }}
              title={row.reasons.join(" · ")}
            >
              <span style={{ color: "#8b949e" }}>{row.tf}</span>
              <span>{row.bias}</span>
              <span style={{ color: "#8b949e" }}>s{row.strength}</span>
              <span
                style={{
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  color: "#8b949e",
                }}
              >
                {row.reasons[0] ?? "—"}
              </span>
            </div>
          ))}

          {result.reasons.length > 0 ? (
            <div style={{ marginTop: 8, borderTop: "1px solid #21262d", paddingTop: 6 }}>
              <div style={{ fontWeight: 600, color: "#8b949e", marginBottom: 4 }}>Notes</div>
              {result.reasons.map((r) => (
                <div key={r} style={{ fontSize: 10, color: "#8b949e" }}>
                  • {r}
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
