/** Dev/ops: last poll entry funnel when `NEXT_PUBLIC_DESK_ENTRY_DEBUG=1`. */

export function deskEntryDebugEnabledFromEnv(): boolean {
  return process.env.NEXT_PUBLIC_DESK_ENTRY_DEBUG === "1";
}

export type DeskEntryPollDebug = {
  pollAt: number;
  pauseEntries: boolean;
  drawdownLocked: boolean;
  /** P2.3.4 intraday −2% soft lock — pauses entries until UTC midnight when tripped. */
  intradayDdLocked: boolean;
  hasMarketData: boolean;
  payloadsReady: number;
  symbolsRequested: number;
  dataHealthStatus: string;
  activeStratCount: number;
  disabledStratCount: number;
  autoDisabledStratCount: number;
  effectiveThreshold: number;
  utcHour: number;
  entryUtcSessionOpen: boolean;
  sessionSkipTotal: number;
  dominantBlocker: string;
  evalPairs: number;
  failDisabled: number;
  failOccupied: number;
  failCooldown: number;
  failSignal: number;
  failConfirm: number;
  candidatesBuilt: number;
  openAttempts: number;
  openedThisPoll: number;
  failSpread: number;
  failSession: number;
  failCategoryCap: number;
  failOpenRegime: number;
  failMinMove: number;
  failSameDirCap: number;
  failMaxLoss: number;
  failMargin: number;
  failLowPriority: number;
};

type DebugCountKey =
  | "failDisabled"
  | "failOccupied"
  | "failCooldown"
  | "failSignal"
  | "failConfirm"
  | "failSpread"
  | "failSession"
  | "failCategoryCap"
  | "failOpenRegime"
  | "failMinMove"
  | "failSameDirCap"
  | "failMaxLoss"
  | "failMargin"
  | "failLowPriority";

const DEBUG_BLOCKER_LABELS: ReadonlyArray<{ key: DebugCountKey; label: string }> = [
  { key: "failSignal", label: "SIGNAL" },
  { key: "failConfirm", label: "CONFIRM" },
  { key: "failOpenRegime", label: "REGIME" },
  { key: "failMinMove", label: "MIN_MOVE" },
  { key: "failSpread", label: "SPREAD" },
  { key: "failSession", label: "SESSION" },
  { key: "failCategoryCap", label: "CATEGORY_CAP" },
  { key: "failSameDirCap", label: "SAME_DIR_CAP" },
  { key: "failMaxLoss", label: "MAX_LOSS" },
  { key: "failMargin", label: "MARGIN" },
  { key: "failLowPriority", label: "LOW_PRIORITY" },
  { key: "failDisabled", label: "DISABLED" },
  { key: "failCooldown", label: "COOLDOWN" },
  { key: "failOccupied", label: "OCCUPIED" },
];

export function dominantEntryBlocker(debug: DeskEntryPollDebug): string {
  if (debug.pauseEntries) return "PAUSE";
  if (debug.intradayDdLocked) return "INTRADAY_DD";
  if (debug.drawdownLocked) return "DRAWDOWN";
  if (!debug.hasMarketData) return "DATA";
  if (debug.activeStratCount <= 0) return "NO_STRATEGIES";
  let best = { label: "NONE", value: 0 };
  for (const row of DEBUG_BLOCKER_LABELS) {
    const value = debug[row.key];
    if (value > best.value) best = { label: row.label, value };
  }
  if (debug.candidatesBuilt > 0 && debug.openedThisPoll <= 0 && best.value <= 0) return "OPEN_GATE";
  return best.value > 0 ? best.label : "NONE";
}

export function finalizeEntryPollDebug(debug: DeskEntryPollDebug): DeskEntryPollDebug {
  return {
    ...debug,
    dominantBlocker: dominantEntryBlocker(debug),
  };
}

export function createEmptyEntryPollDebug(effectiveThreshold: number): DeskEntryPollDebug {
  return {
    pollAt: 0,
    pauseEntries: false,
    drawdownLocked: false,
    intradayDdLocked: false,
    hasMarketData: false,
    payloadsReady: 0,
    symbolsRequested: 0,
    dataHealthStatus: "unknown",
    activeStratCount: 0,
    disabledStratCount: 0,
    autoDisabledStratCount: 0,
    effectiveThreshold,
    utcHour: 0,
    entryUtcSessionOpen: true,
    sessionSkipTotal: 0,
    dominantBlocker: "NONE",
    evalPairs: 0,
    failDisabled: 0,
    failOccupied: 0,
    failCooldown: 0,
    failSignal: 0,
    failConfirm: 0,
    candidatesBuilt: 0,
    openAttempts: 0,
    openedThisPoll: 0,
    failSpread: 0,
    failSession: 0,
    failCategoryCap: 0,
    failOpenRegime: 0,
    failMinMove: 0,
    failSameDirCap: 0,
    failMaxLoss: 0,
    failMargin: 0,
    failLowPriority: 0,
  };
}
