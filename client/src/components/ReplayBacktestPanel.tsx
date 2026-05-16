"use client";

import { useCallback, useEffect, useState } from "react";
import {
  deskSlippageBpsFromEnv,
  deskVolSizedNotionalEnabledFromEnv,
} from "@/lib/futuresDeskPolicy";
import type { PaperReplayApiSuccess } from "@/lib/futuresReplayUi";
import { DeskBanner } from "@/components/desk/ui/DeskBanner";
import { DeskButton } from "@/components/desk/ui/DeskButton";
import { DeskCard } from "@/components/desk/ui/DeskCard";
import { DeskChip } from "@/components/desk/ui/DeskChip";
import { DeskMetricTile } from "@/components/desk/ui/DeskMetricTile";
import { DeskSectionHeader } from "@/components/desk/ui/DeskSectionHeader";
import {
  buildDeskReplaySearchParams,
  formatReplaySummary,
  isDeskReplayUiEnabled,
  mapReplayTradesToTableRows,
  parsePaperReplayApiResponse,
  REPLAY_EMPTY_FIXTURE_HINT,
  replayErrorWithFixtureHint,
  type DeskReplayFixtureKind,
} from "@/lib/futuresReplayUi";

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

export type DeskReplayPanelOptions = {
  symbol?: string;
  bars?: number;
  fixture?: DeskReplayFixtureKind;
  accountKey?: string;
};

type Props = {
  workspaceLabel: string;
  summary: string;
  priceSeries: PricePoint[];
  events: ReplayEvent[];
  /** BTC futures paper desk: fetch dev replay API (does not touch live engine state). */
  deskReplay?: DeskReplayPanelOptions;
};

function formatClock(value: number | string) {
  const date = typeof value === "number" ? new Date(value) : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return date.toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit" });
}

function formatClosedAt(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("en-IN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function fmtUsd(n: number) {
  const abs = Math.abs(n).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return `${n >= 0 ? "+" : "-"}$${abs}`;
}

export default function ReplayBacktestPanel({
  workspaceLabel,
  summary,
  priceSeries,
  events,
  deskReplay,
}: Props) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [cursor, setCursor] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [isPlaying, setIsPlaying] = useState(false);

  const [replayLoading, setReplayLoading] = useState(false);
  const [replayError, setReplayError] = useState<string | null>(null);
  const [replayResult, setReplayResult] = useState<PaperReplayApiSuccess | null>(null);

  const [slippageOverride, setSlippageOverride] = useState<string>("");
  const [volSizedOverride, setVolSizedOverride] = useState<boolean | null>(null);
  const [drawdownLock, setDrawdownLock] = useState(false);
  const [autoDisable, setAutoDisable] = useState(false);

  const replayEnabled = Boolean(deskReplay) && isDeskReplayUiEnabled();
  const fixture: DeskReplayFixtureKind = deskReplay?.fixture ?? "live";

  const visibleSeries = priceSeries.slice(-90);

  const runDeskReplay = useCallback(async () => {
    if (!deskReplay || !isDeskReplayUiEnabled()) return;
    setReplayLoading(true);
    setReplayError(null);
    try {
      const slippageBps =
        slippageOverride.trim() !== ""
          ? Number(slippageOverride)
          : deskSlippageBpsFromEnv();
      const volSized = volSizedOverride ?? deskVolSizedNotionalEnabledFromEnv();
      const params = buildDeskReplaySearchParams({
        symbol: deskReplay.symbol ?? "BTCUSD",
        bars: deskReplay.bars ?? 500,
        fixture,
        slippageBps: Number.isFinite(slippageBps) ? slippageBps : deskSlippageBpsFromEnv(),
        volSized,
        drawdownLock,
        autoDisable,
        accountKey: deskReplay.accountKey,
      });
      const res = await fetch(`/api/paper-replay?${params.toString()}`, { cache: "no-store" });
      const body: unknown = await res.json();
      const parsed = parsePaperReplayApiResponse(body);
      if (!parsed.ok) {
        const err =
          !res.ok && body && typeof body === "object" && "error" in body && typeof (body as { error: string }).error === "string"
            ? (body as { error: string }).error
            : parsed.error;
        setReplayError(replayErrorWithFixtureHint(err, fixture));
        setReplayResult(null);
        return;
      }
      setReplayResult(parsed.data);
      if (parsed.data.trades.length === 0 && fixture === "live") {
        setReplayError(REPLAY_EMPTY_FIXTURE_HINT);
      }
    } catch (e) {
      setReplayError(replayErrorWithFixtureHint(e instanceof Error ? e.message : String(e), fixture));
      setReplayResult(null);
    } finally {
      setReplayLoading(false);
    }
  }, [deskReplay, fixture, slippageOverride, volSizedOverride, drawdownLock, autoDisable]);

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

  const replaySummary = replayResult ? formatReplaySummary(replayResult.stats) : null;
  const replayRows = replayResult ? mapReplayTradesToTableRows(replayResult.trades) : [];

  const onPlayClick = () => {
    if (replayEnabled) {
      void runDeskReplay();
      return;
    }
    setIsPlaying((value) => !value);
  };

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Replay & backtest"
        subtitle={workspaceLabel}
        actions={
          <>
            {deskReplay ? <DeskChip tone="warning">Dev API</DeskChip> : null}
            <DeskButton variant="outlined" onClick={() => setIsExpanded((value) => !value)}>
              {isExpanded ? "Hide replay" : "Open replay"}
            </DeskButton>
            {replayEnabled ? (
              <DeskButton onClick={() => void runDeskReplay()} disabled={replayLoading}>
                {replayLoading ? "Running…" : "Run replay"}
              </DeskButton>
            ) : (
              <DeskButton onClick={onPlayClick} disabled={!isExpanded || visibleSeries.length < 2}>
                {isPlaying ? "Pause" : "Play"}
              </DeskButton>
            )}
          </>
        }
      />
      <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)", marginBottom: 12, maxWidth: 820 }}>
        {summary}
      </p>
      {deskReplay && !isDeskReplayUiEnabled() ? (
        <DeskBanner variant="warning" title="Replay API disabled">
          Enable desk replay in development (see README for the replay UI environment flag).
        </DeskBanner>
      ) : null}

      {replayEnabled && replaySummary ? (
        <div className="mt-4 rounded-[16px] border border-zinc-200 bg-zinc-50/80 px-4 py-3 text-xs text-zinc-700">
          <div className="flex flex-wrap gap-x-4 gap-y-1">
            <span>
              Trades: <span className="font-mono font-medium text-zinc-900">{replaySummary.tradeCount}</span>
            </span>
            <span>
              Sum net:{" "}
              <span
                className={`font-mono font-medium ${replaySummary.sumNet >= 0 ? "text-emerald-700" : "text-rose-700"}`}
              >
                {fmtUsd(replaySummary.sumNet)}
              </span>
            </span>
            <span>
              Expectancy:{" "}
              <span
                className={`font-mono font-medium ${replaySummary.expectancy >= 0 ? "text-emerald-700" : "text-rose-700"}`}
              >
                {fmtUsd(replaySummary.expectancy)}
              </span>
            </span>
            <span className="text-zinc-500">
              {replayResult?.symbol} · {replayResult?.bars} bars · bal {fmtUsd(replayResult?.finalBalance ?? 0)}
            </span>
          </div>
          <p className="mt-1 font-mono text-[10px] leading-relaxed text-zinc-600">{replaySummary.exitReasonLine}</p>
        </div>
      ) : null}

      {replayEnabled && replayLoading ? (
        <div className="mt-3 flex items-center gap-2 text-sm text-zinc-500">
          <span className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-zinc-300 border-t-zinc-700" />
          Running offline desk replay…
        </div>
      ) : null}

      {replayEnabled && replayError ? (
        <pre className="mt-3 whitespace-pre-wrap rounded-[12px] border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-800">
          {replayError}
        </pre>
      ) : null}

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
          <div className="space-y-5">
            {replayEnabled ? (
              <div className="glass-panel px-5 py-5">
                <div className="mb-3 text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                  Desk paper replay (offline)
                </div>
                <div className="mb-3 flex flex-wrap gap-3 text-xs text-zinc-600">
                  <label className="flex items-center gap-1.5">
                    <span>Slip bps</span>
                    <input
                      type="number"
                      min={0}
                      max={50}
                      placeholder={String(deskSlippageBpsFromEnv())}
                      value={slippageOverride}
                      onChange={(e) => setSlippageOverride(e.target.value)}
                      className="w-16 rounded border border-zinc-200 px-1.5 py-0.5 font-mono text-zinc-800"
                    />
                  </label>
                  <label className="flex items-center gap-1.5">
                    <input
                      type="checkbox"
                      checked={volSizedOverride ?? deskVolSizedNotionalEnabledFromEnv()}
                      onChange={(e) => setVolSizedOverride(e.target.checked)}
                    />
                    Vol-sized
                  </label>
                  <label className="flex items-center gap-1.5">
                    <input type="checkbox" checked={drawdownLock} onChange={(e) => setDrawdownLock(e.target.checked)} />
                    Drawdown lock
                  </label>
                  <label className="flex items-center gap-1.5">
                    <input type="checkbox" checked={autoDisable} onChange={(e) => setAutoDisable(e.target.checked)} />
                    Auto-disable (14d)
                  </label>
                </div>
                {replayRows.length > 0 ? (
                  <div className="max-h-[280px] overflow-auto rounded border border-zinc-200 bg-white">
                    <table className="w-full min-w-[520px] text-left text-[10px]">
                      <thead className="sticky top-0 border-b border-zinc-200 bg-zinc-50 text-zinc-500">
                        <tr>
                          <th className="px-2 py-1.5 font-semibold">Closed</th>
                          <th className="px-2 py-1.5 font-semibold">Strategy</th>
                          <th className="px-2 py-1.5 font-semibold">Side</th>
                          <th className="px-2 py-1.5 font-semibold text-right">Net</th>
                          <th className="px-2 py-1.5 font-semibold">Exit</th>
                        </tr>
                      </thead>
                      <tbody className="text-zinc-800">
                        {replayRows.map((row, i) => (
                          <tr key={`${row.closedAt}-${row.strategyName}-${i}`} className="border-b border-zinc-50">
                            <td className="px-2 py-1 font-mono whitespace-nowrap">{formatClosedAt(row.closedAt)}</td>
                            <td className="max-w-[160px] truncate px-2 py-1" title={row.strategyName}>
                              {row.strategyName}
                            </td>
                            <td className="px-2 py-1">{row.side}</td>
                            <td
                              className={`px-2 py-1 text-right font-mono ${row.netPnl >= 0 ? "text-emerald-700" : "text-rose-700"}`}
                            >
                              {fmtUsd(row.netPnl)}
                            </td>
                            <td className="px-2 py-1 font-mono">{row.exitReason}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <p className="text-sm text-zinc-500">
                    Run replay to load closed trades from the fixture. {REPLAY_EMPTY_FIXTURE_HINT}
                  </p>
                )}
              </div>
            ) : null}

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
    </DeskCard>
  );
}
