"use client";

import { useEffect, useState } from "react";

export type ReplayEvent = {
  id: string;
  time: string;
  title: string;
  detail: string;
  tone: "positive" | "negative" | "neutral";
};

type PricePoint = {
  time: number;
  value: number;
};

type Props = {
  workspaceLabel: string;
  summary: string;
  priceSeries: PricePoint[];
  events: ReplayEvent[];
};

function formatClock(value: number | string) {
  const date = typeof value === "number" ? new Date(value) : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return date.toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit" });
}

export default function ReplayBacktestPanel({
  workspaceLabel,
  summary,
  priceSeries,
  events,
}: Props) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [cursor, setCursor] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [isPlaying, setIsPlaying] = useState(false);
  const visibleSeries = priceSeries.slice(-90);

  useEffect(() => {
    if (!isPlaying || visibleSeries.length < 2) {
      return;
    }
    const timer = window.setInterval(() => {
      setCursor((current) => {
        if (current >= visibleSeries.length - 1) {
          setIsPlaying(false);
          return current;
        }
        return current + 1;
      });
    }, Math.max(120, 700 / speed));
    return () => window.clearInterval(timer);
  }, [isPlaying, speed, visibleSeries.length]);

  const stats = (() => {
    if (visibleSeries.length === 0) {
      return { start: 0, end: 0, high: 0, low: 0, changePct: 0 };
    }
    const values = visibleSeries.map((point) => point.value);
    const start = visibleSeries[0].value;
    const end = visibleSeries[Math.min(cursor, visibleSeries.length - 1)].value;
    return {
      start,
      end,
      high: Math.max(...values),
      low: Math.min(...values),
      changePct: start > 0 ? ((end - start) / start) * 100 : 0,
    };
  })();

  return (
    <section className="glass-panel px-5 py-5 md:px-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">Replay And Backtest</div>
          <div className="text-xl font-semibold text-zinc-900">{workspaceLabel}</div>
          <div className="max-w-[820px] text-sm leading-6" style={{ color: "var(--text-secondary)" }}>
            {summary}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className="btn-gold text-sm" onClick={() => setIsExpanded((value) => !value)}>
            {isExpanded ? "Hide Replay" : "Open Replay"}
          </button>
          <button type="button" className="btn-primary text-sm" onClick={() => setIsPlaying((value) => !value)} disabled={!isExpanded || visibleSeries.length < 2}>
            {isPlaying ? "Pause" : "Play"}
          </button>
        </div>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Window Start</div>
          <div className="metric-value text-zinc-900">{visibleSeries.length > 0 ? formatClock(visibleSeries[0].time) : "--"}</div>
        </div>
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Cursor</div>
          <div className="metric-value text-zinc-900">{visibleSeries.length > 0 ? formatClock(visibleSeries[Math.min(cursor, visibleSeries.length - 1)].time) : "--"}</div>
        </div>
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Replay Change</div>
          <div className={`metric-value ${stats.changePct >= 0 ? "text-emerald-600" : "text-rose-600"}`}>{stats.changePct >= 0 ? "+" : ""}{stats.changePct.toFixed(2)}%</div>
        </div>
        <div className="metric-card min-h-[104px]">
          <div className="metric-label">Decision Events</div>
          <div className="metric-value text-zinc-900">{events.length}</div>
        </div>
      </div>

      {isExpanded && (
        <div className="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
          <div className="glass-panel px-5 py-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">Playback Surface</div>
              <div className="flex items-center gap-2">
                <span className="text-xs" style={{ color: "var(--text-secondary)" }}>Speed</span>
                {[1, 2, 4].map((value) => (
                  <button
                    key={value}
                    type="button"
                    className={`settings-chip${speed === value ? " active" : ""}`}
                    onClick={() => setSpeed(value)}
                  >
                    {value}x
                  </button>
                ))}
              </div>
            </div>

            {visibleSeries.length < 2 ? (
              <div className="mt-4 rounded-[20px] border border-dashed px-6 py-10 text-center text-sm" style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}>
                Live candle playback is not available for this workspace yet. The decision timeline still captures trades, operator events, and feed milestones.
              </div>
            ) : (
              <div className="mt-4 space-y-4">
                <input
                  type="range"
                  min="0"
                  max={Math.max(visibleSeries.length - 1, 0)}
                  value={Math.min(cursor, Math.max(visibleSeries.length - 1, 0))}
                  onChange={(event) => setCursor(Number(event.target.value))}
                  className="w-full"
                />
                <div className="grid gap-3 md:grid-cols-3">
                  <div className="metric-card">
                    <div className="metric-label">Replay Price</div>
                    <div className="metric-value text-zinc-900">{stats.end.toFixed(2)}</div>
                  </div>
                  <div className="metric-card">
                    <div className="metric-label">Session High</div>
                    <div className="metric-value text-zinc-900">{stats.high.toFixed(2)}</div>
                  </div>
                  <div className="metric-card">
                    <div className="metric-label">Session Low</div>
                    <div className="metric-value text-zinc-900">{stats.low.toFixed(2)}</div>
                  </div>
                </div>
              </div>
            )}
          </div>

          <div className="glass-panel px-5 py-5">
            <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">Decision Timeline</div>
            <div className="mt-4 space-y-3">
              {events.length === 0 ? (
                <div className="rounded-[20px] border border-dashed px-4 py-6 text-center text-sm" style={{ borderColor: "var(--border)", color: "var(--text-secondary)" }}>
                  No trade or operator events have been recorded yet for this replay window.
                </div>
              ) : (
                events.map((event) => (
                  <div key={event.id} className="timeline-row">
                    <div className={`timeline-dot ${event.tone}`} />
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div className="text-sm font-medium text-zinc-900">{event.title}</div>
                        <div className="text-[11px] uppercase tracking-[0.12em]" style={{ color: "var(--text-muted)" }}>
                          {formatClock(event.time)}
                        </div>
                      </div>
                      <div className="mt-1 text-xs leading-5" style={{ color: "var(--text-secondary)" }}>{event.detail}</div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
