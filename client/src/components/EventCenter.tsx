"use client";

import { useEffect, useRef, useState } from "react";
import { Card } from "@/components/ui/Card";
import { Chip } from "@/components/ui/StatusChip";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select, Switch, TextField } from "@/components/ui/FormControls";
import { StatusChip } from "@/components/ui/StatusChip";

type EventType =
  | "FILL" | "SIGNAL" | "POSITION_OPEN" | "POSITION_CLOSE"
  | "RISK_EVENT" | "KILL_SWITCH" | "RECONCILIATION" | "SYSTEM" | "ORDER";

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

const ALL_TYPES: EventType[] = ["FILL", "SIGNAL", "ORDER", "POSITION_OPEN", "POSITION_CLOSE", "RISK_EVENT", "KILL_SWITCH", "RECONCILIATION", "SYSTEM"];
const ALL_SEVERITIES: EventSeverity[] = ["INFO", "WARNING", "CRITICAL"];

const severityTone = (s: EventSeverity): "success" | "warning" | "error" | "info" => {
  if (s === "CRITICAL") return "error";
  if (s === "WARNING") return "warning";
  return "info";
};

export default function EventCenter() {
  const [events, setEvents] = useState<PlatformEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [filterType, setFilterType] = useState<EventType | "">("");
  const [filterSeverity, setFilterSeverity] = useState<EventSeverity | "">("");
  const [serverTime, setServerTime] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const listRef = useRef<HTMLDivElement>(null);

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
    if (autoScroll && listRef.current) listRef.current.scrollTop = 0;
  }, [events, autoScroll]);

  const filtered = events.filter((e) => {
    if (filterType && e.type !== filterType) return false;
    if (filterSeverity && e.severity !== filterSeverity) return false;
    if (search) {
      const q = search.toLowerCase();
      return e.title.toLowerCase().includes(q) || e.detail.toLowerCase().includes(q) || (e.strategy ?? "").toLowerCase().includes(q);
    }
    return true;
  });

  const columns: DataTableColumn<PlatformEvent>[] = [
    {
      id: "time",
      header: "Time",
      sortable: true,
      sortValue: (r) => r.ts,
      cell: (r) => new Date(r.ts).toLocaleTimeString("en-US", { hour12: false }),
      width: "90px",
    },
    {
      id: "type",
      header: "Type",
      sortable: true,
      sortValue: (r) => r.type,
      cell: (r) => <Chip label={r.type} />,
      width: "120px",
    },
    {
      id: "severity",
      header: "Severity",
      sortable: true,
      sortValue: (r) => r.severity,
      cell: (r) => <StatusChip label={r.severity} tone={severityTone(r.severity)} />,
      width: "110px",
    },
    {
      id: "title",
      header: "Event",
      cell: (r) => (
        <div>
          <div className="m3-event-title">{r.title}</div>
          <div className="m3-event-detail">{r.detail}</div>
        </div>
      ),
    },
  ];

  return (
    <div className="m3-page-stack">
      <PageHeader
        title="Event Console"
        subtitle="Platform event stream — signals, orders, fills, risk events"
        actions={
          <div className="m3-event-header-meta">
            {serverTime ? <span className="m3-event-time">{new Date(serverTime).toLocaleTimeString()}</span> : null}
            <StatusChip label={error ? "Offline" : "Live"} tone={error ? "error" : "success"} />
          </div>
        }
      />

      {error ? (
        <div className="m3-banner m3-banner--error" role="alert">
          Backend authority unavailable: {error}
        </div>
      ) : null}

      <Card title="Filters" subtitle="Search and filter events">
        <div className="m3-event-filters">
          <TextField
            placeholder="Search events…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search events"
          />
          <Select value={filterType} onChange={(e) => setFilterType(e.target.value as EventType | "")} label="Type">
            <option value="">All types</option>
            {ALL_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
          </Select>
          <Select value={filterSeverity} onChange={(e) => setFilterSeverity(e.target.value as EventSeverity | "")} label="Severity">
            <option value="">All severities</option>
            {ALL_SEVERITIES.map((s) => <option key={s} value={s}>{s}</option>)}
          </Select>
          <Switch checked={autoScroll} onCheckedChange={setAutoScroll} label="Auto-scroll" ariaLabel="Auto-scroll event list" />
        </div>
      </Card>

      <Card title="Events" subtitle={`${filtered.length} events`}>
        <div ref={listRef} className="m3-event-list-wrap">
          <DataTable
            columns={columns}
            rows={filtered}
            getRowKey={(r) => r.id}
            loading={loading}
            density="compact"
            emptyTitle="No events"
            emptySubtitle="Events will appear when the engine emits activity"
            pageSize={50}
          />
        </div>
      </Card>
    </div>
  );
}
