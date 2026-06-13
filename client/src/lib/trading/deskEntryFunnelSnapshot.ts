/**
 * Entry funnel snapshot — unified diagnostic for BOTH the browser engine poll
 * and the headless VPS/cron worker tick.
 *
 * Rules:
 *  - Pure functions only. No I/O, no React, no MongoDB imports.
 *  - Both callers build a snapshot per tick and persist it to
 *    paper_state.entry_funnel_snapshot so /api/desk-entry-funnel can serve it.
 *  - dominantBlocker drives the UI "Why no BTC trades?" panel and the worker
 *    console log line; never says "lower threshold".
 */

export type EntryFunnelBlockerKey =
  | "noData"
  | "noStrategies"
  | "signal"
  | "confirm"
  | "regime"
  | "atrFees"
  | "rotation"
  | "suspended"
  | "spread"
  | "session"
  | "category"
  | "sameSide"
  | "margin"
  | "maxOpen"
  | "cooldown"
  | "none";

export type FunnelWorkerMode = "browser" | "worker" | "cron";

export interface EntryFunnelBlockerCounts {
  noData: number;
  noStrategies: number;
  signal: number;
  confirm: number;
  regime: number;
  atrFees: number;
  rotation: number;
  suspended: number;
  spread: number;
  session: number;
  category: number;
  sameSide: number;
  margin: number;
  maxOpen: number;
  cooldown: number;
}

export interface EntryFunnelSnapshot {
  tickAt: number;
  workerMode: FunnelWorkerMode;
  workerFresh: boolean;
  symbol: string;
  markPrice: number;
  bars: number;
  activeStrategies: number;
  evaluatedStrategies: number;
  signalPassed: number;
  confirmPassed: number;
  candidateCount: number;
  openAttempts: number;
  opened: number;
  blockerCounts: EntryFunnelBlockerCounts;
  dominantBlocker: EntryFunnelBlockerKey;
  recommendation: string;
  /** Set when the worker is missing a gate the browser has (parity gap). */
  workerParityWarning?: string;
}

// ── Blocker descriptions — never suggest lowering threshold ──────────────────

const BLOCKER_RECOMMENDATIONS: Record<EntryFunnelBlockerKey, string> = {
  noData:
    "Delta kline/mark feed unavailable. Check Delta API connectivity or DELTA_API_BASE_URL env.",
  noStrategies:
    "Roster empty or strategy IDs invalid. Check DESK_WORKER_STRATEGY_IDS env matches FUTURES_STRAT_DEFS.",
  signal:
    "No strategy signal passed threshold. Wait for a stronger market setup — this is the correct behavior.",
  confirm:
    "Signal fired but entry confirmation failed. Wait for all confirmation factors to align.",
  regime:
    "Current market regime blocks this roster. Wait for a trend/chop-compatible setup.",
  atrFees:
    "Expected move too small relative to fees. No trade is correct — preserving capital is right.",
  rotation:
    "Rotation gate is blocking strategies in PROBATION/SUSPENDED. Inspect Edge Candidates panel.",
  suspended:
    "Rotation has suspended most strategies. Wait for performance recovery or inspect rotation report.",
  spread:
    "Last/mark spread too wide for safe entry. Wait for tighter market conditions.",
  session:
    "Outside UTC entry session window. Entries resume when the configured session opens.",
  category:
    "Per-category concurrent position cap is full. Wait for existing positions to close.",
  sameSide:
    "Same-side correlation cap reached. Wait for directional balance before more positions.",
  margin:
    "Insufficient margin for new positions. Check account balance and open position notional.",
  maxOpen:
    "Maximum concurrent positions reached. Wait for exits before new entries.",
  cooldown:
    "All qualifying strategies are in cooldown. Wait for cooldown expiry.",
  none:
    "Conditions look favorable. Monitor for signal qualification on the next poll.",
};

// ── Helpers ───────────────────────────────────────────────────────────────────

export function emptyBlockerCounts(): EntryFunnelBlockerCounts {
  return {
    noData: 0,
    noStrategies: 0,
    signal: 0,
    confirm: 0,
    regime: 0,
    atrFees: 0,
    rotation: 0,
    suspended: 0,
    spread: 0,
    session: 0,
    category: 0,
    sameSide: 0,
    margin: 0,
    maxOpen: 0,
    cooldown: 0,
  };
}

/**
 * Derive the single most important blocker from the per-tick counts.
 * Priority order: data → roster → hard caps → signal pipeline.
 */
export function computeDominantBlocker(
  counts: EntryFunnelBlockerCounts,
  opened: number,
  activeStrategies: number,
): EntryFunnelBlockerKey {
  if (opened > 0) return "none";
  if (counts.noData > 0) return "noData";
  if (activeStrategies === 0 || counts.noStrategies > 0) return "noStrategies";

  // Hard account caps (don't blame signal if the desk was already full)
  if (counts.maxOpen > 0 && counts.signal === 0 && counts.confirm === 0) return "maxOpen";
  if (counts.margin > 0 && counts.signal === 0 && counts.confirm === 0) return "margin";

  // Session gate: if it blocked everything and nothing else ran, that's the cause
  if (counts.session > 0 && counts.signal === 0 && counts.confirm === 0) return "session";

  // Pipeline: pick the highest-count gate
  const pipeline: Array<[EntryFunnelBlockerKey, number]> = [
    ["signal", counts.signal],
    ["regime", counts.regime],
    ["confirm", counts.confirm],
    ["atrFees", counts.atrFees],
    ["cooldown", counts.cooldown],
    ["rotation", counts.rotation],
    ["suspended", counts.suspended],
    ["spread", counts.spread],
    ["category", counts.category],
    ["sameSide", counts.sameSide],
    ["session", counts.session],
    ["maxOpen", counts.maxOpen],
    ["margin", counts.margin],
  ];
  pipeline.sort((a, b) => b[1] - a[1]);
  if (pipeline[0][1] > 0) return pipeline[0][0];

  return "none";
}

export function funnelRecommendation(blocker: EntryFunnelBlockerKey): string {
  return BLOCKER_RECOMMENDATIONS[blocker] ?? BLOCKER_RECOMMENDATIONS.none;
}

/**
 * Build a complete snapshot from raw counts.
 * Callers fill all fields except dominantBlocker and recommendation, which are
 * computed deterministically from the rest.
 */
export function buildFunnelSnapshot(
  partial: Omit<EntryFunnelSnapshot, "dominantBlocker" | "recommendation">,
): EntryFunnelSnapshot {
  const dominantBlocker = computeDominantBlocker(
    partial.blockerCounts,
    partial.opened,
    partial.activeStrategies,
  );
  return {
    ...partial,
    dominantBlocker,
    recommendation: funnelRecommendation(dominantBlocker),
  };
}

/**
 * Convert the browser's DeskEntryPollDebug into a funnel snapshot so the two
 * paths share the same format without duplicating logic in the hook.
 *
 * Accepts a plain-object subset to avoid importing the full type here (keeps
 * this file free of React / engine dependencies).
 */
export function funnelSnapshotFromBrowserDebug(debug: {
  pollAt: number;
  pauseEntries: boolean;
  drawdownLocked: boolean;
  intradayDdLocked: boolean;
  hasMarketData: boolean;
  activeStratCount: number;
  evalPairs: number;
  failSignal: number;
  failConfirm: number;
  failOpenRegime: number;
  failMinMove: number;
  failSpread: number;
  failSession: number;
  failCategoryCap: number;
  failSameDirCap: number;
  failMargin: number;
  candidatesBuilt: number;
  openAttempts: number;
  openedThisPoll: number;
  failDisabled: number;
  failCooldown: number;
  failOccupied: number;
  dataHealthStatus: string;
  dominantBlocker: string;
}, opts: {
  workerFresh: boolean;
  symbol: string;
  markPrice: number;
  bars: number;
}): EntryFunnelSnapshot {
  const counts = emptyBlockerCounts();

  if (!debug.hasMarketData) counts.noData = 1;
  if (debug.activeStratCount === 0) counts.noStrategies = 1;
  counts.signal = debug.failSignal;
  counts.confirm = debug.failConfirm;
  counts.regime = debug.failOpenRegime;
  counts.atrFees = debug.failMinMove;
  counts.spread = debug.failSpread;
  counts.session = debug.failSession;
  counts.category = debug.failCategoryCap;
  counts.sameSide = debug.failSameDirCap;
  counts.margin = debug.failMargin;
  counts.cooldown = debug.failCooldown;

  // pauseEntries / drawdownLocked / intradayDdLocked → map to maxOpen (desk fully paused)
  if (debug.pauseEntries || debug.drawdownLocked || debug.intradayDdLocked) {
    counts.maxOpen += 1;
  }

  const snapshot = buildFunnelSnapshot({
    tickAt: debug.pollAt || Date.now(),
    workerMode: "browser",
    workerFresh: opts.workerFresh,
    symbol: opts.symbol,
    markPrice: opts.markPrice,
    bars: opts.bars,
    activeStrategies: debug.activeStratCount,
    evaluatedStrategies: debug.evalPairs,
    signalPassed: debug.candidatesBuilt + debug.openedThisPoll,
    confirmPassed: debug.candidatesBuilt + debug.openedThisPoll,
    candidateCount: debug.candidatesBuilt,
    openAttempts: debug.openAttempts,
    opened: debug.openedThisPoll,
    blockerCounts: counts,
    // Browser has all gates; no parity warning needed
    workerParityWarning: undefined,
  });

  return snapshot;
}
