import type { EntryFunnelSnapshot } from "@/lib/trading/deskEntryFunnelSnapshot";
import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import type { SignalTraceSummary } from "@/lib/ai/strategySignalTrace";

export type MockExecutorMode = "cron" | "worker" | "manual";

export type MockExecutorCycleDiagnostics = {
  tickAt: number;
  durationMs: number;
  errors: string[];
  markPrice: number;
  regime: string;
  bars: number;
  candidateCount: number;
  evaluatedStrategies: number;
  activeStrategies: number;
  dominantBlocker: string;
  blockerCounts: Record<string, number>;
  signalSummary: SignalTraceSummary | null;
};

export type MockExecutorCycleResult = {
  ok: boolean;
  accountKey: string;
  mode: MockExecutorMode;
  opened: MockTrade[];
  closed: MockTrade[];
  updatedOpen: number;
  funnel: EntryFunnelSnapshot | null;
  diagnostics: MockExecutorCycleDiagnostics;
};

export type MockExecutorStateDoc = {
  account_key: string;
  last_tick_at: number;
  last_duration_ms: number;
  last_mode: MockExecutorMode;
  last_opened: number;
  last_closed: number;
  last_candidate_count: number;
  last_dominant_blocker: string;
  last_error: string | null;
  entry_funnel_snapshot: EntryFunnelSnapshot | null;
  updated_at: string;
};

export type MockExecutionLogDoc = {
  account_key: string;
  timestamp: number;
  mode: MockExecutorMode;
  candidate_count: number;
  opened_count: number;
  closed_count: number;
  diagnostics: MockExecutorCycleDiagnostics;
  created_at: string;
};

export type MockExecutorHealth = {
  healthy: boolean;
  stale: boolean;
  ageSeconds: number | null;
  lastTickAt: number | null;
  lastMode: MockExecutorMode | null;
  dominantBlocker: string | null;
  candidateCount: number | null;
  openedLastCycle: number | null;
  closedLastCycle: number | null;
  errors: string[];
};
