"use client";

import { DeskBanner, DeskCard, DeskMetricTile, DeskSectionHeader } from "@/components/desk/ui";
import { deskEntryDebugEnabledFromEnv, type DeskEntryPollDebug } from "@/lib/futuresEntryDebug";

type EntryDebugPanelProps = {
  entryDebug: DeskEntryPollDebug | null;
  sessionSkips: {
    minMove: number;
    regime: number;
    spread: number;
    session: number;
    category: number;
    lowPriority: number;
    regimeBreakdown: string;
  };
  pauseEntries: boolean;
  drawdownLocked: boolean;
};

function topSkipRows(debug: DeskEntryPollDebug) {
  return [
    { label: "Signal < threshold", value: debug.failSignal },
    { label: "Confirm failed", value: debug.failConfirm },
    { label: "Open: regime", value: debug.failOpenRegime },
    { label: "Open: min move/fees", value: debug.failMinMove },
    { label: "Open: spread", value: debug.failSpread },
    { label: "Open: UTC session", value: debug.failSession },
    { label: "Open: category cap", value: debug.failCategoryCap },
    { label: "Disabled / auto-off", value: debug.failDisabled },
    { label: "Cooldown", value: debug.failCooldown },
    { label: "Slot occupied", value: debug.failOccupied },
    { label: "Low priority / full", value: debug.failLowPriority },
  ]
    .filter((r) => r.value > 0)
    .sort((a, b) => b.value - a.value)
    .slice(0, 6);
}

export function EntryDebugPanel({
  entryDebug,
  sessionSkips,
  pauseEntries,
  drawdownLocked,
}: EntryDebugPanelProps) {
  if (!deskEntryDebugEnabledFromEnv()) return null;

  const poll = entryDebug;
  const top = poll ? topSkipRows(poll) : [];

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Entry debug (last poll)"
        subtitle="NEXT_PUBLIC_DESK_ENTRY_DEBUG=1 — paper desk only, not exchange orders"
      />
      {(pauseEntries || drawdownLocked) && (
        <div style={{ marginBottom: 12 }}>
          <DeskBanner variant="warning" title="Entries gated">
            {pauseEntries ? "Pause entries is ON. " : null}
            {drawdownLocked ? "Drawdown lock is active (25% peak-to-trough). " : null}
            Resume entries or reset account after reviewing equity.
          </DeskBanner>
        </div>
      )}
      {!poll ? (
        <p className="desk-body-md" style={{ color: "var(--desk-on-surface-variant)" }}>
          Waiting for first poll…
        </p>
      ) : (
        <>
          <div className="desk-metrics-row" style={{ marginBottom: 12 }}>
            <DeskMetricTile label="Feed" value={poll.dataHealthStatus} detail={`${poll.payloadsReady}/${poll.symbolsRequested} symbols`} compact />
            <DeskMetricTile label="Threshold" value={String(poll.effectiveThreshold)} compact />
            <DeskMetricTile label="Strategies" value={String(poll.activeStratCount)} compact />
            <DeskMetricTile label="Eval pairs" value={String(poll.evalPairs)} compact />
            <DeskMetricTile label="Candidates" value={String(poll.candidatesBuilt)} compact />
            <DeskMetricTile label="Opened" value={String(poll.openedThisPoll)} compact />
          </div>
          {top.length > 0 ? (
            <div className="desk-metrics-row" style={{ marginBottom: 12 }}>
              {top.map((r) => (
                <DeskMetricTile key={r.label} label={r.label} value={String(r.value)} compact />
              ))}
            </div>
          ) : (
            <p className="desk-label-md" style={{ marginBottom: 12 }}>
              No skip counters on last poll — check pause/drawdown or insufficient bars.
            </p>
          )}
        </>
      )}
      <p className="desk-label-md" style={{ marginTop: 8, color: "var(--desk-on-surface-variant)" }}>
        Session cumulative skips: regime {sessionSkips.regime}
        {sessionSkips.regimeBreakdown ? ` (${sessionSkips.regimeBreakdown})` : ""} · min-move {sessionSkips.minMove} · spread{" "}
        {sessionSkips.spread} · UTC {sessionSkips.session} · category {sessionSkips.category} · priority {sessionSkips.lowPriority}
      </p>
    </DeskCard>
  );
}
