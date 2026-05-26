/**
 * AI App Tracker — type definitions.
 * Pure types only. No I/O, no MongoDB imports.
 */

export type AiTrackerSeverity = "info" | "warning" | "danger";

export interface AiAppTrackerSnapshot {
  createdAt: string;
  appBuildSha: string | null;
  module: "btc_future_trading";
  /** Last 4 chars of account key only — never the full key. */
  accountKeySuffix: string | null;

  worker: {
    enabled: boolean;
    stale: boolean;
    lastPollAt: number | null;
    ageSeconds: number | null;
    owner: string | null;
  };

  entryFunnel: {
    ageSeconds: number | null;
    dominantBlocker: string | null;
    recommendation: string | null;
    activeStrategies: number | null;
    evaluatedStrategies: number | null;
    candidates: number | null;
    opened: number | null;
  };

  paperState: {
    balance: number | null;
    openPositions: number;
    workerLastPollAt: number | null;
    balanceDriftUsd: number | null;
    pauseEntries: boolean;
    clearedAt: number | null;
  };

  env: {
    profitMode: boolean;
    winnersOnly: boolean;
    researchMode: boolean;
    workerEnabled: boolean;
  };

  /** Human-readable warning strings. No secrets, no full keys. */
  warnings: string[];
}

export interface AiAppTrackerReport {
  report_id: string;
  created_at: string;
  app_build_sha: string | null;
  account_key_suffix: string | null;
  module: "btc_future_trading";
  severity: AiTrackerSeverity;
  summary: string;
  snapshot: AiAppTrackerSnapshot;
  recommendations: string[];
}
