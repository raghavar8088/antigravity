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
  GREEN: "#e6f7ef",
  AMBER: "#fff7e6",
  RED: "#fce8e6",
  UNKNOWN: "#f1f5f9",
};

const STATUS_COLOR: Record<RibbonStatus, string> = {
  GREEN: "#047a50",
  AMBER: "#9a5b00",
  RED: "#b3261e",
  UNKNOWN: "#64748b",
};

const OVERALL_BORDER: Record<RibbonStatus, string> = {
  GREEN: "#bbe8d2",
  AMBER: "#ffe1a6",
  RED: "#f4c7c3",
  UNKNOWN: "#d7dee8",
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
      <div style={ribbonBase("#ffffff", "#e5e7eb")}>
        <span style={{ color: "#64748b", fontSize: 12, fontWeight: 600 }}>Loading system health</span>
      </div>
    );
  }

  if (unavailable || !data) {
    return (
      <div style={ribbonBase("#ffffff", "#f4c7c3")}>
        <span style={{ color: "#b3261e", fontSize: 12, fontWeight: 700 }}>
          System health is updating
        </span>
      </div>
    );
  }

  return (
    <div style={ribbonBase("#ffffff", OVERALL_BORDER[data.overall])}>
      {/* Overall indicator */}
      <div style={{
        display: "flex",
        alignItems: "center",
        gap: 8,
        padding: "7px 12px",
        border: `1px solid ${OVERALL_BORDER[data.overall]}`,
        borderRadius: 999,
        background: STATUS_BG[data.overall],
        marginRight: 10,
        flexShrink: 0,
      }}>
        <div style={{
          width: 7,
          height: 7,
          borderRadius: "50%",
          background: STATUS_COLOR[data.overall],
        }} />
        <span style={{ color: STATUS_COLOR[data.overall], fontSize: 12, fontWeight: 800 }}>
          {statusLabel(data.overall)}
        </span>
      </div>

      {/* Items */}
      <div style={{ display: "flex", gap: 8, flexWrap: "nowrap", overflowX: "auto", flex: 1, paddingBottom: 1 }}>
        {data.items.map((item) => (
          <div
            key={item.label}
            title={item.detail}
            style={{
              display: "flex",
              flexDirection: "row",
              alignItems: "center",
              gap: 8,
              background: STATUS_BG[item.status],
              border: `1px solid ${OVERALL_BORDER[item.status]}`,
              borderRadius: 14,
              padding: "8px 12px",
              minWidth: 104,
              flexShrink: 0,
              boxShadow: "0 1px 2px rgba(60, 64, 67, 0.05)",
            }}
          >
            <span style={{ color: "#64748b", fontSize: 10, fontWeight: 700, letterSpacing: 0.4, textTransform: "uppercase", whiteSpace: "nowrap" }}>
              {shortLabel(item.label)}
            </span>
            <span style={{ color: STATUS_COLOR[item.status], fontSize: 12, fontWeight: 800, whiteSpace: "nowrap" }}>
              {item.value}
            </span>
          </div>
        ))}
      </div>

      {/* Timestamp */}
      <span style={{ color: "#94a3b8", fontSize: 11, marginLeft: 12, flexShrink: 0 }}>
        {safeTime(data.server_time)}
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
    borderBottom: `1px solid #eef2f7`,
    borderTop: `1px solid ${borderColor}`,
    padding: "8px 20px",
    fontFamily: "var(--font-body, system-ui, sans-serif)",
    minHeight: 48,
    overflowX: "auto",
    boxSizing: "border-box",
    boxShadow: "0 1px 2px rgba(60, 64, 67, 0.04)",
  };
}

function statusLabel(status: RibbonStatus) {
  if (status === "GREEN") return "Healthy";
  if (status === "AMBER") return "Watch";
  if (status === "RED") return "Attention";
  return "Updating";
}

function shortLabel(label: string) {
  return label
    .replace("MARKET DATA", "Market")
    .replace("MAX RISK", "Risk")
    .replace("STRATEGY HEALTH", "Strategies")
    .replace("GO ENGINE", "Engine")
    .replace("DATABASE", "Database");
}

function safeTime(value: string) {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? new Date(parsed).toLocaleTimeString() : "";
}
