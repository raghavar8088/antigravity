"use client";

import { useEffect, useRef, useState } from "react";

type EventType =
  | "FILL"
  | "SIGNAL"
  | "POSITION_OPEN"
  | "POSITION_CLOSE"
  | "RISK_EVENT"
  | "KILL_SWITCH"
  | "RECONCILIATION"
  | "SYSTEM";

type EventSeverity = "INFO" | "WARNING" | "CRITICAL";

type PlatformEvent = {
  id: string;
  type: EventType;
  severity: EventSeverity;
  title: string;
  detail: string;
  strategy?: string;
  symbol?: string;
  pnl?: number;
  ts: string;
};

const SEV_COLOR: Record<EventSeverity, string> = {
  INFO: "#22c55e",
  WARNING: "#f59e0b",
  CRITICAL: "#ef4444",
};

const TYPE_COLOR: Record<EventType, string> = {
  FILL: "#3b82f6",
  SIGNAL: "#8b5cf6",
  POSITION_OPEN: "#22c55e",
  POSITION_CLOSE: "#94a3b8",
  RISK_EVENT: "#ef4444",
  KILL_SWITCH: "#dc2626",
  RECONCILIATION: "#f59e0b",
  SYSTEM: "#64748b",
};

const ALL_TYPES: EventType[] = ["FILL", "SIGNAL", "POSITION_OPEN", "POSITION_CLOSE", "RISK_EVENT", "KILL_SWITCH", "RECONCILIATION", "SYSTEM"];
const ALL_SEVERITIES: EventSeverity[] = ["INFO", "WARNING", "CRITICAL"];

export default function EventCenter() {
  const [events, setEvents] = useState<PlatformEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [filterType, setFilterType] = useState<EventType | "">("");
  const [filterSeverity, setFilterSeverity] = useState<EventSeverity | "">("");
  const [serverTime, setServerTime] = useState("");
  const listRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const poll = async () => {
      try {
        const res = await fetch("/api/event-center");
        if (cancelled) return;
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = await res.json();
        if (!data.ok) throw new Error(data.error ?? "Event center unavailable");
        setEvents(data.events);
        setServerTime(data.server_time);
        setError(null);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "Unavailable");
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    poll();
    const id = setInterval(poll, 3000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  useEffect(() => {
    if (autoScroll && listRef.current) {
      listRef.current.scrollTop = 0;
    }
  }, [events, autoScroll]);

  const filtered = events.filter((e) => {
    if (filterType && e.type !== filterType) return false;
    if (filterSeverity && e.severity !== filterSeverity) return false;
    if (search) {
      const q = search.toLowerCase();
      return (
        e.title.toLowerCase().includes(q) ||
        e.detail.toLowerCase().includes(q) ||
        (e.strategy ?? "").toLowerCase().includes(q)
      );
    }
    return true;
  });

  return (
    <div style={{ fontFamily: "monospace", fontSize: 12, color: "#e2e8f0", background: "#0f172a", padding: 16, height: "100%" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
        <h2 style={{ fontSize: 14, fontWeight: 700, color: "#f8fafc", margin: 0 }}>
          INSTITUTIONAL EVENT CONSOLE
        </h2>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span style={{ color: "#334155", fontSize: 10 }}>{serverTime ? new Date(serverTime).toLocaleTimeString() : ""}</span>
          <div style={{
            width: 6, height: 6, borderRadius: "50%",
            background: error ? "#ef4444" : "#22c55e",
            boxShadow: `0 0 4px ${error ? "#ef4444" : "#22c55e"}`,
          }} />
        </div>
      </div>

      {/* Filters */}
      <div style={{ display: "flex", gap: 6, marginBottom: 10, flexWrap: "wrap" }}>
        <input
          type="text"
          placeholder="Search events..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={inputStyle}
        />
        <select
          value={filterType}
          onChange={(e) => setFilterType(e.target.value as EventType | "")}
          style={inputStyle}
        >
          <option value="">All Types</option>
          {ALL_TYPES.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
        <select
          value={filterSeverity}
          onChange={(e) => setFilterSeverity(e.target.value as EventSeverity | "")}
          style={inputStyle}
        >
          <option value="">All Severities</option>
          {ALL_SEVERITIES.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <label style={{ display: "flex", alignItems: "center", gap: 4, color: "#64748b", fontSize: 11 }}>
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
          />
          Auto-scroll
        </label>
        <span style={{ marginLeft: "auto", color: "#475569", fontSize: 11, alignSelf: "center" }}>
          {filtered.length} events
        </span>
      </div>

      {loading && <div style={{ color: "#64748b", padding: 24, textAlign: "center" }}>LOADING EVENTS...</div>}
      {error && (
        <div style={{ color: "#ef4444", padding: 12, background: "#1e293b", borderRadius: 4, border: "1px solid #ef4444", marginBottom: 8 }}>
          BACKEND AUTHORITY UNAVAILABLE: {error}
        </div>
      )}

      {/* Event list */}
      <div
        ref={listRef}
        style={{ height: "calc(100% - 120px)", overflowY: "auto", display: "flex", flexDirection: "column", gap: 2 }}
      >
        {filtered.length === 0 && !loading && !error && (
          <div style={{ color: "#334155", padding: 24, textAlign: "center" }}>NO EVENTS</div>
        )}
        {filtered.map((ev) => (
          <div
            key={ev.id}
            style={{
              display: "grid",
              gridTemplateColumns: "80px 90px 90px 1fr",
              alignItems: "start",
              gap: 8,
              padding: "6px 8px",
              background: ev.severity === "CRITICAL" ? "#1c0a0a" : "#111827",
              borderLeft: `2px solid ${SEV_COLOR[ev.severity]}`,
              borderRadius: "0 4px 4px 0",
            }}
          >
            <span style={{ color: "#475569", fontSize: 10, paddingTop: 1 }}>
              {new Date(ev.ts).toLocaleTimeString("en-US", { hour12: false })}
            </span>
            <span style={{
              color: TYPE_COLOR[ev.type],
              fontSize: 10,
              fontWeight: 600,
              paddingTop: 1,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}>
              {ev.type}
            </span>
            <span style={{ color: SEV_COLOR[ev.severity], fontSize: 10, paddingTop: 1 }}>
              {ev.severity}
            </span>
            <div>
              <div style={{ color: "#e2e8f0", fontWeight: 600, marginBottom: 2 }}>{ev.title}</div>
              <div style={{ color: "#64748b", fontSize: 11 }}>{ev.detail}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

const inputStyle: React.CSSProperties = {
  padding: "4px 8px",
  background: "#1e293b",
  border: "1px solid #334155",
  borderRadius: 3,
  color: "#e2e8f0",
  fontSize: 11,
  fontFamily: "monospace",
};
