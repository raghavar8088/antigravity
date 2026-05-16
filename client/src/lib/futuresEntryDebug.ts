/** Dev/ops: last poll entry funnel when `NEXT_PUBLIC_DESK_ENTRY_DEBUG=1`. */

export function deskEntryDebugEnabledFromEnv(): boolean {
  return process.env.NEXT_PUBLIC_DESK_ENTRY_DEBUG === "1";
}

export type DeskEntryPollDebug = {
  pollAt: number;
  pauseEntries: boolean;
  drawdownLocked: boolean;
  hasMarketData: boolean;
  payloadsReady: number;
  symbolsRequested: number;
  dataHealthStatus: string;
  activeStratCount: number;
  effectiveThreshold: number;
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

export function createEmptyEntryPollDebug(effectiveThreshold: number): DeskEntryPollDebug {
  return {
    pollAt: 0,
    pauseEntries: false,
    drawdownLocked: false,
    hasMarketData: false,
    payloadsReady: 0,
    symbolsRequested: 0,
    dataHealthStatus: "unknown",
    activeStratCount: 0,
    effectiveThreshold,
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
