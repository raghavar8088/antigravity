"use client";

import { useEffect, useState } from "react";

type RibbonStatus = "GREEN" | "AMBER" | "RED" | "UNKNOWN";

type RibbonItem = {
  label: string;
  status: RibbonStatus;
  value: string;
  detail?: string;
};

type RibbonData = {
  overall: RibbonStatus;
  items: RibbonItem[];
  server_time: string;
};

const STATUS_BG: Record<RibbonStatus, string> = {
  GREEN: "#166534",
  AMBER: "#78350f",
  RED: "#7f1d1d",
  UNKNOWN: "#1e293b",
};

const STATUS_COLOR: Record<RibbonStatus, string> = {
  GREEN: "#22c55e",
  AMBER: "#f59e0b",
  RED: "#ef4444",
  UNKNOWN: "#64748b",
};

const OVERALL_BORDER: Record<RibbonStatus, string> = {
  GREEN: "#166534",
  AMBER: "#d97706",
  RED: "#dc2626",
  UNKNOWN: "#334155",
};

export default function RiskRibbon() {
  const [data, setData] = useState<RibbonData | null>(null);
  const [unavailable, setUnavailable] = useState(false);

  useEffect(() => {
    let cancelled = false;

    const poll = async () => {
      try {
        const res = await fetch("/api/risk-ribbon");
        if (!res.ok) { setUnavailable(true); return; }
        const json = await res.json();
        if (cancelled) return;
        if (json.ok) {
          setData(json);
          setUnavailable(false);
        } else {
          setUnavailable(true);
        }
      } catch {
        if (!cancelled) setUnavailable(true);
      }
    };

    poll();
    const id = setInterval(poll, 5000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  if (!data && !unavailable) {
    return (
      <div style={ribbonBase("#0f172a", "#334155")}>
        <span style={{ color: "#64748b", fontSize: 11 }}>RISK RIBBON — LOADING...</span>
      </div>
    );
  }

  if (unavailable || !data) {
    return (
      <div style={ribbonBase("#1c0a0a", "#dc2626")}>
        <span style={{ color: "#ef4444", fontSize: 11, fontWeight: 700 }}>
          ⚠ RISK RIBBON UNAVAILABLE — BACKEND AUTHORITY UNREACHABLE
        </span>
      </div>
    );
  }

  return (
    <div style={ribbonBase("#0a0f1a", OVERALL_BORDER[data.overall])}>
      {/* Overall indicator */}
      <div style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        paddingRight: 16,
        borderRight: "1px solid #1e293b",
        marginRight: 8,
        flexShrink: 0,
      }}>
        <div style={{
          width: 8,
          height: 8,
          borderRadius: "50%",
          background: STATUS_COLOR[data.overall],
          boxShadow: `0 0 6px ${STATUS_COLOR[data.overall]}`,
        }} />
        <span style={{ color: STATUS_COLOR[data.overall], fontSize: 10, fontWeight: 700, letterSpacing: 1 }}>
          {data.overall}
        </span>
      </div>

      {/* Items */}
      <div style={{ display: "flex", gap: 4, flexWrap: "nowrap", overflow: "hidden", flex: 1 }}>
        {data.items.map((item) => (
          <div
            key={item.label}
            title={item.detail}
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              background: STATUS_BG[item.status],
              border: `1px solid ${STATUS_COLOR[item.status]}22`,
              borderRadius: 4,
              padding: "3px 8px",
              minWidth: 72,
              flexShrink: 0,
            }}
          >
            <span style={{ color: "#64748b", fontSize: 9, letterSpacing: 0.5 }}>{item.label}</span>
            <span style={{ color: STATUS_COLOR[item.status], fontSize: 11, fontWeight: 700, marginTop: 1 }}>
              {item.value}
            </span>
          </div>
        ))}
      </div>

      {/* Timestamp */}
      <span style={{ color: "#334155", fontSize: 10, marginLeft: 8, flexShrink: 0 }}>
        {new Date(data.server_time).toLocaleTimeString()}
      </span>
    </div>
  );
}

function ribbonBase(bg: string, borderColor: string): React.CSSProperties {
  return {
    display: "flex",
    alignItems: "center",
    width: "100%",
    background: bg,
    borderBottom: `1px solid ${borderColor}`,
    padding: "4px 12px",
    fontFamily: "monospace",
    minHeight: 32,
    position: "sticky",
    top: 0,
    zIndex: 1000,
    overflowX: "auto",
    boxSizing: "border-box",
  };
}
