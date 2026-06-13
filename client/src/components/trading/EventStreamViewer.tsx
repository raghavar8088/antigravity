"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { cn } from "@/components/ui/cn";
import type { EngineEvent, EngineEventType } from "@/types/trading";

const EVENT_COLOR: Record<EngineEventType, string> = {
  OrderCreated:           "text-[var(--color-info)]",
  OrderAccepted:          "text-[var(--color-info)]",
  OrderFilled:            "text-[var(--color-profit)]",
  PartialFilled:          "text-[var(--color-profit)]",
  OrderRejected:          "text-[var(--color-loss)]",
  OrderCancelled:         "text-[var(--color-neutral)]",
  PositionOpened:         "text-[var(--color-profit)]",
  PositionClosed:         "text-[var(--color-profit)]",
  RiskViolation:          "text-[var(--color-warning)] font-bold",
  KillSwitchTriggered:    "text-[var(--color-loss)] font-bold",
  ReconciliationMismatch: "text-[var(--color-warning)]",
  StrategySignal:         "text-[var(--color-text-secondary)]",
  EngineHealthCheck:      "text-[var(--color-text-muted)]",
};

const ALL_TYPES: EngineEventType[] = [
  "OrderCreated","OrderAccepted","OrderFilled","PartialFilled",
  "OrderRejected","OrderCancelled","PositionOpened","PositionClosed",
  "RiskViolation","KillSwitchTriggered","ReconciliationMismatch",
  "StrategySignal","EngineHealthCheck",
];

function formatMs(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2,"0")}:${String(d.getMinutes()).padStart(2,"0")}:${String(d.getSeconds()).padStart(2,"0")}.${String(d.getMilliseconds()).padStart(3,"0")}`;
}

export function EventStreamViewer() {
  const [events, setEvents] = useState<EngineEvent[]>([]);
  const [paused, setPaused] = useState(false);
  const [filterTypes, setFilterTypes] = useState<Set<EngineEventType>>(new Set(ALL_TYPES));
  const [search, setSearch] = useState("");
  const [connected, setConnected] = useState(false);
  const bufRef = useRef<EngineEvent[]>([]);
  const bottomRef = useRef<HTMLDivElement>(null);
  const abortRef = useRef<AbortController | null>(null);

  const onSSEMessage = useCallback((data: unknown) => {
    const d = data as { ok?: boolean; events?: EngineEvent[] };
    if (!d.ok || !Array.isArray(d.events)) return;
    setConnected(true);
    bufRef.current = [...bufRef.current, ...d.events].slice(-1000);
    if (!paused) {
      setEvents([...bufRef.current]);
    }
  }, [paused]);

  useEffect(() => {
    let cancelled = false;
    async function connect() {
      let backoff = 1000;
      while (!cancelled) {
        abortRef.current = new AbortController();
        try {
          const res = await fetch("/api/engine/events", { signal: abortRef.current.signal, cache: "no-store" });
          if (!res.body) throw new Error("no body");
          const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
          backoff = 1000;
          let buf = "";
          while (!cancelled) {
            const { done, value } = await reader.read();
            if (done) break;
            buf += value;
            const lines = buf.split("\n");
            buf = lines.pop() ?? "";
            for (const line of lines) {
              if (line.startsWith("data: ")) {
                try { onSSEMessage(JSON.parse(line.slice(6))); } catch { /* ignore */ }
              }
            }
          }
        } catch {
          if (cancelled) break;
          await new Promise((r) => setTimeout(r, backoff));
          backoff = Math.min(backoff * 2, 30_000);
          setConnected(false);
        }
      }
    }
    connect();
    return () => { cancelled = true; abortRef.current?.abort(); };
  }, [onSSEMessage]);

  // Auto-scroll when not paused
  useEffect(() => {
    if (!paused) bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [events, paused]);

  // Resume: flush buffer
  function handleResume() {
    setEvents([...bufRef.current]);
    setPaused(false);
  }

  function exportEvents() {
    const blob = new Blob([JSON.stringify(events.slice(-1000), null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `events-${Date.now()}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  function toggleType(t: EngineEventType) {
    setFilterTypes((prev) => {
      const next = new Set(prev);
      if (next.has(t)) next.delete(t); else next.add(t);
      return next;
    });
  }

  const visible = events.filter((e) =>
    filterTypes.has(e.type) &&
    (!search.trim() || [e.type, e.symbol, e.strategyId, e.orderId, e.details]
      .filter(Boolean).join(" ").toLowerCase().includes(search.toLowerCase()))
  );

  return (
    <div className="flex flex-col h-full">
      {/* Controls */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--color-border)] flex-wrap">
        <span className="text-[11px] font-medium text-[var(--color-text-secondary)]">Event Stream</span>
        {connected ? (
          <span className="w-[5px] h-[5px] rounded-full bg-[var(--color-profit)] animate-pulse" />
        ) : (
          <span className="text-[10px] text-[var(--color-warning)]">Reconnecting…</span>
        )}
        <button
          type="button"
          onClick={() => paused ? handleResume() : setPaused(true)}
          className={cn(
            "text-[10px] px-2 py-0.5 rounded border transition-colors",
            paused
              ? "border-[var(--color-profit)] text-[var(--color-profit)]"
              : "border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]",
          )}
        >
          {paused ? `▶ Resume (+${bufRef.current.length - events.length} new)` : "⏸ Pause"}
        </button>
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search order ID, symbol…"
          className="text-[11px] px-2 py-0.5 rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] outline-none focus:border-[var(--color-border-focus)] w-36"
          aria-label="Search events"
        />
        <button
          type="button"
          onClick={exportEvents}
          className="text-[10px] px-2 py-0.5 rounded border border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]"
        >
          Export JSON
        </button>

        {/* Type filter */}
        <div className="flex items-center gap-1 flex-wrap ml-auto">
          {["RiskViolation","KillSwitchTriggered","ReconciliationMismatch"].map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => toggleType(t as EngineEventType)}
              className={cn(
                "text-[9px] px-1.5 py-0.5 rounded border transition-colors",
                filterTypes.has(t as EngineEventType)
                  ? "border-[var(--color-warning)] text-[var(--color-warning)]"
                  : "border-[var(--color-border)] text-[var(--color-text-muted)] opacity-40",
              )}
            >
              {t.replace(/([A-Z])/g, " $1").trim()}
            </button>
          ))}
        </div>
      </div>

      {/* Event rows */}
      <div className="flex-1 overflow-y-auto font-mono text-[11px] px-3 py-1">
        {visible.length === 0 ? (
          <div className="flex items-center justify-center h-full text-[var(--color-text-muted)]">
            No events match the current filter
          </div>
        ) : (
          visible.slice(-500).map((e) => (
            <div
              key={e.eventId}
              className={cn(
                "py-0.5 border-b border-[rgba(255,255,255,0.03)] hover:bg-[var(--color-bg-elevated)]",
                e.type === "KillSwitchTriggered" && "animate-pulse",
              )}
            >
              <span className="text-[var(--color-text-muted)] mr-2">[{formatMs(e.timestamp)}]</span>
              <span className={cn("mr-2", EVENT_COLOR[e.type])}>{e.type}</span>
              {e.symbol ? <span className="text-[var(--color-text-primary)] mr-2">· {e.symbol}</span> : null}
              <span className="text-[var(--color-text-secondary)]">· {e.details}</span>
              {e.strategyId ? <span className="text-[var(--color-text-muted)] ml-2">· {e.strategyId}</span> : null}
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
