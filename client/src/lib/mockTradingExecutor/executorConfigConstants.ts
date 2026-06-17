import { DEFAULT_MOCK_TRADING_CONFIG } from "@/lib/trading/mockTradingEngine";
import { btcFtSignalThresholdFromEnv } from "@/lib/trading/futuresDeskPolicy";

export const SIGNAL_THRESHOLD_MIN = 18;
export const SIGNAL_THRESHOLD_MAX = 32;
export const MIN_SIGNAL_SCORE_MIN = 30;
export const MIN_SIGNAL_SCORE_MAX = 70;

export const EXECUTOR_CONFIG_CACHE_TTL_MS = 5_000;

export type ExecutorConfigSource = "mongodb" | "env" | "fallback";

export type ExecutorThresholdConfigDoc = {
  account_key: string;
  signal_threshold: number;
  min_signal_score: number;
  updated_at: number;
  updated_by?: string;
  notes?: string;
};

export type ExecutorConfigView = {
  accountKey: string;
  signalThreshold: number;
  minSignalScore: number;
  updatedAt: number;
  source: ExecutorConfigSource;
  updatedBy?: string;
  notes?: string;
};

export type ExecutorConfigChangeDoc = {
  account_key: string;
  timestamp: number;
  changes: Record<string, { old: number; new: number }>;
  reason?: string;
  user_id?: string;
};

export type ExecutorConfigChangeView = {
  id: string;
  accountKey: string;
  timestamp: number;
  changes: Record<string, { old: number; new: number }>;
  reason?: string;
  userId?: string;
};

export function clampSignalThreshold(value: number): number {
  if (!Number.isFinite(value)) return btcFtSignalThresholdFromEnv(26);
  return Math.min(SIGNAL_THRESHOLD_MAX, Math.max(SIGNAL_THRESHOLD_MIN, Math.round(value)));
}

export function clampMinSignalScore(value: number): number {
  if (!Number.isFinite(value)) return DEFAULT_MOCK_TRADING_CONFIG.minSignalScore;
  return Math.min(MIN_SIGNAL_SCORE_MAX, Math.max(MIN_SIGNAL_SCORE_MIN, Math.round(value)));
}

export function validateExecutorThresholdInput(
  signalThreshold: number,
  minSignalScore: number,
): string | null {
  if (
    !Number.isFinite(signalThreshold) ||
    signalThreshold < SIGNAL_THRESHOLD_MIN ||
    signalThreshold > SIGNAL_THRESHOLD_MAX
  ) {
    return `signalThreshold must be ${SIGNAL_THRESHOLD_MIN}–${SIGNAL_THRESHOLD_MAX}`;
  }
  if (
    !Number.isFinite(minSignalScore) ||
    minSignalScore < MIN_SIGNAL_SCORE_MIN ||
    minSignalScore > MIN_SIGNAL_SCORE_MAX
  ) {
    return `minSignalScore must be ${MIN_SIGNAL_SCORE_MIN}–${MIN_SIGNAL_SCORE_MAX}`;
  }
  return null;
}

export function defaultExecutorThresholdConfig(accountKey: string): ExecutorThresholdConfigDoc {
  const envThreshold = btcFtSignalThresholdFromEnv(26);
  return {
    account_key: accountKey,
    signal_threshold: clampSignalThreshold(envThreshold),
    min_signal_score: clampMinSignalScore(DEFAULT_MOCK_TRADING_CONFIG.minSignalScore),
    updated_at: Date.now(),
    updated_by: "fallback",
  };
}

export function toExecutorConfigView(
  doc: ExecutorThresholdConfigDoc,
  source: ExecutorConfigSource,
): ExecutorConfigView {
  return {
    accountKey: doc.account_key,
    signalThreshold: doc.signal_threshold,
    minSignalScore: doc.min_signal_score,
    updatedAt: doc.updated_at,
    source,
    updatedBy: doc.updated_by,
    notes: doc.notes,
  };
}
