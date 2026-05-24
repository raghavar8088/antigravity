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

/** Percent of evalPairs accounted for by the dominant blocker. */
function blockerSharePct(debug: DeskEntryPollDebug, blockerKey: string): number | null {
  const map: Record<string, keyof DeskEntryPollDebug> = {
    SIGNAL: "failSignal",
    CONFIRM: "failConfirm",
    REGIME: "failOpenRegime",
    MIN_MOVE: "failMinMove",
    SPREAD: "failSpread",
    SESSION: "failSession",
    CATEGORY: "failCategoryCap",
    DISABLED: "failDisabled",
    COOLDOWN: "failCooldown",
    OCCUPIED: "failOccupied",
    LOW_PRIORITY: "failLowPriority",
  };
  const key = map[blockerKey];
  if (!key || !debug.evalPairs) return null;
  const val = debug[key] as number;
  if (!val) return null;
  return Math.round((val / debug.evalPairs) * 100);
}

function blockerHint(blocker: string): string {
  switch (blocker) {
    case "SIGNAL":
      return "Lower NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD (default 26 → try 24) or wait for volatile conditions.";
    case "CONFIRM":
      return "HTF/confluence extras failing. Enable NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM=1 (dev only) or wait for trend.";
    case "REGIME":
      return "Market is in chop but strategies require trend regime. Wait for breakout or widen DESK_REGIME_EXTRA_TOKENS.";
    case "MIN_MOVE":
      return "ATR below fee hurdle. Reduce NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K or wait for volatility.";
    case "SESSION":
      return "Entry UTC window closed. Check NEXT_PUBLIC_DESK_ENTRY_UTC_START / END.";
    case "SPREAD":
      return "Last-mark spread too wide. Lower NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT or wait.";
    case "CATEGORY":
      return "Category cap reached. Raise NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY or wait for closes.";
    case "INTRADAY_DD":
      return "Daily −2% soft lock tripped — paper desk disables this; redeploy latest BTC FT build or wait until UTC midnight.";
    case "PAUSED":
    case "DRAWDOWN":
      return "Resume entries via the button above or wait for equity recovery.";
    default:
      return "See README#btc-ft-no-trades for full troubleshooting table.";
  }
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
  const blocker = poll?.dominantBlocker ?? "NONE";
  const blockerPct = poll ? blockerSharePct(poll, blocker) : null;
  const isGateBlocker = blocker === "PAUSED" || blocker === "DRAWDOWN" || blocker === "DATA" || blocker === "NO_STRATEGIES";
  const isActiveBlocker = blocker !== "NONE" && blocker !== "OPEN_GATE";

  return (
    <DeskCard>
      <DeskSectionHeader
        title="Entry debug (last poll)"
        subtitle="NEXT_PUBLIC_DESK_ENTRY_DEBUG=1 — paper desk only, not exchange orders"
      />

      {/* ── Dominant blocker prominent callout ── */}
      {poll && isActiveBlocker && (
        <div style={{ marginBottom: 12 }}>
          <DeskBanner
            variant={isGateBlocker ? "warning" : "info"}
            title={
              `Dominant blocker: ${blocker}` +
              (blockerPct !== null ? ` (${blockerPct}% of evals)` : "")
            }
          >
            {blockerHint(blocker)}{" "}
            <a
              href="#btc-ft-no-trades"
              style={{ textDecoration: "underline", fontWeight: 600 }}
              aria-label="How to fix BTC FT no-trades"
            >
              How to fix ↗
            </a>
          </DeskBanner>
        </div>
      )}

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
            <DeskMetricTile
              label="Dominant block"
              value={blocker}
              detail={blockerPct !== null ? `${blockerPct}% of evals` : undefined}
              compact
            />
            <DeskMetricTile
              label="UTC session"
              value={poll.entryUtcSessionOpen ? "open" : "closed"}
              detail={`UTC ${poll.utcHour}; skipped ${poll.sessionSkipTotal}`}
              compact
            />
            <DeskMetricTile
              label="Disabled"
              value={`${poll.disabledStratCount}/${poll.autoDisabledStratCount}`}
              detail="manual/auto"
              compact
            />
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
