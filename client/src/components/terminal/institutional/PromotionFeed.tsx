"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { StrategyEventDoc } from "@/lib/strategyAuthority/types";

const EVENT_CONFIG = {
  PROMOTION: { label: "PROMOTED", colorClass: "text-emerald-400", bgClass: "border-emerald-900/40 bg-emerald-950/15", arrow: "↑" },
  DEMOTION: { label: "DEMOTED", colorClass: "text-rose-400", bgClass: "border-rose-900/40 bg-rose-950/15", arrow: "↓" },
  RETIREMENT: { label: "RETIRED", colorClass: "text-rose-600", bgClass: "border-rose-900/60 bg-rose-950/25", arrow: "✕" },
  MIGRATION: { label: "MIGRATED", colorClass: "text-zinc-400", bgClass: "border-zinc-800 bg-zinc-900/30", arrow: "→" },
};

const STATUS_SHORT: Record<string, string> = {
  GRADE_5: "G5", GRADE_4: "G4", GRADE_3: "G3",
  GRADE_2: "G2", GRADE_1: "G1", MAIN_ENGINE: "MAIN", RETIRED: "RET",
};

function fmt(n: number | null | undefined, dec = 2) {
  if (n == null || isNaN(n)) return "—";
  return n.toFixed(dec);
}

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return new Date(iso).toLocaleDateString();
}

function EventRow({ event }: { event: StrategyEventDoc }) {
  const cfg = EVENT_CONFIG[event.event_type] ?? EVENT_CONFIG.MIGRATION;
  const m = event.metrics_snapshot;

  return (
    <div className={`rounded border px-2.5 py-1.5 ${cfg.bgClass} flex items-start gap-2`}>
      {/* Type badge */}
      <div className={`flex-shrink-0 pt-0.5 w-14 text-[9px] font-bold uppercase ${cfg.colorClass}`}>
        <span>{cfg.arrow}</span>
        <span className="ml-0.5">{cfg.label}</span>
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs font-medium text-zinc-200 truncate">{event.strategy_name}</span>
          <span className="text-[9px] text-zinc-600 flex-shrink-0">{timeAgo(event.created_at)}</span>
        </div>
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-[9px] text-zinc-500">
            {STATUS_SHORT[event.from_status] ?? event.from_status}
            {" → "}
            <span className={cfg.colorClass}>{STATUS_SHORT[event.to_status] ?? event.to_status}</span>
          </span>
          {m && (
            <span className="text-[9px] text-zinc-600 tabular-nums">
              WR {fmt(m.winRate, 1)}% · PF {fmt(m.profitFactor, 2)} · E {fmt(m.expectancy, 2)}
            </span>
          )}
        </div>
        {event.reason && event.event_type !== "MIGRATION" && (
          <div className="mt-0.5 text-[9px] text-zinc-700 truncate">{event.reason}</div>
        )}
      </div>
    </div>
  );
}

type FilterType = "ALL" | "PROMOTION" | "DEMOTION" | "RETIREMENT";

export function PromotionFeed({
  maxHeight = "max-h-96",
  compact = false,
}: {
  maxHeight?: string;
  compact?: boolean;
}) {
  const [events, setEvents] = useState<StrategyEventDoc[]>([]);
  const [filter, setFilter] = useState<FilterType>("ALL");
  const [loading, setLoading] = useState(true);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchEvents = useCallback(async () => {
    try {
      const url = filter === "ALL"
        ? "/api/strategy-authority/events?limit=500"
        : `/api/strategy-authority/events?limit=200&type=${filter}`;
      const res = await fetch(url);
      const data = await res.json();
      if (data.ok) setEvents(data.events);
    } finally {
      setLoading(false);
    }
  }, [filter]);

  useEffect(() => {
    setLoading(true);
    fetchEvents();
    if (pollingRef.current) clearInterval(pollingRef.current);
    pollingRef.current = setInterval(fetchEvents, 15_000);
    return () => { if (pollingRef.current) clearInterval(pollingRef.current); };
  }, [fetchEvents]);

  const promotions = events.filter((e) => e.event_type === "PROMOTION").length;
  const demotions = events.filter((e) => e.event_type === "DEMOTION").length;
  const retirements = events.filter((e) => e.event_type === "RETIREMENT").length;

  const displayed = events.filter((e) =>
    filter === "ALL" ? e.event_type !== "MIGRATION" : e.event_type === filter
  );

  return (
    <div className="flex flex-col h-full">
      {/* Filter bar */}
      {!compact && (
        <div className="flex items-center gap-1 mb-2 flex-wrap">
          {(["ALL", "PROMOTION", "DEMOTION", "RETIREMENT"] as FilterType[]).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`px-2 py-0.5 rounded text-[9px] font-bold uppercase border transition-colors ${
                filter === f
                  ? f === "PROMOTION" ? "border-emerald-700 bg-emerald-900/40 text-emerald-400"
                  : f === "DEMOTION" || f === "RETIREMENT" ? "border-rose-700 bg-rose-900/30 text-rose-400"
                  : "border-zinc-600 bg-zinc-800 text-zinc-300"
                  : "border-zinc-800 text-zinc-600 hover:text-zinc-400"
              }`}
            >
              {f}
              {f === "PROMOTION" && promotions > 0 && <span className="ml-1 text-emerald-600">{promotions}</span>}
              {f === "DEMOTION" && demotions > 0 && <span className="ml-1 text-rose-700">{demotions}</span>}
              {f === "RETIREMENT" && retirements > 0 && <span className="ml-1 text-rose-800">{retirements}</span>}
            </button>
          ))}
          <span className="ml-auto text-[9px] text-zinc-600">{displayed.length} events</span>
        </div>
      )}

      {/* Events list */}
      <div className={`overflow-y-auto space-y-1 ${maxHeight}`}>
        {loading ? (
          <div className="py-6 text-center text-xs text-zinc-600 animate-pulse">Loading events…</div>
        ) : displayed.length === 0 ? (
          <div className="py-8 text-center text-xs text-zinc-600">
            No {filter === "ALL" ? "" : filter.toLowerCase()} events yet.
            <br />
            <span className="text-[10px] text-zinc-700">Run evaluation to generate events.</span>
          </div>
        ) : (
          displayed.map((e) => <EventRow key={e.event_id} event={e} />)
        )}
      </div>
    </div>
  );
}
