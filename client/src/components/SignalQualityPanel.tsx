"use client";

import { useState } from "react";
import type { SignalQualityScore } from "@/lib/futuresSignalQuality";

interface Props {
  quality: SignalQualityScore | null;
  skipCount: number;
}

const DIM_MAX = 20;

function DimBar({ label, value, color }: { label: string; value: number; color: string }) {
  const width = Math.round((value / DIM_MAX) * 100);
  return (
    <div style={{ marginBottom: 4 }}>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 10, color: "#8b949e" }}>
        <span>{label}</span>
        <span>{value}/20</span>
      </div>
      <div
        style={{
          height: 6,
          background: "#21262d",
          borderRadius: 3,
          marginTop: 2,
        }}
      >
        <div
          style={{
            width: `${width}%`,
            height: "100%",
            background: color,
            borderRadius: 3,
          }}
        />
      </div>
    </div>
  );
}

function QualityBadge({ quality }: { quality: SignalQualityScore }) {
  const label = quality.isHighQuality
    ? "HIGH"
    : quality.isMarginal
      ? "MARGINAL"
      : "LOW";
  const color = quality.isHighQuality
    ? "#3fb950"
    : quality.isMarginal
      ? "#d29922"
      : "#f85149";
  return (
    <span
      style={{
        fontSize: 10,
        fontWeight: 700,
        padding: "2px 6px",
        border: `1px solid ${color}`,
        borderRadius: 4,
        color,
      }}
    >
      {label}
    </span>
  );
}

export function SignalQualityPanel({ quality, skipCount }: Props) {
  const [expanded, setExpanded] = useState(false);

  if (!quality) {
    return (
      <div style={{ fontSize: 11, color: "#8b949e", padding: "4px 0", marginTop: 6 }}>
        Signal Quality: awaiting entry evaluation (skips this session: {skipCount})
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
        <span style={{ fontWeight: 700, color: "#79c0ff" }}>
          🎯 Signal Quality — {quality.total}/100
          {quality.pass ? " ✓" : " ✗"}
        </span>
        <span style={{ color: "#8b949e" }}>{expanded ? "▲" : "▼"}</span>
      </div>

      {!expanded ? (
        <div style={{ marginTop: 4, display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
          <QualityBadge quality={quality} />
          <span style={{ color: "#8b949e", fontSize: 10 }}>
            min {quality.minPassScore} · session skips: {skipCount}
          </span>
        </div>
      ) : null}

      {expanded ? (
        <div style={{ marginTop: 8 }}>
          <div style={{ marginBottom: 8, display: "flex", gap: 8, alignItems: "center" }}>
            <QualityBadge quality={quality} />
            <span style={{ color: quality.pass ? "#3fb950" : "#f85149" }}>
              {quality.pass ? "PASS" : "FAIL"} (need ≥{quality.minPassScore})
            </span>
          </div>

          <DimBar label="Momentum" value={quality.momentumScore} color="#58a6ff" />
          <DimBar label="Regime" value={quality.regimeScore} color="#a371f7" />
          <DimBar label="Session" value={quality.sessionScore} color="#d29922" />
          <DimBar label="Strategy history" value={quality.strategyScore} color="#3fb950" />
          <DimBar label="Signal strength" value={quality.signalStrengthScore} color="#f0883e" />

          {quality.bonuses.length > 0 ? (
            <div style={{ marginTop: 8 }}>
              <div style={{ fontWeight: 600, color: "#3fb950", marginBottom: 4 }}>Bonuses</div>
              {quality.bonuses.map((b) => (
                <div key={b} style={{ fontSize: 10, color: "#8b949e" }}>
                  + {b}
                </div>
              ))}
            </div>
          ) : null}

          {quality.deductions.length > 0 ? (
            <div style={{ marginTop: 8 }}>
              <div style={{ fontWeight: 600, color: "#f85149", marginBottom: 4 }}>Deductions</div>
              {quality.deductions.map((d) => (
                <div key={d} style={{ fontSize: 10, color: "#8b949e" }}>
                  − {d}
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
