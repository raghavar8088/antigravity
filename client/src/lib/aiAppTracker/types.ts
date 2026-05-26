/**
 * AI App Tracker — type definitions.
 * Pure types only. No I/O, no MongoDB imports.
 */

export type AiTrackerModule = "btc_future_trading";

export type AiTrackerSeverity = "info" | "warning" | "danger";

/** Warning codes — each maps to a recommendation in summarizeTrackerReport. */
export type AiTrackerWarningCode =
  | "stale_worker"
  | "stale_deployment"
  | "no_trades_30min"
  | "balance_drift"
  | "probe_dominant"
  | "high_fee_gross"
  | "no_strategy_candidates"
  | "no_open_positions_long"
  | "data_gap";

export type AiTrackerWarning = {
  code: AiTrackerWarningCode;
  message: string;
};

/** Key env mode flags captured at snapshot time. */
export type AiTrackerEnvFlags = {
  profitMode: boolean;
  winnersOnly: boolean;
  workerEnabled: boolean;
  researchMode: boolean;
  entryCanary: boolean;
};

/** Rotation summary — nullable when not yet computed. */
export type AiTrackerRotationSummary = {
  active: number;
  suspended: number;
  promoted: number;
  topStrategyId: number | null;
};

/** Per-capture snapshot of the live desk state. */
export type AiTrackerSnapshot = {
  capturedAt: string;
  buildSha: string | null;
  /** Last 4 chars of account key only. Never store the full key. */
  accountKeySuffix: string | null;

  workerLastPollAgeSeconds: number | null;
  workerOwner: "browser" | "vps" | null;

  funnelTickAt: number | null;
  funnelTickAgeSeconds: number | null;
  dominantBlocker: string;
  recommendation: string;
  activeStrategies: number;
  evaluatedStrategies: number;
  signalPassed: number;
  opened: number;

  balance: number | null;
  balanceDriftUsd: number | null;
  openPositionsCount: number;
  closedTradesCount: number;
  lastTradeAt: string | null;

  rotationSummary: AiTrackerRotationSummary | null;

  envFlags: AiTrackerEnvFlags;
  warnings: AiTrackerWarning[];
};

/** The full document stored in ai_app_tracker_reports. */
export type AiTrackerReport = {
  report_id: string;
  created_at: string;
  app_build_sha: string | null;
  account_key_suffix: string | null;
  module: AiTrackerModule;
  severity: AiTrackerSeverity;
  summary: string;
  snapshot: AiTrackerSnapshot;
  recommendations: string[];
};
